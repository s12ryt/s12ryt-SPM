package main

import (
	"context"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestAgentCLIProcess(t *testing.T) {
	if os.Getenv("SPM_TEST_CLI") != "1" {
		return
	}
	os.Args = append([]string{"spm-agent"}, flag.Args()...)
	main()
}

func agentCLI(t *testing.T, ctx context.Context, server, token string, args ...string) *exec.Cmd {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, exe, append([]string{"-test.run=^TestAgentCLIProcess$", "--"}, args...)...)
	command.Env = append(os.Environ(), "SPM_TEST_CLI=1", "SPM_NODE_ID=", "SPM_SERVER="+server, "SPM_TOKEN="+token)
	return command
}

func TestAgentConnectionFlags(t *testing.T) {
	for _, inherited := range []bool{false, true} {
		name := "flags only"
		if inherited {
			name = "flags override environment"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			posted := make(chan string, 1)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				posted <- r.URL.Path + " " + r.Header.Get("Authorization")
				w.Write([]byte(`{"intervalSeconds":3600}`))
				cancel()
			}))
			defer srv.Close()
			server, token := "", ""
			if inherited {
				server, token = "http://127.0.0.1:1", "unused-environment-token"
			}
			output, _ := agentCLI(t, ctx, server, token, "--server", srv.URL, "--token", "cli-agent-token").CombinedOutput()
			select {
			case got := <-posted:
				if got != "/api/ingest Bearer cli-agent-token" {
					t.Fatalf("unexpected upload: %s", got)
				}
			default:
				t.Fatalf("Agent never uploaded using flags: %s", output)
			}
			if strings.Contains(string(output), "cli-agent-token") {
				t.Fatal("Agent logged its token")
			}
		})
	}
}

func TestAgentFlagErrorsAndHelpHideToken(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"--token"}, {"--token", ""}, {"--server", ""}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			output, err := agentCLI(t, ctx, "https://monitor.example.com", "secret-environment-token", args...).CombinedOutput()
			if ctx.Err() != nil {
				t.Fatal("invalid flags started the Agent instead of exiting")
			}
			if args[0] == "--help" {
				if err != nil || !strings.Contains(string(output), "-token") {
					t.Fatalf("missing token usage: %s", output)
				}
			} else if err == nil {
				t.Fatal("invalid flags unexpectedly succeeded")
			}
			if strings.Contains(string(output), "secret-environment-token") {
				t.Fatal("help or error disclosed the environment token")
			}
		})
	}
}

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
