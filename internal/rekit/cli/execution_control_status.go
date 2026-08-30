package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func bindStatusExecutionControls(status *statusInventory) error {
	if status == nil || status.Mode != "case" || status.CaseMission == nil {
		return nil
	}
	board, err := mission.ReadBoard(status.Target)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect lane execution controls: %w", err)
	}

	selected := strings.TrimSpace(status.selectedCurrentLane)
	controls := make([]statusExecutionControl, 0, len(board.Lanes))
	blocked := map[string]statusExecutionControl{}
	for _, lane := range board.Lanes {
		laneID := strings.TrimSpace(lane.ID)
		laneState := strings.ToLower(strings.TrimSpace(lane.Status))
		if laneID == "" || laneState == "closed" || laneState == "archived" || (selected != "" && laneID != selected) {
			continue
		}
		control := inspectStatusExecutionControl(status.Target, laneID)
		controls = append(controls, control)
		if control.Blocked {
			blocked[laneID] = control
		}
	}
	status.ExecutionControls = controls
	if len(blocked) == 0 {
		return nil
	}

	caseMission := *status.CaseMission
	caseMission.LaneExecutorActions = statusExecutionControlledLaneActions(caseMission.LaneExecutorActions, blocked)
	caseMission.MissionCommanderNextActions = statusExecutionControlledActions(caseMission.MissionCommanderNextActions, blocked)
	caseMission.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(caseMission.MissionCommanderNextActions)
	caseMission.ReviewerDispatchIntakeHandoffs = append([]workstream.ReviewerDispatchIntakeHandoff{}, caseMission.ReviewerDispatchIntakeHandoffs...)
	for lane, control := range blocked {
		workstream.PauseReviewerDispatchesForLane(caseMission.ReviewerDispatchIntakeHandoffs, lane, control.Reason)
	}
	caseMission.ReviewerDispatchIntakeSummary = workstream.ReviewerDispatchIntakeSummaryFor(caseMission.ReviewerDispatchIntakeHandoffs)
	reviewerActions := workstream.MissionCommanderNextActionsWithReviewerDispatches(nil, caseMission.ReviewerDispatchIntakeHandoffs)
	caseMission.ReviewerDispatchIntakeActionQueue = mission.MissionCommanderActionQueueFor(reviewerActions)
	caseMission.DailyMissionControlRunbook = workstream.DailyMissionControlRunbookFor(
		status.Target,
		"case",
		caseMission.MissionCommanderActionQueue,
		caseMission.HandoffPreviewCommand,
		caseMission.HandoffApplyCommand,
	)
	caseMission.FirstScreenLaneTakeoverPackage = statusFirstScreenLaneTakeoverPackage(
		status.Target,
		caseMission.LaneExecutorActions,
		caseMission.MissionCommanderActionQueue,
	)
	if current := caseMission.MissionCommanderActionQueue.CurrentAction; current != nil {
		if control, ok := blocked[strings.TrimSpace(current.Lane)]; ok && control.Blocked {
			caseMission.FirstScreenLaneTakeoverPackage = nil
		}
	}
	caseMission.Ready = caseMission.MissionCommanderActionQueue.Counts.Unblocked > 0 &&
		caseMission.MissionCommanderActionQueue.Counts.Blocked == 0 && len(caseMission.Escalations) == 0
	statusExecutionControlLaneCounts(&caseMission)
	status.CaseMission = &caseMission

	projectHandoff := status.ProjectHandoff
	consumption := status.PackMemoryConsumption
	if selected != "" {
		projectHandoff = nil
		consumption = nil
	}
	status.MissionControlRunbook = buildStatusMissionControlRunbookWithConsumption(
		status.Target,
		status.CaseMission,
		projectHandoff,
		consumption,
	)
	bindStatusExecutionControlRunbook(status)
	bindStatusExecutionControlMember(status)
	return nil
}

