package sup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/reihanboo/ember/internal/build"
	"github.com/reihanboo/ember/internal/outdir"
	"github.com/reihanboo/ember/internal/proc"
	"github.com/reihanboo/ember/internal/ui"
)

type Builder interface {
	Build(context.Context, build.Spec) build.Result
}

type Process interface {
	Pid() int
	Wait() proc.ExitResult
	Stop(context.Context, time.Duration) error
}

type Runner interface {
	Start(context.Context, proc.Spec) (Process, error)
}

type Clock interface {
	Now() time.Time
}

type Logger interface {
	Debug(string)
	Error(string)
}

type Supervisor struct {
	events          chan Event
	builder         Builder
	runner          Runner
	clock           Clock
	logger          Logger
	buildSpec       build.Spec
	runSpec         proc.Spec
	killTimeout     time.Duration
	stopsRunning    bool
	process         Process
	buildOutput     string
	currentOutput   string
	lastBuiltOutput string
	buildID         uint64
	buildCancel     context.CancelFunc
	buildOnly       bool
	snapshot        Snapshot
	snapshotM       sync.RWMutex
	buildWG         sync.WaitGroup
	runM            sync.Mutex
	running         bool
}

type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now()
}

func NewSupervisor(builder Builder, runner Runner, buildSpec build.Spec, runSpec proc.Spec, killTimeout time.Duration, stopsRunning bool, clock Clock, logger Logger) *Supervisor {
	if clock == nil {
		clock = systemClock{}
	}
	if logger == nil {
		logger = ui.NewLogger(os.Stderr, clock.Now, false, ui.DebugLevel)
	}
	return &Supervisor{
		events:       make(chan Event, 32),
		builder:      builder,
		runner:       runner,
		clock:        clock,
		logger:       logger,
		buildSpec:    buildSpec,
		runSpec:      runSpec,
		killTimeout:  killTimeout,
		stopsRunning: stopsRunning,
		snapshot: Snapshot{
			State: Idle,
		},
	}
}

func (s *Supervisor) Send(event Event) {
	s.events <- event
}

func (s *Supervisor) Run(ctx context.Context) error {
	s.runM.Lock()
	if s.running {
		s.runM.Unlock()
		return errors.New("supervisor is already running")
	}
	s.running = true
	s.runM.Unlock()
	defer func() {
		s.cancelBuild()
		s.buildWG.Wait()
		s.runM.Lock()
		s.running = false
		s.runM.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-s.events:
			s.handle(ctx, event)
		}
	}
}

func (s *Supervisor) Snapshot() Snapshot {
	s.snapshotM.RLock()
	defer s.snapshotM.RUnlock()
	return s.snapshot
}

func (s *Supervisor) handle(ctx context.Context, event Event) {
	if change, ok := event.(FileChanged); ok {
		changedAt := change.At
		if changedAt.IsZero() {
			changedAt = s.clock.Now()
		}
		s.snapshotM.Lock()
		s.snapshot.LastChangeTime = changedAt
		s.snapshotM.Unlock()
	}

	state := s.Snapshot().State
	switch event.(type) {
	case FileChanged, ReloadRequested, BuildOnlyRequested:
		_, buildOnly := event.(BuildOnlyRequested)
		if state == Building {
			s.startBuild(ctx, buildOnly)
			return
		}
		if state != Idle && state != Running && state != BuildFailed {
			s.logger.Debug(fmt.Sprintf("ignored event %T %+v in state %s", event, event, state))
			return
		}
		if s.builder == nil {
			s.logger.Debug(fmt.Sprintf("ignored build trigger %T %+v without a builder", event, event))
			return
		}
		if !buildOnly && state == Running && s.stopsRunning {
			if err := s.stopApp(ctx); err != nil {
				s.logger.Error(fmt.Sprintf("stop app before build: %v", err))
				return
			}
		}
		s.startBuild(ctx, buildOnly)
	case StopRequested:
		requested := event.(StopRequested)
		if state == Building {
			s.cancelBuild()
			s.buildOnly = false
		}
		err := s.stopApp(ctx)
		if err != nil {
			err = fmt.Errorf("stop app: %w", err)
		}
		if err == nil {
			s.setState(Idle)
		} else if s.process != nil {
			s.setState(Running)
		}
		if requested.Reply != nil {
			requested.Reply <- err
		}
	case StartRequested:
		requested := event.(StartRequested)
		err := s.startLastBuiltApp(ctx)
		if requested.Reply != nil {
			requested.Reply <- err
		}
	case ChildExited:
		exited := event.(ChildExited)
		currentPID := 0
		if s.process != nil {
			currentPID = s.process.Pid()
		}
		if currentPID == 0 || exited.PID != currentPID {
			s.logger.Debug(fmt.Sprintf("ignored stale child exit for pid %d; current pid is %d", exited.PID, currentPID))
			return
		}
		s.process = nil
		s.snapshotM.Lock()
		s.snapshot.PID = 0
		if s.snapshot.State != Building {
			s.snapshot.State = Idle
		}
		s.snapshotM.Unlock()
		s.logger.Debug(fmt.Sprintf("app process %d exited with code %d", exited.PID, exited.Result.Code))
	case BuildFinishedEvent:
		finished := event.(BuildFinishedEvent)
		if finished.BuildID != s.buildID {
			s.logger.Debug(fmt.Sprintf("discarded stale build result %d; active build is %d", finished.BuildID, s.buildID))
			return
		}
		if state != Building {
			s.logger.Debug(fmt.Sprintf("ignored event %T %+v in state %s", event, event, state))
			return
		}
		s.cancelBuild()
		s.finishBuild(ctx, finished.Result)
	default:
		s.logger.Debug(fmt.Sprintf("ignored event %T %+v in state %s", event, event, state))
	}
}

