package statemigration

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	"github.com/shuiyu486/re-context-kits/internal/rekit/packidentity"
)

const projectedOnboardingLineageKind = "retired-onboarding-v2-projection-v1"

type plannedOnboardingFile struct {
	rel          string
	kind         string
	phase        int
	before       []byte
	beforeMode   os.FileMode
	beforeExists bool
	after        []byte
	afterMode    os.FileMode
}

type plannedOnboarding struct {
	projection OnboardingProjection
	files      []plannedOnboardingFile
}

type onboardingLineage struct {
	Kind                       string                  `json:"kind"`
	SourcePack                 string                  `json:"sourcePack"`
	TargetPack                 string                  `json:"targetPack"`
	SourceOnboardingPlanSHA256 string                  `json:"sourceOnboardingPlanSha256"`
	PublicationStamp           string                  `json:"publicationStamp"`
	Before                     []OnboardingFileBinding `json:"before"`
}

func planRetiredOnboarding(caseRoot, sourcePack, targetPack string, contents map[string][]byte, currentInstance, currentState, currentSkill FileBinding) (*plannedOnboarding, error) {
	if !packidentity.IsRetired(sourcePack) {
		return nil, nil
	}
	inspection, err := missionintent.Inspect(caseRoot)
	if err != nil {
		return nil, fmt.Errorf("retired onboarding state is corrupt: %w", err)
	}
	switch inspection.State {
	case "absent":
		return nil, nil
	case "pending":
		return nil, fmt.Errorf("retired onboarding publication is pending; finish its exact original Apply before migrate-state")
	case "committed":
	default:
		return nil, fmt.Errorf("retired onboarding state is not migratable: %s", inspection.State)
	}
	if !inspection.Committed || inspection.Identity.SchemaVersion != 1 || !strings.EqualFold(inspection.Identity.Pack, sourcePack) || !sameMigrationPath(inspection.Identity.Target, caseRoot) {
		return nil, fmt.Errorf("retired onboarding committed identity does not match the selected migration source")
	}
	if inspection.Identity.ProjectName == "" || inspection.PublicationStamp == "" || !validSHA256(inspection.OnboardingPlanSHA256) || !validSHA256(inspection.CommitSHA256) {
		return nil, fmt.Errorf("retired onboarding committed identity is incomplete")
	}

	beforePaths := []string{"mission-intent.json", "onboarding/intent.json", "onboarding/commit.json"}
	before := make([]OnboardingFileBinding, 0, len(beforePaths))
	beforeData := make(map[string][]byte, len(beforePaths))
	beforeModes := make(map[string]os.FileMode, len(beforePaths))
	for _, rel := range beforePaths {
		data, ok := contents[rel]
		if !ok {
			return nil, fmt.Errorf("retired onboarding committed generation is missing: .rekit/%s", rel)
		}
		info, err := os.Lstat(filepath.Join(caseRoot, ".rekit", filepath.FromSlash(rel)))
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("retired onboarding artifact is not a regular file: .rekit/%s", rel)
		}
		fileBinding := bindingFor(".rekit/"+rel, data)
		binding := OnboardingFileBinding{Path: fileBinding.Path, SHA256: fileBinding.SHA256, Size: fileBinding.Size, Mode: uint32(info.Mode().Perm())}
		before = append(before, binding)
		beforeData[rel] = append([]byte(nil), data...)
		beforeModes[rel] = info.Mode().Perm()
	}
	if before[0].SHA256 != inspection.MissionIntentSHA256 || before[1].SHA256 != inspection.IntentSHA256 || before[2].SHA256 != inspection.CommitSHA256 {
		return nil, fmt.Errorf("retired onboarding inspection differs from the exact legacy inventory")
	}

	projectID, err := missionintent.DeriveProjectedProjectID(inspection.CommitSHA256)
	if err != nil {
		return nil, err
	}
	initialLane, err := projectedInitialLane(sourcePack, inspection.Identity.InitialLane)
	if err != nil {
		return nil, err
	}
	identity := inspection.Identity
	identity.SchemaVersion = 2
	identity.Target = "."
	identity.ProjectID = projectID
	identity.Pack = targetPack
	identity.InitialLane = initialLane
	recovery := missionintent.RecoveryEnvelope{
		SchemaVersion: 1,
		RepoRoot:      ".",
		CreatedAt:     inspection.Recovery.CreatedAt,
		Mode:          "attached-adoption",
		AttachedSnapshot: []missionintent.SnapshotArtifact{
			{Path: currentSkill.Path, Kind: "project-local-steamai-skill", SHA256: currentSkill.SHA256, Size: currentSkill.Size},
			{Path: currentInstance.Path, Kind: "instance-metadata", SHA256: currentInstance.SHA256, Size: currentInstance.Size},
			{Path: currentState.Path, Kind: "sync-state", SHA256: currentState.SHA256, Size: currentState.Size},
		},
	}
	sort.Slice(recovery.AttachedSnapshot, func(i, j int) bool { return recovery.AttachedSnapshot[i].Path < recovery.AttachedSnapshot[j].Path })

	lineageSHA, err := onboardingLineageSHA(sourcePack, targetPack, inspection.OnboardingPlanSHA256, inspection.PublicationStamp, before)
	if err != nil {
		return nil, err
	}
	marker, err := missionintent.MarshalCommittedV2Projected(caseRoot, identity, recovery, inspection.PublicationStamp, missionintent.OnboardingPlanSHA256Marker)
	if err != nil {
		return nil, err
	}
	targetPlanSHA, err := missionintent.HashOnboardingPlan(projectedOnboardingPlanHashInput(identity, recovery.CreatedAt, inspection.PublicationStamp, lineageSHA, marker))
	if err != nil {
		return nil, err
	}
	generation, err := missionintent.MarshalCommittedV2Projected(caseRoot, identity, recovery, inspection.PublicationStamp, targetPlanSHA)
	if err != nil {
		return nil, err
	}

	files := []plannedOnboardingFile{
		{rel: "onboarding/intent.json", kind: "onboarding-intent", phase: 0, before: beforeData["onboarding/intent.json"], beforeMode: beforeModes["onboarding/intent.json"], beforeExists: true, after: generation.IntentBytes, afterMode: 0o600},
		{rel: "mission-intent.json", kind: "mission-intent", phase: 1, before: beforeData["mission-intent.json"], beforeMode: beforeModes["mission-intent.json"], beforeExists: true, after: generation.MissionIntentBytes, afterMode: 0o600},
		{rel: "project-binding.json", kind: "project-binding", phase: 2, after: generation.ProjectBindingBytes, afterMode: 0o600},
		{rel: "onboarding/commit.json", kind: "onboarding-commit", phase: 3, before: beforeData["onboarding/commit.json"], beforeMode: beforeModes["onboarding/commit.json"], beforeExists: true, after: generation.CommitBytes, afterMode: 0o600},
	}
	after := make([]OnboardingFileBinding, 0, len(files))
	for _, file := range files {
		binding := bindingFor(".steamai/"+file.rel, file.after)
		after = append(after, OnboardingFileBinding{Path: binding.Path, SHA256: binding.SHA256, Size: binding.Size, Mode: uint32(file.afterMode.Perm())})
	}
	return &plannedOnboarding{
		projection: OnboardingProjection{
			SourceSchemaVersion: 1, TargetSchemaVersion: 2,
			SourceOnboardingPlanSHA256: inspection.OnboardingPlanSHA256, TargetOnboardingPlanSHA256: targetPlanSHA,
			SourceCommitSHA256: inspection.CommitSHA256, ProjectID: projectID, PublicationStamp: inspection.PublicationStamp,
			Before: before, After: after,
		},
		files: files,
	}, nil
}

