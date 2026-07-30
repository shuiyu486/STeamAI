package workstream

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestMissionCommanderNextActionMarkdownLineIncludesIdentity(t *testing.T) {
	line := MissionCommanderNextActionMarkdownLine(mission.MissionCommanderNextActionItem{
		Lane:           "main",
		Label:          "Main",
		GateEventID:    "evt-adapter",
		ActionID:       "evt-adapter:adapter-report-record",
		State:          "ready-to-record-evidence",
		Source:         "adapterReportValidation.missionCommanderAction",
		Command:        "/rekit gate -Apply -GateEventId evt-adapter",
		RequiresReview: true,
	})
	for _, want := range []string{"state=ready-to-record-evidence source=adapterReportValidation.missionCommanderAction", "command=`/rekit gate -Apply -GateEventId evt-adapter` lane=main label=Main", "gateEventId=evt-adapter", "actionId=evt-adapter:adapter-report-record"} {
		if !strings.Contains(line, want) {
			t.Fatalf("markdown line missing %q: %s", want, line)
		}
	}
}

func TestMissionCommanderActionRunLoopMarkdownLinesIncludesCurrentStep(t *testing.T) {
	queue := mission.MissionCommanderActionQueueFor([]mission.MissionCommanderNextActionItem{
		{Lane: "main", Label: "main", State: "needs-gate-decision", Command: "/rekit gate -Action debug -Lane main -WhatIf", Source: "missionCommanderActions", RequiresReview: true},
		{Lane: "main", Label: "main", State: "needs-gate-decision", Command: "/rekit gate -Action debug -Lane main -Apply -Actor <actor>", Source: "missionCommanderActions.followUp", Blocked: true, RequiresReview: true},
	})
	lines := MissionCommanderActionRunLoopMarkdownLines(queue)
	for _, want := range []string{"current run loop：currentRunLoopStep=preview-current steps=4", "run loop step：order=2 step=preview-current actor=main-agent state=needs-gate-decision source=missionCommanderActions command=`/rekit gate -Action debug -Lane main -WhatIf`", "run loop step：order=4 step=follow-up-after-refresh", "run loop boundary：step=follow-up-after-refresh boundary=follow-up commands remain candidates until refreshed state makes them current or unblocked", "driver request：kind=preview-command step=preview-current actor=main-agent executable=true blocked=false requiresReview=true command=`/rekit gate -Action debug -Lane main -WhatIf` guidance=``", "driver request expected receipt：state=refresh-required command=`/rekit gate -Action debug -Lane main -WhatIf`"} {
		if !slices.ContainsFunc(lines, func(line string) bool { return strings.Contains(line, want) }) {
			t.Fatalf("run-loop markdown missing %q: %+v", want, lines)
		}
	}
}

func TestMissionCommanderActionRunLoopMarkdownLinesKeepsGuidanceAsDriverGuidance(t *testing.T) {
	queue := mission.MissionCommanderActionQueueFor([]mission.MissionCommanderNextActionItem{
		{State: "ready-for-next-batch-selection", Command: "select the next Windows-verifiable product-path closure", Source: "releaseHandoffNextBatch"},
	})
	lines := MissionCommanderActionRunLoopMarkdownLines(queue)
	for _, want := range []string{"current run loop：currentRunLoopStep=inspect-current steps=2", "driver request：kind=review-guidance step=inspect-current actor=main-agent executable=false blocked=false requiresReview=false command=`` guidance=`select the next Windows-verifiable product-path closure`", "driver request boundary：guidance text must be reviewed by the main Agent or harness, not executed as a shell command"} {
		if !slices.ContainsFunc(lines, func(line string) bool { return strings.Contains(line, want) }) {
			t.Fatalf("guidance driver markdown missing %q: %+v", want, lines)
		}
	}
}

