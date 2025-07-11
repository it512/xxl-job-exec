package xxl

/**
用来日志查询，显示到xxl-job-admin后台
*/

type LogHandler func(req LogParam) Return[LogResult]

// 默认返回
func defaultLogHandler(req LogParam) Return[LogResult] {
	return Return[LogResult]{
		Code: SuccessCode,
		Msg:  "",
		Content: LogResult{
			FromLineNum: req.FromLineNum,
			ToLineNum:   2,
			LogContent:  "这是日志默认返回，说明没有设置LogHandler",
			IsEnd:       true,
		},
	}
}

func reqErrLogHandler(err error) Return[LogResult] {
	return Return[LogResult]{
		Code: FailureCode,
		Msg:  err.Error(),
		Content: LogResult{
			FromLineNum: 0,
			ToLineNum:   0,
			LogContent:  err.Error(),
			IsEnd:       true,
		},
	}
}
