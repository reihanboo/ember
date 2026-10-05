package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/reihanboo/ember/internal/ctl"
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
	var statusOutput, statusError bytes.Buffer
	if status := RunContext(ctx, nil, []string{"status"}, &statusOutput, &statusError); status != 0 {
		t.Fatalf("status command = %d, stderr %q", status, statusError.String())
	}
	if !strings.Contains(statusOutput.String(), "State: Running\n") || !strings.Contains(statusOutput.String(), "Last build OK: true\n") {
		t.Errorf("status output = %q, want running state and successful build", statusOutput.String())
	}
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

func TestStatusCommandPrintsHumanReadableSnapshot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ember.toml"), []byte("[ui]\ncolor = false\n"), 0o600); err != nil {
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

	controlFile := filepath.Join(root, ".ember", "ctl")
	server, err := ctl.StartServer(controlFile, func(request ctl.Request) []string {
		if request != ctl.RequestStatus {
			return []string{"error: unexpected request"}
		}
		return []string{
			"state=Running",
			"pid=123",
			"last_build_ok=true",
			"last_build_ms=42",
			"last_change=2026-07-08T09:10:11.123456Z",
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	})

	var stdout, stderr bytes.Buffer
	status := RunContext(context.Background(), nil, []string{"status"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("RunContext(status) = %d, stderr %q", status, stderr.String())
	}
	want := "State: Running\nPID: 123\nLast build OK: true\nLast build duration: 42 ms\nLast change: 2026-07-08T09:10:11.123456Z\n"
	if stdout.String() != want {
		t.Errorf("status output = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Errorf("status stderr = %q, want empty", stderr.String())
	}
}

func TestControlCommandsSendRequestsToLiveServer(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ember.toml"), []byte("[ui]\ncolor = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "src", "nested")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDirectory); err != nil {
			t.Error(err)
		}
	})

	requests := make(chan ctl.Request, 5)
	server, err := ctl.StartServer(filepath.Join(root, ".ember", "ctl"), func(request ctl.Request) []string {
		requests <- request
		if request == ctl.RequestStatus {
			return []string{
				"state=Running",
				"pid=321",
				"last_build_ok=true",
				"last_build_ms=8",
				"last_change=2026-07-08T09:10:11Z",
			}
		}
		return []string{"ok"}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	})

	for _, request := range []ctl.Request{ctl.RequestReload, ctl.RequestBuild, ctl.RequestStop, ctl.RequestStart, ctl.RequestStatus} {
		var stdout, stderr bytes.Buffer
		status := RunContext(context.Background(), nil, []string{string(request)}, &stdout, &stderr)
		if status != 0 {
			t.Fatalf("RunContext(%q) = %d, stderr %q", request, status, stderr.String())
		}
		select {
		case got := <-requests:
			if got != request {
				t.Errorf("server request = %q, want %q", got, request)
			}
		case <-time.After(time.Second):
			t.Fatalf("server did not receive %q", request)
		}
		if request == ctl.RequestStatus {
			want := "State: Running\nPID: 321\nLast build OK: true\nLast build duration: 8 ms\nLast change: 2026-07-08T09:10:11Z\n"
			if stdout.String() != want {
				t.Errorf("status output = %q, want %q", stdout.String(), want)
			}
		} else if stdout.String() != "ok\n" {
			t.Errorf("%s output = %q, want ok response", request, stdout.String())
		}
		if stderr.Len() != 0 {
			t.Errorf("%s stderr = %q, want empty", request, stderr.String())
		}
	}
}

func TestControlCommandsReturnTwoWithoutInstance(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ember.toml"), []byte("[ui]\ncolor = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "src")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDirectory); err != nil {
			t.Error(err)
		}
	})

	for _, command := range []string{"reload", "build", "stop", "start", "status"} {
		var stdout, stderr bytes.Buffer
		status := RunContext(context.Background(), nil, []string{command}, &stdout, &stderr)
		if status != 2 {
			t.Errorf("RunContext(%q) = %d, want 2 (stderr %q)", command, status, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Errorf("RunContext(%q) stdout = %q, want empty", command, stdout.String())
		}
		if got := stderr.String(); got != "no running instance found; run `ember run`\n" {
			t.Errorf("RunContext(%q) stderr = %q, want no-instance hint", command, got)
		}
	}
}

func TestControlCommandReturnsOneOnServerError(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ember.toml"), []byte("[ui]\ncolor = false\n"), 0o600); err != nil {
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
	server, err := ctl.StartServer(filepath.Join(root, ".ember", "ctl"), func(ctl.Request) []string {
		return []string{"error: app is not built"}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	})

	var stdout, stderr bytes.Buffer
	status := RunContext(context.Background(), nil, []string{"start"}, &stdout, &stderr)
	if status != 1 || stderr.String() != "app is not built\n" {
		t.Errorf("start command = (%d, %q), want (1, server error)", status, stderr.String())
	}
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
