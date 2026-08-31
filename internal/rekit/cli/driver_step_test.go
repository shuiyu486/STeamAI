package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/plancontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
	"github.com/shuiyu486/re-context-kits/internal/rekit/testfixture"
)

func TestParseProjectVisibleInvocationSupportsCurrentAndLegacy(t *testing.T) {
	for _, command := range []string{
		`/steamai start -Target "C:\case root" -Name triage -WhatIf -Format json`,
		`/rekit start -Target "C:\case root" -Name triage -WhatIf -Format json`,
	} {
		invocation, err := commands.ParsePublicInvocation(command)
		if err != nil {
			t.Fatalf("parse %q: %v", command, err)
		}
		if invocation.Command != commands.Start || len(invocation.Arguments) != 7 || invocation.Arguments[1] != `C:\case root` {
			t.Fatalf("unexpected typed invocation for %q: %+v", command, invocation)
		}
	}
}

func TestParseProjectVisibleInvocationRejectsUnsafeCommands(t *testing.T) {
	for _, command := range []string{
		`/other status -Format json`,
		`/steamai`,
		`/steamai unknown -Format json`,
		`/steamai start main -Lane other -WhatIf -Format json`,
		`/steamai status "unterminated`,
		`/steamai status -Command start`,
	} {
		if invocation, err := commands.ParsePublicInvocation(command); err == nil {
			t.Fatalf("unsafe command parsed as %+v: %s", invocation, command)
		}
	}
}

func TestProjectVisibleCommandUsesResolvedEntrypointAndRejectsMixedRoots(t *testing.T) {
	for _, fixture := range []struct {
		name       string
		stateDir   string
		entrypoint string
	}{
		{name: "current", stateDir: projectstate.CurrentDir, entrypoint: "/steamai"},
		{name: "legacy", stateDir: projectstate.LegacyDir, entrypoint: "/rekit"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			caseRoot := t.TempDir()
			if err := os.MkdirAll(filepath.Join(caseRoot, fixture.stateDir), 0o755); err != nil {
				t.Fatal(err)
			}
			command, err := projectVisibleCommand(caseRoot, `/rekit overview -Format text`)
			if err != nil {
				t.Fatal(err)
			}
			if command != fixture.entrypoint+` overview -Format text` {
				t.Fatalf("command=%q", command)
			}
		})
	}
	caseRoot := t.TempDir()
	for _, stateDir := range []string{projectstate.CurrentDir, projectstate.LegacyDir} {
		if err := os.MkdirAll(filepath.Join(caseRoot, stateDir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := projectVisibleCommand(caseRoot, `/rekit status -Format compact-json`); err == nil {
		t.Fatal("mixed state roots must fail closed")
	}
}

func TestCloneStatusMissionCommanderDriverRequestPreservesExactIdentity(t *testing.T) {
	invocation, err := commands.NewPublicInvocation(
		commands.Status,
		"-Format",
		"compact-json",
	)
	if err != nil {
		t.Fatal(err)
	}
	request := mission.MissionCommanderDriverRequest{
		Kind:              "execute-command",
		RunLoopStepID:     "apply-or-run-current",
		Invocation:        &invocation,
		Command:           "/rekit status -Format compact-json",
		CommandExecutable: true,
		ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
			Command:              "/rekit status -Format compact-json",
			RefreshStatusCommand: "/rekit status -Format compact-json",
			Boundary:             []string{"receipt-a", "receipt-a"},
		},
		Boundary: []string{"request-a", "request-a"},
	}
	beforeSHA, err := mission.MissionCommanderDriverRequestSHA256(request)
	if err != nil {
		t.Fatal(err)
	}
	clone := cloneStatusMissionCommanderDriverRequest(&request)
	afterSHA, err := mission.MissionCommanderDriverRequestSHA256(*clone)
	if err != nil {
		t.Fatal(err)
	}
	if beforeSHA != afterSHA {
		t.Fatalf("clone changed exact request identity: before=%s after=%s", beforeSHA, afterSHA)
	}
	if clone.Invocation == request.Invocation {
		t.Fatal("clone reused typed invocation pointer")
	}
	clone.Invocation.Arguments[0] = "changed"
	clone.Boundary[0] = "changed"
	clone.ExpectedReceipt.Boundary[0] = "changed"
	if request.Invocation.Arguments[0] != "-Format" ||
		request.Boundary[0] != "request-a" ||
		request.ExpectedReceipt.Boundary[0] != "receipt-a" {
		t.Fatal("clone mutation changed source request")
	}
}

func TestProjectVisibleMissionCommanderActionsPreserveGuidanceAndRejectUnknownSlashEntrypoints(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(caseRoot, projectstate.CurrentDir), 0o755); err != nil {
		t.Fatal(err)
	}
	invocation, err := commands.NewPublicInvocation(
		commands.Status,
		"-Format",
		"compact-json",
	)
	if err != nil {
		t.Fatal(err)
	}
	actions := []mission.MissionCommanderNextActionItem{
		{
			State:      "blocked",
			Command:    "match-authorized-gate-event",
			Source:     "test-guidance",
			Invocation: &invocation,
		},
		{
			State:   "ready",
			Command: `/rekit status -Format compact-json`,
			Source:  "test-command",
		},
	}
	visible, err := projectVisibleMissionCommanderActions(caseRoot, actions)
	if err != nil {
		t.Fatal(err)
	}
	if visible[0].Command != actions[0].Command {
		t.Fatalf("guidance command changed: %q", visible[0].Command)
	}
	if visible[1].Command != `/steamai status -Format compact-json` {
		t.Fatalf("project command=%q", visible[1].Command)
	}
	if visible[0].Invocation == actions[0].Invocation {
		t.Fatal("typed invocation was not deep-cloned")
	}
	visible[0].Invocation.Arguments[0] = "changed"
	if actions[0].Invocation.Arguments[0] != "-Format" {
		t.Fatal("projected action mutated the source typed invocation")
	}
	if _, err := projectVisibleMissionCommanderActions(
		caseRoot,
		[]mission.MissionCommanderNextActionItem{{
			State:   "blocked",
			Command: `/other status -Format compact-json`,
			Source:  "test-unknown-command",
		}},
	); err == nil {
		t.Fatal("unknown slash entrypoint must fail closed")
	}
}

