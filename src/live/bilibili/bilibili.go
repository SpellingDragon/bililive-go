package bilibili

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/hr3lxphr6j/requests"
	"github.com/tidwall/gjson"

	"github.com/hr3lxphr6j/bililive-go/src/live"
	"github.com/hr3lxphr6j/bililive-go/src/pkg/utils"
)

const (
	domain = "live.bilibili.com"
	cnName = "哔哩哔哩"

	roomInitUrl  = "https://api.live.bilibili.com/room/v1/Room/room_init"
	roomApiUrl   = "https://api.live.bilibili.com/room/v1/Room/get_info"
	userApiUrl   = "https://api.live.bilibili.com/live_user/v1/UserInfo/get_anchor_in_room"
	liveApiUrlv2 = "https://api.live.bilibili.com/xlive/web-room/v2/index/getRoomPlayInfo"
)

func init() {
	live.Register(domain, new(builder))
}

type builder struct{}

func (b *builder) Build(url *url.URL, opt ...live.Option) (live.Live, error) {
	return &Live{
		BaseLive: live.NewBaseLive(url, opt...),
	}, nil
}

type Live struct {
	live.BaseLive
	RealID string
}

// LiveURLInfoRsp
type LiveURLInfoRsp struct {
	Message string      `yaml:"message" json:"message,omitempty"`
	Ttl     int         `yaml:"ttl" json:"ttl,omitempty"`
	Data    LiveURLInfo `yaml:"data" json:"data"`
	Code    int         `yaml:"code" json:"code,omitempty"`
}

// LiveURLInfo
type LiveURLInfo struct {
	PwdVerified     bool        `yaml:"pwd_verified" json:"pwd_verified,omitempty"`
	AllSpecialTypes []int       `yaml:"all_special_types" json:"all_special_types,omitempty"`
	RoomId          int         `yaml:"room_id" json:"room_id,omitempty"`
	IsPortrait      bool        `yaml:"is_portrait" json:"is_portrait,omitempty"`
	LiveStatus      int         `yaml:"live_status" json:"live_status,omitempty"`
	HiddenTill      int         `yaml:"hidden_till" json:"hidden_till,omitempty"`
	LockTill        int         `yaml:"lock_till" json:"lock_till,omitempty"`
	Uid             int         `yaml:"uid" json:"uid,omitempty"`
	IsLocked        bool        `yaml:"is_locked" json:"is_locked,omitempty"`
	LiveTime        int         `yaml:"live_time" json:"live_time,omitempty"`
	OfficialRoomId  int         `yaml:"official_room_id" json:"official_room_id,omitempty"`
	ShortId         int         `yaml:"short_id" json:"short_id,omitempty"`
	Encrypted       bool        `yaml:"encrypted" json:"encrypted,omitempty"`
	PlayurlInfo     PlayurlInfo `yaml:"playurl_info" json:"playurl_info"`
	IsHidden        bool        `yaml:"is_hidden" json:"is_hidden,omitempty"`
	RoomShield      int         `yaml:"room_shield" json:"room_shield,omitempty"`
	OfficialType    int         `yaml:"official_type" json:"official_type,omitempty"`
}

// PlayurlInfo
type PlayurlInfo struct {
	ConfJson string  `yaml:"conf_json" json:"conf_json,omitempty"`
	Playurl  Playurl `yaml:"playurl" json:"playurl"`
}

// Playurl
type Playurl struct {
	Cid     int         `yaml:"cid" json:"cid,omitempty"`
	GQnDesc []GQnDesc   `yaml:"g_qn_desc" json:"g_qn_desc,omitempty"`
	Stream  []Stream    `yaml:"stream" json:"stream,omitempty"`
	P2pData P2pData     `yaml:"p2p_data" json:"p_2_p_data"`
	DolbyQn interface{} `yaml:"dolby_qn" json:"dolby_qn,omitempty"`
}

// P2pData
type P2pData struct {
	P2p      bool     `yaml:"p2p" json:"p_2_p,omitempty"`
	P2pType  int      `yaml:"p2p_type" json:"p_2_p_type,omitempty"`
	MP2p     bool     `yaml:"m_p2p" json:"mp_2_p,omitempty"`
	MServers []string `yaml:"m_servers" json:"m_servers,omitempty"`
}

// GQnDesc
type GQnDesc struct {
	Qn       int         `yaml:"qn" json:"qn,omitempty"`
	Desc     string      `yaml:"desc" json:"desc,omitempty"`
	HdrDesc  string      `yaml:"hdr_desc" json:"hdr_desc,omitempty"`
	AttrDesc interface{} `yaml:"attr_desc" json:"attr_desc,omitempty"`
}

// Stream
type Stream struct {
	Format       []Format `yaml:"format" json:"format,omitempty"`
	ProtocolName string   `yaml:"protocol_name" json:"protocol_name,omitempty"`
}

// Format
type Format struct {
	FormatName string  `yaml:"format_name" json:"format_name,omitempty"`
	Codec      []Codec `yaml:"codec" json:"codec,omitempty"`
}

