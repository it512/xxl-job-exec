package xxl

import "sync"

// 任务列表 [JobID]执行函数,并行执行时[+LogID]
type taskList2 struct {
	data sync.Map
}

func (t *taskList2) Set(key any, val TaskHandle) {
	t.data.Store(key, val)
}

func (t *taskList2) Get(key any) (TaskHandle, bool) {
	task, ok := t.data.Load(key)
	if !ok {
		return TaskHandle{}, false
	}
	return task.(TaskHandle), true
}

func (t *taskList2) Del(key any) {
	t.data.Delete(key)
}

type taskList3 struct {
	data sync.Map
}

func (t *taskList3) Set(key any, val Task) {
	t.data.Store(key, val)
}

func (t *taskList3) Get(key any) (Task, bool) {
	task, ok := t.data.Load(key)
	if !ok {
		return Task{}, false
	}
	return task.(Task), true
}

func (t *taskList3) Del(key any) {
	t.data.Delete(key)
}

func (t *taskList3) LoadAndDel(key any) (Task, bool) {
	task, ok := t.data.LoadAndDelete(key)
	if ok {
		return task.(Task), ok
	}
	return Task{}, false
}
