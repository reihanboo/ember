package build

import (
	"context"
	"errors"
	"io"
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
