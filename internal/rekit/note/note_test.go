package note

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/capabilitycontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/testfixture"
)

func TestAppendAndListUseCurrentStateRoot(t *testing.T) {
	repoRoot, caseRoot, pack := noteFixtureWithStateRoot(t, projectstate.CurrentDir)
	result, err := Append(repoRoot, caseRoot, pack, Options{Kind: "observation", Lane: "main", Subject: "current root note", EventID: "evt-current-root"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Path != ".steamai/facts/observations.jsonl" {
		t.Fatalf("current note path = %q", result.Path)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, projectstate.CurrentDir, "facts", "observations.jsonl")); err != nil {
		t.Fatal(err)
	}
	assertNoteNotExists(t, filepath.Join(caseRoot, projectstate.LegacyDir, "facts", "observations.jsonl"))
	listed, err := ListEvents(repoRoot, caseRoot, pack, Options{Kind: "observation"})
	if err != nil {
		t.Fatal(err)
	}
	if listed.EventCount != 1 || stringValue(listed.Groups[0].Events[0], "eventId") != "evt-current-root" {
		t.Fatalf("current note list = %+v", listed)
	}
}

func TestAppendRejectsDualStateRoots(t *testing.T) {
	repoRoot, caseRoot, pack := noteFixtureWithStateRoot(t, projectstate.CurrentDir)
	if err := os.Mkdir(filepath.Join(caseRoot, projectstate.LegacyDir), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Append(repoRoot, caseRoot, pack, Options{Kind: "observation", Lane: "main", Subject: "dual root"}, true); err == nil || !strings.Contains(err.Error(), "must not coexist") {
		t.Fatalf("dual-root note error = %v", err)
	}
}

func TestAppendWhatIfDoesNotWrite(t *testing.T) {
	repoRoot, caseRoot, pack := noteFixture(t)

	result, err := Append(repoRoot, caseRoot, pack, Options{
		Kind:         "verification",
		Lane:         "main",
		Subject:      "candidate-alpha",
		Summary:      "reviewer accepted packet shard",
		Actor:        "reviewer-test",
		Target:       "candidate-alpha",
		Verifier:     "manual-review",
		Verdict:      "accepted",
		EvidenceRefs: "ev-alpha",
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Command != "note" || result.IsMutation || result.Applied || result.Reason != "what-if" || result.Path != ".rekit/facts/verifications.jsonl" || result.EventID == "" {
		t.Fatalf("unexpected note what-if result: %+v", result)
	}
	if len(result.RecordArgs) == 0 || result.RecordArgs[0] != "-Command" || result.RecordArgs[1] != "note" || result.RecordArgs[len(result.RecordArgs)-2] != "-Format" || result.RecordArgs[len(result.RecordArgs)-1] != "json" || !strings.Contains(result.RecordCommand, result.EventSHA256) {
		t.Fatalf("note what-if omitted the machine-consumable record route: %+v", result)
	}
	if result.ExecutorAction.Blocked || !result.ExecutorAction.Ready || result.WouldExecutorAction == nil || result.WouldExecutorAction.Blocked || !result.WouldExecutorAction.Ready {
		t.Fatalf("verification what-if should not change executor readiness: %+v", result)
	}
	if result.MissionCommanderAction.State != "ready-to-continue" || !hasNoteCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions", "/rekit continue main -WhatIf -Format json", false, true) || !hasNoteCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit handoff main -WhatIf -Format json", false, true) {
		t.Fatalf("current commander projection drifted: action=%+v next=%+v", result.MissionCommanderAction, result.MissionCommanderNextActions)
	}
	if result.WouldMissionCommanderAction == nil || result.WouldMissionCommanderAction.State != "ready-to-continue" || !hasNoteCommanderNextAction(result.WouldMissionCommanderNextActions, "missionCommanderActions", "/rekit continue main -WhatIf -Format json", false, true) || !hasNoteCommanderNextAction(result.WouldMissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit handoff main -WhatIf -Format json", false, true) {
		t.Fatalf("would commander projection drifted: action=%+v next=%+v", result.WouldMissionCommanderAction, result.WouldMissionCommanderNextActions)
	}
	assertNoteNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl"))
}

func TestAppendWhatIfPreservesExecutionControlBindingOnlyInRecordRoute(t *testing.T) {
	repoRoot, caseRoot, pack := noteFixture(t)
	capability, err := capabilitycontract.Bind(capabilitycontract.Transport())
	if err != nil {
		t.Fatal(err)
	}
	binding := executioncontrol.Binding{
		SchemaVersion: executioncontrol.BindingSchemaVersion,
		Lane:          "main",
		Owner: laneowner.Snapshot{
			Lane: "main", CurrentExecutor: "executor-a", ExecutorGeneration: 1,
		},
		Capability: capability,
	}
	result, err := Append(repoRoot, caseRoot, pack, Options{
		Kind: "verification", Lane: "main", Subject: "execution evidence review accepted",
		Verifier: "tool-review", Verdict: "accepted", Status: "resolved",
		ExpectedExecutionControlBinding: &binding,
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := result.Event["expectedExecutionControlBinding"]; exists {
		t.Fatalf("execution control binding leaked into ledger event: %+v", result.Event)
	}
	index := slices.Index(result.RecordArgs, "-ExpectedExecutionControlBindingJson")
	if index < 0 || index+1 >= len(result.RecordArgs) {
		t.Fatalf("record route omitted execution control binding: %+v", result.RecordArgs)
	}
	var replay executioncontrol.Binding
	if err := json.Unmarshal([]byte(result.RecordArgs[index+1]), &replay); err != nil {
		t.Fatal(err)
	}
	if !executioncontrol.SameBinding(&replay, &binding) || !strings.Contains(result.RecordCommand, "-ExpectedExecutionControlBindingJson") {
		t.Fatalf("record route binding = %+v command=%q", replay, result.RecordCommand)
	}
}

func TestAppendWhatIfOmitsRecordCommandForInternalFields(t *testing.T) {
	repoRoot, caseRoot, pack := noteFixture(t)

	result, err := Append(repoRoot, caseRoot, pack, Options{
		Kind:               "verification",
		Lane:               "main",
		Subject:            "reviewer intake",
		Summary:            "internal reviewer writeback",
		Verifier:           "manual-review",
		Verdict:            "accepted",
		PacketID:           "packet-1",
		RouteID:            "route-a",
		ShardID:            "shard-01",
		ReviewerSession:    "reviewer-session-1",
		ReviewerDecision:   "accept",
		RecommendedVerdict: "accepted",
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied || result.IsMutation || result.Reason != "what-if" || len(result.EventSHA256) != 64 {
		t.Fatalf("unexpected internal-field what-if result: %+v", result)
	}
	if result.RecordCommand != "" || len(result.RecordArgs) != 0 {
		t.Fatalf("internal reviewer fields should not expose a non-replayable record route: command=%q args=%+v", result.RecordCommand, result.RecordArgs)
	}
	if stringValue(result.Event, "packetId") != "packet-1" || stringValue(result.Event, "reviewerSession") != "reviewer-session-1" || stringValue(result.Event, "reviewerDecision") != "accept" {
		t.Fatalf("internal fields missing from event: %+v", result.Event)
	}
	assertNoteNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl"))
}

func TestAppendWhatIfReplaysReviewerBoundCorrectionFields(t *testing.T) {
	repoRoot, caseRoot, pack := noteFixture(t)

	result, err := Append(repoRoot, caseRoot, pack, Options{
		Kind:                      "intervention",
		Lane:                      "main",
		Subject:                   "daily human correction",
		Summary:                   "apply reviewer correction",
		Actor:                     "daily-actor",
		Action:                    "override",
		Status:                    "open",
		Target:                    ".rekit/member/manifest.json",
		PacketID:                  "packet-1",
		RouteID:                   "route-1",
		ShardID:                   "shard-1",
		PacketPath:                ".rekit/reviewer/packet.json",
		ReviewerResultPath:        ".rekit/reviewer/result.json",
		ReviewerSession:           "reviewer-session-1",
		ReviewerDispatchPath:      ".rekit/reviewer/dispatch.json",
		ReviewerDispatchSHA256:    strings.Repeat("a", 64),
		ReviewerCompletionPath:    ".rekit/reviewer/completion.json",
		ReviewerCompletionSHA256:  strings.Repeat("b", 64),
		ReviewerResultInputPath:   ".rekit/reviewer/input.json",
		ReviewerResultInputSHA256: strings.Repeat("c", 64),
		ReviewerResultInputBytes:  "123",
		ReviewerResultSHA256:      strings.Repeat("d", 64),
		ReviewerManifestSHA256:    strings.Repeat("e", 64),
		ReviewerVerificationID:    "verification-1",
		ReviewerDecisionID:        "decision-1",
		OwnerExecutor:             "executor-1",
		OwnerGeneration:           "2",
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Reason != "what-if" || len(result.RecordArgs) == 0 || result.RecordCommand == "" {
		t.Fatalf("reviewer-bound correction omitted replay route: %+v", result)
	}
	for _, flag := range []string{
		"-ReviewerPacketId", "-ReviewerRouteId", "-ReviewerShardId", "-ReviewerPacketPath",
		"-ReviewerResultLineagePath", "-ReviewerLineageSession", "-ReviewerDispatchReceiptPath",
		"-ReviewerDispatchReceiptSha256", "-ReviewerCompletionReceiptPath", "-ReviewerCompletionReceiptSha256",
		"-ReviewerLineageInputPath", "-ReviewerLineageInputSha256", "-ReviewerLineageInputBytes",
		"-ReviewerLineageResultSha256", "-ReviewerManifestSha256", "-ReviewerVerificationEventId",
		"-ReviewerDecisionEventId", "-ReviewerOwnerExecutor", "-ReviewerOwnerGeneration",
	} {
		if !slices.Contains(result.RecordArgs, flag) {
			t.Fatalf("reviewer-bound correction replay args omitted %s: %+v", flag, result.RecordArgs)
		}
	}
}

func TestAppendWhatIfProjectsBlockerKinds(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opt          Options
		wantPending  int
		wantOpen     int
		wantDecision int
	}{
		{name: "candidate", opt: Options{Kind: "candidate", Lane: "main", Subject: "candidate blocker", Confidence: "high", Status: "open"}, wantDecision: 1},
		{name: "decision", opt: Options{Kind: "decision", Lane: "main", Subject: "decision blocker", Decision: "defer"}, wantDecision: 1},
		{name: "intervention", opt: Options{Kind: "intervention", Lane: "main", Subject: "manual stop", Action: "override", Status: "open"}, wantOpen: 1},
		{name: "request", opt: Options{Kind: "request", Lane: "main", Subject: "debug gate", Status: "pending-gate", Risk: "high"}, wantPending: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot, caseRoot, pack := noteFixture(t)
			result, err := Append(repoRoot, caseRoot, pack, tc.opt, true)
			if err != nil {
				t.Fatal(err)
			}
			if result.ExecutorAction.Blocked || !result.ExecutorAction.Ready || result.WouldExecutorAction == nil || !result.WouldExecutorAction.Blocked || result.WouldExecutorAction.Ready {
				t.Fatalf("unexpected current/would action: %+v", result)
			}
			would := result.WouldExecutorAction
			if would.PendingGates != tc.wantPending || would.OpenInterventions != tc.wantOpen || would.OpenDecisions != tc.wantDecision {
				t.Fatalf("unexpected blocker counts: %+v", would)
			}

			if result.MissionCommanderAction.State != "ready-to-continue" || result.WouldMissionCommanderAction == nil || result.WouldMissionCommanderAction.State == "ready-to-continue" {
				t.Fatalf("blocker what-if should expose current and would commander action delta: current=%+v would=%+v", result.MissionCommanderAction, result.WouldMissionCommanderAction)
			}
			if !hasNoteCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions", "/rekit continue main -WhatIf -Format json", false, true) {
				t.Fatalf("current commander next action missing ready continue: %+v", result.MissionCommanderNextActions)
			}
			wantPrimaryBlocked := !strings.Contains(result.WouldMissionCommanderAction.PrimaryCommand, " -WhatIf")
			primaryCommand := result.WouldMissionCommanderAction.PrimaryCommand
			if primaryCommand == "/rekit handoff main" {
				primaryCommand += " -WhatIf -Format json"
			}
			if !hasNoteCommanderNextAction(result.WouldMissionCommanderNextActions, "missionCommanderActions", primaryCommand, wantPrimaryBlocked, true) {
				t.Fatalf("would commander next action missing primary: action=%+v next=%+v", result.WouldMissionCommanderAction, result.WouldMissionCommanderNextActions)
			}
			if tc.name == "intervention" && !hasNoteCommanderNextAction(result.WouldMissionCommanderNextActions, "missionCommanderActions.followUp", strings.Replace(result.WouldMissionCommanderAction.PrimaryCommand, " -WhatIf", " -Apply", 1), true, true) {
				t.Fatalf("would commander next action missing blocked reconcile apply follow-up: action=%+v next=%+v", result.WouldMissionCommanderAction, result.WouldMissionCommanderNextActions)
			}
			if !hasNoteCommanderNextAction(result.WouldMissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit continue main -WhatIf -Format json", true, true) {
				t.Fatalf("would commander next action missing blocked continue what-if follow-up: %+v", result.WouldMissionCommanderNextActions)
			}
		})
	}
}

func TestAppendWhatIfDuplicateReturnsCurrentActionOnly(t *testing.T) {
	repoRoot, caseRoot, pack := noteFixture(t)
	writeNoteText(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"), `{"kind":"observation","lane":"main","eventId":"evt-preview-duplicate"}`+"\n")
	result, err := Append(repoRoot, caseRoot, pack, Options{Kind: "candidate", Lane: "main", Subject: "duplicate candidate", Status: "open", EventID: "evt-preview-duplicate"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied || result.IsMutation || result.Reason != "duplicate eventId" || result.WouldExecutorAction != nil || result.WouldMissionCommanderAction != nil || len(result.WouldMissionCommanderNextActions) != 0 || result.ExecutorAction.Blocked || !result.ExecutorAction.Ready {
		t.Fatalf("duplicate what-if should return unchanged current action only: %+v", result)
	}
	if result.MissionCommanderAction.State != "ready-to-continue" || !hasNoteCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions", "/rekit continue main -WhatIf -Format json", false, true) {
		t.Fatalf("duplicate should preserve current commander projection only: action=%+v next=%+v", result.MissionCommanderAction, result.MissionCommanderNextActions)
	}
	assertNoteNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "candidates.jsonl"))
}

func TestAppendReturnsPostActionForAppliedBlockerKinds(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opt          Options
		wantPending  int
		wantOpen     int
		wantDecision int
	}{
		{name: "candidate", opt: Options{Kind: "candidate", Lane: "main", Subject: "candidate blocker", Confidence: "high", Status: "open"}, wantDecision: 1},
		{name: "decision", opt: Options{Kind: "decision", Lane: "main", Subject: "decision blocker", Decision: "defer"}, wantDecision: 1},
		{name: "intervention", opt: Options{Kind: "intervention", Lane: "main", Subject: "manual stop", Action: "override", Status: "open"}, wantOpen: 1},
		{name: "request", opt: Options{Kind: "request", Lane: "main", Subject: "debug gate", Status: "pending-gate", Risk: "high"}, wantPending: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot, caseRoot, pack := noteFixture(t)
			result, err := Append(repoRoot, caseRoot, pack, tc.opt, false)
			if err != nil {
				t.Fatal(err)
			}
			if !result.Applied || !result.IsMutation || !result.ExecutorAction.Blocked || result.ExecutorAction.Ready || result.WouldExecutorAction != nil {
				t.Fatalf("unexpected applied action: %+v", result)
			}
			if result.ExecutorAction.PendingGates != tc.wantPending || result.ExecutorAction.OpenInterventions != tc.wantOpen || result.ExecutorAction.OpenDecisions != tc.wantDecision {
				t.Fatalf("unexpected applied blocker counts: %+v", result.ExecutorAction)
			}
			if result.MissionCommanderAction.State == "ready-to-continue" || result.WouldMissionCommanderAction != nil || len(result.WouldMissionCommanderNextActions) != 0 {
				t.Fatalf("applied blocker note should expose post commander action only: action=%+v would=%+v", result.MissionCommanderAction, result.WouldMissionCommanderAction)
			}
			wantPrimaryBlocked := !strings.Contains(result.MissionCommanderAction.PrimaryCommand, " -WhatIf")
			primaryCommand := result.MissionCommanderAction.PrimaryCommand
			if primaryCommand == "/rekit handoff main" {
				primaryCommand += " -WhatIf -Format json"
			}
			if !hasNoteCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions", primaryCommand, wantPrimaryBlocked, true) {
				t.Fatalf("post commander next action missing primary: action=%+v next=%+v", result.MissionCommanderAction, result.MissionCommanderNextActions)
			}
			if tc.name == "intervention" && !hasNoteCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions.followUp", strings.Replace(result.MissionCommanderAction.PrimaryCommand, " -WhatIf", " -Apply", 1), true, true) {
				t.Fatalf("post commander next action missing blocked reconcile apply follow-up: action=%+v next=%+v", result.MissionCommanderAction, result.MissionCommanderNextActions)
			}
			if !hasNoteCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit continue main -WhatIf -Format json", true, true) {
				t.Fatalf("post commander next action missing blocked continue what-if follow-up: %+v", result.MissionCommanderNextActions)
			}
		})
	}
}

func TestAppendInterventionRechecksOpenLaneInsideLease(t *testing.T) {
	repoRoot, caseRoot, pack := noteFixture(t)
	previous := interventionBeforeLeaseHook
	interventionBeforeLeaseHook = func() error {
		writeNoteText(t, filepath.Join(caseRoot, ".rekit", "board.json"), `{"lanes":[{"id":"main","status":"closed"}]}`)
		return nil
	}
	t.Cleanup(func() { interventionBeforeLeaseHook = previous })

	_, err := Append(repoRoot, caseRoot, pack, Options{
		Kind:    "intervention",
		Lane:    "main",
		Subject: "must not cross lane closure",
		Action:  "override",
		Status:  "open",
		EventID: "evt-closed-after-preview",
	}, false)
	if err == nil || !strings.Contains(err.Error(), "closed lane main") {
		t.Fatalf("closed-lane intervention error = %v", err)
	}
	assertNoteNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "interventions.jsonl"))
}

