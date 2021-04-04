// 查询预处理器，在查询前，做一些预处理工作，如过滤非法关键词等等
package core

import (
	"search-engine/index/db"
	"strings"
	"time"
)

type parsedQuery struct {
	keywords   []string // 查询关键词，and
	exclusions []string // 排除的关键词，not
	site       string   // 站点内查询，site:xxx.com
}

var illegalKeywords []string

// 定时从数据库中抓取非法关键词
func init() {
	illegal, err := db.GlobalConfigDB.GetIllegalKeyWords()
	if err != nil {
		// todo log
	}
	illegalKeywords = illegal
	go func() {
		time.Sleep(time.Minute)
		illegal, err = db.GlobalConfigDB.GetIllegalKeyWords()
		if err != nil {
			// todo log
		} else {
			// 实时性要求低，不用做并发安全处理
			illegalKeywords = illegal
		}
	}()
}

func hasIllegalKeywords(query string) bool {
	// 拷贝一份切片变量，这样并发修改就不会有问题（忽略可见性）
	illegal := illegalKeywords
	for _, keyword := range illegal {
		if strings.Contains(query, keyword) {
			return true
		}
	}
	return false
}

func parseQuery(query string) *parsedQuery {
	fragments := strings.Split(query, " ")
	ret := &parsedQuery{}
	for _, fragment := range fragments {
		switch {
		case len(fragment) == 0:
			break
		case strings.HasPrefix(fragment, "-"):
			ret.exclusions = append(ret.exclusions, fragment)
		case strings.HasPrefix(fragment, "site:"):
			// 多个取第一个
			if len(ret.site) > 0 {
				break
			}
			ret.site = fragment
		default:
			ret.keywords = append(ret.keywords, fragment)
		}
	}
	return ret
}

func errorCorrect(query string) (string, bool) {
	// 如果词典中没有 query 这个这个单词，则可以判断编辑距离，多级map
	return "", false
}
