//go:build windows

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestReleaseRunProcessHelper(t *testing.T) {
	mode := os.Getenv("REKIT_RELEASE_RUN_PROCESS_HELPER")
	if mode == "" {
		return
	}
	pidPath := os.Getenv("REKIT_RELEASE_RUN_PROCESS_PID")
	cmd := exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		"Start-Sleep -Seconds 30",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		os.Exit(91)
	}
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
		_ = cmd.Process.Kill()
		os.Exit(92)
	}
	_, _ = fmt.Fprintln(os.Stdout, "release-run helper started")
	if mode == "spawn-and-block" {
		for {
			time.Sleep(time.Hour)
		}
	}
}

func TestExecuteReleaseRunArgvDeadlineKillsPipeHoldingDescendant(t *testing.T) {
	pidPath := filepath.Join(t.TempDir(), "descendant.pid")
	t.Setenv("REKIT_RELEASE_RUN_PROCESS_HELPER", "spawn-and-block")
	t.Setenv("REKIT_RELEASE_RUN_PROCESS_PID", pidPath)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	started := time.Now()
	exitCode, output, runErr := executeReleaseRunArgv(
		ctx,
		t.TempDir(),
		os.Args[0],
		"-test.run=TestReleaseRunProcessHelper",
	)
	if !errors.Is(runErr, context.DeadlineExceeded) || exitCode == 0 {
		t.Fatalf(
			"release-run process deadline result: exit=%d output=%q err=%v",
			exitCode,
			output,
			runErr,
		)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("release-run deadline waited on descendant output: %s", elapsed)
	}
	if !strings.Contains(output, "release-run helper started") {
		t.Fatalf("release-run deadline output = %q", output)
	}
	requireReleaseRunProcessDead(t, pidPath)
}

func TestExecuteReleaseRunArgvRootExitKillsPipeHoldingDescendant(t *testing.T) {
	pidPath := filepath.Join(t.TempDir(), "descendant.pid")
	t.Setenv("REKIT_RELEASE_RUN_PROCESS_HELPER", "spawn-and-exit")
	t.Setenv("REKIT_RELEASE_RUN_PROCESS_PID", pidPath)
	started := time.Now()
	exitCode, output, runErr := executeReleaseRunArgv(
		context.Background(),
		t.TempDir(),
		os.Args[0],
		"-test.run=TestReleaseRunProcessHelper",
	)
	if runErr != nil || exitCode != 0 {
		t.Fatalf(
			"release-run root-exit result: exit=%d output=%q err=%v",
			exitCode,
			output,
			runErr,
		)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("release-run root exit waited on descendant output: %s", elapsed)
	}
	if !strings.Contains(output, "release-run helper started") {
		t.Fatalf("release-run root-exit output = %q", output)
	}
	requireReleaseRunProcessDead(t, pidPath)
}

func requireReleaseRunProcessDead(t *testing.T, pidPath string) {
	t.Helper()
	pidData, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		t.Fatal(err)
	}
	if releaseRunProcessAlive(pid) {
		t.Fatalf("release-run descendant survived: pid=%d", pid)
	}
}

func releaseRunProcessAlive(pid int) bool {
	const processQueryLimitedInformation = 0x1000
	handle, err := syscall.OpenProcess(
		processQueryLimitedInformation,
		false,
		uint32(pid),
	)
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(handle)
	var code uint32
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetExitCodeProcess")
	result, _, _ := proc.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&code)),
	)
	const stillActive = 259
	return result != 0 && code == stillActive
}
