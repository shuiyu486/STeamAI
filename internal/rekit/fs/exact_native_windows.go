//go:build windows

package fs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	exactFSCTLRequestOplock       = 0x00090240
	exactOplockLevelCacheRead     = 0x00000001
	exactOplockLevelCacheWrite    = 0x00000004
	exactRequestOplockFlagRequest = 0x00000001
	exactOpenExisting             = 3
	exactMutationGuardName        = ".steamai-exact-mutation-guard"
)

type exactRequestOplockInput struct {
	StructureVersion     uint16
	StructureLength      uint16
	RequestedOplockLevel uint32
	Flags                uint32
}

type exactRequestOplockOutput struct {
	StructureVersion    uint16
	StructureLength     uint16
	OriginalOplockLevel uint32
	NewOplockLevel      uint32
	Flags               uint32
	AccessMode          uint32
	ShareMode           uint16
}

type windowsExactFile struct {
	file       *os.File
	handle     windows.Handle
	event      windows.Handle
	overlapped windows.Overlapped
	output     exactRequestOplockOutput
	pending    bool
}

func (file *windowsExactFile) Stat() (os.FileInfo, error) { return file.file.Stat() }
func (file *windowsExactFile) Read(p []byte) (int, error) { return file.file.Read(p) }
func (file *windowsExactFile) Seek(offset int64, whence int) (int64, error) {
	return file.file.Seek(offset, whence)
}
func (file *windowsExactFile) Sync() error { return file.file.Sync() }

func (file *windowsExactFile) Close() error {
	if file == nil || file.file == nil {
		return nil
	}
	var cancelErr error
	if file.pending {
		cancelErr = windows.CancelIoEx(file.handle, &file.overlapped)
		if errors.Is(cancelErr, windows.ERROR_NOT_FOUND) {
			cancelErr = nil
		}
		file.pending = false
	}
	closeErr := file.file.Close()
	file.file = nil
	eventErr := windows.CloseHandle(file.event)
	file.event = 0
	return errors.Join(cancelErr, closeErr, eventErr)
}

func (file *windowsExactFile) unbroken() error {
	if file == nil || file.file == nil || !file.pending {
		return fmt.Errorf("exact file oplock is not active")
	}
	result, err := windows.WaitForSingleObject(file.event, 0)
	if err != nil {
		return err
	}
	if result != uint32(windows.WAIT_TIMEOUT) {
		return fmt.Errorf("exact file oplock broke during validation")
	}
	return nil
}

type windowsExactCreatedFile struct {
	file   *os.File
	handle windows.Handle
	closed bool
}

func (file *windowsExactCreatedFile) Stat() (os.FileInfo, error) {
	if file == nil || file.closed || file.file == nil {
		return nil, os.ErrClosed
	}
	return file.file.Stat()
}

func (file *windowsExactCreatedFile) Read(data []byte) (int, error) {
	if file == nil || file.closed || file.file == nil {
		return 0, os.ErrClosed
	}
	return file.file.Read(data)
}

func (file *windowsExactCreatedFile) Seek(offset int64, whence int) (int64, error) {
	if file == nil || file.closed || file.file == nil {
		return 0, os.ErrClosed
	}
	return file.file.Seek(offset, whence)
}

func (file *windowsExactCreatedFile) Write(data []byte) (int, error) {
	if file == nil || file.closed || file.file == nil {
		return 0, os.ErrClosed
	}
	return file.file.Write(data)
}

func (file *windowsExactCreatedFile) Sync() error {
	if file == nil || file.closed || file.file == nil {
		return os.ErrClosed
	}
	return file.file.Sync()
}

func (file *windowsExactCreatedFile) Commit() error {
	if file == nil || file.closed {
		return nil
	}
	file.closed = true
	err := file.file.Close()
	file.file = nil
	file.handle = 0
	return err
}

