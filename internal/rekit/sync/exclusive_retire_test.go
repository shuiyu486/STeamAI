package sync

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildExclusiveInitRetirementPlanIsCompactAndDeterministic(t *testing.T) {
	initPlan := applyExclusiveInitRetirementFixture(t, filepath.Join(t.TempDir(), "case"), "owner")
	plan, err := BuildExclusiveInitRetirementPlan(initPlan)
	if err != nil {
		t.Fatal(err)
	}
	if plan.CaseRoot != initPlan.CaseRoot || plan.ProvisionID != initPlan.ProvisionID || plan.Role != initPlan.Role || len(plan.Leaves) != len(initPlan.Writes) {
		t.Fatalf("unexpected compact plan: %+v", plan)
	}
	for i, leaf := range plan.Leaves {
		if leaf.Path != initPlan.Writes[i].Path || leaf.SHA256 != initPlan.Writes[i].SHA256 || leaf.Size != initPlan.Writes[i].Size {
			t.Fatalf("leaf %d lost exact binding: %+v", i, leaf)
		}
	}
}

func TestExclusiveInitRetirementCanonicalizesTrailingRootSeparator(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "case")
	initPlan := applyExclusiveInitRetirementFixture(t, caseRoot, "owner")
	plan := retirementPlanForTest(t, initPlan)
	plan.CaseRoot += string(filepath.Separator)
	inspection, err := InspectExclusiveInitRetirementBatch(ExclusiveInitRetirementFirst, plan)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.RemainingRoots != 1 || inspection.Roots[0].CaseRoot != caseRoot {
		t.Fatalf("trailing-separator inspection = %+v", inspection)
	}
}

func TestExclusiveInitRetirementFirstInspectAndApplyExactTree(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "exact")
	initPlan := applyExclusiveInitRetirementFixture(t, caseRoot, "owner")
	plan := retirementPlanForTest(t, initPlan)

	inspection, err := InspectExclusiveInitRetirementBatch(ExclusiveInitRetirementFirst, plan)
	if err != nil {
		t.Fatal(err)
	}
	wantDirs := retirementDirectoryCount(plan)
	if inspection.RemainingLeaves != len(plan.Leaves) || inspection.RemainingDirectories != wantDirs || inspection.RemainingRoots != 1 {
		t.Fatalf("unexpected remaining counts: %+v", inspection)
	}
	if !inspection.CommonParentIdentity || inspection.CommonParent != filepath.Dir(caseRoot) {
		t.Fatalf("inspection did not bind common pinned parent: %+v", inspection)
	}
	result, err := ApplyExclusiveInitRetirementBatch(ExclusiveInitRetirementFirst, plan)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.RemovedLeaves != len(plan.Leaves) || result.RemovedDirectories != wantDirs || result.RemovedRoots != 1 || result.RemainingLeaves != 0 || result.RemainingDirectories != 0 || result.RemainingRoots != 0 {
		t.Fatalf("unexpected apply result: %+v", result)
	}
	if _, err := os.Lstat(caseRoot); !os.IsNotExist(err) {
		t.Fatalf("retirement left root: %v", err)
	}
}

func TestExclusiveInitRetirementFirstRequiresCompleteTreeAndResumeAllowsSubsetOrAbsent(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "partial")
	initPlan := applyExclusiveInitRetirementFixture(t, caseRoot, "owner")
	plan := retirementPlanForTest(t, initPlan)
	if err := os.Remove(filepath.Join(caseRoot, filepath.FromSlash(plan.Leaves[0].Path))); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectExclusiveInitRetirementBatch(ExclusiveInitRetirementFirst, plan); err == nil || !strings.Contains(err.Error(), "complete leaves") {
		t.Fatalf("first inspect error = %v, want complete-tree rejection", err)
	}
	inspection, err := InspectExclusiveInitRetirementBatch(ExclusiveInitRetirementResume, plan)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.RemainingLeaves != len(plan.Leaves)-1 || inspection.RemainingRoots != 1 {
		t.Fatalf("resume subset counts = %+v", inspection)
	}
	if _, err := ApplyExclusiveInitRetirementBatch(ExclusiveInitRetirementResume, plan); err != nil {
		t.Fatal(err)
	}
	inspection, err = InspectExclusiveInitRetirementBatch(ExclusiveInitRetirementResume, plan)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.RemainingLeaves != 0 || inspection.RemainingDirectories != 0 || inspection.RemainingRoots != 0 || inspection.Roots[0].Present {
		t.Fatalf("absent resume counts = %+v", inspection)
	}
	if err := os.Remove(filepath.Dir(caseRoot)); err != nil {
		t.Fatal(err)
	}
	inspection, err = InspectExclusiveInitRetirementBatch(ExclusiveInitRetirementResume, plan)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.RemainingRoots != 0 || inspection.Roots[0].Present {
		t.Fatalf("missing-parent resume counts = %+v", inspection)
	}
	if _, err := ApplyExclusiveInitRetirementBatch(ExclusiveInitRetirementResume, plan); err != nil {
		t.Fatal(err)
	}
}

