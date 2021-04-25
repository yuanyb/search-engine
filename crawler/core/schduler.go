package core

import (
	"container/heap"
	"container/list"
	"context"
	"github.com/go-redis/redis/v8"
	"log"
	"search-engine/crawler/config"
	"search-engine/crawler/db"
)

// Scheduler 表示爬虫的抓取 URL 的调度策略
type Scheduler interface {
	Offer(group urlGroup)
	Poll() string
	Front() string
	Empty() bool
	AddSeedUrls([]string)
}

// Breath first
type BFScheduler struct {
	queue *list.List
}

func (b *BFScheduler) Poll() string {
	e := b.queue.Front()
	url := b.queue.Remove(e).(string)
	return url
}

func (b *BFScheduler) Offer(group urlGroup) {
	for _, url := range group.members {
		b.queue.PushBack(url)
	}
}

func (b *BFScheduler) Front() string {
	return b.queue.Front().Value.(string)
}

func (b *BFScheduler) Empty() bool {
	return b.queue.Len() == 0
}

func (b *BFScheduler) AddSeedUrls(seedUrls []string) {
	for _, seedUrl := range seedUrls {
		// 初始化种子 url 的 robots.txt
		if Allow(seedUrl, config.Get().Useragent) {
			b.queue.PushBack(seedUrl)
		}
	}
}

func NewBFScheduler() Scheduler {
	scheduler := &BFScheduler{
		queue: list.New(),
	}
	return scheduler
}

// priority queue
type priorityQueue struct {
	array []string
	// pq 所属的 OICPStrategy，用于访问 cashMap
	opic *OPICScheduler
}

func (p *priorityQueue) Len() int {
	return len(p.array)
}

func (p *priorityQueue) Less(i, j int) bool {
	return p.opic.cashMap[p.array[i]] < p.opic.cashMap[p.array[j]]
}

func (p *priorityQueue) Swap(i, j int) {
	p.array[i], p.array[j] = p.array[j], p.array[i]
}

func (p *priorityQueue) Push(x interface{}) {
	p.array = append(p.array, x.(string))
}

func (p *priorityQueue) Pop() interface{} {
	last := p.array[len(p.array)-1]
	p.array = p.array[:len(p.array)-1]
	return last
}

// Online page importance computation
// 规定每个链接初始 cash 值为 1
type OPICScheduler struct {
	pq priorityQueue
	// 保存 scheduler 对应的 cash，为了节省内存，
	// 下载完某个 URL 对应的页面后，要删除 cashMap 中的 k、v
	cashMap map[string]float32
}

func (o *OPICScheduler) Offer(group urlGroup) {
	avg := o.cashMap[group.leader] / float32(len(group.members))
	delete(o.cashMap, group.leader)
	for _, member := range group.members {
		if _, ok := o.cashMap[member]; !ok {
			o.pq.Push(member)
			o.cashMap[member] = 1.0
		}
		o.cashMap[member] += avg
	}
}

func (o *OPICScheduler) Poll() string {
	url := o.pq.Pop().(string)
	// Offer scheduler 对应的 urlGroup 的时候再删除，因为还需要 scheduler 的 value
	//delete(o.pq.cashMap, scheduler)
	return url
}

func (o *OPICScheduler) Front() string {
	return o.pq.array[0]
}

func (o *OPICScheduler) Empty() bool {
	return o.pq.Len() == 0
}

func (o *OPICScheduler) AddSeedUrls(seedUrls []string) {
	for _, seedUrl := range seedUrls {
		o.cashMap[seedUrl] = 1.0
		o.pq.Push(seedUrl)
	}
}

func NewOPICScheduler() Scheduler {
	opic := &OPICScheduler{
		cashMap: make(map[string]float32, 10000),
		pq: priorityQueue{
			array: make([]string, 10000),
		},
	}
	opic.pq.opic = opic
	heap.Init(&opic.pq)
	return opic
}

/////////////////// 简单的分布式调度 //////////////////////

type DistributedScheduler struct {
	localQueue *list.List
	redis      *redis.Client
}

var (
	ctx          = context.Background()
	distQueueKey = "dist_url_queue"
)

func (d *DistributedScheduler) fetch() {
	var result []*redis.StringCmd
	pipeline := d.redis.Pipeline()
	// 每次最多100个
	for i := 0; i < 100; i++ {
		result = append(result, pipeline.LPop(ctx, distQueueKey))
	}
	if _, err := pipeline.Exec(ctx); err != nil && err != redis.Nil {
		log.Println("从 redis 队列获取 url 时发生错误", err)
	}
	for _, r := range result {
		if r.Err() == nil {
			d.localQueue.PushBack(r.Val())
		}
	}
}

func (d *DistributedScheduler) Offer(group urlGroup) {
	var urlList []interface{}
	for _, u := range group.members {
		urlList = append(urlList, u)
	}
	if d.redis.RPush(ctx, distQueueKey, urlList).Err() != nil {
		log.Println("发送 urlList 到 redis 队列时发生错误")
	}
}

func (d *DistributedScheduler) Poll() string {
	if d.localQueue.Len() == 0 {
		d.fetch()
	}
	e := d.localQueue.Front()
	url := d.localQueue.Remove(e).(string)
	return url
}

func (d *DistributedScheduler) Front() string {
	return d.localQueue.Front().Value.(string)
}

func (d *DistributedScheduler) Empty() bool {
	if d.localQueue.Len() != 0 {
		return false
	}
	d.fetch()
	return d.localQueue.Len() == 0
}

func (d *DistributedScheduler) AddSeedUrls(seedUrls []string) {
	var urlList []interface{}
	for _, seedUrl := range seedUrls {
		// 初始化种子 url 的 robots.txt
		if Allow(seedUrl, config.Get().Useragent) {
			urlList = append(urlList, seedUrl)
		}
	}
	if d.redis.RPush(ctx, distQueueKey, urlList...).Err() != nil {
		log.Fatalln("添加种子 URL 失败")
	}
}

func NewDistributedScheduler() Scheduler {
	scheduler := &DistributedScheduler{
		localQueue: list.New(),
		redis:      db.Redis,
	}
	return scheduler
}
