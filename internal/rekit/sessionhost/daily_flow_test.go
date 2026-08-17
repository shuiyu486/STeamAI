package sessionhost

import (
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestDailyCompletionOwnerRequest(t *testing.T) {
	valid := func() publicStatus {
		invocation, err := commands.NewPublicInvocation(commands.Complete, "-Lane", "feature-mission", "-WhatIf", "-Format", "json")
		if err != nil {
			t.Fatal(err)
		}
		return publicStatus{MissionControlRunbook: &publicMissionControlRunbook{Scope: "case", CurrentDriverRequest: &publicDriverRequest{
			Kind:              "preview-command",
			RunLoopStepID:     "preview-current",
			Actor:             "main-agent",
			Source:            "laneCompletion.acceptedReviewerLineage",
			Lane:              "feature-mission",
			Invocation:        &invocation,
			Command:           "/rekit complete -Lane feature-mission -WhatIf -Format json",
			CommandExecutable: true,
			ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
				Command: "/rekit complete -Lane feature-mission -WhatIf -Format json",
			},
		}}}
	}
	if !dailyCompletionOwnerRequest(valid(), "feature-mission") {
		t.Fatal("exact accepted-lineage completion owner was rejected")
	}
	tests := map[string]func(*publicStatus){
		"scope": func(status *publicStatus) { status.MissionControlRunbook.Scope = "reviewer" },
		"kind":  func(status *publicStatus) { status.MissionControlRunbook.CurrentDriverRequest.Kind = "execute-command" },
		"step": func(status *publicStatus) {
			status.MissionControlRunbook.CurrentDriverRequest.RunLoopStepID = "apply-or-run-current"
		},
		"actor": func(status *publicStatus) {
			status.MissionControlRunbook.CurrentDriverRequest.Actor = "main-agent-harness"
		},
		"source": func(status *publicStatus) {
			status.MissionControlRunbook.CurrentDriverRequest.Source = "missionCommanderActions"
		},
		"lane":     func(status *publicStatus) { status.MissionControlRunbook.CurrentDriverRequest.Lane = "feature-other" },
		"blocked":  func(status *publicStatus) { status.MissionControlRunbook.CurrentDriverRequest.Blocked = true },
		"guidance": func(status *publicStatus) { status.MissionControlRunbook.CurrentDriverRequest.Guidance = "complete it" },
		"not-executable": func(status *publicStatus) {
			status.MissionControlRunbook.CurrentDriverRequest.CommandExecutable = false
		},
		"wrong-command": func(status *publicStatus) {
			invocation, err := commands.NewPublicInvocation(commands.Continue, "-Lane", "feature-mission", "-WhatIf", "-Format", "json")
			if err != nil {
				t.Fatal(err)
			}
			status.MissionControlRunbook.CurrentDriverRequest.Invocation = &invocation
			status.MissionControlRunbook.CurrentDriverRequest.Command = "/rekit continue -Lane feature-mission -WhatIf -Format json"
		},
		"command-projection-drift": func(status *publicStatus) {
			status.MissionControlRunbook.CurrentDriverRequest.Command = "/rekit continue -Lane feature-mission -WhatIf -Format json"
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			status := valid()
			mutate(&status)
			if dailyCompletionOwnerRequest(status, "feature-mission") {
				t.Fatalf("drifted completion owner accepted: %+v", status.MissionControlRunbook.CurrentDriverRequest)
			}
		})
	}
}

func TestDailyReviewerOwnerRequest(t *testing.T) {
	valid := func(step string) publicStatus {
		request := &publicDriverRequest{
			Kind:              "preview-command",
			RunLoopStepID:     step,
			Actor:             "main-agent",
			Source:            "reviewerDispatchOperatorPackage",
			Lane:              "feature-mission",
			Command:           "/rekit plan-subagents -WhatIf -Format json",
			CommandExecutable: true,
		}
		if step == "spawn-reviewer" {
			request.Kind = "review-guidance"
			request.Actor = "main-agent-harness"
			request.Command = ""
			request.CommandExecutable = false
			request.Guidance = "invoke the read-only Agent tool request"
		}
		return publicStatus{MissionControlRunbook: &publicMissionControlRunbook{Scope: "reviewer", CurrentDriverRequest: request}}
	}

	for step := range dailyReviewerRunLoopSteps {
		if !dailyReviewerOwnerRequest(valid(step), "feature-mission") {
			t.Fatalf("exact reviewer owner rejected step %q", step)
		}
	}

	tests := map[string]func(*publicStatus){
		"scope": func(status *publicStatus) {
			status.MissionControlRunbook.Scope = "case"
		},
		"source": func(status *publicStatus) {
			status.MissionControlRunbook.CurrentDriverRequest.Source = "missionCommanderActions"
		},
		"lane": func(status *publicStatus) {
			status.MissionControlRunbook.CurrentDriverRequest.Lane = "feature-other"
		},
		"blocked": func(status *publicStatus) {
			status.MissionControlRunbook.CurrentDriverRequest.Blocked = true
		},
		"unknown-step": func(status *publicStatus) {
			status.MissionControlRunbook.CurrentDriverRequest.RunLoopStepID = "future-reviewer-step"
		},
		"spawn-command": func(status *publicStatus) {
			status.MissionControlRunbook.CurrentDriverRequest.Command = "/rekit continue"
		},
		"spawn-executable": func(status *publicStatus) {
			status.MissionControlRunbook.CurrentDriverRequest.CommandExecutable = true
		},
		"spawn-actor": func(status *publicStatus) {
			status.MissionControlRunbook.CurrentDriverRequest.Actor = "main-agent"
		},
		"spawn-guidance": func(status *publicStatus) {
			status.MissionControlRunbook.CurrentDriverRequest.Guidance = ""
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			status := valid("spawn-reviewer")
			mutate(&status)
			if dailyReviewerOwnerRequest(status, "feature-mission") {
				t.Fatalf("drifted reviewer owner accepted: %+v", status.MissionControlRunbook.CurrentDriverRequest)
			}
		})
	}
}
