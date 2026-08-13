//go:build windows

package adapterhost

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

type liveAcceptanceFileDispositionInformation struct {
	DeleteFile byte
}

func removeEmptyLiveAcceptanceQuarantine(
	identity *liveAcceptanceCaseIdentity,
	quarantine string,
	root *os.Root,
) error {
	opened, err := root.Lstat(".")
	if err != nil || !opened.IsDir() || !os.SameFile(identity.caseInfo, opened) {
		return fmt.Errorf("adapter live quarantined case identity changed before final removal: %s", filepath.Join(identity.parentPath, quarantine))
	}
	path := filepath.Join(identity.parentPath, quarantine)
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	const (
		deleteAccess   = 0x00010000
		readAttributes = 0x00000080
	)
	handle, err := syscall.CreateFile(
		pointer,
		deleteAccess|readAttributes,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_FLAG_OPEN_REPARSE_POINT|syscall.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		syscall.CloseHandle(handle)
		return fmt.Errorf("open exact adapter live quarantine handle: %s", path)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.IsDir() || !os.SameFile(identity.caseInfo, info) {
		return fmt.Errorf("adapter live quarantined case identity changed before handle-bound removal: %s", path)
	}
	var native syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &native); err != nil {
		return err
	}
	if native.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("adapter live quarantine became a reparse point before removal: %s", path)
	}
	disposition := liveAcceptanceFileDispositionInformation{DeleteFile: 1}
	result, _, callErr := setFileInformationByHandle.Call(
		uintptr(handle),
		4,
		uintptr(unsafe.Pointer(&disposition)),
		unsafe.Sizeof(disposition),
	)
	if result == 0 {
		return fmt.Errorf("delete exact adapter live quarantine: %s: %v", path, callErr)
	}
	return nil
}