func inspectStatusExecutionControl(caseRoot, lane string) statusExecutionControl {
	control := statusExecutionControl{
		Lane:  lane,
		State: executioncontrol.StateRunning,
		Boundary: []string{
			"status reads the durable control head without creating, repairing, or applying control artifacts",
			"pause and stop block future launch, live publication, and progression without deleting raw execution truth",
			"resume changes only durable control state and never launches or accepts prior work automatically",
		},
	}
	inspection, err := executioncontrol.Inspect(caseRoot, lane)
	if err != nil {
		control.State = "corrupt"
		control.Blocked = true
		control.Reason = "lane execution control is corrupt; no lane action is executable: " + err.Error()
		return control
	}
	control.State = inspection.State
	control.CurrentGeneration = inspection.CurrentGeneration
	control.CurrentReceiptSHA256 = inspection.CurrentReceiptSHA256
	control.Pending = inspection.Pending
	control.PendingGeneration = inspection.PendingGeneration
	if inspection.PendingIntent != nil {
		control.PendingAction = inspection.PendingIntent.Action
	}
	if inspection.Pending {
		control.Blocked = true
		control.Reason = fmt.Sprintf("lane %s has a pending durable control publication; recover only the exact original Apply", lane)
		command, commandErr := executioncontrol.PendingApplyCommand(caseRoot, inspection)
		if commandErr != nil {
			control.State = "corrupt"
			control.Reason = "pending lane execution control cannot be recovered safely: " + commandErr.Error()
			return control
		}
		control.RecoveryCommand = command
		return control
	}
	switch inspection.State {
	case executioncontrol.StateRunning:
		return control
	case executioncontrol.StatePaused:
		control.Blocked = true
		control.Reason = fmt.Sprintf("lane %s is durably paused; no new launch, live result publication, or progression is allowed until a separate reviewed resume commits", lane)
	case executioncontrol.StateStopped:
		control.Blocked = true
		control.Reason = fmt.Sprintf("lane %s is durably stopped; no launch, live result publication, progression, or implicit resume is allowed", lane)
	default:
		control.State = "corrupt"
		control.Blocked = true
		control.Reason = fmt.Sprintf("lane %s has unknown durable control state %q", lane, inspection.State)
	}
	return control
}

func statusExecutionControlledActions(items []mission.MissionCommanderNextActionItem, blocked map[string]statusExecutionControl) []mission.MissionCommanderNextActionItem {
	out := make([]mission.MissionCommanderNextActionItem, 0, len(items)+len(blocked))
	seen := map[string]bool{}
	for _, item := range items {
		control, ok := blocked[strings.TrimSpace(item.Lane)]
		if !ok {
			out = append(out, item)
			continue
		}
		if !seen[control.Lane] {
			out = append(out, statusExecutionControlAction(control, item.Label))
			seen[control.Lane] = true
		}
	}
	lanes := make([]string, 0, len(blocked))
	for lane := range blocked {
		lanes = append(lanes, lane)
	}
	sort.Strings(lanes)
	for _, lane := range lanes {
		if !seen[lane] {
			out = append(out, statusExecutionControlAction(blocked[lane], lane))
		}
	}
	return mission.UniqueCommanderNextActions(out)
}

func statusExecutionControlAction(control statusExecutionControl, label string) mission.MissionCommanderNextActionItem {
	state := "execution-control-" + control.State
	if control.Pending {
		state = "execution-control-recovery-required"
	}
	guidance := control.Reason
	if control.RecoveryCommand != "" {
		guidance = control.RecoveryCommand
	}
	return mission.MissionCommanderNextActionItem{
		Lane:           control.Lane,
		Label:          strings.TrimSpace(label),
		ActionID:       "lane-execution-control:" + control.Lane,
		State:          state,
		Command:        guidance,
		Source:         "executionControl",
		Blocked:        true,
		RequiresReview: true,
		Reasons:        []string{control.Reason},
		Boundary: []string{
			"this is a diagnostic control handoff and cannot launch a member or Reviewer session",
			"pending control recovery must use only the exact original Apply command",
			"resume never releases an earlier held result or continues the lane automatically",
		},
	}
}

func statusExecutionControlledLaneActions(items []mission.LaneExecutorActionSnapshot, blocked map[string]statusExecutionControl) []mission.LaneExecutorActionSnapshot {
	out := append([]mission.LaneExecutorActionSnapshot{}, items...)
	for idx := range out {
		control, ok := blocked[strings.TrimSpace(out[idx].Lane)]
		if !ok {
			continue
		}
		action := &out[idx].ExecutorAction
		action.Blocked = true
		action.Ready = false
		action.BlockerReasons = mission.UniqueStrings(append(action.BlockerReasons, "execution-control-"+control.State))
		action.ResumeCommand = ""
		action.NextAgentActions = []string{control.Reason}
		action.MissionCommanderAction = mission.MissionCommanderAction{
			State:    "execution-control-" + control.State,
			Prompt:   control.Reason,
			Boundary: []string{"durable execution control blocks launch and progression without changing authority or confirmed state"},
		}
	}
	return out
}

