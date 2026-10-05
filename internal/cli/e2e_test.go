package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestEndToEndReloadScenarios(t *testing.T) {
	compiler, buildCommand := endToEndCompiler(t)
	if compiler == "" {
		t.Skip("no supported C compiler found on PATH")
	}

	root := copyHelloExample(t)
	config := endToEndConfig(buildCommand)
	if err := os.WriteFile(filepath.Join(root, "ember.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(root, "main.c")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	program := strings.Replace(string(source),
		`printf("Hello from Ember! Counter: %lu\n", counter++);`,
		`#ifdef _WIN32
        printf("PID:%lu Hello from Ember! Counter: %lu\n", (unsigned long)GetCurrentProcessId(), counter++);
#else
        printf("PID:%ld Hello from Ember! Counter: %lu\n", (long)getpid(), counter++);
#endif`,
		1,
	)
	if program == string(source) {
		t.Fatal("could not add process id to hello example")
	}
	writeEndToEndSource(t, sourcePath, program)

	outputPath := filepath.Join(root, "run.log")
	stderrPath := filepath.Join(root, "stderr.log")
	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDirectory); err != nil {
			t.Error(err)
		}
	})
	output, err := os.OpenFile(outputPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	stderr, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runResult := make(chan int, 1)
	go func() {
		runResult <- RunContext(ctx, nil, []string{"run"}, output, stderr)
	}()
	defer func() {
		cancel()
		select {
		case <-runResult:
		case <-time.After(10 * time.Second):
			t.Error("ember run did not shut down")
		}
	}()

	waitForEndToEnd(t, outputPath, func(contents []byte) bool {
		return len(processIDs(contents)) > 0
	})
	initialPID := processIDs(readEndToEndFile(t, outputPath))[0]
	if err := os.WriteFile(sourcePath, []byte(program+"\nthis is invalid C syntax\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitForEndToEnd(t, stderrPath, func(contents []byte) bool {
		return strings.Contains(string(contents), "build failed:")
	})
	failedOutput, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	failedOffset := len(failedOutput)
	waitForEndToEnd(t, outputPath, func(contents []byte) bool {
		return containsProcessID(contents[failedOffset:], initialPID)
	})
	if got := processIDs(readEndToEndFile(t, outputPath)); hasDifferentProcessID(got, initialPID) {
		t.Fatalf("process changed after failed build: initial pid %d, observed %v", initialPID, got)
	}

	if err := os.WriteFile(sourcePath, []byte(program), 0o600); err != nil {
		t.Fatal(err)
	}
	fixedPID := waitForDifferentProcessID(t, outputPath, initialPID)
	if fixedPID == initialPID {
		t.Fatalf("successful fix kept pid %d, want replacement", initialPID)
	}

	baselinePIDs := processIDs(readEndToEndFile(t, outputPath))
	baselineBuilds := lineCount(readEndToEndFile(t, filepath.Join(root, ".build-count")))
	writeEndToEndSource(t, sourcePath, "#define RELOAD_TRIGGER 1\n"+program)
	waitForEndToEnd(t, filepath.Join(root, ".build-count"), func(contents []byte) bool {
		return lineCount(contents) >= baselineBuilds+1
	})
	writeEndToEndSource(t, sourcePath, "#define RELOAD_SAVE_ONE 1\n"+program)
	waitForEndToEnd(t, filepath.Join(root, ".build-count"), func(contents []byte) bool {
		return lineCount(contents) >= baselineBuilds+2
	})
	writeEndToEndSource(t, sourcePath, "#define RELOAD_SAVE_TWO 1\n"+program)
	waitForEndToEnd(t, filepath.Join(root, ".build-count"), func(contents []byte) bool {
		return lineCount(contents) >= baselineBuilds+3
	})
	finalPID := waitForNewProcessID(t, outputPath, baselinePIDs)
	time.Sleep(2 * time.Second)
	finalPIDs := processIDs(readEndToEndFile(t, outputPath))
	newPIDs := newProcessIDs(finalPIDs, baselinePIDs)
	if len(newPIDs) != 1 || newPIDs[0] != finalPID {
		t.Fatalf("quick saves started %d replacement processes (%v), want exactly one", len(newPIDs), newPIDs)
	}
	if got := lineCount(readEndToEndFile(t, filepath.Join(root, ".build-count"))); got != baselineBuilds+3 {
		t.Errorf("build attempts during quick saves = %d, want %d", got-baselineBuilds, 3)
	}
}

