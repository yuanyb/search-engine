package robots

import (
	"strings"
	"testing"
)

func TestRobots(t *testing.T) {
	robots_txt := `
User-agent: xxx
Disallow: /
Allow: /th?

User-agent: test_crawler
Allow: /*/th?
Allow: /*/th$

User-agent: *
Disallow: /
Allow: /any`

	testCases := []struct {
		path  string
		allow bool
	}{
		{"/b/th", true},
		{"/b/th/c", false},
		{"/b/th?id=123", true},
		{"/a", false},
		{"/any", true},
		{"/any/abc", true},
	}
	robot := NewRobots(strings.NewReader(robots_txt), "test_crawler")
	for _, testCase := range testCases {
		if robot.Allow(testCase.path) != testCase.allow {
			t.Error(testCase.path)
		}
	}
}
