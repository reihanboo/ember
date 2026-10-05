package sup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	supervisor := NewSupervisor(nil, nil, build.Spec{}, proc.Spec{}, 0, false, fakeClock{}, logger)
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
	supervisor := NewSupervisor(nil, nil, build.Spec{}, proc.Spec{}, 0, false, fakeClock{now: now}, logger)
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
				false,
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
	calls     chan proc.Spec
	processes chan *fakeProcess
	order     chan string
	nextPID   int
}

func (runner *fakeRunner) Start(_ context.Context, spec proc.Spec) (Process, error) {
	pid := runner.nextPID
	runner.nextPID++
	process := &fakeProcess{
		pid:   pid,
		order: runner.order,
		done:  make(chan struct{}),
	}
	runner.calls <- spec
	runner.order <- fmt.Sprintf("start:%d", pid)
	runner.processes <- process
	return process, nil
}

type fakeProcess struct {
	pid        int
	order      chan string
	done       chan struct{}
	finish     sync.Once
	exitResult proc.ExitResult
}

func (process *fakeProcess) Pid() int {
	return process.pid
}

func (process *fakeProcess) Wait() proc.ExitResult {
	<-process.done
	return process.exitResult
}

func (process *fakeProcess) Stop(_ context.Context, timeout time.Duration) error {
	process.order <- fmt.Sprintf("stop:%d:%s", process.pid, timeout)
	process.exit(proc.ExitResult{Code: 1})
	return nil
}

func (process *fakeProcess) exit(result proc.ExitResult) {
	process.finish.Do(func() {
		process.exitResult = result
		close(process.done)
	})
}

type controlledBuilder struct {
	calls   chan build.Spec
	results chan build.Result
	order   chan string
}

func (builder controlledBuilder) Build(ctx context.Context, spec build.Spec) build.Result {
	if builder.order != nil {
		builder.order <- "build"
	}
	builder.calls <- spec
	select {
	case result := <-builder.results:
		return result
	case <-ctx.Done():
		return build.Result{Cancelled: true, Err: ctx.Err()}
	}
}

type supersedingBuilder struct {
	calls         chan build.Spec
	firstCanceled chan struct{}
	results       chan build.Result
	mu            sync.Mutex
	started       int
}

func (builder *supersedingBuilder) Build(ctx context.Context, spec build.Spec) build.Result {
	builder.mu.Lock()
	builder.started++
	started := builder.started
	builder.mu.Unlock()
	builder.calls <- spec
	if started == 1 {
		<-ctx.Done()
		close(builder.firstCanceled)
		return build.Result{Cancelled: true, Err: ctx.Err()}
	}
	select {
	case result := <-builder.results:
		return result
	case <-ctx.Done():
		return build.Result{Cancelled: true, Err: ctx.Err()}
	}
}

