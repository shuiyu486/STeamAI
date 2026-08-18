package cli

import (
	"fmt"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/packmemoryconsumption"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func projectStatusPublicEntrypoint(status *statusInventory) error {
	if status == nil || !strings.HasPrefix(strings.TrimSpace(status.Mode), "case") {
		return nil
	}
	entrypoint, err := projectstate.PublicEntrypoint(status.Target)
	if err != nil {
		return fmt.Errorf("resolve status public entrypoint: %w", err)
	}
	if err := projectStatusCaseShimPublicEntrypoint(&status.CaseShim, entrypoint); err != nil {
		return fmt.Errorf("project case shim public entrypoint: %w", err)
	}
	if err := projectPackMemoryConsumptionPublicEntrypoint(status.PackMemoryConsumption, entrypoint); err != nil {
		return fmt.Errorf("project pack-memory consumption public entrypoint: %w", err)
	}
	if err := projectStatusCaseMissionPublicEntrypoint(status.CaseMission, entrypoint); err != nil {
		return fmt.Errorf("project case mission public entrypoint: %w", err)
	}
	if err := projectStatusMissionControlPublicEntrypoint(status.MissionControlRunbook, entrypoint); err != nil {
		return fmt.Errorf("project mission control public entrypoint: %w", err)
	}
	if err := projectMemberExecutionStatusPublicEntrypoint(status.MemberExecution, entrypoint); err != nil {
		return fmt.Errorf("project member execution public entrypoint: %w", err)
	}
	return nil
}

func projectStatusCaseShimPublicEntrypoint(shim *statusCaseShim, entrypoint string) error {
	if shim == nil || shim.Entrypoint == nil {
		return nil
	}
	var err error
	shim.Entrypoint.CaseLocalFirstScreenCommand, err = projectSkillEntrypointSelector(
		shim.Entrypoint.CaseLocalFirstScreenCommand,
		entrypoint,
	)
	if err != nil {
		return fmt.Errorf("entrypoint.caseLocalFirstScreenCommand: %w", err)
	}
	shim.Entrypoint.ExplicitFirstScreenCommand, err = projectPublicCommandForEntrypoint(
		shim.Entrypoint.ExplicitFirstScreenCommand,
		entrypoint,
	)
	if err != nil {
		return fmt.Errorf("entrypoint.explicitFirstScreenCommand: %w", err)
	}
	return nil
}

func projectPackMemoryConsumptionPublicEntrypoint(status *packMemoryConsumptionStatus, entrypoint string) error {
	if status == nil {
		return nil
	}
	for label, changes := range map[string][]packmemoryconsumption.ChangeStatus{
		"available": status.Discovery.Available,
		"consumed":  status.Discovery.Consumed,
		"conflicts": status.Discovery.Conflicts,
	} {
		for index := range changes {
			var err error
			changes[index].PreviewCommand, err = projectPublicCommandForEntrypoint(
				changes[index].PreviewCommand,
				entrypoint,
			)
			if err != nil {
				return fmt.Errorf("discovery.%s[%d].previewCommand: %w", label, index, err)
			}
		}
	}
	if err := projectMissionCommanderActionsPublicEntrypoint(status.MissionCommanderNextActions, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderNextActions: %w", err)
	}
	if err := projectMissionCommanderActionQueuePublicEntrypoint(&status.MissionCommanderActionQueue, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderActionQueue: %w", err)
	}
	return nil
}

func projectMemberExecutionStatusPublicEntrypoint(status *memberExecutionStatus, entrypoint string) error {
	if status == nil {
		return nil
	}
	var err error
	for label, field := range map[string]*string{
		"previewCommand":      &status.PreviewCommand,
		"observationCommand":  &status.ObservationCommand,
		"reviewerPlanCommand": &status.ReviewerPlanCommand,
	} {
		*field, err = projectPublicCommandForEntrypoint(*field, entrypoint)
		if err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
	}
	return nil
}

