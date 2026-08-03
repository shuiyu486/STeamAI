package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewpath"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
	"github.com/shuiyu486/re-context-kits/internal/rekit/subagents"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

var reviewerWaveBeforeApplyObservationHook func(int) error
var reviewerWaveBeforeObservationInterventionCheckHook func(int) error
var reviewerWaveBeforeObservationOpenHook func() error
var reviewerWaveBeforeReturnedCompletionHook func() error

type reviewerWaveObservationFile struct {
	SchemaVersion int                       `json:"schemaVersion"`
	PacketID      string                    `json:"packetId"`
	Observations  []reviewerWaveObservation `json:"observations"`
}

type reviewerWaveObservation struct {
	ShardID                       string `json:"shardId"`
	Kind                          string `json:"kind"`
	ReviewerHarness               string `json:"reviewerHarness,omitempty"`
	ReviewerSession               string `json:"reviewerSession,omitempty"`
	ReviewerDispatchID            string `json:"reviewerDispatchId,omitempty"`
	ReviewerExitStatus            string `json:"reviewerExitStatus,omitempty"`
	ReviewerResultInputSourcePath string `json:"reviewerResultInputSourcePath,omitempty"`
}

type reviewerWaveObservationApplyResult struct {
	Mutated bool
}

type reviewerWaveObservationPreview struct {
	Index                        int    `json:"index"`
	ShardID                      string `json:"shardId"`
	Kind                         string `json:"kind"`
	ExpectedBindingSHA256        string `json:"expectedBindingSha256,omitempty"`
	ExpectedDispatchSHA256       string `json:"expectedDispatchSha256,omitempty"`
	ExpectedReviewerResultSHA256 string `json:"expectedReviewerResultSha256,omitempty"`
	ExpectedInputSaveSHA256      string `json:"expectedInputSaveSha256,omitempty"`
	DispatchID                   string `json:"dispatchId,omitempty"`
	Status                       string `json:"status"`
}

type reviewerWavePlan struct {
	SchemaVersion                  int                                     `json:"schemaVersion"`
	Command                        string                                  `json:"command"`
	CaseRoot                       string                                  `json:"caseRoot"`
	Pack                           string                                  `json:"pack"`
	PacketID                       string                                  `json:"packetId"`
	PacketPath                     string                                  `json:"packetPath"`
	Lane                           string                                  `json:"lane"`
	Actor                          string                                  `json:"actor"`
	WaveSnapshotSHA256             string                                  `json:"waveSnapshotSha256"`
	ObservationPath                string                                  `json:"observationPath"`
	ObservationSHA256              string                                  `json:"observationSha256"`
	ObservationCount               int                                     `json:"observationCount"`
	Previews                       []reviewerWaveObservationPreview        `json:"previews"`
	ExpectedReviewerWavePlanSHA256 string                                  `json:"expectedReviewerWavePlanSha256,omitempty"`
	IsMutation                     bool                                    `json:"isMutation"`
	Applied                        bool                                    `json:"applied"`
	AppliedCount                   int                                     `json:"appliedCount"`
	FailedIndex                    int                                     `json:"failedIndex,omitempty"`
	Failure                        string                                  `json:"failure,omitempty"`
	RefreshedWave                  *workstream.ReviewerDispatchWavePackage `json:"refreshedWave,omitempty"`
	Boundary                       []string                                `json:"boundary"`
}

type reviewerWavePlanIdentity struct {
	PacketID           string                           `json:"packetId"`
	PacketPath         string                           `json:"packetPath"`
	Lane               string                           `json:"lane"`
	Actor              string                           `json:"actor"`
	WaveSnapshotSHA256 string                           `json:"waveSnapshotSha256"`
	ObservationSHA256  string                           `json:"observationSha256"`
	Observations       []reviewerWaveObservation        `json:"observations"`
	Previews           []reviewerWaveObservationPreview `json:"previews"`
}