func (file *windowsExactCreatedFile) Abort() error {
	if file == nil || file.closed {
		return nil
	}
	file.closed = true
	disposition := uint32(windows.FILE_DISPOSITION_DELETE | windows.FILE_DISPOSITION_POSIX_SEMANTICS | windows.FILE_DISPOSITION_IGNORE_READONLY_ATTRIBUTE)
	deleteErr := windows.SetFileInformationByHandle(
		file.handle,
		windows.FileDispositionInfoEx,
		(*byte)(unsafe.Pointer(&disposition)),
		uint32(unsafe.Sizeof(disposition)),
	)
	closeErr := file.file.Close()
	file.file = nil
	file.handle = 0
	return errors.Join(deleteErr, closeErr)
}

type windowsMutationGuard struct {
	handles []windows.Handle
}

func (guard *windowsMutationGuard) Validate() error { return nil }

func (guard *windowsMutationGuard) Close() error {
	if guard == nil {
		return nil
	}
	var errs []error
	for index := len(guard.handles) - 1; index >= 0; index-- {
		if err := windows.CloseHandle(guard.handles[index]); err != nil {
			errs = append(errs, err)
		}
	}
	guard.handles = nil
	return errors.Join(errs...)
}

func createExactFile(parent *os.Root, name string, mode os.FileMode) (exactCreatedFile, error) {
	parentDirectory, err := parent.Open(".")
	if err != nil {
		return nil, err
	}
	defer parentDirectory.Close()
	objectName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return nil, err
	}
	attributes := windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: windows.Handle(parentDirectory.Fd()),
		ObjectName:    objectName,
		Attributes:    windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE,
	}
	fileAttributes := uint32(windows.FILE_ATTRIBUTE_NORMAL)
	if mode.Perm()&0o200 == 0 {
		fileAttributes = windows.FILE_ATTRIBUTE_READONLY
	}
	var handle windows.Handle
	if err := windows.NtCreateFile(
		&handle,
		windows.DELETE|windows.FILE_READ_DATA|windows.FILE_WRITE_DATA|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		&attributes,
		&windows.IO_STATUS_BLOCK{},
		nil,
		fileAttributes,
		windows.FILE_SHARE_READ,
		windows.FILE_CREATE,
		windows.FILE_OPEN_REPARSE_POINT|windows.FILE_OPEN_FOR_BACKUP_INTENT|windows.FILE_WRITE_THROUGH|windows.FILE_SYNCHRONOUS_IO_NONALERT|windows.FILE_NON_DIRECTORY_FILE,
		0,
		0,
	); err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(handle), name)
	if file == nil {
		_ = windows.CloseHandle(handle)
		return nil, os.ErrInvalid
	}
	return &windowsExactCreatedFile{file: file, handle: handle}, nil
}

func openExactFile(parent *os.Root, name string, writeThrough bool) (exactFileHandle, error) {
	parentDirectory, err := parent.Open(".")
	if err != nil {
		return nil, err
	}
	defer parentDirectory.Close()
	objectName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return nil, err
	}
	attributes := windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: windows.Handle(parentDirectory.Fd()),
		ObjectName:    objectName,
		Attributes:    windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE,
	}
	options := uint32(windows.FILE_OPEN_REPARSE_POINT | windows.FILE_OPEN_FOR_BACKUP_INTENT)
	if writeThrough {
		options |= windows.FILE_WRITE_THROUGH
	}
	var handle windows.Handle
	if err := windows.NtCreateFile(
		&handle,
		windows.DELETE|windows.FILE_READ_DATA|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		&attributes,
		&windows.IO_STATUS_BLOCK{},
		nil,
		windows.FILE_ATTRIBUTE_NORMAL,
		windows.FILE_SHARE_READ,
		windows.FILE_OPEN,
		options|windows.FILE_NON_DIRECTORY_FILE,
		0,
		0,
	); err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(handle), name)
	if file == nil {
		_ = windows.CloseHandle(handle)
		return nil, os.ErrInvalid
	}
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	exact := &windowsExactFile{
		file:       file,
		handle:     handle,
		event:      event,
		overlapped: windows.Overlapped{HEvent: event},
		output: exactRequestOplockOutput{
			StructureVersion: 1,
			StructureLength:  uint16(unsafe.Sizeof(exactRequestOplockOutput{})),
		},
	}
	input := exactRequestOplockInput{
		StructureVersion:     1,
		StructureLength:      uint16(unsafe.Sizeof(exactRequestOplockInput{})),
		RequestedOplockLevel: exactOplockLevelCacheRead | exactOplockLevelCacheWrite,
		Flags:                exactRequestOplockFlagRequest,
	}
	var returned uint32
	err = windows.DeviceIoControl(
		handle,
		exactFSCTLRequestOplock,
		(*byte)(unsafe.Pointer(&input)),
		uint32(unsafe.Sizeof(input)),
		(*byte)(unsafe.Pointer(&exact.output)),
		uint32(unsafe.Sizeof(exact.output)),
		&returned,
		&exact.overlapped,
	)
	if !errors.Is(err, windows.ERROR_IO_PENDING) {
		_ = windows.CloseHandle(event)
		_ = file.Close()
		if err == nil {
			return nil, fmt.Errorf("exact file oplock unexpectedly completed")
		}
		return nil, fmt.Errorf("request exact file oplock: %w", err)
	}
	exact.pending = true
	return exact, nil
}

