package proc

import "strings"

func processOutputCommand(executable string) string {
	return "'" + strings.ReplaceAll(executable, "'", "'\\''") + "'"
}