func TestAppendVerificationWithControlRejectsStaleHead(t *testing.T) {
	repoRoot, caseRoot, pack := noteFixtureWithStateRoot(t, projectstate.CurrentDir)
	writeNoteText(t, filepath.Join(caseRoot, projectstate.CurrentDir, "board.json"), `{"lanes":[{"id":"main","status":"open","currentExecutor":"executor-a","executorGeneration":1}]}`+"\n")
	writeNoteText(t, filepath.Join(caseRoot, projectstate.CurrentDir, "lanes", "main", "lane.json"), `{"id":"main","status":"open","currentExecutor":"executor-a","executorGeneration":1}`+"\n")
	owner, err := laneowner.Read(caseRoot, "main")
	if err != nil {
		t.Fatal(err)
	}
	capability, err := capabilitycontract.Bind(capabilitycontract.Transport())
	if err != nil {
		t.Fatal(err)
	}
	binding, err := executioncontrol.CaptureBinding(caseRoot, owner, capability)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := executioncontrol.Preview(caseRoot, executioncontrol.Options{
		Lane: "main", Action: executioncontrol.ActionPause, Actor: "note-control-test",
		Reason: "make the evidence acknowledgement stale", PublicationStamp: "2026-08-19T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executioncontrol.Apply(caseRoot, executioncontrol.Options{
		Lane: preview.Lane, Action: preview.Action, Actor: preview.Actor, Reason: preview.Reason,
		PublicationStamp: preview.PublicationStamp, ExpectedPlanSHA256: preview.ExpectedPlanSHA256,
	}); err != nil {
		t.Fatal(err)
	}

	_, err = Append(repoRoot, caseRoot, pack, Options{
		Kind: "verification", Lane: "main", Subject: "execution evidence review accepted",
		Summary: "accepted exact adapter evidence", Verifier: "tool-review", Verdict: "accepted", Status: "resolved",
		ExpectedExecutionControlBinding: &binding,
	}, false)
	if err == nil || !strings.Contains(err.Error(), "lane execution is paused") {
		t.Fatalf("stale verification acknowledgement error = %v", err)
	}
	assertNoteNotExists(t, filepath.Join(caseRoot, projectstate.CurrentDir, "facts", "verifications.jsonl"))
}

