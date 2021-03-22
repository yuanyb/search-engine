package scheduler

import (
	"container/heap"
	"container/list"
)

// Strategy 表示爬虫的抓取 URL 的优先级策略
type Scheduler interface {
	Offer(group Group)
	Poll() string
	addSeedUrls([]string)
}

// Breath first
type BFScheduler struct {
	queue *list.List
}

func (b *BFScheduler) Poll() string {
	url := b.queue.Remove(b.queue.Front()).(string)
	return url
}

func (b *BFScheduler) Offer(group Group) {
	for _, url := range group.members {
		b.queue.PushBack(url)
	}
}

func (b *BFScheduler) addSeedUrls(seedUrls []string) {
	for _, seedUrl := range seedUrls {
		b.queue.PushBack(seedUrl)
	}
}

func NewBFScheduler(seedUrls []string) Scheduler {
	scheduler := &BFScheduler{
		queue: list.New(),
	}
	scheduler.addSeedUrls(seedUrls)
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

func (o *OPICScheduler) Offer(group Group) {
	avg := o.cashMap[group.leader] / float32(len(group.members))
	delete(o.cashMap, group.leader)
	for _, member := range group.members {
		if _, ok := o.cashMap[member]; !ok {
			o.pq.Push(member)
		}
		o.cashMap[member] += avg
	}
}

func (o *OPICScheduler) Poll() string {
	url := o.pq.Pop().(string)
	// Offer scheduler 对应的 Group 的时候再删除，因为还需要 scheduler 的 value
	//delete(o.pq.cashMap, scheduler)
	return url
}

func (o *OPICScheduler) addSeedUrls(seedUrls []string) {
	for _, seedUrl := range seedUrls {
		o.cashMap[seedUrl] = 1.0
		o.pq.Push(seedUrl)
	}
}

func NewOPICScheduler(seedUrls []string) Scheduler {
	opic := &OPICScheduler{
		cashMap: make(map[string]float32, 10000),
		pq: priorityQueue{
			array: make([]string, 10000),
		},
	}
	opic.pq.opic = opic
	opic.addSeedUrls(seedUrls)
	heap.Init(&opic.pq)
	return opic
}

// 分布式调度
