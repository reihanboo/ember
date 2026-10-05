package watcher

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/reihanboo/ember/internal/config"
)

func TestWatchEmitsDebouncedFilteredChanges(t *testing.T) {
	for _, polling := range []bool{false, true} {
		name := "fsnotify"
		if polling {
			name = "poller"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			for _, directory := range []string{"src", "build"} {
				if err := os.MkdirAll(filepath.Join(root, directory), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			settings := config.WatchConfig{
				Paths:   []string{"src", "build"},
				Include: []string{"src/**/*.c"},
				Ignore:  []string{"build/**"},
				Poll:    polling,
			}
			backend, err := NewWatcher(root, settings)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := backend.Close(); err != nil {
					t.Errorf("close watcher: %v", err)
				}
			}()
			if polling {
				if _, ok := backend.(*Poller); !ok {
					t.Fatalf("NewWatcher() returned %T for polling config", backend)
				}
			} else if _, ok := backend.(*FSNotifyWatcher); !ok {
				t.Fatalf("NewWatcher() returned %T for fsnotify config", backend)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			const delay = 200 * time.Millisecond
			changes := backend.Watch(ctx, delay)

			quiet := 2 * delay
			if polling {
				quiet += pollInterval
			}
			matchingPath := filepath.Join(root, "src", "main.c")
			if err := os.WriteFile(matchingPath, []byte("int main(void) {}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			change := receiveChange(t, changes)
			if !reflect.DeepEqual(change.Paths, []string{matchingPath}) {
				t.Errorf("change paths = %#v, want %#v", change.Paths, []string{matchingPath})
			}
			assertNoChange(t, changes, quiet)

			ignoredPath := filepath.Join(root, "build", "ignored.c")
			if err := os.WriteFile(ignoredPath, []byte("int ignored;\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			assertNoChange(t, changes, quiet)
		})
	}
}

func TestWatchRegistersDirectoriesCreatedAtRuntime(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "src"), 0o700); err != nil {
		t.Fatal(err)
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()
	if err := RegisterDirectories(watcher, root, []string{"src"}, nil); err != nil {
		t.Fatal(err)
	}
	filter, err := NewFilter(root, []string{"src/**/*.c"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const delay = 200 * time.Millisecond
	changes := Watch(ctx, watcher, filter, delay)

	newDirectory := filepath.Join(root, "src", "new")
	filePath := filepath.Join(newDirectory, "x.c")
	if err := os.Mkdir(newDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte("int x;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	change := receiveChange(t, changes)
	if !reflect.DeepEqual(change.Paths, []string{filePath}) {
		t.Errorf("change paths = %#v, want %#v", change.Paths, []string{filePath})
	}

	if err := os.RemoveAll(newDirectory); err != nil {
		t.Fatal(err)
	}
	drainChanges(t, changes, 2*delay)
	if err := os.Mkdir(newDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte("int x;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	change = receiveChange(t, changes)
	if !reflect.DeepEqual(change.Paths, []string{filePath}) {
		t.Errorf("change after recreating directory = %#v, want %#v", change.Paths, []string{filePath})
	}

	cancel()
	waitForClose(t, changes)
}

func TestWatchStopsOnContextCancellation(t *testing.T) {
	for _, polling := range []bool{false, true} {
		name := "fsnotify"
		if polling {
			name = "poller"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, "src"), 0o700); err != nil {
				t.Fatal(err)
			}
			backend, err := NewWatcher(root, config.WatchConfig{
				Paths:   []string{"src"},
				Include: []string{"**/*.c"},
				Poll:    polling,
			})
			if err != nil {
				t.Fatal(err)
			}
			defer backend.Close()
			ctx, cancel := context.WithCancel(context.Background())
			changes := backend.Watch(ctx, time.Hour)

			cancel()
			waitForClose(t, changes)
		})
	}
}

func receiveChange(t *testing.T, changes <-chan Change) Change {
	t.Helper()
	select {
	case change, ok := <-changes:
		if !ok {
			t.Fatal("change channel closed unexpectedly")
		}
		return change
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for change")
		return Change{}
	}
}

func assertNoChange(t *testing.T, changes <-chan Change, duration time.Duration) {
	t.Helper()
	select {
	case change, ok := <-changes:
		if !ok {
			t.Fatal("change channel closed unexpectedly")
		}
		t.Fatalf("unexpected change: %#v", change)
	case <-time.After(duration):
	}
}

func drainChanges(t *testing.T, changes <-chan Change, duration time.Duration) {
	t.Helper()
	timer := time.NewTimer(duration)
	defer timer.Stop()
	for {
		select {
		case _, ok := <-changes:
			if !ok {
				t.Fatal("change channel closed unexpectedly")
			}
		case <-timer.C:
			return
		}
	}
}

func waitForClose(t *testing.T, changes <-chan Change) {
	t.Helper()
	select {
	case _, ok := <-changes:
		if ok {
			t.Fatal("change channel remained open after cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("watch goroutine did not exit after cancellation")
	}
}
