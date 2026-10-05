package watcher

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/reihanboo/ember/internal/ui"
)

func TestWatchErrorLogsWarningAndEmitsSyntheticChange(t *testing.T) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()
	filter, err := NewFilter(t.TempDir(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcherErrors := make(chan error, 1)
	var logOutput bytes.Buffer
	fixedTime := time.Date(2025, time.April, 5, 6, 7, 8, 0, time.UTC)
	logger := ui.NewLogger(&logOutput, func() time.Time { return fixedTime }, false, ui.WarnLevel)
	changes := watch(ctx, watcher, filter, time.Hour, watcher.Events, watcherErrors, logger)

	watcherErrors <- errors.New("queue overflow")
	change := receiveChange(t, changes)
	if change.Paths != nil {
		t.Errorf("synthetic change paths = %#v, want nil", change.Paths)
	}
	if got, want := logOutput.String(), "2025-04-05T06:07:08Z [WARN] fsnotify watcher error: queue overflow\n"; got != want {
		t.Errorf("warning log = %q, want %q", got, want)
	}
	assertNoChange(t, changes, 50*time.Millisecond)

	cancel()
	waitForClose(t, changes)
}
