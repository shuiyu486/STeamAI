package sessionhost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	"github.com/shuiyu486/re-context-kits/internal/rekit/note"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

var dailyReviewerRunLoopSteps = map[string]struct{}{
	"verify-prompt":     {},
	"spawn-reviewer":    {},
	"save-result-input": {},
	"record-completion": {},
	"source-capture":    {},
	"stage-candidate":   {},
	"collect-result":    {},
	"intake-results":    {},
}

func dailyReviewerOwnerRequest(status publicStatus, selected string) bool {
	if status.MissionControlRunbook == nil ||
		status.MissionControlRunbook.Scope != "reviewer" {
		return false
	}
	request := status.MissionControlRunbook.CurrentDriverRequest
	selected = strings.TrimSpace(selected)
	if request == nil || request.Blocked || selected == "" ||
		strings.TrimSpace(request.Source) != "reviewerDispatchOperatorPackage" ||
		strings.TrimSpace(request.Lane) != selected {
		return false
	}
	step := strings.TrimSpace(request.RunLoopStepID)
	if _, ok := dailyReviewerRunLoopSteps[step]; !ok {
		return false
	}
	if step == "spawn-reviewer" {
		return request.Kind == "review-guidance" &&
			strings.TrimSpace(request.Actor) == "main-agent-harness" &&
			!request.CommandExecutable &&
			strings.TrimSpace(request.Command) == "" &&
			strings.TrimSpace(request.Guidance) != ""
	}
	return request.Kind == "preview-command" &&
		strings.TrimSpace(request.Actor) == "main-agent" &&
		request.CommandExecutable &&
		strings.TrimSpace(request.Command) != "" &&
		strings.TrimSpace(request.Guidance) == ""
}

