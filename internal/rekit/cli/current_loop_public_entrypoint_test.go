package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/externalsession"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestCurrentLoopDiagnosticsProjectSelectedEntrypointWithoutMutatingIdentity(t *testing.T) {
	for _, fixture := range publicEntrypointProductFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			caseRoot := publicEntrypointProductCase(t, fixture, "current-loop-diagnostics-entrypoint")
			other := commands.LegacyPublicEntrypoint
			if fixture.entrypoint == commands.LegacyPublicEntrypoint {
				other = commands.CurrentPublicEntrypoint
			}
			previewCommand := commands.LegacyPublicEntrypoint + " run-current-loop -MaxSteps 2 -WhatIf -Format json"
			applyCommand := commands.LegacyPublicEntrypoint + " run-current-loop -MaxSteps 2 -ExpectedCurrentLoopPlanSha256 " + strings.Repeat("a", 64) + " -Apply -Format json"
			request := missionDriverRequestForTest(previewCommand)
			plan := currentLoopPlan{
				ExpectedCurrentLoopPlanSHA256: strings.Repeat("b", 64),
				InitialCurrentDriverRequest:   &request,
				ResumeCommand:                 previewCommand,
				ApplyCommand:                  applyCommand,
				StopReason: currentLoopStopReason{
					CurrentDriverRequest: &request,
				},
				ContinuationRequest: &currentLoopContinuationRequest{
					WhatIfCommand: previewCommand,
				},
				InitialCurrentStep: &currentStepPlan{
					CurrentDriverRequest: request,
					DriverStep: &driverStepPlan{
						CurrentDriverRequest:        request,
						ApplyDriverRequest:          missionDriverRequestForTest(applyCommand),
						MissionCommanderActionQueue: mission.MissionCommanderActionQueueFor(nil),
					},
				},
			}
			beforePlanSHA256 := plan.ExpectedCurrentLoopPlanSHA256
			beforeRequestCommand := plan.InitialCurrentDriverRequest.Command

			diagnostics, err := buildCurrentLoopDiagnosticsDTO(plan, caseRoot)
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(diagnostics)
			if err != nil {
				t.Fatal(err)
			}
			var public any
			if err := json.Unmarshal(data, &public); err != nil {
				t.Fatal(err)
			}
			assertSelectedPublicEntrypoint(t, "currentLoop", public, fixture.entrypoint)
			if strings.Contains(string(data), other+" ") {
				t.Fatalf("current-loop diagnostics leaked unselected entrypoint %s: %s", other, data)
			}
			if diagnostics.ExpectedCurrentLoopPlanSHA256 != beforePlanSHA256 {
				t.Fatalf("projection changed current-loop plan identity: got %s want %s", diagnostics.ExpectedCurrentLoopPlanSHA256, beforePlanSHA256)
			}
			if plan.ExpectedCurrentLoopPlanSHA256 != beforePlanSHA256 || plan.InitialCurrentDriverRequest.Command != beforeRequestCommand {
				t.Fatalf("detached projection mutated domain plan: %+v", plan)
			}
		})
	}
}

func TestRunCurrentLoopDirectResponseProjectsSelectedEntrypoint(t *testing.T) {
	for _, fixture := range publicEntrypointProductFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			caseRoot := publicEntrypointProductCase(t, fixture, "current-loop-direct-entrypoint")
			if err := Run([]string{
				"-Command", commands.Overview,
				"-Target", caseRoot,
				"-Pack", "_template",
				"-Format", "json",
			}, &strings.Builder{}); err != nil {
				t.Fatal(err)
			}

			var out strings.Builder
			if err := Run([]string{
				"-Command", commands.RunCurrentLoop,
				"-Target", caseRoot,
				"-Pack", "_template",
				"-MaxSteps", "2",
				"-WhatIf",
				"-Format", "json",
			}, &out); err != nil {
				t.Fatal(err)
			}
			var public any
			if err := json.Unmarshal([]byte(out.String()), &public); err != nil {
				t.Fatalf("run-current-loop response did not decode: %v\n%s", err, out.String())
			}
			assertSelectedPublicEntrypoint(t, "runCurrentLoop", public, fixture.entrypoint)
			if count := assertDeepProjectionDriverRequests(t, "runCurrentLoop", public, fixture.entrypoint); count == 0 {
				t.Fatal("run-current-loop response omitted typed driver requests")
			}
		})
	}
}

