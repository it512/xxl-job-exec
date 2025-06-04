package xxl

import (
	"context"
	"fmt"
	"runtime/debug"
)

type JobResult struct {
	msg string
}

func NewJobResult(msg string) *JobResult {
	return &JobResult{msg: msg}
}

func (jr JobResult) Sting() string {
	return jr.msg
}

var JobOK = NewJobResult("OK")

type Job interface {
	Run(context.Context, *RunReq) (fmt.Stringer, error)
}

func JobFunc(job Job) TaskFunc {
	return func(ctx context.Context, param *RunReq) (fmt.Stringer, error) {
		return job.Run(ctx, param)
	}
}

// TaskFunc 任务执行函数
type TaskFunc func(cxt context.Context, param *RunReq) (fmt.Stringer, error)

// Task 任务
type Task struct {
	Id        int64
	Name      string
	Ext       context.Context
	Param     *RunReq
	fn        TaskFunc
	Cancel    context.CancelFunc
	StartTime int64
	EndTime   int64
	//日志
	log Logger
}

// Run 运行任务
func (t *Task) Run(callback func(code int64, msg string)) {
	defer func(cancel func()) {
		if err := recover(); err != nil {
			t.log.Info(t.Info()+" panic: %v", err)
			debug.PrintStack() //堆栈跟踪
			callback(FailureCode, fmt.Sprintf("task panic:%v", err))
			cancel()
		}
	}(t.Cancel)

	msger, err := t.fn(t.Ext, t.Param)
	if err != nil {
		callback(FailureCode, err.Error())
	}
	callback(SuccessCode, msger.String())
}

// Info 任务信息
func (t *Task) Info() string {
	return fmt.Sprintf("任务ID[%d]任务名称[%s]参数:%s", t.Id, t.Name, t.Param.ExecutorParams)
}
