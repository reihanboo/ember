package proc

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestStartWaitExitResults(t *testing.T) {
	process, err := Start(context.Background(), Spec{Cmd: "exit 0"})
	if err != nil {
		t.Fatal(err)
	}
	if pid := process.Pid(); pid <= 0 {
		t.Errorf("Pid() = %d, want a positive pid", pid)
	}
	if result := process.Wait(); result.Code != 0 || result.Err != nil {
		t.Errorf("zero exit result = %#v, want code 0 and no error", result)
	}

	process, err = Start(context.Background(), Spec{Cmd: "exit 7"})
	if err != nil {
		t.Fatal(err)
	}
	if result := process.Wait(); result.Code != 7 || result.Err == nil {
		t.Errorf("nonzero exit result = %#v, want code 7 and an exit error", result)
	}
}

func TestStartReportsNonexistentCommand(t *testing.T) {
	process, err := Start(context.Background(), Spec{Cmd: "ember_command_does_not_exist_98431"})
	if err != nil {
		t.Fatal(err)
	}
	result := process.Wait()
	if result.Code == 0 || result.Err == nil {
		t.Errorf("nonexistent command result = %#v, want nonzero code and error", result)
	}
}

func TestStopSendsSIGTERM(t *testing.T) {
	binary := buildTestproc(t)
	process := startLinuxTestproc(t, binary, "sleep", nil)
	if err := process.Stop(context.Background(), time.Second); err != nil {
		t.Fatal(err)
	}
	result := process.Wait()
	if result.Signal != syscall.SIGTERM {
		t.Errorf("exit signal = %v, want %v", result.Signal, syscall.SIGTERM)
	}
}

func TestStopForceKillsProcessGroup(t *testing.T) {
	binary := buildTestproc(t)
	output := make(chan string, 1)
	process := startLinuxTestproc(t, binary, "ignore-term", output)
	select {
	case <-output:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for process to ignore SIGTERM")
	}
	if err := process.Stop(context.Background(), 50*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	result := process.Wait()
	if result.Signal != syscall.SIGKILL {
		t.Errorf("exit signal = %v, want %v", result.Signal, syscall.SIGKILL)
	}
}

func TestStopKillsGrandchild(t *testing.T) {
	binary := buildTestproc(t)
	output := make(chan string, 4)
	process := startLinuxTestproc(t, binary, "spawn-child-sleep", output)
	var childPID int
	select {
	case line := <-output:
		pid, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "[] ")))
		if err != nil {
			t.Fatalf("parse grandchild pid from %q: %v", line, err)
		}
		childPID = pid
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for grandchild pid")
	}

	if err := process.Stop(context.Background(), time.Second); err != nil {
		t.Fatal(err)
	}
	if err := waitForProcessExit(childPID, time.Second); err != nil {
		t.Fatal(err)
	}
}

type linuxTestWriter chan<- string

func (writer linuxTestWriter) Write(data []byte) (int, error) {
	line := string(data)
	writer <- line
	return len(data), nil
}

func startLinuxTestproc(t *testing.T, binary, mode string, output chan string) *Process {
	t.Helper()
	var writer io.Writer = io.Discard
	if output != nil {
		writer = linuxTestWriter(output)
	}
	process, err := Start(context.Background(), Spec{
		Cmd:    fmt.Sprintf("%s %s", processOutputCommand(binary), mode),
		Output: writer,
	})
	if err != nil {
		t.Fatal(err)
	}
	return process
}

func waitForProcessExit(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if err != nil && err == syscall.ESRCH {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("process %d still exists after %s", pid, timeout)
}
