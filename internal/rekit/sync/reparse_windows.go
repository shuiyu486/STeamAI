//go:build windows

package sync

import (
	"fmt"
	"syscall"
)

func rejectExclusiveInitReparsePath(path string) error {
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attributes, err := syscall.GetFileAttributes(pointer)
	if err == syscall.ERROR_FILE_NOT_FOUND || err == syscall.ERROR_PATH_NOT_FOUND {
		return nil
	}
	if err != nil {
		return err
	}
	if attributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("exclusive init root parent or planned path must not be a reparse point: %s", path)
	}
	return nil
}
