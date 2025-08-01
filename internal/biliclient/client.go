package biliclient

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BYT0723/bilichat/internal/model"
	"github.com/BYT0723/go-tools/logx"
	"github.com/BYT0723/go-tools/transport/httpx"
	"github.com/gorilla/websocket"
	"github.com/iyear/biligo"
	"github.com/tidwall/gjson"
)

type Client struct {
	roomID uint32
	cli    *biligo.BiliClient
	conn   *websocket.Conn

	cookie    string
	cookies   map[string]string
	uid       uint32
	wbiImgURL string
	wbiSubURL string

	history    sync.Once
	msgCh      chan *model.Danmaku
	roomInfoCh chan *model.RoomInfo

	ctx context.Context
	cf  context.CancelFunc
}

func NewClient(cookie string, roomID uint32) (c *Client, err error) {
	cookies, err := parseCookie(cookie)
	if err != nil {
		return
	}

	cli, err := biligo.NewBiliClient(&biligo.BiliSetting{Auth: &biligo.CookieAuth{
		SESSDATA:        cookies["SESSDATA"],
		DedeUserID:      cookies["DedeUserID"],
		DedeUserIDCkMd5: cookies["DedeUserID__ckMd5"],
		BiliJCT:         cookies["bili_jct"],
	}, DebugMode: false})
	if err != nil {
		return
	}

	ctx, cf := context.WithCancel(context.Background())
	c = &Client{
		roomID:     roomID,
		cli:        cli,
		cookie:     cookie,
		cookies:    cookies,
		ctx:        ctx,
		cf:         cf,
		msgCh:      make(chan *model.Danmaku, 1024),
		roomInfoCh: make(chan *model.RoomInfo, 8),
	}
	if err = c.connect(); err != nil {
		return
	}

	// 获取房间历史弹幕
	go c.getHistoryDanmaku()
	// 定时获取房间信息
	go func(ctx context.Context) {
		ticker := time.NewTicker(30 * time.Second)
		c.syncRoomInfo()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.syncRoomInfo()
			}
		}
	}(c.ctx)

	go c.handlerMsg()
	go c.videoHeartBeat()
	return
}

func (c *Client) Stop() {
	if c.cf != nil {
		c.cf()
	}
}

func (c *Client) Receive() (msgCh <-chan *model.Danmaku, roomInfoCh <-chan *model.RoomInfo) {
	return c.msgCh, c.roomInfoCh
}

func (c *Client) SendMsg(msg string) error {
	return c.cli.LiveSendDanmaku(int64(c.roomID), 16777215, 25, 1, msg, 0)
}

func (c *Client) connect() error {
	header := http.Header{
		"Cookie":     []string{c.cookie},
		"Origin":     []string{"https://live.bilibili.com"},
		"User-Agent": []string{"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36"},
	}
	header.Set("Accept", "*/*")
	header.Set("Accept-Language", "zh-CN,zh;q=0.8,zh-TW;q=0.7,zh-HK;q=0.5,en-US;q=0.3,en;q=0.2")

	if c.uid == 0 {
		resp, err := httpx.Getx(c.ctx, "https://api.bilibili.com/x/web-interface/nav", httpx.WithHeader(header))
		if err != nil {
			return fmt.Errorf("failed to get user_id, err: %v", err)
		}
		if resp.Code != http.StatusOK || len(resp.Body) == 0 {
			return fmt.Errorf("failed to get user_id, http status: %v", resp.Code)
		}
		if code := gjson.GetBytes(resp.Body, "code").Int(); code != 0 {
			return fmt.Errorf("failed to get user_id, code: %v", code)
		}
		if wbi := gjson.GetBytes(resp.Body, "data.wbi_img"); wbi.Exists() {
			c.wbiImgURL = wbi.Get("img_url").String()
			c.wbiSubURL = wbi.Get("sub_url").String()
		} else {
			c.wbiImgURL = "https://i0.hdslb.com/bfs/wbi/7cd084941338484aae1ad9425b84077c.png"
			c.wbiSubURL = "https://i0.hdslb.com/bfs/wbi/4932caff0ff746eab6f01bf08b70ac45.png"
		}
		c.uid = uint32(gjson.GetBytes(resp.Body, "data.mid").Int())
	}

	hosts, token, err := c.getRoomStreamAddr()
	if err != nil {
		return err
	}

	if len(hosts) == 0 {
		return errors.New("failed to get wss host")
	}

	for _, h := range hosts {
		c.conn, _, err = websocket.DefaultDialer.Dial(fmt.Sprintf("wss://%s/sub", h), header)
		if err == nil {
			break
		}
	}
	if c.conn == nil {
		return fmt.Errorf("websocket connect err: %v", err)
	}
	if err := c.sendAuth(token); err != nil {
		return err
	}
	go c.connHeartBeat()
	return nil
}

