package sync

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
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/review"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
	"github.com/shuiyu486/re-context-kits/internal/rekit/sourceartifact"
	"github.com/shuiyu486/re-context-kits/internal/rekit/statemigration"
)

const (
	currentSyncSchemaVersion = 1
	currentSyncPlanKind      = "steamai-current-sync-plan"
	currentSyncIntentKind    = "steamai-current-sync-intent"
	currentSyncProgressKind  = "steamai-current-sync-progress"
	currentSyncReceiptKind   = "steamai-current-sync-receipt"
	currentSyncResultKind    = "steamai-current-sync-result"

	currentSyncNamespaceRel    = "maintenance/current-sync-v1"
	currentSyncTransactionsRel = currentSyncNamespaceRel + "/transactions"
	currentSyncOwnerRel        = currentSyncNamespaceRel + "/owner.json"
	currentSyncIntentRel       = currentSyncNamespaceRel + "/intent.json"
	currentSyncProgressDirName = "progress"
	currentSyncArchivedIntent  = "intent.json"
	currentSyncReceiptRel      = currentSyncNamespaceRel + "/receipt.json"

	currentSyncMaxFileBytes = int64(128 << 20)
	currentSyncMaxFiles     = 4096
)

var currentSyncControlledRoots = []string{"common", "packs", "rekit", "runtime"}

// CurrentSyncOptions selects the exact central maintenance source used to
// refresh an already-attached schema-v2 STeamAI project.
type CurrentSyncOptions struct {
	Command             string
	ProjectName         string
	ForceLocalTemplates bool
	SourceExecutable    string
}

// CurrentSyncApplyOptions binds the explicit central maintenance invocation to
// the reviewed current project refresh. SourceRepoRoot is kept separate from
// CurrentSyncOptions so a target-local runtime can never become its own source.
type CurrentSyncApplyOptions struct {
	SourceRepoRoot     string
	ExpectedPlanSHA256 string
	CurrentSyncOptions CurrentSyncOptions
}

// CurrentSyncBinding binds one exact regular file into the reviewed plan.
type CurrentSyncBinding struct {
	Path     string                   `json:"path"`
	Kind     string                   `json:"kind"`
	SHA256   string                   `json:"sha256"`
	Size     int64                    `json:"size"`
	Mode     uint32                   `json:"mode,omitempty"`
	Identity *statemigration.Identity `json:"identity,omitempty"`
}

// CurrentSyncInventory binds a complete sorted file inventory.
type CurrentSyncInventory struct {
	SHA256  string               `json:"sha256"`
	Files   int                  `json:"files"`
	Bytes   int64                `json:"bytes"`
	Entries []CurrentSyncBinding `json:"entries"`
}

// CurrentSyncWrite describes one reviewed target transition.
type CurrentSyncWrite struct {
	Path         string `json:"path"`
	Kind         string `json:"kind"`
	Action       string `json:"action"`
	SourcePath   string `json:"sourcePath,omitempty"`
	BeforeExists bool   `json:"beforeExists"`
	BeforeSHA256 string `json:"beforeSha256,omitempty"`
	BeforeSize   int64  `json:"beforeSize,omitempty"`
	AfterExists  bool   `json:"afterExists"`
	AfterSHA256  string `json:"afterSha256,omitempty"`
	AfterSize    int64  `json:"afterSize,omitempty"`
}

// CurrentSyncPlan is the review-first exact identity for a current project
// refresh. Runtime-only publication bytes are deliberately excluded from JSON.
type CurrentSyncPlan struct {
	SchemaVersion        int                     `json:"schemaVersion"`
	Kind                 string                  `json:"kind"`
	Command              string                  `json:"command"`
	Status               string                  `json:"status"`
	CaseRoot             string                  `json:"caseRoot"`
	SourceRepoRoot       string                  `json:"sourceRepoRoot"`
	SourceExecutable     string                  `json:"sourceExecutable"`
	Pack                 string                  `json:"pack"`
	ProjectName          string                  `json:"projectName"`
	ForceLocalTemplates  bool                    `json:"forceLocalTemplates"`
	CaseRootIdentity     statemigration.Identity `json:"caseRootIdentity"`
	StateRootIdentity    statemigration.Identity `json:"stateRootIdentity"`
	SourceRootIdentity   statemigration.Identity `json:"sourceRootIdentity"`
	SourceExecutableFile CurrentSyncBinding      `json:"sourceExecutableFile"`
	CurrentManifest      CurrentSyncBinding      `json:"currentManifest"`
	NextManifest         CurrentSyncBinding      `json:"nextManifest"`
	CurrentControlled    CurrentSyncInventory    `json:"currentControlled"`
	NextControlled       CurrentSyncInventory    `json:"nextControlled"`
	CurrentTargets       CurrentSyncInventory    `json:"currentTargets"`
	NextTargets          CurrentSyncInventory    `json:"nextTargets"`
	PreviousReceipt      *CurrentSyncBinding     `json:"previousReceipt,omitempty"`
	Writes               []CurrentSyncWrite      `json:"writes"`
	ObsoleteControlled   []string                `json:"obsoleteControlledFiles,omitempty"`
	ExpectedPlanSHA256   string                  `json:"expectedPlanSha256"`
	ApplyArgs            []string                `json:"applyArgs,omitempty"`
	IsMutation           bool                    `json:"isMutation"`
	Applied              bool                    `json:"applied"`
	Replay               bool                    `json:"replay"`
	AlreadyCurrent       bool                    `json:"alreadyCurrent"`
	RecoveryPending      bool                    `json:"recoveryPending"`
	RequiresReview       bool                    `json:"requiresReview"`
	RequiresConfirmation bool                    `json:"requiresConfirmation"`
	BlockedActions       []string                `json:"blockedActions"`
	Boundary             []string                `json:"boundary"`
	NextSteps            []string                `json:"nextSteps"`

	prepared *currentSyncPrepared
}

type currentSyncPublication struct {
	rel    string
	kind   string
	source string
	data   []byte
	mode   os.FileMode
}

type currentSyncLeaf struct {
	rel          string
	kind         string
	action       string
	source       string
	before       []byte
	beforeExists bool
	after        []byte
	afterExists  bool
	mode         os.FileMode
}

