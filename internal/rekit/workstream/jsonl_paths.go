package workstream

import "path/filepath"

func LaneJSONLFileNames() []string {
	return []string{"events.jsonl", "tasks.jsonl", "inbox.jsonl", "outbox.jsonl"}
}

func LaneJSONLPaths(laneRoot string) []string {
	paths := []string{}
	for _, name := range LaneJSONLFileNames() {
		paths = append(paths, filepath.Join(laneRoot, name))
	}
	return paths
}

func LaneEventsJSONLPath(laneRoot string) string {
	return filepath.Join(laneRoot, "events.jsonl")
}

func LaneTasksJSONLPath(laneRoot string) string {
	return filepath.Join(laneRoot, "tasks.jsonl")
}

func LaneInboxJSONLPath(laneRoot string) string {
	return filepath.Join(laneRoot, "inbox.jsonl")
}

func LaneOutboxJSONLPath(laneRoot string) string {
	return filepath.Join(laneRoot, "outbox.jsonl")
}

func WorkspaceJSONLFileNames() []string {
	return []string{"observations.jsonl", "requests.jsonl", "candidates.jsonl", "publications.jsonl"}
}

func WorkspaceJSONLPaths(workspace string) []string {
	paths := []string{}
	for _, name := range WorkspaceJSONLFileNames() {
		paths = append(paths, filepath.Join(workspace, name))
	}
	return paths
}

func LaneOutputJSONLPaths(laneRoot, workspace string) []string {
	paths := []string{LaneOutboxJSONLPath(laneRoot)}
	paths = append(paths, WorkspaceJSONLPaths(workspace)...)
	return paths
}
