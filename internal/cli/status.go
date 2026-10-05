package cli

import (
	"fmt"
	"strings"
)

func renderStatus(lines []string) ([]string, error) {
	values := make(map[string]string, len(lines))
	for _, line := range lines {
		key, value, ok := strings.Cut(line, "=")
		if !ok || key == "" {
			return nil, fmt.Errorf("invalid status line %q", line)
		}
		switch key {
		case "state", "pid", "last_build_ok", "last_build_ms", "last_change":
		default:
			return nil, fmt.Errorf("unknown status key %q", key)
		}
		if _, exists := values[key]; exists {
			return nil, fmt.Errorf("duplicate status key %q", key)
		}
		values[key] = value
	}
	for _, key := range []string{"state", "pid", "last_build_ok", "last_build_ms", "last_change"} {
		if _, exists := values[key]; !exists {
			return nil, fmt.Errorf("missing status key %q", key)
		}
	}
	lastChange := values["last_change"]
	if lastChange == "" {
		lastChange = "never"
	}
	return []string{
		"State: " + values["state"],
		"PID: " + values["pid"],
		"Last build OK: " + values["last_build_ok"],
		"Last build duration: " + values["last_build_ms"] + " ms",
		"Last change: " + lastChange,
	}, nil
}
