//go:build windows

package releasecheck

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	kernel32ReplaceFile = syscall.NewLazyDLL("kernel32.dll").NewProc("ReplaceFileW")
	kernel32MoveFileEx  = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")
	replaceFileCall     = func(destination, replacement uintptr) (uintptr, error) {
		result, _, err := kernel32ReplaceFile.Call(destination, replacement, 0, 0, 0, 0)
		return result, err
	}
	moveFileExCall = func(source, destination, flags uintptr) (uintptr, error) {
		result, _, err := kernel32MoveFileEx.Call(source, destination, flags)
		return result, err
	}
)

const (
	moveFileReplaceExisting = 0x1
	moveFileWriteThrough    = 0x8
)

func replaceLocalValidationReceipt(tempPath, path string) error {
	temp, err := syscall.UTF16PtrFromString(tempPath)
	if err != nil {
		return err
	}
	destination, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	if replaced, replaceErr := replaceFileCall(
		uintptr(unsafe.Pointer(destination)),
		uintptr(unsafe.Pointer(temp)),
	); replaced != 0 {
		return nil
	} else if errno, ok := replaceErr.(syscall.Errno); !ok || errno != syscall.ERROR_FILE_NOT_FOUND {
		return fmt.Errorf("replace local validation receipt: %w", replaceErr)
	}
	moved, moveErr := moveFileExCall(
		uintptr(unsafe.Pointer(temp)),
		uintptr(unsafe.Pointer(destination)),
		moveFileReplaceExisting|moveFileWriteThrough,
	)
	if moved == 0 {
		return fmt.Errorf("publish local validation receipt: %w", moveErr)
	}
	return nil
}
