package watcher

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/reihanboo/ember/internal/config"
	"github.com/reihanboo/ember/internal/ui"
)

const pollInterval = 500 * time.Millisecond

type Poller struct {
	root      string
	paths     []string
	filter    *Filter
	logger    warningLogger
	closed    chan struct{}
	closeOnce sync.Once
}

type fileState struct {
	modTime int64
	size    int64
}

func NewPoller(root string, settings config.WatchConfig) (*Poller, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve project root %q: %w", root, err)
	}
	filter, err := NewFilter(absoluteRoot, settings.Include, settings.Ignore)
	if err != nil {
		return nil, err
	}
	for _, watchPath := range settings.Paths {
		path := filepath.Join(absoluteRoot, filepath.FromSlash(watchPath))
		info, err := os.Stat(path)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("watch path %q does not exist: %w", watchPath, err)
		}
		if err != nil {
			return nil, fmt.Errorf("inspect watch path %q: %w", watchPath, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("watch path %q is not a directory", watchPath)
		}
	}
	return &Poller{
		root:   filepath.Clean(absoluteRoot),
		paths:  append([]string(nil), settings.Paths...),
		filter: filter,
		logger: ui.NewLogger(os.Stderr, time.Now, false, ui.WarnLevel),
		closed: make(chan struct{}),
	}, nil
}

func (p *Poller) Watch(ctx context.Context, delay time.Duration) <-chan Change {
	previous, err := p.scan()
	if err != nil {
		p.logger.Warn(fmt.Sprintf("polling watcher scan failed: %v", err))
		previous = make(map[string]fileState)
	}

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

		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.closed:
				return
			case <-ticker.C:
				current, err := p.scan()
				if err != nil {
					p.logger.Warn(fmt.Sprintf("polling watcher scan failed: %v", err))
					continue
				}
				for path, state := range current {
					if previousState, exists := previous[path]; !exists || previousState != state {
						debouncer.Add(path)
					}
				}
				for path := range previous {
					if _, exists := current[path]; !exists {
						debouncer.Add(path)
					}
				}
				previous = current
			}
		}
	}()

	return changes
}

func (p *Poller) Close() error {
	p.closeOnce.Do(func() {
		close(p.closed)
	})
	return nil
}

func (p *Poller) scan() (map[string]fileState, error) {
	files := make(map[string]fileState)
	for _, watchPath := range p.paths {
		path := filepath.Join(p.root, filepath.FromSlash(watchPath))
		err := filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
			if errors.Is(walkErr, fs.ErrNotExist) {
				return nil
			}
			if walkErr != nil {
				return fmt.Errorf("walk watch path %q: %w", watchPath, walkErr)
			}
			if entry.IsDir() {
				if ignoredDirectory(p.root, current, p.filter.ignore) {
					return filepath.SkipDir
				}
				return nil
			}
			info, err := entry.Info()
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			if err != nil {
				return fmt.Errorf("stat watched file %q: %w", current, err)
			}
			if !info.Mode().IsRegular() || !p.filter.Allow(current) {
				return nil
			}
			files[filepath.Clean(current)] = fileState{modTime: info.ModTime().UnixNano(), size: info.Size()}
			return nil
		})
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

var _ ChangeWatcher = (*Poller)(nil)
var _ ChangeWatcher = (*FSNotifyWatcher)(nil)
