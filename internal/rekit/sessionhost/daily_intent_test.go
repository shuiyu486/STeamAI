package sessionhost

import (
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
)

func TestResolveDailyRequestTreatsLaneAsSelectorNotOperation(t *testing.T) {
	for _, fixture := range []struct {
		name      string
		options   DailyOptions
		operation DailyOperation
		control   bool
		adoption  bool
	}{
		{name: "resume", options: DailyOptions{SelectedLane: "binary-analysis-main"}, operation: DailyOperationResume},
		{name: "goal", options: DailyOptions{Goal: "inspect the target", SelectedLane: "binary-analysis-main"}, operation: DailyOperationGoal},
		{name: "correction", options: DailyOptions{Correction: "recheck the evidence", SelectedLane: "binary-analysis-main"}, operation: DailyOperationCorrection},
		{name: "control", options: DailyOptions{SelectedLane: "binary-analysis-main", ControlWhatIf: true, Control: executioncontrol.Options{Action: executioncontrol.ActionPause, Reason: "bounded pause"}}, operation: DailyOperationControl, control: true},
		{name: "adoption", options: DailyOptions{DirectoryAdoptionAction: "initialize-in-place"}, operation: DailyOperationAdoption, adoption: true},
		{name: "adoption-pack", options: DailyOptions{DirectoryAdoptionPack: "web-security"}, operation: DailyOperationAdoption, adoption: true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			request, err := ResolveDailyRequest(fixture.options)
			if err != nil {
				t.Fatal(err)
			}
			if request.Operation != fixture.operation || request.ControlRequested != fixture.control ||
				request.AdoptionRequested != fixture.adoption {
				t.Fatalf("ResolveDailyRequest() = %+v", request)
			}
		})
	}
}

func TestResolveDailyRequestRejectsCrossOperationFields(t *testing.T) {
	for _, fixture := range []struct {
		name    string
		options DailyOptions
		want    string
	}{
		{name: "goal-correction", options: DailyOptions{Goal: "goal", Correction: "correction"}, want: "either -goal or -correction"},
		{name: "goal-control", options: DailyOptions{Goal: "goal", ControlWhatIf: true, Control: executioncontrol.Options{Action: executioncontrol.ActionPause}}, want: "control cannot be combined"},
		{name: "control-adoption", options: DailyOptions{ControlWhatIf: true, Control: executioncontrol.Options{Action: executioncontrol.ActionPause}, DirectoryAdoptionAction: "initialize-in-place"}, want: "control cannot be combined with directory adoption"},
		{name: "control-lane-second-owner", options: DailyOptions{ControlWhatIf: true, Control: executioncontrol.Options{Lane: "binary-analysis-main", Action: executioncontrol.ActionPause}}, want: "control lane must use the daily selected lane"},
		{name: "control-actor-second-owner", options: DailyOptions{ControlWhatIf: true, Control: executioncontrol.Options{Actor: "another-owner", Action: executioncontrol.ActionPause}}, want: "control actor must use the daily actor"},
		{name: "input-control", options: DailyOptions{Input: DailyInputRequest{Mode: DailyInputWorkspaceInventory}, ControlWhatIf: true, Control: executioncontrol.Options{Action: executioncontrol.ActionPause}}, want: "typed daily input cannot be combined with lane control"},
		{name: "input-adoption", options: DailyOptions{Input: DailyInputRequest{Mode: DailyInputWorkspaceInventory}, DirectoryAdoptionAction: "initialize-in-place"}, want: "typed daily input cannot be combined with directory adoption"},
		{name: "input-correction", options: DailyOptions{Input: DailyInputRequest{Mode: DailyInputWorkspaceInventory}, Correction: "recheck"}, want: "typed daily input cannot be combined with -correction"},
		{name: "input-successor", options: DailyOptions{Input: DailyInputRequest{Mode: DailyInputWorkspaceInventory}, SuccessorWhatIf: true, Goal: "successor goal"}, want: "typed daily input cannot be combined with successor mission controls"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			_, err := ResolveDailyRequest(fixture.options)
			if err == nil || !strings.Contains(err.Error(), fixture.want) {
				t.Fatalf("ResolveDailyRequest() error = %v, want %q", err, fixture.want)
			}
		})
	}
}
