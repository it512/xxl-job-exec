package xxl

import (
	"context"
	"log/slog"
	"time"
)

type Options struct {
	ServerAddr   string        `json:"server_addr"`   //调度中心地址
	AccessToken  string        `json:"access_token"`  //请求令牌
	Timeout      time.Duration `json:"timeout"`       //接口超时时间
	ExecutorIp   string        `json:"executor_ip"`   //本地(执行器)IP(可自行获取)
	ExecutorPort string        `json:"executor_port"` //本地(执行器)端口
	RegistryKey  string        `json:"registry_key"`  //执行器名称
	LogDir       string        `json:"log_dir"`       //日志目录

	log        *slog.Logger
	rootCtx    context.Context
	logHandler LogHandler
}

func newOptions(opts ...Option) Options {
	opt := Options{
		ExecutorIp:   "127.0.0.1",
		ExecutorPort: DefaultExecutorPort,
		RegistryKey:  DefaultRegistryKey,

		log:        slog.Default(),
		rootCtx:    context.Background(),
		logHandler: defaultLogHandler,
	}

	for _, o := range opts {
		o(&opt)
	}

	return opt
}

type Option func(o *Options)

var (
	DefaultExecutorPort = "9999"
	DefaultRegistryKey  = "golang-jobs"
)

// ServerAddr 设置调度中心地址
func ServerAddr(addr string) Option {
	return func(o *Options) {
		o.ServerAddr = addr
	}
}

// AccessToken 请求令牌
func AccessToken(token string) Option {
	return func(o *Options) {
		o.AccessToken = token
	}
}

// ExecutorIp 设置执行器IP
func ExecutorIp(ip string) Option {
	return func(o *Options) {
		o.ExecutorIp = ip
	}
}

// ExecutorPort 设置执行器端口
func ExecutorPort(port string) Option {
	return func(o *Options) {
		o.ExecutorPort = port
	}
}

// RegistryKey 设置执行器标识
func RegistryKey(registryKey string) Option {
	return func(o *Options) {
		o.RegistryKey = registryKey
	}
}

// SetLogger 设置日志处理器
func SetLogger(l *slog.Logger) Option {
	return func(o *Options) {
		o.log = l
	}
}

func SetContext(ctx context.Context) Option {
	return func(o *Options) {
		o.rootCtx = ctx
	}
}

func SetLogHandler(h LogHandler) Option {
	return func(o *Options) {
		o.logHandler = h
	}
}
