package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/reihanboo/ember/internal/config"
)

func TestInitCreatesAndPreservesConfig(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	projectDir := t.TempDir()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(workingDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	var stdout, stderr bytes.Buffer
	if status := Run([]string{"init"}, &stdout, &stderr); status != 0 {
		t.Fatalf("first init status = %d, stderr = %q", status, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("first init stderr = %q, want empty", stderr.String())
	}

	configPath := filepath.Join(projectDir, "ember.toml")
	original, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.Watch.Paths, []string{"src", "include"}) {
		t.Errorf("Watch.Paths = %#v, want [src include]", cfg.Watch.Paths)
	}
	if cfg.Build.Cmd != "cmake --build build" || cfg.Build.StopsRunning {
		t.Errorf("Build = %#v, want reference build settings", cfg.Build)
	}
	if cfg.Run.Cmd != `build\app.exe --flag` || cfg.Run.Cwd != "." || cfg.Run.KillTimeoutMS != 2000 {
		t.Errorf("Run = %#v, want reference run settings", cfg.Run)
	}
	if cfg.Run.Env["FOO"] != "bar" {
		t.Errorf("Run.Env = %#v, want FOO=bar", cfg.Run.Env)
	}
	if !cfg.UI.Color {
		t.Error("UI.Color = false, want true")
	}

	stderr.Reset()
	if status := Run([]string{"init"}, &stdout, &stderr); status != 1 {
		t.Fatalf("second init status = %d, want 1", status)
	}
	if got, want := stderr.String(), "ember.toml already exists; refusing to overwrite\n"; got != want {
		t.Errorf("second init stderr = %q, want %q", got, want)
	}
	current, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(current, original) {
		t.Error("second init modified the existing config")
	}
}
