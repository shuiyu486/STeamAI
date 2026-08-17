//go:build windows

package processguard

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

func TestRunTreeHelper(t *testing.T) {
	mode := os.Getenv("REKIT_PROCESS_TREE_HELPER")
	if mode == "" {
		return
	}
	pidPath := os.Getenv("REKIT_PROCESS_TREE_PID")
	switch mode {
	case "marker":
		if err := os.WriteFile(pidPath, []byte("executed\n"), 0o600); err != nil {
			os.Exit(91)
		}
	case "spawn-and-block", "spawn-and-exit":
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
			os.Exit(92)
		}
		if err := os.WriteFile(
			pidPath,
			[]byte(strconv.Itoa(cmd.Process.Pid)),
			0o600,
		); err != nil {
			_ = cmd.Process.Kill()
			os.Exit(93)
		}
		if mode == "spawn-and-block" {
			for {
				time.Sleep(time.Hour)
			}
		}
	case "exit-nonzero":
		_, _ = fmt.Fprint(os.Stdout, "stdout sentinel\n")
		_, _ = fmt.Fprint(os.Stderr, "stderr sentinel\n")
		os.Exit(27)
	default:
		os.Exit(94)
	}
}

func TestRunTreeDeadlineKillsPipeHoldingDescendant(t *testing.T) {
	pidPath := filepath.Join(t.TempDir(), "descendant.pid")
	cmd := processTreeHelperCommand("spawn-and-block", pidPath)
	output := filepath.Join(t.TempDir(), "output.txt")
	outputFile, err := os.Create(output)
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
	requireRecordedProcessDead(t, pidPath)
}

func TestRunTreeRootExitKillsPipeHoldingDescendant(t *testing.T) {
	pidPath := filepath.Join(t.TempDir(), "descendant.pid")
	cmd := processTreeHelperCommand("spawn-and-exit", pidPath)
	output := filepath.Join(t.TempDir(), "output.txt")
	outputFile, err := os.Create(output)
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
	requireRecordedProcessDead(t, pidPath)
}

func TestRunTreeObserverFailurePreventsSuspendedExecution(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "marker.txt")
	cmd := processTreeHelperCommand("marker", marker)
	runErr := RunTreeObserved(
		context.Background(),
		cmd,
		func() error { return errors.New("injected observation failure") },
	)
	if runErr == nil || !strings.Contains(runErr.Error(), "observation failure") {
		t.Fatalf("process tree observer failure = %v", runErr)
	}
	if _, err := os.Lstat(marker); !os.IsNotExist(err) {
		t.Fatalf("suspended process executed before failed observation: %v", err)
	}
}

func TestRunTreePreservesNonzeroExitAndOutput(t *testing.T) {
	cmd := processTreeHelperCommand("exit-nonzero", "")
	outputPath := filepath.Join(t.TempDir(), "output.txt")
	outputFile, err := os.Create(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdout = outputFile
	cmd.Stderr = outputFile
	runErr := RunTree(context.Background(), cmd)
	if err := outputFile.Close(); err != nil {
		t.Fatal(err)
	}
	var exitErr *exec.ExitError
	if !errors.As(runErr, &exitErr) || exitErr.ExitCode() != 27 {
		t.Fatalf("process tree nonzero exit = %v", runErr)
	}
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "stdout sentinel") ||
		!strings.Contains(string(output), "stderr sentinel") {
		t.Fatalf("process tree mixed output = %q", output)
	}
}

func TestRunTreeAlreadyCanceledDoesNotStart(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "marker.txt")
	cmd := processTreeHelperCommand("marker", marker)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := RunTree(ctx, cmd); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled process tree error = %v", err)
	}
	if cmd.Process != nil {
		t.Fatalf("pre-canceled process tree started pid=%d", cmd.Process.Pid)
	}
}

func processTreeHelperCommand(mode, marker string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=TestRunTreeHelper")
	cmd.Env = append(
		os.Environ(),
		"REKIT_PROCESS_TREE_HELPER="+mode,
		"REKIT_PROCESS_TREE_PID="+marker,
	)
	return cmd
}

func requireRecordedProcessDead(t *testing.T, pidPath string) {
	t.Helper()
	pidData, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		t.Fatal(err)
	}
	if processTreeTestAlive(pid) {
		t.Fatalf("process tree descendant survived: pid=%d", pid)
	}
}

func processTreeTestAlive(pid int) bool {
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
