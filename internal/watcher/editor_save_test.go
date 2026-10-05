package watcher

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestEditorSavePatternsEmitOnlyTarget(t *testing.T) {
	tests := []struct {
		name    string
		include []string
		update  func(*testing.T, string, string)
	}{
		{
			name:    "write temp then rename over target",
			include: []string{"**/*.c"},
			update: func(t *testing.T, target, directory string) {
				temp := target + ".tmp"
				writeFile(t, temp, "int updated;\n")
				if err := os.Rename(temp, target); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:    "delete then create",
			include: []string{"**/*.c"},
			update: func(t *testing.T, target, directory string) {
				if err := os.Remove(target); err != nil {
					t.Fatal(err)
				}
				writeFile(t, target, "int recreated;\n")
			},
		},
		{
			name:    "truncate then write",
			include: []string{"**/*.c"},
			update: func(t *testing.T, target, directory string) {
				file, err := os.OpenFile(target, os.O_WRONLY|os.O_TRUNC, 0)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := file.WriteString("int truncated;\n"); err != nil {
					file.Close()
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:    "swp file",
			include: []string{"**/*"},
			update: func(t *testing.T, target, directory string) {
				writeFile(t, target, "int swp;\n")
				writeFile(t, target+".swp", "temporary\n")
			},
		},
		{
			name:    "backup tilde file",
			include: []string{"**/*"},
			update: func(t *testing.T, target, directory string) {
				writeFile(t, target, "int backup;\n")
				writeFile(t, target+"~", "temporary\n")
			},
		},
		{
			name:    "emacs lock file",
			include: []string{"**/*"},
			update: func(t *testing.T, target, directory string) {
				writeFile(t, target, "int locked;\n")
				writeFile(t, filepath.Join(directory, ".#main.c"), "temporary\n")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			sourceDir := filepath.Join(root, "src")
			if err := os.Mkdir(sourceDir, 0o700); err != nil {
				t.Fatal(err)
			}
			watcher, err := newEditorWatcher(root, test.include)
			if err != nil {
				t.Fatal(err)
			}
			defer watcher.close()

			const delay = 150 * time.Millisecond
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			changes := Watch(ctx, watcher.watcher, watcher.filter, delay)
			target := filepath.Join(sourceDir, "main.c")
			writeFile(t, target, "int initial;\n")
			assertTargetChange(t, changes, target, 2*delay)

			test.update(t, target, sourceDir)
			assertTargetChange(t, changes, target, 2*delay)
		})
	}
}

func TestEditorPatternsAreIgnored(t *testing.T) {
	tests := []struct {
		name string
		path func(string) string
	}{
		{name: "swp", path: func(target string) string { return target + ".swp" }},
		{name: "tilde", path: func(target string) string { return target + "~" }},
		{name: "emacs lock", path: func(target string) string { return filepath.Join(filepath.Dir(target), ".#main.c") }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			sourceDir := filepath.Join(root, "src")
			if err := os.Mkdir(sourceDir, 0o700); err != nil {
				t.Fatal(err)
			}
			watcher, err := newEditorWatcher(root, []string{"**/*"})
			if err != nil {
				t.Fatal(err)
			}
			defer watcher.close()

			const delay = 150 * time.Millisecond
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			changes := Watch(ctx, watcher.watcher, watcher.filter, delay)
			tempPath := test.path(filepath.Join(sourceDir, "main.c"))
			writeFile(t, tempPath, "temporary\n")
			assertNoChange(t, changes, 2*delay)
		})
	}
}

type editorWatcher struct {
	watcher *fsnotify.Watcher
	filter  *Filter
}

func newEditorWatcher(root string, include []string) (*editorWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	ignore := []string{"**/*.swp", "**/*~", "**/.#*"}
	if err := RegisterDirectories(watcher, root, []string{"src"}, ignore); err != nil {
		watcher.Close()
		return nil, err
	}
	filter, err := NewFilter(root, include, ignore)
	if err != nil {
		watcher.Close()
		return nil, err
	}
	return &editorWatcher{watcher: watcher, filter: filter}, nil
}

func (w *editorWatcher) close() {
	w.watcher.Close()
}

func assertTargetChange(t *testing.T, changes <-chan Change, target string, quiet time.Duration) {
	t.Helper()
	change := receiveChange(t, changes)
	if !reflect.DeepEqual(change.Paths, []string{target}) {
		t.Errorf("change paths = %#v, want only %q", change.Paths, target)
	}
	assertNoChange(t, changes, quiet)
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
