package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestAgentStartsWithoutNodeID(t *testing.T) {
	if os.Getenv("SPM_TEST_TOKEN_ONLY") == "1" {
		os.Args = []string{"spm-agent"}
		main()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	posted := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posted <- r.URL.Path + " " + r.Header.Get("Authorization")
		w.Write([]byte(`{"intervalSeconds":3600}`))
		cancel()
	}))
	defer srv.Close()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, exe, "-test.run=^TestAgentStartsWithoutNodeID$")
	command.Env = append(os.Environ(), "SPM_TEST_TOKEN_ONLY=1", "SPM_NODE_ID=", "SPM_SERVER="+srv.URL, "SPM_TOKEN=fixture-agent-token")
	output, _ := command.CombinedOutput()
	select {
	case got := <-posted:
		if got != "/api/ingest Bearer fixture-agent-token" {
			t.Fatalf("unexpected upload: %s", got)
		}
	default:
		t.Fatalf("Agent never uploaded without an ID: %s", output)
	}
}
