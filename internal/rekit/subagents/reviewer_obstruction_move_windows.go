//go:build windows

package subagents

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

const (
	reviewerObstructionDeleteAccess     = 0x00010000
	reviewerObstructionReadAttributes   = 0x00000080
	reviewerObstructionShareRead        = 0x00000001
	reviewerObstructionShareWrite       = 0x00000002
	reviewerObstructionShareDelete      = 0x00000004
	reviewerObstructionOpenExisting     = 3
	reviewerObstructionOpenReparsePoint = 0x00200000
	reviewerObstructionBackupSemantics  = 0x02000000
	reviewerObstructionFileRenameInfo   = 10
)

var (
	reviewerObstructionNTDLL                    = syscall.NewLazyDLL("ntdll.dll")
	reviewerObstructionKernel32                 = syscall.NewLazyDLL("kernel32.dll")
	reviewerObstructionNtSetInformationFile     = reviewerObstructionNTDLL.NewProc("NtSetInformationFile")
	reviewerObstructionGetFinalPathNameByHandle = reviewerObstructionKernel32.NewProc("GetFinalPathNameByHandleW")
)

type reviewerObstructionIOStatusBlock struct {
	Status      uintptr
	Information uintptr
}

func reviewerResultObstructionMoveSupported() bool {
	return true
}

type reviewerObstructionFileRenameInformation struct {
	ReplaceIfExists bool
	RootDirectory   syscall.Handle
	FileNameLength  uint32
	FileName        [syscall.MAX_PATH]uint16
}

func reviewerObstructionHandleMatchesPath(handle syscall.Handle, expectedPath string) error {
	buffer := make([]uint16, 32768)
	length, _, callErr := reviewerObstructionGetFinalPathNameByHandle.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
		0,
	)
	if length == 0 {
		return callErr
	}
	if length >= uintptr(len(buffer)) {
		return syscall.ENAMETOOLONG
	}
	actual := syscall.UTF16ToString(buffer[:length])
	actual = strings.TrimPrefix(actual, `\\?\`)
	expected, err := filepath.Abs(expectedPath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Clean(actual), filepath.Clean(expected)) {
		return fmt.Errorf("reviewer result recovery namespace guard moved outside its canonical path")
	}
	return nil
}

func moveReviewerResultObstructionExact(resultPath, quarantinePath, namespaceGuardPath string, expected reviewerResultObstructionSnapshot, validate func() error) error {
	resultPath16, err := syscall.UTF16PtrFromString(resultPath)
	if err != nil {
		return err
	}
	handle, err := syscall.CreateFile(
		resultPath16,
		reviewerObstructionDeleteAccess|reviewerObstructionReadAttributes,
		reviewerObstructionShareRead,
		nil,
		reviewerObstructionOpenExisting,
		reviewerObstructionOpenReparsePoint|reviewerObstructionBackupSemantics,
		0,
	)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(handle)
	namespaceGuardPath16, err := syscall.UTF16PtrFromString(namespaceGuardPath)
	if err != nil {
		return err
	}
	namespaceGuard, err := syscall.CreateFile(
		namespaceGuardPath16,
		reviewerObstructionReadAttributes,
		reviewerObstructionShareRead|reviewerObstructionShareWrite,
		nil,
		reviewerObstructionOpenExisting,
		reviewerObstructionOpenReparsePoint,
		0,
	)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(namespaceGuard)
	var guardInfo syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(namespaceGuard, &guardInfo); err != nil {
		return err
	}
	if guardInfo.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 ||
		guardInfo.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("reviewer result recovery namespace guard is not a regular file")
	}
	if err := reviewerObstructionHandleMatchesPath(namespaceGuard, namespaceGuardPath); err != nil {
		return err
	}
	quarantineRoot16, err := syscall.UTF16PtrFromString(filepath.Dir(quarantinePath))
	if err != nil {
		return err
	}
	quarantineRoot, err := syscall.CreateFile(
		quarantineRoot16,
		reviewerObstructionReadAttributes,
		reviewerObstructionShareRead|reviewerObstructionShareWrite|reviewerObstructionShareDelete,
		nil,
		reviewerObstructionOpenExisting,
		reviewerObstructionOpenReparsePoint|reviewerObstructionBackupSemantics,
		0,
	)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(quarantineRoot)
	if err := reviewerObstructionHandleMatchesPath(quarantineRoot, filepath.Dir(quarantinePath)); err != nil {
		return err
	}
	if err := reviewerObstructionHandleMatchesPath(handle, resultPath); err != nil {
		return err
	}
	var sourceInfo syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &sourceInfo); err != nil {
		return err
	}
	switch expected.Kind {
	case "empty-file":
		if sourceInfo.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
			sourceInfo.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 ||
			sourceInfo.FileSizeHigh != 0 || sourceInfo.FileSizeLow != 0 {
			return fmt.Errorf("reviewer result obstruction source handle is not an empty regular file")
		}
	case "symlink":
		if sourceInfo.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT == 0 ||
			sourceInfo.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 {
			return fmt.Errorf("reviewer result obstruction source handle is not a file symlink")
		}
	default:
		return fmt.Errorf("exact %s reviewer result obstruction move is unavailable", expected.Kind)
	}
	if err := validate(); err != nil {
		return err
	}

	name, err := syscall.UTF16FromString(filepath.Base(quarantinePath))
	if err != nil {
		return err
	}
	if len(name) > syscall.MAX_PATH {
		return syscall.ENAMETOOLONG
	}
	info := reviewerObstructionFileRenameInformation{
		ReplaceIfExists: false,
		RootDirectory:   quarantineRoot,
		FileNameLength:  uint32((len(name) - 1) * 2),
	}
	copy(info.FileName[:], name)

	var ioStatus reviewerObstructionIOStatusBlock
	status, _, _ := reviewerObstructionNtSetInformationFile.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&ioStatus)),
		uintptr(unsafe.Pointer(&info)),
		uintptr(unsafe.Sizeof(info)),
		reviewerObstructionFileRenameInfo,
	)
	if status != 0 {
		return fmt.Errorf("rename exact reviewer result obstruction: NTSTATUS 0x%08x", uint32(status))
	}
	return nil
}