type currentSyncPrepared struct {
	caseInfo          os.FileInfo
	stateInfo         os.FileInfo
	publications      []currentSyncPublication
	leaves            []currentSyncLeaf
	currentRootFiles  map[string][]CurrentSyncBinding
	nextRootFiles     map[string][]CurrentSyncBinding
	currentManifest   runtimebundle.Manifest
	nextManifest      runtimebundle.Manifest
	previousReceipt   []byte
	previousReceiptOK bool
}

type currentSyncPlanIdentity struct {
	SchemaVersion        int                     `json:"schemaVersion"`
	Kind                 string                  `json:"kind"`
	Command              string                  `json:"command"`
	CaseRoot             string                  `json:"caseRoot"`
	SourceRepoRoot       string                  `json:"sourceRepoRoot"`
	SourceExecutable     string                  `json:"sourceExecutable"`
	Pack                 string                  `json:"pack"`
	ProjectName          string                  `json:"projectName"`
	ForceLocalTemplates  bool                    `json:"forceLocalTemplates"`
	CaseRootIdentity     statemigration.Identity `json:"caseRootIdentity"`
	StateRootIdentity    statemigration.Identity `json:"stateRootIdentity"`
	SourceRootIdentity   statemigration.Identity `json:"sourceRootIdentity"`
	SourceExecutableFile CurrentSyncBinding      `json:"sourceExecutableFile"`
	CurrentManifest      CurrentSyncBinding      `json:"currentManifest"`
	NextManifest         CurrentSyncBinding      `json:"nextManifest"`
	CurrentControlled    CurrentSyncInventory    `json:"currentControlled"`
	NextControlled       CurrentSyncInventory    `json:"nextControlled"`
	CurrentTargets       CurrentSyncInventory    `json:"currentTargets"`
	NextTargets          CurrentSyncInventory    `json:"nextTargets"`
	PreviousReceipt      *CurrentSyncBinding     `json:"previousReceipt,omitempty"`
	Writes               []CurrentSyncWrite      `json:"writes"`
	ObsoleteControlled   []string                `json:"obsoleteControlledFiles,omitempty"`
}

// CurrentSyncPreview returns a zero-write, exact plan. If a durable intent is
// present it returns that recovery identity instead of inventing a second plan.
func CurrentSyncPreview(repoRoot, caseRoot, pack string, opt CurrentSyncOptions) (CurrentSyncPlan, error) {
	caseFull, stateRoot, err := currentSyncRoots(caseRoot)
	if err != nil {
		return CurrentSyncPlan{}, err
	}
	pending, err := inspectCurrentSyncPendingOwnership(caseFull, stateRoot.Path)
	if err != nil {
		return CurrentSyncPlan{}, err
	}
	if pending.Exists {
		plan := pending.Intent.Plan
		plan.prepared = nil
		plan.Status = "recovery-required"
		plan.RecoveryPending = true
		plan.RequiresReview = true
		plan.RequiresConfirmation = true
		plan.NextSteps = []string{
			"resume only this exact current sync plan with its expected SHA-256",
			"do not start a replacement refresh while the durable intent is pending",
		}
		return plan, nil
	}
	return buildCurrentSyncPlan(repoRoot, caseFull, pack, opt)
}

