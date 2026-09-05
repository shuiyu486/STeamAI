//go:build windows

package evaluation

import (
	"errors"
	"syscall"
	"unsafe"
)

var (
	modKernel32Evaluation     = syscall.NewLazyDLL("kernel32.dll")
	procMoveFileExWEvaluation = modKernel32Evaluation.NewProc("MoveFileExW")
)

func publishDirectoryNoReplace(staging, target string) error {
	stagingPtr, err := syscall.UTF16PtrFromString(staging)
	if err != nil {
		return err
	}
	targetPtr, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	result, _, callErr := procMoveFileExWEvaluation.Call(
		uintptr(unsafe.Pointer(stagingPtr)),
		uintptr(unsafe.Pointer(targetPtr)),
		0,
	)
	if result != 0 {
		return nil
	}
	if errors.Is(callErr, syscall.ERROR_ALREADY_EXISTS) || errors.Is(callErr, syscall.ERROR_FILE_EXISTS) {
		return errors.New("evaluation run 已存在")
	}
	return callErr
}
