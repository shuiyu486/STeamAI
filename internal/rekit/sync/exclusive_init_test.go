package sync

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/doctor"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	"github.com/shuiyu486/re-context-kits/internal/rekit/plancontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
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
			Path:    ".steamai/caller-marker.txt",
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
		content := readPlannedExclusiveContent(t, plan, write.Path)
		if write.SHA256 == "" || write.Size != int64(len(content)) || sha256Bytes(content) != write.SHA256 {
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
	if got := string(readFile(t, filepath.Join(caseRoot, ".steamai", "verification-role.json"))); !strings.Contains(got, `"provisionId": "provision-559"`) || !strings.Contains(got, `"role": "fresh-case"`) || !strings.Contains(got, `"createdAt": "2026-07-23T04:34:56.000000789Z"`) {
		t.Fatalf("unexpected verification marker:\n%s", got)
	}
	if got := string(readFile(t, filepath.Join(caseRoot, ".steamai", "caller-marker.txt"))); got != "candidate verification\n" {
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
	if _, err := doctor.Case(filepath.Join(caseRoot, ".steamai"), caseRoot, "_template"); err != nil {
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
	writeText(t, write.TargetPath, string(readPlannedExclusiveContent(t, plan, write.Path)))

	result, err := ApplyExclusiveInit(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || !result.Replay {
		t.Fatalf("unexpected partial replay result: %+v", result)
	}
	if got := readFile(t, write.TargetPath); !bytes.Equal(got, readPlannedExclusiveContent(t, plan, write.Path)) {
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
		if got := readFile(t, original); !bytes.Equal(got, readPlannedExclusiveContent(t, plan, write.Path)) {
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

func TestOrdinaryInitRequiresExactHashAndPreservesExistingFiles(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := t.TempDir()
	sentinelPath := filepath.Join(caseRoot, "sentinel.bin")
	sentinel := []byte{0, 1, 2, 3, 255}
	if err := os.WriteFile(sentinelPath, sentinel, 0o600); err != nil {
		t.Fatal(err)
	}
	opt := ApplyOptions{ProjectName: "adopt-demo", CreateLocalFiles: true, Command: "init"}
	preview, err := InitPreview(repoRoot, caseRoot, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if preview.TargetClass != "ordinary-directory" || !preview.AdoptionReady || len(preview.ExpectedPlanSHA256) != 64 || len(preview.ApplyArgs) == 0 || preview.ApplyCommand == "" {
		t.Fatalf("ordinary adoption preview = %+v", preview)
	}
	commandAction, err := commands.ExactActionFromCommand(preview.ApplyCommand)
	if err != nil {
		t.Fatal(err)
	}
	argsAction, err := commands.ExactActionFromCLIArgs(preview.ApplyArgs)
	if err != nil || !commandAction.Equivalent(argsAction) || !strings.HasPrefix(preview.ApplyCommand, commands.CurrentPublicEntrypoint+" init ") {
		t.Fatalf("ordinary adoption exact Apply carrier drifted: command=%q args=%v err=%v", preview.ApplyCommand, preview.ApplyArgs, err)
	}
	if err := commandAction.ValidatePlanApply(commands.Init, preview.ExpectedPlanSHA256); err != nil {
		t.Fatalf("ordinary adoption exact Apply binding: %v", err)
	}
	_, err = Apply(repoRoot, caseRoot, pack, opt)
	failure, typed := plancontract.FromError(err)
	if err == nil || !typed || failure.Code != plancontract.CodePlanMissing || failure.MutationApplied || failure.MutationBoundary != "none" {
		t.Fatalf("ordinary adoption accepted unbound Apply: %v failure=%+v typed=%t", err, failure, typed)
	}
	opt.ExpectedPlanSHA256 = preview.ExpectedPlanSHA256
	result, err := Apply(repoRoot, caseRoot, pack, opt)
	if err != nil || !result.Applied {
		t.Fatalf("ordinary adoption Apply = %+v err=%v", result, err)
	}
	if got := readFile(t, sentinelPath); !bytes.Equal(got, sentinel) {
		t.Fatalf("ordinary adoption changed sentinel: %v", got)
	}
}

func TestOrdinaryInitPreservesSemanticEqualManagedFileBytes(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := t.TempDir()
	target := filepath.Join(caseRoot, "references", "template", "README.md")
	source := filepath.Join(repoRoot, "packs", pack, "references", "template", "README.md")
	sourceBytes := readFile(t, source)
	preserved := []byte(strings.ReplaceAll(string(sourceBytes), "\n", "\r\n"))
	if bytes.Equal(preserved, sourceBytes) {
		t.Fatal("fixture did not create a distinct target line-ending representation")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, preserved, 0o644); err != nil {
		t.Fatal(err)
	}

	opt := ApplyOptions{ProjectName: "semantic-equal", CreateLocalFiles: true, Command: "init"}
	preview, err := InitPreview(repoRoot, caseRoot, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.AdoptionReady {
		t.Fatalf("semantic-equal ordinary target was blocked: %+v", preview.AdoptionBlockers)
	}
	var write WriteResult
	for _, candidate := range preview.Writes {
		if candidate.Path == "references/template/README.md" {
			write = candidate
			break
		}
	}
	if write.Path == "" || write.Action != "unchanged" {
		t.Fatalf("semantic-equal managed file write = %+v, want unchanged", write)
	}
	preservedHash := sha256Bytes(preserved)
	if preview.initTargetSHA256[write.Path] != preservedHash {
		t.Fatalf("preview target hash = %s, want exact preserved hash %s", preview.initTargetSHA256[write.Path], preservedHash)
	}

	opt.ExpectedPlanSHA256 = preview.ExpectedPlanSHA256
	result, err := Apply(repoRoot, caseRoot, pack, opt)
	if err != nil || !result.Applied {
		t.Fatalf("semantic-equal ordinary Apply = %+v err=%v", result, err)
	}
	if got := readFile(t, target); !bytes.Equal(got, preserved) {
		t.Fatalf("semantic-equal target bytes changed: %q != %q", got, preserved)
	}
	var state syncState
	if err := json.Unmarshal(readFile(t, filepath.Join(caseRoot, ".steamai", "state.json")), &state); err != nil {
		t.Fatal(err)
	}
	entry := state.Managed[write.Path]
	if entry.SourceHash != preview.initSourceSHA256[write.Path] || entry.TargetHashAtSync != preservedHash {
		t.Fatalf("semantic-equal state did not bind source and exact target independently: %+v", entry)
	}
}

func TestOrdinaryInitBlocksManagedAndManagedBlockCollisions(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	for _, fixture := range []struct {
		name string
		path string
	}{
		{name: "managed file", path: "references/template/README.md"},
		{name: "managed block host", path: "CLAUDE.local.md"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			caseRoot := t.TempDir()
			writeText(t, filepath.Join(caseRoot, filepath.FromSlash(fixture.path)), "user content\n")
			preview, err := InitPreview(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "blocked", CreateLocalFiles: true, Command: "init"})
			if err != nil {
				t.Fatal(err)
			}
			if preview.AdoptionReady || len(preview.AdoptionBlockers) == 0 {
				t.Fatalf("collision preview did not block: %+v", preview)
			}
			before := readFile(t, filepath.Join(caseRoot, filepath.FromSlash(fixture.path)))
			_, err = Apply(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "blocked", CreateLocalFiles: true, Command: "init", ExpectedPlanSHA256: preview.ExpectedPlanSHA256})
			if err == nil || !strings.Contains(err.Error(), "adoption is blocked") {
				t.Fatalf("collision Apply error = %v", err)
			}
			if after := readFile(t, filepath.Join(caseRoot, filepath.FromSlash(fixture.path))); !bytes.Equal(after, before) {
				t.Fatalf("blocked collision changed bytes: %q", after)
			}
			if _, statErr := os.Lstat(filepath.Join(caseRoot, ".steamai")); !os.IsNotExist(statErr) {
				t.Fatalf("blocked collision wrote .rekit: %v", statErr)
			}
		})
	}
}

func TestOrdinaryInitRejectsPlanDriftBeforeFirstWrite(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := t.TempDir()
	preview, err := InitPreview(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "drift", CreateLocalFiles: true, Command: "init"})
	if err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(caseRoot, "references", "template", "README.md"), "collision\n")
	_, err = Apply(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "drift", CreateLocalFiles: true, Command: "init", ExpectedPlanSHA256: preview.ExpectedPlanSHA256})
	if err == nil || !strings.Contains(err.Error(), "plan changed after preview") {
		t.Fatalf("drift Apply error = %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(caseRoot, ".steamai")); !os.IsNotExist(statErr) {
		t.Fatalf("drift Apply wrote before rejection: %v", statErr)
	}
}

func TestOrdinaryInitPlanHashIsStableAcrossClockTicks(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := t.TempDir()
	first, err := InitPreview(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "stable", CreateLocalFiles: true, Command: "init"})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	second, err := InitPreview(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "stable", CreateLocalFiles: true, Command: "init"})
	if err != nil {
		t.Fatal(err)
	}
	if first.ExpectedPlanSHA256 != second.ExpectedPlanSHA256 {
		t.Fatalf("stable preview hash changed across clock tick: %s != %s", first.ExpectedPlanSHA256, second.ExpectedPlanSHA256)
	}
}

func TestOrdinaryInitPlanAndWritesIgnoreSourceLineEndingRepresentation(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := t.TempDir()
	opt := ApplyOptions{ProjectName: "line-endings", CreateLocalFiles: true, Command: "init"}

	lfPlan, err := InitPreview(repoRoot, caseRoot, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	lfPublications, err := ordinaryInitPublications(lfPlan)
	if err != nil {
		t.Fatal(err)
	}
	convertTreeTextLineEndings(t, repoRoot, "\r\n")
	crlfPlan, err := InitPreview(repoRoot, caseRoot, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	crlfPublications, err := ordinaryInitPublications(crlfPlan)
	if err != nil {
		t.Fatal(err)
	}

	if lfPlan.ExpectedPlanSHA256 == crlfPlan.ExpectedPlanSHA256 {
		t.Fatal("bundle byte representation drift did not alter the exact init plan hash")
	}
	if len(lfPublications) != len(crlfPublications) {
		t.Fatalf("publication count changed: %d != %d", len(lfPublications), len(crlfPublications))
	}
	for index := range lfPublications {
		left, right := lfPublications[index], crlfPublications[index]
		if left.write.Path != right.write.Path {
			t.Fatalf("publication path changed: %s != %s", left.write.Path, right.write.Path)
		}
		if left.write.Kind == "sync-state" {
			var leftState, rightState syncState
			if err := json.Unmarshal(left.data, &leftState); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(right.data, &rightState); err != nil {
				t.Fatal(err)
			}
			leftState.LastSyncAt, rightState.LastSyncAt = "", ""
			left.data, _ = json.Marshal(leftState)
			right.data, _ = json.Marshal(rightState)
		}
		if !bytes.Equal(left.data, right.data) {
			if isBundlePublicationKind(left.write.Kind) || left.write.Kind == "instance-metadata" {
				continue
			}
			t.Fatalf("canonical publication changed at %s: %q != %q", left.write.Path, left.data, right.data)
		}
	}
}

func TestExclusiveInitWritesIgnoreSourceLineEndingRepresentation(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	options := exclusiveInitOptionsForTest()
	caseRoot := filepath.Join(t.TempDir(), "canonical-case")

	lfPlan, err := PlanExclusiveInit(repoRoot, caseRoot, pack, options)
	if err != nil {
		t.Fatal(err)
	}
	convertTreeTextLineEndings(t, repoRoot, "\r\n")
	crlfPlan, err := PlanExclusiveInit(repoRoot, caseRoot, pack, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(lfPlan.Writes) != len(crlfPlan.Writes) {
		t.Fatalf("exclusive write count changed: %d != %d", len(lfPlan.Writes), len(crlfPlan.Writes))
	}
	for index := range lfPlan.Writes {
		left, right := lfPlan.Writes[index], crlfPlan.Writes[index]
		if left.Path != right.Path {
			t.Fatalf("exclusive write path changed: %s != %s", left.Path, right.Path)
		}
		if left.Kind == "runtime-executable" {
			if left.SHA256 != right.SHA256 || left.Size != right.Size {
				t.Fatalf("runtime executable binding changed: left=%+v right=%+v", left, right)
			}
			continue
		}
		leftData := readPlannedExclusiveContent(t, lfPlan, left.Path)
		rightData := readPlannedExclusiveContent(t, crlfPlan, right.Path)
		if isBundlePublicationKind(left.Kind) || left.Kind == "instance-metadata" {
			if bytes.Equal(leftData, rightData) {
				continue
			}
			// Bundle assets intentionally bind exact delivery bytes; changing their
			// representation must also change the canonical manifest binding.
			if left.SHA256 == right.SHA256 {
				t.Fatalf("bundle byte drift did not update binding at %s", left.Path)
			}
			continue
		}
		if left.SHA256 != right.SHA256 || left.Size != right.Size || !bytes.Equal(leftData, rightData) {
			t.Fatalf("canonical exclusive write changed at %s: left=%+v right=%+v", left.Path, left, right)
		}
	}
}

func TestAttachedInitUsesRawSourceText(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := filepath.Join(t.TempDir(), "attached-case")
	if err := os.MkdirAll(caseRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := casebind.WriteInstance(caseRoot, repoRoot, pack, "attached-raw"); err != nil {
		t.Fatal(err)
	}
	convertTreeTextLineEndings(t, repoRoot, "\n")
	opt := ApplyOptions{ProjectName: "attached-raw", CreateLocalFiles: true, Command: "init"}
	preview, err := InitPreview(repoRoot, caseRoot, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if preview.TargetClass != "attached-case" {
		t.Fatalf("attached init target class = %q", preview.TargetClass)
	}
	opt.ExpectedPlanSHA256 = preview.ExpectedPlanSHA256
	if _, err := Apply(repoRoot, caseRoot, pack, opt); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		".claude/skills/rekit/SKILL.md",
		"references/template/README.md",
		"references/template/task-handoff.md",
	} {
		data := readFile(t, filepath.Join(caseRoot, filepath.FromSlash(rel)))
		if bytes.Contains(data, []byte("\r\n")) {
			t.Fatalf("attached init canonicalized raw source at %s: %q", rel, data)
		}
	}
}

func TestMissionInitUsesRawSourceText(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := filepath.Join(t.TempDir(), "mission-case")
	if err := os.MkdirAll(caseRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := casebind.WriteInstance(caseRoot, repoRoot, pack, "mission-raw"); err != nil {
		t.Fatal(err)
	}
	identity := missionintent.Identity{
		SchemaVersion: 1, Target: caseRoot, Pack: pack, ProjectName: "mission-raw",
		Goal: "verify raw source text", Actor: "mission-commander", Executor: "claude-code", InitialLane: "main",
	}
	missionBytes, err := missionintent.MarshalMissionIntent(identity)
	if err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(caseRoot, filepath.FromSlash(missionintent.MissionIntentRel)), string(missionBytes))
	convertTreeTextLineEndings(t, repoRoot, "\n")
	opt := ApplyOptions{ProjectName: "mission-raw", CreateLocalFiles: true, Command: "init"}
	preview, err := InitPreview(repoRoot, caseRoot, pack, opt)
	if err == nil || !strings.Contains(err.Error(), "onboarding intent is missing") {
		t.Fatalf("partial mission fixture did not fail closed: preview=%+v err=%v", preview, err)
	}

	intent := missionintent.Intent{
		SchemaVersion: 1, Kind: "mission-onboarding-intent", PublicationStamp: "20260812-010203004",
		OnboardingPlanSHA256: strings.Repeat("a", 64), Identity: identity,
		Recovery: missionintent.RecoveryEnvelope{
			SchemaVersion: 1, RepoRoot: repoRoot, CreatedAt: "2026-08-12T01:02:03Z", Mode: "attached-adoption",
			AttachedSnapshot: missionInitAttachedSnapshot(t, caseRoot),
		},
	}
	intentBytes, err := missionintent.MarshalIntent(intent)
	if err != nil {
		t.Fatal(err)
	}
	commitBytes, err := missionintent.MarshalCommit(missionintent.Commit{
		SchemaVersion: 1, Kind: "mission-onboarding-commit", PublicationStamp: intent.PublicationStamp,
		OnboardingPlanSHA256: intent.OnboardingPlanSHA256, MissionIntentSHA256: missionintent.SHA256(missionBytes), IntentSHA256: missionintent.SHA256(intentBytes),
	})
	if err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(caseRoot, filepath.FromSlash(missionintent.IntentRel)), string(intentBytes))
	writeText(t, filepath.Join(caseRoot, filepath.FromSlash(missionintent.CommitRel)), string(commitBytes))
	preview, err = InitPreview(repoRoot, caseRoot, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if preview.TargetClass != "mission-case" {
		t.Fatalf("mission init target class = %q", preview.TargetClass)
	}
	opt.ExpectedPlanSHA256 = preview.ExpectedPlanSHA256
	if _, err := Apply(repoRoot, caseRoot, pack, opt); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		".claude/skills/rekit/SKILL.md",
		"references/template/README.md",
		"references/template/task-handoff.md",
	} {
		data := readFile(t, filepath.Join(caseRoot, filepath.FromSlash(rel)))
		if bytes.Contains(data, []byte("\r\n")) {
			t.Fatalf("mission init canonicalized raw source at %s: %q", rel, data)
		}
	}
}

func missionInitAttachedSnapshot(t *testing.T, caseRoot string) []missionintent.SnapshotArtifact {
	t.Helper()
	artifacts := []missionintent.SnapshotArtifact{}
	for _, fixture := range []struct {
		path string
		kind string
	}{
		{path: ".claude/skills/rekit/SKILL.md", kind: "case-local-thin-shim"},
		{path: ".re-template.yml", kind: "legacy-metadata"},
		{path: ".rekit/instance.yml", kind: "instance-metadata"},
		{path: ".rekit/state.json", kind: "sync-state"},
	} {
		path := filepath.Join(caseRoot, filepath.FromSlash(fixture.path))
		if fixture.path != ".rekit/instance.yml" {
			writeText(t, path, "fixture\n")
		}
		data := readFile(t, path)
		artifacts = append(artifacts, missionintent.SnapshotArtifact{Path: fixture.path, Kind: fixture.kind, SHA256: missionintent.SHA256(data), Size: int64(len(data))})
	}
	return artifacts
}

func TestOrdinaryInitRejectsProjectSkillProvenanceDrift(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := t.TempDir()
	first, err := InitPreview(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "source-drift", CreateLocalFiles: true, Command: "init"})
	if err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(repoRoot, "rekit", "templates", "steamai-project", "SKILL.md")
	writeText(t, shim, "# changed project-local STeamAI skill\n")
	_, err = InitPreview(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "source-drift", CreateLocalFiles: true, Command: "init"})
	if err == nil || !strings.Contains(err.Error(), "skill provenance") {
		t.Fatalf("shim provenance drift error = %v; first=%s", err, first.ExpectedPlanSHA256)
	}
}

func TestOrdinaryInitSyncStateUsesFreshPlanSourceHashes(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := t.TempDir()
	preview, err := InitPreview(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "fresh-state", CreateLocalFiles: true, Command: "init"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Apply(repoRoot, caseRoot, pack, ApplyOptions{
		ProjectName: "fresh-state", CreateLocalFiles: true, Command: "init",
		ExpectedPlanSHA256: preview.ExpectedPlanSHA256,
	})
	if err != nil || !result.Applied {
		t.Fatalf("ordinary init fresh state Apply = %+v err=%v", result, err)
	}
	var state syncState
	if err := json.Unmarshal(readFile(t, filepath.Join(caseRoot, ".steamai", "state.json")), &state); err != nil {
		t.Fatal(err)
	}
	for _, write := range preview.Writes {
		if write.Kind != "managed-file" {
			continue
		}
		entry, ok := state.Managed[write.Path]
		if !ok || !strings.EqualFold(entry.SourceHash, preview.initSourceSHA256[write.Path]) || !strings.EqualFold(entry.TargetHashAtSync, preview.initSourceSHA256[write.Path]) {
			t.Fatalf("ordinary init state hash drift for %s: entry=%+v expected=%s", write.Path, entry, preview.initSourceSHA256[write.Path])
		}
	}
}

func TestOrdinaryInitManagedBlockUsesManifestBlockID(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := t.TempDir()
	preview, err := InitPreview(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "managed-block-id", CreateLocalFiles: true, Command: "init"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Apply(repoRoot, caseRoot, pack, ApplyOptions{
		ProjectName: "managed-block-id", CreateLocalFiles: true, Command: "init",
		ExpectedPlanSHA256: preview.ExpectedPlanSHA256,
	})
	if err != nil || !result.Applied {
		t.Fatalf("ordinary init with manifest block id failed: result=%+v err=%v", result, err)
	}
	block := string(readFile(t, filepath.Join(caseRoot, "CLAUDE.local.md")))
	if !strings.Contains(block, "<!-- BEGIN unit:router -->") || !strings.Contains(block, "<!-- END unit:router -->") {
		t.Fatalf("ordinary init used the wrong managed block id:\n%s", block)
	}
}

func TestOrdinaryInitCreateOnlyPublicationRejectsLateCollision(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := t.TempDir()
	preview, err := InitPreview(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "late-collision", CreateLocalFiles: true, Command: "init"})
	if err != nil {
		t.Fatal(err)
	}
	previous := ordinaryInitAfterPlanHook
	defer func() { ordinaryInitAfterPlanHook = previous }()
	collision := filepath.Join(caseRoot, "references", "template", "README.md")
	ordinaryInitAfterPlanHook = func(InitPlan) error {
		writeText(t, collision, "user collision\n")
		return nil
	}
	_, err = Apply(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "late-collision", CreateLocalFiles: true, Command: "init", ExpectedPlanSHA256: preview.ExpectedPlanSHA256})
	if err == nil || !strings.Contains(err.Error(), "plan changed before ordinary-directory publication") {
		t.Fatalf("late collision error = %v", err)
	}
	if got := string(readFile(t, collision)); got != "user collision\n" {
		t.Fatalf("late collision bytes changed: %q", got)
	}
	if _, statErr := os.Lstat(filepath.Join(caseRoot, ".steamai")); !os.IsNotExist(statErr) {
		t.Fatalf("late collision wrote managed state: %v", statErr)
	}
}

func TestOrdinaryInitRollsBackPublishedPrefixAfterLateFailure(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := t.TempDir()
	userDirectory := filepath.Join(caseRoot, ".claude")
	if err := os.Mkdir(userDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	preview, err := InitPreview(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "rollback-prefix", CreateLocalFiles: true, Command: "init"})
	if err != nil {
		t.Fatal(err)
	}
	previous := ordinaryInitAfterPublicationHook
	defer func() { ordinaryInitAfterPublicationHook = previous }()
	ordinaryInitAfterPublicationHook = func(count int, _ InitPlan) error {
		if count == 2 {
			return fmt.Errorf("deterministic publication failure")
		}
		return nil
	}
	_, err = Apply(repoRoot, caseRoot, pack, ApplyOptions{
		ProjectName: "rollback-prefix", CreateLocalFiles: true, Command: "init",
		ExpectedPlanSHA256: preview.ExpectedPlanSHA256,
	})
	if err == nil || !strings.Contains(err.Error(), "deterministic publication failure") {
		t.Fatalf("ordinary init late failure = %v", err)
	}
	for _, write := range preview.Writes[:2] {
		if _, statErr := os.Lstat(write.TargetPath); !os.IsNotExist(statErr) {
			t.Fatalf("ordinary init rollback left published prefix %s: %v; Apply error: %v", write.Path, statErr, err)
		}
	}
	if _, statErr := os.Lstat(filepath.Join(caseRoot, ".steamai")); !os.IsNotExist(statErr) {
		t.Fatalf("ordinary init rollback left created .rekit directory: %v; Apply error: %v", statErr, err)
	}
	if _, statErr := os.Lstat(filepath.Join(userDirectory, "skills")); !os.IsNotExist(statErr) {
		t.Fatalf("ordinary init rollback left created .claude/skills directory: %v", statErr)
	}
	if info, statErr := os.Lstat(userDirectory); statErr != nil || !info.IsDir() {
		t.Fatalf("ordinary init rollback removed user directory: info=%v err=%v", info, statErr)
	}
}

func TestOrdinaryInitRollbackPreservesExternallyChangedPublication(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := t.TempDir()
	preview, err := InitPreview(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "rollback-changed", CreateLocalFiles: true, Command: "init"})
	if err != nil {
		t.Fatal(err)
	}
	changed := []byte("external replacement\n")
	previous := ordinaryInitAfterPublicationHook
	defer func() { ordinaryInitAfterPublicationHook = previous }()
	ordinaryInitAfterPublicationHook = func(count int, plan InitPlan) error {
		if count != 1 {
			return nil
		}
		if err := os.WriteFile(plan.Writes[0].TargetPath, changed, 0o644); err != nil {
			return err
		}
		return fmt.Errorf("deterministic changed-publication failure")
	}
	_, err = Apply(repoRoot, caseRoot, pack, ApplyOptions{
		ProjectName: "rollback-changed", CreateLocalFiles: true, Command: "init",
		ExpectedPlanSHA256: preview.ExpectedPlanSHA256,
	})
	if err == nil || !strings.Contains(err.Error(), "deterministic changed-publication failure") || !strings.Contains(err.Error(), "rollback preserved changed publication") {
		t.Fatalf("ordinary init changed-publication rollback error = %v", err)
	}
	if got := readFile(t, preview.Writes[0].TargetPath); !bytes.Equal(got, changed) {
		t.Fatalf("ordinary init rollback changed external bytes: %q", got)
	}
}

func TestOrdinaryInitFinalValidationRejectsLatePreservedTargetDrift(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := t.TempDir()
	preserved := filepath.Join(caseRoot, "references", "template", "task-handoff.md")
	writeText(t, preserved, "user original\n")
	preview, err := InitPreview(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "preserved-drift", CreateLocalFiles: true, Command: "init"})
	if err != nil {
		t.Fatal(err)
	}
	previous := ordinaryInitBeforeFinalValidationHook
	defer func() { ordinaryInitBeforeFinalValidationHook = previous }()
	ordinaryInitBeforeFinalValidationHook = func(InitPlan) error {
		return os.WriteFile(preserved, []byte("user changed during publication\n"), 0o644)
	}
	_, err = Apply(repoRoot, caseRoot, pack, ApplyOptions{
		ProjectName: "preserved-drift", CreateLocalFiles: true, Command: "init",
		ExpectedPlanSHA256: preview.ExpectedPlanSHA256,
	})
	if err == nil || !strings.Contains(err.Error(), "preserved target changed after preview") {
		t.Fatalf("ordinary init preserved late drift error = %v", err)
	}
	if got := string(readFile(t, preserved)); got != "user changed during publication\n" {
		t.Fatalf("ordinary init changed late preserved bytes: %q", got)
	}
	if _, statErr := os.Lstat(filepath.Join(caseRoot, ".steamai")); !os.IsNotExist(statErr) {
		t.Fatalf("ordinary init preserved late drift left created state: %v; Apply error: %v", statErr, err)
	}
}

