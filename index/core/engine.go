package core

import (
	"math/rand"
	"search-engine/index/config"
	"search-engine/index/db"
	"strings"
)

const searchDocId = -1024

type Engine struct {
	indexManager  *indexManager
	textProcessor *textProcessor
	searcher      *searcher
	db            *db.IndexDB

	indexerChannels    []chan [2]string
	indexerWorkerCount int
}

func NewEngine() *Engine {
	indexDBb := db.NewIndexDB(&db.IndexDBOptions{
		DocUrlBufferSize:   config.GetInt("docUrlBufferSize"),
		TokenIdBufferSize:  config.GetInt("tokenIdBufferSize"),
		PostingsBufferSize: config.GetInt("postingsBufferSize"),
		DocumentDBPath:     config.Get("documentDBPath"),
		IndexDBPath:        config.Get("indexDBPath"),
	})
	e := &Engine{
		indexManager:       newIndexManager(indexDBb),
		textProcessor:      newTextProcessor(2, indexDBb),
		searcher:           newSearcher(indexDBb),
		db:                 indexDBb,
		indexerChannels:    make([]chan [2]string, config.GetInt("indexerWorkerCount")),
		indexerWorkerCount: config.GetInt("indexerWorkerCount"),
	}

	for i := 0; i < len(e.indexerChannels); i++ {
		e.indexerChannels[i] = make(chan [2]string, config.GetInt("indexerChannelLength"))
	}

	// 启动构建索引协程，限制数量
	for i := 0; i < e.indexerWorkerCount; i++ {
		go func(i int) {
			for {
				doc := <-e.indexerChannels[i]
				parsedDocument := parseDocument(doc[1])
				if parsedDocument == nil {
					continue
				}
				docId, err := e.db.AddDocument(doc[0], parsedDocument.title, parsedDocument.body)
				if err != nil {
					continue
				}
				index := e.textProcessor.textToInvertedIndex(docId, parsedDocument)
				e.indexManager.merge(index)
			}
		}(i)
	}
	return e
}

// 为一个文档构建索引
func (e *Engine) AddDocument(url, document string) {
	e.indexerChannels[rand.Intn(e.indexerWorkerCount)] <- [2]string{url, document}
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
	for _, keyword := range parsedQuery.keywords {
		r := e.searcher.searchDocs(e.textProcessor.queryToTokens(keyword), parsedQuery.site)
		searchResults.and(r)
	}
	for _, exclusion := range parsedQuery.exclusions {
		r := e.searcher.searchDocs(e.textProcessor.queryToTokens(exclusion), parsedQuery.site)
		searchResults.not(r)
	}
	// 获取文档信息及高亮结果
	searchResults.applyHighlight(e.db)
	return searchResults
}
