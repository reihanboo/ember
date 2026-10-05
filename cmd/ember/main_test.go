package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/reihanboo/ember/internal/proc"
)

func TestSignalShutdownStopsChildren(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestSignalShutdownHelper$", "-test.v")
	command.Env = append(os.Environ(), "EMBER_SIGNAL_HELPER=1")
	configureShutdownTestParent(command)
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = io.Discard
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	parentExited := false
	var waitResult chan error
	t.Cleanup(func() {
		if !parentExited {
			command.Process.Kill()
			if waitResult != nil {
				<-waitResult
			} else {
				command.Wait()
			}
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
			if !strings.HasPrefix(line, "EMBER_SIGNAL_CHILD_PID=") {
				continue
			}
			pid, err := strconv.Atoi(strings.TrimPrefix(line, "EMBER_SIGNAL_CHILD_PID="))
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
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for child pid")
	}
	if err := sendShutdownSignal(command); err != nil {
		t.Fatalf("send interrupt to parent: %v", err)
	}
	waitResult = make(chan error, 1)
	go func() {
		waitResult <- command.Wait()
	}()
	select {
	case err := <-waitResult:
		parentExited = true
		if err != nil {
			t.Fatalf("parent exit error: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for graceful parent shutdown")
	}
	if err := waitForShutdownChildExit(childPID, 5*time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestSignalShutdownHelper(t *testing.T) {
	if os.Getenv("EMBER_SIGNAL_HELPER") != "1" {
		return
	}
	ctx, stopSignals := newSignalContext()
	defer stopSignals()
	registry := &proc.Registry{}
	status := runWithShutdown(ctx, registry, func(ctx context.Context, registry *proc.Registry) int {
		binary := buildShutdownTestproc(t)
		process, err := proc.Start(context.Background(), proc.Spec{
			Cmd:    shutdownTestprocCommand(binary),
			Output: io.Discard,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := registry.Register(process); err != nil {
			t.Fatal(err)
		}
		fmt.Printf("EMBER_SIGNAL_CHILD_PID=%d\n", process.Pid())
		<-ctx.Done()
		return 0
	}, io.Discard)
	if status != 0 {
		t.Fatalf("shutdown status = %d, want 0", status)
	}
}

func buildShutdownTestproc(t *testing.T) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "../.."))
	name := "testproc"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)
	command := exec.Command("go", "build", "-o", binary, "./internal/proc/internal/testproc")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build testproc: %v\n%s", err, output)
	}
	return binary
}
