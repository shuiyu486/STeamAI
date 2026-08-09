//go:build windows

package adapterhost

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

type ownedOutput struct {
	volumeSerial  uint32
	fileIndexHigh uint32
	fileIndexLow  uint32
}

type fileDispositionInformation struct {
	DeleteFile byte
}

var (
	ownedOutputKernel32        = syscall.NewLazyDLL("kernel32.dll")
	reOpenFile                 = ownedOutputKernel32.NewProc("ReOpenFile")
	setFileInformationByHandle = ownedOutputKernel32.NewProc("SetFileInformationByHandle")
)

func captureOwnedOutput(file *os.File) (*ownedOutput, error) {
	if file == nil {
		return nil, fmt.Errorf("owned output file is missing")
	}
	identity, attributes, err := ownedOutputIdentity(syscall.Handle(file.Fd()))
	if err != nil {
		return nil, err
	}
	if attributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 || attributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return nil, fmt.Errorf("owned output must be a regular non-reparse file")
	}
	return &identity, nil
}

func removeOwnedOutput(root *os.Root, rel string, owned *ownedOutput, afterIdentity func(string) error) error {
	if root == nil || owned == nil {
		return nil
	}
	opened, err := root.Open(filepath.FromSlash(rel))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer opened.Close()
	const (
		deleteAccess   = 0x00010000
		readAttributes = 0x00000080
	)
	handleValue, _, callErr := reOpenFile.Call(
		opened.Fd(),
		deleteAccess|readAttributes,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		syscall.FILE_FLAG_OPEN_REPARSE_POINT,
	)
	if handleValue == 0 || syscall.Handle(handleValue) == syscall.InvalidHandle {
		return fmt.Errorf("reopen exact owned output %s for deletion: %v", rel, callErr)
	}
	handle := syscall.Handle(handleValue)
	defer syscall.CloseHandle(handle)
	identity, attributes, err := ownedOutputIdentity(handle)
	if err != nil {
		return err
	}
	if identity != *owned || attributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 || attributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return fmt.Errorf("refuse cleanup because owned output identity changed: %s", rel)
	}
	if afterIdentity != nil {
		if err := afterIdentity(rel); err != nil {
			return err
		}
	}
	const fileDispositionInfo = 4
	disposition := fileDispositionInformation{DeleteFile: 1}
	result, _, callErr := setFileInformationByHandle.Call(
		uintptr(handle),
		fileDispositionInfo,
		uintptr(unsafe.Pointer(&disposition)),
		unsafe.Sizeof(disposition),
	)
	if result == 0 {
		return fmt.Errorf("delete exact owned output %s: %v", rel, callErr)
	}
	return nil
}

func ownedOutputIdentity(handle syscall.Handle) (ownedOutput, uint32, error) {
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &info); err != nil {
		return ownedOutput{}, 0, err
	}
	return ownedOutput{
		volumeSerial:  info.VolumeSerialNumber,
		fileIndexHigh: info.FileIndexHigh,
		fileIndexLow:  info.FileIndexLow,
	}, info.FileAttributes, nil
}
