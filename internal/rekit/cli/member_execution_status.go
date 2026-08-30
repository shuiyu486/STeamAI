package cli

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

type memberExecutionStatus struct {
	Ready                  bool                                `json:"ready"`
	State                  string                              `json:"state"`
	Lane                   string                              `json:"lane,omitempty"`
	AttemptID              string                              `json:"attemptId,omitempty"`
	Inspection             *memberexecution.Inspection         `json:"inspection,omitempty"`
	ReviewerRejection      *workstream.MemberReviewerRejection `json:"reviewerRejection,omitempty"`
	PreviewCommand         string                              `json:"previewCommand,omitempty"`
	ObservationCommand     string                              `json:"observationCommand,omitempty"`
	ReviewerPlanCommand    string                              `json:"reviewerPlanCommand,omitempty"`
	ReviewerPlanInvocation *commands.PublicInvocation          `json:"reviewerPlanInvocation,omitempty"`
	CorrectionCommand      string                              `json:"correctionCommand,omitempty"`
	CompletionEvidence     []string                            `json:"completionEvidenceRefs,omitempty"`
	Boundary               []string                            `json:"boundary"`
}

func bindStatusMemberExecution(status *statusInventory) {
	if status == nil || status.MissionControlRunbook == nil || status.MissionControlRunbook.Scope != "case" || status.MissionControlRunbook.CurrentDriverRequest == nil {
		return
	}
	request := status.MissionControlRunbook.CurrentDriverRequest
	lane := strings.TrimSpace(request.Lane)
	if lane == "" {
		return
	}
	inspection, ok, err := memberexecution.Latest(status.Target, lane)
	if err != nil {
		status.MemberExecution = &memberExecutionStatus{State: "corrupt", Lane: lane, Boundary: []string{err.Error(), "status remains read-only and does not repair member execution state"}}
		return
	}
	base := []string{"status is read-only and does not spawn, poll, or stop member sessions", "observations must use run-current-step or run-current-loop with exact hash-bound Apply", "member intake does not write authority/confirmed state or execute heavy tools"}
	if !ok {
		status.MemberExecution = &memberExecutionStatus{Ready: true, State: "dispatch-preview-required", Lane: lane, PreviewCommand: memberStatusPreviewCommand(status.Target, status.Pack), Boundary: base}
		return
	}
	ownerCurrent, err := memberexecution.CurrentOwnerMatches(status.Target, status.Pack, inspection.Owner)
	if err != nil {
		status.MemberExecution = &memberExecutionStatus{State: "corrupt", Lane: lane, Boundary: []string{err.Error(), "status remains read-only and does not repair member execution state"}}
		return
	}
	if !ownerCurrent {
		status.MemberExecution = &memberExecutionStatus{Ready: true, State: "dispatch-preview-required", Lane: lane, PreviewCommand: memberStatusPreviewCommand(status.Target, status.Pack), Boundary: append(base, "historical member results remain auditable but cannot drive the current owner generation")}
		return
	}
	status.MemberExecution = &memberExecutionStatus{Ready: true, State: inspection.State, Lane: lane, AttemptID: inspection.AttemptID, Inspection: &inspection, PreviewCommand: memberStatusPreviewCommand(status.Target, status.Pack), Boundary: base}
	if inspection.State == "handoff-ready" || inspection.State == "accepted" {
		controlRequired, rootErr := currentMemberExecutionControlRequired(status.Target)
		if rootErr != nil {
			status.MemberExecution.Ready = false
			status.MemberExecution.State = "corrupt"
			status.MemberExecution.PreviewCommand = ""
			status.MemberExecution.Boundary = []string{rootErr.Error(), "status remains read-only and does not repair member execution state"}
			return
		}
		if controlRequired {
			if inspection.Handoff == nil || inspection.Handoff.LaunchControl == nil {
				status.MemberExecution.Ready = false
				status.MemberExecution.State = "diagnostic-missing-execution-control"
				status.MemberExecution.PreviewCommand = ""
				status.MemberExecution.Boundary = mission.UniqueStrings(append(base,
					"historical current member handoff omitted immutable execution control lineage; it remains diagnostic-only and cannot accept observations or launch a session",
				))
				return
			}
			currentness, err := executioncontrol.InspectBindingReadOnly(status.Target, *inspection.Handoff.LaunchControl)
			if err != nil || !currentness.Current {
				status.MemberExecution.Ready = false
				status.MemberExecution.State = "diagnostic-stale-execution-control"
				status.MemberExecution.PreviewCommand = ""
				reason := "member handoff execution control lineage is unreadable"
				if err != nil {
					reason += ": " + err.Error()
				} else {
					reason = "member handoff execution control lineage is not current: " + currentness.Disposition + ": " + currentness.Reason
				}
				status.MemberExecution.Boundary = mission.UniqueStrings(append(base,
					reason,
					"historical member handoff remains diagnostic-only and cannot accept observations or launch a session",
				))
				return
			}
		}
		status.MemberExecution.ObservationCommand = memberStatusObservationCommand(status.Target, status.Pack, inspection.AttemptID)
	}
	if inspection.State == "intake-ready" && inspection.Manifest != nil {
		manifestRef, err := filepath.Rel(status.Target, inspection.ManifestPath)
		if err != nil {
			status.MemberExecution.Ready = false
			status.MemberExecution.State = "corrupt"
			status.MemberExecution.Boundary = []string{err.Error(), "status remains read-only and does not repair member execution state"}
			return
		}
		manifestRef = filepath.ToSlash(manifestRef)
		status.MemberExecution.CompletionEvidence = []string{manifestRef}
		rejection, rejected, err := workstream.CurrentMemberManifestReviewerRejection(status.Target, lane, manifestRef)
		if err != nil {
			status.MemberExecution.Ready = false
			status.MemberExecution.State = "corrupt"
			status.MemberExecution.Boundary = []string{err.Error(), "status remains read-only and does not repair reviewer rejection lineage"}
			return
		}
		if rejected {
			status.MemberExecution.State = "reviewer-rejected-awaiting-correction"
			status.MemberExecution.ReviewerRejection = &rejection
			status.MemberExecution.CorrectionCommand = joinDriverCommand([]string{"rekit-host", "-daily", "-target", status.Target, "-lane", lane, "-correction", "<human-correction>", "-actor", "<actor>"})
			status.MemberExecution.Boundary = append(base, "the rejected member manifest cannot be reviewed again; apply a human correction bound to this canonical rejection before replacing the owner generation")
			return
		}
		reviewed, err := workstream.HasAcceptedMemberManifestReviewerWriteback(status.Target, lane, manifestRef)
		if err != nil {
			status.MemberExecution.Ready = false
			status.MemberExecution.State = "corrupt"
			status.MemberExecution.Boundary = []string{err.Error(), "status remains read-only and does not repair reviewer lineage"}
			return
		}
		if reviewed {
			status.MemberExecution.Boundary = append(base, "the current member manifest already has a strictly validated accepted reviewer lineage")
			return
		}
		args := []string{"-Target", status.Target, "-Pack", status.Pack, "-TaskType", "feature-analysis"}
		args = append(args, memberReviewerItemsArgs(manifestRef)...)
		args = append(args, "-Lane", lane, "-Format", "json")
		invocation, err := commands.NewPublicInvocation(commands.PlanSubagents, args...)
		if err != nil {
			status.MemberExecution.Ready = false
			status.MemberExecution.State = "corrupt"
			status.MemberExecution.Boundary = []string{err.Error(), "status remains read-only and does not repair reviewer planning state"}
			return
		}
		status.MemberExecution.ReviewerPlanInvocation = &invocation
		status.MemberExecution.ReviewerPlanCommand, err = invocation.Render()
		if err != nil {
			status.MemberExecution.ReviewerPlanInvocation = nil
			status.MemberExecution.Ready = false
			status.MemberExecution.State = "corrupt"
			status.MemberExecution.Boundary = []string{err.Error(), "status remains read-only and does not repair reviewer planning state"}
		}
	}
}