func projectStatusCaseMissionPublicEntrypoint(caseMission *statusCaseMission, entrypoint string) error {
	if caseMission == nil {
		return nil
	}
	for index := range caseMission.LaneExecutorActions {
		if err := projectLaneExecutorActionPublicEntrypoint(&caseMission.LaneExecutorActions[index], entrypoint); err != nil {
			return fmt.Errorf("laneExecutorActions[%d]: %w", index, err)
		}
	}
	if err := projectLaneTakeoverPublicEntrypoint(caseMission.FirstScreenLaneTakeoverPackage, entrypoint); err != nil {
		return fmt.Errorf("firstScreenLaneTakeoverPackage: %w", err)
	}
	for index := range caseMission.PendingGateHandoffs {
		if err := projectPendingGateHandoffPublicEntrypoint(&caseMission.PendingGateHandoffs[index], entrypoint); err != nil {
			return fmt.Errorf("pendingGateHandoffs[%d]: %w", index, err)
		}
	}
	for index := range caseMission.AuthorizedGateHandoffs {
		if err := projectAuthorizedGateHandoffPublicEntrypoint(&caseMission.AuthorizedGateHandoffs[index], entrypoint); err != nil {
			return fmt.Errorf("authorizedGateHandoffs[%d]: %w", index, err)
		}
	}
	for index := range caseMission.OpenDecisionHandoffs {
		if err := projectOpenDecisionHandoffPublicEntrypoint(&caseMission.OpenDecisionHandoffs[index], entrypoint); err != nil {
			return fmt.Errorf("openDecisionHandoffs[%d]: %w", index, err)
		}
	}
	for index := range caseMission.InterventionHandoffs {
		if err := projectInterventionHandoffPublicEntrypoint(&caseMission.InterventionHandoffs[index], entrypoint); err != nil {
			return fmt.Errorf("interventionHandoffs[%d]: %w", index, err)
		}
	}
	for index := range caseMission.ReviewerDispatchIntakeHandoffs {
		if err := projectReviewerDispatchIntakeHandoffPublicEntrypoint(&caseMission.ReviewerDispatchIntakeHandoffs[index], entrypoint); err != nil {
			return fmt.Errorf("reviewerDispatchIntakeHandoffs[%d]: %w", index, err)
		}
	}
	if err := projectReviewerDispatchIntakeSummaryPublicEntrypoint(&caseMission.ReviewerDispatchIntakeSummary, entrypoint); err != nil {
		return fmt.Errorf("reviewerDispatchIntakeSummary: %w", err)
	}
	for index := range caseMission.ReviewerPacketRetirementHandoffs {
		if err := projectReviewerPacketRetirementHandoffPublicEntrypoint(&caseMission.ReviewerPacketRetirementHandoffs[index], entrypoint); err != nil {
			return fmt.Errorf("reviewerPacketRetirementHandoffs[%d]: %w", index, err)
		}
	}
	if err := projectReviewerPacketRetirementSummaryPublicEntrypoint(&caseMission.ReviewerPacketRetirementSummary, entrypoint); err != nil {
		return fmt.Errorf("reviewerPacketRetirementSummary: %w", err)
	}
	if err := projectMissionCommanderActionQueuePublicEntrypoint(&caseMission.MissionCommanderActionQueue, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderActionQueue: %w", err)
	}
	if err := projectMissionCommanderActionsPublicEntrypoint(caseMission.MissionCommanderNextActions, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderNextActions: %w", err)
	}
	if err := projectMissionCommanderActionQueuePublicEntrypoint(&caseMission.ReviewerDispatchIntakeActionQueue, entrypoint); err != nil {
		return fmt.Errorf("reviewerDispatchIntakeActionQueue: %w", err)
	}
	for index := range caseMission.ExecutionEvidenceReview {
		if err := projectExecutionEvidenceReviewItemPublicEntrypoint(&caseMission.ExecutionEvidenceReview[index], entrypoint); err != nil {
			return fmt.Errorf("executionEvidenceReview[%d]: %w", index, err)
		}
	}
	caseMission.ExecutionEvidenceReviewCount = len(caseMission.ExecutionEvidenceReview)
	caseMission.ExecutionEvidenceReviewSummary = workstream.ExecutionEvidenceReviewSummaryFor(
		caseMission.ExecutionEvidenceReview,
		caseMission.MissionCommanderActionQueue,
	)
	if err := projectDailyMissionControlPublicEntrypoint(caseMission.DailyMissionControlRunbook, entrypoint); err != nil {
		return fmt.Errorf("dailyMissionControlRunbook: %w", err)
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"handoffPreviewCommand", &caseMission.HandoffPreviewCommand},
		projectPublicCommandField{"handoffApplyCommand", &caseMission.HandoffApplyCommand},
	); err != nil {
		return err
	}
	return nil
}

