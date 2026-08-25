package lanemutation

import (
	"fmt"
	"os"
)

// InheritedLaneLease proves that a private child received the exact external
// lane-lock handle from a parent that still owns the mutation boundary.
type InheritedLaneLease struct {
	file           *os.File
	path           string
	validateNative func(uintptr) error
}

func (lease *InheritedLaneLease) Validate() error {
	if lease == nil || lease.file == nil || lease.path == "" || lease.validateNative == nil {
		return fmt.Errorf("inherited lane mutation lease proof is missing")
	}
	pathInfo, err := os.Lstat(lease.path)
	if err != nil || pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() {
		return fmt.Errorf("inherited lane mutation lease path is no longer a regular file: %s", lease.path)
	}
	openedInfo, err := lease.file.Stat()
	if err != nil {
		return fmt.Errorf("stat inherited lane mutation lease handle: %w", err)
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(pathInfo, openedInfo) {
		return fmt.Errorf("inherited lane mutation lease handle changed: %s", lease.path)
	}
	return lease.validateNative(lease.file.Fd())
}

func (lease *InheritedLaneLease) Close() error {
	if lease == nil || lease.file == nil {
		return nil
	}
	file := lease.file
	lease.file = nil
	return file.Close()
}
