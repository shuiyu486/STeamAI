package cli

import (
	"fmt"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/overview"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func projectStartResultPublicEntrypoint(result *workstream.StartResult, caseRoot string) error {
	if result == nil {
		return nil
	}
	entrypoint, err := projectstate.PublicEntrypoint(caseRoot)
	if err != nil {
		return fmt.Errorf("resolve start public entrypoint: %w", err)
	}
	if err := projectMissionBriefPublicEntrypoint(&result.MissionBrief, entrypoint); err != nil {
		return fmt.Errorf("missionBrief: %w", err)
	}
	if err := projectLaneTakeoverPublicEntrypoint(result.LaneTakeoverPackage, entrypoint); err != nil {
		return fmt.Errorf("laneTakeoverPackage: %w", err)
	}
	if err := projectAuthorizedGateAdapterHandoffsPublicEntrypoint(result.AuthorizedGateAdapterHandoffs, entrypoint); err != nil {
		return fmt.Errorf("authorizedGateAdapterHandoffs: %w", err)
	}
	if err := projectReviewerResponsePublicEntrypoint(
		result.ReviewerDispatchIntakeHandoffs,
		&result.ReviewerDispatchIntakeSummary,
		result.ReviewerPacketRetirementHandoffs,
		&result.ReviewerPacketRetirementSummary,
		entrypoint,
	); err != nil {
		return err
	}
	if err := projectContinueGateHandoffsPublicEntrypoint(result.PendingGateHandoffs, result.OpenDecisionHandoffs, entrypoint); err != nil {
		return err
	}
	if err := projectExecutorActionPublicEntrypoint(&result.ExecutorAction, entrypoint); err != nil {
		return fmt.Errorf("executorAction: %w", err)
	}
	if err := projectMissionCommanderActionPublicEntrypoint(&result.MissionCommanderAction, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderAction: %w", err)
	}
	if err := projectMissionCommanderActionsPublicEntrypoint(result.MissionCommanderNextActions, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderNextActions: %w", err)
	}
	if err := projectMissionCommanderActionQueuePublicEntrypoint(&result.MissionCommanderActionQueue, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderActionQueue: %w", err)
	}
	if err := projectPublicCommandListPublicEntrypoint("nextSteps", result.NextSteps, entrypoint); err != nil {
		return err
	}
	return nil
}

func projectContinueResultPublicEntrypoint(result *workstream.ContinueResult, caseRoot string) error {
	if result == nil {
		return nil
	}
	entrypoint, err := projectstate.PublicEntrypoint(caseRoot)
	if err != nil {
		return fmt.Errorf("resolve continue public entrypoint: %w", err)
	}
	if err := projectExecutorActionPublicEntrypoint(&result.ExecutorAction, entrypoint); err != nil {
		return fmt.Errorf("executorAction: %w", err)
	}
	if err := projectReviewerResponsePublicEntrypoint(
		result.ReviewerDispatchIntakeHandoffs,
		&result.ReviewerDispatchIntakeSummary,
		result.ReviewerPacketRetirementHandoffs,
		&result.ReviewerPacketRetirementSummary,
		entrypoint,
	); err != nil {
		return err
	}
	if err := projectAuthorizedGateAdapterHandoffsPublicEntrypoint(result.AuthorizedGateAdapterHandoffs, entrypoint); err != nil {
		return fmt.Errorf("authorizedGateAdapterHandoffs: %w", err)
	}
	if err := projectMissionCommanderActionsPublicEntrypoint(result.MissionCommanderNextActions, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderNextActions: %w", err)
	}
	if err := projectMissionCommanderActionQueuePublicEntrypoint(&result.MissionCommanderActionQueue, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderActionQueue: %w", err)
	}
	if err := projectExecutionEvidenceReviewPublicEntrypoint(
		result.ExecutionEvidenceReview,
		result.MissionCommanderActionQueue,
		&result.ExecutionEvidenceReviewSummary,
		entrypoint,
	); err != nil {
		return fmt.Errorf("executionEvidenceReview: %w", err)
	}
	if err := projectMissionCommanderDriverReceiptPublicEntrypoint(result.MissionCommanderDriverReceipt, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderDriverReceipt: %w", err)
	}
	if err := projectMissionBriefPublicEntrypoint(&result.MissionBrief, entrypoint); err != nil {
		return fmt.Errorf("missionBrief: %w", err)
	}
	if err := projectLaneTakeoverPublicEntrypoint(result.LaneTakeoverPackage, entrypoint); err != nil {
		return fmt.Errorf("laneTakeoverPackage: %w", err)
	}
	if err := projectContinueOwnerGuardRecoveryPublicEntrypoint(result.ContinueOwnerGuardRecovery, entrypoint); err != nil {
		return fmt.Errorf("continueOwnerGuardRecovery: %w", err)
	}
	for index := range result.ReconcileHandoffs {
		if err := projectContinueReconcileHandoffPublicEntrypoint(&result.ReconcileHandoffs[index], entrypoint); err != nil {
			return fmt.Errorf("reconcileHandoffs[%d]: %w", index, err)
		}
	}
	if err := projectContinueGateHandoffsPublicEntrypoint(result.PendingGateHandoffs, result.OpenDecisionHandoffs, entrypoint); err != nil {
		return err
	}
	if err := projectPublicCommandListPublicEntrypoint("nextSteps", result.NextSteps, entrypoint); err != nil {
		return err
	}
	return nil
}

