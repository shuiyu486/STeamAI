package cli

import (
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestBindStatusReviewerCorrectionPreservesOpenReconcileAction(t *testing.T) {
	reconcile := mission.MissionCommanderNextActionItem{
		Lane:           "feature-mission",
		Label:          "Feature Mission",
		ActionID:       "reconcile:daily-correction-current",
		State:          "needs-reconcile",
		Command:        `/rekit reconcile "Feature Mission" -InterventionEventId daily-correction-current -WhatIf -Format json`,
		Source:         "missionCommanderActions",
		RequiresReview: true,
	}
	queue := mission.MissionCommanderActionQueueFor([]mission.MissionCommanderNextActionItem{reconcile})
	status := reviewerRejectedStatusForTest(queue)

	bindStatusReviewerCorrection(&status)

	current := status.CaseMission.MissionCommanderActionQueue.CurrentAction
	if current == nil || current.ActionID != reconcile.ActionID || !strings.Contains(current.Command, "/rekit reconcile") {
		t.Fatalf("current action = %+v", current)
	}
	request := status.MissionControlRunbook.CurrentDriverRequest
	if request == nil || !strings.Contains(request.Command, "/rekit reconcile") {
		t.Fatalf("current driver request = %+v", request)
	}
	if !strings.Contains(strings.Join(status.MemberExecution.Boundary, "\n"), "already recorded") {
		t.Fatalf("member boundary = %v", status.MemberExecution.Boundary)
	}
}

func TestBindStatusReviewerCorrectionProjectsCorrectionBeforeIntervention(t *testing.T) {
	status := reviewerRejectedStatusForTest(mission.MissionCommanderActionQueue{})

	bindStatusReviewerCorrection(&status)

	current := status.CaseMission.MissionCommanderActionQueue.CurrentAction
	if current == nil || current.State != "reviewer-rejected-awaiting-correction" || !strings.HasPrefix(current.Command, "rekit-host -daily") {
		t.Fatalf("current action = %+v", current)
	}
}

func reviewerRejectedStatusForTest(queue mission.MissionCommanderActionQueue) statusInventory {
	caseMission := &statusCaseMission{
		MissionCommanderActionQueue: queue,
		DailyMissionControlRunbook: workstream.DailyMissionControlRunbookFor(
			`C:\case`,
			"case",
			queue,
			"",
			"",
		),
	}
	return statusInventory{
		Target:      `C:\case`,
		CaseMission: caseMission,
		MemberExecution: &memberExecutionStatus{
			State:             "reviewer-rejected-awaiting-correction",
			Lane:              "feature-mission",
			CorrectionCommand: `rekit-host -daily -target "C:\case" -correction "<human-correction>" -actor "<actor>"`,
			ReviewerRejection: &workstream.MemberReviewerRejection{
				PacketID:            "packet-1",
				DecisionEventID:     "decision-1",
				VerificationEventID: "verification-1",
				ReviewerSession:     "reviewer-session-1",
				OwnerGeneration:     1,
				Summary:             "required acceptance condition is missing",
			},
		},
		MissionControlRunbook: buildStatusMissionControlRunbook(`C:\case`, caseMission, nil),
	}
}
