package main

import (
	"os/signal"
	"syscall"
)

func ignoreTerm() {
	signal.Ignore(syscall.SIGTERM)
	sleepForever()
}
