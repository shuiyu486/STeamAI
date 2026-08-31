package sessionhost

import (
	"encoding/json"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestDailyUserActionProjectsStableCodes(t *testing.T) {
	tests := []struct {
		name          string
		result        DailyResult
		code          string
		requiresInput bool
		choiceIDs     []string
	}{
		{name: "mission completed", result: DailyResult{FinalState: "mission-complete"}, code: "completed"},
		{name: "lane completed", result: DailyResult{FinalState: "lane-closed"}, code: "ready-to-continue"},
		{name: "member ready", result: DailyResult{FinalState: "member-intake-ready"}, code: "ready-to-continue"},
		{name: "reviewer rejected", result: DailyResult{FinalState: "reviewer-rejected-awaiting-correction", Blocked: true}, code: "waiting-for-correction", requiresInput: true, choiceIDs: []string{"provide-correction", "stop"}},
		{name: "typed input required", result: DailyResult{FinalState: "input-required", Blocked: true}, code: "input-required", requiresInput: true, choiceIDs: []string{"artifact-analysis", "workspace-inventory"}},
		{name: "typed blocker", result: DailyResult{FinalState: "attention-required", Blocked: true}, code: "blocked"},
		{name: "evidence review", result: DailyResult{FinalState: "ready-for-evidence-review"}, code: "ready-for-evidence-review", requiresInput: true, choiceIDs: []string{"review-evidence", "defer"}},
		{name: "unknown state fails closed", result: DailyResult{FinalState: "future-state"}, code: "blocked"},
		{name: "typed failure wins", result: DailyResult{FinalState: "mission-complete", Failure: &FailureDiagnosis{Code: "claude-timeout", State: failureStateRecoverable, Recoverable: true, MutationBoundary: "none", NextAction: "Narrow the goal, then continue."}}, code: "failed"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action := dailyUserAction(test.result)
			if action == nil || action.Code != test.code || action.RequiresInput != test.requiresInput {
				t.Fatalf("dailyUserAction(%+v) = %+v", test.result, action)
			}
			if test.name == "typed failure wins" && action.Recovery != "缩小目标或增加等待时间后，重试同一操作。" {
				t.Fatalf("failure recovery = %q, want ordinary-language guidance", action.Recovery)
			}
			if len(action.Choices) != len(test.choiceIDs) {
				t.Fatalf("choices = %+v, want ids %v", action.Choices, test.choiceIDs)
			}
			for index, want := range test.choiceIDs {
				if action.Choices[index].ID != want {
					t.Fatalf("choice[%d] = %+v, want id %s", index, action.Choices[index], want)
				}
			}
		})
	}
}

func TestDailyUserActionExplainsNowReasonAndNext(t *testing.T) {
	for _, result := range []DailyResult{
		{FinalState: "mission-complete"},
		{FinalState: "lane-closed"},
		{FinalState: "lane-closed", CurrentDriverRequest: &mission.MissionCommanderDriverRequest{State: "ready-to-continue"}},
		{FinalState: "member-intake-ready"},
		{FinalState: "reviewer-rejected-awaiting-correction", Blocked: true},
		{FinalState: "input-required", Blocked: true},
		{FinalState: "attention-required", Blocked: true},
	} {
		action := dailyUserAction(result)
		if action == nil || action.Now == "" || action.Reason == "" || action.Next == "" {
			t.Fatalf("daily action omitted now/reason/next: result=%+v action=%+v", result, action)
		}
	}
}

func TestDailyFailureActionMapsTypedDiagnosisToOneRecovery(t *testing.T) {
	typedNext := "Increase -timeout or narrow the goal, then rerun the host from refreshed status."
	wantNext := "缩小目标或增加等待时间后，重试同一操作。"
	action := dailyUserAction(DailyResult{
		FinalState: "attention-required",
		Failure: &FailureDiagnosis{
			Code: "claude-timeout", State: failureStateReplaceable, Replaceable: true,
			MutationBoundary: "session-launch-recorded", MutationApplied: true,
			AttemptsUsed: 1, AttemptsLimit: 3, NextAction: typedNext,
			Detail: "internal provider output must not become user guidance",
		},
	})
	if action == nil || action.RecoveryState != DailyRecoveryRetryable {
		t.Fatalf("retryable action = %+v", action)
	}
	if action.Reason != "Claude Code 在本次等待时间内没有完成。" {
		t.Fatalf("ordinary-language reason = %q", action.Reason)
	}
	if action.Next != wantNext || action.Recovery != wantNext {
		t.Fatalf("action did not localize typed recovery guidance: %+v", action)
	}
	if action.Next == typedNext || action.Recovery == typedNext {
		t.Fatalf("action leaked diagnostic nextAction into the user view: %+v", action)
	}
	if action.Reason == "internal provider output must not become user guidance" {
		t.Fatalf("failure detail leaked into user reason: %+v", action)
	}
}

