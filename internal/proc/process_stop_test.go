package proc

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestStopIsIdempotent(t *testing.T) {
	binary := buildTestproc(t)
	process := startSharedTestproc(t, binary, "sleep", io.Discard)
	t.Cleanup(func() {
		process.Stop(context.Background(), 100*time.Millisecond)
	})

	if err := process.Stop(context.Background(), 100*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := process.Stop(context.Background(), 100*time.Millisecond); err != nil {
		t.Fatal(err)
	}
}

func TestStopAfterNaturalExit(t *testing.T) {
	binary := buildTestproc(t)
	process := startSharedTestproc(t, binary, "exit 0", io.Discard)
	if result := process.Wait(); result.Code != 0 || result.Err != nil {
		t.Fatalf("natural exit result = %#v, want code 0 and no error", result)
	}
	if err := process.Stop(context.Background(), time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestStopWaitsForWholeProcessTree(t *testing.T) {
	binary := buildTestproc(t)
	output := make(chan string, 4)
	process := startSharedTestproc(t, binary, "spawn-child-sleep", testProcessWriter(output))
	t.Cleanup(func() {
		process.Stop(context.Background(), 100*time.Millisecond)
	})
	childPID := readSharedTestprocPID(t, output)

	if err := process.Stop(context.Background(), time.Second); err != nil {
		t.Fatal(err)
	}
	if err := waitForProcessExit(childPID, 5*time.Second); err != nil {
		t.Fatal(err)
	}
}

type testProcessWriter chan<- string

func (writer testProcessWriter) Write(data []byte) (int, error) {
	writer <- string(data)
	return len(data), nil
}

func startSharedTestproc(t *testing.T, binary, mode string, output io.Writer) *Process {
	t.Helper()
	process, err := Start(context.Background(), Spec{
		Cmd:    fmt.Sprintf("%s %s", processOutputCommand(binary), mode),
		Output: output,
	})
	if err != nil {
		t.Fatal(err)
	}
	return process
}

func readSharedTestprocPID(t *testing.T, output <-chan string) int {
	t.Helper()
	select {
	case line := <-output:
		pid, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "[] ")))
		if err != nil {
			t.Fatalf("parse process pid from %q: %v", line, err)
		}
		return pid
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for process pid")
		return 0
	}
}
