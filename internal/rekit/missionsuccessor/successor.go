package missionsuccessor

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
	"strings"
	"time"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

const (
	transitionRoot  = "transitions/successor"
	maxClosureFile  = int64(32 << 20)
	maxClosureFiles = 8192
)

var applyAfterPublicationHook func(Write) error

// SetApplyAfterPublicationHookForTest injects a cut point after one durable
// successor publication. It is test-only and must not be used by production owners.
func SetApplyAfterPublicationHookForTest(hook func(Write) error) func() {
	previous := applyAfterPublicationHook
	applyAfterPublicationHook = hook
	return func() { applyAfterPublicationHook = previous }
}

type Options struct {
	Goal               string
	Actor              string
	PublicationStamp   string
	ExpectedPlanSHA256 string
}

type Write struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
	Phase  string `json:"phase"`
}

type Plan struct {
	SchemaVersion        int                                 `json:"schemaVersion"`
	Kind                 string                              `json:"kind"`
	Command              string                              `json:"command"`
	State                string                              `json:"state"`
	CaseRoot             string                              `json:"caseRoot"`
	ProjectID            string                              `json:"projectId"`
	Pack                 string                              `json:"pack"`
	PackManifestSHA256   string                              `json:"packManifestSha256"`
	AuthorityLane        string                              `json:"authorityLane"`
	PreviousGeneration   int                                 `json:"previousGeneration"`
	Generation           int                                 `json:"generation"`
	MissionID            string                              `json:"missionId"`
	Goal                 string                              `json:"goal"`
	Actor                string                              `json:"actor"`
	InitialLane          string                              `json:"initialLane"`
	Executor             string                              `json:"executor"`
	PublicationStamp     string                              `json:"publicationStamp"`
	PreviousMissionSHA   string                              `json:"previousMissionSha256"`
	PreviousClosureSHA   string                              `json:"previousClosureSha256"`
	PreviousCompletion   workstream.MissionCompletionHandoff `json:"previousCompletion"`
	Writes               []Write                             `json:"writes"`
	ExpectedPlanSHA256   string                              `json:"expectedPlanSha256"`
	ApplyArgs            []string                            `json:"applyArgs"`
	RequiresReview       bool                                `json:"requiresReview"`
	RequiresConfirmation bool                                `json:"requiresConfirmation"`
	NoAuthority          bool                                `json:"noAuthority"`
	NoConfirmed          bool                                `json:"noConfirmed"`
	NoHeavyTool          bool                                `json:"noHeavyTool"`
	NoAutoResume         bool                                `json:"noAutoResume"`
}

type Result struct {
	Plan
	Applied bool `json:"applied"`
	Replay  bool `json:"replay"`
}

type TransitionIntent struct {
	SchemaVersion      int                                 `json:"schemaVersion"`
	Kind               string                              `json:"kind"`
	TransitionID       string                              `json:"transitionId"`
	PublicationStamp   string                              `json:"publicationStamp"`
	ProjectID          string                              `json:"projectId"`
	Pack               string                              `json:"pack"`
	PreviousGeneration int                                 `json:"previousGeneration"`
	Generation         int                                 `json:"generation"`
	MissionID          string                              `json:"missionId"`
	Goal               string                              `json:"goal"`
	Actor              string                              `json:"actor"`
	InitialLane        string                              `json:"initialLane"`
	Executor           string                              `json:"executor"`
	PreviousMissionSHA string                              `json:"previousMissionSha256"`
	PreviousClosureSHA string                              `json:"previousClosureSha256"`
	PreviousCompletion workstream.MissionCompletionHandoff `json:"previousCompletion"`
	PlanSHA256         string                              `json:"planSha256"`
	NoAuthority        bool                                `json:"noAuthority"`
	NoConfirmed        bool                                `json:"noConfirmed"`
	NoHeavyTool        bool                                `json:"noHeavyTool"`
	NoAutoResume       bool                                `json:"noAutoResume"`
}

type GenerationManifest struct {
	SchemaVersion      int    `json:"schemaVersion"`
	Kind               string `json:"kind"`
	ProjectID          string `json:"projectId"`
	Pack               string `json:"pack"`
	PackManifestSHA256 string `json:"packManifestSha256"`
	AuthorityLane      string `json:"authorityLane"`
	Generation         int    `json:"generation"`
	MissionID          string `json:"missionId"`
	MissionIntentSHA   string `json:"missionIntentSha256"`
	PreviousClosureSHA string `json:"previousClosureSha256"`
	TransitionID       string `json:"transitionId"`
	PlanSHA256         string `json:"planSha256"`
	NoAuthority        bool   `json:"noAuthority"`
	NoConfirmed        bool   `json:"noConfirmed"`
	NoHeavyTool        bool   `json:"noHeavyTool"`
}

type GenerationCommit struct {
	SchemaVersion  int    `json:"schemaVersion"`
	Kind           string `json:"kind"`
	State          string `json:"state"`
	Generation     int    `json:"generation"`
	MissionID      string `json:"missionId"`
	ManifestSHA256 string `json:"manifestSha256"`
	PlanSHA256     string `json:"planSha256"`
}

type TransitionCommit struct {
	SchemaVersion  int    `json:"schemaVersion"`
	Kind           string `json:"kind"`
	State          string `json:"state"`
	TransitionID   string `json:"transitionId"`
	PlanSHA256     string `json:"planSha256"`
	IntentSHA256   string `json:"intentSha256"`
	Generation     int    `json:"generation"`
	MissionID      string `json:"missionId"`
	ManifestSHA256 string `json:"manifestSha256"`
	CommittedAt    string `json:"committedAt"`
	NoAuthority    bool   `json:"noAuthority"`
	NoConfirmed    bool   `json:"noConfirmed"`
	NoHeavyTool    bool   `json:"noHeavyTool"`
	NoAutoResume   bool   `json:"noAutoResume"`
}

