package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fail("missing mode")
	}

	switch os.Args[1] {
	case "exit":
		if len(os.Args) != 3 {
			fail("exit mode requires a code")
		}
		code, err := strconv.Atoi(os.Args[2])
		if err != nil {
			fail(err.Error())
		}
		os.Exit(code)
	case "sleep":
		fmt.Println(os.Getpid())
		sleepForever()
	case "spawn-child-sleep":
		spawnChildSleep()
	case "ignore-term":
		ignoreTerm()
	case "handle-break":
		handleBreak()
	default:
		fail("unknown mode: " + os.Args[1])
	}
}

func spawnChildSleep() {
	executable, err := os.Executable()
	if err != nil {
		fail(err.Error())
	}
	child := exec.Command(executable, "sleep")
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	if err := child.Start(); err != nil {
		fail(err.Error())
	}
	go child.Wait()
	sleepForever()
}

func sleepForever() {
	for {
		time.Sleep(time.Hour)
	}
}

func handleBreak() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	fmt.Println(os.Getpid())
	<-signals
	os.Exit(23)
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}
