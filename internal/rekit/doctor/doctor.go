package doctor

import (
	"fmt"
	"path/filepath"
	"strings"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
)

type Row struct {
	File  string `json:"file"`
	Bytes int64  `json:"bytes"`
	Limit int64  `json:"limit"`
}

func Pack(repoRoot, pack string) ([]Row, error) {
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return nil, err
	}
	rows := []Row{}
	add := func(path string, limit int64) error {
		size, err := refsf.IsTextNonEmptyUnder(path, limit)
		if err != nil {
			return err
		}
		rows = append(rows, Row{File: path, Bytes: size, Limit: limit})
		return nil
	}
	if err := add(m.ManifestPath, 16384); err != nil {
		return nil, err
	}
	if err := add(filepath.Join(repoRoot, ".claude", "skills", "rekit", "SKILL.md"), 32768); err != nil {
		return nil, err
	}
	if err := add(filepath.Join(repoRoot, "rekit", "templates", "case-shim", "SKILL.md"), 16384); err != nil {
		return nil, err
	}
	if err := m.ValidateSchema(); err != nil {
		return nil, err
	}
	policyRows, err := validatePolicyRegistry(m)
	if err != nil {
		return nil, err
	}
	rows = append(rows, policyRows...)
	for _, rel := range m.ManagedFiles {
		path, err := m.SourcePath(rel)
		if err != nil {
			return nil, err
		}
		if err := add(path, m.BudgetLimit(rel)); err != nil {
			return nil, err
		}
	}
	for _, rel := range m.TemplateFiles {
		path, err := m.SourcePath(rel)
		if err != nil {
			return nil, err
		}
		targetRel := strings.TrimSuffix(rel, ".template.md") + ".md"
		if err := add(path, m.BudgetLimit(targetRel)); err != nil {
			return nil, err
		}
	}
	for _, rel := range m.ToolingFiles {
		path, err := m.SourcePath(rel)
		if err != nil {
			return nil, err
		}
		if err := add(path, m.BudgetLimit(rel)); err != nil {
			return nil, err
		}
	}
	for _, rel := range m.PromptFiles {
		path, err := m.RepoPath(rel)
		if err != nil {
			return nil, err
		}
		if err := add(path, 16384); err != nil {
			return nil, err
		}
	}
	blockSource, err := m.SourcePath(m.ManagedBlock["source"])
	if err != nil {
		return nil, err
	}
	if err := add(blockSource, 8192); err != nil {
		return nil, err
	}
	return rows, nil
}

func validatePolicyRegistry(m *manifest.Manifest) ([]Row, error) {
	rows := []Row{}
	add := func(path string, limit int64) error {
		size, err := refsf.IsTextNonEmptyUnder(path, limit)
		if err != nil {
			return err
		}
		rows = append(rows, Row{File: path, Bytes: size, Limit: limit})
		return nil
	}
	commonManifest := filepath.Join(m.RepoRoot, "common", "policies", "manifest.yml")
	if err := add(commonManifest, 16384); err != nil {
		return nil, err
	}
	if err := add(filepath.Join(m.RepoRoot, "common", "policies", "README.md"), 16384); err != nil {
		return nil, err
	}
	commonEntries, err := manifest.ObjectListFromFile(commonManifest, "policies")
	if err != nil {
		return nil, err
	}
	commonIDs := map[string]bool{}
	for _, entry := range commonEntries {
		commonIDs[entry["id"]] = true
		if rel := entry["path"]; rel != "" {
			if err := add(filepath.Join(filepath.Dir(commonManifest), filepath.FromSlash(rel)), 16384); err != nil {
				return nil, err
			}
		}
	}
	for _, id := range m.CommonPolicies {
		if !commonIDs[id] {
			return nil, fmt.Errorf("manifest common policy is not registered: %s", id)
		}
	}
	overlayManifest := filepath.Join(m.PackRoot, "policies", "manifest.yml")
	if err := add(overlayManifest, 16384); err != nil {
		return nil, err
	}
	if err := add(filepath.Join(m.PackRoot, "policies", "README.md"), 16384); err != nil {
		return nil, err
	}
	overlayEntries, err := manifest.ObjectListFromFile(overlayManifest, "overlays")
	if err != nil {
		return nil, err
	}
	overlayPaths := map[string]bool{}
	for _, entry := range overlayEntries {
		if extends := entry["extends"]; extends != "" && !commonIDs[extends] {
			return nil, fmt.Errorf("policy overlay extends unknown common policy: %s -> %s", entry["id"], extends)
		}
		if rel := entry["path"]; rel != "" {
			overlayPaths[rel] = true
			if err := add(filepath.Join(filepath.Dir(overlayManifest), filepath.FromSlash(rel)), 16384); err != nil {
				return nil, err
			}
		}
	}
	for _, rel := range m.PolicyOverlays {
		normalized := strings.TrimPrefix(strings.TrimPrefix(rel, "policies/"), `policies\`)
		if !overlayPaths[normalized] {
			return nil, fmt.Errorf("manifest policy overlay is not registered: %s", rel)
		}
		path, err := m.SourcePath(rel)
		if err != nil {
			return nil, err
		}
		if err := add(path, 16384); err != nil {
			return nil, err
		}
	}
	return rows, nil
}