func runDailyCorrection(parent context.Context, hostOpt Options, inspection missionintent.Inspection, correction string, result DailyResult, dailyOpt DailyOptions) (DailyResult, error) {
	lane, boardLane, existing, err := dailyCorrectionLane(result.CaseRoot, inspection, correction, hostOpt.Actor, hostOpt.SelectedLane)
	if err != nil {
		return result, err
	}
	result.Lane = lane
	hostOpt.SelectedLane = lane
	result.CorrectionEventID = dailyCorrectionEventID(dailyCorrectionScope(result.CaseRoot, inspection), lane, correction)
	terminalState := strings.ToLower(strings.TrimSpace(boardLane.Status))
	if terminalState == "archived" {
		return result, fmt.Errorf("daily correction refuses archived lane %s because no durable archive transition is supported", lane)
	}
	if terminalState == "closed" {
		if existing == nil {
			return result, fmt.Errorf("daily correction refuses closed lane %s", lane)
		}
		result.CorrectionEventID = mission.Value(existing, "eventId")
		if err := validateExistingDailyCorrectionRejection(result.CaseRoot, inspection, lane, correction, existing); err != nil {
			return result, err
		}
		resolution, resolved, err := inspectDailyCorrectionResolution(result.CaseRoot, lane, result.CorrectionEventID)
		if err != nil {
			return result, err
		}
		if !resolved || resolution.ExecutorGeneration != boardLane.ExecutorGeneration {
			return result, fmt.Errorf("daily terminal correction replay requires the durable current reconcile resolution")
		}
		state, generation, terminal, err := dailyLaneTerminal(result.CaseRoot, lane)
		if err != nil {
			return result, err
		}
		if !terminal || generation != resolution.ExecutorGeneration {
			return result, fmt.Errorf("daily terminal correction replay requires the committed current lane completion")
		}
		result.FinalState = state
		result.ExecutorGeneration = generation
		result.Replay = true
		return result, nil
	}

	if existing != nil && (mission.Value(existing, "reviewerVerificationEventId") == "" || mission.Value(existing, "reviewerDecisionEventId") == "") {
		existing = nil
	}
	var rejection workstream.MemberReviewerRejection
	targetRef := ""
	if existing != nil {
		result.CorrectionEventID = mission.Value(existing, "eventId")
		if err := validateExistingDailyCorrectionRejection(result.CaseRoot, inspection, lane, correction, existing); err != nil {
			return result, err
		}
		targetRef = mission.Value(existing, "target")
		var rejected bool
		rejection, rejected, err = workstream.CurrentMemberManifestReviewerRejection(result.CaseRoot, lane, targetRef)
		if err != nil {
			return result, err
		}
		if !rejected || !dailyCorrectionBindsRejection(existing, rejection) {
			return result, fmt.Errorf("existing daily correction does not match canonical reviewer rejection")
		}
	} else {
		rejection, targetRef, err = currentDailyCorrectionMemberTarget(result.CaseRoot, boardLane)
		if err != nil {
			return result, err
		}
		result.CorrectionEventID = dailyCorrectionEventID(dailyCorrectionScope(result.CaseRoot, inspection), lane, correction, rejection)
		existing, err = existingDailyCorrectionByID(result.CaseRoot, result.CorrectionEventID, lane, correction, hostOpt.Actor)
		if err != nil {
			return result, err
		}
	}
	if existing != nil {
		if err := verifyDailyCorrection(existing, lane, correction, hostOpt.Actor, result.CorrectionEventID, targetRef, rejection); err != nil {
			return result, err
		}
	}
	resolution, resolved, err := inspectDailyCorrectionResolution(result.CaseRoot, lane, result.CorrectionEventID)
	if err != nil {
		return result, err
	}
	if !resolved {
		if existing == nil {
			createdAt := nowRFC3339Nano()
			args := dailyCorrectionArgs(result.CaseRoot, result.Pack, lane, correction, hostOpt.Actor, result.CorrectionEventID, createdAt, targetRef, rejection)
			var preview note.AppendResult
			if err := runPublicCLI(args, &preview); err != nil {
				return result, fmt.Errorf("daily public correction preview: %w", err)
			}
			if preview.IsMutation || preview.Applied || preview.Reason != "what-if" || len(preview.RecordArgs) == 0 {
				return result, fmt.Errorf("daily public correction preview omitted a zero-write exact record request")
			}
			var applied note.AppendResult
			if err := runPublicCLI(preview.RecordArgs, &applied); err != nil {
				return result, fmt.Errorf("record daily public correction: %w", err)
			}
			if !applied.Applied {
				return result, fmt.Errorf("daily public correction was not applied: %s", applied.Reason)
			}
			result.DriverSteps = append(result.DriverSteps, "note-intervention")
			existing = applied.Event
		}
		if err := verifyDailyCorrection(existing, lane, correction, hostOpt.Actor, result.CorrectionEventID, targetRef, rejection); err != nil {
			return result, err
		}
		step, err := runPublicDriverStepWithLease(result.CaseRoot, result.Pack, dailyOpt.projectExecutionLease, lane)
		if err != nil {
			return result, fmt.Errorf("daily public correction reconcile: %w", err)
		}
		if step.ResultCommand != "reconcile" {
			return result, fmt.Errorf("daily correction expected reconcile, got %q", step.ResultCommand)
		}
		result.DriverSteps = append(result.DriverSteps, step.ResultCommand)
		resolution, resolved, err = inspectDailyCorrectionResolution(result.CaseRoot, lane, result.CorrectionEventID)
		if err != nil {
			return result, err
		}
		if !resolved || step.Actor != resolution.Actor || step.Executor != resolution.Executor || step.ExecutorGeneration != resolution.ExecutorGeneration {
			return result, fmt.Errorf("daily correction durable reconcile identity differs from public Apply result")
		}
	}
	result.ExecutorGeneration = resolution.ExecutorGeneration

	if state, generation, terminal, err := dailyLaneTerminal(result.CaseRoot, lane); err != nil {
		return result, err
	} else if terminal {
		result.FinalState = state
		result.ExecutorGeneration = generation
		result.Replay = true
		return result, nil
	}
	if dailyOpt.beforeMemberRun != nil {
		if err := dailyOpt.beforeMemberRun(result.CaseRoot, result.Pack, lane); err != nil {
			return result, fmt.Errorf("prepare corrected daily member run: %w", err)
		}
	}
	adapterReady, err := prepareProductionAdapterBeforeMember(parent, dailyOpt, &result)
	if err != nil {
		return result, err
	}
	if !adapterReady {
		return result, nil
	}
	if result.Pack == defaults.DefaultPack {
		inputReady, err := prepareDailyInputReadiness(
			result.CaseRoot,
			result.Pack,
			lane,
			DailyInputRequest{},
			&result,
		)
		if err != nil {
			return result, err
		}
		if !inputReady {
			return result, nil
		}
	}
	owner, err := newDailySessionTransitionOwner(result.CaseRoot, result.Pack, lane, dailyOpt.projectExecutionLease)
	if err != nil {
		return result, err
	}
	if err := owner.runHostSegment(parent, hostOpt, &result); err != nil {
		return result, err
	}
	if err := owner.finish(&result); err != nil {
		return result, err
	}
	return result, nil
}

