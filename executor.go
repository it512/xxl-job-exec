package xxl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

type Executor struct {
	opts    Options
	regList *taskList[string, TaskHead] //注册任务列表
	runList *taskList[int64, *Task]     //正在执行任务列表

	middlewares []Middleware //中间件
}

// NewExecutor 创建执行器
func NewExecutor(opts ...Option) *Executor {
	opt := Options{
		RegistryKey: DefaultRegistryKey,

		log:        slog.Default(),
		rootCtx:    context.Background(),
		logHandler: defaultLogHandler,
		client:     http.DefaultClient,
	}

	for _, o := range opts {
		o(&opt)
	}

	return &Executor{
		opts:    opt,
		regList: newTaskHeadList(),
		runList: newTaskList(),
	}

}

func (e *Executor) Start() {
	go e.registry()
}

func (e *Executor) Use(middlewares ...Middleware) {
	e.middlewares = middlewares
}

func (e *Executor) Stop() error {
	return e.registryRemove()
}

// RegTask 注册任务
func (e *Executor) RegTask(pattern string, task TaskFunc) {
	t := TaskHead{Name: pattern}
	t.fn = e.chain(task)
	e.regList.Set(pattern, t)
}

// 运行一个任务
func (e *Executor) runTask(w http.ResponseWriter, r *http.Request) {
	var param TriggerParam
	if err := Bind(r.Body, &param); err != nil {
		e.opts.log.Error("参数解析错误", slog.Any("error", err))
		JsonTo(http.StatusInternalServerError, CallbackParamList{newCallback(param, FailureCode, "params err")}, w)
		return
	}
	defer r.Body.Close()

	e.opts.log.Info("任务参数", slog.Any("param", param))

	//阻塞策略处理
	if oldTask, ok := e.runList.Get(param.JobID); ok {
		if param.ExecutorBlockStrategy == coverEarly { //覆盖之前调度
			oldTask.cancel()
			e.runList.Del(oldTask.ID)
		} else { //单机串行,丢弃后续调度 都进行阻塞
			e.opts.log.Error("任务已经在运行了", slog.Int64("JobID", param.JobID), slog.String("executorHandler", param.ExecutorHandler))
			JsonTo(http.StatusOK, CallbackParamList{newCallback(param, FailureCode, "tasks already running")}, w)
			return
		}
	}

	task := &Task{
		ID:    param.JobID,
		Name:  param.ExecutorHandler,
		Param: param,

		e: e,
	}

	if th, ok := e.regList.Get(param.ExecutorHandler); ok {
		task.fn = th.fn
		if param.ExecutorTimeout > 0 {
			task.ext, task.cancel = context.WithTimeout(e.opts.rootCtx, time.Duration(param.ExecutorTimeout)*time.Second)
		} else {
			task.ext, task.cancel = context.WithCancel(e.opts.rootCtx)
		}
	} else {
		e.opts.log.Error("任务没有注册", slog.Int64("JobID", param.JobID), slog.String("executorHandler", param.ExecutorHandler))
		JsonTo(http.StatusInternalServerError, CallbackParamList{newCallback(param, FailureCode, "task not registred")}, w)
		return
	}

	e.runList.Set(task.ID, task)
	go task.run(func(code int, msg string) {
		e.callback(task, code, msg)
	})
	e.opts.log.Info("任务开始执行", slog.Int64("JobID", param.JobID), slog.String("executorHandler", param.ExecutorHandler))
	JsonTo(http.StatusOK, ReturnSuccess, w)
}

// 删除一个任务
func (e *Executor) killTask(w http.ResponseWriter, r *http.Request) {
	var param KillParam
	if err := Bind(r.Body, &param); err != nil {
		JsonTo(http.StatusInternalServerError, ReturnFailure, w)
		return
	}
	defer r.Body.Close()

	if task, ok := e.runList.LoadAndDel(param.JobID); ok {
		task.cancel()
		JsonTo(http.StatusOK, ReturnSuccess, w)
		return
	}

	e.opts.log.Error("任务没有运行", slog.Int64("JobID", param.JobID))
	JsonTo(http.StatusOK, ReturnSuccess, w) // 注意这里返回Sucess
}

