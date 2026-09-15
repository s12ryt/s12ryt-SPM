package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

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