func (s *Supervisor) startBuild(ctx context.Context, buildOnly bool) {
	s.cancelBuild()
	s.buildOnly = buildOnly
	s.buildID++
	buildID := s.buildID
	buildCtx, cancel := context.WithCancel(ctx)
	s.buildCancel = cancel

	output := outdir.Next()
	spec := s.buildSpec
	spec.Cmd = outdir.Substitute(spec.Cmd, output)
	s.buildOutput = output

	s.snapshotM.Lock()
	s.snapshot.State = Building
	s.snapshotM.Unlock()

	s.buildWG.Add(1)
	go func() {
		defer s.buildWG.Done()
		result := s.builder.Build(buildCtx, spec)
		s.Send(BuildFinishedEvent{BuildID: buildID, Result: result})
	}()
}

func (s *Supervisor) cancelBuild() {
	if s.buildCancel == nil {
		return
	}
	s.buildCancel()
	s.buildCancel = nil
}

func (s *Supervisor) finishBuild(ctx context.Context, result build.Result) {
	buildOnly := s.buildOnly
	s.buildOnly = false
	s.snapshotM.Lock()
	s.snapshot.LastBuildDuration = result.Duration
	s.snapshot.LastBuildOK = result.Success && !result.Cancelled
	s.snapshotM.Unlock()

	if result.Cancelled {
		state := Idle
		if s.process != nil {
			state = Running
		}
		s.setState(state)
		return
	}
	if !result.Success {
		s.setState(BuildFailed)
		failure := result.Err
		if failure == nil {
			failure = fmt.Errorf("exit code %d", result.ExitCode)
		}
		if s.stopsRunning && s.process == nil {
			s.logger.Error(fmt.Sprintf("build failed: %v; app remains stopped because stops_running is enabled", failure))
			return
		}
		s.logger.Error(fmt.Sprintf("build failed: %v", failure))
		return
	}
	s.lastBuiltOutput = s.buildOutput
	if buildOnly {
		state := Idle
		if s.process != nil {
			state = Running
		}
		s.setState(state)
		return
	}
	s.restartApp(ctx)
}

func (s *Supervisor) restartApp(ctx context.Context) {
	previousOutput := s.currentOutput
	if err := s.stopApp(ctx); err != nil {
		s.setState(Running)
		s.logger.Debug(fmt.Sprintf("stop old app: %v", err))
		return
	}
	if err := s.startApp(ctx, s.buildOutput); err != nil {
		s.setState(BuildFailed)
		s.logger.Debug(err.Error())
		return
	}
	s.pruneOutputs(previousOutput)
}

func (s *Supervisor) startLastBuiltApp(ctx context.Context) error {
	if s.process != nil {
		return nil
	}
	if s.lastBuiltOutput == "" {
		return errors.New("no successfully built output is available")
	}
	previousOutput := s.currentOutput
	state := s.Snapshot().State
	if err := s.startApp(ctx, s.lastBuiltOutput); err != nil {
		s.setState(BuildFailed)
		return err
	}
	if state == Building {
		s.setState(Building)
	}
	s.pruneOutputs(previousOutput)
	return nil
}

func (s *Supervisor) startApp(ctx context.Context, output string) error {
	if s.runner == nil {
		return errors.New("cannot start app without a runner")
	}
	runSpec := s.runSpec
	runSpec.Cmd = outdir.Substitute(runSpec.Cmd, output)
	process, err := s.runner.Start(ctx, runSpec)
	if err != nil {
		return fmt.Errorf("start app: %w", err)
	}
	if process == nil {
		return errors.New("runner returned no process")
	}

	pid := process.Pid()
	s.process = process
	s.currentOutput = output
	s.snapshotM.Lock()
	s.snapshot.State = Running
	s.snapshot.PID = pid
	s.snapshotM.Unlock()
	go func() {
		result := process.Wait()
		s.Send(ChildExited{PID: pid, Result: result})
	}()
	return nil
}

func (s *Supervisor) pruneOutputs(previousOutput string) {
	keep := []string{s.currentOutput}
	if previousOutput != "" && previousOutput != s.currentOutput {
		keep = append(keep, previousOutput)
	}
	if err := outdir.Prune(keep...); err != nil {
		s.logger.Debug(fmt.Sprintf("prune old outputs: %v", err))
	}
}

func (s *Supervisor) stopApp(ctx context.Context) error {
	if s.process == nil {
		return nil
	}
	if err := s.process.Stop(ctx, s.killTimeout); err != nil {
		return err
	}
	s.process = nil
	s.snapshotM.Lock()
	s.snapshot.PID = 0
	s.snapshotM.Unlock()
	return nil
}

func (s *Supervisor) setState(state State) {
	s.snapshotM.Lock()
	s.snapshot.State = state
	s.snapshotM.Unlock()
}
