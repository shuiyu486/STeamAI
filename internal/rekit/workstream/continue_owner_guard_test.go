package workstream

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestContinueOwnerGuardRejectsMissingStaleAndMismatchedBindingsWithoutWrites(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	tests := []struct {
		name               string
		opt                ContinueOptions
		want               string
		wantReceived       string
		wantReceivedGen    int
		wantCurrentCommand string
	}{
		{name: "missing", opt: ContinueOptions{Selector: "devirt-main"}, want: "requires explicit Executor", wantReceived: "", wantReceivedGen: 0, wantCurrentCommand: "/rekit continue main -Executor executor-one -ExpectedExecutorGeneration 1"},
		{name: "missing generation", opt: ContinueOptions{Selector: "devirt-main", Executor: "executor-one"}, want: "requires positive ExpectedExecutorGeneration", wantReceived: "executor-one", wantReceivedGen: 0, wantCurrentCommand: "/rekit continue main -Executor executor-one -ExpectedExecutorGeneration 1"},
		{name: "executor mismatch", opt: ContinueOptions{Selector: "devirt-main", Executor: "executor-two", ExpectedExecutorGeneration: 1}, want: "owner guard is not current", wantReceived: "executor-two", wantReceivedGen: 1, wantCurrentCommand: "/rekit continue main -Executor executor-one -ExpectedExecutorGeneration 1"},
		{name: "generation mismatch", opt: ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 2}, want: "owner guard is not current", wantReceived: "executor-one", wantReceivedGen: 2, wantCurrentCommand: "/rekit continue main -Executor executor-one -ExpectedExecutorGeneration 1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := snapshotWorkstreamTree(t, caseRoot)
			preview, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, tc.opt)
			if err != nil {
				t.Fatalf("ContinuePreview returned error instead of fail-closed recovery: %v", err)
			}
			assertContinueOwnerGuardRecovery(t, preview, tc.want, tc.wantReceived, tc.wantReceivedGen, "executor-one", 1, tc.wantCurrentCommand)
			if after := snapshotWorkstreamTree(t, caseRoot); after != before {
				t.Fatalf("ContinuePreview mutated case\nbefore:\n%s\nafter:\n%s", before, after)
			}
			applied, err := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, tc.opt)
			if err != nil {
				t.Fatalf("ContinueApply returned error instead of fail-closed recovery: %v", err)
			}
			assertContinueOwnerGuardRecovery(t, applied, tc.want, tc.wantReceived, tc.wantReceivedGen, "executor-one", 1, tc.wantCurrentCommand)
			if applied.IsMutation != true || applied.Applied {
				t.Fatalf("ContinueApply recovery should report attempted mutation but no apply: %+v", applied)
			}
			if after := snapshotWorkstreamTree(t, caseRoot); after != before {
				t.Fatalf("ContinueApply mutated case\nbefore:\n%s\nafter:\n%s", before, after)
			}
		})
	}
}

func assertContinueOwnerGuardRecovery(t *testing.T, result ContinueResult, wantReason, wantReceived string, wantReceivedGen int, wantCurrentExecutor string, wantCurrentGen int, wantCurrentCommand string) {
	t.Helper()
	recovery := result.ContinueOwnerGuardRecovery
	if !result.Blocked || result.Applied || recovery == nil || !recovery.Ready {
		t.Fatalf("continue owner guard did not return fail-closed recovery: %+v", result)
	}
	if !strings.Contains(recovery.Reason, wantReason) || recovery.ReceivedExecutor != wantReceived || recovery.ReceivedExecutorGeneration != wantReceivedGen || recovery.CurrentExecutor != wantCurrentExecutor || recovery.CurrentExecutorGeneration != wantCurrentGen || recovery.CurrentContinueCommand != wantCurrentCommand {
		t.Fatalf("continue owner guard recovery drifted: %+v", recovery)
	}
	if recovery.ResumePath != ".rekit/lanes/devirt-main/prompts/RESUME.md" || recovery.CheckpointPath != ".rekit/lanes/devirt-main/checkpoints/latest.json" || recovery.HandoffPath != ".rekit/handovers/devirt-main-latest.md" || recovery.LaneTakeoverPackage == nil || recovery.LaneTakeoverPackage.ContinueCommand != wantCurrentCommand {
		t.Fatalf("continue owner guard recovery omitted durable takeover paths/package: %+v", recovery)
	}
	if !strings.Contains(recovery.StartTakeoverPreviewCommand, "/rekit start main -WhatIf -Executor <new-executor>") || !strings.Contains(recovery.StartTakeoverApplyCommand, "/rekit start main -Apply -Executor <new-executor>") || !containsString(recovery.Boundary, "owner guard mismatch is fail-closed and zero-write") || !containsString(recovery.Boundary, "recovery package is read-only guidance; it does not claim a new executor") || !containsString(recovery.Boundary, "no authority/confirmed writes or heavy-tool execution") {
		t.Fatalf("continue owner guard recovery omitted takeover commands or boundary: %+v", recovery)
	}
	if len(result.Writes) != 0 || len(result.WouldWrites) != 0 || !containsString(result.BlockedActions, "lane continuation with a stale or missing executor owner guard") {
		t.Fatalf("continue owner guard recovery should remain zero-write: writes=%+v would=%+v blocked=%+v", result.Writes, result.WouldWrites, result.BlockedActions)
	}
}