func buildCurrentSyncPlan(repoRoot, caseRoot, pack string, opt CurrentSyncOptions) (CurrentSyncPlan, error) {
	if opt.ForceLocalTemplates {
		return CurrentSyncPlan{}, fmt.Errorf("current sync does not accept -Force; existing local, handoff, and authority files are preserved")
	}
	caseFull, stateRoot, err := currentSyncRoots(caseRoot)
	if err != nil {
		return CurrentSyncPlan{}, err
	}
	repoFull, err := filepath.Abs(strings.TrimSpace(repoRoot))
	if err != nil {
		return CurrentSyncPlan{}, err
	}
	repoFull = filepath.Clean(repoFull)
	if _, err := rekitfs.ValidateNonReparseDirectory(repoFull, "current sync source repository"); err != nil {
		return CurrentSyncPlan{}, err
	}
	if currentSyncPathsOverlap(repoFull, caseFull) {
		return CurrentSyncPlan{}, fmt.Errorf("current sync source repository must be external to the target project: %s", repoFull)
	}
	sourceExecutable := strings.TrimSpace(opt.SourceExecutable)
	if sourceExecutable == "" {
		return CurrentSyncPlan{}, fmt.Errorf("current sync requires an explicit central source executable")
	}
	sourceExecutable, err = filepath.Abs(sourceExecutable)
	if err != nil {
		return CurrentSyncPlan{}, err
	}
	sourceExecutable = filepath.Clean(sourceExecutable)
	if currentSyncPathInside(sourceExecutable, caseFull) {
		return CurrentSyncPlan{}, fmt.Errorf("current sync source executable must be external to the target project: %s", sourceExecutable)
	}

	caseHandle, caseIdentity, err := statemigration.OpenRootIdentity(caseFull)
	if err != nil {
		return CurrentSyncPlan{}, err
	}
	caseInfo, err := caseHandle.Lstat(".")
	closeCaseErr := caseHandle.Close()
	if err != nil || closeCaseErr != nil {
		return CurrentSyncPlan{}, errors.Join(err, closeCaseErr)
	}
	stateHandle, stateIdentity, err := statemigration.OpenRootIdentity(stateRoot.Path)
	if err != nil {
		return CurrentSyncPlan{}, err
	}
	stateInfo, err := stateHandle.Lstat(".")
	closeStateErr := stateHandle.Close()
	if err != nil || closeStateErr != nil {
		return CurrentSyncPlan{}, errors.Join(err, closeStateErr)
	}
	sourceHandle, sourceIdentity, err := statemigration.OpenRootIdentity(repoFull)
	if err != nil {
		return CurrentSyncPlan{}, err
	}
	if err := sourceHandle.Close(); err != nil {
		return CurrentSyncPlan{}, err
	}

	inst, err := instance.Read(caseFull)
	if err != nil {
		return CurrentSyncPlan{}, err
	}
	if inst.Source != "steamai" || inst.StateDir != projectstate.CurrentDir || inst.SchemaVersion != 2 || inst.Mode != "project-local-bundle" || inst.Moved() {
		return CurrentSyncPlan{}, fmt.Errorf("current sync requires a valid attached schema-v2 project-local STeamAI bundle")
	}
	if !strings.EqualFold(strings.TrimSpace(inst.TemplatePack), strings.TrimSpace(pack)) {
		return CurrentSyncPlan{}, fmt.Errorf("current sync pack differs from attached project metadata: %s", inst.TemplatePack)
	}
	if !casebind.SamePath(inst.TemplateRoot, stateRoot.Path) || !casebind.SamePath(inst.BundleRoot, filepath.Join(stateRoot.Path, "runtime")) {
		return CurrentSyncPlan{}, fmt.Errorf("current sync project-local bundle paths do not resolve to the target .steamai root")
	}
	if strings.TrimSpace(inst.BundleManifestSHA256) == "" {
		return CurrentSyncPlan{}, fmt.Errorf("current sync target metadata omits the active bundle manifest SHA-256")
	}
	currentManifest, err := runtimebundle.Validate(stateRoot.Path, inst.BundleManifest, inst.BundleManifestSHA256, pack)
	if err != nil {
		return CurrentSyncPlan{}, fmt.Errorf("current sync target bundle is not a valid refresh base: %w", err)
	}
	currentExecutable := runtimebundle.ExecutablePath(stateRoot.Path, currentManifest)
	sameExecutable, sameExecutableErr := rekitfs.SameExistingPath(sourceExecutable, currentExecutable)
	if sameExecutableErr != nil {
		return CurrentSyncPlan{}, fmt.Errorf("current sync compare source and target runtime executable identity: %w", sameExecutableErr)
	}
	if sameExecutable {
		return CurrentSyncPlan{}, fmt.Errorf("current sync source executable must be physically distinct from the target runtime: %s", sourceExecutable)
	}
	if err := currentSyncValidateMaintenanceProcessExecutable(sourceExecutable); err != nil {
		return CurrentSyncPlan{}, err
	}

	m, err := manifest.Load(repoFull, pack)
	if err != nil {
		return CurrentSyncPlan{}, err
	}
	if err := m.ValidateSchema(); err != nil {
		return CurrentSyncPlan{}, err
	}
	bundle, err := runtimebundle.BuildWithExecutable(repoFull, pack, sourceExecutable)
	if err != nil {
		return CurrentSyncPlan{}, err
	}
	publications, nextBindings, err := currentSyncBundlePublications(bundle)
	if err != nil {
		return CurrentSyncPlan{}, err
	}
	currentBindings, err := currentSyncManifestBindings(stateRoot.Path, inst.BundleManifest, currentManifest)
	if err != nil {
		return CurrentSyncPlan{}, err
	}
	currentControlled, err := currentSyncInventory(currentBindings)
	if err != nil {
		return CurrentSyncPlan{}, err
	}
	nextControlled, err := currentSyncInventory(nextBindings)
	if err != nil {
		return CurrentSyncPlan{}, err
	}
	currentManifestBytes, err := currentSyncReadFile(stateRoot.Path, filepath.Join(stateRoot.Path, filepath.FromSlash(inst.BundleManifest)), "current sync active manifest", 1<<20, false)
	if err != nil {
		return CurrentSyncPlan{}, err
	}
	currentManifestBinding := currentSyncBinding(filepath.ToSlash(filepath.Join(projectstate.CurrentDir, inst.BundleManifest)), "runtime-bundle-manifest", currentManifestBytes, 0o644)
	nextManifestBinding := currentSyncBinding(filepath.ToSlash(filepath.Join(projectstate.CurrentDir, runtimebundle.ManifestRel)), "runtime-bundle-manifest", bundle.ManifestData, 0o644)

	projectName := strings.TrimSpace(opt.ProjectName)
	if projectName == "" {
		projectName = strings.TrimSpace(inst.ProjectName)
	}
	if projectName == "" {
		projectName = casebind.ProjectNameFromRoot(caseFull)
	}
	leaves, writes, err := currentSyncPlanLeaves(repoFull, caseFull, stateRoot.Path, m, pack, projectName, bundle.ManifestSHA256)
	if err != nil {
		return CurrentSyncPlan{}, err
	}
	controlledWrites, obsolete := currentSyncControlledWrites(currentBindings, nextBindings, publications)
	writes = append(controlledWrites, writes...)
	sort.Slice(writes, func(i, j int) bool {
		if writes[i].Path != writes[j].Path {
			return writes[i].Path < writes[j].Path
		}
		return writes[i].Kind < writes[j].Kind
	})

	currentTargets, nextTargets, err := currentSyncTargetInventories(leaves)
	if err != nil {
		return CurrentSyncPlan{}, err
	}
	sourceExecutableBytes, sourceExecutableIdentity, err := currentSyncReadFileIdentity(filepath.Dir(sourceExecutable), sourceExecutable, "current sync source executable", currentSyncMaxFileBytes, false)
	if err != nil {
		return CurrentSyncPlan{}, err
	}
	sourceExecutableBinding := currentSyncBinding(filepath.ToSlash(sourceExecutable), "runtime-executable-source", sourceExecutableBytes, currentSyncSourceMode(sourceExecutable))
	sourceExecutableBinding.Identity = &sourceExecutableIdentity
	previousReceipt, previousReceiptOK, err := currentSyncReadOptional(caseFull, filepath.Join(stateRoot.Path, filepath.FromSlash(currentSyncReceiptRel)), "current sync previous receipt", 4<<20)
	if err != nil {
		return CurrentSyncPlan{}, err
	}
	var previousReceiptBinding *CurrentSyncBinding
	if previousReceiptOK {
		binding := currentSyncBinding(filepath.ToSlash(filepath.Join(projectstate.CurrentDir, currentSyncReceiptRel)), "current-sync-receipt", previousReceipt, 0o600)
		previousReceiptBinding = &binding
	}

	command := strings.TrimSpace(opt.Command)
	if command == "" {
		command = "sync"
	}
	plan := CurrentSyncPlan{
		SchemaVersion: currentSyncSchemaVersion, Kind: currentSyncPlanKind, Command: command,
		Status: "ready-to-refresh", CaseRoot: caseFull, SourceRepoRoot: repoFull,
		SourceExecutable: sourceExecutable, Pack: pack, ProjectName: projectName,
		ForceLocalTemplates: false, CaseRootIdentity: caseIdentity,
		StateRootIdentity: stateIdentity, SourceRootIdentity: sourceIdentity,
		SourceExecutableFile: sourceExecutableBinding, CurrentManifest: currentManifestBinding,
		NextManifest: nextManifestBinding, CurrentControlled: currentControlled,
		NextControlled: nextControlled, CurrentTargets: currentTargets, NextTargets: nextTargets,
		PreviousReceipt: previousReceiptBinding, Writes: writes, ObsoleteControlled: obsolete,
		RequiresReview: true, RequiresConfirmation: true,
		BlockedActions: []string{
			"project-local runtime self-update", "legacy .rekit mutation", "forced local-template replacement", "lane/facts/evidence/reviews/handoffs mutation",
			"authority/confirmed writes", "gate or autonomy expansion", "promote", "heavy-tool execution",
		},
		Boundary: []string{
			"the source repository and executable are external maintenance inputs bound by exact filesystem identity and SHA-256",
			"the active runtime, packs, common assets, runtime assets, schema-v2 instance, sync state, and project-local /steamai skill are one reviewed refresh",
			"lanes, facts, evidence, reviews, handoffs, autonomy profiles, and all other mutable project state are outside the write set",
			"Apply requires the exact preview SHA-256, rechecks every source and target under a mutation lease, and activates instance.yml last",
		},
		NextSteps: []string{
			"review the complete controlled inventory, managed output transitions, obsolete files, and external source executable",
			"run only the returned exact ApplyArgs after confirming this current project refresh",
		},
		prepared: &currentSyncPrepared{
			caseInfo: caseInfo, stateInfo: stateInfo, publications: publications, leaves: leaves,
			currentRootFiles: currentSyncBindingsByRoot(currentBindings), nextRootFiles: currentSyncBindingsByRoot(nextBindings),
			currentManifest: currentManifest, nextManifest: bundle.Manifest,
			previousReceipt: previousReceipt, previousReceiptOK: previousReceiptOK,
		},
	}
	plan.AlreadyCurrent = currentControlled.SHA256 == nextControlled.SHA256 && currentTargets.SHA256 == nextTargets.SHA256 && len(obsolete) == 0
	if plan.AlreadyCurrent {
		plan.Status = "already-current"
		plan.RequiresConfirmation = false
		plan.NextSteps = []string{"no current project refresh is required"}
	}
	identity := currentSyncIdentity(plan)
	plan.ExpectedPlanSHA256, err = currentSyncCanonicalSHA(identity)
	if err != nil {
		return CurrentSyncPlan{}, err
	}
	if !plan.AlreadyCurrent {
		plan.ApplyArgs = []string{
			"-Command", command,
			"-Target", caseFull,
			"-Pack", pack,
			"-ProjectName", projectName,
			"-SourceRepoRoot", repoFull,
			"-SourceExecutable", sourceExecutable,
			"-ExpectedCurrentSyncPlanSha256", plan.ExpectedPlanSHA256,
			"-Apply",
			"-Format", "json",
		}
	}
	return plan, nil
}

