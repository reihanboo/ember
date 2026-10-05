package watcher

import (
	"context"
	"fmt"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/reihanboo/ember/internal/config"
)

type ChangeWatcher interface {
	Watch(context.Context, time.Duration) <-chan Change
	Close() error
}

type FSNotifyWatcher struct {
	watcher *fsnotify.Watcher
	filter  *Filter
}

func NewWatcher(root string, settings config.WatchConfig) (ChangeWatcher, error) {
	if settings.Poll {
		return NewPoller(root, settings)
	}
	return NewFSNotifyWatcher(root, settings)
}

func NewFSNotifyWatcher(root string, settings config.WatchConfig) (*FSNotifyWatcher, error) {
	filter, err := NewFilter(root, settings.Include, settings.Ignore)
	if err != nil {
		return nil, err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}
	if err := RegisterDirectories(watcher, root, settings.Paths, settings.Ignore); err != nil {
		watcher.Close()
		return nil, err
	}
	return &FSNotifyWatcher{watcher: watcher, filter: filter}, nil
}

func (w *FSNotifyWatcher) Watch(ctx context.Context, delay time.Duration) <-chan Change {
	return Watch(ctx, w.watcher, w.filter, delay)
}

func (w *FSNotifyWatcher) Close() error {
	return w.watcher.Close()
}