func currentMemberExecutionControlRequired(caseRoot string) (bool, error) {
	root, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return false, err
	}
	return root.Existing && !root.Legacy, nil
}

func bindStatusReviewerCorrection(status *statusInventory) {
	if status == nil || status.MemberExecution == nil || status.MemberExecution.State != "reviewer-rejected-awaiting-correction" || status.MemberExecution.ReviewerRejection == nil || status.MissionControlRunbook == nil {
		return
	}
	if statusReviewerCorrectionReconcilePending(status, status.MemberExecution.Lane) {
		status.MemberExecution.Boundary = mission.UniqueStrings(append(status.MemberExecution.Boundary, "the evidence-bound correction is already recorded; reconcile the open intervention before replacement dispatch"))
		status.MissionControlRunbook.RoutingReasons = mission.UniqueStrings(append(status.MissionControlRunbook.RoutingReasons, "the open correction intervention takes precedence over the historical reviewer rejection correction entry point"))
		return
	}
	rejection := status.MemberExecution.ReviewerRejection
	action := mission.MissionCommanderNextActionItem{
		Lane: status.MemberExecution.Lane, Label: rejection.PacketID, ActionID: "reviewer-rejection-correction:" + rejection.DecisionEventID,
		State: "reviewer-rejected-awaiting-correction", Command: status.MemberExecution.CorrectionCommand, Source: "memberExecution.reviewerRejection", RequiresReview: true,
		Reasons:  []string{rejection.Summary, "verificationEventId=" + rejection.VerificationEventID, "decisionEventId=" + rejection.DecisionEventID, "reviewerSession=" + rejection.ReviewerSession, "ownerGeneration=" + strconv.Itoa(rejection.OwnerGeneration)},
		Boundary: []string{"replace <human-correction> and <actor> explicitly; do not replay the rejected manifest", "the daily correction command revalidates packet/result/input/receipt/event bindings before reconciliation", "replacement generation and a new reviewer packet/session are required before completion"},
	}
	if status.CaseMission != nil {
		queue := mission.MissionCommanderActionQueueFor([]mission.MissionCommanderNextActionItem{action})
		status.CaseMission.Summary = "canonical reviewer rejection requires evidence-bound human correction and replacement review"
		status.CaseMission.MissionCommanderNextActions = []mission.MissionCommanderNextActionItem{action}
		status.CaseMission.MissionCommanderActionQueue = queue
		status.CaseMission.DailyMissionControlRunbook = workstream.DailyMissionControlRunbookFor(status.Target, "case", queue, status.CaseMission.HandoffPreviewCommand, status.CaseMission.HandoffApplyCommand)
	}
	projectHandoff := status.ProjectHandoff
	packMemoryConsumption := status.PackMemoryConsumption
	if strings.TrimSpace(status.selectedCurrentLane) != "" {
		projectHandoff = nil
		packMemoryConsumption = nil
	}
	status.MissionControlRunbook = buildStatusMissionControlRunbookWithConsumption(status.Target, status.CaseMission, projectHandoff, packMemoryConsumption)
	status.MissionControlRunbook.RoutingReasons = mission.UniqueStrings(append(status.MissionControlRunbook.RoutingReasons, "canonical reviewer reject requires evidence-bound human correction before any member or reviewer redispatch"))
}