func exactFileUnbroken(file exactFileHandle) error {
	windowsFile, ok := file.(*windowsExactFile)
	if !ok {
		return errAnchoredExactMutationUnsupported
	}
	return windowsFile.unbroken()
}

func openExactMutationGuard(rootPath, sourceParentPath, targetParentPath string) (exactMutationGuard, error) {
	paths := uniqueWindowsGuardPaths(rootPath, sourceParentPath, targetParentPath)
	guard := &windowsMutationGuard{}
	for _, path := range paths {
		handle, err := openWindowsDirectoryDescendantGuard(path)
		if err != nil {
			_ = guard.Close()
			return nil, fmt.Errorf("open exact mutation guard for %s: %w", path, err)
		}
		guard.handles = append(guard.handles, handle)
	}
	return guard, nil
}

func uniqueWindowsGuardPaths(paths ...string) []string {
	seen := map[string]bool{}
	var unique []string
	for _, path := range paths {
		path = filepath.Clean(path)
		key := filepath.ToSlash(path)
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, path)
	}
	return unique
}

func openWindowsDirectoryDescendantGuard(path string) (windows.Handle, error) {
	parentUTF16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	parentHandle, err := windows.CreateFile(
		parentUTF16,
		windows.FILE_LIST_DIRECTORY|windows.FILE_WRITE_DATA|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		exactOpenExisting,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(parentHandle)
	objectName, err := windows.NewNTUnicodeString(exactMutationGuardName)
	if err != nil {
		return 0, err
	}
	attributes := windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: parentHandle,
		ObjectName:    objectName,
		Attributes:    windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE,
	}
	var handle windows.Handle
	if err := windows.NtCreateFile(
		&handle,
		windows.DELETE|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		&attributes,
		&windows.IO_STATUS_BLOCK{},
		nil,
		windows.FILE_ATTRIBUTE_HIDDEN|windows.FILE_ATTRIBUTE_SYSTEM,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_CREATE,
		windows.FILE_OPEN_REPARSE_POINT|windows.FILE_OPEN_FOR_BACKUP_INTENT|windows.FILE_DELETE_ON_CLOSE|windows.FILE_SYNCHRONOUS_IO_NONALERT|windows.FILE_DIRECTORY_FILE,
		0,
		0,
	); err != nil {
		return 0, err
	}
	return handle, nil
}

func readExactFileData(file exactFileHandle, limit int64) ([]byte, os.FileInfo, error) {
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Size() < 0 || before.Size() > limit {
		return nil, nil, fmt.Errorf("exact file must be a bounded regular file: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	after, statErr := file.Stat()
	if readErr != nil || statErr != nil || int64(len(data)) > limit || int64(len(data)) != before.Size() || after.Size() != before.Size() ||
		before.Mode() != after.Mode() || !os.SameFile(before, after) {
		return nil, nil, fmt.Errorf("exact file changed while reading: %w", errors.Join(readErr, statErr))
	}
	if err := exactFileUnbroken(file); err != nil {
		return nil, nil, err
	}
	return data, after, nil
}