func dailyCorrectionLane(caseRoot string, inspection missionintent.Inspection, correction, actor string, selected ...string) (string, mission.BoardLane, map[string]any, error) {
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return "", mission.BoardLane{}, nil, fmt.Errorf("daily correction requires an initialized board: %w", err)
	}
	scope := dailyCorrectionScope(caseRoot, inspection)
	if laneID := strings.TrimSpace(firstSelectedLane(selected)); laneID != "" {
		return dailyCorrectionStateForLane(caseRoot, scope, board, laneID, correction, actor)
	}
	if inspection.Committed {
		candidates := []string{}
		for _, lane := range board.Lanes {
			latest, found, latestErr := memberexecution.Latest(caseRoot, lane.ID)
			if latestErr != nil {
				return "", mission.BoardLane{}, nil, latestErr
			}
			if !found || latest.State != "intake-ready" || latest.Manifest == nil || latest.Owner.Executor != lane.CurrentExecutor || latest.Owner.ExecutorGeneration != lane.ExecutorGeneration {
				continue
			}
			targetRef := relativeLiveAcceptancePath(caseRoot, latest.ManifestPath)
			_, rejected, rejectionErr := workstream.CurrentMemberManifestReviewerRejection(caseRoot, lane.ID, targetRef)
			if rejectionErr != nil {
				return "", mission.BoardLane{}, nil, rejectionErr
			}
			if rejected {
				candidates = append(candidates, lane.ID)
			}
		}
		if len(candidates) == 0 {
			items, readErr := mission.ReadStrictFact(caseRoot, "intervention")
			if readErr != nil {
				return "", mission.BoardLane{}, nil, readErr
			}
			seen := map[string]bool{}
			for _, item := range items {
				laneID := mission.Value(item, "lane")
				if laneID == "" || seen[laneID] || mission.Value(item, "kind") != "intervention" || mission.Value(item, "subject") != "daily human correction" || mission.Value(item, "summary") != correction || mission.Value(item, "actor") != actor || mission.Value(item, "action") != "override" || !strings.EqualFold(mission.Value(item, "status"), "open") {
					continue
				}
				if _, ok := mission.LookupBoardLane(board.Lanes, laneID, false); ok {
					seen[laneID] = true
					candidates = append(candidates, laneID)
				}
			}
		}
		if len(candidates) != 1 {
			return "", mission.BoardLane{}, nil, fmt.Errorf("daily correction requires exactly one canonical reviewer rejection lane; found %d", len(candidates))
		}
		return dailyCorrectionStateForLane(caseRoot, scope, board, candidates[0], correction, actor)
	}
	matches := []struct {
		lane  mission.BoardLane
		event map[string]any
	}{}
	for _, lane := range board.Lanes {
		if !strings.EqualFold(strings.TrimSpace(lane.Status), "closed") && !strings.EqualFold(strings.TrimSpace(lane.Status), "archived") {
			continue
		}
		event, err := existingDailyCorrectionForRequest(caseRoot, scope, lane.ID, correction, actor)
		if err != nil {
			return "", mission.BoardLane{}, nil, err
		}
		if event != nil {
			matches = append(matches, struct {
				lane  mission.BoardLane
				event map[string]any
			}{lane, event})
		}
	}
	if len(matches) == 1 {
		return matches[0].lane.ID, matches[0].lane, matches[0].event, nil
	}
	if len(matches) > 1 {
		return "", mission.BoardLane{}, nil, fmt.Errorf("daily correction identity matches multiple legacy lanes")
	}
	candidates := []mission.BoardLane{}
	for _, lane := range mission.OpenBoardLanes(board.Lanes) {
		if lane.Authority || strings.TrimSpace(lane.CurrentExecutor) == "" || lane.ExecutorGeneration < 1 {
			continue
		}
		latest, ok, inspectErr := memberexecution.Latest(caseRoot, lane.ID)
		if inspectErr == nil && ok && latest.State == "intake-ready" && latest.Owner.Executor == lane.CurrentExecutor && latest.Owner.ExecutorGeneration == lane.ExecutorGeneration {
			candidates = append(candidates, lane)
		}
	}
	if len(candidates) != 1 {
		return "", mission.BoardLane{}, nil, fmt.Errorf("daily correction requires exactly one current intake-ready feature lane; found %d", len(candidates))
	}
	return candidates[0].ID, candidates[0], nil, nil
}

