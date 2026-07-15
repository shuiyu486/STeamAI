package workstream

import (
	"os"
	"path/filepath"
	"testing"
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
