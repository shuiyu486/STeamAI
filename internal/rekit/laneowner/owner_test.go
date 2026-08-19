package laneowner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathUsesActiveStateRoot(t *testing.T) {
	for _, stateDir := range []string{".steamai", ".rekit"} {
		t.Run(stateDir, func(t *testing.T) {
			caseRoot := t.TempDir()
			if err := os.Mkdir(filepath.Join(caseRoot, stateDir), 0o755); err != nil {
				t.Fatal(err)
			}
			rel, full, err := Path(caseRoot, "main")
			if err != nil {
				t.Fatal(err)
			}
			wantRel := filepath.ToSlash(filepath.Join(stateDir, "lanes", "main", "lane.json"))
			if rel != wantRel || full != filepath.Join(caseRoot, filepath.FromSlash(wantRel)) {
				t.Fatalf("Path = %q, %q; want %q under active root", rel, full, wantRel)
			}
		})
	}
}

func TestPathRejectsDualStateRoots(t *testing.T) {
	caseRoot := t.TempDir()
	for _, stateDir := range []string{".steamai", ".rekit"} {
		if err := os.Mkdir(filepath.Join(caseRoot, stateDir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := Path(caseRoot, "main"); err == nil || !strings.Contains(err.Error(), "must not coexist") {
		t.Fatalf("Path error = %v, want dual-root rejection", err)
	}
}

func TestReadOptionalDistinguishesMissingCompleteAndIncompleteOwner(t *testing.T) {
	caseRoot := t.TempDir()
	laneRoot := filepath.Join(caseRoot, ".rekit", "lanes", "main")
	if err := os.MkdirAll(laneRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	lanePath := filepath.Join(laneRoot, "lane.json")
	if owner, found, err := ReadOptional(caseRoot, "absent"); err != nil || found || owner != (Snapshot{}) {
		t.Fatalf("missing lane = %+v found=%t err=%v", owner, found, err)
	}
	write := func(data string) {
		t.Helper()
		if err := os.WriteFile(lanePath, []byte(data+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write(`{"schemaVersion":1,"id":"main","status":"open"}`)
	if owner, found, err := ReadOptional(caseRoot, "main"); err != nil || found || owner != (Snapshot{}) {
		t.Fatalf("ownerless lane = %+v found=%t err=%v", owner, found, err)
	}

	write(`{"schemaVersion":1,"id":"main","status":"open","currentExecutor":"member-a","executorGeneration":2}`)
	owner, found, err := ReadOptional(caseRoot, "main")
	if err != nil || !found || owner != (Snapshot{Lane: "main", CurrentExecutor: "member-a", ExecutorGeneration: 2}) {
		t.Fatalf("complete owner = %+v found=%t err=%v", owner, found, err)
	}

	write(`{"schemaVersion":1,"id":"main","status":"open","currentExecutor":"member-a"}`)
	if _, _, err := ReadOptional(caseRoot, "main"); err == nil || !strings.Contains(err.Error(), "incomplete durable executor owner") {
		t.Fatalf("incomplete owner error = %v", err)
	}
}
