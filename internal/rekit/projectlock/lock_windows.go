//go:build windows

package projectlock

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	kernel32Lock     = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = kernel32Lock.NewProc("LockFileEx")
	procUnlockFileEx = kernel32Lock.NewProc("UnlockFileEx")
)

const (
	lockfileExclusiveLock = 0x00000002
	lockfileOffsetLow     = 0xffffffff
	lockfileOffsetHigh    = 0x3fffffff
	localAppDataFolderID  = 28
)

func stableWorkstreamLockRoot() (string, error) {
	var path [syscall.MAX_PATH + 1]uint16
	r1, _, callErr := syscall.NewLazyDLL("shell32.dll").NewProc("SHGetFolderPathW").Call(0, localAppDataFolderID, 0, 0, uintptr(unsafe.Pointer(&path[0])))
	root := syscall.UTF16ToString(path[:])
	if r1 != 0 || root == "" {
		return "", fmt.Errorf("resolve stable workstream lock root: SHGetFolderPathW result=%d: %w", r1, callErr)
	}
	return filepath.Join(root, "rekit", "workstream-locks"), nil
}

func lockFile(fd uintptr, exclusive bool) error {
	var flags uintptr
	if exclusive {
		flags = lockfileExclusiveLock
	}
	overlapped := syscall.Overlapped{Offset: lockfileOffsetLow, OffsetHigh: lockfileOffsetHigh}
	r1, _, callErr := procLockFileEx.Call(fd, flags, 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
	if r1 == 0 {
		return fmt.Errorf("LockFileEx: %w", callErr)
	}
	return nil
}

func unlockFile(fd uintptr) error {
	overlapped := syscall.Overlapped{Offset: lockfileOffsetLow, OffsetHigh: lockfileOffsetHigh}
	r1, _, callErr := procUnlockFileEx.Call(fd, 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
	if r1 == 0 {
		return fmt.Errorf("UnlockFileEx: %w", callErr)
	}
	return nil
}