func TestNewTriggerSupersedesBuildingBuild(t *testing.T) {
	useSupervisorWorkingDirectory(t)
	if err := os.MkdirAll(filepath.Join(".ember", "bin"), 0o700); err != nil {
		t.Fatal(err)
	}

	order := make(chan string, 4)
	builder := &supersedingBuilder{
		calls:         make(chan build.Spec, 2),
		firstCanceled: make(chan struct{}),
		results:       make(chan build.Result, 1),
	}
	runner := &fakeRunner{calls: make(chan proc.Spec, 2), processes: make(chan *fakeProcess, 2), order: order, nextPID: 300}
	logger := make(recordingLogger, 4)
	supervisor := NewSupervisor(
		builder,
		runner,
		build.Spec{Cmd: "cc -o {out}"},
		proc.Spec{Cmd: "{out}"},
		2*time.Second,
		false,
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
	if snapshot := supervisor.Snapshot(); snapshot.State != Building {
		t.Fatalf("state after first trigger = %s, want Building", snapshot.State)
	}

	supervisor.Send(ReloadRequested{})
	secondBuild := readBuildCall(t, builder.calls)
	<-builder.firstCanceled
	if firstBuild.Cmd == secondBuild.Cmd {
		t.Errorf("superseding build reused output command %q", secondBuild.Cmd)
	}
	if message := readSupervisorLog(t, logger); !strings.Contains(message, "stale build result") {
		t.Errorf("cancelled build log = %q, want stale completion discarded", message)
	}
	if snapshot := supervisor.Snapshot(); snapshot.State != Building {
		t.Errorf("state after stale completion = %s, want Building", snapshot.State)
	}

	builder.results <- build.Result{Success: true, Duration: 10 * time.Millisecond}
	if got := readOrder(t, order); got != "start:300" {
		t.Fatalf("runner event = %q, want one app start", got)
	}
	runSpec := readRunCall(t, runner.calls)
	secondOutput := strings.TrimPrefix(secondBuild.Cmd, "cc -o ")
	if runSpec.Cmd != secondOutput {
		t.Errorf("run command = %q, want superseding output %q", runSpec.Cmd, secondOutput)
	}
	waitForSupervisorState(t, supervisor, Running)
	if snapshot := supervisor.Snapshot(); snapshot.PID != 300 || !snapshot.LastBuildOK {
		t.Errorf("snapshot after superseding build = %#v, want one successful app start", snapshot)
	}
	process := readFakeProcess(t, runner.processes)
	select {
	case event := <-order:
		t.Errorf("unexpected extra app operation %q", event)
	default:
	}
	select {
	case <-runner.calls:
		t.Error("runner started more than one app")
	default:
	}

	cancel()
	awaitSupervisorRun(t, runResult)
	process.exit(proc.ExitResult{Code: 0})
}

func TestFailedBuildKeepsRunningAppUntilNextSuccess(t *testing.T) {
	useSupervisorWorkingDirectory(t)
	builder := controlledBuilder{
		calls:   make(chan build.Spec, 3),
		results: make(chan build.Result, 3),
	}
	order := make(chan string, 8)
	runner := &fakeRunner{calls: make(chan proc.Spec, 3), processes: make(chan *fakeProcess, 3), order: order, nextPID: 200}
	logger := make(recordingLogger, 4)
	supervisor := NewSupervisor(
		builder,
		runner,
		build.Spec{Cmd: "cc -o {out}"},
		proc.Spec{Cmd: "{out}"},
		2*time.Second,
		false,
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
	oldProcess := readFakeProcess(t, runner.processes)
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
	newProcess := readFakeProcess(t, runner.processes)
	waitForSupervisorState(t, supervisor, Running)
	if snapshot := supervisor.Snapshot(); snapshot.PID != 201 || !snapshot.LastBuildOK {
		t.Errorf("snapshot after recovery = %#v, want new app running", snapshot)
	}

	cancel()
	awaitSupervisorRun(t, runResult)
	oldProcess.exit(proc.ExitResult{Code: 1})
	newProcess.exit(proc.ExitResult{Code: 0})
}

func TestStopsRunningBeforeBuild(t *testing.T) {
	for _, outcome := range []struct {
		name   string
		result build.Result
		starts bool
	}{
		{name: "success", result: build.Result{Success: true}, starts: true},
		{name: "failure", result: build.Result{ExitCode: 1, Err: errors.New("compiler rejected source")}},
	} {
		t.Run(outcome.name, func(t *testing.T) {
			useSupervisorWorkingDirectory(t)
			order := make(chan string, 8)
			builder := controlledBuilder{
				calls:   make(chan build.Spec, 2),
				results: make(chan build.Result, 2),
				order:   order,
			}
			runner := &fakeRunner{calls: make(chan proc.Spec, 2), processes: make(chan *fakeProcess, 2), order: order, nextPID: 500}
			logger := make(recordingLogger, 8)
			supervisor := NewSupervisor(
				builder,
				runner,
				build.Spec{Cmd: "cc -o {out}"},
				proc.Spec{Cmd: "{out}"},
				2*time.Second,
				true,
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
			if got := readOrder(t, order); got != "build" {
				t.Fatalf("initial order = %q, want build", got)
			}
			readBuildCall(t, builder.calls)
			builder.results <- build.Result{Success: true}
			if got := readOrder(t, order); got != "start:500" {
				t.Fatalf("initial app order = %q, want start:500", got)
			}
			readRunCall(t, runner.calls)
			oldProcess := readFakeProcess(t, runner.processes)
			waitForSupervisorState(t, supervisor, Running)

			supervisor.Send(ReloadRequested{})
			if got := readOrder(t, order); got != "stop:500:2s" {
				t.Fatalf("pre-build order = %q, want old app stopped first", got)
			}
			if got := readOrder(t, order); got != "build" {
				t.Fatalf("pre-build order = %q, want build after stop", got)
			}
			readBuildCall(t, builder.calls)
			if snapshot := supervisor.Snapshot(); snapshot.State != Building || snapshot.PID != 0 {
				t.Errorf("snapshot during build = %#v, want Building with no app", snapshot)
			}
			builder.results <- outcome.result

			if outcome.starts {
				if got := readOrder(t, order); got != "start:501" {
					t.Fatalf("replacement app order = %q, want start:501", got)
				}
				readRunCall(t, runner.calls)
				newProcess := readFakeProcess(t, runner.processes)
				waitForSupervisorState(t, supervisor, Running)
				if snapshot := supervisor.Snapshot(); snapshot.PID != 501 || !snapshot.LastBuildOK {
					t.Errorf("snapshot after successful build = %#v, want replacement app running", snapshot)
				}
				cancel()
				awaitSupervisorRun(t, runResult)
				oldProcess.exit(proc.ExitResult{Code: 1})
				newProcess.exit(proc.ExitResult{Code: 0})
				return
			}

			message := readSupervisorLogContaining(t, logger, "app remains stopped because stops_running is enabled")
			if !strings.Contains(message, "compiler rejected source") {
				t.Errorf("failure log = %q, want compiler error", message)
			}
			if snapshot := supervisor.Snapshot(); snapshot.State != BuildFailed || snapshot.PID != 0 || snapshot.LastBuildOK {
				t.Errorf("snapshot after failed build = %#v, want BuildFailed with no app", snapshot)
			}
			select {
			case <-runner.calls:
				t.Error("runner started an app after failed build")
			default:
			}
			select {
			case event := <-order:
				t.Errorf("unexpected app operation after failed build: %q", event)
			default:
			}

			cancel()
			awaitSupervisorRun(t, runResult)
		})
	}
}

func TestBuildOnlyDoesNotRestartRunningApp(t *testing.T) {
	useSupervisorWorkingDirectory(t)
	if err := os.MkdirAll(filepath.Join(".ember", "bin"), 0o700); err != nil {
		t.Fatal(err)
	}

	order := make(chan string, 8)
	builder := successfulBuilder{calls: make(chan build.Spec, 2), order: order}
	runner := &fakeRunner{calls: make(chan proc.Spec, 2), processes: make(chan *fakeProcess, 2), order: order, nextPID: 700}
	logger := make(recordingLogger, 4)
	supervisor := NewSupervisor(
		builder,
		runner,
		build.Spec{Cmd: "cc -o {out}"},
		proc.Spec{Cmd: "{out} --flag", Cwd: "."},
		1500*time.Millisecond,
		true,
		fakeClock{},
		logger,
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runResult := make(chan error, 1)
	go func() {
		runResult <- supervisor.Run(ctx)
	}()

	supervisor.Send(ReloadRequested{})
	readBuildCall(t, builder.calls)
	if got := readOrder(t, order); got != "build-ok" {
		t.Fatalf("initial build event = %q, want build-ok", got)
	}
	if got := readOrder(t, order); got != "start:700" {
		t.Fatalf("initial runner event = %q, want start:700", got)
	}
	readRunCall(t, runner.calls)
	process := readFakeProcess(t, runner.processes)
	waitForSupervisorState(t, supervisor, Running)

	supervisor.Send(BuildOnlyRequested{})
	readBuildCall(t, builder.calls)
	if got := readOrder(t, order); got != "build-ok" {
		t.Fatalf("build-only event = %q, want build-ok", got)
	}
	waitForSupervisorState(t, supervisor, Running)
	if snapshot := supervisor.Snapshot(); snapshot.PID != process.Pid() || !snapshot.LastBuildOK {
		t.Errorf("snapshot after build-only = %#v, want successful build and unchanged pid %d", snapshot, process.Pid())
	}
	select {
	case spec := <-runner.calls:
		t.Errorf("build-only started another process with spec %#v", spec)
	default:
	}
	select {
	case event := <-order:
		t.Errorf("build-only affected the running app: %q", event)
	default:
	}

	cancel()
	awaitSupervisorRun(t, runResult)
	process.exit(proc.ExitResult{Code: 0})
}

func TestManualStopAndStartUsesLastBuiltOutput(t *testing.T) {
	useSupervisorWorkingDirectory(t)
	if err := os.MkdirAll(filepath.Join(".ember", "bin"), 0o700); err != nil {
		t.Fatal(err)
	}

	order := make(chan string, 8)
	builder := successfulBuilder{calls: make(chan build.Spec, 2), order: order}
	runner := &fakeRunner{calls: make(chan proc.Spec, 2), processes: make(chan *fakeProcess, 2), order: order, nextPID: 800}
	logger := make(recordingLogger, 4)
	supervisor := NewSupervisor(
		builder,
		runner,
		build.Spec{Cmd: "cc -o {out}"},
		proc.Spec{Cmd: "{out} --flag", Cwd: "."},
		1500*time.Millisecond,
		false,
		fakeClock{},
		logger,
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runResult := make(chan error, 1)
	go func() {
		runResult <- supervisor.Run(ctx)
	}()

	supervisor.Send(ReloadRequested{})
	readBuildCall(t, builder.calls)
	if got := readOrder(t, order); got != "build-ok" {
		t.Fatalf("initial build event = %q, want build-ok", got)
	}
	if got := readOrder(t, order); got != "start:800" {
		t.Fatalf("initial runner event = %q, want start:800", got)
	}
	readRunCall(t, runner.calls)
	firstProcess := readFakeProcess(t, runner.processes)
	waitForSupervisorState(t, supervisor, Running)

	supervisor.Send(BuildOnlyRequested{})
	latestBuild := readBuildCall(t, builder.calls)
	if got := readOrder(t, order); got != "build-ok" {
		t.Fatalf("build-only event = %q, want build-ok", got)
	}
	waitForSupervisorState(t, supervisor, Running)
	if snapshot := supervisor.Snapshot(); snapshot.PID != firstProcess.Pid() {
		t.Errorf("snapshot after build-only = %#v, want unchanged pid %d", snapshot, firstProcess.Pid())
	}

	stopReply := make(chan error, 1)
	supervisor.Send(StopRequested{Reply: stopReply})
	select {
	case err := <-stopReply:
		if err != nil {
			t.Fatalf("stop request error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("stop request did not receive a reply")
	}
	if got := readOrder(t, order); got != "stop:800:1.5s" {
		t.Errorf("stop event = %q, want stop:800:1.5s", got)
	}
	waitForSupervisorState(t, supervisor, Idle)
	if snapshot := supervisor.Snapshot(); snapshot.PID != 0 {
		t.Errorf("snapshot after stop = %#v, want pid 0", snapshot)
	}

	startReply := make(chan error, 1)
	supervisor.Send(StartRequested{Reply: startReply})
	select {
	case err := <-startReply:
		if err != nil {
			t.Fatalf("start request error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("start request did not receive a reply")
	}
	if got := readOrder(t, order); got != "start:801" {
		t.Fatalf("start event = %q, want start:801", got)
	}
	runSpec := readRunCall(t, runner.calls)
	output := strings.TrimPrefix(latestBuild.Cmd, "cc -o ")
	if runSpec.Cmd != output+" --flag" {
		t.Errorf("started command = %q, want last successful output %q", runSpec.Cmd, output+" --flag")
	}
	secondProcess := readFakeProcess(t, runner.processes)
	waitForSupervisorState(t, supervisor, Running)
	if snapshot := supervisor.Snapshot(); snapshot.PID != secondProcess.Pid() || snapshot.PID == firstProcess.Pid() {
		t.Errorf("snapshot after start = %#v, want new pid %d", snapshot, secondProcess.Pid())
	}
	select {
	case spec := <-builder.calls:
		t.Errorf("start request unexpectedly rebuilt with spec %#v", spec)
	default:
	}

	cancel()
	awaitSupervisorRun(t, runResult)
	secondProcess.exit(proc.ExitResult{Code: 0})
}

func TestStartWithoutSuccessfulBuildReturnsError(t *testing.T) {
	runner := &fakeRunner{calls: make(chan proc.Spec, 1), processes: make(chan *fakeProcess, 1), order: make(chan string, 1)}
	supervisor := NewSupervisor(nil, runner, build.Spec{}, proc.Spec{Cmd: "{out}"}, 0, false, fakeClock{}, make(recordingLogger, 2))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runResult := make(chan error, 1)
	go func() {
		runResult <- supervisor.Run(ctx)
	}()

	reply := make(chan error, 1)
	supervisor.Send(StartRequested{Reply: reply})
	select {
	case err := <-reply:
		if err == nil || !strings.Contains(err.Error(), "no successfully built output") {
			t.Errorf("start request error = %v, want no-successful-build error", err)
		}
	case <-time.After(time.Second):
		t.Fatal("start request did not receive an error reply")
	}
	if snapshot := supervisor.Snapshot(); snapshot.State != Idle || snapshot.PID != 0 {
		t.Errorf("snapshot after failed start = %#v, want idle without pid", snapshot)
	}
	select {
	case spec := <-runner.calls:
		t.Errorf("start without build called runner with spec %#v", spec)
	default:
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
	runner := &fakeRunner{calls: make(chan proc.Spec, 2), processes: make(chan *fakeProcess, 2), order: order, nextPID: 100}
	logger := make(recordingLogger, 4)
	supervisor := NewSupervisor(
		builder,
		runner,
		build.Spec{Cmd: "cc -o {out}"},
		proc.Spec{Cmd: "{out} --flag", Cwd: "."},
		1500*time.Millisecond,
		false,
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
	firstProcess := readFakeProcess(t, runner.processes)
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
	secondProcess := readFakeProcess(t, runner.processes)
	secondOutput := strings.TrimPrefix(secondBuild.Cmd, "cc -o ")
	if secondRun.Cmd != secondOutput+" --flag" {
		t.Errorf("second run command = %q, want %q", secondRun.Cmd, secondOutput+" --flag")
	}
	if message := readSupervisorLog(t, logger); !strings.Contains(message, "stale child exit") {
		t.Errorf("replaced process exit log = %q, want stale exit ignored", message)
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
	firstProcess.exit(proc.ExitResult{Code: 1})
	secondProcess.exit(proc.ExitResult{Code: 0})
}

func TestChildExitWaitsForNextChangeToRestart(t *testing.T) {
	useSupervisorWorkingDirectory(t)
	if err := os.MkdirAll(filepath.Join(".ember", "bin"), 0o700); err != nil {
		t.Fatal(err)
	}

	order := make(chan string, 8)
	builder := successfulBuilder{calls: make(chan build.Spec, 2), order: order}
	runner := &fakeRunner{calls: make(chan proc.Spec, 2), processes: make(chan *fakeProcess, 2), order: order, nextPID: 400}
	logger := make(recordingLogger, 4)
	supervisor := NewSupervisor(
		builder,
		runner,
		build.Spec{Cmd: "cc -o {out}"},
		proc.Spec{Cmd: "{out}"},
		2*time.Second,
		false,
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
	if got := readOrder(t, order); got != "build-ok" {
		t.Fatalf("first build event = %q, want build-ok", got)
	}
	if got := readOrder(t, order); got != "start:400" {
		t.Fatalf("first runner event = %q, want start:400", got)
	}
	readRunCall(t, runner.calls)
	process := readFakeProcess(t, runner.processes)
	waitForSupervisorState(t, supervisor, Running)

	process.exit(proc.ExitResult{Code: 7})
	if message := readSupervisorLog(t, logger); !strings.Contains(message, "exited with code 7") {
		t.Errorf("child exit log = %q, want exit code 7", message)
	}
	if snapshot := supervisor.Snapshot(); snapshot.State != Idle || snapshot.PID != 0 {
		t.Errorf("snapshot after child exit = %#v, want Idle with no pid", snapshot)
	}
	select {
	case event := <-order:
		t.Errorf("app restarted without a change: %q", event)
	default:
	}
	select {
	case <-runner.calls:
		t.Error("runner started an app without a change")
	default:
	}

	supervisor.Send(FileChanged{Paths: []string{"main.c"}})
	readBuildCall(t, builder.calls)
	if got := readOrder(t, order); got != "build-ok" {
		t.Fatalf("second build event = %q, want build-ok", got)
	}
	if got := readOrder(t, order); got != "start:401" {
		t.Fatalf("second runner event = %q, want start:401 after next change", got)
	}
	readRunCall(t, runner.calls)
	newProcess := readFakeProcess(t, runner.processes)
	waitForSupervisorState(t, supervisor, Running)
	if snapshot := supervisor.Snapshot(); snapshot.PID != 401 {
		t.Errorf("snapshot after next change = %#v, want pid 401", snapshot)
	}

	cancel()
	awaitSupervisorRun(t, runResult)
	newProcess.exit(proc.ExitResult{Code: 0})
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

func readSupervisorLogContaining(t *testing.T, logger <-chan string, wanted string) string {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case message := <-logger:
			if strings.Contains(message, wanted) {
				return message
			}
		case <-deadline:
			t.Fatalf("timed out waiting for supervisor log containing %q", wanted)
			return ""
		}
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

func readFakeProcess(t *testing.T, processes <-chan *fakeProcess) *fakeProcess {
	t.Helper()
	select {
	case process := <-processes:
		return process
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for app process")
		return nil
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