func TestDirectStepResponsesProjectSelectedEntrypoint(t *testing.T) {
	for _, fixture := range publicEntrypointProductFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			caseRoot := publicEntrypointProductCase(t, fixture, "direct-step-entrypoint")
			if err := Run([]string{
				"-Command", commands.Overview,
				"-Target", caseRoot,
				"-Pack", "_template",
				"-Format", "json",
			}, &strings.Builder{}); err != nil {
				t.Fatal(err)
			}

			for _, command := range []string{commands.RunDriverStep, commands.RunCurrentStep} {
				var out strings.Builder
				if err := Run([]string{
					"-Command", command,
					"-Target", caseRoot,
					"-Pack", "_template",
					"-WhatIf",
					"-Format", "json",
				}, &out); err != nil {
					t.Fatal(err)
				}
				var public map[string]any
				if err := json.Unmarshal([]byte(out.String()), &public); err != nil {
					t.Fatalf("%s response did not decode: %v\n%s", command, err, out.String())
				}
				assertSelectedPublicEntrypoint(t, command, public, fixture.entrypoint)
				if count := assertDeepProjectionDriverRequests(t, command, public, fixture.entrypoint); count == 0 {
					t.Fatalf("%s response omitted typed driver requests", command)
				}
				planField := "expectedDriverStepPlanSha256"
				if command == commands.RunCurrentStep {
					planField = "expectedCurrentStepPlanSha256"
				}
				if planSHA256, _ := public[planField].(string); len(planSHA256) != 64 {
					t.Fatalf("%s response omitted plan identity %s: %v", command, planField, public[planField])
				}
			}
		})
	}
}

func TestRunReviewerStepDirectResponseProjectsSelectedEntrypoint(t *testing.T) {
	for _, fixture := range publicEntrypointProductFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			caseRoot := publicEntrypointProductCase(t, fixture, "reviewer-step-direct-entrypoint")
			if err := Run([]string{
				"-Command", commands.Overview,
				"-Target", caseRoot,
				"-Pack", "_template",
				"-Format", "json",
			}, &strings.Builder{}); err != nil {
				t.Fatal(err)
			}
			if err := Run([]string{
				"-Command", commands.Start,
				"-Target", caseRoot,
				"-Pack", "_template",
				"-Name", "review",
				"-Executor", "review-owner-executor",
				"-Actor", "mission-commander",
				"-Reason", "bind reviewer projection campaign owner",
				"-Apply",
				"-Format", "json",
			}, &strings.Builder{}); err != nil {
				t.Fatal(err)
			}
			if err := Run([]string{
				"-Command", commands.PlanSubagents,
				"-Target", caseRoot,
				"-Pack", "_template",
				"-TaskType", "feature-analysis",
				"-Items", "alpha",
				"-Lane", "feature-review",
				"-Format", "json",
			}, &strings.Builder{}); err != nil {
				t.Fatal(err)
			}

			var out strings.Builder
			if err := Run([]string{
				"-Command", commands.RunReviewerStep,
				"-Target", caseRoot,
				"-Pack", "_template",
				"-ReviewerHarness", "projection-test-harness",
				"-ReviewerSession", "projection-test-session",
				"-Actor", "mission-commander",
				"-WhatIf",
				"-Format", "json",
			}, &out); err != nil {
				t.Fatal(err)
			}
			var public map[string]any
			if err := json.Unmarshal([]byte(out.String()), &public); err != nil {
				t.Fatalf("run-reviewer-step response did not decode: %v\n%s", err, out.String())
			}
			assertSelectedPublicEntrypoint(t, "runReviewerStep", public, fixture.entrypoint)
			if count := assertDeepProjectionDriverRequests(t, "runReviewerStep", public, fixture.entrypoint); count == 0 {
				t.Fatal("run-reviewer-step response omitted typed driver requests")
			}
			if planSHA256, _ := public["expectedReviewerStepPlanSha256"].(string); len(planSHA256) != 64 {
				t.Fatalf("run-reviewer-step response omitted plan identity: %v", public["expectedReviewerStepPlanSha256"])
			}
		})
	}
}