func dailyCorrectionStateForLane(caseRoot, scope string, board mission.Board, laneID, correction, actor string) (string, mission.BoardLane, map[string]any, error) {
	lane, ok := mission.LookupBoardLane(board.Lanes, laneID, false)
	if !ok {
		return "", mission.BoardLane{}, nil, fmt.Errorf("selected daily correction lane is not current: %s", laneID)
	}
	latest, found, err := memberexecution.Latest(caseRoot, laneID)
	if err != nil {
		return "", mission.BoardLane{}, nil, err
	}
	if found && latest.State == "intake-ready" && latest.Manifest != nil && latest.Owner.Executor == lane.CurrentExecutor && latest.Owner.ExecutorGeneration == lane.ExecutorGeneration {
		targetRef := relativeLiveAcceptancePath(caseRoot, latest.ManifestPath)
		rejection, rejected, rejectionErr := workstream.CurrentMemberManifestReviewerRejection(caseRoot, laneID, targetRef)
		if rejectionErr != nil {
			return "", mission.BoardLane{}, nil, rejectionErr
		}
		if rejected {
			eventID := dailyCorrectionEventID(scope, laneID, correction, rejection)
			existing, existingErr := existingDailyCorrectionByID(caseRoot, eventID, laneID, correction, actor)
			return laneID, lane, existing, existingErr
		}
	}
	if eventID := strings.TrimSpace(lane.LastReconciledIntervention); eventID != "" {
		existing, existingErr := existingDailyCorrectionByID(caseRoot, eventID, laneID, correction, actor)
		if existingErr != nil {
			return "", mission.BoardLane{}, nil, existingErr
		}
		if existing == nil {
			return "", mission.BoardLane{}, nil, fmt.Errorf("daily correction board reconcile identity is missing from the intervention ledger: %s", eventID)
		}
		return laneID, lane, existing, nil
	}
	existing, err := existingDailyCorrectionForRequest(caseRoot, scope, laneID, correction, actor)
	if err != nil {
		return "", mission.BoardLane{}, nil, err
	}
	if existing == nil {
		return "", mission.BoardLane{}, nil, fmt.Errorf("selected daily correction lane %s has no canonical reviewer rejection", laneID)
	}
	return laneID, lane, existing, nil
}

