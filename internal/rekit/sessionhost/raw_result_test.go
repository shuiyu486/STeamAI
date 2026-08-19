package sessionhost

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureClaudeRawResultArtifactReplaysAndRejectsTamper(t *testing.T) {
	caseRoot := t.TempDir()
	run := claudeRun{
		envelope:         claudeEnvelope{Type: "result", Subtype: "success", SessionID: "session-a"},
		sessionID:        "session-a",
		structuredOutput: []byte(`{"outcome":"returned"}`),
		exitCode:         0,
		observedAt:       "2026-08-18T14:00:00Z",
	}
	first, err := ensureClaudeRawResultArtifact(caseRoot, run)
	if err != nil {
		t.Fatal(err)
	}
	if first.rawResultRef == "" || first.rawResultSHA256 == "" || first.rawResultBytes != int64(len(run.structuredOutput)) {
		t.Fatalf("raw artifact identity=%+v", first)
	}
	replayed, err := ensureClaudeRawResultArtifact(caseRoot, first)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.rawResultRef != first.rawResultRef || replayed.rawResultSHA256 != first.rawResultSHA256 || replayed.rawResultBytes != first.rawResultBytes {
		t.Fatalf("raw artifact replay drifted: first=%+v replay=%+v", first, replayed)
	}
	rel, err := hostCacheArtifactRel(first.rawResultRef)
	if err != nil {
		t.Fatal(err)
	}
	root, err := hostCacheRoot()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, run.structuredOutput) {
		t.Fatalf("raw artifact bytes=%q want=%q", original, run.structuredOutput)
	}
	if err := os.WriteFile(path, []byte(`{"tampered":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureClaudeRawResultArtifact(caseRoot, first); err == nil || !strings.Contains(err.Error(), "no longer matches") {
		t.Fatalf("tampered raw artifact error=%v", err)
	}
}
