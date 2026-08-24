package executioncontrol

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/capabilitycontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
	"github.com/shuiyu486/re-context-kits/internal/rekit/plancontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/testfixture"
)

const testLane = "binary-analysis-main"

func TestDecodeVersionedBindingKeepsLegacyOutOfCurrentPath(t *testing.T) {
	owner := laneowner.Snapshot{Lane: testLane, CurrentExecutor: "member-main", ExecutorGeneration: 7}
	legacy := LegacyBinding{SchemaVersion: LegacyBindingSchemaVersion, Lane: testLane, Owner: owner}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeVersionedBinding(data)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Version != LegacyBindingSchemaVersion || decoded.Legacy == nil || decoded.Current != nil || decoded.WholeSHA256 != hash(data) || !bytes.Equal(decoded.Raw, data) {
		t.Fatalf("legacy binding decode = %+v", decoded)
	}
	currentCapability, err := capabilitycontract.Bind(capabilitycontract.Transport())
	if err != nil {
		t.Fatal(err)
	}
	current := Binding{SchemaVersion: BindingSchemaVersion, Lane: testLane, Owner: owner, Capability: currentCapability}
	currentData, err := json.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	currentDecoded, err := DecodeVersionedBinding(currentData)
	if err != nil || currentDecoded.Current == nil || currentDecoded.Legacy != nil || currentDecoded.Version != BindingSchemaVersion {
		t.Fatalf("current binding decode = %+v err=%v", currentDecoded, err)
	}
}

