package watcher

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/reihanboo/ember/internal/ui"
)

var wslWarningOnce sync.Once

func warnForWSLMount(root string, poll bool) {
	if poll {
		return
	}
	if absoluteRoot, err := filepath.Abs(root); err == nil {
		root = absoluteRoot
	}
	if !isWSLMount(root) {
		return
	}
	wslWarningOnce.Do(func() {
		logger := ui.NewLogger(os.Stderr, time.Now, false, ui.WarnLevel)
		logger.Warn("inotify may be unreliable under WSL mounts; move the project to your ext4 home directory or set watch.poll = true")
	})
}
