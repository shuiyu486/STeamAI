package hostcmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionsuccessor"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/sessionhost"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestProjectDailyResultPublicResponseFailsClosedForDualStateRoots(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "dual-root-project")
	for _, stateDir := range []string{projectstate.CurrentDir, projectstate.LegacyDir} {
		if err := os.MkdirAll(filepath.Join(caseRoot, stateDir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	source := sessionhost.DailyResult{
		SchemaVersion: 1,
		CaseRoot:      caseRoot,
		Completion: &workstream.CompleteResult{
			ApplyCommand: "/rekit complete -Lane main -WhatIf -Format json",
		},
	}
	projected, err := projectDailyResultPublicResponse(source, caseRoot)
	if err == nil || !strings.Contains(err.Error(), "resolve daily public entrypoint") || projected.CaseRoot != "" {
		t.Fatalf("daily public projection did not fail closed: projected=%+v err=%v", projected, err)
	}
}

func TestProjectDailyResultPublicResponseUsesResolvedEntrypointWithoutMutatingDomain(t *testing.T) {
	for _, fixture := range []struct {
		name       string
		stateDir   string
		entrypoint string
	}{
		{name: "current", stateDir: projectstate.CurrentDir, entrypoint: commands.CurrentPublicEntrypoint},
		{name: "legacy", stateDir: projectstate.LegacyDir, entrypoint: commands.LegacyPublicEntrypoint},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			caseRoot := filepath.Join(t.TempDir(), "case")
			if err := os.MkdirAll(filepath.Join(caseRoot, fixture.stateDir), 0o755); err != nil {
				t.Fatal(err)
			}
			source := dailyPublicEntrypointFixture(t, caseRoot)
			before, err := json.Marshal(source)
			if err != nil {
				t.Fatal(err)
			}

			projected, err := projectDailyResultPublicResponse(source, caseRoot)
			if err != nil {
				t.Fatal(err)
			}
			after, err := json.Marshal(source)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("daily public projection mutated the domain result")
			}
			assertDailyPublicEntrypoint(t, projected, fixture.entrypoint)

			data, err := json.Marshal(projected)
			if err != nil {
				t.Fatal(err)
			}
			other := commands.LegacyPublicEntrypoint
			if fixture.entrypoint == commands.LegacyPublicEntrypoint {
				other = commands.CurrentPublicEntrypoint
			}
			if strings.Contains(string(data), other+" ") {
				t.Fatalf("daily public response mixed entrypoint %s: %s", other, data)
			}
		})
	}
}

func dailyPublicEntrypointFixture(t *testing.T, caseRoot string) sessionhost.DailyResult {
	t.Helper()
	requestCommand := "/rekit complete -Lane main -WhatIf -Format json"
	invocation, err := commands.ParsePublicInvocation(requestCommand)
	if err != nil {
		t.Fatal(err)
	}
	request := mission.MissionCommanderDriverRequest{
		Kind:              "preview-command",
		RunLoopStepID:     "complete-preview",
		Actor:             "mission-commander",
		State:             "ready-for-completion-preview",
		Source:            "daily-public-projection-test",
		Lane:              "main",
		Label:             "main",
		ActionID:          "complete:main",
		Invocation:        &invocation,
		Command:           requestCommand,
		CommandExecutable: true,
		RequiresReview:    true,
		ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
			State:                "refresh-required",
			Command:              requestCommand,
			RefreshStatusCommand: "/rekit status -Lane main -Format json",
		},
	}
	requestSHA, err := mission.MissionCommanderDriverRequestSHA256(request)
	if err != nil {
		t.Fatal(err)
	}
	action := mission.MissionCommanderNextActionItem{
		Lane:           "main",
		Label:          "main",
		ActionID:       "complete:main",
		State:          "ready-for-completion-preview",
		Invocation:     &invocation,
		Command:        requestCommand,
		Source:         "daily-public-projection-test",
		RequiresReview: true,
	}
	queue := mission.MissionCommanderActionQueueFor([]mission.MissionCommanderNextActionItem{action})
	completion := &workstream.CompleteResult{
		SchemaVersion:        1,
		Command:              "complete",
		CaseRoot:             caseRoot,
		Pack:                 "web-security",
		CompletionPlanSHA256: strings.Repeat("a", 64),
		ApplyCommand:         "/rekit complete -Lane main -Apply -ExpectedCompletePlanSha256 " + strings.Repeat("a", 64) + " -Format json",
		MissionBrief: mission.Brief{
			NextAgentActions: []string{requestCommand},
		},
		MissionCommanderNextActions: []mission.MissionCommanderNextActionItem{action},
		MissionCommanderActionQueue: queue,
		NextSteps:                   []string{"/rekit status -Lane main -Format json"},
	}
	return sessionhost.DailyResult{
		SchemaVersion:              1,
		Command:                    "rekit-host-daily",
		Mode:                       "resume",
		CaseRoot:                   caseRoot,
		Pack:                       "web-security",
		Lane:                       "main",
		FinalState:                 "lane-closed",
		Completion:                 completion,
		CurrentDriverRequest:       &request,
		CurrentDriverRequestSHA256: requestSHA,
		ExecutionControl: &executioncontrol.Plan{
			ApplyCommand: "/rekit control -Lane main -Action pause -Apply -ExpectedControlPlanSha256 " + strings.Repeat("b", 64) + " -Format json",
		},
		DirectoryAdoption: &sessionhost.DailyDirectoryAdoption{
			Plan: &syncreview.InitPlan{
				ApplyCommand: "/rekit init -Target project -Pack web-security -Apply -ExpectedInitPlanSha256 " + strings.Repeat("c", 64) + " -Format json",
			},
		},
		SuccessorMission: &missionsuccessor.Result{Plan: missionsuccessor.Plan{
			ApplyArgs: []string{"host", "-daily", "-target", caseRoot, "-goal", "successor"},
		}},
	}
}