func projectHandoffResultPublicEntrypoint(result *workstream.HandoffResult, caseRoot string) error {
	if result == nil {
		return nil
	}
	entrypoint, err := projectstate.PublicEntrypoint(caseRoot)
	if err != nil {
		return fmt.Errorf("resolve handoff public entrypoint: %w", err)
	}
	if err := projectMissionBriefPublicEntrypoint(&result.MissionBrief, entrypoint); err != nil {
		return fmt.Errorf("missionBrief: %w", err)
	}
	if err := projectLaneTakeoverPublicEntrypoint(result.LaneTakeoverPackage, entrypoint); err != nil {
		return fmt.Errorf("laneTakeoverPackage: %w", err)
	}
	if err := projectLatestDriverReceiptHandoffPublicEntrypoint(result.LatestDriverReceiptHandoff, entrypoint); err != nil {
		return fmt.Errorf("latestDriverReceiptHandoff: %w", err)
	}
	if err := projectExecutorActionPublicEntrypoint(result.ExecutorAction, entrypoint); err != nil {
		return fmt.Errorf("executorAction: %w", err)
	}
	for index := range result.LaneExecutorActions {
		if err := projectLaneExecutorActionPublicEntrypoint(&result.LaneExecutorActions[index], entrypoint); err != nil {
			return fmt.Errorf("laneExecutorActions[%d]: %w", index, err)
		}
	}
	if err := projectReviewerResponsePublicEntrypoint(
		result.ReviewerDispatchIntakeHandoffs,
		&result.ReviewerDispatchIntakeSummary,
		result.ReviewerPacketRetirementHandoffs,
		&result.ReviewerPacketRetirementSummary,
		entrypoint,
	); err != nil {
		return err
	}
	if err := projectAuthorizedGateAdapterHandoffsPublicEntrypoint(result.AuthorizedGateAdapterHandoffs, entrypoint); err != nil {
		return fmt.Errorf("authorizedGateAdapterHandoffs: %w", err)
	}
	if err := projectMissionCommanderActionsPublicEntrypoint(result.MissionCommanderNextActions, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderNextActions: %w", err)
	}
	if err := projectMissionCommanderActionQueuePublicEntrypoint(&result.MissionCommanderActionQueue, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderActionQueue: %w", err)
	}
	if err := projectExecutionEvidenceReviewPublicEntrypoint(
		result.ExecutionEvidenceReview,
		result.MissionCommanderActionQueue,
		&result.ExecutionEvidenceReviewSummary,
		entrypoint,
	); err != nil {
		return fmt.Errorf("executionEvidenceReview: %w", err)
	}
	if err := projectDailyMissionControlPublicEntrypoint(result.DailyMissionControlRunbook, entrypoint); err != nil {
		return fmt.Errorf("dailyMissionControlRunbook: %w", err)
	}
	if err := projectReplacementExecutorTakeoverPublicEntrypoint(result.ReplacementExecutorTakeoverPackage, entrypoint); err != nil {
		return fmt.Errorf("replacementExecutorTakeoverPackage: %w", err)
	}
	if err := projectCurrentLoopSegmentPublicEntrypoint(result.CurrentLoopSegment, entrypoint); err != nil {
		return fmt.Errorf("currentLoopSegment: %w", err)
	}
	if err := projectCurrentLoopOperatorPublicEntrypoint(result.CurrentLoopOperator, entrypoint); err != nil {
		return fmt.Errorf("currentLoopOperator: %w", err)
	}
	if err := projectProjectNextBatchStarterPublicEntrypoint(result.ProjectNextBatchStarterPackage, entrypoint); err != nil {
		return fmt.Errorf("projectNextBatchStarterPackage: %w", err)
	}
	if err := projectPackMemoryHandoffPublicEntrypoint(result.PackMemoryConsumption, entrypoint); err != nil {
		return fmt.Errorf("packMemoryConsumption: %w", err)
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"applyCommand", &result.ApplyCommand},
	); err != nil {
		return err
	}
	if err := projectPublicCommandListPublicEntrypoint("nextSteps", result.NextSteps, entrypoint); err != nil {
		return err
	}
	return nil
}

