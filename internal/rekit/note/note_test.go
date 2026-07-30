package note

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

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
	if result.ExecutorAction.Blocked || !result.ExecutorAction.Ready || result.WouldExecutorAction == nil || result.WouldExecutorAction.Blocked || !result.WouldExecutorAction.Ready {
		t.Fatalf("verification what-if should not change executor readiness: %+v", result)
	}
	if result.MissionCommanderAction.State != "ready-to-continue" || !hasNoteCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions", "/rekit continue main", false, false) || !hasNoteCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit handoff main", false, false) {
		t.Fatalf("current commander projection drifted: action=%+v next=%+v", result.MissionCommanderAction, result.MissionCommanderNextActions)
	}
	if result.WouldMissionCommanderAction == nil || result.WouldMissionCommanderAction.State != "ready-to-continue" || !hasNoteCommanderNextAction(result.WouldMissionCommanderNextActions, "missionCommanderActions", "/rekit continue main", false, false) || !hasNoteCommanderNextAction(result.WouldMissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit handoff main", false, false) {
		t.Fatalf("would commander projection drifted: action=%+v next=%+v", result.WouldMissionCommanderAction, result.WouldMissionCommanderNextActions)
	}
	assertNoteNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl"))
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
	if result.RecordCommand != "" {
		t.Fatalf("internal reviewer fields should not expose a non-replayable record command: %q", result.RecordCommand)
	}
	if stringValue(result.Event, "packetId") != "packet-1" || stringValue(result.Event, "reviewerSession") != "reviewer-session-1" || stringValue(result.Event, "reviewerDecision") != "accept" {
		t.Fatalf("internal fields missing from event: %+v", result.Event)
	}
	assertNoteNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl"))
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
			if !hasNoteCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions", "/rekit continue main", false, false) {
				t.Fatalf("current commander next action missing ready continue: %+v", result.MissionCommanderNextActions)
			}
			wantPrimaryBlocked := !strings.Contains(result.WouldMissionCommanderAction.PrimaryCommand, " -WhatIf")
			if !hasNoteCommanderNextAction(result.WouldMissionCommanderNextActions, "missionCommanderActions", result.WouldMissionCommanderAction.PrimaryCommand, wantPrimaryBlocked, true) {
				t.Fatalf("would commander next action missing primary: action=%+v next=%+v", result.WouldMissionCommanderAction, result.WouldMissionCommanderNextActions)
			}
			if tc.name == "intervention" && !hasNoteCommanderNextAction(result.WouldMissionCommanderNextActions, "missionCommanderActions.followUp", strings.Replace(result.WouldMissionCommanderAction.PrimaryCommand, " -WhatIf", " -Apply", 1), true, true) {
				t.Fatalf("would commander next action missing blocked reconcile apply follow-up: action=%+v next=%+v", result.WouldMissionCommanderAction, result.WouldMissionCommanderNextActions)
			}
			if !hasNoteCommanderNextAction(result.WouldMissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit continue main -WhatIf", true, true) {
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
	if result.MissionCommanderAction.State != "ready-to-continue" || !hasNoteCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions", "/rekit continue main", false, false) {
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
			if !hasNoteCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions", result.MissionCommanderAction.PrimaryCommand, wantPrimaryBlocked, true) {
				t.Fatalf("post commander next action missing primary: action=%+v next=%+v", result.MissionCommanderAction, result.MissionCommanderNextActions)
			}
			if tc.name == "intervention" && !hasNoteCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions.followUp", strings.Replace(result.MissionCommanderAction.PrimaryCommand, " -WhatIf", " -Apply", 1), true, true) {
				t.Fatalf("post commander next action missing blocked reconcile apply follow-up: action=%+v next=%+v", result.MissionCommanderAction, result.MissionCommanderNextActions)
			}
			if !hasNoteCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit continue main -WhatIf", true, true) {
				t.Fatalf("post commander next action missing blocked continue what-if follow-up: %+v", result.MissionCommanderNextActions)
			}
		})
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
	if result.MissionCommanderAction.State != "ready-to-continue" || !hasNoteCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions", "/rekit continue main", false, false) {
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
	root := t.TempDir()
	repoRoot = filepath.Join(root, "repo")
	caseRoot = filepath.Join(root, "case")
	pack = "vmp-re"
	writeNoteText(t, filepath.Join(repoRoot, "packs", pack, "manifest.yml"), "id: vmp-re\n")
	writeNoteText(t, filepath.Join(caseRoot, ".rekit", "instance.yml"), "templateRoot: \""+repoRoot+"\"\ntemplatePack: \""+pack+"\"\nprojectName: \"note-fixture\"\nprojectRoot: \""+caseRoot+"\"\n")
	writeNoteText(t, filepath.Join(caseRoot, ".rekit", "board.json"), `{"lanes":[{"id":"main"}]}`)
	return repoRoot, caseRoot, pack
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
