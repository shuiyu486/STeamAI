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
	if brief.Summary != "openLanes=2 ready=1 blocked=1 pendingGates=1 authorizedGates=0 openDecisions=1 interventions=0" {
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

func TestFactFileNameMapsLedgerKinds(t *testing.T) {
	cases := map[string]string{
		"observation":  "observations.jsonl",
		"candidate":    "candidates.jsonl",
		"request":      "requests.jsonl",
		"publication":  "publications.jsonl",
		"decision":     "decisions.jsonl",
		"hypothesis":   "hypotheses.jsonl",
		"verification": "verifications.jsonl",
		"intervention": "interventions.jsonl",
		"rollback":     "rollbacks.jsonl",
		"custom":       "customs.jsonl",
	}
	for kind, want := range cases {
		if got := FactFileName(kind); got != want {
			t.Fatalf("FactFileName(%q) = %q, want %q", kind, got, want)
		}
	}
	if got := strings.Join(FactFileNames(), ","); got != "observations.jsonl,candidates.jsonl,requests.jsonl,publications.jsonl,decisions.jsonl,hypotheses.jsonl,verifications.jsonl,interventions.jsonl,rollbacks.jsonl" {
		t.Fatalf("FactFileNames = %q", got)
	}
	if got := FactRelPath("decision"); got != ".rekit/facts/decisions.jsonl" {
		t.Fatalf("FactRelPath(decision) = %q", got)
	}
}

func TestAssertBoardLaneUsesSharedKnownLaneErrors(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	board := `{"lanes":[{"id":"main"},{"id":"FEATURE-Debug"},{"id":""}]}`
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), []byte(board), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AssertBoardLane(caseRoot, "main", LaneGuardOptions{Command: "note"}); err != nil {
		t.Fatalf("main lane error = %v", err)
	}
	if err := AssertBoardLane(caseRoot, "feature-debug", LaneGuardOptions{Command: "gate", CaseInsensitive: true}); err != nil {
		t.Fatalf("case-insensitive lane error = %v", err)
	}
	if err := AssertBoardLane(caseRoot, "feature-debug", LaneGuardOptions{Command: "note"}); err == nil || !strings.Contains(err.Error(), "unknown lane") || !strings.Contains(err.Error(), "FEATURE-Debug") {
		t.Fatalf("case-sensitive unknown lane error = %v", err)
	}
}

func TestLookupBoardLaneSharesCaseSensitivityRules(t *testing.T) {
	lanes := []BoardLane{{ID: "main", Authority: true}, {ID: "FEATURE-Debug", Status: "open"}}
	if lane, ok := LookupBoardLane(lanes, "main", false); !ok || lane.ID != "main" {
		t.Fatalf("exact lookup = %+v, %t", lane, ok)
	}
	if _, ok := LookupBoardLane(lanes, "feature-debug", false); ok {
		t.Fatal("case-sensitive lookup unexpectedly matched")
	}
	if lane, ok := LookupBoardLane(lanes, "feature-debug", true); !ok || lane.ID != "FEATURE-Debug" {
		t.Fatalf("case-insensitive lookup = %+v, %t", lane, ok)
	}
}

