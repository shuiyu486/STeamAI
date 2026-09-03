//go:build windows

package learningbatch

import (
	"errors"
	"syscall"
)

const fileAttributeReparsePoint = 0x400

func rejectReparse(path string) error {
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attributes, err := syscall.GetFileAttributes(pointer)
	if err != nil {
		return err
	}
	if attributes&fileAttributeReparsePoint != 0 {
		return errors.New("拒绝 Windows reparse point")
	}
	return nil
}
