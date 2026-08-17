package subagents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuarantineReviewerResultRejectsRecreatedCanonicalBytes(t *testing.T) {
	requireReviewerResultExactMove(t, "regular-file")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	resultRoot := filepath.Join(root, ".rekit", "reviews", "ambiguity", "results")
	quarantineRoot := filepath.Join(resultRoot, "recoveries")
	if err := os.MkdirAll(quarantineRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(resultRoot, "shard.json")
	quarantinePath := filepath.Join(quarantineRoot, "shard-hash.json")
	guardPath := filepath.Join(quarantineRoot, "intent.json")
	if err := os.WriteFile(guardPath, []byte("intent"), 0o600); err != nil {
		t.Fatal(err)
	}
	expected := []byte(`{"conflict":true}`)
	if err := os.WriteFile(resultPath, expected, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := quarantineReviewerResult(root, resultPath, quarantinePath, guardPath, expected); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resultPath, expected, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := quarantineReviewerResult(root, resultPath, quarantinePath, guardPath, expected); err == nil || !strings.Contains(err.Error(), "cannot prove") {
		t.Fatalf("ambiguous regular recovery error = %v", err)
	}
	if data, err := os.ReadFile(resultPath); err != nil {
		t.Fatal(err)
	} else if string(data) != string(expected) {
		t.Fatalf("recreated canonical bytes changed: %q", data)
	}
}

func TestQuarantineReviewerResultRejectsRecreatedCanonicalObstruction(t *testing.T) {
	requireReviewerResultExactMove(t, "empty-file")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	resultRoot := filepath.Join(root, ".rekit", "reviews", "ambiguity", "results")
	quarantineRoot := filepath.Join(resultRoot, "recoveries")
	if err := os.MkdirAll(quarantineRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(resultRoot, "shard.json")
	quarantinePath := filepath.Join(quarantineRoot, "shard-hash.json")
	guardPath := filepath.Join(quarantineRoot, "intent.json")
	if err := os.WriteFile(guardPath, []byte("intent"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resultPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	expected, err := readReviewerResultObstruction(root, resultPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := quarantineReviewerResultObstruction(root, resultPath, quarantinePath, guardPath, expected); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resultPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := quarantineReviewerResultObstruction(root, resultPath, quarantinePath, guardPath, expected); err == nil || !strings.Contains(err.Error(), "cannot prove") {
		t.Fatalf("ambiguous obstruction recovery error = %v", err)
	}
	if st, err := os.Lstat(resultPath); err != nil {
		t.Fatal(err)
	} else if !st.Mode().IsRegular() || st.Size() != 0 {
		t.Fatalf("recreated canonical obstruction changed: mode=%v size=%d", st.Mode(), st.Size())
	}
}
