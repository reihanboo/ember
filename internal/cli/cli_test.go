package cli

import (
	"bytes"
	"testing"
)

func TestRun(t *testing.T) {
	usage := "Usage: ember <command>\n\nCommands:\n  run\n  reload\n  build\n  stop\n  start\n  status\n  init\n"
	tests := []struct {
		name       string
		args       []string
		wantStatus int
		wantStdout string
		wantStderr string
	}{
		{name: "run", args: []string{"run"}, wantStatus: 1, wantStderr: "not implemented\n"},
		{name: "reload", args: []string{"reload"}, wantStatus: 1, wantStderr: "not implemented\n"},
		{name: "build", args: []string{"build"}, wantStatus: 1, wantStderr: "not implemented\n"},
		{name: "stop", args: []string{"stop"}, wantStatus: 1, wantStderr: "not implemented\n"},
		{name: "start", args: []string{"start"}, wantStatus: 1, wantStderr: "not implemented\n"},
		{name: "status", args: []string{"status"}, wantStatus: 1, wantStderr: "not implemented\n"},
		{name: "init", args: []string{"init"}, wantStatus: 1, wantStderr: "not implemented\n"},
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