func TestDailyFailureActionLocalizesUnknownGuidanceWithoutMutatingDiagnosis(t *testing.T) {
	failure := &FailureDiagnosis{
		Code:             "future-recoverable-failure",
		State:            failureStateRecoverable,
		Recoverable:      true,
		MutationBoundary: "none",
		NextAction:       "Inspect fresh typed state.",
		Detail:           "diagnostic detail remains typed",
	}
	action := dailyUserAction(DailyResult{Failure: failure})
	want := "刷新状态并查看失败详情后，再决定如何处理。"
	if action == nil || action.Next != want || action.Recovery != want {
		t.Fatalf("unknown failure user guidance = %+v", action)
	}
	if action.RecoveryState != DailyRecoveryUserDecisionRequired {
		t.Fatalf("unknown failure automatic recovery classification = %+v", action)
	}
	if failure.NextAction != "Inspect fresh typed state." || failure.Detail != "diagnostic detail remains typed" {
		t.Fatalf("typed failure diagnosis was mutated: %+v", failure)
	}
}

func TestDailyFailureActionFailsClosedAcrossDecisionBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		failure FailureDiagnosis
	}{
		{
			name: "ambiguous lane",
			failure: FailureDiagnosis{
				Code: "current-driver-request-lane-required", State: failureStateRecoverable, Recoverable: true,
				MutationBoundary: "none", NextAction: "Select one typed lane.",
			},
		},
		{
			name: "missing mutation boundary",
			failure: FailureDiagnosis{
				Code: "claude-timeout", State: failureStateRecoverable, Recoverable: true,
				AttemptsUsed: 1, AttemptsLimit: 3, NextAction: "Inspect current state.",
			},
		},
		{
			name: "unknown mutation",
			failure: FailureDiagnosis{
				Code: "claude-timeout", State: failureStateRecoverable, Recoverable: true,
				MutationBoundary: "future-mutation-boundary", MutationApplied: true,
				AttemptsUsed: 1, AttemptsLimit: 3, NextAction: "Inspect current state.",
			},
		},
		{
			name: "generic host failure",
			failure: FailureDiagnosis{
				Code: "claude-host-operation-failed", State: failureStateRecoverable, Recoverable: true,
				MutationBoundary: "none", NextAction: "Inspect the typed stage.",
			},
		},
		{
			name: "sync boundary",
			failure: FailureDiagnosis{
				Code: "future-sync-required", State: failureStateRecoverable, Recoverable: true,
				MutationBoundary: "none", NextAction: "Review exact sync scope.",
			},
		},
		{
			name: "promote boundary",
			failure: FailureDiagnosis{
				Code: "future-promote-required", State: failureStateRecoverable, Recoverable: true,
				MutationBoundary: "none", NextAction: "Review exact promote scope.",
			},
		},
		{
			name: "heavy scope boundary",
			failure: FailureDiagnosis{
				Code: "future-heavy-scope-required", State: failureStateRecoverable, Recoverable: true,
				MutationBoundary: "none", NextAction: "Review authorization and scope.",
			},
		},
		{
			name: "exhausted attempts",
			failure: FailureDiagnosis{
				Code: "claude-timeout", State: failureStateRecoverable, Recoverable: true,
				MutationBoundary: "session-launch-recorded", MutationApplied: true,
				AttemptsUsed: 3, AttemptsLimit: 3, NextAction: "Inspect the last failure.",
			},
		},
		{
			name: "reviewer correction",
			failure: FailureDiagnosis{
				Code: "claude-reported-failure", State: failureStateTerminal, Terminal: true,
				MutationBoundary: "failed-submission-recorded", MutationApplied: true,
				NextAction: "Review the reported reason.",
			},
		},
		{
			name: "permission decision",
			failure: FailureDiagnosis{
				Code: "claude-permission-denied", State: failureStateReplaceable, Replaceable: true,
				MutationBoundary: "session-launch-recorded", MutationApplied: true,
				AttemptsUsed: 1, AttemptsLimit: 3, NextAction: "Review denied access without bypassing permissions.",
			},
		},
		{
			name: "authority boundary",
			failure: FailureDiagnosis{
				Code: "future-authority-required", State: failureStateRecoverable, Recoverable: true,
				MutationBoundary: "none", NextAction: "Ask the user to decide.",
			},
		},
		{
			name: "unknown typed failure",
			failure: FailureDiagnosis{
				Code: "future-recoverable-failure", State: failureStateRecoverable, Recoverable: true,
				MutationBoundary: "none", NextAction: "Inspect fresh typed state.",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action := dailyUserAction(DailyResult{Failure: &test.failure})
			if action == nil || action.RecoveryState != DailyRecoveryUserDecisionRequired {
				t.Fatalf("unsafe automatic recovery classification: %+v", action)
			}
			if action.Next == "" || action.Recovery != action.Next {
				t.Fatalf("decision action omitted its one recovery: %+v", action)
			}
		})
	}
}

