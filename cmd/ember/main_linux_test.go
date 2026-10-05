package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

func configureShutdownTestParent(*exec.Cmd) {}

func sendShutdownSignal(command *exec.Cmd) error {
	return command.Process.Signal(os.Interrupt)
}

func shutdownTestprocCommand(binary string) string {
	return "'" + strings.ReplaceAll(binary, "'", "'\\''") + "' sleep"
}

func waitForShutdownChildExit(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if err != nil && err == syscall.ESRCH {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("child process %d still exists after %s", pid, timeout)
}

func TestLinuxSignalContextIncludesSIGTERM(t *testing.T) {
	ctx, stop := newSignalContext()
	defer stop()
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("signal context was not canceled by SIGTERM")
	}
}
