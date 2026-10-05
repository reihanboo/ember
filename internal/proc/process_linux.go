package proc

import (
	"context"
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

func (p *Process) Stop(ctx context.Context, timeout time.Duration) error {
	p.stopMu.Lock()
	defer p.stopMu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}

	pgid := p.Pid()
	if pgid <= 0 {
		return errors.New("process has no pid")
	}
	processDone := make(chan struct{})
	go func() {
		p.Wait()
		close(processDone)
	}()

	termErr := syscall.Kill(-pgid, syscall.SIGTERM)
	if termErr == nil || errors.Is(termErr, syscall.ESRCH) {
		if waitForProcessGroup(ctx, pgid, timeout, processDone) {
			<-processDone
			return nil
		}
	}
	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("send SIGKILL to process group %d: %w", pgid, err)
	}
	waitForProcessGroupGone(pgid, processDone)
	<-processDone
	return nil
}

func waitForProcessGroup(ctx context.Context, pgid int, timeout time.Duration, processDone <-chan struct{}) bool {
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
		case <-ctx.Done():
			return !processGroupExists(pgid)
		case <-ticker.C:
			if !processGroupExists(pgid) {
				return true
			}
		case <-timer.C:
			return !processGroupExists(pgid)
		}
	}
}

func waitForProcessGroupGone(pgid int, processDone <-chan struct{}) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	done := processDone
	for processGroupExists(pgid) {
		select {
		case <-done:
			done = nil
		case <-ticker.C:
		}
	}
}

func processGroupExists(pgid int) bool {
	err := syscall.Kill(-pgid, 0)
	return err == nil || !errors.Is(err, syscall.ESRCH)
}
