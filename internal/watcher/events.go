package watcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Change struct {
	Paths []string
}

func Watch(ctx context.Context, watcher *fsnotify.Watcher, filter *Filter, delay time.Duration) <-chan Change {
	changes := make(chan Change)
	done := make(chan struct{})
	var emissionMu sync.Mutex
	emissionClosed := false
	debouncer := NewDebouncer(delay, nil, func(paths []string) {
		emissionMu.Lock()
		defer emissionMu.Unlock()
		if emissionClosed || ctx.Err() != nil {
			return
		}
		select {
		case changes <- Change{Paths: paths}:
		case <-ctx.Done():
		case <-done:
		}
	})

	go func() {
		defer func() {
			close(done)
			debouncer.Close()
			emissionMu.Lock()
			emissionClosed = true
			close(changes)
			emissionMu.Unlock()
		}()

		tracked := trackedDirectories(watcher)
		watchErrors := watcher.Errors
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename|fsnotify.Remove) == 0 {
					continue
				}
				path, err := filepath.Abs(event.Name)
				if err != nil {
					continue
				}
				path = filepath.Clean(path)
				if event.Op&fsnotify.Create != 0 {
					if info, err := os.Stat(path); err == nil && info.IsDir() {
						if relative, insideRoot := relativeToRoot(filter.root, path); insideRoot {
							watchPath := filepath.ToSlash(relative)
							err := registerPath(watcher, filter.root, watchPath, filter.ignore, tracked, func(filePath string) {
								filePath = filepath.Clean(filePath)
								if filter.Allow(filePath) {
									debouncer.Add(filePath)
								}
							})
							if err == nil {
								tracked = trackedDirectories(watcher)
							}
						}
					}
				}
				if event.Op&(fsnotify.Rename|fsnotify.Remove) != 0 {
					removeTrackedDirectories(watcher, tracked, path)
				}
				if filter.Allow(path) {
					debouncer.Add(path)
				}
			case _, ok := <-watchErrors:
				if !ok {
					watchErrors = nil
				}
			}
		}
	}()

	return changes
}

func trackedDirectories(watcher *fsnotify.Watcher) map[string]struct{} {
	tracked := make(map[string]struct{})
	for _, path := range watcher.WatchList() {
		tracked[filepath.Clean(path)] = struct{}{}
	}
	return tracked
}

func relativeToRoot(root, path string) (string, bool) {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	return relative, true
}

func removeTrackedDirectories(watcher *fsnotify.Watcher, tracked map[string]struct{}, directory string) {
	for path := range tracked {
		if _, isChild := relativeToRoot(directory, path); !isChild {
			continue
		}
		watcher.Remove(path)
		delete(tracked, path)
	}
}
