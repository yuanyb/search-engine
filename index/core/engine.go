package core

import (
	"search-engine/index/config"
	"search-engine/index/db"
	"search-engine/index/util"
	"strings"
	"time"
)

const searchDocId = -1024

type Engine struct {
	indexManager *indexManager
	searcher     *searcher
	DB           *db.IndexDB
	Birthday     int64
}

func NewEngine() *Engine {
	indexDB := db.NewIndexDB(&db.IndexDBOptions{
		DocUrlBufferSize:         config.GetInt("indexer.docUrlBufferSize"),
		PostingsBufferSize:       config.GetInt("indexer.postingsBufferSize"),
		TokenDocsCountBufferSize: config.GetInt("indexer.tokenDocCountBufferSize"),
		DocumentDBPath:           config.Get("boltdb.docPath"),
		IndexDBPath:              config.Get("boltdb.indexPath"),
	})
	tp := newTextProcessor(2, indexDB)
	e := &Engine{
		indexManager: newIndexManager(indexDB, tp, config.GetInt("indexer.postingsBufferFlushThreshold")),
		searcher:     newSearcher(indexDB, tp),
		DB:           indexDB,
		Birthday:     time.Now().Unix(),
	}
	return e
}

// 为一个文档构建索引
func (e *Engine) AddDocument(url, document string) {
	e.indexManager.indexChannel <- [2]string{url, document}
}

// 并发安全
func (e *Engine) Search(query string) SearchResults {
	var searchResults SearchResults
	// 是否有非法关键词，邪教、黄色
	if hasIllegalKeywords(query) {
		return searchResults
	}
	// 搜索建议，纠错
	if strings.IndexByte(query, ' ') == -1 {
		if c, ok := suggest(query); ok {
			searchResults.Suggestion = c
		}
	}
	// 检索并合并结果
	parsedQuery := parseQuery(query)
	if len(parsedQuery.keywords) == 0 {
		return searchResults
	}
	for _, keyword := range parsedQuery.keywords {
		r := e.searcher.searchDocs(keyword, parsedQuery.site)
		searchResults.and(r)
		if len(searchResults.Items) == 0 {
			return searchResults
		}
	}
	for _, exclusion := range parsedQuery.exclusions {
		r := e.searcher.searchDocs(exclusion, parsedQuery.site)
		searchResults.not(r)
		if len(searchResults.Items) == 0 {
			return searchResults
		}
	}
	// 仅返回30条结果
	searchResults.Items = searchResults.Items[:util.MinInt(30, len(searchResults.Items))]
	// 获取文档信息及高亮结果
	searchResults.applyHighlight(e.DB)
	return searchResults
}
