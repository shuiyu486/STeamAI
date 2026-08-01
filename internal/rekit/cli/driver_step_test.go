package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

func TestRunDriverStepContinueProductPath(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatalf("initialize case mission board: %v\n%s", err, out.String())
	}

	before := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	out.Reset()
	if err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatalf("preview current driver step: %v\n%s", err, out.String())
	}
	var preview driverStepPlan
	decodeJSONStrict(t, out.Bytes(), &preview)
	if preview.Command != "run-driver-step" || preview.IsMutation || preview.Applied || !preview.ReviewRequired || !preview.RequiresConfirmation || preview.CurrentDriverRequest.Kind != "preview-command" || !strings.Contains(preview.CurrentDriverRequest.Command, "/rekit continue -Target ") || !strings.Contains(preview.ApplyDriverRequest.Command, "/rekit continue main") || !strings.Contains(preview.ApplyDriverRequest.Command, "-Apply") || !strings.Contains(preview.ApplyDriverRequest.Command, "-Target ") || len(preview.ExpectedDriverStepPlanSHA256) != 64 {
		t.Fatalf("unexpected driver step preview: %+v", preview)
	}
	assertSnapshotEqual(t, before, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))

	out.Reset()
	err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-ExpectedDriverStepPlanSha256", strings.Repeat("0", 64), "-Apply", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "expected plan sha256 mismatch") {
		t.Fatalf("stale driver step hash should fail closed: err=%v output=%s", err, out.String())
	}
	assertSnapshotEqual(t, before, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))

	out.Reset()
	if err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-ExpectedDriverStepPlanSha256", preview.ExpectedDriverStepPlanSHA256, "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatalf("apply current driver step: %v\n%s", err, out.String())
	}
	var applied driverStepPlan
	decodeJSONStrict(t, out.Bytes(), &applied)
	if !applied.IsMutation || !applied.Applied || applied.ReviewRequired || applied.RequiresConfirmation || applied.Receipt == nil || applied.Receipt.State != "refreshed" || applied.Receipt.Outcome != "applied" || !applied.Receipt.ExpectedReceiptCommandMatched || !applied.Receipt.RefreshStatusCommandMatched || applied.Receipt.RefreshedCurrentDriverRequest == nil || applied.RefreshedStatus == nil || applied.RefreshedStatus.MissionControlRunbook == nil || applied.RefreshedStatus.MissionControlRunbook.CurrentDriverRequest == nil || applied.Receipt.RefreshedCurrentDriverRequest.Command != applied.RefreshedStatus.MissionControlRunbook.CurrentDriverRequest.Command {
		t.Fatalf("unexpected applied driver step receipt: %+v", applied)
	}
	if applied.Receipt.ExecutedCommand != preview.ApplyDriverRequest.Command || applied.Receipt.RequestedCommand != preview.CurrentDriverRequest.Command || applied.Receipt.CommandResultCommand != "continue" {
		t.Fatalf("driver step receipt identity drifted: preview=%+v receipt=%+v", preview, applied.Receipt)
	}
	assertFileNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertFileNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))
}

func TestRunDriverStepRejectsStalePreviewInput(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var preview driverStepPlan
	decodeJSONStrict(t, out.Bytes(), &preview)
	writeCaseFile(t, caseRoot, ".rekit/lanes/main/outbox.jsonl", `{"eventId":"evt-driver-step-stale","kind":"observation","subject":"stale preview input","summary":"added after runner preview","evidence":"evt-driver-step-stale"}`+"\n")
	beforeApply := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	out.Reset()
	err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-ExpectedDriverStepPlanSha256", preview.ExpectedDriverStepPlanSHA256, "-Apply", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "expected plan sha256 mismatch") {
		t.Fatalf("changed continue input should invalidate the reviewed driver step plan: err=%v output=%s", err, out.String())
	}
	assertSnapshotEqual(t, beforeApply, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))
}

