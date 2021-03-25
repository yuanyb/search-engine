// 网页数据处理器
package data

import (
	"fmt"
	"regexp"
	"strings"
)

var urlPattern = regexp.MustCompile(`(?is)<a.+?href.?=.?"(.+?)"`)

// 提取网页文档中有意义的的超链接，参数 document 是 url 对应网页的文本数据
func ExtractUrls(rootUrl, document string) []string {
	result := urlPattern.FindAllStringSubmatch(document, -1)
	urls := make([]string, 0, len(result))
	for _, res := range result {
		url := res[1]
		if !strings.HasPrefix(url, "http") && !strings.HasPrefix(url, "HTTP") {
			url = fmt.Sprintf("%s/%s", rootUrl, url)
			url = trimFragment(url)
		}
		if isValuableUrl(url) {
			urls = append(urls, url)
		}
	}
	return urls
}

func isValuableUrl(url string) bool {
	if strings.HasPrefix(url, "javascript:") { // <a href="javascirpt:xxx">...
		return false
	}
	// ...
	return true
}

func trimFragment(url string) string {
	if p := strings.IndexByte(url, '#'); p != -1 {
		return url[:p]
	}
	return url
}
