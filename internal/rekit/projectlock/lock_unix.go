//go:build darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd

package projectlock

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func stableWorkstreamLockRoot() (string, error) {
	uid := os.Getuid()
	if uid < 0 {
		return "", fmt.Errorf("resolve stable workstream lock root: invalid uid %d", uid)
	}
	return filepath.Join("/tmp", fmt.Sprintf("rekit-%d", uid), "workstream-locks"), nil
}

func lockFile(fd uintptr, exclusive bool) error {
	mode := syscall.LOCK_SH
	if exclusive {
		mode = syscall.LOCK_EX
	}
	return syscall.Flock(int(fd), mode)
}

func unlockFile(fd uintptr) error {
	return syscall.Flock(int(fd), syscall.LOCK_UN)
}