func TestAssertBoardLaneReportsMissingAndEmptyBoard(t *testing.T) {
	missingRoot := t.TempDir()
	if err := AssertBoardLane(missingRoot, "main", LaneGuardOptions{Command: "note"}); err == nil || !strings.Contains(err.Error(), "note requires .rekit/board.json") {
		t.Fatalf("missing board error = %v", err)
	}

	emptyRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(emptyRoot, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(emptyRoot, ".rekit", "board.json"), []byte(`{"lanes":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AssertBoardLane(emptyRoot, "main", LaneGuardOptions{Command: "gate"}); err == nil || !strings.Contains(err.Error(), "gate requires at least one lane") {
		t.Fatalf("empty board error = %v", err)
	}
}

func TestOpenBoardLanesUsesSharedStatusRules(t *testing.T) {
	lanes := OpenBoardLanes([]BoardLane{
		{ID: "main", Status: "active"},
		{ID: "paused-lane", Status: "paused"},
		{ID: "archived-lane", Status: "archived"},
		{ID: "closed-lane", Status: "closed"},
		{ID: "blank-status"},
	})
	if len(lanes) != 2 || lanes[0].ID != "main" || lanes[1].ID != "blank-status" {
		t.Fatalf("open board lanes = %#v", lanes)
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

func TestAppendJSONLineCreatesAndAppendsCRLFJSONLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := AppendJSONLine(path, map[string]any{"kind": "candidate", "eventId": "evt-one"}); err != nil {
		t.Fatal(err)
	}
	if err := AppendJSONLine(path, map[string]any{"kind": "request", "eventId": "evt-two"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if !strings.Contains(text, "\r\n") || !strings.Contains(text, `"evt-one"`) || !strings.Contains(text, `"evt-two"`) {
		t.Fatalf("appended JSONL = %q", text)
	}
	items, err := ReadStrictJSONLineObjects(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || Value(items[0], "eventId") != "evt-one" || Value(items[1], "eventId") != "evt-two" {
		t.Fatalf("items = %#v", items)
	}
}

func TestFactPathUsesSharedMappingAndSafeJoin(t *testing.T) {
	caseRoot := t.TempDir()
	rel, path, err := FactPath(caseRoot, "decision")
	if err != nil {
		t.Fatal(err)
	}
	if rel != ".rekit/facts/decisions.jsonl" || path != filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl") {
		t.Fatalf("fact path = (%q, %q)", rel, path)
	}
	for _, kind := range []string{"../outside", `bad\\kind`} {
		if _, _, err := FactPath(caseRoot, kind); err == nil {
			t.Fatalf("unsafe fact kind %q did not error", kind)
		}
	}
}

func TestAppendFactCreatesSharedFactPathAndAppendsJSONLine(t *testing.T) {
	caseRoot := t.TempDir()
	rel, path, err := AppendFact(caseRoot, "request", map[string]any{"kind": "request", "eventId": "evt-request"})
	if err != nil {
		t.Fatal(err)
	}
	if rel != ".rekit/facts/requests.jsonl" || path != filepath.Join(caseRoot, ".rekit", "facts", "requests.jsonl") {
		t.Fatalf("append fact path = (%q, %q)", rel, path)
	}
	items, err := ReadStrictFact(caseRoot, "request")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || Value(items[0], "eventId") != "evt-request" {
		t.Fatalf("request facts = %#v", items)
	}
}

func TestReadFactUsesSharedFactPathAndStrictVariant(t *testing.T) {
	caseRoot := t.TempDir()
	factsRoot := filepath.Join(caseRoot, ".rekit", "facts")
	if err := os.MkdirAll(factsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(factsRoot, "hypotheses.jsonl"), []byte("not json\n"+`{"kind":"hypothesis","eventId":"evt-hypothesis"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	items, err := ReadFact(caseRoot, "hypothesis")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || Value(items[0], "eventId") != "evt-hypothesis" {
		t.Fatalf("hypothesis facts = %#v", items)
	}
	if _, err := ReadStrictFact(caseRoot, "hypothesis"); err == nil || !strings.Contains(err.Error(), "invalid JSONL") {
		t.Fatalf("strict fact error = %v", err)
	}
	if _, err := ReadFact(caseRoot, "../outside"); err == nil {
		t.Fatalf("unsafe read fact kind did not error")
	}
}

func TestReadFactEventIDsUsesSharedFactReader(t *testing.T) {
	caseRoot := t.TempDir()
	factsRoot := filepath.Join(caseRoot, ".rekit", "facts")
	if err := os.MkdirAll(factsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(factsRoot, "requests.jsonl"), []byte("not json\n"+`{"kind":"request","eventId":"evt-request"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	known, err := ReadFactEventIDs(caseRoot, "request")
	if err != nil {
		t.Fatal(err)
	}
	if !known["evt-request"] || len(known) != 1 {
		t.Fatalf("request event ids = %#v", known)
	}
	if _, err := ReadStrictFactEventIDs(caseRoot, "request"); err == nil || !strings.Contains(err.Error(), "invalid JSONL") {
		t.Fatalf("strict fact event ids error = %v", err)
	}
}

func TestValidateJSONLinesReportsLineNumbersAndSkipsMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.jsonl")
	if err := ValidateJSONLines(missing); err != nil {
		t.Fatalf("missing JSONL validation error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "events.jsonl")
	content := strings.Join([]string{
		string(rune(0xFEFF)) + `{"kind":"candidate","subject":"first"}`,
		``,
		`not json`,
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateJSONLines(path); err == nil || !strings.Contains(err.Error(), "malformed jsonl line") || !strings.Contains(err.Error(), ":3") {
		t.Fatalf("validation error = %v", err)
	}
}

func TestReadLedgerEventIDsUsesSharedFactsMapping(t *testing.T) {
	caseRoot := t.TempDir()
	factsRoot := filepath.Join(caseRoot, ".rekit", "facts")
	if err := os.MkdirAll(factsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(factsRoot, "observations.jsonl"), []byte(`{"kind":"observation","eventId":"evt-observed"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(factsRoot, "requests.jsonl"), []byte("not json\n"+`{"kind":"request","eventId":"evt-request"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	known, err := ReadLedgerEventIDs(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !known["evt-observed"] || !known["evt-request"] || len(known) != 2 {
		t.Fatalf("known event ids = %#v", known)
	}
}

func TestReadStrictLedgerEventIDsReturnsMalformedJSONLError(t *testing.T) {
	caseRoot := t.TempDir()
	factsRoot := filepath.Join(caseRoot, ".rekit", "facts")
	if err := os.MkdirAll(factsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(factsRoot, "observations.jsonl"), []byte("not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadStrictLedgerEventIDs(caseRoot); err == nil || !strings.Contains(err.Error(), "invalid JSONL") {
		t.Fatalf("strict ledger event ids error = %v", err)
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
