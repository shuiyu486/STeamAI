package workstream

import (
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

// DailyMissionControlRunbook is a read-only handoff for the main Agent or an
// external harness. It summarizes the current Mission Commander driver request
// and the safe refresh/handoff cadence without executing any command.
type DailyMissionControlRunbook struct {
	Ready                       bool                                   `json:"ready"`
	Scope                       string                                 `json:"scope"`
	CurrentState                string                                 `json:"currentState,omitempty"`
	CurrentSource               string                                 `json:"currentSource,omitempty"`
	CurrentCommand              string                                 `json:"currentCommand,omitempty"`
	CurrentRunLoopStepID        string                                 `json:"currentRunLoopStepId,omitempty"`
	CurrentDriverRequest        *mission.MissionCommanderDriverRequest `json:"currentDriverRequest,omitempty"`
	RefreshStatusCommand        string                                 `json:"refreshStatusCommand"`
	HandoffPreviewCommand       string                                 `json:"handoffPreviewCommand,omitempty"`
	HandoffApplyCommand         string                                 `json:"handoffApplyCommand,omitempty"`
	HandoffPreviewDriverRequest *mission.MissionCommanderDriverRequest `json:"handoffPreviewDriverRequest,omitempty"`
	HandoffApplyDriverRequest   *mission.MissionCommanderDriverRequest `json:"handoffApplyDriverRequest,omitempty"`
	RunLoop                     []DailyMissionControlRunbookStep       `json:"runLoop"`
	Boundary                    []string                               `json:"boundary,omitempty"`
}

type DailyMissionControlRunbookStep struct {
	StepID            string   `json:"stepId"`
	Order             int      `json:"order"`
	Actor             string   `json:"actor"`
	State             string   `json:"state,omitempty"`
	Source            string   `json:"source,omitempty"`
	DriverKind        string   `json:"driverKind,omitempty"`
	Command           string   `json:"command,omitempty"`
	Guidance          string   `json:"guidance,omitempty"`
	CommandExecutable bool     `json:"commandExecutable"`
	Blocked           bool     `json:"blocked"`
	RequiresReview    bool     `json:"requiresReview"`
	Boundary          []string `json:"boundary,omitempty"`
}

func DailyMissionControlRunbookFor(caseRoot, scope string, queue mission.MissionCommanderActionQueue, handoffPreviewCommand, handoffApplyCommand string) *DailyMissionControlRunbook {
	return DailyMissionControlRunbookForWithHandoffApplyReady(caseRoot, scope, queue, handoffPreviewCommand, handoffApplyCommand, false)
}

func DailyMissionControlRunbookForWithHandoffApplyReady(caseRoot, scope string, queue mission.MissionCommanderActionQueue, handoffPreviewCommand, handoffApplyCommand string, handoffApplyReady bool) *DailyMissionControlRunbook {
	refreshCommand := dailyMissionControlStatusCommand(caseRoot)
	scope = strings.TrimSpace(scope)
	if scope == "" {
		scope = "case"
	}
	runbook := &DailyMissionControlRunbook{
		Ready:                 queue.CurrentAction != nil,
		Scope:                 scope,
		CurrentRunLoopStepID:  strings.TrimSpace(queue.CurrentRunLoopStepID),
		RefreshStatusCommand:  refreshCommand,
		HandoffPreviewCommand: strings.TrimSpace(handoffPreviewCommand),
		HandoffApplyCommand:   strings.TrimSpace(handoffApplyCommand),
		Boundary: mission.UniqueStrings([]string{
			"daily Mission Control runbook is read-only; it does not execute /rekit commands or spawn sessions",
			"only execute currentDriverRequest.command when commandExecutable=true",
			"guidance-only driver requests require main-agent selection/review and must not be run as shell commands",
			"after any preview/apply/continue/reconcile result, refresh status before choosing follow-up work",
			"handoff apply writes case-local handoff/resume/checkpoint files only; it does not write authority/confirmed or execute heavy tools",
		}),
	}
	if queue.CurrentAction != nil {
		runbook.CurrentState = strings.TrimSpace(queue.CurrentAction.State)
		runbook.CurrentSource = strings.TrimSpace(queue.CurrentAction.Source)
		runbook.CurrentCommand = strings.TrimSpace(queue.CurrentAction.Command)
	}
	if queue.CurrentDriverRequest != nil {
		request := mission.MissionCommanderDriverRequestWithRefreshStatusCommand(*queue.CurrentDriverRequest, refreshCommand)
		runbook.CurrentDriverRequest = &request
	}
	runbook.RunLoop = dailyMissionControlRunLoop(runbook)
	runbook.HandoffPreviewDriverRequest = dailyMissionControlHandoffDriverRequest(runbook, "preview-handoff", runbook.HandoffPreviewCommand, false, true)
	runbook.HandoffApplyDriverRequest = dailyMissionControlHandoffDriverRequest(runbook, "write-handoff-for-takeover", runbook.HandoffApplyCommand, true, handoffApplyReady)
	return runbook
}

func dailyMissionControlHandoffDriverRequest(runbook *DailyMissionControlRunbook, stepID, command string, apply, executable bool) *mission.MissionCommanderDriverRequest {
	if runbook == nil || strings.TrimSpace(command) == "" {
		return nil
	}
	action := mission.MissionCommanderNextActionItem{
		Label:          firstNonEmpty(runbook.Scope, "case"),
		State:          "handoff-preview-available",
		Command:        strings.TrimSpace(command),
		Source:         "dailyMissionControlRunbook.handoffPreview",
		RequiresReview: true,
		Reasons: []string{
			"daily Mission Control handoff request is typed; do not reconstruct handoff commands from run-loop prose",
		},
		Boundary: []string{
			"handoff preview is read-only and should use -WhatIf -Format json",
			"handoff requests do not execute reviewer, adapter, heavy-tool, authority, or confirmed actions",
		},
	}
	if apply {
		action.State = "handoff-apply-available"
		action.Source = "dailyMissionControlRunbook.handoffApply"
		action.Boundary = append(action.Boundary,
			"handoff apply writes case-local handoff/resume/checkpoint files only",
			"run handoff apply only after reviewing the current status and handoff preview",
		)
	}
	if !executable {
		action.Command = "review handoff preview before running " + strings.TrimSpace(command)
		action.Boundary = append(action.Boundary, "this handoff apply request is review guidance until a handoff preview/apply result marks it executable")
	}
	request := mission.MissionCommanderCurrentDriverRequest(action, stepID, dailyMissionControlMissionRunLoop(runbook.RunLoop))
	if request == nil {
		return nil
	}
	refreshed := mission.MissionCommanderDriverRequestWithRefreshStatusCommand(*request, runbook.RefreshStatusCommand)
	if !executable {
		refreshed.ExpectedReceipt.Command = ""
	}
	refreshed.Boundary = mission.UniqueStrings(append(refreshed.Boundary, action.Boundary...))
	return &refreshed
}

func dailyMissionControlMissionRunLoop(steps []DailyMissionControlRunbookStep) []mission.MissionCommanderRunLoopStep {
	out := make([]mission.MissionCommanderRunLoopStep, 0, len(steps))
	for _, step := range steps {
		out = append(out, mission.MissionCommanderRunLoopStep{
			StepID:      step.StepID,
			Order:       step.Order,
			Actor:       step.Actor,
			Description: firstNonEmpty(step.State, step.Source, step.StepID),
			Command:     step.Command,
			State:       step.State,
			Source:      step.Source,
			Boundary:    append([]string{}, step.Boundary...),
		})
	}
	return out
}

func dailyMissionControlStatusCommand(caseRoot string) string {
	caseRoot = strings.TrimSpace(caseRoot)
	if caseRoot == "" {
		return "/rekit status -Format json"
	}
	return "/rekit status -Target " + quoteCommandArgAlways(caseRoot) + " -Format json"
}

func quoteCommandArgAlways(arg string) string {
	return `"` + strings.ReplaceAll(arg, `"`, `\"`) + `"`
}

func dailyMissionControlRunLoop(runbook *DailyMissionControlRunbook) []DailyMissionControlRunbookStep {
	steps := []DailyMissionControlRunbookStep{
		{
			StepID:            "inspect-status",
			Order:             1,
			Actor:             "main-agent",
			State:             firstNonEmpty(runbook.CurrentState, "inspect-current-state"),
			Source:            "dailyMissionControlRunbook.status",
			Command:           runbook.RefreshStatusCommand,
			CommandExecutable: true,
			Boundary: []string{
				"read caseMission.missionCommanderActionQueue.currentDriverRequest before taking action",
				"status is read-only and does not mutate lane, board, facts, authority, confirmed, or heavy-tool state",
			},
		},
	}
	request := runbook.CurrentDriverRequest
	if request == nil {
		steps = append(steps, DailyMissionControlRunbookStep{
			StepID:   "select-or-start-lane",
			Order:    2,
			Actor:    "main-agent",
			State:    "no-current-driver-request",
			Source:   "dailyMissionControlRunbook.noCurrentRequest",
			Guidance: "choose /rekit start <name> -WhatIf or rerun status after initializing the case board",
			Boundary: []string{
				"do not infer completion from an absent current driver request",
				"use -WhatIf before any case-local mutation",
			},
		})
	} else {
		steps = append(steps, DailyMissionControlRunbookStep{
			StepID:            "consume-current-driver-request",
			Order:             2,
			Actor:             firstNonEmpty(request.Actor, "main-agent"),
			State:             request.State,
			Source:            request.Source,
			DriverKind:        request.Kind,
			Command:           request.Command,
			Guidance:          request.Guidance,
			CommandExecutable: request.CommandExecutable,
			Blocked:           request.Blocked,
			RequiresReview:    request.RequiresReview,
			Boundary: mission.UniqueStrings(append([]string{
				"consume exactly the current driver request; do not reconstruct commands from terminal prose",
				"when requiresReview=true, review the WhatIf/receipt before Apply",
			}, request.Boundary...)),
		})
	}
	steps = append(steps, DailyMissionControlRunbookStep{
		StepID:            "refresh-after-driver",
		Order:             3,
		Actor:             "main-agent",
		State:             firstNonEmpty(runbook.CurrentState, "refresh-required"),
		Source:            "dailyMissionControlRunbook.refresh",
		Command:           runbook.RefreshStatusCommand,
		CommandExecutable: true,
		Boundary: []string{
			"refresh durable status after each preview/apply/continue/reconcile command result",
			"do not choose a follow-up from stale currentDriverRequest data",
		},
	})
	if runbook.HandoffPreviewCommand != "" {
		steps = append(steps, DailyMissionControlRunbookStep{
			StepID:            "preview-handoff",
			Order:             4,
			Actor:             "main-agent",
			State:             "handoff-preview-available",
			Source:            "dailyMissionControlRunbook.handoffPreview",
			Command:           runbook.HandoffPreviewCommand,
			CommandExecutable: true,
			Boundary: []string{
				"preview handoff before writing durable handoff artifacts when the next session must take over",
				"handoff preview is read-only",
			},
		})
	}
	if runbook.HandoffApplyCommand != "" {
		steps = append(steps, DailyMissionControlRunbookStep{
			StepID:            "write-handoff-for-takeover",
			Order:             5,
			Actor:             "main-agent",
			State:             "handoff-apply-available",
			Source:            "dailyMissionControlRunbook.handoffApply",
			Command:           runbook.HandoffApplyCommand,
			CommandExecutable: true,
			Boundary: []string{
				"write durable handoff only after reviewing the current status/driver request",
				"handoff apply does not execute reviewer, adapter, heavy-tool, authority, or confirmed actions",
			},
		})
	}
	return steps
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