func TestContinueOwnerGuardAllowsExplicitCurrentBinding(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	opt := ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1}
	preview, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Lane.CurrentExecutor != "executor-one" || preview.Lane.ExecutorGeneration != 1 || preview.Applied {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	applied, err := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.Lane.CurrentExecutor != "executor-one" || applied.Lane.ExecutorGeneration != 1 {
		t.Fatalf("unexpected apply result: %+v", applied)
	}
}

func TestContinueLegacyUnassignedLaneAllowsOnlyOmittedGuard(t *testing.T) {
	repoRoot, caseRoot := setupContinueCase(t, "")
	if _, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{Selector: "devirt-main"}); err != nil {
		t.Fatalf("legacy omitted guard should remain compatible: %v", err)
	}
	before := snapshotWorkstreamTree(t, caseRoot)
	blocked, err := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1})
	if err != nil {
		t.Fatalf("legacy explicit mismatch returned error instead of recovery: %v", err)
	}
	assertContinueOwnerGuardRecovery(t, blocked, "legacy unassigned lane", "executor-one", 1, "", 0, "/rekit continue main")
	if after := snapshotWorkstreamTree(t, caseRoot); after != before {
		t.Fatalf("legacy mismatch mutated case\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestLaneMutationLockRejectsSymlinkMetadataRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		if err := os.Symlink(t.TempDir(), filepath.Join(t.TempDir(), "probe")); err != nil {
			t.Skip("symlink creation unavailable")
		}
	}
	caseRoot := t.TempDir()
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(caseRoot, ".rekit")); err != nil {
		t.Fatal(err)
	}
	if _, err := acquireLaneMutationLock(caseRoot, "devirt-main"); err == nil || !strings.Contains(err.Error(), "must be a directory and not a symlink") {
		t.Fatalf("symlink metadata root error = %v", err)
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("symlink target received lock writes: %+v", entries)
	}
}

func TestLaneMutationUnlockReportsFailure(t *testing.T) {
	_, caseRoot := setupOwnedContinueCase(t)
	lease, err := acquireLaneMutationLock(caseRoot, "devirt-main")
	if err != nil {
		t.Fatal(err)
	}
	lease.SetUnlockFileForTest(func(uintptr) error { return os.ErrPermission })
	if err := lease.Unlock(); err == nil || !strings.Contains(err.Error(), os.ErrPermission.Error()) {
		t.Fatalf("Unlock error = %v, want visible injected failure", err)
	}
}

func TestLaneMutationLockSerializesCaseSymlinkAlias(t *testing.T) {
	_, caseRoot := setupOwnedContinueCase(t)
	aliasRoot := filepath.Join(t.TempDir(), "case-alias")
	if err := os.Symlink(caseRoot, aliasRoot); err != nil {
		t.Skipf("case symlink unavailable: %v", err)
	}
	lease, err := acquireLaneMutationLock(caseRoot, "devirt-main")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		aliasLease, err := acquireLaneMutationLock(aliasRoot, "devirt-main")
		if err == nil {
			err = aliasLease.Unlock()
		}
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("case alias acquired a second lease: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	if err := lease.Unlock(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("case alias did not acquire after canonical lease released")
	}
}

func TestLaneMutationLockSerializesAcrossDifferentCacheEnvironments(t *testing.T) {
	_, caseRoot := setupOwnedContinueCase(t)
	readyPath := filepath.Join(t.TempDir(), "ready")
	releasePath := filepath.Join(t.TempDir(), "release")
	cacheOne := t.TempDir()
	cacheTwo := t.TempDir()
	first := exec.Command(os.Args[0], "-test.run=^TestLaneMutationLockHelperProcess$")
	first.Env = lockHelperEnvironment(caseRoot, cacheOne, readyPath, releasePath)
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	firstDone := make(chan error, 1)
	go func() { firstDone <- first.Wait() }()
	waitForPath(t, readyPath)

	secondDone := make(chan error, 1)
	go func() {
		cmd := exec.Command(os.Args[0], "-test.run=^TestLaneMutationLockHelperProcess$")
		cmd.Env = lockHelperEnvironment(caseRoot, cacheTwo, "", "")
		output, err := cmd.CombinedOutput()
		if err != nil {
			secondDone <- fmt.Errorf("second helper failed: %w\n%s", err, output)
			return
		}
		secondDone <- nil
	}()
	select {
	case err := <-secondDone:
		if err != nil {
			t.Fatal(err)
		}
		t.Fatal("second process acquired the same case/lane lease through a different cache environment")
	case <-time.After(150 * time.Millisecond):
	}
	if err := os.WriteFile(releasePath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first lock helper did not release")
	}
	select {
	case err := <-secondDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("second process did not acquire after first released")
	}
}

