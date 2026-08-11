//go:build windows

package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

type ordinaryRollbackFileDispositionInformation struct {
	DeleteFile byte
}

var (
	ordinaryRollbackKernel32                   = syscall.NewLazyDLL("kernel32.dll")
	ordinaryRollbackSetFileInformationByHandle = ordinaryRollbackKernel32.NewProc("SetFileInformationByHandle")
)

func ordinaryInitRollbackCapabilityCheck() error { return nil }

func ordinaryInitOpenRollbackHandle(path string, directory bool) (*os.File, error) {
	return ordinaryInitOpenWindowsHandle(
		path,
		directory,
		0x00000080,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_DELETE,
	)
}

func ordinaryInitOpenWindowsHandle(path string, directory bool, access, share uint32) (*os.File, error) {
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	flags := uint32(syscall.FILE_FLAG_OPEN_REPARSE_POINT)
	if directory {
		flags |= syscall.FILE_FLAG_BACKUP_SEMANTICS
	}
	handle, err := syscall.CreateFile(
		pointer,
		access,
		share,
		nil,
		syscall.OPEN_EXISTING,
		flags,
		0,
	)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		syscall.CloseHandle(handle)
		return nil, fmt.Errorf("ordinary init rollback wrap exact object handle: %s", path)
	}
	return file, nil
}

func ordinaryInitRemoveExact(root *os.Root, rel, _ string, handle *os.File, expected os.FileInfo, directory bool, afterIdentity func(string) error) (bool, error) {
	if root == nil || handle == nil || expected == nil {
		return false, fmt.Errorf("ordinary init rollback identity is incomplete: %s", rel)
	}
	current, err := handle.Stat()
	if err != nil || current.IsDir() != directory || !os.SameFile(expected, current) {
		return false, fmt.Errorf("ordinary init rollback handle identity changed: %s", rel)
	}
	const (
		deleteAccess   = 0x00010000
		readAttributes = 0x00000080
	)
	deletion, err := ordinaryInitOpenWindowsHandle(
		filepath.Join(root.Name(), filepath.FromSlash(rel)),
		directory,
		deleteAccess|readAttributes,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_DELETE,
	)
	if err != nil {
		return false, fmt.Errorf("ordinary init rollback open exact object %s for deletion: %w", rel, err)
	}
	deletionInfo, err := deletion.Stat()
	if err != nil || deletionInfo.IsDir() != directory || !os.SameFile(expected, deletionInfo) {
		deletion.Close()
		return false, fmt.Errorf("ordinary init rollback deletion identity changed: %s", rel)
	}
	if afterIdentity != nil {
		if err := afterIdentity(rel); err != nil {
			deletion.Close()
			return false, err
		}
	}

	rebound := false
	var reboundInfo os.FileInfo
	if named, err := root.Lstat(rel); err == nil {
		if !os.SameFile(expected, named) {
			rebound = true
			reboundInfo = named
		}
	} else if !os.IsNotExist(err) {
		deletion.Close()
		return false, err
	}

	disposition := ordinaryRollbackFileDispositionInformation{DeleteFile: 1}
	const fileDispositionInfo = 4
	removed, _, callErr := ordinaryRollbackSetFileInformationByHandle.Call(
		deletion.Fd(),
		fileDispositionInfo,
		uintptr(unsafe.Pointer(&disposition)),
		unsafe.Sizeof(disposition),
	)
	if removed == 0 {
		deletion.Close()
		return rebound, fmt.Errorf("ordinary init rollback remove exact object %s: %v", rel, callErr)
	}
	if err := handle.Close(); err != nil {
		deletion.Close()
		return rebound, err
	}
	if err := deletion.Close(); err != nil {
		return rebound, err
	}
	if rebound {
		named, err := root.Lstat(rel)
		if err != nil || !os.SameFile(reboundInfo, named) {
			return true, fmt.Errorf("ordinary init rollback replacement changed while preserving it: %s: %w", rel, err)
		}
	} else if _, err := root.Lstat(rel); !os.IsNotExist(err) {
		return false, fmt.Errorf("ordinary init rollback exact object remains at canonical name: %s: %w", rel, err)
	}
	return rebound, nil
}