func TestExclusiveInitRetirementMissingParentRebindFailsClosed(t *testing.T) {
	base := t.TempDir()
	caseRoot := filepath.Join(base, "missing", "case")
	plan := retirementPlanForTest(t, plannedExclusiveInitRetirementFixture(t, caseRoot, "owner"))
	exclusiveInitRetirementAfterPreflightHook = func() error {
		exclusiveInitRetirementAfterPreflightHook = nil
		return os.MkdirAll(caseRoot, 0o755)
	}
	t.Cleanup(func() { exclusiveInitRetirementAfterPreflightHook = nil })
	if _, err := ApplyExclusiveInitRetirementBatch(ExclusiveInitRetirementResume, plan); err == nil || !strings.Contains(err.Error(), "absent boundary rebound") {
		t.Fatalf("Apply error = %v, want missing-parent boundary rejection", err)
	}
}

func TestExclusiveInitRetirementRejectsExtraDifferentSymlinkAndNonRegular(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, plan ExclusiveInitPlan)
		want   string
	}{
		{name: "extra", mutate: func(t *testing.T, plan ExclusiveInitPlan) {
			writeText(t, filepath.Join(plan.CaseRoot, "extra.txt"), "extra\n")
		}, want: "extra object"},
		{name: "different", mutate: func(t *testing.T, plan ExclusiveInitPlan) {
			writeText(t, plan.Writes[0].TargetPath, "different\n")
		}, want: "different bytes"},
		{name: "symlink", mutate: func(t *testing.T, plan ExclusiveInitPlan) {
			target := filepath.Join(t.TempDir(), "target")
			writeText(t, target, "target\n")
			if err := os.Remove(plan.Writes[0].TargetPath); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, plan.Writes[0].TargetPath); err != nil {
				if runtime.GOOS == "windows" {
					t.Skipf("symlink unavailable: %v", err)
				}
				t.Fatal(err)
			}
		}, want: "symlink"},
		{name: "nonregular", mutate: func(t *testing.T, plan ExclusiveInitPlan) {
			if runtime.GOOS == "windows" {
				t.Skip("named pipes are not portable on Windows")
			}
			if err := os.Remove(plan.Writes[0].TargetPath); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(plan.Writes[0].TargetPath, 0o755); err != nil {
				t.Fatal(err)
			}
		}, want: "extra directory"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initPlan := applyExclusiveInitRetirementFixture(t, filepath.Join(t.TempDir(), "case"), "owner")
			tt.mutate(t, initPlan)
			plan := retirementPlanForTest(t, initPlan)
			if _, err := InspectExclusiveInitRetirementBatch(ExclusiveInitRetirementResume, plan); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("inspect error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestExclusiveInitRetirementBatchRejectsOverlappingRoots(t *testing.T) {
	outerRoot := filepath.Join(t.TempDir(), "outer")
	outerInit := applyExclusiveInitRetirementFixture(t, outerRoot, "outer")
	outer := retirementPlanForTest(t, outerInit)
	inner := outer
	inner.CaseRoot = filepath.Join(outerRoot, ".rekit")
	inner.Leaves = nil
	for _, leaf := range outer.Leaves {
		if rel, ok := strings.CutPrefix(leaf.Path, ".rekit/"); ok {
			leaf.Path = rel
			inner.Leaves = append(inner.Leaves, leaf)
		}
	}
	if _, err := InspectExclusiveInitRetirementBatch(ExclusiveInitRetirementFirst, outer, inner); err == nil || !strings.Contains(err.Error(), "overlapping roots") {
		t.Fatalf("Inspect error = %v, want overlap rejection", err)
	}
}

func TestExclusiveInitRetirementBoundToExpectedParent(t *testing.T) {
	workspace := t.TempDir()
	first := retirementPlanForTest(t, applyExclusiveInitRetirementFixture(t, filepath.Join(workspace, "first"), "first"))
	second := retirementPlanForTest(t, applyExclusiveInitRetirementFixture(t, filepath.Join(workspace, "second"), "second"))
	expected, err := os.Stat(workspace)
	if err != nil {
		t.Fatal(err)
	}
	result, err := ApplyExclusiveInitRetirementBatchBoundToParent(ExclusiveInitRetirementFirst, expected, first, second)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.RemovedRoots != 2 {
		t.Fatalf("bound result = %+v", result)
	}
}

func TestExclusiveInitRetirementDifferentExpectedParentMakesNoMutation(t *testing.T) {
	workspace := t.TempDir()
	initPlan := applyExclusiveInitRetirementFixture(t, filepath.Join(workspace, "case"), "owner")
	plan := retirementPlanForTest(t, initPlan)
	different, err := os.Stat(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyExclusiveInitRetirementBatchBoundToParent(ExclusiveInitRetirementFirst, different, plan); err == nil || !strings.Contains(err.Error(), "parent identity mismatch") {
		t.Fatalf("Apply error = %v, want parent mismatch", err)
	}
	for _, write := range initPlan.Writes {
		if got := readFile(t, write.TargetPath); !bytes.Equal(got, write.Content) {
			t.Fatalf("parent mismatch changed %s", write.Path)
		}
	}
}

func TestExclusiveInitRetirementBoundParentPathRebindNeverCrossesIdentity(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "workspace")
	original := filepath.Join(base, "original")
	if err := os.Mkdir(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	plan := retirementPlanForTest(t, applyExclusiveInitRetirementFixture(t, filepath.Join(workspace, "case"), "owner"))
	expected, err := os.Stat(workspace)
	if err != nil {
		t.Fatal(err)
	}
	rebound := false
	exclusiveInitRetirementAfterPreflightHook = func() error {
		exclusiveInitRetirementAfterPreflightHook = nil
		if err := os.Rename(workspace, original); err != nil {
			return fmt.Errorf("parent path rebind failed closed: %w", err)
		}
		rebound = true
		return os.Mkdir(workspace, 0o755)
	}
	t.Cleanup(func() { exclusiveInitRetirementAfterPreflightHook = nil })
	_, err = ApplyExclusiveInitRetirementBatchBoundToParent(ExclusiveInitRetirementFirst, expected, plan)
	if !rebound {
		if err == nil || !strings.Contains(err.Error(), "parent path rebind failed closed") {
			t.Fatalf("Apply error = %v, want platform fail-closed rebind", err)
		}
		return
	}
	if err != nil && !strings.Contains(err.Error(), "identity") {
		t.Fatalf("Apply error = %v", err)
	}
	entries, readErr := os.ReadDir(workspace)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("replacement parent changed: entries=%+v err=%v", entries, readErr)
	}
	if err == nil {
		if _, statErr := os.Lstat(filepath.Join(original, "case")); !os.IsNotExist(statErr) {
			t.Fatalf("successful pinned apply left original case: %v", statErr)
		}
	}
}

func TestExclusiveInitRetirementBatchPreflightsAllRootsBeforeDeletion(t *testing.T) {
	workspace := t.TempDir()
	firstInit := applyExclusiveInitRetirementFixture(t, filepath.Join(workspace, "first"), "first")
	secondInit := applyExclusiveInitRetirementFixture(t, filepath.Join(workspace, "second"), "second")
	first := retirementPlanForTest(t, firstInit)
	second := retirementPlanForTest(t, secondInit)
	writeText(t, filepath.Join(second.CaseRoot, "extra.txt"), "collision\n")

	if _, err := ApplyExclusiveInitRetirementBatch(ExclusiveInitRetirementFirst, first, second); err == nil || !strings.Contains(err.Error(), "extra object") {
		t.Fatalf("Apply error = %v, want second-root rejection", err)
	}
	for _, write := range firstInit.Writes {
		if got := readFile(t, write.TargetPath); !bytes.Equal(got, write.Content) {
			t.Fatalf("second-root failure changed first leaf %s", write.Path)
		}
	}
}

func TestExclusiveInitRetirementBatchRevalidatesSecondRootBeforeDeletingFirst(t *testing.T) {
	workspace := t.TempDir()
	firstInit := applyExclusiveInitRetirementFixture(t, filepath.Join(workspace, "first"), "first")
	secondInit := applyExclusiveInitRetirementFixture(t, filepath.Join(workspace, "second"), "second")
	first := retirementPlanForTest(t, firstInit)
	second := retirementPlanForTest(t, secondInit)
	mutated := false
	exclusiveInitRetirementAfterPreflightHook = func() error {
		exclusiveInitRetirementAfterPreflightHook = nil
		path := secondInit.Writes[0].TargetPath
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("second-root mutation failed closed: %w", err)
		}
		mutated = true
		writeText(t, path, "different\n")
		return nil
	}
	t.Cleanup(func() { exclusiveInitRetirementAfterPreflightHook = nil })

	_, err := ApplyExclusiveInitRetirementBatch(ExclusiveInitRetirementFirst, first, second)
	if !mutated {
		if err == nil || !strings.Contains(err.Error(), "second-root mutation failed closed") {
			t.Fatalf("Apply error = %v, want platform fail-closed mutation", err)
		}
		return
	}
	if err == nil || !(strings.Contains(err.Error(), "different bytes") || strings.Contains(err.Error(), "rebound")) {
		t.Fatalf("Apply error = %v, want second-root revalidation rejection", err)
	}
	for _, write := range firstInit.Writes {
		if got := readFile(t, write.TargetPath); !bytes.Equal(got, write.Content) {
			t.Fatalf("second-root revalidation error changed first leaf %s", write.Path)
		}
	}
}

