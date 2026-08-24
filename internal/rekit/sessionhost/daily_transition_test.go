package sessionhost

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestDailyCompletionSupervisorPreservesTerminalStatesWithoutIO(t *testing.T) {
	for _, test := range []struct {
		name        string
		state       string
		wantBlocked bool
	}{
		{name: "evidence review", state: DailyActionReadyForEvidenceReview},
		{name: "reviewer rejection", state: "reviewer-rejected-awaiting-correction", wantBlocked: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			owner := transitionOwnerForTest(t)
			owner.statusExecutor = func(string, string, string) (publicStatus, error) {
				t.Fatal("terminal transition read status")
				return publicStatus{}, nil
			}
			owner.completionPublisher = func(string, string, string) (publicDriverResult, error) {
				t.Fatal("terminal transition published completion")
				return publicDriverResult{}, nil
			}
			result := DailyResult{FinalState: test.state}
			if err := owner.finish(&result); err != nil {
				t.Fatal(err)
			}
			if result.FinalState != test.state || result.Blocked != test.wantBlocked {
				t.Fatalf("terminal result drifted: %+v", result)
			}
		})
	}
}

func TestDailyCompletionSupervisorOrdersStatusBeforeSinglePublication(t *testing.T) {
	owner := transitionOwnerForTest(t)
	trace := []string{}
	owner.statusExecutor = func(caseRoot, pack, lane string) (publicStatus, error) {
		trace = append(trace, "status")
		assertTransitionBinding(t, caseRoot, pack, lane)
		return transitionCompletionStatus(t), nil
	}
	owner.completionPublisher = func(caseRoot, pack, lane string) (publicDriverResult, error) {
		trace = append(trace, "publish")
		assertTransitionBinding(t, caseRoot, pack, lane)
		return publicDriverResult{
			ResultCommand: "complete",
			Completion: &workstream.CompleteResult{
				Applied: true,
				Lane: workstream.Lane{
					ID:                 "feature-mission",
					Status:             "closed",
					ExecutorGeneration: 7,
				},
			},
		}, nil
	}
	result := DailyResult{FinalState: "member-complete", Lane: "feature-mission"}
	if err := owner.finish(&result); err != nil {
		t.Fatal(err)
	}
	if strings.Join(trace, ",") != "status,publish" {
		t.Fatalf("completion trace=%v, want status,publish", trace)
	}
	if result.FinalState != "lane-closed" || result.Blocked || result.Lane != "feature-mission" || result.ExecutorGeneration != 7 || result.Completion == nil || strings.Join(result.DriverSteps, ",") != "complete" {
		t.Fatalf("completion publication drifted: %+v", result)
	}
}

func TestDailyCompletionSupervisorShortCircuitsCompletedMission(t *testing.T) {
	owner := transitionOwnerForTest(t)
	trace := []string{}
	owner.statusExecutor = func(string, string, string) (publicStatus, error) {
		trace = append(trace, "status")
		return transitionMissionCompleteStatus(t), nil
	}
	owner.completionPublisher = func(string, string, string) (publicDriverResult, error) {
		trace = append(trace, "publish")
		return publicDriverResult{}, errors.New("must not publish")
	}
	result := DailyResult{FinalState: "member-complete", SessionLaunches: 0}
	if err := owner.finish(&result); err != nil {
		t.Fatal(err)
	}
	if strings.Join(trace, ",") != "status" || result.FinalState != "mission-complete" || !result.Replay {
		t.Fatalf("mission completion did not short circuit: trace=%v result=%+v", trace, result)
	}
}

