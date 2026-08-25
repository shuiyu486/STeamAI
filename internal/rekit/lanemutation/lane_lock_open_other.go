//go:build !windows

package lanemutation

import "os"

func openExclusiveLaneLockFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
}
