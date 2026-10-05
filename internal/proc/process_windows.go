package proc

import (
	"errors"
	"fmt"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func createProcessJob() (uintptr, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, fmt.Errorf("create job object: %w", err)
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)),
		uint32(unsafe.Sizeof(limits)),
	); err != nil {
		windows.CloseHandle(job)
		return 0, fmt.Errorf("set job object limits: %w", err)
	}
	return uintptr(job), nil
}

func assignProcessToJob(job uintptr, pid int) error {
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return fmt.Errorf("open process %d: %w", pid, err)
	}
	assignErr := windows.AssignProcessToJobObject(windows.Handle(job), process)
	closeErr := windows.CloseHandle(process)
	if assignErr != nil {
		return errors.Join(fmt.Errorf("assign process %d: %w", pid, assignErr), closeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close process handle %d: %w", pid, closeErr)
	}
	return nil
}

func closeProcessJob(job uintptr) error {
	if job == 0 {
		return nil
	}
	return windows.CloseHandle(windows.Handle(job))
}

func (p *Process) Stop(grace, timeout time.Duration) error {
	p.jobMu.Lock()
	job := p.jobHandle
	if job == 0 {
		p.jobMu.Unlock()
		p.Wait()
		return nil
	}
	p.jobHeld = true
	p.jobMu.Unlock()

	processDone := make(chan struct{})
	go func() {
		p.Wait()
		close(processDone)
	}()

	breakErr := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(p.Pid()))
	if breakErr == nil {
		stopped, err := waitForJobAndProcess(job, processDone, grace)
		if err == nil && stopped {
			return p.releaseJob()
		}
	}

	if err := windows.TerminateJobObject(windows.Handle(job), 1); err != nil {
		closeErr := p.releaseJob()
		return errors.Join(fmt.Errorf("terminate process job: %w", err), closeErr)
	}
	stopped, waitErr := waitForJobAndProcess(job, processDone, timeout)
	closeErr := p.releaseJob()
	if waitErr != nil {
		return errors.Join(fmt.Errorf("wait for process job: %w", waitErr), closeErr)
	}
	if !stopped {
		return errors.Join(fmt.Errorf("process %d did not stop within %s after job termination", p.Pid(), timeout), closeErr)
	}
	return closeErr
}

type windowsJobAccounting struct {
	totalUserTime            int64
	totalKernelTime          int64
	periodTotalUserTime      int64
	periodTotalKernelTime    int64
	totalPageFaultCount      uint32
	totalProcesses           uint32
	activeProcesses          uint32
	totalTerminatedProcesses uint32
}

func waitForJobAndProcess(job uintptr, processDone <-chan struct{}, timeout time.Duration) (bool, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	done := processDone

	for {
		var accounting windowsJobAccounting
		if err := windows.QueryInformationJobObject(
			windows.Handle(job),
			windows.JobObjectBasicAccountingInformation,
			uintptr(unsafe.Pointer(&accounting)),
			uint32(unsafe.Sizeof(accounting)),
			nil,
		); err != nil {
			return false, fmt.Errorf("query job object process count: %w", err)
		}
		if accounting.activeProcesses == 0 && done == nil {
			return true, nil
		}

		select {
		case <-done:
			done = nil
		case <-ticker.C:
		case <-timer.C:
			return false, nil
		}
	}
}

func processSignal(*os.ProcessState) os.Signal {
	return nil
}
