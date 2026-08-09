//go:build !windows

package sessionhost

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

type supervisionOwnerLease struct {
	file *os.File
}

func configureSupervisorCommand(_ *exec.Cmd) {}

func acquireSupervisionOwner(path string, probe bool) (*supervisionOwnerLease, bool, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, err
	}
	operation := syscall.LOCK_EX
	if probe {
		operation |= syscall.LOCK_NB
	}
	if err := syscall.Flock(int(file.Fd()), operation); err != nil {
		file.Close()
		if probe && errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, true, nil
		}
		return nil, false, err
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
	return errors.Join(syscall.Flock(int(file.Fd()), syscall.LOCK_UN), file.Close())
}
