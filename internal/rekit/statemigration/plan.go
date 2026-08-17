package statemigration

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	"github.com/shuiyu486/re-context-kits/internal/rekit/caseshim"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
	"github.com/shuiyu486/re-context-kits/internal/rekit/sourceartifact"
)

const (
	maxLegacyFiles = 4096
	maxLegacyBytes = int64(128 << 20)
	maxStateFile   = int64(16 << 20)
	maxMetadata    = int64(8192)
	maxSkill       = int64(16384)
)

type plannedFile struct {
	rel    string
	kind   string
	data   []byte
	source string
	mode   os.FileMode
}

type preparedPlan struct {
	caseInfo       os.FileInfo
	sourceInfo     os.FileInfo
	legacyEntries  []InventoryEntry
	plannedEntries []InventoryEntry
	publications   []plannedFile
	receipt        []byte
	receiptValue   Receipt
	legacyMetadata []byte
	legacySkill    []byte
}

type planIdentity struct {
	SchemaVersion      int               `json:"schemaVersion"`
	Kind               string            `json:"kind"`
	Command            string            `json:"command"`
	CaseRoot           string            `json:"caseRoot"`
	RepoRoot           string            `json:"repoRoot"`
	Pack               string            `json:"pack"`
	ProjectName        string            `json:"projectName"`
	CaseRootIdentity   Identity          `json:"caseRootIdentity"`
	SourceRootIdentity Identity          `json:"sourceRootIdentity"`
	LegacyInventory    Inventory         `json:"legacyInventory"`
	LegacyInstance     FileBinding       `json:"legacyInstance"`
	LegacyState        FileBinding       `json:"legacyState"`
	LegacyMetadata     *FileBinding      `json:"legacyMetadata,omitempty"`
	LegacySkill        FileBinding       `json:"legacySkill"`
	CurrentInstance    FileBinding       `json:"currentInstance"`
	CurrentState       FileBinding       `json:"currentState"`
	CurrentSkill       FileBinding       `json:"currentSkill"`
	BundleManifest     FileBinding       `json:"bundleManifest"`
	Writes             []Write           `json:"writes"`
	Publications       []publicationHash `json:"publications"`
}

type publicationHash struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
	Mode   uint32 `json:"mode"`
}

func Preview(repoRoot, caseRoot, pack string) (Plan, error) {
	return build(repoRoot, caseRoot, pack)
}

