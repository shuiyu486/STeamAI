package sync

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/doctor"
)

func TestApplyExclusiveInitMissingRootCreatesDoctorReadyCase(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := filepath.Join(t.TempDir(), "exclusive-case")
	createdAt := time.Date(2026, 7, 23, 12, 34, 56, 789, time.FixedZone("fixture", 8*60*60))
	plan, err := PlanExclusiveInit(repoRoot, caseRoot, pack, ExclusiveInitOptions{
		ProjectName: "verification-demo",
		ProvisionID: "provision-559",
		Role:        "fresh-case",
		CreatedAt:   createdAt,
		ExtraFiles: []ExclusiveInitExtraFile{{
			Path:    ".rekit/caller-marker.txt",
			Kind:    "caller-marker",
			Content: []byte("candidate verification\n"),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(caseRoot); !os.IsNotExist(err) {
		t.Fatalf("planning wrote case root, stat err=%v", err)
	}
	if plan.ProjectName != "verification-demo" || plan.ProvisionID != "provision-559" || plan.Role != "fresh-case" || plan.CreatedAt != "2026-07-23T04:34:56.000000789Z" {
		t.Fatalf("unexpected stable identity: %+v", plan)
	}
	if len(plan.Writes) == 0 {
		t.Fatal("exclusive plan has no writes")
	}
	for _, write := range plan.Writes {
		if write.SHA256 == "" || write.Size != int64(len(write.Content)) || !bytes.Equal(write.Content, readPlannedExclusiveContent(t, plan, write.Path)) {
			t.Fatalf("write lacks exact content binding: %+v", write)
		}
	}

	result, err := ApplyExclusiveInit(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.Replay {
		t.Fatalf("unexpected first apply result: %+v", result)
	}
	if got := string(readFile(t, filepath.Join(caseRoot, ".rekit", "verification-role.json"))); !strings.Contains(got, `"provisionId": "provision-559"`) || !strings.Contains(got, `"role": "fresh-case"`) || !strings.Contains(got, `"createdAt": "2026-07-23T04:34:56.000000789Z"`) {
		t.Fatalf("unexpected verification marker:\n%s", got)
	}
	if got := string(readFile(t, filepath.Join(caseRoot, ".rekit", "caller-marker.txt"))); got != "candidate verification\n" {
		t.Fatalf("unexpected caller marker: %q", got)
	}
	assertDoctorReadyCaseFiles(t, plan)

	replay, err := ApplyExclusiveInit(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !replay.Applied || !replay.Replay {
		t.Fatalf("unexpected exact replay result: %+v", replay)
	}
}

func TestApplyExclusiveInitRealPackIsDoctorReady(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve package source path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "doctor-ready")
	plan, err := PlanExclusiveInit(repoRoot, caseRoot, "_template", exclusiveInitOptionsForTest())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyExclusiveInit(plan); err != nil {
		t.Fatal(err)
	}
	if _, err := doctor.Case(repoRoot, caseRoot, "_template"); err != nil {
		t.Fatalf("exclusive init did not create a doctor-ready attached case: %v", err)
	}
}

func TestPlanExclusiveInitRejectsExistingRoot(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := filepath.Join(t.TempDir(), "existing")
	writeText(t, filepath.Join(caseRoot, "keep.txt"), "do not take over")

	_, err := PlanExclusiveInit(repoRoot, caseRoot, pack, exclusiveInitOptionsForTest())
	if err == nil || !strings.Contains(err.Error(), "refuses existing case root") {
		t.Fatalf("PlanExclusiveInit error = %v, want existing-root rejection", err)
	}
	if got := string(readFile(t, filepath.Join(caseRoot, "keep.txt"))); got != "do not take over" {
		t.Fatalf("existing root changed: %q", got)
	}
}

func TestApplyExclusiveInitCompletesPartialExactReplay(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := filepath.Join(t.TempDir(), "partial")
	plan, err := PlanExclusiveInit(repoRoot, caseRoot, pack, exclusiveInitOptionsForTest())
	if err != nil {
		t.Fatal(err)
	}
	write := plan.Writes[0]
	writeText(t, write.TargetPath, string(write.Content))

	result, err := ApplyExclusiveInit(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || !result.Replay {
		t.Fatalf("unexpected partial replay result: %+v", result)
	}
	if got := readFile(t, write.TargetPath); !bytes.Equal(got, write.Content) {
		t.Fatal("partial exact leaf was modified")
	}
	assertDoctorReadyCaseFiles(t, plan)
}

func TestApplyExclusiveInitRejectsUnplannedObject(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := filepath.Join(t.TempDir(), "unplanned")
	plan, err := PlanExclusiveInit(repoRoot, caseRoot, pack, exclusiveInitOptionsForTest())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyExclusiveInit(plan); err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(caseRoot, "unplanned.txt"), "keep\n")

	_, err = ApplyExclusiveInit(plan)
	if err == nil || !strings.Contains(err.Error(), "unplanned object") {
		t.Fatalf("ApplyExclusiveInit error = %v, want unplanned-object rejection", err)
	}
	if got := string(readFile(t, filepath.Join(caseRoot, "unplanned.txt"))); got != "keep\n" {
		t.Fatalf("unplanned object changed: %q", got)
	}
}

func TestReserveExclusiveInitBatchCollisionLeavesNoFirstRoot(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	workspace := filepath.Join(t.TempDir(), "batch")
	firstRoot := filepath.Join(workspace, "first")
	secondRoot := filepath.Join(workspace, "second")
	first, err := PlanExclusiveInit(repoRoot, firstRoot, pack, exclusiveInitOptionsForTest())
	if err != nil {
		t.Fatal(err)
	}
	secondOpt := exclusiveInitOptionsForTest()
	secondOpt.Role = "second"
	second, err := PlanExclusiveInit(repoRoot, secondRoot, pack, secondOpt)
	if err != nil {
		t.Fatal(err)
	}
	writeText(t, secondRoot, "collision")
	if _, err := ReserveExclusiveInitBatch(first, second); err == nil {
		t.Fatal("batch reservation accepted second-root collision")
	}
	if _, err := os.Lstat(firstRoot); !os.IsNotExist(err) {
		t.Fatalf("failed batch reservation left first root: %v", err)
	}
	if got := string(readFile(t, secondRoot)); got != "collision" {
		t.Fatalf("collision changed: %q", got)
	}
}

func TestExclusiveInitBatchApplyPreflightsAllReservedRoots(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	workspace := filepath.Join(t.TempDir(), "batch-apply")
	if err := os.Mkdir(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	firstRoot := filepath.Join(workspace, "first")
	secondRoot := filepath.Join(workspace, "second")
	first, err := PlanExclusiveInit(repoRoot, firstRoot, pack, exclusiveInitOptionsForTest())
	if err != nil {
		t.Fatal(err)
	}
	secondOpt := exclusiveInitOptionsForTest()
	secondOpt.Role = "second"
	second, err := PlanExclusiveInit(repoRoot, secondRoot, pack, secondOpt)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := ReserveExclusiveInitBatch(first, second)
	if err != nil {
		t.Fatal(err)
	}
	defer batch.Rollback()
	writeText(t, filepath.Join(secondRoot, "collision.txt"), "collision")
	if _, err := batch.Apply(); err == nil {
		t.Fatal("batch apply accepted a post-reservation second-root collision")
	}
	entries, err := os.ReadDir(firstRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("batch apply populated first root before detecting second collision: %+v", entries)
	}
}

func TestReserveExclusiveInitBatchPinsParentBeforeCreatingMissingRoot(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	base := t.TempDir()
	parent := filepath.Join(base, "parent")
	originalParent := filepath.Join(base, "original-parent")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	caseRoot := filepath.Join(parent, "case")
	plan, err := PlanExclusiveInit(repoRoot, caseRoot, pack, exclusiveInitOptionsForTest())
	if err != nil {
		t.Fatal(err)
	}

	rebindSucceeded := false
	exclusiveInitAfterParentPinHook = func() error {
		exclusiveInitAfterParentPinHook = nil
		if err := os.Rename(parent, originalParent); err != nil {
			return fmt.Errorf("parent rebind failed closed: %w", err)
		}
		rebindSucceeded = true
		return os.Mkdir(parent, 0o755)
	}
	t.Cleanup(func() { exclusiveInitAfterParentPinHook = nil })
	batch, err := ReserveExclusiveInitBatch(plan)
	if !rebindSucceeded {
		if err == nil || !strings.Contains(err.Error(), "parent rebind failed closed") {
			t.Fatalf("Reserve error = %v, want fail-closed parent-rebind error", err)
		}
		if _, statErr := os.Lstat(caseRoot); !os.IsNotExist(statErr) {
			t.Fatalf("failed parent rebind created replacement case: %v", statErr)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	defer batch.Rollback()
	if _, err := batch.Apply(); err != nil {
		t.Fatal(err)
	}
	for _, write := range plan.Writes {
		original := filepath.Join(originalParent, "case", filepath.FromSlash(write.Path))
		if got := readFile(t, original); !bytes.Equal(got, write.Content) {
			t.Fatalf("pinned parent leaf has wrong bytes %s", write.Path)
		}
		replacement := filepath.Join(caseRoot, filepath.FromSlash(write.Path))
		if _, err := os.Lstat(replacement); !os.IsNotExist(err) {
			t.Fatalf("replacement parent received leaf %s: %v", write.Path, err)
		}
	}
}

func TestExclusiveInitBatchRollbackUsesPinnedParent(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	base := t.TempDir()
	parent := filepath.Join(base, "parent")
	originalParent := filepath.Join(base, "original-parent")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	caseRoot := filepath.Join(parent, "case")
	plan, err := PlanExclusiveInit(repoRoot, caseRoot, pack, exclusiveInitOptionsForTest())
	if err != nil {
		t.Fatal(err)
	}

	rebindSucceeded := false
	exclusiveInitAfterParentPinHook = func() error {
		exclusiveInitAfterParentPinHook = nil
		if err := os.Rename(parent, originalParent); err != nil {
			return fmt.Errorf("parent rebind failed closed: %w", err)
		}
		rebindSucceeded = true
		return os.Mkdir(parent, 0o755)
	}
	t.Cleanup(func() { exclusiveInitAfterParentPinHook = nil })
	batch, err := ReserveExclusiveInitBatch(plan)
	if !rebindSucceeded {
		if err == nil || !strings.Contains(err.Error(), "parent rebind failed closed") {
			t.Fatalf("Reserve error = %v, want fail-closed parent-rebind error", err)
		}
		if _, statErr := os.Lstat(caseRoot); !os.IsNotExist(statErr) {
			t.Fatalf("failed parent rebind created replacement case: %v", statErr)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(caseRoot, "replacement.txt"), "keep replacement\n")
	batch.Rollback()
	if _, err := os.Lstat(filepath.Join(originalParent, "case")); !os.IsNotExist(err) {
		t.Fatalf("rollback left original reserved root: %v", err)
	}
	if got := string(readFile(t, filepath.Join(caseRoot, "replacement.txt"))); got != "keep replacement\n" {
		t.Fatalf("rollback changed replacement root: %q", got)
	}
}

func TestExclusiveInitBatchPinsRootAcrossPathRebind(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	workspace := filepath.Join(t.TempDir(), "pinned-root")
	if err := os.Mkdir(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	caseRoot := filepath.Join(workspace, "case")
	originalRoot := filepath.Join(workspace, "original")
	plan, err := PlanExclusiveInit(repoRoot, caseRoot, pack, exclusiveInitOptionsForTest())
	if err != nil {
		t.Fatal(err)
	}
	batch, err := ReserveExclusiveInitBatch(plan)
	if err != nil {
		t.Fatal(err)
	}
	defer batch.Rollback()

	rebindSucceeded := false
	exclusiveInitAfterPreflightHook = func() error {
		if err := os.Rename(caseRoot, originalRoot); err != nil {
			return fmt.Errorf("root rebind failed closed: %w", err)
		}
		rebindSucceeded = true
		return os.Mkdir(caseRoot, 0o755)
	}
	t.Cleanup(func() { exclusiveInitAfterPreflightHook = nil })
	results, err := batch.Apply()
	if !rebindSucceeded {
		if err == nil || !strings.Contains(err.Error(), "root rebind failed closed") {
			t.Fatalf("Apply error = %v, want fail-closed root-rebind error", err)
		}
		entries, readErr := os.ReadDir(caseRoot)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if len(entries) != 0 {
			t.Fatalf("failed root rebind populated pinned root: %+v", entries)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), "reserved root identity changed") {
		t.Fatalf("Apply error = %v, want pinned-root identity rejection; results=%+v", err, results)
	}
	entries, readErr := os.ReadDir(originalRoot)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("rebound pinned root was populated: %+v", entries)
	}
	entries, readErr = os.ReadDir(caseRoot)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("replacement root was populated: %+v", entries)
	}
}

func TestExclusiveInitBatchApplyRetainsHandlesUntilCallerClose(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := filepath.Join(t.TempDir(), "handle-lifetime")
	plan, err := PlanExclusiveInit(repoRoot, caseRoot, pack, exclusiveInitOptionsForTest())
	if err != nil {
		t.Fatal(err)
	}
	batch, err := ReserveExclusiveInitBatch(plan)
	if err != nil {
		t.Fatal(err)
	}
	defer batch.Rollback()
	if _, err := batch.Apply(); err != nil {
		t.Fatal(err)
	}
	if err := batch.ValidateRoots(); err != nil {
		t.Fatalf("Apply closed pinned handles: %v", err)
	}
	if err := batch.Close(); err != nil {
		t.Fatal(err)
	}
	if err := batch.ValidateRoots(); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("ValidateRoots after Close error = %v", err)
	}
}

func TestExclusiveInitLeafWriteFailureNeverExposesFinalLeaf(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := filepath.Join(t.TempDir(), "leaf-write-failure")
	plan, err := PlanExclusiveInit(repoRoot, caseRoot, pack, exclusiveInitOptionsForTest())
	if err != nil {
		t.Fatal(err)
	}
	failedPath := plan.Writes[0].Path
	exclusiveInitLeafWriteHook = func(stage, path string) error {
		if stage == "before-temp-write" && path == failedPath {
			return fmt.Errorf("simulated write failure")
		}
		return nil
	}
	t.Cleanup(func() { exclusiveInitLeafWriteHook = nil })
	if _, err := ApplyExclusiveInit(plan); err == nil || !strings.Contains(err.Error(), "simulated write failure") {
		t.Fatalf("ApplyExclusiveInit error = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, filepath.FromSlash(failedPath))); !os.IsNotExist(err) {
		t.Fatalf("failed write exposed final leaf: %v", err)
	}
	temp := exclusiveInitTempLeafName(filepath.FromSlash(failedPath), plan.Writes[0])
	if _, err := os.Lstat(filepath.Join(caseRoot, temp)); !os.IsNotExist(err) {
		t.Fatalf("failed write left owned temp: %v", err)
	}
}

func TestExclusiveInitLeafResumesAfterCrashBeforePublish(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := filepath.Join(t.TempDir(), "leaf-crash-resume")
	plan, err := PlanExclusiveInit(repoRoot, caseRoot, pack, exclusiveInitOptionsForTest())
	if err != nil {
		t.Fatal(err)
	}
	crashedPath := plan.Writes[0].Path
	crashed := false
	exclusiveInitLeafWriteHook = func(stage, path string) error {
		if !crashed && stage == "before-publish" && path == crashedPath {
			crashed = true
			return fmt.Errorf("simulated publish crash")
		}
		return nil
	}
	t.Cleanup(func() { exclusiveInitLeafWriteHook = nil })
	if _, err := ApplyExclusiveInit(plan); err == nil || !strings.Contains(err.Error(), "simulated publish crash") {
		t.Fatalf("ApplyExclusiveInit error = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, filepath.FromSlash(crashedPath))); !os.IsNotExist(err) {
		t.Fatalf("publish crash exposed final leaf: %v", err)
	}
	temp := exclusiveInitTempLeafName(filepath.FromSlash(crashedPath), plan.Writes[0])
	if got := readFile(t, filepath.Join(caseRoot, temp)); !bytes.Equal(got, plan.Writes[0].Content) {
		t.Fatalf("owned temp bytes = %q", got)
	}
	exclusiveInitLeafWriteHook = nil
	result, err := ApplyExclusiveInit(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Replay {
		t.Fatalf("crash resume was not replay: %+v", result)
	}
	if got := readFile(t, filepath.Join(caseRoot, filepath.FromSlash(crashedPath))); !bytes.Equal(got, plan.Writes[0].Content) {
		t.Fatalf("resumed final bytes = %q", got)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, temp)); !os.IsNotExist(err) {
		t.Fatalf("resume left owned temp: %v", err)
	}
}

func TestExclusiveInitLeafResumesAfterCrashAfterPublishBeforeTempRemove(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := filepath.Join(t.TempDir(), "leaf-after-publish-crash-resume")
	plan, err := PlanExclusiveInit(repoRoot, caseRoot, pack, exclusiveInitOptionsForTest())
	if err != nil {
		t.Fatal(err)
	}
	crashedPath := plan.Writes[0].Path
	exclusiveInitLeafWriteHook = func(stage, path string) error {
		if stage == "after-publish-before-temp-remove" && path == crashedPath {
			return fmt.Errorf("simulated after-publish crash")
		}
		return nil
	}
	t.Cleanup(func() { exclusiveInitLeafWriteHook = nil })
	if _, err := ApplyExclusiveInit(plan); err == nil || !strings.Contains(err.Error(), "simulated after-publish crash") {
		t.Fatalf("ApplyExclusiveInit error = %v", err)
	}
	temp := exclusiveInitTempLeafName(filepath.FromSlash(crashedPath), plan.Writes[0])
	finalPath := filepath.Join(caseRoot, filepath.FromSlash(crashedPath))
	tempPath := filepath.Join(caseRoot, temp)
	tempInfo, tempErr := os.Lstat(tempPath)
	finalInfo, finalErr := os.Lstat(finalPath)
	if tempErr != nil || finalErr != nil || !os.SameFile(tempInfo, finalInfo) {
		t.Fatalf("published leaf and owned temp do not share identity: temp=%v final=%v", tempErr, finalErr)
	}
	exclusiveInitLeafWriteHook = nil
	result, err := ApplyExclusiveInit(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Replay {
		t.Fatalf("crash resume was not replay: %+v", result)
	}
	if _, err := os.Lstat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("completed replay left owned temp: %v", err)
	}
}

func TestExclusiveInitLeafCompletedReplayRejectsDifferentFinalIdentityWithoutDeletingTemp(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := filepath.Join(t.TempDir(), "leaf-after-publish-different-final")
	plan, err := PlanExclusiveInit(repoRoot, caseRoot, pack, exclusiveInitOptionsForTest())
	if err != nil {
		t.Fatal(err)
	}
	crashedPath := plan.Writes[0].Path
	exclusiveInitLeafWriteHook = func(stage, path string) error {
		if stage == "after-publish-before-temp-remove" && path == crashedPath {
			return fmt.Errorf("simulated after-publish crash")
		}
		return nil
	}
	t.Cleanup(func() { exclusiveInitLeafWriteHook = nil })
	if _, err := ApplyExclusiveInit(plan); err == nil {
		t.Fatal("ApplyExclusiveInit unexpectedly survived simulated crash")
	}
	temp := exclusiveInitTempLeafName(filepath.FromSlash(crashedPath), plan.Writes[0])
	finalPath := filepath.Join(caseRoot, filepath.FromSlash(crashedPath))
	tempPath := filepath.Join(caseRoot, temp)
	if err := os.Remove(finalPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(finalPath, plan.Writes[0].Content, 0o644); err != nil {
		t.Fatal(err)
	}
	exclusiveInitLeafWriteHook = nil
	if _, err := ApplyExclusiveInit(plan); err == nil || !strings.Contains(err.Error(), "different identities") {
		t.Fatalf("ApplyExclusiveInit error = %v", err)
	}
	if got := readFile(t, finalPath); !bytes.Equal(got, plan.Writes[0].Content) {
		t.Fatalf("different final was overwritten: %q", got)
	}
	if got := readFile(t, tempPath); !bytes.Equal(got, plan.Writes[0].Content) {
		t.Fatalf("owned temp was deleted or changed: %q", got)
	}
	tempInfo, _ := os.Lstat(tempPath)
	finalInfo, _ := os.Lstat(finalPath)
	if os.SameFile(tempInfo, finalInfo) {
		t.Fatal("test did not install a different final identity")
	}
}

func TestExclusiveInitLeafCompletedReplayRejectsOwnedTempBytesDrift(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := filepath.Join(t.TempDir(), "leaf-after-publish-temp-drift")
	plan, err := PlanExclusiveInit(repoRoot, caseRoot, pack, exclusiveInitOptionsForTest())
	if err != nil {
		t.Fatal(err)
	}
	crashedPath := plan.Writes[0].Path
	exclusiveInitLeafWriteHook = func(stage, path string) error {
		if stage == "after-publish-before-temp-remove" && path == crashedPath {
			return fmt.Errorf("simulated after-publish crash")
		}
		return nil
	}
	t.Cleanup(func() { exclusiveInitLeafWriteHook = nil })
	if _, err := ApplyExclusiveInit(plan); err == nil {
		t.Fatal("ApplyExclusiveInit unexpectedly survived simulated crash")
	}
	temp := exclusiveInitTempLeafName(filepath.FromSlash(crashedPath), plan.Writes[0])
	tempPath := filepath.Join(caseRoot, temp)
	if err := os.Remove(tempPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tempPath, []byte("drifted temp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	exclusiveInitLeafWriteHook = nil
	if _, err := ApplyExclusiveInit(plan); err == nil || !strings.Contains(err.Error(), "cannot reconcile owned temp") {
		t.Fatalf("ApplyExclusiveInit error = %v", err)
	}
	if got := readFile(t, tempPath); string(got) != "drifted temp\n" {
		t.Fatalf("drifted owned temp was deleted or changed: %q", got)
	}
}

func TestApplyExclusiveInitRejectsAncestorSymlinkBelowExistingDirectory(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	base := t.TempDir()
	realParent := filepath.Join(base, "real")
	existing := filepath.Join(realParent, "existing")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	linkParent := filepath.Join(base, "link")
	if err := os.Symlink(realParent, linkParent); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	caseRoot := filepath.Join(linkParent, "existing", "case")
	plan, err := PlanExclusiveInit(repoRoot, caseRoot, pack, exclusiveInitOptionsForTest())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyExclusiveInit(plan); err == nil || !strings.Contains(err.Error(), "root parent") {
		t.Fatalf("ApplyExclusiveInit error = %v, want ancestor-symlink rejection", err)
	}
	if _, err := os.Lstat(filepath.Join(existing, "case")); !os.IsNotExist(err) {
		t.Fatalf("ancestor symlink target was mutated: %v", err)
	}
}

func TestApplyExclusiveInitRejectsAncestorSymlink(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	base := t.TempDir()
	realParent := filepath.Join(base, "real")
	if err := os.Mkdir(realParent, 0o755); err != nil {
		t.Fatal(err)
	}
	linkParent := filepath.Join(base, "link")
	if err := os.Symlink(realParent, linkParent); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	caseRoot := filepath.Join(linkParent, "case")
	plan, err := PlanExclusiveInit(repoRoot, caseRoot, pack, exclusiveInitOptionsForTest())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyExclusiveInit(plan); err == nil || !strings.Contains(err.Error(), "root parent") {
		t.Fatalf("ApplyExclusiveInit error = %v, want ancestor-symlink rejection", err)
	}
	if _, err := os.Lstat(filepath.Join(realParent, "case")); !os.IsNotExist(err) {
		t.Fatalf("ancestor symlink target was mutated: %v", err)
	}
}

func TestApplyExclusiveInitRejectsDifferentBytes(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := filepath.Join(t.TempDir(), "different")
	plan, err := PlanExclusiveInit(repoRoot, caseRoot, pack, exclusiveInitOptionsForTest())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyExclusiveInit(plan); err != nil {
		t.Fatal(err)
	}
	path := plan.Writes[0].TargetPath
	if err := os.WriteFile(path, []byte("different\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = ApplyExclusiveInit(plan)
	if err == nil || !strings.Contains(err.Error(), "different bytes") {
		t.Fatalf("ApplyExclusiveInit error = %v, want different-bytes rejection", err)
	}
	if got := string(readFile(t, path)); got != "different\n" {
		t.Fatalf("different existing bytes were overwritten: %q", got)
	}
}

func TestExclusiveInitKeepsOrdinaryInitCompatible(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := filepath.Join(t.TempDir(), "ordinary")

	preview, err := InitPreview(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "ordinary-demo", CreateLocalFiles: true, Command: "init"})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Command != "init" || preview.ProjectName != "ordinary-demo" {
		t.Fatalf("unexpected ordinary init preview: %+v", preview)
	}
	result, err := Apply(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "ordinary-demo", CreateLocalFiles: true, Command: "init"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.Command != "init" {
		t.Fatalf("unexpected ordinary init result: %+v", result)
	}
	for _, rel := range []string{".rekit/instance.yml", ".claude/skills/rekit/SKILL.md", ".re-template.yml", ".rekit/state.json", "references/template/README.md", "references/template/task-handoff.md", "CLAUDE.local.md"} {
		if info, err := os.Stat(filepath.Join(caseRoot, filepath.FromSlash(rel))); err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			t.Fatalf("ordinary init missing doctor-required file %s: info=%v err=%v", rel, info, err)
		}
	}
}

func assertDoctorReadyCaseFiles(t *testing.T, plan ExclusiveInitPlan) {
	t.Helper()
	for _, write := range plan.Writes {
		info, err := os.Stat(write.TargetPath)
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("doctor-required planned file %s is not regular: info=%v err=%v", write.Path, info, err)
		}
		if write.Size > 0 && info.Size() == 0 {
			t.Fatalf("doctor-required planned file %s is empty", write.Path)
		}
	}
	block := string(readFile(t, filepath.Join(plan.CaseRoot, "CLAUDE.local.md")))
	if !strings.Contains(block, "<!-- BEGIN unit:router -->") || !strings.Contains(block, "<!-- END unit:router -->") {
		t.Fatalf("managed block host is not doctor-ready:\n%s", block)
	}
}

func exclusiveInitFixture(t *testing.T) (repoRoot, pack string) {
	t.Helper()
	fixtureRepo, _, fixturePack := syncFixture(t)
	repoRoot = filepath.Join(t.TempDir(), "repo")
	if err := copyTreeForExclusiveTest(fixtureRepo, repoRoot); err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(repoRoot, ".claude", "skills", "rekit", "SKILL.md"), "# canonical test skill\n")
	pack = fixturePack
	return repoRoot, pack
}

func copyTreeForExclusiveTest(sourceRoot, targetRoot string) error {
	return filepath.WalkDir(sourceRoot, func(source string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(sourceRoot, source)
		if err != nil {
			return err
		}
		target := filepath.Join(targetRoot, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
}

func exclusiveInitOptionsForTest() ExclusiveInitOptions {
	return ExclusiveInitOptions{
		ProjectName: "exclusive-demo",
		ProvisionID: "provision-test",
		Role:        "fresh-case",
		CreatedAt:   time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC),
	}
}

func readPlannedExclusiveContent(t *testing.T, plan ExclusiveInitPlan, path string) []byte {
	t.Helper()
	for _, write := range plan.Writes {
		if write.Path == path {
			return write.Content
		}
	}
	t.Fatalf("planned write not found: %s", path)
	return nil
}