func projectStatusMissionControlPublicEntrypoint(runbook *statusMissionControlRunbook, entrypoint string) error {
	if runbook == nil {
		return nil
	}
	var err error
	runbook.RefreshStatusCommand, err = projectPublicCommandForEntrypoint(runbook.RefreshStatusCommand, entrypoint)
	if err != nil {
		return fmt.Errorf("refreshStatusCommand: %w", err)
	}
	runbook.CurrentCommand, err = projectPublicCommandForEntrypoint(runbook.CurrentCommand, entrypoint)
	if err != nil {
		return fmt.Errorf("currentCommand: %w", err)
	}
	runbook.HandoffPreviewCommand, err = projectPublicCommandForEntrypoint(runbook.HandoffPreviewCommand, entrypoint)
	if err != nil {
		return fmt.Errorf("handoffPreviewCommand: %w", err)
	}
	runbook.HandoffApplyCommand, err = projectPublicCommandForEntrypoint(runbook.HandoffApplyCommand, entrypoint)
	if err != nil {
		return fmt.Errorf("handoffApplyCommand: %w", err)
	}
	for label, request := range map[string]**mission.MissionCommanderDriverRequest{
		"currentDriverRequest":        &runbook.CurrentDriverRequest,
		"handoffPreviewDriverRequest": &runbook.HandoffPreviewDriverRequest,
		"handoffApplyDriverRequest":   &runbook.HandoffApplyDriverRequest,
	} {
		if err := projectMissionCommanderDriverRequestPublicEntrypoint(request, entrypoint); err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
	}
	if runbook.CurrentDriverRequest != nil && strings.TrimSpace(runbook.CurrentDriverRequest.Command) != "" {
		runbook.CurrentCommand = strings.TrimSpace(runbook.CurrentDriverRequest.Command)
	}
	for index := range runbook.Queues {
		runbook.Queues[index].CurrentCommand, err = projectPublicCommandForEntrypoint(runbook.Queues[index].CurrentCommand, entrypoint)
		if err != nil {
			return fmt.Errorf("queues[%d].currentCommand: %w", index, err)
		}
	}
	for index := range runbook.RunLoop {
		runbook.RunLoop[index].Command, err = projectPublicCommandForEntrypoint(runbook.RunLoop[index].Command, entrypoint)
		if err != nil {
			return fmt.Errorf("runLoop[%d].command: %w", index, err)
		}
	}
	if err := projectMissionCommanderDriverReceiptPublicEntrypoint(runbook.CurrentDriverReceipt, entrypoint); err != nil {
		return fmt.Errorf("currentDriverReceipt: %w", err)
	}
	if err := projectStatusMissionControlQuickstartPublicEntrypoint(runbook.Quickstart, entrypoint); err != nil {
		return fmt.Errorf("quickstart: %w", err)
	}
	if err := projectCurrentLoopSegmentPublicEntrypoint(runbook.CurrentLoopSegment, entrypoint); err != nil {
		return fmt.Errorf("currentLoopSegment: %w", err)
	}
	if err := projectCurrentLoopOperatorPublicEntrypoint(runbook.CurrentLoopOperator, entrypoint); err != nil {
		return fmt.Errorf("currentLoopOperator: %w", err)
	}
	if err := projectReplacementExecutorTakeoverPublicEntrypoint(runbook.ReplacementExecutorTakeover, entrypoint); err != nil {
		return fmt.Errorf("replacementExecutorTakeoverPackage: %w", err)
	}
	if runbook.CurrentDriverRequest == nil {
		runbook.CurrentDriverRequestSHA256 = ""
	} else {
		runbook.CurrentDriverRequestSHA256, err = mission.MissionCommanderDriverRequestSHA256(*runbook.CurrentDriverRequest)
		if err != nil {
			return fmt.Errorf("currentDriverRequestSha256: %w", err)
		}
	}
	return nil
}

