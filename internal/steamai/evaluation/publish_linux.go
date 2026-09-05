//go:build linux

package evaluation

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

func publishDirectoryNoReplace(staging, target string) error {
	parentHandle, err := os.Open(filepath.Dir(target))
	if err != nil {
		return err
	}
	defer parentHandle.Close()
	if err := syscall.Mkdirat(int(parentHandle.Fd()), filepath.Base(target), 0o700); err != nil {
		if errors.Is(err, syscall.EEXIST) {
			return errors.New("evaluation run 已存在")
		}
		return err
	}
	if err := os.Rename(staging, target); err != nil {
		_ = os.Remove(target)
		return err
	}
	return nil
}