func currentSyncRoots(caseRoot string) (string, projectstate.Root, error) {
	caseFull, err := filepath.Abs(strings.TrimSpace(caseRoot))
	if err != nil {
		return "", projectstate.Root{}, err
	}
	caseFull = filepath.Clean(caseFull)
	if _, err := rekitfs.ValidateNonReparseDirectory(caseFull, "current sync project root"); err != nil {
		return "", projectstate.Root{}, err
	}
	root, err := projectstate.Resolve(caseFull)
	if err != nil {
		return "", projectstate.Root{}, err
	}
	if root.Legacy || !root.Existing || root.Dir != projectstate.CurrentDir {
		return "", projectstate.Root{}, fmt.Errorf("current sync requires an existing .steamai-only project")
	}
	return caseFull, root, nil
}

func currentSyncBundlePublications(bundle runtimebundle.Plan) ([]currentSyncPublication, []CurrentSyncBinding, error) {
	artifacts := map[string]runtimebundle.Artifact{bundle.Manifest.Executable.Path: bundle.Manifest.Executable}
	for _, artifact := range bundle.Manifest.Files {
		artifacts[artifact.Path] = artifact
	}
	publications := make([]currentSyncPublication, 0, len(bundle.Publications))
	bindings := make([]CurrentSyncBinding, 0, len(bundle.Publications))
	for _, publication := range bundle.Publications {
		data := append([]byte(nil), publication.Content...)
		if publication.SourcePath != "" {
			artifact, ok := artifacts[publication.Path]
			if !ok {
				return nil, nil, fmt.Errorf("current sync bundle publication lacks a manifest artifact: %s", publication.Path)
			}
			var err error
			data, err = currentSyncReadFile(filepath.Dir(publication.SourcePath), publication.SourcePath, "current sync bundle source", artifact.Size+1, false)
			if err != nil {
				return nil, nil, err
			}
			if int64(len(data)) != artifact.Size || currentSyncSHA(data) != strings.ToLower(artifact.SHA256) {
				return nil, nil, fmt.Errorf("current sync bundle source changed while planning: %s", publication.SourcePath)
			}
		}
		mode := os.FileMode(0o644)
		if publication.Kind == "runtime-executable" {
			mode = 0o755
		}
		rel := filepath.ToSlash(filepath.Clean(filepath.FromSlash(publication.Path)))
		publications = append(publications, currentSyncPublication{rel: rel, kind: publication.Kind, source: publication.SourcePath, data: data, mode: mode})
		bindings = append(bindings, currentSyncBinding(filepath.ToSlash(filepath.Join(projectstate.CurrentDir, rel)), publication.Kind, data, mode))
	}
	sort.Slice(publications, func(i, j int) bool { return publications[i].rel < publications[j].rel })
	sort.Slice(bindings, func(i, j int) bool { return bindings[i].Path < bindings[j].Path })
	return publications, bindings, nil
}

