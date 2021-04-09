// 布隆过滤器，用于记录已经抓取过的网页
package core

import (
	"search-engine/crawler/util"
)

const hashFuncCount = 5

// 计算哈希的种子值
var seeds = [hashFuncCount]int{31, 37, 61, 17, 13}

// 哈希函数数组
var hashFunc [hashFuncCount]func(string) int

// 初始化哈希函数
func init() {
	for i := 0; i < hashFuncCount; i++ {
		seed := seeds[i]
		hashFunc[i] = func(url string) int {
			h := 0
			for _, ch := range url {
				h = h*seed + int(ch)
			}
			return util.AbsInt(h)
		}
	}
}

type BloomFilter struct {
	bitmap []uint64
}

// 判断 url 是否已经爬取过
func (b *BloomFilter) Has(url string) bool {
	for i := 0; i < hashFuncCount; i++ {
		h := hashFunc[i](url) % (len(b.bitmap) << 6)
		if b.bitmap[h>>6]&(1<<(h&63)) == 0 {
			return false
		}
	}
	return true
}

// 将 url 添加到布隆过滤器中
func (b *BloomFilter) Add(url string) {
	for i := 0; i < hashFuncCount; i++ {
		h := hashFunc[i](url) % (len(b.bitmap) << 6)
		b.bitmap[h>>6] |= 1 << (h & 63)
	}
}

// 暂时考虑网页的更新策略是清空 bloomFilter
func (b *BloomFilter) Clear() {
	for i := range b.bitmap {
		b.bitmap[i] = 0
	}
}

func Load() {

}

func Serialized() {

}

func NewBloomFilter(size int) *BloomFilter {
	// 100w / 4 = 25w;  25w * 8b = 200wb; 200wb / 1024 / 1024 ≈ 2mb
	size >>= 3 // 比 size 大 8 倍
	if size <= 0 {
		size = 1
	}
	bf := &BloomFilter{
		bitmap: make([]uint64, size),
	}
	return bf
}
