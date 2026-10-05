package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/reihanboo/ember/internal/proc"
)

type command struct {
	name    string
	handler func(context.Context, *proc.Registry) error
}

var commands = []command{
	{name: "run", handler: runCommand},
	{name: "reload", handler: reloadCommand},
	{name: "build", handler: buildCommand},
	{name: "stop", handler: stopCommand},
	{name: "start", handler: startCommand},
	{name: "status", handler: statusCommand},
	{name: "init", handler: func(context.Context, *proc.Registry) error { return initCommand() }},
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
			if err := item.handler(ctx, registry); err != nil {
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

func runCommand(context.Context, *proc.Registry) error {
	return errors.New("not implemented")
}

func reloadCommand(context.Context, *proc.Registry) error {
	return errors.New("not implemented")
}

func buildCommand(context.Context, *proc.Registry) error {
	return errors.New("not implemented")
}

func stopCommand(context.Context, *proc.Registry) error {
	return errors.New("not implemented")
}

func startCommand(context.Context, *proc.Registry) error {
	return errors.New("not implemented")
}

func statusCommand(context.Context, *proc.Registry) error {
	return errors.New("not implemented")
}
