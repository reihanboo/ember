package watcher

import (
	"fmt"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
}

type fakeTimer struct {
	clock  *fakeClock
	due    time.Time
	fn     func()
	active bool
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Unix(0, 0)}
}

func (c *fakeClock) AfterFunc(delay time.Duration, fn func()) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	timer := &fakeTimer{clock: c, due: c.now.Add(delay), fn: fn, active: true}
	c.timers = append(c.timers, timer)
	return timer
}

func (t *fakeTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	wasActive := t.active
	t.active = false
	return wasActive
}

func (c *fakeClock) Advance(duration time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(duration)
	c.mu.Unlock()

	for {
		c.mu.Lock()
		due := make([]*fakeTimer, 0)
		for _, timer := range c.timers {
			if timer.active && !timer.due.After(c.now) {
				timer.active = false
				due = append(due, timer)
			}
		}
		c.mu.Unlock()

		if len(due) == 0 {
			return
		}
		sort.Slice(due, func(i, j int) bool {
			return due[i].due.Before(due[j].due)
		})
		for _, timer := range due {
			timer.fn()
		}
	}
}

func TestDebouncerFiftyRapidEventsProduceOneBatch(t *testing.T) {
	const delay = 100 * time.Millisecond
	clock := newFakeClock()
	var batches [][]string
	debouncer := NewDebouncer(delay, clock, func(batch []string) {
		batches = append(batches, batch)
	})

	want := make([]string, 50)
	for i := range want {
		want[i] = fmt.Sprintf("file-%02d.c", i)
		debouncer.Add(want[i])
	}
	sort.Strings(want)

	clock.Advance(delay - time.Nanosecond)
	if len(batches) != 0 {
		t.Fatalf("batches before quiet period = %d, want 0", len(batches))
	}
	clock.Advance(time.Nanosecond)
	if len(batches) != 1 {
		t.Fatalf("batch count = %d, want 1", len(batches))
	}
	if !reflect.DeepEqual(batches[0], want) {
		t.Errorf("batch = %#v, want %#v", batches[0], want)
	}
}

func TestDebouncerSeparatesBursts(t *testing.T) {
	const delay = 100 * time.Millisecond
	clock := newFakeClock()
	var batches [][]string
	debouncer := NewDebouncer(delay, clock, func(batch []string) {
		batches = append(batches, batch)
	})

	debouncer.Add("first.c")
	clock.Advance(delay + time.Nanosecond)
	debouncer.Add("second.c")
	clock.Advance(delay + time.Nanosecond)

	if len(batches) != 2 {
		t.Fatalf("batch count = %d, want 2", len(batches))
	}
	if !reflect.DeepEqual(batches, [][]string{{"first.c"}, {"second.c"}}) {
		t.Errorf("batches = %#v, want [[first.c] [second.c]]", batches)
	}
}

func TestDebouncerDeduplicatesPaths(t *testing.T) {
	const delay = 100 * time.Millisecond
	clock := newFakeClock()
	var batches [][]string
	debouncer := NewDebouncer(delay, clock, func(batch []string) {
		batches = append(batches, batch)
	})

	debouncer.Add("src/main.c")
	debouncer.Add("src/main.c")
	debouncer.Add("src/header.h")
	debouncer.Add("src/main.c")
	clock.Advance(delay)

	if !reflect.DeepEqual(batches, [][]string{{"src/header.h", "src/main.c"}}) {
		t.Errorf("batches = %#v, want one batch of unique sorted paths", batches)
	}
}

func TestDebouncerCloseStopsPendingBatch(t *testing.T) {
	clock := newFakeClock()
	var batches [][]string
	debouncer := NewDebouncer(time.Hour, clock, func(batch []string) {
		batches = append(batches, batch)
	})

	debouncer.Add("pending.c")
	debouncer.Close()
	clock.Advance(2 * time.Hour)
	debouncer.Add("after-close.c")

	if len(batches) != 0 {
		t.Errorf("batches after close = %#v, want none", batches)
	}
}