type prepared struct {
	plan             Plan
	transitionID     string
	intentBytes      []byte
	missionBytes     []byte
	manifestBytes    []byte
	generationCommit []byte
	transitionCommit []byte
	activeBytes      []byte
	boardBytes       []byte
	view             projectstate.MissionView
}

func Preview(caseRoot string, opt Options) (Plan, error) {
	prepared, err := prepare(caseRoot, opt, true)
	if err != nil {
		return Plan{}, err
	}
	return prepared.plan, nil
}

func Apply(caseRoot string, opt Options) (Result, error) {
	return ApplyWithLease(caseRoot, opt, nil)
}

func ApplyWithLease(caseRoot string, opt Options, lease *projectexecution.Lease) (Result, error) {
	if !validSHA(opt.ExpectedPlanSHA256) {
		return Result{}, fmt.Errorf("successor mission Apply requires the exact reviewed plan SHA-256")
	}
	owned := false
	var err error
	if lease == nil {
		lease, err = projectexecution.AcquireExclusive(caseRoot)
		if err != nil {
			return Result{}, err
		}
		owned = true
	} else if err := lease.ValidateExclusiveFor(caseRoot); err != nil {
		return Result{}, err
	}
	if owned {
		defer lease.Unlock()
	}
	if replay, ok, replayErr := committedReplay(caseRoot, opt); replayErr != nil || ok {
		if replayErr != nil || !ok {
			return replay, replayErr
		}
		if err := retireCommittedActivePredecessors(caseRoot, replay.Generation); err != nil {
			return Result{}, err
		}
		return replay, nil
	}
	if err := recoverInterruptedActiveReplacement(caseRoot, opt.ExpectedPlanSHA256); err != nil {
		return Result{}, err
	}
	if replay, ok, replayErr := committedReplay(caseRoot, opt); replayErr != nil || ok {
		return replay, replayErr
	}
	prepared, err := prepare(caseRoot, opt, false)
	if err != nil {
		return Result{}, err
	}
	if !strings.EqualFold(prepared.plan.ExpectedPlanSHA256, opt.ExpectedPlanSHA256) {
		return Result{}, fmt.Errorf("successor mission plan changed after review")
	}
	if err := validatePendingTransitionRecovery(
		caseRoot,
		prepared.plan.PreviousGeneration,
		prepared.transitionID,
		prepared.plan.ExpectedPlanSHA256,
	); err != nil {
		return Result{}, err
	}
	if err := prepared.view.ValidateCurrent(caseRoot); err != nil {
		return Result{}, fmt.Errorf("successor mission active view changed after review: %w", err)
	}
	if prepared.plan.PreviousGeneration > 1 && !rekitfs.HandleBoundExactMutationSupported() {
		return Result{}, fmt.Errorf("successor activation requires handle-bound exact filesystem mutation")
	}
	activePath, err := projectstate.MissionActivePath(caseRoot)
	if err != nil {
		return Result{}, err
	}
	var previousActive []byte
	if prepared.plan.PreviousGeneration > 1 {
		previousActive, err = rekitfs.ReadStableRegularFileAnchored(
			prepared.view.Root.Path,
			activePath,
			"previous active mission pointer",
			64<<10,
		)
		if err != nil {
			return Result{}, err
		}
	} else if active, readErr := rekitfs.ReadStableRegularFileAnchored(
		prepared.view.Root.Path,
		activePath,
		"existing active mission pointer",
		64<<10,
	); readErr == nil {
		if bytes.Equal(active, prepared.activeBytes) {
			prepared.plan.State = "ready-to-continue"
			prepared.plan.RequiresConfirmation = false
			return Result{Plan: prepared.plan, Applied: true, Replay: true}, nil
		}
		return Result{}, fmt.Errorf("active mission pointer differs from the reviewed successor")
	} else if _, statErr := os.Lstat(activePath); !os.IsNotExist(statErr) {
		return Result{}, readErr
	}
	publications := []struct {
		path  string
		label string
		data  []byte
	}{
		{transitionPath(prepared.transitionID, "intent.json"), "successor transition intent", prepared.intentBytes},
		{generationPath(prepared.plan.Generation, "mission-intent.json"), "successor mission intent", prepared.missionBytes},
		{generationPath(prepared.plan.Generation, "board.json"), "successor board", prepared.boardBytes},
	}
	for _, name := range missionFactNames() {
		publications = append(publications, struct {
			path, label string
			data        []byte
		}{
			generationPath(prepared.plan.Generation, filepath.ToSlash(filepath.Join("facts", name))),
			"successor fact ledger",
			[]byte("\n"),
		})
	}
	publications = append(publications,
		struct {
			path, label string
			data        []byte
		}{generationPath(prepared.plan.Generation, projectstate.MissionManifestFile), "successor manifest", prepared.manifestBytes},
		struct {
			path, label string
			data        []byte
		}{generationPath(prepared.plan.Generation, "commit.json"), "successor generation commit", prepared.generationCommit},
		struct {
			path, label string
			data        []byte
		}{transitionPath(prepared.transitionID, "commit.json"), "successor transition commit", prepared.transitionCommit},
	)
	for _, publication := range publications {
		if err := validatePlannedWrite(prepared.plan.Writes, publication.path, publication.data); err != nil {
			return Result{}, err
		}
	}
	activeRel := stateRel(caseRoot, activePath)
	if err := validatePlannedWrite(prepared.plan.Writes, activeRel, prepared.activeBytes); err != nil {
		return Result{}, err
	}
	if err := prepared.view.ValidateCurrent(caseRoot); err != nil {
		return Result{}, fmt.Errorf("successor mission active view changed before publication: %w", err)
	}
	for _, publication := range publications {
		if _, err := rekitfs.WriteAtomicNoReplaceRegularFileAnchored(caseRoot, publication.path, publication.label, publication.data); err != nil {
			return Result{}, err
		}
		if applyAfterPublicationHook != nil {
			write := Write{Path: publication.path, SHA256: sha(publication.data), Size: int64(len(publication.data))}
			if err := applyAfterPublicationHook(write); err != nil {
				return Result{}, fmt.Errorf("successor publication may already be durable at %s: %w", publication.path, err)
			}
		}
	}
	if prepared.plan.PreviousGeneration == 1 {
		if _, err := rekitfs.WriteAtomicNoReplaceRegularFileAnchored(caseRoot, stateRel(caseRoot, activePath), "active mission pointer", prepared.activeBytes); err != nil {
			return Result{}, err
		}
	} else if err := replaceActivePointer(caseRoot, activePath, previousActive, prepared.activeBytes); err != nil {
		return Result{}, err
	}
	view, err := projectstate.ResolveMissionView(caseRoot)
	if err != nil || view.Generation != prepared.plan.Generation || view.MissionID != prepared.plan.MissionID {
		return Result{}, fmt.Errorf("successor mission activation verification failed: %v", err)
	}
	prepared.plan.State = "ready-to-continue"
	prepared.plan.RequiresConfirmation = false
	return Result{Plan: prepared.plan, Applied: true}, nil
}

