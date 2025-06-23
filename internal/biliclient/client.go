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
	"time"

	"github.com/BYT0723/bilichat/internal/model"
	"github.com/BYT0723/go-tools/logx"
	"github.com/BYT0723/go-tools/transport/httpx"
	"github.com/gorilla/websocket"
	"github.com/iyear/biligo"
	"github.com/tidwall/gjson"
)

type Client struct {
	roomId uint32
	cli    *biligo.BiliClient
	conn   *websocket.Conn

	cookie  string
	cookies map[string]string
	uid     uint32

	history    sync.Once
	msgCh      chan *model.Danmaku
	roomInfoCh chan *model.RoomInfo

	ctx context.Context
	cf  context.CancelFunc
}

func NewClient(cookie string, roomId uint32) (c *Client, err error) {
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
		roomId:     roomId,
		cli:        cli,
		cookie:     cookie,
		cookies:    cookies,
		ctx:        ctx,
		cf:         cf,
		msgCh:      make(chan *model.Danmaku, 1024),
		roomInfoCh: make(chan *model.RoomInfo, 8),
	}
	if err = c.connect(roomId); err != nil {
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
	return c.cli.LiveSendDanmaku(int64(c.roomId), 16777215, 25, 1, msg, 0)
}

func (c *Client) connect(roomId uint32) error {
	header := http.Header{
		"Cookie":     []string{c.cookie},
		"Origin":     []string{"https://live.bilibili.com"},
		"User-Agent": []string{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/96.0.4664.110 Safari/537.36"},
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
		c.uid = uint32(gjson.GetBytes(resp.Body, "data.mid").Int())
	}

	// FIX: -352 code
	resp, err := httpx.Getx(
		c.ctx,
		"https://api.live.bilibili.com/xlive/web-room/v1/index/getDanmuInfo",
		httpx.WithHeader(header),
		httpx.WithPayload(map[string]any{
			"id":   roomId,
			"type": 0,
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to get token, err: %v", err)
	}
	if resp.Code != http.StatusOK || len(resp.Body) == 0 {
		return fmt.Errorf("failed to get token, status: %v", resp.Code)
	}

	if code := gjson.GetBytes(resp.Body, "code").Int(); code != 0 {
		return fmt.Errorf("failed to get token, code: %v", code)
	}

	os.WriteFile("room.json", resp.Body, os.ModePerm)

	var (
		hsInfo = handShakeInfo{
			UID:      c.uid,
			Roomid:   uint32(roomId),
			Protover: 2,
			Platform: "web",
			Type:     2,
			Buvid:    c.cookies["buvid3"],
			Key:      gjson.GetBytes(resp.Body, "data.token").String(),
		}
		hostList []string
	)
	gjson.GetBytes(resp.Body, "data.host_list").ForEach(func(key, value gjson.Result) bool {
		hostList = append(hostList, value.Get("host").String())
		return true
	})

	if len(hostList) == 0 {
		return errors.New("failed to get wss host")
	}

	for _, h := range hostList {
		c.conn, _, err = websocket.DefaultDialer.Dial(fmt.Sprintf("wss://%s/sub", h), header)
		if err == nil {
			break
		}
	}
	if c.conn == nil {
		return fmt.Errorf("websocket connect err: %v", err)
	}
	body, err := json.Marshal(hsInfo)
	if err != nil {
		return err
	}

	err = c.sendPackage(1, 7, 1, body)
	if err == nil {
		go c.connHeartBeat()
	}
	return err
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

	resp, err := httpx.Getx(c.ctx, "https://api.live.bilibili.com/room/v1/room/get_info", httpx.WithPayload(map[string]any{"room_id": c.roomId}))
	if err != nil {
		logx.Errorf("get room information, err: %v", err)
		return
	}
	if resp.Code != http.StatusOK || len(resp.Body) == 0 {
		logx.Errorf("get room information, status: %v", resp.Code)
		return
	}

	roomInfo.RoomId = int(c.roomId)
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
		"roomId":   c.roomId,
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

func (c *Client) sendPackage(ver uint16, typeID uint32, param uint32, data []byte) (err error) {
	packetHead := new(bytes.Buffer)

	for _, v := range []any{
		uint32(len(data) + 16),
		uint16(16),
		ver,
		typeID,
		param,
	} {
		if err = binary.Write(packetHead, binary.BigEndian, v); err != nil {
			return
		}
	}

	sendData := append(packetHead.Bytes(), data...)

	err = c.conn.WriteMessage(websocket.BinaryMessage, sendData)
	return
}

func (c *Client) getHistoryDanmaku() {
	c.history.Do(func() {
		resp, err := httpx.Getx(c.ctx, "https://api.live.bilibili.com/xlive/web-room/v1/dM/gethistory", httpx.WithPayload(map[string]any{"roomid": c.roomId}))
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
		payload = []byte("5b6f626a656374204f626a6563745d")
	)

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if err := c.sendPackage(1, 2, 1, payload); err != nil {
				logx.Error("send conn heart beat, err: ", err)
			}
		}
	}
}
