package utils

import (
	// "fmt"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"net/url"
)

func Md5Encode(str string) string {
	md5Ctx := md5.New()
	md5Ctx.Write([]byte(str))
	cipherStr := md5Ctx.Sum(nil)
	return hex.EncodeToString(cipherStr)
}

func Base64Encode(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}

func Urlencode(str string) string {
	return url.QueryEscape(str)
}

func Urldecode(str string) (string, error) {
	return url.QueryUnescape(str)
}