func currentDailyCorrectionMemberTarget(caseRoot string, boardLane mission.BoardLane) (workstream.MemberReviewerRejection, string, error) {
	latest, ok, err := memberexecution.Latest(caseRoot, boardLane.ID)
	if err != nil {
		return workstream.MemberReviewerRejection{}, "", err
	}
	if !ok || latest.State != "intake-ready" || latest.Manifest == nil || latest.Owner.Executor != boardLane.CurrentExecutor || latest.Owner.ExecutorGeneration != boardLane.ExecutorGeneration {
		return workstream.MemberReviewerRejection{}, "", fmt.Errorf("daily correction requires the current real member result to be durably intake-ready")
	}
	targetRef := relativeLiveAcceptancePath(caseRoot, latest.ManifestPath)
	rejection, rejected, err := workstream.CurrentMemberManifestReviewerRejection(caseRoot, boardLane.ID, targetRef)
	if err != nil {
		return workstream.MemberReviewerRejection{}, "", err
	}
	if !rejected {
		return workstream.MemberReviewerRejection{}, "", fmt.Errorf("daily correction requires a canonical reviewer rejection for the current member manifest")
	}
	return rejection, targetRef, nil
}

func dailyCorrectionArgs(caseRoot, pack, lane, correction, actor, eventID, createdAt, targetRef string, rejections ...workstream.MemberReviewerRejection) []string {
	args := []string{
		"-Command", "note", "-Target", caseRoot, "-Pack", pack,
		"-Kind", "intervention", "-Lane", lane,
		"-Subject", "daily human correction", "-Summary", correction,
		"-Actor", actor, "-Action", "override", "-Status", "open",
		"-EventId", eventID, "-CreatedAt", createdAt, "-TargetRef", targetRef,
	}
	if len(rejections) > 0 {
		rejection := rejections[0]
		args = append(args,
			"-Related", rejection.VerificationEventID+","+rejection.DecisionEventID,
			"-EvidenceRefs", strings.Join(rejection.EvidenceRefs, ","),
			"-ReviewerPacketId", rejection.PacketID, "-ReviewerRouteId", rejection.RouteID, "-ReviewerShardId", rejection.ShardID,
			"-ReviewerPacketPath", rejection.PacketPath, "-ReviewerResultLineagePath", rejection.ReviewerResultPath, "-ReviewerLineageSession", rejection.ReviewerSession,
			"-ReviewerDispatchReceiptPath", rejection.ReviewerDispatchPath, "-ReviewerDispatchReceiptSha256", rejection.ReviewerDispatchSHA256,
			"-ReviewerCompletionReceiptPath", rejection.ReviewerCompletionPath, "-ReviewerCompletionReceiptSha256", rejection.ReviewerCompletionSHA256,
			"-ReviewerLineageInputPath", rejection.ReviewerResultInputPath, "-ReviewerLineageInputSha256", rejection.ReviewerResultInputSHA256,
			"-ReviewerLineageInputBytes", fmt.Sprint(rejection.ReviewerResultInputBytes), "-ReviewerLineageResultSha256", rejection.ReviewerResultSHA256,
			"-ReviewerManifestSha256", rejection.ManifestSHA256, "-ReviewerVerificationEventId", rejection.VerificationEventID, "-ReviewerDecisionEventId", rejection.DecisionEventID,
			"-ReviewerOwnerExecutor", rejection.OwnerExecutor, "-ReviewerOwnerGeneration", fmt.Sprint(rejection.OwnerGeneration),
		)
	}
	return append(args, "-WhatIf", "-Format", "json")
}

func dailyCorrectionScope(caseRoot string, inspection missionintent.Inspection) string {
	if inspection.Committed && inspection.MissionIntentSHA256 != "" {
		return strings.ToLower(inspection.MissionIntentSHA256)
	}
	return strings.ToLower(filepath.Clean(caseRoot))
}

func dailyCorrectionEventID(scope, lane, correction string, rejection ...workstream.MemberReviewerRejection) string {
	identity := []string{"daily-correction-v1", scope, lane, correction}
	if len(rejection) > 0 {
		item := rejection[0]
		identity = []string{"daily-correction-v2", scope, lane, correction, item.ManifestRef, item.ManifestSHA256, item.PacketID, item.ShardID, item.ReviewerResultInputSHA256, item.VerificationEventID, item.DecisionEventID, item.ReviewerSession, item.OwnerExecutor, fmt.Sprint(item.OwnerGeneration)}
	}
	sum := sha256.Sum256([]byte(strings.Join(identity, "\x00")))
	return "daily-correction-" + hex.EncodeToString(sum[:12])
}

