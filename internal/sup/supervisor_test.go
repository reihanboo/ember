package sup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/reihanboo/ember/internal/build"
	"github.com/reihanboo/ember/internal/proc"
)

type recordingLogger chan string

func (logger recordingLogger) Debug(message string) {
	logger <- message
}

func (logger recordingLogger) Error(message string) {
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
	supervisor := NewSupervisor(nil, nil, build.Spec{}, proc.Spec{}, 0, fakeClock{}, logger)
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
	awaitSupervisorRun(t, runResult)
}

func TestSupervisorUsesClockForChangeTimestamp(t *testing.T) {
	now := time.Date(2026, time.October, 5, 12, 0, 0, 0, time.UTC)
	logger := make(recordingLogger, 1)
	supervisor := NewSupervisor(nil, nil, build.Spec{}, proc.Spec{}, 0, fakeClock{now: now}, logger)
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
	awaitSupervisorRun(t, runResult)
}

type blockingBuilder struct {
	calls chan build.Spec
}

func (builder blockingBuilder) Build(ctx context.Context, spec build.Spec) build.Result {
	builder.calls <- spec
	<-ctx.Done()
	return build.Result{Cancelled: true, Err: ctx.Err()}
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
			builder := blockingBuilder{calls: make(chan build.Spec, 1)}
			logger := make(recordingLogger, 4)
			supervisor := NewSupervisor(
				builder,
				nil,
				build.Spec{Cmd: "cc -o {out} main.c", Cwd: "."},
				proc.Spec{},
				0,
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
			select {
			case <-builder.calls:
				t.Error("builder was called more than once")
			default:
			}

			cancel()
			awaitSupervisorRun(t, runResult)
		})
	}
}

type successfulBuilder struct {
	calls chan build.Spec
	order chan string
}

func (builder successfulBuilder) Build(_ context.Context, spec build.Spec) build.Result {
	output := strings.TrimPrefix(spec.Cmd, "cc -o ")
	result := build.Result{Success: true, Duration: 25 * time.Millisecond}
	if err := os.MkdirAll(filepath.Dir(output), 0o700); err != nil {
		result.Success = false
		result.Err = err
	} else if err := os.WriteFile(output, []byte("binary"), 0o600); err != nil {
		result.Success = false
		result.Err = err
	}
	builder.calls <- spec
	if result.Success {
		builder.order <- "build-ok"
	}
	return result
}

type fakeRunner struct {
	calls   chan proc.Spec
	order   chan string
	nextPID int
}

func (runner *fakeRunner) Start(_ context.Context, spec proc.Spec) (Process, error) {
	pid := runner.nextPID
	runner.nextPID++
	runner.calls <- spec
	runner.order <- fmt.Sprintf("start:%d", pid)
	return fakeProcess{pid: pid, order: runner.order}, nil
}

type fakeProcess struct {
	pid   int
	order chan string
}

func (process fakeProcess) Pid() int {
	return process.pid
}

func (fakeProcess) Wait() proc.ExitResult {
	return proc.ExitResult{}
}

func (process fakeProcess) Stop(_ context.Context, timeout time.Duration) error {
	process.order <- fmt.Sprintf("stop:%d:%s", process.pid, timeout)
	return nil
}

type controlledBuilder struct {
	calls   chan build.Spec
	results chan build.Result
}

func (builder controlledBuilder) Build(ctx context.Context, spec build.Spec) build.Result {
	builder.calls <- spec
	select {
	case result := <-builder.results:
		return result
	case <-ctx.Done():
		return build.Result{Cancelled: true, Err: ctx.Err()}
	}
}

