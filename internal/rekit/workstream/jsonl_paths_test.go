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

func TestLaneLocalJSONLPaths(t *testing.T) {
	laneRoot := filepath.Join("case", ".rekit", "lanes", "main")
	want := []string{
		filepath.Join(laneRoot, "events.jsonl"),
		filepath.Join(laneRoot, "tasks.jsonl"),
		filepath.Join(laneRoot, "inbox.jsonl"),
		filepath.Join(laneRoot, "outbox.jsonl"),
	}
	if got := LaneJSONLPaths(laneRoot); !reflect.DeepEqual(got, want) {
		t.Fatalf("lane local JSONL paths = %v, want %v", got, want)
	}
	gotSingles := []string{
		LaneEventsJSONLPath(laneRoot),
		LaneTasksJSONLPath(laneRoot),
		LaneInboxJSONLPath(laneRoot),
		LaneOutboxJSONLPath(laneRoot),
	}
	if !reflect.DeepEqual(gotSingles, want) {
		t.Fatalf("lane local JSONL single paths = %v, want %v", gotSingles, want)
	}
}

func TestLaneOutputJSONLPaths(t *testing.T) {
	laneRoot := filepath.Join("case", ".rekit", "lanes", "main")
	workspace := filepath.Join("case", "workstreams", "main")
	workspaceWant := []string{
		filepath.Join(workspace, "observations.jsonl"),
		filepath.Join(workspace, "requests.jsonl"),
		filepath.Join(workspace, "candidates.jsonl"),
		filepath.Join(workspace, "publications.jsonl"),
	}
	if got := WorkspaceJSONLPaths(workspace); !reflect.DeepEqual(got, workspaceWant) {
		t.Fatalf("workspace JSONL paths = %v, want %v", got, workspaceWant)
	}
	want := append([]string{filepath.Join(laneRoot, "outbox.jsonl")}, workspaceWant...)
	if got := LaneOutputJSONLPaths(laneRoot, workspace); !reflect.DeepEqual(got, want) {
		t.Fatalf("lane output JSONL paths = %v, want %v", got, want)
	}
}
