package main

import (
	"context"
	"errors"
	"github.com/papergrade/papergrade/internal/app"
	"github.com/papergrade/papergrade/internal/store"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"
)

func main() {
	if e := run(); e != nil {
		slog.Error("papergrade stopped", "error", e)
		os.Exit(1)
	}
}
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if os.Getenv("GOMAXPROCS") == "" {
		runtime.GOMAXPROCS(1)
	}
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(96 << 20)
	}
	command := "api"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	if command == "healthcheck" {
		client := &http.Client{Timeout: 3 * time.Second}
		resp, e := client.Get("http://127.0.0.1:8080/readyz")
		if e != nil {
			return e
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return &app.APIError{Status: 503, Code: "NOT_READY", Message: "API not ready"}
		}
		return nil
	}
	if command == "migrate" {
		s, e := store.Open(ctx, os.Getenv("DATABASE_URL"))
		if e != nil {
			return e
		}
		defer s.Close()
		return s.Migrate(ctx)
	}
	c := app.ConfigFromEnv()
	a, e := app.New(ctx, c)
	if e != nil {
		return e
	}
	defer a.Store.Close()
	if e = a.Init(ctx); e != nil {
		return e
	}
	switch command {
	case "worker":
		return a.Worker(ctx)
	case "api":
		if c.Dev {
			slog.Warn("development mode: insecure cookies, memory data is lost on restart", "storage_mode", a.Store.Mode())
		}
		srv := &http.Server{Addr: c.Addr, Handler: a.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 190 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
		go func() {
			<-ctx.Done()
			shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			srv.Shutdown(shutdown)
		}()
		slog.Info("API starting", "addr", c.Addr, "storage_mode", a.Store.Mode())
		e = srv.ListenAndServe()
		if errors.Is(e, http.ErrServerClosed) {
			return nil
		}
		return e
	default:
		return &app.APIError{Status: 400, Code: "USAGE", Message: "usage: papergrade api|worker|migrate|healthcheck"}
	}
}
