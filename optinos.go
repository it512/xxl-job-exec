package xxl

import (
	"context"
	"log/slog"
)

type Options struct {
	AccessToken string //请求令牌
	ExecutorURL string //本地(执行器) URL
	RegistryKey string //执行器名称

	ServerAddr []string //调度中心地址

	log        *slog.Logger
	client     Doer
	rootCtx    context.Context
	logHandler LogHandler
}

type Option func(o *Options)

var (
	DefaultExecutorPort = "19999"
	DefaultRegistryKey  = "golang-jobs-plus"
	DefaultAccessToken  = "default_token"
)

// ServerAddr 设置调度中心地址
func ServerAddr(urls ...string) Option {
	return func(o *Options) {
		o.ServerAddr = urls
	}
}

// AccessToken 请求令牌
func AccessToken(token string) Option {
	return func(o *Options) {
		o.AccessToken = token
	}
}

// ExecutorURL 设置执行器URL
func ExecutorURL(url string) Option {
	return func(o *Options) {
		o.ExecutorURL = url
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

func SetHttpClient(doer Doer) Option {
	return func(o *Options) {
		o.client = doer
	}
}
