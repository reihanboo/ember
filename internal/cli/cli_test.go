package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestRunBuildsStartsAndReloadsOnChange(t *testing.T) {
	root := t.TempDir()
	runCommand := "echo started >> run.log; sleep 60"
	if runtime.GOOS == "windows" {
		runCommand = "echo started >> run.log & ping -n 61 127.0.0.1 > nul"
	}
	configContents := `[watch]
paths = ["."]
include = ["*.c", "**/*.c"]
ignore = [".ember/**", "build.log", "run.log"]
debounce_ms = 50
poll = true

[build]
cmd = "echo built >> build.log"

[run]
cmd = "` + runCommand + `"
cwd = "."
kill_timeout_ms = 2000

[ui]
color = false
`
	if err := os.WriteFile(filepath.Join(root, "ember.toml"), []byte(configContents), 0o600); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(root, "main.c")
	if err := os.WriteFile(sourcePath, []byte("int main(void) { return 0; }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDirectory); err != nil {
			t.Error(err)
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan int, 1)
	go func() {
		result <- RunContext(ctx, nil, []string{"run"}, &bytes.Buffer{}, &bytes.Buffer{})
	}()
	waitForLineCount(t, filepath.Join(root, "build.log"), 1)
	waitForLineCount(t, filepath.Join(root, "run.log"), 1)
	if err := os.WriteFile(sourcePath, []byte("int main(void) { return 1; }\n\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitForLineCount(t, filepath.Join(root, "build.log"), 2)
	waitForLineCount(t, filepath.Join(root, "run.log"), 2)

	cancel()
	select {
	case status := <-result:
		if status != 0 {
			t.Errorf("RunContext() status = %d, want 0", status)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run command did not stop after context cancellation")
	}
}

func waitForLineCount(t *testing.T, path string, count int) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		contents, err := os.ReadFile(path)
		if err == nil && bytes.Count(contents, []byte("\n")) >= count {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	contents, err := os.ReadFile(path)
	t.Fatalf("%q has %d lines, want at least %d (read error: %v)", path, bytes.Count(contents, []byte("\n")), count, err)
}

func TestRun(t *testing.T) {
	usage := "Usage: ember <command>\n\nCommands:\n  run\n  reload\n  build\n  stop\n  start\n  status\n  init\n"
	tests := []struct {
		name       string
		args       []string
		wantStatus int
		wantStdout string
		wantStderr string
	}{

		{name: "reload", args: []string{"reload"}, wantStatus: 1, wantStderr: "not implemented\n"},
		{name: "build", args: []string{"build"}, wantStatus: 1, wantStderr: "not implemented\n"},
		{name: "stop", args: []string{"stop"}, wantStatus: 1, wantStderr: "not implemented\n"},
		{name: "start", args: []string{"start"}, wantStatus: 1, wantStderr: "not implemented\n"},
		{name: "status", args: []string{"status"}, wantStatus: 1, wantStderr: "not implemented\n"},
		{name: "unknown", args: []string{"unknown"}, wantStatus: 1, wantStderr: usage},
		{name: "missing", wantStatus: 1, wantStderr: usage},
		{name: "help", args: []string{"-h"}, wantStdout: usage},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			status := Run(test.args, &stdout, &stderr)
			if status != test.wantStatus {
				t.Errorf("Run() status = %d, want %d", status, test.wantStatus)
			}
			if stdout.String() != test.wantStdout {
				t.Errorf("Run() stdout = %q, want %q", stdout.String(), test.wantStdout)
			}
			if stderr.String() != test.wantStderr {
				t.Errorf("Run() stderr = %q, want %q", stderr.String(), test.wantStderr)
			}
		})
	}
}
