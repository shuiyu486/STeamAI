//go:build windows

package lanemutation

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"golang.org/x/sys/windows"
)

const inheritedLaneLeaseHelperEnv = "STEAMAI_INHERITED_LANE_LEASE_HELPER"

func TestInheritedLaneLeaseProvesParentLockAcrossProcess(t *testing.T) {
	caseRoot := inheritedLaneLeaseFixture(t)
	lease, err := AcquireOpenLane(caseRoot, "main", "inherited lane lease test")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Unlock()

	childFile, err := lease.DuplicateLaneLockForChild()
	if err != nil {
		t.Fatal(err)
	}
	defer childFile.Close()

	cmd := exec.Command(
		os.Args[0],
		"-test.run=^TestInheritedLaneLeaseChildHelper$",
		"--",
		caseRoot,
		"main",
		strconv.FormatUint(uint64(childFile.Fd()), 10),
	)
	cmd.Env = append(os.Environ(), inheritedLaneLeaseHelperEnv+"=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		AdditionalInheritedHandles: []syscall.Handle{syscall.Handle(childFile.Fd())},
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("inherited lane lease child failed: %v\n%s", err, output)
	}
}

func TestInheritedLaneLeaseRejectsSeparatelyOpenedExactHandle(t *testing.T) {
	caseRoot := inheritedLaneLeaseFixture(t)
	lease, err := AcquireOpenLane(caseRoot, "main", "inherited lane lease test")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Unlock()

	err = openInheritedLaneLeaseTestHandleExpectError(lease.externalLanePath)
	if err == nil || !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		t.Fatalf("separately opened exact lane handle error = %v", err)
	}
}

func TestInheritedLaneLeaseRejectsExactHandleWithoutParentLock(t *testing.T) {
	caseRoot := inheritedLaneLeaseFixture(t)
	lease, err := AcquireOpenLane(caseRoot, "main", "inherited lane lease test")
	if err != nil {
		t.Fatal(err)
	}
	lockPath := lease.externalLanePath
	if err := lease.Unlock(); err != nil {
		t.Fatal(err)
	}

	handle := openInheritedLaneLeaseTestHandle(t, lockPath)
	proof, proofErr := OpenInheritedLaneLease(caseRoot, "main", uintptr(handle))
	if proof != nil {
		_ = proof.Close()
	}
	if proofErr == nil || !strings.Contains(proofErr.Error(), "not held by a parent process") {
		t.Fatalf("unlocked exact lane handle proof error = %v", proofErr)
	}
}

func openInheritedLaneLeaseTestHandle(t *testing.T, path string) syscall.Handle {
	t.Helper()
	handle, err := openInheritedLaneLeaseTestHandleRaw(path)
	if err != nil {
		t.Fatal(err)
	}
	return handle
}

func openInheritedLaneLeaseTestHandleExpectError(path string) error {
	handle, err := openInheritedLaneLeaseTestHandleRaw(path)
	if err == nil {
		_ = syscall.CloseHandle(handle)
	}
	return err
}

func openInheritedLaneLeaseTestHandleRaw(path string) (syscall.Handle, error) {
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	const fileShareDelete = 0x00000004
	return syscall.CreateFile(
		pointer,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|fileShareDelete,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
}

func TestInheritedLaneLeaseChildHelper(t *testing.T) {
	if os.Getenv(inheritedLaneLeaseHelperEnv) != "1" {
		return
	}
	separator := -1
	for index, arg := range os.Args {
		if arg == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || len(os.Args) != separator+4 {
		t.Fatalf("inherited lane lease helper received invalid arguments: %v", os.Args)
	}
	handle, err := strconv.ParseUint(os.Args[separator+3], 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := OpenInheritedLaneLease(os.Args[separator+1], os.Args[separator+2], uintptr(handle))
	if err != nil {
		t.Fatal(err)
	}
	defer proof.Close()
	if err := proof.Validate(); err != nil {
		t.Fatal(err)
	}
}

func inheritedLaneLeaseFixture(t *testing.T) string {
	t.Helper()
	caseRoot := t.TempDir()
	stateRoot := filepath.Join(caseRoot, projectstate.CurrentDir)
	laneRoot := filepath.Join(stateRoot, "lanes", "main")
	if err := os.MkdirAll(laneRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateRoot, "instance.yml"), []byte("projectName: inherited-lease-test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(laneRoot, "lane.json"), []byte(`{"id":"main","status":"open"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return caseRoot
}
