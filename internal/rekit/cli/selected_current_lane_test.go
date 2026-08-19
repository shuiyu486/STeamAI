package cli

import (
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestSelectedCurrentLaneRequiresExecutable(t *testing.T) {
	for _, fixture := range []struct {
		command string
		want    bool
	}{
		{command: "status", want: false},
		{command: "run-current-step", want: true},
		{command: "run-current-loop", want: true},
		{command: "run-driver-step", want: true},
		{command: "run-reviewer-step", want: true},
		{command: "handoff", want: false},
		{command: "run-reviewer-wave", want: false},
	} {
		if got := selectedCurrentLaneRequiresExecutable(fixture.command); got != fixture.want {
			t.Fatalf("selectedCurrentLaneRequiresExecutable(%q) = %t, want %t", fixture.command, got, fixture.want)
		}
	}
}

func TestValidateStatusSelectedCurrentLaneAllowsBlockedPostMutationRefresh(t *testing.T) {
	const selected = "feature-review"
	status := statusInventory{
		MissionControlRunbook: &statusMissionControlRunbook{
			Scope: "case",
			CurrentDriverRequest: &mission.MissionCommanderDriverRequest{
				Lane:     selected,
				Blocked:  true,
				Guidance: "provide a correction for the current Reviewer rejection",
			},
			RefreshStatusCommand: `/rekit status -Target "C:\case root" -Format json -Lane feature-review`,
		},
	}
	if err := validateStatusSelectedCurrentLane(status, selected, false); err != nil {
		t.Fatalf("post-mutation blocked status refresh was rejected: %v", err)
	}
	if err := validateStatusSelectedCurrentLane(status, selected, true); err == nil || !strings.Contains(err.Error(), "blocked or not executable") {
		t.Fatalf("pre-mutation executable guard accepted blocked request: %v", err)
	}
}

func TestValidateStatusSelectedCurrentLaneAllowsOnlyDurableTerminalPostMutationRefresh(t *testing.T) {
	const selected = "main"
	caseRoot := t.TempDir()
	writeCaseFile(t, caseRoot, ".rekit/board.json", `{"schemaVersion":1,"lanes":[{"id":"main","status":"closed","authority":true}]}`)
	status := statusInventory{
		Target: caseRoot,
		CaseMission: &statusCaseMission{
			MissionCompletion: &workstream.MissionCompletionHandoff{
				Ready:                 true,
				State:                 "mission-complete",
				OperationallyComplete: true,
			},
			DailyMissionControlRunbook: workstream.DailyMissionControlRunbookForMissionComplete(caseRoot),
		},
		MissionControlRunbook: &statusMissionControlRunbook{},
	}
	if err := validateStatusSelectedCurrentLane(status, selected, false); err != nil {
		t.Fatalf("durable terminal post-mutation refresh was rejected: %v", err)
	}
	if err := validateStatusSelectedCurrentLane(status, selected, true); err == nil || !strings.Contains(err.Error(), "no current driver request") {
		t.Fatalf("pre-mutation guard accepted a terminal status without executable request: %v", err)
	}
	status.CaseMission.MissionCompletion.State = ""
	if err := validateStatusSelectedCurrentLane(status, selected, false); err == nil || !strings.Contains(err.Error(), "no current driver request") {
		t.Fatalf("non-terminal missing request was accepted after mutation: %v", err)
	}
}

func TestSelectedLaneCommandBindsExactLane(t *testing.T) {
	for _, fixture := range []struct {
		name     string
		command  string
		selected string
		want     string
	}{
		{
			name:     "insert before what-if",
			command:  `/rekit status -Target "C:\case root" -Format compact-json`,
			selected: "main",
			want:     `/rekit status -Target "C:\case root" -Format compact-json -Lane main`,
		},
		{
			name:     "bind current steamai refresh",
			command:  `/steamai status -Target "C:\case root" -Format compact-json`,
			selected: "feature-review",
			want:     `/steamai status -Target "C:\case root" -Format compact-json -Lane feature-review`,
		},
		{
			name:     "normalize feature label start",
			command:  `/rekit start mission -Apply -Target "C:\case root" -Format json`,
			selected: "feature-mission",
			want:     `/rekit start -Name mission -Lane feature-mission -Apply -Target "C:\case root" -Format json`,
		},
		{
			name:     "normalize typed lane start",
			command:  `/steamai start -Target "C:\case root" mission -Executor session-1 -Actor main-agent -WhatIf -Format json`,
			selected: "binary-analysis-mission",
			want:     `/steamai start -Target "C:\case root" -Executor session-1 -Actor main-agent -Name mission -Lane binary-analysis-mission -WhatIf -Format json`,
		},
		{
			name:     "normalize positional continue",
			command:  `/rekit continue -Target "C:\case root" main -Executor session-1 -ExpectedExecutorGeneration 1 -WhatIf -Format json`,
			selected: "main",
			want:     `/rekit continue -Target "C:\case root" -Executor session-1 -ExpectedExecutorGeneration 1 -Lane main -WhatIf -Format json`,
		},
		{
			name:     "normalize feature label continue",
			command:  `/rekit continue -Target "C:\case root" mission -Executor session-1 -ExpectedExecutorGeneration 1 -WhatIf -Format json`,
			selected: "feature-mission",
			want:     `/rekit continue -Target "C:\case root" -Executor session-1 -ExpectedExecutorGeneration 1 -Lane feature-mission -WhatIf -Format json`,
		},
		{
			name:     "normalize feature label handoff",
			command:  `/rekit handoff -Target "C:\case root" mission -WhatIf -Format json`,
			selected: "feature-mission",
			want:     `/rekit handoff -Target "C:\case root" -Lane feature-mission -WhatIf -Format json`,
		},
		{
			name:     "normalize authority label complete",
			command:  `/rekit complete -Target "C:\case root" main -Summary done -WhatIf -Format json`,
			selected: "devirt-main",
			want:     `/rekit complete -Target "C:\case root" -Summary done -Lane devirt-main -WhatIf -Format json`,
		},
		{
			name:     "normalize feature label reconcile",
			command:  `/rekit reconcile -Target "C:\case root" review -InterventionId event-1 -WhatIf -Format json`,
			selected: "feature-review",
			want:     `/rekit reconcile -Target "C:\case root" -InterventionId event-1 -Lane feature-review -WhatIf -Format json`,
		},
		{
			name:     "keep exact flag idempotent",
			command:  `/rekit continue -Target "C:\case root" -Lane main -WhatIf -Format json`,
			selected: "main",
			want:     `/rekit continue -Target "C:\case root" -Lane main -WhatIf -Format json`,
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			got := selectedLaneCommand(fixture.command, fixture.selected)
			if got != fixture.want {
				t.Fatalf("selectedLaneCommand() = %q, want %q", got, fixture.want)
			}
			if again := selectedLaneCommand(got, fixture.selected); again != got {
				t.Fatalf("selectedLaneCommand() is not idempotent: first=%q second=%q", got, again)
			}
		})
	}
}

