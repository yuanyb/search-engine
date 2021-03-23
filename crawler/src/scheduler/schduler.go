package scheduler

import (
	"container/heap"
	"container/list"
)

// Strategy 表示爬虫的抓取 URL 的优先级策略
type Scheduler interface {
	Offer(group UrlGroup)
	Poll() string
	AddSeedUrls([]string)
}

// Breath first
type BFScheduler struct {
	queue *list.List
}

func (b *BFScheduler) Poll() string {
	url := b.queue.Remove(b.queue.Front()).(string)
	return url
}

func (b *BFScheduler) Offer(group UrlGroup) {
	for _, url := range group.Members {
		b.queue.PushBack(url)
	}
}

func (b *BFScheduler) AddSeedUrls(seedUrls []string) {
	for _, seedUrl := range seedUrls {
		b.queue.PushBack(seedUrl)
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
	// 保存 scheduler 对应的 cash，为了节省内存，下载完某个 URL 对应的页面后，要删除 cashMap 中的 k、v
	cashMap map[string]float32
}

func (o *OPICScheduler) Offer(group UrlGroup) {
	avg := o.cashMap[group.Leader] / float32(len(group.Members))
	delete(o.cashMap, group.Leader)
	for _, member := range group.Members {
		if _, ok := o.cashMap[member]; !ok {
			o.pq.Push(member)
		}
		o.cashMap[member] += avg
	}
}

func (o *OPICScheduler) Poll() string {
	url := o.pq.Pop().(string)
	// Offer scheduler 对应的 UrlGroup 的时候再删除，因为还需要 scheduler 的 value
	//delete(o.pq.cashMap, scheduler)
	return url
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

// 分布式调度
