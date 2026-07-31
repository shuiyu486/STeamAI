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
	driverStepApplyBeforeContinueHook = func() error {
		writeCaseFile(t, caseRoot, ".rekit/lanes/main/outbox.jsonl", `{"eventId":"evt-driver-step-toctou","kind":"observation","subject":"changed before continue lock","summary":"must invalidate in-lock preview","evidence":"evt-driver-step-toctou"}`+"\n")
		return nil
	}
	t.Cleanup(func() { driverStepApplyBeforeContinueHook = nil })
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
