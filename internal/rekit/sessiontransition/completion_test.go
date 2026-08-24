package sessiontransition

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestReduceCompletionOwnsPureTransitionMatrix(t *testing.T) {
	complete := completionRequest(t)
	wrong := completionRequest(t)
	wrong.Invocation = invocationFor(t, commands.Continue, "-Lane", "feature-mission", "-WhatIf", "-Format", "json")
	wrong.Command = "/rekit continue -Lane feature-mission -WhatIf -Format json"
	wrong.ExpectedReceipt.Command = wrong.Command

	for _, test := range []struct {
		name    string
		input   CompletionInput
		want    CompletionEffect
		wantErr string
	}{
		{
			name:  "preserve evidence review",
			input: CompletionInput{Stage: CompletionStageInitial, CurrentState: StateReadyForEvidenceReview},
			want:  CompletionEffect{Mode: CompletionModePreserve},
		},
		{
			name:  "await reviewer correction",
			input: CompletionInput{Stage: CompletionStageInitial, CurrentState: StateReviewerRejectedAwaitingCorrection},
			want:  CompletionEffect{Mode: CompletionModeAwaitCorrection},
		},
		{
			name:  "inspect ordinary state",
			input: CompletionInput{Stage: CompletionStageInitial, CurrentState: "member-complete"},
			want:  CompletionEffect{Mode: CompletionModeInspectStatus},
		},
		{
			name: "mission complete replay",
			input: CompletionInput{
				Stage:                        CompletionStageStatus,
				MissionReady:                 true,
				MissionOperationallyComplete: true,
			},
			want: CompletionEffect{Mode: CompletionModeMissionComplete, Replay: true},
		},
		{
			name: "mission complete after launch",
			input: CompletionInput{
				Stage:                        CompletionStageStatus,
				SessionLaunches:              1,
				MissionReady:                 true,
				MissionOperationallyComplete: true,
			},
			want: CompletionEffect{Mode: CompletionModeMissionComplete},
		},
		{
			name:  "missing request needs attention",
			input: CompletionInput{Stage: CompletionStageStatus},
			want:  CompletionEffect{Mode: CompletionModeAttentionRequired},
		},
		{
			name: "blocked request needs attention",
			input: CompletionInput{Stage: CompletionStageStatus, CurrentRequest: &mission.MissionCommanderDriverRequest{
				CommandExecutable: true,
				Blocked:           true,
			}},
			want: CompletionEffect{Mode: CompletionModeAttentionRequired},
		},
		{
			name:  "wrong request needs attention",
			input: CompletionInput{Stage: CompletionStageStatus, CurrentRequest: &wrong},
			want:  CompletionEffect{Mode: CompletionModeAttentionRequired},
		},
		{
			name:  "execute exact completion",
			input: CompletionInput{Stage: CompletionStageStatus, CurrentRequest: &complete},
			want:  CompletionEffect{Mode: CompletionModeExecuteCompletion},
		},
		{
			name: "publish closed lane",
			input: CompletionInput{Stage: CompletionStageExecuted, Execution: CompletionExecution{
				ResultCommand:      "complete",
				CompletionPresent:  true,
				Applied:            true,
				LaneID:             " feature-mission ",
				LaneStatus:         "closed",
				ExecutorGeneration: 4,
			}},
			want: CompletionEffect{Mode: CompletionModeLaneClosed, Lane: "feature-mission", ExecutorGeneration: 4},
		},
		{
			name: "reject incomplete execution",
			input: CompletionInput{Stage: CompletionStageExecuted, Execution: CompletionExecution{
				ResultCommand:     "complete",
				CompletionPresent: true,
				LaneID:            "feature-mission",
				LaneStatus:        "closed",
			}},
			wantErr: "did not close",
		},
		{
			name: "reject missing closed lane identity",
			input: CompletionInput{Stage: CompletionStageExecuted, Execution: CompletionExecution{
				ResultCommand:     "complete",
				CompletionPresent: true,
				Applied:           true,
				LaneStatus:        "closed",
			}},
			wantErr: "omitted its closed lane identity",
		},
		{
			name:    "reject unknown stage",
			input:   CompletionInput{Stage: "future"},
			wantErr: "unsupported daily completion transition stage",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := ReduceCompletion(test.input)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("ReduceCompletion error=%v, want containing %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("ReduceCompletion=%+v, want %+v", got, test.want)
			}
		})
	}
}

func TestReduceCompletionDoesNotMutateCanonicalRequest(t *testing.T) {
	request := completionRequest(t)
	before, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReduceCompletion(CompletionInput{
		Stage:          CompletionStageStatus,
		CurrentRequest: &request,
	}); err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("pure reducer mutated canonical request:\nbefore=%s\nafter=%s", before, after)
	}
}

func completionRequest(t *testing.T) mission.MissionCommanderDriverRequest {
	t.Helper()
	invocation := invocationFor(t, commands.Complete, "-Lane", "feature-mission", "-WhatIf", "-Format", "json")
	command, err := invocation.Render()
	if err != nil {
		t.Fatal(err)
	}
	request := mission.MissionCommanderDriverRequest{
		Kind:              "preview-command",
		RunLoopStepID:     "preview-current",
		Actor:             "main-agent",
		Source:            "laneCompletion.acceptedReviewerLineage",
		Lane:              "feature-mission",
		Invocation:        invocation,
		Command:           command,
		CommandExecutable: true,
		ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
			Command: command,
		},
	}
	if err := mission.ValidateMissionCommanderDriverRequest(request); err != nil {
		t.Fatalf("completion request is invalid: %v", err)
	}
	return request
}

func invocationFor(t *testing.T, command string, arguments ...string) *commands.PublicInvocation {
	t.Helper()
	invocation, err := commands.NewPublicInvocation(command, arguments...)
	if err != nil {
		t.Fatal(err)
	}
	return &invocation
}
