package build

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"time"

	"github.com/reihanboo/ember/internal/proc"
)

type Spec struct {
	Cmd    string
	Cwd    string
	Env    map[string]string
	Output io.Writer
}

type Result struct {
	Success   bool
	Cancelled bool
	Duration  time.Duration
	ExitCode  int
	Err       error
}

func Run(ctx context.Context, spec Spec) Result {
	started := time.Now()
	if err := ctx.Err(); err != nil {
		return Result{Cancelled: true, Duration: time.Since(started), ExitCode: -1, Err: err}
	}
	if hint := commandHint(spec.Cmd, exec.LookPath, runtime.GOOS == "windows"); hint != "" {
		output := spec.Output
		if output == nil {
			output = io.Discard
		}
		if _, err := fmt.Fprintf(output, "[build] %s\n", hint); err != nil {
			return Result{Duration: time.Since(started), ExitCode: -1, Err: fmt.Errorf("write command hint: %w", err)}
		}
		return Result{Duration: time.Since(started), ExitCode: 127, Err: errors.New(hint)}
	}
	process, err := proc.Start(context.Background(), proc.Spec{
		Cmd:    spec.Cmd,
		Cwd:    spec.Cwd,
		Env:    spec.Env,
		Output: spec.Output,
		Tag:    "build",
	})
	if err != nil {
		return Result{
			Cancelled: ctx.Err() != nil,
			Duration:  time.Since(started),
			ExitCode:  -1,
			Err:       errors.Join(err, ctx.Err()),
		}
	}

	exitResults := make(chan proc.ExitResult, 1)
	go func() {
		exitResults <- process.Wait()
	}()

	select {
	case exit := <-exitResults:
		return Result{
			Success:  exit.Code == 0 && exit.Err == nil,
			Duration: time.Since(started),
			ExitCode: exit.Code,
			Err:      exit.Err,
		}
	case <-ctx.Done():
		stopErr := process.Stop(ctx, 0)
		exit := <-exitResults
		return Result{
			Cancelled: true,
			Duration:  time.Since(started),
			ExitCode:  exit.Code,
			Err:       errors.Join(exit.Err, stopErr),
		}
	}
}