func onboardingLineageSHA(sourcePack, targetPack, sourcePlanSHA, stamp string, before []OnboardingFileBinding) (string, error) {
	data, err := canonical(onboardingLineage{
		Kind: projectedOnboardingLineageKind, SourcePack: sourcePack, TargetPack: targetPack,
		SourceOnboardingPlanSHA256: sourcePlanSHA, PublicationStamp: stamp, Before: before,
	})
	if err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}

func projectedOnboardingPlanHashInput(identity missionintent.Identity, createdAt, stamp, lineageSHA string, marker missionintent.CommittedGeneration) missionintent.PlanHashInput {
	value := missionintent.PlanHashInput{
		SchemaVersion: 1, Command: missionintent.AttachedOnboardingPlanCommand, CaseRoot: ".", RepoRoot: ".", Pack: identity.Pack,
		ProjectName: identity.ProjectName, ProvisionID: "onboarding-" + stamp, Role: "mission-onboarding-adoption", CreatedAt: createdAt,
		LineageSHA256: lineageSHA,
	}
	for _, item := range []struct {
		path, kind string
		data       []byte
		phase      int
	}{
		{".steamai/onboarding/intent.json", "onboarding-intent", marker.IntentBytes, 0},
		{".steamai/mission-intent.json", "mission-intent", marker.MissionIntentBytes, 1},
		{".steamai/project-binding.json", "project-binding", marker.ProjectBindingBytes, 2},
		{".steamai/onboarding/commit.json", "onboarding-commit", marker.CommitBytes, 3},
	} {
		value.Writes = append(value.Writes, missionintent.PlanHashWrite{Path: item.path, Kind: item.kind, SHA256: sha256Hex(item.data), Size: int64(len(item.data)), PublicationPhase: item.phase})
	}
	return value
}

