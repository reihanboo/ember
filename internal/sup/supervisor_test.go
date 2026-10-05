package sup

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/reihanboo/ember/internal/build"
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
	supervisor := NewSupervisor(nil, nil, build.Spec{}, fakeClock{}, logger)
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
	supervisor := NewSupervisor(nil, nil, build.Spec{}, fakeClock{now: now}, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runResult := make(chan error, 1)
	go func() {
		runResult <- supervisor.Run(ctx)
	}()

	supervisor.Send(FileChanged{Paths: []string{"main.c"}})
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

type fakeBuilder struct {
	calls chan build.Spec
}

func (builder fakeBuilder) Build(_ context.Context, spec build.Spec) build.Result {
	builder.calls <- spec
	return build.Result{Success: true}
}

func TestBuildTriggersStartOneBuild(t *testing.T) {
	triggers := []struct {
		name  string
		event Event
	}{
		{name: "file change", event: FileChanged{Paths: []string{"main.c"}}},
		{name: "manual reload", event: ReloadRequested{}},
	}

	for _, trigger := range triggers {
		t.Run(trigger.name, func(t *testing.T) {
			builder := fakeBuilder{calls: make(chan build.Spec, 1)}
			logger := make(recordingLogger, 4)
			supervisor := NewSupervisor(
				builder,
				nil,
				build.Spec{Cmd: "cc -o {out} main.c", Cwd: "."},
				fakeClock{},
				logger,
			)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			runResult := make(chan error, 1)
			go func() {
				runResult <- supervisor.Run(ctx)
			}()

			supervisor.Send(trigger.event)
			var spec build.Spec
			select {
			case spec = <-builder.calls:
			case <-time.After(time.Second):
				t.Fatal("builder was not called")
			}
			if !strings.HasPrefix(filepath.ToSlash(spec.Cmd), "cc -o .ember/bin/app-") {
				t.Errorf("build command = %q, want allocated output path", spec.Cmd)
			}
			if strings.Contains(spec.Cmd, "{out}") {
				t.Errorf("build command still contains {out}: %q", spec.Cmd)
			}
			if !strings.HasSuffix(spec.Cmd, " main.c") {
				t.Errorf("build command = %q, want source argument preserved", spec.Cmd)
			}
			if snapshot := supervisor.Snapshot(); snapshot.State != Building {
				t.Errorf("state = %s, want Building", snapshot.State)
			}
			if completion := readSupervisorLog(t, logger); !strings.Contains(completion, "BuildFinishedEvent") {
				t.Errorf("build completion log = %q, want posted build-finished event", completion)
			}
			select {
			case <-builder.calls:
				t.Error("builder was called more than once")
			default:
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
		})
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
