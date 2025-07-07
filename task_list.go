package xxl

import "sync"

type taskList[K comparable, V any] struct {
	data sync.Map
}

func newTaskHeadList() *taskList[string, TaskHead] {
	return &taskList[string, TaskHead]{}
}

func newTaskList() *taskList[int64, Task] {
	return &taskList[int64, Task]{}
}

func (t *taskList[K, V]) Set(key K, val V) {
	t.data.Store(key, val)
}

func (t *taskList[K, V]) Get(key K) (v V, ok bool) {
	var a any
	a, ok = t.data.Load(key)
	if ok {
		v = a.(V)
	}
	return
}

func (t *taskList[K, V]) Del(key K) {
	t.data.Delete(key)
}

func (t *taskList[K, V]) LoadAndDel(key K) (v V, ok bool) {
	var a any
	a, ok = t.data.LoadAndDelete(key)
	if ok {
		v = a.(V)
	}
	return
}
