package workstream

import (
	"fmt"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

type ContinueOwnerGuardRecovery struct {
	Ready                         bool                                    `json:"ready"`
	Reason                        string                                  `json:"reason,omitempty"`
	Lane                          string                                  `json:"lane,omitempty"`
	Label                         string                                  `json:"label,omitempty"`
	ReceivedExecutor              string                                  `json:"receivedExecutor,omitempty"`
	ReceivedExecutorGeneration    int                                     `json:"receivedExecutorGeneration,omitempty"`
	CurrentExecutor               string                                  `json:"currentExecutor,omitempty"`
	CurrentExecutorGeneration     int                                     `json:"currentExecutorGeneration,omitempty"`
	CurrentContinueCommand        string                                  `json:"currentContinueCommand,omitempty"`
	HandoffCommand                string                                  `json:"handoffCommand,omitempty"`
	StartTakeoverPreviewCommand   string                                  `json:"startTakeoverPreviewCommand,omitempty"`
	StartTakeoverApplyCommand     string                                  `json:"startTakeoverApplyCommand,omitempty"`
	ResumePath                    string                                  `json:"resumePath,omitempty"`
	CheckpointPath                string                                  `json:"checkpointPath,omitempty"`
	HandoffPath                   string                                  `json:"handoffPath,omitempty"`
	LaneTakeoverPackage           *LaneTakeoverPackage                    `json:"laneTakeoverPackage,omitempty"`
	MissionCommanderCurrentAction *mission.MissionCommanderNextActionItem `json:"missionCommanderCurrentAction,omitempty"`
	MissionCommanderActionQueue   mission.MissionCommanderActionQueue     `json:"missionCommanderActionQueue"`
	RunbookSteps                  []string                                `json:"runbookSteps,omitempty"`
	Boundary                      []string                                `json:"boundary,omitempty"`
}

func continueOwnerGuardRecoveryFor(caseRoot string, lane Lane, action laneExecutorAction, queue mission.MissionCommanderActionQueue, opt ContinueOptions, reason string) (*ContinueOwnerGuardRecovery, error) {
	pkg, err := laneTakeoverPackageFor(caseRoot, lane, action, queue, false)
	if err != nil {
		return nil, err
	}
	label := workstreamLabel(lane)
	previewCommand := "/rekit start " + quoteCommandArg(label) + " -WhatIf -Executor <new-executor> -Actor <main-agent> -Reason " + quoteCommandArg("replace stale executor after owner guard mismatch")
	applyCommand := "/rekit start " + quoteCommandArg(label) + " -Apply -Executor <new-executor> -Actor <main-agent> -Reason " + quoteCommandArg("replace stale executor after owner guard mismatch")
	recovery := &ContinueOwnerGuardRecovery{
		Ready:                         true,
		Reason:                        strings.TrimSpace(reason),
		Lane:                          lane.ID,
		Label:                         label,
		ReceivedExecutor:              strings.TrimSpace(opt.Executor),
		ReceivedExecutorGeneration:    opt.ExpectedExecutorGeneration,
		CurrentExecutor:               lane.CurrentExecutor,
		CurrentExecutorGeneration:     lane.ExecutorGeneration,
		CurrentContinueCommand:        pkg.ContinueCommand,
		HandoffCommand:                pkg.HandoffCommand,
		StartTakeoverPreviewCommand:   previewCommand,
		StartTakeoverApplyCommand:     applyCommand,
		ResumePath:                    pkg.ResumePath,
		CheckpointPath:                pkg.CheckpointPath,
		HandoffPath:                   pkg.HandoffPath,
		LaneTakeoverPackage:           pkg,
		MissionCommanderCurrentAction: cloneMissionCommanderCurrentAction(queue.CurrentAction),
		MissionCommanderActionQueue:   queue,
	}
	recovery.RunbookSteps = continueOwnerGuardRecoveryRunbookSteps(recovery)
	recovery.Boundary = continueOwnerGuardRecoveryBoundary()
	return recovery, nil
}

func continueOwnerGuardRecoveryRunbookSteps(recovery *ContinueOwnerGuardRecovery) []string {
	if recovery == nil {
		return nil
	}
	steps := []string{
		fmt.Sprintf("do not retry the stale continue command with executor %s generation %d", firstText(recovery.ReceivedExecutor, "unassigned"), recovery.ReceivedExecutorGeneration),
		"read " + recovery.HandoffPath + " or " + recovery.ResumePath + " before choosing takeover",
	}
	if strings.TrimSpace(recovery.CurrentContinueCommand) != "" {
		steps = append(steps, "if you are the current executor, rerun owner-bound continue: "+strings.TrimSpace(recovery.CurrentContinueCommand))
	}
	steps = append(steps,
		"if replacing the executor, run the takeover preview before apply: "+strings.TrimSpace(recovery.StartTakeoverPreviewCommand),
		"only after reviewing the preview, run the explicit takeover apply command with a real executor id",
	)
	return mission.UniqueStrings(steps)
}

func continueOwnerGuardRecoveryBoundary() []string {
	return []string{
		"owner guard mismatch is fail-closed and zero-write",
		"do not bypass -Executor or -ExpectedExecutorGeneration",
		"recovery package is read-only guidance; it does not claim a new executor",
		"start takeover remains explicit -Apply and must use a real executor id",
		"no authority/confirmed writes or heavy-tool execution",
	}
}
