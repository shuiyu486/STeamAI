//go:build windows

package steamai

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestActivateUpdateSwitchesSourceAndExecutable(t *testing.T) {
	local := filepath.Join(t.TempDir(), "local")
	t.Setenv("LOCALAPPDATA", local)
	oldSubkey := installRegistrySubkey
	installRegistrySubkey = `Software\STeamAI-Test-` + filepath.Base(t.TempDir())
	t.Cleanup(func() {
		_ = deleteRegistryKey(syscall.HKEY_CURRENT_USER, installRegistrySubkey)
		installRegistrySubkey = oldSubkey
	})
	active, err := (nativePlatform{}).ActiveExecutable()
	if err != nil {
		t.Fatal(err)
	}
	write := func(path, value string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(active, "old executable")
	source := filepath.Join(t.TempDir(), "source")
	staged := filepath.Join(filepath.Dir(source), "staged-source")
	write(filepath.Join(source, "marker"), "old source")
	write(filepath.Join(staged, "marker"), "new source")
	write(filepath.Join(source, ".claude", "skills", "steamai", "SKILL.md"), "fixture skill")
	write(filepath.Join(staged, ".claude", "skills", "steamai", "SKILL.md"), "fixture skill")
	write(filepath.Join(source, "go.mod"), "module github.com/shuiyu486/STeamAI\n")
	write(filepath.Join(staged, "go.mod"), "module github.com/shuiyu486/STeamAI\n")
	newExe := filepath.Join(t.TempDir(), "new.exe")
	write(newExe, "new executable")

	key, err := createRegistryKey(syscall.HKEY_CURRENT_USER, installRegistrySubkey, syscall.KEY_READ|syscall.KEY_WRITE)
	if err != nil {
		t.Skipf("HKCU registry unavailable: %v", err)
	}
	if err := setRegistryString(key, canonicalSourceValue, source, 1); err != nil {
		t.Fatal(err)
	}
	if err := setRegistryString(key, installedVersionValue, "v1", 1); err != nil {
		t.Fatal(err)
	}
	_ = syscall.RegCloseKey(key)

	git, err := exec.LookPath("git.exe")
	if err != nil {
		t.Skip("git.exe is required")
	}
	initCleanRepository := func(root string) {
		t.Helper()
		for _, args := range [][]string{{"init", "--quiet"}, {"config", "user.name", "fixture"}, {"config", "user.email", "fixture@example.invalid"}, {"add", "."}, {"commit", "--quiet", "-m", "fixture"}} {
			cmd := exec.Command(git, args...)
			cmd.Dir = root
			if output, runErr := cmd.CombinedOutput(); runErr != nil {
				t.Fatalf("git %s: %v: %s", strings.Join(args, " "), runErr, output)
			}
		}
	}
	initCleanRepository(source)
	initCleanRepository(staged)
	baseline, err := captureCanonicalUpdateState(git, source)
	if err != nil {
		t.Fatal(err)
	}
	result, err := (nativePlatform{}).ActivateUpdate(updateInstall{
		Source: source, StagedSource: staged, ReplaceSource: true, Executable: newExe, Version: "v2",
		ExpectedHead: baseline.Head, ExpectedStatus: baseline.Status, ExpectedRefs: baseline.Refs,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(source, "marker")); string(got) != "new source" {
		t.Fatalf("source marker = %q", got)
	}
	if got, _ := os.ReadFile(active); string(got) != "new executable" {
		t.Fatalf("active executable = %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(filepath.Dir(active), "steamai.previous.exe")); string(got) != "old executable" {
		t.Fatalf("previous executable = %q", got)
	}
	if result.CleanupPath == "" {
		t.Fatal("source backup cleanup path missing")
	}
}

func TestActivateUpdateSwitchesExecutableWithoutReplacingSource(t *testing.T) {
	local := filepath.Join(t.TempDir(), "local")
	t.Setenv("LOCALAPPDATA", local)
	oldSubkey := installRegistrySubkey
	installRegistrySubkey = `Software\STeamAI-Test-` + filepath.Base(t.TempDir())
	t.Cleanup(func() {
		_ = deleteRegistryKey(syscall.HKEY_CURRENT_USER, installRegistrySubkey)
		installRegistrySubkey = oldSubkey
	})
	active, err := (nativePlatform{}).ActiveExecutable()
	if err != nil {
		t.Fatal(err)
	}
	write := func(path, value string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(active, "old executable")
	source := filepath.Join(t.TempDir(), "source")
	write(filepath.Join(source, "marker"), "same source")
	write(filepath.Join(source, ".claude", "skills", "steamai", "SKILL.md"), "fixture skill")
	write(filepath.Join(source, "go.mod"), "module github.com/shuiyu486/STeamAI\n")
	git, err := exec.LookPath("git.exe")
	if err != nil {
		t.Skip("git.exe is required")
	}
	for _, args := range [][]string{{"init", "--quiet"}, {"config", "user.name", "fixture"}, {"config", "user.email", "fixture@example.invalid"}, {"add", "."}, {"commit", "--quiet", "-m", "fixture"}} {
		cmd := exec.Command(git, args...)
		cmd.Dir = source
		if output, runErr := cmd.CombinedOutput(); runErr != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), runErr, output)
		}
	}
	baseline, err := captureCanonicalUpdateState(git, source)
	if err != nil {
		t.Fatal(err)
	}
	key, err := createRegistryKey(syscall.HKEY_CURRENT_USER, installRegistrySubkey, syscall.KEY_READ|syscall.KEY_WRITE)
	if err != nil {
		t.Skipf("HKCU registry unavailable: %v", err)
	}
	if err := setRegistryString(key, canonicalSourceValue, source, syscall.REG_SZ); err != nil {
		t.Fatal(err)
	}
	_ = syscall.RegCloseKey(key)
	newExe := filepath.Join(t.TempDir(), "new.exe")
	write(newExe, "new executable")
	result, err := (nativePlatform{}).ActivateUpdate(updateInstall{
		Source: source, ReplaceSource: false, Executable: newExe, Version: "v2",
		ExpectedHead: baseline.Head, ExpectedStatus: baseline.Status, ExpectedRefs: baseline.Refs,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CleanupPath != "" {
		t.Fatalf("exe-only update returned source cleanup path %q", result.CleanupPath)
	}
	if got, _ := os.ReadFile(filepath.Join(source, "marker")); string(got) != "same source" {
		t.Fatal("exe-only update changed canonical source")
	}
	if got, _ := os.ReadFile(active); string(got) != "new executable" {
		t.Fatal("exe-only update did not switch executable")
	}
}

func TestActivateUpdateRejectsFinalSourceDrift(t *testing.T) {
	local := filepath.Join(t.TempDir(), "local")
	t.Setenv("LOCALAPPDATA", local)
	oldSubkey := installRegistrySubkey
	installRegistrySubkey = `Software\STeamAI-Test-` + filepath.Base(t.TempDir())
	t.Cleanup(func() {
		_ = deleteRegistryKey(syscall.HKEY_CURRENT_USER, installRegistrySubkey)
		installRegistrySubkey = oldSubkey
	})
	active, err := (nativePlatform{}).ActiveExecutable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(active), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(active, []byte("old executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ path, value string }{
		{"marker", "old source"},
		{filepath.Join(".claude", "skills", "steamai", "SKILL.md"), "fixture skill"},
		{"go.mod", "module github.com/shuiyu486/STeamAI\n"},
	} {
		path := filepath.Join(source, item.path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(item.value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git, err := exec.LookPath("git.exe")
	if err != nil {
		t.Skip("git.exe is required")
	}
	for _, args := range [][]string{{"init", "--quiet"}, {"config", "user.name", "fixture"}, {"config", "user.email", "fixture@example.invalid"}, {"add", "."}, {"commit", "--quiet", "-m", "fixture"}} {
		cmd := exec.Command(git, args...)
		cmd.Dir = source
		if output, runErr := cmd.CombinedOutput(); runErr != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), runErr, output)
		}
	}
	baseline, err := captureCanonicalUpdateState(git, source)
	if err != nil {
		t.Fatal(err)
	}
	key, err := createRegistryKey(syscall.HKEY_CURRENT_USER, installRegistrySubkey, syscall.KEY_READ|syscall.KEY_WRITE)
	if err != nil {
		t.Skipf("HKCU registry unavailable: %v", err)
	}
	if err := setRegistryString(key, canonicalSourceValue, source, syscall.REG_SZ); err != nil {
		t.Fatal(err)
	}
	_ = syscall.RegCloseKey(key)
	if err := os.WriteFile(filepath.Join(source, "marker"), []byte("concurrent learning"), 0o644); err != nil {
		t.Fatal(err)
	}
	newExe := filepath.Join(t.TempDir(), "new.exe")
	if err := os.WriteFile(newExe, []byte("new executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err = (nativePlatform{}).ActivateUpdate(updateInstall{
		Source: source, Executable: newExe, Version: "v2",
		ExpectedHead: baseline.Head, ExpectedStatus: baseline.Status, ExpectedRefs: baseline.Refs,
	})
	if err == nil {
		t.Fatal("final source drift was accepted")
	}
	if got, _ := os.ReadFile(active); string(got) != "old executable" {
		t.Fatal("executable changed despite final source drift")
	}
}
