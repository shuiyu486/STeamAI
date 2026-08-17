//go:build !windows && !darwin && !dragonfly && !freebsd && !illumos && !linux && !netbsd && !openbsd

package kitmutation

import "fmt"

func stableLockRoot() (string, error) {
	return "", fmt.Errorf("kit mutation lock is unsupported on this platform")
}

func lockFile(uintptr) error {
	return fmt.Errorf("kit mutation lock is unsupported on this platform")
}

func unlockFile(uintptr) error {
	return fmt.Errorf("kit mutation unlock is unsupported on this platform")
}
