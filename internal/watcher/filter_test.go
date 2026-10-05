package watcher

import (
	"path/filepath"
	"testing"
)

func TestFilterIncludeOnly(t *testing.T) {
	root := t.TempDir()
	filter, err := NewFilter(root, []string{"**/*.c", "**/*.cpp"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if !filter.Allow(filepath.Join(root, "src", "main.c")) {
		t.Error("included source file was rejected")
	}
	if !filter.Allow(filepath.Join(root, "main.cpp")) {
		t.Error("root-level included source file was rejected")
	}
	if filter.Allow(filepath.Join(root, "src", "README.md")) {
		t.Error("path outside the include patterns was allowed")
	}
}

func TestFilterIgnoreWins(t *testing.T) {
	root := t.TempDir()
	filter, err := NewFilter(root, []string{"**/*.c"}, []string{"generated/**"})
	if err != nil {
		t.Fatal(err)
	}

	if filter.Allow(filepath.Join(root, "generated", "main.c")) {
		t.Error("ignored path matching include pattern was allowed")
	}
	if !filter.Allow(filepath.Join(root, "src", "main.c")) {
		t.Error("non-ignored included path was rejected")
	}
}

func TestFilterAlwaysIgnoresEmberDirectory(t *testing.T) {
	root := t.TempDir()
	filter, err := NewFilter(root, []string{"**/*"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if filter.Allow(filepath.Join(root, ".ember", "ctl")) {
		t.Error(".ember file was allowed")
	}
	if filter.Allow(filepath.Join(root, ".ember")) {
		t.Error(".ember directory was allowed")
	}
	if !filter.Allow(filepath.Join(root, "src", "main.c")) {
		t.Error("path outside .ember was rejected")
	}
}

func TestFilterRejectsPathsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	filter, err := NewFilter(root, []string{"**/*.c"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(root), filepath.Base(root)+"-other", "main.c")
	if filter.Allow(outside) {
		t.Errorf("path outside root was allowed: %q", outside)
	}
}

func TestFilterAllowsAllWhenIncludeIsEmpty(t *testing.T) {
	root := t.TempDir()
	filter, err := NewFilter(root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !filter.Allow(filepath.Join(root, "README.md")) {
		t.Error("path rejected with no include patterns")
	}
}