func TestOrdinaryInitFinalValidationRejectsLateManifestDrift(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := t.TempDir()
	preview, err := InitPreview(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "manifest-drift", CreateLocalFiles: true, Command: "init"})
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(repoRoot, "packs", pack, "manifest.yml")
	previous := ordinaryInitBeforeFinalValidationHook
	defer func() { ordinaryInitBeforeFinalValidationHook = previous }()
	ordinaryInitBeforeFinalValidationHook = func(InitPlan) error {
		file, err := os.OpenFile(manifestPath, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_, writeErr := file.WriteString("\n# changed during ordinary init\n")
		return errors.Join(writeErr, file.Close())
	}
	_, err = Apply(repoRoot, caseRoot, pack, ApplyOptions{
		ProjectName: "manifest-drift", CreateLocalFiles: true, Command: "init",
		ExpectedPlanSHA256: preview.ExpectedPlanSHA256,
	})
	if err == nil || !strings.Contains(err.Error(), "manifest changed during publication") {
		t.Fatalf("ordinary init late manifest drift error = %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(caseRoot, ".steamai")); !os.IsNotExist(statErr) {
		t.Fatalf("ordinary init late manifest drift left created state: %v; Apply error: %v", statErr, err)
	}
}

func TestOrdinaryInitTracksCreatedDirectoryWhenRollbackHandleOpenFails(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := t.TempDir()
	preview, err := InitPreview(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "directory-handle-failure", CreateLocalFiles: true, Command: "init"})
	if err != nil {
		t.Fatal(err)
	}
	previous := ordinaryInitOpenRollbackHandleForApply
	defer func() { ordinaryInitOpenRollbackHandleForApply = previous }()
	failed := false
	ordinaryInitOpenRollbackHandleForApply = func(path string, directory bool) (*os.File, error) {
		if directory && !failed {
			failed = true
			return nil, fmt.Errorf("directory rollback handle fixture")
		}
		return ordinaryInitOpenRollbackHandle(path, directory)
	}
	_, err = Apply(repoRoot, caseRoot, pack, ApplyOptions{
		ProjectName: "directory-handle-failure", CreateLocalFiles: true, Command: "init",
		ExpectedPlanSHA256: preview.ExpectedPlanSHA256,
	})
	if err == nil || !strings.Contains(err.Error(), "directory rollback handle fixture") || !strings.Contains(err.Error(), "rollback directory handle is missing") {
		t.Fatalf("ordinary init directory handle failure = %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(caseRoot, ".steamai")); statErr != nil {
		t.Fatalf("tracked residue must remain visible when exact rollback handle is unavailable: %v", statErr)
	}
}

func TestOrdinaryInitRollbackCapabilityFailureWritesNothing(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := t.TempDir()
	preview, err := InitPreview(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "unsupported-rollback", CreateLocalFiles: true, Command: "init"})
	if err != nil {
		t.Fatal(err)
	}
	previous := ordinaryInitRollbackCapability
	defer func() { ordinaryInitRollbackCapability = previous }()
	ordinaryInitRollbackCapability = func() error { return fmt.Errorf("rollback unsupported fixture") }
	_, err = Apply(repoRoot, caseRoot, pack, ApplyOptions{
		ProjectName: "unsupported-rollback", CreateLocalFiles: true, Command: "init",
		ExpectedPlanSHA256: preview.ExpectedPlanSHA256,
	})
	if err == nil || !strings.Contains(err.Error(), "rollback unsupported fixture") {
		t.Fatalf("ordinary init rollback capability error = %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(caseRoot, ".steamai")); !os.IsNotExist(statErr) {
		t.Fatalf("ordinary init rollback capability failure wrote state: %v", statErr)
	}
}

func TestOrdinaryInitFinalValidationRejectsLateSourceDrift(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := t.TempDir()
	preview, err := InitPreview(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "late-source", CreateLocalFiles: true, Command: "init"})
	if err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(repoRoot, "rekit", "templates", "steamai-project", "SKILL.md")
	previous := ordinaryInitBeforeFinalValidationHook
	defer func() { ordinaryInitBeforeFinalValidationHook = previous }()
	ordinaryInitBeforeFinalValidationHook = func(InitPlan) error {
		return os.WriteFile(shim, []byte("# late source replacement\n"), 0o644)
	}
	_, err = Apply(repoRoot, caseRoot, pack, ApplyOptions{
		ProjectName: "late-source", CreateLocalFiles: true, Command: "init",
		ExpectedPlanSHA256: preview.ExpectedPlanSHA256,
	})
	if err == nil || !strings.Contains(err.Error(), "skill provenance changed during publication") {
		t.Fatalf("ordinary init late source drift error = %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(caseRoot, ".steamai")); !os.IsNotExist(statErr) {
		t.Fatalf("ordinary init late source drift left created state: %v", statErr)
	}
}

func TestOrdinaryInitFinalValidationRejectsLateCanonicalSkillDrift(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := t.TempDir()
	preview, err := InitPreview(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "late-canonical-skill", CreateLocalFiles: true, Command: "init"})
	if err != nil {
		t.Fatal(err)
	}
	canonical := filepath.Join(repoRoot, ".claude", "skills", "steamai", "SKILL.md")
	previous := ordinaryInitBeforeFinalValidationHook
	defer func() { ordinaryInitBeforeFinalValidationHook = previous }()
	ordinaryInitBeforeFinalValidationHook = func(InitPlan) error {
		return os.WriteFile(canonical, []byte("# late canonical replacement\n"), 0o644)
	}
	_, err = Apply(repoRoot, caseRoot, pack, ApplyOptions{
		ProjectName: "late-canonical-skill", CreateLocalFiles: true, Command: "init",
		ExpectedPlanSHA256: preview.ExpectedPlanSHA256,
	})
	if err == nil || !strings.Contains(err.Error(), "skill provenance changed during publication") {
		t.Fatalf("ordinary init late canonical skill drift error = %v", err)
	}
	for _, path := range []string{
		filepath.Join(caseRoot, ".steamai"),
		filepath.Join(caseRoot, ".claude", "skills", "steamai", "SKILL.md"),
	} {
		if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
			t.Fatalf("ordinary init late canonical skill drift left publication %s: %v", path, statErr)
		}
	}
}

