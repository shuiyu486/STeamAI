package cli

import (
	"fmt"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func usesSelectedCurrentLaneProjection(command string) bool {
	switch strings.ToLower(strings.TrimSpace(command)) {
	case "status", "handoff", "run-current-step", "run-current-loop", "run-driver-step", "run-reviewer-step", "run-reviewer-wave":
		return true
	default:
		return false
	}
}

func selectedCurrentLaneRequiresExecutable(command string) bool {
	switch strings.ToLower(strings.TrimSpace(command)) {
	case "handoff", "run-reviewer-wave":
		return false
	default:
		return true
	}
}

func optionsWithEffectiveSelectedCurrentLane(opt Options, reviewedLane string) (Options, error) {
	selected := strings.TrimSpace(opt.SelectedCurrentLane)
	reviewedLane = strings.TrimSpace(reviewedLane)
	if selected != "" && reviewedLane != "" && selected != reviewedLane {
		return Options{}, fmt.Errorf("selected current lane %q does not match reviewed lane %q", selected, reviewedLane)
	}
	if selected == "" {
		opt.SelectedCurrentLane = reviewedLane
	}
	return opt, nil
}

func buildInvocationStatusInventory(ctx runtime.Context, opt Options) (statusInventory, error) {
	return buildInvocationStatusInventoryWithExecutableRequirement(
		ctx,
		opt,
		selectedCurrentLaneRequiresExecutable(opt.Command),
	)
}

func buildInvocationStatusInventoryAfterMutation(ctx runtime.Context, opt Options) (statusInventory, error) {
	return buildInvocationStatusInventoryWithExecutableRequirement(ctx, opt, false)
}

func buildInvocationStatusInventoryWithExecutableRequirement(ctx runtime.Context, opt Options, requireExecutable bool) (statusInventory, error) {
	selected := strings.TrimSpace(opt.SelectedCurrentLane)
	if selected == "" || !usesSelectedCurrentLaneProjection(opt.Command) {
		return buildStatusInventory(ctx, statusPackSource(ctx, opt))
	}
	status, err := buildStatusInventoryBase(ctx, statusPackSource(ctx, opt))
	if err != nil {
		return statusInventory{}, err
	}
	if err := bindStatusSelectedCurrentLane(&status, selected); err != nil {
		return statusInventory{}, err
	}
	bindStatusMemberExecution(&status)
	bindStatusReviewerCorrection(&status)
	bindStatusSelectedCurrentLaneCommands(&status, selected)
	bindStatusCurrentLoop(status.Target, status.CaseMission, status.MissionControlRunbook)
	bindStatusSelectedCurrentLaneCommands(&status, selected)
	if err := validateStatusSelectedCurrentLane(status, selected, requireExecutable); err != nil {
		return statusInventory{}, err
	}
	return status, nil
}

func bindStatusSelectedCurrentLane(status *statusInventory, selected string) error {
	selected = strings.TrimSpace(selected)
	if status == nil || selected == "" {
		return fmt.Errorf("selected current lane is missing")
	}
	if status.CaseMission == nil || status.Mode != "case" {
		return fmt.Errorf("selected current lane requires an attached case mission")
	}
	board, err := mission.ReadBoard(status.Target)
	if err != nil {
		return fmt.Errorf("selected current lane board: %w", err)
	}
	if _, ok := mission.LookupBoardLane(board.Lanes, selected, false); !ok {
		return fmt.Errorf("selected current lane %q is not current", selected)
	}

	caseMission := *status.CaseMission
	caseActions := selectedLaneActions(caseMission.MissionCommanderNextActions, selected)
	reviewerHandoffs := selectedLaneReviewerHandoffs(caseMission.ReviewerDispatchIntakeHandoffs, selected)
	reviewerActions := selectedLaneActions(
		workstream.MissionCommanderNextActionsWithReviewerDispatches(nil, reviewerHandoffs),
		selected,
	)

	caseMission.MissionCommanderNextActions = caseActions
	caseMission.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(caseActions)
	caseMission.ReviewerDispatchIntakeHandoffs = reviewerHandoffs
	caseMission.ReviewerDispatchIntakeSummary = workstream.ReviewerDispatchIntakeSummaryFor(reviewerHandoffs)
	facts, err := mission.ReadStrictLedgerFacts(status.Target)
	if err != nil {
		return fmt.Errorf("selected current lane ledger: %w", err)
	}
	pauseReviewerWaveForOpenIntervention(&caseMission.ReviewerDispatchIntakeSummary, facts.Facts)
	caseMission.ReviewerDispatchIntakeActionQueue = mission.MissionCommanderActionQueueFor(reviewerActions)
	caseMission.DailyMissionControlRunbook = workstream.DailyMissionControlRunbookFor(
		status.Target,
		"case",
		caseMission.MissionCommanderActionQueue,
		caseMission.HandoffPreviewCommand,
		caseMission.HandoffApplyCommand,
	)
	status.CaseMission = &caseMission
	status.PackMemoryConsumption = nil
	status.selectedCurrentLane = selected
	status.MissionControlRunbook = buildStatusMissionControlRunbookWithConsumption(status.Target, status.CaseMission, nil, nil)
	return nil
}

func selectedLaneActions(items []mission.MissionCommanderNextActionItem, selected string) []mission.MissionCommanderNextActionItem {
	out := make([]mission.MissionCommanderNextActionItem, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Lane) != selected {
			continue
		}
		item.Command = selectedLaneCommand(item.Command, selected)
		item.Invocation = nil
		out = append(out, item)
	}
	return mission.UniqueCommanderNextActions(out)
}