// fuck 每次启动总容易失败panic
func (c *Client) handlerMsg() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			_, rawMsg, err := c.conn.ReadMessage()
			if err != nil {
				logx.Errorf("receiveRawMsg, err: %v", err)
				return
			}

			version := binary.BigEndian.Uint16(rawMsg[6:8])
			if len(rawMsg) >= 8 && version == 2 {
				for _, msg := range splitMsg(zlibUnCompress(rawMsg[16:])) {
					var (
						body = gjson.ParseBytes(msg[16:])
						dmk  = &model.Danmaku{}
						cmd  = body.Get("cmd").String()
					)

					if os.Getenv("BILICHAT_DEBUG") == "1" {
						_ = os.MkdirAll("danmaku", os.ModePerm)
						_ = os.WriteFile(fmt.Sprintf("danmaku/%s-%s.json", cmd, time.Now().Format(time.RFC3339)), msg[16:], os.ModePerm)
					}
					switch cmd {
					case "DANMU_MSG":
						dmk.Author = body.Get("info.2.1").String()
						dmk.Content = body.Get("info.1").String()
					case "SUPER_CHAT_MESSAGE", "SUPER_CHAT_MESSAGE_JPN":
						dmk.Author = fmt.Sprintf("%s [¥ %d]",
							body.Get("data.user_info.uname").String(),
							body.Get("data.price").Int(),
						)
						dmk.Content = body.Get("data.message").String()
					case "COMBO_SEND":
						dmk.Author = body.Get("data.r_uname").String()
						dmk.Content = fmt.Sprintf(
							"%s %d * %s",
							body.Get("data.action").String(),
							body.Get("data.combo_num").Int(),
							body.Get("data.gift_name").String(),
						)
					case "GUARD_BUY":
						dmk.Author = body.Get("data.username").String()
						dmk.Content = fmt.Sprintf(
							"%d * %s",
							body.Get("data.num").Int(),
							body.Get("data.gift_name").String(),
						)
					case "INTERACT_WORD":
						dmk.Author = body.Get("data.uname").String()
						dmk.Content = "进入直播间"
					case "SEND_GIFT":
						dmk.Author = body.Get("data.uname").String()
						dmk.Content = fmt.Sprintf(
							"%s %d * %s",
							body.Get("data.action").String(),
							body.Get("data.num").Int(),
							body.Get("data.giftName").String(),
						)
					default: // "LIVE" "ACTIVITY_BANNER_UPDATE_V2" "ONLINE_RANK_COUNT" "ONLINE_RANK_TOP3" "ONLINE_RANK_V2" "PANEL" "PREPARING" "WIDGET_BANNER" "LIVE_INTERACTIVE_GAME"
						continue
					}
					// GUARD_BUY        上舰长
					// USER_TOAST_MSG   续费了舰长
					// NOTICE_MSG       在本房间续费了舰长
					// ANCHOR_LOT_START 天选之人开始完整信息
					// ANCHOR_LOT_END   天选之人获奖id
					// ANCHOR_LOT_AWARD 天选之人获奖完整信息

					dmk.Content = strings.ReplaceAll(dmk.Content, "\r", "")
					dmk.Type = cmd
					dmk.T = time.Now()
					c.msgCh <- dmk
				}
			}
		}
	}
}