func TestLaneMutationLockSurvivesMetadataRebindAcrossDifferentCacheEnvironments(t *testing.T) {
	_, caseRoot := setupOwnedContinueCase(t)
	before := snapshotWorkstreamTree(t, caseRoot)
	readyPath := filepath.Join(t.TempDir(), "ready")
	releasePath := filepath.Join(t.TempDir(), "release")
	first := exec.Command(os.Args[0], "-test.run=^TestLaneMutationLockHelperProcess$")
	first.Env = lockHelperEnvironment(caseRoot, t.TempDir(), readyPath, releasePath)
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	firstDone := make(chan error, 1)
	go func() { firstDone <- first.Wait() }()
	waitForPath(t, readyPath)

	metadataPath := filepath.Join(caseRoot, ".rekit")
	movedPath := filepath.Join(caseRoot, ".rekit-moved")
	if err := os.Rename(metadataPath, movedPath); err != nil {
		_ = os.WriteFile(releasePath, nil, 0o600)
		<-firstDone
		t.Skipf("metadata namespace cannot be rebound while helper holds it: %v", err)
	}
	if err := copyWorkstreamTree(movedPath, metadataPath); err != nil {
		t.Fatal(err)
	}
	replacementBefore := snapshotWorkstreamTree(t, caseRoot)
	secondDone := make(chan error, 1)
	go func() {
		cmd := exec.Command(os.Args[0], "-test.run=^TestLaneMutationLockHelperProcess$")
		cmd.Env = lockHelperEnvironment(caseRoot, t.TempDir(), "", "")
		secondDone <- cmd.Run()
	}()
	select {
	case err := <-secondDone:
		t.Fatalf("replacement metadata acquired a second lease: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	if after := snapshotWorkstreamTree(t, caseRoot); after != replacementBefore {
		t.Fatalf("held writer mutated replacement metadata\nbefore:\n%s\nafter:\n%s", replacementBefore, after)
	}
	if err := os.WriteFile(releasePath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-firstDone:
		if err == nil || !strings.Contains(err.Error(), "exit status") {
			t.Fatalf("old namespace helper should fail identity validation on unlock: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first helper did not exit after release")
	}
	select {
	case err := <-secondDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("replacement metadata helper did not acquire after old lease released")
	}
	if before == "" {
		t.Fatal("unexpected empty original snapshot")
	}
	if after := snapshotWorkstreamTree(t, caseRoot); after != replacementBefore {
		t.Fatalf("old writer mutated replacement metadata during release\nbefore:\n%s\nafter:\n%s", replacementBefore, after)
	}
}

func TestLaneMutationLockRecoversAfterProcessExit(t *testing.T) {
	_, caseRoot := setupOwnedContinueCase(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestLaneMutationLockHelperProcess$")
	cmd.Env = append(os.Environ(), "REKIT_LOCK_HELPER=1", "REKIT_LOCK_CASE_ROOT="+caseRoot)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("lock helper failed: %v\n%s", err, output)
	}
	lease, err := acquireLaneMutationLock(caseRoot, "devirt-main")
	if err != nil {
		t.Fatalf("process exit should release kernel lease: %v", err)
	}
	if err := lease.Unlock(); err != nil {
		t.Fatal(err)
	}
}

func TestLaneMutationLockHelperProcess(t *testing.T) {
	if os.Getenv("REKIT_LOCK_HELPER") != "1" {
		return
	}
	lease, err := acquireLaneMutationLock(os.Getenv("REKIT_LOCK_CASE_ROOT"), "devirt-main")
	if err != nil {
		t.Fatal(err)
	}
	if readyPath := os.Getenv("REKIT_LOCK_READY_PATH"); readyPath != "" {
		if err := os.WriteFile(readyPath, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if releasePath := os.Getenv("REKIT_LOCK_RELEASE_PATH"); releasePath != "" {
		for !refsf.Exists(releasePath) {
			time.Sleep(10 * time.Millisecond)
		}
		if err := lease.Unlock(); err != nil {
			t.Fatal(err)
		}
		return
	}
	runtime.KeepAlive(lease)
}

func lockHelperEnvironment(caseRoot, cacheRoot, readyPath, releasePath string) []string {
	env := append([]string{}, os.Environ()...)
	env = append(env,
		"REKIT_LOCK_HELPER=1",
		"REKIT_LOCK_CASE_ROOT="+caseRoot,
		"REKIT_LOCK_READY_PATH="+readyPath,
		"REKIT_LOCK_RELEASE_PATH="+releasePath,
		"XDG_CACHE_HOME="+cacheRoot,
		"HOME="+cacheRoot,
		"LOCALAPPDATA="+cacheRoot,
	)
	return env
}

func waitForPath(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !refsf.Exists(path) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestLaneMutationLeaseRejectsMetadataRebindBeforeMutation(t *testing.T) {
	_, caseRoot := setupOwnedContinueCase(t)
	metadataPath := filepath.Join(caseRoot, ".rekit")
	lease, err := acquireLaneMutationLock(caseRoot, "devirt-main")
	if err != nil {
		t.Fatal(err)
	}
	movedPath := filepath.Join(caseRoot, ".rekit-moved")
	if err := os.Rename(metadataPath, movedPath); err != nil {
		if unlockErr := lease.Unlock(); unlockErr != nil {
			t.Fatal(unlockErr)
		}
		t.Skipf("metadata namespace cannot be rebound on this platform while handle is open: %v", err)
	}
	if err := os.Mkdir(metadataPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := lease.Validate(); err == nil || !strings.Contains(err.Error(), "namespace changed") {
		t.Fatalf("metadata rebind validation error = %v", err)
	}
	if err := lease.Unlock(); err == nil || !strings.Contains(err.Error(), "namespace changed") {
		t.Fatalf("metadata rebind unlock error = %v", err)
	}
}

func TestStartApplyRejectsNamespaceRebindBeforeBusinessMutation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows metadata directory handle denies rename; Unix cross-rename behavior is covered here")
	}
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	before := snapshotWorkstreamTree(t, caseRoot)
	metadataPath := filepath.Join(caseRoot, ".rekit")
	movedPath := filepath.Join(caseRoot, ".rekit-moved")
	workstreamMutationAfterLockHook = func(*workstreamMutationLease) {
		workstreamMutationAfterLockHook = nil
		if err := os.Rename(metadataPath, movedPath); err != nil {
			t.Skipf("metadata namespace cannot be rebound on this platform while handle is open: %v", err)
		}
		if err := copyWorkstreamTree(movedPath, metadataPath); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { workstreamMutationAfterLockHook = nil })
	_, err := StartApply(repoRoot, caseRoot, defaults.DefaultPack, StartOptions{Selector: "devirt-main", Executor: "executor-two", Actor: "main-agent", TakeoverReason: "must not write replacement namespace"})
	if err == nil || !strings.Contains(err.Error(), "namespace changed") {
		t.Fatalf("StartApply namespace rebind error = %v", err)
	}
	after := snapshotWorkstreamTree(t, caseRoot)
	if after != before {
		t.Fatalf("replacement namespace received business mutation\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func copyWorkstreamTree(source, target string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(target, rel)
		if info.IsDir() {
			return os.MkdirAll(dest, info.Mode().Perm())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dest, data, info.Mode().Perm())
	})
}

func TestLegacyAttachedCaseMutationLeaseUsesLegacyInstanceBinding(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	canonical := filepath.Join(caseRoot, ".rekit", "instance.yml")
	if err := os.Remove(canonical); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(caseRoot, ".re-template.yml")
	if _, err := os.Stat(legacy); err != nil {
		t.Fatal(err)
	}
	lease, err := acquireLaneMutationLock(caseRoot, "devirt-main")
	if err != nil {
		t.Fatalf("legacy attached case lease failed: %v", err)
	}
	if !strings.EqualFold(filepath.Clean(lease.InstancePath()), filepath.Clean(legacy)) {
		t.Fatalf("legacy lease instance path = %s, want %s", lease.InstancePath(), legacy)
	}
	if err := lease.Unlock(); err != nil {
		t.Fatal(err)
	}
	preview, err := HandoffPreview(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{Selector: "devirt-main"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := HandoffApply(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{Selector: "devirt-main", ExpectedPublicationPlanSHA256: preview.PublicationPlanSHA256, PublicationStamp: preview.PublicationStamp}); err != nil {
		t.Fatalf("legacy attached case handoff failed: %v", err)
	}
}

func TestLaneMutationLeaseRejectsCanonicalLaneRebindBeforeMutation(t *testing.T) {
	_, caseRoot := setupOwnedContinueCase(t)
	lease, err := acquireLaneMutationLock(caseRoot, "devirt-main")
	if err != nil {
		t.Fatal(err)
	}
	movedPath := lease.CanonicalLanePath() + ".moved"
	if err := os.Rename(lease.CanonicalLanePath(), movedPath); err != nil {
		_ = lease.Unlock()
		t.Skipf("canonical lane cannot be rebound on this platform while handle is open: %v", err)
	}
	if err := os.WriteFile(lease.CanonicalLanePath(), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := lease.Validate(); err == nil || !strings.Contains(err.Error(), "canonical lane namespace changed") {
		t.Fatalf("canonical lane rebind validation error = %v", err)
	}
	if err := lease.Unlock(); err == nil || !strings.Contains(err.Error(), "canonical lane namespace changed") {
		t.Fatalf("canonical lane rebind unlock error = %v", err)
	}
}

func TestLaneMutationLeaseRejectsCanonicalInstanceRebindBeforeMutation(t *testing.T) {
	_, caseRoot := setupOwnedContinueCase(t)
	lease, err := acquireLaneMutationLock(caseRoot, "devirt-main")
	if err != nil {
		t.Fatal(err)
	}
	movedPath := lease.InstancePath() + ".moved"
	if err := os.Rename(lease.InstancePath(), movedPath); err != nil {
		_ = lease.Unlock()
		t.Skipf("canonical instance cannot be rebound on this platform while handle is open: %v", err)
	}
	if err := os.WriteFile(lease.InstancePath(), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := lease.Validate(); err == nil || !strings.Contains(err.Error(), "canonical instance namespace changed") {
		t.Fatalf("canonical instance rebind validation error = %v", err)
	}
	if err := lease.Unlock(); err == nil || !strings.Contains(err.Error(), "canonical instance namespace changed") {
		t.Fatalf("canonical instance rebind unlock error = %v", err)
	}
}

func TestLaneMutationLeaseCannotBeReboundToSecondCanonicalLease(t *testing.T) {
	_, caseRoot := setupOwnedContinueCase(t)
	metadataPath := filepath.Join(caseRoot, ".rekit")
	oldLease, err := acquireLaneMutationLock(caseRoot, "devirt-main")
	if err != nil {
		t.Fatal(err)
	}
	movedPath := filepath.Join(caseRoot, ".rekit-moved")
	if err := os.Rename(metadataPath, movedPath); err != nil {
		_ = oldLease.Unlock()
		t.Skipf("metadata namespace cannot be rebound on this platform while handle is open: %v", err)
	}
	if err := os.Mkdir(metadataPath, 0o755); err != nil {
		t.Fatal(err)
	}
	acquired := make(chan *workstreamMutationLease, 1)
	errCh := make(chan error, 1)
	go func() {
		lease, err := acquireLaneMutationLock(caseRoot, "devirt-main")
		if err != nil {
			errCh <- err
			return
		}
		acquired <- lease
	}()
	select {
	case lease := <-acquired:
		_ = lease.Unlock()
		t.Fatal("replacement namespace acquired a second canonical lane lease while old lease remained held")
	case err := <-errCh:
		t.Fatalf("replacement namespace acquisition failed instead of blocking: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := oldLease.Validate(); err == nil {
		t.Fatal("old writer remained mutation-valid after metadata rebind")
	}
	if err := oldLease.Unlock(); err == nil || !strings.Contains(err.Error(), "namespace changed") {
		t.Fatalf("old lease unlock error = %v", err)
	}
	select {
	case lease := <-acquired:
		if err := lease.Unlock(); err != nil {
			t.Fatal(err)
		}
	case err := <-errCh:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		t.Fatal("replacement namespace did not acquire canonical lease after old lease released")
	}
}

func TestConcurrentTakeoverAndContinueRejectsStaleCallerBeforeRunWrite(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	lease, err := acquireLaneMutationLock(caseRoot, "devirt-main")
	if err != nil {
		t.Fatal(err)
	}

	takeoverDone := make(chan error, 1)
	go func() {
		_, err := StartApply(repoRoot, caseRoot, defaults.DefaultPack, StartOptions{Selector: "devirt-main", Executor: "executor-two", Actor: "main-agent", TakeoverReason: "test replacement"})
		takeoverDone <- err
	}()
	if err := lease.Unlock(); err != nil {
		t.Fatal(err)
	}
	if err := <-takeoverDone; err != nil {
		t.Fatal(err)
	}

	stale, continueErr := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1})
	lane, err := readLaneByID(caseRoot, "devirt-main")
	if err != nil {
		t.Fatal(err)
	}
	if lane.CurrentExecutor != "executor-two" || lane.ExecutorGeneration != 2 {
		t.Fatalf("takeover owner = %s/%d, want executor-two/2", lane.CurrentExecutor, lane.ExecutorGeneration)
	}
	if continueErr != nil {
		t.Fatalf("stale continue returned error instead of recovery: %v", continueErr)
	}
	assertContinueOwnerGuardRecovery(t, stale, "owner guard is not current", "executor-one", 1, "executor-two", 2, "/rekit continue main -Executor executor-two -ExpectedExecutorGeneration 2")
	entries, err := os.ReadDir(filepath.Join(caseRoot, ".rekit", "runs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("stale continue created run entries: %+v", entries)
	}
}

func TestLaneHandoffApplyRejectsPreviewAfterTakeoverWithoutPublishing(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	preview, err := HandoffPreview(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{Selector: "devirt-main"})
	if err != nil {
		t.Fatal(err)
	}
	var hookErr error
	handoffApplyBeforeLockHook = func() {
		handoffApplyBeforeLockHook = nil
		_, hookErr = StartApply(repoRoot, caseRoot, defaults.DefaultPack, StartOptions{Selector: "devirt-main", Executor: "executor-two", Actor: "main-agent", TakeoverReason: "replacement during handoff"})
	}
	t.Cleanup(func() { handoffApplyBeforeLockHook = nil })
	_, err = HandoffApply(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{Selector: "devirt-main", ExpectedPublicationPlanSHA256: preview.PublicationPlanSHA256, PublicationStamp: preview.PublicationStamp})
	if hookErr != nil {
		t.Fatal(hookErr)
	}
	if err == nil || !strings.Contains(err.Error(), "publication plan sha256 mismatch") {
		t.Fatalf("stale handoff error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(caseRoot, ".rekit", "handovers", "devirt-main-latest.md")); !os.IsNotExist(statErr) {
		t.Fatalf("stale handoff published latest markdown: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(caseRoot, ".rekit", "handovers", "devirt-main-latest-replacement-executor-takeover.json")); !os.IsNotExist(statErr) {
		t.Fatalf("stale handoff published latest takeover artifact: %v", statErr)
	}
}

func TestLaneHandoffApplyRejectsInboxDriftWithoutPublishing(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	preview, err := HandoffPreview(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{Selector: "devirt-main"})
	if err != nil {
		t.Fatal(err)
	}
	inbox := filepath.Join(caseRoot, ".rekit", "lanes", "devirt-main", "inbox.jsonl")
	file, err := os.OpenFile(inbox, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(`{"eventId":"evt-preview-drift","kind":"message","summary":"changed after preview"}` + "\n"); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = HandoffApply(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{Selector: "devirt-main", ExpectedPublicationPlanSHA256: preview.PublicationPlanSHA256, PublicationStamp: preview.PublicationStamp})
	if err == nil || !strings.Contains(err.Error(), "publication plan sha256 mismatch") {
		t.Fatalf("inbox drift handoff error = %v", err)
	}
	for _, path := range []string{
		filepath.Join(caseRoot, ".rekit", "handovers", "devirt-main-latest.md"),
		filepath.Join(caseRoot, ".rekit", "handovers", "devirt-main-latest-replacement-executor-takeover.json"),
	} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("inbox drift handoff published %s: %v", path, statErr)
		}
	}
}

func TestLaneHandoffApplyRejectsInputDriftBeforeFirstWrite(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	preview, err := HandoffPreview(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{Selector: "devirt-main"})
	if err != nil {
		t.Fatal(err)
	}
	inbox := filepath.Join(caseRoot, ".rekit", "lanes", "devirt-main", "inbox.jsonl")
	resumePath := filepath.Join(caseRoot, ".rekit", "lanes", "devirt-main", "prompts", "RESUME.md")
	checkpointPath := filepath.Join(caseRoot, ".rekit", "lanes", "devirt-main", "checkpoints", "latest.json")
	resumeBefore, err := os.ReadFile(resumePath)
	if err != nil {
		t.Fatal(err)
	}
	checkpointBefore, err := os.ReadFile(checkpointPath)
	if err != nil {
		t.Fatal(err)
	}
	handoffApplyBeforeWriteHook = func() {
		handoffApplyBeforeWriteHook = nil
		file, hookErr := os.OpenFile(inbox, os.O_APPEND|os.O_WRONLY, 0o600)
		if hookErr != nil {
			t.Error(hookErr)
			return
		}
		defer file.Close()
		if _, hookErr := file.WriteString(`{"eventId":"evt-plan-write-drift","kind":"message","summary":"changed before first write"}` + "\n"); hookErr != nil {
			t.Error(hookErr)
		}
	}
	t.Cleanup(func() { handoffApplyBeforeWriteHook = nil })
	_, err = HandoffApply(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{Selector: "devirt-main", ExpectedPublicationPlanSHA256: preview.PublicationPlanSHA256, PublicationStamp: preview.PublicationStamp})
	if err == nil || !strings.Contains(err.Error(), "inputs changed before first write") {
		t.Fatalf("pre-write drift handoff error = %v", err)
	}
	resumeAfter, readErr := os.ReadFile(resumePath)
	if readErr != nil || !bytes.Equal(resumeBefore, resumeAfter) {
		t.Fatalf("pre-write drift changed RESUME.md: err=%v", readErr)
	}
	checkpointAfter, readErr := os.ReadFile(checkpointPath)
	if readErr != nil || !bytes.Equal(checkpointBefore, checkpointAfter) {
		t.Fatalf("pre-write drift changed checkpoint: err=%v", readErr)
	}
	for _, path := range []string{
		filepath.Join(caseRoot, ".rekit", "handovers", "devirt-main-latest.md"),
		filepath.Join(caseRoot, ".rekit", "handovers", "devirt-main-latest-replacement-executor-takeover.json"),
	} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("pre-write drift handoff published %s: %v", path, statErr)
		}
	}
}

func TestLaneHandoffApplyFailureDoesNotAdvanceCommittedGeneration(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	firstPreview, err := HandoffPreview(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{Selector: "devirt-main"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := HandoffApply(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{
		Selector:                      "devirt-main",
		ExpectedPublicationPlanSHA256: firstPreview.PublicationPlanSHA256,
		PublicationStamp:              firstPreview.PublicationStamp,
	}); err != nil {
		t.Fatal(err)
	}
	generationPath := filepath.Join(caseRoot, ".rekit", "handovers", "devirt-main-latest-generation.json")
	generationBefore, err := os.ReadFile(generationPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := StartApply(repoRoot, caseRoot, defaults.DefaultPack, StartOptions{
		Selector:       "devirt-main",
		Executor:       "executor-two",
		Actor:          "main-agent",
		TakeoverReason: "next committed generation",
	}); err != nil {
		t.Fatal(err)
	}
	secondPreview, err := HandoffPreview(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{Selector: "devirt-main"})
	if err != nil {
		t.Fatal(err)
	}
	handoffApplyAfterWriteHook = func(path string, generation bool) error {
		if !generation && strings.HasSuffix(filepath.ToSlash(path), "/devirt-main-latest-replacement-executor-takeover.json") {
			handoffApplyAfterWriteHook = nil
			return fmt.Errorf("injected publication failure before generation commit")
		}
		return nil
	}
	t.Cleanup(func() { handoffApplyAfterWriteHook = nil })
	_, err = HandoffApply(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{
		Selector:                      "devirt-main",
		ExpectedPublicationPlanSHA256: secondPreview.PublicationPlanSHA256,
		PublicationStamp:              secondPreview.PublicationStamp,
	})
	if err == nil || !strings.Contains(err.Error(), "injected publication failure before generation commit") {
		t.Fatalf("mid-publication handoff error = %v", err)
	}
	generationAfter, err := os.ReadFile(generationPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generationBefore, generationAfter) {
		t.Fatal("failed publication advanced latest generation commit")
	}
	if !generationTargetMismatch(t, caseRoot, generationBefore) {
		t.Fatal("failed publication did not leave an observable mixed generation")
	}
}

func generationTargetMismatch(t *testing.T, caseRoot string, data []byte) bool {
	t.Helper()
	var generation HandoffPublicationGeneration
	if err := json.Unmarshal(data, &generation); err != nil {
		t.Fatal(err)
	}
	for _, entry := range generation.Entries {
		path := filepath.Join(caseRoot, filepath.FromSlash(entry.Path))
		published, err := os.ReadFile(path)
		if err != nil {
			return true
		}
		canonical, ready := CanonicalHandoffPublicationBytes(published, generation.PublicationPlanSHA256, entry.PlanSHA256Occurrences)
		if !ready {
			return true
		}
		sum := sha256.Sum256(canonical)
		if len(canonical) != entry.Bytes || !strings.EqualFold(hex.EncodeToString(sum[:]), entry.CanonicalSHA256) {
			return true
		}
	}
	return false
}

func TestProjectHandoffApplySerializesAndRejectsStalePreviewWithoutWrites(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	preview, err := HandoffPreview(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var hookErr error
	handoffApplyBeforeLockHook = func() {
		handoffApplyBeforeLockHook = nil
		_, hookErr = StartApply(repoRoot, caseRoot, defaults.DefaultPack, StartOptions{Selector: "devirt-main", Executor: "executor-two", Actor: "main-agent", TakeoverReason: "replacement during project handoff"})
	}
	t.Cleanup(func() { handoffApplyBeforeLockHook = nil })
	done := make(chan error, 1)
	go func() {
		_, err := HandoffApply(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{ExpectedPublicationPlanSHA256: preview.PublicationPlanSHA256, PublicationStamp: preview.PublicationStamp})
		done <- err
	}()
	select {
	case err := <-done:
		if hookErr != nil {
			t.Fatal(hookErr)
		}
		if err == nil || !strings.Contains(err.Error(), "publication plan sha256 mismatch") {
			t.Fatalf("stale project handoff error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("project handoff deadlocked with lane takeover serialization")
	}
	for _, path := range []string{
		filepath.Join(caseRoot, ".rekit", "handovers", "latest.md"),
		filepath.Join(caseRoot, ".rekit", "handovers", "latest-replacement-executor-takeover.json"),
	} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("stale project handoff published %s: %v", path, statErr)
		}
	}
}

func TestReconcileTakeoverSerializesWithContinueAndMakesCallerStale(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	intervention := map[string]any{
		"schemaVersion": 1,
		"eventId":       "intervention-reconcile-lock",
		"kind":          "intervention",
		"lane":          "devirt-main",
		"status":        "open",
		"subject":       "replace executor",
	}
	if _, _, err := mission.AppendFact(caseRoot, "intervention", intervention); err != nil {
		t.Fatal(err)
	}
	lease, err := acquireLaneMutationLock(caseRoot, "devirt-main")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := ReconcileApply(repoRoot, caseRoot, defaults.DefaultPack, ReconcileOptions{Selector: "devirt-main", InterventionID: "intervention-reconcile-lock", Actor: "main-agent", Executor: "executor-two", Reason: "replacement"})
		done <- err
	}()
	if err := lease.Unlock(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	stale, continueErr := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1})
	if continueErr != nil {
		t.Fatalf("continue after reconcile takeover returned error instead of recovery: %v", continueErr)
	}
	assertContinueOwnerGuardRecovery(t, stale, "owner guard is not current", "executor-one", 1, "executor-two", 2, "/rekit continue main -Executor executor-two -ExpectedExecutorGeneration 2")
	entries, err := os.ReadDir(filepath.Join(caseRoot, ".rekit", "runs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("stale continue created run entries after reconcile: %+v", entries)
	}
}

func TestReconcileDistinctInterventionAdvancesSameExecutorGeneration(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	appendIntervention := func(eventID string) {
		t.Helper()
		intervention := map[string]any{
			"schemaVersion": 1,
			"eventId":       eventID,
			"kind":          "intervention",
			"lane":          "devirt-main",
			"status":        "open",
			"subject":       "retry current executor with corrected evidence",
		}
		if _, _, err := mission.AppendFact(caseRoot, "intervention", intervention); err != nil {
			t.Fatal(err)
		}
	}

	appendIntervention("intervention-same-executor-generation-2")
	firstOpt := ReconcileOptions{
		Selector: "devirt-main", InterventionID: "intervention-same-executor-generation-2",
		Actor: "main-agent", Executor: "executor-one", Reason: "first corrected attempt",
	}
	firstPreview, err := ReconcilePreview(repoRoot, caseRoot, defaults.DefaultPack, firstOpt)
	if err != nil {
		t.Fatal(err)
	}
	if firstPreview.ExecutorGeneration != 1 || !strings.Contains(firstPreview.ExecutorAction.ResumeCommand, "-ExpectedExecutorGeneration 2") {
		t.Fatalf("same-executor preview did not project the next generation: %+v", firstPreview)
	}
	first, err := ReconcileApply(repoRoot, caseRoot, defaults.DefaultPack, firstOpt)
	if err != nil {
		t.Fatal(err)
	}
	if first.PreviousExecutor != "executor-one" || first.Executor != "executor-one" || first.ExecutorGeneration != 2 || first.Lane.ExecutorGeneration != 2 {
		t.Fatalf("first same-executor reconcile did not advance generation: %+v", first)
	}

	replay, err := ReconcileApply(repoRoot, caseRoot, defaults.DefaultPack, firstOpt)
	if err != nil {
		t.Fatal(err)
	}
	if replay.ExecutorGeneration != 2 || replay.Lane.ExecutorGeneration != 2 {
		t.Fatalf("same intervention replay advanced generation: %+v", replay)
	}

	appendIntervention("intervention-same-executor-generation-3")
	second, err := ReconcileApply(repoRoot, caseRoot, defaults.DefaultPack, ReconcileOptions{
		Selector: "devirt-main", InterventionID: "intervention-same-executor-generation-3",
		Actor: "main-agent", Executor: "executor-one", Reason: "second corrected attempt",
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.ExecutorGeneration != 3 || second.Lane.ExecutorGeneration != 3 {
		t.Fatalf("distinct same-executor intervention reused prior generation: %+v", second)
	}
}

func TestDifferentLaneWriterBlocksOnProjectLease(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	if _, err := StartApply(repoRoot, caseRoot, defaults.DefaultPack, StartOptions{Selector: "feature-other", Executor: "executor-other", Actor: "main-agent", TakeoverReason: "second lane setup"}); err != nil {
		t.Fatal(err)
	}
	lease, err := acquireLaneMutationLock(caseRoot, "devirt-main")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := StartApply(repoRoot, caseRoot, defaults.DefaultPack, StartOptions{Selector: "feature-other", Executor: "executor-replacement", Actor: "main-agent", TakeoverReason: "must wait for project lease"})
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
		t.Fatal("different lane writer bypassed the project lease")
	case <-time.After(150 * time.Millisecond):
	}
	if err := lease.Unlock(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("different lane writer did not resume after project lease release")
	}
}

func TestSerializedDifferentLaneTakeoversPreserveBothBoardOwners(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	if _, err := StartApply(repoRoot, caseRoot, defaults.DefaultPack, StartOptions{Selector: "feature-other", Executor: "executor-other", Actor: "main-agent", TakeoverReason: "second lane setup"}); err != nil {
		t.Fatal(err)
	}
	lease, err := acquireLaneMutationLock(caseRoot, "devirt-main")
	if err != nil {
		t.Fatal(err)
	}
	mainDone := make(chan error, 1)
	otherDone := make(chan error, 1)
	go func() {
		_, err := StartApply(repoRoot, caseRoot, defaults.DefaultPack, StartOptions{Name: "devirt-main", Selector: "devirt-main", Executor: "executor-main-two", Actor: "main-agent", TakeoverReason: "main takeover"})
		mainDone <- err
	}()
	go func() {
		_, err := StartApply(repoRoot, caseRoot, defaults.DefaultPack, StartOptions{Name: "feature-other", Selector: "feature-other", Executor: "executor-other-two", Actor: "main-agent", TakeoverReason: "other takeover"})
		otherDone <- err
	}()
	select {
	case err := <-mainDone:
		t.Fatalf("main takeover completed while project lease held: %v", err)
	case err := <-otherDone:
		t.Fatalf("other takeover completed while project lease held: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	if err := lease.Unlock(); err != nil {
		t.Fatal(err)
	}
	for name, done := range map[string]<-chan error{"main": mainDone, "other": otherDone} {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("%s takeover failed: %v", name, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s takeover did not complete", name)
		}
	}
	b, err := readBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	owners := map[string]string{}
	for _, lane := range b.Lanes {
		owners[lane.ID] = lane.CurrentExecutor
	}
	if owners["devirt-main"] != "executor-main-two" || owners["feature-feature-other"] != "executor-other-two" {
		t.Fatalf("board lost a serialized lane takeover: %+v", owners)
	}
}

func setupOwnedContinueCase(t *testing.T) (string, string) {
	t.Helper()
	return setupContinueCase(t, "executor-one")
}

func setupContinueCase(t *testing.T, executor string) (string, string) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	repoRoot, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	caseRoot := t.TempDir()
	if _, err := casebind.WriteInstance(caseRoot, repoRoot, defaults.DefaultPack, "continue-owner-guard"); err != nil {
		t.Fatal(err)
	}
	if _, err := StartApply(repoRoot, caseRoot, defaults.DefaultPack, StartOptions{Selector: "bootstrap"}); err != nil {
		t.Fatal(err)
	}
	if executor != "" {
		opt := StartOptions{Selector: "devirt-main", Executor: executor, Actor: "main-agent", TakeoverReason: "test owner registration"}
		if _, err := StartApply(repoRoot, caseRoot, defaults.DefaultPack, opt); err != nil {
			t.Fatal(err)
		}
	}
	return repoRoot, caseRoot
}

func snapshotWorkstreamTree(t *testing.T, caseRoot string) string {
	t.Helper()
	root := filepath.Join(caseRoot, ".rekit")
	rows := []string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			rows = append(rows, filepath.ToSlash(rel)+"/")
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rows = append(rows, filepath.ToSlash(rel)+"="+string(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(rows, "\n")
}
