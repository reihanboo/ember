package proc

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAbruptParentKillsChild(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestAbruptParentHelper$", "-test.v")
	command.Env = append(os.Environ(), "EMBER_PROC_PARENT_HELPER=1")
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = io.Discard
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	parentExited := false
	t.Cleanup(func() {
		if !parentExited {
			command.Process.Kill()
			command.Wait()
		}
	})

	pidResult := make(chan struct {
		pid int
		err error
	}, 1)
	go func() {
		reader := bufio.NewReader(stdout)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				pidResult <- struct {
					pid int
					err error
				}{err: err}
				return
			}
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "EMBER_PROC_CHILD_PID:") {
				continue
			}
			pid, err := strconv.Atoi(strings.TrimPrefix(line, "EMBER_PROC_CHILD_PID:"))
			pidResult <- struct {
				pid int
				err error
			}{pid: pid, err: err}
			return
		}
	}()

	var childPID int
	select {
	case result := <-pidResult:
		if result.err != nil {
			t.Fatalf("read child pid: %v", result.err)
		}
		childPID = result.pid
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for child pid")
	}
	if err := command.Process.Kill(); err != nil {
		t.Fatalf("kill parent helper: %v", err)
	}
	command.Wait()
	parentExited = true
	if err := waitForProcessExit(childPID, 5*time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestAbruptParentHelper(t *testing.T) {
	if os.Getenv("EMBER_PROC_PARENT_HELPER") != "1" {
		return
	}
	binary := buildTestproc(t)
	process := startSharedTestproc(t, binary, "sleep", io.Discard)
	var registry Registry
	if err := registry.Register(process); err != nil {
		t.Fatal(err)
	}
	fmt.Printf("EMBER_PROC_CHILD_PID:%d\n", process.Pid())
	for {
		time.Sleep(time.Hour)
	}
}