type ordinaryInitUnlockErrorLease struct {
	mutationLease
}

func (lease ordinaryInitUnlockErrorLease) Unlock() error {
	return errors.Join(lease.mutationLease.Unlock(), errors.New("ordinary init unlock fixture"))
}

func TestOrdinaryInitReportsCommittedCleanupWarningWithoutApplyError(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := t.TempDir()
	preview, err := InitPreview(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "cleanup-warning", CreateLocalFiles: true, Command: "init"})
	if err != nil {
		t.Fatal(err)
	}
	previous := ordinaryInitLeaseForTest
	defer func() { ordinaryInitLeaseForTest = previous }()
	ordinaryInitLeaseForTest = func(lease mutationLease) mutationLease {
		return ordinaryInitUnlockErrorLease{mutationLease: lease}
	}
	result, err := Apply(repoRoot, caseRoot, pack, ApplyOptions{
		ProjectName: "cleanup-warning", CreateLocalFiles: true, Command: "init",
		ExpectedPlanSHA256: preview.ExpectedPlanSHA256,
	})
	if err != nil || !result.Applied {
		t.Fatalf("ordinary init cleanup warning lost committed result: result=%+v err=%v", result, err)
	}
	if !strings.Contains(strings.Join(result.NextSteps, "\n"), "do not retry the original plan") || !strings.Contains(strings.Join(result.NextSteps, "\n"), "ordinary init unlock fixture") {
		t.Fatalf("ordinary init cleanup warning missing from committed result: %+v", result.NextSteps)
	}
}

