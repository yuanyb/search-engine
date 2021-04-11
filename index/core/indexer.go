// 索引
package core

import (
	"encoding/binary"
	"log"
	"search-engine/index/db"
	"sync"
)

// 索引管理器
type indexManager struct {
	indexBuffer          invertedIndex // 内存中的索引，待写入磁盘
	bufferFlushThreshold int           // 阈值
	indexCount           int           // 当前索引的文档数量
	db                   *db.IndexDB
	lock                 sync.Mutex
}

// 倒排索引 tokenId->tokenIndexItem
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
	positions  []int         // 词元在该文档中的位置信息
	next       *postingsList // 指向下一个倒排列表的指针
	titleEnd   int           // positions[:titleEnd] 是词元在标题中的位置信息，小于0则表示词元没有在对应文档标题中出现
}

// 文本处理器
type textProcessor struct {
	db *db.IndexDB
	n  int // n-gram
}

func newTextProcessor(n int, db *db.IndexDB) *textProcessor {
	return &textProcessor{
		db: db,
		n:  n,
	}
}

func newIndexManager(db *db.IndexDB) *indexManager {
	return &indexManager{
		bufferFlushThreshold: 0,
		db:                   db,
		lock:                 sync.Mutex{},
	}
}

func (p *textProcessor) textToInvertedIndex(documentId int, document *parsedDocument) invertedIndex {
	index := invertedIndex{}
	nGramSplit(document.title, p.n, func(token string, pos int) error {
		return p.tokenToPostingsLists(index, documentId, token, pos, true)
	})
	nGramSplit(document.body, p.n, func(token string, pos int) error {
		return p.tokenToPostingsLists(index, documentId, token, pos, false)
	})
	return index
}

// 将查询内容转换成倒排索引形式
func (p *textProcessor) queryToTokens(query string) []*tokenIndexItem {
	index := invertedIndex{}
	nGramSplit(query, p.n, func(token string, pos int) error {
		return p.tokenToPostingsLists(index, searchDocId, token, pos, true)
	})
	ret := make([]*tokenIndexItem, 0)
	for _, item := range index {
		ret = append(ret, item)
	}
	return ret
}

// 将一个词元转换成倒排列表
func (p *textProcessor) tokenToPostingsLists(index invertedIndex, documentId int, token string, pos int, isTitle bool) error {
	tokenId, docsCount, err := p.db.GetTokenId(token)
	if err != nil {
		log.Println(err.Error())
		return err
	}
	item, ok := index[tokenId]
	if !ok {
		item = &tokenIndexItem{
			tokenId:        tokenId,
			positionsCount: 0,
			postings: &postingsList{
				documentId: documentId,
				positions:  nil,
				next:       nil,
			},
		}
		// 如果时检索时调用，则文档数量就是实际值；建索引时调用就是当前文档（个数1）
		if documentId == searchDocId {
			item.documentCount = docsCount
		} else {
			item.documentCount = 1
		}
		index[tokenId] = item
	}
	// 词元出现次数 +1
	item.postings.positions = append(item.postings.positions, pos)
	item.positionsCount++
	// 标题
	if isTitle {
		item.postings.titleEnd++
	}
	return nil
}

// 合并倒排索引
func (i invertedIndex) merge(index invertedIndex) {
	for tokenId, item := range index {
		baseItem, ok := i[tokenId]
		if ok {
			baseItem.documentCount += item.documentCount
			baseItem.positionsCount += item.positionsCount
			baseItem.postings = baseItem.postings.merge(item.postings)
		} else {
			i[tokenId] = item
		}
	}
}

// 合并倒排列表，文档ID小的列表项在前
func (p *postingsList) merge(postings *postingsList) *postingsList {
	if postings == nil {
		return p
	} else if p == nil {
		return postings
	}
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
	return head.next
}

// 将倒排列表编码成二进制数据，使用 variable-byte 编码压缩索引
func (p *postingsList) encode() []byte {
	var buf []byte
	tmp := make([]byte, binary.MaxVarintLen64)
	for ; p != nil; p = p.next {
		length := binary.PutVarint(tmp, int64(p.documentId))
		buf = append(buf, tmp[:length]...)
		length = binary.PutVarint(tmp, int64(len(p.positions)))
		buf = append(buf, tmp[:length]...)
		length = binary.PutVarint(tmp, int64(p.titleEnd))
		buf = append(buf, tmp[:length]...)
		for _, pos := range p.positions {
			length = binary.PutVarint(tmp, int64(pos))
			buf = append(buf, tmp[:length]...)
		}
	}
	return buf
}

// 将二进制数据解码成倒排列表
func decodePostings(data []byte) (*postingsList, int) {
	head := new(postingsList)
	p := head
	postingsLength := 0
	for pos := 0; pos < len(data); {
		p.next = new(postingsList)
		p = p.next
		postingsLength++
		x, length := binary.Varint(data[pos:])
		pos += length
		p.documentId = int(x)

		x, length = binary.Varint(data[pos:])
		pos += length
		sliceLen := int(x)

		x, length = binary.Varint(data[pos:])
		pos += length
		p.titleEnd = int(x)

		for i := 0; i < sliceLen; i++ {
			x, length = binary.Varint(data[pos:])
			pos += length
			p.positions = append(p.positions, int(x))
		}
	}
	return head.next, postingsLength
}

// 将 index 合并进索引管理器
func (m *indexManager) merge(index invertedIndex) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if len(m.indexBuffer) == 0 {
		m.indexBuffer = index
	} else {
		m.indexBuffer.merge(index)
	}
	m.indexCount++
	// 缓存数量大于阈值，异步刷新到存储器
	if m.indexCount > m.bufferFlushThreshold {
		go flushIndex(m.indexBuffer, m.db)
		m.indexBuffer = make(invertedIndex)
		m.indexCount = 0
	}
}

// 将内存中的缓冲的索引与存储器中的索引合并后刷新到存储器中
func flushIndex(index invertedIndex, destDB *db.IndexDB) {
	for tokenId, item := range index {
		// 从数据库中取出来旧的索引
		data, err := destDB.GetPostings(tokenId)
		if err != nil {
			log.Println(err.Error())
			continue
		}
		postings, docsCount := decodePostings(data)
		// 内存中合并
		postings = postings.merge(item.postings)
		// 写回
		data = postings.encode()
		err = destDB.UpdatePostings(tokenId, item.documentCount+docsCount, data)
		if err != nil {
			log.Println(err.Error())
		}
	}
}
