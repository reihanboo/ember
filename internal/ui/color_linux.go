package ui

import (
	"os"

	"golang.org/x/term"
)

func consoleSupportsColor() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}
