package ui

import (
	"testing"
	"time"
)

func TestRenderBuildStatus(t *testing.T) {
	tests := []struct {
		name    string
		status  BuildStatus
		plain   string
		colored string
	}{
		{
			name:    "building",
			status:  BuildStatus{Kind: BuildStarted},
			plain:   "building...",
			colored: "\x1b[33mbuilding...\x1b[0m",
		},
		{
			name: "successful restart",
			status: BuildStatus{
				Kind:     BuildSucceeded,
				Duration: 1200 * time.Millisecond,
				PID:      1234,
			},
			plain:   "ok 1.2s -> restarted pid 1234",
			colored: "\x1b[32mok 1.2s -> restarted pid 1234\x1b[0m",
		},
		{
			name:    "failed build keeps process",
			status:  BuildStatus{Kind: BuildFailed, ExitCode: 2, PID: 1234},
			plain:   "build FAILED (exit 2) - keeping pid 1234 running",
			colored: "\x1b[31mbuild FAILED (exit 2) - keeping pid 1234 running\x1b[0m",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := RenderBuildStatus(test.status, false); got != test.plain {
				t.Errorf("plain status = %q, want %q", got, test.plain)
			}
			if got := RenderBuildStatus(test.status, true); got != test.colored {
				t.Errorf("colored status = %q, want %q", got, test.colored)
			}
		})
	}
}