func TestDailyCompletionSupervisorDoesNotAdvanceOrRetryAfterErrors(t *testing.T) {
	for _, test := range []struct {
		name       string
		statusErr  error
		publishErr error
		wantTrace  string
		wantError  string
	}{
		{
			name:      "status failure",
			statusErr: errors.New("status unavailable"),
			wantTrace: "status",
			wantError: "status unavailable",
		},
		{
			name:       "publication failure",
			publishErr: errors.New("publication uncertain"),
			wantTrace:  "status,publish",
			wantError:  "daily public completion: publication uncertain",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			owner := transitionOwnerForTest(t)
			trace := []string{}
			owner.statusExecutor = func(string, string, string) (publicStatus, error) {
				trace = append(trace, "status")
				if test.statusErr != nil {
					return publicStatus{}, test.statusErr
				}
				return transitionCompletionStatus(t), nil
			}
			owner.completionPublisher = func(string, string, string) (publicDriverResult, error) {
				trace = append(trace, "publish")
				return publicDriverResult{}, test.publishErr
			}
			result := DailyResult{FinalState: "member-complete", Lane: "feature-mission"}
			err := owner.finish(&result)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("finish error=%v, want containing %q", err, test.wantError)
			}
			if strings.Join(trace, ",") != test.wantTrace {
				t.Fatalf("failure trace=%v, want %s", trace, test.wantTrace)
			}
			if len(result.DriverSteps) != 0 || result.Completion != nil || result.FinalState != "member-complete" {
				t.Fatalf("failed effect published result state: %+v", result)
			}
		})
	}
}

func TestDailySessionSupervisorOrdersStatusBeforeSessionExecutor(t *testing.T) {
	owner := transitionOwnerForTest(t)
	status := transitionCompletionStatus(t)
	trace := []string{}
	owner.statusExecutor = func(caseRoot, pack, lane string) (publicStatus, error) {
		trace = append(trace, "status")
		assertTransitionBinding(t, caseRoot, pack, lane)
		return status, nil
	}
	owner.sessionExecutor = func(_ context.Context, opt Options) (Result, error) {
		trace = append(trace, "session")
		if opt.Target != "case-root" || opt.Pack != "binary-re" || opt.SelectedLane != "feature-mission" || !opt.requireCurrentDriverRequest || len(opt.ExpectedCurrentDriverRequestSHA256) != 64 {
			t.Fatalf("session executor binding drifted: %+v", opt)
		}
		return Result{FinalMode: "member-complete"}, nil
	}
	result := DailyResult{FinalState: "not-started"}
	if err := owner.runHostSegment(context.Background(), Options{
		Target:       "case-root",
		Pack:         "binary-re",
		SelectedLane: "feature-mission",
	}, &result); err != nil {
		t.Fatal(err)
	}
	if strings.Join(trace, ",") != "status,session" || result.FinalState != "member-complete" || len(result.HostRuns) != 1 {
		t.Fatalf("session supervisor trace/result drifted: trace=%v result=%+v", trace, result)
	}
}

func transitionOwnerForTest(t *testing.T) dailySessionTransitionOwner {
	t.Helper()
	owner, err := newDailySessionTransitionOwner("case-root", "binary-re", "feature-mission")
	if err != nil {
		t.Fatal(err)
	}
	return owner
}

func transitionCompletionStatus(t *testing.T) publicStatus {
	t.Helper()
	invocation, err := commands.NewPublicInvocation(
		commands.Complete,
		"-Lane", "feature-mission",
		"-WhatIf", "-Format", "json",
	)
	if err != nil {
		t.Fatal(err)
	}
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
		Invocation:        &invocation,
		Command:           command,
		CommandExecutable: true,
		ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
			Command: command,
		},
	}
	identity, err := mission.MissionCommanderDriverRequestSHA256(request)
	if err != nil {
		t.Fatal(err)
	}
	return publicStatus{MissionControlRunbook: &publicMissionControlRunbook{
		Scope:                      "case",
		CurrentDriverRequest:       &request,
		CurrentDriverRequestSHA256: identity,
	}}
}

func transitionMissionCompleteStatus(t *testing.T) publicStatus {
	t.Helper()
	var status publicStatus
	if err := json.Unmarshal([]byte(`{
		"caseMission":{"missionCompletion":{
			"ready":true,
			"state":"mission-complete",
			"operationallyComplete":true
		}}
	}`), &status); err != nil {
		t.Fatal(err)
	}
	return status
}

func assertTransitionBinding(t *testing.T, caseRoot, pack, lane string) {
	t.Helper()
	if caseRoot != "case-root" || pack != "binary-re" || lane != "feature-mission" {
		t.Fatalf("transition binding drifted: case=%q pack=%q lane=%q", caseRoot, pack, lane)
	}
}
