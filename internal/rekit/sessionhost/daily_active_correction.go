package sessionhost

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	"github.com/shuiyu486/re-context-kits/internal/rekit/note"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

const dailyActiveCorrectionSubject = "daily active lane correction"

var dailyActiveCorrectionAfterPreviewHook func() error

func runDailyActiveCorrection(
	hostOpt Options,
	inspection missionintent.Inspection,
	correction string,
	result DailyResult,
	dailyOpt DailyOptions,
) (DailyResult, error) {
	lane := strings.TrimSpace(hostOpt.SelectedLane)
	if lane == "" {
		return result, fmt.Errorf("daily active correction requires one explicitly selected open lane")
	}
	board, err := mission.ReadBoard(result.CaseRoot)
	if err != nil {
		return result, err
	}
	current, ok := mission.LookupBoardLane(board.Lanes, lane, false)
	if !ok || !strings.EqualFold(strings.TrimSpace(current.Status), "open") || current.Authority {
		return result, fmt.Errorf("daily active correction selected lane is no longer an open non-authority lane: %s", lane)
	}

	eventID := dailyActiveCorrectionEventID(
		dailyCorrectionScope(result.CaseRoot, inspection),
		lane,
		correction,
		hostOpt.Actor,
	)
	result.Lane = lane
	result.CorrectionEventID = eventID
	existing, existingExecutor, existingGeneration, err := existingDailyActiveCorrection(
		result.CaseRoot,
		eventID,
		lane,
		correction,
		hostOpt.Actor,
	)
	if err != nil {
		return result, err
	}
	if existing != nil {
		if existingExecutor == "" || existingGeneration < 1 {
			return result, fmt.Errorf("daily active correction durable event omitted its owner generation: %s", eventID)
		}
		if _, committed, commitErr := inspectDailyActiveCorrectionCommit(
			result.CaseRoot,
			lane,
			eventID,
			hostOpt.Actor,
			existingExecutor,
			existingGeneration+1,
		); commitErr != nil {
			return result, commitErr
		} else if committed {
			result.ExecutorGeneration = existingGeneration + 1
			result.FinalState = "active-correction-recorded"
			result.Replay = true
			result.Blocked = false
			result.Action = dailyAction(DailyActionReadyToContinue)
			return finalizeDailyActiveCorrectionBoundary(result), nil
		}
		if current.CurrentExecutor != existingExecutor || current.ExecutorGeneration != existingGeneration {
			return result, fmt.Errorf("daily active correction has an unresolved durable event bound to a stale owner generation: %s", eventID)
		}
		current.CurrentExecutor = existingExecutor
		current.ExecutorGeneration = existingGeneration
	}
	if existing == nil && (strings.TrimSpace(current.CurrentExecutor) == "" || current.ExecutorGeneration < 1) {
		return result, fmt.Errorf("daily active correction selected lane is no longer owned: %s", lane)
	}
	if existing == nil {
		createdAt := nowRFC3339Nano()
		args := dailyActiveCorrectionArgs(
			result.CaseRoot,
			result.Pack,
			lane,
			correction,
			hostOpt.Actor,
			eventID,
			createdAt,
			current.CurrentExecutor,
			current.ExecutorGeneration,
		)
		var preview note.AppendResult
		if err := runPublicCLI(args, &preview); err != nil {
			return result, fmt.Errorf("daily active correction preview: %w", err)
		}
		if preview.IsMutation || preview.Applied || preview.Reason != "what-if" ||
			preview.EventID != eventID || len(preview.RecordArgs) == 0 ||
			mission.Value(preview.Event, "ownerExecutor") != current.CurrentExecutor ||
			mission.Value(preview.Event, "ownerGeneration") != fmt.Sprint(current.ExecutorGeneration) {
			return result, fmt.Errorf("daily active correction preview omitted its zero-write owner-generation binding")
		}
		if dailyActiveCorrectionAfterPreviewHook != nil {
			if err := dailyActiveCorrectionAfterPreviewHook(); err != nil {
				return result, err
			}
		}

		fresh, err := mission.ReadBoard(result.CaseRoot)
		if err != nil {
			return result, err
		}
		freshLane, ok := mission.LookupBoardLane(fresh.Lanes, lane, false)
		if !ok || !strings.EqualFold(strings.TrimSpace(freshLane.Status), "open") ||
			freshLane.CurrentExecutor != current.CurrentExecutor ||
			freshLane.ExecutorGeneration != current.ExecutorGeneration {
			return result, fmt.Errorf("daily active correction owner generation changed after preview")
		}
		var applied note.AppendResult
		if err := runPublicCLI(preview.RecordArgs, &applied); err != nil {
			return result, fmt.Errorf("record daily active correction: %w", err)
		}
		if !applied.Applied || applied.EventID != eventID ||
			mission.Value(applied.Event, "ownerExecutor") != current.CurrentExecutor ||
			mission.Value(applied.Event, "ownerGeneration") != fmt.Sprint(current.ExecutorGeneration) {
			return result, fmt.Errorf("daily active correction did not append its exact typed owner-generation event")
		}
		result.DriverSteps = append(result.DriverSteps, "note-active-correction")
		existing = applied.Event
	}
	if err := verifyDailyActiveCorrection(
		existing,
		eventID,
		lane,
		correction,
		hostOpt.Actor,
		current.CurrentExecutor,
		current.ExecutorGeneration,
	); err != nil {
		return result, err
	}

	request, err := dailyActiveCorrectionReconcileRequest(
		result.CaseRoot,
		result.Pack,
		lane,
		eventID,
		hostOpt.Actor,
		current.CurrentExecutor,
	)
	if err != nil {
		return result, err
	}
	step, stepErr := runPublicExactDriverStepWithLease(
		result.CaseRoot,
		result.Pack,
		dailyOpt.projectExecutionLease,
		&request,
		lane,
	)
	if stepErr != nil && step.ResultCommand == "" {
		return result, fmt.Errorf("daily active correction reconcile: %w", stepErr)
	}
	if step.ResultCommand != "reconcile" ||
		step.PreviousExecutor != current.CurrentExecutor ||
		step.Executor != current.CurrentExecutor ||
		step.ExecutorGeneration != current.ExecutorGeneration+1 {
		return result, fmt.Errorf("daily active correction reconcile omitted its generation fence")
	}
	result.DriverSteps = append(result.DriverSteps, step.ResultCommand)
	result.ExecutorGeneration = step.ExecutorGeneration

	_, committed, verifyErr := inspectDailyActiveCorrectionCommit(
		result.CaseRoot,
		lane,
		eventID,
		hostOpt.Actor,
		current.CurrentExecutor,
		current.ExecutorGeneration+1,
	)
	if verifyErr != nil {
		return result, verifyErr
	}
	if !committed {
		return result, fmt.Errorf("daily active correction generation fence differs from current lane state")
	}
	if stepErr != nil {
		result.Replay = true
	}
	result.FinalState = "active-correction-recorded"
	result.Blocked = false
	result.Action = dailyAction(DailyActionReadyToContinue)
	return finalizeDailyActiveCorrectionBoundary(result), nil
}

