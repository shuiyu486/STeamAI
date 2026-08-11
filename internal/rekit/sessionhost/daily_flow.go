package sessionhost

import (
	"context"
	"strings"
)

var dailyReviewerRunLoopSteps = map[string]struct{}{
	"verify-prompt":     {},
	"spawn-reviewer":    {},
	"save-result-input": {},
	"record-completion": {},
	"source-capture":    {},
	"stage-candidate":   {},
	"collect-result":    {},
	"intake-results":    {},
}

func dailyReviewerOwnerRequest(status publicStatus, selected string) bool {
	if status.MissionControlRunbook == nil ||
		status.MissionControlRunbook.Scope != "reviewer" {
		return false
	}
	request := status.MissionControlRunbook.CurrentDriverRequest
	selected = strings.TrimSpace(selected)
	if request == nil || request.Blocked || selected == "" ||
		strings.TrimSpace(request.Source) != "reviewerDispatchOperatorPackage" ||
		strings.TrimSpace(request.Lane) != selected {
		return false
	}
	step := strings.TrimSpace(request.RunLoopStepID)
	if _, ok := dailyReviewerRunLoopSteps[step]; !ok {
		return false
	}
	if step == "spawn-reviewer" {
		return request.Kind == "review-guidance" &&
			strings.TrimSpace(request.Actor) == "main-agent-harness" &&
			!request.CommandExecutable &&
			strings.TrimSpace(request.Command) == "" &&
			strings.TrimSpace(request.Guidance) != ""
	}
	return request.Kind == "preview-command" &&
		strings.TrimSpace(request.Actor) == "main-agent" &&
		request.CommandExecutable &&
		strings.TrimSpace(request.Command) != "" &&
		strings.TrimSpace(request.Guidance) == ""
}

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
	command, err := publicCommandName(request.Command)
	return err == nil && command == "complete"
}

func runDailyGoalFlow(
	parent context.Context,
	hostOpt Options,
	caseRoot,
	pack string,
	result *DailyResult,
) error {
	hostOpt.StopAfterMemberIntake = true
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
	reviewer, err := Run(parent, hostOpt)
	addDailyHostRun(result, reviewer)
	if err != nil {
		return err
	}
	result.FinalState = reviewer.FinalMode
	return finishDailyCompletion(caseRoot, pack, result)
}