func runReviewerWave(ctx runtime.Context, opt Options, out io.Writer) error {
	if !ctx.TargetProvided {
		return fmt.Errorf("run-reviewer-wave requires -Target for an attached case")
	}
	if opt.WhatIf == opt.Apply {
		return fmt.Errorf("run-reviewer-wave requires exactly one of -WhatIf or -Apply")
	}
	if strings.ToLower(strings.TrimSpace(opt.Format)) != "json" {
		return fmt.Errorf("run-reviewer-wave supports only -Format json")
	}
	if strings.TrimSpace(opt.PacketPath) == "" || strings.TrimSpace(opt.Note.Lane) == "" || strings.TrimSpace(opt.ReviewerWaveObservationsPath) == "" || strings.TrimSpace(opt.Note.Actor) == "" {
		return fmt.Errorf("run-reviewer-wave requires -PacketPath, -Lane, -ReviewerWaveObservationsPath, and -Actor")
	}
	plan, observations, err := buildReviewerWavePlan(ctx, opt)
	if err != nil {
		return err
	}
	if opt.WhatIf {
		if strings.TrimSpace(opt.ExpectedReviewerWavePlanSHA256) != "" {
			return fmt.Errorf("run-reviewer-wave -WhatIf does not accept -ExpectedReviewerWavePlanSha256")
		}
		return writeJSON(out, plan)
	}
	if !strings.EqualFold(strings.TrimSpace(opt.ExpectedReviewerWavePlanSHA256), plan.ExpectedReviewerWavePlanSHA256) {
		return fmt.Errorf("run-reviewer-wave expected plan sha256 mismatch: got %s want %s", strings.TrimSpace(opt.ExpectedReviewerWavePlanSHA256), plan.ExpectedReviewerWavePlanSHA256)
	}
	plan.IsMutation = true
	for idx, observation := range observations {
		if reviewerWaveBeforeApplyObservationHook != nil {
			if err := reviewerWaveBeforeApplyObservationHook(idx + 1); err != nil {
				return writeReviewerWavePartialFailure(ctx, opt, out, &plan, idx+1, err)
			}
		}
		if reviewerWaveBeforeObservationInterventionCheckHook != nil {
			if err := reviewerWaveBeforeObservationInterventionCheckHook(idx + 1); err != nil {
				return writeReviewerWavePartialFailure(ctx, opt, out, &plan, idx+1, err)
			}
		}
		result, err := applyReviewerWaveObservationWithInterventionGuard(ctx, opt, observation, plan.Previews[idx])
		if err != nil {
			plan.Applied = plan.AppliedCount > 0 || result.Mutated
			plan.FailedIndex = idx + 1
			plan.Failure = err.Error()
			refreshed, _ := buildStatusInventory(ctx, statusPackSource(ctx, opt))
			plan.RefreshedWave = reviewerWaveFromStatus(refreshed)
			return writeJSON(out, plan)
		}
		plan.AppliedCount++
	}
	plan.Applied = true
	refreshed, err := buildStatusInventory(ctx, statusPackSource(ctx, opt))
	if err != nil {
		return err
	}
	plan.RefreshedWave = reviewerWaveFromStatus(refreshed)
	return writeJSON(out, plan)
}

func writeReviewerWavePartialFailure(ctx runtime.Context, opt Options, out io.Writer, plan *reviewerWavePlan, failedIndex int, cause error) error {
	plan.Applied = plan.AppliedCount > 0
	plan.FailedIndex = failedIndex
	plan.Failure = cause.Error()
	refreshed, _ := buildStatusInventory(ctx, statusPackSource(ctx, opt))
	plan.RefreshedWave = reviewerWaveFromStatus(refreshed)
	return writeJSON(out, plan)
}

func ensureReviewerWaveLaneNotIntervened(caseRoot, lane string) error {
	facts, err := mission.ReadStrictLedgerFacts(caseRoot)
	if err != nil {
		return err
	}
	if interventions := mission.EffectiveOpenLaneInterventions(facts.Facts, lane); len(interventions) > 0 {
		return fmt.Errorf("run-reviewer-wave is paused by open intervention %q on lane %q; reconcile the intervention and refresh status before recording reviewer observations", mission.Value(interventions[0], "eventId"), lane)
	}
	return nil
}

