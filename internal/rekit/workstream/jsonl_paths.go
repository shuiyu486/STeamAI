package workstream

import "path/filepath"

func LaneJSONLFileNames() []string {
	return []string{"events.jsonl", "tasks.jsonl", "inbox.jsonl", "outbox.jsonl"}
}

func WorkspaceJSONLFileNames() []string {
	return []string{"observations.jsonl", "requests.jsonl", "candidates.jsonl", "publications.jsonl"}
}

func LaneOutputJSONLPaths(laneRoot, workspace string) []string {
	paths := []string{filepath.Join(laneRoot, "outbox.jsonl")}
	for _, name := range WorkspaceJSONLFileNames() {
		paths = append(paths, filepath.Join(workspace, name))
	}
	return paths
}
