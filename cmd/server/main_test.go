package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestLogTaskFailureIncludesReason(t *testing.T) {
	var output bytes.Buffer
	log.SetOutput(&output)
	defer log.SetOutput(os.Stderr)
	logTaskFailure(context.Background(), errors.New("database is locked"))
	if !strings.Contains(output.String(), "database is locked") {
		t.Fatalf("background task log should include the failing reason, got %q", output.String())
	}
	output.Reset()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	logTaskFailure(cancelled, errors.New("shutting down"))
	if output.Len() != 0 {
		t.Fatalf("cancelled context should not log task failures, got %q", output.String())
	}
}

func TestListenerFailureExitsNonzero(t *testing.T) {
	if os.Getenv("SPM_TEST_BAD_LISTENER") == "1" {
		os.Args = []string{"spm-server", "--listen", "invalid:listener:address"}
		main()
		return
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, exe, "-test.run=^TestListenerFailureExitsNonzero$")
	command.Env = append(os.Environ(), "SPM_TEST_BAD_LISTENER=1", "SPM_DATA_DIR="+t.TempDir(), "SPM_ADMIN_USER=admin", "SPM_ADMIN_PASSWORD=test-password-123", "DATABASE_URL=")
	output, err := command.CombinedOutput()
	if !strings.Contains(string(output), "HTTP listener failed") {
		t.Fatalf("test did not reach listener failure: %s", output)
	}
	if ctx.Err() != nil {
		t.Fatalf("server did not exit promptly: %s", output)
	}
	if err == nil {
		t.Fatalf("listener failure reported success to service manager: %s", output)
	}
}
