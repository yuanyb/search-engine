package core

import (
	"fmt"
	"testing"
)

func Test_nGramSplit(t *testing.T) {
	str := "hello,你好，world.世界。"
	nGramSplit(str, 1, func(token string, pos int) error {
		fmt.Println(token, pos)
		return nil
	})
}
