package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadOverlaysDefaults(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "ember.toml")
	contents := "[watch]\ndebounce_ms = 275\n\n[ui]\ncolor = false\n"
	if err := os.WriteFile(configPath, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Watch.DebounceMS != 275 {
		t.Errorf("Watch.DebounceMS = %d, want 275", cfg.Watch.DebounceMS)
	}
	if !reflect.DeepEqual(cfg.Watch.Paths, []string{"src", "include"}) {
		t.Errorf("Watch.Paths = %#v, want default paths", cfg.Watch.Paths)
	}
	if !reflect.DeepEqual(cfg.Watch.Include, []string{"**/*.c", "**/*.cpp", "**/*.h", "**/*.hpp", "CMakeLists.txt"}) {
		t.Errorf("Watch.Include = %#v, want default include patterns", cfg.Watch.Include)
	}
	if cfg.Watch.Poll {
		t.Error("Watch.Poll = true, want default false")
	}
	if cfg.Run.Cwd != "." || cfg.Run.KillTimeoutMS != 2000 {
		t.Errorf("Run defaults = %#v, want cwd . and kill timeout 2000ms", cfg.Run)
	}
	if cfg.UI.Color {
		t.Error("UI.Color = true, want configured false")
	}
}

func TestFindFromNestedDirectory(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "ember.toml")
	if err := os.WriteFile(configPath, []byte("[ui]\ncolor = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "src", "nested")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}

	gotRoot, gotConfigPath, err := Find(nested)
	if err != nil {
		t.Fatal(err)
	}
	if gotRoot != root {
		t.Errorf("Find() root = %q, want %q", gotRoot, root)
	}
	if gotConfigPath != configPath {
		t.Errorf("Find() config path = %q, want %q", gotConfigPath, configPath)
	}
}

func TestFindNotFoundReturnsTypedError(t *testing.T) {
	startDir := t.TempDir()
	_, _, err := Find(startDir)
	var notFound *NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("Find() error = %v, want *NotFoundError", err)
	}
	if notFound.StartDir != startDir {
		t.Errorf("NotFoundError.StartDir = %q, want %q", notFound.StartDir, startDir)
	}
}
