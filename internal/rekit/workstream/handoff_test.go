package workstream

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestWriteCurrentLoopOperatorPackageIncludesExternalMemberHandoff(t *testing.T) {
	checkpointSHA256 := strings.Repeat("c", 64)
	attemptID := "g000002-a000001-0123456789abcdef"
	preview := `/rekit run-current-loop -ResumeCurrentLoop -ExpectedCurrentLoopCheckpointSha256 ` + checkpointSHA256 + ` -MemberExecutionAttemptId ` + attemptID + ` -MemberExecutionOutcome returned -WhatIf -Format json`
	contract := mission.CurrentLoopObservationContract{
		Alternatives: []mission.CurrentLoopObservationAlternative{{
			Kind:                   "member-session-returned",
			RequiredFlags:          []string{"-MemberExecutionAttemptId", "-MemberExecutionOutcome"},
			PreviewCommandTemplate: preview,
			Transition:             "accepted-to-returned",
			Constraints:            []string{"strict manifest required"},
		}},
	}
	pkg := &mission.CurrentLoopOperatorPackage{
		State: "observation-inbox-review-required",
		Route: "case",
		Lane:  "main",
		ObservationInbox: &mission.CurrentLoopObservationInbox{
			State: "ready", Path: ".rekit/external-session-observations/inbox", CandidateCount: 1, MatchingCount: 1,
			SelectedCandidate: &mission.CurrentLoopObservationInboxCandidate{Path: ".rekit/external-session-observations/inbox/member.json", SHA256: strings.Repeat("d", 64), ObservationKind: "member-session-returned", Actor: "harness"},
		},
		ObservationReceipt: &mission.CurrentLoopObservationReceipt{State: "processed", SourceCheckpointSHA256: strings.Repeat("a", 64), SuccessorCheckpointSHA256: strings.Repeat("b", 64), ObservationPath: ".rekit/external-session-observations/inbox/prior.json", ObservationSHA256: strings.Repeat("e", 64), ObservationKind: "member-session-accepted", Actor: "harness"},
		ExternalSessionJob: &mission.CurrentLoopExternalSessionJob{
			State: "awaiting-submission", SessionKind: "member", JobID: "member-job", JobSHA256: strings.Repeat("f", 64), CheckpointSHA256: checkpointSHA256,
			SubmissionPath: ".rekit/external-session-attempt-inputs/member-job/000001/submission.json", AllowedOutcomes: []string{"returned", "failed"}, SubmissionLast: true,
			HarnessPackage: &mission.CurrentLoopExternalSessionHarnessPackage{
				State: "running", SessionKind: "member", JobID: "member-job", JobSHA256: strings.Repeat("f", 64), RefreshStatusCommand: "/rekit status -Format json",
				Launch: &mission.CurrentLoopExternalSessionHarnessLaunch{Ready: true, Tool: "Claude Code session", AgentType: "durable-member-executor", Input: mission.CurrentLoopExternalSessionHarnessInput{Role: "durable-member-handoff", Path: ".rekit/member/handoff.json", SHA256: strings.Repeat("1", 64)}, Attempt: mission.CurrentLoopExternalSessionAttempt{AttemptSHA256: strings.Repeat("2", 64), Generation: 1, Harness: "harness", Session: "session"}},
				Return: &mission.CurrentLoopExternalSessionReturnContract{SubmissionPath: ".rekit/external-session-attempt-inputs/member-job/000001/submission.json", SubmissionOutputs: ".rekit/external-session-attempt-inputs/member-job/000001/outputs", SubmissionLast: true, Templates: []mission.CurrentLoopExternalSessionSubmissionTemplate{{Outcome: "returned", JSON: "{\n  \"outcome\": \"returned\",\n  \"attemptSha256\": \"" + strings.Repeat("2", 64) + "\"\n}\n", RequiredWrites: []string{".rekit/external-session-attempt-inputs/member-job/000001/outputs/**", ".rekit/external-session-attempt-inputs/member-job/000001/submission.json (last)"}, RequiredReplace: []string{"<actor>", "<summary>"}}, {Outcome: "failed", JSON: "{\n  \"outcome\": \"failed\"\n}\n", RequiredWrites: []string{".rekit/external-session-attempt-inputs/member-job/000001/submission.json (last)"}, RequiredReplace: []string{"<actor>", "<reason>"}}}},
			},
		},
		ExternalMemberHandoff: &mission.CurrentLoopExternalMemberHandoff{
			State:               "accepted",
			AttemptID:           attemptID,
			Lane:                "main",
			Executor:            "member-session",
			ExecutorGeneration:  2,
			HandoffPath:         ".rekit/lanes/main/member-executions/" + attemptID + "/handoff.json",
			ManifestPath:        ".rekit/lanes/main/member-executions/" + attemptID + "/result/manifest.json",
			OutputsRoot:         ".rekit/lanes/main/member-executions/" + attemptID + "/result/outputs",
			ObservationContract: contract,
		},
	}
	var out bytes.Buffer
	writeCurrentLoopOperatorPackage(&out, pkg)
	for _, want := range []string{"## Current-loop operator", "observation inbox: state=ready", "selected inbox observation: kind=member-session-returned actor=harness", "observation receipt: state=processed kind=member-session-accepted", "harness package: state=running kind=member", "harness launch: ready=true tool=Claude Code session agentType=durable-member-executor", "inputSha256=" + strings.Repeat("1", 64), "harness return: submission=", "templates=2", "harness submission template: outcome=returned requiredWrites=.rekit/external-session-attempt-inputs/member-job/000001/outputs/**; .rekit/external-session-attempt-inputs/member-job/000001/submission.json (last) requiredReplacements=<actor>; <summary>", "```json", "\"attemptSha256\": \"" + strings.Repeat("2", 64) + "\"", "harness submission template: outcome=failed", "external member: state=accepted attempt=" + attemptID, "owner=member-session/2", "member observation: kind=member-session-returned transition=accepted-to-returned", checkpointSHA256, "strict manifest required"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("current-loop operator Markdown missing %q:\n%s", want, out.String())
		}
	}
}

func TestHandoffReplacementExecutorTakeoverIncludesDispatcherRunbookAtConstruction(t *testing.T) {
	queueRequest := mission.MissionCommanderDriverRequest{
		Kind: "execute-command", State: "ready-to-continue", Source: "missionCommanderActions",
		Command: "/rekit continue -Lane main", CommandExecutable: true,
	}
	selectedRequest := mission.MissionCommanderDriverRequest{
		Kind: "preview-command-template", State: "ready-for-dispatch-claim-review", Source: "current-loop-external-session-dispatch",
		Command: "/rekit run-current-loop -ClaimExternalSessionDispatch -WhatIf -Format json", RequiresReview: true,
	}
	queue := mission.MissionCommanderActionQueue{CurrentDriverRequest: &queueRequest}
	for _, test := range []struct {
		name        string
		state       string
		preferQueue bool
		wantStep    string
		wantCommand string
	}{
		{name: "queued-selected", state: "queued", wantStep: "consume dispatcher.claimRequest before any launch", wantCommand: selectedRequest.Command},
		{name: "claimed-selected", state: "claimed", wantStep: "actual launch is not yet recorded", wantCommand: selectedRequest.Command},
		{name: "launch-failed-selected", state: "launch-failed", wantStep: "launch failed", wantCommand: selectedRequest.Command},
		{name: "running-queue", state: "running", wantStep: "actually running under dispatcher.launchReceipt", wantCommand: queueRequest.Command},
		{name: "queued-explicit-queue", state: "queued", preferQueue: true, wantStep: "consume dispatcher.claimRequest before any launch", wantCommand: queueRequest.Command},
		{name: "claimed-explicit-queue", state: "claimed", preferQueue: true, wantStep: "actual launch is not yet recorded", wantCommand: queueRequest.Command},
		{name: "launch-failed-explicit-queue", state: "launch-failed", preferQueue: true, wantStep: "launch failed", wantCommand: queueRequest.Command},
	} {
		t.Run(test.name, func(t *testing.T) {
			operator := &mission.CurrentLoopOperatorPackage{
				SelectedDriverRequest: &selectedRequest,
				ExternalSessionJob: &mission.CurrentLoopExternalSessionJob{
					State:      "awaiting-submission",
					Dispatcher: &mission.CurrentLoopExternalSessionDispatcher{State: test.state},
				},
			}
			pkg, err := handoffReplacementExecutorTakeoverPackage(
				t.TempDir(), "project", nil, queue, nil, nil,
				".rekit/handovers/latest-replacement-executor-takeover.json", operator, test.preferQueue,
			)
			if err != nil {
				t.Fatal(err)
			}
			if pkg == nil || pkg.CurrentLoopOperator != operator || pkg.CurrentDriverRequest.Command != test.wantCommand || pkg.CurrentDriverRequestSHA256 != mission.ReplacementExecutorDriverRequestSHA256(pkg.CurrentDriverRequest) || !slices.ContainsFunc(pkg.RunbookSteps, func(step string) bool {
				return strings.Contains(step, test.wantStep)
			}) {
				t.Fatalf("dispatcher %s was not part of takeover construction: %+v", test.state, pkg)
			}
		})
	}
}

