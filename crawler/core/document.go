// 网页数据处理器
package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"search-engine/crawler/config"
	"strings"
)

var urlPattern = regexp.MustCompile(`(?is)<a.+?href.?=.?"(.+?)"`)

// 提取网页文档中有意义的的超链接，参数 document 是 url 对应网页的文本数据
func ExtractUrls(rootUrl, document string) []string {
	result := urlPattern.FindAllStringSubmatch(document, -1)
	urls := make([]string, 0, len(result))
	for _, res := range result {
		u := res[1]
		u = trimFragment(u)
		if !isValuableUrl(u) {
			continue
		}
		if !strings.HasPrefix(u, "http") && !strings.HasPrefix(u, "HTTP") {
			u = fmt.Sprintf("%s/%s", rootUrl, u)
		}
		urls = append(urls, u)
	}
	return urls
}

func isValuableUrl(u string) bool {
	if strings.HasPrefix(u, "javascript:") { // <a href="javascirpt:xxx">...
		return false
	}
	return true
}

func trimFragment(url string) string {
	if p := strings.IndexByte(url, '#'); p != -1 {
		return url[:p]
	}
	return url
}

func SendDocument(url, document string) {
	// 异步发送
	go func() {
		j, _ := json.Marshal(map[string]string{
			"url":      url,
			"document": document,
		})
		retryCount := config.Get().RetryCount
		addrList := indexerAddrList.Load().([]string)
		indexerAddr := addrList[rand.Intn(len(addrList))]
		for i := 0; i < retryCount+1; i++ {
			req, _ := http.NewRequest("PUT", indexerAddr+"/index", bytes.NewReader(j))
			_, err := http.DefaultClient.Do(req)
			if err == nil {
				break
			}
		}
	}()
}