func TestFailedBuildKeepsRunningAppUntilNextSuccess(t *testing.T) {
	useSupervisorWorkingDirectory(t)
	builder := controlledBuilder{
		calls:   make(chan build.Spec, 3),
		results: make(chan build.Result, 3),
	}
	order := make(chan string, 8)
	runner := &fakeRunner{calls: make(chan proc.Spec, 3), order: order, nextPID: 200}
	logger := make(recordingLogger, 4)
	supervisor := NewSupervisor(
		builder,
		runner,
		build.Spec{Cmd: "cc -o {out}"},
		proc.Spec{Cmd: "{out}"},
		2*time.Second,
		fakeClock{},
		logger,
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runResult := make(chan error, 1)
	go func() {
		runResult <- supervisor.Run(ctx)
	}()

	supervisor.Send(FileChanged{Paths: []string{"main.c"}})
	readBuildCall(t, builder.calls)
	builder.results <- build.Result{Success: true}
	if got := readOrder(t, order); got != "start:200" {
		t.Fatalf("first app event = %q, want start:200", got)
	}
	readRunCall(t, runner.calls)
	waitForSupervisorState(t, supervisor, Running)

	supervisor.Send(ReloadRequested{})
	readBuildCall(t, builder.calls)
	builder.results <- build.Result{ExitCode: 1, Err: errors.New("compiler rejected source")}
	if message := readSupervisorLog(t, logger); !strings.Contains(message, "compiler rejected source") {
		t.Errorf("build failure log = %q, want error summary", message)
	}
	if snapshot := supervisor.Snapshot(); snapshot.State != BuildFailed || snapshot.PID != 200 || snapshot.LastBuildOK {
		t.Errorf("snapshot after failed build = %#v, want failed state with old process still running", snapshot)
	}
	select {
	case event := <-order:
		t.Errorf("failed build affected the old process: %q", event)
	default:
	}
	select {
	case <-runner.calls:
		t.Error("runner started a process after failed build")
	default:
	}

	supervisor.Send(ReloadRequested{})
	readBuildCall(t, builder.calls)
	builder.results <- build.Result{Success: true}
	if got := readOrder(t, order); got != "stop:200:2s" {
		t.Fatalf("recovery stop event = %q, want old app stopped first", got)
	}
	if got := readOrder(t, order); got != "start:201" {
		t.Fatalf("recovery start event = %q, want new app started after stop", got)
	}
	readRunCall(t, runner.calls)
	waitForSupervisorState(t, supervisor, Running)
	if snapshot := supervisor.Snapshot(); snapshot.PID != 201 || !snapshot.LastBuildOK {
		t.Errorf("snapshot after recovery = %#v, want new app running", snapshot)
	}

	cancel()
	awaitSupervisorRun(t, runResult)
}

func TestSuccessfulBuildRestartsAppAndPrunesOutputs(t *testing.T) {
	useSupervisorWorkingDirectory(t)
	if err := os.MkdirAll(filepath.Join(".ember", "bin"), 0o700); err != nil {
		t.Fatal(err)
	}

	order := make(chan string, 8)
	builder := successfulBuilder{calls: make(chan build.Spec, 2), order: order}
	runner := &fakeRunner{calls: make(chan proc.Spec, 2), order: order, nextPID: 100}
	logger := make(recordingLogger, 4)
	supervisor := NewSupervisor(
		builder,
		runner,
		build.Spec{Cmd: "cc -o {out}"},
		proc.Spec{Cmd: "{out} --flag", Cwd: "."},
		1500*time.Millisecond,
		fakeClock{},
		logger,
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runResult := make(chan error, 1)
	go func() {
		runResult <- supervisor.Run(ctx)
	}()

	supervisor.Send(FileChanged{Paths: []string{"main.c"}})
	firstBuild := readBuildCall(t, builder.calls)
	if got := readOrder(t, order); got != "build-ok" {
		t.Fatalf("first build event = %q, want build-ok", got)
	}
	if got := readOrder(t, order); got != "start:100" {
		t.Fatalf("first runner event = %q, want start:100 without stopping", got)
	}
	firstRun := readRunCall(t, runner.calls)
	firstOutput := strings.TrimPrefix(firstBuild.Cmd, "cc -o ")
	if firstRun.Cmd != firstOutput+" --flag" {
		t.Errorf("first run command = %q, want %q", firstRun.Cmd, firstOutput+" --flag")
	}
	waitForSupervisorState(t, supervisor, Running)
	if snapshot := supervisor.Snapshot(); snapshot.PID != 100 {
		t.Errorf("first app pid = %d, want 100", snapshot.PID)
	}

	staleOutput := filepath.Join(".ember", "bin", "app-stale.exe")
	stalePDB := filepath.Join(".ember", "bin", "app-stale.pdb")
	if err := os.WriteFile(staleOutput, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stalePDB, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}

	supervisor.Send(ReloadRequested{})
	secondBuild := readBuildCall(t, builder.calls)
	if got := readOrder(t, order); got != "build-ok" {
		t.Fatalf("second build event = %q, want build-ok", got)
	}
	if got := readOrder(t, order); got != "stop:100:1.5s" {
		t.Fatalf("old process event = %q, want stop before start", got)
	}
	if got := readOrder(t, order); got != "start:101" {
		t.Fatalf("new process event = %q, want start after stop", got)
	}
	secondRun := readRunCall(t, runner.calls)
	secondOutput := strings.TrimPrefix(secondBuild.Cmd, "cc -o ")
	if secondRun.Cmd != secondOutput+" --flag" {
		t.Errorf("second run command = %q, want %q", secondRun.Cmd, secondOutput+" --flag")
	}
	waitForSupervisorState(t, supervisor, Running)
	if snapshot := supervisor.Snapshot(); snapshot.PID != 101 || !snapshot.LastBuildOK {
		t.Errorf("snapshot after restart = %#v, want running pid 101 and successful build", snapshot)
	}
	for _, output := range []string{firstOutput, secondOutput} {
		if _, err := os.Stat(output); err != nil {
			t.Errorf("retained output %q is missing: %v", output, err)
		}
	}
	for _, stale := range []string{staleOutput, stalePDB} {
		waitForPathMissing(t, stale)
	}

	cancel()
	awaitSupervisorRun(t, runResult)
}

func useSupervisorWorkingDirectory(t *testing.T) {
	t.Helper()
	directory := t.TempDir()
	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDirectory); err != nil {
			t.Error(err)
		}
	})
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

func readBuildCall(t *testing.T, calls <-chan build.Spec) build.Spec {
	t.Helper()
	select {
	case spec := <-calls:
		return spec
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for build call")
		return build.Spec{}
	}
}

func readRunCall(t *testing.T, calls <-chan proc.Spec) proc.Spec {
	t.Helper()
	select {
	case spec := <-calls:
		return spec
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for run call")
		return proc.Spec{}
	}
}

func readOrder(t *testing.T, order <-chan string) string {
	t.Helper()
	select {
	case event := <-order:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for supervisor operation")
		return ""
	}
}

func waitForPathMissing(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Errorf("path %q still exists after pruning", path)
}

func waitForSupervisorState(t *testing.T, supervisor *Supervisor, state State) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if supervisor.Snapshot().State == state {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("supervisor state = %s, want %s", supervisor.Snapshot().State, state)
}

func awaitSupervisorRun(t *testing.T, result <-chan error) {
	t.Helper()
	select {
	case err := <-result:
		if err != nil {
			t.Errorf("Run() error = %v, want nil after cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not exit after context cancellation")
	}
}
