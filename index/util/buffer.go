// 缓存：token->Items，lru
package util

import (
	"container/list"
	"sync"
	"time"
)

type getFunc = func(interface{}) interface{}

// lru
type Buffer struct {
	list            *list.List
	_map            map[interface{}]*list.Element
	maxSize         int
	validityPeriod  int64   // 永不失效使用：math.MaxInt64
	totalCountOfGet int     // Get 的总共次数
	hitCount        int     // Get 命中次数
	getFunc         getFunc // 缓存不命中时，获取数据的函数
	lock            sync.Mutex
}

type node struct {
	key, value interface{}
	birthday   int64 // 创建这个 node 时的时间戳，避免高频 token 一直存在 Buffer 中，而得不到刷新
}

func NewBuffer(size int, validityPeriod int64, getFunc getFunc) *Buffer {
	return &Buffer{
		list:           list.New(),
		_map:           make(map[interface{}]*list.Element, size),
		maxSize:        size,
		validityPeriod: validityPeriod,
		getFunc:        getFunc,
	}
}

func (b *Buffer) Get(key interface{}) interface{} {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.totalCountOfGet++
	e, ok := b._map[key]
	if !ok || time.Now().Unix()-e.Value.(*node).birthday > b.validityPeriod {
		v := b.getFunc(key)
		b._add(key, v)
		return v
	}
	b.hitCount++
	b.list.Remove(e)
	b.list.PushFront(e)
	return e.Value.(*node).value
}

func (b *Buffer) GetHitRate() float64 {
	return float64(b.hitCount) / float64(b.totalCountOfGet)
}

func (b *Buffer) _add(key, value interface{}) {
	if e, ok := b._map[key]; ok {
		b.list.Remove(e)
		delete(b._map, key)
	}
	if b.list.Len() >= b.maxSize {
		back := b.list.Back()
		b.list.Remove(back)
		delete(b._map, back.Value.(*node).key)
	}
	e := b.list.PushFront(&node{
		key:      key,
		value:    value,
		birthday: time.Now().Unix(),
	})
	b._map[key] = e
}

func (b *Buffer) Del(key interface{}) {
	b.lock.Lock()
	defer b.lock.Unlock()
	e, ok := b._map[key]
	if !ok {
		return
	}
	b.list.Remove(e)
	delete(b._map, key)
}
