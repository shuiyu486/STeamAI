//go:build windows

package missionintent

import (
	"fmt"
	"syscall"
)

func rejectReparsePath(path string) error {
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attributes, err := syscall.GetFileAttributes(pointer)
	if err != nil {
		return err
	}
	if attributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("onboarding path must not be a reparse point: %s", path)
	}
	return nil
}
