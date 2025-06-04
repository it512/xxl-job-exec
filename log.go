package xxl

import (
	"fmt"
)

// LogFunc 应用日志
type LogFunc func(req LogReq, res *LogRes) []byte

// Logger 系统日志
type Logger interface {
	Info(format string, a ...any)
	Error(format string, a ...any)
}

type logger struct {
}

func (l *logger) Info(format string, a ...any) {
	fmt.Println(fmt.Sprintf(format, a...))
}

func (l *logger) Error(format string, a ...any) {
	fmt.Println(fmt.Sprintf(format, a...))
}
