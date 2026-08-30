package cli

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/overview"
	"github.com/shuiyu486/re-context-kits/internal/rekit/subagents"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

type gatePlanDiagnosticsDTO gate.Plan

type gateApplyDiagnosticsDTO gate.ApplyResult

type startDiagnosticsDTO workstream.StartResult

type handoffDiagnosticsDTO workstream.HandoffResult

type reconcileDiagnosticsDTO workstream.ReconcileResult

type completeDiagnosticsDTO workstream.CompleteResult

type reopenDiagnosticsDTO workstream.ReopenResult

type driverStepDiagnosticsDTO driverStepPlan

type reviewerStepDiagnosticsDTO reviewerStepPlan

type currentStepDiagnosticsDTO currentStepPlan

type currentLoopDiagnosticsDTO currentLoopPlan

type currentLoopExternalSessionAttemptDiagnosticsDTO currentLoopExternalSessionAttemptResult

type currentLoopExternalSessionDispatchDiagnosticsDTO currentLoopExternalSessionDispatchResult

type currentLoopExternalSessionRelayDiagnosticsDTO currentLoopExternalSessionRelayResult

type currentLoopExternalSessionTurnDiagnosticsDTO currentLoopExternalSessionTurnPlan

func buildGatePlanDiagnosticsDTO(result gate.Plan, caseRoot string) (gatePlanDiagnosticsDTO, error) {
	var diagnostics gatePlanDiagnosticsDTO
	if err := cloneDiagnostics(result, &diagnostics, "gate plan"); err != nil {
		return gatePlanDiagnosticsDTO{}, err
	}
	if err := projectGatePlanPublicEntrypoint((*gate.Plan)(&diagnostics), caseRoot); err != nil {
		return gatePlanDiagnosticsDTO{}, err
	}
	return diagnostics, nil
}

func buildGateApplyDiagnosticsDTO(result gate.ApplyResult, caseRoot string) (gateApplyDiagnosticsDTO, error) {
	var diagnostics gateApplyDiagnosticsDTO
	if err := cloneDiagnostics(result, &diagnostics, "gate Apply"); err != nil {
		return gateApplyDiagnosticsDTO{}, err
	}
	if err := projectGateApplyPublicEntrypoint((*gate.ApplyResult)(&diagnostics), caseRoot); err != nil {
		return gateApplyDiagnosticsDTO{}, err
	}
	return diagnostics, nil
}

func buildStartDiagnosticsDTO(result workstream.StartResult, caseRoot string) (startDiagnosticsDTO, error) {
	var diagnostics startDiagnosticsDTO
	if err := cloneDiagnostics(result, &diagnostics, "start"); err != nil {
		return startDiagnosticsDTO{}, err
	}
	if err := projectStartResultPublicEntrypoint((*workstream.StartResult)(&diagnostics), caseRoot); err != nil {
		return startDiagnosticsDTO{}, err
	}
	return diagnostics, nil
}

func buildHandoffDiagnosticsDTO(result workstream.HandoffResult, caseRoot string) (handoffDiagnosticsDTO, error) {
	var diagnostics handoffDiagnosticsDTO
	if err := cloneDiagnostics(result, &diagnostics, "handoff"); err != nil {
		return handoffDiagnosticsDTO{}, err
	}
	if err := projectHandoffResultPublicEntrypoint((*workstream.HandoffResult)(&diagnostics), caseRoot); err != nil {
		return handoffDiagnosticsDTO{}, err
	}
	return diagnostics, nil
}

func buildReconcileDiagnosticsDTO(result workstream.ReconcileResult, caseRoot string) (reconcileDiagnosticsDTO, error) {
	var diagnostics reconcileDiagnosticsDTO
	if err := cloneDiagnostics(result, &diagnostics, "reconcile"); err != nil {
		return reconcileDiagnosticsDTO{}, err
	}
	if err := projectReconcileResultPublicEntrypoint((*workstream.ReconcileResult)(&diagnostics), caseRoot); err != nil {
		return reconcileDiagnosticsDTO{}, err
	}
	return diagnostics, nil
}

func buildCompleteDiagnosticsDTO(result workstream.CompleteResult, caseRoot string) (completeDiagnosticsDTO, error) {
	var diagnostics completeDiagnosticsDTO
	if err := cloneDiagnostics(result, &diagnostics, "complete"); err != nil {
		return completeDiagnosticsDTO{}, err
	}
	if err := projectCompleteResultPublicEntrypoint((*workstream.CompleteResult)(&diagnostics), caseRoot); err != nil {
		return completeDiagnosticsDTO{}, err
	}
	return diagnostics, nil
}

func buildReopenDiagnosticsDTO(result workstream.ReopenResult, caseRoot string) (reopenDiagnosticsDTO, error) {
	var diagnostics reopenDiagnosticsDTO
	if err := cloneDiagnostics(result, &diagnostics, "reopen"); err != nil {
		return reopenDiagnosticsDTO{}, err
	}
	if err := projectReopenResultPublicEntrypoint((*workstream.ReopenResult)(&diagnostics), caseRoot); err != nil {
		return reopenDiagnosticsDTO{}, err
	}
	return diagnostics, nil
}

func buildDriverStepDiagnosticsDTO(result driverStepPlan, caseRoot string) (driverStepDiagnosticsDTO, error) {
	var diagnostics driverStepDiagnosticsDTO
	if err := cloneDiagnostics(result, &diagnostics, "driver step"); err != nil {
		return driverStepDiagnosticsDTO{}, err
	}
	projected := (*driverStepPlan)(&diagnostics)
	projected.PreviewResult = result.PreviewResult
	projection, err := resolveProjectPublicProjection(caseRoot)
	if err != nil {
		return driverStepDiagnosticsDTO{}, fmt.Errorf("resolve driver-step public entrypoint: %w", err)
	}
	if err := projectDriverStepPlanPublicEntrypoint(projected, projection.entrypoint); err != nil {
		return driverStepDiagnosticsDTO{}, err
	}
	return diagnostics, nil
}