func TestExclusiveInitRetirementBatchRejectsPostPreflightExtraBeforeDeletingFirst(t *testing.T) {
	workspace := t.TempDir()
	firstInit := applyExclusiveInitRetirementFixture(t, filepath.Join(workspace, "first"), "first")
	secondInit := applyExclusiveInitRetirementFixture(t, filepath.Join(workspace, "second"), "second")
	first := retirementPlanForTest(t, firstInit)
	second := retirementPlanForTest(t, secondInit)
	exclusiveInitRetirementAfterPreflightHook = func() error {
		exclusiveInitRetirementAfterPreflightHook = nil
		writeText(t, filepath.Join(second.CaseRoot, "extra.txt"), "extra\n")
		return nil
	}
	t.Cleanup(func() { exclusiveInitRetirementAfterPreflightHook = nil })

	if _, err := ApplyExclusiveInitRetirementBatch(ExclusiveInitRetirementFirst, first, second); err == nil || !strings.Contains(err.Error(), "extra object") {
		t.Fatalf("Apply error = %v, want post-preflight extra rejection", err)
	}
	for _, write := range firstInit.Writes {
		if got := readFile(t, write.TargetPath); !bytes.Equal(got, write.Content) {
			t.Fatalf("post-preflight second-root extra changed first leaf %s", write.Path)
		}
	}
}

