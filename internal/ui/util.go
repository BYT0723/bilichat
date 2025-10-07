package ui

import (
	"fmt"
	"strings"
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

func SanitizeViewportText(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		// 零宽字符
		case '\u200B', '\u200C', '\u200D', '\u2060', '\uFEFF', '\u180E':
			return -1
		// 变体选择符（emoji 颜色控制）
		case '\uFE0E', '\uFE0F':
			return -1
		// Regional Indicator（国家旗帜）
		case '\U0001F1E6', '\U0001F1E7', '\U0001F1E8', '\U0001F1E9',
			'\U0001F1EA', '\U0001F1EB', '\U0001F1EC', '\U0001F1ED',
			'\U0001F1EE', '\U0001F1EF', '\U0001F1F0', '\U0001F1F1',
			'\U0001F1F2', '\U0001F1F3', '\U0001F1F4', '\U0001F1F5',
			'\U0001F1F6', '\U0001F1F7', '\U0001F1F8', '\U0001F1F9',
			'\U0001F1FA', '\U0001F1FB', '\U0001F1FC', '\U0001F1FD',
			'\U0001F1FE', '\U0001F1FF':
			return -1
		}
		// 删除控制字符
		if r < 32 && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, s)
}
