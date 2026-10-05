package ui

import (
	"fmt"
	"time"
)

type BuildStatusKind uint8

const (
	BuildStarted BuildStatusKind = iota
	BuildSucceeded
	BuildFailed
)

type BuildStatus struct {
	Kind     BuildStatusKind
	Duration time.Duration
	ExitCode int
	PID      int
}

func RenderBuildStatus(status BuildStatus, color bool) string {
	var message string
	var colorCode string
	switch status.Kind {
	case BuildStarted:
		message = "building..."
		colorCode = "33"
	case BuildSucceeded:
		message = fmt.Sprintf("ok %s", status.Duration.Round(100*time.Millisecond))
		if status.PID > 0 {
			message += fmt.Sprintf(" -> restarted pid %d", status.PID)
		}
		colorCode = "32"
	case BuildFailed:
		message = fmt.Sprintf("build FAILED (exit %d)", status.ExitCode)
		if status.PID > 0 {
			message += fmt.Sprintf(" - keeping pid %d running", status.PID)
		}
		colorCode = "31"
	default:
		return ""
	}
	if color {
		return fmt.Sprintf("\x1b[%sm%s\x1b[0m", colorCode, message)
	}
	return message
}