func build(repoRoot, caseRoot, pack string) (Plan, error) {
	caseRoot, err := filepath.Abs(strings.TrimSpace(caseRoot))
	if err != nil {
		return Plan{}, err
	}
	caseRoot = filepath.Clean(caseRoot)
	if _, err := rekitfs.ValidateNonReparseDirectory(caseRoot, "state migration project root"); err != nil {
		return Plan{}, err
	}
	caseRootHandle, caseIdentity, err := OpenRootIdentity(caseRoot)
	if err != nil {
		return Plan{}, err
	}
	caseInfo, err := caseRootHandle.Lstat(".")
	_ = caseRootHandle.Close()
	if err != nil {
		return Plan{}, err
	}

	currentExists, currentInfo, err := lstatOptional(filepath.Join(caseRoot, ".steamai"))
	if err != nil {
		return Plan{}, err
	}
	legacyExists, legacyInfo, err := lstatOptional(filepath.Join(caseRoot, ".rekit"))
	if err != nil {
		return Plan{}, err
	}
	if currentExists && legacyExists {
		return Plan{}, fmt.Errorf("state migration refuses dual roots: both .rekit and .steamai exist")
	}
	if currentExists {
		return currentPlan(caseRoot, pack, currentInfo)
	}
	if !legacyExists {
		return Plan{}, fmt.Errorf("state migration requires an attached legacy-only project with .rekit")
	}
	if !legacyInfo.IsDir() || legacyInfo.Mode()&os.ModeSymlink != 0 {
		return Plan{}, fmt.Errorf("legacy state root must be a non-symlink directory: %s", filepath.Join(caseRoot, ".rekit"))
	}
	if err := rekitfs.ValidateTreeNoReparse(filepath.Join(caseRoot, ".rekit"), "legacy state tree"); err != nil {
		return Plan{}, err
	}

	legacyRoot, sourceIdentity, err := OpenRootIdentity(filepath.Join(caseRoot, ".rekit"))
	if err != nil {
		return Plan{}, err
	}
	sourceInfo, err := legacyRoot.Lstat(".")
	if err != nil {
		_ = legacyRoot.Close()
		return Plan{}, err
	}
	entries, contents, inventory, err := inventoryRoot(legacyRoot, ".rekit")
	if err != nil {
		_ = legacyRoot.Close()
		return Plan{}, err
	}
	if err := legacyRoot.Close(); err != nil {
		return Plan{}, err
	}

	legacyInstance, ok := contents["instance.yml"]
	if !ok {
		return Plan{}, fmt.Errorf("legacy state root is partial: missing .rekit/instance.yml")
	}
	legacyInstanceBinding := bindingFor(".rekit/instance.yml", legacyInstance)
	legacyState, ok := contents["state.json"]
	if !ok {
		return Plan{}, fmt.Errorf("legacy state root is partial: missing .rekit/state.json")
	}
	legacyStateBinding := bindingFor(".rekit/state.json", legacyState)
	inst, err := instance.Read(caseRoot)
	if err != nil {
		return Plan{}, err
	}
	if inst.Source != "instance" || inst.StateDir != ".rekit" || inst.SchemaVersion != 1 || inst.Moved() {
		return Plan{}, fmt.Errorf("state migration requires a valid attached legacy .rekit instance")
	}
	if strings.TrimSpace(inst.TemplateRoot) == "" {
		return Plan{}, fmt.Errorf("legacy instance metadata omits templateRoot")
	}
	repoRoot, err = filepath.Abs(strings.TrimSpace(repoRoot))
	if err != nil {
		return Plan{}, err
	}
	if !casebind.SameExistingPath(inst.TemplateRoot, repoRoot) {
		return Plan{}, fmt.Errorf("legacy project is attached to a different templateRoot: %s", inst.TemplateRoot)
	}
	if strings.TrimSpace(pack) == "" {
		pack = inst.TemplatePack
	}
	if !strings.EqualFold(strings.TrimSpace(pack), strings.TrimSpace(inst.TemplatePack)) {
		return Plan{}, fmt.Errorf("legacy project is attached to a different templatePack: %s", inst.TemplatePack)
	}
	pack = inst.TemplatePack
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return Plan{}, err
	}
	if err := m.ValidateSchema(); err != nil {
		return Plan{}, err
	}

	legacySkillRoot := filepath.Join(caseRoot, ".claude", "skills", "rekit")
	if err := validateExactLegacySkillTree(legacySkillRoot); err != nil {
		return Plan{}, err
	}
	legacySkillPath := filepath.Join(legacySkillRoot, "SKILL.md")
	legacySkill, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, legacySkillPath, "legacy project skill", maxSkill)
	if err != nil {
		return Plan{}, fmt.Errorf("state migration requires the exact legacy project skill: %w", err)
	}
	readiness := caseshim.InspectInstalled(repoRoot, caseRoot)
	if !readiness.Ready || !readiness.MatchesTemplate {
		return Plan{}, fmt.Errorf("legacy project skill is not current: %s", strings.Join(readiness.Warnings, "; "))
	}
	currentSkillPath := filepath.Join(caseRoot, ".claude", "skills", "steamai", "SKILL.md")
	if exists, _, err := lstatOptional(currentSkillPath); err != nil {
		return Plan{}, err
	} else if exists {
		return Plan{}, fmt.Errorf("state migration refuses partial current project skill: %s", currentSkillPath)
	}

	legacyMetadataPath := filepath.Join(caseRoot, ".re-template.yml")
	exists, info, err := lstatOptional(legacyMetadataPath)
	if err != nil {
		return Plan{}, err
	}
	if !exists {
		return Plan{}, fmt.Errorf("state migration requires complete legacy metadata: %s", legacyMetadataPath)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return Plan{}, fmt.Errorf("legacy metadata must be a regular non-symlink file: %s", legacyMetadataPath)
	}
	legacyMetadata, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, legacyMetadataPath, "legacy project metadata", maxMetadata)
	if err != nil {
		return Plan{}, err
	}
	binding := bindingFor(".re-template.yml", legacyMetadata)
	legacyMetadataBinding := &binding

	projectName := strings.TrimSpace(inst.ProjectName)
	if projectName == "" {
		projectName = casebind.ProjectNameFromRoot(caseRoot)
	}
	bundle, err := runtimebundle.Build(repoRoot, pack)
	if err != nil {
		return Plan{}, err
	}
	currentInstance := []byte(casebind.STeamAIInstanceText(caseRoot, pack, projectName, runtimebundle.ManifestRel, bundle.ManifestSHA256))
	currentState, err := relocatableState(legacyState, pack)
	if err != nil {
		return Plan{}, err
	}
	if bytes.Equal(currentInstance, legacyInstance) || bytes.Equal(currentState, legacyState) {
		return Plan{}, fmt.Errorf("legacy metadata does not require the exact relocatable migration replacement")
	}
	currentSkill, err := sourceartifact.ReadCanonical(filepath.Join(repoRoot, "rekit", "templates", "steamai-project", "SKILL.md"))
	if err != nil {
		return Plan{}, err
	}

	publications := make([]plannedFile, 0, len(bundle.Publications)+3)
	for _, publication := range bundle.Publications {
		data := publication.Content
		if publication.SourcePath != "" {
			data, err = stableSource(publication.SourcePath)
			if err != nil {
				return Plan{}, err
			}
		}
		mode := os.FileMode(0o644)
		if publication.Kind == "runtime-executable" {
			mode = 0o755
		}
		publications = append(publications, plannedFile{rel: filepath.ToSlash(publication.Path), kind: publication.Kind, data: data, source: publication.SourcePath, mode: mode})
	}
	publications = append(publications,
		plannedFile{rel: "instance.yml", kind: "instance-metadata", data: currentInstance, mode: 0o644},
		plannedFile{rel: "state.json", kind: "sync-state", data: currentState, mode: 0o644},
	)
	sort.Slice(publications, func(i, j int) bool { return publications[i].rel < publications[j].rel })

	legacySkillBinding := bindingFor(".claude/skills/rekit/SKILL.md", legacySkill)
	currentInstanceBinding := bindingFor(".steamai/instance.yml", currentInstance)
	currentStateBinding := bindingFor(".steamai/state.json", currentState)
	currentSkillBinding := bindingFor(".claude/skills/steamai/SKILL.md", currentSkill)
	bundleBinding := bindingFor(".steamai/"+runtimebundle.ManifestRel, bundle.ManifestData)
	writes := plannedWrites(publications, currentSkillBinding, legacySkillBinding)
	receiptWrite := Write{Path: ReceiptRel, Kind: "state-root-migration-receipt", Action: "create"}
	identityWrites := append(append([]Write(nil), writes...), receiptWrite)
	publicationBindings := make([]publicationHash, 0, len(publications)+1)
	for _, publication := range publications {
		publicationBindings = append(publicationBindings, publicationHash{Path: ".steamai/" + publication.rel, Kind: publication.kind, SHA256: sha256Hex(publication.data), Size: int64(len(publication.data)), Mode: uint32(publication.mode.Perm())})
	}
	publicationBindings = append(publicationBindings, publicationHash{Path: currentSkillBinding.Path, Kind: "project-local-steamai-skill", SHA256: currentSkillBinding.SHA256, Size: currentSkillBinding.Size, Mode: 0o644})

	identity := planIdentity{
		SchemaVersion: SchemaVersion, Kind: PlanKind, Command: Command, CaseRoot: caseRoot, RepoRoot: repoRoot, Pack: pack, ProjectName: projectName,
		CaseRootIdentity: caseIdentity, SourceRootIdentity: sourceIdentity, LegacyInventory: inventory,
		LegacyInstance: legacyInstanceBinding, LegacyState: legacyStateBinding, LegacyMetadata: legacyMetadataBinding, LegacySkill: legacySkillBinding,
		CurrentInstance: currentInstanceBinding, CurrentState: currentStateBinding, CurrentSkill: currentSkillBinding, BundleManifest: bundleBinding,
		Writes: identityWrites, Publications: publicationBindings,
	}
	planSHA, err := canonicalSHA(identity)
	if err != nil {
		return Plan{}, err
	}
	plannedEntries := replaceInventoryEntries(entries, currentInstanceBinding, currentStateBinding, publications)
	plannedInventory := inventoryForEntries(".steamai", plannedEntries)
	receipt := Receipt{
		SchemaVersion: SchemaVersion, Kind: ReceiptKind, Command: Command, State: "committed", PlanSHA256: planSHA, Pack: pack,
		Before:   ReceiptState{StateRoot: ".rekit", RootIdentity: sourceIdentity, InventorySHA: inventory.SHA256, Files: inventory.Files, Bytes: inventory.Bytes},
		After:    ReceiptState{StateRoot: ".steamai", RootIdentity: sourceIdentity, InventorySHA: plannedInventory.SHA256, Files: plannedInventory.Files, Bytes: plannedInventory.Bytes},
		Instance: currentInstanceBinding, StateMetadata: currentStateBinding, Skill: currentSkillBinding, BundleManifest: bundleBinding,
		LegacyInstance: legacyInstanceBinding, LegacyState: legacyStateBinding, LegacyMetadata: legacyMetadataBinding, LegacySkill: legacySkillBinding,
		NoAuthority: true, NoConfirmed: true, NoHeavyTool: true, NoSyncPromote: true,
	}
	receiptBytes, err := canonical(receipt)
	if err != nil {
		return Plan{}, err
	}
	receiptPublication := plannedFile{rel: strings.TrimPrefix(ReceiptRel, ".steamai/"), kind: "state-root-migration-receipt", data: receiptBytes, mode: 0o600}
	publications = append(publications, receiptPublication)
	writes = append(writes, receiptWrite)

	applyArgs := []string{"-Command", Command, "-Target", caseRoot, "-Pack", pack, "-ExpectedMigrationPlanSha256", planSHA, "-Apply", "-Format", "json"}
	return Plan{
		SchemaVersion: SchemaVersion, Kind: PlanKind, Command: Command, Status: "ready-to-migrate", CaseRoot: caseRoot, RepoRoot: repoRoot, Pack: pack, ProjectName: projectName,
		SourceStateRoot: ".rekit", TargetStateRoot: ".steamai", CaseRootIdentity: caseIdentity, SourceRootIdentity: sourceIdentity,
		LegacyInventory: inventory, PlannedInventory: plannedInventory, LegacyInstance: legacyInstanceBinding, LegacyState: legacyStateBinding,
		LegacyMetadata: legacyMetadataBinding, LegacySkill: legacySkillBinding, CurrentInstance: currentInstanceBinding, CurrentState: currentStateBinding,
		CurrentSkill: currentSkillBinding, BundleManifest: bundleBinding, Writes: writes, ExpectedPlanSHA256: planSHA, ApplyArgs: applyArgs,
		RequiresReview: true, RequiresConfirmation: true,
		BlockedActions: []string{"dual-root merge", "authority/confirmed writes", "gate or autonomy expansion", "sync/promote", "heavy-tool execution"},
		NextSteps:      []string{"review the complete legacy inventory and planned publications", "run the exact ApplyArgs with the expected migration plan SHA-256"},
		prepared:       &preparedPlan{caseInfo: caseInfo, sourceInfo: sourceInfo, legacyEntries: entries, plannedEntries: plannedEntries, publications: publications, receipt: receiptBytes, receiptValue: receipt, legacyMetadata: legacyMetadata, legacySkill: legacySkill},
	}, nil
}

