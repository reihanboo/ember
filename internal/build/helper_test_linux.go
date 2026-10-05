package build

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"time"
)

func buildHelperCommand(executable string) string {
	quoted := "'" + strings.ReplaceAll(executable, "'", "'\\''") + "'"
	return quoted
}

func waitForBuildHelperExit(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if err != nil && !errors.Is(err, syscall.EPERM) {
			return fmt.Errorf("check build grandchild %d: %w", pid, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("build grandchild %d still exists after %s", pid, timeout)
}
