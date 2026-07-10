package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/review"
)

type ApplyOptions struct {
	ProjectName         string
	ForceLocalTemplates bool
	CreateLocalFiles    bool
	Command             string
}

type WriteResult struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Action     string `json:"action"`
	SourcePath string `json:"sourcePath,omitempty"`
	TargetPath string `json:"targetPath,omitempty"`
	BackupPath string `json:"backupPath,omitempty"`
}

type ApplyResult struct {
	SchemaVersion int           `json:"schemaVersion"`
	Command       string        `json:"command"`
	CaseRoot      string        `json:"caseRoot"`
	RepoRoot      string        `json:"repoRoot"`
	Pack          string        `json:"pack"`
	IsMutation    bool          `json:"isMutation"`
	Applied       bool          `json:"applied"`
	BackupRoot    string        `json:"backupRoot"`
	Writes        []WriteResult `json:"writes"`
	NextSteps     []string      `json:"nextSteps"`
}

type InitPlan struct {
	SchemaVersion        int           `json:"schemaVersion"`
	Command              string        `json:"command"`
	CaseRoot             string        `json:"caseRoot"`
	RepoRoot             string        `json:"repoRoot"`
	Pack                 string        `json:"pack"`
	ProjectName          string        `json:"projectName"`
	IsMutation           bool          `json:"isMutation"`
	ReviewRequired       bool          `json:"reviewRequired"`
	RequiresConfirmation bool          `json:"requiresConfirmation"`
	BackupRoot           string        `json:"backupRoot"`
	Writes               []WriteResult `json:"writes"`
	BlockedActions       []string      `json:"blockedActions"`
	NextSteps            []string      `json:"nextSteps"`
}

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