func buildReviewerStepDiagnosticsDTO(result reviewerStepPlan, caseRoot string) (reviewerStepDiagnosticsDTO, error) {
	var diagnostics reviewerStepDiagnosticsDTO
	if err := cloneDiagnostics(result, &diagnostics, "reviewer step"); err != nil {
		return reviewerStepDiagnosticsDTO{}, err
	}
	projected := (*reviewerStepPlan)(&diagnostics)
	projected.PreviewResult = result.PreviewResult
	projection, err := resolveProjectPublicProjection(caseRoot)
	if err != nil {
		return reviewerStepDiagnosticsDTO{}, fmt.Errorf("resolve reviewer-step public entrypoint: %w", err)
	}
	if err := projectReviewerStepPlanPublicEntrypoint(projected, projection.entrypoint); err != nil {
		return reviewerStepDiagnosticsDTO{}, err
	}
	return diagnostics, nil
}

func buildCurrentStepDiagnosticsDTO(result currentStepPlan, caseRoot string) (currentStepDiagnosticsDTO, error) {
	var diagnostics currentStepDiagnosticsDTO
	if err := cloneDiagnostics(result, &diagnostics, "current step"); err != nil {
		return currentStepDiagnosticsDTO{}, err
	}
	projected := (*currentStepPlan)(&diagnostics)
	copyCurrentStepKnownPreviewResults(&result, projected)
	projection, err := resolveProjectPublicProjection(caseRoot)
	if err != nil {
		return currentStepDiagnosticsDTO{}, fmt.Errorf("resolve current-step public entrypoint: %w", err)
	}
	if err := projectCurrentStepPlanPublicEntrypoint(projected, projection.entrypoint); err != nil {
		return currentStepDiagnosticsDTO{}, err
	}
	return diagnostics, nil
}

func buildCurrentLoopDiagnosticsDTO(result currentLoopPlan, caseRoot string) (currentLoopDiagnosticsDTO, error) {
	var diagnostics currentLoopDiagnosticsDTO
	if err := cloneDiagnostics(result, &diagnostics, "current loop"); err != nil {
		return currentLoopDiagnosticsDTO{}, err
	}
	projected := (*currentLoopPlan)(&diagnostics)
	copyCurrentLoopKnownPreviewResults(result, projected)
	if err := projectCurrentLoopPlanPublicEntrypoint(projected, caseRoot); err != nil {
		return currentLoopDiagnosticsDTO{}, err
	}
	return diagnostics, nil
}

func copyCurrentLoopKnownPreviewResults(source currentLoopPlan, target *currentLoopPlan) {
	if target == nil {
		return
	}
	copyCurrentStepKnownPreviewResults(source.InitialCurrentStep, target.InitialCurrentStep)
}

func copyCurrentStepKnownPreviewResults(source, target *currentStepPlan) {
	if source == nil || target == nil {
		return
	}
	if source.DriverStep != nil && target.DriverStep != nil {
		target.DriverStep.PreviewResult = source.DriverStep.PreviewResult
	}
	if source.ReviewerStep != nil && target.ReviewerStep != nil {
		target.ReviewerStep.PreviewResult = source.ReviewerStep.PreviewResult
	}
	if source.ExternalSessionStep != nil && target.ExternalSessionStep != nil &&
		source.ExternalSessionStep.Turn != nil && target.ExternalSessionStep.Turn != nil {
		copyCurrentLoopKnownPreviewResults(
			source.ExternalSessionStep.Turn.Resume,
			&target.ExternalSessionStep.Turn.Resume,
		)
	}
}

func buildCurrentLoopExternalSessionAttemptDiagnosticsDTO(result currentLoopExternalSessionAttemptResult, caseRoot string) (currentLoopExternalSessionAttemptDiagnosticsDTO, error) {
	var diagnostics currentLoopExternalSessionAttemptDiagnosticsDTO
	if err := cloneDiagnostics(result, &diagnostics, "current-loop external session attempt"); err != nil {
		return currentLoopExternalSessionAttemptDiagnosticsDTO{}, err
	}
	projection, err := resolveProjectPublicProjection(caseRoot)
	if err != nil {
		return currentLoopExternalSessionAttemptDiagnosticsDTO{}, fmt.Errorf("resolve current-loop external session attempt public entrypoint: %w", err)
	}
	projected := (*currentLoopExternalSessionAttemptResult)(&diagnostics)
	if err := projectPublicCommandFields(projection.entrypoint,
		projectPublicCommandField{"applyCommand", &projected.ApplyCommand},
	); err != nil {
		return currentLoopExternalSessionAttemptDiagnosticsDTO{}, err
	}
	if projected.RefreshedStatus != nil {
		if err := projectStatusPublicEntrypoint(projected.RefreshedStatus); err != nil {
			return currentLoopExternalSessionAttemptDiagnosticsDTO{}, fmt.Errorf("refreshedStatus: %w", err)
		}
	}
	return diagnostics, nil
}

