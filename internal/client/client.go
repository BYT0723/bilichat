package client

import "context"

type Client interface {
	Start(ctx context.Context) error
	Stop() error
	Receive() <-chan Message
	Send(content string) error
}

type MessageType int

const (
	Unknown MessageType = iota
	BiliBiliDanmaku
	BiliBiliRoomInfo
	BiliBiliRankInfo
)

type Message struct {
	Type MessageType
	Data any
}