func TestSelectedLaneCommandFailsClosedOnLaneDrift(t *testing.T) {
	for _, command := range []string{
		`/rekit continue feature-login -WhatIf -Format json`,
		`/rekit continue login -WhatIf -Format json`,
		`/rekit continue -Lane feature-login -WhatIf -Format json`,
		`/rekit continue main -Lane main -WhatIf -Format json`,
		`/rekit continue -Lane main -Lane main -WhatIf -Format json`,
		`/rekit complete -Target "C:\case root" feature-login -WhatIf -Format json`,
	} {
		if got := selectedLaneCommand(command, "main"); got != "" {
			t.Fatalf("selected lane drift returned command %q for %q", got, command)
		}
	}
}

func TestSelectedLaneCommandPositionalParserSkipsBoundedFlagValues(t *testing.T) {
	fields, err := splitDriverCommand(`/rekit continue -Target "C:\case root" -Reason "keep main selected" main -WhatIf -Format json`)
	if err != nil {
		t.Fatal(err)
	}
	index, ok := selectedLaneCommandPositionalIndex(fields)
	if !ok || index >= len(fields) || strings.TrimSpace(fields[index]) != "main" {
		t.Fatalf("positional lane index=%d ok=%t fields=%q", index, ok, fields)
	}
	if !selectedLaneCommandValueFlag("-Target") || !selectedLaneCommandValueFlag("--expected-executor-generation") || selectedLaneCommandValueFlag("-WhatIf") {
		t.Fatal("selected lane value-flag allowlist drifted")
	}
}

