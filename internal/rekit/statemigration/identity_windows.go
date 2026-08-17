//go:build windows

package statemigration

import (
	"fmt"
	"os"
	"syscall"
)

func identityForRoot(root *os.Root) (Identity, error) {
	file, err := root.Open(".")
	if err != nil {
		return Identity{}, err
	}
	defer file.Close()
	return identityForFile(file)
}

func identityForFile(file *os.File) (Identity, error) {
	raw, err := file.SyscallConn()
	if err != nil {
		return Identity{}, err
	}
	var info syscall.ByHandleFileInformation
	var identityErr error
	if err := raw.Control(func(handle uintptr) {
		identityErr = syscall.GetFileInformationByHandle(syscall.Handle(handle), &info)
	}); err != nil {
		return Identity{}, err
	}
	if identityErr != nil {
		return Identity{}, identityErr
	}
	index := uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow)
	if info.VolumeSerialNumber == 0 || index == 0 {
		return Identity{}, fmt.Errorf("Windows filesystem identity is incomplete")
	}
	return Identity{Scheme: "windows-volume-file-index-v1", VolumeSerial: info.VolumeSerialNumber, FileIndex: index}, nil
}