func (c *Client) syncRoomInfo() {
	roomInfo := new(model.RoomInfo)

	resp, err := httpx.Getx(c.ctx, "https://api.live.bilibili.com/room/v1/room/get_info", httpx.WithPayload(map[string]any{"room_id": c.roomID}))
	if err != nil {
		logx.Errorf("get room information, err: %v", err)
		return
	}
	if resp.Code != http.StatusOK || len(resp.Body) == 0 {
		logx.Errorf("get room information, status: %v", resp.Code)
		return
	}

	roomInfo.RoomId = int(c.roomID)
	roomInfo.Uid = int(gjson.Get(string(resp.Body), "data.uid").Int())
	roomInfo.Title = gjson.Get(string(resp.Body), "data.title").String()
	roomInfo.AreaName = gjson.Get(string(resp.Body), "data.area_name").String()
	roomInfo.ParentAreaName = gjson.Get(string(resp.Body), "data.parent_area_name").String()
	roomInfo.Online = gjson.Get(string(resp.Body), "data.online").Int()
	roomInfo.Attention = gjson.Get(string(resp.Body), "data.attention").Int()
	if _time, err := time.ParseInLocation(time.DateTime, gjson.Get(string(resp.Body), "data.live_time").String(), time.Local); err == nil {
		dur := time.Since(_time)
		if dur > 0 {
			roomInfo.Uptime = dur
		}
	}

	resp, err = httpx.Getx(c.ctx, "https://api.live.bilibili.com/xlive/general-interface/v1/rank/getOnlineGoldRank", httpx.WithPayload(map[string]any{
		"ruid":     roomInfo.Uid,
		"roomId":   c.roomID,
		"page":     1,
		"pageSize": 50,
	}))
	if err != nil {
		logx.Errorf("get room rank, err: %v", err)
		return
	}
	if resp.Code != http.StatusOK || len(resp.Body) == 0 {
		logx.Errorf("get room rank, status: %v", resp.Code)
		return
	}

	rawUsers := gjson.Get(string(resp.Body), "data.OnlineRankItem").Array()
	for _, rawUser := range rawUsers {
		user := model.OnlineRankUser{
			Name:  rawUser.Get("name").String(),
			Score: rawUser.Get("score").Int(),
			Rank:  rawUser.Get("userRank").Int(),
		}
		roomInfo.OnlineRankUsers = append(roomInfo.OnlineRankUsers, user)
	}
	c.roomInfoCh <- roomInfo
}

var pktSeq uint32

func (c *Client) sendPackage(ver uint16, typeID uint32, data []byte) (err error) {
	packetHead := new(bytes.Buffer)

	for _, v := range []any{
		uint32(len(data) + 16),
		uint16(16),
		ver,
		typeID,
		pktSeq,
	} {
		if err = binary.Write(packetHead, binary.BigEndian, v); err != nil {
			return
		}
	}

	atomic.AddUint32(&pktSeq, 1)

	sendData := append(packetHead.Bytes(), data...)

	err = c.conn.WriteMessage(websocket.BinaryMessage, sendData)
	return
}

func (c *Client) sendAuth(token string) (err error) {
	hsInfo := handShakeInfo{
		UID:      c.uid,
		Roomid:   c.roomID,
		Protover: 2,
		Platform: "web",
		Type:     2,
		Key:      token,
		Buvid:    c.cookies["buvid3"],
	}
	body, err := json.Marshal(hsInfo)
	if err != nil {
		return err
	}

	if err = c.sendPackage(1, 7, body); err != nil {
		return
	}

	_, rawMsg, err := c.conn.ReadMessage()
	if err != nil {
		return
	}
	if len(rawMsg) < 16 {
		return errors.New("invalid auth response")
	}
	if code := binary.BigEndian.Uint32(rawMsg[8:12]); code != 8 {
		return fmt.Errorf("invalid auth response op code: %v", code)
	}

	if code := gjson.GetBytes(rawMsg[16:], "code"); code.Exists() {
		if code.Int() != 0 {
			return fmt.Errorf("invalid auth response code: %v, resp: %s", code.Int(), rawMsg[16:])
		}
	} else {
		return errors.New("invalid auth response, not found code")
	}
	return
}