func existingDailyCorrectionByID(caseRoot, eventID, lane, correction, actor string) (map[string]any, error) {
	items, err := mission.ReadStrictFact(caseRoot, "intervention")
	if err != nil {
		return nil, err
	}
	var found map[string]any
	for _, item := range items {
		if mission.Value(item, "eventId") != eventID {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("daily correction event is duplicated: %s", eventID)
		}
		if err := verifyDailyCorrection(item, lane, correction, actor, eventID); err != nil {
			return nil, err
		}
		found = item
	}
	return found, nil
}

func validateExistingDailyCorrectionRejection(caseRoot string, inspection missionintent.Inspection, lane, correction string, event map[string]any) error {
	if event == nil || mission.Value(event, "reviewerVerificationEventId") == "" || mission.Value(event, "reviewerDecisionEventId") == "" {
		return nil
	}
	targetRef := mission.Value(event, "target")
	rejection, rejected, err := workstream.CurrentMemberManifestReviewerRejection(caseRoot, lane, targetRef)
	if err != nil {
		return err
	}
	if !rejected || dailyCorrectionEventID(dailyCorrectionScope(caseRoot, inspection), lane, correction, rejection) != mission.Value(event, "eventId") || !dailyCorrectionBindsRejection(event, rejection) {
		return fmt.Errorf("existing daily correction does not match its canonical reviewer rejection")
	}
	return nil
}

func existingDailyCorrectionForRequest(caseRoot, scope, lane, correction, actor string) (map[string]any, error) {
	items, err := mission.ReadStrictFact(caseRoot, "intervention")
	if err != nil {
		return nil, err
	}
	legacyID := dailyCorrectionEventID(scope, lane, correction)
	var found map[string]any
	for _, item := range items {
		if mission.Value(item, "lane") != lane || mission.Value(item, "subject") != "daily human correction" || mission.Value(item, "summary") != correction || mission.Value(item, "actor") != actor || mission.Value(item, "action") != "override" || !strings.EqualFold(mission.Value(item, "status"), "open") {
			continue
		}
		eventID := mission.Value(item, "eventId")
		isLegacy := eventID == legacyID
		isRejectionBound := mission.Value(item, "reviewerVerificationEventId") != "" && mission.Value(item, "reviewerDecisionEventId") != ""
		if !isLegacy && !isRejectionBound {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("daily correction logical request matches multiple durable events for lane %s", lane)
		}
		if err := verifyDailyCorrection(item, lane, correction, actor, eventID); err != nil {
			return nil, err
		}
		found = item
	}
	return found, nil
}

func verifyDailyCorrection(event map[string]any, lane, correction, actor, eventID string, bindings ...any) error {
	if event == nil || mission.Value(event, "eventId") != eventID || mission.Value(event, "kind") != "intervention" || mission.Value(event, "lane") != lane || mission.Value(event, "subject") != "daily human correction" || mission.Value(event, "summary") != correction || mission.Value(event, "actor") != actor || mission.Value(event, "action") != "override" || !strings.EqualFold(mission.Value(event, "status"), "open") {
		return fmt.Errorf("existing daily correction does not match the exact logical request: %s", eventID)
	}
	if len(bindings) > 0 {
		targetRef, ok := bindings[0].(string)
		if !ok || mission.Value(event, "target") != targetRef {
			return fmt.Errorf("existing daily correction does not match the exact logical request: %s", eventID)
		}
	}
	if len(bindings) > 1 {
		rejection, ok := bindings[1].(workstream.MemberReviewerRejection)
		if !ok || !dailyCorrectionBindsRejection(event, rejection) {
			return fmt.Errorf("existing daily correction does not match canonical reviewer rejection: %s", eventID)
		}
	}
	return nil
}

