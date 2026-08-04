//go:build windows

package packmemoryconsumption

import (
	"fmt"
	"syscall"
)

func caseRootIdentity(root rootIdentitySource) (CaseRootIdentity, error) {
	file, err := root.Open(".")
	if err != nil {
		return CaseRootIdentity{}, err
	}
	defer file.Close()

	raw, err := file.SyscallConn()
	if err != nil {
		return CaseRootIdentity{}, err
	}
	var info syscall.ByHandleFileInformation
	var identityErr error
	if err := raw.Control(func(handle uintptr) {
		identityErr = syscall.GetFileInformationByHandle(syscall.Handle(handle), &info)
	}); err != nil {
		return CaseRootIdentity{}, err
	}
	if identityErr != nil {
		return CaseRootIdentity{}, identityErr
	}
	fileIndex := uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow)
	if info.VolumeSerialNumber == 0 || fileIndex == 0 {
		return CaseRootIdentity{}, fmt.Errorf("Windows case-root identity is incomplete")
	}
	return CaseRootIdentity{
		Scheme:       "windows-volume-file-index-v1",
		VolumeSerial: info.VolumeSerialNumber,
		FileIndex:    fileIndex,
	}, nil
}
