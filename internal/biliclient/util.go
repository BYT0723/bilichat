package biliclient

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"io"
	"math"
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

func zlibUnCompress(compressSrc []byte) []byte {
	b := bytes.NewReader(compressSrc)
	var out bytes.Buffer
	r, _ := zlib.NewReader(b)
	io.Copy(&out, r)
	return out.Bytes()
}

func ByteArrToDecimal(src []byte) (sum int) {
	if src == nil {
		return 0
	}
	b := []byte(hex.EncodeToString(src))
	l := len(b)
	for i := l - 1; i >= 0; i-- {
		base := int(math.Pow(16, float64(l-i-1)))
		var mul int
		if int(b[i]) >= 97 {
			mul = int(b[i]) - 87
		} else {
			mul = int(b[i]) - 48
		}

		sum += base * mul
	}
	return
}

func splitMsg(src []byte) (msgs [][]byte) {
	lens := ByteArrToDecimal(src[:4])
	totalLen := len(src)
	startLoc := 0
	for {
		if startLoc+lens <= totalLen {
			msgs = append(msgs, src[startLoc:startLoc+lens])
			startLoc += lens
			if startLoc < totalLen {
				lens = ByteArrToDecimal(src[startLoc : startLoc+4])
			} else {
				break
			}
		} else {
			break
		}
	}
	return msgs
}

func brotliDecode(src []byte) []byte {
	bs, _ := io.ReadAll(brotli.NewReader(bytes.NewBuffer(src)))
	return bs
}
