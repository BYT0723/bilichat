package biliclient

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"path"
	"sort"
	"strings"
)

// MIXIN_KEY_ENC_TAB: 字符顺序混淆表
var mixinKeyEncTab = []int{
	46, 47, 18, 2, 53, 8, 23, 32, 15, 50,
	10, 31, 58, 3, 45, 35, 27, 43, 5, 49,
	33, 9, 42, 19, 29, 28, 14, 39, 12, 38,
	41, 13, 37, 48, 7, 16, 24, 55, 40, 61,
	26, 17, 0, 1, 60, 51, 30, 4, 22, 25,
	54, 21, 56, 59, 6, 63, 57, 62, 11, 36,
	20, 34, 44, 52,
}

// 提取文件名（无扩展名）
func extractKey(urlStr string) string {
	base := path.Base(urlStr)
	return strings.TrimSuffix(base, path.Ext(base))
}

// 混淆 key 生成器
func getMixinKey(imgKey, subKey string) string {
	raw := imgKey + subKey
	runes := []rune(raw)
	var builder strings.Builder
	for _, i := range mixinKeyEncTab {
		if i < len(runes) {
			builder.WriteRune(runes[i])
		}
		if builder.Len() >= 32 {
			break
		}
	}
	return builder.String()
}

// EncodeWbi 生成最终的 w_rid 签名
func EncodeWbi(params map[string]any, imgURL, subURL string) string {
	imgKey := extractKey(imgURL)
	subKey := extractKey(subURL)
	mixinKey := getMixinKey(imgKey, subKey)

	// 排序参数
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var queryBuilder strings.Builder
	for i, k := range keys {
		queryBuilder.WriteString(k)
		queryBuilder.WriteByte('=')
		queryBuilder.WriteString(fmt.Sprint(params[k]))
		if i != len(keys)-1 {
			queryBuilder.WriteByte('&')
		}
	}
	query := queryBuilder.String()

	// 拼接并 md5 签名
	sign := md5.Sum([]byte(query + mixinKey))
	return hex.EncodeToString(sign[:])
}