func buildCurrentLoopExternalSessionDispatchDiagnosticsDTO(result currentLoopExternalSessionDispatchResult, caseRoot string) (currentLoopExternalSessionDispatchDiagnosticsDTO, error) {
	var diagnostics currentLoopExternalSessionDispatchDiagnosticsDTO
	if err := cloneDiagnostics(result, &diagnostics, "current-loop external session dispatch"); err != nil {
		return currentLoopExternalSessionDispatchDiagnosticsDTO{}, err
	}
	projection, err := resolveProjectPublicProjection(caseRoot)
	if err != nil {
		return currentLoopExternalSessionDispatchDiagnosticsDTO{}, fmt.Errorf("resolve current-loop external session dispatch public entrypoint: %w", err)
	}
	projected := (*currentLoopExternalSessionDispatchResult)(&diagnostics)
	if err := projectPublicCommandFields(projection.entrypoint,
		projectPublicCommandField{"applyCommand", &projected.ApplyCommand},
	); err != nil {
		return currentLoopExternalSessionDispatchDiagnosticsDTO{}, err
	}
	if projected.RefreshedStatus != nil {
		if err := projectStatusPublicEntrypoint(projected.RefreshedStatus); err != nil {
			return currentLoopExternalSessionDispatchDiagnosticsDTO{}, fmt.Errorf("refreshedStatus: %w", err)
		}
	}
	return diagnostics, nil
}

func buildCurrentLoopExternalSessionRelayDiagnosticsDTO(result currentLoopExternalSessionRelayResult, caseRoot string) (currentLoopExternalSessionRelayDiagnosticsDTO, error) {
	var diagnostics currentLoopExternalSessionRelayDiagnosticsDTO
	if err := cloneDiagnostics(result, &diagnostics, "current-loop external session relay"); err != nil {
		return currentLoopExternalSessionRelayDiagnosticsDTO{}, err
	}
	projection, err := resolveProjectPublicProjection(caseRoot)
	if err != nil {
		return currentLoopExternalSessionRelayDiagnosticsDTO{}, fmt.Errorf("resolve current-loop external session relay public entrypoint: %w", err)
	}
	projected := (*currentLoopExternalSessionRelayResult)(&diagnostics)
	if err := projectPublicCommandFields(projection.entrypoint,
		projectPublicCommandField{"applyCommand", &projected.ApplyCommand},
	); err != nil {
		return currentLoopExternalSessionRelayDiagnosticsDTO{}, err
	}
	if projected.RefreshedStatus != nil {
		if err := projectStatusPublicEntrypoint(projected.RefreshedStatus); err != nil {
			return currentLoopExternalSessionRelayDiagnosticsDTO{}, fmt.Errorf("refreshedStatus: %w", err)
		}
	}
	return diagnostics, nil
}

func buildCurrentLoopExternalSessionTurnDiagnosticsDTO(result currentLoopExternalSessionTurnPlan, caseRoot string) (currentLoopExternalSessionTurnDiagnosticsDTO, error) {
	var diagnostics currentLoopExternalSessionTurnDiagnosticsDTO
	if err := cloneDiagnostics(result, &diagnostics, "current-loop external session turn"); err != nil {
		return currentLoopExternalSessionTurnDiagnosticsDTO{}, err
	}
	copyCurrentLoopKnownPreviewResults(
		result.Resume,
		&((*currentLoopExternalSessionTurnPlan)(&diagnostics).Resume),
	)
	projection, err := resolveProjectPublicProjection(caseRoot)
	if err != nil {
		return currentLoopExternalSessionTurnDiagnosticsDTO{}, fmt.Errorf("resolve current-loop external session turn public entrypoint: %w", err)
	}
	if err := projectCurrentLoopExternalSessionTurnPublicEntrypoint((*currentLoopExternalSessionTurnPlan)(&diagnostics), projection.entrypoint); err != nil {
		return currentLoopExternalSessionTurnDiagnosticsDTO{}, err
	}
	return diagnostics, nil
}

func projectGatePlanPublicEntrypoint(result *gate.Plan, caseRoot string) error {
	if result == nil {
		return nil
	}
	projection, err := resolveProjectPublicProjection(caseRoot)
	if err != nil {
		return fmt.Errorf("resolve gate plan public entrypoint: %w", err)
	}
	entrypoint := projection.entrypoint
	if err := projectMissionBriefPublicEntrypoint(&result.MissionBrief, entrypoint); err != nil {
		return fmt.Errorf("missionBrief: %w", err)
	}
	if err := projectExecutorActionPublicEntrypoint(&result.ExecutorAction, entrypoint); err != nil {
		return fmt.Errorf("executorAction: %w", err)
	}
	if err := projectExecutorActionPublicEntrypoint(&result.WouldExecutorAction, entrypoint); err != nil {
		return fmt.Errorf("wouldExecutorAction: %w", err)
	}
	if err := projectMissionCommanderActionPublicEntrypoint(&result.MissionCommanderAction, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderAction: %w", err)
	}
	if err := projectMissionCommanderActionsPublicEntrypoint(result.MissionCommanderNextActions, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderNextActions: %w", err)
	}
	return projectPublicCommandListPublicEntrypoint("nextSteps", result.NextSteps, entrypoint)
}

