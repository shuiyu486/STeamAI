package cli

import (
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
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
	status.MemberExecution = &memberExecutionStatus{Ready: true, State: inspection.State, Lane: lane, AttemptID: inspection.AttemptID, Inspection: &inspection, PreviewCommand: memberStatusPreviewCommand(status.Target, status.Pack), Boundary: base}
	if inspection.State == "handoff-ready" || inspection.State == "accepted" {
		status.MemberExecution.ObservationCommand = memberStatusObservationCommand(status.Target, status.Pack, inspection.AttemptID)
	}
	if inspection.State == "intake-ready" && inspection.Manifest != nil {
		manifestRef, _ := filepath.Rel(status.Target, inspection.ManifestPath)
		status.MemberExecution.CompletionEvidence = []string{filepath.ToSlash(manifestRef)}
		if inspection.Manifest.ReviewerItemsPath != "" {
			items := filepath.Join(inspection.OutputsRoot, filepath.FromSlash(inspection.Manifest.ReviewerItemsPath))
			status.MemberExecution.ReviewerPlanCommand = joinDriverCommand([]string{"/rekit", "plan-subagents", "-Target", status.Target, "-Pack", status.Pack, "-TaskType", "feature-analysis", "-ItemsFile", items, "-Lane", lane, "-Format", "json"})
		}
	}
}

func memberStatusPreviewCommand(target, pack string) string {
	return joinDriverCommand([]string{"/rekit", "run-current-step", "-Target", target, "-Pack", pack, "-WhatIf", "-Format", "json"})
}

func memberStatusObservationCommand(target, pack, attempt string) string {
	return joinDriverCommand([]string{"/rekit", "run-current-step", "-Target", target, "-Pack", pack, "-MemberExecutionAttemptId", attempt, "-MemberExecutionOutcome", "<accepted|returned|failed>", "-MemberExecutionObservedAt", "<RFC3339Nano>", "-Actor", "<harness>", "-WhatIf", "-Format", "json"})
}
