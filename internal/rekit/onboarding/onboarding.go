package onboarding

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casehealth"
	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneid"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	"github.com/shuiyu486/re-context-kits/internal/rekit/plancontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

const (
	planMarker          = missionintent.OnboardingPlanSHA256Marker
	attachedPlanCommand = missionintent.AttachedOnboardingPlanCommand
)

var (
	validInitialLane                 = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$`)
	attachedAdoptionBeforeCommitHook func() error
	attachedControlLeaves            = []string{
		"board.json",
		"policy.yml",
		"verification-role.json",
		"backups",
		"lanes",
		"facts",
		"runs",
		"handovers",
		"reviews",
		"reviewer-adoptions",
		"reopen-evidence",
		"reopen-operations",
		"member-executions",
		"external-session-attempts",
		"external-session-attempt-inputs",
		"external-session-dispatch",
		"external-session-jobs",
		"external-session-observations",
		"external-session-relays",
		"pack-memory",
		"session-host",
	}
)

type Options struct {
	Target                       string
	ProjectID                    string
	Pack                         string
	ProjectName                  string
	Goal                         string
	Actor                        string
	Executor                     string
	InitialLane                  string
	PublicationStamp             string
	ExpectedOnboardingPlanSHA256 string
	SourceExecutable             string
}

type Write struct {
	Path             string `json:"path"`
	Kind             string `json:"kind"`
	SHA256           string `json:"sha256"`
	Size             int64  `json:"size"`
	PublicationPhase int    `json:"publicationPhase"`
}

type Plan struct {
	SchemaVersion        int                          `json:"schemaVersion"`
	Command              string                       `json:"command"`
	CaseRoot             string                       `json:"caseRoot"`
	RepoRoot             string                       `json:"repoRoot"`
	ProjectID            string                       `json:"projectId,omitempty"`
	Pack                 string                       `json:"pack"`
	ProjectName          string                       `json:"projectName"`
	Goal                 string                       `json:"goal"`
	Actor                string                       `json:"actor"`
	Executor             string                       `json:"executor"`
	InitialLane          string                       `json:"initialLane"`
	IsMutation           bool                         `json:"isMutation"`
	ReviewRequired       bool                         `json:"reviewRequired"`
	RequiresConfirmation bool                         `json:"requiresConfirmation"`
	Replay               bool                         `json:"replay"`
	PublicationStamp     string                       `json:"publicationStamp"`
	OnboardingPlanSHA256 string                       `json:"onboardingPlanSha256"`
	ApplyCommand         string                       `json:"applyCommand"`
	ApplyArgs            []string                     `json:"applyArgs"`
	Writes               []Write                      `json:"writes"`
	BlockedActions       []string                     `json:"blockedActions"`
	NextSteps            []string                     `json:"nextSteps"`
	Identity             missionintent.Identity       `json:"-"`
	ExclusivePlan        syncreview.ExclusiveInitPlan `json:"-"`
	DurableRecovery      bool                         `json:"-"`
}

type Result struct {
	SchemaVersion        int                      `json:"schemaVersion"`
	Command              string                   `json:"command"`
	CaseRoot             string                   `json:"caseRoot"`
	ProjectID            string                   `json:"projectId,omitempty"`
	Pack                 string                   `json:"pack"`
	ProjectName          string                   `json:"projectName"`
	Applied              bool                     `json:"applied"`
	Replay               bool                     `json:"replay"`
	PublicationStamp     string                   `json:"publicationStamp"`
	OnboardingPlanSHA256 string                   `json:"onboardingPlanSha256"`
	ApplyCommand         string                   `json:"applyCommand"`
	ApplyArgs            []string                 `json:"applyArgs"`
	Writes               []Write                  `json:"writes"`
	Inspection           missionintent.Inspection `json:"inspection"`
	NextSteps            []string                 `json:"nextSteps"`
}

func Preview(repoRoot string, opt Options) (Plan, error) {
	if _, err := plancontract.ValidatePhase(
		commands.Onboard,
		"-ExpectedOnboardingPlanSha256",
		true,
		false,
		opt.ExpectedOnboardingPlanSHA256,
	); err != nil {
		return Plan{}, err
	}
	return build(repoRoot, opt, false)
}

func Apply(repoRoot string, opt Options) (Result, error) {
	expectedPlanSHA256, err := plancontract.RequireApplyBinding(
		commands.Onboard,
		"-ExpectedOnboardingPlanSha256",
		opt.ExpectedOnboardingPlanSHA256,
	)
	if err != nil {
		return Result{}, err
	}
	opt.ExpectedOnboardingPlanSHA256 = expectedPlanSHA256
	if strings.TrimSpace(opt.PublicationStamp) == "" {
		return Result{}, fmt.Errorf("onboard -Apply requires -OnboardingPublicationStamp from the exact preview")
	}
	plan, err := build(repoRoot, opt, true)
	if err != nil {
		return Result{}, err
	}
	if _, err := plancontract.Match(
		commands.Onboard,
		"-ExpectedOnboardingPlanSha256",
		opt.ExpectedOnboardingPlanSHA256,
		plan.OnboardingPlanSHA256,
	); err != nil {
		return Result{}, err
	}
	if plan.ExclusivePlan.Command == attachedPlanCommand {
		return applyAttachedAdoption(plan)
	}
	if !plan.Replay && !plan.DurableRecovery {
		createdAt, err := time.Parse(time.RFC3339Nano, plan.ExclusivePlan.CreatedAt)
		if err != nil {
			return Result{}, err
		}
		ordinary, err := ordinaryPlan(repoRoot, plan.CaseRoot, plan.Identity, plan.PublicationStamp, createdAt, true, opt.SourceExecutable)
		if err != nil {
			return Result{}, err
		}
		marker, err := planFromOrdinary(ordinary, plan.Identity, plan.PublicationStamp, planMarker)
		if err != nil {
			return Result{}, err
		}
		currentHash, err := hashExclusivePlan(marker, plan.Identity)
		if err != nil || !strings.EqualFold(currentHash, plan.OnboardingPlanSHA256) {
			return Result{}, fmt.Errorf("onboarding source snapshot changed after reviewed preview")
		}
		current, err := planFromOrdinary(ordinary, plan.Identity, plan.PublicationStamp, currentHash)
		if err != nil {
			return Result{}, err
		}
		plannedBytes, _ := json.Marshal(plan.ExclusivePlan)
		currentBytes, _ := json.Marshal(current)
		if string(plannedBytes) != string(currentBytes) {
			return Result{}, fmt.Errorf("onboarding source snapshot changed after plan reconstruction")
		}
	}
	if plan.Replay {
		inspection, err := missionintent.Inspect(plan.CaseRoot)
		if err != nil {
			return Result{}, err
		}
		return resultFor(plan, inspection, true), nil
	}
	if _, err := syncreview.ApplyExclusiveInit(plan.ExclusivePlan); err != nil {
		return Result{}, err
	}
	inspection, err := missionintent.Inspect(plan.CaseRoot)
	if err != nil {
		return Result{}, err
	}
	if !inspection.Committed || !strings.EqualFold(inspection.OnboardingPlanSHA256, plan.OnboardingPlanSHA256) {
		return Result{}, fmt.Errorf("onboarding publication did not reach the exact committed generation")
	}
	return resultFor(plan, inspection, false), nil
}

func build(repoRoot string, opt Options, allowExisting bool) (Plan, error) {
	caseRoot, identity, err := normalizeIdentity(opt)
	if err != nil {
		return Plan{}, err
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return Plan{}, err
	}
	if existing, statErr := os.Lstat(caseRoot); statErr == nil {
		if !existing.IsDir() || existing.Mode()&os.ModeSymlink != 0 {
			return Plan{}, fmt.Errorf("onboard target is not a regular directory: %s", caseRoot)
		}
		inspection, inspectErr := missionintent.Inspect(caseRoot)
		if inspectErr != nil {
			return Plan{}, inspectErr
		}
		if inspection.State == "absent" {
			return buildAttachedAdoption(repoRoot, caseRoot, identity, opt)
		}
		if inspection.Identity != identity {
			return Plan{}, fmt.Errorf("onboard identity differs from the immutable existing mission intent")
		}
		if inspection.Committed {
			if identity.SchemaVersion == 1 && inspection.Recovery.BundleManifest == (missionintent.BundleBinding{}) && !samePath(inspection.Recovery.RepoRoot, repoRoot) {
				return Plan{}, fmt.Errorf("committed onboarding belongs to a different canonical kit root: %s", inspection.Recovery.RepoRoot)
			}
			if opt.PublicationStamp != "" && opt.PublicationStamp != inspection.PublicationStamp {
				return Plan{}, fmt.Errorf("onboarding publication stamp differs from committed identity")
			}
			if opt.ExpectedOnboardingPlanSHA256 != "" && !strings.EqualFold(opt.ExpectedOnboardingPlanSHA256, inspection.OnboardingPlanSHA256) {
				return Plan{}, fmt.Errorf("onboarding plan hash differs from committed identity")
			}
			return replayPlan(repoRoot, caseRoot, inspection), nil
		}
		if !allowExisting && opt.PublicationStamp == "" {
			return Plan{}, fmt.Errorf("onboarding publication is pending; rerun the exact Apply command with stamp %s and plan hash %s", inspection.PublicationStamp, inspection.OnboardingPlanSHA256)
		}
		if opt.PublicationStamp != inspection.PublicationStamp || (opt.ExpectedOnboardingPlanSHA256 != "" && !strings.EqualFold(opt.ExpectedOnboardingPlanSHA256, inspection.OnboardingPlanSHA256)) {
			return Plan{}, fmt.Errorf("pending onboarding requires the exact publication stamp and plan hash")
		}
		if identity.SchemaVersion == 1 && inspection.Recovery.BundleManifest == (missionintent.BundleBinding{}) && !samePath(inspection.Recovery.RepoRoot, repoRoot) {
			return Plan{}, fmt.Errorf("pending onboarding recovery is bound to a different canonical kit root: %s", inspection.Recovery.RepoRoot)
		}
		if inspection.Recovery.Mode == "attached-adoption" {
			if err := validateRecoveryEnvelopeAt(caseRoot, identity, inspection.Recovery); err != nil {
				return Plan{}, err
			}
			createdAt, err := time.Parse(time.RFC3339Nano, inspection.Recovery.CreatedAt)
			if err != nil {
				return Plan{}, err
			}
			marker, err := attachedPlan(repoRoot, caseRoot, identity, inspection.PublicationStamp, createdAt, inspection.Recovery.AttachedSnapshot, planMarker)
			if err != nil {
				return Plan{}, err
			}
			hash, err := hashExclusivePlan(marker, identity)
			if err != nil || !strings.EqualFold(hash, inspection.OnboardingPlanSHA256) {
				return Plan{}, fmt.Errorf("durable attached onboarding recovery envelope does not reconstruct the reviewed plan hash")
			}
			exact, err := attachedPlan(repoRoot, caseRoot, identity, inspection.PublicationStamp, createdAt, inspection.Recovery.AttachedSnapshot, inspection.OnboardingPlanSHA256)
			if err != nil {
				return Plan{}, err
			}
			plan := publicPlan(exact, identity, inspection.PublicationStamp, inspection.OnboardingPlanSHA256, false)
			plan.DurableRecovery = true
			return plan, nil
		}
		ordinary, err := ordinaryPlanFromRecovery(caseRoot, identity, inspection.Recovery)
		if err != nil {
			return Plan{}, err
		}
		marker, err := planFromOrdinary(ordinary, identity, inspection.PublicationStamp, planMarker)
		if err != nil {
			return Plan{}, err
		}
		hash, err := hashExclusivePlan(marker, identity)
		if err != nil || !strings.EqualFold(hash, inspection.OnboardingPlanSHA256) {
			return Plan{}, fmt.Errorf("durable onboarding recovery envelope does not reconstruct the reviewed plan hash")
		}
		exact, err := planFromOrdinary(ordinary, identity, inspection.PublicationStamp, inspection.OnboardingPlanSHA256)
		if err != nil {
			return Plan{}, err
		}
		plan := publicPlan(exact, identity, inspection.PublicationStamp, inspection.OnboardingPlanSHA256, false)
		plan.DurableRecovery = true
		return plan, nil
	} else if !os.IsNotExist(statErr) {
		return Plan{}, statErr
	}
	if err := validateInitialLane(repoRoot, identity.Pack, identity.InitialLane); err != nil {
		return Plan{}, err
	}

	stamp := strings.TrimSpace(opt.PublicationStamp)
	if stamp == "" {
		stamp = time.Now().UTC().Format("20060102-150405000")
	}
	if len(stamp) != len("20060102-150405000") {
		return Plan{}, fmt.Errorf("invalid onboarding publication stamp: %s", stamp)
	}
	createdAt, err := time.Parse("20060102-150405.000", stamp[:15]+"."+stamp[15:])
	if err != nil {
		return Plan{}, fmt.Errorf("invalid onboarding publication stamp: %s", stamp)
	}
	ordinary, err := ordinaryPlan(repoRoot, caseRoot, identity, stamp, createdAt, allowExisting, opt.SourceExecutable)
	if err != nil {
		return Plan{}, err
	}
	placeholder, err := planFromOrdinary(ordinary, identity, stamp, planMarker)
	if err != nil {
		return Plan{}, err
	}
	planSHA256, err := hashExclusivePlan(placeholder, identity)
	if err != nil {
		return Plan{}, err
	}
	exact, err := planFromOrdinary(ordinary, identity, stamp, planSHA256)
	if err != nil {
		return Plan{}, err
	}
	plan := publicPlan(exact, identity, stamp, planSHA256, false)
	return plan, nil
}

func buildAttachedAdoption(repoRoot, caseRoot string, identity missionintent.Identity, opt Options) (Plan, error) {
	attachedRepoRoot := repoRoot
	inst, err := instance.Read(caseRoot)
	if err != nil {
		return Plan{}, err
	}
	if inst.StateDir == projectstate.CurrentDir {
		attachedRepoRoot = inst.TemplateRoot
	}
	if _, err := instance.AssertAttached(caseRoot, attachedRepoRoot, identity.Pack); err != nil {
		return Plan{}, fmt.Errorf("onboard attached adoption requires a current attached case: %w", err)
	}
	if err := validateInitialLane(attachedRepoRoot, identity.Pack, identity.InitialLane); err != nil {
		return Plan{}, err
	}
	if err := ensureAttachedMissionControlEmpty(caseRoot); err != nil {
		return Plan{}, err
	}
	rows, err := casehealth.Static(attachedRepoRoot, caseRoot, identity.Pack)
	if err != nil {
		return Plan{}, fmt.Errorf("onboard attached adoption requires doctor-ready case files: %w", err)
	}
	stamp, createdAt, err := onboardingStamp(opt.PublicationStamp)
	if err != nil {
		return Plan{}, err
	}
	snapshot, err := attachedSnapshot(caseRoot, rows)
	if err != nil {
		return Plan{}, err
	}
	if opt.PublicationStamp != "" && strings.TrimSpace(opt.ExpectedOnboardingPlanSHA256) == "" {
		return Plan{}, fmt.Errorf("attached onboarding Apply requires the exact reviewed plan hash")
	}
	marker, err := attachedPlan(repoRoot, caseRoot, identity, stamp, createdAt, snapshot, planMarker)
	if err != nil {
		return Plan{}, err
	}
	hash, err := hashExclusivePlan(marker, identity)
	if err != nil {
		return Plan{}, err
	}
	if expected := strings.TrimSpace(opt.ExpectedOnboardingPlanSHA256); expected != "" && !strings.EqualFold(expected, hash) {
		return Plan{}, fmt.Errorf("attached onboarding snapshot changed after reviewed preview")
	}
	exact, err := attachedPlan(repoRoot, caseRoot, identity, stamp, createdAt, snapshot, hash)
	if err != nil {
		return Plan{}, err
	}
	return publicPlan(exact, identity, stamp, hash, false), nil
}

func attachedPlan(repoRoot, caseRoot string, identity missionintent.Identity, stamp string, createdAt time.Time, snapshot []missionintent.SnapshotArtifact, hash string) (syncreview.ExclusiveInitPlan, error) {
	recoveryRepoRoot := repoRoot
	if identity.SchemaVersion == 2 {
		recoveryRepoRoot = "."
	}
	recovery := missionintent.RecoveryEnvelope{SchemaVersion: 1, RepoRoot: recoveryRepoRoot, CreatedAt: createdAt.UTC().Format(time.RFC3339Nano), Mode: "attached-adoption", AttachedSnapshot: append([]missionintent.SnapshotArtifact{}, snapshot...)}
	if err := validateRecoveryEnvelopeAt(caseRoot, identity, recovery); err != nil {
		return syncreview.ExclusiveInitPlan{}, err
	}
	base := syncreview.ExclusiveInitPlan{SchemaVersion: 1, Command: attachedPlanCommand, CaseRoot: caseRoot, RepoRoot: recoveryRepoRoot, Pack: identity.Pack, ProjectName: identity.ProjectName, ProvisionID: "onboarding-" + stamp, Role: "mission-onboarding-adoption", CreatedAt: recovery.CreatedAt, BlockedActions: []string{"existing case content writes", "existing Mission Control takeover", "overwrite", "backup", "force", "authority/confirmed writes", "heavy-tool execution"}}
	generated, err := generatedOnboardingWrites(caseRoot, identity, recovery, stamp, hash)
	if err != nil {
		return syncreview.ExclusiveInitPlan{}, err
	}
	base.Writes = append(base.Writes, generated...)
	return base, nil
}

func validateRecoveryEnvelopeAt(caseRoot string, identity missionintent.Identity, recovery missionintent.RecoveryEnvelope) error {
	if identity.SchemaVersion == 2 {
		return missionintent.ValidateRecoveryEnvelopeAt(caseRoot, identity, recovery)
	}
	return missionintent.ValidateRecoveryEnvelope(identity, recovery)
}

func generatedOnboardingWrites(caseRoot string, identity missionintent.Identity, recovery missionintent.RecoveryEnvelope, stamp, planSHA256 string) ([]syncreview.ExclusiveInitWrite, error) {
	artifactPaths, err := missionintent.Paths(caseRoot)
	if err != nil {
		return nil, err
	}
	var missionBytes []byte
	if identity.SchemaVersion == 2 {
		missionBytes, err = missionintent.MarshalMissionIntentAt(caseRoot, identity)
	} else {
		missionBytes, err = missionintent.MarshalMissionIntent(identity)
	}
	if err != nil {
		return nil, err
	}

	intent := missionintent.Intent{
		SchemaVersion:        identity.SchemaVersion,
		Kind:                 "mission-onboarding-intent",
		PublicationStamp:     stamp,
		OnboardingPlanSHA256: planSHA256,
		Identity:             identity,
		Recovery:             recovery,
	}
	generated := []struct {
		path, kind string
		content    []byte
		phase      int
	}{}
	if identity.SchemaVersion == 2 {
		generation, err := missionintent.MarshalCommittedV2Projected(caseRoot, identity, recovery, stamp, planSHA256)
		if err != nil {
			return nil, err
		}
		missionBytes = generation.MissionIntentBytes
		bindingBytes := generation.ProjectBindingBytes
		intentBytes := generation.IntentBytes
		commitBytes := generation.CommitBytes
		generated = append(generated,
			struct {
				path, kind string
				content    []byte
				phase      int
			}{artifactPaths.Intent, "onboarding-intent", intentBytes, 0},
			struct {
				path, kind string
				content    []byte
				phase      int
			}{artifactPaths.MissionIntent, "mission-intent", missionBytes, 1},
			struct {
				path, kind string
				content    []byte
				phase      int
			}{artifactPaths.ProjectBinding, "project-binding", bindingBytes, 2},
			struct {
				path, kind string
				content    []byte
				phase      int
			}{artifactPaths.Commit, "onboarding-commit", commitBytes, 3},
		)
	} else {
		intentBytes, err := missionintent.MarshalIntent(intent)
		if err != nil {
			return nil, err
		}
		commit := missionintent.Commit{SchemaVersion: 1, Kind: "mission-onboarding-commit", PublicationStamp: stamp, OnboardingPlanSHA256: planSHA256, MissionIntentSHA256: missionintent.SHA256(missionBytes), IntentSHA256: missionintent.SHA256(intentBytes)}
		commitBytes, err := missionintent.MarshalCommit(commit)
		if err != nil && planSHA256 == planMarker {
			commitBytes, err = marshalMarkerCommit(stamp, commit.MissionIntentSHA256, commit.IntentSHA256)
		}
		if err != nil {
			return nil, err
		}
		generated = append(generated,
			struct {
				path, kind string
				content    []byte
				phase      int
			}{artifactPaths.Intent, "onboarding-intent", intentBytes, 0},
			struct {
				path, kind string
				content    []byte
				phase      int
			}{artifactPaths.MissionIntent, "mission-intent", missionBytes, 1},
			struct {
				path, kind string
				content    []byte
				phase      int
			}{artifactPaths.Commit, "onboarding-commit", commitBytes, 2},
		)
	}
	writes := make([]syncreview.ExclusiveInitWrite, 0, len(generated))
	for _, artifact := range generated {
		writes = append(writes, syncreview.ExclusiveInitWrite{
			Path:             artifact.path,
			Kind:             artifact.kind,
			TargetPath:       filepath.Join(caseRoot, filepath.FromSlash(artifact.path)),
			SHA256:           missionintent.SHA256(artifact.content),
			Size:             int64(len(artifact.content)),
			Content:          artifact.content,
			PublicationPhase: artifact.phase,
		})
	}
	return writes, nil
}

func attachedSnapshot(caseRoot string, rows []casehealth.Row) ([]missionintent.SnapshotArtifact, error) {
	stateRoot, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return nil, err
	}
	statePrefix := filepath.ToSlash(stateRoot.Dir)
	paths := map[string]string{
		statePrefix + "/instance.yml": "instance-metadata",
		statePrefix + "/state.json":   "sync-state",
	}
	if stateRoot.Legacy {
		paths[".claude/skills/rekit/SKILL.md"] = "case-local-thin-shim"
		paths[".re-template.yml"] = "legacy-metadata"
	} else {
		paths[".claude/skills/steamai/SKILL.md"] = "project-local-steamai-skill"
	}
	for _, row := range rows {
		rel, err := filepath.Rel(caseRoot, row.File)
		if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		path := filepath.ToSlash(filepath.Clean(rel))
		if _, fixed := paths[path]; fixed {
			continue
		}
		if !stateRoot.Legacy && strings.HasPrefix(strings.ToLower(path), strings.ToLower(statePrefix)+"/") {
			// The strict bundle manifest already binds every project-local runtime
			// asset. Attached adoption snapshots case content, not a second copy of
			// the runtime/control namespace.
			continue
		}
		paths[path] = "doctor-validated-artifact"
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	snapshot := make([]missionintent.SnapshotArtifact, 0, len(ordered))
	for _, rel := range ordered {
		path := filepath.Join(caseRoot, filepath.FromSlash(rel))
		data, err := refsf.ReadStableRegularFileAnchored(caseRoot, path, "attached onboarding snapshot", 5*1024*1024+1)
		if err != nil {
			return nil, err
		}
		snapshot = append(snapshot, missionintent.SnapshotArtifact{Path: rel, Kind: paths[rel], SHA256: missionintent.SHA256(data), Size: int64(len(data))})
	}
	return snapshot, nil
}

func validateAttachedSnapshot(caseRoot string, expected []missionintent.SnapshotArtifact) error {
	for _, artifact := range expected {
		path := filepath.Join(caseRoot, filepath.FromSlash(artifact.Path))
		data, err := refsf.ReadStableRegularFileAnchored(caseRoot, path, "attached onboarding snapshot", artifact.Size+1)
		if err != nil || int64(len(data)) != artifact.Size || !strings.EqualFold(missionintent.SHA256(data), artifact.SHA256) {
			return fmt.Errorf("attached onboarding snapshot changed: %s", artifact.Path)
		}
	}
	return nil
}

func ensureAttachedMissionControlEmpty(caseRoot string) error {
	for _, leaf := range attachedControlLeaves {
		rel, err := projectstate.Rel(caseRoot, leaf)
		if err != nil {
			return err
		}
		path, err := projectstate.Join(caseRoot, leaf)
		if err != nil {
			return err
		}
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("onboard attached adoption refuses existing Mission Control state: %s", rel)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func applyAttachedAdoption(plan Plan) (result Result, retErr error) {
	lease, err := lanemutation.AcquireProject(plan.CaseRoot)
	if err != nil {
		return Result{}, err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	if err := lease.Validate(); err != nil {
		return Result{}, err
	}
	inspection, err := missionintent.Inspect(plan.CaseRoot)
	if err != nil {
		return Result{}, err
	}
	if inspection.State == "committed" {
		if inspection.Identity != plan.Identity || !strings.EqualFold(inspection.OnboardingPlanSHA256, plan.OnboardingPlanSHA256) {
			return Result{}, fmt.Errorf("committed onboarding differs from attached adoption request")
		}
		return resultFor(plan, inspection, true), nil
	}
	if inspection.State == "absent" {
		attachedRepoRoot := plan.RepoRoot
		inst, err := instance.Read(plan.CaseRoot)
		if err != nil {
			return Result{}, err
		}
		if inst.StateDir == projectstate.CurrentDir {
			attachedRepoRoot = inst.TemplateRoot
		}
		if _, err := instance.AssertAttached(plan.CaseRoot, attachedRepoRoot, plan.Pack); err != nil {
			return Result{}, err
		}
	}
	if err := ensureAttachedMissionControlEmpty(plan.CaseRoot); err != nil {
		return Result{}, err
	}
	snapshot, err := recoverySnapshot(plan.ExclusivePlan)
	if err != nil {
		return Result{}, err
	}
	if err := validateAttachedSnapshot(plan.CaseRoot, snapshot); err != nil {
		return Result{}, err
	}
	for index, write := range plan.ExclusivePlan.Writes {
		if index == len(plan.ExclusivePlan.Writes)-1 {
			if attachedAdoptionBeforeCommitHook != nil {
				if err := attachedAdoptionBeforeCommitHook(); err != nil {
					return Result{}, err
				}
			}
			if err := lease.Validate(); err != nil {
				return Result{}, err
			}
			if err := ensureAttachedMissionControlEmpty(plan.CaseRoot); err != nil {
				return Result{}, err
			}
			if err := validateAttachedSnapshot(plan.CaseRoot, snapshot); err != nil {
				return Result{}, err
			}
		}
		if _, err := refsf.WriteExclusiveRegularFileAnchored(plan.CaseRoot, write.Path, write.Kind, write.Content); err != nil {
			return Result{}, err
		}
	}
	inspection, err = missionintent.Inspect(plan.CaseRoot)
	if err != nil {
		return Result{}, err
	}
	if !inspection.Committed || inspection.Identity != plan.Identity || !strings.EqualFold(inspection.OnboardingPlanSHA256, plan.OnboardingPlanSHA256) {
		return Result{}, fmt.Errorf("attached onboarding publication did not reach the exact committed generation")
	}
	return resultFor(plan, inspection, false), nil
}

func recoverySnapshot(plan syncreview.ExclusiveInitPlan) ([]missionintent.SnapshotArtifact, error) {
	artifactPaths, err := missionintent.Paths(plan.CaseRoot)
	if err != nil {
		return nil, err
	}
	expectedTarget := filepath.Join(plan.CaseRoot, filepath.FromSlash(artifactPaths.Intent))
	for _, write := range plan.Writes {
		if write.Path != artifactPaths.Intent {
			continue
		}
		if write.Kind != "onboarding-intent" || write.TargetPath != expectedTarget {
			return nil, fmt.Errorf("attached onboarding plan has an invalid intent artifact binding: %s", write.Path)
		}
		var intent missionintent.Intent
		if err := json.Unmarshal(write.Content, &intent); err != nil || intent.Kind != "mission-onboarding-intent" || intent.Recovery.Mode != "attached-adoption" {
			return nil, fmt.Errorf("attached onboarding plan has an invalid intent artifact: %s", write.Path)
		}
		if intent.SchemaVersion == 1 {
			if intent.Identity.Target != plan.CaseRoot {
				return nil, fmt.Errorf("attached onboarding plan has an invalid intent artifact: %s", write.Path)
			}
		} else if err := missionintent.ValidateIntentAt(plan.CaseRoot, intent); err != nil {
			return nil, fmt.Errorf("attached onboarding plan has an invalid intent artifact: %s", write.Path)
		}
		return append([]missionintent.SnapshotArtifact{}, intent.Recovery.AttachedSnapshot...), nil
	}
	return nil, fmt.Errorf("attached onboarding plan omits its exact intent artifact: %s", artifactPaths.Intent)
}

func onboardingStamp(value string) (string, time.Time, error) {
	stamp := strings.TrimSpace(value)
	if stamp == "" {
		stamp = time.Now().UTC().Format("20060102-150405000")
	}
	if len(stamp) != len("20060102-150405000") {
		return "", time.Time{}, fmt.Errorf("invalid onboarding publication stamp: %s", stamp)
	}
	createdAt, err := time.Parse("20060102-150405.000", stamp[:15]+"."+stamp[15:])
	if err != nil {
		return "", time.Time{}, fmt.Errorf("invalid onboarding publication stamp: %s", stamp)
	}
	return stamp, createdAt, nil
}

func ordinaryPlan(repoRoot, caseRoot string, identity missionintent.Identity, stamp string, createdAt time.Time, allowExisting bool, sourceExecutable string) (syncreview.ExclusiveInitPlan, error) {
	options := syncreview.ExclusiveInitOptions{ProjectName: identity.ProjectName, ProvisionID: "onboarding-" + stamp, Role: "mission-onboarding", CreatedAt: createdAt, SkipVerificationMarker: true, DefaultPublicationPhase: 1, SourceExecutable: strings.TrimSpace(sourceExecutable)}
	var (
		plan syncreview.ExclusiveInitPlan
		err  error
	)
	if allowExisting {
		plan, err = syncreview.PlanExclusiveInitReplay(repoRoot, caseRoot, identity.Pack, options)
	} else {
		plan, err = syncreview.PlanExclusiveInit(repoRoot, caseRoot, identity.Pack, options)
	}
	if err != nil {
		return syncreview.ExclusiveInitPlan{}, err
	}
	return remapOrdinaryPlanStateRoot(plan, caseRoot)
}

func remapOrdinaryPlanStateRoot(plan syncreview.ExclusiveInitPlan, caseRoot string) (syncreview.ExclusiveInitPlan, error) {
	stateRoot, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return syncreview.ExclusiveInitPlan{}, err
	}
	if stateRoot.Legacy {
		return plan, nil
	}
	// Current exclusive init already publishes the canonical .steamai bundle,
	// metadata, skill, and relocatable sync state. The historical remap is kept
	// only as a guard against accidentally feeding a legacy plan into this path.
	for _, write := range plan.Writes {
		if strings.HasPrefix(strings.ToLower(filepath.ToSlash(write.Path)), ".rekit/") || strings.EqualFold(filepath.ToSlash(write.Path), ".re-template.yml") || strings.EqualFold(filepath.ToSlash(write.Path), ".claude/skills/rekit/SKILL.md") {
			return syncreview.ExclusiveInitPlan{}, fmt.Errorf("current onboarding received legacy exclusive-init publication: %s", write.Path)
		}
	}
	return plan, nil
}

func ordinaryPlanFromRecovery(caseRoot string, identity missionintent.Identity, recovery missionintent.RecoveryEnvelope) (syncreview.ExclusiveInitPlan, error) {
	if err := validateRecoveryEnvelopeAt(caseRoot, identity, recovery); err != nil {
		return syncreview.ExclusiveInitPlan{}, err
	}
	plan := syncreview.ExclusiveInitPlan{SchemaVersion: 1, Command: "exclusive-init", CaseRoot: caseRoot, RepoRoot: recovery.RepoRoot, Pack: identity.Pack, ProjectName: identity.ProjectName, ProvisionID: "onboarding-" + strings.ReplaceAll(strings.ReplaceAll(recovery.CreatedAt, "-", ""), ":", ""), Role: "mission-onboarding", CreatedAt: recovery.CreatedAt, BlockedActions: []string{"existing root takeover", "overwrite", "backup", "force", "authority/confirmed writes", "heavy-tool execution"}}
	stampTime, err := time.Parse(time.RFC3339Nano, recovery.CreatedAt)
	if err != nil {
		return syncreview.ExclusiveInitPlan{}, err
	}
	plan.ProvisionID = "onboarding-" + stampTime.UTC().Format("20060102-150405000")
	for _, write := range recovery.Writes {
		materialized := write
		if identity.SchemaVersion == 2 {
			materialized, err = missionintent.MaterializeRecoveryWriteAt(caseRoot, identity, write)
			if err != nil {
				return syncreview.ExclusiveInitPlan{}, err
			}
		}
		planned := syncreview.ExclusiveInitWrite{Path: materialized.Path, Kind: materialized.Kind, TargetPath: filepath.Join(caseRoot, filepath.FromSlash(materialized.Path)), SHA256: materialized.SHA256, Size: materialized.Size, Content: append([]byte{}, materialized.Content...), PublicationPhase: materialized.PublicationPhase}
		if write.Kind == "runtime-executable" {
			source, err := runtimebundle.SourceExecutable()
			if err != nil {
				return syncreview.ExclusiveInitPlan{}, err
			}
			data, err := refsf.ReadStableRegularFileAnchored(filepath.Dir(source), source, "onboarding recovery runtime executable", write.Size+1)
			if err != nil || int64(len(data)) != write.Size || !strings.EqualFold(missionintent.SHA256(data), write.SHA256) {
				return syncreview.ExclusiveInitPlan{}, fmt.Errorf("current STeamAI executable does not match onboarding recovery bundle binding")
			}
			planned, err = syncreview.BindExclusiveInitWriteSnapshot(planned, data)
			if err != nil {
				return syncreview.ExclusiveInitPlan{}, err
			}
		}
		plan.Writes = append(plan.Writes, planned)
	}
	return plan, nil
}

func planFromOrdinary(ordinary syncreview.ExclusiveInitPlan, identity missionintent.Identity, stamp, planSHA256 string) (syncreview.ExclusiveInitPlan, error) {
	recoveryRepoRoot := ordinary.RepoRoot
	for _, write := range ordinary.Writes {
		if write.Kind == "runtime-bundle-manifest" {
			recoveryRepoRoot = "."
			break
		}
	}
	recovery := missionintent.RecoveryEnvelope{SchemaVersion: 1, RepoRoot: recoveryRepoRoot, CreatedAt: ordinary.CreatedAt}
	for _, write := range ordinary.Writes {
		content := append([]byte{}, write.Content...)
		if write.Kind == "runtime-executable" {
			content = nil
		}
		if strings.TrimSpace(write.SourcePath) != "" && write.Kind != "runtime-executable" {
			var err error
			content, err = refsf.ReadStableRegularFileAnchored(filepath.Dir(write.SourcePath), write.SourcePath, "onboarding recovery bundle asset", write.Size+1)
			if err != nil || int64(len(content)) != write.Size || !strings.EqualFold(missionintent.SHA256(content), write.SHA256) {
				return syncreview.ExclusiveInitPlan{}, fmt.Errorf("onboarding bundle source changed while building recovery: %s", write.Path)
			}
		}
		recoveryWrite := missionintent.RecoveryWrite{Path: write.Path, Kind: write.Kind, SHA256: write.SHA256, Size: write.Size, Content: content, PublicationPhase: write.PublicationPhase}
		if identity.SchemaVersion == 2 {
			var err error
			recoveryWrite, err = missionintent.CanonicalRecoveryWriteAt(ordinary.CaseRoot, identity, recoveryWrite)
			if err != nil {
				return syncreview.ExclusiveInitPlan{}, err
			}
		}
		recovery.Writes = append(recovery.Writes, recoveryWrite)
		if write.Kind == "runtime-bundle-manifest" {
			recovery.BundleManifest.Path = runtimebundle.ManifestRel
			recovery.BundleManifest.SHA256 = write.SHA256
		}
	}
	if recovery.BundleManifest.Path != "" {
		manifestWrite, ok := recoveryWriteByKind(recovery.Writes, "runtime-bundle-manifest")
		if !ok {
			return syncreview.ExclusiveInitPlan{}, fmt.Errorf("onboarding recovery is missing runtime bundle manifest")
		}
		manifest, err := runtimebundle.ValidateManifestData(manifestWrite.Content, manifestWrite.SHA256, identity.Pack)
		if err != nil {
			return syncreview.ExclusiveInitPlan{}, err
		}
		recovery.BundleManifest.Files = len(manifest.Files) + 1
	}
	generated, err := generatedOnboardingWrites(ordinary.CaseRoot, identity, recovery, stamp, planSHA256)
	if err != nil {
		return syncreview.ExclusiveInitPlan{}, err
	}
	plan := ordinary
	plan.RepoRoot = recovery.RepoRoot
	plan.Writes = append([]syncreview.ExclusiveInitWrite{}, ordinary.Writes...)
	plan.Writes = append(plan.Writes, generated...)
	sort.Slice(plan.Writes, func(i, j int) bool {
		if plan.Writes[i].PublicationPhase != plan.Writes[j].PublicationPhase {
			return plan.Writes[i].PublicationPhase < plan.Writes[j].PublicationPhase
		}
		return plan.Writes[i].Path < plan.Writes[j].Path
	})
	return plan, nil
}

func recoveryWriteByKind(writes []missionintent.RecoveryWrite, kind string) (missionintent.RecoveryWrite, bool) {
	for _, write := range writes {
		if write.Kind == kind {
			return write, true
		}
	}
	return missionintent.RecoveryWrite{}, false
}

func hashExclusivePlan(plan syncreview.ExclusiveInitPlan, identity missionintent.Identity) (string, error) {
	caseRoot := plan.CaseRoot
	if identity.SchemaVersion == 2 {
		caseRoot = "."
	}
	value := missionintent.PlanHashInput{
		SchemaVersion: plan.SchemaVersion, Command: plan.Command, CaseRoot: caseRoot, RepoRoot: plan.RepoRoot,
		Pack: plan.Pack, ProjectName: plan.ProjectName, ProvisionID: plan.ProvisionID, Role: plan.Role, CreatedAt: plan.CreatedAt,
	}
	for _, write := range plan.Writes {
		hash := strings.ToLower(write.SHA256)
		size := write.Size
		if identity.SchemaVersion == 2 && write.Kind == "template-file" {
			canonical, err := missionintent.CanonicalRecoveryWriteAt(plan.CaseRoot, identity, missionintent.RecoveryWrite{
				Path: write.Path, Kind: write.Kind, SHA256: write.SHA256, Size: write.Size,
				Content: write.Content, PublicationPhase: write.PublicationPhase,
			})
			if err != nil {
				return "", err
			}
			hash = strings.ToLower(canonical.SHA256)
			size = canonical.Size
		}
		value.Writes = append(value.Writes, missionintent.PlanHashWrite{Path: write.Path, Kind: write.Kind, SHA256: hash, Size: size, PublicationPhase: write.PublicationPhase})
	}
	return missionintent.HashOnboardingPlan(value)
}

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(leftAbs), filepath.Clean(rightAbs))
	}
	return filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}

func normalizeIdentity(opt Options) (string, missionintent.Identity, error) {
	caseRoot, err := filepath.Abs(strings.TrimSpace(opt.Target))
	if err != nil || strings.TrimSpace(opt.Target) == "" {
		return "", missionintent.Identity{}, fmt.Errorf("onboard requires Target")
	}
	stateRoot, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return "", missionintent.Identity{}, err
	}
	identity := missionintent.Identity{
		SchemaVersion: 1,
		Target:        caseRoot,
		Pack:          strings.TrimSpace(opt.Pack),
		ProjectName:   strings.TrimSpace(opt.ProjectName),
		Goal:          opt.Goal,
		Actor:         strings.TrimSpace(opt.Actor),
		Executor:      strings.TrimSpace(opt.Executor),
		InitialLane:   strings.TrimSpace(opt.InitialLane),
	}
	projectID := strings.TrimSpace(opt.ProjectID)
	if stateRoot.Legacy {
		if projectID != "" {
			return "", missionintent.Identity{}, fmt.Errorf("legacy onboarding must not contain ProjectId")
		}
	} else {
		inspection, inspectErr := missionintent.Inspect(caseRoot)
		if inspectErr != nil {
			return "", missionintent.Identity{}, inspectErr
		}
		if inspection.State != "absent" && inspection.Identity.SchemaVersion == 1 && projectID == "" {
			// Existing current schema v1 generations remain readable at their
			// original physical root, but they do not become relocatable.
		} else {
			identity.SchemaVersion = 2
			identity.Target = "."
			if projectID == "" && inspection.State != "absent" && inspection.Identity.SchemaVersion == 2 {
				projectID = inspection.Identity.ProjectID
			}
			if projectID == "" {
				projectID, err = missionintent.GenerateProjectID()
				if err != nil {
					return "", missionintent.Identity{}, err
				}
			}
			identity.ProjectID = projectID
		}
	}
	if identity.SchemaVersion == 2 {
		err = missionintent.ValidateIdentityAt(caseRoot, identity)
	} else {
		err = missionintent.ValidateIdentity(identity)
	}
	if err != nil {
		return "", missionintent.Identity{}, err
	}
	if !utf8.ValidString(identity.Goal) {
		return "", missionintent.Identity{}, fmt.Errorf("onboard Goal must be valid UTF-8")
	}
	if len([]byte(identity.Goal)) > 4096 {
		return "", missionintent.Identity{}, fmt.Errorf("onboard Goal exceeds 4096 bytes")
	}
	for name, value := range map[string]string{"Pack": identity.Pack, "ProjectName": identity.ProjectName, "Actor": identity.Actor, "Executor": identity.Executor, "InitialLane": identity.InitialLane} {
		if !utf8.ValidString(value) {
			return "", missionintent.Identity{}, fmt.Errorf("onboard %s must be valid UTF-8", name)
		}
		if len([]byte(value)) > 256 {
			return "", missionintent.Identity{}, fmt.Errorf("onboard %s exceeds 256 bytes", name)
		}
	}
	return caseRoot, identity, nil
}

func validateInitialLane(repoRoot, pack, lane string) error {
	if !validInitialLane.MatchString(lane) {
		return fmt.Errorf("onboard InitialLane is not a valid lane selector: %q", lane)
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return err
	}
	if err := m.ValidateSchema(); err != nil {
		return err
	}
	defaultType := strings.TrimSpace(m.WorkstreamDefaults["defaultStartLaneType"])
	if _, err := m.LaneType(defaultType); err != nil {
		return fmt.Errorf("onboard InitialLane %q cannot be resolved by pack %q: %w", lane, pack, err)
	}
	for _, laneType := range m.LaneTypes {
		if lane == laneType.ID && laneType.Authority {
			return nil
		}
	}
	label, ok := laneid.Label(defaultType, lane)
	if ok && validInitialLane.MatchString(label) {
		return nil
	}
	return fmt.Errorf("onboard InitialLane %q cannot be generated exactly by pack %q default start lane type %q", lane, pack, defaultType)
}

func publicPlan(exclusive syncreview.ExclusiveInitPlan, identity missionintent.Identity, stamp, hash string, replay bool) Plan {
	writes := publicWrites(exclusive.Writes)
	args := applyArgs(exclusive.CaseRoot, identity, stamp, hash)
	return Plan{SchemaVersion: 1, Command: "onboard", CaseRoot: exclusive.CaseRoot, RepoRoot: exclusive.RepoRoot, ProjectID: identity.ProjectID, Pack: identity.Pack, ProjectName: identity.ProjectName, Goal: identity.Goal, Actor: identity.Actor, Executor: identity.Executor, InitialLane: identity.InitialLane, ReviewRequired: true, RequiresConfirmation: true, Replay: replay, PublicationStamp: stamp, OnboardingPlanSHA256: hash, ApplyCommand: applyCommand(args, identity.Goal), ApplyArgs: args, Writes: writes, BlockedActions: []string{"board creation", "lane creation", "goal inference", "authority/confirmed writes", "heavy-tool execution"}, NextSteps: []string{"review the exact mission intent and write set", "run applyArgs as the exact Apply request", "after commit, run status to use the durable onboarding route"}, Identity: identity, ExclusivePlan: exclusive}
}

func replayPlan(repoRoot, caseRoot string, inspection missionintent.Inspection) Plan {
	args := applyArgs(caseRoot, inspection.Identity, inspection.PublicationStamp, inspection.OnboardingPlanSHA256)
	return Plan{SchemaVersion: 1, Command: "onboard", CaseRoot: caseRoot, RepoRoot: repoRoot, ProjectID: inspection.Identity.ProjectID, Pack: inspection.Identity.Pack, ProjectName: inspection.Identity.ProjectName, Goal: inspection.Identity.Goal, Actor: inspection.Identity.Actor, Executor: inspection.Identity.Executor, InitialLane: inspection.Identity.InitialLane, ReviewRequired: true, RequiresConfirmation: true, Replay: true, PublicationStamp: inspection.PublicationStamp, OnboardingPlanSHA256: inspection.OnboardingPlanSHA256, ApplyCommand: applyCommand(args, inspection.Identity.Goal), ApplyArgs: args, BlockedActions: []string{"committed onboarding replacement", "board/lane mutation"}, NextSteps: []string{"onboarding is already committed; run status"}, Identity: inspection.Identity}
}

func resultFor(plan Plan, inspection missionintent.Inspection, replay bool) Result {
	return Result{SchemaVersion: 1, Command: "onboard", CaseRoot: plan.CaseRoot, ProjectID: plan.ProjectID, Pack: plan.Pack, ProjectName: plan.ProjectName, Applied: !replay, Replay: replay, PublicationStamp: plan.PublicationStamp, OnboardingPlanSHA256: plan.OnboardingPlanSHA256, ApplyCommand: plan.ApplyCommand, ApplyArgs: append([]string{}, plan.ApplyArgs...), Writes: plan.Writes, Inspection: inspection, NextSteps: []string{"run status and consume its single typed onboarding request", "create board/lanes only through the reviewed public onboarding actions"}}
}

func publicWrites(writes []syncreview.ExclusiveInitWrite) []Write {
	out := make([]Write, 0, len(writes))
	for _, write := range writes {
		out = append(out, Write{Path: write.Path, Kind: write.Kind, SHA256: write.SHA256, Size: write.Size, PublicationPhase: write.PublicationPhase})
	}
	return out
}

func applyArgs(caseRoot string, identity missionintent.Identity, stamp, hash string) []string {
	args := []string{"-Command", "onboard", "-Target", caseRoot}
	if identity.SchemaVersion == 2 {
		args = append(args, "-ProjectId", identity.ProjectID)
	}
	return append(args, "-Pack", identity.Pack, "-ProjectName", identity.ProjectName, "-Goal", identity.Goal, "-Actor", identity.Actor, "-Executor", identity.Executor, "-InitialLane", identity.InitialLane, "-OnboardingPublicationStamp", stamp, "-ExpectedOnboardingPlanSha256", hash, "-Apply", "-Format", "json")
}

func applyCommand(args []string, goal string) string {
	if strings.ContainsAny(goal, "\r\n") {
		return "opaque Goal is not safely representable as a single command string; execute applyArgs exactly"
	}
	parts := append([]string{"/rekit", "onboard"}, args[2:]...)
	for i := range parts {
		parts[i] = quoteCommandArg(parts[i])
	}
	return strings.Join(parts, " ")
}

func quoteCommandArg(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\"") {
		return value
	}
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func marshalMarkerCommit(stamp, missionSHA, intentSHA string) ([]byte, error) {
	value := missionintent.Commit{SchemaVersion: 1, Kind: "mission-onboarding-commit", PublicationStamp: stamp, OnboardingPlanSHA256: planMarker, MissionIntentSHA256: missionSHA, IntentSHA256: intentSHA}
	data, err := json.MarshalIndent(value, "", "  ")
	return append(data, '\n'), err
}