func TestRunDriverStepRevalidatesPreviewInsideMutationLease(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var preview driverStepPlan
	decodeJSONStrict(t, out.Bytes(), &preview)
	driverStepApplyBeforeMutationHook = func(command string) error {
		if command != "continue" {
			t.Fatalf("unexpected mutation command: %s", command)
		}
		writeCaseFile(t, caseRoot, ".rekit/lanes/main/outbox.jsonl", `{"eventId":"evt-driver-step-toctou","kind":"observation","subject":"changed before continue lock","summary":"must invalidate in-lock preview","evidence":"evt-driver-step-toctou"}`+"\n")
		return nil
	}
	t.Cleanup(func() { driverStepApplyBeforeMutationHook = nil })
	out.Reset()
	err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-ExpectedDriverStepPlanSha256", preview.ExpectedDriverStepPlanSHA256, "-Apply", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "continue preview sha256 mismatch") {
		t.Fatalf("in-lock preview drift should fail closed: err=%v output=%s", err, out.String())
	}
	observations, readErr := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if strings.Contains(string(observations), "evt-driver-step-toctou") {
		t.Fatalf("in-lock stale event was written to observations ledger: %s", observations)
	}
	assertFileNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertFileNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))
}

func TestRunDriverStepRevalidatesStartAndReconcileInsideMutationLease(t *testing.T) {
	t.Run("start", func(t *testing.T) {
		caseRoot := attachedCase(t)
		seedEmptyLaneCaseBoard(t, caseRoot)
		var out bytes.Buffer
		if err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, &out); err != nil {
			t.Fatal(err)
		}
		var preview driverStepPlan
		decodeJSONStrict(t, out.Bytes(), &preview)
		driverStepApplyBeforeMutationHook = func(command string) error {
			if command != "start" {
				t.Fatalf("unexpected mutation command: %s", command)
			}
			var nested bytes.Buffer
			return Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "triage", "-Apply", "-Format", "json"}, &nested)
		}
		t.Cleanup(func() { driverStepApplyBeforeMutationHook = nil })
		out.Reset()
		err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-ExpectedDriverStepPlanSha256", preview.ExpectedDriverStepPlanSHA256, "-Apply", "-Format", "json"}, &out)
		if err == nil || !strings.Contains(err.Error(), "start preview sha256 mismatch") {
			t.Fatalf("in-lock start preview drift should fail closed: err=%v output=%s", err, out.String())
		}
		laneEventsPath := filepath.Join(caseRoot, ".rekit", "lanes", "feature-triage", "events.jsonl")
		laneEvents, readErr := os.ReadFile(laneEventsPath)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if lines := strings.Count(strings.TrimSpace(string(laneEvents)), "\n") + 1; lines != 1 {
			t.Fatalf("stale runner appended another start event: %s", laneEvents)
		}
	})

	t.Run("reconcile", func(t *testing.T) {
		caseRoot := fullAttachedCase(t)
		var out bytes.Buffer
		if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
			t.Fatal(err)
		}
		writeCaseFile(t, caseRoot, ".rekit/facts/interventions.jsonl", `{"kind":"intervention","eventId":"int-reconcile-toctou","lane":"main","subject":"reconcile drift","summary":"must invalidate reconcile preview","action":"override","target":"batch-toctou","approvedBy":"lead","scope":"metadata","status":"open","batchId":"batch-toctou"}`+"\n")
		out.Reset()
		if err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, &out); err != nil {
			t.Fatal(err)
		}
		var preview driverStepPlan
		decodeJSONStrict(t, out.Bytes(), &preview)
		if command := driverStepCommandName(preview.CurrentDriverRequest.Command); command != "reconcile" {
			t.Fatalf("unexpected preview command: %s", command)
		}
		driverStepApplyBeforeMutationHook = func(command string) error {
			if command != "reconcile" {
				t.Fatalf("unexpected mutation command: %s", command)
			}
			var nested bytes.Buffer
			return Run([]string{"-Command", "reconcile", "-Target", caseRoot, "-Pack", "_template", "main", "-InterventionId", "int-reconcile-toctou", "-Apply", "-Format", "json"}, &nested)
		}
		t.Cleanup(func() { driverStepApplyBeforeMutationHook = nil })
		out.Reset()
		err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-ExpectedDriverStepPlanSha256", preview.ExpectedDriverStepPlanSHA256, "-Apply", "-Format", "json"}, &out)
		if err == nil || (!strings.Contains(err.Error(), "reconcile preview sha256 mismatch") && !strings.Contains(err.Error(), "not an effective open intervention")) {
			t.Fatalf("in-lock reconcile preview drift should fail closed: err=%v output=%s", err, out.String())
		}
		interventions, readErr := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "interventions.jsonl"))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if strings.Count(string(interventions), `"resolvesEventId":"int-reconcile-toctou"`) != 1 {
			t.Fatalf("stale runner appended another reconcile resolution: %s", interventions)
		}
	})
}

