package core

import (
	"search-engine/index/util"
	"testing"
)

func TestFindHighLight(t *testing.T) {
	// query:ABC  DOC:ABCXABGC  =>  AB:{0,4} BC:{1} => {0,1} {1,2}, {4,5} => {0,5}
	var cursors []phraseSearchCursor
	cursors = append(cursors, phraseSearchCursor{positions: []int{0, 4}}) // AB
	cursors = append(cursors, phraseSearchCursor{positions: []int{1}})    // BC
	h := findHighlight(cursors)
	except := [][2]int{{0, 5}}
	if !util.IntSliceEquals(h, except) {
		t.Error(h)
	}
	// 测试滑动窗口
	//  {1,2,3,4,9,19,200,201,202,203,204,205,215,217,324,456}
	//↓ {1,5}_5, {9,10}_2, {19,20}_2, {200,206}_7, {215,218}_4, {324,325}_2, {456,457}_2
	//↓ {        length:9        }  {      length:11       }
	cursors = cursors[:0]
	cursors = append(cursors, phraseSearchCursor{positions: []int{1, 2, 3, 4, 9, 19, 200, 201, 202, 203, 204, 205, 215, 217, 324, 456}})
	h = findHighlight(cursors)
	except = [][2]int{{200, 206}, {215, 218}}
	if !util.IntSliceEquals(h, except) {
		t.Error(h)
	}
}
