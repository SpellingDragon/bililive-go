//go:generate mockgen -package recorders -destination mock_test.go github.com/hr3lxphr6j/bililive-go/src/recorders Recorder,Manager
package recorders

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/bluele/gcache"
	"github.com/sirupsen/logrus"

	"github.com/hr3lxphr6j/bililive-go/src/configs"
	"github.com/hr3lxphr6j/bililive-go/src/instance"
	"github.com/hr3lxphr6j/bililive-go/src/interfaces"
	"github.com/hr3lxphr6j/bililive-go/src/live"
	"github.com/hr3lxphr6j/bililive-go/src/pkg/events"
	"github.com/hr3lxphr6j/bililive-go/src/pkg/parser"
	"github.com/hr3lxphr6j/bililive-go/src/pkg/parser/ffmpeg"
	"github.com/hr3lxphr6j/bililive-go/src/pkg/parser/native/flv"
	"github.com/hr3lxphr6j/bililive-go/src/pkg/utils"
)

const (
	begin uint32 = iota
	pending
	running
	stopped
)

// for test
var (
	newParser = func(u *url.URL, useNativeFlvParser bool, cfg map[string]string) (parser.Parser, error) {
		parserName := ffmpeg.Name
		if strings.Contains(u.Path, ".flv") && useNativeFlvParser {
			parserName = flv.Name
		}
		return parser.New(parserName, cfg)
	}

	mkdir = func(path string) error {
		return os.MkdirAll(path, os.ModePerm)
	}

	removeEmptyFile = func(file string) {
		if stat, err := os.Stat(file); err == nil && stat.Size() == 0 {
			os.Remove(file)
		}
	}
)

func getDefaultFileNameTmpl(config *configs.Config) *template.Template {
	return template.Must(template.New("filename").Funcs(utils.GetFuncMap(config)).
		Parse(`{{ .Live.GetPlatformCNName }}/{{ .HostName | filenameFilter }}/[{{ now | date "2006-01-02 15-04-05"}}][{{ .HostName | filenameFilter }}][{{ .RoomName | filenameFilter }}].flv`))
}

type Recorder interface {
	Start(ctx context.Context) error
	StartTime() time.Time
	GetStatus() (map[string]string, error)
	Close()
}

type SimpleRecorder struct {
	Live       live.Live
	OutPutPath string

	Config     *configs.Config
	ed         events.Dispatcher
	logger     *interfaces.Logger
	cache      gcache.Cache
	startTime  time.Time
	parser     parser.Parser
	parserLock *sync.RWMutex

	stop  chan struct{}
	state uint32
}

func NewRecorder(ctx context.Context, live live.Live) (Recorder, error) {
	inst := instance.GetInstance(ctx)
	return &SimpleRecorder{
		Live:       live,
		OutPutPath: instance.GetInstance(ctx).Config.OutPutPath,
		Config:     inst.Config,
		cache:      inst.Cache,
		startTime:  time.Now(),
		ed:         inst.EventDispatcher.(events.Dispatcher),
		logger:     inst.Logger,
		state:      begin,
		stop:       make(chan struct{}),
		parserLock: new(sync.RWMutex),
	}, nil
}