func TestExclusiveInitRetirementBatchRejectsSecondRootDirectoryQuarantineChildBeforeFirstMutation(t *testing.T) {
	workspace := t.TempDir()
	firstInit := applyExclusiveInitRetirementFixture(t, filepath.Join(workspace, "first"), "first")
	secondInit := applyExclusiveInitRetirementFixture(t, filepath.Join(workspace, "second"), "second")
	first := retirementPlanForTest(t, firstInit)
	second := retirementPlanForTest(t, secondInit)
	_, directories, err := normalizeExclusiveInitRetirementPlan(second)
	if err != nil {
		t.Fatal(err)
	}
	victim := directories[0]
	canonical := filepath.Join(second.CaseRoot, filepath.FromSlash(victim))
	quarantine := filepath.Join(second.CaseRoot, filepath.FromSlash(exclusiveInitRetirementDirectoryQuarantinePath(victim, second)))
	if err := os.Rename(canonical, quarantine); err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(quarantine, "drift.txt"), "drift\n")

	if _, err := ApplyExclusiveInitRetirementBatch(ExclusiveInitRetirementResume, first, second); err == nil || !strings.Contains(err.Error(), "quarantine is not empty") {
		t.Fatalf("Apply error = %v, want second-root quarantine child rejection", err)
	}
	for _, write := range firstInit.Writes {
		if got := readFile(t, write.TargetPath); !bytes.Equal(got, write.Content) {
			t.Fatalf("second-root quarantine drift changed first leaf %s", write.Path)
		}
	}
}

