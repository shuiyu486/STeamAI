//go:build windows

package steamai

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
)

func TestNativeCommanderMutexRejectsDuplicate(t *testing.T) {
	p := nativePlatform{}
	name := commanderMutexName(t.TempDir())
	lease, err := p.AcquireCommander(name)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.release()
	if second, err := p.AcquireCommander(name); !errors.Is(err, errCommanderRunning) {
		if second.release != nil {
			second.release()
		}
		t.Fatalf("duplicate mutex returned %v", err)
	}
}

func TestNativeCanonicalMutationMutexRejectsDuplicate(t *testing.T) {
	p := nativePlatform{}
	name := canonicalMutationMutexName(t.TempDir())
	lease, err := p.AcquireCanonicalMutation(name)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.release()
	if second, err := p.AcquireCanonicalMutation(name); !errors.Is(err, errCanonicalMutationRunning) {
		if second.release != nil {
			second.release()
		}
		t.Fatalf("duplicate canonical mutation mutex returned %v", err)
	}
}

func TestInstallExecutableDoesNotOverwritePrecreatedHardlink(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.exe")
	target := filepath.Join(root, "steamai.exe")
	guard := filepath.Join(root, "guard.exe")
	if err := os.WriteFile(source, []byte("new executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(guard, []byte("guard content"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(guard, target); err != nil {
		t.Skipf("hardlink unavailable: %v", err)
	}
	if err := installExecutable(source, target); err == nil {
		t.Fatal("预置 hardlink target 未被拒绝")
	}
	got, err := os.ReadFile(guard)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "guard content" {
		t.Fatal("预置 hardlink 内容被覆盖")
	}
}

func TestUninstallCleanupArgsRejectOutsideInstallDirectory(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	active, err := (nativePlatform{}).ActiveExecutable()
	if err != nil {
		t.Fatal(err)
	}
	allowed := filepath.Join(filepath.Dir(active), "steamai.uninstalling.exe")
	if _, paths, err := parseUninstallCleanupArgs([]string{"--parent-pid", "42", "--path", allowed}); err != nil || len(paths) != 1 {
		t.Fatalf("allowed cleanup args rejected: paths=%v err=%v", paths, err)
	}
	outside := filepath.Join(t.TempDir(), "outside.exe")
	if _, _, err := parseUninstallCleanupArgs([]string{"--parent-pid", "42", "--path", outside}); err == nil {
		t.Fatal("outside cleanup path accepted")
	}
}

func TestUninstallCleanupExecutableRequiresTemporaryHelper(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if err := requireUninstallCleanupExecutable(); err == nil {
		t.Fatal("non-helper executable accepted as uninstall cleanup")
	}
}

func TestWindowsEnvironmentBlockRoundTrips(t *testing.T) {
	block, err := windowsEnvironmentBlock([]string{"A=1", "UNICODE=测试"})
	if err != nil {
		t.Fatal(err)
	}
	wantSuffix := []uint16{0, 0}
	if len(block) < 2 || !reflect.DeepEqual(block[len(block)-2:], wantSuffix) {
		t.Fatalf("environment block does not end in two NULs: %#v", block)
	}
	if got := syscall.UTF16ToString(block); got != "A=1" {
		t.Fatalf("first environment entry = %q", got)
	}
}
