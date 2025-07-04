package xxl

import (
	"net/http"
)

func Handle(exec Executor) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /run", exec.RunTask)
	mux.HandleFunc("POST /kill", exec.KillTask)
	mux.HandleFunc("POST /log", exec.TaskLog)
	mux.HandleFunc("POST /beat", exec.Beat)
	mux.HandleFunc("POST /idleBeat", exec.IdleBeat)

	return mux
}