func projectedInitialLane(sourcePack, lane string) (string, error) {
	lane = strings.TrimSpace(lane)
	if sourcePack == packidentity.RetiredGeneric && lane == "main" {
		return "devirt-main", nil
	}
	switch lane {
	case "devirt-main", "feature-analysis", "binary-analysis", "tooling":
		return lane, nil
	default:
		return "", fmt.Errorf("retired onboarding initial lane cannot be mapped exactly to binary-re: %s", lane)
	}
}

func sameMigrationPath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	leftAbs, rightAbs = filepath.Clean(leftAbs), filepath.Clean(rightAbs)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(leftAbs, rightAbs)
	}
	return leftAbs == rightAbs
}

func onboardingProjection(plan *plannedOnboarding) *OnboardingProjection {
	if plan == nil {
		return nil
	}
	value := plan.projection
	value.Before = append([]OnboardingFileBinding(nil), plan.projection.Before...)
	value.After = append([]OnboardingFileBinding(nil), plan.projection.After...)
	return &value
}

func onboardingProjectionWrites(plan *plannedOnboarding) []Write {
	if plan == nil {
		return nil
	}
	out := make([]Write, 0, len(plan.files))
	for _, file := range plan.files {
		action := "create"
		if file.beforeExists {
			action = "replace-exact-after-rename"
		}
		out = append(out, Write{Path: ".steamai/" + file.rel, Kind: file.kind, Action: action, SHA256: sha256Hex(file.after), Size: int64(len(file.after))})
	}
	return out
}

func onboardingProjectionPublicationHashes(plan *plannedOnboarding) []publicationHash {
	if plan == nil {
		return nil
	}
	out := make([]publicationHash, 0, len(plan.files))
	for _, file := range plan.files {
		out = append(out, publicationHash{Path: ".steamai/" + file.rel, Kind: file.kind, SHA256: sha256Hex(file.after), Size: int64(len(file.after)), Mode: uint32(file.afterMode.Perm())})
	}
	return out
}