func assertDailyPublicEntrypoint(t *testing.T, result sessionhost.DailyResult, entrypoint string) {
	t.Helper()
	if result.CurrentDriverRequest == nil || result.Completion == nil || result.ExecutionControl == nil ||
		result.DirectoryAdoption == nil || result.DirectoryAdoption.Plan == nil || result.SuccessorMission == nil {
		t.Fatalf("daily public projection omitted typed carriers: %+v", result)
	}
	request := result.CurrentDriverRequest
	if !strings.HasPrefix(request.Command, entrypoint+" complete ") ||
		!strings.HasPrefix(request.ExpectedReceipt.Command, entrypoint+" complete ") ||
		!strings.HasPrefix(request.ExpectedReceipt.RefreshStatusCommand, entrypoint+" status ") {
		t.Fatalf("daily current driver request uses mixed entrypoint: %+v", request)
	}
	parsed, err := commands.ParsePublicInvocation(request.Command)
	if err != nil || request.Invocation == nil || !request.Invocation.Equivalent(parsed) {
		t.Fatalf("daily current driver request command differs from typed invocation: %+v err=%v", request, err)
	}
	requestSHA, err := mission.MissionCommanderDriverRequestSHA256(*request)
	if err != nil || !strings.EqualFold(requestSHA, result.CurrentDriverRequestSHA256) {
		t.Fatalf("daily current driver request identity drifted: computed=%s stored=%s err=%v", requestSHA, result.CurrentDriverRequestSHA256, err)
	}
	completion := result.Completion
	if !strings.HasPrefix(completion.ApplyCommand, entrypoint+" complete ") ||
		len(completion.MissionBrief.NextAgentActions) != 1 || !strings.HasPrefix(completion.MissionBrief.NextAgentActions[0], entrypoint+" complete ") ||
		len(completion.MissionCommanderNextActions) != 1 || !strings.HasPrefix(completion.MissionCommanderNextActions[0].Command, entrypoint+" complete ") ||
		completion.MissionCommanderActionQueue.CurrentAction == nil || !strings.HasPrefix(completion.MissionCommanderActionQueue.CurrentAction.Command, entrypoint+" complete ") ||
		len(completion.NextSteps) != 1 || !strings.HasPrefix(completion.NextSteps[0], entrypoint+" status ") {
		t.Fatalf("daily completion uses mixed entrypoint: %+v", completion)
	}
	if strings.Contains(completion.MissionCommanderActionQueue.Summary, "/rekit ") && entrypoint != commands.LegacyPublicEntrypoint {
		t.Fatalf("daily completion queue summary retained legacy entrypoint: %s", completion.MissionCommanderActionQueue.Summary)
	}
	if !strings.HasPrefix(result.ExecutionControl.ApplyCommand, entrypoint+" control ") ||
		!strings.HasPrefix(result.DirectoryAdoption.Plan.ApplyCommand, entrypoint+" init ") {
		t.Fatalf("daily auxiliary exact commands use mixed entrypoint: control=%q adoption=%q", result.ExecutionControl.ApplyCommand, result.DirectoryAdoption.Plan.ApplyCommand)
	}
	if got := result.SuccessorMission.ApplyArgs; len(got) == 0 || got[0] != "host" {
		t.Fatalf("daily successor executable argv was projected as a public command: %v", got)
	}
}