func TestPreviewIsZeroWriteAndBindsDurableOwner(t *testing.T) {
	caseRoot := controlFixture(t, false)
	plan, err := Preview(caseRoot, Options{
		Lane: testLane, Action: ActionPause, Actor: "main-agent", Reason: "operator requested a pause",
		PublicationStamp: "2026-08-18T12:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.State != StatePaused || plan.ControlGeneration != 1 || plan.Owner.CurrentExecutor != "member-main" || plan.Owner.ExecutorGeneration != 7 || len(plan.ExpectedPlanSHA256) != 64 {
		t.Fatalf("unexpected control preview: %+v", plan)
	}
	if !strings.HasPrefix(plan.ApplyCommand, "/steamai control ") || !strings.Contains(plan.ApplyCommand, "-ExpectedControlPlanSha256 "+plan.ExpectedPlanSHA256) {
		t.Fatalf("current control preview has invalid typed apply command: %s", plan.ApplyCommand)
	}
	if !plan.ReviewRequired || !plan.RequiresConfirmation || !plan.NoAuthority || !plan.NoConfirmed || !plan.NoHeavyTool || !plan.NoAutoResume {
		t.Fatalf("control preview crossed its boundary: %+v", plan)
	}
	controlRoot := filepath.Join(caseRoot, ".steamai", "lanes", testLane, controlDir)
	if _, err := os.Lstat(controlRoot); !os.IsNotExist(err) {
		t.Fatalf("preview wrote control state: %v", err)
	}
}

func TestApplyPauseResumeStopUsesIndependentAppendOnlyGeneration(t *testing.T) {
	caseRoot := controlFixture(t, false)
	applyControl(t, caseRoot, ActionPause, "pause for operator review", "2026-08-18T12:00:00Z", 1, StatePaused)
	applyControl(t, caseRoot, ActionResume, "resume only future work", "2026-08-18T12:01:00Z", 2, StateRunning)
	applyControl(t, caseRoot, ActionStop, "stop this lane campaign", "2026-08-18T12:02:00Z", 3, StateStopped)

	inspection, err := Inspect(caseRoot, testLane)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Pending || inspection.State != StateStopped || inspection.CurrentGeneration != 3 || len(inspection.CurrentReceiptSHA256) != 64 || len(inspection.Receipts) != 3 {
		t.Fatalf("unexpected control chain: %+v", inspection)
	}
	if _, err := Preview(caseRoot, Options{Lane: testLane, Action: ActionResume, Actor: "main-agent", Reason: "must not revive stopped work", PublicationStamp: "2026-08-18T12:03:00Z"}); err == nil || !strings.Contains(err.Error(), "invalid while lane state is stopped") {
		t.Fatalf("stopped lane accepted resume: %v", err)
	}
}

func TestApplyRejectsStaleReviewedHeadBeforeWriting(t *testing.T) {
	caseRoot := controlFixture(t, false)
	stale, err := Preview(caseRoot, Options{Lane: testLane, Action: ActionStop, Actor: "main-agent", Reason: "reviewed stop", PublicationStamp: "2026-08-18T12:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	applyControl(t, caseRoot, ActionPause, "newer pause", "2026-08-18T12:01:00Z", 1, StatePaused)
	_, err = Apply(caseRoot, Options{
		Lane: testLane, Action: stale.Action, Actor: stale.Actor, Reason: stale.Reason,
		PublicationStamp: stale.PublicationStamp, ExpectedPlanSHA256: stale.ExpectedPlanSHA256,
	})
	failure, typed := plancontract.FromError(err)
	if err == nil || !typed || failure.Code != plancontract.CodePlanMismatch || failure.MutationApplied || failure.MutationBoundary != "none" {
		t.Fatalf("stale control head was accepted: %v failure=%+v typed=%t", err, failure, typed)
	}
	inspection, inspectErr := Inspect(caseRoot, testLane)
	if inspectErr != nil {
		t.Fatal(inspectErr)
	}
	if inspection.CurrentGeneration != 1 || inspection.State != StatePaused || inspection.Pending {
		t.Fatalf("stale Apply changed control state: %+v", inspection)
	}
}

func TestApplyRecoversPendingIntentAndCommittedResponseLoss(t *testing.T) {
	caseRoot := controlFixture(t, false)
	preview, err := Preview(caseRoot, Options{Lane: testLane, Action: ActionPause, Actor: "main-agent", Reason: "recover exact pause", PublicationStamp: "2026-08-18T12:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	opt := Options{Lane: preview.Lane, Action: preview.Action, Actor: preview.Actor, Reason: preview.Reason, PublicationStamp: preview.PublicationStamp, ExpectedPlanSHA256: preview.ExpectedPlanSHA256}
	applyAfterIntentHook = func() error { return errors.New("stop after durable control intent") }
	_, err = Apply(caseRoot, opt)
	applyAfterIntentHook = nil
	if err == nil || !strings.Contains(err.Error(), "stop after durable control intent") {
		t.Fatalf("intent interruption was not exercised: %v", err)
	}
	inspection, err := Inspect(caseRoot, testLane)
	if err != nil {
		t.Fatal(err)
	}
	if !inspection.Pending || inspection.PendingGeneration != 1 || inspection.CurrentGeneration != 0 || inspection.State != StateRunning {
		t.Fatalf("interrupted intent is not recoverable: %+v", inspection)
	}
	wrong := opt
	wrong.ExpectedPlanSHA256 = strings.Repeat("0", 64)
	_, err = Apply(caseRoot, wrong)
	failure, typed := plancontract.FromError(err)
	if err == nil || !typed || failure.Code != plancontract.CodePlanMismatch || failure.MutationApplied || failure.MutationBoundary != "none" {
		t.Fatalf("mismatched recovery was accepted: %v failure=%+v typed=%t", err, failure, typed)
	}

	applyAfterReceiptHook = func() error { return errors.New("response lost after durable control receipt") }
	_, err = Apply(caseRoot, opt)
	applyAfterReceiptHook = nil
	if err == nil || !strings.Contains(err.Error(), "response lost after durable control receipt") {
		t.Fatalf("receipt response loss was not exercised: %v", err)
	}
	before := snapshotControlTree(t, caseRoot)
	replayed, err := Apply(caseRoot, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Applied || !replayed.AlreadyApplied || replayed.State != StatePaused || replayed.ControlGeneration != 1 {
		t.Fatalf("committed control response was not replayed: %+v", replayed)
	}
	if after := snapshotControlTree(t, caseRoot); after != before {
		t.Fatal("exact committed replay changed durable control state")
	}
}

func TestControlUsesSelectedStateRootOnly(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "current"
		selected, other := ".steamai", ".rekit"
		if legacy {
			name, selected, other = "legacy", ".rekit", ".steamai"
		}
		t.Run(name, func(t *testing.T) {
			caseRoot := controlFixture(t, legacy)
			preview, err := Preview(caseRoot, Options{Lane: testLane, Action: ActionPause, Actor: "main-agent", Reason: "selected root only", PublicationStamp: "2026-08-18T12:00:00Z"})
			if err != nil {
				t.Fatal(err)
			}
			entrypoint := "/steamai control "
			if legacy {
				entrypoint = "/rekit control "
			}
			if !strings.HasPrefix(preview.ApplyCommand, entrypoint) {
				t.Fatalf("control command did not use selected public entrypoint: %s", preview.ApplyCommand)
			}
			applied, err := Apply(caseRoot, Options{Lane: testLane, Action: ActionPause, Actor: "main-agent", Reason: "selected root only", PublicationStamp: preview.PublicationStamp, ExpectedPlanSHA256: preview.ExpectedPlanSHA256})
			if err != nil || !applied.Applied {
				t.Fatalf("apply selected-root control: result=%+v err=%v", applied, err)
			}
			selectedReceipt := filepath.Join(caseRoot, selected, "lanes", testLane, controlDir, "00000000000000000001.json")
			if _, err := os.Stat(selectedReceipt); err != nil {
				t.Fatalf("selected state root lacks receipt: %v", err)
			}
			if _, err := os.Lstat(filepath.Join(caseRoot, other)); !os.IsNotExist(err) {
				t.Fatalf("control wrote the non-selected state root: %v", err)
			}
		})
	}
}

func TestControlDualRootFailsClosedWithoutWrites(t *testing.T) {
	caseRoot := controlFixture(t, false)
	if err := os.Mkdir(filepath.Join(caseRoot, ".rekit"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := Preview(caseRoot, Options{Lane: testLane, Action: ActionPause, Actor: "main-agent", Reason: "must fail closed", PublicationStamp: "2026-08-18T12:00:00Z"})
	if err == nil || !strings.Contains(err.Error(), "both .steamai and .rekit") {
		t.Fatalf("dual state root was accepted: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, ".steamai", "lanes", testLane, controlDir)); !os.IsNotExist(err) {
		t.Fatalf("dual-root preview wrote state: %v", err)
	}
}

func TestInspectSurvivesWholeProjectMove(t *testing.T) {
	parent := t.TempDir()
	caseRoot := filepath.Join(parent, "before")
	controlFixtureAt(t, caseRoot, false)
	applyControl(t, caseRoot, ActionPause, "pause before move", "2026-08-18T12:00:00Z", 1, StatePaused)
	moved := filepath.Join(parent, "after")
	if err := os.Rename(caseRoot, moved); err != nil {
		t.Fatal(err)
	}
	inspection, err := Inspect(moved, testLane)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.State != StatePaused || inspection.CurrentGeneration != 1 || inspection.Pending {
		t.Fatalf("moved control chain lost current state: %+v", inspection)
	}
}

func applyControl(t *testing.T, caseRoot, action, reason, stamp string, generation int, state string) Plan {
	t.Helper()
	preview, err := Preview(caseRoot, Options{Lane: testLane, Action: action, Actor: "main-agent", Reason: reason, PublicationStamp: stamp})
	if err != nil {
		t.Fatal(err)
	}
	applied, err := Apply(caseRoot, Options{
		Lane: preview.Lane, Action: preview.Action, Actor: preview.Actor, Reason: preview.Reason,
		PublicationStamp: preview.PublicationStamp, ExpectedPlanSHA256: preview.ExpectedPlanSHA256,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.AlreadyApplied || applied.ControlGeneration != generation || applied.State != state {
		t.Fatalf("unexpected applied control plan: %+v", applied)
	}
	return applied
}

func controlFixture(t *testing.T, legacy bool) string {
	t.Helper()
	caseRoot := filepath.Join(t.TempDir(), "case")
	controlFixtureAt(t, caseRoot, legacy)
	return caseRoot
}

func controlFixtureAt(t *testing.T, caseRoot string, legacy bool) {
	t.Helper()
	layout := testfixture.CurrentProject
	if legacy {
		layout = testfixture.LegacyCase
	}
	project := testfixture.NewProject(t, testfixture.ProjectOptions{
		Layout:      layout,
		CaseRoot:    caseRoot,
		Pack:        "_template",
		ProjectName: "execution-control-test",
	})
	laneRoot := filepath.Join(project.StateRoot, "lanes", testLane)
	if err := os.MkdirAll(laneRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	lane := `{"schemaVersion":1,"id":"` + testLane + `","status":"open","currentExecutor":"member-main","executorGeneration":7}` + "\n"
	if err := os.WriteFile(filepath.Join(laneRoot, "lane.json"), []byte(lane), 0o600); err != nil {
		t.Fatal(err)
	}
}

func snapshotControlTree(t *testing.T, caseRoot string) string {
	t.Helper()
	stateRoot := ".steamai"
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit")); err == nil {
		stateRoot = ".rekit"
	}
	root := filepath.Join(caseRoot, stateRoot, "lanes", testLane, controlDir)
	var out strings.Builder
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out.WriteString(filepath.ToSlash(rel))
		out.WriteByte('\n')
		out.Write(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out.String()
}
