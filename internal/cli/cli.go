package cli

import (
	"errors"
	"fmt"
	"io"
)

type command struct {
	name    string
	handler func() error
}

var commands = []command{
	{name: "run", handler: runCommand},
	{name: "reload", handler: reloadCommand},
	{name: "build", handler: buildCommand},
	{name: "stop", handler: stopCommand},
	{name: "start", handler: startCommand},
	{name: "status", handler: statusCommand},
	{name: "init", handler: initCommand},
}

func Run(args []string, stdout, stderr io.Writer) int {
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
			if err := item.handler(); err != nil {
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

func runCommand() error {
	return errors.New("not implemented")
}

func reloadCommand() error {
	return errors.New("not implemented")
}

func buildCommand() error {
	return errors.New("not implemented")
}

func stopCommand() error {
	return errors.New("not implemented")
}

func startCommand() error {
	return errors.New("not implemented")
}

func statusCommand() error {
	return errors.New("not implemented")
}

func initCommand() error {
	return errors.New("not implemented")
}
