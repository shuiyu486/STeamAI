//go:build windows

package fs

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const anchoredFileRenameInformation = 10

type anchoredFileRenameInformationValue struct {
	replaceIfExists byte
	rootDirectory   windows.Handle
	fileNameLength  uint32
	fileName        [syscall.MAX_PATH]uint16
}

func renameNoReplaceExactNative(request exactRenameRequest) error {
	sourceDirectory, err := request.SourceParent.Open(".")
	if err != nil {
		return err
	}
	defer sourceDirectory.Close()
	sourceHandle, sourceFile, err := openWindowsRenameSource(windows.Handle(sourceDirectory.Fd()), request.SourceName, request.ExpectedTree != nil)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	opened, err := sourceFile.Stat()
	if err != nil || request.ExpectedInfo == nil || !os.SameFile(request.ExpectedInfo, opened) || opened.IsDir() != (request.ExpectedTree != nil) {
		return fmt.Errorf("exact rename source identity changed: %w", err)
	}
	var treeHandles []*os.File
	if request.ExpectedTree == nil {
		data, after, err := readWindowsHandleExact(sourceFile, int64(len(request.ExpectedBytes)))
		if err != nil || !bytes.Equal(data, request.ExpectedBytes) || !anchoredModeMatches(request.ExpectedMode, after.Mode()) {
			return fmt.Errorf("exact rename source file changed: %w", err)
		}
	} else {
		treeHandles, err = validateWindowsExpectedTree(sourceHandle, opened, *request.ExpectedTree)
		if err != nil {
			return err
		}
		defer func() {
			if treeHandles != nil {
				_ = closeWindowsTreeHandles(treeHandles)
			}
		}()
		if anchoredRootTreeAfterValidationHook != nil {
			if err := anchoredRootTreeAfterValidationHook(); err != nil {
				return errors.Join(err, closeWindowsTreeHandles(treeHandles))
			}
		}
		if err := closeWindowsTreeHandles(treeHandles); err != nil {
			return err
		}
		treeHandles = nil
		treeHandles, err = validateWindowsExpectedTree(sourceHandle, opened, *request.ExpectedTree)
		if err != nil {
			return err
		}
		opened, err = sourceFile.Stat()
		if err != nil || !opened.IsDir() || !os.SameFile(request.ExpectedInfo, opened) {
			return fmt.Errorf("exact rename tree root changed after validation: %w", err)
		}
	}
	if _, err := request.TargetParent.Lstat(request.TargetName); err == nil {
		return os.ErrExist
	} else if !os.IsNotExist(err) {
		return err
	}
	targetDirectory, err := request.TargetParent.Open(".")
	if err != nil {
		return err
	}
	defer targetDirectory.Close()
	name, err := syscall.UTF16FromString(request.TargetName)
	if err != nil {
		return err
	}
	if len(name) > syscall.MAX_PATH {
		return syscall.ENAMETOOLONG
	}
	if err := closeWindowsTreeHandles(treeHandles); err != nil {
		return err
	}
	treeHandles = nil
	if request.ExpectedTree != nil {
		treeHandles, err = validateWindowsExpectedTree(sourceHandle, opened, *request.ExpectedTree)
		if err != nil {
			return err
		}
		if err := closeWindowsTreeHandles(treeHandles); err != nil {
			return err
		}
		treeHandles = nil
	}
	value := anchoredFileRenameInformationValue{
		rootDirectory:  windows.Handle(targetDirectory.Fd()),
		fileNameLength: uint32((len(name) - 1) * 2),
	}
	copy(value.fileName[:], name)
	err = windows.NtSetInformationFile(
		sourceHandle,
		&windows.IO_STATUS_BLOCK{},
		(*byte)(unsafe.Pointer(&value)),
		uint32(unsafe.Sizeof(value)),
		anchoredFileRenameInformation,
	)
	if status, ok := err.(windows.NTStatus); ok {
		return status.Errno()
	}
	return err
}