func TestRunDriverStepUsesValidatedImmutableInputSnapshot(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var preview driverStepPlan
	decodeJSONStrict(t, out.Bytes(), &preview)
	driverStepAfterPreviewValidationHook = func() error {
		writeCaseFile(t, caseRoot, ".rekit/lanes/main/outbox.jsonl", `{"eventId":"evt-after-preview-validation","kind":"observation","subject":"late executor output","summary":"must wait for next continue","evidence":"evt-after-preview-validation"}`+"\n")
		return nil
	}
	t.Cleanup(func() { driverStepAfterPreviewValidationHook = nil })
	out.Reset()
	if err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-ExpectedDriverStepPlanSha256", preview.ExpectedDriverStepPlanSHA256, "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatalf("apply validated snapshot: %v\n%s", err, out.String())
	}
	observations, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(observations), "evt-after-preview-validation") {
		t.Fatalf("late executor output was consumed by reviewed apply: %s", observations)
	}
}

func TestRunDriverStepSupportsLeadingGoRunSeparator(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"--", "-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatalf("leading go-run separator should be accepted: %v\n%s", err, out.String())
	}
	var preview driverStepPlan
	decodeJSONStrict(t, out.Bytes(), &preview)
	out.Reset()
	if err := Run([]string{"--", "-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-ExpectedDriverStepPlanSha256", preview.ExpectedDriverStepPlanSHA256, "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatalf("leading go-run separator apply should be accepted: %v\n%s", err, out.String())
	}
}

