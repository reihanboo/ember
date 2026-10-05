package proc

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestTestprocModes(t *testing.T) {
	binary := buildTestproc(t)

	for _, code := range []string{"0", "7"} {
		t.Run("exit "+code, func(t *testing.T) {
			cmd := exec.Command(binary, "exit", code)
			err := cmd.Run()
			if code == "0" {
				if err != nil {
					t.Fatalf("exit 0 error = %v", err)
				}
				return
			}
			exitError, ok := err.(*exec.ExitError)
			if !ok || exitError.ExitCode() != 7 {
				t.Fatalf("exit 7 error = %v, want exit code 7", err)
			}
		})
	}

	t.Run("sleep", func(t *testing.T) {
		cmd, pid := startTestproc(t, binary, "sleep")
		if pid != cmd.Process.Pid {
			t.Errorf("sleep pid = %d, process pid = %d", pid, cmd.Process.Pid)
		}
	})

	t.Run("spawn child sleep", func(t *testing.T) {
		cmd, childPID := startTestproc(t, binary, "spawn-child-sleep")
		if childPID == cmd.Process.Pid {
			t.Errorf("child pid = %d, want a different pid from parent", childPID)
		}
		child, err := os.FindProcess(childPID)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			child.Kill()
		})
	})

	t.Run("ignore term", func(t *testing.T) {
		startTestproc(t, binary, "ignore-term")
	})
}

func buildTestproc(t *testing.T) string {
	t.Helper()
	name := "testproc"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("go", "build", "-o", binary, "./internal/testproc")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build testproc: %v\n%s", err, output)
	}
	return binary
}

func startTestproc(t *testing.T, binary, mode string) (*exec.Cmd, int) {
	t.Helper()
	cmd := exec.Command(binary, mode)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	})

	readResult := make(chan struct {
		pid int
		err error
	}, 1)
	go func() {
		line, err := bufio.NewReader(stdout).ReadString('\n')
		if err != nil {
			readResult <- struct {
				pid int
				err error
			}{err: err}
			return
		}
		pid, err := strconv.Atoi(strings.TrimSpace(line))
		readResult <- struct {
			pid int
			err error
		}{pid: pid, err: err}
	}()

	select {
	case result := <-readResult:
		if result.err != nil {
			t.Fatalf("read pid from %s: %v", mode, result.err)
		}
		return cmd, result.pid
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for pid from %s", mode)
		return nil, 0
	}
}