func currentSyncManifestBindings(assetRoot, manifestRel string, value runtimebundle.Manifest) ([]CurrentSyncBinding, error) {
	artifacts := []runtimebundle.Artifact{{Path: manifestRel, Kind: "runtime-bundle-manifest"}, value.Executable}
	artifacts = append(artifacts, value.Files...)
	bindings := make([]CurrentSyncBinding, 0, len(artifacts))
	for _, artifact := range artifacts {
		path := filepath.Join(assetRoot, filepath.FromSlash(artifact.Path))
		limit := artifact.Size + 1
		if artifact.Path == manifestRel {
			limit = 1 << 20
		}
		data, err := currentSyncReadFile(assetRoot, path, "current sync controlled target", limit, false)
		if err != nil {
			return nil, err
		}
		kind := artifact.Kind
		if artifact.Path == manifestRel {
			kind = "runtime-bundle-manifest"
		}
		mode := os.FileMode(0o644)
		if kind == "runtime-executable" {
			mode = 0o755
		}
		bindings = append(bindings, currentSyncBinding(filepath.ToSlash(filepath.Join(projectstate.CurrentDir, artifact.Path)), kind, data, mode))
	}
	sort.Slice(bindings, func(i, j int) bool { return bindings[i].Path < bindings[j].Path })
	return bindings, nil
}

type currentSyncTargetProtection struct {
	local     bool
	authority bool
	handoff   bool
}

func (protection currentSyncTargetProtection) any() bool {
	return protection.local || protection.authority || protection.handoff
}

func (protection currentSyncTargetProtection) String() string {
	labels := []string{}
	if protection.local {
		labels = append(labels, "localNeverOverwrite")
	}
	if protection.authority {
		labels = append(labels, "authorityFiles")
	}
	if protection.handoff {
		labels = append(labels, "workstream handoff")
	}
	return strings.Join(labels, ", ")
}

func currentSyncPlanTargetRel(rel, kind string) (string, error) {
	clean, err := currentSyncNormalizeTargetRel(rel)
	if err != nil || !currentSyncSafeTargetRel(clean) {
		return "", fmt.Errorf("current sync %s target is outside the allowed refresh leaves: %s", kind, rel)
	}
	return clean, nil
}