func TestExclusiveInitRetirementInterruptedApplyResumesExactSubset(t *testing.T) {
	initPlan := applyExclusiveInitRetirementFixture(t, filepath.Join(t.TempDir(), "case"), "owner")
	plan := retirementPlanForTest(t, initPlan)
	interruptPath := plan.Leaves[1].Path
	exclusiveInitRetirementRemoveHook = func(stage, caseRoot, rel string) error {
		if stage == "before-leaf" && rel == interruptPath {
			return fmt.Errorf("simulated retirement interrupt")
		}
		return nil
	}
	t.Cleanup(func() { exclusiveInitRetirementRemoveHook = nil })
	result, err := ApplyExclusiveInitRetirementBatch(ExclusiveInitRetirementFirst, plan)
	if err == nil || !strings.Contains(err.Error(), "simulated retirement interrupt") || result.RemovedLeaves != 1 || result.RemainingLeaves != len(plan.Leaves)-1 {
		t.Fatalf("interrupted result=%+v err=%v", result, err)
	}
	if result.Roots[0].RemainingLeaves != result.RemainingLeaves || result.Roots[0].RemainingDirectories != result.RemainingDirectories || result.Roots[0].RemainingRoots != result.RemainingRoots {
		t.Fatalf("interrupted per-root counts disagree with totals: %+v", result)
	}
	exclusiveInitRetirementRemoveHook = nil
	inspection, err := InspectExclusiveInitRetirementBatch(ExclusiveInitRetirementResume, plan)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.RemainingLeaves != len(plan.Leaves)-1 {
		t.Fatalf("resume inspection = %+v", inspection)
	}
	if _, err := ApplyExclusiveInitRetirementBatch(ExclusiveInitRetirementResume, plan); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(plan.CaseRoot); !os.IsNotExist(err) {
		t.Fatalf("resume left root: %v", err)
	}
}

func TestExclusiveInitRetirementResumesPostLeafQuarantineCrash(t *testing.T) {
	initPlan := applyExclusiveInitRetirementFixture(t, filepath.Join(t.TempDir(), "case"), "owner")
	plan := retirementPlanForTest(t, initPlan)
	victim := plan.Leaves[0]
	exclusiveInitRetirementRemoveHook = func(stage, caseRoot, rel string) error {
		if stage == "after-leaf-quarantine" && rel == victim.Path {
			return fmt.Errorf("simulated post-rename crash")
		}
		return nil
	}
	t.Cleanup(func() { exclusiveInitRetirementRemoveHook = nil })
	if _, err := ApplyExclusiveInitRetirementBatch(ExclusiveInitRetirementFirst, plan); err == nil || !strings.Contains(err.Error(), "post-rename crash") {
		t.Fatalf("first Apply error = %v", err)
	}
	canonical := filepath.Join(plan.CaseRoot, filepath.FromSlash(victim.Path))
	quarantine := filepath.Join(plan.CaseRoot, filepath.FromSlash(exclusiveInitRetirementLeafQuarantinePath(victim)))
	if _, err := os.Lstat(canonical); !os.IsNotExist(err) {
		t.Fatalf("crash retained canonical leaf: %v", err)
	}
	if got := readFile(t, quarantine); !bytes.Equal(got, initPlan.Writes[0].Content) {
		t.Fatal("crash did not retain exact owned quarantine")
	}
	exclusiveInitRetirementRemoveHook = nil
	if _, err := ApplyExclusiveInitRetirementBatch(ExclusiveInitRetirementResume, plan); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(plan.CaseRoot); !os.IsNotExist(err) {
		t.Fatalf("resume left root: %v", err)
	}
}

