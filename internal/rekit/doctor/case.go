package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	"github.com/shuiyu486/re-context-kits/internal/rekit/casehealth"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func Case(repoRoot, caseRoot, pack string) ([]Row, error) {
	staticRows, err := casehealth.Static(repoRoot, caseRoot, pack)
	if err != nil {
		return nil, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return nil, err
	}
	if err := validateWorkstreamState(caseRoot, m); err != nil {
		return nil, err
	}
	rows := make([]Row, 0, len(staticRows))
	for _, row := range staticRows {
		rows = append(rows, Row{File: row.File, Bytes: row.Bytes, Limit: row.Limit})
	}
	return rows, nil
}

func validateWorkstreamState(caseRoot string, m *manifest.Manifest) error {
	factRelPaths, err := mission.FactRelPathsFor(caseRoot)
	if err != nil {
		return err
	}
	for _, rel := range factRelPaths {
		path := filepath.Join(caseRoot, filepath.FromSlash(rel))
		if err := mission.ValidateJSONLines(path); err != nil {
			return err
		}
	}
	boardPath, err := projectstate.Join(caseRoot, "board.json")
	if err != nil {
		return err
	}
	board, exists, err := readJSONMapIfExists(boardPath)
	if err != nil {
		return err
	}
	if exists && board != nil {
		if recorded := jsonString(board, "caseRoot"); recorded != "" && !samePath(recorded, caseRoot) {
			return fmt.Errorf("board caseRoot mismatch: %s", recorded)
		}
	}
	lanesRoot, err := projectstate.Join(caseRoot, "lanes")
	if err != nil {
		return err
	}
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
		profile, _, exists, err := autonomy.Read(caseRoot, id)
		if err != nil {
			return err
		}
		if exists {
			if err := autonomy.Validate(profile, id, m, caseRoot); err != nil {
				return err
			}
		}
		for _, path := range workstream.LaneJSONLPaths(laneRoot) {
			if err := mission.ValidateJSONLines(path); err != nil {
				return err
			}
		}
		for _, path := range workstream.WorkspaceJSONLPaths(workspace) {
			if err := mission.ValidateJSONLines(path); err != nil {
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
