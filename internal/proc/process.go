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
	command   *exec.Cmd
	outputs   []*lineWriter
	waitOnce  sync.Once
	jobMu     sync.Mutex
	jobHandle uintptr
	jobHeld   bool
	result    ExitResult
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
	jobHandle, err := createProcessJob()
	if err != nil {
		return nil, fmt.Errorf("create process job: %w", err)
	}
	if err := command.Start(); err != nil {
		closeErr := closeProcessJob(jobHandle)
		return nil, errors.Join(fmt.Errorf("start command %q: %w", spec.Cmd, err), closeErr)
	}
	if err := assignProcessToJob(jobHandle, command.Process.Pid); err != nil {
		command.Process.Kill()
		command.Wait()
		closeProcessJob(jobHandle)
		return nil, fmt.Errorf("assign process %d to job: %w", command.Process.Pid, err)
	}
	return &Process{command: command, outputs: []*lineWriter{stdout, stderr}, jobHandle: jobHandle}, nil
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
		if closeErr := p.closeJob(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close process job: %w", closeErr))
		}
		p.result = ExitResult{Code: -1, Err: err}
		if state := p.command.ProcessState; state != nil {
			p.result.Code = state.ExitCode()
			p.result.Signal = processSignal(state)
		}
	})
	return p.result
}

func (p *Process) closeJob() error {
	p.jobMu.Lock()
	defer p.jobMu.Unlock()
	if p.jobHeld || p.jobHandle == 0 {
		return nil
	}
	if err := closeProcessJob(p.jobHandle); err != nil {
		return err
	}
	p.jobHandle = 0
	return nil
}

func (p *Process) releaseJob() error {
	p.jobMu.Lock()
	defer p.jobMu.Unlock()
	p.jobHeld = false
	if p.jobHandle == 0 {
		return nil
	}
	if err := closeProcessJob(p.jobHandle); err != nil {
		return err
	}
	p.jobHandle = 0
	return nil
}

func (p *Process) Pid() int {
	if p.command.Process == nil {
		return 0
	}
	return p.command.Process.Pid
}
