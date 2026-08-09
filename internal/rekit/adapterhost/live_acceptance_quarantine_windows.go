//go:build windows

package adapterhost

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const liveAcceptanceFileRenameInfoClass = 10

var (
	liveAcceptanceNTDLL                = syscall.NewLazyDLL("ntdll.dll")
	liveAcceptanceNtSetInformationFile = liveAcceptanceNTDLL.NewProc("NtSetInformationFile")
)

type liveAcceptanceIOStatusBlock struct {
	Status      uintptr
	Information uintptr
}

type liveAcceptanceFileRenameInfo struct {
	ReplaceIfExists bool
	RootDirectory   syscall.Handle
	FileNameLength  uint32
	FileName        [syscall.MAX_PATH]uint16
}

func quarantineLiveAcceptanceCase(identity *liveAcceptanceCaseIdentity, quarantine string) error {
	sourcePath := filepath.Join(identity.parentPath, identity.name)
	sourcePointer, err := syscall.UTF16PtrFromString(sourcePath)
	if err != nil {
		return err
	}
	const (
		deleteAccess   = 0x00010000
		readAttributes = 0x00000080
	)
	sourceHandle, err := syscall.CreateFile(
		sourcePointer,
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
	source := os.NewFile(uintptr(sourceHandle), sourcePath)
	defer source.Close()
	sourceInfo, err := source.Stat()
	if err != nil || !sourceInfo.IsDir() || !os.SameFile(identity.caseInfo, sourceInfo) {
		return fmt.Errorf("adapter live case identity changed before exact quarantine: %s", sourcePath)
	}
	var sourceNative syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(sourceHandle, &sourceNative); err != nil {
		return err
	}
	if sourceNative.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("adapter live case root became a reparse point before quarantine: %s", sourcePath)
	}

	parentPointer, err := syscall.UTF16PtrFromString(identity.parentPath)
	if err != nil {
		return err
	}
	parentHandle, err := syscall.CreateFile(
		parentPointer,
		readAttributes,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_FLAG_OPEN_REPARSE_POINT|syscall.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return err
	}
	parent := os.NewFile(uintptr(parentHandle), identity.parentPath)
	defer parent.Close()
	parentInfo, err := parent.Stat()
	if err != nil || !parentInfo.IsDir() || !os.SameFile(identity.parentInfo, parentInfo) {
		return fmt.Errorf("adapter live case parent identity changed before exact quarantine: %s", identity.parentPath)
	}
	var parentNative syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(parentHandle, &parentNative); err != nil {
		return err
	}
	if parentNative.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("adapter live case parent became a reparse point before quarantine: %s", identity.parentPath)
	}

	name, err := syscall.UTF16FromString(quarantine)
	if err != nil {
		return err
	}
	if len(name) > syscall.MAX_PATH {
		return syscall.ENAMETOOLONG
	}
	info := liveAcceptanceFileRenameInfo{
		ReplaceIfExists: false,
		RootDirectory:   parentHandle,
		FileNameLength:  uint32((len(name) - 1) * 2),
	}
	copy(info.FileName[:], name)
	var ioStatus liveAcceptanceIOStatusBlock
	status, _, _ := liveAcceptanceNtSetInformationFile.Call(
		uintptr(sourceHandle),
		uintptr(unsafe.Pointer(&ioStatus)),
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
		liveAcceptanceFileRenameInfoClass,
	)
	if status != 0 {
		return fmt.Errorf("quarantine exact adapter live case without replacement: NTSTATUS 0x%08x", uint32(status))
	}
	return nil
}
