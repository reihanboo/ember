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
)

func TestVerboseFlagChangesRunOutput(t *testing.T) {
	plainOutput := runWithVerboseFlag(t, false)
	verboseOutput := runWithVerboseFlag(t, true)

	if strings.Contains(plainOutput, "[DEBUG]") {
		t.Errorf("default output contains debug logs: %q", plainOutput)
	}
	for _, message := range []string{"[DEBUG] changed paths:", "[DEBUG] build 1 started", "[DEBUG] build 1 finished in"} {
		if !strings.Contains(verboseOutput, message) {
			t.Errorf("verbose output %q does not contain %q", verboseOutput, message)
		}
	}
}

func runWithVerboseFlag(t *testing.T, verbose bool) string {
	t.Helper()
	root := t.TempDir()
	runCommand := "echo started >> run.log; sleep 60"
	if runtime.GOOS == "windows" {
		runCommand = "echo started >> run.log & ping -n 61 127.0.0.1 > nul"
	}
	configContents := `[watch]
paths = ["."]
include = ["*.c"]
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
	defer func() {
		if err := os.Chdir(previousDirectory); err != nil {
			t.Error(err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stdout, stderr bytes.Buffer
	args := []string{"run"}
	if verbose {
		args = append(args, "--verbose")
	}
	result := make(chan int, 1)
	go func() {
		result <- RunContext(ctx, nil, args, &stdout, &stderr)
	}()
	waitForLineCount(t, filepath.Join(root, "build.log"), 1)
	waitForLineCount(t, filepath.Join(root, "run.log"), 1)
	if err := os.WriteFile(sourcePath, []byte("int main(void) { return 1; }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitForLineCount(t, filepath.Join(root, "build.log"), 2)
	waitForLineCount(t, filepath.Join(root, "run.log"), 2)

	cancel()
	select {
	case status := <-result:
		if status != 0 {
			t.Errorf("RunContext() status = %d, stderr %q", status, stderr.String())
		}
	case <-time.After(8 * time.Second):
		t.Fatal("run command did not stop after context cancellation")
	}
	return stderr.String()
}
