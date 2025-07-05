package xxl

import "sync"

type taskList2 struct {
	data sync.Map
}

func (t *taskList2) Set(key any, val Task) {
	t.data.Store(key, val)
}
func (t *taskList2) Get(key any) (Task, bool) {
	task, ok := t.data.Load(key)
	if !ok {
		return Task{}, false
	}
	return task.(Task), true
}

func (t *taskList2) Del(key any) {
	t.data.Delete(key)
}

func (t *taskList2) Exists(key any) bool {
	_, ok := t.Get(key)
	return ok
}

// 任务列表 [JobID]执行函数,并行执行时[+LogID]
type taskList struct {
	mu   sync.RWMutex
	data map[string]*Task
}

// Set 设置数据
func (t *taskList) Set(key string, val *Task) {
	t.mu.Lock()
	t.data[key] = val
	t.mu.Unlock()
}

// Get 获取数据
func (t *taskList) Get(key string) *Task {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.data[key]
}

// Del 设置数据
func (t *taskList) Del(key string) {
	t.mu.Lock()
	delete(t.data, key)
	t.mu.Unlock()
}

// Exists Key是否存在
func (t *taskList) Exists(key string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, ok := t.data[key]
	return ok
}
