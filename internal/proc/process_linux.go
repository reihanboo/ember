package proc

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

func createProcessJob() (uintptr, error) {
	return 0, nil
}

func assignProcessToJob(uintptr, int) error {
	return nil
}

func closeProcessJob(uintptr) error {
	return nil
}

func processSignal(state *os.ProcessState) os.Signal {
	status, ok := state.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() {
		return nil
	}
	return status.Signal()
}

func (p *Process) Stop(grace, timeout time.Duration) error {
	pgid := p.Pid()
	if pgid <= 0 {
		return errors.New("process has no pid")
	}
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("send SIGTERM to process group %d: %w", pgid, err)
	}

	processDone := make(chan struct{})
	go func() {
		p.Wait()
		close(processDone)
	}()
	if waitForProcessGroup(pgid, grace, processDone) {
		<-processDone
		return nil
	}
	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("send SIGKILL to process group %d: %w", pgid, err)
	}
	if !waitForProcessGroup(pgid, timeout, processDone) {
		return fmt.Errorf("process group %d did not stop within %s after SIGKILL", pgid, timeout)
	}
	<-processDone
	return nil
}

func waitForProcessGroup(pgid int, timeout time.Duration, processDone <-chan struct{}) bool {
	if !processGroupExists(pgid) {
		return true
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	done := processDone

	for {
		select {
		case <-done:
			done = nil
		case <-ticker.C:
			if !processGroupExists(pgid) {
				return true
			}
		case <-timer.C:
			return !processGroupExists(pgid)
		}
	}
}

func processGroupExists(pgid int) bool {
	err := syscall.Kill(-pgid, 0)
	return err == nil || !errors.Is(err, syscall.ESRCH)
}
