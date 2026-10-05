package build

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

func buildHelperCommand(executable string) string {
	quoted := `"` + strings.ReplaceAll(executable, `"`, `""`) + `"`
	return quoted
}

func waitForBuildHelperExit(pid int, timeout time.Duration) error {
	process, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return nil
		}
		return fmt.Errorf("open build grandchild %d: %w", pid, err)
	}
	defer windows.CloseHandle(process)

	result, err := windows.WaitForSingleObject(process, uint32(timeout.Milliseconds()))
	if err != nil {
		return fmt.Errorf("wait for build grandchild %d: %w", pid, err)
	}
	if result != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("build grandchild %d still exists after %s", pid, timeout)
	}
	return nil
}