func currentPlan(caseRoot, pack string, currentInfo os.FileInfo) (Plan, error) {
	if !currentInfo.IsDir() || currentInfo.Mode()&os.ModeSymlink != 0 {
		return Plan{}, fmt.Errorf("current state root must be a non-symlink directory: %s", filepath.Join(caseRoot, ".steamai"))
	}
	if err := rekitfs.ValidateTreeNoReparse(filepath.Join(caseRoot, ".steamai"), "current state tree"); err != nil {
		return Plan{}, err
	}
	migrationPath := filepath.Join(caseRoot, ".steamai", "migration")
	migrationExists, migrationInfo, err := lstatOptional(migrationPath)
	if err != nil {
		return Plan{}, err
	}
	if migrationExists {
		if !migrationInfo.IsDir() || migrationInfo.Mode()&os.ModeSymlink != 0 {
			return Plan{}, fmt.Errorf("current project has an invalid state migration namespace")
		}
		if err := validateExactMigrationTree(migrationPath); err != nil {
			return Plan{}, err
		}
	}
	if exists, _, err := lstatOptional(filepath.Join(caseRoot, ".claude", "skills", "rekit", "SKILL.md")); err != nil {
		return Plan{}, err
	} else if exists {
		return Plan{}, fmt.Errorf("current project contains a leftover legacy /rekit skill")
	}
	inst, err := instance.Read(caseRoot)
	if err != nil {
		return Plan{}, err
	}
	if inst.Source != "steamai" || inst.SchemaVersion != 2 || inst.Mode != "project-local-bundle" || inst.Moved() {
		return Plan{}, fmt.Errorf("current state root is partial or has invalid relocatable metadata")
	}
	if strings.TrimSpace(pack) != "" && !strings.EqualFold(pack, inst.TemplatePack) {
		return Plan{}, fmt.Errorf("current project uses a different pack: %s", inst.TemplatePack)
	}
	pack = inst.TemplatePack
	if _, err := runtimebundle.Validate(filepath.Join(caseRoot, ".steamai"), inst.BundleManifest, inst.BundleManifestSHA256, pack); err != nil {
		return Plan{}, fmt.Errorf("current project runtime bundle is invalid: %w", err)
	}
	currentInstancePath := filepath.Join(caseRoot, ".steamai", "instance.yml")
	currentInstance, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, currentInstancePath, "current instance metadata", maxMetadata)
	if err != nil {
		return Plan{}, fmt.Errorf("current instance metadata is invalid: %w", err)
	}
	currentStatePath := filepath.Join(caseRoot, ".steamai", "state.json")
	currentState, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, currentStatePath, "current state metadata", maxStateFile)
	if err != nil {
		return Plan{}, fmt.Errorf("current state metadata is invalid: %w", err)
	}
	currentSkillPath := filepath.Join(caseRoot, ".claude", "skills", "steamai", "SKILL.md")
	currentSkill, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, currentSkillPath, "current project skill", maxSkill)
	if err != nil {
		return Plan{}, fmt.Errorf("current project skill is invalid: %w", err)
	}
	bundledSkillPath := filepath.Join(caseRoot, ".steamai", "rekit", "templates", "steamai-project", "SKILL.md")
	bundledSkill, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, bundledSkillPath, "bundled current project skill", maxSkill)
	if err != nil {
		return Plan{}, fmt.Errorf("bundled current project skill is invalid: %w", err)
	}
	if !bytes.Equal(sourceartifact.SemanticText(currentSkill), sourceartifact.SemanticText(bundledSkill)) {
		return Plan{}, fmt.Errorf("current project skill differs from its verified runtime bundle")
	}
	manifestPath := filepath.Join(caseRoot, ".steamai", filepath.FromSlash(runtimebundle.ManifestRel))
	manifestBytes, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, manifestPath, "current runtime manifest", 1<<20)
	if err != nil {
		return Plan{}, fmt.Errorf("current runtime manifest is invalid: %w", err)
	}
	bundleManifest := bindingFor(".steamai/"+runtimebundle.ManifestRel, manifestBytes)
	if bundleManifest.SHA256 != inst.BundleManifestSHA256 {
		return Plan{}, fmt.Errorf("current runtime manifest binding differs from current metadata")
	}

	base := Plan{
		SchemaVersion: SchemaVersion, Kind: PlanKind, Command: Command, Status: "already-current", CaseRoot: caseRoot, RepoRoot: filepath.Join(caseRoot, ".steamai"), Pack: pack, ProjectName: inst.ProjectName,
		TargetStateRoot: ".steamai", IsMutation: false, Applied: false, Replay: false, AlreadyCurrent: true,
		CurrentInstance: bindingFor(".steamai/instance.yml", currentInstance), CurrentState: bindingFor(".steamai/state.json", currentState),
		CurrentSkill: bindingFor(".claude/skills/steamai/SKILL.md", currentSkill), BundleManifest: bundleManifest,
		RequiresReview: false, RequiresConfirmation: false,
		BlockedActions: []string{"dual-root merge", "authority/confirmed writes", "gate or autonomy expansion", "sync/promote", "heavy-tool execution"},
		NextSteps:      []string{"continue using the project-local /steamai entrypoint"},
	}
	if !migrationExists {
		return base, nil
	}
	receiptPath := filepath.Join(caseRoot, filepath.FromSlash(ReceiptRel))
	receiptBytes, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, receiptPath, "state migration receipt", 1<<20)
	if err != nil {
		return Plan{}, fmt.Errorf("current project has a partial or invalid state migration receipt: %w", err)
	}
	receipt, err := decodeReceipt(receiptBytes)
	if err != nil {
		return Plan{}, err
	}
	if receipt.Pack != pack {
		return Plan{}, fmt.Errorf("state migration receipt pack differs from current project metadata")
	}
	if err := validateCurrentReceiptBindings(caseRoot, inst, receipt); err != nil {
		return Plan{}, err
	}
	base.Replay = true
	base.ExpectedPlanSHA256 = receipt.PlanSHA256
	base.CurrentInstance = receipt.Instance
	base.CurrentState = receipt.StateMetadata
	base.CurrentSkill = receipt.Skill
	base.BundleManifest = receipt.BundleManifest
	base.prepared = &preparedPlan{receipt: receiptBytes, receiptValue: receipt}
	return base, nil
}