func InitPreview(repoRoot, caseRoot, pack string, opt ApplyOptions) (InitPlan, error) {
	caseFull, err := filepath.Abs(caseRoot)
	if err != nil {
		return InitPlan{}, err
	}
	repoFull, err := filepath.Abs(repoRoot)
	if err != nil {
		return InitPlan{}, err
	}
	command := strings.TrimSpace(opt.Command)
	if command == "" {
		command = "init"
	}
	if casebind.SamePath(caseFull, repoFull) {
		return InitPlan{}, fmt.Errorf("%s target must be an external case directory, not the kit repo root: %s", command, caseFull)
	}
	inst, err := readApplyInstance(caseFull, repoFull, pack, true)
	if err != nil {
		return InitPlan{}, err
	}
	m, err := manifest.Load(repoFull, pack)
	if err != nil {
		return InitPlan{}, err
	}
	projectName := strings.TrimSpace(opt.ProjectName)
	if projectName == "" {
		projectName = strings.TrimSpace(inst.ProjectName)
	}
	if projectName == "" {
		projectName = casebind.ProjectNameFromRoot(caseFull)
	}
	backupRoot, err := syncBackupRoot(caseFull, m)
	if err != nil {
		return InitPlan{}, err
	}
	writes := []WriteResult{
		{Path: ".rekit/instance.yml", Kind: "instance-metadata", Action: casebind.ActionFor(filepath.Join(caseFull, ".rekit", "instance.yml")), TargetPath: filepath.Join(caseFull, ".rekit", "instance.yml")},
		{Path: ".claude/skills/rekit/SKILL.md", Kind: "case-local-thin-shim", Action: casebind.ActionFor(filepath.Join(caseFull, ".claude", "skills", "rekit", "SKILL.md")), TargetPath: filepath.Join(caseFull, ".claude", "skills", "rekit", "SKILL.md")},
		{Path: ".re-template.yml", Kind: "legacy-metadata", Action: casebind.ActionFor(filepath.Join(caseFull, ".re-template.yml")), TargetPath: filepath.Join(caseFull, ".re-template.yml")},
	}
	for _, rel := range m.ManagedFiles {
		source, err := m.SourcePath(rel)
		if err != nil {
			return InitPlan{}, err
		}
		dest, err := refsf.SafeJoin(caseFull, rel)
		if err != nil {
			return InitPlan{}, err
		}
		sourceText, err := readRequiredText(source)
		if err != nil {
			return InitPlan{}, err
		}
		destText, destExists, err := review.ReadTextIfExists(dest)
		if err != nil {
			return InitPlan{}, err
		}
		action := "create-managed-file"
		if destExists {
			if destText == sourceText {
				action = "unchanged"
			} else {
				action = "overwrite-with-backup"
			}
		}
		writes = append(writes, WriteResult{Path: rel, Kind: "managed-file", Action: action, SourcePath: source, TargetPath: dest})
	}
	for _, rel := range m.TemplateFiles {
		source, err := m.SourcePath(rel)
		if err != nil {
			return InitPlan{}, err
		}
		targetRel := strings.TrimSuffix(rel, ".template.md") + ".md"
		dest, err := refsf.SafeJoin(caseFull, targetRel)
		if err != nil {
			return InitPlan{}, err
		}
		if _, err := readRequiredText(source); err != nil {
			return InitPlan{}, err
		}
		destExists := refsf.Exists(dest)
		action := "create-local-template-file"
		if destExists && opt.ForceLocalTemplates {
			action = "overwrite-local-template-file-with-force"
		} else if destExists {
			action = "skip-existing-local-file"
		}
		writes = append(writes, WriteResult{Path: targetRel, Kind: "template-file", Action: action, SourcePath: source, TargetPath: dest})
	}
	blockSource, err := m.SourcePath(m.ManagedBlock["source"])
	if err != nil {
		return InitPlan{}, err
	}
	blockHost, err := refsf.SafeJoin(caseFull, m.ManagedBlock["file"])
	if err != nil {
		return InitPlan{}, err
	}
	hostText, _, err := review.ReadTextIfExists(blockHost)
	if err != nil {
		return InitPlan{}, err
	}
	blockText, err := readRequiredText(blockSource)
	if err != nil {
		return InitPlan{}, err
	}
	nextHostText := review.ApplyManagedBlock(hostText, m.ManagedBlock["blockId"], blockText)
	writes = append(writes, WriteResult{Path: m.ManagedBlock["file"], Kind: "managed-block", Action: managedBlockAction(hostText, nextHostText, m.ManagedBlock["blockId"]), SourcePath: blockSource, TargetPath: blockHost})
	gitignoreSource, err := m.SourcePath("examples/gitignore.example")
	if err == nil && refsf.Exists(gitignoreSource) {
		gitignoreTarget, err := refsf.SafeJoin(caseFull, ".gitignore")
		if err != nil {
			return InitPlan{}, err
		}
		action := "create-support-file"
		if refsf.Exists(gitignoreTarget) {
			action = "skip-existing-support-file"
		}
		writes = append(writes, WriteResult{Path: ".gitignore", Kind: "support-file", Action: action, SourcePath: gitignoreSource, TargetPath: gitignoreTarget})
	}
	writes = append(writes, WriteResult{Path: ".rekit/state.json", Kind: "sync-state", Action: casebind.ActionFor(filepath.Join(caseFull, ".rekit", "state.json")), TargetPath: filepath.Join(caseFull, ".rekit", "state.json")})
	return InitPlan{SchemaVersion: 1, Command: command, CaseRoot: caseFull, RepoRoot: repoFull, Pack: pack, ProjectName: projectName, IsMutation: false, ReviewRequired: true, RequiresConfirmation: true, BackupRoot: backupRoot, Writes: writes, BlockedActions: []string{"pack writes", "promote", "authority/confirmed writes", "heavy-tool execution", "board/facts/lanes migration"}, NextSteps: []string{"review this plan, then re-run " + command + " with -Apply to initialize the case", "PowerShell /rekit remains the public entrypoint; this is a manual Go CLI path"}}, nil
}

