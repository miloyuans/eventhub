package utils

import (
	"crypto/md5"
	"encoding/hex"
	//"fmt"
)

// GenerateID 生成唯一指纹
func GenerateID(parts ...string) string {
	h := md5.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte("|")) // 分隔符防止粘连
	}
	return hex.EncodeToString(h.Sum(nil))
}
