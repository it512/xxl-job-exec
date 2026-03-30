package xxl

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func jsonTo(code int, resp any, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)

	return marshalWrite(w, resp)
}

func jsonToNoErr(code int, resp any, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = marshalWrite(w, resp)
}

func marshalWrite(w io.Writer, a any) error {
	enc := json.NewEncoder(w)
	return enc.Encode(a)
}

func bind(r io.Reader, p any) error {
	dec := json.NewDecoder(r)
	return dec.Decode(p)
}

func TaskJsonParam(task *Task, p any) error {
	r := strings.NewReader(task.Param.ExecutorParams)
	return bind(r, p)
}
