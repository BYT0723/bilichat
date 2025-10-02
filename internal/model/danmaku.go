package model

import "time"

type (
	Danmaku struct {
		Medal   *Medal
		Author  string
		Content string
		Type    string
		T       time.Time
	}
	Medal struct {
		Name  string
		Level int
	}
)
