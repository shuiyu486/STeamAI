package cli

import (
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

type memberExecutionStatus struct {
	Ready               bool                        `json:"ready"`
	State               string                      `json:"state"`
	Lane                string                      `json:"lane,omitempty"`
	AttemptID           string                      `json:"attemptId,omitempty"`
	Inspection          *memberexecution.Inspection `json:"inspection,omitempty"`
	PreviewCommand      string                      `json:"previewCommand,omitempty"`
	ObservationCommand  string                      `json:"observationCommand,omitempty"`
	ReviewerPlanCommand string                      `json:"reviewerPlanCommand,omitempty"`
	CompletionEvidence  []string                    `json:"completionEvidenceRefs,omitempty"`
	Boundary            []string                    `json:"boundary"`
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
		args := []string{"/rekit", "plan-subagents", "-Target", status.Target, "-Pack", status.Pack, "-TaskType", "feature-analysis"}
		args = append(args, memberReviewerItemsArgs(manifestRef)...)
		args = append(args, "-Lane", lane, "-Format", "json")
		status.MemberExecution.ReviewerPlanCommand = joinDriverCommand(args)
	}
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