func projectGateApplyPublicEntrypoint(result *gate.ApplyResult, caseRoot string) error {
	if result == nil {
		return nil
	}
	projection, err := resolveProjectPublicProjection(caseRoot)
	if err != nil {
		return fmt.Errorf("resolve gate Apply public entrypoint: %w", err)
	}
	entrypoint := projection.entrypoint
	if err := projectMissionBriefPublicEntrypoint(&result.MissionBrief, entrypoint); err != nil {
		return fmt.Errorf("missionBrief: %w", err)
	}
	if err := projectExecutorActionPublicEntrypoint(&result.ExecutorAction, entrypoint); err != nil {
		return fmt.Errorf("executorAction: %w", err)
	}
	if err := projectMissionCommanderActionPublicEntrypoint(&result.MissionCommanderAction, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderAction: %w", err)
	}
	for index := range result.ExecutionEvidenceReview {
		if err := projectExecutionEvidenceReviewItemPublicEntrypoint(&result.ExecutionEvidenceReview[index], entrypoint); err != nil {
			return fmt.Errorf("executionEvidenceReview[%d]: %w", index, err)
		}
	}
	if err := projectAuthorizedExecutionFollowThroughPublicEntrypoint(&result.AuthorizedExecutionFollowThrough, entrypoint); err != nil {
		return fmt.Errorf("authorizedExecutionFollowThrough: %w", err)
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
	if err := projectPublicCommandListPublicEntrypoint("runbookSteps", result.RunbookSteps, entrypoint); err != nil {
		return err
	}
	return projectPublicCommandListPublicEntrypoint("nextSteps", result.NextSteps, entrypoint)
}

func projectAuthorizedExecutionFollowThroughPublicEntrypoint(follow *gate.AuthorizedExecutionFollowThrough, entrypoint string) error {
	if follow == nil {
		return nil
	}
	for index := range follow.Outcomes {
		outcome := &follow.Outcomes[index]
		if err := projectPublicCommandFields(entrypoint,
			projectPublicCommandField{fmt.Sprintf("outcomes[%d].command", index), &outcome.Command},
		); err != nil {
			return err
		}
		if err := projectPublicCommandListPublicEntrypoint(
			fmt.Sprintf("outcomes[%d].verificationCommands", index),
			outcome.VerificationCommands,
			entrypoint,
		); err != nil {
			return err
		}
	}
	return projectMissionCommanderActionQueuePublicEntrypoint(&follow.ActionQueue, entrypoint)
}

func projectStartResultPublicEntrypoint(result *workstream.StartResult, caseRoot string) error {
	projection, err := resolveProjectPublicProjection(caseRoot)
	if err != nil {
		return fmt.Errorf("resolve start public entrypoint: %w", err)
	}
	return projectStartResultForEntrypoint(result, projection.entrypoint)
}

func projectStartResultForEntrypoint(result *workstream.StartResult, entrypoint string) error {
	if result == nil {
		return nil
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

// continueDiagnosticsDTO is the full machine-facing continue response. It is a
// wire-faithful deep clone of the durable domain result, so entrypoint
// presentation cannot mutate the value that owns plan and receipt identity.
type continueDiagnosticsDTO workstream.ContinueResult

func cloneDiagnostics(source, target any, label string) error {
	data, err := json.Marshal(source)
	if err != nil {
		return fmt.Errorf("clone %s diagnostics: %w", label, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("clone %s diagnostics: %w", label, err)
	}
	return nil
}

func buildContinueDiagnosticsDTO(result workstream.ContinueResult, caseRoot string) (continueDiagnosticsDTO, error) {
	var diagnostics continueDiagnosticsDTO
	if err := cloneDiagnostics(result, &diagnostics, "continue"); err != nil {
		return continueDiagnosticsDTO{}, err
	}
	projected := (*workstream.ContinueResult)(&diagnostics)
	if err := projectContinueDiagnosticsPublicEntrypoint(projected, caseRoot); err != nil {
		return continueDiagnosticsDTO{}, err
	}
	return diagnostics, nil
}

func projectContinueDiagnosticsPublicEntrypoint(result *workstream.ContinueResult, caseRoot string) error {
	projection, err := resolveProjectPublicProjection(caseRoot)
	if err != nil {
		return fmt.Errorf("resolve continue public entrypoint: %w", err)
	}
	return projectContinueResultForEntrypoint(result, projection.entrypoint)
}

func projectContinueResultForEntrypoint(result *workstream.ContinueResult, entrypoint string) error {
	if result == nil {
		return nil
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
	projection, err := resolveProjectPublicProjection(caseRoot)
	if err != nil {
		return fmt.Errorf("resolve handoff public entrypoint: %w", err)
	}
	entrypoint := projection.entrypoint
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
	projection, err := resolveProjectPublicProjection(caseRoot)
	if err != nil {
		return fmt.Errorf("resolve reconcile public entrypoint: %w", err)
	}
	return projectReconcileResultForEntrypoint(result, projection.entrypoint)
}

func projectReconcileResultForEntrypoint(result *workstream.ReconcileResult, entrypoint string) error {
	if result == nil {
		return nil
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

// ProjectCompleteResultForPublicEntrypoint projects only explicit public
// command carriers. Callers must pass a detached response value, not the
// durable result.
func ProjectCompleteResultForPublicEntrypoint(result *workstream.CompleteResult, entrypoint string) error {
	if result == nil {
		return nil
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

func projectCompleteResultPublicEntrypoint(result *workstream.CompleteResult, caseRoot string) error {
	projection, err := resolveProjectPublicProjection(caseRoot)
	if err != nil {
		return fmt.Errorf("resolve complete public entrypoint: %w", err)
	}
	return ProjectCompleteResultForPublicEntrypoint(result, projection.entrypoint)
}

func projectCurrentLoopPlanPublicEntrypoint(result *currentLoopPlan, caseRoot string) error {
	if result == nil {
		return nil
	}
	projection, err := resolveProjectPublicProjection(caseRoot)
	if err != nil {
		return fmt.Errorf("resolve current-loop public entrypoint: %w", err)
	}
	return projectCurrentLoopPlanForEntrypoint(result, projection.entrypoint)
}

func projectCurrentLoopPlanForEntrypoint(result *currentLoopPlan, entrypoint string) error {
	if result == nil {
		return nil
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"resumeCommand", &result.ResumeCommand},
		projectPublicCommandField{"applyCommand", &result.ApplyCommand},
	); err != nil {
		return err
	}
	if err := projectMissionCommanderDriverRequestPublicEntrypoint(&result.InitialCurrentDriverRequest, entrypoint); err != nil {
		return fmt.Errorf("initialCurrentDriverRequest: %w", err)
	}
	if err := projectCurrentStepPlanPublicEntrypoint(result.InitialCurrentStep, entrypoint); err != nil {
		return fmt.Errorf("initialCurrentStep: %w", err)
	}
	if err := projectCurrentLoopStopReasonPublicEntrypoint(&result.StopReason, entrypoint); err != nil {
		return fmt.Errorf("stopReason: %w", err)
	}
	if err := projectCurrentLoopPlanContinuationPublicEntrypoint(result.ContinuationRequest, entrypoint); err != nil {
		return fmt.Errorf("continuationRequest: %w", err)
	}
	if err := projectCurrentLoopSegmentPublicEntrypoint(result.ResumeSource, entrypoint); err != nil {
		return fmt.Errorf("resumeSource: %w", err)
	}
	if err := projectCurrentLoopSegmentPublicEntrypoint(result.SegmentCheckpoint, entrypoint); err != nil {
		return fmt.Errorf("segmentCheckpoint: %w", err)
	}
	if result.FinalStatus != nil {
		if err := projectStatusPublicEntrypoint(result.FinalStatus); err != nil {
			return fmt.Errorf("finalStatus: %w", err)
		}
	}
	for index := range result.Steps {
		step := &result.Steps[index]
		if err := projectMissionCommanderDriverRequestValuePublicEntrypoint(&step.RequestBefore, entrypoint); err != nil {
			return fmt.Errorf("steps[%d].requestBefore: %w", index, err)
		}
		if err := projectMissionCommanderDriverRequestPublicEntrypoint(&step.RequestAfter, entrypoint); err != nil {
			return fmt.Errorf("steps[%d].requestAfter: %w", index, err)
		}
		if step.CurrentStepReceipt != nil {
			if err := projectCurrentStepReceiptPublicEntrypoint(step.CurrentStepReceipt, entrypoint); err != nil {
				return fmt.Errorf("steps[%d].currentStepReceipt: %w", index, err)
			}
		}
	}
	return nil
}

func projectCurrentLoopStopReasonPublicEntrypoint(reason *currentLoopStopReason, entrypoint string) error {
	if reason == nil {
		return nil
	}
	if err := projectMissionCommanderDriverRequestPublicEntrypoint(&reason.CurrentDriverRequest, entrypoint); err != nil {
		return fmt.Errorf("currentDriverRequest: %w", err)
	}
	if err := projectReviewerStepExternalHandoffPublicEntrypoint(reason.ExternalHandoff, entrypoint); err != nil {
		return fmt.Errorf("externalHandoff: %w", err)
	}
	if reason.ExternalMemberHandoff != nil {
		if err := projectCurrentLoopObservationContractPublicEntrypoint(&reason.ExternalMemberHandoff.ObservationContract, entrypoint); err != nil {
			return fmt.Errorf("externalMemberHandoff.observationContract: %w", err)
		}
	}
	return nil
}

func projectCurrentStepPlanPublicEntrypoint(plan *currentStepPlan, entrypoint string) error {
	if plan == nil {
		return nil
	}
	if err := projectMissionCommanderDriverRequestValuePublicEntrypoint(&plan.CurrentDriverRequest, entrypoint); err != nil {
		return fmt.Errorf("currentDriverRequest: %w", err)
	}
	if err := projectDriverStepPlanPublicEntrypoint(plan.DriverStep, entrypoint); err != nil {
		return fmt.Errorf("driverStep: %w", err)
	}
	if err := projectReviewerStepPlanPublicEntrypoint(plan.ReviewerStep, entrypoint); err != nil {
		return fmt.Errorf("reviewerStep: %w", err)
	}
	if err := projectCurrentStepExternalSessionPublicEntrypoint(plan.ExternalSessionStep, entrypoint); err != nil {
		return fmt.Errorf("externalSessionStep: %w", err)
	}
	if err := projectCurrentStepReceiptPublicEntrypoint(plan.Receipt, entrypoint); err != nil {
		return fmt.Errorf("receipt: %w", err)
	}
	if plan.RefreshedStatus != nil {
		if err := projectStatusPublicEntrypoint(plan.RefreshedStatus); err != nil {
			return fmt.Errorf("refreshedStatus: %w", err)
		}
	}
	return nil
}

func projectCurrentStepReceiptPublicEntrypoint(receipt *currentStepReceipt, entrypoint string) error {
	if receipt == nil {
		return nil
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"nestedCommand", &receipt.NestedCommand},
	); err != nil {
		return err
	}
	return projectMissionCommanderDriverRequestPublicEntrypoint(&receipt.RefreshedCurrentDriverRequest, entrypoint)
}

func projectDriverStepPlanPublicEntrypoint(plan *driverStepPlan, entrypoint string) error {
	if plan == nil {
		return nil
	}
	if err := projectMissionCommanderDriverRequestValuePublicEntrypoint(&plan.CurrentDriverRequest, entrypoint); err != nil {
		return fmt.Errorf("currentDriverRequest: %w", err)
	}
	if err := projectMissionCommanderDriverRequestValuePublicEntrypoint(&plan.ApplyDriverRequest, entrypoint); err != nil {
		return fmt.Errorf("applyDriverRequest: %w", err)
	}
	if err := projectMissionCommanderActionQueuePublicEntrypoint(&plan.MissionCommanderActionQueue, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderActionQueue: %w", err)
	}
	if err := projectDriverStepPreviewResultPublicEntrypoint(&plan.PreviewResult, entrypoint); err != nil {
		return fmt.Errorf("previewResult: %w", err)
	}
	if plan.Receipt != nil {
		if err := projectPublicCommandFields(entrypoint,
			projectPublicCommandField{"requestedCommand", &plan.Receipt.RequestedCommand},
			projectPublicCommandField{"executedCommand", &plan.Receipt.ExecutedCommand},
			projectPublicCommandField{"commandResultCommand", &plan.Receipt.CommandResultCommand},
			projectPublicCommandField{"expectedReceiptCommand", &plan.Receipt.ExpectedReceiptCommand},
			projectPublicCommandField{"refreshStatusCommand", &plan.Receipt.RefreshStatusCommand},
		); err != nil {
			return fmt.Errorf("receipt: %w", err)
		}
		if err := projectMissionCommanderDriverRequestPublicEntrypoint(&plan.Receipt.RefreshedCurrentDriverRequest, entrypoint); err != nil {
			return fmt.Errorf("receipt.refreshedCurrentDriverRequest: %w", err)
		}
	}
	if plan.RefreshedStatus != nil {
		if err := projectStatusPublicEntrypoint(plan.RefreshedStatus); err != nil {
			return fmt.Errorf("refreshedStatus: %w", err)
		}
	}
	return nil
}

func projectDriverStepPreviewResultPublicEntrypoint(result *any, entrypoint string) error {
	if result == nil || *result == nil {
		return nil
	}
	var err error
	switch value := (*result).(type) {
	case workstream.StartResult:
		err = projectStartResultForEntrypoint(&value, entrypoint)
		*result = value
	case workstream.ContinueResult:
		err = projectContinueResultForEntrypoint(&value, entrypoint)
		*result = value
	case workstream.ReconcileResult:
		err = projectReconcileResultForEntrypoint(&value, entrypoint)
		*result = value
	case workstream.CompleteResult:
		err = ProjectCompleteResultForPublicEntrypoint(&value, entrypoint)
		*result = value
	}
	return err
}

func projectReviewerStepPreviewResultPublicEntrypoint(result *any, entrypoint string) error {
	if result == nil || *result == nil {
		return nil
	}
	var err error
	switch value := (*result).(type) {
	case subagents.ReviewerPromptArtifactRepairResult:
		err = projectReviewerCommandResultPublicEntrypoint(&value.ApplyCommand, &value.MissionCommanderAction, value.MissionCommanderNextActions, &value.MissionCommanderActionQueue, value.NextSteps, entrypoint)
		*result = value
	case subagents.ReviewerSessionReceiptResult:
		err = projectReviewerCommandResultPublicEntrypoint(&value.ApplyCommand, &value.MissionCommanderAction, value.MissionCommanderNextActions, &value.MissionCommanderActionQueue, value.NextSteps, entrypoint)
		*result = value
	case subagents.ReviewerResultInputSaveResult:
		err = projectReviewerCommandResultWithRunbookPublicEntrypoint(&value.MissionCommanderAction, value.MissionCommanderNextActions, &value.MissionCommanderActionQueue, value.NextSteps, value.RunbookSteps, entrypoint)
		*result = value
	case subagents.ReviewerResultSourceCaptureResult:
		err = projectReviewerCommandResultWithRunbookPublicEntrypoint(&value.MissionCommanderAction, value.MissionCommanderNextActions, &value.MissionCommanderActionQueue, value.NextSteps, value.RunbookSteps, entrypoint)
		*result = value
	case subagents.ReviewerResultStagingResult:
		err = projectReviewerCommandResultWithRunbookPublicEntrypoint(&value.MissionCommanderAction, value.MissionCommanderNextActions, &value.MissionCommanderActionQueue, value.NextSteps, value.RunbookSteps, entrypoint)
		*result = value
	case subagents.ReviewerResultCollectionResult:
		err = projectReviewerCommandResultWithRunbookPublicEntrypoint(&value.MissionCommanderAction, value.MissionCommanderNextActions, &value.MissionCommanderActionQueue, value.NextSteps, value.RunbookSteps, entrypoint)
		*result = value
	case subagents.ReviewerBatchIntakeResult:
		err = projectReviewerBatchIntakeResultPublicEntrypoint(&value, entrypoint)
		*result = value
	}
	return err
}

func projectReviewerCommandResultPublicEntrypoint(
	applyCommand *string,
	action *mission.MissionCommanderAction,
	nextActions []mission.MissionCommanderNextActionItem,
	queue *mission.MissionCommanderActionQueue,
	nextSteps []string,
	entrypoint string,
) error {
	if applyCommand != nil {
		if err := projectPublicCommandFields(entrypoint,
			projectPublicCommandField{"applyCommand", applyCommand},
		); err != nil {
			return err
		}
	}
	if err := projectMissionCommanderActionPublicEntrypoint(action, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderAction: %w", err)
	}
	if err := projectMissionCommanderActionsPublicEntrypoint(nextActions, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderNextActions: %w", err)
	}
	if err := projectMissionCommanderActionQueuePublicEntrypoint(queue, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderActionQueue: %w", err)
	}
	return projectPublicCommandListPublicEntrypoint("nextSteps", nextSteps, entrypoint)
}

func projectReviewerCommandResultWithRunbookPublicEntrypoint(
	action *mission.MissionCommanderAction,
	nextActions []mission.MissionCommanderNextActionItem,
	queue *mission.MissionCommanderActionQueue,
	nextSteps,
	runbookSteps []string,
	entrypoint string,
) error {
	if err := projectReviewerCommandResultPublicEntrypoint(nil, action, nextActions, queue, nextSteps, entrypoint); err != nil {
		return err
	}
	return projectPublicCommandListPublicEntrypoint("runbookSteps", runbookSteps, entrypoint)
}

func projectReviewerBatchIntakeResultPublicEntrypoint(result *subagents.ReviewerBatchIntakeResult, entrypoint string) error {
	if result == nil {
		return nil
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"rerunCommand", &result.RerunCommand},
	); err != nil {
		return err
	}
	if result.RecoveryAction != nil {
		if err := projectMissionCommanderNextActionPublicEntrypoint(result.RecoveryAction, entrypoint); err != nil {
			return fmt.Errorf("recoveryAction: %w", err)
		}
	}
	for index := range result.Results {
		item := &result.Results[index]
		if item.Summary.RepairGuidanceSummary != nil {
			if err := projectPublicCommandFields(entrypoint,
				projectPublicCommandField{"summary.repairGuidanceSummary.nextSafeCommand", &item.Summary.RepairGuidanceSummary.NextSafeCommand},
			); err != nil {
				return fmt.Errorf("results[%d]: %w", index, err)
			}
		}
		if item.Summary.CurrentAction != nil {
			if err := projectPublicCommandFields(entrypoint,
				projectPublicCommandField{"summary.currentAction.command", &item.Summary.CurrentAction.Command},
			); err != nil {
				return fmt.Errorf("results[%d]: %w", index, err)
			}
		}
		if item.PostValidation != nil {
			if err := projectOverviewInventoryPublicEntrypoint(&item.PostValidation.Overview, item.PostValidation.Overview.CaseRoot); err != nil {
				return fmt.Errorf("results[%d].postValidation.overview: %w", index, err)
			}
			if err := projectHandoffResultPublicEntrypoint(&item.PostValidation.Handoff, item.PostValidation.Handoff.CaseRoot); err != nil {
				return fmt.Errorf("results[%d].postValidation.handoff: %w", index, err)
			}
			if item.PostValidation.Summary.CurrentAction != nil {
				if err := projectPublicCommandFields(entrypoint,
					projectPublicCommandField{"postValidation.summary.currentAction.command", &item.PostValidation.Summary.CurrentAction.Command},
				); err != nil {
					return fmt.Errorf("results[%d]: %w", index, err)
				}
			}
			for actionIndex := range item.PostValidation.Summary.NextActions {
				if err := projectPublicCommandFields(entrypoint,
					projectPublicCommandField{"postValidation.summary.nextActions.command", &item.PostValidation.Summary.NextActions[actionIndex].Command},
				); err != nil {
					return fmt.Errorf("results[%d].postValidation.summary.nextActions[%d]: %w", index, actionIndex, err)
				}
			}
		}
		for actionIndex := range item.Summary.NextActions {
			if err := projectPublicCommandFields(entrypoint,
				projectPublicCommandField{"summary.nextActions.command", &item.Summary.NextActions[actionIndex].Command},
			); err != nil {
				return fmt.Errorf("results[%d].summary.nextActions[%d]: %w", index, actionIndex, err)
			}
		}
		if err := projectMissionCommanderActionPublicEntrypoint(&item.MissionCommanderAction, entrypoint); err != nil {
			return fmt.Errorf("results[%d].missionCommanderAction: %w", index, err)
		}
		if err := projectMissionCommanderActionsPublicEntrypoint(item.MissionCommanderNextActions, entrypoint); err != nil {
			return fmt.Errorf("results[%d].missionCommanderNextActions: %w", index, err)
		}
		if err := projectMissionCommanderActionQueuePublicEntrypoint(&item.MissionCommanderActionQueue, entrypoint); err != nil {
			return fmt.Errorf("results[%d].missionCommanderActionQueue: %w", index, err)
		}
		if err := projectPublicCommandListPublicEntrypoint(fmt.Sprintf("results[%d].nextSteps", index), item.NextSteps, entrypoint); err != nil {
			return err
		}
	}
	if err := projectMissionCommanderDriverReceiptPublicEntrypoint(result.MissionCommanderDriverReceipt, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderDriverReceipt: %w", err)
	}
	return projectReviewerCommandResultPublicEntrypoint(nil, &result.MissionCommanderAction, result.MissionCommanderNextActions, &result.MissionCommanderActionQueue, result.NextSteps, entrypoint)
}

func projectReviewerStepPlanPublicEntrypoint(plan *reviewerStepPlan, entrypoint string) error {
	if plan == nil {
		return nil
	}
	if err := projectMissionCommanderDriverRequestValuePublicEntrypoint(&plan.CurrentDriverRequest, entrypoint); err != nil {
		return fmt.Errorf("currentDriverRequest: %w", err)
	}
	if err := projectMissionCommanderDriverRequestPublicEntrypoint(&plan.PreviewDriverRequest, entrypoint); err != nil {
		return fmt.Errorf("previewDriverRequest: %w", err)
	}
	if err := projectMissionCommanderDriverRequestPublicEntrypoint(&plan.ApplyDriverRequest, entrypoint); err != nil {
		return fmt.Errorf("applyDriverRequest: %w", err)
	}
	if err := projectMissionCommanderActionQueuePublicEntrypoint(&plan.MissionCommanderActionQueue, entrypoint); err != nil {
		return fmt.Errorf("missionCommanderActionQueue: %w", err)
	}
	if err := projectReviewerStepPreviewResultPublicEntrypoint(&plan.PreviewResult, entrypoint); err != nil {
		return fmt.Errorf("previewResult: %w", err)
	}
	if err := projectReviewerStepExternalHandoffPublicEntrypoint(plan.ExternalHandoff, entrypoint); err != nil {
		return fmt.Errorf("externalHandoff: %w", err)
	}
	if plan.Receipt != nil {
		if err := projectPublicCommandFields(entrypoint,
			projectPublicCommandField{"executedCommand", &plan.Receipt.ExecutedCommand},
			projectPublicCommandField{"commandResultCommand", &plan.Receipt.CommandResultCommand},
			projectPublicCommandField{"refreshStatusCommand", &plan.Receipt.RefreshStatusCommand},
		); err != nil {
			return fmt.Errorf("receipt: %w", err)
		}
		if err := projectMissionCommanderDriverRequestPublicEntrypoint(&plan.Receipt.RefreshedCurrentDriverRequest, entrypoint); err != nil {
			return fmt.Errorf("receipt.refreshedCurrentDriverRequest: %w", err)
		}
	}
	if plan.RefreshedStatus != nil {
		if err := projectStatusPublicEntrypoint(plan.RefreshedStatus); err != nil {
			return fmt.Errorf("refreshedStatus: %w", err)
		}
	}
	return nil
}

func projectReviewerStepExternalHandoffPublicEntrypoint(handoff *reviewerStepExternalHandoff, entrypoint string) error {
	if handoff == nil {
		return nil
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"recordDispatchPreviewTemplate", &handoff.RecordDispatchPreviewTemplate},
	); err != nil {
		return err
	}
	for index := range handoff.ObservationContract.Alternatives {
		alternative := &handoff.ObservationContract.Alternatives[index]
		if err := projectPublicCommandFields(entrypoint,
			projectPublicCommandField{"previewCommandTemplate", &alternative.PreviewCommandTemplate},
		); err != nil {
			return fmt.Errorf("observationContract.alternatives[%d]: %w", index, err)
		}
	}
	return nil
}

func projectCurrentStepExternalSessionPublicEntrypoint(plan *currentStepExternalSessionPlan, entrypoint string) error {
	if plan == nil {
		return nil
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"refreshStatusCommand", &plan.RefreshStatusCommand},
	); err != nil {
		return err
	}
	if plan.Attempt != nil {
		if err := projectPublicCommandFields(entrypoint,
			projectPublicCommandField{"attempt.applyCommand", &plan.Attempt.ApplyCommand},
		); err != nil {
			return err
		}
	}
	if plan.Dispatch != nil {
		if err := projectPublicCommandFields(entrypoint,
			projectPublicCommandField{"dispatch.applyCommand", &plan.Dispatch.ApplyCommand},
		); err != nil {
			return err
		}
	}
	if err := projectCurrentLoopExternalSessionTurnPublicEntrypoint(plan.Turn, entrypoint); err != nil {
		return fmt.Errorf("turn: %w", err)
	}
	if err := projectCurrentLoopExternalSessionHarnessPackagePublicEntrypoint(plan.HarnessPackage, entrypoint); err != nil {
		return fmt.Errorf("harnessPackage: %w", err)
	}
	if err := projectMissionCommanderDriverRequestPublicEntrypoint(&plan.ReturnRequest, entrypoint); err != nil {
		return fmt.Errorf("returnRequest: %w", err)
	}
	if err := projectMissionCommanderDriverRequestPublicEntrypoint(&plan.ReplacementRequest, entrypoint); err != nil {
		return fmt.Errorf("replacementRequest: %w", err)
	}
	return nil
}

func projectCurrentLoopExternalSessionTurnPublicEntrypoint(plan *currentLoopExternalSessionTurnPlan, entrypoint string) error {
	if plan == nil {
		return nil
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"applyCommand", &plan.ApplyCommand},
		projectPublicCommandField{"relay.applyCommand", &plan.Relay.ApplyCommand},
	); err != nil {
		return err
	}
	if err := projectCurrentLoopPlanForEntrypoint(&plan.Resume, entrypoint); err != nil {
		return fmt.Errorf("resume: %w", err)
	}
	if plan.RefreshedStatus != nil {
		if err := projectStatusPublicEntrypoint(plan.RefreshedStatus); err != nil {
			return fmt.Errorf("refreshedStatus: %w", err)
		}
	}
	return nil
}

func projectCurrentLoopPlanContinuationPublicEntrypoint(continuation *currentLoopContinuationRequest, entrypoint string) error {
	if continuation == nil {
		return nil
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"whatIfCommand", &continuation.WhatIfCommand},
	); err != nil {
		return err
	}
	if continuation.ObservationContract != nil {
		if err := projectCurrentLoopObservationContractPublicEntrypoint(continuation.ObservationContract, entrypoint); err != nil {
			return fmt.Errorf("observationContract: %w", err)
		}
	}
	if continuation.ExternalMemberHandoff != nil {
		if err := projectCurrentLoopObservationContractPublicEntrypoint(&continuation.ExternalMemberHandoff.ObservationContract, entrypoint); err != nil {
			return fmt.Errorf("externalMemberHandoff.observationContract: %w", err)
		}
	}
	return nil
}

func projectCurrentLoopExternalSessionHarnessPackagePublicEntrypoint(pkg *mission.CurrentLoopExternalSessionHarnessPackage, entrypoint string) error {
	if pkg == nil {
		return nil
	}
	if err := projectMissionCommanderDriverRequestPublicEntrypoint(&pkg.AttemptReviewRequest, entrypoint); err != nil {
		return fmt.Errorf("attemptReviewRequest: %w", err)
	}
	if err := projectPublicCommandFields(entrypoint,
		projectPublicCommandField{"refreshStatusCommand", &pkg.RefreshStatusCommand},
	); err != nil {
		return err
	}
	if pkg.Return != nil {
		if err := projectMissionCommanderDriverRequestPublicEntrypoint(&pkg.Return.ReviewRequest, entrypoint); err != nil {
			return fmt.Errorf("return.reviewRequest: %w", err)
		}
		if err := projectMissionCommanderDriverRequestPublicEntrypoint(&pkg.Return.RelayRecoveryRequest, entrypoint); err != nil {
			return fmt.Errorf("return.relayRecoveryRequest: %w", err)
		}
	}
	return nil
}

func projectMissionCommanderDriverRequestValuePublicEntrypoint(request *mission.MissionCommanderDriverRequest, entrypoint string) error {
	if request == nil {
		return nil
	}
	projected, err := mission.MissionCommanderDriverRequestForEntrypoint(*request, entrypoint)
	if err != nil {
		return err
	}
	*request = projected
	return nil
}

func projectReopenResultPublicEntrypoint(result *workstream.ReopenResult, caseRoot string) error {
	if result == nil {
		return nil
	}
	projection, err := resolveProjectPublicProjection(caseRoot)
	if err != nil {
		return fmt.Errorf("resolve reopen public entrypoint: %w", err)
	}
	entrypoint := projection.entrypoint
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
	projection, err := resolveProjectPublicProjection(caseRoot)
	if err != nil {
		return fmt.Errorf("resolve overview public entrypoint: %w", err)
	}
	entrypoint := projection.entrypoint
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
