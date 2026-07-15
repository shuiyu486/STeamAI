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
	assertNoteNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl"))
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