func TestDriverStepRefreshCommandMatchesCurrentAndLegacyEntrypoints(t *testing.T) {
	for _, fixture := range []struct {
		name       string
		stateDir   string
		entrypoint string
		other      string
	}{
		{name: "current", stateDir: projectstate.CurrentDir, entrypoint: "/steamai", other: "/rekit"},
		{name: "legacy", stateDir: projectstate.LegacyDir, entrypoint: "/rekit", other: "/steamai"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			caseRoot := t.TempDir()
			if err := os.MkdirAll(filepath.Join(caseRoot, fixture.stateDir), 0o755); err != nil {
				t.Fatal(err)
			}
			ctx := runtime.Context{Target: caseRoot}
			valid := fixture.entrypoint + ` status -Target "` + caseRoot + `" -Format compact-json`
			if !driverStepRefreshCommandMatches(ctx, valid) {
				t.Fatalf("valid compact refresh command rejected: %s", valid)
			}
			for _, command := range []string{
				fixture.entrypoint + ` status -Target "` + caseRoot + `" -Format json`,
				fixture.other + ` status -Target "` + caseRoot + `" -Format compact-json`,
				`/other status -Target "` + caseRoot + `" -Format compact-json`,
				fixture.entrypoint + ` status -Target "` + filepath.Join(caseRoot, "other") + `" -Format compact-json`,
			} {
				if driverStepRefreshCommandMatches(ctx, command) {
					t.Fatalf("invalid refresh command accepted: %s", command)
				}
			}
		})
	}
}

func TestBoundedDriverRequestRequiresCanonicalProjectEntrypoint(t *testing.T) {
	for _, fixture := range []struct {
		name       string
		stateDir   string
		entrypoint string
		other      string
	}{
		{name: "current", stateDir: projectstate.CurrentDir, entrypoint: "/steamai", other: "/rekit"},
		{name: "legacy", stateDir: projectstate.LegacyDir, entrypoint: "/rekit", other: "/steamai"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			caseRoot := t.TempDir()
			if err := os.MkdirAll(filepath.Join(caseRoot, fixture.stateDir), 0o755); err != nil {
				t.Fatal(err)
			}
			ctx := runtime.Context{Target: caseRoot, Pack: "_template"}
			command := fixture.entrypoint + ` start -Target "` + caseRoot + `" -Name triage -WhatIf -Format json`
			request := missionDriverRequestForTest(command)
			if _, err := parseBoundedDriverRequest(ctx, request, false); err != nil {
				t.Fatalf("canonical request rejected: %v", err)
			}
			wrong := fixture.other + strings.TrimPrefix(command, fixture.entrypoint)
			if _, err := parseBoundedDriverRequest(ctx, missionDriverRequestForTest(wrong), false); err == nil || !strings.Contains(err.Error(), "canonical project entrypoint") {
				t.Fatalf("non-canonical request was not rejected: %v", err)
			}
		})
	}
}