func TestAppendDuplicateWithControlRechecksStaleHead(t *testing.T) {
	repoRoot, caseRoot, pack := noteFixtureWithStateRoot(t, projectstate.CurrentDir)
	writeNoteText(t, filepath.Join(caseRoot, projectstate.CurrentDir, "board.json"), `{"lanes":[{"id":"main","status":"open","currentExecutor":"executor-a","executorGeneration":1}]}`+"\n")
	writeNoteText(t, filepath.Join(caseRoot, projectstate.CurrentDir, "lanes", "main", "lane.json"), `{"id":"main","status":"open","currentExecutor":"executor-a","executorGeneration":1}`+"\n")
	owner, err := laneowner.Read(caseRoot, "main")
	if err != nil {
		t.Fatal(err)
	}
	capability, err := capabilitycontract.Bind(capabilitycontract.Transport())
	if err != nil {
		t.Fatal(err)
	}
	binding, err := executioncontrol.CaptureBinding(caseRoot, owner, capability)
	if err != nil {
		t.Fatal(err)
	}
	opt := Options{
		Kind: "verification", Lane: "main", Subject: "execution evidence review accepted",
		Summary: "accepted exact adapter evidence", Verifier: "tool-review", Verdict: "accepted", Status: "resolved",
		EventID: "verification-control-duplicate", CreatedAt: "2026-08-19T00:00:00Z",
		ExpectedExecutionControlBinding: &binding,
	}
	first, err := Append(repoRoot, caseRoot, pack, opt, false)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Applied {
		t.Fatalf("initial controlled verification was not applied: %+v", first)
	}
	preview, err := executioncontrol.Preview(caseRoot, executioncontrol.Options{
		Lane: "main", Action: executioncontrol.ActionPause, Actor: "note-control-test",
		Reason: "make duplicate acknowledgement stale", PublicationStamp: "2026-08-19T00:00:01Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executioncontrol.Apply(caseRoot, executioncontrol.Options{
		Lane: preview.Lane, Action: preview.Action, Actor: preview.Actor, Reason: preview.Reason,
		PublicationStamp: preview.PublicationStamp, ExpectedPlanSHA256: preview.ExpectedPlanSHA256,
	}); err != nil {
		t.Fatal(err)
	}

	_, err = Append(repoRoot, caseRoot, pack, opt, false)
	if err == nil || !strings.Contains(err.Error(), "lane execution is paused") {
		t.Fatalf("stale duplicate verification error = %v", err)
	}
}