func openWindowsRenameSource(parent windows.Handle, name string, directory bool) (windows.Handle, *os.File, error) {
	objectName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return 0, nil, err
	}
	attributes := windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: parent,
		ObjectName:    objectName,
		Attributes:    windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE,
	}
	options := uint32(windows.FILE_OPEN_REPARSE_POINT | windows.FILE_OPEN_FOR_BACKUP_INTENT | windows.FILE_SYNCHRONOUS_IO_NONALERT)
	if directory {
		options |= windows.FILE_DIRECTORY_FILE
	} else {
		options |= windows.FILE_NON_DIRECTORY_FILE
	}
	var handle windows.Handle
	access := uint32(windows.DELETE | windows.FILE_READ_ATTRIBUTES | windows.SYNCHRONIZE)
	if directory {
		access |= windows.FILE_LIST_DIRECTORY
	} else {
		access |= windows.FILE_READ_DATA
	}
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
		return 0, nil, err
	}
	file := os.NewFile(uintptr(handle), name)
	if file == nil {
		_ = windows.CloseHandle(handle)
		return 0, nil, os.ErrInvalid
	}
	return handle, file, nil
}

func closeWindowsTreeHandles(handles []*os.File) error {
	var errs []error
	for index := len(handles) - 1; index >= 0; index-- {
		errs = append(errs, handles[index].Close())
	}
	return errors.Join(errs...)
}

func validateWindowsExpectedTree(sourceHandle windows.Handle, rootInfo os.FileInfo, expected ExpectedTree) ([]*os.File, error) {
	if !rootInfo.IsDir() {
		return nil, fmt.Errorf("expected tree root changed")
	}
	expectedDirectories := map[string]ExpectedDirectory{}
	for _, directory := range expected.Directories {
		expectedDirectories[directory.Path] = directory
	}
	rootDirectory, ok := expectedDirectories["."]
	if !ok || !anchoredModeMatches(rootDirectory.Mode, rootInfo.Mode()) {
		return nil, fmt.Errorf("expected tree root mode changed")
	}
	expectedFiles := map[string]ExpectedFile{}
	for _, file := range expected.Files {
		expectedFiles[file.Path] = file
	}
	seenDirectories := map[string]bool{".": true}
	seenFiles := map[string]bool{}
	var openedFiles []*os.File
	valid := false
	defer func() {
		if !valid {
			_ = closeWindowsTreeHandles(openedFiles)
		}
	}()
	var walk func(windows.Handle, string) error
	walk = func(directoryHandle windows.Handle, relative string) error {
		entries, err := listWindowsDirectoryHandle(directoryHandle)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			childRel := entry.name
			if relative != "." {
				childRel = filepath.ToSlash(filepath.Join(relative, entry.name))
			}
			if entry.directory {
				expectedDirectory, ok := expectedDirectories[childRel]
				if !ok {
					return fmt.Errorf("expected tree has an extra directory: %s", childRel)
				}
				childHandle, childFile, err := openWindowsRenameSource(directoryHandle, entry.name, true)
				if err != nil {
					return err
				}
				openedFiles = append(openedFiles, childFile)
				childInfo, err := childFile.Stat()
				if err != nil || !childInfo.IsDir() || !anchoredModeMatches(expectedDirectory.Mode, childInfo.Mode()) {
					return fmt.Errorf("expected tree directory changed: %s: %w", childRel, err)
				}
				seenDirectories[childRel] = true
				if err := walk(childHandle, childRel); err != nil {
					return err
				}
				continue
			}
			expectedFile, ok := expectedFiles[childRel]
			if !ok {
				return fmt.Errorf("expected tree has an extra file: %s", childRel)
			}
			file, err := openWindowsTreeFileAt(directoryHandle, entry.name)
			if err != nil {
				return err
			}
			openedFiles = append(openedFiles, file)
			info, err := file.Stat()
			if err != nil || !info.Mode().IsRegular() || info.Size() != expectedFile.Size || !anchoredModeMatches(expectedFile.Mode, info.Mode()) {
				return fmt.Errorf("expected tree file metadata changed: %s: %w", childRel, err)
			}
			data, after, err := readWindowsHandleExact(file, expectedFile.Size)
			if err != nil || after.Size() != expectedFile.Size || sha256.Sum256(data) != expectedFile.SHA256 || !bytes.Equal(data, expectedFile.Data) {
				return fmt.Errorf("expected tree file content changed: %s: %w", childRel, err)
			}
			seenFiles[childRel] = true
		}
		return nil
	}
	if err := walk(sourceHandle, "."); err != nil {
		return nil, err
	}
	if len(seenDirectories) != len(expectedDirectories) || len(seenFiles) != len(expectedFiles) {
		return nil, fmt.Errorf("expected tree has missing relative names")
	}
	valid = true
	return openedFiles, nil
}

