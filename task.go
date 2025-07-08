package xxl

import (
	"context"
	"fmt"
	"log/slog"
)

// TaskFunc 任务执行函数
type TaskFunc func(cxt context.Context, task *Task) (fmt.Stringer, error)

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

	excec *Executor
}

func (t *Task) Run(callback func(code int, msg string)) {
	defer func(cancel func()) {
		if err := recover(); err != nil {
			t.excec.log.Error("error", slog.Any("error", err))
			callback(FailureCode, fmt.Sprintf("task panic:%v", err))
			cancel()
		}
	}(t.cancel)

	msger, err := t.fn(t.ext, t)
	if err != nil {
		callback(FailureCode, err.Error())
		return
	}
	callback(SuccessCode, msger.String())
}