func currentSyncProtectedTargets(m *manifest.Manifest) (map[string]currentSyncTargetProtection, error) {
	result := map[string]currentSyncTargetProtection{}
	add := func(rel, kind string) error {
		clean, err := currentSyncPlanTargetRel(rel, kind)
		if err != nil {
			return err
		}
		key := strings.ToLower(clean)
		protection := result[key]
		switch kind {
		case "localNeverOverwrite":
			protection.local = true
		case "authorityFiles":
			protection.authority = true
		case "workstream handoff":
			protection.handoff = true
		}
		result[key] = protection
		return nil
	}
	for _, rel := range m.LocalFiles {
		if err := add(rel, "localNeverOverwrite"); err != nil {
			return nil, err
		}
	}
	for _, rel := range m.AuthorityFiles {
		if err := add(rel, "authorityFiles"); err != nil {
			return nil, err
		}
	}
	if rel := strings.TrimSpace(m.WorkstreamDefaults["handoffPath"]); rel != "" {
		if err := add(rel, "workstream handoff"); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func currentSyncPlanLeaves(repoRoot, caseRoot, stateRoot string, m *manifest.Manifest, pack, projectName, manifestSHA string) ([]currentSyncLeaf, []CurrentSyncWrite, error) {
	leaves := []currentSyncLeaf{}
	seen := map[string]string{}
	add := func(rel, kind, action, source string, before []byte, beforeExists bool, after []byte, afterExists bool, mode os.FileMode) error {
		clean, err := currentSyncNormalizeTargetRel(rel)
		if err != nil || !currentSyncSafeTargetRel(clean) {
			return fmt.Errorf("current sync %s target is outside the allowed refresh leaves: %s", kind, rel)
		}
		key := strings.ToLower(clean)
		if previous, ok := seen[key]; ok {
			return fmt.Errorf("current sync target is declared more than once: %s (%s and %s)", clean, previous, kind)
		}
		for other, otherKind := range seen {
			if strings.HasPrefix(key, other+"/") || strings.HasPrefix(other, key+"/") {
				return fmt.Errorf("current sync targets overlap: %s (%s) and %s (%s)", clean, kind, other, otherKind)
			}
		}
		if len(leaves) >= currentSyncMaxFiles {
			return fmt.Errorf("current sync target inventory exceeds %d files", currentSyncMaxFiles)
		}
		seen[key] = kind
		leaf := currentSyncLeaf{rel: clean, kind: kind, action: action, source: source, before: append([]byte(nil), before...), beforeExists: beforeExists, after: append([]byte(nil), after...), afterExists: afterExists, mode: mode}
		leaves = append(leaves, leaf)
		return nil
	}
	protected, err := currentSyncProtectedTargets(m)
	if err != nil {
		return nil, nil, err
	}
	for _, rel := range m.ManagedFiles {
		targetRel, err := currentSyncPlanTargetRel(rel, "managed-file")
		if err != nil {
			return nil, nil, err
		}
		if protection := protected[strings.ToLower(targetRel)]; protection.any() {
			return nil, nil, fmt.Errorf("current sync managed-file target is protected as %s: %s", protection, targetRel)
		}
		source, err := m.SourcePath(rel)
		if err != nil {
			return nil, nil, err
		}
		after, err := sourceartifact.ReadCanonical(source)
		if err != nil {
			return nil, nil, err
		}
		before, exists, err := currentSyncReadOptional(caseRoot, filepath.Join(caseRoot, filepath.FromSlash(targetRel)), "current sync managed target", currentSyncMaxFileBytes)
		if err != nil {
			return nil, nil, err
		}
		action := "replace-managed-file"
		if !exists {
			action = "create-managed-file"
		} else if bytes.Equal(before, after) {
			action = "unchanged"
		}
		if err := add(targetRel, "managed-file", action, source, before, exists, after, true, 0o644); err != nil {
			return nil, nil, err
		}
	}
	for _, rel := range m.TemplateFiles {
		source, err := m.SourcePath(rel)
		if err != nil {
			return nil, nil, err
		}
		data, err := sourceartifact.ReadCanonical(source)
		if err != nil {
			return nil, nil, err
		}
		targetRel, err := currentSyncPlanTargetRel(strings.TrimSuffix(rel, ".template.md")+".md", "template-file")
		if err != nil {
			return nil, nil, err
		}
		after := []byte(strings.ReplaceAll(strings.ReplaceAll(string(data), "<PROJECT_NAME>", projectName), "<PROJECT_ROOT>", caseRoot))
		before, exists, err := currentSyncReadOptional(caseRoot, filepath.Join(caseRoot, filepath.FromSlash(targetRel)), "current sync local template target", currentSyncMaxFileBytes)
		if err != nil {
			return nil, nil, err
		}
		action := "create-local-template-file"
		if exists {
			action, after = "skip-existing-local-file", before
		}
		if protection := protected[strings.ToLower(targetRel)]; protection.any() && !exists {
			action = "create-protected-local-file"
		}
		if err := add(targetRel, "template-file", action, source, before, exists, after, true, 0o644); err != nil {
			return nil, nil, err
		}
	}
	blockSource, err := m.SourcePath(m.ManagedBlock["source"])
	if err != nil {
		return nil, nil, err
	}
	blockData, err := sourceartifact.ReadCanonical(blockSource)
	if err != nil {
		return nil, nil, err
	}
	blockRel, err := currentSyncPlanTargetRel(m.ManagedBlock["file"], "managed-block")
	if err != nil {
		return nil, nil, err
	}
	if protection := protected[strings.ToLower(blockRel)]; protection.authority || protection.handoff {
		return nil, nil, fmt.Errorf("current sync managed-block target is protected as %s: %s", protection, blockRel)
	}
	blockBefore, blockExists, err := currentSyncReadOptional(caseRoot, filepath.Join(caseRoot, filepath.FromSlash(blockRel)), "current sync managed block host", currentSyncMaxFileBytes)
	if err != nil {
		return nil, nil, err
	}
	blockAfter := []byte(review.ApplyManagedBlock(string(blockBefore), m.ManagedBlock["blockId"], string(blockData)))
	blockAction := managedBlockAction(string(blockBefore), string(blockAfter), m.ManagedBlock["blockId"])
	if err := add(blockRel, "managed-block", blockAction, blockSource, blockBefore, blockExists, blockAfter, true, 0o644); err != nil {
		return nil, nil, err
	}

	gitignoreSource, gitignoreErr := m.SourcePath("examples/gitignore.example")
	if gitignoreErr == nil {
		gitignoreRaw, exists, err := currentSyncReadOptional(m.PackRoot, gitignoreSource, "current sync optional gitignore source", currentSyncMaxFileBytes)
		if err != nil {
			return nil, nil, err
		}
		if !exists {
			gitignoreSource = ""
		}
		gitignoreData := sourceartifact.CanonicalText(gitignoreRaw)
		if gitignoreSource != "" {
			before, exists, err := currentSyncReadOptional(caseRoot, filepath.Join(caseRoot, ".gitignore"), "current sync support target", currentSyncMaxFileBytes)
			if err != nil {
				return nil, nil, err
			}
			action, after := "create-support-file", gitignoreData
			if exists {
				action, after = "skip-existing-support-file", before
			}
			if err := add(".gitignore", "support-file", action, gitignoreSource, before, exists, after, true, 0o644); err != nil {
				return nil, nil, err
			}
		}
	}

	skillSource := filepath.Join(repoRoot, "rekit", "templates", "steamai-project", "SKILL.md")
	skillAfter, err := sourceartifact.ReadCanonical(skillSource)
	if err != nil {
		return nil, nil, err
	}
	skillRel := filepath.ToSlash(filepath.Join(".claude", "skills", "steamai", "SKILL.md"))
	skillBefore, skillExists, err := currentSyncReadOptional(caseRoot, filepath.Join(caseRoot, filepath.FromSlash(skillRel)), "current sync project skill", 1<<20)
	if err != nil {
		return nil, nil, err
	}
	skillAction := "replace-project-skill"
	if !skillExists {
		skillAction = "create-project-skill"
	} else if bytes.Equal(skillBefore, skillAfter) {
		skillAction = "unchanged"
	}
	if err := add(skillRel, "project-local-steamai-skill", skillAction, skillSource, skillBefore, skillExists, skillAfter, true, 0o644); err != nil {
		return nil, nil, err
	}

	stateRel := filepath.ToSlash(filepath.Join(projectstate.CurrentDir, "state.json"))
	statePath := filepath.Join(stateRoot, "state.json")
	stateBefore, stateExists, err := currentSyncReadOptional(caseRoot, statePath, "current sync state metadata", 16<<20)
	if err != nil || !stateExists {
		return nil, nil, errors.Join(err, fmt.Errorf("current sync requires existing .steamai/state.json"))
	}
	managed := map[string]syncManagedEntry{}
	for _, leaf := range leaves {
		if leaf.kind != "managed-file" {
			continue
		}
		managed[leaf.rel] = syncManagedEntry{SourceHash: currentSyncSHA(leaf.after), TargetHashAtSync: currentSyncSHA(leaf.after), LastAction: "sync"}
	}
	stateAfter, err := currentSyncStateBytes(stateBefore, pack, managed)
	if err != nil {
		return nil, nil, err
	}
	stateAction := "replace-sync-state"
	if bytes.Equal(stateBefore, stateAfter) {
		stateAction = "unchanged"
	}
	if err := add(stateRel, "sync-state", stateAction, "", stateBefore, true, stateAfter, true, 0o644); err != nil {
		return nil, nil, err
	}

	instanceRel := filepath.ToSlash(filepath.Join(projectstate.CurrentDir, "instance.yml"))
	instancePath := filepath.Join(stateRoot, "instance.yml")
	instanceBefore, instanceExists, err := currentSyncReadOptional(caseRoot, instancePath, "current sync instance metadata", 1<<20)
	if err != nil || !instanceExists {
		return nil, nil, errors.Join(err, fmt.Errorf("current sync requires existing .steamai/instance.yml"))
	}
	instanceAfter := []byte(casebind.STeamAIInstanceText(caseRoot, pack, projectName, runtimebundle.ManifestRel, manifestSHA))
	instanceAction := "activate-instance-last"
	if bytes.Equal(instanceBefore, instanceAfter) {
		instanceAction = "unchanged"
	}
	if err := add(instanceRel, "instance-metadata", instanceAction, "", instanceBefore, true, instanceAfter, true, 0o644); err != nil {
		return nil, nil, err
	}

	writes := make([]CurrentSyncWrite, 0, len(leaves))
	for _, leaf := range leaves {
		writes = append(writes, currentSyncWriteForLeaf(leaf))
	}
	return leaves, writes, nil
}

func currentSyncStateBytes(before []byte, pack string, managed map[string]syncManagedEntry) ([]byte, error) {
	var state map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(before))
	if err := decoder.Decode(&state); err != nil {
		return nil, fmt.Errorf("current sync state.json is invalid: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("current sync state.json has trailing data")
	}
	var version int
	if err := json.Unmarshal(state["schemaVersion"], &version); err != nil || version != 1 {
		return nil, fmt.Errorf("current sync state.json schemaVersion must be 1")
	}
	var statePack string
	if err := json.Unmarshal(state["templatePack"], &statePack); err != nil || !strings.EqualFold(strings.TrimSpace(statePack), strings.TrimSpace(pack)) {
		return nil, fmt.Errorf("current sync state.json templatePack does not match attached metadata")
	}
	previous := map[string]syncManagedEntry{}
	if raw := state["managed"]; len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &previous); err != nil {
			return nil, fmt.Errorf("current sync state.json managed map is invalid: %w", err)
		}
	}
	merged := map[string]syncManagedEntry{}
	for path, entry := range previous {
		if entry.LastAction != "sync" {
			merged[path] = entry
		}
	}
	for path, entry := range managed {
		merged[path] = entry
	}
	managedBytes, err := json.Marshal(merged)
	if err != nil {
		return nil, err
	}
	state["schemaVersion"] = json.RawMessage("1")
	state["templateRoot"] = json.RawMessage(strconv.Quote("."))
	state["templatePack"] = json.RawMessage(strconv.Quote(pack))
	state["managed"] = managedBytes
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func currentSyncControlledWrites(current, next []CurrentSyncBinding, publications []currentSyncPublication) ([]CurrentSyncWrite, []string) {
	before := map[string]CurrentSyncBinding{}
	after := map[string]CurrentSyncBinding{}
	sources := map[string]string{}
	for _, binding := range current {
		before[binding.Path] = binding
	}
	for _, binding := range next {
		after[binding.Path] = binding
	}
	for _, publication := range publications {
		sources[filepath.ToSlash(filepath.Join(projectstate.CurrentDir, publication.rel))] = publication.source
	}
	paths := make([]string, 0, len(before)+len(after))
	seen := map[string]bool{}
	for path := range before {
		seen[path] = true
		paths = append(paths, path)
	}
	for path := range after {
		if !seen[path] {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	writes := make([]CurrentSyncWrite, 0, len(paths))
	obsolete := []string{}
	for _, path := range paths {
		left, leftOK := before[path]
		right, rightOK := after[path]
		action := "replace-controlled-tree-file"
		kind := right.Kind
		if !leftOK {
			action = "create-controlled-tree-file"
		} else if !rightOK {
			action, kind = "remove-obsolete-controlled-tree-file", left.Kind
			obsolete = append(obsolete, path)
		} else if left.SHA256 == right.SHA256 && left.Size == right.Size {
			action = "unchanged"
		}
		writes = append(writes, CurrentSyncWrite{
			Path: path, Kind: kind, Action: action, SourcePath: sources[path],
			BeforeExists: leftOK, BeforeSHA256: left.SHA256, BeforeSize: left.Size,
			AfterExists: rightOK, AfterSHA256: right.SHA256, AfterSize: right.Size,
		})
	}
	return writes, obsolete
}

func currentSyncTargetInventories(leaves []currentSyncLeaf) (CurrentSyncInventory, CurrentSyncInventory, error) {
	before := []CurrentSyncBinding{}
	after := []CurrentSyncBinding{}
	for _, leaf := range leaves {
		if leaf.beforeExists {
			before = append(before, currentSyncBinding(leaf.rel, leaf.kind, leaf.before, leaf.mode))
		}
		if leaf.afterExists {
			after = append(after, currentSyncBinding(leaf.rel, leaf.kind, leaf.after, leaf.mode))
		}
	}
	left, err := currentSyncInventory(before)
	if err != nil {
		return CurrentSyncInventory{}, CurrentSyncInventory{}, err
	}
	right, err := currentSyncInventory(after)
	return left, right, err
}

func currentSyncWriteForLeaf(leaf currentSyncLeaf) CurrentSyncWrite {
	write := CurrentSyncWrite{
		Path: leaf.rel, Kind: leaf.kind, Action: leaf.action, SourcePath: leaf.source,
		BeforeExists: leaf.beforeExists, AfterExists: leaf.afterExists,
	}
	if leaf.beforeExists {
		write.BeforeSHA256, write.BeforeSize = currentSyncSHA(leaf.before), int64(len(leaf.before))
	}
	if leaf.afterExists {
		write.AfterSHA256, write.AfterSize = currentSyncSHA(leaf.after), int64(len(leaf.after))
	}
	return write
}

func currentSyncInventory(entries []CurrentSyncBinding) (CurrentSyncInventory, error) {
	entries = append([]CurrentSyncBinding(nil), entries...)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Path != entries[j].Path {
			return entries[i].Path < entries[j].Path
		}
		return entries[i].Kind < entries[j].Kind
	})
	var total int64
	for index, entry := range entries {
		if entry.Path == "" || entry.Kind == "" || entry.Size < 0 || entry.SHA256 == "" {
			return CurrentSyncInventory{}, fmt.Errorf("current sync inventory entry is invalid: %s", entry.Path)
		}
		if index > 0 && strings.EqualFold(entries[index-1].Path, entry.Path) {
			return CurrentSyncInventory{}, fmt.Errorf("current sync inventory contains duplicate path: %s", entry.Path)
		}
		total += entry.Size
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return CurrentSyncInventory{}, err
	}
	return CurrentSyncInventory{SHA256: currentSyncSHA(data), Files: len(entries), Bytes: total, Entries: entries}, nil
}

func currentSyncBindingsByRoot(bindings []CurrentSyncBinding) map[string][]CurrentSyncBinding {
	result := map[string][]CurrentSyncBinding{}
	prefix := projectstate.CurrentDir + "/"
	for _, binding := range bindings {
		rel := strings.TrimPrefix(filepath.ToSlash(binding.Path), prefix)
		root := strings.SplitN(rel, "/", 2)[0]
		result[root] = append(result[root], binding)
	}
	for root := range result {
		sort.Slice(result[root], func(i, j int) bool { return result[root][i].Path < result[root][j].Path })
	}
	return result
}

func currentSyncIdentity(plan CurrentSyncPlan) currentSyncPlanIdentity {
	return currentSyncPlanIdentity{
		SchemaVersion: plan.SchemaVersion, Kind: plan.Kind, Command: plan.Command,
		CaseRoot: plan.CaseRoot, SourceRepoRoot: plan.SourceRepoRoot, SourceExecutable: plan.SourceExecutable,
		Pack: plan.Pack, ProjectName: plan.ProjectName, ForceLocalTemplates: plan.ForceLocalTemplates,
		CaseRootIdentity: plan.CaseRootIdentity, StateRootIdentity: plan.StateRootIdentity,
		SourceRootIdentity: plan.SourceRootIdentity, SourceExecutableFile: plan.SourceExecutableFile,
		CurrentManifest: plan.CurrentManifest, NextManifest: plan.NextManifest,
		CurrentControlled: plan.CurrentControlled, NextControlled: plan.NextControlled,
		CurrentTargets: plan.CurrentTargets, NextTargets: plan.NextTargets,
		PreviousReceipt: plan.PreviousReceipt, Writes: plan.Writes, ObsoleteControlled: plan.ObsoleteControlled,
	}
}

func currentSyncBinding(path, kind string, data []byte, mode os.FileMode) CurrentSyncBinding {
	return CurrentSyncBinding{Path: filepath.ToSlash(path), Kind: kind, SHA256: currentSyncSHA(data), Size: int64(len(data)), Mode: uint32(mode.Perm())}
}

func currentSyncReadOptional(anchor, path, label string, limit int64) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > limit {
		return nil, false, fmt.Errorf("%s must be a bounded regular non-symlink file: %s", label, path)
	}
	data, err := currentSyncReadFile(anchor, path, label, limit, true)
	return data, err == nil, err
}