func validateCurrentReceiptBindings(caseRoot string, inst instance.Instance, receipt Receipt) error {
	bindings := []struct {
		binding FileBinding
		label   string
		limit   int64
	}{
		{receipt.Instance, "migrated instance metadata", maxMetadata},
		{receipt.StateMetadata, "migrated state metadata", maxStateFile},
		{receipt.Skill, "migrated project skill", maxSkill},
		{receipt.BundleManifest, "migrated runtime manifest", 1 << 20},
	}
	for _, candidate := range bindings {
		path := filepath.Join(caseRoot, filepath.FromSlash(candidate.binding.Path))
		data, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, path, candidate.label, candidate.limit)
		if err != nil {
			return fmt.Errorf("read state migration receipt binding from current project: %s: %w", candidate.binding.Path, err)
		}
		if sha256Hex(data) != candidate.binding.SHA256 || int64(len(data)) != candidate.binding.Size {
			return fmt.Errorf("state migration receipt binding differs from current project: %s", candidate.binding.Path)
		}
	}
	if inst.BundleManifestSHA256 != receipt.BundleManifest.SHA256 {
		return fmt.Errorf("state migration receipt runtime manifest differs from current metadata")
	}
	legacyMetadataPath := filepath.Join(caseRoot, filepath.FromSlash(receipt.LegacyMetadata.Path))
	legacyMetadata, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, legacyMetadataPath, "migrated legacy compatibility metadata", maxMetadata)
	if err != nil {
		return fmt.Errorf("read state migration legacy metadata binding from current project: %w", err)
	}
	if sha256Hex(legacyMetadata) != receipt.LegacyMetadata.SHA256 || int64(len(legacyMetadata)) != receipt.LegacyMetadata.Size {
		return fmt.Errorf("state migration legacy metadata binding differs from current project")
	}
	root, _, err := OpenRootIdentity(filepath.Join(caseRoot, ".steamai"))
	if err != nil {
		return err
	}
	defer root.Close()
	entries, _, _, err := inventoryRoot(root, ".steamai")
	if err != nil {
		return err
	}
	if inventoryWithoutReceipt(entries).SHA256 != receipt.After.InventorySHA {
		return fmt.Errorf("current state inventory differs from migration receipt")
	}
	return nil
}

