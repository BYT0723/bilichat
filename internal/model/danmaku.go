package model

import "time"

type Danmaku struct {
	Author  string
	Content string
	Type    string
	T       time.Time
}
