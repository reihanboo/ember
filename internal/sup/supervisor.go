package sup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/reihanboo/ember/internal/build"
	"github.com/reihanboo/ember/internal/proc"
	"github.com/reihanboo/ember/internal/ui"
)

type Builder interface {
	Build(context.Context) build.Result
}

type Process interface {
	Pid() int
	Wait() proc.ExitResult
	Stop(context.Context, time.Duration) error
}

type Runner interface {
	Start(context.Context) (Process, error)
}

type Clock interface {
	Now() time.Time
}

type Logger interface {
	Debug(string)
}

type Supervisor struct {
	events    chan Event
	builder   Builder
	runner    Runner
	clock     Clock
	logger    Logger
	snapshot  Snapshot
	snapshotM sync.RWMutex
	runM      sync.Mutex
	running   bool
}

type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now()
}

func NewSupervisor(builder Builder, runner Runner, clock Clock, logger Logger) *Supervisor {
	if clock == nil {
		clock = systemClock{}
	}
	if logger == nil {
		logger = ui.NewLogger(os.Stderr, clock.Now, false, ui.DebugLevel)
	}
	return &Supervisor{
		events:  make(chan Event, 32),
		builder: builder,
		runner:  runner,
		clock:   clock,
		logger:  logger,
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
		s.runM.Lock()
		s.running = false
		s.runM.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-s.events:
			s.handle(event)
		}
	}
}

func (s *Supervisor) Snapshot() Snapshot {
	s.snapshotM.RLock()
	defer s.snapshotM.RUnlock()
	return s.snapshot
}

func (s *Supervisor) handle(event Event) {
	s.snapshotM.Lock()
	state := s.snapshot.State
	if change, ok := event.(ChangeEvent); ok {
		changedAt := change.At
		if changedAt.IsZero() {
			changedAt = s.clock.Now()
		}
		s.snapshot.LastChangeTime = changedAt
	}
	s.snapshotM.Unlock()

	s.logger.Debug(fmt.Sprintf("ignored event %+v in state %s", event, state))
}
