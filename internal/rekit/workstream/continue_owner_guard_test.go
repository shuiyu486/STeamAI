package workstream

import (
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
		name string
		opt  ContinueOptions
		want string
	}{
		{name: "missing", opt: ContinueOptions{Selector: "devirt-main"}, want: "requires explicit Executor"},
		{name: "missing generation", opt: ContinueOptions{Selector: "devirt-main", Executor: "executor-one"}, want: "requires positive ExpectedExecutorGeneration"},
		{name: "executor mismatch", opt: ContinueOptions{Selector: "devirt-main", Executor: "executor-two", ExpectedExecutorGeneration: 1}, want: "owner guard is not current"},
		{name: "generation mismatch", opt: ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 2}, want: "owner guard is not current"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := snapshotWorkstreamTree(t, caseRoot)
			if _, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, tc.opt); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ContinuePreview error = %v, want containing %q", err, tc.want)
			}
			if after := snapshotWorkstreamTree(t, caseRoot); after != before {
				t.Fatalf("ContinuePreview mutated case\nbefore:\n%s\nafter:\n%s", before, after)
			}
			if _, err := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, tc.opt); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ContinueApply error = %v, want containing %q", err, tc.want)
			}
			if after := snapshotWorkstreamTree(t, caseRoot); after != before {
				t.Fatalf("ContinueApply mutated case\nbefore:\n%s\nafter:\n%s", before, after)
			}
		})
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
	_, err := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1})
	if err == nil || !strings.Contains(err.Error(), "legacy unassigned lane") {
		t.Fatalf("legacy explicit mismatch error = %v", err)
	}
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
	lease.unlockFile = func(uintptr) error { return os.ErrPermission }
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
	if !strings.EqualFold(filepath.Clean(lease.instancePath), filepath.Clean(legacy)) {
		t.Fatalf("legacy lease instance path = %s, want %s", lease.instancePath, legacy)
	}
	if err := lease.Unlock(); err != nil {
		t.Fatal(err)
	}
	if _, err := HandoffApply(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{Selector: "devirt-main"}); err != nil {
		t.Fatalf("legacy attached case handoff failed: %v", err)
	}
}

func TestLaneMutationLeaseRejectsCanonicalLaneRebindBeforeMutation(t *testing.T) {
	_, caseRoot := setupOwnedContinueCase(t)
	lease, err := acquireLaneMutationLock(caseRoot, "devirt-main")
	if err != nil {
		t.Fatal(err)
	}
	movedPath := lease.canonicalLanePath + ".moved"
	if err := os.Rename(lease.canonicalLanePath, movedPath); err != nil {
		_ = lease.Unlock()
		t.Skipf("canonical lane cannot be rebound on this platform while handle is open: %v", err)
	}
	if err := os.WriteFile(lease.canonicalLanePath, nil, 0o600); err != nil {
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
	movedPath := lease.instancePath + ".moved"
	if err := os.Rename(lease.instancePath, movedPath); err != nil {
		_ = lease.Unlock()
		t.Skipf("canonical instance cannot be rebound on this platform while handle is open: %v", err)
	}
	if err := os.WriteFile(lease.instancePath, nil, 0o600); err != nil {
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

	_, continueErr := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1})
	lane, err := readLaneByID(caseRoot, "devirt-main")
	if err != nil {
		t.Fatal(err)
	}
	if lane.CurrentExecutor != "executor-two" || lane.ExecutorGeneration != 2 {
		t.Fatalf("takeover owner = %s/%d, want executor-two/2", lane.CurrentExecutor, lane.ExecutorGeneration)
	}
	if continueErr == nil || !strings.Contains(continueErr.Error(), "owner guard is not current") {
		t.Fatalf("stale continue error = %v", continueErr)
	}
	entries, err := os.ReadDir(filepath.Join(caseRoot, ".rekit", "runs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("stale continue created run entries: %+v", entries)
	}
}

func TestLaneHandoffApplyRereadsAfterTakeoverAndDoesNotPublishStaleOwner(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	var hookErr error
	handoffApplyBeforeLockHook = func() {
		handoffApplyBeforeLockHook = nil
		_, hookErr = StartApply(repoRoot, caseRoot, defaults.DefaultPack, StartOptions{Selector: "devirt-main", Executor: "executor-two", Actor: "main-agent", TakeoverReason: "replacement during handoff"})
	}
	t.Cleanup(func() { handoffApplyBeforeLockHook = nil })
	result, err := HandoffApply(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{Selector: "devirt-main"})
	if hookErr != nil {
		t.Fatal(hookErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if result.Lane == nil || result.Lane.CurrentExecutor != "executor-two" || result.Lane.ExecutorGeneration != 2 {
		t.Fatalf("handoff result retained stale owner: %+v", result.Lane)
	}
	assertCurrentOwnerArtifacts(t, caseRoot, "executor-two", 2, "executor-one", 1, filepath.Join(".rekit", "handovers", "devirt-main-latest.md"))
}

func TestProjectHandoffApplySerializesWithoutDeadlockAndRereadsAllLanes(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	var hookErr error
	handoffApplyBeforeLockHook = func() {
		handoffApplyBeforeLockHook = nil
		_, hookErr = StartApply(repoRoot, caseRoot, defaults.DefaultPack, StartOptions{Selector: "devirt-main", Executor: "executor-two", Actor: "main-agent", TakeoverReason: "replacement during project handoff"})
	}
	t.Cleanup(func() { handoffApplyBeforeLockHook = nil })
	done := make(chan error, 1)
	go func() {
		_, err := HandoffApply(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{})
		done <- err
	}()
	select {
	case err := <-done:
		if hookErr != nil {
			t.Fatal(hookErr)
		}
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("project handoff deadlocked with lane takeover serialization")
	}
	assertCurrentOwnerArtifacts(t, caseRoot, "executor-two", 2, "executor-one", 1,
		filepath.Join(".rekit", "handovers", "latest.md"),
		filepath.Join(".rekit", "lanes", "devirt-main", "prompts", "RESUME.md"),
		filepath.Join(".rekit", "lanes", "devirt-main", "checkpoints", "latest.json"),
	)
}

func assertCurrentOwnerArtifacts(t *testing.T, caseRoot, current string, generation int, stale string, staleGeneration int, paths ...string) {
	t.Helper()
	currentBinding := "-Executor " + current + " -ExpectedExecutorGeneration " + fmt.Sprintf("%d", generation)
	staleBinding := "-Executor " + stale + " -ExpectedExecutorGeneration " + fmt.Sprintf("%d", staleGeneration)
	for _, rel := range paths {
		data, err := os.ReadFile(filepath.Join(caseRoot, rel))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), currentBinding) || strings.Contains(string(data), staleBinding) {
			t.Fatalf("artifact %s does not contain only current owner binding:\n%s", rel, data)
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
	_, continueErr := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1})
	if continueErr == nil || !strings.Contains(continueErr.Error(), "owner guard is not current") {
		t.Fatalf("continue after reconcile takeover error = %v", continueErr)
	}
	entries, err := os.ReadDir(filepath.Join(caseRoot, ".rekit", "runs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("stale continue created run entries after reconcile: %+v", entries)
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