func TestDailyUserActionMarksOnlyDeterministicRecovery(t *testing.T) {
	recoveredSession := Result{Sessions: []Session{{Recovered: true, Outcome: "returned-recovered"}}}
	recovered := DailyResult{FinalState: "member-intake-ready", HostRuns: []Result{recoveredSession}}
	if action := dailyUserAction(recovered); action == nil || action.RecoveryState != DailyRecoveryAutoRecovered {
		t.Fatalf("deterministically recovered action = %+v", action)
	}

	for _, result := range []DailyResult{
		{FinalState: "reviewer-rejected-awaiting-correction", Blocked: true, HostRuns: []Result{recoveredSession}},
		{FinalState: "ready-for-evidence-review", HostRuns: []Result{recoveredSession}},
		{FinalState: "attention-required", Blocked: true, HostRuns: []Result{recoveredSession}},
		{FinalState: "future-state", HostRuns: []Result{recoveredSession}},
		{FinalState: "member-intake-ready", Replacements: 1},
		{FinalState: "member-intake-ready", HostRuns: []Result{{Sessions: []Session{{Recovered: true, Outcome: "failed-recovered"}}}}},
		{FinalState: "member-intake-ready", HostRuns: []Result{{Sessions: []Session{{Recovered: true, Outcome: "returned-recovered", Failure: &FailureDiagnosis{Code: "claude-reported-failure"}}}}}},
	} {
		if action := dailyUserAction(result); action == nil || action.RecoveryState == DailyRecoveryAutoRecovered {
			t.Fatalf("uncertain result marked auto-recovered: result=%+v action=%+v", result, action)
		}
	}
}

func TestDailyLaneSelectionRequiresUserDecision(t *testing.T) {
	action := dailyLaneSelectionAction([]DailyChoice{{ID: "lane-a", Label: "Lane A"}, {ID: "lane-b", Label: "Lane B"}})
	if action == nil || !action.RequiresInput || action.RecoveryState != DailyRecoveryUserDecisionRequired || action.Now == "" || action.Reason == "" || action.Next == "" {
		t.Fatalf("lane selection action = %+v", action)
	}
	if len(action.Choices) != 2 || action.Choices[0].ID != "lane-a" || action.Choices[1].ID != "lane-b" {
		t.Fatalf("lane choices = %+v", action.Choices)
	}

	selectedBlocked := dailySelectedLaneBlockedAction(DailyChoice{ID: "lane-a", Label: "Lane A"})
	if selectedBlocked == nil || selectedBlocked.RecoveryState != DailyRecoveryUserDecisionRequired || selectedBlocked.RequiresInput || selectedBlocked.Now == "" || selectedBlocked.Reason == "" || selectedBlocked.Next == "" {
		t.Fatalf("selected blocked lane action = %+v", selectedBlocked)
	}
}

func TestDailyUserActionInputViewsUseStableChoiceIDs(t *testing.T) {
	tests := []struct {
		name      string
		action    *DailyUserAction
		code      string
		choiceIDs []string
	}{
		{name: "directory adoption", action: dailyAction(DailyActionDirectoryAdoptionRequired), code: DailyActionDirectoryAdoptionRequired, choiceIDs: []string{"initialize-in-place", "cancel"}},
		{name: "confirmation", action: dailyAction(DailyActionConfirmationRequired), code: DailyActionConfirmationRequired, choiceIDs: []string{"confirm-exact-plan", "cancel"}},
		{name: "evidence review", action: dailyAction(DailyActionReadyForEvidenceReview), code: DailyActionReadyForEvidenceReview, choiceIDs: []string{"review-evidence", "defer"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.action == nil || test.action.Code != test.code || !test.action.RequiresInput || len(test.action.Choices) != len(test.choiceIDs) {
				t.Fatalf("action = %+v", test.action)
			}
			for index, want := range test.choiceIDs {
				if test.action.Choices[index].ID != want {
					t.Fatalf("choice[%d] = %+v, want id %s", index, test.action.Choices[index], want)
				}
			}
		})
	}
}

func TestDailyResultActionIsAdditiveJSON(t *testing.T) {
	legacy := []byte(`{"schemaVersion":1,"command":"rekit-host-daily","mode":"resume","caseRoot":"case","finalState":"lane-closed","replay":true,"sessionLaunches":0,"sessionCompletions":0,"replacements":0,"boundary":[]}`)
	var result DailyResult
	if err := json.Unmarshal(legacy, &result); err != nil {
		t.Fatal(err)
	}
	if result.Action != nil || result.FinalState != "lane-closed" || !result.Replay {
		t.Fatalf("legacy DailyResult changed: %+v", result)
	}

	result.Action = dailyUserAction(result)
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip struct {
		Action *DailyUserAction `json:"action"`
	}
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if roundTrip.Action == nil || roundTrip.Action.Code != DailyActionReadyToContinue || roundTrip.Action.Now == "" || roundTrip.Action.Reason == "" || roundTrip.Action.Next == "" {
		t.Fatalf("round-trip action = %+v", roundTrip.Action)
	}

	legacyAction := []byte(`{"code":"failed","message":"legacy","requiresInput":false,"recovery":"retry the exact step"}`)
	var action DailyUserAction
	if err := json.Unmarshal(legacyAction, &action); err != nil {
		t.Fatal(err)
	}
	if action.Code != "failed" || action.Message != "legacy" || action.Recovery != "retry the exact step" || action.Now != "" || action.Reason != "" || action.Next != "" || action.RecoveryState != "" {
		t.Fatalf("legacy DailyUserAction changed: %+v", action)
	}
}