func TestStatusMissionControlRunbookRequiresLaneChoiceBeforePublishingDriverRequest(t *testing.T) {
	actions := []mission.MissionCommanderNextActionItem{
		{
			Lane:    "feature-login",
			Label:   "login",
			State:   "ready-to-continue",
			Command: "/rekit continue login",
			Source:  "missionCommanderActions",
		},
		{
			Lane:    "main",
			Label:   "main",
			State:   "ready-to-continue",
			Command: "/rekit continue main",
			Source:  "missionCommanderActions",
		},
	}
	project := &statusProjectHandoff{
		MissionCommanderActionQueue: mission.MissionCommanderActionQueueFor([]mission.MissionCommanderNextActionItem{{
			ActionID: "latest-batch-next-action",
			State:    "implementation-in-progress",
			Command:  "/rekit status -Format json",
			Source:   "releaseHandoffLatestBatch",
		}}),
	}

	for _, ordered := range [][]mission.MissionCommanderNextActionItem{
		actions,
		{actions[1], actions[0]},
	} {
		caseMission := &statusCaseMission{
			MissionCommanderActionQueue: mission.MissionCommanderActionQueueFor(ordered),
		}
		runbook := buildStatusMissionControlRunbook("", caseMission, project)
		if runbook.Focus != "case-lane-choice" || runbook.Scope != "case" || !runbook.Ready {
			t.Fatalf("ambiguous lane focus drifted: %+v", runbook)
		}
		if runbook.CurrentDriverRequest != nil || runbook.CurrentDriverRequestSHA256 != "" || runbook.CurrentDriverReceipt != nil {
			t.Fatalf("ambiguous status published a driver request: %+v", runbook)
		}
		if runbook.CurrentLoopOperator != nil || runbook.ReplacementExecutorTakeover != nil {
			t.Fatalf("ambiguous status leaked an executable operator/takeover: %+v", runbook)
		}
		if runbook.Quickstart == nil || runbook.Quickstart.Ready || runbook.Quickstart.CommandExecutable || runbook.Quickstart.CurrentDriverRequest != nil || runbook.Quickstart.NextStepID != "select-current-lane" || runbook.Quickstart.State != "lane-choice-required" || !runbook.Quickstart.RequiresReview {
			t.Fatalf("ambiguous quickstart did not require a typed lane choice: %+v", runbook.Quickstart)
		}
		if !mission.MissionCommanderActionQueueRequiresLaneChoice(caseMission.MissionCommanderActionQueue) || len(caseMission.MissionCommanderActionQueue.UnblockedActions) != 2 {
			t.Fatalf("ambiguous queue lost typed candidates: %+v", caseMission.MissionCommanderActionQueue)
		}
	}
}

func TestStatusMissionControlRunbookPublishesSelectedLaneDriverRequest(t *testing.T) {
	caseMission := &statusCaseMission{
		MissionCommanderActionQueue: mission.MissionCommanderActionQueueFor([]mission.MissionCommanderNextActionItem{{
			Lane:    "feature-login",
			Label:   "login",
			State:   "ready-to-continue",
			Command: "/rekit continue login",
			Source:  "missionCommanderActions",
		}}),
	}
	runbook := buildStatusMissionControlRunbook("", caseMission, nil)
	if runbook.Focus != "case-current-action" || runbook.Scope != "case" || runbook.CurrentDriverRequest == nil {
		t.Fatalf("selected lane did not publish its current request: %+v", runbook)
	}
	if runbook.CurrentDriverRequest.Lane != "feature-login" || !runbook.CurrentDriverRequest.CommandExecutable || runbook.CurrentDriverRequestSHA256 == "" {
		t.Fatalf("selected lane request identity is incomplete: request=%+v sha=%q", runbook.CurrentDriverRequest, runbook.CurrentDriverRequestSHA256)
	}
	if runbook.Quickstart == nil || !runbook.Quickstart.Ready || !runbook.Quickstart.CommandExecutable || runbook.Quickstart.CurrentDriverRequest == nil || runbook.Quickstart.CurrentDriverRequest.Lane != "feature-login" {
		t.Fatalf("selected lane quickstart is not request-bound: %+v", runbook.Quickstart)
	}
	if runbook.ReplacementExecutorTakeover == nil || !runbook.ReplacementExecutorTakeover.Ready || runbook.ReplacementExecutorTakeover.CurrentDriverRequest.Lane != "feature-login" {
		t.Fatalf("selected lane takeover is not request-bound: %+v", runbook.ReplacementExecutorTakeover)
	}
}

