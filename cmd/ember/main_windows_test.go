package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

func configureShutdownTestParent(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
}

func sendShutdownSignal(command *exec.Cmd) error {
	return windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(command.Process.Pid))
}

func shutdownTestprocCommand(binary string) string {
	return `"` + strings.ReplaceAll(binary, `"`, `""`) + `" sleep`
}

func waitForShutdownChildExit(pid int, timeout time.Duration) error {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return nil
		}
		return fmt.Errorf("open child process %d: %w", pid, err)
	}
	defer windows.CloseHandle(handle)

	result, err := windows.WaitForSingleObject(handle, uint32(timeout.Milliseconds()))
	if err != nil {
		return fmt.Errorf("wait for child process %d: %w", pid, err)
	}
	if result != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("child process %d still exists after %s", pid, timeout)
	}
	return nil
}