// 任务日志
func (e *Executor) taskLog(w http.ResponseWriter, r *http.Request) {
	var logParam LogParam
	if err := Bind(r.Body, &logParam); err != nil {
		e.opts.log.Error("日志请求失败", slog.Any("error", err))
		JsonTo(http.StatusInternalServerError, reqErrLogHandler(err), w)
		return
	}
	defer r.Body.Close()

	e.opts.log.Info("日志请求参数", slog.Any("req", logParam))
	logResult := e.opts.logHandler(logParam)
	JsonTo(http.StatusOK, logResult, w)
}

// 心跳检测
func (e *Executor) beat(w http.ResponseWriter, _ *http.Request) {
	e.opts.log.Info("心跳检测")
	JsonTo(http.StatusOK, ReturnSuccess, w)
}

// 忙碌检测
func (e *Executor) idleBeat(w http.ResponseWriter, r *http.Request) {
	var param IdleBeatParam
	if err := Bind(r.Body, &param); err != nil {
		e.opts.log.Error("参数解析错误", slog.Any("error", err))
		JsonTo(http.StatusInternalServerError, ReturnFailure, w)
		return
	}
	defer r.Body.Close()

	e.opts.log.Info("忙碌检测任务参数", slog.Any("param", param))

	if _, ok := e.runList.Get(param.JobID); ok {
		e.opts.log.Error("idleBeat任务正在运行", slog.Int64("JobID", param.JobID))
		JsonTo(http.StatusInternalServerError, ReturnFailure, w)
		return
	}
	JsonTo(http.StatusOK, ReturnSuccess, w)
}

// 注册执行器到调度中心
func (e *Executor) registry() {
	t := time.NewTimer(time.Second * 0) //初始立即执行
	defer t.Stop()

	regParam := &RegistryParam{
		RegistryGroup: "EXECUTOR",
		RegistryKey:   e.opts.RegistryKey,
		RegistryValue: e.opts.ExecutorURL,
	}

	for {
		<-t.C
		t.Reset(20 * time.Second) //20秒心跳防止过期

		func() {
			resp, err := e.post("/api/registry", regParam)
			if err != nil {
				e.opts.log.Error("执行器注册失败1", slog.Any("error", err))
				return
			}
			defer resp.Body.Close()

			var r Return[string]
			if err := Bind(resp.Body, &r); err != nil {
				e.opts.log.Error("执行器注册失败2", slog.Any("error", err))
				return
			}

			if r.Code != SuccessCode {
				e.opts.log.Error("执行器注册失败3", slog.Any("body", r))
				return
			}
			e.opts.log.Info("执行器注册成功", slog.Any("body", r))
		}()
	}
}

// 执行器注册摘除
func (e *Executor) registryRemove() error {
	regParam := &RegistryParam{
		RegistryGroup: "EXECUTOR",
		RegistryKey:   e.opts.RegistryKey,
		RegistryValue: e.opts.ExecutorURL,
	}
	resp, err := e.post("/api/registryRemove", regParam)
	if err != nil {
		e.opts.log.Error("执行器摘除失败1", slog.Any("error", err))
		return err
	}
	defer resp.Body.Close()

	var r Return[string]
	if err := Bind(resp.Body, &r); err != nil {
		e.opts.log.Error("执行器摘除失败2", slog.Any("error", err))
		return err
	}

	if r.Code != SuccessCode {
		e.opts.log.Error("执行器摘除失败3", slog.Any("body", r))
		return fmt.Errorf("error code = %d", r.Code)
	}
	e.opts.log.Info("执行器摘除成功", slog.Any("body", r))
	return nil
}

// 回调任务列表
func (e *Executor) callback(task *Task, code int, msg string) {
	e.runList.Del(task.ID)
	resp, err := e.post("/api/callback", CallbackParamList{newCallback(task.Param, code, msg)})
	if err != nil {
		e.opts.log.Error("callback error", slog.Any("error", err))
		return
	}
	defer resp.Body.Close()
	var r Return[string]
	if err := Bind(resp.Body, &r); err != nil {
		e.opts.log.Error("callback ReadBody error", slog.Any("error", err))
		return
	}
	e.opts.log.Info("任务回调成功", slog.Any("body", r))
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

	return e.opts.client.Do(request)
}

func (e *Executor) Handle() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /run", e.runTask)
	mux.HandleFunc("POST /kill", e.killTask)
	mux.HandleFunc("POST /log", e.taskLog)
	mux.HandleFunc("POST /beat", e.beat)
	mux.HandleFunc("POST /idleBeat", e.idleBeat)
	return mux
}