func TestAppendRejectsOversizedEventBeforeWrite(t *testing.T) {
	for _, whatIf := range []bool{true, false} {
		t.Run(map[bool]string{true: "what-if", false: "append"}[whatIf], func(t *testing.T) {
			repoRoot, caseRoot, pack := noteFixture(t)
			_, err := Append(repoRoot, caseRoot, pack, Options{Kind: "observation", Lane: "main", Subject: strings.Repeat("x", maxEventJSONBytes)}, whatIf)
			if err == nil || !strings.Contains(err.Error(), "JSONL limit") {
				t.Fatalf("oversized event error = %v", err)
			}
			assertNoteNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
		})
	}
}

func TestAppendRejectsMalformedLedgerBeforeProjectionOrWrite(t *testing.T) {
	for _, whatIf := range []bool{true, false} {
		t.Run(map[bool]string{true: "what-if", false: "append"}[whatIf], func(t *testing.T) {
			repoRoot, caseRoot, pack := noteFixture(t)
			writeNoteText(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"), "not json\n")
			_, err := Append(repoRoot, caseRoot, pack, Options{Kind: "candidate", Lane: "main", Subject: "must fail closed", Status: "open"}, whatIf)
			if err == nil || !strings.Contains(err.Error(), "invalid JSONL") {
				t.Fatalf("strict ledger error = %v", err)
			}
			assertNoteNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "candidates.jsonl"))
		})
	}
}

