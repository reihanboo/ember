package build

import (
	"bytes"
	"context"
	"runtime"
	"strings"
	"testing"
)

func TestRunSuccess(t *testing.T) {
	var output bytes.Buffer
	result := Run(context.Background(), Spec{Cmd: "echo build-output", Output: &output})

	if !result.Success {
		t.Fatalf("Run() success = false, result = %#v", result)
	}
	if result.ExitCode != 0 {
		t.Errorf("Run() exit code = %d, want 0", result.ExitCode)
	}
	if result.Err != nil {
		t.Errorf("Run() error = %v, want nil", result.Err)
	}
	if got := output.String(); !strings.Contains(got, "[build] build-output") {
		t.Errorf("Run() output = %q, want tagged build output", got)
	}
}

func TestRunNonzeroExit(t *testing.T) {
	command := "exit 7"
	if runtime.GOOS == "windows" {
		command = "exit /b 7"
	}

	result := Run(context.Background(), Spec{Cmd: command})
	if result.Success {
		t.Fatalf("Run() success = true, want false")
	}
	if result.ExitCode != 7 {
		t.Errorf("Run() exit code = %d, want 7", result.ExitCode)
	}
	if result.Err == nil {
		t.Error("Run() error = nil, want nonzero exit error")
	}
}

func TestRunCommandNotFound(t *testing.T) {
	result := Run(context.Background(), Spec{Cmd: "ember-command-that-does-not-exist"})
	if result.Success {
		t.Fatalf("Run() success = true, want false")
	}
	if result.ExitCode == 0 {
		t.Errorf("Run() exit code = 0, want nonzero")
	}
	if result.Err == nil {
		t.Error("Run() error = nil, want command-not-found error")
	}
}
