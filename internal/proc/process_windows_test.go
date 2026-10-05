package proc

import (
	"context"
	"errors"
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

func TestStopAllowsCtrlBreakHandlerToExit(t *testing.T) {
	binary := buildTestproc(t)
	output := make(chan string, 4)
	process := startWindowsTestproc(t, binary, "handle-break", output)
	if pid := readWindowsTestprocPID(t, output); pid <= 0 {
		t.Fatalf("helper pid = %d, want a positive pid", pid)
	}
	if err := process.Stop(context.Background(), 3*time.Second); err != nil {
		t.Fatal(err)
	}
	if result := process.Wait(); result.Code != 23 {
		t.Errorf("exit result = %#v, want helper exit code 23", result)
	}
}

func TestStopForceKillsIgnoringCtrlBreak(t *testing.T) {
	binary := buildTestproc(t)
	output := make(chan string, 4)
	process := startWindowsTestproc(t, binary, "ignore-term", output)
	pid := readWindowsTestprocPID(t, output)
	if pid <= 0 {
		t.Fatalf("helper pid = %d, want a positive pid", pid)
	}
	processHandle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		t.Fatalf("open helper process %d: %v", pid, err)
	}
	defer windows.CloseHandle(processHandle)

	grace := 300 * time.Millisecond
	started := time.Now()
	if err := process.Stop(context.Background(), grace); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed < grace {
		t.Errorf("Stop returned after %s, want it to wait at least %s", elapsed, grace)
	}
	if result := process.Wait(); result.Code == 0 {
		t.Errorf("exit result = %#v, want a force-terminated process", result)
	}
	waitResult, err := windows.WaitForSingleObject(processHandle, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if waitResult != windows.WAIT_OBJECT_0 {
		t.Errorf("helper process wait result = %d, want %d", waitResult, windows.WAIT_OBJECT_0)
	}
}

func TestStopTerminatesSleepingProcess(t *testing.T) {
	binary := buildTestproc(t)
	process := startWindowsTestproc(t, binary, "sleep", nil)
	if err := process.Stop(context.Background(), time.Second); err != nil {
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

	if err := process.Stop(context.Background(), time.Second); err != nil {
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

func readWindowsTestprocPID(t *testing.T, output <-chan string) int {
	t.Helper()
	select {
	case line := <-output:
		pid, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "[] ")))
		if err != nil {
			t.Fatalf("parse helper pid from %q: %v", line, err)
		}
		return pid
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for helper pid")
		return 0
	}
}

func waitForProcessExit(pid int, timeout time.Duration) error {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return nil
		}
		return fmt.Errorf("open process %d: %w", pid, err)
	}
	defer windows.CloseHandle(handle)

	result, err := windows.WaitForSingleObject(handle, uint32(timeout.Milliseconds()))
	if err != nil {
		return fmt.Errorf("wait for process %d: %w", pid, err)
	}
	if result != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("process %d did not exit within %s", pid, timeout)
	}
	return nil
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