func buildReviewerWavePlan(ctx runtime.Context, opt Options) (reviewerWavePlan, []reviewerWaveObservation, error) {
	status, err := buildStatusInventory(ctx, statusPackSource(ctx, opt))
	if err != nil {
		return reviewerWavePlan{}, nil, err
	}
	wave := reviewerWaveFromStatus(status)
	if wave == nil {
		return reviewerWavePlan{}, nil, fmt.Errorf("run-reviewer-wave requires a ready reviewerDispatch operator wave")
	}
	if !sameReviewerWavePath(opt.PacketPath, wave.PacketPath) || strings.TrimSpace(opt.Note.Lane) != strings.TrimSpace(wave.TargetLane) {
		return reviewerWavePlan{}, nil, fmt.Errorf("run-reviewer-wave packet or lane does not match the current reviewer wave")
	}
	if err := ensureReviewerWaveLaneNotIntervened(ctx.Target, wave.TargetLane); err != nil {
		return reviewerWavePlan{}, nil, err
	}
	if !wave.Ready {
		return reviewerWavePlan{}, nil, fmt.Errorf("run-reviewer-wave requires a ready reviewerDispatch operator wave")
	}
	path, data, observations, err := readReviewerWaveObservations(ctx.Target, opt.ReviewerWaveObservationsPath)
	if err != nil {
		return reviewerWavePlan{}, nil, err
	}
	if observations.PacketID != wave.PacketID {
		return reviewerWavePlan{}, nil, fmt.Errorf("reviewer wave observation packetId %q does not match current packet %q", observations.PacketID, wave.PacketID)
	}
	if len(observations.Observations) == 0 {
		return reviewerWavePlan{}, nil, fmt.Errorf("reviewer wave observation file requires at least one observation")
	}
	plan := reviewerWavePlan{SchemaVersion: 1, Command: commands.RunReviewerWave, CaseRoot: ctx.Target, Pack: ctx.Pack, PacketID: wave.PacketID, PacketPath: wave.PacketPath, Lane: wave.TargetLane, Actor: strings.TrimSpace(opt.Note.Actor), WaveSnapshotSHA256: wave.SnapshotSHA256, ObservationPath: path, ObservationSHA256: reviewerWaveSHA256(data), ObservationCount: len(observations.Observations), Boundary: []string{"preview binds the full current wave snapshot and exact observation file before any receipt or result-input write", "observations are applied in file order and each shard retains its existing independent lock, prompt, receipt, result, and intake guards", "a failed observation stops the bundle, reports its index, preserves earlier successful immutable writes, and leaves later observations unapplied", "runtime records explicit external harness observations only; it does not spawn, poll, stop, or manage reviewer sessions", "run-reviewer-wave does not execute heavy tools or write authority/confirmed state"}}
	spawnable := map[string]bool{}
	for _, item := range wave.SpawnWave {
		spawnable[strings.TrimSpace(item.ShardID)] = true
	}
	active := map[string]workstream.ReviewerDispatchWavePackageItem{}
	for _, item := range wave.Active {
		active[strings.TrimSpace(item.ShardID)] = item
	}
	seen := map[string]bool{}
	for idx, observation := range observations.Observations {
		shardID := strings.TrimSpace(observation.ShardID)
		if seen[shardID] {
			return reviewerWavePlan{}, nil, fmt.Errorf("reviewer wave observation file repeats shardId %q", shardID)
		}
		seen[shardID] = true
		if err := validateReviewerWaveObservationShape(observation); err != nil {
			return reviewerWavePlan{}, nil, fmt.Errorf("reviewer wave observation %d shard %s: %w", idx+1, shardID, err)
		}
		switch strings.TrimSpace(observation.Kind) {
		case "accepted":
			if !spawnable[shardID] {
				return reviewerWavePlan{}, nil, fmt.Errorf("reviewer wave observation %d shard %s: accepted observation requires a shard in the current spawnWave", idx+1, shardID)
			}
		case "returned", "failed":
			item, ok := active[shardID]
			if !ok {
				return reviewerWavePlan{}, nil, fmt.Errorf("reviewer wave observation %d shard %s: terminal observation requires a shard in the current active wave", idx+1, shardID)
			}
			if strings.TrimSpace(observation.Kind) == "failed" && strings.TrimSpace(observation.ReviewerDispatchID) != strings.TrimSpace(item.ReviewerDispatchID) {
				return reviewerWavePlan{}, nil, fmt.Errorf("reviewer wave observation %d shard %s: failed observation dispatch does not match the current active shard", idx+1, shardID)
			}
		}
		preview, err := previewReviewerWaveObservation(ctx, opt, observation, idx, active[shardID].ReviewerDispatchID)
		if err != nil {
			return reviewerWavePlan{}, nil, fmt.Errorf("reviewer wave observation %d shard %s: %w", idx+1, observation.ShardID, err)
		}
		if strings.TrimSpace(observation.Kind) == "returned" && strings.TrimSpace(preview.DispatchID) != strings.TrimSpace(active[shardID].ReviewerDispatchID) {
			return reviewerWavePlan{}, nil, fmt.Errorf("reviewer wave observation %d shard %s: returned result dispatch does not match the current active shard", idx+1, shardID)
		}
		plan.Previews = append(plan.Previews, preview)
	}
	identity := reviewerWavePlanIdentity{PacketID: plan.PacketID, PacketPath: plan.PacketPath, Lane: plan.Lane, Actor: plan.Actor, WaveSnapshotSHA256: plan.WaveSnapshotSHA256, ObservationSHA256: plan.ObservationSHA256, Observations: observations.Observations, Previews: plan.Previews}
	encoded, _ := json.Marshal(identity)
	plan.ExpectedReviewerWavePlanSHA256 = reviewerWaveSHA256(encoded)
	return plan, observations.Observations, nil
}

