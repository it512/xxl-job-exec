package xxl

// 响应码
const (
	SuccessCode = 200
	FailureCode = 500
)

// Middleware 中间件构造函数
type Middleware func(TaskFunc) TaskFunc

func (e *Executor) chain(next TaskFunc) TaskFunc {
	for i := range e.middlewares {
		next = e.middlewares[len(e.middlewares)-1-i](next)
	}
	return next
}

// 通用响应
type Return[T any] struct {
	Code    int    `json:"code"`              // 200 表示正常、其他失败
	Msg     string `json:"msg,omitempty"`     // 错误提示消息
	Content T      `json:"content,omitempty"` // 响应内容
}

/*****************  上行参数  *********************/

// RegistryParam 注册参数
type RegistryParam struct {
	RegistryGroup string `json:"registryGroup"`
	RegistryKey   string `json:"registryKey"`
	RegistryValue string `json:"registryValue"`
}

// 执行器执行完任务后，回调任务结果时使用
type CallbackParamList []HandleCallbackParam

type HandleCallbackParam struct {
	LogID      int64  `json:"logId"`
	LogDateTim int64  `json:"logDateTim"`
	HandleCode int    `json:"handleCode"` //200表示正常,500表示失败
	HandleMsg  string `json:"handleMsg"`
}

/*****************  下行参数  *********************/

// 阻塞处理策略
const (
	serialExecution = "SERIAL_EXECUTION" //单机串行
	discardLater    = "DISCARD_LATER"    //丢弃后续调度
	coverEarly      = "COVER_EARLY"      //覆盖之前调度
)

// TriggerParam 触发任务请求参数
type TriggerParam struct {
	JobID                 int64  `json:"jobId"`                 // 任务ID
	ExecutorHandler       string `json:"executorHandler"`       // 任务标识
	ExecutorParams        string `json:"executorParams"`        // 任务参数
	ExecutorBlockStrategy string `json:"executorBlockStrategy"` // 任务阻塞策略
	ExecutorTimeout       int64  `json:"executorTimeout"`       // 任务超时时间，单位秒，大于零时生效
	LogID                 int64  `json:"logId"`                 // 本次调度日志ID
	LogDateTime           int64  `json:"logDateTime"`           // 本次调度日志时间
	GlueType              string `json:"glueType"`              // 任务模式，可选值参考 com.xxl.job.core.glue.GlueTypeEnum
	GlueSource            string `json:"glueSource"`            // GLUE脚本代码
	GlueUpdatetime        int64  `json:"glueUpdatetime"`        // GLUE脚本更新时间，用于判定脚本是否变更以及是否需要刷新
	BroadcastIndex        int64  `json:"broadcastIndex"`        // 分片参数：当前分片
	BroadcastTotal        int64  `json:"broadcastTotal"`        // 分片参数：总分片
}

// 终止任务请求参数
type KillParam struct {
	JobID int64 `json:"jobId"` // 任务ID
}

// 忙碌检测请求参数
type IdleBeatParam struct {
	JobID int64 `json:"jobId"` // 任务ID
}

// LogParam 日志请求
type LogParam struct {
	LogDateTim  int64 `json:"logDateTim"`  // 本次调度日志时间
	LogID       int64 `json:"logId"`       // 本次调度日志ID
	FromLineNum int   `json:"fromLineNum"` // 日志开始行号，滚动加载日志
}

// LogResult 日志响应内容
type LogResult struct {
	FromLineNum int    `json:"fromLineNum"` // 本次请求，日志开始行数
	ToLineNum   int    `json:"toLineNum"`   // 本次请求，日志结束行号
	LogContent  string `json:"logContent"`  // 本次请求日志内容
	IsEnd       bool   `json:"isEnd"`       // 日志是否全部加载完
}

func newCallback(t TriggerParam, code int, msg string) HandleCallbackParam {
	return HandleCallbackParam{
		LogID:      t.LogID,
		LogDateTim: t.LogDateTime,
		HandleCode: code,
		HandleMsg:  msg,
	}
}

var ReturnSuccess = Success("")
var ReturnFailure = Failure("")

func Success(msg string) Return[string] {
	return Return[string]{
		Code: SuccessCode,
		Msg:  msg,
	}
}

func Failure(msg string) Return[string] {
	return Return[string]{
		Code: FailureCode,
		Msg:  msg,
	}
}
