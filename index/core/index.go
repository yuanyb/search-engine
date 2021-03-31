// 索引
package core

import "search-engine/index/db"

// 索引管理器
type indexManager struct {
	indexBuffer          invertedIndex
	bufferFlushThreshold int
	bufferCount          int
	db                   *db.DB
}

// 倒排索引
type invertedIndex map[int]*tokenIndexItem

// token 对应的索引项
type tokenIndexItem struct {
	tokenId        int           // 词元编号
	documentCount  int           // 包含该词元的文档的数目
	positionsCount int           // 在所有文档中该词元出现的次数
	postings       *postingsList // 该词元的倒排列表
}

// 倒排列表
type postingsList struct {
	documentId int           // 文档编号
	positions  []int         // 词元在该文档中的位置信息，如果在标题中出现了，则positions[0]表示在标题中出现的位置
	next       *postingsList // 指向下一个倒排列表的指针
	flag       uint8         // todo 0000_0000 0号位表示在正文中出现，1号位表示在标题中出现
}

// 文本处理器
type textProcessor struct {
	db *db.DB
	n  int // n-gram
}

func newTextProcessor(n int, db *db.DB) *textProcessor {
	return &textProcessor{
		db: db,
		n:  n,
	}
}

func (p *textProcessor) textToInvertedIndex(documentId int, text string) invertedIndex {
	index := invertedIndex{}
	// todo N
	nGramSplit(text, p.n, func(token string, pos int) error {
		return p.tokenToPostingsLists(index, documentId, token, pos)
	})
	return index
}

// 将一个词元转换成倒排列表
func (p *textProcessor) tokenToPostingsLists(index invertedIndex, documentId int, token string, pos int) error {
	tokenId, err := p.db.GetTokenId(token)
	if err != nil {
		return err
	}
	entry, ok := index[tokenId]
	if !ok {
		entry = &tokenIndexItem{
			tokenId:        tokenId,
			documentCount:  1,
			positionsCount: 0,
			postings: &postingsList{
				documentId: documentId,
				positions:  nil,
				next:       nil,
			},
		}
		index[tokenId] = entry
	}
	// 词元出现次数 +1
	entry.postings.positions = append(entry.postings.positions, pos)
	entry.positionsCount++
	return nil
}

// 合并倒排索引
func (i invertedIndex) merge(index invertedIndex) {
	for tokenId, item := range index {
		baseItem, ok := i[tokenId]
		if ok {
			baseItem.documentCount++
			baseItem.positionsCount += item.positionsCount
			baseItem.postings.merge(item.postings)
		} else {
			i[tokenId] = item
		}
	}
}

// 合并倒排列表，文档ID小的列表项在前
func (p *postingsList) merge(postings *postingsList) {
	head := new(postingsList)
	pc, pa, pb := head, p, postings
	for pa != nil || pb != nil {
		if pb == nil || (pa != nil && pa.documentId <= pb.documentId) {
			pc.next = pa
			pa = pa.next
		} else {
			pc.next = pb
			pb = pb.next
		}
		pc = pc.next
		pc.next = nil
	}
	*p = *(head.next)
}

// 将倒排列表编码成二进制数据
func (p *postingsList) encode() []byte {
	// todo
	return nil
}

// 将二进制数据解码成倒排列表

func decodePostingsList(data []byte) *postingsList {
	return nil
}

// 将 index 合并进索引管理器
func (m *indexManager) merge(index invertedIndex) {
	if m.indexBuffer == nil {
		m.indexBuffer = index
		return
	}
	m.indexBuffer.merge(index)

	if m.bufferCount > m.bufferFlushThreshold {
		m.flush()
	}
}

// 将内存中的缓冲的索引与存储器中的索引合并后刷新到存储器中
func (m *indexManager) flush() {
	for tokenId, item := range m.indexBuffer {
		// 从数据库中取出来旧的索引
		data, err := m.db.GetPostingsList(tokenId)
		if err != nil {
			// todo log
			continue
		}
		postings := decodePostingsList(data)
		// 内存中合并
		postings.merge(item.postings)
		// 写回
		data = postings.encode()
		err = m.db.UpdatePostingsList(tokenId, data)
		if err != nil {
			// todo log
		}
	}
	m.indexBuffer = make(invertedIndex)
}
