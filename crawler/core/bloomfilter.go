// 布隆过滤器，用于记录已经抓取过的网页
package core

import (
	"context"
	"github.com/go-redis/redis/v8"
	"log"
	"search-engine/crawler/db"
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

type BloomFilter interface {
	has(url string) bool
	add(url string)
}
type LocalBloomFilter struct {
	bitmap []uint64
}

// 判断 url 是否已经爬取过
func (b *LocalBloomFilter) has(url string) bool {
	for i := 0; i < hashFuncCount; i++ {
		h := hashFunc[i](url) % (len(b.bitmap) << 6)
		if b.bitmap[h>>6]&(1<<(h&63)) == 0 {
			return false
		}
	}
	return true
}

// 将 url 添加到布隆过滤器中
func (b *LocalBloomFilter) add(url string) {
	for i := 0; i < hashFuncCount; i++ {
		h := hashFunc[i](url) % (len(b.bitmap) << 6)
		b.bitmap[h>>6] |= 1 << (h & 63)
	}
}

//// 暂时考虑网页的更新策略是清空 bloomFilter
// todo
//func (b *LocalBloomFilter) Clear() {
//	for i := range b.bitmap {
//		b.bitmap[i] = 0
//	}
//}
//
//func Load() {
//
//}
//
//func Serialized() {
//
//}

func NewLocalBloomFilter(maxDocCount int) BloomFilter {
	// 100w / 4 = 25w;  25w * 8b = 200wb; 200wb / 1024 / 1024 ≈ 2mb
	maxDocCount >>= 3 // 比 maxDocCount 大 8 倍
	if maxDocCount <= 0 {
		maxDocCount = 1
	}
	bf := &LocalBloomFilter{
		bitmap: make([]uint64, maxDocCount),
	}
	return bf
}

/////////// 分布式调度 BloomFilter ////////////

type DistBloomFilter struct {
	redis       *redis.Client
	maxDocCount int
}

var (
	bfHasLuaScript = `
for i, v in ipairs(ARGV)
do
	if (redis.call("getbit", KEYS[1], v) == 0)
	then
		return 0
	end
end
return 1
`
	bfAddLuaScript = `
for i, v in ipairs(ARGV)
do
	redis.call("setbit", KEYS[1], v, 1)
end
`
	bfHasSHA1 string
	bfAddSHA1 string
)

func (d *DistBloomFilter) has(url string) bool {
	var argv = make([]interface{}, hashFuncCount)
	for i := 0; i < hashFuncCount; i++ {
		argv[i] = hashFunc[i](url) % d.maxDocCount
	}
	c := d.redis.EvalSha(context.Background(), bfHasSHA1, []string{"dist_bloom_filter"}, argv...)
	if c.Err() != nil {
		return false
	}
	ret, _ := c.Int()
	return ret == 1
}

func (d *DistBloomFilter) add(url string) {
	var argv = make([]interface{}, hashFuncCount)
	for i := 0; i < hashFuncCount; i++ {
		argv[i] = hashFunc[i](url) % d.maxDocCount
	}
	c := d.redis.EvalSha(context.Background(), bfAddSHA1, []string{"dist_bloom_filter"}, argv...)
	if c.Err() != nil && c.Err() != redis.Nil {
		log.Println("添加 url 到 redis-bloomFilter 时失败")
	}
}

func NewDistBloomFilter(maxDocCount int) BloomFilter {
	bf := &DistBloomFilter{
		redis:       db.Redis,
		maxDocCount: maxDocCount,
	}
	r := bf.redis.ScriptLoad(context.Background(), bfAddLuaScript)
	if r.Err() != nil {
		log.Fatalln("加载脚本失败")
	}
	bfAddSHA1 = r.Val()
	r = bf.redis.ScriptLoad(context.Background(), bfHasLuaScript)
	if r.Err() != nil {
		log.Fatalln("加载脚本失败")
	}
	bfHasSHA1 = r.Val()
	return bf
}
