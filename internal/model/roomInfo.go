package model

import "time"

type (
	RoomInfo struct {
		RoomId          int              `json:"room_id,omitempty"`
		Uid             int              `json:"uid,omitempty"`
		Title           string           `json:"title,omitempty"`
		ParentAreaName  string           `json:"parent_area_name,omitempty"`
		AreaName        string           `json:"area_name,omitempty"`
		Online          string           `json:"online,omitempty"`    // 在线人数
		Watched         string           `json:"watched,omitempty"`   // 累计观看
		Liked           string           `json:"liked,omitempty"`     // 点赞数
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
