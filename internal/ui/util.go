package ui

import (
	"fmt"
	"time"
)

func FormatDurationZH(d time.Duration) string {
	d = d.Round(time.Second) // 去掉纳秒级别

	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	str := ""
	if h > 0 {
		str += fmt.Sprintf("%d时", h)
	}
	if m > 0 {
		str += fmt.Sprintf("%d分", m)
	}
	if s > 0 || (h == 0 && m == 0) {
		str += fmt.Sprintf("%d秒", s)
	}

	return str
}