func projectReconcileResultPublicEntrypoint(result *workstream.ReconcileResult, caseRoot string) error {
	if result == nil {
		return nil
	}
	entrypoint, err := projectstate.PublicEntrypoint(caseRoot)
	if err != nil {
		return fmt.Errorf("resolve reconcile public entrypoint: %w", err)
	}
	if err := projectMissionBriefPublicEntrypoint(&result.MissionBrief, entrypoint); err != nil {
		return fmt.Errorf("missionBrief: %w", err)
	}
	if err := projectAuthorizedGateAdapterHandoffsPublicEntrypoint(result.AuthorizedGateAdapterHandoffs, entrypoint); err != nil {
		return fmt.Errorf("authorizedGateAdapterHandoffs: %w", err)
	}
	if err := projectReviewerResponsePublicEntrypoint(
		result.ReviewerDispatchIntakeHandoffs,
		&result.ReviewerDispatchIntakeSummary,
		result.ReviewerPacketRetirementHandoffs,
		&result.ReviewerPacketRetirementSummary,
		entrypoint,
	); err != nil {
		return err
	}
	if err := projectContinueGateHandoffsPublicEntrypoint(result.PendingGateHandoffs, result.OpenDecisionHandoffs, entrypoint); err != nil {
		return err
	}
	if err := projectExecutorActionPublicEntrypoint(&result.ExecutorAction, entrypoint); err != nil {
		return fmt.Errorf("executorAction: %w", err)
	}
	if err := projectMissionCommanderActionPublicEntrypoint(&result.MissionCommanderAction, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderAction: %w", err)
	}
	if err := projectMissionCommanderActionsPublicEntrypoint(result.MissionCommanderNextActions, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderNextActions: %w", err)
	}
	if err := projectMissionCommanderActionQueuePublicEntrypoint(&result.MissionCommanderActionQueue, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderActionQueue: %w", err)
	}
	if err := projectMissionCommanderDriverReceiptPublicEntrypoint(result.MissionCommanderDriverReceipt, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderDriverReceipt: %w", err)
	}
	if err := projectReplacementExecutorTakeoverPublicEntrypoint(result.ReplacementExecutorTakeoverPackage, entrypoint); err != nil {
		return fmt.Errorf("replacementExecutorTakeoverPackage: %w", err)
	}
	if err := projectPublicCommandListPublicEntrypoint("nextSteps", result.NextSteps, entrypoint); err != nil {
		return err
	}
	return nil
}