func TestQualifyDriverStepApplyRequestRequiresContinuePreviewOwnedApply(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	ctx := runtime.Context{Target: caseRoot, Pack: "_template"}
	preview := missionDriverRequestForTest(`/rekit continue -Lane main -WhatIf -Format json`)
	if _, err := qualifyDriverStepApplyRequest(ctx, preview); err == nil || !strings.Contains(err.Error(), "must come from the exact workstream preview") {
		t.Fatalf("runner synthesized continue Apply without workstream plan binding: %v", err)
	}
	apply := missionDriverRequestForTest(`/rekit continue -Lane main -Apply -Format json -ExpectedContinuePlanSha256 ` + strings.Repeat("a", 64))
	qualified, err := qualifyDriverStepApplyRequest(ctx, apply)
	if err != nil {
		t.Fatal(err)
	}
	if qualified.Invocation == nil || !qualified.Invocation.HasFlag("-Apply") || qualified.Invocation.HasFlag("-WhatIf") || qualified.Invocation.HasFlag("--what-if") || qualified.ExpectedReceipt.Command != qualified.Command {
		t.Fatalf("preview-owned Apply request drifted: %+v", qualified)
	}
	if _, err := parseBoundedDriverRequest(ctx, qualified, true); err != nil {
		t.Fatalf("preview-owned Apply request is not executable as emitted: %v", err)
	}
}

