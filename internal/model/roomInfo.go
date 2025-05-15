package model

import "time"

type (
	RoomInfo struct {
		RoomId          int              `json:"room_id,omitempty"`
		Uid             int              `json:"uid,omitempty"`
		Title           string           `json:"title,omitempty"`
		ParentAreaName  string           `json:"parent_area_name,omitempty"`
		AreaName        string           `json:"area_name,omitempty"`
		Online          int64            `json:"online,omitempty"`
		Attention       int64            `json:"attention,omitempty"` // 关注数
		Uptime          time.Duration    `json:"time,omitempty"`      // 在线时间
		OnlineRankUsers []OnlineRankUser `json:"online_rank_users,omitempty"`
	}
	OnlineRankUser struct {
		Name  string
		Score int64
		Rank  int64
	}
)
