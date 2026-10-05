package sup

import (
	"context"
	"strings"
	"testing"
	"time"
)

type recordingLogger chan string

func (logger recordingLogger) Debug(message string) {
	logger <- message
}

type fakeClock struct {
	now time.Time
}

func (clock fakeClock) Now() time.Time {
	return clock.now
}

func TestSupervisorProcessesEventsInOrderAndStopsOnCancellation(t *testing.T) {
	logger := make(recordingLogger, 2)
	supervisor := NewSupervisor(nil, nil, fakeClock{}, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runResult := make(chan error, 1)
	go func() {
		runResult <- supervisor.Run(ctx)
	}()

	supervisor.Send(ControlEvent{Command: "first"})
	supervisor.Send(KeyEvent{Key: 'r'})
	first := readSupervisorLog(t, logger)
	second := readSupervisorLog(t, logger)
	if !strings.Contains(first, "Command:first") {
		t.Errorf("first debug log = %q, want first control event", first)
	}
	if !strings.Contains(second, "Key:114") {
		t.Errorf("second debug log = %q, want reload key event", second)
	}

	cancel()
	select {
	case err := <-runResult:
		if err != nil {
			t.Errorf("Run() error = %v, want nil after cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not exit after context cancellation")
	}
}

func TestSupervisorUsesClockForChangeTimestamp(t *testing.T) {
	now := time.Date(2026, time.October, 5, 12, 0, 0, 0, time.UTC)
	logger := make(recordingLogger, 1)
	supervisor := NewSupervisor(nil, nil, fakeClock{now: now}, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runResult := make(chan error, 1)
	go func() {
		runResult <- supervisor.Run(ctx)
	}()

	supervisor.Send(ChangeEvent{Paths: []string{"main.c"}})
	readSupervisorLog(t, logger)
	if got := supervisor.Snapshot().LastChangeTime; !got.Equal(now) {
		t.Errorf("last change time = %s, want %s", got, now)
	}

	cancel()
	select {
	case err := <-runResult:
		if err != nil {
			t.Errorf("Run() error = %v, want nil after cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not exit after context cancellation")
	}
}

func readSupervisorLog(t *testing.T, logger <-chan string) string {
	t.Helper()
	select {
	case message := <-logger:
		return message
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for supervisor event handling")
		return ""
	}
}
