//go:build windows

package evaluation

import (
	"errors"
	"syscall"
)

func rejectReparse(path string) error {
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attributes, err := syscall.GetFileAttributes(pointer)
	if err != nil {
		return err
	}
	if attributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return errors.New("拒绝 Windows reparse point")
	}
	return nil
}
