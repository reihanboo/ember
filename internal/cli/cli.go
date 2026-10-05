package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/reihanboo/ember/internal/build"
	"github.com/reihanboo/ember/internal/config"
	"github.com/reihanboo/ember/internal/ctl"
	"github.com/reihanboo/ember/internal/proc"
	"github.com/reihanboo/ember/internal/sup"
	"github.com/reihanboo/ember/internal/ui"
	"github.com/reihanboo/ember/internal/watcher"
)

type command struct {
	name    string
	handler func(context.Context, *proc.Registry, io.Writer, io.Writer) error
}

var commands = []command{
	{name: "run", handler: runCommand},
	{name: "reload", handler: reloadCommand},
	{name: "build", handler: buildCommand},
	{name: "stop", handler: stopCommand},
	{name: "start", handler: startCommand},
	{name: "status", handler: statusCommand},
	{name: "init", handler: func(context.Context, *proc.Registry, io.Writer, io.Writer) error { return initCommand() }},
}

func Run(args []string, stdout, stderr io.Writer) int {
	return RunContext(context.Background(), nil, args, stdout, stderr)
}

func RunContext(ctx context.Context, registry *proc.Registry, args []string, stdout, stderr io.Writer) int {
	if ctx == nil {
		ctx = context.Background()
	}
	if registry == nil {
		registry = &proc.Registry{}
	}
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	if len(args) == 0 {
		printUsage(stderr)
		return 1
	}
	if args[0] == "-h" {
		printUsage(stdout)
		return 0
	}
	for _, item := range commands {
		if args[0] == item.name {
			if err := item.handler(ctx, registry, stdout, stderr); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			return 0
		}
	}
	printUsage(stderr)
	return 1
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: ember <command>")
	fmt.Fprintln(w, "\nCommands:")
	for _, item := range commands {
		fmt.Fprintf(w, "  %s\n", item.name)
	}
}

func runCommand(ctx context.Context, registry *proc.Registry, stdout, stderr io.Writer) (returnErr error) {
	if registry == nil {
		registry = &proc.Registry{}
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	root, configPath, err := config.Find(workingDirectory)
	if err != nil {
		return err
	}
	settings, err := config.Load(configPath)
	if err != nil {
		return err
	}
	if err := os.Chdir(root); err != nil {
		return fmt.Errorf("change to project root %q: %w", root, err)
	}
	defer func() {
		if err := os.Chdir(workingDirectory); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("restore working directory %q: %w", workingDirectory, err))
		}
	}()

	if err := os.MkdirAll(filepath.Join(root, ".ember", "bin"), 0o700); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	projectWatcher, err := watcher.NewWatcher(root, settings.Watch)
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	defer func() {
		if err := projectWatcher.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close watcher: %w", err))
		}
	}()

	killTimeout := time.Duration(settings.Run.KillTimeoutMS) * time.Millisecond
	runCwd := settings.Run.Cwd
	if runCwd == "" {
		runCwd = "."
	}
	if !filepath.IsAbs(runCwd) {
		runCwd = filepath.Join(root, runCwd)
	}
	logger := ui.NewLogger(stderr, time.Now, settings.UI.Color, ui.InfoLevel)
	supervisor := sup.NewSupervisor(
		commandBuilder{},
		appRunner{registry: registry},
		build.Spec{Cmd: settings.Build.Cmd, Cwd: root, Output: stdout},
		proc.Spec{Cmd: settings.Run.Cmd, Cwd: runCwd, Env: settings.Run.Env, Output: stdout, Tag: "app"},
		killTimeout,
		settings.Build.StopsRunning,
		nil,
		logger,
	)
	controlServer, err := ctl.StartServer(filepath.Join(root, ".ember", "ctl"), ctl.NewDispatcher(supervisor))
	if err != nil {
		return fmt.Errorf("start control server: %w", err)
	}
	defer func() {
		if err := controlServer.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close control server: %w", err))
		}
	}()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	changes := projectWatcher.Watch(runCtx, time.Duration(settings.Watch.DebounceMS)*time.Millisecond)
	forwardingDone := make(chan struct{})
	go func() {
		defer close(forwardingDone)
		for {
			select {
			case <-runCtx.Done():
				return
			case change, ok := <-changes:
				if !ok {
					return
				}
				supervisor.Send(sup.FileChanged{Paths: change.Paths})
			}
		}
	}()

	supervisorDone := make(chan error, 1)
	go func() {
		supervisorDone <- supervisor.Run(runCtx)
	}()
	supervisor.Send(sup.ReloadRequested{})
	select {
	case err := <-supervisorDone:
		returnErr = err
	case <-runCtx.Done():
		returnErr = <-supervisorDone
	}
	cancel()
	<-forwardingDone
	if err := registry.StopAll(context.Background(), killTimeout); err != nil {
		returnErr = errors.Join(returnErr, fmt.Errorf("stop app: %w", err))
	}
	return returnErr
}

type commandBuilder struct{}

func (commandBuilder) Build(ctx context.Context, spec build.Spec) build.Result {
	return build.Run(ctx, spec)
}

type appRunner struct {
	registry *proc.Registry
}

func (runner appRunner) Start(_ context.Context, spec proc.Spec) (sup.Process, error) {
	process, err := proc.Start(context.Background(), spec)
	if err != nil {
		return nil, err
	}
	if err := runner.registry.Register(process); err != nil {
		stopErr := process.Stop(context.Background(), 0)
		return nil, errors.Join(fmt.Errorf("register app process: %w", err), stopErr)
	}
	return &registeredProcess{process: process, registry: runner.registry}, nil
}

type registeredProcess struct {
	process  *proc.Process
	registry *proc.Registry
	waitOnce sync.Once
}

func (process *registeredProcess) Pid() int {
	return process.process.Pid()
}

func (process *registeredProcess) Wait() proc.ExitResult {
	result := process.process.Wait()
	process.waitOnce.Do(func() {
		process.registry.Unregister(process.process)
	})
	return result
}

func (process *registeredProcess) Stop(ctx context.Context, timeout time.Duration) error {
	return process.process.Stop(ctx, timeout)
}

func reloadCommand(context.Context, *proc.Registry, io.Writer, io.Writer) error {
	return errors.New("not implemented")
}

func buildCommand(context.Context, *proc.Registry, io.Writer, io.Writer) error {
	return errors.New("not implemented")
}

func stopCommand(context.Context, *proc.Registry, io.Writer, io.Writer) error {
	return errors.New("not implemented")
}

func startCommand(context.Context, *proc.Registry, io.Writer, io.Writer) error {
	return errors.New("not implemented")
}

func statusCommand(ctx context.Context, _ *proc.Registry, stdout, _ io.Writer) error {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	root, _, err := config.Find(workingDirectory)
	if err != nil {
		return err
	}
	lines, err := ctl.SendRequest(ctx, filepath.Join(root, ".ember", "ctl"), ctl.RequestStatus)
	if err != nil {
		return err
	}
	status, err := renderStatus(lines)
	if err != nil {
		return fmt.Errorf("render control status: %w", err)
	}
	for _, line := range status {
		if _, err := fmt.Fprintln(stdout, line); err != nil {
			return fmt.Errorf("write status: %w", err)
		}
	}
	return nil
}
