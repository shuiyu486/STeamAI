//go:build windows

package casebootstrap

import (
	"errors"
	"syscall"
)

func rejectReparse(path string) error {
	value, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attributes, err := syscall.GetFileAttributes(value)
	if err != nil {
		return err
	}
	if attributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return errors.New("拒绝 symlink/junction/reparse 路径")
	}
	return nil
}
