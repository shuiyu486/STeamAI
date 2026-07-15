package mission

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
)

type Board struct {
	SchemaVersion        int         `json:"schemaVersion"`
	CaseRoot             string      `json:"caseRoot"`
	RepoRoot             string      `json:"repoRoot"`
	Pack                 string      `json:"pack"`
	AutomationMode       string      `json:"automationMode"`
	DefaultAuthorityLane string      `json:"defaultAuthorityLane"`
	Lanes                []BoardLane `json:"lanes"`
	FactsRoot            string      `json:"factsRoot"`
	UpdatedAt            string      `json:"updatedAt"`
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

type LedgerFacts struct {
	Facts
	Observations    []map[string]any
	Publications    []map[string]any
	Hypotheses      []map[string]any
	Verifications   []map[string]any
	Rollbacks       []map[string]any
	AllBatchEvents  []map[string]any
	PendingDecision int
}

func FactFileName(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	switch kind {
	case "observation":
		return "observations.jsonl"
	case "candidate":
		return "candidates.jsonl"
	case "request":
		return "requests.jsonl"
	case "publication":
		return "publications.jsonl"
	case "decision":
		return "decisions.jsonl"
	case "hypothesis":
		return "hypotheses.jsonl"
	case "verification":
		return "verifications.jsonl"
	case "intervention":
		return "interventions.jsonl"
	case "rollback":
		return "rollbacks.jsonl"
	default:
		return kind + "s.jsonl"
	}
}

func ReadFacts(caseRoot string) (Facts, error) {
	ledger, err := ReadLedgerFacts(caseRoot)
	if err != nil {
		return Facts{}, err
	}
	return ledger.Facts, nil
}

func ReadLedgerFacts(caseRoot string) (LedgerFacts, error) {
	return readLedgerFacts(caseRoot, ReadFactFile)
}

func ReadStrictLedgerFacts(caseRoot string) (LedgerFacts, error) {
	return readLedgerFacts(caseRoot, ReadStrictFactFile)
}

func readLedgerFacts(caseRoot string, readFact func(string, string) ([]map[string]any, error)) (LedgerFacts, error) {
	read := func(kind string) ([]map[string]any, error) {
		return readFact(caseRoot, FactFileName(kind))
	}
	var err error
	out := LedgerFacts{}
	if out.Observations, err = read("observation"); err != nil {
		return out, err
	}
	if out.Candidates, err = read("candidate"); err != nil {
		return out, err
	}
	if out.Requests, err = read("request"); err != nil {
		return out, err
	}
	if out.Publications, err = read("publication"); err != nil {
		return out, err
	}
	if out.Decisions, err = read("decision"); err != nil {
		return out, err
	}
	if out.Hypotheses, err = read("hypothesis"); err != nil {
		return out, err
	}
	if out.Verifications, err = read("verification"); err != nil {
		return out, err
	}
	if out.Interventions, err = read("intervention"); err != nil {
		return out, err
	}
	if out.Rollbacks, err = read("rollback"); err != nil {
		return out, err
	}
	out.PendingDecision = PendingDecisionCount(out.Decisions)
	out.AllBatchEvents = BatchEvents(out)
	return out, nil
}

func ReadFactFile(caseRoot, name string) ([]map[string]any, error) {
	factsRoot, err := refsf.SafeJoin(caseRoot, ".rekit/facts")
	if err != nil {
		return nil, err
	}
	return ReadJSONLineObjects(filepath.Join(factsRoot, name))
}

func ReadStrictFactFile(caseRoot, name string) ([]map[string]any, error) {
	factsRoot, err := refsf.SafeJoin(caseRoot, ".rekit/facts")
	if err != nil {
		return nil, err
	}
	return ReadStrictJSONLineObjects(filepath.Join(factsRoot, name))
}

func ReadJSONLineObjects(path string) ([]map[string]any, error) {
	return readJSONLineObjects(path, false)
}

func ReadStrictJSONLineObjects(path string) ([]map[string]any, error) {
	return readJSONLineObjects(path, true)
}

func readJSONLineObjects(path string, strict bool) ([]map[string]any, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return []map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	items := []map[string]any{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item map[string]any
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			if strict {
				return nil, fmt.Errorf("invalid JSONL %s: %w", path, err)
			}
			continue
		}
		items = append(items, item)
	}
	return items, scanner.Err()
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

type LaneGuardOptions struct {
	Command         string
	CaseInsensitive bool
}

func AssertBoardLane(caseRoot, lane string, opts LaneGuardOptions) error {
	board, err := ReadBoard(caseRoot)
	command := strings.TrimSpace(opts.Command)
	if command == "" {
		command = "command"
	}
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s requires .rekit/board.json to validate lane: %s", command, filepath.Join(caseRoot, ".rekit", "board.json"))
		}
		return err
	}
	known := BoardLaneIDs(board.Lanes)
	if BoardHasLane(board.Lanes, lane, opts.CaseInsensitive) {
		return nil
	}
	if len(known) == 0 {
		return fmt.Errorf("%s requires at least one lane in .rekit/board.json", command)
	}
	return fmt.Errorf("unknown lane %q; known: %s", lane, strings.Join(known, ","))
}

func BoardLaneIDs(lanes []BoardLane) []string {
	known := []string{}
	for _, item := range lanes {
		id := strings.TrimSpace(item.ID)
		if id != "" {
			known = append(known, id)
		}
	}
	return known
}

func BoardHasLane(lanes []BoardLane, lane string, caseInsensitive bool) bool {
	lane = strings.TrimSpace(lane)
	for _, item := range lanes {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		if caseInsensitive {
			if strings.EqualFold(id, lane) {
				return true
			}
			continue
		}
		if id == lane {
			return true
		}
	}
	return false
}

func PendingDecisionCount(decisions []map[string]any) int {
	pending := 0
	for _, decision := range decisions {
		status := strings.ToLower(strings.TrimSpace(Value(decision, "status")))
		decisionValue := strings.ToLower(strings.TrimSpace(FirstText(Value(decision, "decision"), Value(decision, "action"))))
		if decisionValue == "pending-user" || (status == "" && decisionValue == "defer") || (status != "" && !IsTerminalStatus(status)) {
			pending++
		}
	}
	return pending
}

func BatchEvents(facts LedgerFacts) []map[string]any {
	out := []map[string]any{}
	for _, list := range [][]map[string]any{facts.Observations, facts.Hypotheses, facts.Candidates, facts.Verifications, facts.Decisions, facts.Interventions, facts.Rollbacks, facts.Publications, facts.Requests} {
		for _, event := range list {
			if strings.TrimSpace(Value(event, "batchId")) != "" {
				out = append(out, event)
			}
		}
	}
	return out
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
