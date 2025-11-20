package xxl

import (
	"encoding/json/v2"
	"io"
	"net/http"
	"strings"
)

func jsonTo(code int, resp any, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)

	return json.MarshalWrite(w, resp)
}

func bind(r io.Reader, p any) error {
	return json.UnmarshalRead(r, p)
}

func TaskJsonParam(task *Task, p any) error {
	r := strings.NewReader(task.Param.ExecutorParams)
	return bind(r, p)
}