func TestOrdinaryInitRollbackPreservesCanonicalReplacementAfterIdentityCheck(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := t.TempDir()
	preview, err := InitPreview(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "rollback-rebound", CreateLocalFiles: true, Command: "init"})
	if err != nil {
		t.Fatal(err)
	}
	previousPublication := ordinaryInitAfterPublicationHook
	previousIdentity := ordinaryInitRollbackAfterIdentityHook
	defer func() {
		ordinaryInitAfterPublicationHook = previousPublication
		ordinaryInitRollbackAfterIdentityHook = previousIdentity
	}()
	ordinaryInitAfterPublicationHook = func(count int, _ InitPlan) error {
		if count == 2 {
			return fmt.Errorf("deterministic rebound rollback")
		}
		return nil
	}
	replacement := []byte("external canonical replacement\n")
	replaced := false
	ordinaryInitRollbackAfterIdentityHook = func(rel string) error {
		if replaced || !strings.EqualFold(filepath.Clean(rel), filepath.Clean(preview.Writes[1].Path)) {
			return nil
		}
		target := preview.Writes[1].TargetPath
		moved := target + ".owned"
		if err := os.Rename(target, moved); err != nil {
			return err
		}
		if err := os.WriteFile(target, replacement, 0o644); err != nil {
			return err
		}
		replaced = true
		return nil
	}
	_, err = Apply(repoRoot, caseRoot, pack, ApplyOptions{
		ProjectName: "rollback-rebound", CreateLocalFiles: true, Command: "init",
		ExpectedPlanSHA256: preview.ExpectedPlanSHA256,
	})
	if err == nil || !strings.Contains(err.Error(), "deterministic rebound rollback") || !strings.Contains(err.Error(), "publication name rebound") {
		t.Fatalf("ordinary init rebound rollback error = %v", err)
	}
	if got := readFile(t, preview.Writes[1].TargetPath); !bytes.Equal(got, replacement) {
		t.Fatalf("ordinary init rollback removed canonical replacement: %q", got)
	}
	if _, statErr := os.Lstat(preview.Writes[1].TargetPath + ".owned"); !os.IsNotExist(statErr) {
		t.Fatalf("ordinary init rollback left exact owned object: %v", statErr)
	}
}

