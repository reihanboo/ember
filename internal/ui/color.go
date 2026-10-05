package ui

import "os"

func EnableColor() bool {
	if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
		return false
	}
	return consoleSupportsColor()
}
