package ctl

import (
	"reflect"
	"testing"
	"time"

	"github.com/reihanboo/ember/internal/sup"
)

func TestSnapshotLines(t *testing.T) {
	snapshot := sup.Snapshot{
		State:             sup.BuildFailed,
		PID:               817,
		LastBuildDuration: 1250 * time.Millisecond,
		LastBuildOK:       false,
		LastChangeTime:    time.Date(2026, time.July, 8, 9, 10, 11, 123456000, time.UTC),
	}
	want := []string{
		"state=BuildFailed",
		"pid=817",
		"last_build_ok=false",
		"last_build_ms=1250",
		"last_change=2026-07-08T09:10:11.123456Z",
	}
	if got := SnapshotLines(snapshot); !reflect.DeepEqual(got, want) {
		t.Errorf("SnapshotLines() = %#v, want %#v", got, want)
	}
}

func TestSnapshotLinesUsesEmptyLastChangeWhenUnset(t *testing.T) {
	want := []string{"state=Idle", "pid=0", "last_build_ok=false", "last_build_ms=0", "last_change="}
	if got := SnapshotLines(sup.Snapshot{}); !reflect.DeepEqual(got, want) {
		t.Errorf("SnapshotLines() = %#v, want %#v", got, want)
	}
}
