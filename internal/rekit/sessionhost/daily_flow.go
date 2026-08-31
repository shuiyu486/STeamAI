package sessionhost

import (
	"context"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func dailyCompletionOwnerRequest(status publicStatus, selected string) bool {
	if status.MissionControlRunbook == nil ||
		status.MissionControlRunbook.Scope != "case" {
		return false
	}
	request := status.MissionControlRunbook.CurrentDriverRequest
	selected = strings.TrimSpace(selected)
	if request == nil || request.Blocked || selected == "" ||
		request.Kind != "preview-command" ||
		strings.TrimSpace(request.RunLoopStepID) != "preview-current" ||
		strings.TrimSpace(request.Actor) != "main-agent" ||
		strings.TrimSpace(request.Source) != "laneCompletion.acceptedReviewerLineage" ||
		strings.TrimSpace(request.Lane) != selected ||
		!request.CommandExecutable ||
		strings.TrimSpace(request.Command) == "" ||
		strings.TrimSpace(request.Guidance) != "" {
		return false
	}
	return mission.ValidateMissionCommanderDriverRequest(*request) == nil && request.Invocation.Command == commands.Complete
}

func runDailyGoalFlow(
	parent context.Context,
	hostOpt Options,
	caseRoot,
	pack string,
	result *DailyResult,
) error {
	owner, err := newDailySessionTransitionOwner(caseRoot, pack, result.Lane, hostOpt.projectExecutionLease)
	if err != nil {
		return err
	}
	hostOpt.StopAfterMemberIntake = true
	if err := owner.runHostSegment(parent, hostOpt, result); err != nil {
		return err
	}
	if result.FinalState == "reviewer-rejected-awaiting-correction" {
		result.Blocked = true
		return nil
	}
	if result.FinalState != "reviewer-ready" {
		return owner.finish(result)
	}

	status, err := runPublicStatusWithLease(caseRoot, pack, result.Lane, hostOpt.projectExecutionLease)
	if err != nil {
		return err
	}
	if !dailyReviewerOwnerRequest(status, result.Lane) {
		result.Blocked = true
		result.FinalState = "attention-required"
		return nil
	}
	hostOpt.StopAfterMemberIntake = false
	if err := owner.runHostSegment(parent, hostOpt, result); err != nil {
		return err
	}
	return owner.finish(result)
}
