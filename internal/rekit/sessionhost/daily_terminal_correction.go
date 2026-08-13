package sessionhost

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/lanecompletion"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

const (
	dailyCorrectionRouteReviewer = "reviewer-rejection"
	dailyCorrectionRouteTerminal = "terminal-reopen"
)

var dailyTerminalCorrectionAfterPreviewHook func() error

type dailyCorrectionRoute struct {
	Lane string
	Kind string
}

func resolveDailyCorrectionRoute(caseRoot, pack, correction, actor, selected string, inspection missionintent.Inspection) (dailyCorrectionRoute, *DailyUserAction, error) {
	status, err := runPublicStatus(caseRoot, pack, "")
	if err != nil {
		return dailyCorrectionRoute{}, nil, fmt.Errorf("refresh daily correction status: %w", err)
	}
	if status.CaseMission == nil {
		return dailyCorrectionRoute{}, nil, fmt.Errorf("daily correction requires fresh typed case mission state")
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return dailyCorrectionRoute{}, nil, fmt.Errorf("daily correction requires an initialized board: %w", err)
	}

	selected = strings.TrimSpace(selected)
	if selected != "" {
		lane, ok := mission.LookupBoardLane(board.Lanes, selected, false)
		if !ok {
			return dailyCorrectionRoute{}, nil, fmt.Errorf("selected daily correction lane %q is not current", selected)
		}
		if strings.EqualFold(strings.TrimSpace(lane.Status), "archived") {
			return dailyCorrectionRoute{}, nil, fmt.Errorf("daily correction refuses archived lane %s because no durable archive transition is supported", selected)
		}
		route, err := classifyDailyCorrectionLane(caseRoot, correction, actor, dailyCorrectionScope(caseRoot, inspection), lane)
		if err != nil {
			return dailyCorrectionRoute{}, nil, err
		}
		if route.Kind == "" {
			return dailyCorrectionRoute{}, nil, fmt.Errorf("selected daily correction lane %s has neither a canonical reviewer rejection nor a committed completion", selected)
		}
		return route, nil, nil
	}

	routes := make([]dailyCorrectionRoute, 0, len(board.Lanes))
	choices := make([]DailyChoice, 0, len(board.Lanes))
	for _, lane := range board.Lanes {
		route, classifyErr := classifyDailyCorrectionLane(caseRoot, correction, actor, dailyCorrectionScope(caseRoot, inspection), lane)
		if classifyErr != nil {
			return dailyCorrectionRoute{}, nil, classifyErr
		}
		if route.Kind == "" {
			continue
		}
		routes = append(routes, route)
		choices = append(choices, DailyChoice{ID: lane.ID, Label: mission.BoardLaneLabel(lane)})
	}
	if len(routes) > 1 {
		return dailyCorrectionRoute{}, dailyLaneSelectionAction(choices), nil
	}
	if len(routes) == 1 {
		return routes[0], nil, nil
	}

	// Preserve the zero-write lane choice for multiple active member lanes.
	// The selected lane still has to pass canonical reviewer-lineage validation.
	for _, lane := range mission.OpenBoardLanes(board.Lanes) {
		if lane.Authority || strings.TrimSpace(lane.CurrentExecutor) == "" || lane.ExecutorGeneration < 1 {
			continue
		}
		choices = append(choices, DailyChoice{ID: lane.ID, Label: mission.BoardLaneLabel(lane)})
	}
	if len(choices) > 1 {
		return dailyCorrectionRoute{}, dailyLaneSelectionAction(choices), nil
	}
	return dailyCorrectionRoute{}, dailyAction(DailyActionBlocked), nil
}

func inspectDailyCorrectionMemberState(caseRoot string, lane mission.BoardLane) (bool, bool, error) {
	latest, found, err := memberexecution.Latest(caseRoot, lane.ID)
	if err != nil {
		return false, false, err
	}
	if !found || latest.State != "intake-ready" || latest.Manifest == nil || latest.Owner.Executor != lane.CurrentExecutor || latest.Owner.ExecutorGeneration != lane.ExecutorGeneration {
		return false, false, nil
	}
	targetRef := relativeLiveAcceptancePath(caseRoot, latest.ManifestPath)
	_, rejected, err := workstream.CurrentMemberManifestReviewerRejection(caseRoot, lane.ID, targetRef)
	if err != nil {
		return false, false, err
	}
	return true, rejected, nil
}

