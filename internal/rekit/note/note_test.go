package note

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestAppendDuplicateExplicitEventIDDoesNotAppend(t *testing.T) {
	repoRoot, caseRoot, pack := noteFixture(t)
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

	first, err := Append(repoRoot, caseRoot, pack, opt, false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Append(repoRoot, caseRoot, pack, opt, false)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Applied || second.Applied || second.EventID != first.EventID || second.Reason != "duplicate eventId" {
		t.Fatalf("unexpected duplicate append results: first=%+v second=%+v", first, second)
	}
	lines := readNoteLines(t, filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl"))
	if len(lines) != 1 {
		t.Fatalf("duplicate append wrote %d lines, want 1: %q", len(lines), lines)
	}
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

func readNoteLines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.TrimSpace(strings.ReplaceAll(string(b), "\r\n", "\n"))
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
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
