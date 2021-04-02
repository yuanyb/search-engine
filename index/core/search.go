// 检索
package core

import (
	"sort"
)

type searcher struct {
	indexManager *indexManager
}

// 文档查询游标，用于指示当前词元处理到了哪个文档
type docSearchCursor = *postingsList

// 短语查询游标
type phraseSearchCursor struct {
	positions []int // 词元在文档中位置信息
	base      int   // 词元在查询中的位置
	curIdx    int   // 游标的当前位置 [0, len(positions))
}

func (c *phraseSearchCursor) curPos() int {
	return c.positions[c.curIdx]
}

func (c *phraseSearchCursor) hasNextPos() bool {
	return c.curIdx < len(c.positions)
}

type searchResults []searchResultItem

type searchResultItem struct {
	documentId int
	score      float64
}

// 检索文档
// 没必要返回和在内存中保存全部结果，因为使用者也看不了那么多结果
// abc xyz -ijk 通过空格切割，每部分再短语查询，'-'取补集
// site:host 索引：host -> docId list
func (s *searcher) searchDocs(queryTokens []*tokenIndexItem) (results searchResults) {
	// 将 queryToken 按文档数量升序排序，这样可以尽早结束比较
	sort.Slice(queryTokens, func(i, j int) bool {
		return queryTokens[i].documentCount < queryTokens[j].documentCount
	})

	// 构建文档查询游标
	cursors := make([]docSearchCursor, len(queryTokens))
	for i, item := range queryTokens {
		// 词元 i 还没有建过索引
		if item == nil {
			return searchResults{}
		}
		postings := s.indexManager.fetchPostingsList(item.tokenId)
		// 词元 i 没有倒排列表
		if postings == nil {
			return searchResults{}
		}
		cursors[i] = postings
	}

	// 检索候选文档，以第一个词元的倒排列表（最短）为基准
	for ; cursors[0] != nil; cursors[0] = cursors[0].next {
		baseDocId := cursors[0].documentId
		nextDocId := -1
		// 对除基准词元外的所有词元，不断获其取下一个docId，直到当前docId不小于基准词元的docId
		for i := 1; i < len(cursors); i++ {
			for cursors[i] != nil && cursors[i].documentId < baseDocId {
				cursors[i] = cursors[i].next
			}
			// cursors[i] 中的所有文档id都小于baseDocId，则不可能有结果
			if cursors[i] == nil {
				return searchResults{}
			}
			if cursors[i].documentId > baseDocId {
				nextDocId = cursors[i].documentId
				break
			}
		}
		if nextDocId > 0 {
			// 不断获取基准词元的下一个docId，直到不小于nextDocId
			for cursors[0] != nil && cursors[0].documentId < nextDocId {
				cursors[0] = cursors[0].next
			}
			continue
		}
		// 进行短语搜索，如果短语搜索的结果过于少或者没有，
		// 则可以返回包含查询内容最长部分的网页，或者进行同义词替换
		phraseCount := searchPhrase(queryTokens, cursors)
		if phraseCount > 0 {
			// 打分
			score := 0.0 // calcTfIdf()
			results = append(results, searchResultItem{
				documentId: baseDocId,
				score:      score,
			})
		}
	}
	return
}

// 检索文档中是否存在完全匹配的短语
func searchPhrase(queryTokens []*tokenIndexItem, docCursors []docSearchCursor) int {
	phraseCount := 0
	// 初始化游标
	count := 0 // 查询中的词元的总数
	for _, item := range queryTokens {
		count += item.positionsCount // 之所以加上posCount是因为某个词元可能重复出现了
	}
	cursors := make([]phraseSearchCursor, count)
	cursorPos := 0
	for i, item := range queryTokens {
		// query:aba123，这个查询中，a出现了两次，a对应item的positions就是 {0,2}
		for _, pos := range item.postings.positions {
			cursors[cursorPos].base = pos
			cursors[cursorPos].positions = docCursors[i].positions
			cursors[cursorPos].curIdx = 0
			cursorPos++
		}
	}

	// 检索短语
	for cursors[0].hasNextPos() {
		// 用第一个词元在文档中的出现位置减去在查询短语中出现的位置得到该词元的偏移量，
		// 然后逐一获得其他词元的偏移量，对比是否和第一个词元的偏移量相等，如果不相等
		// 说明文档中的各个词元不相邻，也就不存在短语
		offset := cursors[0].curPos() - cursors[0].base
		nextOffset := offset
		// 对于除第一个词元以外的所有词元，不断地向后读取出现位置，
		// 直到其偏移量不小于第一个词元的偏移量为止
		for i := 1; i < len(cursors); i++ {
			for cursors[i].hasNextPos() && cursors[i].curPos()-cursors[i].base < offset {
				cursors[i].curIdx++
			}
			// 不能能再找到了
			if !cursors[i].hasNextPos() {
				return phraseCount
			}
			// 对于其他词元，如果偏移量不等于第一个词元的偏移量就退出循环
			if cursors[i].curPos()-cursors[i].base != offset {
				nextOffset = cursors[i].curPos() - cursors[i].base
				break
			}
		}
		if nextOffset > offset {
			// 不断向后读取，直到第一个词元的偏移量不小于nextOffset为止
			for cursors[0].hasNextPos() && cursors[0].curPos()-cursors[0].base < nextOffset {
				cursors[0].curIdx++
			}
		} else {
			// 找到了短语
			phraseCount++
			cursors[0].curIdx++
		}
	}
	return phraseCount
}

func calcBM25() float64 {
	return 0
}

func calcTfIdf() float64 {
	return 0
}
