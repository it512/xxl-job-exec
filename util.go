package xxl

import (
	"encoding/json"
	"io"
	"net/http"
)

func JsonTo(code int, resp any, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	return enc.Encode(resp)
}

func BindAndClose(r *http.Request, p any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(p); err != nil {
		return err
	}
	return nil
}

func Bind(r io.Reader, p any) error {
	dec := json.NewDecoder(r)
	if err := dec.Decode(p); err != nil {
		return err
	}
	return nil
}

func newCall(req RunReq, code int, msg string) *call {
	data := &call{
		&callElement{
			LogID:      req.LogID,
			LogDateTim: req.LogDateTime,
			ExecuteResult: &ExecuteResult{
				Code: code,
				Msg:  msg,
			},
			HandleCode: code,
			HandleMsg:  msg,
		},
	}
	return data
}

func returnCall2(req RunReq, code int, msg string, w http.ResponseWriter) error {
	data := call{
		&callElement{
			LogID:      req.LogID,
			LogDateTim: req.LogDateTime,
			ExecuteResult: &ExecuteResult{
				Code: code,
				Msg:  msg,
			},
			HandleCode: code,
			HandleMsg:  msg,
		},
	}
	return JsonTo(http.StatusOK, data, w)
}

func returnCode(code int, w http.ResponseWriter) error {
	data := res{
		Code: code,
		Msg:  "",
	}
	return JsonTo(http.StatusOK, data, w)
}