func committedReplay(caseRoot string, opt Options) (Result, bool, error) {
	view, err := projectstate.ResolveMissionView(caseRoot)
	if err != nil {
		return Result{}, false, err
	}
	if view.Generation == 1 || view.Active == nil {
		return Result{}, false, nil
	}
	commitPath := filepath.Join(view.Root.Path, "transitions", "successor", view.Active.TransitionID, "commit.json")
	commitBytes, err := rekitfs.ReadStableRegularFileAnchored(view.Root.Path, commitPath, "committed successor transition", 1<<20)
	if err != nil {
		return Result{}, false, err
	}
	var commit TransitionCommit
	if err := decodeCanonical(commitBytes, &commit); err != nil {
		return Result{}, false, fmt.Errorf("decode committed successor transition: %w", err)
	}
	if !strings.EqualFold(commit.PlanSHA256, opt.ExpectedPlanSHA256) {
		return Result{}, false, nil
	}
	if commit.SchemaVersion != 1 || commit.Kind != "mission-successor-commit" || commit.State != "committed" || commit.TransitionID != view.Active.TransitionID || commit.Generation != view.Generation || commit.MissionID != view.MissionID || !strings.EqualFold(commit.ManifestSHA256, view.Active.ManifestSHA256) || !strings.EqualFold(sha(commitBytes), view.Active.TransitionCommitSHA) || !commit.NoAuthority || !commit.NoConfirmed || !commit.NoHeavyTool || !commit.NoAutoResume {
		return Result{}, false, fmt.Errorf("committed successor transition does not bind the active mission")
	}
	intentPath := filepath.Join(view.Root.Path, "transitions", "successor", view.Active.TransitionID, "intent.json")
	intentBytes, err := rekitfs.ReadStableRegularFileAnchored(view.Root.Path, intentPath, "committed successor intent", 1<<20)
	if err != nil {
		return Result{}, false, err
	}
	var intent TransitionIntent
	if err := decodeCanonical(intentBytes, &intent); err != nil {
		return Result{}, false, fmt.Errorf("decode committed successor intent: %w", err)
	}
	if intent.Goal != strings.TrimSpace(opt.Goal) || intent.Actor != strings.TrimSpace(opt.Actor) || intent.PublicationStamp != strings.TrimSpace(opt.PublicationStamp) || intent.MissionID != view.MissionID {
		return Result{}, false, nil
	}
	if intent.SchemaVersion != 1 || intent.Kind != "mission-successor-intent" || intent.TransitionID != commit.TransitionID || intent.Generation != commit.Generation || intent.ProjectID != view.Active.ProjectID || !strings.EqualFold(intent.PlanSHA256, commit.PlanSHA256) || !strings.EqualFold(sha(intentBytes), commit.IntentSHA256) || !intent.NoAuthority || !intent.NoConfirmed || !intent.NoHeavyTool || !intent.NoAutoResume {
		return Result{}, false, fmt.Errorf("committed successor intent does not bind the active transition")
	}
	missionBytes, err := rekitfs.ReadStableRegularFileAnchored(
		view.Root.Path,
		filepath.Join(view.Path, "mission-intent.json"),
		"committed successor mission intent",
		64<<10,
	)
	if err != nil {
		return Result{}, false, err
	}
	manifestBytes, err := rekitfs.ReadStableRegularFileAnchored(
		view.Root.Path,
		filepath.Join(view.Path, projectstate.MissionManifestFile),
		"committed successor manifest",
		4<<20,
	)
	if err != nil {
		return Result{}, false, err
	}
	var generationManifest GenerationManifest
	if err := decodeCanonical(manifestBytes, &generationManifest); err != nil {
		return Result{}, false, fmt.Errorf("decode committed successor manifest: %w", err)
	}
	plan := Plan{SchemaVersion: 1, Kind: "mission-successor-plan", Command: "successor", State: "confirmation-required", CaseRoot: caseRoot, ProjectID: intent.ProjectID, Pack: intent.Pack, PackManifestSHA256: generationManifest.PackManifestSHA256, AuthorityLane: generationManifest.AuthorityLane, PreviousGeneration: intent.PreviousGeneration, Generation: intent.Generation, MissionID: intent.MissionID, Goal: intent.Goal, Actor: intent.Actor, InitialLane: intent.InitialLane, Executor: intent.Executor, PublicationStamp: intent.PublicationStamp, PreviousMissionSHA: intent.PreviousMissionSHA, PreviousClosureSHA: intent.PreviousClosureSHA, PreviousCompletion: intent.PreviousCompletion, ExpectedPlanSHA256: commit.PlanSHA256, RequiresReview: true, RequiresConfirmation: true, NoAuthority: true, NoConfirmed: true, NoHeavyTool: true, NoAutoResume: true}
	plan.Writes = plannedWrites(intent.Generation, intent.TransitionID, missionBytes)
	artifacts, _ := buildSuccessorArtifacts(plan, intent.TransitionID, missionBytes, intent.PreviousClosureSHA)
	plan.Writes = setWriteHashes(plan.Writes, artifacts)
	if err := validateCommittedWriteIdentity(view.Root.Path, plan.Writes); err != nil {
		return Result{}, false, fmt.Errorf("verify committed successor publications: %w", err)
	}
	if planSHA, err := canonicalPlanSHA(plan); err != nil || !strings.EqualFold(planSHA, commit.PlanSHA256) {
		return Result{}, false, fmt.Errorf("committed successor intent differs from its plan binding")
	}
	plan.ExpectedPlanSHA256 = commit.PlanSHA256
	plan.ApplyArgs = successorApplyArgs(caseRoot, intent.Goal, intent.Actor, intent.PublicationStamp, commit.PlanSHA256)
	plan.State = "ready-to-continue"
	plan.RequiresConfirmation = false
	return Result{Plan: plan, Applied: true, Replay: true}, true, nil
}