type windowsDirectoryEntry struct {
	name      string
	directory bool
}

type windowsFileIDBothDirectoryInfo struct {
	NextEntryOffset uint32
	FileIndex       uint32
	CreationTime    int64
	LastAccessTime  int64
	LastWriteTime   int64
	ChangeTime      int64
	EndOfFile       int64
	AllocationSize  int64
	FileAttributes  uint32
	FileNameLength  uint32
	EaSize          uint32
	ShortNameLength byte
	Reserved        byte
	ShortName       [12]uint16
	FileID          int64
	FileName        [1]uint16
}

func listWindowsDirectoryHandle(handle windows.Handle) ([]windowsDirectoryEntry, error) {
	buffer := make([]byte, 64*1024)
	var entries []windowsDirectoryEntry
	class := uint32(windows.FileIdBothDirectoryRestartInfo)
	for {
		err := windows.GetFileInformationByHandleEx(handle, class, &buffer[0], uint32(len(buffer)))
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			return entries, nil
		}
		if err != nil {
			return nil, err
		}
		offset := uint32(0)
		for {
			entry := (*windowsFileIDBothDirectoryInfo)(unsafe.Pointer(&buffer[offset]))
			nameStart := unsafe.Add(unsafe.Pointer(entry), unsafe.Offsetof(entry.FileName))
			nameUnits := unsafe.Slice((*uint16)(nameStart), entry.FileNameLength/2)
			name := syscall.UTF16ToString(nameUnits)
			if name != "." && name != ".." {
				entries = append(entries, windowsDirectoryEntry{name: name, directory: entry.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0})
			}
			if entry.NextEntryOffset == 0 {
				break
			}
			offset += entry.NextEntryOffset
			if offset >= uint32(len(buffer)) {
				return nil, fmt.Errorf("malformed exact tree directory enumeration")
			}
		}
		class = uint32(windows.FileIdBothDirectoryInfo)
	}
}

func openWindowsTreeFileAt(parent windows.Handle, name string) (*os.File, error) {
	objectName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return nil, err
	}
	attributes := windows.OBJECT_ATTRIBUTES{Length: uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})), RootDirectory: parent, ObjectName: objectName, Attributes: windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE}
	var handle windows.Handle
	if err := windows.NtCreateFile(&handle, windows.FILE_READ_DATA|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE, &attributes, &windows.IO_STATUS_BLOCK{}, nil, windows.FILE_ATTRIBUTE_NORMAL, windows.FILE_SHARE_READ, windows.FILE_OPEN, windows.FILE_OPEN_REPARSE_POINT|windows.FILE_OPEN_FOR_BACKUP_INTENT|windows.FILE_SYNCHRONOUS_IO_NONALERT|windows.FILE_NON_DIRECTORY_FILE, 0, 0); err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(handle), name)
	if file == nil {
		_ = windows.CloseHandle(handle)
		return nil, os.ErrInvalid
	}
	return file, nil
}

func readWindowsHandleExact(file *os.File, limit int64) ([]byte, os.FileInfo, error) {
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Size() != limit {
		return nil, nil, fmt.Errorf("exact file metadata changed: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	after, statErr := file.Stat()
	if readErr != nil || statErr != nil || int64(len(data)) != limit || before.Size() != after.Size() || before.Mode() != after.Mode() || !os.SameFile(before, after) {
		return nil, nil, errors.Join(readErr, statErr, fmt.Errorf("exact file changed while reading"))
	}
	return data, after, nil
}
