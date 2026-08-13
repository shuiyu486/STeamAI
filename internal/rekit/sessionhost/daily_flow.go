package sessionhost

import (
	"context"
	"fmt"
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
	hostOpt.StopAfterMemberIntake = true
	if _, err := bindHostCurrentDriverRequest(&hostOpt); err != nil {
		return fmt.Errorf("bind daily member current driver request: %w", err)
	}
	member, err := Run(parent, hostOpt)
	addDailyHostRun(result, member)
	if err != nil {
		return err
	}
	result.FinalState = member.FinalMode
	if member.FinalMode == "reviewer-rejected-awaiting-correction" {
		result.Blocked = true
		return nil
	}
	if member.FinalMode != "reviewer-ready" {
		return finishDailyCompletion(caseRoot, pack, result)
	}

	status, err := runPublicStatus(caseRoot, pack, result.Lane)
	if err != nil {
		return err
	}
	if !dailyReviewerOwnerRequest(status, result.Lane) {
		result.Blocked = true
		result.FinalState = "attention-required"
		return nil
	}
	hostOpt.StopAfterMemberIntake = false
	if _, err := bindHostCurrentDriverRequest(&hostOpt); err != nil {
		return fmt.Errorf("bind daily Reviewer current driver request: %w", err)
	}
	reviewer, err := Run(parent, hostOpt)
	addDailyHostRun(result, reviewer)
	if err != nil {
		return err
	}
	result.FinalState = reviewer.FinalMode
	return finishDailyCompletion(caseRoot, pack, result)
}