func projectCompleteResultPublicEntrypoint(result *workstream.CompleteResult, caseRoot string) error {
	if result == nil {
		return nil
	}
	entrypoint, err := projectstate.PublicEntrypoint(caseRoot)
	if err != nil {
		return fmt.Errorf("resolve complete public entrypoint: %w", err)
	}
	if err := projectMissionBriefPublicEntrypoint(&result.MissionBrief, entrypoint); err != nil {
		return fmt.Errorf("missionBrief: %w", err)
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"applyCommand", &result.ApplyCommand},
	); err != nil {
		return err
	}
	if err := projectMissionCommanderActionsPublicEntrypoint(result.MissionCommanderNextActions, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderNextActions: %w", err)
	}
	if err := projectMissionCommanderActionQueuePublicEntrypoint(&result.MissionCommanderActionQueue, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderActionQueue: %w", err)
	}
	if err := projectPublicCommandListPublicEntrypoint("nextSteps", result.NextSteps, entrypoint); err != nil {
		return err
	}
	return nil
}

func projectReopenResultPublicEntrypoint(result *workstream.ReopenResult, caseRoot string) error {
	if result == nil {
		return nil
	}
	entrypoint, err := projectstate.PublicEntrypoint(caseRoot)
	if err != nil {
		return fmt.Errorf("resolve reopen public entrypoint: %w", err)
	}
	if err := projectMissionBriefPublicEntrypoint(&result.MissionBrief, entrypoint); err != nil {
		return fmt.Errorf("missionBrief: %w", err)
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"applyCommand", &result.ApplyCommand},
	); err != nil {
		return err
	}
	if err := projectMissionCommanderActionsPublicEntrypoint(result.MissionCommanderNextActions, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderNextActions: %w", err)
	}
	if err := projectMissionCommanderActionQueuePublicEntrypoint(&result.MissionCommanderActionQueue, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderActionQueue: %w", err)
	}
	if err := projectPublicCommandListPublicEntrypoint("nextSteps", result.NextSteps, entrypoint); err != nil {
		return err
	}
	return nil
}

func projectOverviewInventoryPublicEntrypoint(result *overview.Inventory, caseRoot string) error {
	if result == nil {
		return nil
	}
	entrypoint, err := projectstate.PublicEntrypoint(caseRoot)
	if err != nil {
		return fmt.Errorf("resolve overview public entrypoint: %w", err)
	}
	if err := projectMissionBriefPublicEntrypoint(&result.MissionBrief, entrypoint); err != nil {
		return fmt.Errorf("missionBrief: %w", err)
	}
	for index := range result.LaneExecutorActions {
		if err := projectLaneExecutorActionPublicEntrypoint(&result.LaneExecutorActions[index], entrypoint); err != nil {
			return fmt.Errorf("laneExecutorActions[%d]: %w", index, err)
		}
	}
	for index := range result.MissionCommanderActions {
		if err := projectOverviewMissionCommanderActionPublicEntrypoint(&result.MissionCommanderActions[index], entrypoint); err != nil {
			return fmt.Errorf("missionCommanderActions[%d]: %w", index, err)
		}
	}
	if err := projectMissionCommanderActionsPublicEntrypoint(result.MissionCommanderNextActions, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderNextActions: %w", err)
	}
	if err := projectMissionCommanderActionQueuePublicEntrypoint(&result.MissionCommanderActionQueue, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderActionQueue: %w", err)
	}
	if err := projectExecutionEvidenceReviewPublicEntrypoint(
		result.ExecutionEvidenceReview,
		result.MissionCommanderActionQueue,
		&result.ExecutionEvidenceReviewSummary,
		entrypoint,
	); err != nil {
		return fmt.Errorf("executionEvidenceReview: %w", err)
	}
	if err := projectAuthorizedGateAdapterHandoffsPublicEntrypoint(result.AuthorizedGateAdapterHandoffs, entrypoint); err != nil {
		return fmt.Errorf("authorizedGateAdapterHandoffs: %w", err)
	}
	if err := projectReviewerResponsePublicEntrypoint(
		result.ReviewerDispatchIntakeHandoffs,
		&result.ReviewerDispatchIntakeSummary,
		result.ReviewerPacketRetirementHandoffs,
		&result.ReviewerPacketRetirementSummary,
		entrypoint,
	); err != nil {
		return err
	}
	if err := projectPublicCommandListPublicEntrypoint("nextSteps", result.NextSteps, entrypoint); err != nil {
		return err
	}
	return nil
}

