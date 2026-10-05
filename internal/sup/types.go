package sup

import (
	"time"

	"github.com/reihanboo/ember/internal/build"
	"github.com/reihanboo/ember/internal/proc"
)

type Event interface {
	isEvent()
}

type ChangeEvent struct {
	Paths []string
	At    time.Time
}

type ControlEvent struct {
	Command string
}

type KeyEvent struct {
	Key rune
}

type ChildExitEvent struct {
	PID    int
	Result proc.ExitResult
}

type BuildFinishedEvent struct {
	Result build.Result
}

func (ChangeEvent) isEvent()        {}
func (ControlEvent) isEvent()       {}
func (KeyEvent) isEvent()           {}
func (ChildExitEvent) isEvent()     {}
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
