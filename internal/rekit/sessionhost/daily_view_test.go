package sessionhost

import (
	"encoding/json"
	"testing"
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
		{name: "lane completed", result: DailyResult{FinalState: "lane-closed"}, code: "completed"},
		{name: "member ready", result: DailyResult{FinalState: "member-intake-ready"}, code: "ready-to-continue"},
		{name: "reviewer rejected", result: DailyResult{FinalState: "reviewer-rejected-awaiting-correction", Blocked: true}, code: "waiting-for-correction", requiresInput: true, choiceIDs: []string{"provide-correction", "stop"}},
		{name: "typed blocker", result: DailyResult{FinalState: "attention-required", Blocked: true}, code: "blocked"},
		{name: "evidence review", result: DailyResult{FinalState: "ready-for-evidence-review"}, code: "ready-for-evidence-review", requiresInput: true, choiceIDs: []string{"review-evidence", "defer"}},
		{name: "unknown state fails closed", result: DailyResult{FinalState: "future-state"}, code: "blocked"},
		{name: "typed failure wins", result: DailyResult{FinalState: "mission-complete", Failure: &FailureDiagnosis{Code: "claude-timeout", Recoverable: true}}, code: "failed"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action := dailyUserAction(test.result)
			if action == nil || action.Code != test.code || action.RequiresInput != test.requiresInput {
				t.Fatalf("dailyUserAction(%+v) = %+v", test.result, action)
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
	if roundTrip.Action == nil || roundTrip.Action.Code != "completed" {
		t.Fatalf("round-trip action = %+v", roundTrip.Action)
	}
}
