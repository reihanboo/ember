package main

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

var ignoreBreakCallback = syscall.NewCallback(func(event uint32) uintptr {
	if event == windows.CTRL_BREAK_EVENT {
		return 1
	}
	return 0
})

func ignoreTerm() {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	setConsoleCtrlHandler := kernel32.NewProc("SetConsoleCtrlHandler")
	if result, _, _ := setConsoleCtrlHandler.Call(ignoreBreakCallback, 1); result == 0 {
		fail("register Ctrl+Break handler failed")
	}
	fmt.Println(os.Getpid())
	sleepForever()
}