func projectStatusMissionControlQuickstartPublicEntrypoint(quickstart *statusMissionControlQuickstart, entrypoint string) error {
	if quickstart == nil {
		return nil
	}
	var err error
	quickstart.Command, err = projectPublicCommandForEntrypoint(quickstart.Command, entrypoint)
	if err != nil {
		return fmt.Errorf("command: %w", err)
	}
	quickstart.RefreshStatusCommand, err = projectPublicCommandForEntrypoint(quickstart.RefreshStatusCommand, entrypoint)
	if err != nil {
		return fmt.Errorf("refreshStatusCommand: %w", err)
	}
	if err := projectMissionCommanderDriverRequestPublicEntrypoint(&quickstart.CurrentDriverRequest, entrypoint); err != nil {
		return fmt.Errorf("currentDriverRequest: %w", err)
	}
	if quickstart.CurrentDriverRequest != nil && strings.TrimSpace(quickstart.CurrentDriverRequest.Command) != "" {
		quickstart.Command = strings.TrimSpace(quickstart.CurrentDriverRequest.Command)
	}
	if err := projectMissionCommanderDriverReceiptPublicEntrypoint(quickstart.CurrentDriverReceipt, entrypoint); err != nil {
		return fmt.Errorf("currentDriverReceipt: %w", err)
	}
	if err := projectCurrentLoopOperatorPublicEntrypoint(quickstart.CurrentLoopOperator, entrypoint); err != nil {
		return fmt.Errorf("currentLoopOperator: %w", err)
	}
	return nil
}

func projectDailyMissionControlPublicEntrypoint(runbook *workstream.DailyMissionControlRunbook, entrypoint string) error {
	if runbook == nil {
		return nil
	}
	var err error
	for label, field := range map[string]*string{
		"currentCommand":        &runbook.CurrentCommand,
		"refreshStatusCommand":  &runbook.RefreshStatusCommand,
		"handoffPreviewCommand": &runbook.HandoffPreviewCommand,
		"handoffApplyCommand":   &runbook.HandoffApplyCommand,
	} {
		*field, err = projectPublicCommandForEntrypoint(*field, entrypoint)
		if err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
	}
	for label, request := range map[string]**mission.MissionCommanderDriverRequest{
		"currentDriverRequest":        &runbook.CurrentDriverRequest,
		"handoffPreviewDriverRequest": &runbook.HandoffPreviewDriverRequest,
		"handoffApplyDriverRequest":   &runbook.HandoffApplyDriverRequest,
	} {
		if err := projectMissionCommanderDriverRequestPublicEntrypoint(request, entrypoint); err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
	}
	if runbook.CurrentDriverRequest != nil && strings.TrimSpace(runbook.CurrentDriverRequest.Command) != "" {
		runbook.CurrentCommand = strings.TrimSpace(runbook.CurrentDriverRequest.Command)
	}
	for index := range runbook.RunLoop {
		runbook.RunLoop[index].Command, err = projectPublicCommandForEntrypoint(runbook.RunLoop[index].Command, entrypoint)
		if err != nil {
			return fmt.Errorf("runLoop[%d].command: %w", index, err)
		}
	}
	return nil
}

func projectLaneExecutorActionPublicEntrypoint(snapshot *mission.LaneExecutorActionSnapshot, entrypoint string) error {
	if snapshot == nil {
		return nil
	}
	return projectExecutorActionPublicEntrypoint(&snapshot.ExecutorAction, entrypoint)
}

func projectMissionBriefPublicEntrypoint(brief *mission.Brief, entrypoint string) error {
	if brief == nil {
		return nil
	}
	for index := range brief.NextAgentActions {
		projected, err := projectPublicCommandForEntrypoint(brief.NextAgentActions[index], entrypoint)
		if err != nil {
			return fmt.Errorf("nextAgentActions[%d]: %w", index, err)
		}
		brief.NextAgentActions[index] = projected
	}
	return nil
}

func projectExecutorActionPublicEntrypoint(action *mission.ExecutorAction, entrypoint string) error {
	if action == nil {
		return nil
	}
	var err error
	action.ResumeCommand, err = projectPublicCommandForEntrypoint(action.ResumeCommand, entrypoint)
	if err != nil {
		return fmt.Errorf("resumeCommand: %w", err)
	}
	action.HandoffCommand, err = projectPublicCommandForEntrypoint(action.HandoffCommand, entrypoint)
	if err != nil {
		return fmt.Errorf("handoffCommand: %w", err)
	}
	for index := range action.NextAgentActions {
		action.NextAgentActions[index], err = projectPublicCommandForEntrypoint(action.NextAgentActions[index], entrypoint)
		if err != nil {
			return fmt.Errorf("nextAgentActions[%d]: %w", index, err)
		}
	}
	if err := projectMissionCommanderActionPublicEntrypoint(&action.MissionCommanderAction, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderAction: %w", err)
	}
	return nil
}

