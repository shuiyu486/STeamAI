//go:build !windows && !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package workstream

import "fmt"

func stableWorkstreamLockRoot() (string, error) {
	return "", fmt.Errorf("workstream mutation lock is unsupported on this platform")
}

func lockLaneLeaseFile(uintptr, bool) error {
	return fmt.Errorf("lane mutation lock is unsupported on this platform")
}

func unlockLaneLeaseFile(uintptr) error {
	return fmt.Errorf("lane mutation unlock is unsupported on this platform")
}
