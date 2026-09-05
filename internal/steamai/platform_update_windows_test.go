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
	stagedState, err := captureCanonicalUpdateState(git, staged)
	if err != nil {
		t.Fatal(err)
	}
	result, err := (nativePlatform{}).ActivateUpdate(updateInstall{
		Source: source, StagedSource: staged, ReplaceSource: true, Executable: newExe, ExecutableSHA256: hashUpdateBytes([]byte("new executable")), Version: "v2",
		ExpectedHead: baseline.Head, ExpectedStatus: baseline.Status, ExpectedRefs: baseline.Refs,
		ExpectedStagedHead: stagedState.Head, ExpectedStagedStatus: stagedState.Status, ExpectedStagedRefs: stagedState.Refs,
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
	if got, _ := os.ReadFile(filepath.Join(result.CleanupPath, "marker")); string(got) != "old source" {
		t.Fatalf("preserved source marker = %q", got)
	}
	preservedState, err := captureCanonicalUpdateState(git, result.CleanupPath)
	if err != nil || preservedState != baseline {
		t.Fatalf("preserved source state = %+v, %v", preservedState, err)
	}
}

func TestRollbackUpdatedExecutablePreservesRecoveryPaths(t *testing.T) {
	originalRemove, originalRename := removeUpdatePath, renameUpdatePath
	t.Cleanup(func() {
		removeUpdatePath, renameUpdatePath = originalRemove, originalRename
	})
	newFixture := func(t *testing.T) (string, string) {
		t.Helper()
		root := t.TempDir()
		active := filepath.Join(root, "steamai.exe")
		previous := filepath.Join(root, "steamai.previous.exe")
		if err := os.WriteFile(active, []byte("new executable"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(previous, []byte("old executable"), 0o755); err != nil {
			t.Fatal(err)
		}
		return active, previous
	}

	t.Run("remove failure preserves both executables", func(t *testing.T) {
		active, previous := newFixture(t)
		removeUpdatePath = func(string) error { return syscall.ERROR_ACCESS_DENIED }
		renameUpdatePath = originalRename
		err := rollbackUpdatedExecutable(active, previous)
		if err == nil || !strings.Contains(err.Error(), "移除未完成") || !strings.Contains(err.Error(), "旧版保留") {
			t.Fatalf("rollback error = %v", err)
		}
		for _, path := range []string{active, previous} {
			if _, statErr := os.Stat(path); statErr != nil {
				t.Fatalf("recovery executable %s missing: %v", path, statErr)
			}
		}
	})

	t.Run("rename failure preserves previous executable", func(t *testing.T) {
		active, previous := newFixture(t)
		removeUpdatePath = originalRemove
		renameUpdatePath = func(old, new string) error {
			if old == previous && new == active {
				return syscall.ERROR_ACCESS_DENIED
			}
			return originalRename(old, new)
		}
		err := rollbackUpdatedExecutable(active, previous)
		if err == nil || !strings.Contains(err.Error(), previous) {
			t.Fatalf("rollback error = %v", err)
		}
		if _, statErr := os.Stat(previous); statErr != nil {
			t.Fatalf("previous executable was not preserved: %v", statErr)
		}
	})
}

func TestActivateUpdateRollsBackExecutableAndSourceOnVersionWriteFailure(t *testing.T) {
	local := filepath.Join(t.TempDir(), "local")
	t.Setenv("LOCALAPPDATA", local)
	oldSubkey := installRegistrySubkey
	installRegistrySubkey = `Software\STeamAI-Test-` + filepath.Base(t.TempDir())
	originalSetVersion := setUpdateVersion
	t.Cleanup(func() {
		_ = deleteRegistryKey(syscall.HKEY_CURRENT_USER, installRegistrySubkey)
		installRegistrySubkey = oldSubkey
		setUpdateVersion = originalSetVersion
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
	for _, root := range []string{source, staged} {
		write(filepath.Join(root, ".claude", "skills", "steamai", "SKILL.md"), "fixture skill")
		write(filepath.Join(root, "go.mod"), "module github.com/shuiyu486/STeamAI\n")
	}
	write(filepath.Join(source, "marker"), "old source")
	write(filepath.Join(staged, "marker"), "new source")
	git, err := exec.LookPath("git.exe")
	if err != nil {
		t.Skip("git.exe is required")
	}
	for _, root := range []string{source, staged} {
		for _, args := range [][]string{{"init", "--quiet"}, {"config", "user.name", "fixture"}, {"config", "user.email", "fixture@example.invalid"}, {"add", "."}, {"commit", "--quiet", "-m", "fixture"}} {
			cmd := exec.Command(git, args...)
			cmd.Dir = root
			if output, runErr := cmd.CombinedOutput(); runErr != nil {
				t.Fatalf("git %s: %v: %s", strings.Join(args, " "), runErr, output)
			}
		}
	}
	baseline, err := captureCanonicalUpdateState(git, source)
	if err != nil {
		t.Fatal(err)
	}
	stagedState, err := captureCanonicalUpdateState(git, staged)
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
	if err := setRegistryString(key, installedVersionValue, "v1", syscall.REG_SZ); err != nil {
		t.Fatal(err)
	}
	_ = syscall.RegCloseKey(key)
	newExe := filepath.Join(t.TempDir(), "new.exe")
	write(newExe, "new executable")
	setUpdateVersion = func(syscall.Handle, string, string, uint32) error { return syscall.ERROR_ACCESS_DENIED }
	if _, err := (nativePlatform{}).ActivateUpdate(updateInstall{
		Source: source, StagedSource: staged, ReplaceSource: true, Executable: newExe, ExecutableSHA256: hashUpdateBytes([]byte("new executable")), Version: "v2",
		ExpectedHead: baseline.Head, ExpectedStatus: baseline.Status, ExpectedRefs: baseline.Refs,
		ExpectedStagedHead: stagedState.Head, ExpectedStagedStatus: stagedState.Status, ExpectedStagedRefs: stagedState.Refs,
	}); err == nil || !strings.Contains(err.Error(), "installed version") {
		t.Fatalf("version write failure returned %v", err)
	}
	if got, _ := os.ReadFile(active); string(got) != "old executable" {
		t.Fatalf("old executable was not restored: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(source, "marker")); string(got) != "old source" {
		t.Fatalf("old source was not restored: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(staged, "marker")); string(got) != "new source" {
		t.Fatalf("staged source was not preserved: %q", got)
	}
	key, err = openRegistryKey(syscall.HKEY_CURRENT_USER, installRegistrySubkey, syscall.KEY_READ)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.RegCloseKey(key)
	if version, err := queryRegistryString(key, installedVersionValue); err != nil || version != "v1" {
		t.Fatalf("installed version = %q, %v", version, err)
	}
}

func TestActivateUpdatePreservesForwardSourceWhenExecutableRollbackFails(t *testing.T) {
	local := filepath.Join(t.TempDir(), "local")
	t.Setenv("LOCALAPPDATA", local)
	oldSubkey := installRegistrySubkey
	installRegistrySubkey = `Software\STeamAI-Test-` + filepath.Base(t.TempDir())
	originalSetVersion, originalRemove := setUpdateVersion, removeUpdatePath
	t.Cleanup(func() {
		_ = deleteRegistryKey(syscall.HKEY_CURRENT_USER, installRegistrySubkey)
		installRegistrySubkey = oldSubkey
		setUpdateVersion, removeUpdatePath = originalSetVersion, originalRemove
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
	for _, root := range []string{source, staged} {
		write(filepath.Join(root, ".claude", "skills", "steamai", "SKILL.md"), "fixture skill")
		write(filepath.Join(root, "go.mod"), "module github.com/shuiyu486/STeamAI\n")
	}
	write(filepath.Join(source, "marker"), "old source")
	write(filepath.Join(staged, "marker"), "new source")
	git, err := exec.LookPath("git.exe")
	if err != nil {
		t.Skip("git.exe is required")
	}
	for _, root := range []string{source, staged} {
		for _, args := range [][]string{{"init", "--quiet"}, {"config", "user.name", "fixture"}, {"config", "user.email", "fixture@example.invalid"}, {"add", "."}, {"commit", "--quiet", "-m", "fixture"}} {
			cmd := exec.Command(git, args...)
			cmd.Dir = root
			if output, runErr := cmd.CombinedOutput(); runErr != nil {
				t.Fatalf("git %s: %v: %s", strings.Join(args, " "), runErr, output)
			}
		}
	}
	baseline, err := captureCanonicalUpdateState(git, source)
	if err != nil {
		t.Fatal(err)
	}
	stagedState, err := captureCanonicalUpdateState(git, staged)
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
	if err := setRegistryString(key, installedVersionValue, "v1", syscall.REG_SZ); err != nil {
		t.Fatal(err)
	}
	_ = syscall.RegCloseKey(key)
	newExe := filepath.Join(t.TempDir(), "new.exe")
	write(newExe, "new executable")
	setUpdateVersion = func(syscall.Handle, string, string, uint32) error { return syscall.ERROR_ACCESS_DENIED }
	removeUpdatePath = func(string) error { return syscall.Errno(32) }
	result, err := (nativePlatform{}).ActivateUpdate(updateInstall{
		Source: source, StagedSource: staged, ReplaceSource: true, Executable: newExe, ExecutableSHA256: hashUpdateBytes([]byte("new executable")), Version: "v2",
		ExpectedHead: baseline.Head, ExpectedStatus: baseline.Status, ExpectedRefs: baseline.Refs,
		ExpectedStagedHead: stagedState.Head, ExpectedStagedStatus: stagedState.Status, ExpectedStagedRefs: stagedState.Refs,
	})
	if err == nil || !strings.Contains(err.Error(), "新版保留") || !result.PreserveStagedSource || result.CleanupPath == "" {
		t.Fatalf("partial forward recovery result=%+v err=%v", result, err)
	}
	if got, _ := os.ReadFile(active); string(got) != "new executable" {
		t.Fatalf("forward executable was not preserved: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(source, "marker")); string(got) != "new source" {
		t.Fatalf("forward source was rolled back despite active new executable: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(result.CleanupPath, "marker")); string(got) != "old source" {
		t.Fatalf("old source backup was not preserved: %q", got)
	}
}

func TestActivateUpdateRollsBackPublishedSourceOnExecutableFailure(t *testing.T) {
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
	write(filepath.Join(filepath.Dir(active), "steamai.next.exe"), "occupied")
	source := filepath.Join(t.TempDir(), "source")
	staged := filepath.Join(filepath.Dir(source), "staged-source")
	for _, root := range []string{source, staged} {
		write(filepath.Join(root, ".claude", "skills", "steamai", "SKILL.md"), "fixture skill")
		write(filepath.Join(root, "go.mod"), "module github.com/shuiyu486/STeamAI\n")
	}
	write(filepath.Join(source, "marker"), "old source")
	write(filepath.Join(staged, "marker"), "new source")
	git, err := exec.LookPath("git.exe")
	if err != nil {
		t.Skip("git.exe is required")
	}
	for _, root := range []string{source, staged} {
		for _, args := range [][]string{{"init", "--quiet"}, {"config", "user.name", "fixture"}, {"config", "user.email", "fixture@example.invalid"}, {"add", "."}, {"commit", "--quiet", "-m", "fixture"}} {
			cmd := exec.Command(git, args...)
			cmd.Dir = root
			if output, runErr := cmd.CombinedOutput(); runErr != nil {
				t.Fatalf("git %s: %v: %s", strings.Join(args, " "), runErr, output)
			}
		}
	}
	baseline, err := captureCanonicalUpdateState(git, source)
	if err != nil {
		t.Fatal(err)
	}
	stagedState, err := captureCanonicalUpdateState(git, staged)
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
	if _, err := (nativePlatform{}).ActivateUpdate(updateInstall{
		Source: source, StagedSource: staged, ReplaceSource: true, Executable: newExe, ExecutableSHA256: hashUpdateBytes([]byte("new executable")), Version: "v2",
		ExpectedHead: baseline.Head, ExpectedStatus: baseline.Status, ExpectedRefs: baseline.Refs,
		ExpectedStagedHead: stagedState.Head, ExpectedStagedStatus: stagedState.Status, ExpectedStagedRefs: stagedState.Refs,
	}); err == nil {
		t.Fatal("occupied next executable did not fail activation")
	}
	if got, _ := os.ReadFile(filepath.Join(source, "marker")); string(got) != "old source" {
		t.Fatalf("old source was not restored: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(staged, "marker")); string(got) != "new source" {
		t.Fatalf("new staged source was not preserved: %q", got)
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
		Source: source, ReplaceSource: false, Executable: newExe, ExecutableSHA256: hashUpdateBytes([]byte("new executable")), Version: "v2",
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
		Source: source, Executable: newExe, ExecutableSHA256: hashUpdateBytes([]byte("new executable")), Version: "v2",
		ExpectedHead: baseline.Head, ExpectedStatus: baseline.Status, ExpectedRefs: baseline.Refs,
	})
	if err == nil {
		t.Fatal("final source drift was accepted")
	}
	if got, _ := os.ReadFile(active); string(got) != "old executable" {
		t.Fatal("executable changed despite final source drift")
	}
}
