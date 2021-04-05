// 缓存：token->Items，lru
package db

import (
	"container/list"
	"sync"
)

type getFunc = func(interface{}) (interface{}, error)

// lru
type buffer struct {
	list    *list.List
	_map    map[interface{}]*list.Element
	maxSize int
	getFunc getFunc // 缓存不命中时，获取元素的函数
	lock    sync.Mutex
}

type node struct {
	key, value interface{}
}

func newBuffer(size int, getFunc getFunc) *buffer {
	return &buffer{
		list:    list.New(),
		_map:    make(map[interface{}]*list.Element, size),
		maxSize: size,
		getFunc: getFunc,
	}
}

func (b *buffer) get(key interface{}) (interface{}, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	e, ok := b._map[key]
	if !ok {
		v, err := b.getFunc(key)
		if err != nil {
			// todo log
			return nil, err
		}
		b._add(key, v)
		return v, nil
	}
	b.list.Remove(e)
	b.list.PushFront(e)
	return e.Value.(*node).value, nil
}

func (b *buffer) _add(key, value interface{}) {
	if b.list.Len() >= b.maxSize {
		e := b.list.Remove(b.list.Back())
		delete(b._map, e.(*node).key)
	}
	e := b.list.PushFront(&node{key, value})
	b._map[key] = e
}

//
//func (b *buffer) _del(key interface{}) {
//	e, ok := b._map[key]
//	if !ok {
//		return
//	}
//	b.list.Remove(e)
//	delete(b._map, key)
//}
