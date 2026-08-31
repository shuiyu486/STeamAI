package sessionhost

import (
	"context"
	"fmt"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/sessiontransition"
)

// dailySessionTransitionOwner owns the bounded transition from a fresh typed
// daily request to one host segment or one public driver step. It does not
// own durable state, choose a lane, or infer a next action from prose.
type dailyStatusExecutor func(caseRoot, pack, lane string) (publicStatus, error)
type dailySessionExecutor func(context.Context, Options) (Result, error)
type dailyCompletionPublisher func(caseRoot, pack, lane string) (publicDriverResult, error)

type dailySessionTransitionOwner struct {
	caseRoot            string
	pack                string
	lane                string
	statusExecutor      dailyStatusExecutor
	sessionExecutor     dailySessionExecutor
	completionPublisher dailyCompletionPublisher
}

func newDailySessionTransitionOwner(caseRoot, pack, lane string, held ...*projectexecution.Lease) (dailySessionTransitionOwner, error) {
	caseRoot = strings.TrimSpace(caseRoot)
	pack = strings.TrimSpace(pack)
	lane = strings.TrimSpace(lane)
	if caseRoot == "" || pack == "" {
		return dailySessionTransitionOwner{}, fmt.Errorf("daily session transition requires an exact case root and pack")
	}
	var lease *projectexecution.Lease
	if len(held) > 0 {
		lease = held[0]
	}
	return dailySessionTransitionOwner{
		caseRoot: caseRoot,
		pack:     pack,
		lane:     lane,
		statusExecutor: func(caseRoot, pack, lane string) (publicStatus, error) {
			return runPublicStatusWithLease(caseRoot, pack, lane, lease)
		},
		sessionExecutor: Run,
		completionPublisher: func(caseRoot, pack, lane string) (publicDriverResult, error) {
			return runPublicDriverStepWithLease(caseRoot, pack, lease, lane)
		},
	}, nil
}

func (owner dailySessionTransitionOwner) readStatus() (publicStatus, error) {
	if owner.statusExecutor == nil {
		return publicStatus{}, fmt.Errorf("daily status executor is unavailable")
	}
	return owner.statusExecutor(owner.caseRoot, owner.pack, owner.lane)
}

func (owner dailySessionTransitionOwner) bindHostCurrentDriverRequest(opt *Options) error {
	if opt == nil {
		return fmt.Errorf("daily session transition host options are missing")
	}
	if strings.TrimSpace(opt.Target) != owner.caseRoot || strings.TrimSpace(opt.Pack) != owner.pack {
		return fmt.Errorf("daily session transition host target or pack changed")
	}
	if strings.TrimSpace(opt.SelectedLane) != owner.lane {
		return fmt.Errorf("daily session transition selected lane changed")
	}
	if owner.statusExecutor == nil {
		return fmt.Errorf("daily status executor is unavailable")
	}
	status, err := owner.statusExecutor(owner.caseRoot, owner.pack, owner.lane)
	if err != nil {
		return err
	}
	if _, err := bindHostCurrentDriverRequestFromStatus(opt, status); err != nil {
		return err
	}
	return nil
}

func (owner dailySessionTransitionOwner) runHostSegment(
	parent context.Context,
	opt Options,
	result *DailyResult,
) error {
	if result == nil {
		return fmt.Errorf("daily session transition result is missing")
	}
	if owner.lane == "" {
		return fmt.Errorf("daily session transition host segment requires a selected lane")
	}
	if err := owner.bindHostCurrentDriverRequest(&opt); err != nil {
		return err
	}
	if owner.sessionExecutor == nil {
		return fmt.Errorf("daily session executor is unavailable")
	}
	hostResult, err := owner.sessionExecutor(parent, opt)
	addDailyHostRun(result, hostResult)
	if err != nil {
		return err
	}
	result.FinalState = hostResult.FinalMode
	return nil
}

func (owner dailySessionTransitionOwner) runPublicDriverStep(result *DailyResult) (publicDriverResult, error) {
	if owner.lane == "" {
		return publicDriverResult{}, fmt.Errorf("daily session transition driver step requires a selected lane")
	}
	if owner.completionPublisher == nil {
		return publicDriverResult{}, fmt.Errorf("daily completion publisher is unavailable")
	}
	step, err := owner.completionPublisher(owner.caseRoot, owner.pack, owner.lane)
	if err != nil {
		return publicDriverResult{}, err
	}
	if result != nil {
		if step.ResultCommand != "" {
			result.DriverSteps = append(result.DriverSteps, step.ResultCommand)
		}
	}
	return step, nil
}

