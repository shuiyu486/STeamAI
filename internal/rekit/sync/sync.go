package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/kitmutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/review"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
	"github.com/shuiyu486/re-context-kits/internal/rekit/sourceartifact"
)

var acquireMutationLease = func(caseRoot string) (mutationLease, error) {
	return kitmutation.Acquire(caseRoot)
}

var ordinaryInitAfterPlanHook func(InitPlan) error
var ordinaryInitAfterPublicationHook func(int, InitPlan) error
var ordinaryInitBeforeFinalValidationHook func(InitPlan) error
var ordinaryInitBeforeRollbackHook func(string, InitPlan) error
var ordinaryInitRollbackAfterIdentityHook func(string) error
var ordinaryInitLeaseForTest func(mutationLease) mutationLease
var ordinaryInitRollbackCapability = ordinaryInitRollbackCapabilityCheck
var ordinaryInitOpenRollbackHandleForApply = ordinaryInitOpenRollbackHandle

type mutationLease interface{ Unlock() error }

type ApplyOptions struct {
	ProjectName         string
	ForceLocalTemplates bool
	CreateLocalFiles    bool
	Command             string
	ExpectedPlanSHA256  string
}

type WriteResult struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Action     string `json:"action"`
	SourcePath string `json:"sourcePath,omitempty"`
	TargetPath string `json:"targetPath,omitempty"`
	BackupPath string `json:"backupPath,omitempty"`
	blockID    string
	rawContent []byte
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
	TargetClass          string        `json:"targetClass"`
	AdoptionReady        bool          `json:"adoptionReady"`
	AdoptionBlockers     []string      `json:"adoptionBlockers,omitempty"`
	ExpectedPlanSHA256   string        `json:"expectedPlanSha256"`
	ApplyArgs            []string      `json:"applyArgs,omitempty"`
	BackupRoot           string        `json:"backupRoot"`
	Writes               []WriteResult `json:"writes"`
	BlockedActions       []string      `json:"blockedActions"`
	NextSteps            []string      `json:"nextSteps"`

	initSourceSHA256     map[string]string
	initTargetSHA256     map[string]string
	initManifestSHA256   string
	initGitignorePresent bool
	bundleManifestSHA256 string
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
	if err := m.ValidateSchema(); err != nil {
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
	targetClass, inst, err := classifyInitTarget(caseFull, repoFull, pack)
	if err != nil {
		return InitPlan{}, err
	}
	m, err := manifest.Load(repoFull, pack)
	if err != nil {
		return InitPlan{}, err
	}
	if err := m.ValidateSchema(); err != nil {
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
	canonicalSources := initPublishesCanonicalText(targetClass)
	stateRoot, err := projectstate.Resolve(caseFull)
	if err != nil {
		return InitPlan{}, err
	}
	skill := "steamai"
	skillKind := "project-local-steamai-skill"
	skillSource := filepath.Join(repoFull, "rekit", "templates", "steamai-project", "SKILL.md")
	if stateRoot.Legacy {
		skill = "rekit"
		skillKind = "case-local-thin-shim"
		skillSource = filepath.Join(repoFull, "rekit", "templates", "case-shim", "SKILL.md")
	}
	if _, err := readSourceText(skillSource, canonicalSources); err != nil {
		return InitPlan{}, err
	}
	instanceRel := filepath.ToSlash(filepath.Join(stateRoot.Dir, "instance.yml"))
	instanceTarget := filepath.Join(stateRoot.Path, "instance.yml")
	skillRel := filepath.ToSlash(filepath.Join(".claude", "skills", skill, "SKILL.md"))
	skillTarget := filepath.Join(caseFull, ".claude", "skills", skill, "SKILL.md")
	writes := []WriteResult{
		{Path: instanceRel, Kind: "instance-metadata", Action: casebind.ActionFor(instanceTarget), TargetPath: instanceTarget},
		{Path: skillRel, Kind: skillKind, Action: casebind.ActionFor(skillTarget), SourcePath: skillSource, TargetPath: skillTarget},
	}
	bundleManifestSHA256 := ""
	if !stateRoot.Legacy && initPublishesCanonicalText(targetClass) {
		bundlePlan, err := runtimebundle.Build(repoFull, pack)
		if err != nil {
			return InitPlan{}, err
		}
		bundleManifestSHA256 = bundlePlan.ManifestSHA256
		for _, publication := range bundlePlan.Publications {
			path := filepath.ToSlash(filepath.Join(stateRoot.Dir, filepath.FromSlash(publication.Path)))
			target := filepath.Join(stateRoot.Path, filepath.FromSlash(publication.Path))
			writes = append(writes, WriteResult{Path: path, Kind: publication.Kind, Action: casebind.ActionFor(target), SourcePath: publication.SourcePath, TargetPath: target, rawContent: append([]byte(nil), publication.Content...)})
		}
	}
	if stateRoot.Legacy {
		writes = append(writes, WriteResult{Path: ".re-template.yml", Kind: "legacy-metadata", Action: casebind.ActionFor(filepath.Join(caseFull, ".re-template.yml")), TargetPath: filepath.Join(caseFull, ".re-template.yml")})
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
		sourceText, err := readSourceText(source, canonicalSources)
		if err != nil {
			return InitPlan{}, err
		}
		destText, destExists, err := review.ReadTextIfExists(dest)
		if err != nil {
			return InitPlan{}, err
		}
		action := "create-managed-file"
		if destExists {
			sameText := destText == sourceText
			if targetClass == "ordinary-directory" {
				sameText = sourceTextEquivalent(destText, sourceText)
			}
			if sameText {
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
		if _, err := readSourceText(source, canonicalSources); err != nil {
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
	blockText, err := readSourceText(blockSource, canonicalSources)
	if err != nil {
		return InitPlan{}, err
	}
	nextHostText := review.ApplyManagedBlock(hostText, m.ManagedBlock["blockId"], blockText)
	writes = append(writes, WriteResult{Path: m.ManagedBlock["file"], Kind: "managed-block", Action: managedBlockAction(hostText, nextHostText, m.ManagedBlock["blockId"]), SourcePath: blockSource, TargetPath: blockHost, blockID: m.ManagedBlock["blockId"]})
	gitignoreSource, err := m.SourcePath("examples/gitignore.example")
	gitignorePresent := err == nil && refsf.Exists(gitignoreSource)
	if gitignorePresent {
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
	stateRel := filepath.ToSlash(filepath.Join(stateRoot.Dir, "state.json"))
	stateTarget := filepath.Join(stateRoot.Path, "state.json")
	writes = append(writes, WriteResult{Path: stateRel, Kind: "sync-state", Action: casebind.ActionFor(stateTarget), TargetPath: stateTarget})
	manifestBytes, err := os.ReadFile(m.ManifestPath)
	if err != nil {
		return InitPlan{}, err
	}
	entrypoint := "/steamai"
	if stateRoot.Legacy {
		entrypoint = "/rekit"
	}
	return finalizeInitPlan(InitPlan{SchemaVersion: 1, Command: command, CaseRoot: caseFull, RepoRoot: repoFull, Pack: pack, ProjectName: projectName, TargetClass: targetClass, IsMutation: false, ReviewRequired: true, RequiresConfirmation: true, BackupRoot: backupRoot, Writes: writes, BlockedActions: []string{"pack writes", "promote", "authority/confirmed writes", "heavy-tool execution", "board/facts/lanes migration"}, NextSteps: []string{"review this plan, then re-run " + command + " with -Apply and the exact plan hash to initialize the case", "use " + entrypoint + " as the Mission Commander entrypoint; this remains a review-first Go runtime path"}, initManifestSHA256: sha256Bytes(sourceartifact.SemanticText(manifestBytes)), initGitignorePresent: gitignorePresent, bundleManifestSHA256: bundleManifestSHA256})
}

func initPublishesCanonicalText(targetClass string) bool {
	return targetClass == "missing" || targetClass == "ordinary-directory"
}

func sourceTextEquivalent(left, right string) bool {
	return string(sourceartifact.SemanticText([]byte(left))) == string(sourceartifact.SemanticText([]byte(right)))
}

func readApplyInstance(caseRoot, repoRoot, pack string, createLocalFiles bool) (instance.Instance, error) {
	if !createLocalFiles {
		return instance.AssertAttached(caseRoot, repoRoot, pack)
	}
	_, inst, err := classifyInitTarget(caseRoot, repoRoot, pack)
	return inst, err
}

func ApplyPreview(repoRoot, caseRoot, pack string, opt ApplyOptions) (ApplyResult, error) {
	caseFull, repoFull, command, m, _, err := prepareApply(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return ApplyResult{}, err
	}
	backupRoot, err := syncBackupRoot(caseFull, m)
	if err != nil {
		return ApplyResult{}, err
	}
	writes, err := planApplyWrites(caseFull, m, opt, backupRoot)
	if err != nil {
		return ApplyResult{}, err
	}
	return ApplyResult{SchemaVersion: 1, Command: command, CaseRoot: caseFull, RepoRoot: repoFull, Pack: pack, IsMutation: false, Applied: false, BackupRoot: backupRoot, Writes: writes, NextSteps: []string{"review this non-writing preview, then re-run " + command + " with -Apply after confirming the exact scope", "sync/update PowerShell fallback has been retired; remove REKIT_GO_DISABLE or run the Go backend directly if facade delegation is unavailable"}}, nil
}

func Apply(repoRoot, caseRoot, pack string, opt ApplyOptions) (_ ApplyResult, retErr error) {
	caseFull, repoFull, command, m, inst, err := prepareApply(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return ApplyResult{}, err
	}
	lease, err := acquireMutationLease(caseFull)
	if err != nil {
		return ApplyResult{}, err
	}
	leaseOwned := true
	defer func() {
		if leaseOwned {
			retErr = errors.Join(retErr, lease.Unlock())
		}
	}()
	canonicalSources := false
	if opt.CreateLocalFiles {
		if strings.TrimSpace(opt.ExpectedPlanSHA256) != "" && !validInitPlanSHA256(opt.ExpectedPlanSHA256) {
			return ApplyResult{}, fmt.Errorf("%s -Apply requires a valid -ExpectedInitPlanSha256 from -WhatIf", command)
		}
		fresh, err := InitPreview(repoFull, caseFull, pack, opt)
		if err != nil {
			return ApplyResult{}, err
		}
		if (fresh.TargetClass == "missing" || fresh.TargetClass == "ordinary-directory") && !validInitPlanSHA256(opt.ExpectedPlanSHA256) {
			return ApplyResult{}, fmt.Errorf("%s new project initialization requires a valid -ExpectedInitPlanSha256 from -WhatIf", command)
		}
		if validInitPlanSHA256(opt.ExpectedPlanSHA256) && !strings.EqualFold(fresh.ExpectedPlanSHA256, opt.ExpectedPlanSHA256) {
			return ApplyResult{}, fmt.Errorf("%s plan changed after preview; rerun -WhatIf", command)
		}
		if fresh.TargetClass == "ordinary-directory" && !fresh.AdoptionReady {
			return ApplyResult{}, fmt.Errorf("%s ordinary-directory adoption is blocked: %s", command, strings.Join(fresh.AdoptionBlockers, ", "))
		}
		if fresh.TargetClass != "missing" && fresh.TargetClass != "ordinary-directory" && fresh.TargetClass != "attached-case" && fresh.TargetClass != "mission-case" {
			return ApplyResult{}, fmt.Errorf("%s refuses target class %s", command, fresh.TargetClass)
		}
		canonicalSources = initPublishesCanonicalText(fresh.TargetClass)
		if fresh.TargetClass == "missing" || fresh.TargetClass == "ordinary-directory" {
			if fresh.TargetClass == "missing" {
				if err := os.Mkdir(caseFull, 0o755); err != nil {
					return ApplyResult{}, fmt.Errorf("%s create missing project root: %w", command, err)
				}
				defer func() {
					if retErr != nil {
						_ = os.Remove(caseFull)
					}
				}()
			}
			if ordinaryInitLeaseForTest != nil {
				lease = ordinaryInitLeaseForTest(lease)
			}
			leaseOwned = false
			return applyOrdinaryInit(fresh, lease)
		}
	}

	caseRoot = caseFull
	repoRoot = repoFull
	projectName := applyProjectName(caseFull, inst, opt)
	backupRoot, err := syncBackupRoot(caseFull, m)
	if err != nil {
		return ApplyResult{}, err
	}
	writes := []WriteResult{}

	instancePath, err := casebind.WriteInstance(caseFull, repoFull, pack, projectName)
	if err != nil {
		return ApplyResult{}, err
	}
	instanceRel, err := filepath.Rel(caseFull, instancePath)
	if err != nil {
		return ApplyResult{}, err
	}
	writes = append(writes, WriteResult{Path: filepath.ToSlash(instanceRel), Kind: "instance-metadata", Action: "refresh", TargetPath: instancePath})
	var shimPath string
	if canonicalSources {
		shimPath, err = casebind.WriteCanonicalCaseShim(caseFull, repoFull)
	} else {
		shimPath, err = casebind.WriteCaseShim(caseFull, repoFull)
	}
	if err != nil {
		return ApplyResult{}, err
	}
	shimRel, err := filepath.Rel(caseFull, shimPath)
	if err != nil {
		return ApplyResult{}, err
	}
	shimKind := "project-local-steamai-skill"
	if strings.Contains(filepath.ToSlash(shimRel), "/rekit/") {
		shimKind = "case-local-thin-shim"
	}
	writes = append(writes, WriteResult{Path: filepath.ToSlash(shimRel), Kind: shimKind, Action: "refresh", TargetPath: shimPath})
	stateRoot, err := projectstate.Resolve(caseFull)
	if err != nil {
		return ApplyResult{}, err
	}
	if stateRoot.Legacy {
		if _, err := casebind.WriteLegacyMetadataForAttach(caseFull, repoFull, pack); err != nil {
			return ApplyResult{}, err
		}
		writes = append(writes, WriteResult{Path: ".re-template.yml", Kind: "legacy-metadata", Action: "refresh", TargetPath: filepath.Join(caseFull, ".re-template.yml")})
	}

	for _, rel := range m.ManagedFiles {
		source, err := m.SourcePath(rel)
		if err != nil {
			return ApplyResult{}, err
		}
		dest, err := refsf.SafeJoin(caseFull, rel)
		if err != nil {
			return ApplyResult{}, err
		}
		sourceText, err := readSourceText(source, canonicalSources)
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
		sourceText, err := readSourceText(source, canonicalSources)
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
	blockText, err := readSourceText(blockSource, canonicalSources)
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
			text, err := readSourceText(gitignoreSource, canonicalSources)
			if err != nil {
				return ApplyResult{}, err
			}
			if err := os.WriteFile(gitignoreTarget, []byte(text), 0o644); err != nil {
				return ApplyResult{}, err
			}
			writes = append(writes, WriteResult{Path: ".gitignore", Kind: "support-file", Action: "create-support-file", SourcePath: gitignoreSource, TargetPath: gitignoreTarget})
		}
	}

	statePath, err := writeSyncState(caseRoot, m, canonicalSources)
	if err != nil {
		return ApplyResult{}, err
	}
	stateRel, err := filepath.Rel(caseRoot, statePath)
	if err != nil {
		return ApplyResult{}, err
	}
	writes = append(writes, WriteResult{Path: filepath.ToSlash(stateRel), Kind: "sync-state", Action: "refresh", TargetPath: statePath})

	return ApplyResult{SchemaVersion: 1, Command: command, CaseRoot: caseRoot, RepoRoot: repoRoot, Pack: pack, IsMutation: true, Applied: true, BackupRoot: backupRoot, Writes: writes, NextSteps: []string{"run doctor after apply", "review backupRoot if any overwritten file must be restored"}}, nil
}

func prepareApply(repoRoot, caseRoot, pack string, opt ApplyOptions) (string, string, string, *manifest.Manifest, instance.Instance, error) {
	caseFull, err := filepath.Abs(caseRoot)
	if err != nil {
		return "", "", "", nil, instance.Instance{}, err
	}
	repoFull, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", "", "", nil, instance.Instance{}, err
	}
	command := strings.TrimSpace(opt.Command)
	if command == "" {
		command = "sync"
	}
	if opt.CreateLocalFiles && casebind.SamePath(caseFull, repoFull) {
		return "", "", "", nil, instance.Instance{}, fmt.Errorf("%s target must be an external case directory, not the kit repo root: %s", command, caseFull)
	}
	inst, err := readApplyInstance(caseFull, repoFull, pack, opt.CreateLocalFiles)
	if err != nil {
		return "", "", "", nil, instance.Instance{}, err
	}
	m, err := manifest.Load(repoFull, pack)
	if err != nil {
		return "", "", "", nil, instance.Instance{}, err
	}
	if err := m.ValidateSchema(); err != nil {
		return "", "", "", nil, instance.Instance{}, err
	}
	return caseFull, repoFull, command, m, inst, nil
}

func applyProjectName(caseRoot string, inst instance.Instance, opt ApplyOptions) string {
	projectName := strings.TrimSpace(opt.ProjectName)
	if projectName == "" {
		projectName = strings.TrimSpace(inst.ProjectName)
	}
	if projectName == "" {
		projectName = casebind.ProjectNameFromRoot(caseRoot)
	}
	return projectName
}

func planApplyWrites(caseRoot string, m *manifest.Manifest, opt ApplyOptions, backupRoot string) ([]WriteResult, error) {
	stateRoot, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return nil, err
	}
	instanceRel := filepath.ToSlash(filepath.Join(stateRoot.Dir, "instance.yml"))
	instanceTarget := filepath.Join(stateRoot.Path, "instance.yml")
	skill := "steamai"
	skillKind := "project-local-steamai-skill"
	if stateRoot.Legacy {
		skill = "rekit"
		skillKind = "case-local-thin-shim"
	}
	skillRel := filepath.ToSlash(filepath.Join(".claude", "skills", skill, "SKILL.md"))
	writes := []WriteResult{
		{Path: instanceRel, Kind: "instance-metadata", Action: "refresh", TargetPath: instanceTarget},
		{Path: skillRel, Kind: skillKind, Action: "refresh", TargetPath: filepath.Join(caseRoot, filepath.FromSlash(skillRel))},
	}
	if stateRoot.Legacy {
		writes = append(writes, WriteResult{Path: ".re-template.yml", Kind: "legacy-metadata", Action: "refresh", TargetPath: filepath.Join(caseRoot, ".re-template.yml")})
	}
	for _, rel := range m.ManagedFiles {
		source, err := m.SourcePath(rel)
		if err != nil {
			return nil, err
		}
		dest, err := refsf.SafeJoin(caseRoot, rel)
		if err != nil {
			return nil, err
		}
		sourceText, err := readSourceText(source, false)
		if err != nil {
			return nil, err
		}
		destText, destExists, err := review.ReadTextIfExists(dest)
		if err != nil {
			return nil, err
		}
		action := "create-managed-file"
		backupPath := ""
		if destExists {
			if destText == sourceText {
				action = "unchanged"
			} else {
				action = "overwrite-with-backup"
				backupPath = previewBackupPath(dest, caseRoot, backupRoot)
			}
		}
		writes = append(writes, WriteResult{Path: rel, Kind: "managed-file", Action: action, SourcePath: source, TargetPath: dest, BackupPath: backupPath})
	}
	for _, rel := range m.TemplateFiles {
		source, err := m.SourcePath(rel)
		if err != nil {
			return nil, err
		}
		targetRel := strings.TrimSuffix(rel, ".template.md") + ".md"
		dest, err := refsf.SafeJoin(caseRoot, targetRel)
		if err != nil {
			return nil, err
		}
		if _, err := readSourceText(source, false); err != nil {
			return nil, err
		}
		destExists := refsf.Exists(dest)
		if destExists && !opt.ForceLocalTemplates {
			writes = append(writes, WriteResult{Path: targetRel, Kind: "template-file", Action: "skip-existing-local-file", SourcePath: source, TargetPath: dest})
			continue
		}
		action := "create-local-template-file"
		backupPath := ""
		if destExists && opt.ForceLocalTemplates {
			action = "overwrite-local-template-file-with-force"
			backupPath = previewBackupPath(dest, caseRoot, backupRoot)
		}
		writes = append(writes, WriteResult{Path: targetRel, Kind: "template-file", Action: action, SourcePath: source, TargetPath: dest, BackupPath: backupPath})
	}
	blockSource, err := m.SourcePath(m.ManagedBlock["source"])
	if err != nil {
		return nil, err
	}
	blockHost, err := refsf.SafeJoin(caseRoot, m.ManagedBlock["file"])
	if err != nil {
		return nil, err
	}
	hostText, hostExists, err := review.ReadTextIfExists(blockHost)
	if err != nil {
		return nil, err
	}
	blockText, err := readSourceText(blockSource, opt.CreateLocalFiles)
	if err != nil {
		return nil, err
	}
	nextHostText := review.ApplyManagedBlock(hostText, m.ManagedBlock["blockId"], blockText)
	blockBackup := ""
	if hostExists {
		blockBackup = previewBackupPath(blockHost, caseRoot, backupRoot)
	}
	writes = append(writes, WriteResult{Path: m.ManagedBlock["file"], Kind: "managed-block", Action: managedBlockAction(hostText, nextHostText, m.ManagedBlock["blockId"]), SourcePath: blockSource, TargetPath: blockHost, BackupPath: blockBackup})
	gitignoreSource, err := m.SourcePath("examples/gitignore.example")
	if err == nil && refsf.Exists(gitignoreSource) {
		gitignoreTarget, err := refsf.SafeJoin(caseRoot, ".gitignore")
		if err != nil {
			return nil, err
		}
		action := "create-support-file"
		if refsf.Exists(gitignoreTarget) {
			action = "skip-existing-support-file"
		}
		writes = append(writes, WriteResult{Path: ".gitignore", Kind: "support-file", Action: action, SourcePath: gitignoreSource, TargetPath: gitignoreTarget})
	}
	stateRel := filepath.ToSlash(filepath.Join(stateRoot.Dir, "state.json"))
	writes = append(writes, WriteResult{Path: stateRel, Kind: "sync-state", Action: "refresh", TargetPath: filepath.Join(stateRoot.Path, "state.json")})
	return writes, nil
}

func previewBackupPath(path, caseRoot, backupRoot string) string {
	caseFull, err := filepath.Abs(caseRoot)
	if err != nil {
		return ""
	}
	pathFull, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(caseFull, pathFull)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return ""
	}
	return filepath.Join(backupRoot, rel)
}

func readSourceText(path string, canonical bool) (string, error) {
	var (
		data []byte
		err  error
	)
	if canonical {
		data, err = sourceartifact.ReadCanonical(path)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func syncBackupRoot(caseRoot string, m *manifest.Manifest) (string, error) {
	backupRel := strings.TrimSpace(m.WorkstreamDefaults["backupRoot"])
	stamp := time.Now().Format("20060102-150405")
	if backupRel == "" {
		return projectstate.Join(caseRoot, "backups", "sync", stamp)
	}
	slash := strings.ReplaceAll(backupRel, `\`, "/")
	for _, stateDir := range []string{projectstate.CurrentDir, projectstate.LegacyDir} {
		if local, ok := strings.CutPrefix(slash, stateDir+"/"); ok {
			return projectstate.Join(caseRoot, local, stamp)
		}
	}
	if _, err := projectstate.Resolve(caseRoot); err != nil {
		return "", err
	}
	return refsf.SafeJoin(caseRoot, filepath.ToSlash(filepath.Join(filepath.FromSlash(backupRel), stamp)))
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

func writeSyncState(caseRoot string, m *manifest.Manifest, canonical bool) (string, error) {
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
		sourceText, err := readSourceText(source, canonical)
		if err != nil {
			return "", err
		}
		managed[rel] = syncManagedEntry{SourceHash: sha256Bytes([]byte(sourceText)), TargetHashAtSync: review.FileHash(target), LastAction: "sync"}
	}
	state := syncState{SchemaVersion: 1, TemplateRoot: m.RepoRoot, TemplatePack: m.Pack, LastSyncAt: time.Now().Format("2006-01-02T15:04:05-07:00"), Managed: managed}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return "", err
	}
	statePath, err := projectstate.Join(caseRoot, "state.json")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(statePath, append(b, '\n'), 0o644); err != nil {
		return "", err
	}
	return statePath, nil
}
