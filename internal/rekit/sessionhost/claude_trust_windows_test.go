//go:build windows

package sessionhost

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

func TestVerifyClaudeAuthenticodePublisherUsesLockedHandle(t *testing.T) {
	if testing.Short() {
		t.Skip("requires the installed signed Claude Code executable")
	}
	identity, err := resolveLiveAcceptanceClaude("")
	if err != nil {
		t.Skipf("signed canonical Claude Code installation unavailable: %v", err)
	}
	locked, err := openClaudeExecutableReadLock(identity.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer locked.Close()
	unsigned := filepath.Join(t.TempDir(), "unsigned.exe")
	if err := os.WriteFile(unsigned, []byte("unsigned fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	publisher, err := verifyClaudeAuthenticodePublisherWithPath(locked, unsigned)
	if err != nil {
		t.Fatalf("signed handle with unsigned display path failed: %v", err)
	}
	if publisher != liveAcceptanceClaudePublisher {
		t.Fatalf("signed handle publisher=%q", publisher)
	}
	unsignedHandle, err := openClaudeExecutableReadLock(unsigned)
	if err != nil {
		t.Fatal(err)
	}
	defer unsignedHandle.Close()
	if publisher, err := verifyClaudeAuthenticodePublisherWithPath(unsignedHandle, identity.Path); err == nil {
		t.Fatalf("unsigned handle with signed display path passed as %q", publisher)
	}
}

func TestValidateTrustedClaudeProcessRejectsDifferentSuspendedImage(t *testing.T) {
	if testing.Short() {
		t.Skip("requires the installed signed Claude Code executable")
	}
	identity, err := resolveLiveAcceptanceClaude("")
	if err != nil {
		t.Skipf("signed canonical Claude Code installation unavailable: %v", err)
	}
	binding, err := acquireClaudeExecutableLaunchBinding(Options{
		ClaudePath:                        identity.Path,
		ExpectedClaudeExecutableSHA256:    identity.SHA256,
		ExpectedClaudeExecutablePublisher: identity.Publisher,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer binding.Close()
	marker := filepath.Join(t.TempDir(), "marker.txt")
	windowsRoot, err := windowsDirectoryForTest()
	if err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(windowsRoot, "System32", "cmd.exe")
	cmd := exec.Command(helper, "/c", "type nul > "+strconv.Quote(marker))
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000004}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if err := validateAndResumeTrustedClaudeProcess(cmd.Process, binding); err == nil || (!strings.Contains(err.Error(), "invalid native image path") && !strings.Contains(err.Error(), "native image path does not match")) {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("different suspended image validation error=%v", err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("killed suspended helper unexpectedly exited successfully")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("rejected suspended helper executed marker write: %v", err)
	}
}

func windowsDirectoryForTest() (string, error) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	procedure := kernel32.NewProc("GetWindowsDirectoryW")
	buffer := make([]uint16, syscall.MAX_PATH)
	length, _, callErr := procedure.Call(
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
	)
	if length == 0 || length >= uintptr(len(buffer)) {
		return "", callErr
	}
	return syscall.UTF16ToString(buffer[:length]), nil
}

func TestOpenNativeClaudeExecutableMatchesLockedHandle(t *testing.T) {
	if testing.Short() {
		t.Skip("requires the installed signed Claude Code executable")
	}
	identity, err := resolveLiveAcceptanceClaude("")
	if err != nil {
		t.Skipf("signed canonical Claude Code installation unavailable: %v", err)
	}
	binding, err := lockTrustedClaudeExecutable(identity.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer binding.Close()
	native, err := openNativeClaudeExecutableReadLock(binding.nativePath)
	if err != nil {
		t.Fatal(err)
	}
	defer native.Close()
	expected, err := binding.file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	actual, err := native.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(expected, actual) {
		t.Fatal("native NtOpenFile handle does not match the locked Claude executable")
	}
}
