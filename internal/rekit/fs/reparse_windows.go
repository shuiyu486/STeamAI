//go:build windows

package fs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func rejectReparsePath(path string) error {
	attributes, err := windowsFileAttributes(path)
	if err != nil {
		return err
	}
	return rejectWindowsReparseAttributes(path, attributes)
}

func validateNoReparsePath(path string) error {
	volume := filepath.VolumeName(path)
	rootPath := volume + string(filepath.Separator)
	rel, err := filepath.Rel(rootPath, path)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	rootDirectory, err := os.Open(rootPath)
	if err != nil {
		return err
	}
	defer rootDirectory.Close()
	objectName, err := windows.NewNTUnicodeString(rel)
	if err != nil {
		return err
	}
	attributes := windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: windows.Handle(rootDirectory.Fd()),
		ObjectName:    objectName,
		Attributes:    windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE,
	}
	var handle windows.Handle
	err = windows.NtCreateFile(
		&handle,
		windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		&attributes,
		&windows.IO_STATUS_BLOCK{},
		nil,
		windows.FILE_ATTRIBUTE_NORMAL,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_OPEN,
		windows.FILE_OPEN_FOR_BACKUP_INTENT|windows.FILE_SYNCHRONOUS_IO_NONALERT,
		0,
		0,
	)
	if err == nil {
		return windows.CloseHandle(handle)
	}
	if status, ok := err.(windows.NTStatus); ok {
		if status == windows.STATUS_REPARSE_POINT_ENCOUNTERED {
			return fmt.Errorf("path must not traverse symlink or reparse point: %s", path)
		}
		err = status.Errno()
	}
	if errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) || errors.Is(err, syscall.ERROR_PATH_NOT_FOUND) {
		return nil
	}
	return err
}

func windowsFileAttributes(path string) (uint32, error) {
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	return syscall.GetFileAttributes(pointer)
}

func rejectWindowsReparseAttributes(path string, attributes uint32) error {
	if attributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("anchored path must not be a reparse point: %s", path)
	}
	return nil
}

func rejectReparseAncestors(path string) error {
	volume := filepath.VolumeName(path)
	current := volume + string(filepath.Separator)
	rel, err := filepath.Rel(current, path)
	if err != nil {
		return err
	}
	for _, component := range splitPathComponents(rel) {
		current = filepath.Join(current, component)
		if err := rejectReparsePath(current); err != nil {
			return err
		}
	}
	return nil
}
