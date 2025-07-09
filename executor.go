package xxl

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// NewExecutor 创建执行器
func NewExecutor(opts ...Option) *Executor {
	options := newOptions(opts...)
	e := &Executor{
		opts: options,
	}
	return e
}

type Executor struct {
	opts    Options
	address string
	regList *taskList[string, TaskHead] //注册任务列表
	runList *taskList[int64, *Task]     //正在执行任务列表

	doer Doer

	log         *slog.Logger
	logHandler  LogHandler   //日志查询handler
	middlewares []Middleware //中间件
	rootCtx     context.Context
}

func (e *Executor) Init(opts ...Option) {
	for _, o := range opts {
		o(&e.opts)
	}

	e.regList = newTaskHeadList()
	e.runList = newTaskList()

	e.log = e.opts.log
	e.rootCtx = e.opts.rootCtx
	e.address = e.opts.ExecutorIp + ":" + e.opts.ExecutorPort
	e.logHandler = e.opts.logHandler

	go e.registry()
}

func (e *Executor) Use(middlewares ...Middleware) {
	e.middlewares = middlewares
}

func (e *Executor) Stop() {
	e.registryRemove()
}

// RegTask 注册任务
func (e *Executor) RegTask(pattern string, task TaskFunc) {
	t := TaskHead{Name: pattern}
	t.fn = e.chain(task)
	e.regList.Set(pattern, t)
}

// 运行一个任务
func (e *Executor) runTask(w http.ResponseWriter, request *http.Request) {
	var param TriggerParam
	if err := BindAndClose(request, &param); err != nil {
		e.log.Error("参数解析错误", slog.Any("error", err))
		returnCall2(param, FailureCode, "params err", w)
		return
	}

	e.log.Info("任务参数", slog.Any("param", param))

	//阻塞策略处理
	if oldTask, ok := e.runList.Get(param.JobID); ok {
		if param.ExecutorBlockStrategy == coverEarly { //覆盖之前调度
			oldTask.cancel()
			e.runList.Del(oldTask.ID)
		} else { //单机串行,丢弃后续调度 都进行阻塞
			e.log.Error("任务已经在运行了", slog.Int64("JobID", param.JobID), slog.String("executorHandler", param.ExecutorHandler))
			returnCall2(param, FailureCode, "There are tasks running", w)
			return
		}
	}

	task := &Task{
		ID:    param.JobID,
		Name:  param.ExecutorHandler,
		excec: e,
		Param: param,
	}

	if th, ok := e.regList.Get(param.ExecutorHandler); ok {
		task.fn = th.fn
		if param.ExecutorTimeout > 0 {
			task.ext, task.cancel = context.WithTimeout(e.rootCtx, time.Duration(param.ExecutorTimeout)*time.Second)
		} else {
			task.ext, task.cancel = context.WithCancel(e.rootCtx)
		}
	} else {
		e.log.Error("任务没有注册", slog.Int64("JobID", param.JobID), slog.String("executorHandler", param.ExecutorHandler))
		returnCall2(param, FailureCode, "Task not registered", w)
		return
	}

	e.runList.Set(task.ID, task)
	go task.Run(func(code int, msg string) {
		e.callback(task, code, msg)
	})
	e.log.Info("任务开始执行", slog.Int64("JobID", param.JobID), slog.String("executorHandler", param.ExecutorHandler))
	returnCode(SuccessCode, w)
}

// 删除一个任务
func (e *Executor) killTask(w http.ResponseWriter, request *http.Request) {
	var param KillParam
	BindAndClose(request, &param)

	if task, ok := e.runList.LoadAndDel(param.JobID); ok {
		task.cancel()
		// returnCode(SuccessCode, w)
		JsonTo(http.StatusOK, ReturnSuccess, w)
		return
	}

	e.log.Error("任务没有运行", slog.Int64("JobID", param.JobID))
	// returnCode(FailureCode, w)
	JsonTo(http.StatusInternalServerError, ReturnFailure, w)
}

// 任务日志
func (e *Executor) taskLog(w http.ResponseWriter, r *http.Request) {
	var logParam LogParam
	if err := BindAndClose(r, &logParam); err != nil {
		e.log.Error("日志请求失败", slog.Any("error", err))
		//reqErrLogHandler(writer, req, err)
		JsonTo(http.StatusInternalServerError, reqErrLogHandler2(err), w)
		return
	}
	e.log.Info("日志请求参数", slog.Any("req", logParam))
	logResult := e.logHandler(logParam)
	JsonTo(http.StatusOK, logResult, w)
}

// 心跳检测
func (e *Executor) beat(writer http.ResponseWriter, _ *http.Request) {
	e.log.Info("心跳检测")
	returnCode(SuccessCode, writer)
}