func reviewerWaveFromStatus(status statusInventory) *workstream.ReviewerDispatchWavePackage {
	if status.CaseMission == nil || status.CaseMission.ReviewerDispatchIntakeSummary.OperatorPackage == nil {
		return nil
	}
	return status.CaseMission.ReviewerDispatchIntakeSummary.OperatorPackage.Wave
}

func pauseReviewerWaveForOpenIntervention(summary *workstream.ReviewerDispatchIntakeSummary, facts mission.Facts) {
	if summary == nil || summary.OperatorPackage == nil || summary.OperatorPackage.Wave == nil {
		return
	}
	pkg := summary.OperatorPackage
	wave := pkg.Wave
	interventions := mission.EffectiveOpenLaneInterventions(facts, wave.TargetLane)
	if len(interventions) == 0 {
		return
	}
	interventionID := mission.Value(interventions[0], "eventId")
	reason := fmt.Sprintf("reviewer wave is paused by open intervention %q on lane %q; reconcile the intervention and refresh status before dispatching or recording reviewer observations", interventionID, wave.TargetLane)
	pkg.Ready = false
	pkg.Paused = true
	pkg.PauseReason = reason
	pkg.InterventionID = interventionID
	pkg.CurrentDriverRequest = nil
	pkg.Summary = reason
	pkg.Boundary = mission.UniqueStrings(append(pkg.Boundary, "open lane intervention pauses reviewer dispatch and observation recording; active, returned, failed, blocked, and complete shards remain diagnostic only"))
	pauseReviewerOperatorItem(pkg.Current)
	for idx := range pkg.RunLoop {
		pkg.RunLoop[idx].Command = ""
		pkg.RunLoop[idx].PreviewCommand = ""
		pkg.RunLoop[idx].ApplyCommand = ""
		pkg.RunLoop[idx].AgentToolRequest = nil
	}
	wave.Ready = false
	wave.Paused = true
	wave.PauseReason = reason
	wave.InterventionID = interventionID
	wave.SnapshotSHA256 = ""
	wave.AvailableSlots = 0
	wave.SpawnWave = nil
	wave.Boundary = mission.UniqueStrings(append(wave.Boundary, "open lane intervention pauses new reviewer dispatch and observation recording; reconcile before using this wave"))
	pauseReviewerWaveItems(wave.Shards)
	pauseReviewerWaveItems(wave.Active)
	pauseReviewerWaveItems(wave.Returned)
	pauseReviewerWaveItems(wave.Failed)
	pauseReviewerWaveItems(wave.Blocked)
	pauseReviewerWaveItems(wave.Complete)
}

func pauseReviewerWaveItems(items []workstream.ReviewerDispatchWavePackageItem) {
	for idx := range items {
		items[idx].AgentToolRequest = nil
		items[idx].RecordDispatchCommand = ""
		items[idx].RecordCompletionCommand = ""
		items[idx].CurrentDriverRequest = nil
	}
}