func projectOverviewMissionCommanderActionPublicEntrypoint(item *overview.MissionCommanderActionIndexItem, entrypoint string) error {
	if item == nil {
		return nil
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"primaryCommand", &item.PrimaryCommand},
	); err != nil {
		return err
	}
	for index := range item.FollowUpCommands {
		if err := projectPublicCommandFields(entrypoint,
			projectPublicCommandField{fmt.Sprintf("followUpCommands[%d]", index), &item.FollowUpCommands[index]},
		); err != nil {
			return err
		}
	}
	if err := projectMissionCommanderActionPublicEntrypoint(&item.Action, entrypoint); err != nil {
		return fmt.Errorf("action: %w", err)
	}
	return nil
}

func projectExecutionEvidenceReviewPublicEntrypoint(
	items []workstream.ExecutionEvidenceReviewItem,
	queue mission.MissionCommanderActionQueue,
	summary *workstream.ExecutionEvidenceReviewSummary,
	entrypoint string,
) error {
	for index := range items {
		if err := projectExecutionEvidenceReviewItemPublicEntrypoint(&items[index], entrypoint); err != nil {
			return fmt.Errorf("[%d]: %w", index, err)
		}
	}
	if summary != nil {
		*summary = workstream.ExecutionEvidenceReviewSummaryFor(items, queue)
	}
	return nil
}

func projectPublicCommandListPublicEntrypoint(label string, commands []string, entrypoint string) error {
	for index := range commands {
		projected, err := projectPublicCommandOrProseForEntrypoint(commands[index], entrypoint)
		if err != nil {
			return fmt.Errorf("%s[%d]: %w", label, index, err)
		}
		commands[index] = projected
	}
	return nil
}

func projectReviewerResponsePublicEntrypoint(
	intake []workstream.ReviewerDispatchIntakeHandoff,
	intakeSummary *workstream.ReviewerDispatchIntakeSummary,
	retirement []workstream.ReviewerPacketRetirementHandoff,
	retirementSummary *workstream.ReviewerPacketRetirementSummary,
	entrypoint string,
) error {
	for index := range intake {
		if err := projectReviewerDispatchIntakeHandoffPublicEntrypoint(&intake[index], entrypoint); err != nil {
			return fmt.Errorf("reviewerDispatchIntakeHandoffs[%d]: %w", index, err)
		}
	}
	if err := projectReviewerDispatchIntakeSummaryPublicEntrypoint(intakeSummary, entrypoint); err != nil {
		return fmt.Errorf("reviewerDispatchIntakeSummary: %w", err)
	}
	for index := range retirement {
		if err := projectReviewerPacketRetirementHandoffPublicEntrypoint(&retirement[index], entrypoint); err != nil {
			return fmt.Errorf("reviewerPacketRetirementHandoffs[%d]: %w", index, err)
		}
	}
	if err := projectReviewerPacketRetirementSummaryPublicEntrypoint(retirementSummary, entrypoint); err != nil {
		return fmt.Errorf("reviewerPacketRetirementSummary: %w", err)
	}
	return nil
}

func projectAuthorizedGateAdapterHandoffsPublicEntrypoint(handoffs []workstream.AuthorizedGateAdapterHandoff, entrypoint string) error {
	for index := range handoffs {
		if err := projectAuthorizedGateAdapterHandoffPublicEntrypoint(&handoffs[index], entrypoint); err != nil {
			return fmt.Errorf("[%d]: %w", index, err)
		}
	}
	return nil
}