func prepare(caseRoot string, opt Options, allowStamp bool) (prepared, error) {
	caseRoot, err := filepath.Abs(strings.TrimSpace(caseRoot))
	if err != nil {
		return prepared{}, err
	}
	root, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return prepared{}, err
	}
	if !root.Existing || root.Legacy || root.Dir != projectstate.CurrentDir {
		return prepared{}, fmt.Errorf("successor mission is supported only for an existing current .steamai project")
	}
	view, err := projectstate.ResolveMissionView(caseRoot)
	if err != nil {
		return prepared{}, err
	}
	inspection, err := missionintent.Inspect(caseRoot)
	if err != nil || !inspection.Committed || inspection.Identity.SchemaVersion != 2 {
		return prepared{}, fmt.Errorf("successor mission requires a committed current mission intent: %v", err)
	}
	completion, err := workstream.InspectMissionCompletion(caseRoot)
	if err != nil || !completion.Ready || !completion.OperationallyComplete || completion.State != "mission-complete" {
		return prepared{}, fmt.Errorf("successor mission requires an operationally complete current mission: %v", err)
	}
	inst, err := instance.Read(caseRoot)
	if err != nil || inst.Source != "steamai" || !strings.EqualFold(inst.TemplatePack, inspection.Identity.Pack) {
		return prepared{}, fmt.Errorf("successor mission pack differs from the project-local runtime owner")
	}
	packManifest, err := manifest.Load(inst.TemplateRoot, inspection.Identity.Pack)
	if err != nil {
		return prepared{}, fmt.Errorf("load successor mission project-local pack manifest: %w", err)
	}
	if err := packManifest.ValidateSchema(); err != nil {
		return prepared{}, fmt.Errorf("validate successor mission project-local pack manifest: %w", err)
	}
	packManifestBytes, err := rekitfs.ReadStableRegularFileAnchored(
		packManifest.PackRoot,
		packManifest.ManifestPath,
		"successor mission project-local pack manifest",
		4<<20,
	)
	if err != nil {
		return prepared{}, err
	}
	authorityLane := strings.TrimSpace(packManifest.WorkstreamDefaults["defaultAuthorityLane"])
	if authorityLane == "" {
		return prepared{}, fmt.Errorf("successor mission pack omits default authority lane")
	}
	goal := strings.TrimSpace(opt.Goal)
	actor := strings.TrimSpace(opt.Actor)
	if goal == "" || actor == "" || strings.ContainsAny(actor, "\r\n") {
		return prepared{}, fmt.Errorf("successor mission requires one natural-language goal and a single-line actor")
	}
	if goal == strings.TrimSpace(inspection.Identity.Goal) {
		return prepared{}, fmt.Errorf("successor mission goal must differ from the completed mission goal")
	}
	stamp := strings.TrimSpace(opt.PublicationStamp)
	if stamp == "" && allowStamp {
		stamp = time.Now().UTC().Format("20060102-150405000")
	}
	if stamp == "" {
		return prepared{}, fmt.Errorf("successor mission Apply requires the reviewed publication stamp")
	}
	if len(stamp) != len("20060102-150405000") {
		return prepared{}, fmt.Errorf("invalid successor mission publication stamp")
	}
	if _, err := time.Parse("20060102-150405.000", stamp[:15]+"."+stamp[15:]); err != nil {
		return prepared{}, fmt.Errorf("invalid successor mission publication stamp")
	}
	generation := view.Generation + 1
	closureSHA, err := closureSHA256(view, generation)
	if err != nil {
		return prepared{}, err
	}
	initialLane := inspection.Identity.InitialLane
	executor := fmt.Sprintf("daily-successor-g%06d", generation)
	identity := inspection.Identity
	identity.Goal, identity.Actor, identity.Executor, identity.InitialLane = goal, actor, executor, initialLane
	missionBytes, err := missionintent.MarshalMissionIntentAt(caseRoot, identity)
	if err != nil {
		return prepared{}, err
	}
	base := Plan{SchemaVersion: 1, Kind: "mission-successor-plan", Command: "successor", State: "confirmation-required", CaseRoot: caseRoot, ProjectID: identity.ProjectID, Pack: identity.Pack, PackManifestSHA256: sha(packManifestBytes), AuthorityLane: authorityLane, PreviousGeneration: view.Generation, Generation: generation, Goal: goal, Actor: actor, InitialLane: initialLane, Executor: executor, PublicationStamp: stamp, PreviousMissionSHA: inspection.MissionIntentSHA256, PreviousClosureSHA: closureSHA, PreviousCompletion: completion, RequiresReview: true, RequiresConfirmation: true, NoAuthority: true, NoConfirmed: true, NoHeavyTool: true, NoAutoResume: true}
	seed, _ := canonicalSHA(base)
	missionID, err := projectstate.MissionID(generation, seed)
	if err != nil {
		return prepared{}, err
	}
	base.MissionID = missionID
	transitionID := stamp + "-" + missionID
	base.Writes = plannedWrites(generation, transitionID, missionBytes)
	planSHA, err := canonicalPlanSHA(base)
	if err != nil {
		return prepared{}, err
	}
	base.ExpectedPlanSHA256 = planSHA
	base.ApplyArgs = successorApplyArgs(caseRoot, goal, actor, stamp, planSHA)
	artifacts, _ := buildSuccessorArtifacts(base, transitionID, missionBytes, closureSHA)
	base.Writes = setWriteHashes(base.Writes, artifacts)
	if finalPlanSHA, err := canonicalPlanSHA(base); err != nil || !strings.EqualFold(finalPlanSHA, planSHA) {
		return prepared{}, fmt.Errorf("successor publication plan binding changed after artifact hashes")
	}
	activeRel := filepath.ToSlash(filepath.Join(projectstate.CurrentDir, projectstate.MissionsDir, projectstate.MissionActiveFile))
	return prepared{plan: base, transitionID: transitionID, intentBytes: artifacts[transitionPath(transitionID, "intent.json")], missionBytes: missionBytes, manifestBytes: artifacts[generationPath(generation, projectstate.MissionManifestFile)], generationCommit: artifacts[generationPath(generation, "commit.json")], transitionCommit: artifacts[transitionPath(transitionID, "commit.json")], activeBytes: artifacts[activeRel], boardBytes: artifacts[generationPath(generation, "board.json")], view: view}, nil
}