func inventoryRoot(root *os.Root, stateRoot string) ([]InventoryEntry, map[string][]byte, Inventory, error) {
	entries := []InventoryEntry{}
	contents := map[string][]byte{}
	var total int64
	var walk func(string) error
	walk = func(directory string) error {
		file, err := root.Open(directory)
		if err != nil {
			return err
		}
		children, readErr := file.ReadDir(-1)
		closeErr := file.Close()
		if readErr != nil || closeErr != nil {
			return errors.Join(readErr, closeErr)
		}
		sort.Slice(children, func(i, j int) bool { return children[i].Name() < children[j].Name() })
		for _, child := range children {
			name := child.Name()
			rel := name
			if directory != "." {
				rel = filepath.Join(directory, name)
			}
			info, err := root.Lstat(rel)
			if err != nil {
				return fmt.Errorf("inspect legacy state entry %s: %w", filepath.ToSlash(rel), err)
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("legacy state entry is invalid: %s", filepath.ToSlash(rel))
			}
			if info.IsDir() {
				entries = append(entries, InventoryEntry{Path: filepath.ToSlash(rel), Kind: "directory"})
				if err := walk(rel); err != nil {
					return err
				}
				continue
			}
			if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > maxStateFile {
				return fmt.Errorf("legacy state entry must be a bounded regular file or directory: %s", filepath.ToSlash(rel))
			}
			data, err := readRootFile(root, rel, info, maxStateFile)
			if err != nil {
				return err
			}
			total += int64(len(data))
			if total > maxLegacyBytes {
				return fmt.Errorf("legacy state tree exceeds %d bytes", maxLegacyBytes)
			}
			entry := InventoryEntry{Path: filepath.ToSlash(rel), Kind: "file", SHA256: sha256Hex(data), Size: int64(len(data))}
			entries = append(entries, entry)
			contents[entry.Path] = data
			if len(entries) > maxLegacyFiles*4 || len(contents) > maxLegacyFiles {
				return fmt.Errorf("legacy state tree exceeds %d files", maxLegacyFiles)
			}
		}
		return nil
	}
	if err := walk("."); err != nil {
		return nil, nil, Inventory{}, err
	}
	return entries, contents, inventoryForEntries(stateRoot, entries), nil
}

