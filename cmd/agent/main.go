package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"spm/internal/collector"
	"syscall"
	"time"
)

func main() {
	endpoint := flag.String("server", os.Getenv("SPM_SERVER"), "Server URL")
	id := flag.String("id", os.Getenv("SPM_NODE_ID"), "Optional legacy node ID")
	flag.Parse()
	token := os.Getenv("SPM_TOKEN")
	if *endpoint == "" || token == "" {
		log.Fatal("SPM_SERVER and SPM_TOKEN are required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client := &http.Client{Timeout: 10 * time.Second}
	log.Print("SPM Agent started")
	collector.Run(ctx, client, *endpoint, *id, token, func(err error) { log.Print(err) })
	log.Print("SPM Agent stopped")
}
