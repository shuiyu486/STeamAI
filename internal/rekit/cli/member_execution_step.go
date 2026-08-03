package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

func buildMemberExecutionStep(ctx runtime.Context, opt Options, request mission.MissionCommanderDriverRequest) (*memberexecution.Plan, bool, error) {
	lane := strings.TrimSpace(request.Lane)
	if lane == "" {
		return nil, false, fmt.Errorf("current case driver request has no durable lane")
	}
	hasObservation := strings.TrimSpace(opt.MemberExecutionAttemptID) != "" || strings.TrimSpace(opt.MemberExecutionOutcome) != "" || strings.TrimSpace(opt.MemberExecutionObservedAt) != "" || strings.TrimSpace(opt.MemberExecutionReason) != ""
	if opt.skipMemberExecutionDispatch && !hasObservation {
		return nil, true, nil
	}
	board, err := mission.ReadBoard(ctx.Target)
	if err != nil {
		return nil, false, err
	}
	boardLane, ownerReady := mission.LookupBoardLane(board.Lanes, lane, false)
	ownerReady = ownerReady && strings.TrimSpace(boardLane.CurrentExecutor) != "" && boardLane.ExecutorGeneration > 0
	if !ownerReady && !hasObservation {
		return nil, true, nil
	}
	if !ownerReady {
		return nil, false, fmt.Errorf("member observation requires a current durable executor generation")
	}
	if hasObservation {
		if strings.TrimSpace(opt.MemberExecutionAttemptID) == "" || strings.TrimSpace(opt.MemberExecutionOutcome) == "" {
			return nil, false, fmt.Errorf("member observation requires -MemberExecutionAttemptId and -MemberExecutionOutcome")
		}
		plan, err := memberexecution.PreviewObservation(memberexecution.ObservationOptions{
			CaseRoot:   ctx.Target,
			Pack:       ctx.Pack,
			Lane:       lane,
			AttemptID:  opt.MemberExecutionAttemptID,
			Outcome:    opt.MemberExecutionOutcome,
			Actor:      opt.Start.Actor,
			Reason:     opt.MemberExecutionReason,
			ObservedAt: opt.MemberExecutionObservedAt,
		})
		if err != nil {
			return nil, false, err
		}
		return &plan, plan.Inspection.State == "intake-ready", nil
	}
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, false, err
	}
	sum := sha256.Sum256(requestBytes)
	requestSHA := hex.EncodeToString(sum[:])
	if latest, ok, err := memberexecution.Latest(ctx.Target, lane); err == nil && ok && latest.Intent != nil && strings.EqualFold(latest.Intent.RequestSHA256, requestSHA) {
		if latest.State == "intake-ready" {
			return nil, true, nil
		}
		if latest.State != "failed" {
			plan, err := memberexecution.PreviewDispatch(memberexecution.DispatchOptions{CaseRoot: ctx.Target, Pack: ctx.Pack, Lane: lane, RequestSHA256: requestSHA, CreatedAt: latest.Intent.CreatedAt})
			if err != nil {
				return nil, false, err
			}
			return &plan, false, nil
		}
	} else if err != nil && !memberexecution.IsPendingDispatch(err) {
		return nil, false, err
	}
	plan, err := memberexecution.PreviewDispatch(memberexecution.DispatchOptions{CaseRoot: ctx.Target, Pack: ctx.Pack, Lane: lane, RequestSHA256: requestSHA})
	if err != nil {
		return nil, false, err
	}
	return &plan, false, nil
}

func boundariesForMemberReceipt() []string {
	return []string{
		"receipt records only durable external member handoff or observation publication",
		"the Go runtime did not spawn, poll, or stop a member session and did not execute a heavy tool",
		"no authority or confirmed state was written",
	}
}

func currentStepHasMemberObservation(opt Options) bool {
	return strings.TrimSpace(opt.MemberExecutionAttemptID) != "" || strings.TrimSpace(opt.MemberExecutionOutcome) != "" || strings.TrimSpace(opt.MemberExecutionObservedAt) != "" || strings.TrimSpace(opt.MemberExecutionReason) != ""
}