func TestExclusiveInitRetirementQuarantineDriftFailsClosed(t *testing.T) {
	initPlan := applyExclusiveInitRetirementFixture(t, filepath.Join(t.TempDir(), "case"), "owner")
	plan := retirementPlanForTest(t, initPlan)
	victim := plan.Leaves[0]
	exclusiveInitRetirementRemoveHook = func(stage, caseRoot, rel string) error {
		if stage == "after-leaf-quarantine" && rel == victim.Path {
			return fmt.Errorf("simulated post-rename crash")
		}
		return nil
	}
	t.Cleanup(func() { exclusiveInitRetirementRemoveHook = nil })
	if _, err := ApplyExclusiveInitRetirementBatch(ExclusiveInitRetirementFirst, plan); err == nil {
		t.Fatal("first Apply unexpectedly succeeded")
	}
	exclusiveInitRetirementRemoveHook = nil
	quarantine := filepath.Join(plan.CaseRoot, filepath.FromSlash(exclusiveInitRetirementLeafQuarantinePath(victim)))
	writeText(t, quarantine, "drifted quarantine\n")
	if _, err := ApplyExclusiveInitRetirementBatch(ExclusiveInitRetirementResume, plan); err == nil || !strings.Contains(err.Error(), "different bytes") {
		t.Fatalf("resume error = %v, want quarantine drift rejection", err)
	}
	if got := string(readFile(t, quarantine)); got != "drifted quarantine\n" {
		t.Fatalf("drifted quarantine changed: %q", got)
	}
}

func TestExclusiveInitRetirementResumesPostDirectoryQuarantineCrash(t *testing.T) {
	initPlan := applyExclusiveInitRetirementFixture(t, filepath.Join(t.TempDir(), "case"), "owner")
	plan := retirementPlanForTest(t, initPlan)
	_, directories, err := normalizeExclusiveInitRetirementPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	victim := directories[0]
	exclusiveInitRetirementRemoveHook = func(stage, caseRoot, rel string) error {
		if stage == "after-directory-quarantine" && rel == victim {
			return fmt.Errorf("simulated directory post-rename crash")
		}
		return nil
	}
	t.Cleanup(func() { exclusiveInitRetirementRemoveHook = nil })
	if _, err := ApplyExclusiveInitRetirementBatch(ExclusiveInitRetirementFirst, plan); err == nil || !strings.Contains(err.Error(), "directory post-rename crash") {
		t.Fatalf("first Apply error = %v", err)
	}
	canonical := filepath.Join(plan.CaseRoot, filepath.FromSlash(victim))
	quarantine := filepath.Join(plan.CaseRoot, filepath.FromSlash(exclusiveInitRetirementDirectoryQuarantinePath(victim, plan)))
	if _, err := os.Lstat(canonical); !os.IsNotExist(err) {
		t.Fatalf("crash retained canonical directory: %v", err)
	}
	if info, err := os.Lstat(quarantine); err != nil || !info.IsDir() {
		t.Fatalf("crash did not retain directory quarantine: %v", err)
	}
	exclusiveInitRetirementRemoveHook = nil
	if _, err := ApplyExclusiveInitRetirementBatch(ExclusiveInitRetirementResume, plan); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(plan.CaseRoot); !os.IsNotExist(err) {
		t.Fatalf("resume left root: %v", err)
	}
}

