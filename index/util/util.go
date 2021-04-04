package util

import (
	"net/url"
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
