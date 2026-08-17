package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func TestCurrentSyncStageValidatedBarrierAcceptsFrozenStaging(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	_, intent := stageCurrentSyncExactObjectFixture(t, fixture)
	progress, err := newCurrentSyncProgress(intent)
	if err != nil {
		t.Fatal(err)
	}
	progress, err = currentSyncBeginProgressOperation(progress, intent)
	if err != nil {
		t.Fatal(err)
	}
	if err := executeCurrentSyncValidateOperation(
		fixture.caseRoot,
		intent,
		progress,
		*progress.Pending,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(fixture.repoRoot); err != nil {
		t.Fatal(err)
	}
	if err := executeCurrentSyncValidateOperation(
		fixture.caseRoot,
		intent,
		progress,
		*progress.Pending,
	); err != nil {
		t.Fatalf("current sync stage barrier depended on missing source: %v", err)
	}
}

func TestCurrentSyncReadyToActivateBarrierRejectsActivationDrift(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	plan, intent := currentSyncBarrierPlan(t, fixture)
	writeCurrentSyncNextBarrierState(t, fixture.caseRoot, plan, false)
	progress := currentSyncProgressBeforeOperation(t, intent, "ready-to-activate")
	if err := executeCurrentSyncValidateOperation(
		fixture.caseRoot,
		intent,
		progress,
		*progress.Pending,
	); err != nil {
		t.Fatal(err)
	}
	activation := currentSyncTestLeaf(t, plan, ".steamai/instance.yml")
	if err := os.WriteFile(
		filepath.Join(fixture.caseRoot, projectstate.CurrentDir, "instance.yml"),
		activation.after,
		activation.mode,
	); err != nil {
		t.Fatal(err)
	}
	if err := executeCurrentSyncValidateOperation(
		fixture.caseRoot,
		intent,
		progress,
		*progress.Pending,
	); err == nil || !strings.Contains(err.Error(), "pre-activation leaf") {
		t.Fatalf("current sync ready-to-activate accepted early activation: %v", err)
	}
}

func TestCurrentSyncActivatedBarrierRequiresExactInstance(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	plan, intent := currentSyncBarrierPlan(t, fixture)
	writeCurrentSyncNextBarrierState(t, fixture.caseRoot, plan, true)
	progress := currentSyncProgressBeforeOperation(t, intent, "activated")
	if err := executeCurrentSyncValidateOperation(
		fixture.caseRoot,
		intent,
		progress,
		*progress.Pending,
	); err != nil {
		t.Fatal(err)
	}
	activation := currentSyncTestLeaf(t, plan, ".steamai/instance.yml")
	if err := os.WriteFile(
		filepath.Join(fixture.caseRoot, projectstate.CurrentDir, "instance.yml"),
		activation.before,
		activation.mode,
	); err != nil {
		t.Fatal(err)
	}
	if err := executeCurrentSyncValidateOperation(
		fixture.caseRoot,
		intent,
		progress,
		*progress.Pending,
	); err == nil || !strings.Contains(err.Error(), "active instance") {
		t.Fatalf("current sync activated barrier accepted previous instance: %v", err)
	}
}

func TestCurrentSyncBundleValidatedBarrierRejectsLiveDrift(t *testing.T) {
	t.Run("exact", func(t *testing.T) {
		fixture := newCurrentSyncFixture(t, "")
		plan, intent := currentSyncBarrierPlan(t, fixture)
		writeCurrentSyncNextBarrierState(t, fixture.caseRoot, plan, true)
		progress := currentSyncProgressBeforeOperation(t, intent, "bundle-validated")
		if err := executeCurrentSyncValidateOperation(
			fixture.caseRoot,
			intent,
			progress,
			*progress.Pending,
		); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("controlled", func(t *testing.T) {
		fixture := newCurrentSyncFixture(t, "")
		plan, intent := currentSyncBarrierPlan(t, fixture)
		writeCurrentSyncNextBarrierState(t, fixture.caseRoot, plan, true)
		var target CurrentSyncBinding
		for _, binding := range plan.NextControlled.Entries {
			if binding.Kind != "runtime-executable" {
				target = binding
				break
			}
		}
		if target.Path == "" {
			t.Fatal("current sync next controlled inventory has no drift target")
		}
		if err := os.WriteFile(
			filepath.Join(fixture.caseRoot, filepath.FromSlash(target.Path)),
			[]byte("drifted controlled file\n"),
			os.FileMode(target.Mode),
		); err != nil {
			t.Fatal(err)
		}
		progress := currentSyncProgressBeforeOperation(t, intent, "bundle-validated")
		if err := executeCurrentSyncValidateOperation(
			fixture.caseRoot,
			intent,
			progress,
			*progress.Pending,
		); err == nil {
			t.Fatal("current sync bundle barrier accepted controlled drift")
		}
	})

	t.Run("target", func(t *testing.T) {
		fixture := newCurrentSyncFixture(t, "")
		plan, intent := currentSyncBarrierPlan(t, fixture)
		writeCurrentSyncNextBarrierState(t, fixture.caseRoot, plan, true)
		var target CurrentSyncBinding
		for _, binding := range plan.NextTargets.Entries {
			if binding.Path == ".claude/skills/steamai/SKILL.md" {
				target = binding
				break
			}
		}
		if target.Path == "" {
			t.Fatal("current sync next target inventory has no skill drift target")
		}
		if err := os.WriteFile(
			filepath.Join(fixture.caseRoot, filepath.FromSlash(target.Path)),
			[]byte("drifted project skill\n"),
			os.FileMode(target.Mode),
		); err != nil {
			t.Fatal(err)
		}
		progress := currentSyncProgressBeforeOperation(t, intent, "bundle-validated")
		if err := executeCurrentSyncValidateOperation(
			fixture.caseRoot,
			intent,
			progress,
			*progress.Pending,
		); err == nil || !strings.Contains(err.Error(), "target inventory") {
			t.Fatalf("current sync bundle barrier accepted target drift: %v", err)
		}
	})
}

func TestCurrentSyncReceiptCommittedBarrierRequiresDurableLineage(t *testing.T) {
	t.Run("exact", func(t *testing.T) {
		fixture := newCurrentSyncFixture(t, "")
		plan, intent := currentSyncBarrierPlan(t, fixture)
		progress, _ := currentSyncReceiptCommitBarrierFixture(
			t,
			fixture,
			plan,
			intent,
		)
		if err := executeCurrentSyncValidateOperation(
			fixture.caseRoot,
			intent,
			progress,
			*progress.Pending,
		); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("missing-receipt-progress", func(t *testing.T) {
		fixture := newCurrentSyncFixture(t, "")
		plan, intent := currentSyncBarrierPlan(t, fixture)
		progress, receipt := currentSyncReceiptCommitBarrierFixture(
			t,
			fixture,
			plan,
			intent,
		)
		receipt.ProgressSHA256 = strings.Repeat("a", 64)
		var err error
		receipt.ReceiptSHA256, err = currentSyncCanonicalSHA(
			currentSyncReceiptIdentityFor(receipt),
		)
		if err != nil {
			t.Fatal(err)
		}
		writeCurrentSyncBarrierReceipt(t, fixture.caseRoot, receipt)
		if err := executeCurrentSyncValidateOperation(
			fixture.caseRoot,
			intent,
			progress,
			*progress.Pending,
		); err == nil || !strings.Contains(err.Error(), "lineage") {
			t.Fatalf("current sync receipt barrier accepted missing lineage: %v", err)
		}
	})
}

func currentSyncBarrierPlan(
	t *testing.T,
	fixture currentSyncFixtureState,
) (CurrentSyncPlan, currentSyncIntent) {
	t.Helper()
	plan, err := CurrentSyncPreview(
		fixture.repoRoot,
		fixture.caseRoot,
		fixture.pack,
		CurrentSyncOptions{
			Command:          "sync",
			ProjectName:      "refreshed-project",
			SourceExecutable: fixture.sourceExecutable,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	intent, err := buildCurrentSyncIntent(plan)
	if err != nil {
		t.Fatal(err)
	}
	return plan, intent
}

func writeCurrentSyncNextBarrierState(
	t *testing.T,
	caseRoot string,
	plan CurrentSyncPlan,
	activate bool,
) {
	t.Helper()
	if plan.prepared == nil {
		t.Fatal("current sync barrier plan omitted prepared state")
	}
	for _, publication := range plan.prepared.publications {
		target := filepath.Join(
			caseRoot,
			projectstate.CurrentDir,
			filepath.FromSlash(publication.rel),
		)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, publication.data, publication.mode); err != nil {
			t.Fatal(err)
		}
	}
	for _, rel := range plan.ObsoleteControlled {
		if err := os.Remove(filepath.Join(
			caseRoot,
			filepath.FromSlash(rel),
		)); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	for _, leaf := range plan.prepared.leaves {
		if leaf.rel == projectstate.CurrentDir+"/instance.yml" && !activate {
			continue
		}
		target := filepath.Join(caseRoot, filepath.FromSlash(leaf.rel))
		if !leaf.afterExists {
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, leaf.after, leaf.mode); err != nil {
			t.Fatal(err)
		}
	}
}

func currentSyncReceiptCommitBarrierFixture(
	t *testing.T,
	fixture currentSyncFixtureState,
	plan CurrentSyncPlan,
	intent currentSyncIntent,
) (currentSyncProgress, currentSyncReceipt) {
	t.Helper()
	if _, err := publishCurrentSyncIntent(fixture.caseRoot, intent); err != nil {
		t.Fatal(err)
	}
	if err := stageCurrentSyncMaterial(fixture.caseRoot, intent, plan); err != nil {
		t.Fatal(err)
	}
	writeCurrentSyncNextBarrierState(t, fixture.caseRoot, plan, true)
	progress, err := newCurrentSyncProgress(intent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := publishCurrentSyncProgress(
		fixture.caseRoot,
		intent,
		progress,
	); err != nil {
		t.Fatal(err)
	}
	var receipt currentSyncReceipt
	for {
		progress, err = currentSyncBeginProgressOperation(progress, intent)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := publishCurrentSyncProgress(
			fixture.caseRoot,
			intent,
			progress,
		); err != nil {
			t.Fatal(err)
		}
		switch progress.Pending.Kind {
		case "receipt-stage-to-live":
			receipt, err = buildCurrentSyncReceipt(intent, progress)
			if err != nil {
				t.Fatal(err)
			}
		case "receipt-committed":
			if receipt.Kind == "" {
				t.Fatal("current sync receipt commit fixture omitted receipt")
			}
			writeCurrentSyncBarrierReceipt(t, fixture.caseRoot, receipt)
			return progress, receipt
		}
		progress, err = currentSyncCompleteProgressOperation(progress, intent)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := publishCurrentSyncProgress(
			fixture.caseRoot,
			intent,
			progress,
		); err != nil {
			t.Fatal(err)
		}
	}
}

func writeCurrentSyncBarrierReceipt(
	t *testing.T,
	caseRoot string,
	receipt currentSyncReceipt,
) {
	t.Helper()
	data, err := currentSyncCanonicalData(receipt)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(
		caseRoot,
		projectstate.CurrentDir,
		filepath.FromSlash(currentSyncReceiptRel),
	)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
