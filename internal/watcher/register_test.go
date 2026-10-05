package watcher

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type recordingWatcher struct {
	paths []string
	err   error
}

func (w *recordingWatcher) Add(path string) error {
	if w.err != nil {
		return w.err
	}
	w.paths = append(w.paths, path)
	return nil
}

func TestRegisterDirectoriesAddsExpectedTree(t *testing.T) {
	root := t.TempDir()
	directories := []string{
		"src/lib",
		"src/generated/nested",
		"src/build/cache",
		"include/detail",
		".ember",
	}
	for _, directory := range directories {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o700); err != nil {
			t.Fatal(err)
		}
	}

	watcher := &recordingWatcher{}
	err := RegisterDirectories(
		watcher,
		root,
		[]string{"src", "include", ".ember"},
		[]string{"src/generated/**", "src/build/**"},
	)
	if err != nil {
		t.Fatal(err)
	}

	got := append([]string(nil), watcher.paths...)
	want := []string{
		filepath.Join(root, "src"),
		filepath.Join(root, "src", "lib"),
		filepath.Join(root, "include"),
		filepath.Join(root, "include", "detail"),
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("watched directories = %#v, want %#v", got, want)
	}
}

func TestRegisterDirectoriesMissingPathReturnsClearError(t *testing.T) {
	watcher := &recordingWatcher{}
	err := RegisterDirectories(watcher, t.TempDir(), []string{"missing"}, nil)
	if err == nil {
		t.Fatal("RegisterDirectories() error = nil, want missing-path error")
	}
	if got, want := err.Error(), `watch path "missing" does not exist`; !strings.Contains(got, want) {
		t.Errorf("error = %q, want it to contain %q", got, want)
	}
}

func TestRegisterDirectoriesReturnsAddError(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "src"), 0o700); err != nil {
		t.Fatal(err)
	}
	watchError := errors.New("watch failed")
	watcher := &recordingWatcher{err: watchError}
	if err := RegisterDirectories(watcher, root, []string{"src"}, nil); !errors.Is(err, watchError) {
		t.Errorf("RegisterDirectories() error = %v, want %v", err, watchError)
	}
}