func TestRunDriverStepRejectsUnsupportedOuterArguments(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	for _, args := range [][]string{
		{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-Lane", "other", "-WhatIf", "-Format", "json"},
		{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-Actor", "other", "-WhatIf", "-Format", "json"},
		{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-Unexpected", "value", "-WhatIf", "-Format", "json"},
		{"-Command", "run-driver-step", "--", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"},
		{"--", "--", "-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"},
	} {
		var out bytes.Buffer
		err := Run(args, &out)
		if err == nil || (!strings.Contains(err.Error(), "run-driver-step contains unsupported flag") && !strings.Contains(err.Error(), "accepts -- only once")) {
			t.Fatalf("outer args should fail closed: args=%v err=%v output=%s", args, err, out.String())
		}
	}
}

func TestRunDriverStepRejectsOnboardingAndUnsupportedNestedRequests(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, &out)
	if err == nil || (!strings.Contains(err.Error(), "outside the run-driver-step allowlist") && !strings.Contains(err.Error(), "outside the review-first runner boundary")) {
		t.Fatalf("onboarding overview should stay outside runner MVP: err=%v output=%s", err, out.String())
	}

	ctx, err := runtimeContextForDriverStepTest(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name    string
		command string
		want    string
	}{
		{name: "gate", command: `/rekit gate -Target "` + caseRoot + `" -WhatIf -Format json`, want: "outside the run-driver-step allowlist"},
		{name: "handoff outside MVP", command: `/rekit handoff -Target "` + caseRoot + `" -WhatIf -Format json`, want: "outside the run-driver-step allowlist"},
		{name: "unknown flag", command: `/rekit continue -Target "` + caseRoot + `" main -WhatIf -Format json -Unexpected value`, want: "unsupported flag"},
		{name: "cross command actor", command: `/rekit continue -Target "` + caseRoot + `" main -Actor other -WhatIf -Format json`, want: "unsupported flag"},
		{name: "start lane alias", command: `/rekit start -Target "` + caseRoot + `" -Lane triage -WhatIf -Format json`, want: "outside its bounded contract"},
		{name: "start actor without executor", command: `/rekit start -Target "` + caseRoot + `" -Name triage -Actor other -WhatIf -Format json`, want: "outside its bounded contract"},
		{name: "start reason without executor", command: `/rekit start -Target "` + caseRoot + `" -Name triage -Reason other -WhatIf -Format json`, want: "outside its bounded contract"},
		{name: "duplicate selector", command: `/rekit continue -Target "` + caseRoot + `" main -Lane other -WhatIf -Format json`, want: "exactly one lane selector"},
		{name: "duplicate phase", command: `/rekit continue -Target "` + caseRoot + `" main -WhatIf -WhatIf -Format json`, want: "repeats flag"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseBoundedDriverRequest(ctx, missionDriverRequestForTest(test.command), false)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("request should fail closed with %q: err=%v", test.want, err)
			}
		})
	}
}

func TestRunDriverStepStartContinueReconcileProductPath(t *testing.T) {
	caseRoot := attachedCase(t)
	seedEmptyLaneCaseBoard(t, caseRoot)
	var out bytes.Buffer

	beforeStart := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	if err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatalf("preview current start step: %v\n%s", err, out.String())
	}
	var startPreview driverStepPlan
	decodeJSONStrict(t, out.Bytes(), &startPreview)
	if driverStepCommandName(startPreview.CurrentDriverRequest.Command) != "start" || driverStepCommandName(startPreview.ApplyDriverRequest.Command) != "start" || !strings.Contains(startPreview.ApplyDriverRequest.Command, "-Apply") {
		t.Fatalf("unexpected start driver preview: %+v", startPreview)
	}
	assertSnapshotEqual(t, beforeStart, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))

	out.Reset()
	if err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-ExpectedDriverStepPlanSha256", startPreview.ExpectedDriverStepPlanSHA256, "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatalf("apply current start step: %v\n%s", err, out.String())
	}
	var startApplied driverStepPlan
	decodeJSONStrict(t, out.Bytes(), &startApplied)
	if !startApplied.Applied || startApplied.Receipt == nil || startApplied.Receipt.CommandResultCommand != "start" || startApplied.Receipt.RefreshedCurrentDriverRequest == nil || driverStepCommandName(startApplied.Receipt.RefreshedCurrentDriverRequest.Command) != "continue" {
		t.Fatalf("start driver apply did not advance to continue: %+v", startApplied)
	}
	assertFileExists(t, filepath.Join(caseRoot, ".rekit", "lanes", "feature-triage", "lane.json"))

	out.Reset()
	if err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatalf("preview current continue step: %v\n%s", err, out.String())
	}
	var continuePreview driverStepPlan
	decodeJSONStrict(t, out.Bytes(), &continuePreview)
	if driverStepCommandName(continuePreview.CurrentDriverRequest.Command) != "continue" {
		t.Fatalf("start refresh did not focus continue: %+v", continuePreview.CurrentDriverRequest)
	}
	out.Reset()
	if err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-ExpectedDriverStepPlanSha256", continuePreview.ExpectedDriverStepPlanSHA256, "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatalf("apply current continue step: %v\n%s", err, out.String())
	}
	var continueApplied driverStepPlan
	decodeJSONStrict(t, out.Bytes(), &continueApplied)
	if !continueApplied.Applied || continueApplied.Receipt == nil || continueApplied.Receipt.CommandResultCommand != "continue" {
		t.Fatalf("continue driver apply failed: %+v", continueApplied)
	}

	interventionsPath := filepath.Join(caseRoot, ".rekit", "facts", "interventions.jsonl")
	interventionsFile, err := os.OpenFile(interventionsPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := interventionsFile.WriteString(`{"kind":"intervention","eventId":"int-driver-loop-1","lane":"feature-triage","subject":"redirect triage","summary":"operator changed the current direction","action":"override","target":"batch-driver-loop","approvedBy":"lead","scope":"metadata","status":"open","batchId":"batch-driver-loop"}` + "\n"); err != nil {
		_ = interventionsFile.Close()
		t.Fatal(err)
	}
	if err := interventionsFile.Close(); err != nil {
		t.Fatal(err)
	}

	beforeReconcile := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	out.Reset()
	if err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatalf("preview current reconcile step: %v\n%s", err, out.String())
	}
	var reconcilePreview driverStepPlan
	decodeJSONStrict(t, out.Bytes(), &reconcilePreview)
	if driverStepCommandName(reconcilePreview.CurrentDriverRequest.Command) != "reconcile" || driverStepCommandName(reconcilePreview.ApplyDriverRequest.Command) != "reconcile" || !strings.Contains(reconcilePreview.ApplyDriverRequest.Command, "int-driver-loop-1") {
		t.Fatalf("unexpected reconcile driver preview: %+v", reconcilePreview)
	}
	assertSnapshotEqual(t, beforeReconcile, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))

	out.Reset()
	if err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-ExpectedDriverStepPlanSha256", reconcilePreview.ExpectedDriverStepPlanSHA256, "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatalf("apply current reconcile step: %v\n%s", err, out.String())
	}
	var reconcileApplied driverStepPlan
	decodeJSONStrict(t, out.Bytes(), &reconcileApplied)
	if !reconcileApplied.Applied || reconcileApplied.Receipt == nil || reconcileApplied.Receipt.CommandResultCommand != "reconcile" || reconcileApplied.Receipt.RefreshedCurrentDriverRequest == nil || driverStepCommandName(reconcileApplied.Receipt.RefreshedCurrentDriverRequest.Command) != "continue" {
		t.Fatalf("reconcile driver apply did not restore continue: %+v", reconcileApplied)
	}
	assertFileNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertFileNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))
}

