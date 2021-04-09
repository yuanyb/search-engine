package core

import (
	"fmt"
	"strconv"
	"testing"
)

func TestNewBloomFilter(t *testing.T) {
	bf := NewBloomFilter(4)
	bf.Add("http://baidu.com/")
	fmt.Println(bf.Has("http://baidu.com/"))
	bf.Add("http://google.com/")
	fmt.Println(bf.Has("http://google.com/"))
	bf.Add("http://bing.com/")
	fmt.Println(bf.Has("http://bing.com/"))
	bf.Add("http://yahoo.com/")
	fmt.Println(bf.Has("http://yahoo.com/"))
	println("=========")
	count := 0
	for i := 0; i < 10000; i++ {
		if bf.Has(strconv.Itoa(i*61) + "base_str") {
			count++
		}
	}
	fmt.Println("fail count", count)
}
