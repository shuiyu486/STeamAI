//go:build windows

package processguard

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func TestProcessGuardHelper(t *testing.T) {
	mode := os.Getenv("REKIT_PROCESSGUARD_HELPER")
	if mode == "" {
		return
	}
	marker := os.Getenv("REKIT_PROCESSGUARD_MARKER")
	switch mode {
	case "marker":
		if err := os.WriteFile(marker, []byte("executed\n"), 0o600); err != nil {
			os.Exit(91)
		}
	case "descendant":
		cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "Start-Sleep -Seconds 30")
		if err := cmd.Start(); err != nil {
			os.Exit(92)
		}
		if err := os.WriteFile(marker, []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
			_ = cmd.Process.Kill()
			os.Exit(93)
		}
	case "breakaway-child":
		cmd := helperCommand(os.Args[0], "marker", marker)
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x01000000}
		if err := cmd.Run(); err != nil {
			os.Exit(95)
		}
	default:
		os.Exit(94)
	}
	os.Exit(0)
}

func TestValidateContainAndResumeRejectsMismatchedImageBeforeExecution(t *testing.T) {
	current, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(t.TempDir(), "bound-copy.exe")
	data, err := os.ReadFile(current)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copyPath, data, 0o700); err != nil {
		t.Fatal(err)
	}
	binding, err := LockExecutable(copyPath, 128<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer binding.Close()
	marker := filepath.Join(t.TempDir(), "marker.txt")
	cmd := helperCommand(current, "marker", marker)
	if err := ConfigureSuspended(cmd, binding); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	containment, validateErr := ValidateContainAndResume(cmd.Process, binding)
	if containment != nil {
		_ = containment.Close()
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	if validateErr == nil || !strings.Contains(validateErr.Error(), "does not match") {
		t.Fatalf("mismatched process image was accepted: %v", validateErr)
	}
	if _, err := os.Lstat(marker); !os.IsNotExist(err) {
		t.Fatalf("mismatched suspended image executed before validation: %v", err)
	}
}

func TestExecutableBindingAllowsNestedMatchingImageLaunch(t *testing.T) {
	current, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binding, err := LockExecutable(current, 128<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer binding.Close()
	marker := filepath.Join(t.TempDir(), "marker.txt")
	cmd := helperCommand(binding.Path(), "marker", marker)
	if err := cmd.Run(); err != nil {
		t.Fatalf("matching image could not launch while executable binding remained held: %v", err)
	}
	if got, err := os.ReadFile(marker); err != nil || string(got) != "executed\n" {
		t.Fatalf("nested matching image did not execute: got=%q err=%v", got, err)
	}
}

func TestValidateContainAndResumeAllowsMatchingImage(t *testing.T) {
	current, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binding, err := LockExecutable(current, 128<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer binding.Close()
	marker := filepath.Join(t.TempDir(), "marker.txt")
	cmd := helperCommand(binding.Path(), "marker", marker)
	if err := ConfigureSuspended(cmd, binding); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	containment, err := ValidateContainAndResume(cmd.Process, binding)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		_ = containment.Close()
		t.Fatal(err)
	}
	if err := containment.Close(); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(marker); err != nil || string(got) != "executed\n" {
		t.Fatalf("matching image did not execute: got=%q err=%v", got, err)
	}
}

func TestAllowBreakawayContainmentPermitsNestedSupervisorLaunch(t *testing.T) {
	current, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binding, err := LockExecutable(current, 128<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer binding.Close()
	marker := filepath.Join(t.TempDir(), "marker.txt")
	cmd := helperCommand(binding.Path(), "breakaway-child", marker)
	if err := ConfigureSuspended(cmd, binding); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	containment, err := ValidateContainAndResumeAllowBreakaway(cmd.Process, binding)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		_ = containment.Close()
		t.Fatal(err)
	}
	if err := containment.Close(); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(marker); err != nil || string(got) != "executed\n" {
		t.Fatalf("contained parent could not launch breakaway child: got=%q err=%v", got, err)
	}
}

func TestContainmentCloseTerminatesDescendantTree(t *testing.T) {
	current, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binding, err := LockExecutable(current, 128<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer binding.Close()
	pidPath := filepath.Join(t.TempDir(), "descendant.pid")
	cmd := helperCommand(binding.Path(), "descendant", pidPath)
	if err := ConfigureSuspended(cmd, binding); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	containment, err := ValidateContainAndResume(cmd.Process, binding)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		_ = containment.Close()
		t.Fatal(err)
	}
	pidData, err := os.ReadFile(pidPath)
	if err != nil {
		_ = containment.Close()
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		_ = containment.Close()
		t.Fatal(err)
	}
	const synchronize = 0x00100000
	descendant, err := syscall.OpenProcess(synchronize, false, uint32(pid))
	if err != nil {
		_ = containment.Close()
		t.Fatal(err)
	}
	defer syscall.CloseHandle(descendant)
	if err := containment.Close(); err != nil {
		t.Fatal(err)
	}
	const waitObject0 = 0
	const waitTimeout = 0x00000102
	result, err := waitForSingleObject(descendant, 5000)
	if err != nil || result == waitTimeout || result != waitObject0 {
		t.Fatalf("descendant survived containment close: result=0x%x err=%v", result, err)
	}
}

func helperCommand(path, mode, marker string) *exec.Cmd {
	cmd := exec.Command(path, "-test.run=TestProcessGuardHelper")
	cmd.Env = append(os.Environ(),
		"REKIT_PROCESSGUARD_HELPER="+mode,
		"REKIT_PROCESSGUARD_MARKER="+marker,
	)
	return cmd
}

func waitForSingleObject(handle syscall.Handle, milliseconds uint32) (uint32, error) {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("WaitForSingleObject")
	result, _, callErr := proc.Call(uintptr(handle), uintptr(milliseconds))
	if result == 0xffffffff {
		if callErr != nil && !errors.Is(callErr, syscall.Errno(0)) {
			return uint32(result), callErr
		}
		return uint32(result), syscall.EINVAL
	}
	return uint32(result), nil
}
