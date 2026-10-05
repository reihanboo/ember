package proc

import (
	"context"
	"testing"
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
