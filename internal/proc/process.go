package proc

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
)

type Spec struct {
	Cmd string
	Cwd string
	Env map[string]string
}

type ExitResult struct {
	Code   int
	Signal os.Signal
	Err    error
}

type Process struct {
	command  *exec.Cmd
	waitOnce sync.Once
	result   ExitResult
}

func Start(ctx context.Context, spec Spec) (*Process, error) {
	command := commandWithContext(ctx, spec.Cmd, spec.Cwd, spec.Env)
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start command %q: %w", spec.Cmd, err)
	}
	return &Process{command: command}, nil
}

func (p *Process) Wait() ExitResult {
	p.waitOnce.Do(func() {
		err := p.command.Wait()
		p.result = ExitResult{Code: -1, Err: err}
		if state := p.command.ProcessState; state != nil {
			p.result.Code = state.ExitCode()
			p.result.Signal = processSignal(state)
		}
	})
	return p.result
}

func (p *Process) Pid() int {
	if p.command.Process == nil {
		return 0
	}
	return p.command.Process.Pid
}