func (owner dailySessionTransitionOwner) bindResultCurrentDriverRequest(result *DailyResult) {
	if result == nil {
		return
	}
	status, err := owner.readStatus()
	if err != nil || status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentDriverRequest == nil {
		return
	}
	request := *status.MissionControlRunbook.CurrentDriverRequest
	if err := mission.ValidateMissionCommanderDriverRequest(request); err != nil {
		return
	}
	identity, err := mission.MissionCommanderDriverRequestSHA256(request)
	if err != nil || !strings.EqualFold(identity, status.MissionControlRunbook.CurrentDriverRequestSHA256) {
		return
	}
	result.CurrentDriverRequest = &request
	result.CurrentDriverRequestSHA256 = identity
}

func (owner dailySessionTransitionOwner) finish(result *DailyResult) error {
	if result == nil {
		return fmt.Errorf("daily session transition completion result is missing")
	}
	initial, err := sessiontransition.ReduceCompletion(sessiontransition.CompletionInput{
		Stage:        sessiontransition.CompletionStageInitial,
		CurrentState: result.FinalState,
	})
	if err != nil {
		return err
	}
	switch initial.Mode {
	case sessiontransition.CompletionModePreserve:
		return nil
	case sessiontransition.CompletionModeAwaitCorrection:
		result.Blocked = true
		return nil
	case sessiontransition.CompletionModeInspectStatus:
		// Continue with the fresh status observation below.
	default:
		return fmt.Errorf("unexpected initial daily completion effect: %s", initial.Mode)
	}

	status, err := owner.readStatus()
	if err != nil {
		return err
	}
	input := sessiontransition.CompletionInput{
		Stage:           sessiontransition.CompletionStageStatus,
		SessionLaunches: result.SessionLaunches,
	}
	if status.CaseMission != nil && status.CaseMission.MissionCompletion != nil {
		input.MissionReady = status.CaseMission.MissionCompletion.Ready
		input.MissionOperationallyComplete = status.CaseMission.MissionCompletion.OperationallyComplete
	}
	input.CurrentRequest = currentDailyRequest(status)
	decision, err := sessiontransition.ReduceCompletion(input)
	if err != nil {
		return err
	}
	switch decision.Mode {
	case sessiontransition.CompletionModeMissionComplete:
		result.FinalState = "mission-complete"
		result.Replay = decision.Replay
		return nil
	case sessiontransition.CompletionModeAttentionRequired:
		result.Blocked = true
		result.FinalState = "attention-required"
		return nil
	case sessiontransition.CompletionModeExecuteCompletion:
		// The existing public driver coordinator remains the sole publisher.
	default:
		return fmt.Errorf("unexpected status daily completion effect: %s", decision.Mode)
	}

	step, err := owner.runPublicDriverStep(result)
	if err != nil {
		return fmt.Errorf("daily public completion: %w", err)
	}
	executed, err := sessiontransition.ReduceCompletion(sessiontransition.CompletionInput{
		Stage: sessiontransition.CompletionStageExecuted,
		Execution: sessiontransition.CompletionExecution{
			ResultCommand:      step.ResultCommand,
			CompletionPresent:  step.Completion != nil,
			Blocked:            step.Completion != nil && step.Completion.Blocked,
			Applied:            step.Completion != nil && step.Completion.Applied,
			LaneID:             completionLaneID(step),
			LaneStatus:         completionLaneStatus(step),
			ExecutorGeneration: completionLaneGeneration(step),
		},
	})
	if err != nil {
		return err
	}
	if executed.Mode != sessiontransition.CompletionModeLaneClosed || step.Completion == nil {
		return fmt.Errorf("unexpected executed daily completion effect: %s", executed.Mode)
	}
	result.Completion = step.Completion
	result.Lane = executed.Lane
	result.ExecutorGeneration = executed.ExecutorGeneration
	result.FinalState = "lane-closed"
	return nil
}

func completionLaneID(step publicDriverResult) string {
	if step.Completion == nil {
		return ""
	}
	return step.Completion.Lane.ID
}

func completionLaneStatus(step publicDriverResult) string {
	if step.Completion == nil {
		return ""
	}
	return step.Completion.Lane.Status
}

func completionLaneGeneration(step publicDriverResult) int {
	if step.Completion == nil {
		return 0
	}
	return step.Completion.Lane.ExecutorGeneration
}
