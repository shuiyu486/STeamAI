package cli

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
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
	InputReadiness         *memberInputReadinessStatus         `json:"inputReadiness,omitempty"`
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

type memberInputReadinessStatus struct {
	State        string `json:"state"`
	Mode         string `json:"mode,omitempty"`
	ArtifactPath string `json:"artifactPath,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

func bindStatusMemberExecution(status *statusInventory) {
	if status == nil || status.MissionControlRunbook == nil {
		bindStatusSingleLaneInputRequired(status)
		return
	}
	if status.MissionControlRunbook.Scope != "case" {
		return
	}
	if status.MissionControlRunbook.CurrentDriverRequest == nil {
		bindStatusSingleLaneInputRequired(status)
		return
	}
	request := status.MissionControlRunbook.CurrentDriverRequest
	memberContinuation := mission.MissionCommanderDriverRequestOwnsMemberContinuation(*request)
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
		if !memberContinuation {
			return
		}
		status.MemberExecution = &memberExecutionStatus{Ready: true, State: "dispatch-preview-required", Lane: lane, PreviewCommand: memberStatusPreviewCommand(status.Target, status.Pack), Boundary: base}
		bindStatusMemberInputReadiness(status, lane)
		return
	}
	ownerCurrent, err := memberexecution.CurrentOwnerMatches(status.Target, status.Pack, inspection.Owner)
	if err != nil {
		status.MemberExecution = &memberExecutionStatus{State: "corrupt", Lane: lane, Boundary: []string{err.Error(), "status remains read-only and does not repair member execution state"}}
		return
	}
	if !ownerCurrent {
		if !memberContinuation {
			return
		}
		status.MemberExecution = &memberExecutionStatus{Ready: true, State: "dispatch-preview-required", Lane: lane, PreviewCommand: memberStatusPreviewCommand(status.Target, status.Pack), Boundary: append(base, "historical member results remain auditable but cannot drive the current owner generation")}
		bindStatusMemberInputReadiness(status, lane)
		return
	}
	status.MemberExecution = &memberExecutionStatus{Ready: true, State: inspection.State, Lane: lane, AttemptID: inspection.AttemptID, Inspection: &inspection, PreviewCommand: memberStatusPreviewCommand(status.Target, status.Pack), Boundary: base}
	if memberContinuation {
		bindStatusMemberInputReadiness(status, lane)
		if status.MemberExecution.State == "input-stale" {
			return
		}
	}
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

func bindStatusSingleLaneInputRequired(status *statusInventory) {
	if status == nil || status.Mode != "case" ||
		!strings.EqualFold(strings.TrimSpace(status.Pack), defaults.DefaultPack) {
		return
	}
	board, err := mission.ReadBoard(status.Target)
	if err != nil {
		return
	}
	lane := ""
	for _, candidate := range board.Lanes {
		state := strings.ToLower(strings.TrimSpace(candidate.Status))
		if candidate.Authority || state == "closed" || state == "archived" ||
			strings.TrimSpace(candidate.CurrentExecutor) == "" || candidate.ExecutorGeneration < 1 {
			continue
		}
		if lane != "" {
			return
		}
		lane = strings.TrimSpace(candidate.ID)
	}
	if lane == "" {
		return
	}
	binding, err := memberexecution.CurrentTaskBinding(status.Target, lane)
	if err != nil || binding != nil {
		return
	}
	status.MemberExecution = &memberExecutionStatus{
		State:          "input-required",
		Lane:           lane,
		InputReadiness: &memberInputReadinessStatus{State: "input-required"},
		Boundary: []string{
			"binary-re requires an explicit artifact-analysis or workspace-inventory binding before member dispatch",
			"status is read-only and does not infer an artifact or rewrite the Mission Control action queue",
		},
	}
}

func bindStatusMemberInputReadiness(status *statusInventory, lane string) {
	if status == nil || status.MemberExecution == nil || !strings.EqualFold(strings.TrimSpace(status.Pack), defaults.DefaultPack) {
		return
	}
	binding, err := memberexecution.CurrentTaskBinding(status.Target, lane)
	if err != nil {
		status.MemberExecution.Ready = false
		status.MemberExecution.State = "corrupt"
		status.MemberExecution.PreviewCommand = ""
		status.MemberExecution.Boundary = mission.UniqueStrings(append(
			status.MemberExecution.Boundary,
			"typed member input binding is unreadable: "+err.Error(),
			"status remains read-only and does not infer or repair typed input",
		))
		return
	}
	if binding == nil {
		status.MemberExecution.Ready = false
		status.MemberExecution.State = "input-required"
		status.MemberExecution.PreviewCommand = ""
		status.MemberExecution.InputReadiness = &memberInputReadinessStatus{State: "input-required"}
		status.MemberExecution.Boundary = mission.UniqueStrings(append(
			status.MemberExecution.Boundary,
			"binary-re requires an explicit artifact-analysis or workspace-inventory binding before member dispatch",
			"status does not guess an artifact from natural language, filenames, or directory contents",
		))
		return
	}
	if !memberexecution.IsTaskInputBinding(*binding) {
		return
	}
	readiness := &memberInputReadinessStatus{State: "ready", Mode: binding.Kind}
	if binding.Kind == memberexecution.TaskBindingArtifactAnalysis {
		readiness.ArtifactPath = binding.Values["artifact-path"]
	} else {
		readiness.Scope = binding.Values["workspace-scope"]
	}
	if err := memberexecution.ValidateTaskInputBinding(status.Target, *binding); err != nil {
		readiness.State = "stale"
		status.MemberExecution.Ready = false
		status.MemberExecution.State = "input-stale"
		status.MemberExecution.PreviewCommand = ""
		status.MemberExecution.Boundary = mission.UniqueStrings(append(
			status.MemberExecution.Boundary,
			"typed member input is not current: "+err.Error(),
			"refresh or replace the owner generation before member dispatch",
		))
	}
	status.MemberExecution.InputReadiness = readiness
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