func currentSyncReadFile(anchor, path, label string, limit int64, allowEmpty bool) ([]byte, error) {
	data, _, err := currentSyncReadFileIdentity(anchor, path, label, limit, allowEmpty)
	return data, err
}

func currentSyncReadFileIdentity(anchor, path, label string, limit int64, allowEmpty bool) ([]byte, statemigration.Identity, error) {
	anchor, err := filepath.Abs(anchor)
	if err != nil {
		return nil, statemigration.Identity{}, err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, statemigration.Identity{}, err
	}
	rel, err := filepath.Rel(anchor, path)
	if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, statemigration.Identity{}, fmt.Errorf("%s path escapes its anchored root: %s", label, path)
	}
	if err := rekitfs.ValidateNoReparseComponents(path); err != nil {
		return nil, statemigration.Identity{}, err
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, statemigration.Identity{}, err
	}
	minimum := int64(1)
	if allowEmpty {
		minimum = 0
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() || before.Size() < minimum || before.Size() > limit {
		return nil, statemigration.Identity{}, fmt.Errorf("%s must be a bounded regular non-symlink file: %s", label, path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, statemigration.Identity{}, err
	}
	identity, identityErr := statemigration.IdentityForFile(file)
	opened, statErr := file.Stat()
	data, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	closeErr := file.Close()
	after, afterErr := os.Lstat(path)
	if identityErr != nil || statErr != nil || readErr != nil || closeErr != nil || afterErr != nil || !os.SameFile(before, opened) || !os.SameFile(opened, after) || int64(len(data)) != opened.Size() || int64(len(data)) > limit {
		return nil, statemigration.Identity{}, fmt.Errorf("%s changed while reading: %s: %w", label, path, errors.Join(identityErr, statErr, readErr, closeErr, afterErr))
	}
	return data, identity, nil
}

func currentSyncSourceMode(path string) os.FileMode {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Mode().Perm()
}

func currentSyncValidateMaintenanceProcessExecutable(sourceExecutable string) error {
	runningExecutable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("current sync resolve central maintenance process executable: %w", err)
	}
	if err := rekitfs.ValidateNoReparseComponents(runningExecutable); err != nil {
		return fmt.Errorf("current sync central maintenance process executable is invalid: %w", err)
	}
	if _, projectLocal, layoutErr := runtimebundle.AssetRootForExecutable(runningExecutable); layoutErr != nil {
		return fmt.Errorf("current sync maintenance process layout is invalid: %w", layoutErr)
	} else if projectLocal {
		return fmt.Errorf("current sync maintenance process must be external to every project-local runtime bundle")
	}
	same, err := rekitfs.SameExistingPath(sourceExecutable, runningExecutable)
	if err != nil {
		return fmt.Errorf("current sync compare source executable with central maintenance process image: %w", err)
	}
	if !same {
		return fmt.Errorf("current sync source executable must be the running central maintenance process image: %s", sourceExecutable)
	}
	return nil
}