func (c *Client) getHistoryDanmaku() {
	c.history.Do(func() {
		resp, err := httpx.Getx(c.ctx, "https://api.live.bilibili.com/xlive/web-room/v1/dM/gethistory", httpx.WithPayload(map[string]any{"roomid": c.roomID}))
		if err != nil {
			logx.Errorf("getHistoryDanmaku, err: %v", err)
			return
		}
		if resp.Code != http.StatusOK || len(resp.Body) == 0 {
			logx.Errorf("getHistoryDanmaku, status: %v", resp.Code)
			return
		}

		histories := gjson.GetBytes(resp.Body, "data.room").Array()
		for _, history := range histories {
			t, _ := time.Parse(time.DateTime, history.Get("timeline").String())
			c.msgCh <- &model.Danmaku{
				Author:  history.Get("nickname").String(),
				Content: history.Get("text").String(),
				Type:    "DANMU_MSG",
				T:       t,
			}
		}
	})
}

func (c *Client) videoHeartBeat() {
	var (
		start  = time.Now()
		ticker = time.NewTicker(10 * time.Second)
	)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if err := c.cli.VideoHeartBeat(242531611, 173439442, int64(time.Since(start).Seconds())); err != nil {
				logx.Errorf("VideoHeartBeat Err: %v", err)
			}
		}
	}
}

func (c *Client) connHeartBeat() {
	var (
		ticker  = time.NewTicker(30 * time.Second)
		payload = []byte("[object Object]")
	)

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if err := c.sendPackage(1, 2, payload); err != nil {
				logx.Error("send conn heart beat, err: ", err)
			}
		}
	}
}

func (c *Client) getRoomStreamAddr() (hosts []string, token string, err error) {
	var (
		header = http.Header{
			"Cookie":          []string{c.cookie},
			"Origin":          []string{"https://live.bilibili.com"},
			"User-Agent":      []string{"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36"},
			"Referer":         []string{fmt.Sprintf("https://live.bilibili.com/%d", c.roomID)},
			"Accept":          []string{"*/*"},
			"Accept-Language": []string{"zh-CN,zh;q=0.8,zh-TW;q=0.7,zh-HK;q=0.5,en-US;q=0.3,en;q=0.2"},
		}
		params = map[string]any{
			"id":           c.roomID,
			"type":         0,
			"wts":          time.Now().Unix(),
			"web_location": "444.8",
		}
	)
	params["w_rid"] = EncodeWbi(params, c.wbiImgURL, c.wbiSubURL)

	resp, err := httpx.Getx(
		c.ctx,
		"https://api.live.bilibili.com/xlive/web-room/v1/index/getDanmuInfo",
		httpx.WithHeader(header),
		httpx.WithPayload(params),
	)
	if err != nil {
		err = fmt.Errorf("failed to get token, err: %v", err)
		return
	}
	if resp.Code != http.StatusOK || len(resp.Body) == 0 {
		err = fmt.Errorf("failed to get token, status: %v", resp.Code)
		return
	}

	if code := gjson.GetBytes(resp.Body, "code").Int(); code != 0 {
		err = fmt.Errorf("failed to get token, code: %v", code)
		return
	}

	token = gjson.GetBytes(resp.Body, "data.token").String()
	gjson.GetBytes(resp.Body, "data.host_list").ForEach(func(key, value gjson.Result) bool {
		hosts = append(hosts, fmt.Sprintf("%s:%d", value.Get("host").String(), value.Get("wss_port").Int()))
		return true
	})
	return
}
