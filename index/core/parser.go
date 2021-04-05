// 解析 HTML 文档
package core

import (
	"regexp"
	"strings"
)

type ParsedDocument struct {
	Url   string
	Title string
	Body  string
	// h1 []string // h1标签 权重高
}

var (
	titlePattern     = regexp.MustCompile(`(?im)<title.*?>(.*?)</title>`)
	trimTagPattern   = regexp.MustCompile(`(?im)<script.*?>.*?</script>|<style.*?>.*?</style>|<title.*?>.*?</title>|<.+?>`)
	trimSpacePattern = regexp.MustCompile(`(?m)\s+`)
)

func parseDocument(document string) *ParsedDocument {
	parsedDocument := &ParsedDocument{}
	// title
	result := titlePattern.FindStringSubmatch(document)
	if len(result) == 0 {
		return nil
	}
	parsedDocument.Title = result[1]

	// body
	document = trimTagPattern.ReplaceAllString(document, " ")
	document = trimSpacePattern.ReplaceAllString(document, " ")
	document = strings.TrimSpace(document)
	parsedDocument.Body = document
	return parsedDocument
}