func readApplyInstance(caseRoot, repoRoot, pack string, createLocalFiles bool) (instance.Instance, error) {
	if !createLocalFiles {
		return instance.AssertAttached(caseRoot, repoRoot, pack)
	}
	if st, err := os.Stat(caseRoot); err == nil && !st.IsDir() {
		return instance.Instance{}, fmt.Errorf("case target is not a directory: %s", caseRoot)
	} else if err != nil && !os.IsNotExist(err) {
		return instance.Instance{}, err
	}
	inst, err := instance.Read(caseRoot)
	if err != nil {
		return instance.Instance{}, err
	}
	if inst.Source == "missing" {
		return inst, nil
	}
	if inst.Moved() {
		return instance.Instance{}, fmt.Errorf("case metadata points to a different directory. Run 'rekit repair -Target %q -Apply' after confirming the move", caseRoot)
	}
	if strings.TrimSpace(inst.TemplateRoot) == "" {
		return instance.Instance{}, fmt.Errorf("missing templateRoot in case metadata: %s", caseRoot)
	}
	if !casebind.SamePath(inst.TemplateRoot, repoRoot) {
		return instance.Instance{}, fmt.Errorf("case is attached to a different templateRoot: %s", inst.TemplateRoot)
	}
	if strings.TrimSpace(inst.TemplatePack) != "" && !strings.EqualFold(inst.TemplatePack, pack) {
		return instance.Instance{}, fmt.Errorf("case is attached to a different templatePack: %s", inst.TemplatePack)
	}
	return inst, nil
}

