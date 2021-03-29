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

// 抽取网页中的文本信息
func ExtractText(document string) string {
	return ""
}