func retireCommittedActivePredecessors(caseRoot string, activeGeneration int) error {
	if activeGeneration < 2 {
		return nil
	}
	root, err := rekitfs.OpenAnchoredRoot(caseRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	missionsRel := filepath.ToSlash(filepath.Join(projectstate.CurrentDir, projectstate.MissionsDir))
	entries, err := root.ListNoFollow(missionsRel, maxClosureFiles)
	if err != nil {
		return err
	}
	prefix := "." + projectstate.MissionActiveFile + ".predecessor-"
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) || !strings.HasSuffix(entry.Name(), ".tmp") {
			continue
		}
		rel := filepath.ToSlash(filepath.Join(missionsRel, entry.Name()))
		data, _, err := root.ReadStableFile(rel, 64<<10)
		if err != nil {
			return err
		}
		var predecessor projectstate.MissionActive
		if err := decodeCanonical(data, &predecessor); err != nil || projectstate.ValidateMissionActive(predecessor) != nil || predecessor.Generation >= activeGeneration {
			return fmt.Errorf("committed active mission predecessor recovery is invalid: %s", rel)
		}
		if err := root.RemoveExactFile(rel, data, 0o600); err != nil {
			return fmt.Errorf("retire committed active mission predecessor: %w", err)
		}
	}
	return nil
}

func recoverInterruptedActiveReplacement(caseRoot, expectedPlanSHA string) error {
	state, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return err
	}
	activePath, err := projectstate.MissionActivePath(caseRoot)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(activePath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	root, err := rekitfs.OpenAnchoredRoot(caseRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	entries, err := root.ListNoFollow(
		filepath.ToSlash(filepath.Join(projectstate.CurrentDir, projectstate.MissionsDir)),
		maxClosureFiles,
	)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	prefix := "." + projectstate.MissionActiveFile + ".predecessor-"
	var recoveryRel string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".tmp") {
			if recoveryRel != "" {
				return fmt.Errorf("multiple interrupted active mission predecessors require manual review")
			}
			recoveryRel = filepath.ToSlash(filepath.Join(projectstate.CurrentDir, projectstate.MissionsDir, name))
		}
	}
	if recoveryRel == "" {
		return nil
	}
	previousBytes, _, err := root.ReadStableFile(recoveryRel, 64<<10)
	if err != nil {
		return err
	}
	var previous projectstate.MissionActive
	if err := decodeCanonical(previousBytes, &previous); err != nil || projectstate.ValidateMissionActive(previous) != nil {
		return fmt.Errorf("interrupted active mission predecessor is invalid: %w", err)
	}
	transitionEntries, err := root.ListNoFollow(
		filepath.ToSlash(filepath.Join(projectstate.CurrentDir, filepath.FromSlash(transitionRoot))),
		maxClosureFiles,
	)
	if err != nil {
		return err
	}
	var pending TransitionIntent
	matches := 0
	for _, entry := range transitionEntries {
		if !entry.IsDir() {
			continue
		}
		intentRel := filepath.ToSlash(filepath.Join(projectstate.CurrentDir, filepath.FromSlash(transitionRoot), entry.Name(), "intent.json"))
		intentBytes, _, readErr := root.ReadStableFile(intentRel, 1<<20)
		if readErr != nil {
			return readErr
		}
		var intent TransitionIntent
		if err := decodeCanonical(intentBytes, &intent); err != nil {
			return err
		}
		if intent.PreviousGeneration == previous.Generation && strings.EqualFold(intent.PlanSHA256, expectedPlanSHA) {
			pending = intent
			matches++
		}
	}
	if matches != 1 || pending.Generation != previous.Generation+1 {
		return fmt.Errorf("interrupted active mission replacement does not bind one exact reviewed successor")
	}
	plannedPath := filepath.Join(state.Path, projectstate.MissionsDir, fmt.Sprintf("g%06d", pending.Generation), projectstate.MissionManifestFile)
	manifestBytes, err := rekitfs.ReadStableRegularFileAnchored(state.Path, plannedPath, "interrupted successor manifest", 4<<20)
	if err != nil {
		return err
	}
	var manifest GenerationManifest
	if err := decodeCanonical(manifestBytes, &manifest); err != nil || manifest.Generation != pending.Generation || manifest.MissionID != pending.MissionID || !strings.EqualFold(manifest.PlanSHA256, expectedPlanSHA) {
		return fmt.Errorf("interrupted successor manifest does not bind the reviewed plan: %w", err)
	}
	activeRel := filepath.ToSlash(filepath.Join(projectstate.CurrentDir, projectstate.MissionsDir, projectstate.MissionActiveFile))
	if err := root.RenameFileNoReplaceExact(recoveryRel, activeRel, previousBytes, 0o600); err != nil {
		return fmt.Errorf("restore interrupted active mission predecessor: %w", err)
	}
	return nil
}