func overlayOnboardingInventory(entries []InventoryEntry, plan *plannedOnboarding) []InventoryEntry {
	if plan == nil {
		return entries
	}
	byPath := make(map[string]InventoryEntry, len(entries)+1)
	for _, entry := range entries {
		byPath[entry.Path] = entry
	}
	for _, binding := range plan.projection.Before {
		delete(byPath, strings.TrimPrefix(binding.Path, ".rekit/"))
	}
	for _, file := range plan.files {
		byPath[file.rel] = InventoryEntry{Path: file.rel, Kind: "file", SHA256: sha256Hex(file.after), Size: int64(len(file.after))}
		for parent := filepath.ToSlash(filepath.Dir(file.rel)); parent != "." && parent != ""; parent = filepath.ToSlash(filepath.Dir(parent)) {
			byPath[parent] = InventoryEntry{Path: parent, Kind: "directory"}
		}
	}
	out := make([]InventoryEntry, 0, len(byPath))
	for _, entry := range byPath {
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func validateOnboardingBeforeRename(root *os.Root, plan *plannedOnboarding) error {
	if plan == nil {
		return nil
	}
	for _, file := range plan.files {
		info, err := root.Lstat(filepath.FromSlash(file.rel))
		if !file.beforeExists {
			if err == nil {
				return fmt.Errorf("retired onboarding target appeared after preview: .rekit/%s", file.rel)
			}
			if !os.IsNotExist(err) {
				return err
			}
			continue
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || !rootFileModeMatches(uint32(file.beforeMode.Perm()), info) {
			return fmt.Errorf("retired onboarding source changed after preview: .rekit/%s", file.rel)
		}
		data, err := readRootFile(root, filepath.FromSlash(file.rel), info, maxStateFile)
		if err != nil || !bytes.Equal(data, file.before) {
			return fmt.Errorf("retired onboarding source changed after preview: .rekit/%s", file.rel)
		}
	}
	return nil
}

func applyProjectedOnboarding(caseRoot string, plan *plannedOnboarding) error {
	if plan == nil {
		return nil
	}
	root, err := rekitfs.OpenAnchoredRoot(filepath.Join(caseRoot, ".steamai"))
	if err != nil {
		return err
	}
	defer root.Close()
	byRel := map[string]plannedOnboardingFile{}
	for _, file := range plan.files {
		byRel[file.rel] = file
	}
	for _, rel := range []string{"onboarding/commit.json", "onboarding/intent.json", "mission-intent.json"} {
		file := byRel[rel]
		if err := root.RenameFileNoReplaceExact(rel, onboardingBackupPath(rel), file.before, file.beforeMode); err != nil {
			return fmt.Errorf("retire legacy onboarding artifact %s: %w", rel, err)
		}
	}
	for _, rel := range []string{"onboarding/intent.json", "mission-intent.json", "project-binding.json", "onboarding/commit.json"} {
		file := byRel[rel]
		if _, err := root.WriteExclusiveFileWriteThrough(rel, file.after, file.afterMode, false); err != nil {
			return fmt.Errorf("publish projected onboarding artifact %s: %w", rel, err)
		}
	}
	for _, rel := range []string{"onboarding/commit.json", "onboarding/intent.json", "mission-intent.json"} {
		file := byRel[rel]
		if err := root.RemoveExactFile(onboardingBackupPath(rel), file.before, file.beforeMode); err != nil {
			return fmt.Errorf("remove retired onboarding backup %s: %w", rel, err)
		}
	}
	return nil
}

func onboardingBackupPath(rel string) string {
	return filepath.ToSlash(filepath.Join("migration", ".onboarding-before-"+sha256Hex([]byte(filepath.ToSlash(rel)))[:16]))
}

func validateAppliedOnboarding(caseRoot string, plan *plannedOnboarding) error {
	if plan == nil {
		return nil
	}
	for _, file := range plan.files {
		path := filepath.Join(caseRoot, ".steamai", filepath.FromSlash(file.rel))
		data, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, path, "projected onboarding artifact", maxStateFile)
		if err != nil {
			return fmt.Errorf("read projected onboarding artifact %s: %w", file.rel, err)
		}
		if !bytes.Equal(data, file.after) {
			return fmt.Errorf("projected onboarding artifact differs from exact plan: %s", file.rel)
		}
	}
	return missionintent.ValidateCommittedV2ProjectedAt(caseRoot, projectedGeneration(plan))
}

func projectedGeneration(plan *plannedOnboarding) missionintent.CommittedGeneration {
	byRel := map[string][]byte{}
	for _, file := range plan.files {
		byRel[file.rel] = file.after
	}
	return missionintent.CommittedGeneration{
		MissionIntentBytes: byRel["mission-intent.json"], ProjectBindingBytes: byRel["project-binding.json"],
		IntentBytes: byRel["onboarding/intent.json"], CommitBytes: byRel["onboarding/commit.json"],
	}
}

func validateCurrentOnboardingProjection(caseRoot, sourcePack, targetPack string, projection *OnboardingProjection) error {
	if projection == nil {
		if !packidentity.IsRetired(sourcePack) {
			return nil
		}
		inspection, err := missionintent.Inspect(caseRoot)
		if err != nil {
			return fmt.Errorf("current onboarding state without a migration projection is invalid: %w", err)
		}
		if inspection.State != "absent" {
			return fmt.Errorf("retired migration receipt omits the current onboarding projection")
		}
		return nil
	}
	if err := validateOnboardingProjection(projection, sourcePack, targetPack); err != nil {
		return err
	}
	inspection, err := missionintent.Inspect(caseRoot)
	if err != nil {
		return fmt.Errorf("current onboarding projection is invalid: %w", err)
	}
	if !inspection.Committed || inspection.Identity.SchemaVersion != 2 || inspection.Identity.ProjectID != projection.ProjectID || inspection.Identity.Pack != targetPack || inspection.Identity.Target != "." || inspection.PublicationStamp != projection.PublicationStamp || inspection.OnboardingPlanSHA256 != projection.TargetOnboardingPlanSHA256 || inspection.CommitSHA256 != bindingSHA(projection.After, ".steamai/onboarding/commit.json") {
		return fmt.Errorf("current onboarding projection differs from migration receipt")
	}
	root, err := rekitfs.OpenAnchoredRoot(filepath.Join(caseRoot, ".steamai"))
	if err != nil {
		return err
	}
	defer root.Close()
	for _, binding := range projection.After {
		rel := strings.TrimPrefix(binding.Path, ".steamai/")
		data, info, exists, err := inspectRootTarget(root, rel, "current onboarding projection", maxStateFile)
		if err != nil {
			return fmt.Errorf("inspect current onboarding projection binding %s: %w", binding.Path, err)
		}
		if !exists || sha256Hex(data) != binding.SHA256 || int64(len(data)) != binding.Size || !rootFileModeMatches(binding.Mode, info) {
			return fmt.Errorf("current onboarding projection binding differs: %s", binding.Path)
		}
	}
	projectID, err := missionintent.DeriveProjectedProjectID(projection.SourceCommitSHA256)
	if err != nil || projectID != projection.ProjectID {
		return fmt.Errorf("current onboarding project id differs from retired source lineage")
	}
	lineageSHA, err := onboardingLineageSHA(sourcePack, targetPack, projection.SourceOnboardingPlanSHA256, projection.PublicationStamp, projection.Before)
	if err != nil {
		return err
	}
	marker, err := missionintent.MarshalCommittedV2Projected(caseRoot, inspection.Identity, inspection.Recovery, projection.PublicationStamp, missionintent.OnboardingPlanSHA256Marker)
	if err != nil {
		return err
	}
	targetPlanSHA, err := missionintent.HashOnboardingPlan(projectedOnboardingPlanHashInput(inspection.Identity, inspection.Recovery.CreatedAt, projection.PublicationStamp, lineageSHA, marker))
	if err != nil || targetPlanSHA != projection.TargetOnboardingPlanSHA256 {
		return fmt.Errorf("current onboarding plan hash differs from retired source projection")
	}
	generation, err := missionintent.MarshalCommittedV2Projected(caseRoot, inspection.Identity, inspection.Recovery, projection.PublicationStamp, targetPlanSHA)
	if err != nil {
		return err
	}
	for path, data := range map[string][]byte{
		".steamai/mission-intent.json":    generation.MissionIntentBytes,
		".steamai/project-binding.json":   generation.ProjectBindingBytes,
		".steamai/onboarding/intent.json": generation.IntentBytes,
		".steamai/onboarding/commit.json": generation.CommitBytes,
	} {
		if bindingSHA(projection.After, path) != sha256Hex(data) {
			return fmt.Errorf("current onboarding projection generation differs: %s", path)
		}
	}
	return nil
}

func validateOnboardingProjection(projection *OnboardingProjection, sourcePack, targetPack string) error {
	if projection == nil {
		return nil
	}
	if !packidentity.IsRetired(sourcePack) || targetPack != packidentity.Canonical || projection.SourceSchemaVersion != 1 || projection.TargetSchemaVersion != 2 || !validSHA256(projection.SourceOnboardingPlanSHA256) || !validSHA256(projection.TargetOnboardingPlanSHA256) || !validSHA256(projection.SourceCommitSHA256) || len(projection.ProjectID) != 16 || projection.PublicationStamp == "" || len(projection.Before) != 3 || len(projection.After) != 4 {
		return fmt.Errorf("state migration onboarding projection identity is invalid")
	}
	expectedBefore := []string{".rekit/mission-intent.json", ".rekit/onboarding/intent.json", ".rekit/onboarding/commit.json"}
	expectedAfter := []string{".steamai/onboarding/intent.json", ".steamai/mission-intent.json", ".steamai/project-binding.json", ".steamai/onboarding/commit.json"}
	for index, binding := range projection.Before {
		if binding.Path != expectedBefore[index] || !validSHA256(binding.SHA256) || binding.Size < 1 || binding.Mode == 0 {
			return fmt.Errorf("state migration onboarding before binding is invalid: %s", binding.Path)
		}
	}
	for index, binding := range projection.After {
		if binding.Path != expectedAfter[index] || !validSHA256(binding.SHA256) || binding.Size < 1 || binding.Mode == 0 {
			return fmt.Errorf("state migration onboarding after binding is invalid: %s", binding.Path)
		}
	}
	if projection.SourceCommitSHA256 != projection.Before[2].SHA256 {
		return fmt.Errorf("state migration onboarding source commit binding is inconsistent")
	}
	return nil
}

func bindingSHA(bindings []OnboardingFileBinding, path string) string {
	for _, binding := range bindings {
		if binding.Path == path {
			return binding.SHA256
		}
	}
	return ""
}

func expectedRetiredRootProjection(repoRoot, sourcePack string, transitions []RootFileTransition) error {
	if !packidentity.IsRetired(sourcePack) {
		return nil
	}
	m, err := manifest.Load(repoRoot, packidentity.Canonical)
	if err != nil {
		return err
	}
	if len(transitions) != len(m.ManagedFiles)+len(m.TemplateFiles)+2 {
		return fmt.Errorf("state migration receipt root projection is incomplete")
	}
	expected := map[string]struct {
		kind, sourcePath, sourceSHA string
	}{}
	for _, rel := range m.ManagedFiles {
		source, err := m.SourcePath(rel)
		if err != nil {
			return err
		}
		data, err := stableCanonicalRootSource(source, m.BudgetLimit(rel))
		if err != nil {
			return err
		}
		expected[filepath.ToSlash(rel)] = struct{ kind, sourcePath, sourceSHA string }{"managed-file", rootSourceRel(m, rel), sha256Hex(data)}
	}
	for _, rel := range m.TemplateFiles {
		source, err := m.SourcePath(rel)
		if err != nil {
			return err
		}
		data, err := stableCanonicalRootSource(source, maxRootProjectionFile)
		if err != nil {
			return err
		}
		target := strings.TrimSuffix(rel, ".template.md") + ".md"
		expected[filepath.ToSlash(target)] = struct{ kind, sourcePath, sourceSHA string }{"template-file", rootSourceRel(m, rel), sha256Hex(data)}
	}
	blockRel := m.ManagedBlock["file"]
	blockSourceRel := m.ManagedBlock["source"]
	blockSource, err := m.SourcePath(blockSourceRel)
	if err != nil {
		return err
	}
	blockData, err := stableCanonicalRootSource(blockSource, m.BudgetLimit(blockRel))
	if err != nil {
		return err
	}
	expected[filepath.ToSlash(blockRel)] = struct{ kind, sourcePath, sourceSHA string }{"managed-block", rootSourceRel(m, blockSourceRel), sha256Hex(blockData)}
	gitignoreSource, err := m.SourcePath("examples/gitignore.example")
	if err != nil {
		return err
	}
	gitignoreData, err := stableCanonicalRootSource(gitignoreSource, maxRootProjectionFile)
	if err != nil {
		return err
	}
	expected[".gitignore"] = struct{ kind, sourcePath, sourceSHA string }{"support-file", rootSourceRel(m, "examples/gitignore.example"), sha256Hex(gitignoreData)}
	for _, transition := range transitions {
		want, ok := expected[transition.Path]
		if !ok || transition.Kind != want.kind || transition.SourcePath != want.sourcePath || transition.SourceSHA256 != want.sourceSHA {
			return fmt.Errorf("state migration receipt root projection differs from canonical manifest: %s", transition.Path)
		}
		delete(expected, transition.Path)
	}
	if len(expected) != 0 {
		return fmt.Errorf("state migration receipt root projection omits canonical manifest targets")
	}
	return nil
}
