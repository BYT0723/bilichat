package biliclient

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"io"
	"net/http"

	"github.com/andybalholm/brotli"
)

func parseCookie(cookie string) (map[string]string, error) {
	cookies, err := http.ParseCookie(cookie)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, c := range cookies {
		result[c.Name] = c.Value
	}
	return result, nil
}

func brotliDecode(src []byte) []byte {
	bs, _ := io.ReadAll(brotli.NewReader(bytes.NewBuffer(src)))
	return bs
}

func zlibUnCompress(src []byte) []byte {
	r, _ := zlib.NewReader(bytes.NewBuffer(src))
	b, _ := io.ReadAll(r)
	return b
}

func splitMsg(src []byte) (msgs [][]byte) {
	var (
		totalLen = len(src)
		offset   = 0
	)

	for offset+4 <= totalLen {
		// 读取长度字段（大端序）
		msgLen := int(binary.BigEndian.Uint32(src[offset : offset+4]))
		if offset+msgLen > totalLen || msgLen < 4 {
			// 非法或不完整消息，终止处理
			break
		}
		msgs = append(msgs, src[offset:offset+msgLen])
		offset += msgLen
	}
	return msgs
}
