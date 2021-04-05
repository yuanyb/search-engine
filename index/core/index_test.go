package core

import (
	"testing"
)

func buildPostingsList(baseId int) *postingsList {
	p1 := &postingsList{
		documentId: baseId,
		positions:  []int{1, 2, 3},
	}
	p2 := &postingsList{
		documentId: baseId + 1,
		positions:  []int{4, 6, 8},
	}
	p3 := &postingsList{
		documentId: baseId + 2,
		positions:  []int{6, 7, 8},
	}
	p1.next, p2.next = p2, p3
	return p1
}

func TestPostingsList_Encode(t *testing.T) {
	p := buildPostingsList(0)
	data := p.encode()

	// test
	pa, pb := p, decodePostings(data)
	for ; pa != nil && pb != nil; pa, pb = pa.next, pb.next {
		if pa.documentId != pb.documentId || len(pa.positions) != len(pb.positions) {
			t.Error("failed")
		}
		for i := 0; i < len(pa.positions) && i < len(pb.positions); i++ {
			if pa.positions[i] != pb.positions[i] {
				t.Error("failed")
			}
		}
	}
	if pa != nil || pb != nil {
		t.Error("failed")
	}
}

func TestPostingsList_Merge(t *testing.T) {
	p1, p2 := buildPostingsList(5), buildPostingsList(0)
	docIdList := []int{0, 1, 2, 5, 6, 7}
	p1 = p1.merge(p2)
	for i := 0; p1 != nil; p1, i = p1.next, i+1 {
		if p1.documentId != docIdList[i] {
			t.Errorf("p1:%d, excepted:%d\n", p1.documentId, docIdList[i])
		}
	}
}
