package proc

import (
	"bytes"
	"context"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

const outputHelperEnv = "EMBER_PROC_OUTPUT_HELPER"

func TestMain(m *testing.M) {
	if os.Getenv(outputHelperEnv) == "1" {
		runOutputHelper()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestProcessStreamsPrefixedOutputByLine(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	process, err := Start(context.Background(), Spec{
		Cmd:    processOutputCommand(executable),
		Env:    map[string]string{outputHelperEnv: "1"},
		Output: &output,
		Tag:    "app",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result := process.Wait(); result.Code != 0 || result.Err != nil {
		t.Fatalf("helper process result = %#v, want code 0 and no error", result)
	}

	got := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
	sort.Strings(got)
	want := []string{
		"[app] stderr-line",
		"[app] stderr-tail",
		"[app] stdout-line  ",
		"[app] stdout-tail",
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("output lines = %#v, want %#v", got, want)
	}
}

func runOutputHelper() {
	stdoutWritten := make(chan struct{})
	stderrWritten := make(chan struct{})
	var writers sync.WaitGroup
	writers.Add(2)
	go func() {
		defer writers.Done()
		os.Stdout.Write([]byte("stdout-"))
		close(stdoutWritten)
		<-stderrWritten
		os.Stdout.Write([]byte("line  \nstdout-tail"))
	}()
	go func() {
		defer writers.Done()
		<-stdoutWritten
		os.Stderr.Write([]byte("stderr-line\nstderr-tail"))
		close(stderrWritten)
	}()
	writers.Wait()
}