func TestBindSelectedLaneCurrentLoopOperatorProjectsEveryReviewerWaveCollection(t *testing.T) {
	const selected = "feature-review"
	newAttempt := func() *mission.CurrentLoopReviewerAttempt {
		return &mission.CurrentLoopReviewerAttempt{
			Identity: mission.CurrentLoopReviewerAttemptIdentity{Lane: selected},
			CurrentReviewerDriverRequest: &mission.MissionCommanderDriverRequest{
				Lane:              selected,
				Command:           `/rekit plan-subagents -WhatIf -Format json`,
				CommandExecutable: true,
				ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
					Command:              `/rekit plan-subagents -WhatIf -Format json`,
					RefreshStatusCommand: `/rekit status -Target "C:\case root" -Format compact-json`,
				},
			},
			DurableContinuationDriverRequest: &mission.MissionCommanderDriverRequest{
				Lane:              selected,
				Command:           `/rekit run-current-loop -WhatIf -Format json`,
				CommandExecutable: true,
				ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
					Command: `/rekit run-current-loop -WhatIf -Format json`,
				},
			},
			RefreshStatusCommand: `/rekit status -Target "C:\case root" -Format compact-json`,
			SelectedAction: mission.CurrentLoopReviewerAttemptAction{
				ObservationContract: mission.CurrentLoopObservationContract{
					Alternatives: []mission.CurrentLoopObservationAlternative{{
						PreviewCommandTemplate: `/rekit run-current-loop -WhatIf -Format json`,
					}},
				},
			},
		}
	}
	attempts := map[string]*mission.CurrentLoopReviewerAttempt{
		"spawn wave": newAttempt(),
		"active":     newAttempt(),
		"returned":   newAttempt(),
		"failed":     newAttempt(),
		"blocked":    newAttempt(),
		"complete":   newAttempt(),
		"shard":      newAttempt(),
	}
	wave := &mission.CurrentLoopReviewerWave{
		Lane:      selected,
		SpawnWave: []*mission.CurrentLoopReviewerAttempt{attempts["spawn wave"]},
		Active:    []*mission.CurrentLoopReviewerAttempt{attempts["active"]},
		Returned:  []*mission.CurrentLoopReviewerAttempt{attempts["returned"]},
		Failed:    []*mission.CurrentLoopReviewerAttempt{attempts["failed"]},
		Blocked:   []*mission.CurrentLoopReviewerAttempt{attempts["blocked"]},
		Complete:  []*mission.CurrentLoopReviewerAttempt{attempts["complete"]},
		Shards:    []*mission.CurrentLoopReviewerAttempt{attempts["shard"]},
	}
	operator := &mission.CurrentLoopOperatorPackage{
		ExternalReviewerHandoff: &mission.CurrentLoopExternalReviewerHandoff{Wave: wave},
	}

	bindSelectedLaneCurrentLoopOperator(operator, selected)

	for label, attempt := range attempts {
		if err := validateSelectedLaneReviewerAttempt(attempt, selected, "reviewer wave "+label+" 1"); err != nil {
			t.Fatalf("%s attempt was not projected to the exact lane: %v", label, err)
		}
		for nestedLabel, command := range map[string]string{
			"current request":         attempt.CurrentReviewerDriverRequest.Command,
			"current receipt":         attempt.CurrentReviewerDriverRequest.ExpectedReceipt.Command,
			"current receipt refresh": attempt.CurrentReviewerDriverRequest.ExpectedReceipt.RefreshStatusCommand,
			"durable request":         attempt.DurableContinuationDriverRequest.Command,
			"durable receipt":         attempt.DurableContinuationDriverRequest.ExpectedReceipt.Command,
			"attempt refresh":         attempt.RefreshStatusCommand,
			"observation alternative": attempt.SelectedAction.ObservationContract.Alternatives[0].PreviewCommandTemplate,
		} {
			if command == "" || selectedLaneCommand(command, selected) != command {
				t.Fatalf("%s %s was not bound to %s: %q", label, nestedLabel, selected, command)
			}
		}
	}
}