func (r *SimpleRecorder) TryRecord(ctx context.Context) {
	urls, err := r.Live.GetStreamUrls()
	if err != nil || len(urls) == 0 {
		r.getLogger().WithError(err).Warn("failed to get stream url, will retry after 5s...")
		time.Sleep(5 * time.Second)
		return
	}

	obj, err := r.cache.Get(r.Live)
	var info *live.Info
	if err == nil && obj != nil {
		info = obj.(*live.Info)
	} else {
		info, _ = r.Live.GetInfo()
	}

	tmpl := getDefaultFileNameTmpl(r.Config)
	if r.Config.OutputTmpl != "" {
		_tmpl, err := template.New("user_filename").Funcs(utils.GetFuncMap(r.Config)).Parse(r.Config.OutputTmpl)
		if err == nil {
			tmpl = _tmpl
		}
	}

	buf := new(bytes.Buffer)
	if err = tmpl.Execute(buf, info); err != nil {
		panic(fmt.Sprintf("failed to render filename, err: %v", err))
	}
	fileName := filepath.Join(r.OutPutPath, buf.String())
	outputPath, _ := filepath.Split(fileName)
	url := urls[0]

	if strings.Contains(url.Path, "m3u8") {
		fileName = fileName[:len(fileName)-4] + ".ts"
	}

	if info.AudioOnly {
		fileName = fileName[:strings.LastIndex(fileName, ".")] + ".aac"
	}

	if err = mkdir(outputPath); err != nil {
		r.getLogger().WithError(err).Errorf("failed to create output path[%s]", outputPath)
		return
	}
	parserCfg := map[string]string{
		"timeout_in_us": strconv.Itoa(r.Config.TimeoutInUs),
	}
	if r.Config.Debug {
		parserCfg["debug"] = "true"
	}
	parser, err := newParser(url, r.Config.Feature.UseNativeFlvParser, parserCfg)
	if err != nil {
		r.getLogger().WithError(err).Error("failed to init parse")
		return
	}
	r.setAndCloseParser(parser)
	r.startTime = time.Now()
	r.getLogger().Debugln("Start ParseLiveStream(" + url.String() + ", " + fileName + ")")
	r.getLogger().Println(r.parser.ParseLiveStream(ctx, url, r.Live, fileName))
	r.getLogger().Debugln("End ParseLiveStream(" + url.String() + ", " + fileName + ")")
	removeEmptyFile(fileName)
	if r.Config.OnRecordFinished.ConvertToMp4 {
		path := instance.GetInstance(ctx).Config.FfmpegPath
		ffmpegPath, err := utils.GetFFmpegPath(path)
		if err != nil {
			r.getLogger().WithError(err).Error("failed to find ffmpeg")
			return
		}
		convertCmd := exec.Command(
			ffmpegPath,
			"-hide_banner",
			"-i",
			fileName,
			"-c",
			"copy",
			fileName+".mp4",
		)
		if err = convertCmd.Run(); err != nil {
			convertCmd.Process.Kill()
			r.getLogger().Debugln(err)
		} else if r.Config.OnRecordFinished.DeleteFlvAfterConvert {
			os.Remove(fileName)
		}
	}
}

func (r *SimpleRecorder) run(ctx context.Context) {
	for {
		select {
		case <-r.stop:
			return
		default:
			r.TryRecord(ctx)
		}
	}
}

func (r *SimpleRecorder) getParser() parser.Parser {
	r.parserLock.RLock()
	defer r.parserLock.RUnlock()
	return r.parser
}

func (r *SimpleRecorder) setAndCloseParser(p parser.Parser) {
	r.parserLock.Lock()
	defer r.parserLock.Unlock()
	if r.parser != nil {
		r.parser.Stop()
	}
	r.parser = p
}

func (r *SimpleRecorder) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapUint32(&r.state, begin, pending) {
		return nil
	}
	go r.run(ctx)
	r.getLogger().Info("Record Start")
	r.ed.DispatchEvent(events.NewEvent(RecorderStart, r.Live))
	atomic.CompareAndSwapUint32(&r.state, pending, running)
	return nil
}

func (r *SimpleRecorder) StartTime() time.Time {
	return r.startTime
}

func (r *SimpleRecorder) Close() {
	if !atomic.CompareAndSwapUint32(&r.state, running, stopped) {
		return
	}
	close(r.stop)
	if p := r.getParser(); p != nil {
		p.Stop()
	}
	r.getLogger().Info("Record End")
	r.ed.DispatchEvent(events.NewEvent(RecorderStop, r.Live))
}

func (r *SimpleRecorder) getLogger() *logrus.Entry {
	return r.logger.WithFields(r.getFields())
}

func (r *SimpleRecorder) getFields() map[string]interface{} {
	obj, err := r.cache.Get(r.Live)
	if err != nil {
		return nil
	}
	info := obj.(*live.Info)
	return map[string]interface{}{
		"host": info.HostName,
		"room": info.RoomName,
	}
}

func (r *SimpleRecorder) GetStatus() (map[string]string, error) {
	statusP, ok := r.getParser().(parser.StatusParser)
	if !ok {
		return nil, ErrParserNotSupportStatus
	}
	return statusP.Status()
}
