package proc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

type Spec struct {
	Cmd    string
	Cwd    string
	Env    map[string]string
	Output io.Writer
	Tag    string
}

type ExitResult struct {
	Code   int
	Signal os.Signal
	Err    error
}

type Process struct {
	command  *exec.Cmd
	outputs  []*lineWriter
	waitOnce sync.Once
	result   ExitResult
}

func Start(ctx context.Context, spec Spec) (*Process, error) {
	output := spec.Output
	if output == nil {
		output = io.Discard
	}
	var outputMu sync.Mutex
	stdout := newLineWriter(output, spec.Tag, &outputMu)
	stderr := newLineWriter(output, spec.Tag, &outputMu)
	command := commandWithContext(ctx, spec.Cmd, spec.Cwd, spec.Env)
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start command %q: %w", spec.Cmd, err)
	}
	return &Process{command: command, outputs: []*lineWriter{stdout, stderr}}, nil
}

func (p *Process) Wait() ExitResult {
	p.waitOnce.Do(func() {
		err := p.command.Wait()
		for index, output := range p.outputs {
			if flushErr := output.Flush(); flushErr != nil {
				stream := "stdout"
				if index == 1 {
					stream = "stderr"
				}
				err = errors.Join(err, fmt.Errorf("flush %s: %w", stream, flushErr))
			}
		}
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
