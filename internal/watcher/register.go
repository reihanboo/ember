package watcher

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/reihanboo/ember/internal/watcher/glob"
)

type DirectoryWatcher interface {
	Add(name string) error
}

func RegisterDirectories(watcher DirectoryWatcher, root string, paths, ignore []string) error {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve project root %q: %w", root, err)
	}
	absoluteRoot = filepath.Clean(absoluteRoot)
	added := make(map[string]struct{})

	for _, watchPath := range paths {
		path := filepath.Join(absoluteRoot, filepath.FromSlash(watchPath))
		info, err := os.Stat(path)
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("watch path %q does not exist: %w", watchPath, err)
		}
		if err != nil {
			return fmt.Errorf("inspect watch path %q: %w", watchPath, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("watch path %q is not a directory", watchPath)
		}

		err = filepath.WalkDir(path, func(directory string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return fmt.Errorf("walk watch path %q: %w", watchPath, walkErr)
			}
			if !entry.IsDir() {
				return nil
			}
			if ignoredDirectory(absoluteRoot, directory, ignore) {
				return filepath.SkipDir
			}

			directory = filepath.Clean(directory)
			if _, exists := added[directory]; exists {
				return nil
			}
			if err := watcher.Add(directory); err != nil {
				return fmt.Errorf("add watch directory %q: %w", directory, err)
			}
			added[directory] = struct{}{}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func ignoredDirectory(root, directory string, ignore []string) bool {
	relative, err := filepath.Rel(root, directory)
	if err != nil {
		return true
	}
	path := filepath.ToSlash(relative)
	if path == ".ember" || glob.Match(".ember/**", path) {
		return true
	}
	for _, pattern := range ignore {
		pattern = strings.ReplaceAll(pattern, `\`, "/")
		if glob.Match(pattern, path) {
			return true
		}
		if strings.HasSuffix(pattern, "/**") && glob.Match(strings.TrimSuffix(pattern, "/**"), path) {
			return true
		}
	}
	return false
}
