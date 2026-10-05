package build

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

const buildHelperEnv = "EMBER_BUILD_TEST_HELPER"

func TestMain(m *testing.M) {
	if mode := os.Getenv(buildHelperEnv); mode != "" {
		runBuildTestHelper(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestBuildProcessHelper(t *testing.T) {}

func runBuildTestHelper(mode string) {
	switch mode {
	case "child":
		for {
			time.Sleep(time.Hour)
		}
	case "parent":
		executable, err := os.Executable()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		child := exec.Command(executable)
		child.Env = buildHelperEnvironment("child")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("child-pid=%d\n", child.Process.Pid)
		for {
			time.Sleep(time.Hour)
		}
	}
	os.Exit(1)
}

func buildHelperEnvironment(mode string) []string {
	environment := os.Environ()
	filtered := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		key, _, ok := strings.Cut(entry, "=")
		if ok && strings.EqualFold(key, buildHelperEnv) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return append(filtered, buildHelperEnv+"="+mode)
}

type buildOutput chan string

func (output buildOutput) Write(data []byte) (int, error) {
	output <- string(data)
	return len(data), nil
}

func TestRunSuccess(t *testing.T) {
	var output bytes.Buffer
	result := Run(context.Background(), Spec{Cmd: "echo build-output", Output: &output})

	if !result.Success {
		t.Fatalf("Run() success = false, result = %#v", result)
	}
	if result.ExitCode != 0 {
		t.Errorf("Run() exit code = %d, want 0", result.ExitCode)
	}
	if result.Err != nil {
		t.Errorf("Run() error = %v, want nil", result.Err)
	}
	if got := output.String(); !strings.Contains(got, "[build] build-output") {
		t.Errorf("Run() output = %q, want tagged build output", got)
	}
}

func TestRunNonzeroExit(t *testing.T) {
	command := "exit 7"
	if runtime.GOOS == "windows" {
		command = "exit /b 7"
	}

	result := Run(context.Background(), Spec{Cmd: command})
	if result.Success {
		t.Fatalf("Run() success = true, want false")
	}
	if result.ExitCode != 7 {
		t.Errorf("Run() exit code = %d, want 7", result.ExitCode)
	}
	if result.Err == nil {
		t.Error("Run() error = nil, want nonzero exit error")
	}
}

func TestRunCancellationKillsProcessTree(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	output := make(buildOutput, 8)
	result := make(chan Result, 1)
	go func() {
		result <- Run(ctx, Spec{
			Cmd:    buildHelperCommand(executable),
			Env:    map[string]string{buildHelperEnv: "parent"},
			Output: output,
		})
	}()

	var childPID int
	select {
	case line := <-output:
		pid, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(line), "[build] child-pid="))
		if err != nil {
			t.Fatalf("parse child pid from %q: %v", line, err)
		}
		childPID = pid
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for build grandchild pid")
	}

	cancelledAt := time.Now()
	cancel()
	var got Result
	select {
	case got = <-result:
	case <-time.After(time.Second):
		t.Fatal("cancelled build did not return within one second")
	}
	if elapsed := time.Since(cancelledAt); elapsed >= time.Second {
		t.Errorf("cancelled build returned after %s, want under one second", elapsed)
	}
	if !got.Cancelled {
		t.Errorf("Run() cancelled = false, result = %#v", got)
	}
	if err := waitForBuildHelperExit(childPID, time.Second); err != nil {
		t.Error(err)
	}
}

func TestRunCommandNotFound(t *testing.T) {
	result := Run(context.Background(), Spec{Cmd: "ember-command-that-does-not-exist"})
	if result.Success {
		t.Fatalf("Run() success = true, want false")
	}
	if result.ExitCode == 0 {
		t.Errorf("Run() exit code = 0, want nonzero")
	}
	if result.Err == nil {
		t.Error("Run() error = nil, want command-not-found error")
	}
}
