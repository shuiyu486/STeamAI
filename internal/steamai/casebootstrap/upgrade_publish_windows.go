//go:build windows

package casebootstrap

import (
	"errors"
	"fmt"
	"io/fs"
	"syscall"
	"unsafe"
)

const upgradeMoveWriteThrough = 0x8

var upgradeMoveFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")

// 不带 REPLACE_EXISTING 或 COPY_ALLOWED：目标存在或跨卷就停止，不覆盖、不复制删除。
func upgradeMove(source, target string) error {
	from, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	result, _, callErr := upgradeMoveFileEx.Call(
		uintptr(unsafe.Pointer(from)), uintptr(unsafe.Pointer(to)), upgradeMoveWriteThrough,
	)
	if result != 0 {
		return nil
	}
	if errors.Is(callErr, syscall.ERROR_ALREADY_EXISTS) || errors.Is(callErr, syscall.ERROR_FILE_EXISTS) {
		return fmt.Errorf("%s: %w", target, fs.ErrExist)
	}
	return fmt.Errorf("%s: %w", target, callErr)
}