func selectedLaneReviewerHandoffs(items []workstream.ReviewerDispatchIntakeHandoff, selected string) []workstream.ReviewerDispatchIntakeHandoff {
	out := make([]workstream.ReviewerDispatchIntakeHandoff, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.TargetLane) == selected {
			out = append(out, item)
		}
	}
	return out
}

func validateStatusSelectedCurrentLane(status statusInventory, selected string, requireExecutable bool) error {
	if status.MissionControlRunbook == nil {
		return fmt.Errorf("selected current lane %q has no mission control runbook", selected)
	}
	if status.MissionControlRunbook.CurrentDriverRequest == nil {
		if !requireExecutable && statusSelectedLaneMissionComplete(status, selected) {
			return nil
		}
		return fmt.Errorf("selected current lane %q has no current driver request", selected)
	}
	route := strings.TrimSpace(status.MissionControlRunbook.Scope)
	request := status.MissionControlRunbook.CurrentDriverRequest
	if route != "case" && route != "reviewer" {
		return fmt.Errorf("selected current lane %q resolved outside the case/reviewer route", selected)
	}
	if err := validateSelectedLaneDriverRequest(request, selected, "current driver request", requireExecutable && route == "case"); err != nil {
		return err
	}
	if route == "case" &&
		((requireExecutable && request.Blocked) ||
			(!request.CommandExecutable && strings.TrimSpace(request.Guidance) == "")) {
		return fmt.Errorf("selected current lane %q current driver request is blocked or has no typed action", selected)
	}
	if route == "reviewer" && !request.CommandExecutable && strings.TrimSpace(request.Guidance) == "" && status.MissionControlRunbook.CurrentLoopOperator == nil {
		return fmt.Errorf("selected current lane %q reviewer request has no typed external handoff", selected)
	}
	if command := strings.TrimSpace(status.MissionControlRunbook.RefreshStatusCommand); command == "" || selectedLaneCommand(command, selected) != command {
		return fmt.Errorf("selected current lane %q has an invalid refresh command", selected)
	}
	if operator := status.MissionControlRunbook.CurrentLoopOperator; operator != nil {
		if strings.TrimSpace(operator.Lane) != selected {
			return fmt.Errorf("selected current lane %q resolved current-loop lane %q", selected, operator.Lane)
		}
		if err := validateSelectedLaneDriverRequest(operator.SourceCurrentDriverRequest, selected, "current-loop source request", false); err != nil {
			return err
		}
		if err := validateSelectedLaneDriverRequest(operator.SelectedDriverRequest, selected, "current-loop selected request", false); err != nil {
			return err
		}
		if err := validateSelectedLaneDriverRequest(operator.StartDriverRequest, selected, "current-loop start request", false); err != nil {
			return err
		}
		if err := validateSelectedLaneDriverRequest(operator.ResumeDriverRequest, selected, "current-loop resume request", false); err != nil {
			return err
		}
		if handoff := operator.ExternalReviewerHandoff; handoff != nil {
			if err := validateSelectedLaneReviewerAttempt(handoff.Attempt, selected, "current reviewer attempt"); err != nil {
				return err
			}
			if err := validateSelectedLaneObservationContract(handoff.ObservationContract, selected, "current reviewer observation contract"); err != nil {
				return err
			}
			if wave := handoff.Wave; wave != nil {
				if strings.TrimSpace(wave.Lane) != selected {
					return fmt.Errorf("selected current lane %q resolved reviewer wave lane %q", selected, wave.Lane)
				}
				for idx, attempt := range wave.Shards {
					if err := validateSelectedLaneReviewerAttempt(attempt, selected, fmt.Sprintf("reviewer wave shard %d", idx+1)); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func statusSelectedLaneMissionComplete(status statusInventory, selected string) bool {
	if status.CaseMission == nil || status.CaseMission.MissionCompletion == nil ||
		!status.CaseMission.MissionCompletion.Ready ||
		!status.CaseMission.MissionCompletion.OperationallyComplete ||
		status.CaseMission.MissionCompletion.State != "mission-complete" ||
		status.CaseMission.DailyMissionControlRunbook == nil ||
		status.CaseMission.DailyMissionControlRunbook.CurrentState != "mission-complete" ||
		status.CaseMission.DailyMissionControlRunbook.CurrentDriverRequest != nil {
		return false
	}
	board, err := mission.ReadBoard(status.Target)
	if err != nil {
		return false
	}
	lane, ok := mission.LookupBoardLane(board.Lanes, selected, false)
	return ok && strings.EqualFold(strings.TrimSpace(lane.Status), "closed")
}

func validateSelectedLaneReviewerAttempt(attempt *mission.CurrentLoopReviewerAttempt, selected, label string) error {
	if attempt == nil {
		return nil
	}
	if strings.TrimSpace(attempt.Identity.Lane) != selected {
		return fmt.Errorf("selected current lane %q resolved %s lane %q", selected, label, attempt.Identity.Lane)
	}
	if err := validateSelectedLaneDriverRequest(attempt.CurrentReviewerDriverRequest, selected, label+" current request", false); err != nil {
		return err
	}
	if err := validateSelectedLaneDriverRequest(attempt.DurableContinuationDriverRequest, selected, label+" durable request", false); err != nil {
		return err
	}
	if command := strings.TrimSpace(attempt.RefreshStatusCommand); command != "" && selectedLaneCommand(command, selected) != command {
		return fmt.Errorf("selected current lane %q has an invalid %s refresh command", selected, label)
	}
	return validateSelectedLaneObservationContract(attempt.SelectedAction.ObservationContract, selected, label+" selected action observation contract")
}

func validateSelectedLaneDriverRequest(request *mission.MissionCommanderDriverRequest, selected, label string, requireExecutable bool) error {
	if request == nil {
		if requireExecutable {
			return fmt.Errorf("selected current lane %q has no %s", selected, label)
		}
		return nil
	}
	if strings.TrimSpace(request.Lane) != selected {
		return fmt.Errorf("selected current lane %q resolved %s lane %q", selected, label, request.Lane)
	}
	if requireExecutable && (request.Blocked || !request.CommandExecutable) {
		return fmt.Errorf("selected current lane %q %s is blocked or not executable", selected, label)
	}
	if request.CommandExecutable {
		command := strings.TrimSpace(request.Command)
		if command == "" || selectedLaneCommand(command, selected) != command {
			return fmt.Errorf("selected current lane %q has an invalid %s command: %q", selected, label, command)
		}
	}
	for nestedLabel, command := range map[string]string{
		"expected receipt command":         request.ExpectedReceipt.Command,
		"expected receipt refresh command": request.ExpectedReceipt.RefreshStatusCommand,
	} {
		command = strings.TrimSpace(command)
		if command != "" && selectedLaneCommand(command, selected) != command {
			return fmt.Errorf("selected current lane %q has an invalid %s %s", selected, label, nestedLabel)
		}
	}
	return nil
}

func validateSelectedLaneObservationContract(contract mission.CurrentLoopObservationContract, selected, label string) error {
	for idx, alternative := range contract.Alternatives {
		command := strings.TrimSpace(alternative.PreviewCommandTemplate)
		if command == "" || selectedLaneCommand(command, selected) != command {
			return fmt.Errorf("selected current lane %q has an invalid %s alternative %d command", selected, label, idx+1)
		}
	}
	return nil
}

func bindStatusSelectedCurrentLaneCommands(status *statusInventory, selected string) {
	if status == nil || status.MissionControlRunbook == nil {
		return
	}
	runbook := status.MissionControlRunbook
	runbook.RefreshStatusCommand = selectedLaneCommand(runbook.RefreshStatusCommand, selected)
	bindSelectedLaneDriverRequest(runbook.CurrentDriverRequest, selected)
	if runbook.CurrentDriverRequest != nil {
		runbook.CurrentCommand = strings.TrimSpace(runbook.CurrentDriverRequest.Command)
	}
	bindSelectedLaneDriverRequest(runbook.HandoffPreviewDriverRequest, selected)
	bindSelectedLaneDriverRequest(runbook.HandoffApplyDriverRequest, selected)
	if runbook.CurrentLoopOperator != nil {
		bindSelectedLaneCurrentLoopOperator(runbook.CurrentLoopOperator, selected)
	}
	runbook.CurrentDriverRequestSHA256 = ""
	if runbook.CurrentDriverRequest != nil {
		runbook.CurrentDriverRequestSHA256, _ = mission.MissionCommanderDriverRequestSHA256(*runbook.CurrentDriverRequest)
	}
	runbook.CurrentDriverReceipt = statusMissionControlCurrentDriverReceipt(runbook)
	runbook.ReplacementExecutorTakeover = statusReplacementExecutorTakeoverPackageFor(status.Target, runbook, status.ProjectHandoff)
	runbook.RunLoop = statusMissionControlRunbookSteps(runbook)
	runbook.Quickstart = statusMissionControlQuickstartFor(runbook, nil)
}

func bindSelectedLaneCurrentLoopOperator(pkg *mission.CurrentLoopOperatorPackage, selected string) {
	if pkg == nil {
		return
	}
	bindSelectedLaneDriverRequest(pkg.SourceCurrentDriverRequest, selected)
	bindSelectedLaneDriverRequest(pkg.SelectedDriverRequest, selected)
	bindSelectedLaneDriverRequest(pkg.StartDriverRequest, selected)
	bindSelectedLaneDriverRequest(pkg.ResumeDriverRequest, selected)
	if pkg.ObservationInbox != nil {
		bindSelectedLaneDriverRequest(pkg.ObservationInbox.SelectedDriverRequest, selected)
	}
	if pkg.ExternalSessionJob != nil {
		bindSelectedLaneExternalSessionJob(pkg.ExternalSessionJob, selected)
	}
	if handoff := pkg.ExternalReviewerHandoff; handoff != nil {
		bindSelectedLaneReviewerAttempt(handoff.Attempt, selected)
		if wave := handoff.Wave; wave != nil {
			for _, attempt := range wave.Shards {
				bindSelectedLaneReviewerAttempt(attempt, selected)
			}
		}
	}
	bindSelectedLaneObservationContract(&pkg.ExternalMemberHandoff, pkg.ExternalReviewerHandoff, selected)
}

func bindSelectedLaneReviewerAttempt(attempt *mission.CurrentLoopReviewerAttempt, selected string) {
	if attempt == nil {
		return
	}
	bindSelectedLaneDriverRequest(attempt.CurrentReviewerDriverRequest, selected)
	bindSelectedLaneDriverRequest(attempt.DurableContinuationDriverRequest, selected)
	attempt.RefreshStatusCommand = selectedLaneCommand(attempt.RefreshStatusCommand, selected)
	for idx := range attempt.SelectedAction.ObservationContract.Alternatives {
		alternative := &attempt.SelectedAction.ObservationContract.Alternatives[idx]
		alternative.PreviewCommandTemplate = selectedLaneCommand(alternative.PreviewCommandTemplate, selected)
	}
}

func bindSelectedLaneExternalSessionJob(job *mission.CurrentLoopExternalSessionJob, selected string) {
	bindSelectedLaneDriverRequest(job.AttemptRequest, selected)
	bindSelectedLaneDriverRequest(job.RelayPreviewRequest, selected)
	if job.Dispatcher != nil {
		bindSelectedLaneDriverRequest(job.Dispatcher.ClaimRequest, selected)
		bindSelectedLaneDriverRequest(job.Dispatcher.LaunchAcceptedRequest, selected)
		bindSelectedLaneDriverRequest(job.Dispatcher.LaunchFailedRequest, selected)
	}
	if job.HarnessPackage != nil {
		job.HarnessPackage.RefreshStatusCommand = selectedLaneCommand(job.HarnessPackage.RefreshStatusCommand, selected)
		bindSelectedLaneDriverRequest(job.HarnessPackage.AttemptReviewRequest, selected)
		if job.HarnessPackage.Return != nil {
			bindSelectedLaneDriverRequest(job.HarnessPackage.Return.ReviewRequest, selected)
			bindSelectedLaneDriverRequest(job.HarnessPackage.Return.RelayRecoveryRequest, selected)
		}
	}
}

func bindSelectedLaneObservationContract(member **mission.CurrentLoopExternalMemberHandoff, reviewer *mission.CurrentLoopExternalReviewerHandoff, selected string) {
	if member != nil && *member != nil {
		for idx := range (*member).ObservationContract.Alternatives {
			alternative := &(*member).ObservationContract.Alternatives[idx]
			alternative.PreviewCommandTemplate = selectedLaneCommand(alternative.PreviewCommandTemplate, selected)
		}
	}
	if reviewer != nil {
		for idx := range reviewer.ObservationContract.Alternatives {
			alternative := &reviewer.ObservationContract.Alternatives[idx]
			alternative.PreviewCommandTemplate = selectedLaneCommand(alternative.PreviewCommandTemplate, selected)
		}
	}
}

func bindSelectedLaneDriverRequest(request *mission.MissionCommanderDriverRequest, selected string) {
	if request == nil {
		return
	}
	request.Command = selectedLaneCommand(request.Command, selected)
	request.ExpectedReceipt.Command = selectedLaneCommand(request.ExpectedReceipt.Command, selected)
	request.ExpectedReceipt.RefreshStatusCommand = selectedLaneCommand(request.ExpectedReceipt.RefreshStatusCommand, selected)
	if request.Invocation != nil && request.Command != "" {
		if invocation, err := commands.ParsePublicInvocation(request.Command); err == nil {
			request.Invocation = &invocation
		}
	}
}

func selectedLaneCommand(command, selected string) string {
	command = strings.TrimSpace(command)
	selected = strings.TrimSpace(selected)
	if command == "" || selected == "" {
		return command
	}
	fields, err := splitDriverCommand(command)
	if err != nil || len(fields) < 2 || fields[0] != "/rekit" {
		return command
	}
	laneFlag := -1
	for idx := 2; idx < len(fields); idx++ {
		if !strings.EqualFold(fields[idx], "-Lane") && !strings.EqualFold(fields[idx], "--lane") {
			continue
		}
		if laneFlag >= 0 || idx+1 >= len(fields) || strings.TrimSpace(fields[idx+1]) != selected {
			return ""
		}
		laneFlag = idx
		idx++
	}
	positional, hasPositional := selectedLaneCommandPositionalIndex(fields)
	if laneFlag >= 0 {
		if hasPositional {
			return ""
		}
		return joinDriverCommand(fields)
	}
	if hasPositional {
		positionalLane := strings.TrimSpace(fields[positional])
		commandName := strings.ToLower(strings.TrimSpace(fields[1]))
		if !selectedLanePositionalSelectorMatches(positionalLane, selected) {
			return ""
		}
		fields = append(fields[:positional], fields[positional+1:]...)
		insertAt := len(fields)
		for idx, field := range fields {
			if strings.EqualFold(field, "-WhatIf") || strings.EqualFold(field, "--what-if") || strings.EqualFold(field, "-Apply") || strings.EqualFold(field, "--apply") {
				insertAt = idx
				break
			}
		}
		selector := []string{"-Lane", selected}
		if commandName == "start" {
			selector = append(
				[]string{"-Name", positionalLane},
				selector...,
			)
		}
		fields = append(
			fields[:insertAt],
			append(selector, fields[insertAt:]...)...,
		)
		return joinDriverCommand(fields)
	}
	insertAt := len(fields)
	for idx, field := range fields {
		if strings.EqualFold(field, "-WhatIf") || strings.EqualFold(field, "--what-if") || strings.EqualFold(field, "-Apply") || strings.EqualFold(field, "--apply") {
			insertAt = idx
			break
		}
	}
	fields = append(fields[:insertAt], append([]string{"-Lane", selected}, fields[insertAt:]...)...)
	return joinDriverCommand(fields)
}

func selectedLanePositionalSelectorMatches(selector, selected string) bool {
	selector = strings.TrimSpace(selector)
	selected = strings.TrimSpace(selected)
	if selector == "" || selected == "" {
		return false
	}
	if selector == selected {
		return true
	}
	if selector == "main" && strings.HasSuffix(selected, "-main") {
		return true
	}
	label, ok := strings.CutPrefix(selected, "feature-")
	return ok && label != "" && selector == label
}

func selectedLaneCommandPositionalIndex(fields []string) (int, bool) {
	if len(fields) < 3 {
		return 0, false
	}
	switch strings.ToLower(strings.TrimSpace(fields[1])) {
	case "start", "continue", "complete", "reconcile", "handoff":
	default:
		return 0, false
	}
	for idx := 2; idx < len(fields); idx++ {
		if strings.HasPrefix(fields[idx], "-") {
			if selectedLaneCommandValueFlag(fields[idx]) {
				idx++
			}
			continue
		}
		return idx, true
	}
	return 0, false
}

func selectedLaneCommandValueFlag(flag string) bool {
	switch strings.ToLower(strings.TrimSpace(flag)) {
	case "-target", "--target", "-pack", "--pack", "-format", "--format", "-name", "--name", "-lane", "--lane", "-executor", "--executor", "-actor", "--actor", "-reason", "--reason", "-summary", "--summary", "-evidencerefs", "--evidence-refs", "-interventionid", "--intervention-id", "-expectedexecutorgeneration", "--expected-executor-generation", "-expectedcompleteplansha256", "--expected-complete-plan-sha256":
		return true
	default:
		return false
	}
}