func currentSyncValidateDurableMaintenanceProcess(plan CurrentSyncPlan) error {
	if err := validateCurrentSyncPlanForIntent(plan); err != nil {
		return err
	}
	if err := currentSyncValidateMaintenanceProcessExecutable(plan.SourceExecutable); err != nil {
		return err
	}
	runningExecutable, err := os.Executable()
	if err != nil {
		return err
	}
	data, identity, err := currentSyncReadFileIdentity(
		filepath.Dir(runningExecutable),
		runningExecutable,
		"current sync running maintenance executable",
		currentSyncMaxFileBytes,
		false,
	)
	if err != nil {
		return err
	}
	expected := plan.SourceExecutableFile
	info, err := os.Lstat(runningExecutable)
	if err != nil {
		return err
	}
	if expected.Identity == nil || identity != *expected.Identity ||
		expected.Size != int64(len(data)) ||
		!strings.EqualFold(expected.SHA256, currentSyncSHA(data)) ||
		!currentSyncModeMatches(os.FileMode(expected.Mode), info.Mode()) {
		return fmt.Errorf("current sync running maintenance executable differs from the durable reviewed source identity")
	}
	return nil
}

func currentSyncPathInside(path, root string) bool {
	path, pathErr := filepath.Abs(path)
	root, rootErr := filepath.Abs(root)
	if pathErr != nil || rootErr != nil {
		return false
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func currentSyncPathsOverlap(left, right string) bool {
	return currentSyncPathInside(left, right) || currentSyncPathInside(right, left)
}

func currentSyncCanonicalSHA(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return currentSyncSHA(data), nil
}

func currentSyncSHA(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validCurrentSyncSHA(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