// Codec
type Codec struct {
	CurrentQn int         `yaml:"current_qn" json:"current_qn,omitempty"`
	AcceptQn  []int       `yaml:"accept_qn" json:"accept_qn,omitempty"`
	BaseUrl   string      `yaml:"base_url" json:"base_url,omitempty"`
	UrlInfo   []UrlInfo   `yaml:"url_info" json:"url_info,omitempty"`
	HdrQn     interface{} `yaml:"hdr_qn" json:"hdr_qn,omitempty"`
	DolbyType int         `yaml:"dolby_type" json:"dolby_type,omitempty"`
	AttrName  string      `yaml:"attr_name" json:"attr_name,omitempty"`
	CodecName string      `yaml:"codec_name" json:"codec_name,omitempty"`
}

// UrlInfo
type UrlInfo struct {
	Extra     string `yaml:"extra" json:"extra,omitempty"`
	StreamTtl int    `yaml:"stream_ttl" json:"stream_ttl,omitempty"`
	Host      string `yaml:"host" json:"host,omitempty"`
}

func (l *Live) parseRealId() error {
	paths := strings.Split(l.Url.Path, "/")
	if len(paths) < 2 {
		return live.ErrRoomUrlIncorrect
	}
	cookies := l.Options.Cookies.Cookies(l.Url)
	cookieKVs := make(map[string]string)
	for _, item := range cookies {
		cookieKVs[item.Name] = item.Value
	}
	resp, err := requests.Get(roomInitUrl, live.CommonUserAgent, requests.Query("id", paths[1]),
		requests.Cookies(cookieKVs))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return live.ErrRoomNotExist
	}
	body, err := resp.Bytes()
	if err != nil || gjson.GetBytes(body, "code").Int() != 0 {
		return live.ErrRoomNotExist
	}
	l.RealID = gjson.GetBytes(body, "data.room_id").String()
	return nil
}

func (l *Live) GetInfo() (info *live.Info, err error) {
	// Parse the short id from URL to full id
	if l.RealID == "" {
		if err := l.parseRealId(); err != nil {
			return nil, err
		}
	}
	cookies := l.Options.Cookies.Cookies(l.Url)
	cookieKVs := make(map[string]string)
	for _, item := range cookies {
		cookieKVs[item.Name] = item.Value
	}
	resp, err := requests.Get(
		roomApiUrl,
		live.CommonUserAgent,
		requests.Query("room_id", l.RealID),
		requests.Query("from", "room"),
		requests.Cookies(cookieKVs),
	)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, live.ErrRoomNotExist
	}
	body, err := resp.Bytes()
	if err != nil {
		return nil, err
	}
	if gjson.GetBytes(body, "code").Int() != 0 {
		return nil, live.ErrRoomNotExist
	}

	info = &live.Info{
		Live:     l,
		RoomName: gjson.GetBytes(body, "data.title").String(),
		Status:   gjson.GetBytes(body, "data.live_status").Int() == 1,
	}

	resp, err = requests.Get(userApiUrl, live.CommonUserAgent, requests.Query("roomid", l.RealID))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, live.ErrInternalError
	}
	body, err = resp.Bytes()
	if err != nil {
		return nil, err
	}
	if gjson.GetBytes(body, "code").Int() != 0 {
		return nil, live.ErrInternalError
	}

	info.HostName = gjson.GetBytes(body, "data.info.uname").String()
	return info, nil
}

func (l *Live) GetStreamUrls() (us []*url.URL, err error) {
	if l.RealID == "" {
		if err := l.parseRealId(); err != nil {
			return nil, err
		}
	}
	var cookies []*http.Cookie
	if l.Options != nil && l.Options.Cookies != nil {
		cookies = l.Options.Cookies.Cookies(l.Url)
	}
	cookieKVs := make(map[string]string)
	for _, item := range cookies {
		cookieKVs[item.Name] = item.Value
	}
	query := fmt.Sprintf("?room_id=%s&protocol=0,1&format=0,1,2&codec=0,1&platform=web&ptype=8&dolby=5&panorama=1",
		l.RealID)
	resp, err := requests.Get(liveApiUrlv2+query, live.CommonUserAgent, requests.Cookies(cookieKVs))

	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, live.ErrRoomNotExist
	}
	body, err := resp.Bytes()
	if err != nil {
		return nil, err
	}
	urlInfoRsp := &LiveURLInfoRsp{}
	json.Unmarshal(body, urlInfoRsp)
	urls := make([]string, 0, 4)

	selectedCodec := Codec{}
	urlInfoData := urlInfoRsp.Data
	for _, stream := range urlInfoData.PlayurlInfo.Playurl.Stream {
		for _, format := range stream.Format {
			for _, codec := range format.Codec {
				if (l.Options.Quality == 0 && format.FormatName == "m3u8" && codec.CodecName == "hevc") ||
					(format.FormatName == "flv" && codec.CodecName == "avc") {
					selectedCodec = codec
				}
			}
		}
	}
	minAcceptQn := 10000
	for _, qn := range selectedCodec.AcceptQn {
		if qn < minAcceptQn {
			minAcceptQn = qn
		}
	}
	for _, urlInfo := range selectedCodec.UrlInfo {
		urls = append(urls, fmt.Sprintf("%s&qn=%d",
			urlInfo.Host+selectedCodec.BaseUrl+urlInfo.Extra, minAcceptQn))
	}

	return utils.GenUrls(urls...)
}

func (l *Live) GetPlatformCNName() string {
	return cnName
}