func TestMissionControlTypedSeamMatchesPublicStatusAndRejectsModifiedPreview(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}

	snapshot, err := ReadMissionSnapshot(MissionSnapshotOptions{
		CaseRoot:            caseRoot,
		Pack:                "_template",
		SelectedCurrentLane: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status statusInventory
	decodeJSONStrict(t, out.Bytes(), &status)
	if snapshot.MissionControl == nil || status.MissionControlRunbook == nil ||
		snapshot.MissionControl.CurrentDriverRequestSHA256 != status.MissionControlRunbook.CurrentDriverRequestSHA256 ||
		snapshot.MissionControl.CurrentDriverRequest == nil || status.MissionControlRunbook.CurrentDriverRequest == nil {
		t.Fatalf("typed snapshot omitted public status identity: snapshot=%+v status=%+v", snapshot, status.MissionControlRunbook)
	}
	typedRequest, err := json.Marshal(snapshot.MissionControl.CurrentDriverRequest)
	if err != nil {
		t.Fatal(err)
	}
	publicRequest, err := json.Marshal(status.MissionControlRunbook.CurrentDriverRequest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(typedRequest, publicRequest) {
		t.Fatalf("typed current request differs from public status: typed=%s public=%s", typedRequest, publicRequest)
	}

	preview, err := PreviewDriverStep(DriverStepPreviewOptions{
		CaseRoot:            caseRoot,
		Pack:                "_template",
		SelectedCurrentLane: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	beforeApply := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	preview.ApplyDriverRequest.Command += " "
	if _, err := ApplyDriverStep(preview, DriverStepApplyOptions{}); err == nil || !strings.Contains(err.Error(), "identity was modified") {
		t.Fatalf("modified exported preview identity did not fail closed: %v", err)
	}
	assertSnapshotEqual(t, beforeApply, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))
}

func TestMissionControlTypedSeamReusesHeldProjectLease(t *testing.T) {
	project := testfixture.NewProject(t, testfixture.ProjectOptions{
		Layout:      testfixture.CurrentProject,
		SourceRepo:  repoRoot(t),
		Pack:        "_template",
		ProjectName: "typed-seam-lease",
		Components: testfixture.Components{
			InitialState: true,
		},
	})
	lease, err := projectexecution.AcquireShared(project.CaseRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Unlock()

	snapshot, err := ReadMissionSnapshot(MissionSnapshotOptions{
		CaseRoot:              project.CaseRoot,
		Pack:                  "_template",
		ProjectExecutionLease: lease,
	})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.CaseRoot != project.CaseRoot || snapshot.MissionControl == nil {
		t.Fatalf("held-lease typed snapshot = %+v", snapshot)
	}
	other := testfixture.NewProject(t, testfixture.ProjectOptions{
		Layout:      testfixture.CurrentProject,
		SourceRepo:  repoRoot(t),
		Pack:        "_template",
		ProjectName: "typed-seam-other",
	})
	if _, err := ReadMissionSnapshot(MissionSnapshotOptions{
		CaseRoot:              other.CaseRoot,
		Pack:                  "_template",
		ProjectExecutionLease: lease,
	}); err == nil || !strings.Contains(err.Error(), "lease target changed") {
		t.Fatalf("wrong-target held lease did not fail closed: %v", err)
	}
}

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
	if preview.Command != "run-driver-step" || preview.IsMutation || preview.Applied || !preview.ReviewRequired || !preview.RequiresConfirmation || preview.CurrentDriverRequest.Kind != "preview-command" || !strings.Contains(preview.CurrentDriverRequest.Command, "/rekit continue -Target ") || !strings.Contains(preview.ApplyDriverRequest.Command, "/rekit continue -Lane main") || !strings.Contains(preview.ApplyDriverRequest.Command, "-Apply") || !strings.Contains(preview.ApplyDriverRequest.Command, "-Target ") || len(preview.ExpectedDriverStepPlanSHA256) != 64 {
		t.Fatalf("unexpected driver step preview: %+v", preview)
	}
	if err := mission.ValidateMissionCommanderDriverRequest(preview.CurrentDriverRequest); err != nil {
		t.Fatalf("current driver request violates the typed invariant: %v", err)
	}
	if err := mission.ValidateMissionCommanderDriverRequest(preview.ApplyDriverRequest); err != nil {
		t.Fatalf("qualified Apply request violates the typed invariant: %v", err)
	}
	if preview.ApplyDriverRequest.ExpectedReceipt.Command != preview.ApplyDriverRequest.Command {
		t.Fatalf("qualified Apply request expected receipt drifted: %+v", preview.ApplyDriverRequest)
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
	failure, typed := plancontract.FromError(err)
	if err == nil || !typed || failure.Code != plancontract.CodePlanMismatch || failure.MutationApplied || failure.MutationBoundary != "none" {
		t.Fatalf("in-lock preview drift should fail closed: err=%v failure=%+v typed=%t output=%s", err, failure, typed, out.String())
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

func TestRunDriverStepSupportsExactSelectedLane(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatalf("preview exact selected lane: %v\n%s", err, out.String())
	}
	var preview driverStepPlan
	decodeJSONStrict(t, out.Bytes(), &preview)
	if preview.CurrentDriverRequest.Lane != "main" || !strings.Contains(preview.CurrentDriverRequest.Command, "-Lane main") {
		t.Fatalf("selected lane was not preserved in the driver request: %+v", preview.CurrentDriverRequest)
	}
	out.Reset()
	if err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-ExpectedDriverStepPlanSha256", preview.ExpectedDriverStepPlanSHA256, "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatalf("apply exact selected lane: %v\n%s", err, out.String())
	}
	var applied driverStepPlan
	decodeJSONStrict(t, out.Bytes(), &applied)
	if !applied.Applied || applied.Receipt == nil || applied.Receipt.CommandResultCommand != "continue" {
		t.Fatalf("selected-lane driver step did not apply the exact current request: %+v", applied)
	}

	out.Reset()
	err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-Lane", "other", "-WhatIf", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "selected current lane") {
		t.Fatalf("non-current selected lane should fail closed: err=%v output=%s", err, out.String())
	}
}

func TestRunDriverStepRejectsUnsupportedOuterArguments(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	tests := []struct {
		args      []string
		wantError string
	}{
		{
			args:      []string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-Actor", "other", "-WhatIf", "-Format", "json"},
			wantError: "run-driver-step contains unsupported flag",
		},
		{
			args:      []string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-Unexpected", "value", "-WhatIf", "-Format", "json"},
			wantError: "unknown argument: -Unexpected",
		},
		{
			args:      []string{"-Command", "run-driver-step", "--", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"},
			wantError: "accepts -- only once",
		},
		{
			args:      []string{"--", "--", "-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"},
			wantError: "accepts -- only once",
		},
	}
	for _, test := range tests {
		var out bytes.Buffer
		err := Run(test.args, &out)
		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Fatalf("outer args should fail closed with %q: args=%v err=%v output=%s", test.wantError, test.args, err, out.String())
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
		{name: "duplicate start selector", command: `/rekit start -Target "` + caseRoot + `" triage -Name triage -Lane feature-triage -WhatIf -Format json`, want: "both positional and -Lane selectors"},
		{name: "start actor without executor", command: `/rekit start -Target "` + caseRoot + `" -Name triage -Actor other -WhatIf -Format json`, want: "outside its bounded contract"},
		{name: "start reason without executor", command: `/rekit start -Target "` + caseRoot + `" -Name triage -Reason other -WhatIf -Format json`, want: "outside its bounded contract"},
		{name: "duplicate selector", command: `/rekit continue -Target "` + caseRoot + `" main -Lane other -WhatIf -Format json`, want: "both positional and -Lane selectors"},
		{name: "duplicate matching selector", command: `/rekit complete -Target "` + caseRoot + `" main -Lane main -Actor main-agent -Reason done -EvidenceRefs evidence.md -WhatIf -Format json`, want: "both positional and -Lane selectors"},
		{name: "duplicate phase", command: `/rekit continue -Target "` + caseRoot + `" main -WhatIf -WhatIf -Format json`, want: "repeats flag"},
	} {
		t.Run(test.name, func(t *testing.T) {
			invocation, parseErr := commands.ParsePublicInvocation(test.command)
			if parseErr != nil {
				if !strings.Contains(parseErr.Error(), test.want) {
					t.Fatalf("typed request should fail closed with %q: err=%v", test.want, parseErr)
				}
				return
			}
			request := mission.MissionCommanderDriverRequest{
				Kind:              "preview-command",
				RunLoopStepID:     "preview-current",
				Invocation:        &invocation,
				Command:           test.command,
				CommandExecutable: true,
				RequiresReview:    true,
				ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
					Command: test.command,
				},
			}
			_, err := parseBoundedDriverRequest(ctx, request, false)
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
	if err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-Lane", "feature-triage", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatalf("preview current continue step: %v\n%s", err, out.String())
	}
	var continuePreview driverStepPlan
	decodeJSONStrict(t, out.Bytes(), &continuePreview)
	if driverStepCommandName(continuePreview.CurrentDriverRequest.Command) != "continue" {
		t.Fatalf("start refresh did not focus continue: %+v", continuePreview.CurrentDriverRequest)
	}
	out.Reset()
	if err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-Lane", "feature-triage", "-ExpectedDriverStepPlanSha256", continuePreview.ExpectedDriverStepPlanSHA256, "-Apply", "-Format", "json"}, &out); err != nil {
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
	if err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-Lane", "feature-triage", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatalf("preview current reconcile step: %v\n%s", err, out.String())
	}
	var reconcilePreview driverStepPlan
	decodeJSONStrict(t, out.Bytes(), &reconcilePreview)
	if driverStepCommandName(reconcilePreview.CurrentDriverRequest.Command) != "reconcile" || driverStepCommandName(reconcilePreview.ApplyDriverRequest.Command) != "reconcile" || !strings.Contains(reconcilePreview.ApplyDriverRequest.Command, "int-driver-loop-1") {
		t.Fatalf("unexpected reconcile driver preview: %+v", reconcilePreview)
	}
	assertSnapshotEqual(t, beforeReconcile, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))

	out.Reset()
	if err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-Lane", "feature-triage", "-ExpectedDriverStepPlanSha256", reconcilePreview.ExpectedDriverStepPlanSHA256, "-Apply", "-Format", "json"}, &out); err != nil {
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
		err := Run([]string{"-Command", "run-driver-step", "-Target", caseRoot, "-Pack", "_template", "-Lane", "feature-triage", "-ExpectedDriverStepPlanSha256", preview.ExpectedDriverStepPlanSHA256, "-Apply", "-Format", "json"}, &out)
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
	invocation, err := commands.ParsePublicInvocation(command)
	if err != nil {
		panic(err)
	}
	return mission.MissionCommanderDriverRequest{
		Kind:              "preview-command",
		RunLoopStepID:     "preview-current",
		Invocation:        &invocation,
		Command:           command,
		CommandExecutable: true,
		RequiresReview:    true,
		ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
			Command: command,
		},
	}
}

func runtimeContextForDriverStepTest(caseRoot string) (runtime.Context, error) {
	return runtime.New(caseRoot, "_template")
}
