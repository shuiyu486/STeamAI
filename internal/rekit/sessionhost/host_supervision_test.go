package sessionhost

import (
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestRequireSameRunningHandoffAcceptsExactCurrentPackage(t *testing.T) {
	plan := supervisedRunningPlanForTest()
	if err := requireSameRunningHandoff(plan, plan); err != nil {
		t.Fatal(err)
	}
}

func TestRequireSameRunningHandoffRejectsLateGenerationAndBindingDrift(t *testing.T) {
	original := supervisedRunningPlanForTest()
	tests := map[string]func(*currentStepPlan){
		"generation": func(plan *currentStepPlan) { plan.ExternalSessionStep.HarnessPackage.Launch.Attempt.Generation++ },
		"attempt": func(plan *currentStepPlan) {
			plan.ExternalSessionStep.HarnessPackage.Launch.Attempt.AttemptSHA256 = "late-attempt"
		},
		"session": func(plan *currentStepPlan) {
			plan.ExternalSessionStep.HarnessPackage.Launch.Attempt.Session = "replacement-session"
		},
		"job": func(plan *currentStepPlan) { plan.ExternalSessionStep.HarnessPackage.JobSHA256 = "replacement-job" },
		"checkpoint": func(plan *currentStepPlan) {
			plan.ExternalSessionStep.HarnessPackage.CheckpointSHA256 = "replacement-checkpoint"
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			fresh := supervisedRunningPlanForTest()
			mutate(&fresh)
			if err := requireSameRunningHandoff(original, fresh); err == nil {
				t.Fatal("late or drifted supervised result should be fenced")
			}
		})
	}
}

func TestRequireSameRunningHandoffRejectsCompletedOrSubmissionTurn(t *testing.T) {
	original := supervisedRunningPlanForTest()
	for _, fresh := range []currentStepPlan{
		{},
		{ExternalSessionStep: &externalSessionStep{Mode: "result-turn", HarnessPackage: original.ExternalSessionStep.HarnessPackage}},
	} {
		if err := requireSameRunningHandoff(original, fresh); err == nil {
			t.Fatal("non-running current step should reject late supervised publication")
		}
	}
}

func TestCurrentStepIsEvidenceReviewStopRequiresExactTypedShape(t *testing.T) {
	plan := currentStepPlan{CurrentDriverRequest: mission.MissionCommanderDriverRequest{
		Source: "executionEvidenceReview",
		State:  "ready-for-evidence-review",
	}}
	if !currentStepIsEvidenceReviewStop(plan) {
		t.Fatal("typed evidence review stop was not recognized")
	}
	plan.CurrentDriverRequest.Source = "executionEvidenceReview.followUp"
	if !currentStepIsEvidenceReviewStop(plan) {
		t.Fatal("typed evidence review follow-up stop was not recognized")
	}
	plan.MemberExecution = &memberexecution.Plan{}
	if currentStepIsEvidenceReviewStop(plan) {
		t.Fatal("evidence review stop accepted a member dispatch plan")
	}
}

func supervisedRunningPlanForTest() currentStepPlan {
	return currentStepPlan{ExternalSessionStep: &externalSessionStep{
		Mode: "running-handoff",
		HarnessPackage: &mission.CurrentLoopExternalSessionHarnessPackage{
			JobSHA256: "job-sha", CheckpointSHA256: "checkpoint-sha", SessionKind: "member",
			Launch: &mission.CurrentLoopExternalSessionHarnessLaunch{Attempt: mission.CurrentLoopExternalSessionAttempt{
				AttemptID: "attempt-id", AttemptSHA256: "attempt-sha", Generation: 1, Session: "session-id",
			}},
		},
	}}
}