func Apply(repoRoot, caseRoot, pack string, opt ApplyOptions) (ApplyResult, error) {
	caseFull, err := filepath.Abs(caseRoot)
	if err != nil {
		return ApplyResult{}, err
	}
	repoFull, err := filepath.Abs(repoRoot)
	if err != nil {
		return ApplyResult{}, err
	}
	caseRoot = caseFull
	repoRoot = repoFull
	command := strings.TrimSpace(opt.Command)
	if command == "" {
		command = "sync"
	}
	if opt.CreateLocalFiles {
		if casebind.SamePath(caseFull, repoFull) {
			return ApplyResult{}, fmt.Errorf("%s target must be an external case directory, not the kit repo root: %s", command, caseFull)
		}
	}
	inst, err := readApplyInstance(caseFull, repoFull, pack, opt.CreateLocalFiles)
	if err != nil {
		return ApplyResult{}, err
	}
	m, err := manifest.Load(repoFull, pack)
	if err != nil {
		return ApplyResult{}, err
	}
	projectName := strings.TrimSpace(opt.ProjectName)
	if projectName == "" {
		projectName = strings.TrimSpace(inst.ProjectName)
	}
	if projectName == "" {
		projectName = casebind.ProjectNameFromRoot(caseFull)
	}
	backupRoot, err := syncBackupRoot(caseFull, m)
	if err != nil {
		return ApplyResult{}, err
	}
	writes := []WriteResult{}

	if _, err := casebind.WriteInstance(caseFull, repoFull, pack, projectName); err != nil {
		return ApplyResult{}, err
	}
	writes = append(writes, WriteResult{Path: ".rekit/instance.yml", Kind: "instance-metadata", Action: "refresh", TargetPath: filepath.Join(caseFull, ".rekit", "instance.yml")})
	if _, err := casebind.WriteCaseShim(caseFull, repoFull); err != nil {
		return ApplyResult{}, err
	}
	writes = append(writes, WriteResult{Path: ".claude/skills/rekit/SKILL.md", Kind: "case-local-thin-shim", Action: "refresh", TargetPath: filepath.Join(caseFull, ".claude", "skills", "rekit", "SKILL.md")})
	if _, err := casebind.WriteLegacyMetadataForAttach(caseFull, repoFull, pack); err != nil {
		return ApplyResult{}, err
	}
	writes = append(writes, WriteResult{Path: ".re-template.yml", Kind: "legacy-metadata", Action: "refresh", TargetPath: filepath.Join(caseFull, ".re-template.yml")})

	for _, rel := range m.ManagedFiles {
		source, err := m.SourcePath(rel)
		if err != nil {
			return ApplyResult{}, err
		}
		dest, err := refsf.SafeJoin(caseFull, rel)
		if err != nil {
			return ApplyResult{}, err
		}
		sourceText, err := readRequiredText(source)
		if err != nil {
			return ApplyResult{}, err
		}
		destText, destExists, err := review.ReadTextIfExists(dest)
		if err != nil {
			return ApplyResult{}, err
		}
		action := "create-managed-file"
		backupPath := ""
		if destExists {
			if destText == sourceText {
				action = "unchanged"
			} else {
				action = "overwrite-with-backup"
				backupPath, err = backupCaseFile(dest, caseRoot, backupRoot)
				if err != nil {
					return ApplyResult{}, err
				}
			}
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return ApplyResult{}, err
		}
		if err := os.WriteFile(dest, []byte(sourceText), 0o644); err != nil {
			return ApplyResult{}, err
		}
		writes = append(writes, WriteResult{Path: rel, Kind: "managed-file", Action: action, SourcePath: source, TargetPath: dest, BackupPath: backupPath})
	}

	for _, rel := range m.TemplateFiles {
		source, err := m.SourcePath(rel)
		if err != nil {
			return ApplyResult{}, err
		}
		targetRel := strings.TrimSuffix(rel, ".template.md") + ".md"
		dest, err := refsf.SafeJoin(caseRoot, targetRel)
		if err != nil {
			return ApplyResult{}, err
		}
		sourceText, err := readRequiredText(source)
		if err != nil {
			return ApplyResult{}, err
		}
		planned := strings.ReplaceAll(sourceText, "<PROJECT_NAME>", projectName)
		planned = strings.ReplaceAll(planned, "<PROJECT_ROOT>", caseRoot)
		destExists := refsf.Exists(dest)
		if destExists && !opt.ForceLocalTemplates {
			writes = append(writes, WriteResult{Path: targetRel, Kind: "template-file", Action: "skip-existing-local-file", SourcePath: source, TargetPath: dest})
			continue
		}
		action := "create-local-template-file"
		backupPath := ""
		if destExists && opt.ForceLocalTemplates {
			action = "overwrite-local-template-file-with-force"
			backupPath, err = backupCaseFile(dest, caseRoot, backupRoot)
			if err != nil {
				return ApplyResult{}, err
			}
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return ApplyResult{}, err
		}
		if err := os.WriteFile(dest, []byte(planned), 0o644); err != nil {
			return ApplyResult{}, err
		}
		writes = append(writes, WriteResult{Path: targetRel, Kind: "template-file", Action: action, SourcePath: source, TargetPath: dest, BackupPath: backupPath})
	}

	blockSource, err := m.SourcePath(m.ManagedBlock["source"])
	if err != nil {
		return ApplyResult{}, err
	}
	blockHost, err := refsf.SafeJoin(caseRoot, m.ManagedBlock["file"])
	if err != nil {
		return ApplyResult{}, err
	}
	hostText, hostExists, err := review.ReadTextIfExists(blockHost)
	if err != nil {
		return ApplyResult{}, err
	}
	blockText, err := readRequiredText(blockSource)
	if err != nil {
		return ApplyResult{}, err
	}
	nextHostText := review.ApplyManagedBlock(hostText, m.ManagedBlock["blockId"], blockText)
	blockAction := managedBlockAction(hostText, nextHostText, m.ManagedBlock["blockId"])
	blockBackup := ""
	if hostExists {
		blockBackup, err = backupCaseFile(blockHost, caseRoot, backupRoot)
		if err != nil {
			return ApplyResult{}, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(blockHost), 0o755); err != nil {
		return ApplyResult{}, err
	}
	if err := os.WriteFile(blockHost, []byte(nextHostText), 0o644); err != nil {
		return ApplyResult{}, err
	}
	writes = append(writes, WriteResult{Path: m.ManagedBlock["file"], Kind: "managed-block", Action: blockAction, SourcePath: blockSource, TargetPath: blockHost, BackupPath: blockBackup})

	gitignoreSource, err := m.SourcePath("examples/gitignore.example")
	if err == nil && refsf.Exists(gitignoreSource) {
		gitignoreTarget, err := refsf.SafeJoin(caseRoot, ".gitignore")
		if err != nil {
			return ApplyResult{}, err
		}
		if refsf.Exists(gitignoreTarget) {
			writes = append(writes, WriteResult{Path: ".gitignore", Kind: "support-file", Action: "skip-existing-support-file", SourcePath: gitignoreSource, TargetPath: gitignoreTarget})
		} else {
			text, err := readRequiredText(gitignoreSource)
			if err != nil {
				return ApplyResult{}, err
			}
			if err := os.WriteFile(gitignoreTarget, []byte(text), 0o644); err != nil {
				return ApplyResult{}, err
			}
			writes = append(writes, WriteResult{Path: ".gitignore", Kind: "support-file", Action: "create-support-file", SourcePath: gitignoreSource, TargetPath: gitignoreTarget})
		}
	}

	statePath, err := writeSyncState(caseRoot, m)
	if err != nil {
		return ApplyResult{}, err
	}
	writes = append(writes, WriteResult{Path: ".rekit/state.json", Kind: "sync-state", Action: "refresh", TargetPath: statePath})

	return ApplyResult{SchemaVersion: 1, Command: command, CaseRoot: caseRoot, RepoRoot: repoRoot, Pack: pack, IsMutation: true, Applied: true, BackupRoot: backupRoot, Writes: writes, NextSteps: []string{"run doctor after apply", "review backupRoot if any overwritten file must be restored"}}, nil
}

func readRequiredText(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func syncBackupRoot(caseRoot string, m *manifest.Manifest) (string, error) {
	backupRel := strings.TrimSpace(m.WorkstreamDefaults["backupRoot"])
	if backupRel == "" {
		backupRel = ".rekit/backups/sync"
	}
	return refsf.SafeJoin(caseRoot, filepath.ToSlash(filepath.Join(filepath.FromSlash(backupRel), time.Now().Format("20060102-150405"))))
}

func backupCaseFile(path, caseRoot, backupRoot string) (string, error) {
	if !refsf.Exists(path) {
		return "", nil
	}
	caseFull, err := filepath.Abs(caseRoot)
	if err != nil {
		return "", err
	}
	pathFull, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(caseFull, pathFull)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("cannot backup file outside case root: %s", path)
	}
	dest := filepath.Join(backupRoot, rel)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	b, err := os.ReadFile(pathFull)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(dest, b, 0o644); err != nil {
		return "", err
	}
	return dest, nil
}

func managedBlockAction(hostText, nextHostText, blockID string) string {
	if hostText == nextHostText {
		return "unchanged"
	}
	if strings.TrimSpace(hostText) == "" {
		return "create-managed-block-host"
	}
	if strings.Contains(hostText, "<!-- BEGIN "+blockID) {
		return "replace-managed-block"
	}
	return "append-managed-block"
}

type syncState struct {
	SchemaVersion int                         `json:"schemaVersion"`
	TemplateRoot  string                      `json:"templateRoot"`
	TemplatePack  string                      `json:"templatePack"`
	LastSyncAt    string                      `json:"lastSyncAt"`
	Managed       map[string]syncManagedEntry `json:"managed"`
}

type syncManagedEntry struct {
	SourceHash       string `json:"sourceHash"`
	TargetHashAtSync string `json:"targetHashAtSync"`
	LastAction       string `json:"lastAction"`
}

func writeSyncState(caseRoot string, m *manifest.Manifest) (string, error) {
	managed := map[string]syncManagedEntry{}
	for _, rel := range m.ManagedFiles {
		source, err := m.SourcePath(rel)
		if err != nil {
			return "", err
		}
		target, err := refsf.SafeJoin(caseRoot, rel)
		if err != nil {
			return "", err
		}
		managed[rel] = syncManagedEntry{SourceHash: review.FileHash(source), TargetHashAtSync: review.FileHash(target), LastAction: "sync"}
	}
	state := syncState{SchemaVersion: 1, TemplateRoot: m.RepoRoot, TemplatePack: m.Pack, LastSyncAt: time.Now().Format("2006-01-02T15:04:05-07:00"), Managed: managed}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return "", err
	}
	statePath := filepath.Join(caseRoot, ".rekit", "state.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(statePath, append(b, '\n'), 0o644); err != nil {
		return "", err
	}
	return statePath, nil
}
