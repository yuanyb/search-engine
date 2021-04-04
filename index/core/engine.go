package core

import (
	"search-engine/index/db"
	"strings"
)

const searchDocId = -1024

type Engine struct {
	// todo buffer
	indexManager  *indexManager
	textProcessor *textProcessor
	searcher      *searcher
	db            *db.IndexDB
}

func NewEngine() *Engine {
	e := &Engine{}
	return e
}

func (e *Engine) AddDocument(document string) {
	parsedDocument := parseDocument(document)
	docId := 0
	index := e.textProcessor.textToInvertedIndex(docId, parsedDocument)
	e.indexManager.merge(index)
}

func (e *Engine) Search(query string) SearchResults {
	var searchResults SearchResults
	// 是否有非法关键词，邪教、黄色
	if hasIllegalKeywords(query) {
		return searchResults
	}
	// 搜索建议，纠错
	if strings.IndexByte(query, ' ') == -1 {
		if c, ok := errorCorrect(query); ok {
			searchResults.suggestion = c
		}
	}
	parsedQuery := parseQuery(query)
	for _, keyword := range parsedQuery.keywords {
		r := e.searcher.searchDocs(e.textProcessor.queryToTokens(keyword), parsedQuery.site)
		searchResults.and(r)
	}
	for _, exclusion := range parsedQuery.exclusions {
		r := e.searcher.searchDocs(e.textProcessor.queryToTokens(exclusion), parsedQuery.site)
		searchResults.not(r)
	}
	return searchResults
}
