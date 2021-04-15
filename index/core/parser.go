// 解析 HTML 文档
package core

import (
	"regexp"
	"strings"
)

type parsedDocument struct {
	title string
	body  string
	// <meta name="keywords" content="xxx">
	// h1 []string // h1标签 权重高
}

var (
	titlePattern     = regexp.MustCompile(`(?ism)<title.*?>(.*?)</title>`)
	trimTagPattern   = regexp.MustCompile(`(?ism)<script.*?>.*?</script>|<style.*?>.*?</style>|<title.*?>.*?</title>|<.+?>`)
	trimSpacePattern = regexp.MustCompile(`(?m)\s+`)
)

func parseDocument(document string) *parsedDocument {
	parsedDocument := &parsedDocument{}
	// title
	result := titlePattern.FindStringSubmatch(document)
	if len(result) == 0 {
		return nil
	}
	parsedDocument.title = strings.TrimSpace(result[1])

	// body
	document = trimTagPattern.ReplaceAllString(document, " ")
	document = trimSpacePattern.ReplaceAllString(document, " ")
	document = strings.TrimSpace(document)
	parsedDocument.body = document
	return parsedDocument
}
