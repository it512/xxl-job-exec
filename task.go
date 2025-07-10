package xxl

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// TaskFunc 任务执行函数
type TaskFunc func(cxt context.Context, task *Task) error

type TaskHead struct {
	Name string
	fn   TaskFunc
}

type Task struct {
	ID    int64
	Name  string
	Param TriggerParam

	startTime int64
	endTime   int64
	cancel    context.CancelFunc
	ext       context.Context
	fn        TaskFunc

	e *Executor
}

func (t *Task) run(callback func(code int, msg string)) {
	defer func() {
		if err := recover(); err != nil {
			t.e.opts.log.Error("error", slog.Any("error", err))
			callback(FailureCode, panicTask(t, err))
		}
	}()

	if err := runTask(t); err != nil {
		callback(FailureCode, failure(t, err))
		return
	}
	callback(SuccessCode, success(t))
}

func panicTask(task *Task, a any) string {
	return fmt.Sprintf("Panic @ ID = %d, Name = %s, LogID = %d, cost = %d(ms), error = %#v",
		task.ID, task.Name, task.Param.LogID,
		task.endTime-task.startTime, a)
}

func failure(task *Task, err error) string {
	return fmt.Sprintf("Failure @ ID = %d, Name = %s, LogID = %d, cost = %d(ms), error = %#v",
		task.ID, task.Name, task.Param.LogID,
		task.endTime-task.startTime, err)
}

func success(task *Task) string {
	return fmt.Sprintf("Success @ ID = %d, Name = %s, LogID = %d, cost = %d(ms)",
		task.ID, task.Name, task.Param.LogID,
		task.endTime-task.startTime)
}

func runTask(task *Task) error {
	defer task.cancel()
	task.startTime = time.Now().UnixMilli()
	err := task.fn(task.ext, task)
	task.endTime = time.Now().UnixMilli()
	return err
}
