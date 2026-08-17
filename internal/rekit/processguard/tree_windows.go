//go:build windows

package processguard

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func runTree(ctx context.Context, cmd *exec.Cmd, afterStart func() error) error {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("create process tree containment job: %w", err)
	}
	jobOpen := true
	closeJob := func() error {
		if !jobOpen {
			return nil
		}
		jobOpen = false
		return windows.CloseHandle(job)
	}
	defer closeJob()

	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)),
		uint32(unsafe.Sizeof(limits)),
	); err != nil {
		return fmt.Errorf("set process tree containment job limits: %w", err)
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED}
	if err := cmd.Start(); err != nil {
		return err
	}
	assigned := false
	failStarted := func(cause error) error {
		var terminateErr error
		var drainErr error
		if assigned {
			terminateErr = terminateJob(job)
		}
		killErr := cmd.Process.Kill()
		if errors.Is(killErr, os.ErrProcessDone) {
			killErr = nil
		}
		waitErr := cmd.Wait()
		if assigned {
			drainErr = waitForJobEmpty(job, 5*time.Second)
		}
		closeErr := closeJob()
		return errors.Join(cause, terminateErr, killErr, waitErr, drainErr, closeErr)
	}

	var prepareErr error
	if err := cmd.Process.WithHandle(func(handle uintptr) {
		processHandle := windows.Handle(handle)
		if err := windows.AssignProcessToJobObject(job, processHandle); err != nil {
			prepareErr = fmt.Errorf("assign suspended process to containment job: %w", err)
			return
		}
		assigned = true
		if err := treeContextError(ctx); err != nil {
			prepareErr = err
			return
		}
		if afterStart != nil {
			if err := afterStart(); err != nil {
				prepareErr = fmt.Errorf("observe contained process before resume: %w", err)
				return
			}
		}
		status, _, callErr := ntResumeProcess.Call(handle)
		if int32(status) < 0 {
			prepareErr = fmt.Errorf(
				"resume contained process tree: NTSTATUS 0x%s: %v",
				strconv.FormatUint(uint64(uint32(status)), 16),
				callErr,
			)
		}
	}); err != nil {
		return failStarted(fmt.Errorf("access suspended process for tree containment: %w", err))
	}
	if prepareErr != nil {
		return failStarted(prepareErr)
	}

	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()
	rootExited := false
	var waitErr error
	var contextErr error
	select {
	case waitErr = <-waitDone:
		rootExited = true
	case <-ctx.Done():
		contextErr = treeContextError(ctx)
	}

	terminateErr := terminateJob(job)
	var killErr error
	if contextErr != nil && terminateErr != nil {
		killErr = cmd.Process.Kill()
		if errors.Is(killErr, os.ErrProcessDone) {
			killErr = nil
		}
	}
	if !rootExited {
		waitErr = <-waitDone
	}
	if errors.Is(waitErr, exec.ErrWaitDelay) {
		waitErr = nil
	}
	drainErr := waitForJobEmpty(job, 5*time.Second)
	closeErr := closeJob()
	return errors.Join(waitErr, contextErr, terminateErr, killErr, drainErr, closeErr)
}

type jobObjectBasicAccountingInformation struct {
	TotalUserTime             int64
	TotalKernelTime           int64
	ThisPeriodTotalUserTime   int64
	ThisPeriodTotalKernelTime int64
	TotalPageFaultCount       uint32
	TotalProcesses            uint32
	ActiveProcesses           uint32
	TotalTerminatedProcesses  uint32
}

func jobActiveProcesses(job windows.Handle) (uint32, error) {
	accounting := jobObjectBasicAccountingInformation{}
	if err := windows.QueryInformationJobObject(
		job,
		windows.JobObjectBasicAccountingInformation,
		uintptr(unsafe.Pointer(&accounting)),
		uint32(unsafe.Sizeof(accounting)),
		nil,
	); err != nil {
		return 0, err
	}
	return accounting.ActiveProcesses, nil
}

func terminateJob(job windows.Handle) error {
	active, err := jobActiveProcesses(job)
	if err != nil {
		return fmt.Errorf("query process tree before termination: %w", err)
	}
	if active == 0 {
		return nil
	}
	if err := windows.TerminateJobObject(job, 1); err != nil {
		remaining, queryErr := jobActiveProcesses(job)
		if queryErr == nil && remaining == 0 {
			return nil
		}
		return fmt.Errorf("terminate process tree: %w", errors.Join(err, queryErr))
	}
	return nil
}

func waitForJobEmpty(job windows.Handle, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		active, err := jobActiveProcesses(job)
		if err != nil {
			return fmt.Errorf("query terminated process tree: %w", err)
		}
		if active == 0 {
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf(
				"process tree did not terminate within %s: active=%d",
				timeout,
				active,
			)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
