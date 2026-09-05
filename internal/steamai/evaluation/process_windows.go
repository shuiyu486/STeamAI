//go:build windows

package evaluation

import (
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

const (
	createSuspended                   = 0x00000004
	createNewProcessGroup             = 0x00000200
	jobObjectExtendedLimitInformation = 9
	jobObjectLimitKillOnJobClose      = 0x00002000
	processSetQuota                   = 0x0100
	processTerminate                  = 0x0001
	processSetInformation             = 0x0200
	processSuspendResume              = 0x0800
)

var (
	modKernel32Process           = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObjectW         = modKernel32Process.NewProc("CreateJobObjectW")
	procSetInformationJobObject  = modKernel32Process.NewProc("SetInformationJobObject")
	procAssignProcessToJobObject = modKernel32Process.NewProc("AssignProcessToJobObject")
	procTerminateJobObject       = modKernel32Process.NewProc("TerminateJobObject")
	procNtResumeProcess          = syscall.NewLazyDLL("ntdll.dll").NewProc("NtResumeProcess")
)

type armProcessControl struct {
	job syscall.Handle
}

type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobObjectExtendedLimitInformationStruct struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

func configureArmProcess(cmd *exec.Cmd) (*armProcessControl, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup | createSuspended}
	job, _, err := procCreateJobObjectW.Call(0, 0)
	if job == 0 {
		return nil, err
	}
	control := &armProcessControl{job: syscall.Handle(job)}
	limit := jobObjectExtendedLimitInformationStruct{}
	limit.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
	ok, _, err := procSetInformationJobObject.Call(
		job, jobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limit)), unsafe.Sizeof(limit),
	)
	if ok == 0 {
		_ = control.close()
		return nil, err
	}
	return control, nil
}

func (control *armProcessControl) attach(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return os.ErrProcessDone
	}
	process, err := syscall.OpenProcess(processSetQuota|processTerminate|processSetInformation|processSuspendResume, false, uint32(cmd.Process.Pid))
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(process)
	if ok, _, err := procAssignProcessToJobObject.Call(uintptr(control.job), uintptr(process)); ok == 0 {
		return err
	}
	status, _, callErr := procNtResumeProcess.Call(uintptr(process))
	if status != 0 {
		if callErr != syscall.Errno(0) {
			return callErr
		}
		return syscall.Errno(status)
	}
	return nil
}

func (control *armProcessControl) cancel(cmd *exec.Cmd) error {
	if control.job == 0 {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		return cmd.Process.Kill()
	}
	if ok, _, err := procTerminateJobObject.Call(uintptr(control.job), 1); ok == 0 {
		return err
	}
	return nil
}

func (control *armProcessControl) close() error {
	if control.job == 0 {
		return nil
	}
	err := syscall.CloseHandle(control.job)
	control.job = 0
	return err
}
