package memberexecution

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
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

const (
	maxJSONBytes   = 256 * 1024
	maxOutputBytes = 4 * 1024 * 1024
	maxOutputs     = 64
)

var segment = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`)
var applyLeaseHook func(Plan) error

func PreviewDispatch(opt DispatchOptions) (Plan, error) {
	caseRoot, owner, err := currentOwner(opt.CaseRoot, opt.Pack, opt.Lane)
	if err != nil {
		return Plan{}, err
	}
	if !validSHA(opt.RequestSHA256) {
		return Plan{}, fmt.Errorf("member execution dispatch requires request sha256")
	}
	createdAt := strings.TrimSpace(opt.CreatedAt)
	if createdAt == "" {
		createdAt, err = currentOwnerUpdatedAt(caseRoot, owner.Lane)
		if err != nil {
			return Plan{}, err
		}
	}
	created, err := parseTime(createdAt)
	if err != nil {
		return Plan{}, err
	}
	attemptSequence, err := nextAttemptSequence(caseRoot, owner, opt.RequestSHA256)
	if err != nil {
		return Plan{}, err
	}
	attemptID := fmt.Sprintf("g%06d-a%06d-%s", owner.ExecutorGeneration, attemptSequence, strings.ToLower(opt.RequestSHA256[:16]))
	if !segment.MatchString(attemptID) {
		return Plan{}, fmt.Errorf("invalid member execution attempt id")
	}
	root, err := attemptRoot(caseRoot, owner.Lane, attemptID)
	if err != nil {
		return Plan{}, err
	}
	intent := Intent{SchemaVersion: 1, Kind: KindIntent, AttemptID: attemptID, CaseRoot: caseRoot, Pack: opt.Pack, Owner: owner, RequestSHA256: strings.ToLower(opt.RequestSHA256), CreatedAt: created.Format(time.RFC3339Nano), NoSpawn: true, NoPoll: true, NoStop: true, NoHeavyTool: true, NoAuthority: true, NoConfirmed: true}
	intentBytes, err := canonical(intent)
	if err != nil {
		return Plan{}, err
	}
	handoff := Handoff{SchemaVersion: 1, Kind: KindHandoff, AttemptID: attemptID, Owner: owner, IntentSHA256: hash(intentBytes), ManifestPath: rel(caseRoot, filepath.Join(root, "result", "manifest.json")), OutputsRoot: rel(caseRoot, filepath.Join(root, "result", "outputs")), NextSteps: []string{"external harness accepts this handoff", "external member writes bounded outputs and strict manifest", "record accepted then returned or failed observation through run-current-step"}, Boundary: boundaries()}
	handoffBytes, err := canonical(handoff)
	if err != nil {
		return Plan{}, err
	}
	commit := Commit{SchemaVersion: 1, Kind: KindCommit, AttemptID: attemptID, IntentSHA256: hash(intentBytes), HandoffSHA256: hash(handoffBytes)}
	commitBytes, err := canonical(commit)
	if err != nil {
		return Plan{}, err
	}
	writes := []plannedWrite{{filepath.Join(root, "intent.json"), intentBytes}, {filepath.Join(root, "handoff.json"), handoffBytes}, {filepath.Join(root, "commit.json"), commitBytes}}
	inspection := Inspection{State: "handoff-ready", AttemptID: attemptID, Owner: owner, Intent: &intent, Handoff: &handoff, AttemptRoot: root, ManifestPath: filepath.Join(root, "result", "manifest.json"), OutputsRoot: filepath.Join(root, "result", "outputs")}
	return finishPlan(Plan{SchemaVersion: 1, Mode: "dispatch", CaseRoot: caseRoot, Pack: opt.Pack, AttemptID: attemptID, Owner: owner, ExternalHandoff: &handoff, Inspection: inspection, ReviewRequired: true, RequiresConfirmation: true, Boundary: boundaries(), writes: writes})
}

func PreviewObservation(opt ObservationOptions) (Plan, error) {
	caseRoot, owner, err := currentOwner(opt.CaseRoot, opt.Pack, opt.Lane)
	if err != nil {
		return Plan{}, err
	}
	if !segment.MatchString(opt.AttemptID) {
		return Plan{}, fmt.Errorf("invalid member execution attempt id")
	}
	inspection, err := Inspect(caseRoot, owner.Lane, opt.AttemptID)
	if err != nil {
		return Plan{}, err
	}
	if inspection.Owner != owner {
		return Plan{}, fmt.Errorf("member execution attempt owner is stale; current executor generation differs")
	}
	outcome := strings.ToLower(strings.TrimSpace(opt.Outcome))
	if outcome != "accepted" && outcome != "returned" && outcome != "failed" {
		return Plan{}, fmt.Errorf("member execution outcome must be accepted, returned, or failed")
	}
	actor := strings.TrimSpace(opt.Actor)
	if actor == "" {
		return Plan{}, fmt.Errorf("member execution observation requires actor")
	}
	if strings.TrimSpace(opt.ObservedAt) == "" {
		return Plan{}, fmt.Errorf("member execution observation requires observedAt")
	}
	observed, err := parseTime(opt.ObservedAt)
	if err != nil {
		return Plan{}, err
	}
	if inspection.Latest != nil {
		if inspection.Latest.Outcome == outcome && inspection.Latest.Actor == actor && inspection.Latest.Reason == opt.Reason && inspection.Latest.ObservedAt == observed.Format(time.RFC3339Nano) {
			return finishPlan(Plan{SchemaVersion: 1, Mode: "observe", CaseRoot: caseRoot, Pack: opt.Pack, AttemptID: opt.AttemptID, Owner: owner, Outcome: outcome, Actor: actor, Reason: opt.Reason, Inspection: inspection, ReviewRequired: true, RequiresConfirmation: true, Boundary: boundaries()})
		}
		if inspection.Latest.Outcome == "returned" || inspection.Latest.Outcome == "failed" {
			return Plan{}, fmt.Errorf("member execution attempt is final: %s", inspection.Latest.Outcome)
		}
		if outcome == "accepted" {
			return Plan{}, fmt.Errorf("member execution accepted observation already exists")
		}
	} else if outcome != "accepted" && outcome != "failed" {
		return Plan{}, fmt.Errorf("member execution returned requires accepted observation first")
	}
	manifestSHA := ""
	resultWrites := []plannedWrite{}
	if outcome == "returned" {
		manifest, sum, writes, err := snapshotResultPlan(inspection)
		if err != nil {
			return Plan{}, err
		}
		inspection.Manifest, inspection.ManifestSHA256, manifestSHA = &manifest, sum, sum
		inspection.ManifestPath, inspection.OutputsRoot = canonicalResultPaths(inspection.AttemptRoot)
		resultWrites = writes
	}
	sequence := 1
	if inspection.Latest != nil {
		sequence = inspection.Latest.Sequence + 1
	}
	observation := Observation{SchemaVersion: 1, Kind: KindObservation, Sequence: sequence, AttemptID: opt.AttemptID, Owner: owner, Outcome: outcome, Actor: actor, Reason: opt.Reason, ObservedAt: observed.Format(time.RFC3339Nano), ManifestSHA256: manifestSHA, IntentSHA256: hashMustCanonical(*inspection.Intent), NoAuthority: true, NoConfirmed: true, NoHeavyTool: true}
	data, err := canonical(observation)
	if err != nil {
		return Plan{}, err
	}
	path := filepath.Join(inspection.AttemptRoot, "observations", fmt.Sprintf("%020d-%s.json", sequence, outcome))
	resultWrites = append(resultWrites, plannedWrite{path, data})
	inspection.Latest = &observation
	state := outcome
	if outcome == "returned" {
		state = "intake-ready"
	}
	inspection.State = state
	return finishPlan(Plan{SchemaVersion: 1, Mode: "observe", CaseRoot: caseRoot, Pack: opt.Pack, AttemptID: opt.AttemptID, Owner: owner, Outcome: outcome, Actor: actor, Reason: opt.Reason, Inspection: inspection, ReviewRequired: true, RequiresConfirmation: true, Boundary: boundaries(), writes: resultWrites})
}

func Apply(plan Plan, expected string) (_ Result, retErr error) {
	if !validSHA(expected) || !strings.EqualFold(expected, plan.ExpectedPlanSHA256) {
		return Result{}, fmt.Errorf("member execution expected plan sha256 mismatch")
	}
	lease, err := lanemutation.AcquireOpenLane(plan.CaseRoot, plan.Owner.Lane, "member execution")
	if err != nil {
		return Result{}, err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	if applyLeaseHook != nil {
		if err := applyLeaseHook(plan); err != nil {
			return Result{}, err
		}
	}
	facts, err := mission.ReadStrictLedgerFacts(plan.CaseRoot)
	if err != nil {
		return Result{}, err
	}
	if len(mission.EffectiveOpenLaneInterventions(facts.Facts, plan.Owner.Lane)) > 0 {
		return Result{}, fmt.Errorf("member execution refuses lane with an open intervention")
	}
	rebuilt, err := rebuildApplyPlan(plan)
	if err != nil {
		return Result{}, err
	}
	if !strings.EqualFold(rebuilt.ExpectedPlanSHA256, expected) {
		return Result{}, fmt.Errorf("member execution plan changed before Apply")
	}
	plan = rebuilt
	if err := lease.Validate(); err != nil {
		return Result{}, err
	}
	anchor, err := openAnchoredCase(plan.CaseRoot)
	if err != nil {
		return Result{}, err
	}
	defer anchor.Close()
	current, err := currentOwnerAnchored(anchor, plan.Pack, plan.Owner.Lane)
	if err != nil {
		return Result{}, err
	}
	if current != plan.Owner {
		return Result{}, fmt.Errorf("member execution owner changed before Apply")
	}
	caseRoot := anchor.path
	if plan.Mode == "observe" {
		currentInspection, err := inspectAnchored(anchor, plan.Owner.Lane, plan.AttemptID)
		if err != nil {
			return Result{}, err
		}
		if currentInspection.Owner != plan.Owner {
			return Result{}, fmt.Errorf("member execution attempt owner changed before Apply")
		}
		if plan.Outcome == "returned" {
			_, sum, err := inspectManifestAnchored(anchor, currentInspection)
			if err != nil {
				return Result{}, err
			}
			if !strings.EqualFold(sum, plan.Inspection.ManifestSHA256) {
				return Result{}, fmt.Errorf("member execution result manifest changed after preview")
			}
		}
	}
	if len(plan.writes) == 0 {
		if err := anchor.revalidate(); err != nil {
			return Result{}, err
		}
		return Result{Plan: plan, AlreadyApplied: true}, nil
	}
	rels := make([]string, len(plan.writes))
	firstMissing := len(plan.writes)
	for index, write := range plan.writes {
		rel, err := relativeToCase(caseRoot, write.path)
		if err != nil {
			return Result{}, err
		}
		rels[index] = rel
		existing, err := anchor.readFile(rel, int64(len(write.data))+1)
		if err == nil {
			if firstMissing != len(plan.writes) {
				return Result{}, fmt.Errorf("member execution publication is non-prefix at %s", rel)
			}
			if !bytes.Equal(existing, write.data) {
				return Result{}, fmt.Errorf("member execution existing artifact differs: %s", rel)
			}
			continue
		}
		if !os.IsNotExist(err) {
			return Result{}, err
		}
		if firstMissing == len(plan.writes) {
			firstMissing = index
		}
	}
	written := 0
	for index := firstMissing; index < len(plan.writes); index++ {
		if err := anchor.writeExclusive(rels[index], plan.writes[index].data); err != nil {
			return Result{}, err
		}
		written++
	}
	inspection, err := inspectAnchored(anchor, plan.Owner.Lane, plan.AttemptID)
	if err != nil {
		return Result{}, err
	}
	if err := anchor.revalidate(); err != nil {
		return Result{}, err
	}
	if err := lease.Validate(); err != nil {
		return Result{}, err
	}
	plan.Inspection = inspection
	return Result{Plan: plan, Applied: written > 0, AlreadyApplied: written == 0}, nil
}

func rebuildApplyPlan(plan Plan) (Plan, error) {
	if plan.Mode == "dispatch" && plan.Inspection.Intent != nil {
		return PreviewDispatch(DispatchOptions{CaseRoot: plan.CaseRoot, Pack: plan.Pack, Lane: plan.Owner.Lane, RequestSHA256: plan.Inspection.Intent.RequestSHA256, CreatedAt: plan.Inspection.Intent.CreatedAt})
	}
	if plan.Mode == "observe" && plan.Inspection.Latest != nil {
		observation := plan.Inspection.Latest
		return PreviewObservation(ObservationOptions{CaseRoot: plan.CaseRoot, Pack: plan.Pack, Lane: plan.Owner.Lane, AttemptID: plan.AttemptID, Outcome: plan.Outcome, Actor: plan.Actor, Reason: plan.Reason, ObservedAt: observation.ObservedAt})
	}
	return Plan{}, fmt.Errorf("member execution plan cannot be rebuilt")
}

func Inspect(caseRoot, lane, attemptID string) (Inspection, error) {
	anchor, err := openAnchoredCase(caseRoot)
	if err != nil {
		return Inspection{}, err
	}
	defer anchor.Close()
	inspection, err := inspectAnchored(anchor, lane, attemptID)
	if err != nil {
		return Inspection{}, err
	}
	if err := anchor.revalidate(); err != nil {
		return Inspection{}, err
	}
	return inspection, nil
}

func inspectAnchored(anchor *anchoredCase, lane, attemptID string) (Inspection, error) {
	root, err := attemptRoot(anchor.path, lane, attemptID)
	if err != nil {
		return Inspection{}, err
	}
	rootRel, err := relativeToCase(anchor.path, root)
	if err != nil {
		return Inspection{}, err
	}
	intentBytes, err := anchor.readFile(filepath.Join(rootRel, "intent.json"), maxJSONBytes)
	if err != nil {
		return Inspection{}, err
	}
	var intent Intent
	if err := strictCanonical(intentBytes, &intent); err != nil {
		return Inspection{}, err
	}
	handoffBytes, err := anchor.readFile(filepath.Join(rootRel, "handoff.json"), maxJSONBytes)
	if err != nil {
		return Inspection{}, err
	}
	var handoff Handoff
	if err := strictCanonical(handoffBytes, &handoff); err != nil {
		return Inspection{}, err
	}
	commitBytes, err := anchor.readFile(filepath.Join(rootRel, "commit.json"), maxJSONBytes)
	if err != nil {
		return Inspection{}, err
	}
	var commit Commit
	if err := strictCanonical(commitBytes, &commit); err != nil {
		return Inspection{}, err
	}
	if intent.SchemaVersion != SchemaVersion || intent.Kind != KindIntent || intent.AttemptID != attemptID || !samePath(intent.CaseRoot, anchor.path) || intent.Owner.Lane != lane || !validSHA(intent.RequestSHA256) || strings.TrimSpace(intent.Pack) == "" || !intent.NoSpawn || !intent.NoPoll || !intent.NoStop || !intent.NoAuthority || !intent.NoConfirmed || !intent.NoHeavyTool {
		return Inspection{}, fmt.Errorf("invalid member execution intent")
	}
	if handoff.SchemaVersion != SchemaVersion || handoff.Kind != KindHandoff || handoff.AttemptID != attemptID || handoff.Owner != intent.Owner || !strings.EqualFold(handoff.IntentSHA256, hash(intentBytes)) {
		return Inspection{}, fmt.Errorf("invalid member execution handoff binding")
	}
	manifestRel := filepath.ToSlash(filepath.Join(rootRel, "result", "manifest.json"))
	outputsRel := filepath.ToSlash(filepath.Join(rootRel, "result", "outputs"))
	if handoff.ManifestPath != manifestRel || handoff.OutputsRoot != outputsRel {
		return Inspection{}, fmt.Errorf("invalid member execution handoff result paths")
	}
	if commit.SchemaVersion != SchemaVersion || commit.Kind != KindCommit || commit.AttemptID != attemptID || !strings.EqualFold(commit.IntentSHA256, hash(intentBytes)) || !strings.EqualFold(commit.HandoffSHA256, hash(handoffBytes)) {
		return Inspection{}, fmt.Errorf("invalid member execution commit binding")
	}
	inspection := Inspection{State: "handoff-ready", AttemptID: attemptID, Owner: intent.Owner, Intent: &intent, Handoff: &handoff, AttemptRoot: root, ManifestPath: filepath.Join(root, "result", "manifest.json"), OutputsRoot: filepath.Join(root, "result", "outputs")}
	observationRel := filepath.Join(rootRel, "observations")
	entries, err := anchor.readDir(observationRel)
	if err != nil && !os.IsNotExist(err) {
		return Inspection{}, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	previous := ""
	for index, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() || !entry.Type().IsRegular() {
			return Inspection{}, fmt.Errorf("member execution observations contain invalid entry: %s", entry.Name())
		}
		data, err := anchor.readFile(filepath.Join(observationRel, entry.Name()), maxJSONBytes)
		if err != nil {
			return Inspection{}, err
		}
		var observation Observation
		if err := strictCanonical(data, &observation); err != nil {
			return Inspection{}, err
		}
		expectedName := fmt.Sprintf("%020d-%s.json", index+1, observation.Outcome)
		validTransition := (index == 0 && (observation.Outcome == "accepted" || observation.Outcome == "failed")) || (index == 1 && previous == "accepted" && (observation.Outcome == "returned" || observation.Outcome == "failed"))
		if entry.Name() != expectedName || !validTransition || observation.SchemaVersion != SchemaVersion || observation.Kind != KindObservation || observation.AttemptID != attemptID || observation.Owner != intent.Owner || observation.Sequence != index+1 || !strings.EqualFold(observation.IntentSHA256, hash(intentBytes)) || !observation.NoAuthority || !observation.NoConfirmed || !observation.NoHeavyTool {
			return Inspection{}, fmt.Errorf("invalid member execution observation chain")
		}
		if observation.Outcome == "returned" && !validSHA(observation.ManifestSHA256) {
			return Inspection{}, fmt.Errorf("returned member execution observation requires manifest sha256")
		}
		if observation.Outcome != "returned" && observation.ManifestSHA256 != "" {
			return Inspection{}, fmt.Errorf("non-returned member execution observation must not bind a manifest")
		}
		copy := observation
		inspection.Latest = &copy
		inspection.State = observation.Outcome
		previous = observation.Outcome
	}
	if inspection.Latest != nil && inspection.Latest.Outcome == "returned" {
		inspection.ManifestPath, inspection.OutputsRoot = canonicalResultPaths(inspection.AttemptRoot)
		manifest, sum, err := inspectManifestAnchored(anchor, inspection)
		if err != nil {
			return Inspection{}, err
		}
		if !strings.EqualFold(sum, inspection.Latest.ManifestSHA256) {
			return Inspection{}, fmt.Errorf("member execution manifest drift after returned observation")
		}
		inspection.Manifest, inspection.ManifestSHA256, inspection.State = &manifest, sum, "intake-ready"
	}
	return inspection, nil
}

func currentOwner(caseRoot, pack, lane string) (string, Owner, error) {
	if !segment.MatchString(lane) {
		return "", Owner{}, fmt.Errorf("invalid member execution lane")
	}
	anchor, err := openAnchoredCase(caseRoot)
	if err != nil {
		return "", Owner{}, err
	}
	defer anchor.Close()
	owner, err := currentOwnerAnchored(anchor, pack, lane)
	if err != nil {
		return "", Owner{}, err
	}
	if err := anchor.revalidate(); err != nil {
		return "", Owner{}, err
	}
	return anchor.path, owner, nil
}

func currentOwnerAnchored(anchor *anchoredCase, pack, lane string) (Owner, error) {
	data, err := anchor.readFile(filepath.Join(".rekit", "board.json"), maxJSONBytes)
	if err != nil {
		return Owner{}, err
	}
	var board mission.Board
	if err := json.Unmarshal(data, &board); err != nil {
		return Owner{}, fmt.Errorf("invalid member execution board: %w", err)
	}
	if !samePath(board.CaseRoot, anchor.path) || !strings.EqualFold(board.Pack, pack) {
		return Owner{}, fmt.Errorf("member execution board identity mismatch")
	}
	entry, ok := mission.LookupBoardLane(board.Lanes, lane, false)
	if !ok {
		return Owner{}, fmt.Errorf("member execution lane is not on board")
	}
	if strings.TrimSpace(entry.CurrentExecutor) == "" || entry.ExecutorGeneration < 1 {
		return Owner{}, fmt.Errorf("member execution lane has no current executor generation")
	}
	return Owner{Lane: entry.ID, Executor: entry.CurrentExecutor, ExecutorGeneration: entry.ExecutorGeneration}, nil
}

func currentOwnerUpdatedAt(caseRoot, lane string) (string, error) {
	anchor, err := openAnchoredCase(caseRoot)
	if err != nil {
		return "", err
	}
	defer anchor.Close()
	data, err := anchor.readFile(filepath.Join(".rekit", "board.json"), maxJSONBytes)
	if err != nil {
		return "", err
	}
	var board mission.Board
	if err := json.Unmarshal(data, &board); err != nil {
		return "", err
	}
	entry, ok := mission.LookupBoardLane(board.Lanes, lane, false)
	if !ok || strings.TrimSpace(entry.UpdatedAt) == "" {
		return "", fmt.Errorf("member execution lane has no durable updatedAt")
	}
	return entry.UpdatedAt, nil
}

func canonicalResultPaths(attemptRoot string) (string, string) {
	root := filepath.Join(attemptRoot, "evidence")
	return filepath.Join(root, "manifest.json"), filepath.Join(root, "outputs")
}

func snapshotResultPlan(inspection Inspection) (ResultManifest, string, []plannedWrite, error) {
	anchor, err := openAnchoredCase(inspection.Intent.CaseRoot)
	if err != nil {
		return ResultManifest{}, "", nil, err
	}
	defer anchor.Close()
	manifest, sum, err := inspectManifestAnchored(anchor, inspection)
	if err != nil {
		return ResultManifest{}, "", nil, err
	}
	manifestData, err := canonical(manifest)
	if err != nil {
		return ResultManifest{}, "", nil, err
	}
	manifestPath, outputsRoot := canonicalResultPaths(inspection.AttemptRoot)
	sourceOutputsRoot, err := relativeToCase(anchor.path, inspection.OutputsRoot)
	if err != nil {
		return ResultManifest{}, "", nil, err
	}
	writes := []plannedWrite{}
	for _, output := range manifest.Outputs {
		data, err := anchor.readFile(filepath.Join(sourceOutputsRoot, filepath.FromSlash(output.Path)), output.Bytes)
		if err != nil {
			return ResultManifest{}, "", nil, err
		}
		if int64(len(data)) != output.Bytes || !strings.EqualFold(hash(data), output.SHA256) {
			return ResultManifest{}, "", nil, fmt.Errorf("member execution output changed while snapshotting: %s", output.Path)
		}
		writes = append(writes, plannedWrite{filepath.Join(outputsRoot, filepath.FromSlash(output.Path)), data})
	}
	if err := anchor.revalidate(); err != nil {
		return ResultManifest{}, "", nil, err
	}
	writes = append(writes, plannedWrite{manifestPath, manifestData})
	return manifest, sum, writes, nil
}

func inspectManifestAnchored(anchor *anchoredCase, inspection Inspection) (ResultManifest, string, error) {
	manifestRel, err := relativeToCase(anchor.path, inspection.ManifestPath)
	if err != nil {
		return ResultManifest{}, "", err
	}
	data, err := anchor.readFile(manifestRel, maxJSONBytes)
	if err != nil {
		return ResultManifest{}, "", err
	}
	var manifest ResultManifest
	if err := strictCanonical(data, &manifest); err != nil {
		return ResultManifest{}, "", err
	}
	if manifest.SchemaVersion != SchemaVersion || manifest.Kind != KindManifest || manifest.AttemptID != inspection.AttemptID || manifest.Owner != inspection.Owner || strings.TrimSpace(manifest.Summary) == "" || len(manifest.Outputs) == 0 || len(manifest.Outputs) > maxOutputs || !manifest.NoAuthority || !manifest.NoConfirmed || !manifest.NoHeavyTool {
		return ResultManifest{}, "", fmt.Errorf("invalid member execution result manifest")
	}
	outputsRel, err := relativeToCase(anchor.path, inspection.OutputsRoot)
	if err != nil {
		return ResultManifest{}, "", err
	}
	seen := map[string]bool{}
	for _, output := range manifest.Outputs {
		key := strings.ToLower(output.Path)
		if !validRelative(output.Path) || seen[key] || !validSHA(output.SHA256) || output.Bytes < 1 || output.Bytes > maxOutputBytes {
			return ResultManifest{}, "", fmt.Errorf("invalid member execution output contract: %s", output.Path)
		}
		seen[key] = true
		content, err := anchor.readFile(filepath.Join(outputsRel, filepath.FromSlash(output.Path)), output.Bytes)
		if err != nil {
			return ResultManifest{}, "", err
		}
		if int64(len(content)) != output.Bytes || !strings.EqualFold(hash(content), output.SHA256) {
			return ResultManifest{}, "", fmt.Errorf("member execution output hash or size drift: %s", output.Path)
		}
	}
	expectedEntries := map[string]bool{}
	for path := range seen {
		expectedEntries[path] = true
		for parent := filepath.ToSlash(filepath.Dir(path)); parent != "."; parent = filepath.ToSlash(filepath.Dir(parent)) {
			expectedEntries[strings.ToLower(parent)+"/"] = true
		}
	}
	actual, err := memberOutputPathsAnchored(anchor, outputsRel, "")
	if err != nil {
		return ResultManifest{}, "", err
	}
	if len(actual) != len(expectedEntries) {
		return ResultManifest{}, "", fmt.Errorf("member execution outputs do not exactly match manifest")
	}
	for _, path := range actual {
		if !expectedEntries[strings.ToLower(filepath.ToSlash(path))] {
			return ResultManifest{}, "", fmt.Errorf("member execution output component is not declared by manifest: %s", path)
		}
	}
	if manifest.ReviewerItemsPath != "" && (!validRelative(manifest.ReviewerItemsPath) || !seen[strings.ToLower(manifest.ReviewerItemsPath)]) {
		return ResultManifest{}, "", fmt.Errorf("reviewerItemsPath must name a declared output")
	}
	return manifest, hash(data), nil
}

func memberOutputPathsAnchored(anchor *anchoredCase, rootRel, prefix string) ([]string, error) {
	entries, err := anchor.readDir(rootRel)
	if err != nil {
		return nil, err
	}
	paths := []string{}
	for _, entry := range entries {
		name := filepath.ToSlash(filepath.Join(prefix, entry.Name()))
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("member execution outputs contain symlink entry: %s", name)
		}
		if entry.IsDir() {
			paths = append(paths, name+"/")
			nested, err := memberOutputPathsAnchored(anchor, filepath.Join(rootRel, entry.Name()), name)
			if err != nil {
				return nil, err
			}
			paths = append(paths, nested...)
			continue
		}
		if !entry.Type().IsRegular() {
			return nil, fmt.Errorf("member execution outputs contain invalid entry: %s", name)
		}
		paths = append(paths, name)
	}
	sort.Strings(paths)
	return paths, nil
}

func Latest(caseRoot, lane string) (Inspection, bool, error) {
	if !segment.MatchString(lane) {
		return Inspection{}, false, fmt.Errorf("invalid member execution lane")
	}
	anchor, err := openAnchoredCase(caseRoot)
	if err != nil {
		return Inspection{}, false, err
	}
	defer anchor.Close()
	rootRel := filepath.Join(".rekit", "lanes", lane, "member-executions")
	entries, err := anchor.readDir(rootRel)
	if os.IsNotExist(err) {
		return Inspection{}, false, nil
	}
	if err != nil {
		return Inspection{}, false, err
	}
	names := []string{}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() || !segment.MatchString(entry.Name()) {
			return Inspection{}, false, fmt.Errorf("member execution root contains invalid attempt entry: %s", entry.Name())
		}
		names = append(names, entry.Name())
	}
	if len(names) == 0 {
		return Inspection{}, false, nil
	}
	sort.Strings(names)
	inspection, err := inspectAnchored(anchor, lane, names[len(names)-1])
	if err != nil {
		return Inspection{}, false, err
	}
	if err := anchor.revalidate(); err != nil {
		return Inspection{}, false, err
	}
	return inspection, true, nil
}

func MarshalResultManifest(manifest ResultManifest) ([]byte, error) {
	return canonical(manifest)
}

func IsPendingDispatch(err error) bool {
	return os.IsNotExist(err)
}

func nextAttemptSequence(caseRoot string, owner Owner, requestSHA string) (int, error) {
	anchor, err := openAnchoredCase(caseRoot)
	if err != nil {
		return 0, err
	}
	defer anchor.Close()
	rootRel := filepath.Join(".rekit", "lanes", owner.Lane, "member-executions")
	entries, readErr := anchor.readDir(rootRel)
	if os.IsNotExist(readErr) {
		return 1, nil
	}
	if readErr != nil {
		return 0, readErr
	}
	maxSequence := 0
	latestName := ""
	prefix := fmt.Sprintf("g%06d-a", owner.ExecutorGeneration)
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() || !segment.MatchString(entry.Name()) {
			return 0, fmt.Errorf("member execution root contains invalid attempt entry: %s", entry.Name())
		}
		parts := strings.Split(entry.Name(), "-")
		if !strings.HasPrefix(entry.Name(), prefix) || len(parts) < 3 {
			continue
		}
		sequence, parseErr := strconv.Atoi(strings.TrimPrefix(parts[1], "a"))
		if parseErr != nil || sequence < 1 {
			return 0, fmt.Errorf("invalid member execution attempt sequence: %s", entry.Name())
		}
		if sequence > maxSequence {
			maxSequence = sequence
			latestName = entry.Name()
		}
	}
	if latestName == "" {
		return 1, nil
	}
	inspection, inspectErr := inspectAnchored(anchor, owner.Lane, latestName)
	if inspectErr == nil {
		if inspection.Owner == owner && inspection.Intent != nil && strings.EqualFold(inspection.Intent.RequestSHA256, requestSHA) && inspection.Latest == nil {
			return maxSequence, nil
		}
		return maxSequence + 1, nil
	}
	intentData, intentErr := anchor.readFile(filepath.Join(rootRel, latestName, "intent.json"), maxJSONBytes)
	if intentErr != nil {
		return 0, fmt.Errorf("member execution attempt is missing durable intent: %s: %w", latestName, intentErr)
	}
	var intent Intent
	if err := strictCanonical(intentData, &intent); err != nil || intent.SchemaVersion != SchemaVersion || intent.Kind != KindIntent || intent.AttemptID != latestName || !samePath(intent.CaseRoot, anchor.path) || intent.Owner != owner || !strings.EqualFold(intent.RequestSHA256, requestSHA) || !intent.NoSpawn || !intent.NoPoll || !intent.NoStop || !intent.NoAuthority || !intent.NoConfirmed || !intent.NoHeavyTool {
		return 0, fmt.Errorf("pending member execution attempt does not match reviewed dispatch identity: %s", latestName)
	}
	if err := anchor.revalidate(); err != nil {
		return 0, err
	}
	return maxSequence, nil
}

func attemptRoot(caseRoot, lane, attempt string) (string, error) {
	if !segment.MatchString(lane) || !segment.MatchString(attempt) {
		return "", fmt.Errorf("invalid member execution path segment")
	}
	root := filepath.Join(caseRoot, ".rekit", "lanes", lane, "member-executions", attempt)
	if !contained(caseRoot, root) {
		return "", fmt.Errorf("member execution root escapes case")
	}
	return root, nil
}

func finishPlan(plan Plan) (Plan, error) {
	value := struct {
		SchemaVersion                   int `json:"schemaVersion"`
		Mode, CaseRoot, Pack, AttemptID string
		Owner                           Owner
		Outcome, Actor, Reason          string
		Inspection                      Inspection
	}{plan.SchemaVersion, plan.Mode, plan.CaseRoot, plan.Pack, plan.AttemptID, plan.Owner, plan.Outcome, plan.Actor, plan.Reason, plan.Inspection}
	data, err := json.Marshal(value)
	if err != nil {
		return Plan{}, err
	}
	plan.ExpectedPlanSHA256 = hash(data)
	return plan, nil
}

func strictCanonical(data []byte, out any) error {
	if err := decodeStrict(data, out); err != nil {
		return err
	}
	expected, err := canonical(out)
	if err != nil || !bytes.Equal(data, expected) {
		return fmt.Errorf("member execution JSON is not canonical")
	}
	return nil
}
func decodeStrict(data []byte, out any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("member execution JSON contains trailing data")
	}
	return nil
}
func canonical(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
func hash(data []byte) string            { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func hashMustCanonical(value any) string { data, _ := canonical(value); return hash(data) }
func validSHA(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
func parseTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Now().UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid member execution time: %w", err)
	}
	return parsed.UTC(), nil
}
func validRelative(value string) bool {
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	return value != "" && clean == value && clean != "." && clean != ".." && !strings.HasPrefix(clean, "../") && !filepath.IsAbs(filepath.FromSlash(value))
}
func contained(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
func samePath(left, right string) bool {
	left, _ = filepath.Abs(left)
	right, _ = filepath.Abs(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return filepath.Clean(left) == filepath.Clean(right)
}
func rel(root, path string) string {
	value, _ := filepath.Rel(root, path)
	return filepath.ToSlash(value)
}
func boundaries() []string {
	return []string{"external harness owns member session lifecycle; Go does not spawn, poll, stop, or manage sessions", "member execution does not execute heavy tools and does not write authority or confirmed state", "owner executor generation and exact output hashes are required for intake"}
}
