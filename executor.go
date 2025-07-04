package xxl

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Executor 执行器
type Executor interface {
	// Init 初始化
	Init(...Option)
	// Use 使用中间件
	Use(middlewares ...Middleware)
	// RegTask 注册任务
	RegTask(pattern string, task TaskFunc)
	// RegJob 注册Job
	RegJob(pattern string, job Job)
	// RunTask 运行任务
	RunTask(writer http.ResponseWriter, request *http.Request)
	// KillTask 杀死任务
	KillTask(writer http.ResponseWriter, request *http.Request)
	// TaskLog 任务日志
	TaskLog(writer http.ResponseWriter, request *http.Request)
	// Beat 心跳检测
	Beat(writer http.ResponseWriter, request *http.Request)
	// IdleBeat 忙碌检测
	IdleBeat(writer http.ResponseWriter, request *http.Request)
	// Stop 停止服务
	Stop()
}

// NewExecutor 创建执行器
func NewExecutor(opts ...Option) Executor {
	return newExecutor(opts...)
}

func newExecutor(opts ...Option) *executor {
	options := newOptions(opts...)
	e := &executor{
		opts: options,
	}
	return e
}

type executor struct {
	opts    Options
	address string
	regList *taskList //注册任务列表
	runList *taskList //正在执行任务列表
	mu      sync.RWMutex

	log         *slog.Logger
	logHandler  LogHandler   //日志查询handler
	middlewares []Middleware //中间件
	rootCtx     context.Context
}

func (e *executor) Init(opts ...Option) {
	for _, o := range opts {
		o(&e.opts)
	}

	e.regList = &taskList{
		data: make(map[string]*Task),
	}
	e.runList = &taskList{
		data: make(map[string]*Task),
	}

	e.log = e.opts.log
	e.rootCtx = e.opts.rootCtx
	e.address = e.opts.ExecutorIp + ":" + e.opts.ExecutorPort
	e.logHandler = e.opts.logHandler

	go e.registry()
}

func (e *executor) Use(middlewares ...Middleware) {
	e.middlewares = middlewares
}

func (e *executor) Stop() {
	e.registryRemove()
}

// RegTask 注册任务
func (e *executor) RegTask(pattern string, task TaskFunc) {
	var t = &Task{}
	t.fn = e.chain(task)
	e.regList.Set(pattern, t)
}

// RegJob 注册Job
func (e *executor) RegJob(pattern string, job Job) {
	e.RegTask(pattern, JobFunc(job))
}

// 运行一个任务
func (e *executor) runTask(writer http.ResponseWriter, request *http.Request) {
	e.mu.Lock()
	defer e.mu.Unlock()

	req, _ := io.ReadAll(request.Body)
	param := &RunReq{}
	err := json.Unmarshal(req, &param)
	if err != nil {
		_, _ = writer.Write(returnCall(param, FailureCode, "params err"))
		e.log.Error("参数解析错误", slog.Any("error", err), slog.String("raw", string(req)))
		return
	}
	e.log.Info("任务参数", slog.Any("param", param))
	if !e.regList.Exists(param.ExecutorHandler) {
		_, _ = writer.Write(returnCall(param, FailureCode, "Task not registered"))
		//e.log.Error("任务[" + Int64ToStr(param.JobID) + "]没有注册:" + param.ExecutorHandler)
		e.log.Error("任务没有注册", slog.Int64("JobID", param.JobID), slog.String("executorHandler", param.ExecutorHandler))
		return
	}

	//阻塞策略处理
	if e.runList.Exists(Int64ToStr(param.JobID)) {
		if param.ExecutorBlockStrategy == coverEarly { //覆盖之前调度
			oldTask := e.runList.Get(Int64ToStr(param.JobID))
			if oldTask != nil {
				oldTask.Cancel()
				e.runList.Del(Int64ToStr(oldTask.Id))
			}
		} else { //单机串行,丢弃后续调度 都进行阻塞
			_, _ = writer.Write(returnCall(param, FailureCode, "There are tasks running"))
			e.log.Error("任务已经在运行了", slog.Int64("JobID", param.JobID), slog.String("executorHandler", param.ExecutorHandler))
			return
		}
	}

	task := e.regList.Get(param.ExecutorHandler)
	if param.ExecutorTimeout > 0 {
		task.Ext, task.Cancel = context.WithTimeout(e.rootCtx, time.Duration(param.ExecutorTimeout)*time.Second)
	} else {
		task.Ext, task.Cancel = context.WithCancel(e.rootCtx)
	}
	task.Id = param.JobID
	task.Name = param.ExecutorHandler
	task.Param = param
	task.log = e.log.With(slog.Int64("JobID", param.JobID))

	e.runList.Set(Int64ToStr(task.Id), task)
	go task.Run(func(code int64, msg string) {
		e.callback(task, code, msg)
	})
	e.log.Info("任务开始执行", slog.Int64("JobID", param.JobID), slog.String("executorHandler", param.ExecutorHandler))
	_, _ = writer.Write(returnGeneral())
}

// 删除一个任务
func (e *executor) killTask(writer http.ResponseWriter, request *http.Request) {
	e.mu.Lock()
	defer e.mu.Unlock()
	req, _ := io.ReadAll(request.Body)
	param := &killReq{}
	_ = json.Unmarshal(req, &param)
	if !e.runList.Exists(Int64ToStr(param.JobID)) {
		_, _ = writer.Write(returnKill(param, FailureCode))
		e.log.Error("任务没有运行", slog.Int64("JobID", param.JobID))
		return
	}
	task := e.runList.Get(Int64ToStr(param.JobID))
	task.Cancel()
	e.runList.Del(Int64ToStr(param.JobID))
	_, _ = writer.Write(returnGeneral())
}

