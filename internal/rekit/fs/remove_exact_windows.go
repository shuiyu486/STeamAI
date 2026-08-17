//go:build windows

package fs

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type anchoredFileDispositionInformation struct {
	deleteFile byte
}

var (
	anchoredKernel32                   = syscall.NewLazyDLL("kernel32.dll")
	anchoredSetFileInformationByHandle = anchoredKernel32.NewProc("SetFileInformationByHandle")
)

func removeExactObject(parent *os.Root, name string, expectedInfo os.FileInfo, expected []byte, mode fs.FileMode, directory bool) error {
	parentDirectory, err := parent.Open(".")
	if err != nil {
		return err
	}
	defer parentDirectory.Close()
	objectName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return err
	}
	attributes := windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: windows.Handle(parentDirectory.Fd()),
		ObjectName:    objectName,
		Attributes:    windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE,
	}
	options := uint32(windows.FILE_OPEN_REPARSE_POINT | windows.FILE_OPEN_FOR_BACKUP_INTENT | windows.FILE_SYNCHRONOUS_IO_NONALERT)
	access := uint32(windows.DELETE | windows.FILE_READ_ATTRIBUTES | windows.SYNCHRONIZE)
	if directory {
		options |= windows.FILE_DIRECTORY_FILE
		access |= windows.FILE_LIST_DIRECTORY
	} else {
		options |= windows.FILE_NON_DIRECTORY_FILE
		access |= windows.FILE_READ_DATA
	}
	var handle windows.Handle
	if err := windows.NtCreateFile(
		&handle,
		access,
		&attributes,
		&windows.IO_STATUS_BLOCK{},
		nil,
		windows.FILE_ATTRIBUTE_NORMAL,
		windows.FILE_SHARE_READ,
		windows.FILE_OPEN,
		options,
		0,
		0,
	); err != nil {
		return err
	}
	file := os.NewFile(uintptr(handle), name)
	if file == nil {
		_ = windows.CloseHandle(handle)
		return fmt.Errorf("anchored wrap exact object handle: %s", name)
	}
	defer file.Close()
	current, err := file.Stat()
	if err != nil || expectedInfo == nil || current.IsDir() != directory || !os.SameFile(expectedInfo, current) {
		return fmt.Errorf("anchored remove exact object identity changed: %s", name)
	}
	if !directory {
		data, after, readErr := readWindowsHandleExact(file, int64(len(expected)))
		if readErr != nil || !bytes.Equal(data, expected) || !anchoredModeMatches(mode, after.Mode()) {
			return fmt.Errorf("anchored remove exact object bytes or mode changed: %s: %w", name, readErr)
		}
	} else {
		entries, err := listWindowsDirectoryHandle(handle)
		if err != nil {
			return err
		}
		if len(entries) != 0 {
			return fmt.Errorf("anchored remove exact directory is not empty: %s", name)
		}
	}
	const fileDispositionInfo = 4
	disposition := anchoredFileDispositionInformation{deleteFile: 1}
	removed, _, callErr := anchoredSetFileInformationByHandle.Call(
		uintptr(handle),
		fileDispositionInfo,
		uintptr(unsafe.Pointer(&disposition)),
		unsafe.Sizeof(disposition),
	)
	if removed == 0 {
		return fmt.Errorf("anchored remove exact object %s: %v", name, callErr)
	}
	return nil
}