func dailyCorrectionBindsRejection(event map[string]any, rejection workstream.MemberReviewerRejection) bool {
	return mission.Value(event, "packetId") == rejection.PacketID &&
		mission.Value(event, "routeId") == rejection.RouteID &&
		mission.Value(event, "shardId") == rejection.ShardID &&
		mission.Value(event, "packetPath") == rejection.PacketPath &&
		mission.Value(event, "reviewerResultPath") == rejection.ReviewerResultPath &&
		mission.Value(event, "reviewerSession") == rejection.ReviewerSession &&
		mission.Value(event, "reviewerDispatchReceiptPath") == rejection.ReviewerDispatchPath &&
		strings.EqualFold(mission.Value(event, "reviewerDispatchReceiptSha256"), rejection.ReviewerDispatchSHA256) &&
		mission.Value(event, "reviewerCompletionReceiptPath") == rejection.ReviewerCompletionPath &&
		strings.EqualFold(mission.Value(event, "reviewerCompletionReceiptSha256"), rejection.ReviewerCompletionSHA256) &&
		mission.Value(event, "reviewerResultInputPath") == rejection.ReviewerResultInputPath &&
		strings.EqualFold(mission.Value(event, "reviewerResultInputSha256"), rejection.ReviewerResultInputSHA256) &&
		mission.Value(event, "reviewerResultInputBytes") == fmt.Sprint(rejection.ReviewerResultInputBytes) &&
		strings.EqualFold(mission.Value(event, "reviewerResultSha256"), rejection.ReviewerResultSHA256) &&
		strings.EqualFold(mission.Value(event, "reviewerManifestSha256"), rejection.ManifestSHA256) &&
		mission.Value(event, "reviewerVerificationEventId") == rejection.VerificationEventID &&
		mission.Value(event, "reviewerDecisionEventId") == rejection.DecisionEventID &&
		mission.Value(event, "ownerExecutor") == rejection.OwnerExecutor &&
		mission.Value(event, "ownerGeneration") == fmt.Sprint(rejection.OwnerGeneration)
}

type dailyCorrectionResolution struct {
	EventID            string
	Time               string
	Actor              string
	Executor           string
	ExecutorGeneration int
}

func inspectDailyCorrectionResolution(caseRoot, lane, eventID string) (dailyCorrectionResolution, bool, error) {
	items, err := mission.ReadStrictFact(caseRoot, "intervention")
	if err != nil {
		return dailyCorrectionResolution{}, false, err
	}
	var resolution dailyCorrectionResolution
	found := false
	for _, item := range items {
		if mission.Value(item, "lane") != lane || mission.Value(item, "resolvesEventId") != eventID || mission.Value(item, "action") != "reconcile" || !strings.EqualFold(mission.Value(item, "status"), "resolved") {
			continue
		}
		if found {
			return dailyCorrectionResolution{}, false, fmt.Errorf("daily correction has multiple reconcile resolutions: %s", eventID)
		}
		resolution = dailyCorrectionResolution{
			EventID:  strings.TrimSpace(mission.Value(item, "eventId")),
			Time:     strings.TrimSpace(mission.Value(item, "time")),
			Actor:    strings.TrimSpace(mission.Value(item, "actor")),
			Executor: strings.TrimSpace(mission.Value(item, "executor")),
		}
		if resolution.EventID == "" || resolution.Time == "" || resolution.Actor == "" || resolution.Executor == "" {
			return dailyCorrectionResolution{}, false, fmt.Errorf("daily correction resolution omitted durable identity: %s", eventID)
		}
		found = true
	}
	if !found {
		return dailyCorrectionResolution{}, false, nil
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return dailyCorrectionResolution{}, false, err
	}
	boardLane, ok := mission.LookupBoardLane(board.Lanes, lane, false)
	if !ok || boardLane.LastReconciledIntervention != eventID || boardLane.LastReconcileAt != resolution.Time || boardLane.CurrentExecutor != resolution.Executor || boardLane.ExecutorGeneration < 1 {
		return dailyCorrectionResolution{}, false, fmt.Errorf("daily correction durable resolution differs from current board owner: %s", eventID)
	}
	resolution.ExecutorGeneration = boardLane.ExecutorGeneration
	return resolution, true, nil
}
