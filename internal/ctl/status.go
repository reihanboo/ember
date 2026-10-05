package ctl

import (
	"strconv"
	"time"

	"github.com/reihanboo/ember/internal/sup"
)

func SnapshotLines(snapshot sup.Snapshot) []string {
	lastChange := ""
	if !snapshot.LastChangeTime.IsZero() {
		lastChange = snapshot.LastChangeTime.Format(time.RFC3339Nano)
	}
	return []string{
		"state=" + snapshot.State.String(),
		"pid=" + strconv.Itoa(snapshot.PID),
		"last_build_ok=" + strconv.FormatBool(snapshot.LastBuildOK),
		"last_build_ms=" + strconv.FormatInt(snapshot.LastBuildDuration.Milliseconds(), 10),
		"last_change=" + lastChange,
	}
}