// 忙碌检测
func (e *Executor) idleBeat(writer http.ResponseWriter, request *http.Request) {
	var param IdleBeatParam
	if err := BindAndClose(request, &param); err != nil {
		e.log.Error("参数解析错误", slog.Any("error", err))
		returnCode(FailureCode, writer)
		return
	}
	e.log.Info("忙碌检测任务参数", slog.Any("param", param))

	if _, ok := e.runList.Get(param.JobID); ok {
		returnCode(FailureCode, writer)
		e.log.Error("idleBeat任务正在运行", slog.Int64("JobID", param.JobID))
		return
	}
	returnCode(SuccessCode, writer)
}

// 注册执行器到调度中心
func (e *Executor) registry() {

	t := time.NewTimer(time.Second * 0) //初始立即执行
	defer t.Stop()
	regParam := &RegistryParam{
		RegistryGroup: "EXECUTOR",
		RegistryKey:   e.opts.RegistryKey,
		RegistryValue: "http://" + e.address,
	}

	for {
		<-t.C
		t.Reset(20 * time.Second) //20秒心跳防止过期

		{
			resp, err := e.post("/api/registry", regParam)
			if err != nil {
				e.log.Error("执行器注册失败1", slog.Any("error", err))
				return
			}
			defer resp.Body.Close()

			var r Return[string]
			if err := Bind(resp.Body, &r); err != nil {
				e.log.Error("执行器注册失败2", slog.Any("error", err))
				return
			}

			if r.Code != SuccessCode {
				e.log.Error("执行器注册失败3", slog.Any("body", r))
				return
			}
			e.log.Info("执行器注册成功", slog.Any("body", r))
		}
	}
}

// 执行器注册摘除
func (e *Executor) registryRemove() {
	regParam := &RegistryParam{
		RegistryGroup: "EXECUTOR",
		RegistryKey:   e.opts.RegistryKey,
		RegistryValue: "http://" + e.address,
	}
	resp, err := e.post("/api/registryRemove", regParam)
	if err != nil {
		e.log.Error("执行器摘除失败1", slog.Any("error", err))
		return
	}
	defer resp.Body.Close()

	var r Return[string]
	if err := Bind(resp.Body, &r); err != nil {
		e.log.Error("执行器摘除失败2", slog.Any("error", err))
		return
	}

	if r.Code != SuccessCode {
		e.log.Error("执行器摘除失败3", slog.Any("body", r))
		return
	}
	e.log.Info("执行器摘除成功", slog.Any("body", r))
}

// 回调任务列表
func (e *Executor) callback(task *Task, code int, msg string) {
	e.runList.Del(task.ID)
	r, err := e.post("/api/callback", CallbackParamList{newCallback(task, code, msg)})
	if err != nil {
		e.log.Error("callback error", slog.Any("error", err))
		return
	}
	defer r.Body.Close()
	var rr Return[string]
	if err := Bind(r.Body, &rr); err != nil {
		e.log.Error("callback ReadBody error", slog.Any("error", err))
		return
	}
	e.log.Info("任务回调成功", slog.Any("body", rr))
}

func (e *Executor) post2(action string, body any) (rtn Return[string], err error) {
	ry := resty.New().SetBaseURL(e.opts.ServerAddr)

	_, err = ry.R().
		SetBody(body).
		SetResult(&rtn).
		ForceContentType("application/json;charset=UTF-8").
		SetHeader("XXL-JOB-ACCESS-TOKEN", e.opts.AccessToken).
		Post(action)

	return
}

func (e *Executor) post(action string, body any) (*http.Response, error) {
	var bs bytes.Buffer
	bs.Grow(512)
	enc := json.NewEncoder(&bs)
	if err := enc.Encode(body); err != nil {
		return nil, err
	}

	request, err := http.NewRequest(http.MethodPost, e.opts.ServerAddr+action, &bs)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json;charset=UTF-8")
	request.Header.Set("XXL-JOB-ACCESS-TOKEN", e.opts.AccessToken)

	return e.doer.Do(request)
}

// RunTask 运行任务
func (e *Executor) RunTask(writer http.ResponseWriter, request *http.Request) {
	e.runTask(writer, request)
}

// KillTask 删除任务
func (e *Executor) KillTask(writer http.ResponseWriter, request *http.Request) {
	e.killTask(writer, request)
}

// TaskLog 任务日志
func (e *Executor) TaskLog(writer http.ResponseWriter, request *http.Request) {
	e.taskLog(writer, request)
}

// Beat 心跳检测
func (e *Executor) Beat(writer http.ResponseWriter, request *http.Request) {
	e.beat(writer, request)
}

// IdleBeat 忙碌检测
func (e *Executor) IdleBeat(writer http.ResponseWriter, request *http.Request) {
	e.idleBeat(writer, request)
}

func (e *Executor) Handle() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /run", e.RunTask)
	mux.HandleFunc("POST /kill", e.KillTask)
	mux.HandleFunc("POST /log", e.TaskLog)
	mux.HandleFunc("POST /beat", e.Beat)
	mux.HandleFunc("POST /idleBeat", e.IdleBeat)

	return mux
}
