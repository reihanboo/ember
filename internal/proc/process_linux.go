package proc

import (
	"os"
	"syscall"
)

func processSignal(state *os.ProcessState) os.Signal {
	status, ok := state.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() {
		return nil
	}
	return status.Signal()
}
