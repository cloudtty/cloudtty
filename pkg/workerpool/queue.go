package workpool

import (
	"reflect"
	"sync"
)

type Interface interface {
	Add(item interface{})
	Len() int
	Get() interface{}
	Remove(item interface{})
	Has(item interface{}) bool
	All() []interface{}
}

type Type struct {
	queue []t
	dirty set

	sync.RWMutex
}

func newQueue() *Type {
	return &Type{dirty: set{}}
}

type empty struct{}
type t interface{}
type set map[t]empty

func (s set) has(item t) bool {
	_, exists := s[item]
	return exists
}

func (s set) insert(item t) {
	s[item] = empty{}
}

func (s set) delete(item t) {
	delete(s, item)
}

func (q *Type) Add(item interface{}) {
	q.RWMutex.Lock()
	defer q.RWMutex.Unlock()

	if q.dirty.has(item) {
		return
	}

	q.dirty.insert(item)
	q.queue = append(q.queue, item)
}

func (q *Type) Len() int {
	q.RWMutex.RLock()
	defer q.RWMutex.RUnlock()
	return len(q.queue)
}

func (q *Type) Get() interface{} {
	q.RWMutex.RLock()
	defer q.RWMutex.RUnlock()

	if len(q.queue) == 0 {
		return nil
	}

	item := q.queue[0]
	q.queue[0] = nil
	q.queue = q.queue[1:]

	q.dirty.delete(item)

	return item
}

func (q *Type) Remove(item interface{}) {
	q.RWMutex.Lock()
	defer q.RWMutex.Unlock()

	if len(q.queue) == 0 {
		return
	}

	if !q.dirty.has(item) {
		return
	}

	for idx, obj := range q.queue {
		if reflect.DeepEqual(obj, item) {
			q.queue = append(q.queue[:idx], q.queue[idx+1:]...)
			break
		}
	}

	q.dirty.delete(item)
}

func (q *Type) Has(item interface{}) bool {
	q.RWMutex.RLock()
	defer q.RWMutex.RUnlock()

	return q.dirty.has(item)
}

func (q *Type) All() []interface{} {
	q.RWMutex.RLock()
	defer q.RWMutex.RUnlock()

	items := make([]interface{}, 0, q.Len())
	for _, item := range q.queue {
		items = append(items, item)
	}

	return items
}
