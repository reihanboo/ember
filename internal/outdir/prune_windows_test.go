package outdir

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestPruneLogsAndIgnoresLockedFile(t *testing.T) {
	workingDirectory := useTemporaryWorkingDirectory(t)
	binDirectory := filepath.Join(workingDirectory, ".ember", "bin")
	if err := os.MkdirAll(binDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	lockedPath := filepath.Join(binDirectory, "locked-output.exe")
	if err := os.WriteFile(lockedPath, []byte("output"), 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := windows.UTF16PtrFromString(lockedPath)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(
		path,
		windows.GENERIC_READ,
		0,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := Prune(); err != nil {
		windows.CloseHandle(handle)
		t.Fatalf("Prune() error = %v, want nil for locked file", err)
	}
	if _, err := os.Stat(lockedPath); err != nil {
		t.Errorf("locked file was removed or is inaccessible: %v", err)
	}
	if err := windows.CloseHandle(handle); err != nil {
		t.Fatal(err)
	}
	if err := Prune(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(lockedPath); !os.IsNotExist(err) {
		t.Errorf("unlocked stale file still exists or returned an unexpected error: %v", err)
	}
}
