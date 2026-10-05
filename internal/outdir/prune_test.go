package outdir

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPruneRemovesStaleOutputsAndSidecars(t *testing.T) {
	workingDirectory := useTemporaryWorkingDirectory(t)
	binDirectory := filepath.Join(workingDirectory, ".ember", "bin")
	if err := os.MkdirAll(binDirectory, 0o700); err != nil {
		t.Fatal(err)
	}

	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	kept := filepath.Join(binDirectory, "app-2"+suffix)
	keptStem := filepath.Join(binDirectory, "app-2")
	stale := filepath.Join(binDirectory, "app-1"+suffix)
	staleStem := filepath.Join(binDirectory, "app-1")
	paths := []string{
		kept,
		keptStem + ".pdb",
		keptStem + ".ilk",
		stale,
		staleStem + ".pdb",
		staleStem + ".ilk",
		filepath.Join(binDirectory, "old-file"),
	}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("output"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := Prune(kept); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths[:3] {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("kept output path %q is missing: %v", path, err)
		}
	}
	for _, path := range paths[3:] {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("stale output path %q still exists or returned an unexpected error: %v", path, err)
		}
	}
}

func TestPruneMissingDirectory(t *testing.T) {
	useTemporaryWorkingDirectory(t)
	if err := Prune(); err != nil {
		t.Fatalf("Prune() error = %v, want nil", err)
	}
}

func useTemporaryWorkingDirectory(t *testing.T) string {
	t.Helper()
	workingDirectory := t.TempDir()
	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workingDirectory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDirectory); err != nil {
			t.Error(err)
		}
	})
	return workingDirectory
}