func TestLimitProjectMissionCommanderNextActionItemsKeepsNextBatchCandidateQueue(t *testing.T) {
	items := []mission.MissionCommanderNextActionItem{
		{Label: "next-batch", ActionID: "next-batch-selection", State: "ready-for-next-batch-selection", Source: "releaseHandoffNextBatch", Command: "select the next batch"},
		{Label: "mission-commander", ActionID: "next-batch-mission-commander-operational-closure", State: "next-batch-candidate-domain", Source: "releaseHandoffNextBatch.followUp.candidateDomain", Command: "select mission commander closure"},
		{Label: "replacement-executor", ActionID: "next-batch-replacement-executor-takeover", State: "next-batch-candidate-domain", Source: "releaseHandoffNextBatch.followUp.candidateDomain", Command: "select replacement executor takeover"},
		{Label: "reviewer-orchestration", ActionID: "next-batch-reviewer-orchestration-closure", State: "next-batch-candidate-domain", Source: "releaseHandoffNextBatch.followUp.candidateDomain", Command: "select reviewer orchestration closure"},
	}
	limited := limitProjectMissionCommanderNextActionItems(items, 2)
	if len(limited) != len(items) || limited[0].ActionID != "next-batch-selection" || !slices.ContainsFunc(limited, func(item mission.MissionCommanderNextActionItem) bool {
		return item.ActionID == "next-batch-replacement-executor-takeover"
	}) {
		t.Fatalf("next-batch candidate-domain queue should not be truncated in project durable handoff: %+v", limited)
	}

	mixed := append([]mission.MissionCommanderNextActionItem{{Label: "main", State: "ready-to-continue", Source: "missionCommanderActions", Command: "/rekit continue main"}}, items...)
	mixedLimited := limitProjectMissionCommanderNextActionItems(mixed, 2)
	if len(mixedLimited) != len(mixed) || mixedLimited[0].Command != "/rekit continue main" || !slices.ContainsFunc(mixedLimited, func(item mission.MissionCommanderNextActionItem) bool {
		return item.ActionID == "next-batch-replacement-executor-takeover"
	}) {
		t.Fatalf("mixed project queue should preserve injected next-batch guidance for durable handoff: %+v", mixedLimited)
	}

	ordinary := append([]mission.MissionCommanderNextActionItem{}, items...)
	for idx := range ordinary {
		ordinary[idx].Source = "missionCommanderActions"
	}
	ordinaryLimited := limitProjectMissionCommanderNextActionItems(ordinary, 2)
	if len(ordinaryLimited) != 2 || slices.ContainsFunc(ordinaryLimited, func(item mission.MissionCommanderNextActionItem) bool { return item.ActionID == "next-batch-selection" }) {
		t.Fatalf("ordinary project action queue should keep existing tail limit semantics: %+v", ordinaryLimited)
	}
}

func TestMissionCommanderNextActionsWithAuthorizedGateAdaptersPrioritizesRepair(t *testing.T) {
	base := []mission.MissionCommanderNextActionItem{{
		Lane:    "main",
		Label:   "main",
		State:   "ready-to-continue",
		Source:  "missionCommanderActions",
		Command: "/rekit continue main",
	}}
	handoffs := []AuthorizedGateAdapterHandoff{
		{
			EventID: "evt-record",
			missionCommanderNextActions: []mission.MissionCommanderNextActionItem{{
				Lane:           "main",
				Label:          "main",
				State:          "ready-to-record-evidence",
				Source:         "adapterReportValidation.missionCommanderAction",
				Command:        "/rekit gate -Apply -GateEventId evt-record",
				RequiresReview: true,
			}},
		},
		{
			EventID: "evt-repair",
			missionCommanderNextActions: []mission.MissionCommanderNextActionItem{{
				Lane:           "main",
				Label:          "main",
				State:          "repair-adapter-report",
				Source:         "adapterReportValidation.repairHints",
				Command:        "move-evidence-refs-under-authorized-output-paths",
				RequiresReview: true,
			}},
		},
		{
			EventID: "evt-acknowledged",
			ReportSummary: &gate.AdapterReportHandoffSummary{
				State: "evidence-already-recorded",
			},
			missionCommanderNextActions: []mission.MissionCommanderNextActionItem{{
				Lane:           "main",
				Label:          "main",
				State:          "evidence-already-recorded",
				Source:         "adapterReportLiveSnapshot.recordedEvidence",
				Command:        "/rekit handoff main",
				RequiresReview: true,
			}},
		},
	}
	items := MissionCommanderNextActionsWithAuthorizedGateAdaptersAndAcknowledgements(base, handoffs, map[string]bool{"evt-acknowledged": true})
	queue := mission.MissionCommanderActionQueueFor(items)
	if queue.CurrentAction == nil || queue.CurrentAction.GateEventID != "evt-repair" || queue.CurrentAction.State != "repair-adapter-report" || queue.CurrentAction.Command != "move-evidence-refs-under-authorized-output-paths" {
		t.Fatalf("repair action should be current before record-ready and acknowledged provenance: queue=%+v items=%+v", queue, items)
	}
	if len(items) < 2 || items[0].GateEventID != "evt-repair" || items[1].GateEventID != "evt-record" {
		t.Fatalf("adapter actions should be sorted repair before record-ready: %+v", items)
	}
	for _, item := range items {
		if item.GateEventID == "evt-acknowledged" {
			t.Fatalf("acknowledged recorded adapter action should not re-enter queue: %+v", items)
		}
	}
}

