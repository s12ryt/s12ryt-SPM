package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"spm/internal/server"
	"spm/internal/store"
	"spm/web"
	"sync"
	"syscall"
	"time"
)

func main() {
	os.Exit(run())
}

func run() int {
	address := flag.String("listen", env("SPM_LISTEN", "127.0.0.1:8080"), "HTTP listen address")
	data := flag.String("data", env("SPM_DATA_DIR", "data"), "Data directory")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, runtime, err := store.Load(ctx, *data, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Print("cannot open database; verify DATABASE_URL and data directory")
		return 1
	}
	defer db.DB.Close()
	app, err := server.New(ctx, db, runtime, os.Getenv("SPM_ADMIN_USER"), os.Getenv("SPM_ADMIN_PASSWORD"))
	if err != nil {
		log.Print("cannot initialize server; administrator username and 12–72 byte password required")
		return 1
	}
	if runtime.LastError != "" {
		log.Print(runtime.LastError)
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", app.Handler())
	mux.Handle("/", web.Handler())
	httpServer := &http.Server{Addr: *address, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	var workers sync.WaitGroup
	loop := func(interval time.Duration, fn func(context.Context) error) {
		workers.Go(func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					task, cancel := context.WithTimeout(ctx, 30*time.Second)
					logTaskFailure(ctx, fn(task))
					cancel()
				}
			}
		})
	}
	loop(time.Second, app.Tick)
	loop(2*time.Second, func(c context.Context) error { return app.Notify(c, &http.Client{Timeout: 5 * time.Second}) })
	loop(time.Hour, app.Prune)
	done := make(chan error, 1)
	go func() { log.Printf("SPM listening on %s", *address); done <- httpServer.ListenAndServe() }()
	exitCode := 0
	select {
	case <-ctx.Done():
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Print("HTTP listener failed")
			exitCode = 1
		}
		stop()
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if httpServer.Shutdown(shutdown) != nil {
		httpServer.Close()
	}
	workers.Wait()
	return exitCode
}
func logTaskFailure(ctx context.Context, err error) {
	if err != nil && ctx.Err() == nil {
		log.Printf("background task failed: %v", err)
	}
}
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
