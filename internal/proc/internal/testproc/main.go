package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
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
		select {}
	case "spawn-child-sleep":
		spawnChildSleep()
	case "ignore-term":
		fmt.Println(os.Getpid())
		ignoreTerm()
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
	select {}
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}
