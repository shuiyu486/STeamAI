//go:build windows

package adapterhost

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/shuiyu486/re-context-kits/internal/rekit/processguard"
)

func TestContainedProcessHelper(t *testing.T) {
	if os.Getenv("REKIT_CONTAINED_PROCESS_HELPER") == "" {
		return
	}
	pidPath := os.Getenv("REKIT_CONTAINED_PROCESS_PID")
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "Start-Sleep -Seconds 30")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		os.Exit(91)
	}
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
		_ = cmd.Process.Kill()
		os.Exit(92)
	}
	select {}
}

func TestRunContainedProcessTimeoutKillsPipeHoldingDescendant(t *testing.T) {
	current, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binding, err := processguard.LockExecutable(current, 128<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer binding.Close()
	pidPath := filepath.Join(t.TempDir(), "descendant.pid")
	env := append(os.Environ(),
		"REKIT_CONTAINED_PROCESS_HELPER=1",
		"REKIT_CONTAINED_PROCESS_PID="+pidPath,
	)
	started := time.Now()
	_, _, _, runErr := runContainedProcess(binding, []string{"-test.run=TestContainedProcessHelper"}, env, 250*time.Millisecond)
	if runErr == nil || !strings.Contains(runErr.Error(), "exceeded dispatch runtime budget") {
		t.Fatalf("contained process timeout should fail closed: %v", runErr)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("contained process timeout waited on descendant pipes: %s", elapsed)
	}
	pidData, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		t.Fatal(err)
	}
	if processAlive(pid) {
		t.Fatalf("pipe-holding descendant survived timeout containment: pid=%d", pid)
	}
}

func processAlive(pid int) bool {
	const processQueryLimitedInformation = 0x1000
	handle, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(handle)
	var code uint32
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetExitCodeProcess")
	result, _, _ := proc.Call(uintptr(handle), uintptr(unsafe.Pointer(&code)))
	const stillActive = 259
	return result != 0 && code == stillActive
}