func pauseReviewerOperatorItem(item *workstream.ReviewerDispatchOperatorPackageItem) {
	if item == nil {
		return
	}
	item.DispatchPromptRepairCommand = ""
	item.ReviewerDispatchRecordCommand = ""
	item.ReviewerCompletionRecordCommand = ""
	item.AgentToolRequest = nil
	item.ReviewerResultInputSavePreviewCommand = ""
	item.ReviewerResultInputSaveApplyCommand = ""
	item.ReviewerResultSourceCapturePreviewCommand = ""
	item.ReviewerResultSourceCaptureApplyCommand = ""
	item.ReviewerResultStagingPreviewCommand = ""
	item.ReviewerResultCollectionPreviewCommand = ""
	item.ReviewerResultCollectionApplyCommand = ""
	item.ReviewerResultIntakePreviewCommand = ""
	item.ReviewerResultIntakeApplyCommand = ""
	item.ReviewerResultBatchIntakePreviewCommand = ""
	item.ReviewerResultBatchIntakeApplyCommand = ""
	item.DispatchCommand = ""
	item.NextAction = ""
}

func executeReviewerMutationWithInterventionGuard[T any](caseRoot, lane string, whatIf bool, execute func() (T, error)) (result T, err error) {
	lane = strings.TrimSpace(lane)
	if lane == "" {
		return execute()
	}
	if whatIf {
		if err := lanemutation.AssertLaneOpen(caseRoot, lane, "reviewer mutation"); err != nil {
			return result, err
		}
		if err := ensureReviewerWaveLaneNotIntervened(caseRoot, lane); err != nil {
			return result, err
		}
		return execute()
	}
	lease, err := lanemutation.AcquireOpenLane(caseRoot, lane, "reviewer mutation")
	if err != nil {
		return result, currentStepZeroProgressError{cause: err}
	}
	defer func() {
		if unlockErr := lease.Unlock(); unlockErr != nil {
			err = errors.Join(err, unlockErr)
		}
	}()
	if err := lease.Validate(); err != nil {
		return result, currentStepZeroProgressError{cause: err}
	}
	if err := ensureReviewerWaveLaneNotIntervened(caseRoot, lane); err != nil {
		return result, currentStepZeroProgressError{cause: err}
	}
	return execute()
}

