package sessiontransition

import (
	"fmt"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

type CompletionStage string

const (
	CompletionStageInitial  CompletionStage = "initial"
	CompletionStageStatus   CompletionStage = "status"
	CompletionStageExecuted CompletionStage = "executed"
)

type CompletionMode string

const (
	CompletionModePreserve          CompletionMode = "preserve"
	CompletionModeInspectStatus     CompletionMode = "inspect-status"
	CompletionModeAwaitCorrection   CompletionMode = "await-correction"
	CompletionModeMissionComplete   CompletionMode = "mission-complete"
	CompletionModeAttentionRequired CompletionMode = "attention-required"
	CompletionModeExecuteCompletion CompletionMode = "execute-completion"
	CompletionModeLaneClosed        CompletionMode = "lane-closed"
)

const (
	StateReadyForEvidenceReview             = "ready-for-evidence-review"
	StateReviewerRejectedAwaitingCorrection = "reviewer-rejected-awaiting-correction"
)

type CompletionInput struct {
	Stage                        CompletionStage
	CurrentState                 string
	SessionLaunches              int
	MissionReady                 bool
	MissionOperationallyComplete bool
	CurrentRequest               *mission.MissionCommanderDriverRequest
	Execution                    CompletionExecution
}

type CompletionExecution struct {
	ResultCommand      string
	CompletionPresent  bool
	Blocked            bool
	Applied            bool
	LaneID             string
	LaneStatus         string
	ExecutorGeneration int
}

type CompletionEffect struct {
	Mode               CompletionMode
	Replay             bool
	Lane               string
	ExecutorGeneration int
}

// ReduceCompletion is a pure state transition. It does not inspect durable
// state, execute a command, mutate a result, or publish a response.
func ReduceCompletion(input CompletionInput) (CompletionEffect, error) {
	switch input.Stage {
	case CompletionStageInitial:
		switch strings.TrimSpace(input.CurrentState) {
		case StateReadyForEvidenceReview:
			return CompletionEffect{Mode: CompletionModePreserve}, nil
		case StateReviewerRejectedAwaitingCorrection:
			return CompletionEffect{Mode: CompletionModeAwaitCorrection}, nil
		default:
			return CompletionEffect{Mode: CompletionModeInspectStatus}, nil
		}
	case CompletionStageStatus:
		if input.MissionReady && input.MissionOperationallyComplete {
			return CompletionEffect{
				Mode:   CompletionModeMissionComplete,
				Replay: input.SessionLaunches == 0,
			}, nil
		}
		request := input.CurrentRequest
		if request == nil || !request.CommandExecutable || request.Blocked {
			return CompletionEffect{Mode: CompletionModeAttentionRequired}, nil
		}
		if mission.ValidateMissionCommanderDriverRequest(*request) != nil || request.Invocation == nil || request.Invocation.Command != "complete" {
			return CompletionEffect{Mode: CompletionModeAttentionRequired}, nil
		}
		return CompletionEffect{Mode: CompletionModeExecuteCompletion}, nil
	case CompletionStageExecuted:
		execution := input.Execution
		if execution.ResultCommand != "complete" || !execution.CompletionPresent || execution.Blocked || !execution.Applied || execution.LaneStatus != "closed" {
			return CompletionEffect{}, fmt.Errorf("daily public completion did not close the current lane")
		}
		if strings.TrimSpace(execution.LaneID) == "" || execution.ExecutorGeneration < 1 {
			return CompletionEffect{}, fmt.Errorf("daily public completion omitted its closed lane identity")
		}
		return CompletionEffect{
			Mode:               CompletionModeLaneClosed,
			Lane:               strings.TrimSpace(execution.LaneID),
			ExecutorGeneration: execution.ExecutorGeneration,
		}, nil
	default:
		return CompletionEffect{}, fmt.Errorf("unsupported daily completion transition stage: %s", input.Stage)
	}
}
