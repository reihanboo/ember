package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/reihanboo/ember/internal/cli"
	"github.com/reihanboo/ember/internal/proc"
)

var version = "dev"

func main() {
	if writeVersion(os.Args[1:], os.Stdout) {
		return
	}
	ctx, stopSignals := newSignalContext()
	registry := &proc.Registry{}
	status := runWithShutdown(ctx, registry, func(ctx context.Context, registry *proc.Registry) int {
		return cli.RunContext(ctx, registry, os.Args[1:], os.Stdout, os.Stderr)
	}, os.Stderr)
	stopSignals()
	os.Exit(status)
}

func writeVersion(args []string, stdout io.Writer) bool {
	if len(args) != 1 || args[0] != "--version" {
		return false
	}
	fmt.Fprintf(stdout, "ember %s\n", version)
	return true
}

func runWithShutdown(ctx context.Context, registry *proc.Registry, run func(context.Context, *proc.Registry) int, stderr io.Writer) int {
	if registry == nil {
		registry = &proc.Registry{}
	}
	status := run(ctx, registry)
	if err := registry.StopAll(context.Background(), 2*time.Second); err != nil {
		fmt.Fprintln(stderr, err)
		if status == 0 {
			return 1
		}
	}
	return status
}