func TestHandoffContextInjectsValidatedFinalDriverRequest(t *testing.T) {
	repoRoot, caseRoot := setupContinueCase(t, "")
	request := handoffFinalDriverRequestForTest(t, caseRoot, "devirt-main")
	opt := HandoffOptions{Selector: "devirt-main", CurrentDriverRequest: &request}

	ctx, err := newHandoffContext(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	request.Command = "/rekit status -Format json"
	request.Invocation.Arguments[1] = t.TempDir()
	request.Boundary[0] = "mutated"
	request.ExpectedReceipt.Boundary[0] = "mutated"

	result, err := ctx.result(false, false, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	queue := result.MissionCommanderActionQueue
	got := queue.CurrentDriverRequest
	if got == nil || got.Command != `/rekit continue -Target "`+caseRoot+`" -Lane devirt-main -WhatIf -Format json` || got.Invocation == nil || got.Invocation.Arguments[1] != caseRoot || got.Boundary[0] == "mutated" || got.ExpectedReceipt.Boundary[0] == "mutated" {
		t.Fatalf("handoff final request was not deep-cloned into the action queue: %+v", got)
	}
	assertHandoffQueueCurrentIdentity(t, "result queue", queue, got)
	assertSameHandoffDriverRequest(t, "runbook", got, result.DailyMissionControlRunbook.CurrentDriverRequest)
	if result.DailyMissionControlRunbook.RefreshStatusCommand != got.ExpectedReceipt.RefreshStatusCommand {
		t.Fatalf("handoff runbook refresh command drifted: %+v", result.DailyMissionControlRunbook)
	}
	pkg := result.ReplacementExecutorTakeoverPackage
	if pkg == nil {
		t.Fatalf("handoff takeover package is missing")
	}
	assertSameHandoffDriverRequest(t, "takeover", got, &pkg.CurrentDriverRequest)
	if pkg.RefreshStatusCommand != got.ExpectedReceipt.RefreshStatusCommand {
		t.Fatalf("handoff takeover refresh command drifted: %+v", pkg)
	}
	gotSHA, err := mission.MissionCommanderDriverRequestSHA256(*got)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.CurrentDriverRequestSHA256 != gotSHA {
		t.Fatalf("handoff takeover request SHA drifted: got=%s want=%s", pkg.CurrentDriverRequestSHA256, gotSHA)
	}
	resumePublications, err := ctx.buildResumePublications()
	if err != nil {
		t.Fatal(err)
	}
	if len(resumePublications) != 1 || !strings.Contains(string(resumePublications[0].ResumeBytes), "command=`"+got.Command+"`") || !strings.Contains(string(resumePublications[0].ResumeBytes), "refreshStatusCommand=`"+got.ExpectedReceipt.RefreshStatusCommand+"`") {
		t.Fatalf("handoff RESUME publication did not preserve the final request identity: %+v", resumePublications)
	}
	var checkpoint struct {
		MissionCommanderActionQueue mission.MissionCommanderActionQueue `json:"missionCommanderActionQueue"`
	}
	if err := json.Unmarshal(resumePublications[0].CheckpointBytes, &checkpoint); err != nil {
		t.Fatalf("handoff checkpoint publication did not decode: %v", err)
	}
	checkpointRequest := checkpoint.MissionCommanderActionQueue.CurrentDriverRequest
	if checkpointRequest == nil || checkpointRequest.Command != got.Command || checkpointRequest.ExpectedReceipt.RefreshStatusCommand != got.ExpectedReceipt.RefreshStatusCommand {
		t.Fatalf("handoff checkpoint publication did not preserve the final request identity: %+v", checkpointRequest)
	}
}

func TestProjectHandoffInjectsFinalRequestOnlyIntoMatchingLaneResume(t *testing.T) {
	repoRoot, caseRoot := setupContinueCase(t, "")
	other, err := StartApply(repoRoot, caseRoot, defaults.DefaultPack, StartOptions{Selector: "analysis-other"})
	if err != nil {
		t.Fatal(err)
	}
	request := handoffFinalDriverRequestForTest(t, caseRoot, "devirt-main")
	ctx, err := newHandoffContext(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{CurrentDriverRequest: &request})
	if err != nil {
		t.Fatal(err)
	}
	publications, err := ctx.buildResumePublications()
	if err != nil {
		t.Fatal(err)
	}
	if len(publications) < 2 {
		t.Fatalf("project handoff resume publication count=%d want at least 2", len(publications))
	}
	seenMatching := false
	seenOther := false
	for _, publication := range publications {
		var checkpoint struct {
			Lane                        string                              `json:"lane"`
			MissionCommanderActionQueue mission.MissionCommanderActionQueue `json:"missionCommanderActionQueue"`
		}
		if err := json.Unmarshal(publication.CheckpointBytes, &checkpoint); err != nil {
			t.Fatal(err)
		}
		if checkpoint.Lane == "devirt-main" {
			seenMatching = true
			assertSameHandoffDriverRequest(t, "matching lane checkpoint", ctx.currentDriverRequest, checkpoint.MissionCommanderActionQueue.CurrentDriverRequest)
			assertHandoffQueueCurrentIdentity(t, "matching lane checkpoint", checkpoint.MissionCommanderActionQueue, checkpoint.MissionCommanderActionQueue.CurrentDriverRequest)
			continue
		}
		if checkpoint.Lane == other.Lane.ID {
			seenOther = true
		}
		if checkpoint.MissionCommanderActionQueue.CurrentDriverRequest != nil && checkpoint.MissionCommanderActionQueue.CurrentDriverRequest.Command == request.Command {
			t.Fatalf("project handoff copied one lane request into %s: %+v", checkpoint.Lane, checkpoint)
		}
	}
	if !seenMatching || !seenOther {
		t.Fatalf("project handoff resume publications omitted lanes: matching=%t other=%t", seenMatching, seenOther)
	}
	markdown, _, err := ctx.renderProject(false)
	if err != nil {
		t.Fatal(err)
	}
	otherSection := projectHandoffLaneSectionForTest(markdown, other.Lane.ID)
	if strings.Contains(otherSection, "command=`"+request.Command+"`") || strings.Contains(otherSection, "refreshStatusCommand=`"+request.ExpectedReceipt.RefreshStatusCommand+"`") {
		t.Fatalf("project handoff copied one lane request into non-matching lane Markdown:\n%s", otherSection)
	}
}

func TestProjectHandoffCurrentLoopOperatorStaysInExactLane(t *testing.T) {
	repoRoot, caseRoot := setupContinueCase(t, "")
	other, err := StartApply(repoRoot, caseRoot, defaults.DefaultPack, StartOptions{Selector: "analysis-other"})
	if err != nil {
		t.Fatal(err)
	}
	request := handoffFinalDriverRequestForTest(t, caseRoot, "devirt-main")
	operatorRequest := request
	operatorRequest.Source = "currentLoopOperator.selectedDriverRequest"
	operator := &mission.CurrentLoopOperatorPackage{
		Ready:                      true,
		State:                      "fresh-loop-review-required",
		CaseRoot:                   caseRoot,
		Route:                      "case",
		Lane:                       "devirt-main",
		SourceCurrentDriverRequest: &operatorRequest,
		SelectedDriverRequest:      &operatorRequest,
		RunbookSteps:               []string{"consume exact lane operator"},
	}
	ctx, err := newHandoffContext(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{CurrentDriverRequest: &request, CurrentLoopOperator: operator})
	if err != nil {
		t.Fatal(err)
	}
	markdown, _, err := ctx.renderProject(false)
	if err != nil {
		t.Fatal(err)
	}
	matchingSection := projectHandoffLaneSectionForTest(markdown, "devirt-main")
	otherSection := projectHandoffLaneSectionForTest(markdown, other.Lane.ID)
	if !strings.Contains(matchingSection, "## Current-loop operator") || !strings.Contains(matchingSection, operatorRequest.Command) {
		t.Fatalf("matching lane omitted current-loop operator:\n%s", matchingSection)
	}
	if strings.Contains(otherSection, "## Current-loop operator") || strings.Contains(otherSection, operatorRequest.Command) {
		t.Fatalf("non-matching lane received current-loop operator:\n%s", otherSection)
	}

	assertOperatorDriftOmitted := func(name string, mutate func(*mission.CurrentLoopOperatorPackage)) {
		t.Helper()
		candidate := *operator
		mutate(&candidate)
		ctx.currentLoopOperator = &candidate
		markdown, _, err = ctx.renderProject(false)
		if err != nil {
			t.Fatal(err)
		}
		matchingSection = projectHandoffLaneSectionForTest(markdown, "devirt-main")
		if strings.Contains(matchingSection, "## Current-loop operator") {
			t.Fatalf("%s lane drift was partially published:\n%s", name, matchingSection)
		}
	}
	drifted := operatorRequest
	drifted.Lane = other.Lane.ID
	assertOperatorDriftOmitted("direct nested request", func(candidate *mission.CurrentLoopOperatorPackage) {
		candidate.ResumeDriverRequest = &drifted
	})
	assertOperatorDriftOmitted("reviewer attempt current request", func(candidate *mission.CurrentLoopOperatorPackage) {
		candidate.ExternalReviewerHandoff = &mission.CurrentLoopExternalReviewerHandoff{
			Attempt: &mission.CurrentLoopReviewerAttempt{CurrentReviewerDriverRequest: &drifted},
		}
	})
	assertOperatorDriftOmitted("reviewer attempt durable continuation", func(candidate *mission.CurrentLoopOperatorPackage) {
		candidate.ExternalReviewerHandoff = &mission.CurrentLoopExternalReviewerHandoff{
			Attempt: &mission.CurrentLoopReviewerAttempt{DurableContinuationDriverRequest: &drifted},
		}
	})
	assertOperatorDriftOmitted("reviewer wave shard", func(candidate *mission.CurrentLoopOperatorPackage) {
		candidate.ExternalReviewerHandoff = &mission.CurrentLoopExternalReviewerHandoff{
			Wave: &mission.CurrentLoopReviewerWave{
				Lane: "devirt-main",
				Shards: []*mission.CurrentLoopReviewerAttempt{{
					Identity:                     mission.CurrentLoopReviewerAttemptIdentity{Lane: "devirt-main"},
					CurrentReviewerDriverRequest: &drifted,
				}},
			},
		}
	})
	assertOperatorDriftOmitted("reviewer attempt identity", func(candidate *mission.CurrentLoopOperatorPackage) {
		candidate.ExternalReviewerHandoff = &mission.CurrentLoopExternalReviewerHandoff{
			Attempt: &mission.CurrentLoopReviewerAttempt{Identity: mission.CurrentLoopReviewerAttemptIdentity{Lane: other.Lane.ID}},
		}
	})
	assertOperatorDriftOmitted("reviewer wave identity", func(candidate *mission.CurrentLoopOperatorPackage) {
		candidate.ExternalReviewerHandoff = &mission.CurrentLoopExternalReviewerHandoff{
			Wave: &mission.CurrentLoopReviewerWave{Lane: other.Lane.ID},
		}
	})
	assertOperatorDriftOmitted("external member identity", func(candidate *mission.CurrentLoopOperatorPackage) {
		candidate.ExternalMemberHandoff = &mission.CurrentLoopExternalMemberHandoff{Lane: other.Lane.ID}
	})
	assertOperatorDriftOmitted("external session member owner", func(candidate *mission.CurrentLoopOperatorPackage) {
		candidate.ExternalSessionJob = &mission.CurrentLoopExternalSessionJob{
			MemberOwner: &mission.CurrentLoopExternalSessionJobOwner{Lane: other.Lane.ID},
		}
	})
}

func TestLaneHandoffResultFiltersCurrentLoopOperator(t *testing.T) {
	repoRoot, caseRoot := setupContinueCase(t, "")
	request := handoffFinalDriverRequestForTest(t, caseRoot, "devirt-main")
	operatorRequest := request
	operatorRequest.Source = "currentLoopOperator.selectedDriverRequest"
	operator := &mission.CurrentLoopOperatorPackage{
		Ready:                      true,
		State:                      "fresh-loop-review-required",
		CaseRoot:                   caseRoot,
		Route:                      "case",
		Lane:                       "devirt-main",
		SourceCurrentDriverRequest: &operatorRequest,
		SelectedDriverRequest:      &operatorRequest,
	}
	ctx, err := newHandoffContext(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{
		Selector:             "devirt-main",
		CurrentDriverRequest: &request,
		CurrentLoopOperator:  operator,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := ctx.result(false, false, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.CurrentLoopOperator != operator || result.ReplacementExecutorTakeoverPackage == nil || result.ReplacementExecutorTakeoverPackage.CurrentLoopOperator != operator {
		t.Fatalf("matching lane result omitted current-loop operator: %+v", result)
	}

	drifted := operatorRequest
	drifted.Lane = "analysis-other"
	operator.ExternalReviewerHandoff = &mission.CurrentLoopExternalReviewerHandoff{
		Wave: &mission.CurrentLoopReviewerWave{
			Lane: "devirt-main",
			Active: []*mission.CurrentLoopReviewerAttempt{{
				DurableContinuationDriverRequest: &drifted,
			}},
		},
	}
	ctx.currentLoopOperator = operator
	result, err = ctx.result(false, false, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.CurrentLoopOperator != nil || result.ReplacementExecutorTakeoverPackage == nil || result.ReplacementExecutorTakeoverPackage.CurrentLoopOperator != nil {
		t.Fatalf("lane result published cross-lane operator: result=%+v package=%+v", result.CurrentLoopOperator, result.ReplacementExecutorTakeoverPackage)
	}
}

func TestHandoffContextRejectsInvalidFinalDriverRequest(t *testing.T) {
	repoRoot, caseRoot := setupContinueCase(t, "")
	other, err := StartApply(repoRoot, caseRoot, defaults.DefaultPack, StartOptions{Selector: "analysis-other"})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name      string
		selector  string
		mutate    func(*mission.MissionCommanderDriverRequest)
		wantError string
	}{
		{
			name: "missing-target",
			mutate: func(request *mission.MissionCommanderDriverRequest) {
				setHandoffRequestCommandForTest(t, request, `/rekit continue -Lane devirt-main -WhatIf -Format json`)
			},
			wantError: "-Target must bind",
		},
		{
			name: "request-lane-differs-from-invocation",
			mutate: func(request *mission.MissionCommanderDriverRequest) {
				request.Lane = other.Lane.ID
			},
			wantError: "bind one exact -Lane to request.lane",
		},
		{
			name: "refresh-lane-differs",
			mutate: func(request *mission.MissionCommanderDriverRequest) {
				request.ExpectedReceipt.RefreshStatusCommand = `/rekit status -Target "` + caseRoot + `" -Lane ` + other.Lane.ID + ` -Format compact-json`
			},
			wantError: "status refresh command must bind the exact",
		},
		{
			name:     "resolved-handoff-lane-differs",
			selector: "devirt-main",
			mutate: func(request *mission.MissionCommanderDriverRequest) {
				*request = handoffFinalDriverRequestForTest(t, caseRoot, other.Lane.ID)
			},
			wantError: "differs from resolved handoff lane",
		},
		{
			name: "refresh-target-differs",
			mutate: func(request *mission.MissionCommanderDriverRequest) {
				request.ExpectedReceipt.RefreshStatusCommand = `/rekit status -Target "` + t.TempDir() + `" -Lane devirt-main -Format compact-json`
			},
			wantError: "status refresh command must bind -Target",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := handoffFinalDriverRequestForTest(t, caseRoot, "devirt-main")
			test.mutate(&request)
			before := snapshotWorkstreamTree(t, caseRoot)
			selector := test.selector
			if selector == "" {
				selector = "devirt-main"
			}
			_, err := HandoffPreview(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{Selector: selector, CurrentDriverRequest: &request})
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("invalid final request error = %v; want %q", err, test.wantError)
			}
			if after := snapshotWorkstreamTree(t, caseRoot); after != before {
				t.Fatalf("invalid final request changed workstream state")
			}
		})
	}
}

func TestHandoffApplyRejectsFinalDriverRequestDriftWithoutWrites(t *testing.T) {
	repoRoot, caseRoot := setupContinueCase(t, "")
	request := handoffFinalDriverRequestForTest(t, caseRoot, "devirt-main")
	preview, err := HandoffPreview(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{Selector: "devirt-main", CurrentDriverRequest: &request})
	if err != nil {
		t.Fatal(err)
	}
	before := snapshotWorkstreamTree(t, caseRoot)
	drifted := handoffFinalDriverRequestForTest(t, caseRoot, "devirt-main")
	drifted.Boundary = append(drifted.Boundary, "fresh request drift")
	_, err = HandoffApply(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{
		Selector:                      "devirt-main",
		ExpectedPublicationPlanSHA256: preview.PublicationPlanSHA256,
		PublicationStamp:              preview.PublicationStamp,
		CurrentDriverRequest:          &drifted,
	})
	if err == nil || !strings.Contains(err.Error(), "publication plan sha256 mismatch") {
		t.Fatalf("drifted final request error = %v", err)
	}
	if after := snapshotWorkstreamTree(t, caseRoot); after != before {
		t.Fatalf("drifted final request wrote handoff state")
	}
}

func handoffFinalDriverRequestForTest(t *testing.T, caseRoot, lane string) mission.MissionCommanderDriverRequest {
	t.Helper()
	command := `/rekit continue -Target "` + caseRoot + `" -Lane ` + lane + ` -WhatIf -Format json`
	invocation, err := commands.ParsePublicInvocation(command)
	if err != nil {
		t.Fatal(err)
	}
	refresh := `/rekit status -Target "` + caseRoot + `" -Lane ` + lane + ` -Format compact-json`
	return mission.MissionCommanderDriverRequest{
		Kind:              "preview-command",
		RunLoopStepID:     "preview-current",
		Actor:             "main-agent",
		State:             "ready-to-continue",
		Source:            "missionCommanderActions",
		Lane:              lane,
		Label:             lane,
		Invocation:        &invocation,
		Command:           command,
		CommandExecutable: true,
		RequiresReview:    true,
		ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
			State:                "refresh-required",
			Command:              command,
			RefreshStatusCommand: refresh,
			Boundary:             []string{"refresh durable state"},
		},
		Boundary: []string{"read-only request envelope"},
	}
}

func projectHandoffLaneSectionForTest(markdown, laneID string) string {
	marker := "`" + laneID + "`："
	start := strings.Index(markdown, marker)
	if start < 0 {
		return ""
	}
	section := markdown[start:]
	end := len(section)
	for _, next := range []string{"\n- 主线 `", "\n- 功能支线 `", "\n## 注意边界"} {
		if index := strings.Index(section[1:], next); index >= 0 && index+1 < end {
			end = index + 1
		}
	}
	return section[:end]
}

func setHandoffRequestCommandForTest(t *testing.T, request *mission.MissionCommanderDriverRequest, command string) {
	t.Helper()
	invocation, err := commands.ParsePublicInvocation(command)
	if err != nil {
		t.Fatal(err)
	}
	request.Invocation = &invocation
	request.Command = command
	request.ExpectedReceipt.Command = command
}

func assertHandoffQueueCurrentIdentity(t *testing.T, surface string, queue mission.MissionCommanderActionQueue, request *mission.MissionCommanderDriverRequest) {
	t.Helper()
	if request == nil || queue.CurrentAction == nil {
		t.Fatalf("%s current identity is missing: %+v", surface, queue)
	}
	if queue.CurrentAction.Command != request.Command || queue.CurrentAction.Invocation == nil || request.Invocation == nil || !queue.CurrentAction.Invocation.Equivalent(*request.Invocation) || queue.CurrentAction.Lane != request.Lane || queue.CurrentAction.Source != request.Source || queue.CurrentAction.State != request.State || queue.CurrentAction.Blocked != request.Blocked || queue.CurrentAction.RequiresReview != request.RequiresReview {
		t.Fatalf("%s current action differs from final request: action=%+v request=%+v", surface, queue.CurrentAction, request)
	}
	if queue.CurrentRunLoopStepID != request.RunLoopStepID || len(queue.CurrentActionRunLoop) != 1 {
		t.Fatalf("%s current run loop differs from final request: %+v", surface, queue)
	}
	step := queue.CurrentActionRunLoop[0]
	if step.StepID != request.RunLoopStepID || step.Command != request.Command || step.Actor != request.Actor || step.State != request.State || step.Source != request.Source {
		t.Fatalf("%s current run-loop step differs from final request: step=%+v request=%+v", surface, step, request)
	}
}

func assertSameHandoffDriverRequest(t *testing.T, surface string, want, got *mission.MissionCommanderDriverRequest) {
	t.Helper()
	if want == nil || got == nil {
		t.Fatalf("%s request missing: want=%+v got=%+v", surface, want, got)
	}
	if !reflect.DeepEqual(*got, *want) {
		t.Fatalf("%s request structure drifted:\nwant=%+v\ngot=%+v", surface, want, got)
	}
	wantSHA, err := mission.MissionCommanderDriverRequestSHA256(*want)
	if err != nil {
		t.Fatal(err)
	}
	gotSHA, err := mission.MissionCommanderDriverRequestSHA256(*got)
	if err != nil {
		t.Fatal(err)
	}
	if gotSHA != wantSHA {
		t.Fatalf("%s request SHA drifted: got=%s want=%s", surface, gotSHA, wantSHA)
	}
}

func TestMissionCommanderNextActionMarkdownLineIncludesIdentity(t *testing.T) {
	line := MissionCommanderNextActionMarkdownLine(mission.MissionCommanderNextActionItem{
		Lane:           "main",
		Label:          "Main",
		GateEventID:    "evt-adapter",
		ActionID:       "evt-adapter:adapter-report-record",
		State:          "ready-to-record-evidence",
		Source:         "adapterReportValidation.missionCommanderAction",
		Command:        "/rekit gate -Apply -GateEventId evt-adapter",
		RequiresReview: true,
	})
	for _, want := range []string{"state=ready-to-record-evidence source=adapterReportValidation.missionCommanderAction", "command=`/rekit gate -Apply -GateEventId evt-adapter` lane=main label=Main", "gateEventId=evt-adapter", "actionId=evt-adapter:adapter-report-record"} {
		if !strings.Contains(line, want) {
			t.Fatalf("markdown line missing %q: %s", want, line)
		}
	}
}

func TestMissionCommanderActionRunLoopMarkdownLinesIncludesCurrentStep(t *testing.T) {
	queue := mission.MissionCommanderActionQueueFor([]mission.MissionCommanderNextActionItem{
		{Lane: "main", Label: "main", State: "needs-gate-decision", Command: "/rekit gate -Action debug -Lane main -WhatIf", Source: "missionCommanderActions", RequiresReview: true},
		{Lane: "main", Label: "main", State: "needs-gate-decision", Command: "/rekit gate -Action debug -Lane main -Apply -Actor <actor>", Source: "missionCommanderActions.followUp", Blocked: true, RequiresReview: true},
	})
	lines := MissionCommanderActionRunLoopMarkdownLines(queue)
	for _, want := range []string{"current run loop：currentRunLoopStep=preview-current steps=4", "run loop step：order=2 step=preview-current actor=main-agent state=needs-gate-decision source=missionCommanderActions command=`/rekit gate -Action debug -Lane main -WhatIf`", "run loop step：order=4 step=follow-up-after-refresh", "run loop boundary：step=follow-up-after-refresh boundary=follow-up commands remain candidates until refreshed state makes them current or unblocked", "driver request：kind=preview-command step=preview-current actor=main-agent executable=true blocked=false requiresReview=true command=`/rekit gate -Action debug -Lane main -WhatIf` guidance=``", "driver request expected receipt：state=refresh-required command=`/rekit gate -Action debug -Lane main -WhatIf`"} {
		if !slices.ContainsFunc(lines, func(line string) bool { return strings.Contains(line, want) }) {
			t.Fatalf("run-loop markdown missing %q: %+v", want, lines)
		}
	}
}

func TestMissionCommanderActionRunLoopMarkdownLinesKeepsGuidanceAsDriverGuidance(t *testing.T) {
	queue := mission.MissionCommanderActionQueueFor([]mission.MissionCommanderNextActionItem{
		{State: "ready-for-next-batch-selection", Command: "select the next Windows-verifiable product-path closure", Source: "releaseHandoffNextBatch"},
	})
	lines := MissionCommanderActionRunLoopMarkdownLines(queue)
	for _, want := range []string{"current run loop：currentRunLoopStep=inspect-current steps=2", "driver request：kind=review-guidance step=inspect-current actor=main-agent executable=false blocked=false requiresReview=false command=`` guidance=`select the next Windows-verifiable product-path closure`", "driver request boundary：guidance text must be reviewed by the main Agent or harness, not executed as a shell command"} {
		if !slices.ContainsFunc(lines, func(line string) bool { return strings.Contains(line, want) }) {
			t.Fatalf("guidance driver markdown missing %q: %+v", want, lines)
		}
	}
}

func TestDailyMissionControlRunbookHandoffDriverRequestsGateApply(t *testing.T) {
	contains := func(items []string, want string) bool {
		return slices.ContainsFunc(items, func(item string) bool { return strings.Contains(item, want) })
	}
	findStep := func(items []DailyMissionControlRunbookStep, stepID string) *DailyMissionControlRunbookStep {
		for index := range items {
			if items[index].StepID == stepID {
				return &items[index]
			}
		}
		return nil
	}
	queue := mission.MissionCommanderActionQueueFor([]mission.MissionCommanderNextActionItem{{
		Lane:    "feature-triage",
		Label:   "triage",
		State:   "ready-to-continue",
		Source:  "missionCommanderActions",
		Command: "/rekit continue triage",
	}})
	statusRunbook := DailyMissionControlRunbookFor("C:/case", "case", queue, `/rekit handoff -Target "C:/case" -WhatIf -Format json`, `/rekit handoff -Target "C:/case" -Apply -Format json`)
	if statusRunbook.HandoffPreviewDriverRequest == nil || statusRunbook.HandoffPreviewDriverRequest.Kind != "preview-command" || !statusRunbook.HandoffPreviewDriverRequest.CommandExecutable || statusRunbook.HandoffPreviewDriverRequest.Command != statusRunbook.HandoffPreviewCommand || statusRunbook.HandoffPreviewDriverRequest.ExpectedReceipt.RefreshStatusCommand != statusRunbook.RefreshStatusCommand {
		t.Fatalf("status runbook should expose executable typed handoff preview request: %+v", statusRunbook.HandoffPreviewDriverRequest)
	}
	if statusRunbook.HandoffApplyDriverRequest == nil || statusRunbook.HandoffApplyDriverRequest.Kind != "review-guidance" || statusRunbook.HandoffApplyDriverRequest.CommandExecutable || statusRunbook.HandoffApplyDriverRequest.Command != "" || !strings.Contains(statusRunbook.HandoffApplyDriverRequest.Guidance, statusRunbook.HandoffApplyCommand) || statusRunbook.HandoffApplyDriverRequest.ExpectedReceipt.Command != "" || !contains(statusRunbook.HandoffApplyDriverRequest.Boundary, "review guidance until a handoff preview/apply result marks it executable") {
		t.Fatalf("status runbook should gate handoff apply as review guidance: %+v", statusRunbook.HandoffApplyDriverRequest)
	}
	statusApplyStep := findStep(statusRunbook.RunLoop, "write-handoff-for-takeover")
	if statusApplyStep == nil || statusApplyStep.CommandExecutable {
		t.Fatalf("status run loop should gate handoff apply as review guidance: %+v", statusApplyStep)
	}

	handoffRunbook := DailyMissionControlRunbookForWithHandoffApplyReady("C:/case", "case", queue, `/rekit handoff -Target "C:/case" -WhatIf -Format json`, `/rekit handoff -Target "C:/case" -Apply -Format json`, true)
	if handoffRunbook.HandoffApplyDriverRequest == nil || handoffRunbook.HandoffApplyDriverRequest.Kind != "preview-command" || !handoffRunbook.HandoffApplyDriverRequest.CommandExecutable || handoffRunbook.HandoffApplyDriverRequest.Command != handoffRunbook.HandoffApplyCommand || handoffRunbook.HandoffApplyDriverRequest.ExpectedReceipt.Command != handoffRunbook.HandoffApplyCommand || handoffRunbook.HandoffApplyDriverRequest.ExpectedReceipt.RefreshStatusCommand != handoffRunbook.RefreshStatusCommand || !contains(handoffRunbook.HandoffApplyDriverRequest.Boundary, "handoff apply writes case-local handoff/resume/checkpoint files only") {
		t.Fatalf("handoff result runbook should expose executable typed handoff apply request: %+v", handoffRunbook.HandoffApplyDriverRequest)
	}
	handoffApplyStep := findStep(handoffRunbook.RunLoop, "write-handoff-for-takeover")
	if handoffApplyStep == nil || !handoffApplyStep.CommandExecutable || handoffApplyStep.Command != handoffRunbook.HandoffApplyCommand {
		t.Fatalf("handoff result run loop should expose the executable apply command: %+v", handoffApplyStep)
	}
}

func TestMissingBoardHandoffPreviewKeepsApplyNonExecutable(t *testing.T) {
	repoRoot, _ := setupContinueCase(t, "")
	caseRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := casebind.WriteInstance(
		caseRoot,
		repoRoot,
		defaults.DefaultPack,
		"handoff-missing-board",
	); err != nil {
		t.Fatal(err)
	}

	result, err := HandoffPreview(
		repoRoot,
		caseRoot,
		defaults.DefaultPack,
		HandoffOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied || !result.Project || len(result.Writes) != 0 ||
		result.PublicationPlanSHA256 != "" || result.ApplyCommand != "" ||
		result.DailyMissionControlRunbook == nil ||
		result.DailyMissionControlRunbook.HandoffApplyDriverRequest == nil ||
		result.DailyMissionControlRunbook.HandoffApplyDriverRequest.CommandExecutable ||
		result.DailyMissionControlRunbook.HandoffApplyDriverRequest.Command != "" ||
		result.MissionCommanderActionQueue.CurrentDriverRequest == nil ||
		result.MissionCommanderActionQueue.CurrentDriverRequest.Source != "caseMissionOnboarding" ||
		result.ReplacementExecutorTakeoverPackage == nil ||
		!result.ReplacementExecutorTakeoverPackage.CommandExecutable ||
		result.ReplacementExecutorTakeoverPackage.Command != result.MissionCommanderActionQueue.CurrentDriverRequest.Command {
		t.Fatalf("missing-board handoff projection crossed onboarding boundary: %+v", result)
	}
	for _, step := range result.DailyMissionControlRunbook.RunLoop {
		if step.StepID == "write-handoff-for-takeover" && step.CommandExecutable {
			t.Fatalf("missing-board handoff exposed executable apply: %+v", step)
		}
	}
}

func TestLaneHandoffApplyPersistsExactExecutableRoute(t *testing.T) {
	repoRoot, caseRoot := setupContinueCase(t, "")
	preview, err := HandoffPreview(
		repoRoot,
		caseRoot,
		defaults.DefaultPack,
		HandoffOptions{Selector: "devirt-main"},
	)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := HandoffApply(
		repoRoot,
		caseRoot,
		defaults.DefaultPack,
		HandoffOptions{
			Selector:                      "devirt-main",
			ExpectedPublicationPlanSHA256: preview.PublicationPlanSHA256,
			PublicationStamp:              preview.PublicationStamp,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	exact := applied.ApplyCommand
	if exact == "" || !strings.Contains(exact, applied.PublicationPlanSHA256) ||
		!strings.Contains(exact, applied.PublicationStamp) ||
		!strings.Contains(exact, "-Lane devirt-main") {
		t.Fatalf("lane handoff result omitted exact bounded apply: %+v", applied)
	}

	markdown := readHandoffTestFile(
		t,
		filepath.Join(caseRoot, ".rekit", "handovers", "devirt-main-latest.md"),
	)
	assertPersistedExecutableHandoffRunbook(t, markdown, exact)

	takeoverData := readHandoffTestFile(
		t,
		filepath.Join(
			caseRoot,
			".rekit",
			"handovers",
			"devirt-main-latest-replacement-executor-takeover.json",
		),
	)
	var takeover mission.ReplacementExecutorTakeoverPackage
	if err := json.Unmarshal([]byte(takeoverData), &takeover); err != nil {
		t.Fatalf("decode persisted lane takeover: %v\n%s", err, takeoverData)
	}
	if !takeover.Ready || !takeover.CommandExecutable ||
		takeover.Command != takeover.CurrentDriverRequest.Command ||
		takeover.CurrentDriverRequest.ExpectedReceipt.Command != takeover.Command {
		t.Fatalf("persisted lane takeover route drifted: %+v", takeover)
	}
}

func TestProjectHandoffApplyPersistsOnlyProjectExactRoute(t *testing.T) {
	repoRoot, caseRoot := setupContinueCase(t, "")
	preview, err := HandoffPreview(
		repoRoot,
		caseRoot,
		defaults.DefaultPack,
		HandoffOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := HandoffApply(
		repoRoot,
		caseRoot,
		defaults.DefaultPack,
		HandoffOptions{
			ExpectedPublicationPlanSHA256: preview.PublicationPlanSHA256,
			PublicationStamp:              preview.PublicationStamp,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	exact := applied.ApplyCommand
	if exact == "" || !strings.Contains(exact, applied.PublicationPlanSHA256) ||
		!strings.Contains(exact, applied.PublicationStamp) ||
		strings.Contains(exact, "-Lane") {
		t.Fatalf("project handoff result omitted project-scoped apply: %+v", applied)
	}

	markdown := readHandoffTestFile(
		t,
		filepath.Join(caseRoot, ".rekit", "handovers", "latest.md"),
	)
	assertPersistedExecutableHandoffRunbook(t, markdown, exact)
	for _, selector := range []string{"main", "bootstrap"} {
		laneExact := handoffApplyCommand(
			caseRoot,
			selector,
			applied.PublicationPlanSHA256,
			applied.PublicationStamp,
		)
		if strings.Contains(markdown, laneExact) {
			t.Fatalf("project handoff bound project plan to lane command: %s", laneExact)
		}
	}
	if strings.Count(markdown, "handoff apply driver request: kind=review-guidance executable=false") < 2 ||
		strings.Count(markdown, "step=write-handoff-for-takeover") < 3 ||
		strings.Count(markdown, "step=write-handoff-for-takeover actor=main-agent state=handoff-apply-available source=dailyMissionControlRunbook.handoffApply driverKind= executable=false") < 2 {
		t.Fatalf("project handoff lane sub-runbook became executable:\n%s", markdown)
	}
}

func readHandoffTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func assertPersistedExecutableHandoffRunbook(
	t *testing.T,
	markdown,
	exact string,
) {
	t.Helper()
	if strings.Contains(markdown, handoffPublicationPlanSHA256Marker) ||
		!strings.Contains(markdown, "handoff apply: `"+exact+"`") ||
		!strings.Contains(markdown, "handoff apply driver request: kind=preview-command executable=true") ||
		!strings.Contains(markdown, "handoff apply driver request expected receipt: state=refresh-required command=`"+exact+"`") ||
		!strings.Contains(markdown, "step=write-handoff-for-takeover") ||
		!strings.Contains(markdown, "executable=true") ||
		!strings.Contains(markdown, "command=`"+exact+"`") {
		t.Fatalf("persisted handoff route is not exact and executable:\n%s", markdown)
	}
}

func TestLimitProjectMissionCommanderNextActionItemsKeepsNextBatchCandidateQueue(t *testing.T) {
	items := []mission.MissionCommanderNextActionItem{
		{Label: "next-batch", ActionID: "next-batch-selection", State: "ready-for-next-batch-selection", Source: "releaseHandoffNextBatch", Command: "select the next batch"},
		{Label: "mission-commander", ActionID: "next-batch-mission-commander-operational-closure", State: "next-batch-candidate-domain", Source: "releaseHandoffNextBatch.followUp.candidateDomain", Command: "select mission commander closure"},
		{Label: "replacement-executor", ActionID: "next-batch-replacement-executor-takeover", State: "next-batch-candidate-domain", Source: "releaseHandoffNextBatch.followUp.candidateDomain", Command: "select replacement executor takeover"},
		{Label: "reviewer-orchestration", ActionID: "next-batch-reviewer-orchestration-closure", State: "next-batch-candidate-domain", Source: "releaseHandoffNextBatch.followUp.candidateDomain", Command: "select reviewer orchestration closure"},
	}
	limited := limitProjectMissionCommanderNextActionItems(items, 2)
	if len(limited) != len(items) || limited[0].ActionID != "next-batch-selection" || !slices.ContainsFunc(limited, func(item mission.MissionCommanderNextActionItem) bool {
		return item.ActionID == "next-batch-replacement-executor-takeover"
	}) {
		t.Fatalf("next-batch candidate-domain queue should not be truncated in project durable handoff: %+v", limited)
	}

	mixed := append([]mission.MissionCommanderNextActionItem{{Label: "main", State: "ready-to-continue", Source: "missionCommanderActions", Command: "/rekit continue main"}}, items...)
	mixedLimited := limitProjectMissionCommanderNextActionItems(mixed, 2)
	if len(mixedLimited) != len(mixed) || mixedLimited[0].Command != "/rekit continue main" || !slices.ContainsFunc(mixedLimited, func(item mission.MissionCommanderNextActionItem) bool {
		return item.ActionID == "next-batch-replacement-executor-takeover"
	}) {
		t.Fatalf("mixed project queue should preserve injected next-batch guidance for durable handoff: %+v", mixedLimited)
	}

	ordinary := append([]mission.MissionCommanderNextActionItem{}, items...)
	for idx := range ordinary {
		ordinary[idx].Source = "missionCommanderActions"
	}
	ordinaryLimited := limitProjectMissionCommanderNextActionItems(ordinary, 2)
	if len(ordinaryLimited) != 2 || slices.ContainsFunc(ordinaryLimited, func(item mission.MissionCommanderNextActionItem) bool { return item.ActionID == "next-batch-selection" }) {
		t.Fatalf("ordinary project action queue should keep existing tail limit semantics: %+v", ordinaryLimited)
	}
}

func TestMissionCommanderNextActionsWithAuthorizedGateAdaptersPrioritizesRepair(t *testing.T) {
	base := []mission.MissionCommanderNextActionItem{{
		Lane:    "main",
		Label:   "main",
		State:   "ready-to-continue",
		Source:  "missionCommanderActions",
		Command: "/rekit continue main",
	}}
	handoffs := []AuthorizedGateAdapterHandoff{
		{
			EventID: "evt-record",
			missionCommanderNextActions: []mission.MissionCommanderNextActionItem{{
				Lane:           "main",
				Label:          "main",
				State:          "ready-to-record-evidence",
				Source:         "adapterReportValidation.missionCommanderAction",
				Command:        "/rekit gate -Apply -GateEventId evt-record",
				RequiresReview: true,
			}},
		},
		{
			EventID: "evt-repair",
			missionCommanderNextActions: []mission.MissionCommanderNextActionItem{{
				Lane:           "main",
				Label:          "main",
				State:          "repair-adapter-report",
				Source:         "adapterReportValidation.repairHints",
				Command:        "move-evidence-refs-under-authorized-output-paths",
				RequiresReview: true,
			}},
		},
		{
			EventID: "evt-acknowledged",
			ReportSummary: &gate.AdapterReportHandoffSummary{
				State: "evidence-already-recorded",
			},
			missionCommanderNextActions: []mission.MissionCommanderNextActionItem{{
				Lane:           "main",
				Label:          "main",
				State:          "evidence-already-recorded",
				Source:         "adapterReportLiveSnapshot.recordedEvidence",
				Command:        "/rekit handoff main",
				RequiresReview: true,
			}},
		},
	}
	items := MissionCommanderNextActionsWithAuthorizedGateAdaptersAndAcknowledgements(base, handoffs, map[string]bool{"evt-acknowledged": true})
	queue := mission.MissionCommanderActionQueueFor(items)
	if queue.CurrentAction == nil || queue.CurrentAction.GateEventID != "evt-repair" || queue.CurrentAction.State != "repair-adapter-report" || queue.CurrentAction.Command != "move-evidence-refs-under-authorized-output-paths" {
		t.Fatalf("repair action should be current before record-ready and acknowledged provenance: queue=%+v items=%+v", queue, items)
	}
	if len(items) < 2 || items[0].GateEventID != "evt-repair" || items[1].GateEventID != "evt-record" {
		t.Fatalf("adapter actions should be sorted repair before record-ready: %+v", items)
	}
	for _, item := range items {
		if item.GateEventID == "evt-acknowledged" {
			t.Fatalf("acknowledged recorded adapter action should not re-enter queue: %+v", items)
		}
	}
}

func TestAuthorizedGateAdapterAcknowledgementPreservesMainEscalation(t *testing.T) {
	action := mission.MissionCommanderNextActionItem{
		GateEventID:    "evt-escalated",
		State:          "needs-main-escalation",
		Command:        "/rekit handoff main",
		RequiresReview: true,
	}
	handoff := AuthorizedGateAdapterHandoff{
		EventID:                     "evt-escalated",
		ReportSummary:               &gate.AdapterReportHandoffSummary{State: "needs-main-escalation", RequiresMainEscalation: true, CurrentAction: action.Command, NextActionCount: 1, ReviewRequiredActionCount: 1},
		missionCommanderNextActions: []mission.MissionCommanderNextActionItem{action},
	}

	applyAuthorizedGateAdapterAcknowledgement(&handoff, map[string]bool{"evt-escalated": true})

	if handoff.Acknowledged || handoff.AcknowledgementState != "" || len(handoff.missionCommanderNextActions) != 1 {
		t.Fatalf("ordinary evidence acknowledgement must not close main escalation: %+v", handoff)
	}
	if handoff.ReportSummary == nil || !handoff.ReportSummary.RequiresMainEscalation || handoff.ReportSummary.CurrentAction != action.Command || handoff.ReportSummary.NextActionCount != 1 {
		t.Fatalf("main escalation summary was hidden by acknowledgement: %+v", handoff.ReportSummary)
	}
}

func TestCompletionKeepsOnlyOpenAuthorizedGateAdapterHandoffs(t *testing.T) {
	open := AuthorizedGateAdapterHandoff{EventID: "evt-open"}
	acknowledged := AuthorizedGateAdapterHandoff{
		EventID:      "evt-acknowledged",
		Acknowledged: true,
	}

	items := openAuthorizedGateAdapterHandoffs([]AuthorizedGateAdapterHandoff{
		open,
		acknowledged,
	})
	if len(items) != 1 || items[0].EventID != open.EventID {
		t.Fatalf("completion should retain only open adapter handoffs: %+v", items)
	}
	if !acknowledged.Acknowledged {
		t.Fatalf("test fixture lost acknowledged provenance: %+v", acknowledged)
	}
}

func TestAuthorizedGateAdapterHandoffsLimitKeepsActionableEarlierGate(t *testing.T) {
	repair := AuthorizedGateAdapterHandoff{
		EventID:       "evt-repair-earlier",
		ReportSummary: &gate.AdapterReportHandoffSummary{State: "repair-adapter-report", ReportPresent: true, RequiresRepair: true},
		missionCommanderNextActions: []mission.MissionCommanderNextActionItem{{
			Lane:           "main",
			Label:          "main",
			State:          "repair-adapter-report",
			Source:         "adapterReportValidation.repairHints",
			Command:        "move-evidence-refs-under-authorized-output-paths",
			RequiresReview: true,
		}},
	}
	items := []AuthorizedGateAdapterHandoff{repair}
	for idx := 1; idx <= maxHandoffRows; idx++ {
		items = append(items, AuthorizedGateAdapterHandoff{
			EventID:       fmt.Sprintf("evt-low-%d", idx),
			Acknowledged:  true,
			ReportSummary: &gate.AdapterReportHandoffSummary{State: "evidence-already-recorded", ReportPresent: true, Valid: true},
		})
	}

	limited := limitAuthorizedGateAdapterHandoffs(items, maxHandoffRows)
	if len(limited) != maxHandoffRows || limited[0].EventID != "evt-repair-earlier" || slices.ContainsFunc(limited, func(item AuthorizedGateAdapterHandoff) bool { return item.EventID == "evt-low-1" }) {
		t.Fatalf("authorized adapter handoff limiter should keep earlier repair and drop oldest low-value item: %+v", limited)
	}
	merged := MissionCommanderNextActionsWithAuthorizedGateAdapters([]mission.MissionCommanderNextActionItem{{Lane: "main", Label: "main", State: "ready-to-continue", Source: "missionCommanderActions", Command: "/rekit continue main"}}, limited)
	queue := mission.MissionCommanderActionQueueFor(merged)
	if queue.CurrentAction == nil || queue.CurrentAction.GateEventID != "evt-repair-earlier" || queue.CurrentAction.State != "repair-adapter-report" {
		t.Fatalf("limited authorized adapter handoffs should still surface repair current action: queue=%+v items=%+v", queue, merged)
	}
}

func TestReadHandoffFactsUsesMissionLedgerSnapshot(t *testing.T) {
	caseRoot := t.TempDir()
	factsRoot := filepath.Join(caseRoot, ".rekit", "facts")
	if err := os.MkdirAll(factsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeHandoffFactLines(t, factsRoot, "observations.jsonl", `{"kind":"observation","lane":"main","subject":"obs","batchId":"batch-handoff-ledger"}`)
	writeHandoffFactLines(t, factsRoot, "candidates.jsonl", `{"kind":"candidate","lane":"main","subject":"candidate","status":"open","batchId":"batch-handoff-ledger"}`)
	writeHandoffFactLines(t, factsRoot, "requests.jsonl", `{"kind":"request","lane":"main","subject":"gate","status":"pending-gate","batchId":"batch-handoff-ledger"}`)
	writeHandoffFactLines(t, factsRoot, "publications.jsonl", `{"kind":"publication","lane":"main","subject":"pub","batchId":"batch-handoff-ledger"}`)
	writeHandoffFactLines(t, factsRoot, "decisions.jsonl", `{"kind":"decision","lane":"main","subject":"decision","decision":"defer","batchId":"batch-handoff-ledger"}`)
	writeHandoffFactLines(t, factsRoot, "hypotheses.jsonl", `{"kind":"hypothesis","lane":"main","subject":"hypothesis","batchId":"batch-handoff-ledger"}`)
	writeHandoffFactLines(t, factsRoot, "verifications.jsonl", `{"kind":"verification","lane":"main","subject":"verify","batchId":"batch-handoff-ledger"}`)
	writeHandoffFactLines(t, factsRoot, "interventions.jsonl", `{"kind":"intervention","lane":"main","subject":"intervention","status":"open","batchId":"batch-handoff-ledger"}`)
	writeHandoffFactLines(t, factsRoot, "rollbacks.jsonl", `{"kind":"rollback","lane":"main","subject":"rollback","batchId":"batch-handoff-ledger"}`)

	facts, err := readHandoffFacts(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts.Observations) != 1 || len(facts.Candidates) != 1 || len(facts.Requests) != 1 || len(facts.Publications) != 1 || len(facts.Decisions) != 1 || len(facts.Hypotheses) != 1 || len(facts.Verifications) != 1 || len(facts.Interventions) != 1 || len(facts.Rollbacks) != 1 {
		t.Fatalf("handoff facts did not read the shared ledger snapshot: %+v", facts)
	}
	if facts.PendingDecision != 1 || len(facts.AllBatchEvents) != 9 {
		t.Fatalf("handoff facts did not include shared ledger summaries: pending=%d batches=%d", facts.PendingDecision, len(facts.AllBatchEvents))
	}
}

func TestWriteProjectMissionBriefBindsCurrentLaneAuthority(t *testing.T) {
	lanes := []boardLane{{ID: "main", Authority: true, Status: "open", CurrentExecutor: "stale-executor", ExecutorGeneration: 1}}
	current := []Lane{{ID: "main", Authority: true, Status: "open", CurrentExecutor: "replacement-executor", ExecutorGeneration: 2}}
	var out bytes.Buffer
	writeProjectMissionBrief(&out, lanes, mission.LedgerFacts{}, current)
	text := out.String()
	want := "/rekit continue main -Executor replacement-executor -ExpectedExecutorGeneration 2"
	if !strings.Contains(text, want) || strings.Contains(text, "stale-executor") {
		t.Fatalf("project mission brief did not rebuild continue command from current lane authority:\n%s", text)
	}
}

func TestMissionCommanderNextActionsWithAuthorizedGateAdaptersOrdersMultipleGateStates(t *testing.T) {
	base := []mission.MissionCommanderNextActionItem{
		{Lane: "main", Label: "gate-recorded", GateEventID: "gate-recorded", State: "ready-for-evidence-review", Command: "/rekit handoff main", Source: "executionEvidenceReview", Blocked: true, RequiresReview: true},
		{Lane: "main", Label: "gate-recorded", GateEventID: "gate-recorded", State: "ready-for-evidence-review", Command: "/rekit overview", Source: "executionEvidenceReview.followUp", Blocked: true, RequiresReview: true},
		{Lane: "main", Label: "gate-changed", GateEventID: "gate-changed", State: "ready-for-evidence-review", Command: "/rekit handoff main", Source: "executionEvidenceReview", RequiresReview: true},
		{Lane: "main", Label: "gate-escalated", GateEventID: "gate-escalated", State: "needs-main-escalation", Command: "/rekit handoff main", Source: "executionEvidenceReview", Blocked: true, RequiresReview: true},
	}
	adapterAction := func(state, command, source string) []mission.MissionCommanderNextActionItem {
		return []mission.MissionCommanderNextActionItem{{Lane: "main", Label: "main", State: state, Command: command, Source: source, RequiresReview: true}}
	}
	handoffs := []AuthorizedGateAdapterHandoff{
		{
			EventID:                     "gate-recorded",
			ReportSummary:               &gate.AdapterReportHandoffSummary{State: "evidence-already-recorded", ReportPresent: true, Valid: true},
			missionCommanderNextActions: adapterAction("evidence-already-recorded", "/rekit handoff main", "adapterReportLiveSnapshot.recordedEvidence"),
		},
		{
			EventID:                     "gate-changed",
			ReportSummary:               &gate.AdapterReportHandoffSummary{State: "repair-adapter-report", ReportPresent: true, RequiresRepair: true},
			missionCommanderNextActions: adapterAction("repair-adapter-report", "add-boundary-marker", "adapterReportValidation.repairHints"),
		},
		{
			EventID:                     "gate-valid",
			ReportSummary:               &gate.AdapterReportHandoffSummary{State: "ready-to-record-evidence", ReportPresent: true, Valid: true, RecordReady: true},
			missionCommanderNextActions: adapterAction("ready-to-record-evidence", "/rekit gate -Apply -GateEventId gate-valid", "adapterReportValidation.missionCommanderAction"),
		},
		{
			EventID:                     "gate-escalated",
			ReportSummary:               &gate.AdapterReportHandoffSummary{State: "needs-main-escalation", ReportPresent: true, Valid: true, RequiresMainEscalation: true},
			missionCommanderNextActions: adapterAction("needs-main-escalation", "/rekit handoff main", "adapterReportLiveSnapshot.recordedEvidence"),
		},
	}

	items := MissionCommanderNextActionsWithAuthorizedGateAdapters(base, handoffs)
	if len(items) != 5 {
		t.Fatalf("unexpected merged action count: %+v", items)
	}
	if items[0].GateEventID != "gate-escalated" || items[0].State != "needs-main-escalation" || items[1].GateEventID != "gate-recorded" || items[1].Source != "executionEvidenceReview" || items[2].Source != "executionEvidenceReview.followUp" || items[3].State != "repair-adapter-report" || items[3].GateEventID != "gate-changed" || items[4].State != "ready-to-record-evidence" || items[4].GateEventID != "gate-valid" {
		t.Fatalf("merged queue did not preserve per-gate evidence and adapter ordering: %+v", items)
	}
	queue := mission.MissionCommanderActionQueueFor(items)
	if queue.CurrentAction == nil || queue.CurrentAction.GateEventID != "gate-escalated" || queue.CurrentAction.State != "needs-main-escalation" {
		t.Fatalf("main escalation evidence should be current: %+v", queue)
	}
	for _, item := range items[3:] {
		if !item.Blocked || !slices.Contains(item.Reasons, "another execution evidence item requires main review before this adapter action") {
			t.Fatalf("adapter action should remain visible but blocked by main review: %+v", item)
		}
	}
	if slices.ContainsFunc(items, func(item mission.MissionCommanderNextActionItem) bool {
		return item.Command == "/rekit continue main"
	}) {
		t.Fatalf("autonomous continue should not survive main escalation: %+v", items)
	}
}

func TestMissionCommanderNextActionsWithAuthorizedGateAdaptersKeepsEvidenceForMissingSidecar(t *testing.T) {
	base := []mission.MissionCommanderNextActionItem{
		{Lane: "main", Label: "gate-missing", GateEventID: "gate-missing", State: "needs-main-escalation", Command: "/rekit handoff main", Source: "executionEvidenceReview", Blocked: true, RequiresReview: true},
	}
	handoff := AuthorizedGateAdapterHandoff{
		EventID:                     "gate-missing",
		ReportSummary:               &gate.AdapterReportHandoffSummary{State: "needs-adapter-report-validation", RequiresValidation: true},
		missionCommanderNextActions: []mission.MissionCommanderNextActionItem{{Lane: "main", State: "needs-adapter-report-validation", Command: "/rekit gate -ValidateExecutionReport", Source: "adapterReportContract.missionCommanderAction", RequiresReview: true}},
	}

	items := MissionCommanderNextActionsWithAuthorizedGateAdapters(base, []AuthorizedGateAdapterHandoff{handoff})
	if len(items) != 2 || items[0].Source != "executionEvidenceReview" || items[1].Source != "adapterReportContract.missionCommanderAction" || !items[1].Blocked {
		t.Fatalf("missing sidecar should preserve evidence review and blocked adapter validation: %+v", items)
	}
}

func writeHandoffFactLines(t *testing.T, root, name string, lines ...string) {
	t.Helper()
	text := ""
	for _, line := range lines {
		text += line + "\n"
	}
	if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
