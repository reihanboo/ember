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
	events        chan Event
	builder       Builder
	runner        Runner
	clock         Clock
	logger        Logger
	buildSpec     build.Spec
	runSpec       proc.Spec
	killTimeout   time.Duration
	process       Process
	buildOutput   string
	currentOutput string
	buildID       uint64
	buildCancel   context.CancelFunc
	snapshot      Snapshot
	snapshotM     sync.RWMutex
	runM          sync.Mutex
	running       bool
}

type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now()
}

func NewSupervisor(builder Builder, runner Runner, buildSpec build.Spec, runSpec proc.Spec, killTimeout time.Duration, clock Clock, logger Logger) *Supervisor {
	if clock == nil {
		clock = systemClock{}
	}
	if logger == nil {
		logger = ui.NewLogger(os.Stderr, clock.Now, false, ui.DebugLevel)
	}
	return &Supervisor{
		events:      make(chan Event, 32),
		builder:     builder,
		runner:      runner,
		clock:       clock,
		logger:      logger,
		buildSpec:   buildSpec,
		runSpec:     runSpec,
		killTimeout: killTimeout,
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
	case FileChanged, ReloadRequested:
		if state == Building {
			s.startBuild(ctx)
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
		s.startBuild(ctx)
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

func (s *Supervisor) startBuild(ctx context.Context) {
	s.cancelBuild()
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

	go func() {
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
		s.logger.Error(fmt.Sprintf("build failed: %v", failure))
		return
	}
	s.restartApp(ctx)
}

func (s *Supervisor) restartApp(ctx context.Context) {
	if s.runner == nil {
		s.setState(BuildFailed)
		s.logger.Debug("cannot start app without a runner")
		return
	}

	previousOutput := s.currentOutput
	if s.process != nil {
		if err := s.process.Stop(ctx, s.killTimeout); err != nil {
			s.setState(Running)
			s.logger.Debug(fmt.Sprintf("stop old app: %v", err))
			return
		}
		s.process = nil
		s.snapshotM.Lock()
		s.snapshot.PID = 0
		s.snapshotM.Unlock()
	}

	runSpec := s.runSpec
	runSpec.Cmd = outdir.Substitute(runSpec.Cmd, s.buildOutput)
	process, err := s.runner.Start(ctx, runSpec)
	if err != nil {
		s.setState(BuildFailed)
		s.logger.Debug(fmt.Sprintf("start new app: %v", err))
		return
	}
	if process == nil {
		s.setState(BuildFailed)
		s.logger.Debug("runner returned no process")
		return
	}

	s.process = process
	s.currentOutput = s.buildOutput
	s.snapshotM.Lock()
	s.snapshot.State = Running
	s.snapshot.PID = process.Pid()
	s.snapshotM.Unlock()

	keep := []string{s.currentOutput}
	if previousOutput != "" && previousOutput != s.currentOutput {
		keep = append(keep, previousOutput)
	}
	if err := outdir.Prune(keep...); err != nil {
		s.logger.Debug(fmt.Sprintf("prune old outputs: %v", err))
	}
}

func (s *Supervisor) setState(state State) {
	s.snapshotM.Lock()
	s.snapshot.State = state
	s.snapshotM.Unlock()
}