func TestExclusiveInitKeepsOrdinaryInitCompatible(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := filepath.Join(t.TempDir(), "ordinary")

	preview, err := InitPreview(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "ordinary-demo", CreateLocalFiles: true, Command: "init"})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Command != "init" || preview.ProjectName != "ordinary-demo" || preview.TargetClass != "missing" || len(preview.ExpectedPlanSHA256) != 64 {
		t.Fatalf("unexpected ordinary init preview: %+v", preview)
	}
	result, err := Apply(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "ordinary-demo", CreateLocalFiles: true, Command: "init", ExpectedPlanSHA256: preview.ExpectedPlanSHA256})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.Command != "init" {
		t.Fatalf("unexpected ordinary init result: %+v", result)
	}
	for _, rel := range []string{".steamai/instance.yml", ".claude/skills/steamai/SKILL.md", ".steamai/state.json", "references/template/README.md", "references/template/task-handoff.md", "CLAUDE.local.md"} {
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
	for _, rel := range []string{
		".claude/skills/steamai/SKILL.md",
		"rekit/templates/steamai-project/SKILL.md",
		"rekit/schemas/instance.schema.yml",
		"rekit/schemas/pack-manifest.schema.yml",
		"rekit/tests/catalog.json",
		"common/policies/manifest.yml",
		"common/policies/README.md",
	} {
		source := filepath.Join(exclusiveInitKitRoot(t), filepath.FromSlash(rel))
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		writeText(t, filepath.Join(repoRoot, filepath.FromSlash(rel)), string(data))
	}
	executable := filepath.Join(t.TempDir(), runtimebundle.ExecutableName())
	if err := os.WriteFile(executable, []byte("exclusive init test executable"), 0o700); err != nil {
		t.Fatal(err)
	}
	restore := runtimebundle.SetExecutableSourceForTest(executable)
	t.Cleanup(restore)
	pack = fixturePack
	return repoRoot, pack
}

func exclusiveInitKitRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve kit root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func convertTreeTextLineEndings(t *testing.T, root, lineEnding string) {
	t.Helper()
	for _, rel := range []string{
		"rekit/templates/case-shim/SKILL.md",
		"rekit/templates/steamai-project/SKILL.md",
		"packs/unit-pack/manifest.yml",
		"packs/unit-pack/references/template/README.md",
		"packs/unit-pack/references/template/task-handoff.template.md",
		"packs/unit-pack/CLAUDE.local.snippet.md",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		data := readFile(t, path)
		semantic := strings.ReplaceAll(strings.ReplaceAll(string(data), "\r\n", "\n"), "\r", "\n")
		if err := os.WriteFile(path, []byte(strings.ReplaceAll(semantic, "\n", lineEnding)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
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

func isBundlePublicationKind(kind string) bool {
	switch kind {
	case "runtime-executable", "runtime-bundle-manifest", "pack-asset", "common-asset", "runtime-asset":
		return true
	default:
		return false
	}
}

func readPlannedExclusiveContent(t *testing.T, plan ExclusiveInitPlan, path string) []byte {
	t.Helper()
	for _, write := range plan.Writes {
		if write.Path == path {
			content, err := exclusiveInitWriteBytes(write)
			if err != nil {
				t.Fatalf("read planned write %s: %v", path, err)
			}
			return content
		}
	}
	t.Fatalf("planned write not found: %s", path)
	return nil
}
