// 解析 robots.txt
package robots

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"src/downloader"
	"strings"
)

var robotsMap = make(map[string]*robots, 10000)

// 规则
type rule struct {
	matchSuffix   bool     // 是否 $ 结尾，是则匹配后缀
	pathFragments []string // xxx*xxx*xxx  split => [xxx, xxx, xxx]
}

// 创建一个 rule
func newRule(ruleValue string) *rule {
	r := &rule{}
	r.matchSuffix = ruleValue[len(ruleValue)-1] == '$'
	if r.matchSuffix {
		ruleValue = strings.TrimSuffix(ruleValue, "$")
	}
	r.pathFragments = strings.Split(ruleValue, "*")
	return r
}

// 判断 path 和 rule 是否匹配
func (r *rule) match(path string) bool {
	// 长度为 1，则不含有 *
	if len(r.pathFragments) == 1 {
		if r.matchSuffix {
			return r.pathFragments[0] == path
		}
		return strings.HasPrefix(path, r.pathFragments[0])
	}
	tmp := path
	for _, fragment := range r.pathFragments {
		if i := strings.Index(tmp, fragment); i >= 0 {
			tmp = tmp[i+len(fragment):]
		} else {
			return false
		}
	}
	// 如果 $ 结尾，则后缀必须能够匹配
	lastElem := r.pathFragments[len(r.pathFragments)-1]
	if r.matchSuffix && !strings.HasSuffix(path, lastElem) {
		return false
	}
	return true
}

// robots 表示 robots.txt 中和自己有关的规则
type robots struct {
	allowRules    []*rule
	disallowRules []*rule
}

// 分割一行规则，返回 key、value
func splitLine(line string) (string, string, bool) {
	pos := strings.IndexByte(line, ':')
	if pos == -1 {
		return "", "", false
	}
	return line[:pos], strings.TrimSpace(line[pos+1:]), true
}

// 创建一个 robots
func newRobots(reader io.Reader, useragent string) *robots {
	robots := &robots{}
	scanner := bufio.NewScanner(bufio.NewReader(reader))
	// 当前行规则是否需要处理
	needHandle := true
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		key, value, ok := splitLine(line)
		if ok == false {
			continue
		}

		key = strings.ToLower(key)
		switch key {
		case "user-agent", "useragent":
			// 不是针对自己的规则就忽略掉
			if value != useragent && value != "*" {
				needHandle = false
				break
			}
			needHandle = true
		case "allow":
			if needHandle == false {
				break
			}
			robots.allowRules = append(robots.allowRules, newRule(value))
		case "disallow":
			if needHandle == false {
				break
			}
			robots.disallowRules = append(robots.disallowRules, newRule(value))
		default:
			// 其他属性忽略
		}
	}
	return robots
}

// 判断是否允许爬取 path
func Allow(rawUrl, userAgent string) bool {
	parsedUrl, err := url.Parse(rawUrl)
	if err != nil {
		// rawUrl 格式错误，就没必要访问了
		return false
	}
	path := parsedUrl.Path + "?" + parsedUrl.RawQuery

	// 从 robotsMap 中先获取 robots，如果没有则添加
	if _, ok := robotsMap[parsedUrl.Host]; !ok {
		robotsUrl := fmt.Sprintf("%s://%s/robots.txt", parsedUrl.Scheme, parsedUrl.Host)
		robotsTxt, err := downloader.GlobalDownloader.DownloadText(robotsUrl)
		if err != nil {
			return true
		}
		robotsMap[parsedUrl.Host] = newRobots(strings.NewReader(robotsTxt), userAgent)
	}
	robots, _ := robotsMap[parsedUrl.Host]

	// Allow 优先级更高
	for _, rule := range robots.allowRules {
		if rule.match(path) {
			return true
		}
	}
	for _, rule := range robots.disallowRules {
		if rule.match(path) {
			return false
		}
	}
	return true
}