func projectMissionCommanderActionPublicEntrypoint(action *mission.MissionCommanderAction, entrypoint string) error {
	if action == nil {
		return nil
	}
	var err error
	action.PrimaryCommand, err = projectPublicCommandForEntrypoint(action.PrimaryCommand, entrypoint)
	if err != nil {
		return fmt.Errorf("primaryCommand: %w", err)
	}
	for index := range action.FollowUpCommands {
		action.FollowUpCommands[index], err = projectPublicCommandForEntrypoint(action.FollowUpCommands[index], entrypoint)
		if err != nil {
			return fmt.Errorf("followUpCommands[%d]: %w", index, err)
		}
	}
	return nil
}

func projectLaneTakeoverPublicEntrypoint(pkg *workstream.LaneTakeoverPackage, entrypoint string) error {
	if pkg == nil {
		return nil
	}
	var err error
	for label, field := range map[string]*string{
		"continueCommand": &pkg.ContinueCommand,
		"handoffCommand":  &pkg.HandoffCommand,
		"currentCommand":  &pkg.CurrentCommand,
	} {
		*field, err = projectPublicCommandForEntrypoint(*field, entrypoint)
		if err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
	}
	if err := projectMissionCommanderActionPublicEntrypoint(&pkg.MissionCommanderAction, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderAction: %w", err)
	}
	if pkg.MissionCommanderCurrentAction != nil {
		if err := projectMissionCommanderNextActionPublicEntrypoint(pkg.MissionCommanderCurrentAction, entrypoint); err != nil {
			return fmt.Errorf("missionCommanderCurrentAction: %w", err)
		}
	}
	if err := projectMissionCommanderActionQueuePublicEntrypoint(&pkg.MissionCommanderActionQueue, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderActionQueue: %w", err)
	}
	return nil
}

func projectMissionCommanderActionQueuePublicEntrypoint(queue *mission.MissionCommanderActionQueue, entrypoint string) error {
	if queue == nil {
		return nil
	}
	if queue.CurrentAction != nil {
		if err := projectMissionCommanderNextActionPublicEntrypoint(queue.CurrentAction, entrypoint); err != nil {
			return fmt.Errorf("currentAction: %w", err)
		}
	}
	for label, actions := range map[string][]mission.MissionCommanderNextActionItem{
		"unblockedActions":      queue.UnblockedActions,
		"blockedActions":        queue.BlockedActions,
		"reviewRequiredActions": queue.ReviewRequiredActions,
		"followUpActions":       queue.FollowUpActions,
	} {
		if err := projectMissionCommanderActionsPublicEntrypoint(actions, entrypoint); err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
	}
	var err error
	for index := range queue.CurrentActionRunLoop {
		queue.CurrentActionRunLoop[index].Command, err = projectPublicCommandForEntrypoint(queue.CurrentActionRunLoop[index].Command, entrypoint)
		if err != nil {
			return fmt.Errorf("currentActionRunLoop[%d].command: %w", index, err)
		}
	}
	if err := projectMissionCommanderDriverRequestPublicEntrypoint(&queue.CurrentDriverRequest, entrypoint); err != nil {
		return fmt.Errorf("currentDriverRequest: %w", err)
	}
	return nil
}

func projectMissionCommanderActionsPublicEntrypoint(actions []mission.MissionCommanderNextActionItem, entrypoint string) error {
	for index := range actions {
		if err := projectMissionCommanderNextActionPublicEntrypoint(&actions[index], entrypoint); err != nil {
			return fmt.Errorf("[%d]: %w", index, err)
		}
	}
	return nil
}

func projectMissionCommanderNextActionPublicEntrypoint(action *mission.MissionCommanderNextActionItem, entrypoint string) error {
	if action == nil {
		return nil
	}
	projected, err := projectPublicCommandForEntrypoint(action.Command, entrypoint)
	if err != nil {
		return err
	}
	action.Command = projected
	if action.Invocation == nil || strings.TrimSpace(projected) == "" {
		return nil
	}
	parsed, err := commands.ParsePublicInvocation(projected)
	if err != nil || !action.Invocation.Equivalent(parsed) {
		return fmt.Errorf("command differs from its typed invocation")
	}
	return nil
}

func projectMissionCommanderDriverRequestPublicEntrypoint(request **mission.MissionCommanderDriverRequest, entrypoint string) error {
	if request == nil || *request == nil {
		return nil
	}
	projected, err := mission.MissionCommanderDriverRequestForEntrypoint(**request, entrypoint)
	if err != nil {
		return fmt.Errorf(
			"request source=%q state=%q executable=%t: %w",
			strings.TrimSpace((*request).Source),
			strings.TrimSpace((*request).State),
			(*request).CommandExecutable,
			err,
		)
	}
	*request = &projected
	return nil
}

