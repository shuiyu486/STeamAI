package sync

import (
	"path/filepath"
	"strings"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/review"
)

func Plan(repoRoot, caseRoot, pack string) (review.Plan, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return review.Plan{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return review.Plan{}, err
	}
	projectName := inst.ProjectName
	if strings.TrimSpace(projectName) == "" {
		projectName = filepath.Base(filepath.Clean(caseRoot))
	}
	items := []review.Item{}
	items = append(items, review.Item{Path: ".rekit/instance.yml + .claude/skills/rekit/SKILL.md", Kind: "case-metadata", Direction: "kit-to-case", Action: "refresh-metadata-and-shim", RiskLevel: "low"})
	for _, rel := range m.ManagedFiles {
		source, err := m.SourcePath(rel)
		if err != nil {
			return review.Plan{}, err
		}
		dest, err := refsf.SafeJoin(caseRoot, rel)
		if err != nil {
			return review.Plan{}, err
		}
		sourceText, _, err := review.ReadTextIfExists(source)
		if err != nil {
			return review.Plan{}, err
		}
		destText, destExists, err := review.ReadTextIfExists(dest)
		if err != nil {
			return review.Plan{}, err
		}
		sourceHash := review.FileHash(source)
		targetHash := review.FileHash(dest)
		action := "overwrite-with-backup"
		if !destExists {
			action = "create-managed-file"
		} else if sourceText == destText || sourceHash == targetHash {
			action = "unchanged"
		}
		risk := "medium"
		if action == "unchanged" {
			risk = "none"
		}
		items = append(items, review.Item{Path: rel, Kind: "managed-file", Direction: "kit-to-case", Action: action, RiskLevel: risk, SourcePath: source, TargetPath: dest, SourceHash: sourceHash, TargetHash: targetHash})
	}
	for _, rel := range m.TemplateFiles {
		source, err := m.SourcePath(rel)
		if err != nil {
			return review.Plan{}, err
		}
		targetRel := strings.TrimSuffix(rel, ".template.md") + ".md"
		dest, err := refsf.SafeJoin(caseRoot, targetRel)
		if err != nil {
			return review.Plan{}, err
		}
		sourceText, _, err := review.ReadTextIfExists(source)
		if err != nil {
			return review.Plan{}, err
		}
		sourceText = strings.ReplaceAll(sourceText, "<PROJECT_NAME>", projectName)
		sourceText = strings.ReplaceAll(sourceText, "<PROJECT_ROOT>", caseRoot)
		destText, destExists, err := review.ReadTextIfExists(dest)
		if err != nil {
			return review.Plan{}, err
		}
		action := "create-local-template-file"
		if destExists {
			action = "skip-existing-local-file"
		} else if sourceText == destText {
			action = "unchanged"
		}
		risk := "low"
		if action == "skip-existing-local-file" || action == "unchanged" {
			risk = "none"
		}
		items = append(items, review.Item{Path: targetRel, Kind: "template-file", Direction: "kit-to-case", Action: action, RiskLevel: risk, SourcePath: source, TargetPath: dest, SourceHash: review.FileHash(source), TargetHash: review.FileHash(dest), PlannedText: sourceText})
	}
	blockSource, err := m.SourcePath(m.ManagedBlock["source"])
	if err != nil {
		return review.Plan{}, err
	}
	blockHost, err := refsf.SafeJoin(caseRoot, m.ManagedBlock["file"])
	if err != nil {
		return review.Plan{}, err
	}
	hostText, _, err := review.ReadTextIfExists(blockHost)
	if err != nil {
		return review.Plan{}, err
	}
	blockText, _, err := review.ReadTextIfExists(blockSource)
	if err != nil {
		return review.Plan{}, err
	}
	nextHostText := review.ApplyManagedBlock(hostText, m.ManagedBlock["blockId"], blockText)
	blockAction := "append-managed-block"
	if hostText == nextHostText {
		blockAction = "unchanged"
	} else if strings.TrimSpace(hostText) == "" {
		blockAction = "create-managed-block-host"
	} else if strings.Contains(hostText, "<!-- BEGIN "+m.ManagedBlock["blockId"]) {
		blockAction = "replace-managed-block"
	}
	blockRisk := "medium"
	if blockAction == "unchanged" {
		blockRisk = "none"
	}
	items = append(items, review.Item{Path: m.ManagedBlock["file"], Kind: "managed-block", Direction: "kit-to-case", Action: blockAction, RiskLevel: blockRisk, SourcePath: blockSource, TargetPath: blockHost, BlockID: m.ManagedBlock["blockId"], SourceHash: review.FileHash(blockSource), TargetHash: review.FileHash(blockHost)})
	gitignoreSource, err := m.SourcePath("examples/gitignore.example")
	if err == nil && refsf.Exists(gitignoreSource) {
		gitignoreTarget, err := refsf.SafeJoin(caseRoot, ".gitignore")
		if err != nil {
			return review.Plan{}, err
		}
		action := "create-support-file"
		if refsf.Exists(gitignoreTarget) {
			action = "skip-existing-support-file"
		}
		items = append(items, review.Item{Path: ".gitignore", Kind: "support-file", Direction: "kit-to-case", Action: action, RiskLevel: "low", SourcePath: gitignoreSource, TargetPath: gitignoreTarget, SourceHash: review.FileHash(gitignoreSource), TargetHash: review.FileHash(gitignoreTarget)})
	}
	return review.Plan{SchemaVersion: 1, Command: "sync", Direction: "kit-to-case", CaseRoot: caseRoot, RepoRoot: repoRoot, Pack: pack, ManifestPath: m.ManifestPath, ManifestVersion: m.Version, Items: items}, nil
}
