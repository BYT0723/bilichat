package model

import "time"

type Danmaku struct {
	Author  string
	Content string
	Type    string
	Time    time.Time
}
