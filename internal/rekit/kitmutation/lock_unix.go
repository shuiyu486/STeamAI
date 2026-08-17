//go:build darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd

package kitmutation

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func stableLockRoot() (string, error) {
	uid := os.Getuid()
	if uid < 0 {
		return "", fmt.Errorf("resolve stable kit mutation lock root: invalid uid %d", uid)
	}
	return filepath.Join("/tmp", fmt.Sprintf("rekit-%d", uid), "kit-mutation-locks"), nil
}

func lockFile(fd uintptr) error {
	return syscall.Flock(int(fd), syscall.LOCK_EX)
}

func unlockFile(fd uintptr) error {
	return syscall.Flock(int(fd), syscall.LOCK_UN)
}
