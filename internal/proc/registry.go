package proc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Registry struct {
	mu        sync.Mutex
	processes map[*Process]struct{}
	stopping  bool
}

func (r *Registry) Register(process *Process) error {
	if process == nil {
		return errors.New("cannot register a nil process")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopping {
		return errors.New("process registry is stopping")
	}
	if r.processes == nil {
		r.processes = make(map[*Process]struct{})
	}
	r.processes[process] = struct{}{}
	return nil
}

func (r *Registry) Unregister(process *Process) {
	r.mu.Lock()
	delete(r.processes, process)
	r.mu.Unlock()
}

func (r *Registry) StopAll(ctx context.Context, timeout time.Duration) error {
	r.mu.Lock()
	r.stopping = true
	processes := make([]*Process, 0, len(r.processes))
	for process := range r.processes {
		processes = append(processes, process)
	}
	r.mu.Unlock()

	var stopErrors []error
	for _, process := range processes {
		if err := process.Stop(ctx, timeout); err != nil {
			stopErrors = append(stopErrors, fmt.Errorf("stop process %d: %w", process.Pid(), err))
			continue
		}
		r.Unregister(process)
	}
	return errors.Join(stopErrors...)
}
