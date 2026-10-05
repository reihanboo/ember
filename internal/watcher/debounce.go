package watcher

import (
	"sort"
	"sync"
	"time"
)

type Timer interface {
	Stop() bool
}

type Clock interface {
	AfterFunc(time.Duration, func()) Timer
}

type Debouncer struct {
	delay      time.Duration
	clock      Clock
	emit       func([]string)
	mu         sync.Mutex
	pending    map[string]struct{}
	timer      Timer
	generation uint64
}

type systemClock struct{}

func (systemClock) AfterFunc(delay time.Duration, callback func()) Timer {
	return time.AfterFunc(delay, callback)
}

func NewDebouncer(delay time.Duration, clock Clock, emit func([]string)) *Debouncer {
	if clock == nil {
		clock = systemClock{}
	}
	if emit == nil {
		emit = func([]string) {}
	}
	return &Debouncer{
		delay: delay,
		clock: clock,
		emit:  emit,
	}
}

func (d *Debouncer) Add(path string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.pending == nil {
		d.pending = make(map[string]struct{})
	}
	d.pending[path] = struct{}{}
	d.generation++
	generation := d.generation
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = d.clock.AfterFunc(d.delay, func() {
		d.flush(generation)
	})
}

func (d *Debouncer) flush(generation uint64) {
	d.mu.Lock()
	if generation != d.generation || len(d.pending) == 0 {
		d.mu.Unlock()
		return
	}

	batch := make([]string, 0, len(d.pending))
	for path := range d.pending {
		batch = append(batch, path)
	}
	sort.Strings(batch)
	d.pending = nil
	d.timer = nil
	d.mu.Unlock()

	d.emit(batch)
}
