package util

import (
	"encoding/binary"
	"net/url"
	"strconv"
	"strings"
)

func RetryWhenFailed(retryCount int, f func() error) {
	for i := 0; i < retryCount+1; i++ {
		err := f()
		if err == nil {
			return
		}
	}
	// todo log
}

func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func MinInt(a, b int) int {
	if a > b {
		return b
	}
	return a
}

func UrlToHost(u string) string {
	parsedUrl, err := url.Parse(u)
	if err != nil {
		return "\x00" // 不确定就不返回给用户
	}
	host := parsedUrl.Host
	if p := strings.LastIndexByte(host, ':'); p != -1 {
		host = host[:p]
	}
	return host
}

func ToInt(dest *int, value string) {
	if i, err := strconv.ParseInt(value, 10, 64); err == nil {
		*dest = int(i)
	}
}

func ToInt64(dest *int64, value string) {
	if i, err := strconv.ParseInt(value, 10, 64); err == nil {
		*dest = i
	}
}

func ToBool(dest *bool, value string) {
	if b, err := strconv.ParseBool(value); err == nil {
		*dest = b
	}
}

func EncodeVarInt(buf []byte, i int64) []byte {
	if len(buf) < binary.MaxVarintLen64 {
		panic("buf length too small")
	}
	length := binary.PutVarint(buf, i)
	ret := make([]byte, length)
	copy(ret, buf[:length])
	return ret
}
