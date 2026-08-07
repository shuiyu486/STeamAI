package casehealth

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/caseshim"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
)

type Row struct {
	File  string `json:"file"`
	Bytes int64  `json:"bytes"`
	Limit int64  `json:"limit"`
}

// Static validates the attached-case files that do not depend on workstream
// runtime state. Callers that permit existing Mission Control state must add
// the corresponding board, lane, ledger, and autonomy validation themselves.
func Static(repoRoot, caseRoot, pack string) ([]Row, error) {
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
	readiness := caseshim.InspectInstalled(repoRoot, inst.CaseRoot)
	if !readiness.Ready {
		return nil, fmt.Errorf("installed case shim readiness failed: %s", strings.Join(readiness.Warnings, "; "))
	}
	if err := caseshim.AssertReady(repoRoot); err != nil {
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
	for _, route := range m.SubagentRoutes {
		if strings.TrimSpace(route.Reference) == "" {
			continue
		}
		if _, err := refsf.SafeJoin(inst.CaseRoot, route.Reference); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

func assertManagedBlock(path, blockID string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("missing managed block host: %s", path)
	}
	text := string(data)
	if !strings.Contains(text, "<!-- BEGIN "+blockID) {
		return fmt.Errorf("missing managed block begin marker in %s: %s", path, blockID)
	}
	if !strings.Contains(text, "<!-- END "+blockID+" -->") {
		return fmt.Errorf("missing managed block end marker in %s: %s", path, blockID)
	}
	return nil
}