func statusExecutionControlLaneCounts(caseMission *statusCaseMission) {
	if caseMission == nil {
		return
	}
	ready := []string{}
	blocked := []string{}
	for _, item := range caseMission.LaneExecutorActions {
		if strings.EqualFold(strings.TrimSpace(item.Status), "closed") {
			continue
		}
		label := strings.TrimSpace(item.Label)
		if label == "" {
			label = strings.TrimSpace(item.Lane)
		}
		if item.ExecutorAction.Blocked {
			reasons := mission.UniqueStrings(item.ExecutorAction.BlockerReasons)
			if len(reasons) > 0 {
				label += " (" + strings.Join(reasons, ",") + ")"
			}
			blocked = append(blocked, label)
		} else if item.ExecutorAction.Ready {
			ready = append(ready, label)
		}
	}
	caseMission.ReadyLanes = ready
	caseMission.BlockedLanes = blocked
	caseMission.ReadyLaneCount = len(ready)
	caseMission.BlockedLaneCount = len(blocked)
}

func bindStatusExecutionControlRunbook(status *statusInventory) {
	if status == nil || status.CaseMission == nil || status.MissionControlRunbook == nil {
		return
	}
	runbook := status.MissionControlRunbook
	request := status.CaseMission.MissionCommanderActionQueue.CurrentDriverRequest
	if request == nil {
		return
	}
	control, ok := statusExecutionControlForLane(status, request.Lane)
	if !ok || !control.Blocked {
		return
	}
	projected := statusMissionControlInvocationDriverRequest(status.Target, *request)
	projected = mission.MissionCommanderDriverRequestWithRefreshStatusCommand(projected, runbook.RefreshStatusCommand)
	runbook.Ready = false
	runbook.Scope = "case"
	runbook.CurrentCommand = ""
	runbook.CurrentRunLoopStepID = projected.RunLoopStepID
	runbook.CurrentDriverRequest = &projected
	runbook.CurrentDriverRequestSHA256, _ = mission.MissionCommanderDriverRequestSHA256(projected)
	runbook.CurrentDriverReceipt = statusMissionControlCurrentDriverReceipt(runbook)
	runbook.CurrentLoopSegment = nil
	runbook.CurrentLoopOperator = nil
	runbook.ReplacementExecutorTakeover = nil
	runbook.RoutingReasons = mission.UniqueStrings(append(runbook.RoutingReasons, control.Reason))
	runbook.Boundary = mission.UniqueStrings(append(runbook.Boundary,
		"durable execution control suppresses current-loop, replacement-executor, member, and Reviewer execution packages",
		"raw historical execution truth remains diagnostic and is not promoted into live outputs or progression",
	))
	runbook.GuidanceHandoff = statusMissionControlGuidanceHandoffFor(runbook, status.ProjectHandoff)
	runbook.RunLoop = statusMissionControlRunbookSteps(runbook)
	runbook.Quickstart = statusMissionControlQuickstartFor(runbook, nil)
}

func bindStatusExecutionControlMember(status *statusInventory) {
	if status == nil || status.MemberExecution == nil {
		return
	}
	control, ok := statusExecutionControlForLane(status, status.MemberExecution.Lane)
	if !ok || !control.Blocked {
		return
	}
	status.MemberExecution.Ready = false
	status.MemberExecution.State = "execution-control-" + control.State
	status.MemberExecution.PreviewCommand = ""
	status.MemberExecution.ObservationCommand = ""
	status.MemberExecution.ReviewerPlanCommand = ""
	status.MemberExecution.ReviewerPlanInvocation = nil
	status.MemberExecution.CorrectionCommand = ""
	status.MemberExecution.Boundary = mission.UniqueStrings(append(status.MemberExecution.Boundary,
		control.Reason,
		"member inspection remains raw diagnostic truth; no result is published or progressed while control is blocked",
	))
}

func statusExecutionControlForLane(status *statusInventory, lane string) (statusExecutionControl, bool) {
	lane = strings.TrimSpace(lane)
	if status == nil || lane == "" {
		return statusExecutionControl{}, false
	}
	for _, control := range status.ExecutionControls {
		if strings.TrimSpace(control.Lane) == lane {
			return control, true
		}
	}
	return statusExecutionControl{}, false
}