func projectMissionCommanderDriverReceiptPublicEntrypoint(receipt *workstream.MissionCommanderDriverReceipt, entrypoint string) error {
	if receipt == nil {
		return nil
	}
	var err error
	receipt.Command, err = projectPublicCommandForEntrypoint(receipt.Command, entrypoint)
	if err != nil {
		return fmt.Errorf("command: %w", err)
	}
	if err := projectMissionCommanderDriverRequestPublicEntrypoint(&receipt.RefreshedCurrentDriverRequest, entrypoint); err != nil {
		return fmt.Errorf("refreshedCurrentDriverRequest: %w", err)
	}
	return nil
}

func projectReplacementExecutorTakeoverPublicEntrypoint(pkg *mission.ReplacementExecutorTakeoverPackage, entrypoint string) error {
	if pkg == nil {
		return nil
	}
	projected, err := mission.MissionCommanderDriverRequestForEntrypoint(pkg.CurrentDriverRequest, entrypoint)
	if err != nil {
		return fmt.Errorf("currentDriverRequest: %w", err)
	}
	pkg.CurrentDriverRequest = projected
	pkg.Command = strings.TrimSpace(projected.Command)
	pkg.RefreshStatusCommand, err = projectPublicCommandForEntrypoint(pkg.RefreshStatusCommand, entrypoint)
	if err != nil {
		return fmt.Errorf("refreshStatusCommand: %w", err)
	}
	if err := projectMissionCommanderDriverRequestPublicEntrypoint(&pkg.DurableArtifactRefreshDriverRequest, entrypoint); err != nil {
		return fmt.Errorf("durableArtifactRefreshDriverRequest: %w", err)
	}
	if err := projectCurrentLoopOperatorPublicEntrypoint(pkg.CurrentLoopOperator, entrypoint); err != nil {
		return fmt.Errorf("currentLoopOperator: %w", err)
	}
	pkg.CurrentDriverRequestSHA256 = mission.ReplacementExecutorDriverRequestSHA256(pkg.CurrentDriverRequest)
	if pkg.CurrentDriverRequestSHA256 == "" {
		return fmt.Errorf("current driver request identity is invalid")
	}
	return nil
}

func projectCurrentLoopOperatorPublicEntrypoint(pkg *mission.CurrentLoopOperatorPackage, entrypoint string) error {
	if pkg == nil {
		return nil
	}
	for label, request := range map[string]**mission.MissionCommanderDriverRequest{
		"sourceCurrentDriverRequest": &pkg.SourceCurrentDriverRequest,
		"selectedDriverRequest":      &pkg.SelectedDriverRequest,
		"startDriverRequest":         &pkg.StartDriverRequest,
		"resumeDriverRequest":        &pkg.ResumeDriverRequest,
	} {
		if err := projectMissionCommanderDriverRequestPublicEntrypoint(request, entrypoint); err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
	}
	if pkg.ObservationInbox != nil {
		if err := projectMissionCommanderDriverRequestPublicEntrypoint(&pkg.ObservationInbox.SelectedDriverRequest, entrypoint); err != nil {
			return fmt.Errorf("observationInbox.selectedDriverRequest: %w", err)
		}
	}
	if pkg.ExternalMemberHandoff != nil {
		if err := projectCurrentLoopObservationContractPublicEntrypoint(&pkg.ExternalMemberHandoff.ObservationContract, entrypoint); err != nil {
			return fmt.Errorf("externalMemberHandoff.observationContract: %w", err)
		}
	}
	if err := projectCurrentLoopExternalReviewerPublicEntrypoint(pkg.ExternalReviewerHandoff, entrypoint); err != nil {
		return fmt.Errorf("externalReviewerHandoff: %w", err)
	}
	if err := projectCurrentLoopExternalSessionJobPublicEntrypoint(pkg.ExternalSessionJob, entrypoint); err != nil {
		return fmt.Errorf("externalSessionJob: %w", err)
	}
	return nil
}

