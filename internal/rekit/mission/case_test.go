package mission

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCaseBriefReadsBoardAndFacts(t *testing.T) {
	caseRoot := t.TempDir()
	factsRoot := filepath.Join(caseRoot, ".rekit", "facts")
	if err := os.MkdirAll(factsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	board := `{
  "lanes": [
    {"id": "main", "status": "active", "authority": true},
    {"id": "feature-login", "status": "active"},
    {"id": "feature-done", "status": "closed"}
  ]
}`
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), []byte(board), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(factsRoot, "requests.jsonl"), []byte(`{"kind":"request","lane":"feature-login","subject":"debug gate","status":"pending-gate","risk":"high"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(factsRoot, "candidates.jsonl"), []byte(`{"kind":"candidate","lane":"feature-login","subject":"authority candidate","status":"open"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	brief, err := CaseBrief(caseRoot, BuildOptions{MaxRows: 10, OpenDecisionAction: "review open candidate/decision item(s) with evidence and authority boundary"})
	if err != nil {
		t.Fatal(err)
	}
	if brief.Summary != "openLanes=2 ready=1 blocked=1 pendingGates=1 openDecisions=1 interventions=0" {
		t.Fatalf("summary = %q", brief.Summary)
	}
	if !slices.Contains(brief.ReadyLanes, "main") || !slices.Contains(brief.BlockedLanes, "login (pending-gate,open-decision)") {
		t.Fatalf("unexpected lanes: ready=%v blocked=%v", brief.ReadyLanes, brief.BlockedLanes)
	}
	if !containsSubstring(brief.PendingGates, "debug gate") || !containsSubstring(brief.OpenDecisions, "candidate: authority candidate") {
		t.Fatalf("unexpected brief details: gates=%v decisions=%v", brief.PendingGates, brief.OpenDecisions)
	}
}

func TestBoardLaneLabelTreatsMainIDAsMain(t *testing.T) {
	label := BoardLaneLabel(BoardLane{ID: "main", Authority: false})
	if label != "main" {
		t.Fatalf("label = %q", label)
	}
	if got := BoardLaneLabel(BoardLane{ID: "feature-login"}); got != "login" {
		t.Fatalf("feature label = %q", got)
	}
}

func TestReadJSONLineObjectsSkipsBlankAndInvalidLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	content := strings.Join([]string{
		`{"kind":"candidate","subject":"first"}`,
		`not json`,
		``,
		`{"kind":"request","subject":"second"}`,
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	items, err := ReadJSONLineObjects(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || Value(items[0], "subject") != "first" || Value(items[1], "subject") != "second" {
		t.Fatalf("items = %#v", items)
	}
	if _, err := ReadStrictJSONLineObjects(path); err == nil || !strings.Contains(err.Error(), "invalid JSONL") {
		t.Fatalf("strict JSONL error = %v", err)
	}
}

func TestReadStrictLedgerFactsSummarizesPendingAndBatches(t *testing.T) {
	caseRoot := t.TempDir()
	factsRoot := filepath.Join(caseRoot, ".rekit", "facts")
	if err := os.MkdirAll(factsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(factsRoot, "decisions.jsonl"), []byte(strings.Join([]string{
		`{"kind":"decision","subject":"pending user","action":"pending-user","batchId":"b2"}`,
		`{"kind":"decision","subject":"deferred terminal","status":"deferred","decision":"defer","batchId":"b1"}`,
	}, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(factsRoot, "requests.jsonl"), []byte(`{"kind":"request","subject":"debug gate","status":"pending-gate","batchId":"b2"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	facts, err := ReadStrictLedgerFacts(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if facts.PendingDecision != 1 {
		t.Fatalf("pending decisions = %d", facts.PendingDecision)
	}
	if len(facts.AllBatchEvents) != 3 {
		t.Fatalf("batch events = %#v", facts.AllBatchEvents)
	}
}