func classifyDailyCorrectionLane(caseRoot, correction, actor, scope string, lane mission.BoardLane) (dailyCorrectionRoute, error) {
	state := strings.ToLower(strings.TrimSpace(lane.Status))
	if state == "archived" {
		return dailyCorrectionRoute{}, nil
	}
	existing, err := existingDailyCorrectionForRequest(caseRoot, scope, lane.ID, correction, actor)
	if err != nil {
		return dailyCorrectionRoute{}, err
	}
	if existing != nil {
		return dailyCorrectionRoute{Lane: lane.ID, Kind: dailyCorrectionRouteReviewer}, nil
	}
	ready, rejected, err := inspectDailyCorrectionMemberState(caseRoot, lane)
	if err != nil {
		return dailyCorrectionRoute{}, err
	}
	if ready && rejected {
		if state == "closed" {
			return dailyCorrectionRoute{}, fmt.Errorf("closed daily correction lane %s still exposes a current reviewer rejection without its durable correction identity", lane.ID)
		}
		return dailyCorrectionRoute{Lane: lane.ID, Kind: dailyCorrectionRouteReviewer}, nil
	}

	lifecycle, err := lanecompletion.Inspect(caseRoot, lane.ID)
	if err != nil {
		return dailyCorrectionRoute{}, err
	}
	if state == "closed" {
		if lifecycle.State != lanecompletion.StateComplete || lifecycle.CurrentCompletion == nil {
			return dailyCorrectionRoute{}, fmt.Errorf("closed daily correction lane %s does not have a current committed completion: state=%s", lane.ID, lifecycle.State)
		}
		return dailyCorrectionRoute{Lane: lane.ID, Kind: dailyCorrectionRouteTerminal}, nil
	}
	if lifecycle.State == lanecompletion.StateComplete {
		return dailyCorrectionRoute{}, fmt.Errorf("daily correction lane %s has a committed completion but is not closed", lane.ID)
	}
	return dailyCorrectionRoute{}, nil
}

func resumeDailyTerminalCorrection(caseRoot, pack, correction, actor, selected string, result DailyResult) (bool, DailyResult, error) {
	operations, err := lanecompletion.InspectOperations(caseRoot)
	if err != nil {
		return true, result, fmt.Errorf("inspect resumable daily terminal correction: %w", err)
	}
	if operations.Pending {
		intent, err := lanecompletion.ReadOperationIntent(caseRoot, operations.PendingIntentPath)
		if err != nil {
			return true, result, fmt.Errorf("read pending daily terminal correction: %w", err)
		}
		return applyDailyTerminalCorrectionIntent(caseRoot, pack, correction, actor, selected, intent, false, result)
	}
	if len(operations.Commits) == 0 {
		return false, result, nil
	}
	selected = strings.TrimSpace(selected)
	for index := len(operations.Commits) - 1; index >= 0; index-- {
		commit := operations.Commits[index]
		intent, err := lanecompletion.ReadOperationIntent(caseRoot, lanecompletion.OperationIntentPath(caseRoot, commit.Sequence))
		if err != nil {
			return true, result, err
		}
		if intent.OperationID != commit.OperationID || intent.Sequence != commit.Sequence || intent.RequestedLane != commit.RequestedLane || intent.Actor != commit.Actor || intent.Reason != commit.Reason || !strings.EqualFold(intent.PreviewSHA256, commit.PreviewSHA256) {
			return true, result, fmt.Errorf("committed daily terminal correction intent differs from its commit: sequence=%d", commit.Sequence)
		}
		if selected != "" && selected != intent.RequestedLane && selected != intent.RequestedSelector {
			continue
		}
		if intent.Actor != actor || intent.Reason != correction {
			continue
		}
		lifecycle, err := lanecompletion.Inspect(caseRoot, intent.RequestedLane)
		if err != nil {
			return true, result, err
		}
		if lifecycle.State != lanecompletion.StateReopened || lifecycle.CurrentReopen == nil || lifecycle.CurrentReopen.OperationID != commit.OperationID {
			continue
		}
		return applyDailyTerminalCorrectionIntent(caseRoot, pack, correction, actor, selected, intent, true, result)
	}
	return false, result, nil
}