func TestAuthorizedGateAdapterAcknowledgementPreservesMainEscalation(t *testing.T) {
	action := mission.MissionCommanderNextActionItem{
		GateEventID:    "evt-escalated",
		State:          "needs-main-escalation",
		Command:        "/rekit handoff main",
		RequiresReview: true,
	}
	handoff := AuthorizedGateAdapterHandoff{
		EventID:                     "evt-escalated",
		ReportSummary:               &gate.AdapterReportHandoffSummary{State: "needs-main-escalation", RequiresMainEscalation: true, CurrentAction: action.Command, NextActionCount: 1, ReviewRequiredActionCount: 1},
		missionCommanderNextActions: []mission.MissionCommanderNextActionItem{action},
	}

	applyAuthorizedGateAdapterAcknowledgement(&handoff, map[string]bool{"evt-escalated": true})

	if handoff.Acknowledged || handoff.AcknowledgementState != "" || len(handoff.missionCommanderNextActions) != 1 {
		t.Fatalf("ordinary evidence acknowledgement must not close main escalation: %+v", handoff)
	}
	if handoff.ReportSummary == nil || !handoff.ReportSummary.RequiresMainEscalation || handoff.ReportSummary.CurrentAction != action.Command || handoff.ReportSummary.NextActionCount != 1 {
		t.Fatalf("main escalation summary was hidden by acknowledgement: %+v", handoff.ReportSummary)
	}
}

func TestAuthorizedGateAdapterHandoffsLimitKeepsActionableEarlierGate(t *testing.T) {
	repair := AuthorizedGateAdapterHandoff{
		EventID:       "evt-repair-earlier",
		ReportSummary: &gate.AdapterReportHandoffSummary{State: "repair-adapter-report", ReportPresent: true, RequiresRepair: true},
		missionCommanderNextActions: []mission.MissionCommanderNextActionItem{{
			Lane:           "main",
			Label:          "main",
			State:          "repair-adapter-report",
			Source:         "adapterReportValidation.repairHints",
			Command:        "move-evidence-refs-under-authorized-output-paths",
			RequiresReview: true,
		}},
	}
	items := []AuthorizedGateAdapterHandoff{repair}
	for idx := 1; idx <= maxHandoffRows; idx++ {
		items = append(items, AuthorizedGateAdapterHandoff{
			EventID:       fmt.Sprintf("evt-low-%d", idx),
			Acknowledged:  true,
			ReportSummary: &gate.AdapterReportHandoffSummary{State: "evidence-already-recorded", ReportPresent: true, Valid: true},
		})
	}

	limited := limitAuthorizedGateAdapterHandoffs(items, maxHandoffRows)
	if len(limited) != maxHandoffRows || limited[0].EventID != "evt-repair-earlier" || slices.ContainsFunc(limited, func(item AuthorizedGateAdapterHandoff) bool { return item.EventID == "evt-low-1" }) {
		t.Fatalf("authorized adapter handoff limiter should keep earlier repair and drop oldest low-value item: %+v", limited)
	}
	merged := MissionCommanderNextActionsWithAuthorizedGateAdapters([]mission.MissionCommanderNextActionItem{{Lane: "main", Label: "main", State: "ready-to-continue", Source: "missionCommanderActions", Command: "/rekit continue main"}}, limited)
	queue := mission.MissionCommanderActionQueueFor(merged)
	if queue.CurrentAction == nil || queue.CurrentAction.GateEventID != "evt-repair-earlier" || queue.CurrentAction.State != "repair-adapter-report" {
		t.Fatalf("limited authorized adapter handoffs should still surface repair current action: queue=%+v items=%+v", queue, merged)
	}
}

