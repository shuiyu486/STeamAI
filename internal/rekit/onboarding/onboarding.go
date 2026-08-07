package onboarding

import (
	"crypto/sha256"
	"encoding/hex"
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
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneid"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

const (
	planMarker          = "<onboarding-plan-sha256>"
	attachedPlanCommand = "attached-onboarding-adoption"
)

var (
	validInitialLane                 = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$`)
	attachedAdoptionBeforeCommitHook func() error
	attachedControlRoots             = []string{
		".rekit/board.json",
		".rekit/policy.yml",
		".rekit/verification-role.json",
		".rekit/backups",
		".rekit/lanes",
		".rekit/facts",
		".rekit/runs",
		".rekit/handovers",
		".rekit/reviews",
		".rekit/reviewer-adoptions",
		".rekit/reopen-evidence",
		".rekit/reopen-operations",
		".rekit/member-executions",
		".rekit/external-session-attempts",
		".rekit/external-session-attempt-inputs",
		".rekit/external-session-dispatch",
		".rekit/external-session-jobs",
		".rekit/external-session-observations",
		".rekit/external-session-relays",
		".rekit/pack-memory",
		".rekit/session-host",
	}
)

type Options struct {
	Target                       string
	Pack                         string
	ProjectName                  string
	Goal                         string
	Actor                        string
	Executor                     string
	InitialLane                  string
	PublicationStamp             string
	ExpectedOnboardingPlanSHA256 string
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
	return build(repoRoot, opt, false)
}

func Apply(repoRoot string, opt Options) (Result, error) {
	if strings.TrimSpace(opt.PublicationStamp) == "" || strings.TrimSpace(opt.ExpectedOnboardingPlanSHA256) == "" {
		return Result{}, fmt.Errorf("onboard -Apply requires -OnboardingPublicationStamp and -ExpectedOnboardingPlanSha256 from the exact preview")
	}
	plan, err := build(repoRoot, opt, true)
	if err != nil {
		return Result{}, err
	}
	if !strings.EqualFold(opt.ExpectedOnboardingPlanSHA256, plan.OnboardingPlanSHA256) {
		return Result{}, fmt.Errorf("onboarding plan hash mismatch: expected %s current %s", opt.ExpectedOnboardingPlanSHA256, plan.OnboardingPlanSHA256)
	}
	if plan.ExclusivePlan.Command == attachedPlanCommand {
		return applyAttachedAdoption(plan)
	}
	if !plan.Replay && !plan.DurableRecovery {
		createdAt, err := time.Parse(time.RFC3339Nano, plan.ExclusivePlan.CreatedAt)
		if err != nil {
			return Result{}, err
		}
		ordinary, err := ordinaryPlan(plan.RepoRoot, plan.Identity, plan.PublicationStamp, createdAt, true)
		if err != nil {
			return Result{}, err
		}
		marker, err := planFromOrdinary(ordinary, plan.Identity, plan.PublicationStamp, planMarker)
		if err != nil {
			return Result{}, err
		}
		currentHash, err := hashExclusivePlan(marker)
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
	identity, err := normalizeIdentity(opt)
	if err != nil {
		return Plan{}, err
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return Plan{}, err
	}
	if existing, statErr := os.Lstat(identity.Target); statErr == nil {
		if !existing.IsDir() || existing.Mode()&os.ModeSymlink != 0 {
			return Plan{}, fmt.Errorf("onboard target is not a regular directory: %s", identity.Target)
		}
		inspection, inspectErr := missionintent.Inspect(identity.Target)
		if inspectErr != nil {
			return Plan{}, inspectErr
		}
		if inspection.State == "absent" {
			return buildAttachedAdoption(repoRoot, identity, opt)
		}
		if inspection.Identity != identity {
			return Plan{}, fmt.Errorf("onboard identity differs from the immutable existing mission intent")
		}
		if inspection.Committed {
			if !samePath(inspection.Recovery.RepoRoot, repoRoot) {
				return Plan{}, fmt.Errorf("committed onboarding belongs to a different canonical kit root: %s", inspection.Recovery.RepoRoot)
			}
			if opt.PublicationStamp != "" && opt.PublicationStamp != inspection.PublicationStamp {
				return Plan{}, fmt.Errorf("onboarding publication stamp differs from committed identity")
			}
			if opt.ExpectedOnboardingPlanSHA256 != "" && !strings.EqualFold(opt.ExpectedOnboardingPlanSHA256, inspection.OnboardingPlanSHA256) {
				return Plan{}, fmt.Errorf("onboarding plan hash differs from committed identity")
			}
			return replayPlan(repoRoot, inspection), nil
		}
		if !allowExisting && opt.PublicationStamp == "" {
			return Plan{}, fmt.Errorf("onboarding publication is pending; rerun the exact Apply command with stamp %s and plan hash %s", inspection.PublicationStamp, inspection.OnboardingPlanSHA256)
		}
		if opt.PublicationStamp != inspection.PublicationStamp || (opt.ExpectedOnboardingPlanSHA256 != "" && !strings.EqualFold(opt.ExpectedOnboardingPlanSHA256, inspection.OnboardingPlanSHA256)) {
			return Plan{}, fmt.Errorf("pending onboarding requires the exact publication stamp and plan hash")
		}
		if !samePath(inspection.Recovery.RepoRoot, repoRoot) {
			return Plan{}, fmt.Errorf("pending onboarding recovery is bound to a different canonical kit root: %s", inspection.Recovery.RepoRoot)
		}
		if inspection.Recovery.Mode == "attached-adoption" {
			if err := missionintent.ValidateRecoveryEnvelope(identity, inspection.Recovery); err != nil {
				return Plan{}, err
			}
			createdAt, err := time.Parse(time.RFC3339Nano, inspection.Recovery.CreatedAt)
			if err != nil {
				return Plan{}, err
			}
			marker, err := attachedPlan(repoRoot, identity, inspection.PublicationStamp, createdAt, inspection.Recovery.AttachedSnapshot, planMarker)
			if err != nil {
				return Plan{}, err
			}
			hash, err := hashExclusivePlan(marker)
			if err != nil || !strings.EqualFold(hash, inspection.OnboardingPlanSHA256) {
				return Plan{}, fmt.Errorf("durable attached onboarding recovery envelope does not reconstruct the reviewed plan hash")
			}
			exact, err := attachedPlan(repoRoot, identity, inspection.PublicationStamp, createdAt, inspection.Recovery.AttachedSnapshot, inspection.OnboardingPlanSHA256)
			if err != nil {
				return Plan{}, err
			}
			plan := publicPlan(exact, identity, inspection.PublicationStamp, inspection.OnboardingPlanSHA256, false)
			plan.DurableRecovery = true
			return plan, nil
		}
		ordinary, err := ordinaryPlanFromRecovery(identity, inspection.Recovery)
		if err != nil {
			return Plan{}, err
		}
		marker, err := planFromOrdinary(ordinary, identity, inspection.PublicationStamp, planMarker)
		if err != nil {
			return Plan{}, err
		}
		hash, err := hashExclusivePlan(marker)
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
	ordinary, err := ordinaryPlan(repoRoot, identity, stamp, createdAt, allowExisting)
	if err != nil {
		return Plan{}, err
	}
	placeholder, err := planFromOrdinary(ordinary, identity, stamp, planMarker)
	if err != nil {
		return Plan{}, err
	}
	planSHA256, err := hashExclusivePlan(placeholder)
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

func buildAttachedAdoption(repoRoot string, identity missionintent.Identity, opt Options) (Plan, error) {
	if _, err := instance.AssertAttached(identity.Target, repoRoot, identity.Pack); err != nil {
		return Plan{}, fmt.Errorf("onboard attached adoption requires a current attached case: %w", err)
	}
	if err := validateInitialLane(repoRoot, identity.Pack, identity.InitialLane); err != nil {
		return Plan{}, err
	}
	if err := ensureAttachedMissionControlEmpty(identity.Target); err != nil {
		return Plan{}, err
	}
	rows, err := casehealth.Static(repoRoot, identity.Target, identity.Pack)
	if err != nil {
		return Plan{}, fmt.Errorf("onboard attached adoption requires doctor-ready case files: %w", err)
	}
	stamp, createdAt, err := onboardingStamp(opt.PublicationStamp)
	if err != nil {
		return Plan{}, err
	}
	snapshot, err := attachedSnapshot(identity.Target, rows)
	if err != nil {
		return Plan{}, err
	}
	if opt.PublicationStamp != "" && strings.TrimSpace(opt.ExpectedOnboardingPlanSHA256) == "" {
		return Plan{}, fmt.Errorf("attached onboarding Apply requires the exact reviewed plan hash")
	}
	marker, err := attachedPlan(repoRoot, identity, stamp, createdAt, snapshot, planMarker)
	if err != nil {
		return Plan{}, err
	}
	hash, err := hashExclusivePlan(marker)
	if err != nil {
		return Plan{}, err
	}
	if expected := strings.TrimSpace(opt.ExpectedOnboardingPlanSHA256); expected != "" && !strings.EqualFold(expected, hash) {
		return Plan{}, fmt.Errorf("attached onboarding snapshot changed after reviewed preview")
	}
	exact, err := attachedPlan(repoRoot, identity, stamp, createdAt, snapshot, hash)
	if err != nil {
		return Plan{}, err
	}
	return publicPlan(exact, identity, stamp, hash, false), nil
}

func attachedPlan(repoRoot string, identity missionintent.Identity, stamp string, createdAt time.Time, snapshot []missionintent.SnapshotArtifact, hash string) (syncreview.ExclusiveInitPlan, error) {
	recovery := missionintent.RecoveryEnvelope{SchemaVersion: 1, RepoRoot: repoRoot, CreatedAt: createdAt.UTC().Format(time.RFC3339Nano), Mode: "attached-adoption", AttachedSnapshot: append([]missionintent.SnapshotArtifact{}, snapshot...)}
	if err := missionintent.ValidateRecoveryEnvelope(identity, recovery); err != nil {
		return syncreview.ExclusiveInitPlan{}, err
	}
	base := syncreview.ExclusiveInitPlan{SchemaVersion: 1, Command: attachedPlanCommand, CaseRoot: identity.Target, RepoRoot: repoRoot, Pack: identity.Pack, ProjectName: identity.ProjectName, ProvisionID: "onboarding-" + stamp, Role: "mission-onboarding-adoption", CreatedAt: recovery.CreatedAt, BlockedActions: []string{"existing case content writes", "existing Mission Control takeover", "overwrite", "backup", "force", "authority/confirmed writes", "heavy-tool execution"}}
	missionBytes, err := missionintent.MarshalMissionIntent(identity)
	if err != nil {
		return syncreview.ExclusiveInitPlan{}, err
	}
	intentBytes, err := missionintent.MarshalIntent(missionintent.Intent{SchemaVersion: 1, Kind: "mission-onboarding-intent", PublicationStamp: stamp, OnboardingPlanSHA256: hash, Identity: identity, Recovery: recovery})
	if err != nil {
		return syncreview.ExclusiveInitPlan{}, err
	}
	commit := missionintent.Commit{SchemaVersion: 1, Kind: "mission-onboarding-commit", PublicationStamp: stamp, OnboardingPlanSHA256: hash, MissionIntentSHA256: missionintent.SHA256(missionBytes), IntentSHA256: missionintent.SHA256(intentBytes)}
	commitBytes, err := missionintent.MarshalCommit(commit)
	if err != nil && hash == planMarker {
		commitBytes, err = marshalMarkerCommit(stamp, commit.MissionIntentSHA256, commit.IntentSHA256)
	}
	if err != nil {
		return syncreview.ExclusiveInitPlan{}, err
	}
	for _, generated := range []struct {
		path, kind string
		content    []byte
		phase      int
	}{{missionintent.IntentRel, "onboarding-intent", intentBytes, 0}, {missionintent.MissionIntentRel, "mission-intent", missionBytes, 1}, {missionintent.CommitRel, "onboarding-commit", commitBytes, 2}} {
		base.Writes = append(base.Writes, syncreview.ExclusiveInitWrite{Path: generated.path, Kind: generated.kind, TargetPath: filepath.Join(identity.Target, filepath.FromSlash(generated.path)), SHA256: missionintent.SHA256(generated.content), Size: int64(len(generated.content)), Content: generated.content, PublicationPhase: generated.phase})
	}
	return base, nil
}

func attachedSnapshot(caseRoot string, rows []casehealth.Row) ([]missionintent.SnapshotArtifact, error) {
	paths := map[string]string{
		".rekit/instance.yml":           "instance-metadata",
		".claude/skills/rekit/SKILL.md": "case-local-thin-shim",
		".re-template.yml":              "legacy-metadata",
		".rekit/state.json":             "sync-state",
	}
	for _, row := range rows {
		rel, err := filepath.Rel(caseRoot, row.File)
		if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		path := filepath.ToSlash(filepath.Clean(rel))
		if _, fixed := paths[path]; !fixed {
			paths[path] = "doctor-validated-artifact"
		}
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
	for _, rel := range attachedControlRoots {
		if _, err := os.Lstat(filepath.Join(caseRoot, filepath.FromSlash(rel))); err == nil {
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
		if _, err := instance.AssertAttached(plan.CaseRoot, plan.RepoRoot, plan.Pack); err != nil {
			return Result{}, err
		}
	}
	if err := ensureAttachedMissionControlEmpty(plan.CaseRoot); err != nil {
		return Result{}, err
	}
	if err := validateAttachedSnapshot(plan.CaseRoot, recoverySnapshot(plan.ExclusivePlan)); err != nil {
		return Result{}, err
	}
	snapshot := recoverySnapshot(plan.ExclusivePlan)
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

func recoverySnapshot(plan syncreview.ExclusiveInitPlan) []missionintent.SnapshotArtifact {
	for _, write := range plan.Writes {
		if write.Path != missionintent.IntentRel {
			continue
		}
		var intent missionintent.Intent
		if json.Unmarshal(write.Content, &intent) == nil {
			return append([]missionintent.SnapshotArtifact{}, intent.Recovery.AttachedSnapshot...)
		}
	}
	return nil
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

func ordinaryPlan(repoRoot string, identity missionintent.Identity, stamp string, createdAt time.Time, allowExisting bool) (syncreview.ExclusiveInitPlan, error) {
	options := syncreview.ExclusiveInitOptions{ProjectName: identity.ProjectName, ProvisionID: "onboarding-" + stamp, Role: "mission-onboarding", CreatedAt: createdAt, SkipVerificationMarker: true, DefaultPublicationPhase: 1}
	if allowExisting {
		return syncreview.PlanExclusiveInitReplay(repoRoot, identity.Target, identity.Pack, options)
	}
	return syncreview.PlanExclusiveInit(repoRoot, identity.Target, identity.Pack, options)
}

func ordinaryPlanFromRecovery(identity missionintent.Identity, recovery missionintent.RecoveryEnvelope) (syncreview.ExclusiveInitPlan, error) {
	if err := missionintent.ValidateRecoveryEnvelope(identity, recovery); err != nil {
		return syncreview.ExclusiveInitPlan{}, err
	}
	plan := syncreview.ExclusiveInitPlan{SchemaVersion: 1, Command: "exclusive-init", CaseRoot: identity.Target, RepoRoot: recovery.RepoRoot, Pack: identity.Pack, ProjectName: identity.ProjectName, ProvisionID: "onboarding-" + strings.ReplaceAll(strings.ReplaceAll(recovery.CreatedAt, "-", ""), ":", ""), Role: "mission-onboarding", CreatedAt: recovery.CreatedAt, BlockedActions: []string{"existing root takeover", "overwrite", "backup", "force", "authority/confirmed writes", "heavy-tool execution"}}
	stampTime, err := time.Parse(time.RFC3339Nano, recovery.CreatedAt)
	if err != nil {
		return syncreview.ExclusiveInitPlan{}, err
	}
	plan.ProvisionID = "onboarding-" + stampTime.UTC().Format("20060102-150405000")
	for _, write := range recovery.Writes {
		plan.Writes = append(plan.Writes, syncreview.ExclusiveInitWrite{Path: write.Path, Kind: write.Kind, TargetPath: filepath.Join(identity.Target, filepath.FromSlash(write.Path)), SHA256: write.SHA256, Size: write.Size, Content: append([]byte{}, write.Content...), PublicationPhase: write.PublicationPhase})
	}
	return plan, nil
}

func planFromOrdinary(ordinary syncreview.ExclusiveInitPlan, identity missionintent.Identity, stamp, planSHA256 string) (syncreview.ExclusiveInitPlan, error) {
	recovery := missionintent.RecoveryEnvelope{SchemaVersion: 1, RepoRoot: ordinary.RepoRoot, CreatedAt: ordinary.CreatedAt}
	for _, write := range ordinary.Writes {
		recovery.Writes = append(recovery.Writes, missionintent.RecoveryWrite{Path: write.Path, Kind: write.Kind, SHA256: write.SHA256, Size: write.Size, Content: append([]byte{}, write.Content...), PublicationPhase: write.PublicationPhase})
	}
	missionBytes, err := missionintent.MarshalMissionIntent(identity)
	if err != nil {
		return syncreview.ExclusiveInitPlan{}, err
	}
	intentBytes, err := missionintent.MarshalIntent(missionintent.Intent{SchemaVersion: 1, Kind: "mission-onboarding-intent", PublicationStamp: stamp, OnboardingPlanSHA256: planSHA256, Identity: identity, Recovery: recovery})
	if err != nil {
		return syncreview.ExclusiveInitPlan{}, err
	}
	commit := missionintent.Commit{SchemaVersion: 1, Kind: "mission-onboarding-commit", PublicationStamp: stamp, OnboardingPlanSHA256: planSHA256, MissionIntentSHA256: missionintent.SHA256(missionBytes), IntentSHA256: missionintent.SHA256(intentBytes)}
	commitBytes, err := missionintent.MarshalCommit(commit)
	if err != nil && planSHA256 == planMarker {
		commitBytes, err = marshalMarkerCommit(stamp, commit.MissionIntentSHA256, commit.IntentSHA256)
	}
	if err != nil {
		return syncreview.ExclusiveInitPlan{}, err
	}
	plan := ordinary
	plan.Writes = append([]syncreview.ExclusiveInitWrite{}, ordinary.Writes...)
	for _, generated := range []struct {
		path, kind string
		content    []byte
		phase      int
	}{{missionintent.IntentRel, "onboarding-intent", intentBytes, 0}, {missionintent.MissionIntentRel, "mission-intent", missionBytes, 1}, {missionintent.CommitRel, "onboarding-commit", commitBytes, 2}} {
		sum := sha256.Sum256(generated.content)
		plan.Writes = append(plan.Writes, syncreview.ExclusiveInitWrite{Path: generated.path, Kind: generated.kind, TargetPath: filepath.Join(identity.Target, filepath.FromSlash(generated.path)), SHA256: hex.EncodeToString(sum[:]), Size: int64(len(generated.content)), Content: generated.content, PublicationPhase: generated.phase})
	}
	sort.Slice(plan.Writes, func(i, j int) bool {
		if plan.Writes[i].PublicationPhase != plan.Writes[j].PublicationPhase {
			return plan.Writes[i].PublicationPhase < plan.Writes[j].PublicationPhase
		}
		return plan.Writes[i].Path < plan.Writes[j].Path
	})
	return plan, nil
}

func hashExclusivePlan(plan syncreview.ExclusiveInitPlan) (string, error) {
	type hashWrite struct {
		Path             string `json:"path"`
		Kind             string `json:"kind"`
		Content          string `json:"content"`
		PublicationPhase int    `json:"publicationPhase"`
	}
	value := struct {
		SchemaVersion int         `json:"schemaVersion"`
		Command       string      `json:"command"`
		CaseRoot      string      `json:"caseRoot"`
		RepoRoot      string      `json:"repoRoot"`
		Pack          string      `json:"pack"`
		ProjectName   string      `json:"projectName"`
		ProvisionID   string      `json:"provisionId"`
		Role          string      `json:"role"`
		CreatedAt     string      `json:"createdAt"`
		Writes        []hashWrite `json:"writes"`
	}{plan.SchemaVersion, plan.Command, plan.CaseRoot, plan.RepoRoot, plan.Pack, plan.ProjectName, plan.ProvisionID, plan.Role, plan.CreatedAt, nil}
	for _, write := range plan.Writes {
		value.Writes = append(value.Writes, hashWrite{write.Path, write.Kind, string(write.Content), write.PublicationPhase})
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
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

func normalizeIdentity(opt Options) (missionintent.Identity, error) {
	target, err := filepath.Abs(strings.TrimSpace(opt.Target))
	if err != nil || strings.TrimSpace(opt.Target) == "" {
		return missionintent.Identity{}, fmt.Errorf("onboard requires Target")
	}
	identity := missionintent.Identity{SchemaVersion: 1, Target: target, Pack: strings.TrimSpace(opt.Pack), ProjectName: strings.TrimSpace(opt.ProjectName), Goal: opt.Goal, Actor: strings.TrimSpace(opt.Actor), Executor: strings.TrimSpace(opt.Executor), InitialLane: strings.TrimSpace(opt.InitialLane)}
	if err := missionintent.ValidateIdentity(identity); err != nil {
		return missionintent.Identity{}, err
	}
	if !utf8.ValidString(identity.Goal) {
		return missionintent.Identity{}, fmt.Errorf("onboard Goal must be valid UTF-8")
	}
	if len([]byte(identity.Goal)) > 4096 {
		return missionintent.Identity{}, fmt.Errorf("onboard Goal exceeds 4096 bytes")
	}
	for name, value := range map[string]string{"Pack": identity.Pack, "ProjectName": identity.ProjectName, "Actor": identity.Actor, "Executor": identity.Executor, "InitialLane": identity.InitialLane} {
		if !utf8.ValidString(value) {
			return missionintent.Identity{}, fmt.Errorf("onboard %s must be valid UTF-8", name)
		}
		if len([]byte(value)) > 256 {
			return missionintent.Identity{}, fmt.Errorf("onboard %s exceeds 256 bytes", name)
		}
	}
	return identity, nil
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
	args := applyArgs(identity, stamp, hash)
	return Plan{SchemaVersion: 1, Command: "onboard", CaseRoot: identity.Target, RepoRoot: exclusive.RepoRoot, Pack: identity.Pack, ProjectName: identity.ProjectName, Goal: identity.Goal, Actor: identity.Actor, Executor: identity.Executor, InitialLane: identity.InitialLane, ReviewRequired: true, RequiresConfirmation: true, Replay: replay, PublicationStamp: stamp, OnboardingPlanSHA256: hash, ApplyCommand: applyCommand(args, identity.Goal), ApplyArgs: args, Writes: writes, BlockedActions: []string{"board creation", "lane creation", "goal inference", "authority/confirmed writes", "heavy-tool execution"}, NextSteps: []string{"review the exact mission intent and write set", "run applyArgs as the exact Apply request", "after commit, run status to use the durable onboarding route"}, Identity: identity, ExclusivePlan: exclusive}
}

func replayPlan(repoRoot string, inspection missionintent.Inspection) Plan {
	args := applyArgs(inspection.Identity, inspection.PublicationStamp, inspection.OnboardingPlanSHA256)
	return Plan{SchemaVersion: 1, Command: "onboard", CaseRoot: inspection.Identity.Target, RepoRoot: repoRoot, Pack: inspection.Identity.Pack, ProjectName: inspection.Identity.ProjectName, Goal: inspection.Identity.Goal, Actor: inspection.Identity.Actor, Executor: inspection.Identity.Executor, InitialLane: inspection.Identity.InitialLane, ReviewRequired: true, RequiresConfirmation: true, Replay: true, PublicationStamp: inspection.PublicationStamp, OnboardingPlanSHA256: inspection.OnboardingPlanSHA256, ApplyCommand: applyCommand(args, inspection.Identity.Goal), ApplyArgs: args, BlockedActions: []string{"committed onboarding replacement", "board/lane mutation"}, NextSteps: []string{"onboarding is already committed; run status"}, Identity: inspection.Identity}
}

func resultFor(plan Plan, inspection missionintent.Inspection, replay bool) Result {
	return Result{SchemaVersion: 1, Command: "onboard", CaseRoot: plan.CaseRoot, Pack: plan.Pack, ProjectName: plan.ProjectName, Applied: !replay, Replay: replay, PublicationStamp: plan.PublicationStamp, OnboardingPlanSHA256: plan.OnboardingPlanSHA256, ApplyCommand: plan.ApplyCommand, ApplyArgs: append([]string{}, plan.ApplyArgs...), Writes: plan.Writes, Inspection: inspection, NextSteps: []string{"run status and consume its single typed onboarding request", "create board/lanes only through the reviewed public onboarding actions"}}
}

func publicWrites(writes []syncreview.ExclusiveInitWrite) []Write {
	out := make([]Write, 0, len(writes))
	for _, write := range writes {
		out = append(out, Write{Path: write.Path, Kind: write.Kind, SHA256: write.SHA256, Size: write.Size, PublicationPhase: write.PublicationPhase})
	}
	return out
}

func applyArgs(identity missionintent.Identity, stamp, hash string) []string {
	return []string{"-Command", "onboard", "-Target", identity.Target, "-Pack", identity.Pack, "-ProjectName", identity.ProjectName, "-Goal", identity.Goal, "-Actor", identity.Actor, "-Executor", identity.Executor, "-InitialLane", identity.InitialLane, "-OnboardingPublicationStamp", stamp, "-ExpectedOnboardingPlanSha256", hash, "-Apply", "-Format", "json"}
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
