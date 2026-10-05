package proc

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestStartWaitExitResults(t *testing.T) {
	process, err := Start(context.Background(), Spec{Cmd: "exit /b 0"})
	if err != nil {
		t.Fatal(err)
	}
	if pid := process.Pid(); pid <= 0 {
		t.Errorf("Pid() = %d, want a positive pid", pid)
	}
	if result := process.Wait(); result.Code != 0 || result.Err != nil {
		t.Errorf("zero exit result = %#v, want code 0 and no error", result)
	}

	process, err = Start(context.Background(), Spec{Cmd: "exit /b 7"})
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

func TestStopTerminatesSleepingProcess(t *testing.T) {
	binary := buildTestproc(t)
	process := startWindowsTestproc(t, binary, "sleep", nil)
	if err := process.Stop(time.Second, time.Second); err != nil {
		t.Fatal(err)
	}
	if result := process.Wait(); result.Code == 0 {
		t.Errorf("exit result = %#v, want a terminated process", result)
	}
}

func TestStopTerminatesGrandchild(t *testing.T) {
	binary := buildTestproc(t)
	output := make(chan string, 4)
	process := startWindowsTestproc(t, binary, "spawn-child-sleep", output)
	var childPID uint32
	select {
	case line := <-output:
		pid, err := strconv.ParseUint(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "[] ")), 10, 32)
		if err != nil {
			t.Fatalf("parse grandchild pid from %q: %v", line, err)
		}
		childPID = uint32(pid)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for grandchild pid")
	}

	child, err := windows.OpenProcess(windows.SYNCHRONIZE, false, childPID)
	if err != nil {
		t.Fatalf("open grandchild process %d: %v", childPID, err)
	}
	defer windows.CloseHandle(child)

	if err := process.Stop(time.Second, time.Second); err != nil {
		t.Fatal(err)
	}
	result, err := windows.WaitForSingleObject(child, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if result != windows.WAIT_OBJECT_0 {
		t.Errorf("grandchild wait result = %d, want %d", result, windows.WAIT_OBJECT_0)
	}
}

type windowsTestWriter chan<- string

func (writer windowsTestWriter) Write(data []byte) (int, error) {
	line := string(data)
	writer <- line
	return len(data), nil
}

func startWindowsTestproc(t *testing.T, binary, mode string, output chan string) *Process {
	t.Helper()
	var writer io.Writer = io.Discard
	if output != nil {
		writer = windowsTestWriter(output)
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
