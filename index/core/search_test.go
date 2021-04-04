package core

import (
	"testing"
)

func TestFindHighLight(t *testing.T) {
	// query:ABC  DOC:ABCXABGC  =>  AB:{0,4} BC:{1} => {0,1} {1,2}, {4,5} => {0,5}
	var cursors []phraseSearchCursor
	cursors = append(cursors, phraseSearchCursor{positions: []int{0, 4}}) // AB
	cursors = append(cursors, phraseSearchCursor{positions: []int{1}})    // BC
	h := findHighLight(cursors)
	except := [][2]int{{0, 5}}
	if h[0] != except[0] {
		t.Error(h)
	}
}