func projectCurrentLoopExternalReviewerPublicEntrypoint(handoff *mission.CurrentLoopExternalReviewerHandoff, entrypoint string) error {
	if handoff == nil {
		return nil
	}
	if err := projectCurrentLoopObservationContractPublicEntrypoint(&handoff.ObservationContract, entrypoint); err != nil {
		return fmt.Errorf("observationContract: %w", err)
	}
	var err error
	handoff.RecordDispatchPreviewTemplate, err = projectPublicCommandForEntrypoint(handoff.RecordDispatchPreviewTemplate, entrypoint)
	if err != nil {
		return fmt.Errorf("recordDispatchPreviewTemplate: %w", err)
	}
	if err := projectCurrentLoopReviewerAttemptPublicEntrypoint(handoff.Attempt, entrypoint); err != nil {
		return fmt.Errorf("attempt: %w", err)
	}
	if handoff.Wave != nil {
		collections := [][]*mission.CurrentLoopReviewerAttempt{
			handoff.Wave.SpawnWave,
			handoff.Wave.Active,
			handoff.Wave.Returned,
			handoff.Wave.Failed,
			handoff.Wave.Blocked,
			handoff.Wave.Complete,
			handoff.Wave.Shards,
		}
		for collectionIndex, attempts := range collections {
			for attemptIndex, attempt := range attempts {
				if err := projectCurrentLoopReviewerAttemptPublicEntrypoint(attempt, entrypoint); err != nil {
					return fmt.Errorf("wave collection %d attempt %d: %w", collectionIndex, attemptIndex, err)
				}
			}
		}
	}
	return nil
}

func projectCurrentLoopReviewerAttemptPublicEntrypoint(attempt *mission.CurrentLoopReviewerAttempt, entrypoint string) error {
	if attempt == nil {
		return nil
	}
	if err := projectCurrentLoopObservationContractPublicEntrypoint(&attempt.SelectedAction.ObservationContract, entrypoint); err != nil {
		return fmt.Errorf("selectedAction.observationContract: %w", err)
	}
	if err := projectMissionCommanderDriverRequestPublicEntrypoint(&attempt.CurrentReviewerDriverRequest, entrypoint); err != nil {
		return fmt.Errorf("currentReviewerDriverRequest: %w", err)
	}
	if err := projectMissionCommanderDriverRequestPublicEntrypoint(&attempt.DurableContinuationDriverRequest, entrypoint); err != nil {
		return fmt.Errorf("durableContinuationDriverRequest: %w", err)
	}
	var err error
	attempt.RefreshStatusCommand, err = projectPublicCommandForEntrypoint(attempt.RefreshStatusCommand, entrypoint)
	if err != nil {
		return fmt.Errorf("refreshStatusCommand: %w", err)
	}
	attempt.AttemptSnapshotSHA256 = statusCurrentLoopReviewerAttemptSHA256(attempt)
	return nil
}

func projectCurrentLoopObservationContractPublicEntrypoint(contract *mission.CurrentLoopObservationContract, entrypoint string) error {
	if contract == nil {
		return nil
	}
	for index := range contract.Alternatives {
		alternative := &contract.Alternatives[index]
		var err error
		alternative.PreviewCommandTemplate, err = projectPublicCommandForEntrypoint(alternative.PreviewCommandTemplate, entrypoint)
		if err != nil {
			return fmt.Errorf("alternatives[%d].previewCommandTemplate: %w", index, err)
		}
		alternative.ObservationPathCommand, err = projectPublicCommandForEntrypoint(alternative.ObservationPathCommand, entrypoint)
		if err != nil {
			return fmt.Errorf("alternatives[%d].observationPathCommand: %w", index, err)
		}
	}
	return nil
}