func endToEndCompiler(t *testing.T) (string, string) {
	t.Helper()
	candidates := []string{"gcc", "clang"}
	if runtime.GOOS == "windows" {
		candidates = []string{"cl", "clang"}
	}
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate); err != nil {
			continue
		}
		var compile string
		switch candidate {
		case "cl":
			compile = "cl /nologo /W4 /Fe{out} main.c"
		case "clang":
			compile = "clang -std=c11 -Wall -Wextra -Wpedantic -O0 main.c -o {out}"
		default:
			compile = "gcc -std=c11 -Wall -Wextra -Wpedantic -O0 main.c -o {out}"
		}
		if runtime.GOOS == "windows" {
			return candidate, "echo build>>.build-count & ping -n 4 127.0.0.1 > nul & " + compile
		}
		return candidate, "echo build >> .build-count; sleep 3; " + compile
	}
	return "", ""
}

func endToEndConfig(buildCommand string) string {
	return fmt.Sprintf(`[watch]
paths = ["."]
include = ["**/*.c", "**/*.h"]
ignore = [".git/**", ".ember/**", ".build-count", "run.log", "stderr.log"]
debounce_ms = 100
poll = true

[build]
cmd = %q
stops_running = false

[run]
cmd = "{out}"
cwd = "."
kill_timeout_ms = 2000

[ui]
color = false
`, buildCommand)
}

func copyHelloExample(t *testing.T) string {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	example := filepath.Join(filepath.Dir(testFile), "..", "..", "examples", "hello")
	root := t.TempDir()
	err := filepath.WalkDir(example, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(example, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(root, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o700)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, contents, 0o600)
	})
	if err != nil {
		t.Fatalf("copy hello example: %v", err)
	}
	return root
}

func writeEndToEndSource(t *testing.T, path, source string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
}

func waitForEndToEnd(t *testing.T, path string, predicate func([]byte) bool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		contents, err := os.ReadFile(path)
		if err == nil && predicate(contents) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	contents, err := os.ReadFile(path)
	t.Fatalf("condition not met for %q (read error: %v): %s", path, err, contents)
}

func readEndToEndFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

func processIDs(contents []byte) []int {
	seen := make(map[int]struct{})
	var ids []int
	for _, line := range strings.Split(string(contents), "\n") {
		marker := strings.Index(line, "PID:")
		if marker < 0 {
			continue
		}
		fields := strings.Fields(line[marker+len("PID:"):])
		if len(fields) == 0 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		if _, exists := seen[pid]; exists {
			continue
		}
		seen[pid] = struct{}{}
		ids = append(ids, pid)
	}
	return ids
}

func containsProcessID(contents []byte, pid int) bool {
	for _, got := range processIDs(contents) {
		if got == pid {
			return true
		}
	}
	return false
}

func hasDifferentProcessID(ids []int, expected int) bool {
	for _, id := range ids {
		if id != expected {
			return true
		}
	}
	return false
}

func waitForDifferentProcessID(t *testing.T, path string, oldPID int) int {
	t.Helper()
	var result int
	waitForEndToEnd(t, path, func(contents []byte) bool {
		for _, pid := range processIDs(contents) {
			if pid != oldPID {
				result = pid
				return true
			}
		}
		return false
	})
	return result
}

func waitForNewProcessID(t *testing.T, path string, existing []int) int {
	t.Helper()
	var result int
	waitForEndToEnd(t, path, func(contents []byte) bool {
		pids := newProcessIDs(processIDs(contents), existing)
		if len(pids) > 0 {
			result = pids[0]
			return true
		}
		return false
	})
	return result
}

func newProcessIDs(current, existing []int) []int {
	old := make(map[int]struct{}, len(existing))
	for _, pid := range existing {
		old[pid] = struct{}{}
	}
	seen := make(map[int]struct{})
	var added []int
	for _, pid := range current {
		if _, exists := old[pid]; exists {
			continue
		}
		if _, exists := seen[pid]; exists {
			continue
		}
		seen[pid] = struct{}{}
		added = append(added, pid)
	}
	return added
}

func lineCount(contents []byte) int {
	return strings.Count(string(contents), "\n")
}
