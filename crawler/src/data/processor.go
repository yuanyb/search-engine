// 网页数据处理器
package data

import (
	"regexp"
)

var urlPattern = regexp.MustCompile(`href="(.+?)"`)

// 提取网页文档中的超链接
func ExtractUrls(document string) []string {
	result := urlPattern.FindAllStringSubmatch(document, -1)
	urls := make([]string, 0, len(result))
	for _, res := range result {
		urls = append(urls, res[0])
	}
	return urls
}