func TestCurrentLoopExternalDiagnosticsProjectSelectedEntrypoint(t *testing.T) {
	for _, fixture := range publicEntrypointProductFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			caseRoot := publicEntrypointProductCase(t, fixture, "current-loop-external-entrypoint")
			legacyApply := commands.LegacyPublicEntrypoint + " run-current-loop -ResumeCurrentLoop -Apply -Format json"
			status := statusInventory{Target: caseRoot, Mode: "case"}

			attempt, err := buildCurrentLoopExternalSessionAttemptDiagnosticsDTO(
				currentLoopExternalSessionAttemptResult{
					AttemptPlan: externalsession.AttemptPlan{
						ExpectedPlanSHA256: strings.Repeat("a", 64),
						ApplyCommand:       legacyApply,
					},
					RefreshedStatus: &status,
				},
				caseRoot,
			)
			if err != nil {
				t.Fatal(err)
			}
			dispatch, err := buildCurrentLoopExternalSessionDispatchDiagnosticsDTO(
				currentLoopExternalSessionDispatchResult{
					DispatchPlan: externalsession.DispatchPlan{
						ExpectedPlanSHA256: strings.Repeat("b", 64),
						ApplyCommand:       legacyApply,
					},
					RefreshedStatus: &status,
				},
				caseRoot,
			)
			if err != nil {
				t.Fatal(err)
			}
			relay, err := buildCurrentLoopExternalSessionRelayDiagnosticsDTO(
				currentLoopExternalSessionRelayResult{
					Plan:            externalsession.Plan{ExpectedPlanSHA256: strings.Repeat("c", 64), ApplyCommand: legacyApply},
					RefreshedStatus: &status,
				},
				caseRoot,
			)
			if err != nil {
				t.Fatal(err)
			}
			turn, err := buildCurrentLoopExternalSessionTurnDiagnosticsDTO(
				currentLoopExternalSessionTurnPlan{
					ExpectedPlanSHA256: strings.Repeat("d", 64),
					ApplyCommand:       legacyApply,
					Relay:              externalsession.Plan{ExpectedPlanSHA256: strings.Repeat("e", 64), ApplyCommand: legacyApply},
					Resume:             currentLoopPlan{ExpectedCurrentLoopPlanSHA256: strings.Repeat("f", 64), ResumeCommand: legacyApply},
					RefreshedStatus:    &status,
				},
				caseRoot,
			)
			if err != nil {
				t.Fatal(err)
			}

			for label, value := range map[string]any{
				"attempt":  attempt,
				"dispatch": dispatch,
				"relay":    relay,
				"turn":     turn,
			} {
				data, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				var public any
				if err := json.Unmarshal(data, &public); err != nil {
					t.Fatal(err)
				}
				assertSelectedPublicEntrypoint(t, label, public, fixture.entrypoint)
			}
			if attempt.ExpectedPlanSHA256 != strings.Repeat("a", 64) ||
				dispatch.ExpectedPlanSHA256 != strings.Repeat("b", 64) ||
				relay.ExpectedPlanSHA256 != strings.Repeat("c", 64) ||
				turn.ExpectedPlanSHA256 != strings.Repeat("d", 64) ||
				turn.Resume.ExpectedCurrentLoopPlanSHA256 != strings.Repeat("f", 64) {
				t.Fatalf("external diagnostics projection changed plan identities")
			}
		})
	}
}
