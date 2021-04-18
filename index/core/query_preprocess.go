// 查询预处理器，在查询前，做一些预处理工作，如过滤非法关键词等等
package core

import (
	"strings"
)

type parsedQuery struct {
	keywords   []string // 查询关键词，and
	exclusions []string // 排除的关键词，not
	site       string   // 站点内查询，site:xxx.com
}

func parseQuery(query string) *parsedQuery {
	fragments := strings.Split(query, " ")
	ret := &parsedQuery{}
	for _, fragment := range fragments {
		switch {
		case len(fragment) == 0:
			break
		case strings.HasPrefix(fragment, "-"):
			ret.exclusions = append(ret.exclusions, fragment[1:])
		case strings.HasPrefix(fragment, "site:"):
			// 多个取第一个
			if len(ret.site) > 0 {
				break
			}
			ret.site = fragment[5:]
		default:
			ret.keywords = append(ret.keywords, fragment)
		}
	}
	return ret
}