func applyDailyTerminalCorrectionIntent(caseRoot, pack, correction, actor, selected string, intent lanecompletion.OperationIntent, committed bool, result DailyResult) (bool, DailyResult, error) {
	if intent.Actor != actor || intent.Reason != correction {
		return true, result, fmt.Errorf("pending daily terminal correction belongs to a different actor or correction; recover the exact original correction")
	}
	selected = strings.TrimSpace(selected)
	if selected != "" && selected != intent.RequestedLane && selected != intent.RequestedSelector {
		return true, result, fmt.Errorf("pending daily terminal correction belongs to lane %s", intent.RequestedLane)
	}
	if len(intent.PreviewSHA256) != 64 || intent.PreviewSHA256 != intent.ExactPublicationSHA256 || strings.TrimSpace(intent.RequestedSelector) == "" {
		return true, result, fmt.Errorf("pending daily terminal correction omitted its exact reviewed identity")
	}
	applyCommand := dailyPendingReopenApplyCommand(intent)
	var applied workstream.ReopenResult
	if err := runPublicApplyCommand(applyCommand, "reopen", caseRoot, pack, &applied); err != nil {
		result = recordDailyReopenMutation(result, err)
		return true, result, fmt.Errorf("recover pending daily terminal correction Apply: %w", err)
	}
	commit := applied.OperationCommit
	if !applied.Applied || applied.RequestedLane != intent.RequestedLane || !strings.EqualFold(applied.ReopenPlanSHA256, intent.PreviewSHA256) || commit == nil || commit.OperationID != intent.OperationID || commit.State != "committed" || !commit.NoAuthority || !commit.NoConfirmed || !commit.NoHeavyTool || !commit.NoAutoResume {
		return true, result, fmt.Errorf("pending daily terminal correction recovery omitted the matching committed operation")
	}
	result.Lane = intent.RequestedLane
	result.ReopenOperationID = commit.OperationID
	result.ReopenOperationCommit = commit
	if !applied.Replay {
		result.DriverSteps = append(result.DriverSteps, "reopen")
	}
	if err := validateDailyTerminalReopen(caseRoot, pack, intent.RequestedLane, correction, actor, commit); err != nil {
		return true, result, err
	}
	if applied.Replay {
		result.DriverSteps = append(result.DriverSteps, "reopen")
	}
	result.FinalState = "terminal-correction-reopened"
	result.Replay = committed || applied.Replay
	result.Blocked = false
	result.Action = dailyAction(DailyActionReadyToContinue)
	return true, result, nil
}

func recordDailyReopenMutation(result DailyResult, err error) DailyResult {
	if workstream.IsZeroProgress(err) {
		return result
	}
	result.DriverSteps = append(result.DriverSteps, "reopen")
	return result
}

func dailyPendingReopenApplyCommand(intent lanecompletion.OperationIntent) string {
	parts := []string{
		"/rekit", "reopen", intent.RequestedSelector,
		"-Actor", intent.Actor,
		"-Reason", intent.Reason,
		"-EvidenceRefs", strings.Join(intent.EvidenceRefs, ","),
		"-ReopenPublicationStamp", intent.CreatedAt,
		"-ExpectedReopenPlanSha256", intent.PreviewSHA256,
		"-Apply", "-Format", "json",
	}
	for index := range parts {
		parts[index] = quotePublicCommandArg(parts[index])
	}
	return strings.Join(parts, " ")
}

