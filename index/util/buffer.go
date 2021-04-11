// 缓存：token->Items，lru
package util

import (
	"container/list"
	"log"
	"sync"
)

type getFunc = func(interface{}) (interface{}, error)

// lru
type Buffer struct {
	list    *list.List
	_map    map[interface{}]*list.Element
	maxSize int
	getFunc getFunc // 缓存不命中时，获取数据的函数
	lock    sync.Mutex
}

type node struct {
	key, value interface{}
}

func NewBuffer(size int, getFunc getFunc) *Buffer {
	return &Buffer{
		list:    list.New(),
		_map:    make(map[interface{}]*list.Element, size),
		maxSize: size,
		getFunc: getFunc,
	}
}

func (b *Buffer) Get(key interface{}) (interface{}, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	e, ok := b._map[key]
	if !ok {
		v, err := b.getFunc(key)
		if err != nil {
			log.Println(err.Error())
			return nil, err
		}
		b._add(key, v)
		return v, nil
	}
	b.list.Remove(e)
	b.list.PushFront(e)
	return e.Value.(*node).value, nil
}

func (b *Buffer) _add(key, value interface{}) {
	if b.list.Len() >= b.maxSize {
		e := b.list.Remove(b.list.Back())
		delete(b._map, e.(*node).key)
	}
	e := b.list.PushFront(&node{key, value})
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