func readReviewerWaveObservations(caseRoot, requested string) (string, []byte, reviewerWaveObservationFile, error) {
	path, err := filepath.Abs(strings.TrimSpace(requested))
	if err != nil {
		return "", nil, reviewerWaveObservationFile{}, err
	}
	if !reviewpath.CollectionNamespacePathSafe(caseRoot, path, false) {
		return "", nil, reviewerWaveObservationFile{}, fmt.Errorf("reviewer wave observation file must be an existing symlink-free case-local file")
	}
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() <= 0 || before.Size() > 256*1024 {
		return "", nil, reviewerWaveObservationFile{}, fmt.Errorf("reviewer wave observation file must be a bounded non-empty regular file")
	}
	if reviewerWaveBeforeObservationOpenHook != nil {
		if err := reviewerWaveBeforeObservationOpenHook(); err != nil {
			return "", nil, reviewerWaveObservationFile{}, err
		}
	}
	file, err := os.Open(path)
	if err != nil {
		return "", nil, reviewerWaveObservationFile{}, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return "", nil, reviewerWaveObservationFile{}, fmt.Errorf("reviewer wave observation file changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(file, 256*1024+1))
	if err != nil {
		return "", nil, reviewerWaveObservationFile{}, err
	}
	if len(data) == 0 || len(data) > 256*1024 {
		return "", nil, reviewerWaveObservationFile{}, fmt.Errorf("reviewer wave observation file must be a bounded non-empty regular file")
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(opened, after) {
		return "", nil, reviewerWaveObservationFile{}, fmt.Errorf("reviewer wave observation file changed while reading")
	}
	var observations reviewerWaveObservationFile
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&observations); err != nil {
		return "", nil, reviewerWaveObservationFile{}, fmt.Errorf("decode reviewer wave observations: %w", err)
	}
	if observations.SchemaVersion != 1 {
		return "", nil, reviewerWaveObservationFile{}, fmt.Errorf("reviewer wave observation schemaVersion must be 1")
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return "", nil, reviewerWaveObservationFile{}, fmt.Errorf("reviewer wave observation file must contain exactly one JSON object")
	}
	return path, data, observations, nil
}

func validateReviewerWaveObservationShape(observation reviewerWaveObservation) error {
	shardID := strings.TrimSpace(observation.ShardID)
	kind := strings.TrimSpace(observation.Kind)
	if shardID == "" {
		return fmt.Errorf("shardId is required")
	}
	harness := strings.TrimSpace(observation.ReviewerHarness)
	session := strings.TrimSpace(observation.ReviewerSession)
	dispatchID := strings.TrimSpace(observation.ReviewerDispatchID)
	exitStatus := strings.TrimSpace(observation.ReviewerExitStatus)
	resultSource := strings.TrimSpace(observation.ReviewerResultInputSourcePath)
	switch kind {
	case "accepted":
		if harness == "" || session == "" {
			return fmt.Errorf("accepted observation requires reviewerHarness and reviewerSession")
		}
		if dispatchID != "" || exitStatus != "" || resultSource != "" {
			return fmt.Errorf("accepted observation does not accept terminal dispatch, exit-status, or result-input fields")
		}
	case "returned":
		if resultSource == "" {
			return fmt.Errorf("returned observation requires reviewerResultInputSourcePath")
		}
		if harness != "" || session != "" || dispatchID != "" {
			return fmt.Errorf("returned observation does not accept reviewerHarness, reviewerSession, or reviewerDispatchId")
		}
	case "failed":
		if dispatchID == "" || exitStatus == "" {
			return fmt.Errorf("failed observation requires reviewerDispatchId and reviewerExitStatus")
		}
		if harness != "" || session != "" || resultSource != "" {
			return fmt.Errorf("failed observation does not accept reviewerHarness, reviewerSession, or reviewerResultInputSourcePath")
		}
	default:
		return fmt.Errorf("kind must be accepted, returned, or failed")
	}
	return nil
}

func previewReviewerWaveObservation(ctx runtime.Context, opt Options, observation reviewerWaveObservation, idx int, currentDispatchID string) (reviewerWaveObservationPreview, error) {
	preview := reviewerWaveObservationPreview{Index: idx + 1, ShardID: strings.TrimSpace(observation.ShardID), Kind: strings.TrimSpace(observation.Kind), Status: "previewed"}
	if preview.ShardID == "" {
		return preview, fmt.Errorf("shardId is required")
	}
	switch preview.Kind {
	case "accepted":
		result, err := subagents.RecordReviewerSessionDispatch(ctx.RepoRoot, ctx.Target, ctx.Pack, subagents.ReviewerSessionDispatchOptions{PacketPath: opt.PacketPath, ShardID: preview.ShardID, Lane: opt.Note.Lane, Actor: opt.Note.Actor, ReviewerHarness: observation.ReviewerHarness, ReviewerSession: observation.ReviewerSession, WhatIf: true})
		if err != nil {
			return preview, err
		}
		preview.ExpectedBindingSHA256, preview.DispatchID = result.BindingSHA256, result.DispatchID
	case "returned":
		save, err := subagents.SaveReviewerResultInput(ctx.RepoRoot, ctx.Target, ctx.Pack, subagents.ReviewerResultInputSaveOptions{PacketPath: opt.PacketPath, ShardID: preview.ShardID, SourcePath: observation.ReviewerResultInputSourcePath, Lane: opt.Note.Lane, Actor: opt.Note.Actor, ExpectedReviewerDispatchID: currentDispatchID, WhatIf: true})
		if err != nil {
			return preview, err
		}
		preview.ExpectedInputSaveSHA256 = save.InputSourceSHA256
		preview.ExpectedDispatchSHA256 = save.ReviewerDispatchReceiptSHA256
		preview.ExpectedReviewerResultSHA256 = save.InputSourceSHA256
		preview.DispatchID = save.ReviewerDispatchID
	case "failed":
		completion, err := subagents.RecordReviewerSessionCompletion(ctx.RepoRoot, ctx.Target, ctx.Pack, subagents.ReviewerSessionCompletionOptions{PacketPath: opt.PacketPath, DispatchID: observation.ReviewerDispatchID, Lane: opt.Note.Lane, Actor: opt.Note.Actor, Outcome: "failed", ExitStatus: observation.ReviewerExitStatus, WhatIf: true})
		if err != nil {
			return preview, err
		}
		preview.ExpectedDispatchSHA256, preview.DispatchID = completion.DispatchReceiptSHA256, completion.DispatchID
	default:
		return preview, fmt.Errorf("kind must be accepted, returned, or failed")
	}
	return preview, nil
}

func applyReviewerWaveObservationWithInterventionGuard(ctx runtime.Context, opt Options, observation reviewerWaveObservation, preview reviewerWaveObservationPreview) (result reviewerWaveObservationApplyResult, err error) {
	lease, err := lanemutation.AcquireOpenLane(ctx.Target, strings.TrimSpace(opt.Note.Lane), "run-reviewer-wave")
	if err != nil {
		return reviewerWaveObservationApplyResult{}, err
	}
	defer func() {
		if unlockErr := lease.Unlock(); unlockErr != nil {
			err = errors.Join(err, unlockErr)
		}
	}()
	if err := lease.Validate(); err != nil {
		return reviewerWaveObservationApplyResult{}, err
	}
	if err := ensureReviewerWaveLaneNotIntervened(ctx.Target, strings.TrimSpace(opt.Note.Lane)); err != nil {
		return reviewerWaveObservationApplyResult{}, err
	}
	return applyReviewerWaveObservation(ctx, opt, observation, preview)
}

func applyReviewerWaveObservation(ctx runtime.Context, opt Options, observation reviewerWaveObservation, preview reviewerWaveObservationPreview) (reviewerWaveObservationApplyResult, error) {
	switch preview.Kind {
	case "accepted":
		_, err := subagents.RecordReviewerSessionDispatch(ctx.RepoRoot, ctx.Target, ctx.Pack, subagents.ReviewerSessionDispatchOptions{PacketPath: opt.PacketPath, ShardID: preview.ShardID, Lane: opt.Note.Lane, Actor: opt.Note.Actor, ReviewerHarness: observation.ReviewerHarness, ReviewerSession: observation.ReviewerSession, ExpectedBindingSHA256: preview.ExpectedBindingSHA256})
		return reviewerWaveObservationApplyResult{Mutated: err == nil}, err
	case "returned":
		save, err := subagents.SaveReviewerResultInput(ctx.RepoRoot, ctx.Target, ctx.Pack, subagents.ReviewerResultInputSaveOptions{PacketPath: opt.PacketPath, ShardID: preview.ShardID, SourcePath: observation.ReviewerResultInputSourcePath, Lane: opt.Note.Lane, Actor: opt.Note.Actor, ExpectedReviewerDispatchID: preview.DispatchID, ExpectedReviewerResultSHA256: preview.ExpectedInputSaveSHA256})
		if err != nil {
			return reviewerWaveObservationApplyResult{}, err
		}
		if reviewerWaveBeforeReturnedCompletionHook != nil {
			if err := reviewerWaveBeforeReturnedCompletionHook(); err != nil {
				return reviewerWaveObservationApplyResult{Mutated: true}, err
			}
		}
		_, err = subagents.RecordReviewerSessionCompletion(ctx.RepoRoot, ctx.Target, ctx.Pack, subagents.ReviewerSessionCompletionOptions{PacketPath: opt.PacketPath, DispatchID: save.ReviewerDispatchID, Lane: opt.Note.Lane, Actor: opt.Note.Actor, Outcome: "succeeded", ExitStatus: firstReviewerWaveText(observation.ReviewerExitStatus, "completed"), ReviewerResultInputPath: save.InputPath, ExpectedDispatchReceiptSHA256: preview.ExpectedDispatchSHA256, ExpectedReviewerResultSHA256: preview.ExpectedReviewerResultSHA256})
		return reviewerWaveObservationApplyResult{Mutated: true}, err
	case "failed":
		_, err := subagents.RecordReviewerSessionCompletion(ctx.RepoRoot, ctx.Target, ctx.Pack, subagents.ReviewerSessionCompletionOptions{PacketPath: opt.PacketPath, DispatchID: observation.ReviewerDispatchID, Lane: opt.Note.Lane, Actor: opt.Note.Actor, Outcome: "failed", ExitStatus: observation.ReviewerExitStatus, ExpectedDispatchReceiptSHA256: preview.ExpectedDispatchSHA256})
		return reviewerWaveObservationApplyResult{Mutated: err == nil}, err
	}
	return reviewerWaveObservationApplyResult{}, fmt.Errorf("unsupported reviewer wave observation kind %q", preview.Kind)
}

func sameReviewerWavePath(a, b string) bool {
	left, err := filepath.Abs(strings.TrimSpace(a))
	if err != nil {
		return false
	}
	right, err := filepath.Abs(strings.TrimSpace(b))
	return err == nil && strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func reviewerWaveSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func firstReviewerWaveText(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
