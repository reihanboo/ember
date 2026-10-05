package ui

import "golang.org/x/sys/windows"

func consoleSupportsColor() bool {
	handle, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil || handle == 0 {
		return false
	}

	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return false
	}
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
		return true
	}
	return windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) == nil
}