func projectCurrentLoopExternalSessionJobPublicEntrypoint(job *mission.CurrentLoopExternalSessionJob, entrypoint string) error {
	if job == nil {
		return nil
	}
	for label, request := range map[string]**mission.MissionCommanderDriverRequest{
		"attemptRequest":      &job.AttemptRequest,
		"relayPreviewRequest": &job.RelayPreviewRequest,
	} {
		if err := projectMissionCommanderDriverRequestPublicEntrypoint(request, entrypoint); err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
	}
	if job.HarnessPackage != nil {
		if err := projectMissionCommanderDriverRequestPublicEntrypoint(&job.HarnessPackage.AttemptReviewRequest, entrypoint); err != nil {
			return fmt.Errorf("harnessPackage.attemptReviewRequest: %w", err)
		}
		var err error
		job.HarnessPackage.RefreshStatusCommand, err = projectPublicCommandForEntrypoint(job.HarnessPackage.RefreshStatusCommand, entrypoint)
		if err != nil {
			return fmt.Errorf("harnessPackage.refreshStatusCommand: %w", err)
		}
		if job.HarnessPackage.Return != nil {
			if err := projectMissionCommanderDriverRequestPublicEntrypoint(&job.HarnessPackage.Return.ReviewRequest, entrypoint); err != nil {
				return fmt.Errorf("harnessPackage.return.reviewRequest: %w", err)
			}
			if err := projectMissionCommanderDriverRequestPublicEntrypoint(&job.HarnessPackage.Return.RelayRecoveryRequest, entrypoint); err != nil {
				return fmt.Errorf("harnessPackage.return.relayRecoveryRequest: %w", err)
			}
		}
	}
	if job.Dispatcher != nil {
		for label, request := range map[string]**mission.MissionCommanderDriverRequest{
			"claimRequest":          &job.Dispatcher.ClaimRequest,
			"launchAcceptedRequest": &job.Dispatcher.LaunchAcceptedRequest,
			"launchFailedRequest":   &job.Dispatcher.LaunchFailedRequest,
		} {
			if err := projectMissionCommanderDriverRequestPublicEntrypoint(request, entrypoint); err != nil {
				return fmt.Errorf("dispatcher.%s: %w", label, err)
			}
		}
	}
	if job.Transport != nil {
		for label, request := range map[string]**mission.MissionCommanderDriverRequest{
			"discoveryRequest":   &job.Transport.DiscoveryRequest,
			"deliveryRequest":    &job.Transport.DeliveryRequest,
			"launchRequest":      &job.Transport.LaunchRequest,
			"returnRequest":      &job.Transport.ReturnRequest,
			"replacementRequest": &job.Transport.ReplacementRequest,
		} {
			if err := projectMissionCommanderDriverRequestPublicEntrypoint(request, entrypoint); err != nil {
				return fmt.Errorf("transport.%s: %w", label, err)
			}
		}
	}
	return nil
}

func projectSkillEntrypointSelector(command, entrypoint string) (string, error) {
	command = strings.TrimSpace(command)
	if command == commands.LegacyPublicEntrypoint || command == commands.CurrentPublicEntrypoint {
		if entrypoint != commands.LegacyPublicEntrypoint && entrypoint != commands.CurrentPublicEntrypoint {
			return "", fmt.Errorf("unsupported public entrypoint %q", entrypoint)
		}
		return entrypoint, nil
	}
	return projectPublicCommandForEntrypoint(command, entrypoint)
}

func projectPublicCommandForEntrypoint(command, entrypoint string) (string, error) {
	original := command
	command = strings.TrimSpace(command)
	if command == "" {
		return "", nil
	}
	if !strings.HasPrefix(command, "/") {
		return original, nil
	}
	invocation, err := commands.ParsePublicInvocation(command)
	if err != nil {
		return "", err
	}
	return invocation.RenderForEntrypoint(entrypoint)
}

func projectPublicCommandOrProseForEntrypoint(text, entrypoint string) (string, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || !startsWithPublicEntrypoint(trimmed) {
		return text, nil
	}
	invocation, err := commands.ParsePublicInvocation(trimmed)
	if err != nil {
		return "", err
	}
	if !publicInvocationShapeIsCommand(invocation) {
		return text, nil
	}
	return invocation.RenderForEntrypoint(entrypoint)
}

func startsWithPublicEntrypoint(text string) bool {
	for _, entrypoint := range []string{commands.LegacyPublicEntrypoint, commands.CurrentPublicEntrypoint} {
		if text == entrypoint || strings.HasPrefix(text, entrypoint+" ") ||
			strings.HasPrefix(text, entrypoint+"\t") ||
			strings.HasPrefix(text, entrypoint+"\r") ||
			strings.HasPrefix(text, entrypoint+"\n") {
			return true
		}
	}
	return false
}

func publicInvocationShapeIsCommand(invocation commands.PublicInvocation) bool {
	if len(invocation.Arguments) == 0 || strings.HasPrefix(strings.TrimSpace(invocation.Arguments[0]), "-") {
		return true
	}
	switch invocation.Command {
	case commands.Start, commands.Handoff, commands.Complete,
		commands.Reopen, commands.Continue, commands.Reconcile:
		return len(invocation.Arguments) == 1 ||
			strings.HasPrefix(strings.TrimSpace(invocation.Arguments[1]), "-")
	default:
		return false
	}
}
