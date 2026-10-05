package proc

import (
	"context"
	"io"
	"testing"
	"time"
)

func TestRegistryStopsRegisteredProcesses(t *testing.T) {
	binary := buildTestproc(t)
	process := startSharedTestproc(t, binary, "sleep", io.Discard)
	var registry Registry
	if err := registry.Register(process); err != nil {
		t.Fatal(err)
	}

	if err := registry.StopAll(context.Background(), 100*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if result := process.Wait(); result.Code == 0 {
		t.Errorf("exit result = %#v, want a stopped process", result)
	}
	if err := registry.StopAll(context.Background(), 100*time.Millisecond); err != nil {
		t.Errorf("second StopAll() error = %v, want nil", err)
	}
}

func TestRegistryRejectsRegistrationAfterStopAll(t *testing.T) {
	var registry Registry
	if err := registry.StopAll(context.Background(), time.Second); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(&Process{}); err == nil {
		t.Fatal("Register() error = nil, want a stopping-registry error")
	}
}
