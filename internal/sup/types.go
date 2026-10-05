package sup

import (
	"time"

	"github.com/reihanboo/ember/internal/build"
	"github.com/reihanboo/ember/internal/proc"
)

type Event interface {
	isEvent()
}

type FileChanged struct {
	Paths []string
	At    time.Time
}

type ChangeEvent = FileChanged

type ReloadRequested struct{}

type BuildOnlyRequested struct{}

type StopRequested struct {
	Reply chan error
}

type StartRequested struct {
	Reply chan error
}

type StatusRequested struct {
	Reply chan Snapshot
}

type ControlEvent struct {
	Command string
}

type KeyEvent struct {
	Key rune
}

type ChildExited struct {
	PID    int
	Result proc.ExitResult
}

type ChildExitEvent = ChildExited

type BuildFinishedEvent struct {
	BuildID uint64
	Result  build.Result
}

func (FileChanged) isEvent()        {}
func (ReloadRequested) isEvent()    {}
func (BuildOnlyRequested) isEvent() {}
func (StopRequested) isEvent()      {}
func (StartRequested) isEvent()     {}
func (StatusRequested) isEvent()    {}
func (ControlEvent) isEvent()       {}
func (KeyEvent) isEvent()           {}
func (ChildExited) isEvent()        {}
func (BuildFinishedEvent) isEvent() {}

type State uint8

const (
	Idle State = iota
	Building
	Running
	BuildFailed
)

func (state State) String() string {
	switch state {
	case Idle:
		return "Idle"
	case Building:
		return "Building"
	case Running:
		return "Running"
	case BuildFailed:
		return "BuildFailed"
	default:
		return "Unknown"
	}
}

type Snapshot struct {
	State             State
	PID               int
	LastBuildDuration time.Duration
	LastBuildOK       bool
	LastChangeTime    time.Time
}
