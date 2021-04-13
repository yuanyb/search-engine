// 查询预处理器，在查询前，做一些预处理工作，如过滤非法关键词等等
package core

import (
	"search-engine/index/config"
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
	configDB := db.NewConfigDB(&db.ConfigDBOptions{
		User:     config.Get("mysql.username"),
		Password: config.Get("mysql.password"),
		Host:     config.Get("mysql.host"),
		Port:     config.GetInt("mysql.port"),
		DBName:   config.Get("mysql.dbname"),
	})
	initialized := false
	initDone := make(chan struct{})
	go func() {
		for {
			illegal, err := configDB.GetIllegalKeyWords()
			if err == nil {
				// 实时性要求低，不用做并发安全处理
				illegalKeywords = illegal
			}
			if !initialized {
				initialized = true
				initDone <- struct{}{}
			}
			time.Sleep(time.Minute)
		}
	}()
	<-initDone
	close(initDone)
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

func suggest(query string) (string, bool) {
	// 如果词典中没有 query 这个这个单词，则可以判断编辑距离，多级map
	return "", false
}
