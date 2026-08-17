//go:build darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd

package processguard

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestRunTreeUnixHelper(t *testing.T) {
	mode := os.Getenv("REKIT_PROCESS_TREE_HELPER")
	if mode == "" {
		return
	}
	pidPath := os.Getenv("REKIT_PROCESS_TREE_PID")
	if mode == "exit-zero" {
		return
	}
	cmd := exec.Command("sh", "-c", "sleep 30")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		os.Exit(91)
	}
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
		_ = cmd.Process.Kill()
		os.Exit(92)
	}
	if mode == "spawn-and-block" {
		for {
			time.Sleep(time.Hour)
		}
	}
}

func TestRunTreeUnixFastExitSucceeds(t *testing.T) {
	for range 100 {
		cmd := unixProcessTreeHelperCommand("exit-zero", "")
		if err := RunTree(context.Background(), cmd); err != nil {
			t.Fatalf("fast exit failed: %v", err)
		}
	}
}

func TestRunTreeUnixDeadlineKillsPipeHoldingDescendant(t *testing.T) {
	pidPath := filepath.Join(t.TempDir(), "descendant.pid")
	cmd := unixProcessTreeHelperCommand("spawn-and-block", pidPath)
	outputFile, err := os.Create(filepath.Join(t.TempDir(), "output.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer outputFile.Close()
	cmd.Stdout = outputFile
	cmd.Stderr = outputFile
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	started := time.Now()
	runErr := RunTree(ctx, cmd)
	if !errors.Is(runErr, context.DeadlineExceeded) {
		t.Fatalf("process tree deadline error = %v", runErr)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("process tree deadline waited on descendant pipes: %s", elapsed)
	}
	requireUnixRecordedProcessDead(t, pidPath)
}

func TestRunTreeUnixRootExitKillsPipeHoldingDescendant(t *testing.T) {
	pidPath := filepath.Join(t.TempDir(), "descendant.pid")
	cmd := unixProcessTreeHelperCommand("spawn-and-exit", pidPath)
	outputFile, err := os.Create(filepath.Join(t.TempDir(), "output.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer outputFile.Close()
	cmd.Stdout = outputFile
	cmd.Stderr = outputFile
	started := time.Now()
	if err := RunTree(context.Background(), cmd); err != nil {
		t.Fatalf("process tree root-exit cleanup failed: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("process tree root exit waited on descendant pipes: %s", elapsed)
	}
	requireUnixRecordedProcessDead(t, pidPath)
}

func unixProcessTreeHelperCommand(mode, marker string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=TestRunTreeUnixHelper")
	cmd.Env = append(
		os.Environ(),
		"REKIT_PROCESS_TREE_HELPER="+mode,
		"REKIT_PROCESS_TREE_PID="+marker,
	)
	return cmd
}

func requireUnixRecordedProcessDead(t *testing.T, pidPath string) {
	t.Helper()
	pidData, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Kill(pid, 0); err == nil || errors.Is(err, unix.EPERM) {
		t.Fatalf("process tree descendant survived: pid=%d err=%v", pid, err)
	}
}
