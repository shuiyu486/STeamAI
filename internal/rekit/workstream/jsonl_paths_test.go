package workstream

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestWorkstreamLocalJSONLHelpers(t *testing.T) {
	if got, want := LaneJSONLFileNames(), []string{"events.jsonl", "tasks.jsonl", "inbox.jsonl", "outbox.jsonl"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("lane JSONL files = %v, want %v", got, want)
	}
	if got, want := WorkspaceJSONLFileNames(), []string{"observations.jsonl", "requests.jsonl", "candidates.jsonl", "publications.jsonl"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("workspace JSONL files = %v, want %v", got, want)
	}
}

func TestLaneOutputJSONLPaths(t *testing.T) {
	laneRoot := filepath.Join("case", ".rekit", "lanes", "main")
	workspace := filepath.Join("case", "workstreams", "main")
	got := LaneOutputJSONLPaths(laneRoot, workspace)
	want := []string{
		filepath.Join(laneRoot, "outbox.jsonl"),
		filepath.Join(workspace, "observations.jsonl"),
		filepath.Join(workspace, "requests.jsonl"),
		filepath.Join(workspace, "candidates.jsonl"),
		filepath.Join(workspace, "publications.jsonl"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lane output JSONL paths = %v, want %v", got, want)
	}
}
