package workstream

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

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
