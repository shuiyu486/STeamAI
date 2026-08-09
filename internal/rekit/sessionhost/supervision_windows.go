//go:build windows

package sessionhost

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

const (
	supervisionLockExclusive   = 0x00000002
	supervisionLockFailFast    = 0x00000001
	supervisionDetached        = 0x00000008
	supervisionNewProcessGroup = 0x00000200
	supervisionBreakaway       = 0x01000000
)

var (
	supervisionKernel32   = syscall.NewLazyDLL("kernel32.dll")
	supervisionLockFile   = supervisionKernel32.NewProc("LockFileEx")
	supervisionUnlockFile = supervisionKernel32.NewProc("UnlockFileEx")
)

type supervisionOwnerLease struct {
	file *os.File
}

func configureSupervisorCommand(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= supervisionDetached | supervisionNewProcessGroup | supervisionBreakaway
	cmd.SysProcAttr.HideWindow = true
}

func acquireSupervisionOwner(path string, probe bool) (*supervisionOwnerLease, bool, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, err
	}
	overlapped := syscall.Overlapped{Offset: 0xfffffffe, OffsetHigh: 0x3fffffff}
	flags := uintptr(supervisionLockExclusive)
	if probe {
		flags |= supervisionLockFailFast
	}
	result, _, callErr := supervisionLockFile.Call(file.Fd(), flags, 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
	if result == 0 {
		file.Close()
		if probe && errors.Is(callErr, syscall.Errno(33)) {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("lock Claude supervisor owner lease: %w", callErr)
	}
	return &supervisionOwnerLease{file: file}, false, nil
}

func supervisionOwnerBusy(path string) (bool, error) {
	lease, busy, err := acquireSupervisionOwner(path, true)
	if lease != nil {
		_ = lease.Close()
	}
	return busy, err
}

func (lease *supervisionOwnerLease) Close() error {
	if lease == nil || lease.file == nil {
		return nil
	}
	file := lease.file
	lease.file = nil
	overlapped := syscall.Overlapped{Offset: 0xfffffffe, OffsetHigh: 0x3fffffff}
	result, _, callErr := supervisionUnlockFile.Call(file.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
	if result == 0 {
		return errors.Join(fmt.Errorf("unlock Claude supervisor owner lease: %w", callErr), file.Close())
	}
	return file.Close()
}
