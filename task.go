package xxl

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// TaskFunc 任务执行函数
type TaskFunc func(cxt context.Context, task *Task) error

type taskHead struct {
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
			t.e.opts.log.Error("task panic", slog.Any("panic", err), slog.Int64("JobID", t.ID))
			callback(FailureCode, panicTask(t, err))
		}
	}()

	if err := runTask(t); err != nil {
		callback(FailureCode, failure(t, err))
		return
	}

	callback(SuccessCode, success(t))
}

func runTask(task *Task) error {
	defer func() {
		task.endTime = time.Now().UnixMilli()
		task.cancel()
	}()
	task.startTime = time.Now().UnixMilli()
	return task.fn(task.ext, task)
}

func panicTask(task *Task, a any) string {
	return fmt.Sprintf("Panic @ ID = %d, Name = %s, LogID = %d, cost = %d(ms), panic = %#v",
		task.ID, task.Name, task.Param.LogID,
		task.endTime-task.startTime, a)
}

func failure(task *Task, err error) string {
	return fmt.Sprintf("Failure @ ID = %d, Name = %s, LogID = %d, cost = %d(ms), error = %s",
		task.ID, task.Name, task.Param.LogID,
		task.endTime-task.startTime, err)
}

func success(task *Task) string {
	return fmt.Sprintf("Success @ ID = %d, Name = %s, LogID = %d, cost = %d(ms)",
		task.ID, task.Name, task.Param.LogID,
		task.endTime-task.startTime)
}
