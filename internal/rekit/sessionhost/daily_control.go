package sessionhost

import (
	"fmt"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

func dailyControlRequested(opt DailyOptions) bool {
	return opt.ControlWhatIf || opt.ControlApply ||
		strings.TrimSpace(opt.Control.Lane) != "" ||
		strings.TrimSpace(opt.Control.Action) != "" ||
		strings.TrimSpace(opt.Control.Reason) != "" ||
		strings.TrimSpace(opt.Control.PublicationStamp) != "" ||
		strings.TrimSpace(opt.Control.ExpectedPlanSHA256) != ""
}

func runDailyControl(opt DailyOptions, result DailyResult) (DailyResult, error) {
	if opt.ControlWhatIf == opt.ControlApply {
		return result, fmt.Errorf("daily control requires exactly one of -WhatIf or -Apply")
	}
	if strings.TrimSpace(opt.DirectoryAdoptionAction) != "" ||
		strings.TrimSpace(opt.ExpectedInitPlanSHA256) != "" ||
		strings.TrimSpace(opt.InitializationRepoRoot) != "" {
		return result, fmt.Errorf("daily control cannot be combined with directory adoption controls")
	}
	if opt.ControlWhatIf && (strings.TrimSpace(opt.Control.PublicationStamp) != "" ||
		strings.TrimSpace(opt.Control.ExpectedPlanSHA256) != "") {
		return result, fmt.Errorf("daily control preview creates its own publication stamp and plan SHA-256")
	}

	target, err := classifyDailyTarget(opt.Target)
	if err != nil {
		return result, err
	}
	result.CaseRoot = target.Root
	if target.Kind != dailyTargetMission {
		return result, fmt.Errorf("daily control requires an existing project with committed mission state")
	}
	stateRoot, err := projectstate.Resolve(target.Root)
	if err != nil {
		return result, err
	}
	if stateRoot.Existing && !stateRoot.Legacy && stateRoot.Dir == projectstate.CurrentDir {
		recovery, recoveryErr := syncreview.InspectCurrentSyncRecovery(target.Root)
		if recoveryErr != nil {
			return result, fmt.Errorf("inspect current project update recovery: %w", recoveryErr)
		}
		if recovery.Pending {
			result.Pack = recovery.Pack
			result.FinalState = "maintenance-recovery-required"
			result.Blocked = true
			result.CurrentSyncRecovery = &recovery
			result.Action = dailyCurrentSyncRecoveryAction(recovery)
			return result, nil
		}
	}

	inspection, err := missionintent.Inspect(target.Root)
	if err != nil {
		return result, fmt.Errorf("inspect daily mission intent for control: %w", err)
	}
	if !inspection.Committed {
		return result, fmt.Errorf("daily control requires committed mission intent")
	}
	pack, err := dailyPack(target.Root, inspection)
	if err != nil {
		return result, err
	}
	result.Pack = pack

	selected := strings.TrimSpace(opt.SelectedLane)
	controlLane := strings.TrimSpace(opt.Control.Lane)
	if selected != "" && controlLane != "" && selected != controlLane {
		return result, fmt.Errorf("daily control lane differs from the selected lane")
	}
	if selected == "" {
		selected = controlLane
	}
	selected, action, err := dailyControlSelectedLane(target.Root, selected)
	if err != nil {
		return result, err
	}
	if action != nil {
		result.Blocked = true
		result.Action = action
		return result, nil
	}
	result.Lane = selected

	control := opt.Control
	control.Lane = selected
	dailyActor := strings.TrimSpace(opt.Actor)
	if dailyActor == "" {
		dailyActor = defaultDailyActor
	}
	if strings.TrimSpace(control.Actor) == "" {
		control.Actor = dailyActor
	} else if strings.TrimSpace(opt.Actor) != "" && strings.TrimSpace(control.Actor) != dailyActor {
		return result, fmt.Errorf("daily control actor differs from the daily actor")
	}

	var plan executioncontrol.Plan
	if opt.ControlWhatIf {
		plan, err = executioncontrol.Preview(target.Root, control)
	} else {
		plan, err = executioncontrol.Apply(target.Root, control)
	}
	if err != nil {
		return result, err
	}
	result.ExecutionControl = &plan
	result.Replay = plan.AlreadyApplied
	result.Boundary = mission.UniqueStrings(append(result.Boundary,
		"daily control returns before Claude discovery or session launch",
		"control records no authority, confirmed state, or heavy-tool authorization",
		"resume affects only future work and never launches, progresses, or releases an earlier result automatically",
	))
	if opt.ControlWhatIf {
		result.FinalState = DailyActionConfirmationRequired
		result.Blocked = true
		result.Action = dailyAction(DailyActionConfirmationRequired)
		return result, nil
	}

	switch plan.Action {
	case executioncontrol.ActionPause:
		result.FinalState = DailyActionPaused
		result.Blocked = true
	case executioncontrol.ActionResume:
		result.FinalState = DailyActionResumed
	case executioncontrol.ActionStop:
		result.FinalState = DailyActionStopped
		result.Blocked = true
	default:
		return result, fmt.Errorf("daily control returned unsupported action %q", plan.Action)
	}
	result.Action = dailyAction(result.FinalState)
	return result, nil
}

func dailyControlSelectedLane(caseRoot, selected string) (string, *DailyUserAction, error) {
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return "", nil, err
	}
	choices := make([]DailyChoice, 0, len(board.Lanes))
	for _, lane := range board.Lanes {
		if !strings.EqualFold(strings.TrimSpace(lane.Status), "open") {
			continue
		}
		choices = append(choices, DailyChoice{ID: lane.ID, Label: mission.BoardLaneLabel(lane)})
	}
	selected = strings.TrimSpace(selected)
	if selected == "" {
		switch len(choices) {
		case 0:
			return "", dailyAction(DailyActionBlocked), nil
		case 1:
			return choices[0].ID, nil, nil
		default:
			return "", dailyControlLaneSelectionAction(choices), nil
		}
	}
	lane, ok := mission.LookupBoardLane(board.Lanes, selected, false)
	if !ok {
		return "", nil, fmt.Errorf("selected daily control lane %q is not current", selected)
	}
	if !strings.EqualFold(strings.TrimSpace(lane.Status), "open") {
		return "", dailySelectedLaneBlockedAction(DailyChoice{ID: lane.ID, Label: mission.BoardLaneLabel(lane)}), nil
	}
	return selected, nil, nil
}
