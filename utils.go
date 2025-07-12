package xxl

import (
	"encoding/json"
	"io"
	"net/http"
)

func jsonTo(code int, resp any, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	return enc.Encode(resp)
}

func bind(r io.Reader, p any) error {
	dec := json.NewDecoder(r)
	if err := dec.Decode(p); err != nil {
		return err
	}
	return nil
}