func finalizeDailyActiveCorrectionBoundary(result DailyResult) DailyResult {
	result.Boundary = mission.UniqueStrings(append(result.Boundary,
		"active-lane correction appends one typed intervention and advances only the selected lane owner generation",
		"the correction does not terminate a process, launch Claude, or grant authority, confirmed state, gate, or heavy-tool permission",
		"results from the superseded owner generation remain stale or held and cannot advance the current lane",
	))
	return result
}

func inspectDailyActiveCorrectionCommit(
	caseRoot,
	lane,
	eventID,
	actor,
	executor string,
	generation int,
) (mission.BoardLane, bool, error) {
	resolution, resolved, err := inspectDailyCorrectionResolution(caseRoot, lane, eventID)
	if err != nil {
		return mission.BoardLane{}, false, err
	}
	if !resolved {
		return mission.BoardLane{}, false, nil
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return mission.BoardLane{}, false, err
	}
	current, ok := mission.LookupBoardLane(board.Lanes, lane, false)
	if !ok || !strings.EqualFold(strings.TrimSpace(current.Status), "open") ||
		current.CurrentExecutor != executor || current.ExecutorGeneration != generation ||
		current.LastReconciledIntervention != eventID ||
		resolution.Actor != actor || resolution.Executor != executor ||
		resolution.ExecutorGeneration != generation {
		return mission.BoardLane{}, false, fmt.Errorf(
			"daily active correction durable resolution differs from its exact owner-generation commit: %s",
			eventID,
		)
	}
	return current, true, nil
}

