// 索引
package core

import (
	"encoding/binary"
	"search-engine/index/db"
)

// 索引管理器
type indexManager struct {
	indexBuffer          invertedIndex
	bufferFlushThreshold int
	bufferCount          int
	db                   *db.DB
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
			baseItem.postings = baseItem.postings.merge(item.postings)
		} else {
			i[tokenId] = item
		}
	}
}

// 合并倒排列表，文档ID小的列表项在前
func (p *postingsList) merge(postings *postingsList) *postingsList {
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

// 将倒排列表编码成二进制数据
func (p *postingsList) encode() []byte {
	size := 0
	for p2 := p; p2 != nil; p2 = p2.next {
		size += 4 + 4 + 4*len(p2.positions)
	}
	buf := make([]byte, size)
	for pos := 0; p != nil; p = p.next {
		binary.BigEndian.PutUint32(buf[pos:], uint32(p.documentId))
		pos += 4
		binary.BigEndian.PutUint32(buf[pos:], uint32(len(p.positions)))
		pos += 4
		for _, v := range p.positions {
			binary.BigEndian.PutUint32(buf[pos:], uint32(v))
			pos += 4
		}
	}
	return buf
}

// 将二进制数据解码成倒排列表
func decodePostingsList(data []byte) *postingsList {
	head := new(postingsList)
	p := head
	for pos := 0; pos < len(data); {
		p.next = new(postingsList)
		p = p.next
		p.documentId = int(binary.BigEndian.Uint32(data[pos:]))
		pos += 4
		length := int(binary.BigEndian.Uint32(data[pos:]))
		pos += 4
		for i := 0; i < length; i++ {
			p.positions = append(p.positions, int(binary.BigEndian.Uint32(data[pos:])))
			pos += 4
		}
	}
	return head.next
}

// 将 index 合并进索引管理器
func (m *indexManager) merge(index invertedIndex) {
	if m.indexBuffer == nil {
		m.indexBuffer = index
		return
	}
	m.indexBuffer.merge(index)
	// 缓存数量大于阈值，刷新到存储器
	if m.bufferCount > m.bufferFlushThreshold {
		m.flush()
	}
}

// 将内存中的缓冲的索引与存储器中的索引合并后刷新到存储器中
func (m *indexManager) flush() {
	for tokenId, item := range m.indexBuffer {
		// 从数据库中取出来旧的索引
		postings := m.fetchPostingsList(tokenId)
		if postings == nil {
			continue
		}
		// 内存中合并
		postings = postings.merge(item.postings)
		// 写回
		data := postings.encode()
		err := m.db.UpdatePostingsList(tokenId, data)
		if err != nil {
			// todo log
		}
	}
	m.indexBuffer = make(invertedIndex)
}

func (m *indexManager) fetchPostingsList(tokenId int) *postingsList {
	// 先判断缓存是否命中
	// ...
	data, err := m.db.GetPostingsList(tokenId)
	if err != nil || len(data) == 0 {
		// todo log
		return nil
	}
	return decodePostingsList(data)
}
