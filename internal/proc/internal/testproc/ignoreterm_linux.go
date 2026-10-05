package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func ignoreTerm() {
	signal.Ignore(syscall.SIGTERM)
	fmt.Println(os.Getpid())
	sleepForever()
}