func TestExclusiveInitRetirementResumesPostRootQuarantineCrash(t *testing.T) {
	workspace := t.TempDir()
	caseRoot := filepath.Join(workspace, "case")
	initPlan := applyExclusiveInitRetirementFixture(t, caseRoot, "owner")
	plan := retirementPlanForTest(t, initPlan)
	exclusiveInitRetirementRemoveHook = func(stage, caseRoot, rel string) error {
		if stage == "after-root-quarantine" {
			return fmt.Errorf("simulated root post-rename crash")
		}
		return nil
	}
	t.Cleanup(func() { exclusiveInitRetirementRemoveHook = nil })
	if _, err := ApplyExclusiveInitRetirementBatch(ExclusiveInitRetirementFirst, plan); err == nil || !strings.Contains(err.Error(), "root post-rename crash") {
		t.Fatalf("first Apply error = %v", err)
	}
	quarantine := filepath.Join(workspace, exclusiveInitRetirementRootQuarantineName("case", plan))
	if _, err := os.Lstat(caseRoot); !os.IsNotExist(err) {
		t.Fatalf("crash retained canonical root: %v", err)
	}
	if info, err := os.Lstat(quarantine); err != nil || !info.IsDir() {
		t.Fatalf("crash did not retain root quarantine: %v", err)
	}
	exclusiveInitRetirementRemoveHook = nil
	if _, err := ApplyExclusiveInitRetirementBatch(ExclusiveInitRetirementResume, plan); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(quarantine); !os.IsNotExist(err) {
		t.Fatalf("resume left root quarantine: %v", err)
	}
}

func TestExclusiveInitRetirementRejectsRootRebindBeforeMutation(t *testing.T) {
	workspace := t.TempDir()
	caseRoot := filepath.Join(workspace, "case")
	originalRoot := filepath.Join(workspace, "original")
	initPlan := applyExclusiveInitRetirementFixture(t, caseRoot, "owner")
	plan := retirementPlanForTest(t, initPlan)
	rebindSucceeded := false
	exclusiveInitRetirementAfterPreflightHook = func() error {
		if err := os.Rename(caseRoot, originalRoot); err != nil {
			return fmt.Errorf("root rebind failed closed: %w", err)
		}
		rebindSucceeded = true
		return os.Mkdir(caseRoot, 0o755)
	}
	t.Cleanup(func() { exclusiveInitRetirementAfterPreflightHook = nil })
	_, err := ApplyExclusiveInitRetirementBatch(ExclusiveInitRetirementFirst, plan)
	if !rebindSucceeded {
		if err == nil || !strings.Contains(err.Error(), "root rebind failed closed") {
			t.Fatalf("Apply error = %v, want platform fail-closed rebind", err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), "root identity changed") {
		t.Fatalf("Apply error = %v, want identity rejection", err)
	}
	for _, write := range initPlan.Writes {
		if got := readFile(t, filepath.Join(originalRoot, filepath.FromSlash(write.Path))); !bytes.Equal(got, write.Content) {
			t.Fatalf("rebind changed original leaf %s", write.Path)
		}
	}
	entries, readErr := os.ReadDir(caseRoot)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("replacement root changed: entries=%+v err=%v", entries, readErr)
	}
}

func TestExclusiveInitRetirementQuarantineKeepsSameBytesReplacement(t *testing.T) {
	initPlan := applyExclusiveInitRetirementFixture(t, filepath.Join(t.TempDir(), "case"), "owner")
	plan := retirementPlanForTest(t, initPlan)
	victim := plan.Leaves[0]
	original := filepath.Join(plan.CaseRoot, filepath.FromSlash(victim.Path))
	exclusiveInitRetirementRemoveHook = func(stage, caseRoot, rel string) error {
		if stage != "after-leaf-quarantine" || rel != victim.Path {
			return nil
		}
		exclusiveInitRetirementRemoveHook = nil
		writeText(t, original, string(initPlan.Writes[0].Content))
		return nil
	}
	t.Cleanup(func() { exclusiveInitRetirementRemoveHook = nil })
	if _, err := ApplyExclusiveInitRetirementBatch(ExclusiveInitRetirementFirst, plan); err == nil || !strings.Contains(err.Error(), "name rebound") {
		t.Fatalf("Apply error = %v, want same-bytes name rebound rejection", err)
	}
	if got := readFile(t, original); !bytes.Equal(got, initPlan.Writes[0].Content) {
		t.Fatal("same-bytes replacement changed")
	}
}

