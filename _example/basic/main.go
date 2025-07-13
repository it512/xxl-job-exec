package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/it512/xxl-job-exec"
)

func main() {
	exec := xxl.NewExecutor(
		xxl.ServerAddr("http://127.0.0.1:8080/xxl-job-admin"),
		xxl.AccessToken("default_token"),
		xxl.RegistryKey("test"),
		xxl.ExecutorURL("http://127.0.0.1:10010/xxl-job"),
	)

	exec.RegTask("test", func(ctx context.Context, task *xxl.Task) error {
		fmt.Println("OK")
		return nil
	})

	exec.RegTask("ok", func(ctx context.Context, task *xxl.Task) error {
		fmt.Println("OK")
		return nil
	})

	exec.RegTask("error", func(ctx context.Context, task *xxl.Task) error {
		return errors.New("this is a error")
	})

	exec.RegTask("panic", func(ctx context.Context, task *xxl.Task) error {
		panic("this is a panic")
	})

	exec.RegTask("timeout", func(ctx context.Context, task *xxl.Task) error {
		<-ctx.Done()
		fmt.Println("timeout or killed!")
		return nil
	})

	exec.Start()
	defer exec.Stop()

	mux := chi.NewMux()

	mux.Mount("/xxl-job", exec.Handle("/xxl-job"))

	http.ListenAndServe(":10010", mux)
}
