package proc

import (
	"strings"
	"testing"
)

func commandOutput(t *testing.T, command, cwd string, env map[string]string) string {
	t.Helper()
	output, err := Command(command, cwd, env).Output()
	if err != nil {
		t.Fatalf("Command(%q).Output() error = %v", command, err)
	}
	return strings.TrimSpace(string(output))
}
