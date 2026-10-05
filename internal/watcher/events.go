package watcher

import (
	"context"
	"path/filepath"
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
				if err != nil || !filter.Allow(path) {
					continue
				}
				debouncer.Add(filepath.Clean(path))
			case _, ok := <-watchErrors:
				if !ok {
					watchErrors = nil
				}
			}
		}
	}()

	return changes
}
