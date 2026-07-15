package mission

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
)

type Board struct {
	Lanes []BoardLane `json:"lanes"`
}

type BoardLane struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Authority bool   `json:"authority"`
	Workspace string `json:"workspace"`
	UpdatedAt string `json:"updatedAt"`
}

func ReadBoard(caseRoot string) (Board, error) {
	path, err := refsf.SafeJoin(caseRoot, ".rekit/board.json")
	if err != nil {
		return Board{}, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Board{}, err
	}
	var out Board
	if err := json.Unmarshal(b, &out); err != nil {
		return Board{}, fmt.Errorf("invalid board json %s: %w", path, err)
	}
	return out, nil
}

func ReadFacts(caseRoot string) (Facts, error) {
	var err error
	out := Facts{}
	if out.Candidates, err = ReadFactFile(caseRoot, "candidates.jsonl"); err != nil {
		return out, err
	}
	if out.Requests, err = ReadFactFile(caseRoot, "requests.jsonl"); err != nil {
		return out, err
	}
	if out.Decisions, err = ReadFactFile(caseRoot, "decisions.jsonl"); err != nil {
		return out, err
	}
	if out.Interventions, err = ReadFactFile(caseRoot, "interventions.jsonl"); err != nil {
		return out, err
	}
	return out, nil
}

func ReadFactFile(caseRoot, name string) ([]map[string]any, error) {
	factsRoot, err := refsf.SafeJoin(caseRoot, ".rekit/facts")
	if err != nil {
		return nil, err
	}
	return ReadJSONLineObjects(filepath.Join(factsRoot, name))
}

func ReadJSONLineObjects(path string) ([]map[string]any, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := []map[string]any{}
	for line := range strings.SplitSeq(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var item map[string]any
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func CaseBrief(caseRoot string, opts BuildOptions) (Brief, error) {
	board, err := ReadBoard(caseRoot)
	if err != nil {
		return Brief{}, err
	}
	facts, err := ReadFacts(caseRoot)
	if err != nil {
		return Brief{}, err
	}
	return BuildWithOptions(BoardLanes(board.Lanes), facts, opts), nil
}

func BoardLanes(lanes []BoardLane) []Lane {
	out := make([]Lane, 0, len(lanes))
	for _, lane := range lanes {
		out = append(out, Lane{ID: lane.ID, Label: BoardLaneLabel(lane), Status: lane.Status})
	}
	return out
}

func BoardLaneLabel(lane BoardLane) string {
	if lane.Authority || lane.ID == "main" {
		return "main"
	}
	if name, ok := strings.CutPrefix(lane.ID, "feature-"); ok {
		return name
	}
	return lane.ID
}
