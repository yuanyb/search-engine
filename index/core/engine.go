package core

import "search-engine/index/db"

type Engine struct {
	// todo buffer
	indexManager  *indexManager
	textProcessor *textProcessor
	db            *db.IndexDB
}

func NewEngine() *Engine {
	e := &Engine{}
	return e
}

func (e *Engine) AddDocument(document string) {
	title, body := "", "" // parseDocument(document)
	_ = title
	docId := 0
	index := e.textProcessor.textToInvertedIndex(docId, body)
	e.indexManager.merge(index)
}

func (e *Engine) Search(query string) {

}