func TestReadHandoffFactsUsesMissionLedgerSnapshot(t *testing.T) {
	caseRoot := t.TempDir()
	factsRoot := filepath.Join(caseRoot, ".rekit", "facts")
	if err := os.MkdirAll(factsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeHandoffFactLines(t, factsRoot, "observations.jsonl", `{"kind":"observation","lane":"main","subject":"obs","batchId":"batch-handoff-ledger"}`)
	writeHandoffFactLines(t, factsRoot, "candidates.jsonl", `{"kind":"candidate","lane":"main","subject":"candidate","status":"open","batchId":"batch-handoff-ledger"}`)
	writeHandoffFactLines(t, factsRoot, "requests.jsonl", `{"kind":"request","lane":"main","subject":"gate","status":"pending-gate","batchId":"batch-handoff-ledger"}`)
	writeHandoffFactLines(t, factsRoot, "publications.jsonl", `{"kind":"publication","lane":"main","subject":"pub","batchId":"batch-handoff-ledger"}`)
	writeHandoffFactLines(t, factsRoot, "decisions.jsonl", `{"kind":"decision","lane":"main","subject":"decision","decision":"defer","batchId":"batch-handoff-ledger"}`)
	writeHandoffFactLines(t, factsRoot, "hypotheses.jsonl", `{"kind":"hypothesis","lane":"main","subject":"hypothesis","batchId":"batch-handoff-ledger"}`)
	writeHandoffFactLines(t, factsRoot, "verifications.jsonl", `{"kind":"verification","lane":"main","subject":"verify","batchId":"batch-handoff-ledger"}`)
	writeHandoffFactLines(t, factsRoot, "interventions.jsonl", `{"kind":"intervention","lane":"main","subject":"intervention","status":"open","batchId":"batch-handoff-ledger"}`)
	writeHandoffFactLines(t, factsRoot, "rollbacks.jsonl", `{"kind":"rollback","lane":"main","subject":"rollback","batchId":"batch-handoff-ledger"}`)

	facts, err := readHandoffFacts(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts.Observations) != 1 || len(facts.Candidates) != 1 || len(facts.Requests) != 1 || len(facts.Publications) != 1 || len(facts.Decisions) != 1 || len(facts.Hypotheses) != 1 || len(facts.Verifications) != 1 || len(facts.Interventions) != 1 || len(facts.Rollbacks) != 1 {
		t.Fatalf("handoff facts did not read the shared ledger snapshot: %+v", facts)
	}
	if facts.PendingDecision != 1 || len(facts.AllBatchEvents) != 9 {
		t.Fatalf("handoff facts did not include shared ledger summaries: pending=%d batches=%d", facts.PendingDecision, len(facts.AllBatchEvents))
	}
}

func TestWriteProjectMissionBriefBindsCurrentLaneAuthority(t *testing.T) {
	lanes := []boardLane{{ID: "main", Authority: true, Status: "open", CurrentExecutor: "stale-executor", ExecutorGeneration: 1}}
	current := []Lane{{ID: "main", Authority: true, Status: "open", CurrentExecutor: "replacement-executor", ExecutorGeneration: 2}}
	var out bytes.Buffer
	writeProjectMissionBrief(&out, lanes, mission.LedgerFacts{}, current)
	text := out.String()
	want := "/rekit continue main -Executor replacement-executor -ExpectedExecutorGeneration 2"
	if !strings.Contains(text, want) || strings.Contains(text, "stale-executor") {
		t.Fatalf("project mission brief did not rebuild continue command from current lane authority:\n%s", text)
	}
}

func TestMissionCommanderNextActionsWithAuthorizedGateAdaptersOrdersMultipleGateStates(t *testing.T) {
	base := []mission.MissionCommanderNextActionItem{
		{Lane: "main", Label: "gate-recorded", GateEventID: "gate-recorded", State: "ready-for-evidence-review", Command: "/rekit handoff main", Source: "executionEvidenceReview", Blocked: true, RequiresReview: true},
		{Lane: "main", Label: "gate-recorded", GateEventID: "gate-recorded", State: "ready-for-evidence-review", Command: "/rekit overview", Source: "executionEvidenceReview.followUp", Blocked: true, RequiresReview: true},
		{Lane: "main", Label: "gate-changed", GateEventID: "gate-changed", State: "ready-for-evidence-review", Command: "/rekit handoff main", Source: "executionEvidenceReview", RequiresReview: true},
		{Lane: "main", Label: "gate-escalated", GateEventID: "gate-escalated", State: "needs-main-escalation", Command: "/rekit handoff main", Source: "executionEvidenceReview", Blocked: true, RequiresReview: true},
	}
	adapterAction := func(state, command, source string) []mission.MissionCommanderNextActionItem {
		return []mission.MissionCommanderNextActionItem{{Lane: "main", Label: "main", State: state, Command: command, Source: source, RequiresReview: true}}
	}
	handoffs := []AuthorizedGateAdapterHandoff{
		{
			EventID:                     "gate-recorded",
			ReportSummary:               &gate.AdapterReportHandoffSummary{State: "evidence-already-recorded", ReportPresent: true, Valid: true},
			missionCommanderNextActions: adapterAction("evidence-already-recorded", "/rekit handoff main", "adapterReportLiveSnapshot.recordedEvidence"),
		},
		{
			EventID:                     "gate-changed",
			ReportSummary:               &gate.AdapterReportHandoffSummary{State: "repair-adapter-report", ReportPresent: true, RequiresRepair: true},
			missionCommanderNextActions: adapterAction("repair-adapter-report", "add-boundary-marker", "adapterReportValidation.repairHints"),
		},
		{
			EventID:                     "gate-valid",
			ReportSummary:               &gate.AdapterReportHandoffSummary{State: "ready-to-record-evidence", ReportPresent: true, Valid: true, RecordReady: true},
			missionCommanderNextActions: adapterAction("ready-to-record-evidence", "/rekit gate -Apply -GateEventId gate-valid", "adapterReportValidation.missionCommanderAction"),
		},
		{
			EventID:                     "gate-escalated",
			ReportSummary:               &gate.AdapterReportHandoffSummary{State: "needs-main-escalation", ReportPresent: true, Valid: true, RequiresMainEscalation: true},
			missionCommanderNextActions: adapterAction("needs-main-escalation", "/rekit handoff main", "adapterReportLiveSnapshot.recordedEvidence"),
		},
	}

	items := MissionCommanderNextActionsWithAuthorizedGateAdapters(base, handoffs)
	if len(items) != 5 {
		t.Fatalf("unexpected merged action count: %+v", items)
	}
	if items[0].GateEventID != "gate-escalated" || items[0].State != "needs-main-escalation" || items[1].GateEventID != "gate-recorded" || items[1].Source != "executionEvidenceReview" || items[2].Source != "executionEvidenceReview.followUp" || items[3].State != "repair-adapter-report" || items[3].GateEventID != "gate-changed" || items[4].State != "ready-to-record-evidence" || items[4].GateEventID != "gate-valid" {
		t.Fatalf("merged queue did not preserve per-gate evidence and adapter ordering: %+v", items)
	}
	queue := mission.MissionCommanderActionQueueFor(items)
	if queue.CurrentAction == nil || queue.CurrentAction.GateEventID != "gate-escalated" || queue.CurrentAction.State != "needs-main-escalation" {
		t.Fatalf("main escalation evidence should be current: %+v", queue)
	}
	for _, item := range items[3:] {
		if !item.Blocked || !slices.Contains(item.Reasons, "another execution evidence item requires main review before this adapter action") {
			t.Fatalf("adapter action should remain visible but blocked by main review: %+v", item)
		}
	}
	if slices.ContainsFunc(items, func(item mission.MissionCommanderNextActionItem) bool {
		return item.Command == "/rekit continue main"
	}) {
		t.Fatalf("autonomous continue should not survive main escalation: %+v", items)
	}
}

func TestMissionCommanderNextActionsWithAuthorizedGateAdaptersKeepsEvidenceForMissingSidecar(t *testing.T) {
	base := []mission.MissionCommanderNextActionItem{
		{Lane: "main", Label: "gate-missing", GateEventID: "gate-missing", State: "needs-main-escalation", Command: "/rekit handoff main", Source: "executionEvidenceReview", Blocked: true, RequiresReview: true},
	}
	handoff := AuthorizedGateAdapterHandoff{
		EventID:                     "gate-missing",
		ReportSummary:               &gate.AdapterReportHandoffSummary{State: "needs-adapter-report-validation", RequiresValidation: true},
		missionCommanderNextActions: []mission.MissionCommanderNextActionItem{{Lane: "main", State: "needs-adapter-report-validation", Command: "/rekit gate -ValidateExecutionReport", Source: "adapterReportContract.missionCommanderAction", RequiresReview: true}},
	}

	items := MissionCommanderNextActionsWithAuthorizedGateAdapters(base, []AuthorizedGateAdapterHandoff{handoff})
	if len(items) != 2 || items[0].Source != "executionEvidenceReview" || items[1].Source != "adapterReportContract.missionCommanderAction" || !items[1].Blocked {
		t.Fatalf("missing sidecar should preserve evidence review and blocked adapter validation: %+v", items)
	}
}

func writeHandoffFactLines(t *testing.T, root, name string, lines ...string) {
	t.Helper()
	text := ""
	for _, line := range lines {
		text += line + "\n"
	}
	if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