func validatePendingTransitionRecovery(
	caseRoot string,
	previousGeneration int,
	transitionID,
	planSHA string,
) error {
	state, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return err
	}
	root, err := rekitfs.OpenAnchoredRoot(state.Path)
	if err != nil {
		return err
	}
	defer root.Close()
	entries, err := root.ListNoFollow(transitionRoot, maxClosureFiles)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("list successor transitions: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			return fmt.Errorf("successor transition root contains a non-directory entry: %s", entry.Name())
		}
		name := entry.Name()
		intentRel := filepath.ToSlash(filepath.Join(transitionRoot, name, "intent.json"))
		intentBytes, _, err := root.ReadStableFile(intentRel, 1<<20)
		if err != nil {
			return fmt.Errorf("read successor transition intent %s: %w", name, err)
		}
		var intent TransitionIntent
		if err := decodeCanonical(intentBytes, &intent); err != nil {
			return fmt.Errorf("decode successor transition intent %s: %w", name, err)
		}
		if intent.SchemaVersion != 1 || intent.Kind != "mission-successor-intent" ||
			intent.TransitionID != name || intent.PreviousGeneration < 1 ||
			intent.Generation != intent.PreviousGeneration+1 ||
			!validSHA(intent.PlanSHA256) || !intent.NoAuthority || !intent.NoConfirmed ||
			!intent.NoHeavyTool || !intent.NoAutoResume {
			return fmt.Errorf("successor transition intent %s is invalid", name)
		}
		commitRel := filepath.ToSlash(filepath.Join(transitionRoot, name, "commit.json"))
		commitBytes, _, commitErr := root.ReadStableFile(commitRel, 1<<20)
		if commitErr == nil {
			var commit TransitionCommit
			if err := decodeCanonical(commitBytes, &commit); err != nil ||
				commit.SchemaVersion != 1 || commit.Kind != "mission-successor-commit" ||
				commit.State != "committed" || commit.TransitionID != name ||
				commit.Generation != intent.Generation || commit.MissionID != intent.MissionID ||
				!strings.EqualFold(commit.PlanSHA256, intent.PlanSHA256) ||
				!strings.EqualFold(commit.IntentSHA256, sha(intentBytes)) ||
				!commit.NoAuthority || !commit.NoConfirmed || !commit.NoHeavyTool || !commit.NoAutoResume {
				return fmt.Errorf("successor transition commit %s is invalid", name)
			}
			if intent.Generation <= previousGeneration {
				continue
			}
			if intent.PreviousGeneration != previousGeneration || intent.TransitionID != transitionID || !strings.EqualFold(intent.PlanSHA256, planSHA) {
				return fmt.Errorf("a different committed successor transition is pending activation; recover the exact original Apply")
			}
			continue
		}
		if !os.IsNotExist(commitErr) {
			return fmt.Errorf("read successor transition commit %s: %w", name, commitErr)
		}
		if intent.PreviousGeneration != previousGeneration {
			return fmt.Errorf("a stale successor transition is pending; recover or retire the exact original Apply")
		}
		if intent.TransitionID != transitionID || !strings.EqualFold(intent.PlanSHA256, planSHA) {
			return fmt.Errorf("a different successor transition is pending; recover the exact original Apply")
		}
	}
	return nil
}