func TestAppendWritesAndListsEvent(t *testing.T) {
	repoRoot, caseRoot, pack := noteFixture(t)

	result, err := Append(repoRoot, caseRoot, pack, Options{
		Kind:         "verification",
		Lane:         "main",
		Subject:      "candidate-alpha",
		Summary:      "reviewer accepted packet shard",
		Actor:        "reviewer-test",
		Target:       "candidate-alpha",
		Verifier:     "manual-review",
		Verdict:      "accepted",
		EvidenceRefs: "ev-alpha,ev-beta",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Command != "note" || !result.IsMutation || !result.Applied || result.EventID == "" || result.Path != ".rekit/facts/verifications.jsonl" {
		t.Fatalf("unexpected note append result: %+v", result)
	}
	if got := stringValue(result.Event, "kind"); got != "verification" {
		t.Fatalf("event kind = %q", got)
	}
	if got := stringValue(result.Event, "evidenceRefs"); got != "ev-alpha,ev-beta" {
		t.Fatalf("event evidenceRefs = %q", got)
	}

	listed, err := ListEvents(repoRoot, caseRoot, pack, Options{Kind: "verification", Lane: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if listed.Command != "note" || listed.IsMutation || listed.EventCount != 1 || len(listed.Groups) != 1 || listed.Groups[0].Kind != "verification" || listed.Groups[0].Total != 1 {
		t.Fatalf("unexpected note list result: %+v", listed)
	}
	event := listed.Groups[0].Events[0]
	if stringValue(event, "eventId") != result.EventID || stringValue(event, "verifier") != "manual-review" || stringValue(event, "verdict") != "accepted" || stringValue(event, "target") != "candidate-alpha" {
		t.Fatalf("unexpected listed event: %+v", event)
	}
	assertNoteNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertNoteNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))
}

func TestListRejectsInvalidJSONL(t *testing.T) {
	repoRoot, caseRoot, pack := noteFixture(t)
	writeNoteText(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"), "not json\n")

	_, err := ListEvents(repoRoot, caseRoot, pack, Options{Kind: "observation"})
	if err == nil || !strings.Contains(err.Error(), "invalid JSONL") {
		t.Fatalf("invalid JSONL error = %v", err)
	}
}

func TestListReadsSharedFactFileMapping(t *testing.T) {
	repoRoot, caseRoot, pack := noteFixture(t)
	writeNoteText(t, filepath.Join(caseRoot, ".rekit", "facts", "hypotheses.jsonl"), `{"kind":"hypothesis","lane":"main","subject":"shared mapping"}`+"\n")

	listed, err := ListEvents(repoRoot, caseRoot, pack, Options{Kind: "hypothesis", Lane: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if listed.EventCount != 1 || len(listed.Groups) != 1 || stringValue(listed.Groups[0].Events[0], "subject") != "shared mapping" {
		t.Fatalf("unexpected mapped list result: %+v", listed)
	}
}

func TestListUsesSharedLedgerKinds(t *testing.T) {
	repoRoot, caseRoot, pack := noteFixture(t)
	factsRoot := filepath.Join(caseRoot, ".rekit", "facts")
	for _, kind := range mission.LedgerKinds() {
		writeNoteText(t, filepath.Join(factsRoot, mission.FactFileName(kind)), `{"kind":"`+kind+`","lane":"main","subject":"`+kind+` event"}`+"\n")
	}

	listed, err := ListEvents(repoRoot, caseRoot, pack, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if listed.EventCount != len(mission.LedgerKinds()) || len(listed.Groups) != len(mission.LedgerKinds()) {
		t.Fatalf("list did not use shared ledger kinds: %+v", listed)
	}
	for i, kind := range mission.LedgerKinds() {
		if listed.Groups[i].Kind != kind || stringValue(listed.Groups[i].Events[0], "subject") != kind+" event" {
			t.Fatalf("group %d = %+v, want kind %s", i, listed.Groups[i], kind)
		}
	}
}

func TestAppendDuplicateExplicitEventIDUsesSharedLedgerEventIDs(t *testing.T) {
	repoRoot, caseRoot, pack := noteFixture(t)
	writeNoteText(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"), `{"kind":"observation","lane":"main","eventId":"evt-note-duplicate"}`+"\n")
	opt := Options{
		Kind:     "decision",
		Lane:     "main",
		Subject:  "candidate-alpha",
		Summary:  "main accepted reviewer verdict",
		Actor:    "main-test",
		Target:   "candidate-alpha",
		Decision: "accept",
		Status:   "accepted",
		EventID:  "evt-note-duplicate",
	}

	result, err := Append(repoRoot, caseRoot, pack, opt, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied || result.Reason != "duplicate eventId" || result.EventID != opt.EventID {
		t.Fatalf("unexpected duplicate append result: %+v", result)
	}
	if result.WouldExecutorAction != nil || result.WouldMissionCommanderAction != nil || len(result.WouldMissionCommanderNextActions) != 0 || result.ExecutorAction.Blocked || !result.ExecutorAction.Ready {
		t.Fatalf("duplicate should return the unchanged current action only: %+v", result)
	}
	if result.MissionCommanderAction.State != "ready-to-continue" || !hasNoteCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions", "/rekit continue main -WhatIf -Format json", false, true) {
		t.Fatalf("duplicate should preserve current commander projection only: action=%+v next=%+v", result.MissionCommanderAction, result.MissionCommanderNextActions)
	}
	assertNoteNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl"))
}

func TestAppendRejectsInvalidKindAndUnknownLaneWithoutWrite(t *testing.T) {
	repoRoot, caseRoot, pack := noteFixture(t)

	_, err := Append(repoRoot, caseRoot, pack, Options{Kind: "unknown", Lane: "main", Subject: "bad kind"}, false)
	if err == nil || !strings.Contains(err.Error(), "invalid note kind") {
		t.Fatalf("invalid kind error = %v", err)
	}
	_, err = Append(repoRoot, caseRoot, pack, Options{Kind: "observation", Lane: "missing", Subject: "bad lane"}, false)
	if err == nil || !strings.Contains(err.Error(), "unknown lane") {
		t.Fatalf("unknown lane error = %v", err)
	}
	assertNoteNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
}

func hasNoteCommanderNextAction(items []mission.MissionCommanderNextActionItem, source, command string, blocked, requiresReview bool) bool {
	for _, item := range items {
		if item.Source == source && item.Command == command && item.Blocked == blocked && item.RequiresReview == requiresReview {
			return true
		}
	}
	return false
}

func noteFixture(t *testing.T) (repoRoot, caseRoot, pack string) {
	t.Helper()
	return noteFixtureWithStateRoot(t, projectstate.LegacyDir)
}

func noteFixtureWithStateRoot(t *testing.T, stateDir string) (repoRoot, caseRoot, pack string) {
	t.Helper()
	layout := testfixture.LegacyCase
	if stateDir == projectstate.CurrentDir {
		layout = testfixture.CurrentProject
	} else if stateDir != projectstate.LegacyDir {
		t.Fatalf("unsupported note fixture state root: %s", stateDir)
	}
	project := testfixture.NewProject(t, testfixture.ProjectOptions{
		Layout:      layout,
		Pack:        "binary-re",
		ProjectName: "note-fixture",
	})
	writeNoteText(t, filepath.Join(project.StateRoot, "board.json"), `{"lanes":[{"id":"main"}]}`)
	return project.RuntimeRepoRoot, project.CaseRoot, project.Pack
}

func writeNoteText(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertNoteNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil || !os.IsNotExist(err) {
		t.Fatalf("path exists or stat failed unexpectedly for %s: %v", path, err)
	}
}
