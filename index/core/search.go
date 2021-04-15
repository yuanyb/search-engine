// 检索
package core

import (
	"math"
	"search-engine/index/db"
	"search-engine/index/util"
	"sort"
	"strings"
)

type searcher struct {
	db            *db.IndexDB
	textProcessor *textProcessor
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

// 搜索结果
type SearchResults struct {
	Items      []*searchResultItem `json:"items"`
	Suggestion string              `json:"suggestion,omitempty"`
	Duration   int64               `json:"duration"` //  搜索耗时，毫秒
}

func (s *SearchResults) Push(x interface{}) {
	s.Items = append(s.Items, x.(*searchResultItem))
}

func (s *SearchResults) Pop() interface{} {
	ret := s.Items[len(s.Items)-1]
	s.Items = s.Items[:len(s.Items)-1]
	return ret
}

func (s *SearchResults) Len() int { return len(s.Items) }

func (s *SearchResults) Less(i, j int) bool { return s.Items[j].score < s.Items[i].score }

func (s *SearchResults) Swap(i, j int) { s.Items[i], s.Items[j] = s.Items[j], s.Items[i] }

// 取交集 list ∩ list2，list 和 list2 均有序
func (s *SearchResults) and(s2 *SearchResults) {
	if s.Items == nil {
		s.Items = s2.Items
		return
	}
	i, j, putIdx := 0, 0, 0
	len1, len2 := len(s.Items), len(s2.Items)
	for i < len1 {
		base := s.Items[i]
		for j < len2 && s2.Items[j].docId < base.docId {
			j++
		}
		if j >= len2 {
			break
		}
		if s2.Items[j].docId > base.docId {
			for i < len1 && s.Items[i].docId < s2.Items[j].docId {
				i++
			}
			continue
		}
		s.Items[putIdx] = base
		putIdx++
		i++
	}
	s.Items = s2.Items[:putIdx]
}

// 取差集 list1 - list2，list1 和 list2 均有序
func (s *SearchResults) not(s2 *SearchResults) {
	putIdx := 0
	set := make(map[int]struct{}, len(s2.Items))
	for _, item := range s2.Items {
		set[item.docId] = struct{}{}
	}
	for i, len1 := 0, len(s.Items); i < len1; i++ {
		if _, ok := set[s.Items[i].docId]; !ok {
			s.Items[putIdx] = s.Items[i]
			putIdx++
		}
	}
	s.Items = s.Items[:putIdx]
}

const (
	highlightPrefix = `<span style='color:red'>`
	highlightSuffix = `</span>`
)

// 获取结果信息及结果高亮
func (s *SearchResults) applyHighlight(db *db.IndexDB) {
	for _, item := range s.Items {
		url, title0, body0 := db.GetDocument(item.docId)
		title, body := []rune(title0), []rune(body0)

		item.Url = url

		builder := &strings.Builder{}
		var pos int
		// body 插入高亮标签
		if bh := item.bodyHighlight; len(bh) > 0 {
			// 计算网页摘要的区间
			start, end := bh[0][0], bh[len(bh)-1][1]
			padding := (100 - (end - start + 1)) / 2
			start = util.MaxInt(start-padding, 0)
			if start-padding >= 0 {
				end = util.MinInt(len(body), end+padding)
			} else {
				end = util.MinInt(len(body), end+padding-start)
			}
			abstract := body[start:end]
			for _, h := range item.bodyHighlight {
				h[0], h[1] = h[0]-start, h[1]-start
				builder.WriteString(string(abstract[pos:h[0]]))
				builder.WriteString(highlightPrefix)
				builder.WriteString(string(abstract[h[0] : h[1]+1]))
				builder.WriteString(highlightSuffix)
				pos = h[1] + 1
			}
			// 剩余的字符串
			if pos < len(abstract) {
				builder.WriteString(string(abstract[pos:]))
			}
			item.Abstract = builder.String()
		} else {
			item.Abstract = body0[:util.MinInt(100, len(body0))]
		}
		// title 插入高亮标签
		if len(item.titleHighlight) > 0 {
			// title:abcd1234  h:{{2,3}, {5,6}}  =>  ab <span...> cd </span> 1 <span...> 12 </span> 34
			builder.Reset()
			pos = 0
			for _, h := range item.titleHighlight {
				builder.WriteString(string(title[pos:h[0]]))
				builder.WriteString(highlightPrefix)
				builder.WriteString(string(title[h[0] : h[1]+1]))
				builder.WriteString(highlightSuffix)
				pos = h[1] + 1
			}
			// 剩余的字符串
			if pos < len(title) {
				builder.WriteString(string(title[pos:]))
			}
			item.Title = builder.String()
		} else {
			item.Title = title0
		}
	}
}

type searchResultItem struct {
	docId          int
	score          float64
	titleHighlight [][2]int // 结果高亮
	bodyHighlight  [][2]int
	// 返回的数据
	Url      string `json:"url"`
	Title    string `json:"title"`
	Abstract string `json:"abstract"`
}

func newSearcher(db *db.IndexDB, processor *textProcessor) *searcher {
	return &searcher{
		db:            db,
		textProcessor: processor,
	}
}

// 检索文档
func (s *searcher) searchDocs(query, site string) *SearchResults {
	results := &SearchResults{}
	queryTokens := s.textProcessor.queryToTokens(query)
	if len(queryTokens) == 0 {
		return &SearchResults{}
	}
	// 将 queryToken 按文档数量升序排序，这样可以尽早结束比较
	sort.Slice(queryTokens, func(i, j int) bool {
		return queryTokens[i].documentCount < queryTokens[j].documentCount
	})

	// 构建文档查询游标
	cursors := make([]docSearchCursor, len(queryTokens))
	for i, item := range queryTokens {
		// 词元 i 还没有建过索引
		if item == nil {
			return results
		}
		data := s.db.FetchPostings(item.token)
		postings, _ := decodePostings(data)
		// 词元 i 没有倒排列表
		if postings == nil {
			return results
		}
		cursors[i] = postings
	}

	// 检索候选文档，以第一个词元的倒排列表（最短）为基准
	for cursors[0] != nil {
		baseDocId := cursors[0].documentId
		nextDocId := -1
		// 对除基准词元外的所有词元，不断获其取下一个docId，直到当前docId不小于基准词元的docId
		for i := 1; i < len(cursors); i++ {
			for cursors[i] != nil && cursors[i].documentId < baseDocId {
				cursors[i] = cursors[i].next
			}
			// cursors[i] 中的所有文档id都小于baseDocId，则不可能有结果
			if cursors[i] == nil {
				return results
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
			u := s.db.GetDocumentUrl(baseDocId)
			if !strings.HasSuffix(util.UrlToHost(u), site) {
				continue
			}
		}
		item := &searchResultItem{docId: baseDocId}
		searchTitleOrBody := func(inTitle bool) {
			// 进行短语搜索
			phraseCount, highLight := searchPhrase(queryTokens, cursors, inTitle)
			// 打分
			docsCount := s.db.GetDocumentsCount()
			score := calcTfIdf(queryTokens, cursors, docsCount)
			if phraseCount > 0 {
				// 有完整短语权重更大，只要有完整短语，score 就至少3倍，凭感觉来的
				score *= 3 + math.Log(float64(phraseCount))
			}
			if inTitle { // 标题中的词元权值更高
				score *= 3
				item.titleHighlight = highLight
			} else {
				item.bodyHighlight = highLight
			}
			item.score += score
		}
		searchTitleOrBody(true)
		searchTitleOrBody(false)
		results.Items = append(results.Items, item)
		// 不能在 for 首部，因为循环体中有 continue
		cursors[0] = cursors[0].next
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
		// queryTokens[i] 和 docCursors[i] 是一一对应的
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
		nextOffset := offset
		// 对于除第一个词元以外的所有词元，不断地向后读取出现位置，
		// 直到其偏移量不小于第一个词元的偏移量为止
		for i := 1; i < len(cursors); i++ {
			for cursors[i].hasNextPos() && cursors[i].curPos()-cursors[i].base < offset {
				cursors[i].curIdx++
			}
			// 不能能再找到了
			if !cursors[i].hasNextPos() {
				return phraseCount, findHighlight(cursors)
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
	highLight = findHighlight(cursors)
	return phraseCount, highLight
}

// 构造高亮区间
func findHighlight(cursors []phraseSearchCursor) [][2]int {
	length := 0
	// 重置 curIdx
	for i := range cursors {
		cursors[i].curIdx = 0
		length += len(cursors[i].positions)
	}
	// 如果词元仅在标题（文档）中，而 findHighlight 又都会尝试查找，
	// 由于位置信息为空导致 intervals 为空，继续执行下面代码就会导致切片溢出 panic
	if length == 0 {
		return nil
	}
	// 将位置信息转换成区间信息
	intervals := make([][2]int, 0, length)
	for i := range cursors {
		for _, pos := range cursors[i].positions {
			intervals = append(intervals, [2]int{pos, pos + 1}) // todo N(n-gram)
		}
	}
	// 排序合并区间
	sort.Slice(intervals, func(i, j int) bool {
		return intervals[i][0] < intervals[j][0]
	})
	pos := 0
	for i := 1; i < len(intervals); i++ {
		// query:ABC  DOC:ABCABGC  =>  AB:{0,3} BC:{1} => {0,1,3} => {0,1} {1,2}, {3,4} => {0,4}
		if intervals[i][0]-intervals[i-1][1] <= 1 {
			intervals[pos][1] = intervals[i][1]
		} else {
			pos++
			intervals[pos] = intervals[i]
		}
	}
	intervals = intervals[:pos+1]

	LEN := func(l [][2]int, i, j int) int {
		return l[j][1] - l[i][0] + 1
	}
	// 100长度的滑动窗口，找到包含最长短语的窗口
	maxLen, maxLenIdx := 0, 0
	for i := 0; i < len(intervals); i++ {
		if maxLen < LEN(intervals, i, i) {
			maxLen = LEN(intervals, i, i)
			maxLenIdx = i
		}
	}
	i, j := maxLenIdx, maxLenIdx
	for LEN(intervals, i, j) > 100 && (i > 0 || j < len(intervals)-1) {
		len1, len2 := 0, 0
		if i > 0 {
			len1 = LEN(intervals, i-1, i-1)
		}
		if j < len(intervals)-1 {
			len2 = LEN(intervals, j+1, j+1)
		}
		if len1 > len2 {
			i--
		} else {
			j++
		}
	}
	return intervals[i : j+1]
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
	for i, item := range tokens {
		// cursors[tokenId]对应一篇文档的倒排列表项，它的positions就是在对应文档的出现次数
		TF := 1 + math.Log(float64(len(cursors[i].positions)))
		//
		IDF := math.Log(float64(docsCount) / float64(item.documentCount))
		score += TF * IDF
	}
	return score
}
