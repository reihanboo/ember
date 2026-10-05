package proc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommand(t *testing.T) {
	if got := commandOutput(t, "echo hi", "", nil); got != "hi" {
		t.Errorf("echo output = %q, want hi", got)
	}

	t.Setenv("EMBER_PROC_OVERLAY", "base")
	if got := commandOutput(t, "printf '%s' \"$EMBER_PROC_OVERLAY\"", "", map[string]string{"EMBER_PROC_OVERLAY": "visible"}); got != "visible" {
		t.Errorf("environment output = %q, want visible", got)
	}

	cwd := filepath.Join(t.TempDir(), "working directory")
	if err := os.Mkdir(cwd, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := commandOutput(t, "pwd", cwd, nil); filepath.Clean(got) != filepath.Clean(cwd) {
		t.Errorf("working directory output = %q, want %q", got, cwd)
	}
}