// 任务日志
func (e *executor) taskLog(writer http.ResponseWriter, request *http.Request) {
	data, err := io.ReadAll(request.Body)
	req := &LogReq{}
	if err != nil {
		e.log.Error("日志请求失败", slog.Any("error", err))
		reqErrLogHandler(writer, req, err)
		return
	}
	err = json.Unmarshal(data, &req)
	if err != nil {
		e.log.Error("日志请求解析失败", slog.Any("error", err))
		reqErrLogHandler(writer, req, err)
		return
	}
	e.log.Info("日志请求参数", slog.Any("req", req))
	res := e.logHandler(req)
	str, _ := json.Marshal(res)
	_, _ = writer.Write(str)
}

// 心跳检测
func (e *executor) beat(writer http.ResponseWriter, _ *http.Request) {
	e.log.Info("心跳检测")
	_, _ = writer.Write(returnGeneral())
}

// 忙碌检测
func (e *executor) idleBeat(writer http.ResponseWriter, request *http.Request) {
	e.mu.Lock()
	defer e.mu.Unlock()
	defer request.Body.Close()
	req, _ := io.ReadAll(request.Body)
	param := &idleBeatReq{}
	err := json.Unmarshal(req, &param)
	if err != nil {
		e.log.Error("参数解析错误", slog.Any("error", err), slog.String("body", string(req)))
		_, _ = writer.Write(returnIdleBeat(FailureCode))
		return
	}
	if e.runList.Exists(Int64ToStr(param.JobID)) {
		_, _ = writer.Write(returnIdleBeat(FailureCode))
		e.log.Error("idleBeat任务正在运行", slog.Int64("JobID", param.JobID))
		return
	}
	e.log.Info("忙碌检测任务参数", slog.Any("param", param))
	_, _ = writer.Write(returnGeneral())
}

// 注册执行器到调度中心
func (e *executor) registry() {

	t := time.NewTimer(time.Second * 0) //初始立即执行
	defer t.Stop()
	req := &Registry{
		RegistryGroup: "EXECUTOR",
		RegistryKey:   e.opts.RegistryKey,
		RegistryValue: "http://" + e.address,
	}
	param, err := json.Marshal(req)
	if err != nil {
		e.log.Error("执行器注册信息解析失败", slog.Any("error", err))
		return
	}

	for {
		<-t.C
		t.Reset(20 * time.Second) //20秒心跳防止过期
		func() {
			result, err := e.post("/api/registry", string(param))
			if err != nil {
				e.log.Error("执行器注册失败1", slog.Any("error", err))
				return
			}
			defer result.Body.Close()
			body, err := io.ReadAll(result.Body)
			if err != nil {
				e.log.Error("执行器注册失败2", slog.Any("error", err))
				return
			}
			res := &res{}
			_ = json.Unmarshal(body, &res)
			if res.Code != SuccessCode {
				e.log.Error("执行器注册失败3", slog.String("body", string(body)))
				return
			}
			e.log.Info("执行器注册成功", slog.String("body", string(body)))
		}()

	}
}

// 执行器注册摘除
func (e *executor) registryRemove() {
	t := time.NewTimer(time.Second * 0) //初始立即执行
	defer t.Stop()
	req := &Registry{
		RegistryGroup: "EXECUTOR",
		RegistryKey:   e.opts.RegistryKey,
		RegistryValue: "http://" + e.address,
	}
	param, err := json.Marshal(req)
	if err != nil {
		e.log.Error("执行器摘除失败", slog.Any("error", err))
		return
	}
	res, err := e.post("/api/registryRemove", string(param))
	if err != nil {
		e.log.Error("执行器摘除失败", slog.Any("error", err))
		return
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	e.log.Info("执行器摘除成功", slog.String("body", string(body)))
}

// 回调任务列表
func (e *executor) callback(task *Task, code int64, msg string) {
	e.runList.Del(Int64ToStr(task.Id))
	res, err := e.post("/api/callback", string(returnCall(task.Param, code, msg)))
	if err != nil {
		e.log.Error("callback error", slog.Any("error", err))
		return
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		e.log.Error("callback ReadBody error", slog.Any("error", err))
		return
	}
	e.log.Info("任务回调成功", slog.String("body", string(body)))
}

// post
func (e *executor) post(action, body string) (resp *http.Response, err error) {
	request, err := http.NewRequest("POST", e.opts.ServerAddr+action, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json;charset=UTF-8")
	request.Header.Set("XXL-JOB-ACCESS-TOKEN", e.opts.AccessToken)
	client := http.Client{
		Timeout: e.opts.Timeout,
	}
	return client.Do(request)
}

// RunTask 运行任务
func (e *executor) RunTask(writer http.ResponseWriter, request *http.Request) {
	e.runTask(writer, request)
}

// KillTask 删除任务
func (e *executor) KillTask(writer http.ResponseWriter, request *http.Request) {
	e.killTask(writer, request)
}

// TaskLog 任务日志
func (e *executor) TaskLog(writer http.ResponseWriter, request *http.Request) {
	e.taskLog(writer, request)
}

// Beat 心跳检测
func (e *executor) Beat(writer http.ResponseWriter, request *http.Request) {
	e.beat(writer, request)
}

// IdleBeat 忙碌检测
func (e *executor) IdleBeat(writer http.ResponseWriter, request *http.Request) {
	e.idleBeat(writer, request)
}
