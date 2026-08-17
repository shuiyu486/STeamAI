//go:build !windows && !darwin && !dragonfly && !freebsd && !illumos && !linux && !netbsd && !openbsd

package projectlock

import "fmt"

func stableWorkstreamLockRoot() (string, error) {
	return "", fmt.Errorf("project lock is unsupported on this platform")
}

func lockFile(uintptr, bool) error {
	return fmt.Errorf("project lock is unsupported on this platform")
}

func unlockFile(uintptr) error {
	return fmt.Errorf("project unlock is unsupported on this platform")
}
