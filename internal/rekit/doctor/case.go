package doctor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func Case(repoRoot, caseRoot, pack string) ([]Row, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return nil, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return nil, err
	}
	if err := m.ValidateSchema(); err != nil {
		return nil, err
	}
	rows := []Row{}
	addRequired := func(path string, limit int64) error {
		size, err := refsf.IsTextNonEmptyUnder(path, limit)
		if err != nil {
			return err
		}
		rows = append(rows, Row{File: path, Bytes: size, Limit: limit})
		return nil
	}
	addIfExists := func(path string, limit int64) error {
		if !refsf.Exists(path) {
			return nil
		}
		return addRequired(path, limit)
	}

	if err := addIfExists(filepath.Join(inst.CaseRoot, ".rekit", "instance.yml"), 8192); err != nil {
		return nil, err
	}
	if err := addIfExists(filepath.Join(inst.CaseRoot, ".re-template.yml"), 8192); err != nil {
		return nil, err
	}
	shim := filepath.Join(inst.CaseRoot, ".claude", "skills", "rekit", "SKILL.md")
	if err := addRequired(shim, 16384); err != nil {
		return nil, err
	}
	canonicalSkill := filepath.Join(repoRoot, ".claude", "skills", "rekit", "SKILL.md")
	if err := addRequired(canonicalSkill, 32768); err != nil {
		return nil, err
	}
	if err := assertCaseShimMatchesTemplate(inst.CaseRoot, repoRoot); err != nil {
		return nil, err
	}

	for _, rel := range m.ManagedFiles {
		path, err := refsf.SafeJoin(inst.CaseRoot, rel)
		if err != nil {
			return nil, err
		}
		if err := addRequired(path, m.BudgetLimit(rel)); err != nil {
			return nil, err
		}
	}
	for _, rel := range m.TemplateFiles {
		targetRel := strings.TrimSuffix(rel, ".template.md") + ".md"
		path, err := refsf.SafeJoin(inst.CaseRoot, targetRel)
		if err != nil {
			return nil, err
		}
		if err := addRequired(path, m.BudgetLimit(targetRel)); err != nil {
			return nil, err
		}
	}
	blockHost, err := refsf.SafeJoin(inst.CaseRoot, m.ManagedBlock["file"])
	if err != nil {
		return nil, err
	}
	if err := assertManagedBlock(blockHost, m.ManagedBlock["blockId"]); err != nil {
		return nil, err
	}
	if err := validateSubagentRoutesInstance(m, inst.CaseRoot); err != nil {
		return nil, err
	}
	if err := validateWorkstreamState(inst.CaseRoot); err != nil {
		return nil, err
	}
	return rows, nil
}

func assertCaseShimMatchesTemplate(caseRoot, repoRoot string) error {
	shim := filepath.Join(caseRoot, ".claude", "skills", "rekit", "SKILL.md")
	template := filepath.Join(repoRoot, "rekit", "templates", "case-shim", "SKILL.md")
	left, err := os.ReadFile(shim)
	if err != nil {
		return err
	}
	right, err := os.ReadFile(template)
	if err != nil {
		return err
	}
	if !bytes.Equal(left, right) {
		return fmt.Errorf("case-local /rekit shim differs from canonical thin shim template: %s", shim)
	}
	return nil
}

func assertManagedBlock(path, blockID string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("missing managed block host: %s", path)
	}
	text := string(b)
	if !strings.Contains(text, "<!-- BEGIN "+blockID) {
		return fmt.Errorf("missing managed block begin marker in %s: %s", path, blockID)
	}
	if !strings.Contains(text, "<!-- END "+blockID+" -->") {
		return fmt.Errorf("missing managed block end marker in %s: %s", path, blockID)
	}
	return nil
}

func validateSubagentRoutesInstance(m *manifest.Manifest, caseRoot string) error {
	for _, route := range m.SubagentRoutes {
		if strings.TrimSpace(route.Reference) == "" {
			continue
		}
		if _, err := refsf.SafeJoin(caseRoot, route.Reference); err != nil {
			return err
		}
	}
	return nil
}