func projectAuthorizedGateAdapterHandoffPublicEntrypoint(handoff *workstream.AuthorizedGateAdapterHandoff, entrypoint string) error {
	if handoff == nil {
		return nil
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"reportContract", &handoff.ReportContract},
		projectPublicCommandField{"handoffCommand", &handoff.HandoffCommand},
	); err != nil {
		return err
	}
	if handoff.ReportSummary != nil {
		if err := projectPublicCommandFields(entrypoint,
			projectPublicCommandField{"reportSummary.currentAction", &handoff.ReportSummary.CurrentAction},
		); err != nil {
			return err
		}
	}
	for index := range handoff.LiveValidationRepairHints {
		if err := projectPublicCommandFields(entrypoint,
			projectPublicCommandField{
				fmt.Sprintf("liveValidationRepairHints[%d].repairAction", index),
				&handoff.LiveValidationRepairHints[index].RepairAction,
			},
		); err != nil {
			return err
		}
	}
	if err := projectAuthorizedGateAdapterLiveValidationPublicEntrypoint(handoff.LiveValidation, entrypoint); err != nil {
		return fmt.Errorf("liveValidation: %w", err)
	}
	return nil
}

func projectAuthorizedGateAdapterLiveValidationPublicEntrypoint(handoff *workstream.AuthorizedGateLiveValidationHandoff, entrypoint string) error {
	if handoff == nil {
		return nil
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"dispatchCommand", &handoff.DispatchCommand},
		projectPublicCommandField{"validateCommand", &handoff.ValidateCommand},
		projectPublicCommandField{"recordCommand", &handoff.RecordCommand},
		projectPublicCommandField{"scaffoldCommand", &handoff.ScaffoldCommand},
		projectPublicCommandField{"scaffoldApplyCommand", &handoff.ScaffoldApplyCommand},
		projectPublicCommandField{"draftCommand", &handoff.DraftCommand},
		projectPublicCommandField{"draftApplyCommand", &handoff.DraftApplyCommand},
		projectPublicCommandField{"receiptPreviewCommand", &handoff.ReceiptPreviewCommand},
		projectPublicCommandField{"caseRelativeValidateCommand", &handoff.CaseRelativeValidateCommand},
		projectPublicCommandField{"caseRelativeRecordCommand", &handoff.CaseRelativeRecordCommand},
		projectPublicCommandField{"caseRelativeScaffoldCommand", &handoff.CaseRelativeScaffoldCommand},
		projectPublicCommandField{"caseRelativeScaffoldApplyCommand", &handoff.CaseRelativeScaffoldApplyCommand},
		projectPublicCommandField{"caseRelativeDraftCommand", &handoff.CaseRelativeDraftCommand},
		projectPublicCommandField{"caseRelativeDraftApplyCommand", &handoff.CaseRelativeDraftApplyCommand},
	); err != nil {
		return err
	}
	for index := range handoff.RunLoop {
		if err := projectPublicCommandFields(entrypoint,
			projectPublicCommandField{fmt.Sprintf("runLoop[%d].command", index), &handoff.RunLoop[index].Command},
		); err != nil {
			return err
		}
	}
	return nil
}

func projectContinueGateHandoffsPublicEntrypoint(
	pending []workstream.ContinuePendingGateHandoff,
	open []workstream.ContinueOpenDecisionHandoff,
	entrypoint string,
) error {
	for index := range pending {
		if err := projectContinuePendingGateHandoffPublicEntrypoint(&pending[index], entrypoint); err != nil {
			return fmt.Errorf("pendingGateHandoffs[%d]: %w", index, err)
		}
	}
	for index := range open {
		if err := projectContinueOpenDecisionHandoffPublicEntrypoint(&open[index], entrypoint); err != nil {
			return fmt.Errorf("openDecisionHandoffs[%d]: %w", index, err)
		}
	}
	return nil
}