func statusReviewerCorrectionReconcilePending(status *statusInventory, lane string) bool {
	if status == nil || status.CaseMission == nil {
		return false
	}
	queue := status.CaseMission.MissionCommanderActionQueue
	if queue.CurrentAction == nil ||
		!strings.EqualFold(strings.TrimSpace(queue.CurrentAction.Lane), strings.TrimSpace(lane)) ||
		queue.CurrentDriverRequest == nil ||
		queue.CurrentDriverRequest.Blocked ||
		!queue.CurrentDriverRequest.CommandExecutable {
		return false
	}
	command, err := SplitPublicCommand(queue.CurrentDriverRequest.Command)
	return err == nil && len(command) >= 2 &&
		strings.EqualFold(command[0], "-Command") &&
		strings.EqualFold(command[1], "reconcile")
}

func memberReviewerItemsArgs(manifestRef string) []string {
	return []string{"-Items", manifestRef}
}

func memberStatusPreviewCommand(target, pack string) string {
	return joinDriverCommand([]string{"/rekit", "run-current-step", "-Target", target, "-Pack", pack, "-WhatIf", "-Format", "json"})
}

func memberStatusObservationCommand(target, pack, attempt string) string {
	return joinDriverCommand([]string{"/rekit", "run-current-step", "-Target", target, "-Pack", pack, "-MemberExecutionAttemptId", attempt, "-MemberExecutionOutcome", "<accepted|returned|failed>", "-MemberExecutionObservedAt", "<RFC3339Nano>", "-Actor", "<harness>", "-WhatIf", "-Format", "json"})
}
