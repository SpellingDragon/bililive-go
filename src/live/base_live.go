package live

import (
	"fmt"
	"net/url"
	"time"

	"github.com/hr3lxphr6j/bililive-go/src/configs"
	"github.com/hr3lxphr6j/bililive-go/src/pkg/utils"
)

type BaseLive struct {
	Url           *url.URL
	LastStartTime time.Time
	LiveId        configs.ID
	Options       *Options
}

func genLiveId(url *url.URL) configs.ID {
	return genLiveIdByString(fmt.Sprintf("%s%s", url.Host, url.Path))
}

func genLiveIdByString(value string) configs.ID {
	return configs.ID(utils.GetMd5String([]byte(value)))
}

func NewBaseLive(url *url.URL, opt ...Option) BaseLive {
	return BaseLive{
		Url:     url,
		LiveId:  genLiveId(url),
		Options: MustNewOptions(opt...),
	}
}

func (a *BaseLive) SetLiveIdByString(value string) {
	a.LiveId = genLiveIdByString(value)
}

func (a *BaseLive) GetLiveId() configs.ID {
	return a.LiveId
}

func (a *BaseLive) GetRawUrl() string {
	return a.Url.String()
}

func (a *BaseLive) GetLastStartTime() time.Time {
	return a.LastStartTime
}

func (a *BaseLive) SetLastStartTime(time time.Time) {
	a.LastStartTime = time
}