func validateWorkstreamState(caseRoot string) error {
	facts := []string{
		filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"),
		filepath.Join(caseRoot, ".rekit", "facts", "candidates.jsonl"),
		filepath.Join(caseRoot, ".rekit", "facts", "requests.jsonl"),
		filepath.Join(caseRoot, ".rekit", "facts", "publications.jsonl"),
		filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl"),
		filepath.Join(caseRoot, ".rekit", "facts", "hypotheses.jsonl"),
		filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl"),
		filepath.Join(caseRoot, ".rekit", "facts", "interventions.jsonl"),
		filepath.Join(caseRoot, ".rekit", "facts", "rollbacks.jsonl"),
	}
	for _, path := range facts {
		if err := mission.ValidateJSONLines(path); err != nil {
			return err
		}
	}
	boardPath := filepath.Join(caseRoot, ".rekit", "board.json")
	board, exists, err := readJSONMapIfExists(boardPath)
	if err != nil {
		return err
	}
	if exists && board != nil {
		if recorded := jsonString(board, "caseRoot"); recorded != "" && !samePath(recorded, caseRoot) {
			return fmt.Errorf("board caseRoot mismatch: %s", recorded)
		}
	}
	lanesRoot := filepath.Join(caseRoot, ".rekit", "lanes")
	entries, err := os.ReadDir(lanesRoot)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		laneRoot := filepath.Join(lanesRoot, entry.Name())
		lanePath := filepath.Join(laneRoot, "lane.json")
		if !refsf.Exists(lanePath) {
			continue
		}
		lane, exists, err := readJSONMapIfExists(lanePath)
		if err != nil {
			return err
		}
		if !exists || lane == nil {
			return fmt.Errorf("empty lane file: %s", lanePath)
		}
		id := jsonString(lane, "id")
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("lane is missing id: %s", lanePath)
		}
		if !strings.EqualFold(id, entry.Name()) {
			return fmt.Errorf("lane id does not match directory: %s", lanePath)
		}
		workspaceRel := jsonString(lane, "workspace")
		if strings.TrimSpace(workspaceRel) == "" {
			return fmt.Errorf("lane is missing workspace: %s", lanePath)
		}
		workspace, err := refsf.SafeJoin(caseRoot, workspaceRel)
		if err != nil {
			return err
		}
		if laneRootRel := jsonString(lane, "laneRoot"); strings.TrimSpace(laneRootRel) != "" {
			if _, err := refsf.SafeJoin(caseRoot, laneRootRel); err != nil {
				return err
			}
		}
		for _, name := range []string{"events.jsonl", "tasks.jsonl", "inbox.jsonl", "outbox.jsonl"} {
			if err := mission.ValidateJSONLines(filepath.Join(laneRoot, name)); err != nil {
				return err
			}
		}
		for _, name := range []string{"observations.jsonl", "requests.jsonl", "candidates.jsonl", "publications.jsonl"} {
			if err := mission.ValidateJSONLines(filepath.Join(workspace, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func readJSONMapIfExists(path string) (map[string]any, bool, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	text := trimUTF8BOM(string(b))
	if strings.TrimSpace(text) == "" {
		return nil, true, nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return nil, true, fmt.Errorf("malformed json file: %s :: %w", path, err)
	}
	return out, true, nil
}

func trimUTF8BOM(value string) string {
	return strings.TrimPrefix(value, string(rune(0xFEFF)))
}

func jsonString(values map[string]any, key string) string {
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprint(value)
}

func samePath(a, b string) bool {
	left, err := filepath.Abs(a)
	if err != nil {
		return false
	}
	right, err := filepath.Abs(b)
	if err != nil {
		return false
	}
	left = strings.TrimRight(filepath.Clean(left), string(filepath.Separator))
	right = strings.TrimRight(filepath.Clean(right), string(filepath.Separator))
	return strings.EqualFold(left, right)
}
