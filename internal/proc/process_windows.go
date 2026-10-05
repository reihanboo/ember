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

func (p *Process) Stop(_, _ time.Duration) error {
	p.jobMu.Lock()
	job := p.jobHandle
	var err error
	if job != 0 {
		err = windows.TerminateJobObject(windows.Handle(job), 1)
	}
	p.jobMu.Unlock()
	if err != nil {
		return fmt.Errorf("terminate process job: %w", err)
	}
	p.Wait()
	return nil
}

func processSignal(*os.ProcessState) os.Signal {
	return nil
}
