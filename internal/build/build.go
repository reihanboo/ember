package build

import (
	"context"
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
	Success  bool
	Duration time.Duration
	ExitCode int
	Err      error
}

func Run(ctx context.Context, spec Spec) Result {
	started := time.Now()
	process, err := proc.Start(ctx, proc.Spec{
		Cmd:    spec.Cmd,
		Cwd:    spec.Cwd,
		Env:    spec.Env,
		Output: spec.Output,
		Tag:    "build",
	})
	if err != nil {
		return Result{Duration: time.Since(started), ExitCode: -1, Err: err}
	}

	exit := process.Wait()
	return Result{
		Success:  exit.Code == 0 && exit.Err == nil,
		Duration: time.Since(started),
		ExitCode: exit.Code,
		Err:      exit.Err,
	}
}
