//go:build windows

package lanemutation

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectlock"
)

var (
	inheritedLaneKernel32     = syscall.NewLazyDLL("kernel32.dll")
	inheritedLaneLockFileEx   = inheritedLaneKernel32.NewProc("LockFileEx")
	inheritedLaneUnlockFileEx = inheritedLaneKernel32.NewProc("UnlockFileEx")
)

const (
	inheritedLaneLockfileFailImmediately = 0x00000001
	inheritedLaneLockfileExclusive       = 0x00000002
	inheritedLaneLockOffsetLow           = 0xffffffff
	inheritedLaneLockOffsetHigh          = 0x3fffffff
)

func (lease *Lease) DuplicateLaneLockForChild() (*os.File, error) {
	if lease == nil || lease.externalLaneFile == nil || strings.TrimSpace(lease.externalLanePath) == "" || strings.TrimSpace(lease.laneID) == "" {
		return nil, fmt.Errorf("lane mutation lease has no inheritable lane lock")
	}
	if err := lease.Validate(); err != nil {
		return nil, err
	}
	current, err := syscall.GetCurrentProcess()
	if err != nil {
		return nil, err
	}
	var duplicate syscall.Handle
	if err := syscall.DuplicateHandle(
		current,
		syscall.Handle(lease.externalLaneFile.Fd()),
		current,
		&duplicate,
		0,
		true,
		syscall.DUPLICATE_SAME_ACCESS,
	); err != nil {
		return nil, fmt.Errorf("duplicate lane mutation lock for child: %w", err)
	}
	file := os.NewFile(uintptr(duplicate), lease.externalLanePath)
	if file == nil {
		_ = syscall.CloseHandle(duplicate)
		return nil, fmt.Errorf("duplicate lane mutation lock returned an invalid child handle")
	}
	pathInfo, pathErr := os.Lstat(lease.externalLanePath)
	openedInfo, openedErr := file.Stat()
	if pathErr != nil || openedErr != nil || pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() || !openedInfo.Mode().IsRegular() || !os.SameFile(pathInfo, openedInfo) {
		return nil, errors.Join(pathErr, openedErr, file.Close(), fmt.Errorf("duplicated lane mutation lock identity changed"))
	}
	return file, nil
}

func OpenInheritedLaneLease(caseRoot, laneID string, handle uintptr) (*InheritedLaneLease, error) {
	if handle == 0 || !safeLaneIDSegment.MatchString(strings.TrimSpace(laneID)) {
		return nil, fmt.Errorf("private child inherited lane mutation lease handle is invalid")
	}
	casePath, err := filepath.Abs(strings.TrimSpace(caseRoot))
	if err != nil {
		return nil, err
	}
	identity, err := projectlock.CanonicalProjectIdentity(casePath)
	if err != nil {
		return nil, err
	}
	root, err := projectlock.WorkstreamRoot()
	if err != nil {
		return nil, err
	}
	expectedPath := filepath.Join(root, externalLaneLockName(identity, strings.TrimSpace(laneID)))
	file := os.NewFile(handle, expectedPath)
	if file == nil {
		return nil, fmt.Errorf("private child inherited lane mutation lease handle is unavailable")
	}
	proof := &InheritedLaneLease{
		file:           file,
		path:           expectedPath,
		validateNative: requireParentLaneLockHeld,
	}
	if err := proof.Validate(); err != nil {
		return nil, errors.Join(err, proof.Close())
	}
	return proof, nil
}

func requireParentLaneLockHeld(handle uintptr) error {
	overlapped := syscall.Overlapped{
		Offset:     inheritedLaneLockOffsetLow,
		OffsetHigh: inheritedLaneLockOffsetHigh,
	}
	flags := uintptr(inheritedLaneLockfileExclusive | inheritedLaneLockfileFailImmediately)
	locked, _, callErr := inheritedLaneLockFileEx.Call(
		handle,
		flags,
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&overlapped)),
	)
	if locked == 0 {
		if errors.Is(callErr, syscall.Errno(33)) {
			return nil
		}
		return fmt.Errorf("probe inherited lane mutation lock: %w", callErr)
	}
	unlocked, _, unlockErr := inheritedLaneUnlockFileEx.Call(
		handle,
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&overlapped)),
	)
	if unlocked == 0 {
		return fmt.Errorf("release unexpected inherited lane mutation lock probe: %w", unlockErr)
	}
	return fmt.Errorf("inherited lane mutation lease is not held by a parent process")
}