func quotePublicCommandArg(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func runDailyTerminalCorrection(hostOpt Options, correction string, result DailyResult) (DailyResult, error) {
	lane := strings.TrimSpace(hostOpt.SelectedLane)
	if lane == "" {
		return result, fmt.Errorf("daily terminal correction requires one selected completed lane")
	}
	evidenceRef, err := dailyTerminalCorrectionEvidenceRef(result.CaseRoot, lane)
	if err != nil {
		return result, err
	}
	board, err := mission.ReadBoard(result.CaseRoot)
	if err != nil {
		return result, err
	}
	boardLane, ok := mission.LookupBoardLane(board.Lanes, lane, false)
	if !ok {
		return result, fmt.Errorf("daily terminal correction selected lane is no longer current: %s", lane)
	}
	selector := mission.BoardLaneLabel(boardLane)
	previewArgs := []string{
		"-Command", "reopen", selector, "-Target", result.CaseRoot, "-Pack", result.Pack,
		"-Actor", hostOpt.Actor, "-Reason", correction, "-EvidenceRefs", evidenceRef,
		"-WhatIf", "-Format", "json",
	}
	var preview workstream.ReopenResult
	if err := runPublicCLI(previewArgs, &preview); err != nil {
		return result, fmt.Errorf("daily terminal correction reopen preview: %w", err)
	}
	if preview.IsMutation || preview.Applied || !preview.RequiresConfirmation || preview.RequestedLane != lane || len(strings.TrimSpace(preview.ReopenPlanSHA256)) != 64 || strings.TrimSpace(preview.PublicationStamp) == "" || strings.TrimSpace(preview.ApplyCommand) == "" || len(preview.EffectiveTargets) == 0 {
		return result, fmt.Errorf("daily terminal correction reopen preview omitted the zero-write exact Apply request")
	}
	if dailyTerminalCorrectionAfterPreviewHook != nil {
		if err := dailyTerminalCorrectionAfterPreviewHook(); err != nil {
			return result, err
		}
	}
	var applied workstream.ReopenResult
	if err := runPublicApplyCommand(preview.ApplyCommand, "reopen", result.CaseRoot, result.Pack, &applied); err != nil {
		result = recordDailyReopenMutation(result, err)
		return result, fmt.Errorf("daily terminal correction reopen Apply: %w", err)
	}
	commit := applied.OperationCommit
	if !applied.Applied || applied.RequestedLane != lane || !strings.EqualFold(applied.ReopenPlanSHA256, preview.ReopenPlanSHA256) || commit == nil || commit.State != "committed" || !commit.NoAuthority || !commit.NoConfirmed || !commit.NoHeavyTool || !commit.NoAutoResume {
		return result, fmt.Errorf("daily terminal correction reopen Apply omitted the matching committed operation")
	}
	result.Lane = lane
	result.ReopenOperationID = commit.OperationID
	result.ReopenOperationCommit = commit
	result.DriverSteps = append(result.DriverSteps, "reopen")

	if err := validateDailyTerminalReopen(result.CaseRoot, result.Pack, lane, correction, hostOpt.Actor, commit); err != nil {
		return result, err
	}
	result.FinalState = "terminal-correction-reopened"
	result.Blocked = false
	result.Action = dailyAction(DailyActionReadyToContinue)
	return result, nil
}

func dailyTerminalCorrectionEvidenceRef(caseRoot, lane string) (string, error) {
	lifecycle, err := lanecompletion.Inspect(caseRoot, lane)
	if err != nil {
		return "", err
	}
	if lifecycle.State != lanecompletion.StateComplete || lifecycle.CurrentCompletion == nil || lifecycle.HeadKind != "complete" {
		return "", fmt.Errorf("daily terminal correction requires the current committed lane completion: %s state=%s", lane, lifecycle.State)
	}
	for index := len(lifecycle.Transitions) - 1; index >= 0; index-- {
		transition := lifecycle.Transitions[index]
		if transition.Sequence != lifecycle.HeadSequence || transition.Kind != "complete" || strings.TrimSpace(transition.ReceiptPath) == "" {
			continue
		}
		rel, err := filepath.Rel(filepath.Clean(caseRoot), filepath.Clean(transition.ReceiptPath))
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("daily terminal correction completion receipt escapes case root")
		}
		return filepath.ToSlash(rel), nil
	}
	return "", fmt.Errorf("daily terminal correction completion receipt is missing: %s", lane)
}

func validateDailyTerminalReopen(caseRoot, pack, lane, correction, actor string, commit *lanecompletion.OperationCommit) error {
	status, err := runPublicStatus(caseRoot, pack, lane)
	if err != nil {
		return fmt.Errorf("refresh daily terminal correction status: %w", err)
	}
	if status.CaseMission == nil {
		return fmt.Errorf("daily terminal correction reopen omitted fresh typed case mission state")
	}
	if commit == nil || len(commit.Targets) == 0 || commit.Actor != actor || commit.Reason != correction {
		return fmt.Errorf("daily terminal correction reopen omitted its committed request identity or targets")
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return err
	}
	requestedCurrent := false
	for _, target := range commit.Targets {
		lifecycle, err := lanecompletion.Inspect(caseRoot, target.Lane)
		if err != nil {
			return err
		}
		if lifecycle.State != lanecompletion.StateReopened || lifecycle.CurrentReopen == nil || lifecycle.CurrentReopen.OperationID != commit.OperationID || lifecycle.CurrentReopen.Reason != correction || lifecycle.CurrentReopen.Actor != actor || !lifecycle.CurrentReopen.NoAuthority || !lifecycle.CurrentReopen.NoConfirmed || !lifecycle.CurrentReopen.NoHeavyTool || !lifecycle.CurrentReopen.NoAutoResume {
			return fmt.Errorf("daily terminal correction reopen target differs from the committed current lane lifecycle: %s", target.Lane)
		}
		boardLane, ok := mission.LookupBoardLane(board.Lanes, target.Lane, false)
		if !ok || !strings.EqualFold(strings.TrimSpace(boardLane.Status), "open") || boardLane.ExecutorGeneration != lifecycle.CurrentReopen.ResultingExecutorGeneration || strings.TrimSpace(boardLane.CurrentExecutor) != "" {
			return fmt.Errorf("daily terminal correction reopen target differs from the current board lane: %s", target.Lane)
		}
		if target.Lane == lane {
			requestedCurrent = lifecycle.CurrentReopen.Reason == correction
		}
	}
	if !requestedCurrent {
		return fmt.Errorf("daily terminal correction reopen omitted the selected current lane")
	}
	choices := dailyStatusLaneChoices(status)
	if _, ok := dailyChoiceForLane(choices, lane); !ok {
		return fmt.Errorf("daily terminal correction reopen did not expose the selected lane as a fresh typed action")
	}
	return nil
}