func TestExclusiveInitRetirementQuarantineKeepsDifferentBytesReplacement(t *testing.T) {
	initPlan := applyExclusiveInitRetirementFixture(t, filepath.Join(t.TempDir(), "case"), "owner")
	plan := retirementPlanForTest(t, initPlan)
	victim := plan.Leaves[0]
	original := filepath.Join(plan.CaseRoot, filepath.FromSlash(victim.Path))
	exclusiveInitRetirementRemoveHook = func(stage, caseRoot, rel string) error {
		if stage != "after-leaf-quarantine" || rel != victim.Path {
			return nil
		}
		exclusiveInitRetirementRemoveHook = nil
		writeText(t, original, "different replacement\n")
		return nil
	}
	t.Cleanup(func() { exclusiveInitRetirementRemoveHook = nil })
	if _, err := ApplyExclusiveInitRetirementBatch(ExclusiveInitRetirementFirst, plan); err == nil || !strings.Contains(err.Error(), "name rebound") {
		t.Fatalf("Apply error = %v, want different-bytes name rebound rejection", err)
	}
	if got := string(readFile(t, original)); got != "different replacement\n" {
		t.Fatalf("different replacement changed: %q", got)
	}
}

func TestExclusiveInitRetirementRejectsLeafRebindAtRemoveHook(t *testing.T) {
	initPlan := applyExclusiveInitRetirementFixture(t, filepath.Join(t.TempDir(), "case"), "owner")
	plan := retirementPlanForTest(t, initPlan)
	victim := plan.Leaves[0]
	original := filepath.Join(plan.CaseRoot, filepath.FromSlash(victim.Path))
	replacement := filepath.Join(t.TempDir(), "replacement")
	writeText(t, replacement, string(readFile(t, original)))
	exclusiveInitRetirementRemoveHook = func(stage, caseRoot, rel string) error {
		if stage != "before-leaf" || rel != victim.Path {
			return nil
		}
		exclusiveInitRetirementRemoveHook = nil
		if err := os.Remove(original); err != nil {
			return err
		}
		return os.Rename(replacement, original)
	}
	t.Cleanup(func() { exclusiveInitRetirementRemoveHook = nil })
	if _, err := ApplyExclusiveInitRetirementBatch(ExclusiveInitRetirementFirst, plan); err == nil || !strings.Contains(err.Error(), "rebound") {
		t.Fatalf("Apply error = %v, want leaf rebind rejection", err)
	}
	if _, err := os.Lstat(original); !os.IsNotExist(err) {
		t.Fatalf("rebound failure unexpectedly retained original name: %v", err)
	}
}

func applyExclusiveInitRetirementFixture(t *testing.T, caseRoot, role string) ExclusiveInitPlan {
	t.Helper()
	plan := plannedExclusiveInitRetirementFixture(t, caseRoot, role)
	if _, err := ApplyExclusiveInit(plan); err != nil {
		t.Fatal(err)
	}
	return plan
}

func plannedExclusiveInitRetirementFixture(t *testing.T, caseRoot, role string) ExclusiveInitPlan {
	t.Helper()
	repoRoot, pack := exclusiveInitFixture(t)
	opt := exclusiveInitOptionsForTest()
	opt.Role = role
	plan, err := PlanExclusiveInit(repoRoot, caseRoot, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func retirementPlanForTest(t *testing.T, plan ExclusiveInitPlan) ExclusiveInitRetirementPlan {
	t.Helper()
	retirement, err := BuildExclusiveInitRetirementPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	return retirement
}

func retirementDirectoryCount(plan ExclusiveInitRetirementPlan) int {
	dirs := map[string]struct{}{}
	for _, leaf := range plan.Leaves {
		for dir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(leaf.Path))); dir != "."; dir = filepath.ToSlash(filepath.Dir(filepath.FromSlash(dir))) {
			dirs[dir] = struct{}{}
		}
	}
	return len(dirs)
}