func projectContinueReconcileHandoffPublicEntrypoint(handoff *workstream.ContinueReconcileHandoff, entrypoint string) error {
	if handoff == nil {
		return nil
	}
	return projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"reviewCommand", &handoff.ReviewCommand},
		projectPublicCommandField{"whatIfCommand", &handoff.WhatIfCommand},
		projectPublicCommandField{"applyCommand", &handoff.ApplyCommand},
	)
}

func projectContinuePendingGateHandoffPublicEntrypoint(handoff *workstream.ContinuePendingGateHandoff, entrypoint string) error {
	if handoff == nil {
		return nil
	}
	return projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"reviewCommand", &handoff.ReviewCommand},
		projectPublicCommandField{"whatIfCommand", &handoff.WhatIfCommand},
		projectPublicCommandField{"applyCommand", &handoff.ApplyCommand},
	)
}

func projectContinueOpenDecisionHandoffPublicEntrypoint(handoff *workstream.ContinueOpenDecisionHandoff, entrypoint string) error {
	if handoff == nil {
		return nil
	}
	return projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"sourceCommand", &handoff.SourceCommand},
		projectPublicCommandField{"reviewCommand", &handoff.ReviewCommand},
		projectPublicCommandField{"whatIfCommand", &handoff.WhatIfCommand},
		projectPublicCommandField{"recordCommand", &handoff.RecordCommand},
	)
}

func projectContinueOwnerGuardRecoveryPublicEntrypoint(recovery *workstream.ContinueOwnerGuardRecovery, entrypoint string) error {
	if recovery == nil {
		return nil
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"currentContinueCommand", &recovery.CurrentContinueCommand},
		projectPublicCommandField{"handoffCommand", &recovery.HandoffCommand},
		projectPublicCommandField{"startTakeoverPreviewCommand", &recovery.StartTakeoverPreviewCommand},
		projectPublicCommandField{"startTakeoverApplyCommand", &recovery.StartTakeoverApplyCommand},
	); err != nil {
		return err
	}
	if err := projectLaneTakeoverPublicEntrypoint(recovery.LaneTakeoverPackage, entrypoint); err != nil {
		return fmt.Errorf("laneTakeoverPackage: %w", err)
	}
	if err := projectMissionCommanderNextActionPublicEntrypoint(recovery.MissionCommanderCurrentAction, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderCurrentAction: %w", err)
	}
	if err := projectMissionCommanderActionQueuePublicEntrypoint(&recovery.MissionCommanderActionQueue, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderActionQueue: %w", err)
	}
	return nil
}

func projectLatestDriverReceiptHandoffPublicEntrypoint(handoff *workstream.LatestDriverReceiptHandoff, entrypoint string) error {
	if handoff == nil {
		return nil
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"command", &handoff.Command},
	); err != nil {
		return err
	}
	if err := projectMissionCommanderDriverReceiptPublicEntrypoint(handoff.MissionCommanderDriverReceipt, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderDriverReceipt: %w", err)
	}
	return nil
}

func projectProjectNextBatchStarterPublicEntrypoint(pkg *workstream.ProjectNextBatchStarterPackage, entrypoint string) error {
	if pkg == nil {
		return nil
	}
	for index := range pkg.RunLoop {
		if err := projectPublicCommandFields(entrypoint,
			projectPublicCommandField{fmt.Sprintf("runLoop[%d].command", index), &pkg.RunLoop[index].Command},
		); err != nil {
			return err
		}
	}
	return nil
}

func projectPackMemoryHandoffPublicEntrypoint(handoff *workstream.PackMemoryConsumptionHandoff, entrypoint string) error {
	if handoff == nil {
		return nil
	}
	if err := projectMissionCommanderActionsPublicEntrypoint(handoff.MissionCommanderNextActions, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderNextActions: %w", err)
	}
	if err := projectMissionCommanderActionQueuePublicEntrypoint(&handoff.MissionCommanderActionQueue, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderActionQueue: %w", err)
	}
	return nil
}
