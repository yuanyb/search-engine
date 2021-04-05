// 检索
package core

import (
	"container/heap"
	"math"
	"search-engine/index/db"
	"search-engine/index/util"
	"sort"
	"strings"
)

// buffer 转移到db包中
type searcher struct {
	db *db.IndexDB
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

type SearchResults struct {
	Items      []searchResultItem
	suggestion string
}

func (s SearchResults) Push(x interface{}) {
	s.Items = append(s.Items, x.(searchResultItem))
}

func (s SearchResults) Pop() interface{} {
	ret := s.Items[len(s.Items)-1]
	s.Items = s.Items[:len(s.Items)-1]
	return ret
}

func (s SearchResults) Len() int { return len(s.Items) }

func (s SearchResults) Less(i, j int) bool { return s.Items[j].score < s.Items[i].score }

func (s SearchResults) Swap(i, j int) { s.Items[i], s.Items[j] = s.Items[j], s.Items[i] }

// 结果集合 s and s2 操作
func (s SearchResults) and(s2 SearchResults) {
	if s.Items == nil {
		s.Items = s2.Items
		return
	}
	set := make(map[int]*searchResultItem)
	for _, item := range s2.Items {
		set[item.documentId] = &item
	}
	newItems := make([]searchResultItem, 0)
	for _, item := range s.Items {
		if item, ok := set[item.documentId]; ok {
			newItems = append(newItems, *item)
		}
	}
	s.Items = newItems
}

// 结果集合 s not s2 操作
func (s SearchResults) not(s2 SearchResults) {
	set := make(map[int]*searchResultItem)
	for _, item := range s2.Items {
		set[item.documentId] = &item
	}
	newItems := make([]searchResultItem, 0)
	for _, item := range s.Items {
		if item, ok := set[item.documentId]; !ok {
			newItems = append(newItems, *item)
		}
	}
	s.Items = newItems
}

type searchResultItem struct {
	documentId     int
	score          float64
	titleHighLight [][2]int // 结果高亮
	bodyHighLight  [][2]int // 结果高亮
}

func newSearcher(db *db.IndexDB, postingsBufferSize int) *searcher {
	return &searcher{
		db: db,
	}
}

// 检索文档
// 没必要返回和在内存中保存全部结果，因为使用者也看不了那么多结果
func (s *searcher) searchDocs(queryTokens []*tokenIndexItem, site string) SearchResults {
	// 将 queryToken 按文档数量升序排序，这样可以尽早结束比较
	sort.Slice(queryTokens, func(i, j int) bool {
		return queryTokens[i].documentCount < queryTokens[j].documentCount
	})

	// 构建文档查询游标
	cursors := make([]docSearchCursor, len(queryTokens))
	for i, item := range queryTokens {
		// 词元 i 还没有建过索引
		if item == nil {
			return SearchResults{}
		}
		data, err := s.db.GetPostings(item.tokenId)
		if err != nil {
			// todo log
			return SearchResults{}
		}
		postings := decodePostings(data)
		// 词元 i 没有倒排列表
		if postings == nil {
			return SearchResults{}
		}
		cursors[i] = postings
	}

	results := SearchResults{Items: make([]searchResultItem, 50)}
	heap.Init(results)

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
				return SearchResults{}
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
		// 如果该文档的URL不是指定域名下的
		if site != "" {
			u, err := s.db.GetDocUrl(baseDocId)
			if err != nil || !strings.HasSuffix(util.UrlToHost(u), site) {
				continue
			}
		}
		item := &searchResultItem{documentId: baseDocId}
		searchTitleOrBody := func(inTitle bool) {
			// 进行短语搜索
			phraseCount, highLight := searchPhrase(queryTokens, cursors, true)
			// 打分
			docsCount, err := s.db.GetDocumentsCount()
			if err != nil {
				// todo log
			}
			score := calcTfIdf(queryTokens, cursors, docsCount)
			if phraseCount > 0 {
				// 有完整短语权重更大，只要有完整短语，score 就至少3倍，凭感觉来的
				score *= 3 + math.Log(float64(phraseCount))
			}
			if inTitle { // 标题中的词元权值更高
				score *= 3
				item.titleHighLight = highLight
			} else {
				item.bodyHighLight = highLight
			}
			item.score += score
		}
		// 使用优先级队列限制结果数
		results.Push(item)
		if results.Len() > 100 {
			heap.Pop(results) // 移除 score 最小的结果项
		}
		searchTitleOrBody(true)
		searchTitleOrBody(false)
	}
	return results
}