func closureSHA256(view projectstate.MissionView, successorGeneration int) (string, error) {
	entries := []string{}
	err := filepath.WalkDir(view.Path, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == view.Path {
			return nil
		}
		if view.Generation == 1 && view.Root.Path == view.Path {
			successorRoot := filepath.Join(view.Root.Path, filepath.FromSlash(transitionRoot))
			if path == successorRoot {
				return filepath.SkipDir
			}
			successorMissionRoot := filepath.Join(
				view.Root.Path,
				projectstate.MissionsDir,
				fmt.Sprintf("g%06d", successorGeneration),
			)
			if path == successorMissionRoot {
				return filepath.SkipDir
			}
		}
		rel, err := filepath.Rel(view.Path, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if len(entries) >= maxClosureFiles {
			return fmt.Errorf("mission closure exceeds %d files", maxClosureFiles)
		}
		data, err := rekitfs.ReadStableRegularFileAllowEmptyAnchored(
			view.Path,
			path,
			"mission closure artifact",
			maxClosureFile,
		)
		if err != nil {
			return err
		}
		entries = append(entries, filepath.ToSlash(rel)+"\x00"+sha(data)+fmt.Sprintf("\x00%d", len(data)))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(entries)
	return sha([]byte(strings.Join(entries, "\n"))), nil
}

func plannedWrites(generation int, transitionID string, missionBytes []byte) []Write {
	writes := []Write{
		writeFor(transitionPath(transitionID, "intent.json"), "successor-intent", nil, "prepare"),
		writeFor(generationPath(generation, "mission-intent.json"), "mission-intent", missionBytes, "prepare"),
		writeFor(generationPath(generation, "board.json"), "board", nil, "prepare"),
	}
	for _, name := range missionFactNames() {
		writes = append(writes, writeFor(generationPath(generation, filepath.ToSlash(filepath.Join("facts", name))), "fact-jsonl", []byte("\n"), "prepare"))
	}
	writes = append(writes,
		writeFor(generationPath(generation, projectstate.MissionManifestFile), "mission-manifest", nil, "prepare"),
		writeFor(generationPath(generation, "commit.json"), "mission-generation-commit", nil, "commit"),
		writeFor(transitionPath(transitionID, "commit.json"), "successor-transition-commit", nil, "commit"),
		writeFor(filepath.ToSlash(filepath.Join(projectstate.CurrentDir, projectstate.MissionsDir, projectstate.MissionActiveFile)), "active-mission-pointer", nil, "activate-last"),
	)
	return writes
}

func writeFor(path, kind string, data []byte, phase string) Write {
	write := Write{Path: path, Kind: kind, Phase: phase}
	if len(data) > 0 {
		write.SHA256, write.Size = sha(data), int64(len(data))
	}
	return write
}

func canonicalPlanSHA(plan Plan) (string, error) {
	// Artifact digests are deliberately excluded from the plan identity. The
	// durable intent and commit artifacts carry the plan digest, while the
	// preview itself carries the exact digest and size for every write. This
	// keeps the plan hash acyclic even though commits refer to one another.
	copy := plan
	copy.ExpectedPlanSHA256 = ""
	copy.ApplyArgs = nil
	copy.Writes = append([]Write(nil), plan.Writes...)
	for i := range copy.Writes {
		copy.Writes[i].SHA256 = ""
		copy.Writes[i].Size = 0
	}
	return canonicalSHA(copy)
}

func buildSuccessorArtifacts(
	plan Plan,
	transitionID string,
	missionBytes []byte,
	closureSHA string,
) (map[string][]byte, TransitionIntent) {
	boardBytes := successorBoardBytes(plan, plan.AuthorityLane)
	manifestBytes := mustCanonical(GenerationManifest{
		SchemaVersion:      1,
		Kind:               "mission-generation-manifest",
		ProjectID:          plan.ProjectID,
		Pack:               plan.Pack,
		PackManifestSHA256: plan.PackManifestSHA256,
		AuthorityLane:      plan.AuthorityLane,
		Generation:         plan.Generation,
		MissionID:          plan.MissionID,
		MissionIntentSHA:   sha(missionBytes),
		PreviousClosureSHA: closureSHA,
		TransitionID:       transitionID,
		PlanSHA256:         plan.ExpectedPlanSHA256,
		NoAuthority:        true,
		NoConfirmed:        true,
		NoHeavyTool:        true,
	})
	intent := TransitionIntent{
		SchemaVersion:      1,
		Kind:               "mission-successor-intent",
		TransitionID:       transitionID,
		PublicationStamp:   plan.PublicationStamp,
		ProjectID:          plan.ProjectID,
		Pack:               plan.Pack,
		PreviousGeneration: plan.PreviousGeneration,
		Generation:         plan.Generation,
		MissionID:          plan.MissionID,
		Goal:               plan.Goal,
		Actor:              plan.Actor,
		InitialLane:        plan.InitialLane,
		Executor:           plan.Executor,
		PreviousMissionSHA: plan.PreviousMissionSHA,
		PreviousClosureSHA: closureSHA,
		PreviousCompletion: plan.PreviousCompletion,
		PlanSHA256:         plan.ExpectedPlanSHA256,
		NoAuthority:        true,
		NoConfirmed:        true,
		NoHeavyTool:        true,
		NoAutoResume:       true,
	}
	intentBytes := mustCanonical(intent)
	generationCommit := mustCanonical(GenerationCommit{
		SchemaVersion:  1,
		Kind:           "mission-generation-commit",
		State:          "committed",
		Generation:     plan.Generation,
		MissionID:      plan.MissionID,
		ManifestSHA256: sha(manifestBytes),
		PlanSHA256:     plan.ExpectedPlanSHA256,
	})
	transitionCommit := mustCanonical(TransitionCommit{
		SchemaVersion:  1,
		Kind:           "mission-successor-commit",
		State:          "committed",
		TransitionID:   transitionID,
		PlanSHA256:     plan.ExpectedPlanSHA256,
		IntentSHA256:   sha(intentBytes),
		Generation:     plan.Generation,
		MissionID:      plan.MissionID,
		ManifestSHA256: sha(manifestBytes),
		CommittedAt:    plan.PublicationStamp,
		NoAuthority:    true,
		NoConfirmed:    true,
		NoHeavyTool:    true,
		NoAutoResume:   true,
	})
	activeBytes := mustCanonical(projectstate.MissionActive{
		SchemaVersion:       1,
		Kind:                "active-mission-pointer",
		ProjectID:           plan.ProjectID,
		Generation:          plan.Generation,
		MissionID:           plan.MissionID,
		MissionIntentSHA256: sha(missionBytes),
		ManifestSHA256:      sha(manifestBytes),
		TransitionID:        transitionID,
		TransitionCommitSHA: sha(transitionCommit),
	})
	artifacts := map[string][]byte{
		transitionPath(transitionID, "intent.json"):                                                                        intentBytes,
		generationPath(plan.Generation, "mission-intent.json"):                                                             missionBytes,
		generationPath(plan.Generation, "board.json"):                                                                      boardBytes,
		generationPath(plan.Generation, projectstate.MissionManifestFile):                                                  manifestBytes,
		generationPath(plan.Generation, "commit.json"):                                                                     generationCommit,
		transitionPath(transitionID, "commit.json"):                                                                        transitionCommit,
		filepath.ToSlash(filepath.Join(projectstate.CurrentDir, projectstate.MissionsDir, projectstate.MissionActiveFile)): activeBytes,
	}
	for _, name := range missionFactNames() {
		artifacts[generationPath(plan.Generation, filepath.ToSlash(filepath.Join("facts", name)))] = []byte("\n")
	}
	return artifacts, intent
}

func setWriteHashes(writes []Write, artifacts map[string][]byte) []Write {
	result := append([]Write(nil), writes...)
	for i := range result {
		data, ok := artifacts[result[i].Path]
		if !ok {
			continue
		}
		result[i].SHA256 = sha(data)
		result[i].Size = int64(len(data))
	}
	return result
}

func validatePlannedWrite(writes []Write, path string, data []byte) error {
	for _, write := range writes {
		if write.Path != path {
			continue
		}
		if write.Size != int64(len(data)) || !strings.EqualFold(write.SHA256, sha(data)) {
			return fmt.Errorf("successor publication differs from reviewed bytes: %s", path)
		}
		return nil
	}
	return fmt.Errorf("successor publication is absent from reviewed writes: %s", path)
}

func validateCommittedWriteIdentity(stateRoot string, writes []Write) error {
	caseRoot := filepath.Dir(stateRoot)
	for _, write := range writes {
		switch write.Kind {
		case "board", "fact-jsonl":
			// These mission-owned artifacts are intentionally mutable after activation.
			continue
		}
		path := filepath.Join(caseRoot, filepath.FromSlash(write.Path))
		data, err := rekitfs.ReadStableRegularFileAllowEmptyAnchored(
			caseRoot,
			path,
			"committed successor publication",
			maxClosureFile,
		)
		if err != nil {
			return err
		}
		if write.Size != int64(len(data)) || !strings.EqualFold(write.SHA256, sha(data)) {
			return fmt.Errorf("committed successor publication differs from reviewed bytes: %s", write.Path)
		}
	}
	return nil
}

func successorBoardBytes(plan Plan, authorityLane string) []byte {
	return mustCanonical(map[string]any{
		"schemaVersion":        1,
		"caseRoot":             plan.CaseRoot,
		"repoRoot":             filepath.Join(plan.CaseRoot, projectstate.CurrentDir),
		"pack":                 plan.Pack,
		"automationMode":       "assisted-autopilot",
		"defaultAuthorityLane": authorityLane,
		"lanes":                []any{},
		"factsRoot":            filepath.ToSlash(filepath.Join(projectstate.CurrentDir, projectstate.MissionsDir, fmt.Sprintf("g%06d", plan.Generation), "facts")),
		"updatedAt":            plan.PublicationStamp,
	})
}

func successorApplyArgs(caseRoot, goal, actor, stamp, planSHA string) []string {
	return []string{"host", "-daily", "-target", caseRoot, "-goal", goal, "-actor", actor, "-successor-apply", "-successor-publication-stamp", stamp, "-expected-successor-plan-sha256", planSHA}
}

func missionFactNames() []string {
	return []string{"observations.jsonl", "candidates.jsonl", "requests.jsonl", "publications.jsonl", "decisions.jsonl", "hypotheses.jsonl", "verifications.jsonl", "interventions.jsonl", "rollbacks.jsonl"}
}

func generationPath(generation int, rel string) string {
	return filepath.ToSlash(filepath.Join(projectstate.CurrentDir, projectstate.MissionsDir, fmt.Sprintf("g%06d", generation), filepath.FromSlash(rel)))
}

func transitionPath(id, name string) string {
	return filepath.ToSlash(filepath.Join(projectstate.CurrentDir, filepath.FromSlash(transitionRoot), id, name))
}

func stateRel(caseRoot, path string) string {
	rel, _ := filepath.Rel(caseRoot, path)
	return filepath.ToSlash(rel)
}

func replaceActivePointer(caseRoot, path string, previous, planned []byte) error {
	root, err := rekitfs.OpenAnchoredRoot(caseRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	activeRel := stateRel(caseRoot, path)
	current, _, err := root.ReadStableFile(activeRel, 64<<10)
	if err != nil || !bytes.Equal(current, previous) {
		return fmt.Errorf("active mission pointer changed before successor activation: %w", err)
	}
	tempRel := filepath.ToSlash(filepath.Join(
		filepath.Dir(activeRel),
		"."+projectstate.MissionActiveFile+".successor-"+sha(planned)[:16]+".tmp",
	))
	if err := root.MkdirAllNoFollow(filepath.Dir(tempRel), 0o700); err != nil {
		return err
	}
	if _, err := root.WriteExclusiveFileWriteThrough(tempRel, planned, 0o600, true); err != nil {
		return fmt.Errorf("publish exact active mission pointer replacement: %w", err)
	}
	if err := root.ReplaceFileExact(tempRel, activeRel, planned, 0o600, previous, 0o600); err != nil {
		return fmt.Errorf("atomically replace active mission pointer: %w", err)
	}
	published, _, err := root.ReadStableFile(activeRel, 64<<10)
	if err != nil || !bytes.Equal(published, planned) {
		return fmt.Errorf("published active mission pointer differs after replacement: %w", err)
	}
	return nil
}

func canonicalSHA(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return sha(data), nil
}

func decodeCanonical(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return fmt.Errorf("successor artifact contains trailing JSON")
	} else if !errors.Is(err, io.EOF) {
		return fmt.Errorf("successor artifact contains trailing data: %w", err)
	}
	canonical := mustCanonical(target)
	if !bytes.Equal(data, canonical) && !bytes.Equal(data, bytes.TrimSuffix(canonical, []byte{'\n'})) {
		return fmt.Errorf("successor artifact is not canonical")
	}
	return nil
}

func mustCanonical(value any) []byte {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err)
	}
	return append(data, '\n')
}

func sha(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validSHA(value string) bool {
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	return err == nil && len(decoded) == sha256.Size
}

func IsZeroProgress(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}