func TestRunDriverStepStartAndReconcileRejectStateDrift(t *testing.T) {
	t.Run("start", func(t *testing.T) {
		caseRoot := attachedCase(t)
		seedEmptyLaneCaseBoard(t, caseRoot)
		var out bytes.Buffer
		if err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, &out); err != nil {
			t.Fatal(err)
		}
		var preview driverStepPlan
		decodeJSONStrict(t, out.Bytes(), &preview)
		if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "triage", "-Apply", "-Format", "json"}, &out); err != nil {
			t.Fatal(err)
		}
		beforeApply := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
		out.Reset()
		err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-ExpectedDriverStepPlanSha256", preview.ExpectedDriverStepPlanSHA256, "-Apply", "-Format", "json"}, &out)
		if err == nil || !strings.Contains(err.Error(), "expected plan sha256 mismatch") {
			t.Fatalf("stale start plan should fail closed: err=%v output=%s", err, out.String())
		}
		assertSnapshotEqual(t, beforeApply, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))
	})

	t.Run("reconcile", func(t *testing.T) {
		caseRoot := fullAttachedCase(t)
		var out bytes.Buffer
		if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(caseRoot, ".rekit", "facts", "interventions.jsonl")
		file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.WriteString(`{"kind":"intervention","eventId":"int-driver-stale","lane":"main","subject":"stale reconcile","summary":"must invalidate","action":"override","target":"batch-stale","approvedBy":"lead","scope":"metadata","status":"open","batchId":"batch-stale"}` + "\n"); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		out.Reset()
		if err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, &out); err != nil {
			t.Fatal(err)
		}
		var preview driverStepPlan
		decodeJSONStrict(t, out.Bytes(), &preview)
		if err := Run([]string{"-Command", "reconcile", "-Target", caseRoot, "-Pack", "_template", "main", "-InterventionId", "int-driver-stale", "-Apply", "-Format", "json"}, &out); err != nil {
			t.Fatal(err)
		}
		beforeApply := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
		out.Reset()
		err = Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-ExpectedDriverStepPlanSha256", preview.ExpectedDriverStepPlanSHA256, "-Apply", "-Format", "json"}, &out)
		if err == nil || (!strings.Contains(err.Error(), "expected plan sha256 mismatch") && !strings.Contains(err.Error(), "not runnable")) {
			t.Fatalf("stale reconcile plan should fail closed: err=%v output=%s", err, out.String())
		}
		assertSnapshotEqual(t, beforeApply, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))
	})
}

func decodeJSONStrict(t *testing.T, data []byte, target any) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, string(data))
	}
}

func missionDriverRequestForTest(command string) mission.MissionCommanderDriverRequest {
	return mission.MissionCommanderDriverRequest{
		Kind:              "preview-command",
		RunLoopStepID:     "preview-current",
		Command:           command,
		CommandExecutable: true,
		RequiresReview:    true,
	}
}

func runtimeContextForDriverStepTest(caseRoot string) (runtime.Context, error) {
	return runtime.New(caseRoot, "_template")
}