// 检索文档或标题中是否存在完全匹配的短语
// 还要返回不完整短语的位置信息，用于结果高亮
// inTitle=true 表示在title中查询，否则在body中查询
func searchPhrase(queryTokens []*tokenIndexItem, docCursors []docSearchCursor, inTitle bool) (int, [][2]int) {
	phraseCount := 0
	highLight := make([][2]int, 0)
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
			if inTitle {
				cursors[cursorPos].positions = docCursors[i].positions[:docCursors[i].titleEnd]
			} else {
				cursors[cursorPos].positions = docCursors[i].positions[docCursors[i].titleEnd:]
			}
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
		maxNextOffset, nextOffset := offset, offset
		// 对于除第一个词元以外的所有词元，不断地向后读取出现位置，
		// 直到其偏移量不小于第一个词元的偏移量为止
		for i := 1; i < len(cursors); i++ {
			for cursors[i].hasNextPos() && cursors[i].curPos()-cursors[i].base < offset {
				cursors[i].curIdx++
			}
			// 不能能再找到了
			if !cursors[i].hasNextPos() {
				return phraseCount, highLight
			}
			// 对于其他词元，如果偏移量不等于第一个词元的偏移量就退出循环
			if cursors[i].curPos()-cursors[i].base != offset {
				nextOffset = cursors[i].curPos() - cursors[i].base
				break
			}
			maxNextOffset = util.MaxInt(maxNextOffset, nextOffset)
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
			// 记住位置
			highLight = append(highLight, [2]int{offset, maxNextOffset + 1})
		}
	}
	if len(highLight) == 0 {
		highLight = append(highLight, findHighLight(cursors)...)
	}
	return phraseCount, highLight
}

// 构造高亮区间
func findHighLight(cursors []phraseSearchCursor) [][2]int {
	length := 0
	for i := range cursors {
		cursors[i].curIdx = 0
		length += len(cursors[i].positions)
	}
	intervals := make([][2]int, length)
	for i := range cursors {
		for _, pos := range cursors[i].positions {
			intervals = append(intervals, [2]int{pos, pos + 1})
		}
	}
	sort.Slice(intervals, func(i, j int) bool {
		return intervals[i][0] < intervals[j][0]
	})
	pos := 0
	for i := 1; i < len(intervals); i++ {
		// query:ABC  DOC:ABCXABGC  =>  AB:{0,4} BC:{1} => {0,1} {1,2}, {4,5} => {0,5}
		if intervals[i][0]-intervals[i-1][1] <= 2 {
			intervals[pos][1] = intervals[i][1]
		} else {
			pos++
			intervals[pos] = intervals[i]
		}
	}
	return intervals[:pos+1] // 100长度的滑动窗口，判断哪个窗口高亮字符最多
}

func calcBM25() float64 {
	return 0
}

//   TF 词频因子，表示一个单词在文档中出现的次数，一般在某个文档中反复出现的单词，
// 往往能够表示文档的主题，即TF值越大，月能代表文档反应的内容，那么应给这个单词更大的权值。
// 直接使用词频数作为TF值，不太准确，如单词T在D1中出现10次，在D2中出现了1次数次，不应该权
// 值大10倍。因此一种计算公式为：W_tf = 1 + log(TF)，即词频数取log值来抑制过大的差异，+1
// 是为了平滑结果，当log(TF)为0时，不至于权值为0。
//   IDF 逆文档频率因子，表示文档集合范围的一种全局因子，`IDF = log(N/n_k)`，N是文档集合
// 的文档个数，n_k表示单词k在其中多少个文档出现过（即文档频率），由公式可知，n_k越高，则其
// IDF越小，即越多的文档包含某个单词，那么其IDF值越小，这个词区分不同文档的能力越差。
//   TF*IDF 结合了两个特征向量。
func calcTfIdf(tokens []*tokenIndexItem, cursors []docSearchCursor, docsCount int) float64 {
	var score float64
	for tokenId, item := range tokens {
		// cursors[tokenId]对应一篇文档的倒排列表项，它的positions就是在对应文档的出现次数
		TF := 1 + math.Log(float64(len(cursors[tokenId].positions)))
		//
		IDF := math.Log(float64(docsCount) / float64(item.documentCount))
		score += TF * IDF
	}
	return score
}