func dailyActiveCorrectionReconcileRequest(
	caseRoot,
	pack,
	lane,
	eventID,
	actor,
	executor string,
) (mission.MissionCommanderDriverRequest, error) {
	args := []string{
		"-Lane", lane,
		"-InterventionId", eventID,
		"-Executor", executor,
		"-Actor", actor,
		"-Target", caseRoot,
		"-Pack", pack,
		"-WhatIf",
		"-Format", "json",
	}
	invocation, err := commands.NewPublicInvocation(commands.Reconcile, args...)
	if err != nil {
		return mission.MissionCommanderDriverRequest{}, err
	}
	root, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return mission.MissionCommanderDriverRequest{}, err
	}
	entrypoint := commands.LegacyPublicEntrypoint
	if root.Existing && !root.Legacy {
		entrypoint = commands.CurrentPublicEntrypoint
	}
	command, err := invocation.RenderForEntrypoint(entrypoint)
	if err != nil {
		return mission.MissionCommanderDriverRequest{}, err
	}
	request := mission.MissionCommanderDriverRequest{
		Kind:              "preview-command",
		RunLoopStepID:     "preview-current",
		Actor:             "main-agent",
		State:             "needs-reconcile",
		Source:            "missionCommanderActions",
		Lane:              lane,
		Label:             lane,
		Invocation:        &invocation,
		Command:           command,
		CommandExecutable: true,
		RequiresReview:    true,
		ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
			Command: command,
		},
		Boundary: []string{
			"reconcile only the exact active correction intervention",
			"preserve the current executor and advance only its generation",
			"no authority/confirmed writes or heavy-tool execution",
		},
	}
	if err := mission.ValidateMissionCommanderDriverRequest(request); err != nil {
		return mission.MissionCommanderDriverRequest{}, err
	}
	return request, nil
}

func dailyActiveCorrectionEventID(
	scope,
	lane,
	correction,
	actor string,
) string {
	return dailyCorrectionEventID(
		scope,
		lane,
		strings.Join([]string{
			dailyActiveCorrectionSubject,
			correction,
			actor,
		}, "\x00"),
	)
}

func dailyActiveCorrectionArgs(
	caseRoot,
	pack,
	lane,
	correction,
	actor,
	eventID,
	createdAt,
	executor string,
	generation int,
) []string {
	return []string{
		"-Command", "note", "-Target", caseRoot, "-Pack", pack,
		"-Kind", "intervention", "-Lane", lane,
		"-Subject", dailyActiveCorrectionSubject, "-Summary", correction,
		"-Actor", actor, "-Action", "override", "-Status", "open",
		"-EventId", eventID, "-CreatedAt", createdAt,
		"-ReviewerOwnerExecutor", executor,
		"-ReviewerOwnerGeneration", fmt.Sprint(generation),
		"-ReviewerOwnerBindingMode", "daily-active-correction",
		"-WhatIf", "-Format", "json",
	}
}

func existingDailyActiveCorrection(
	caseRoot,
	eventID,
	lane,
	correction,
	actor string,
) (map[string]any, string, int, error) {
	items, err := mission.ReadStrictFact(caseRoot, "intervention")
	if err != nil {
		return nil, "", 0, err
	}
	var found map[string]any
	foundExecutor := ""
	foundGeneration := 0
	for _, item := range items {
		if mission.Value(item, "eventId") != eventID {
			continue
		}
		if found != nil {
			return nil, "", 0, fmt.Errorf("daily active correction event is duplicated: %s", eventID)
		}
		executor := strings.TrimSpace(mission.Value(item, "ownerExecutor"))
		generation, err := strconv.Atoi(strings.TrimSpace(mission.Value(item, "ownerGeneration")))
		if err != nil || generation < 1 {
			return nil, "", 0, fmt.Errorf("daily active correction owner generation is invalid: %s", eventID)
		}
		if err := verifyDailyActiveCorrection(item, eventID, lane, correction, actor, executor, generation); err != nil {
			return nil, "", 0, err
		}
		found = item
		foundExecutor = executor
		foundGeneration = generation
	}
	return found, foundExecutor, foundGeneration, nil
}

func verifyDailyActiveCorrection(
	event map[string]any,
	eventID,
	lane,
	correction,
	actor,
	executor string,
	generation int,
) error {
	if event == nil || mission.Value(event, "eventId") != eventID ||
		mission.Value(event, "kind") != "intervention" ||
		mission.Value(event, "lane") != lane ||
		mission.Value(event, "subject") != dailyActiveCorrectionSubject ||
		mission.Value(event, "summary") != correction ||
		mission.Value(event, "actor") != actor ||
		mission.Value(event, "action") != "override" ||
		!strings.EqualFold(mission.Value(event, "status"), "open") ||
		mission.Value(event, "ownerExecutor") != executor ||
		mission.Value(event, "ownerGeneration") != fmt.Sprint(generation) {
		return fmt.Errorf("daily active correction does not match its exact owner-generation request: %s", eventID)
	}
	return nil
}