func readRootFile(root *os.Root, rel string, before os.FileInfo, limit int64) ([]byte, error) {
	file, err := root.Open(rel)
	if err != nil {
		return nil, err
	}
	opened, statErr := file.Stat()
	data, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	closeErr := file.Close()
	after, afterErr := root.Lstat(rel)
	if statErr != nil || readErr != nil || closeErr != nil || afterErr != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(opened, after) || int64(len(data)) != opened.Size() || int64(len(data)) > limit {
		return nil, fmt.Errorf("legacy state file changed while reading: %s: %w", filepath.ToSlash(rel), errors.Join(statErr, readErr, closeErr, afterErr))
	}
	return data, nil
}

func inventoryForEntries(stateRoot string, entries []InventoryEntry) Inventory {
	clone := append([]InventoryEntry(nil), entries...)
	sort.Slice(clone, func(i, j int) bool { return clone[i].Path < clone[j].Path })
	data, _ := json.Marshal(clone)
	var files int
	var total int64
	for _, entry := range clone {
		if entry.Kind == "file" {
			files++
			total += entry.Size
		}
	}
	return Inventory{StateRoot: stateRoot, SHA256: sha256Hex(data), Files: files, Bytes: total, Entries: clone}
}

func replaceInventoryEntries(legacy []InventoryEntry, currentInstance, currentState FileBinding, publications []plannedFile) []InventoryEntry {
	byPath := map[string]InventoryEntry{}
	for _, entry := range legacy {
		if entry.Path == "instance.yml" || entry.Path == "state.json" {
			continue
		}
		byPath[entry.Path] = entry
	}
	for _, publication := range publications {
		byPath[publication.rel] = InventoryEntry{Path: publication.rel, Kind: "file", SHA256: sha256Hex(publication.data), Size: int64(len(publication.data))}
		for parent := filepath.ToSlash(filepath.Dir(publication.rel)); parent != "." && parent != ""; parent = filepath.ToSlash(filepath.Dir(parent)) {
			byPath[parent] = InventoryEntry{Path: parent, Kind: "directory"}
		}
	}
	byPath["instance.yml"] = InventoryEntry{Path: "instance.yml", Kind: "file", SHA256: currentInstance.SHA256, Size: currentInstance.Size}
	byPath["state.json"] = InventoryEntry{Path: "state.json", Kind: "file", SHA256: currentState.SHA256, Size: currentState.Size}
	out := make([]InventoryEntry, 0, len(byPath))
	for _, entry := range byPath {
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func plannedWrites(publications []plannedFile, currentSkill, legacySkill FileBinding) []Write {
	writes := []Write{{Path: ".rekit", Kind: "legacy-state-root", Action: "rename-to-.steamai", SourcePath: ".rekit"}}
	for _, publication := range publications {
		action := "create"
		if publication.rel == "instance.yml" || publication.rel == "state.json" {
			action = "replace-after-rename"
		}
		writes = append(writes, Write{Path: ".steamai/" + publication.rel, Kind: publication.kind, Action: action, SHA256: sha256Hex(publication.data), Size: int64(len(publication.data)), SourcePath: publication.source})
	}
	writes = append(writes,
		Write{Path: currentSkill.Path, Kind: "project-local-steamai-skill", Action: "create", SHA256: currentSkill.SHA256, Size: currentSkill.Size},
		Write{Path: legacySkill.Path, Kind: "legacy-project-skill", Action: "remove-after-current-skill-publication", SHA256: legacySkill.SHA256, Size: legacySkill.Size},
	)
	return writes
}

func relocatableState(data []byte, pack string) ([]byte, error) {
	var state map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&state); err != nil {
		return nil, fmt.Errorf("legacy state.json is invalid: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("legacy state.json has trailing data")
	}
	var version int
	if err := json.Unmarshal(state["schemaVersion"], &version); err != nil || version != 1 {
		return nil, fmt.Errorf("legacy state.json schemaVersion must be 1")
	}
	var statePack string
	if err := json.Unmarshal(state["templatePack"], &statePack); err != nil || !strings.EqualFold(strings.TrimSpace(statePack), strings.TrimSpace(pack)) {
		return nil, fmt.Errorf("legacy state.json templatePack does not match attached metadata")
	}
	state["templateRoot"] = json.RawMessage(strconv.Quote("."))
	return canonicalRawObject(state)
}

func canonicalRawObject(value map[string]json.RawMessage) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func stableSource(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("migration bundle source is unavailable: %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("migration bundle source is not a regular file: %s", path)
	}
	limit := maxStateFile
	if info.Size() > limit {
		limit = info.Size()
	}
	return rekitfs.ReadStableRegularFileAnchored(filepath.Dir(path), path, "migration bundle source", limit+1)
}

func validateExactLegacySkillTree(root string) error {
	if _, err := rekitfs.ValidateNonReparseDirectory(root, "legacy project skill root"); err != nil {
		return fmt.Errorf("state migration requires a complete legacy project skill: %w", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	if len(entries) != 1 || entries[0].Name() != "SKILL.md" {
		return fmt.Errorf("legacy project skill root contains unplanned residue")
	}
	info, err := entries[0].Info()
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("legacy project skill must be one regular non-symlink SKILL.md")
	}
	return nil
}

func validateExactMigrationTree(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("read state migration namespace: %w", err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(ReceiptRel) {
		return fmt.Errorf("current project has a partial state migration namespace")
	}
	info, err := entries[0].Info()
	if err != nil {
		return fmt.Errorf("current project has an invalid state migration receipt: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() < 1 {
		return fmt.Errorf("current project has an invalid state migration receipt")
	}
	return nil
}

func lstatOptional(path string) (bool, os.FileInfo, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil, nil
	}
	return err == nil, info, err
}

func bindingFor(path string, data []byte) FileBinding {
	return FileBinding{Path: filepath.ToSlash(path), SHA256: sha256Hex(data), Size: int64(len(data))}
}

func canonical(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func canonicalSHA(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validSHA256(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validFileBinding(binding FileBinding, path string) bool {
	return binding.Path == path && validSHA256(binding.SHA256) && binding.Size > 0
}

func decodeReceipt(data []byte) (Receipt, error) {
	var receipt Receipt
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return Receipt{}, fmt.Errorf("invalid state migration receipt: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Receipt{}, fmt.Errorf("invalid state migration receipt trailing data")
	}
	canonicalData, err := canonical(receipt)
	if err != nil || !bytes.Equal(data, canonicalData) {
		return Receipt{}, fmt.Errorf("state migration receipt is not canonical")
	}
	if receipt.SchemaVersion != SchemaVersion || receipt.Kind != ReceiptKind || receipt.Command != Command || receipt.State != "committed" || !validSHA256(receipt.PlanSHA256) || strings.TrimSpace(receipt.Pack) == "" || receipt.Before.StateRoot != ".rekit" || receipt.After.StateRoot != ".steamai" || !validSHA256(receipt.Before.InventorySHA) || !validSHA256(receipt.After.InventorySHA) || !validFileBinding(receipt.Instance, ".steamai/instance.yml") || !validFileBinding(receipt.StateMetadata, ".steamai/state.json") || !validFileBinding(receipt.Skill, ".claude/skills/steamai/SKILL.md") || !validFileBinding(receipt.BundleManifest, ".steamai/"+runtimebundle.ManifestRel) || !validFileBinding(receipt.LegacyInstance, ".rekit/instance.yml") || !validFileBinding(receipt.LegacyState, ".rekit/state.json") || !validFileBinding(receipt.LegacySkill, ".claude/skills/rekit/SKILL.md") || !receipt.NoAuthority || !receipt.NoConfirmed || !receipt.NoHeavyTool || !receipt.NoSyncPromote {
		return Receipt{}, fmt.Errorf("state migration receipt identity or safety boundary is invalid")
	}
	if receipt.LegacyMetadata == nil || !validFileBinding(*receipt.LegacyMetadata, ".re-template.yml") {
		return Receipt{}, fmt.Errorf("state migration receipt legacy metadata binding is invalid")
	}
	if err := receipt.Before.RootIdentity.Validate(); err != nil {
		return Receipt{}, err
	}
	if err := receipt.After.RootIdentity.Validate(); err != nil {
		return Receipt{}, err
	}
	if receipt.Before.RootIdentity != receipt.After.RootIdentity {
		return Receipt{}, fmt.Errorf("state migration receipt must preserve state root filesystem identity")
	}
	return receipt, nil
}