func TestValidateStatusSelectedCurrentLaneRejectsDriftInEveryReviewerWaveCollection(t *testing.T) {
	const selected = "feature-review"
	collections := []struct {
		label string
		set   func(*mission.CurrentLoopReviewerWave, *mission.CurrentLoopReviewerAttempt)
	}{
		{label: "spawn wave", set: func(w *mission.CurrentLoopReviewerWave, a *mission.CurrentLoopReviewerAttempt) {
			w.SpawnWave = []*mission.CurrentLoopReviewerAttempt{a}
		}},
		{label: "active", set: func(w *mission.CurrentLoopReviewerWave, a *mission.CurrentLoopReviewerAttempt) {
			w.Active = []*mission.CurrentLoopReviewerAttempt{a}
		}},
		{label: "returned", set: func(w *mission.CurrentLoopReviewerWave, a *mission.CurrentLoopReviewerAttempt) {
			w.Returned = []*mission.CurrentLoopReviewerAttempt{a}
		}},
		{label: "failed", set: func(w *mission.CurrentLoopReviewerWave, a *mission.CurrentLoopReviewerAttempt) {
			w.Failed = []*mission.CurrentLoopReviewerAttempt{a}
		}},
		{label: "blocked", set: func(w *mission.CurrentLoopReviewerWave, a *mission.CurrentLoopReviewerAttempt) {
			w.Blocked = []*mission.CurrentLoopReviewerAttempt{a}
		}},
		{label: "complete", set: func(w *mission.CurrentLoopReviewerWave, a *mission.CurrentLoopReviewerAttempt) {
			w.Complete = []*mission.CurrentLoopReviewerAttempt{a}
		}},
		{label: "shard", set: func(w *mission.CurrentLoopReviewerWave, a *mission.CurrentLoopReviewerAttempt) {
			w.Shards = []*mission.CurrentLoopReviewerAttempt{a}
		}},
	}
	for _, collection := range collections {
		t.Run(collection.label, func(t *testing.T) {
			wave := &mission.CurrentLoopReviewerWave{Lane: selected}
			collection.set(wave, &mission.CurrentLoopReviewerAttempt{
				Identity: mission.CurrentLoopReviewerAttemptIdentity{Lane: selected},
				CurrentReviewerDriverRequest: &mission.MissionCommanderDriverRequest{
					Lane: "feature-other",
				},
			})
			status := statusInventory{
				MissionControlRunbook: &statusMissionControlRunbook{
					Scope: "reviewer",
					CurrentDriverRequest: &mission.MissionCommanderDriverRequest{
						Lane:     selected,
						Guidance: "run the selected reviewer wave",
					},
					RefreshStatusCommand: `/rekit status -Target "C:\case root" -Format compact-json -Lane feature-review`,
					CurrentLoopOperator: &mission.CurrentLoopOperatorPackage{
						Lane: selected,
						ExternalReviewerHandoff: &mission.CurrentLoopExternalReviewerHandoff{
							Wave: wave,
						},
					},
				},
			}

			err := validateStatusSelectedCurrentLane(status, selected, false)
			want := "reviewer wave " + collection.label + " 1 current request lane"
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("validateStatusSelectedCurrentLane() error = %v, want %q", err, want)
			}
		})
	}
}

func TestValidateStatusSelectedCurrentLaneFailsClosedOnNestedReviewerWaveDrift(t *testing.T) {
	const selected = "feature-review"
	for _, fixture := range []struct {
		name    string
		command string
	}{
		{
			name:    "conflicting lane",
			command: `/rekit reviewer-result -Target "C:\case root" -Lane feature-other -WhatIf -Format json`,
		},
		{
			name: "empty command",
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			attempt := &mission.CurrentLoopReviewerAttempt{
				Identity: mission.CurrentLoopReviewerAttemptIdentity{
					Lane: selected,
				},
				SelectedAction: mission.CurrentLoopReviewerAttemptAction{
					ObservationContract: mission.CurrentLoopObservationContract{
						Alternatives: []mission.CurrentLoopObservationAlternative{
							{PreviewCommandTemplate: fixture.command},
						},
					},
				},
			}
			status := statusInventory{
				MissionControlRunbook: &statusMissionControlRunbook{
					Scope: "reviewer",
					CurrentDriverRequest: &mission.MissionCommanderDriverRequest{
						Lane:     selected,
						Guidance: "run the selected reviewer wave",
					},
					RefreshStatusCommand: `/rekit status -Target "C:\case root" -Format json -Lane feature-review`,
					CurrentLoopOperator: &mission.CurrentLoopOperatorPackage{
						Lane: selected,
						ExternalReviewerHandoff: &mission.CurrentLoopExternalReviewerHandoff{
							Wave: &mission.CurrentLoopReviewerWave{
								Lane:   selected,
								Shards: []*mission.CurrentLoopReviewerAttempt{attempt},
							},
						},
					},
				},
			}

			if err := validateStatusSelectedCurrentLane(status, selected, false); err == nil || !strings.Contains(err.Error(), "reviewer wave shard 1") {
				t.Fatalf("validateStatusSelectedCurrentLane() error = %v, want nested shard failure", err)
			}
		})
	}
}
