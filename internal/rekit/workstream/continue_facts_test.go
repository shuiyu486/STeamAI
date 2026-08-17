package workstream

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestWouldFactKindsUsesMissionFactPaths(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := wouldFactKinds(caseRoot, "candidate", "decision")
	if err != nil {
		t.Fatal(err)
	}
	want := []StartWrite{
		{Path: mission.FactRelPath("candidate"), Kind: "fact-jsonl", Action: "would-append"},
		{Path: mission.FactRelPath("decision"), Kind: "fact-jsonl", Action: "would-append"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("would fact writes = %+v, want %+v", got, want)
	}
}

func TestWouldFactKindsRejectsConflictingStateRoots(t *testing.T) {
	caseRoot := t.TempDir()
	for _, root := range []string{".steamai", ".rekit"} {
		if err := os.MkdirAll(filepath.Join(caseRoot, root), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := wouldFactKinds(caseRoot, "decision"); err == nil {
		t.Fatal("conflicting state roots were not rejected")
	}
}

func TestAppendContinueFactUsesMissionFactPath(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx := continueContext{inst: instance.Instance{CaseRoot: caseRoot}}
	writes := []StartWrite{}
	if err := ctx.appendContinueFact(&writes, "request", map[string]any{"kind": "request", "eventId": "evt-test"}); err != nil {
		t.Fatal(err)
	}
	wantRel := mission.FactRelPath("request")
	wantPath := filepath.Join(caseRoot, filepath.FromSlash(wantRel))
	if len(writes) != 1 || writes[0].Path != wantRel || writes[0].Kind != "fact-jsonl" || writes[0].Action != "append" || writes[0].TargetPath != wantPath {
		t.Fatalf("writes = %+v, want rel=%s path=%s", writes, wantRel, wantPath)
	}
	items, err := mission.ReadJSONLineObjects(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || mission.Value(items[0], "eventId") != "evt-test" {
		t.Fatalf("appended facts = %+v", items)
	}
}
