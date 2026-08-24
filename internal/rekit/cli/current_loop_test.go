package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/currentloop"
	"github.com/shuiyu486/re-context-kits/internal/rekit/externalsession"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewersession"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
	"github.com/shuiyu486/re-context-kits/internal/rekit/subagents"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestCurrentLoopExternalReviewerRelayResultUsesResolvedReviewRoot(t *testing.T) {
	for _, stateDir := range []string{projectstate.CurrentDir, projectstate.LegacyDir} {
		t.Run(stateDir, func(t *testing.T) {
			caseRoot := t.TempDir()
			packetPath := filepath.Join(caseRoot, stateDir, "reviews", "packet-a", "packet.json")
			if err := os.MkdirAll(filepath.Dir(packetPath), 0o755); err != nil {
				t.Fatal(err)
			}
			got, err := currentLoopExternalReviewerRelayResultPath(caseRoot, packetPath, "reviewer-job-a")
			if err != nil {
				t.Fatal(err)
			}
			want := filepath.ToSlash(filepath.Join(stateDir, "reviews", "packet-a", "results", "external-session-returns", "reviewer-job-a.json"))
			if got != want {
				t.Fatalf("relay result path = %q, want %q", got, want)
			}
		})
	}
}

func TestCurrentLoopExternalReviewerRelayResultRejectsMixedStateRoots(t *testing.T) {
	caseRoot := t.TempDir()
	packetPath := filepath.Join(caseRoot, projectstate.CurrentDir, "reviews", "packet-a", "packet.json")
	for _, stateDir := range []string{projectstate.CurrentDir, projectstate.LegacyDir} {
		if err := os.MkdirAll(filepath.Join(caseRoot, stateDir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := currentLoopExternalReviewerRelayResultPath(caseRoot, packetPath, "reviewer-job-a"); err == nil {
		t.Fatal("mixed state roots unexpectedly produced a reviewer relay result path")
	}
}

func TestCurrentLoopExternalMemberHandoffUsesResolvedStateRoot(t *testing.T) {
	for _, fixture := range []struct {
		name     string
		stateDir string
	}{
		{name: "current", stateDir: projectstate.CurrentDir},
		{name: "legacy", stateDir: projectstate.LegacyDir},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			caseRoot := t.TempDir()
			if err := os.MkdirAll(filepath.Join(caseRoot, fixture.stateDir), 0o755); err != nil {
				t.Fatal(err)
			}
			inspection := memberexecution.Inspection{
				State:     "handoff-ready",
				AttemptID: "attempt-a",
				Owner: memberexecution.Owner{
					Lane:               "feature-a",
					Executor:           "member-a",
					ExecutorGeneration: 2,
				},
				Handoff: &memberexecution.Handoff{
					TaskContextPath:   fixture.stateDir + "/task-context.json",
					TaskContextSHA256: strings.Repeat("a", 64),
					ManifestPath:      fixture.stateDir + "/manifest.json",
					OutputsRoot:       fixture.stateDir + "/outputs",
				},
				HandoffSHA256: strings.Repeat("b", 64),
			}
			handoff := currentLoopExternalMemberHandoff(runtime.Context{Target: caseRoot, Pack: "_template"}, inspection, nil)
			want, err := projectstate.Rel(caseRoot, "lanes", "feature-a", "member-executions", "attempt-a", "handoff.json")
			if err != nil {
				t.Fatal(err)
			}
			if handoff == nil || handoff.HandoffPath != want {
				t.Fatalf("external member handoff path=%+v want=%q", handoff, want)
			}
		})
	}
}

func TestCurrentLoopExternalMemberHandoffRejectsMixedStateRoots(t *testing.T) {
	caseRoot := t.TempDir()
	for _, stateDir := range []string{projectstate.CurrentDir, projectstate.LegacyDir} {
		if err := os.MkdirAll(filepath.Join(caseRoot, stateDir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	inspection := memberexecution.Inspection{
		State:     "handoff-ready",
		AttemptID: "attempt-a",
		Owner:     memberexecution.Owner{Lane: "feature-a", Executor: "member-a", ExecutorGeneration: 2},
		Handoff:   &memberexecution.Handoff{},
	}
	if handoff := currentLoopExternalMemberHandoff(runtime.Context{Target: caseRoot, Pack: "_template"}, inspection, nil); handoff != nil {
		t.Fatalf("mixed state roots should not project an external member handoff: %+v", handoff)
	}
}

func TestRemoteControlTransportPackageProjectsTypedLifecycleRequests(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(caseRoot, projectstate.CurrentDir), 0o755); err != nil {
		t.Fatal(err)
	}
	job := externalsession.Job{
		CaseRoot: caseRoot, JobID: "reviewer-job", CheckpointSHA256: strings.Repeat("a", 64), SessionKind: "reviewer",
		Reviewer: &externalsession.ReviewerIdentity{DispatchID: "dispatch-a", Harness: externalsession.RemoteControlHarness, Session: "transport-binding-a"},
	}
	attempt := externalsession.AttemptInspection{AttemptSHA256: strings.Repeat("b", 64), Current: &externalsession.Attempt{AttemptID: "reviewer-job-g000001", Generation: 1, Harness: externalsession.RemoteControlHarness, Session: "transport-binding-a"}}
	dispatch := externalsession.DispatchInspection{
		State: "claimed", TicketSHA256: strings.Repeat("c", 64), ClaimSHA256: strings.Repeat("d", 64),
		Claim: &externalsession.DispatchClaim{Actor: "mission-commander"},
	}
	binding := externalsession.TransportBinding{
		Transport: externalsession.TransportRemoteControl, TransportBindingID: "transport-binding-a", JobID: job.JobID,
		JobSHA256: strings.Repeat("e", 64), CheckpointSHA256: job.CheckpointSHA256, AttemptID: attempt.Current.AttemptID,
		AttemptSHA256: attempt.AttemptSHA256, Generation: 1, DispatchSHA256: dispatch.TicketSHA256, ClaimSHA256: dispatch.ClaimSHA256,
	}

	discovery, err := externalSessionTransportPackage(job, attempt, dispatch, externalsession.TransportInspection{Applicable: true, State: "endpoint-required", Binding: binding})
	if err != nil {
		t.Fatal(err)
	}
	if discovery == nil || discovery.DiscoveryTool != "ListAgents" || discovery.DiscoveryRequest == nil || discovery.DiscoveryRequest.CommandExecutable || discovery.DiscoveryRequest.Source != "current-loop-external-session-transport" || !strings.Contains(discovery.DiscoveryRequest.Guidance, "ListAgents") {
		t.Fatalf("Remote Control endpoint discovery package=%+v", discovery)
	}

	endpoint := &externalsession.TransportEndpointSnapshot{Endpoint: "reviewer [opaque-ref]", Actor: "mission-commander", ObservedAt: "2026-08-12T01:00:00Z", Envelope: externalsession.TransportMessageEnvelope{Operation: "SendMessage", Recipient: "reviewer [opaque-ref]", Message: "bounded message", MessageSHA256: strings.Repeat("f", 64), NoFileTransfer: true}}
	delivery, err := externalSessionTransportPackage(job, attempt, dispatch, externalsession.TransportInspection{Applicable: true, State: "delivery-required", Binding: binding, Endpoint: endpoint, EndpointSHA256: strings.Repeat("1", 64), EnvelopeSHA256: strings.Repeat("2", 64)})
	if err != nil {
		t.Fatal(err)
	}
	if delivery == nil || delivery.Message == nil || delivery.Message.Operation != "SendMessage" || !delivery.Message.NoFileTransfer || delivery.DeliveryRequest == nil || delivery.DeliveryRequest.CommandExecutable || !strings.Contains(delivery.DeliveryRequest.Guidance, "exactly once") {
		t.Fatalf("Remote Control delivery package=%+v", delivery)
	}

	acceptedObservation := &externalsession.TransportDeliveryObservation{Outcome: "accepted", Actor: "mission-commander", ObservedAt: "2026-08-12T01:01:00Z"}
	running := dispatch
	running.State = "running"
	running.LaunchSHA256 = strings.Repeat("4", 64)
	returned, err := externalSessionTransportPackage(job, attempt, running, externalsession.TransportInspection{Applicable: true, State: "delivery-accepted", Binding: binding, Endpoint: endpoint, EndpointSHA256: strings.Repeat("1", 64), EnvelopeSHA256: strings.Repeat("2", 64), Delivery: acceptedObservation, DeliverySHA256: strings.Repeat("3", 64)})
	if err != nil {
		t.Fatal(err)
	}
	if returned == nil || returned.ReturnRequest == nil || returned.ReturnRequest.CommandExecutable || returned.ReturnRequest.State != "remote-control-reviewer-result-required" || returned.ReturnRequest.ExpectedReceipt.State != "transport-return-preview" || !strings.Contains(returned.ReturnRequest.Guidance, "ExternalSessionReviewerResultSourcePath") || returned.LaunchRequest != nil {
		t.Fatalf("Remote Control return package=%+v", returned)
	}

	uncertainObservation := &externalsession.TransportDeliveryObservation{Outcome: "uncertain", Actor: "mission-commander", ObservedAt: "2026-08-12T01:01:00Z", Reason: "no stable acknowledgement"}
	uncertain, err := externalSessionTransportPackage(job, attempt, dispatch, externalsession.TransportInspection{Applicable: true, State: "delivery-uncertain", Binding: binding, Delivery: uncertainObservation, DeliverySHA256: strings.Repeat("3", 64)})
	if err != nil {
		t.Fatal(err)
	}
	if uncertain == nil || uncertain.ReplacementRequest == nil || uncertain.ReplacementRequest.CommandExecutable || uncertain.ReplacementRequest.State != "transport-delivery-uncertain-new-dispatch-required" || !strings.Contains(uncertain.ReplacementRequest.Guidance, "Do not resend or replace this job") || uncertain.ReplacementRequest.Command != "" {
		t.Fatalf("Remote Control uncertain package=%+v", uncertain)
	}
}

func TestRemoteControlReturnContractUsesResolvedStateRoot(t *testing.T) {
	for _, stateDir := range []string{projectstate.CurrentDir, projectstate.LegacyDir} {
		t.Run(stateDir, func(t *testing.T) {
			caseRoot := t.TempDir()
			if err := os.MkdirAll(filepath.Join(caseRoot, stateDir), 0o755); err != nil {
				t.Fatal(err)
			}
			job := externalsession.Job{
				CaseRoot: caseRoot, JobID: "reviewer-job", SessionKind: "reviewer", AllowedOutcomes: []string{"returned"},
				Reviewer: &externalsession.ReviewerIdentity{Harness: externalsession.RemoteControlHarness},
			}
			attempt := externalsession.AttemptInspection{Current: &externalsession.Attempt{
				Generation: 3, SubmissionPath: stateDir + "/attempt/submission.json", SubmissionResult: stateDir + "/attempt/result.json",
			}}
			contract, err := externalSessionReturnContract(job, externalsession.Inspection{}, attempt, &mission.CurrentLoopOperatorPackage{}, mission.CurrentLoopExternalSessionJob{})
			if err != nil {
				t.Fatal(err)
			}
			wantReceipt, err := externalsession.TransportReturnReceiptPathForCase(caseRoot, job.JobID, attempt.Current.Generation)
			if err != nil {
				t.Fatal(err)
			}
			if contract == nil || len(contract.Templates) != 1 || !slices.Contains(contract.Templates[0].RequiredWrites, wantReceipt) || !strings.HasPrefix(wantReceipt, stateDir+"/") {
				t.Fatalf("state root %s return contract=%+v receipt=%q", stateDir, contract, wantReceipt)
			}
		})
	}
}

func TestExternalSessionRunningRequestForDurableReviewerForbidsSameJobReplacement(t *testing.T) {
	job := &mission.CurrentLoopExternalSessionJob{
		JobID: "reviewer-job", SessionKind: "reviewer", Reviewer: &mission.CurrentLoopExternalSessionJobReviewer{DispatchID: "dispatch-a", Harness: "claude-code", Session: "session-a"},
		HarnessPackage: &mission.CurrentLoopExternalSessionHarnessPackage{RefreshStatusCommand: "/rekit status"},
	}
	request := externalSessionRunningRequest(job)
	if request.CommandExecutable || request.ExpectedReceipt.State != "submission-or-new-reviewer-dispatch-review" || !strings.Contains(request.Guidance, "Do not replace this durable Reviewer job") || !slices.Contains(request.ExpectedReceipt.Boundary, "no same-job Reviewer replacement") {
		t.Fatalf("durable Reviewer running request=%+v", request)
	}
}

func TestExternalSessionHarnessPackageFailsClosedWithoutStableMemberInput(t *testing.T) {
	caseRoot := t.TempDir()
	job := externalsession.Job{
		CaseRoot: caseRoot, JobID: "member-job", CheckpointSHA256: strings.Repeat("a", 64),
		SessionKind: "member", AllowedOutcomes: []string{"returned", "failed"},
	}
	current := &externalsession.Attempt{
		AttemptID: "member-job-g000001", Generation: 1, Harness: "test-harness", Session: "test-session",
		SubmissionPath:    ".rekit/external-session-attempt-inputs/member-job/000001/submission.json",
		SubmissionOutputs: ".rekit/external-session-attempt-inputs/member-job/000001/outputs",
	}
	attempt := externalsession.AttemptInspection{State: "running", Current: current, AttemptSHA256: strings.Repeat("b", 64)}
	typed := mission.CurrentLoopExternalSessionJob{CurrentAttempt: &mission.CurrentLoopExternalSessionAttempt{
		AttemptID: current.AttemptID, AttemptSHA256: attempt.AttemptSHA256, Generation: current.Generation,
		Harness: current.Harness, Session: current.Session, SubmissionPath: current.SubmissionPath, SubmissionOutputs: current.SubmissionOutputs,
	}}
	operator := &mission.CurrentLoopOperatorPackage{ExternalMemberHandoff: &mission.CurrentLoopExternalMemberHandoff{TaskContextPath: ".rekit/member-executions/missing/task-context.json", TaskContextSHA256: strings.Repeat("d", 64)}}
	pkg := externalSessionHarnessPackage(job, externalsession.Inspection{Job: job, JobSHA256: strings.Repeat("c", 64), State: "awaiting-submission"}, attempt, operator, typed)
	if pkg == nil || pkg.State != "running" || pkg.Launch == nil || pkg.Launch.Ready || pkg.Launch.Input.Path != operator.ExternalMemberHandoff.TaskContextPath || pkg.Launch.Input.SHA256 != operator.ExternalMemberHandoff.TaskContextSHA256 || len(pkg.Warnings) != 1 || pkg.Return == nil {
		t.Fatalf("missing stable member input did not fail closed while preserving return contract: %+v", pkg)
	}
}

func TestExternalSessionHarnessPackageFailsClosedOnMemberHandoffCommitDrift(t *testing.T) {
	caseRoot := t.TempDir()
	handoffPath := filepath.Join(caseRoot, ".rekit", "lanes", "main", "member-executions", "attempt", "handoff.json")
	if err := os.MkdirAll(filepath.Dir(handoffPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoffPath, []byte("replaced handoff bytes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	job := externalsession.Job{CaseRoot: caseRoot, JobID: "member-job", CheckpointSHA256: strings.Repeat("a", 64), SessionKind: "member", AllowedOutcomes: []string{"returned", "failed"}}
	current := &externalsession.Attempt{AttemptID: "member-job-g000001", Generation: 1, Harness: "test-harness", Session: "test-session", SubmissionPath: ".rekit/external-session-attempt-inputs/member-job/000001/submission.json", SubmissionOutputs: ".rekit/external-session-attempt-inputs/member-job/000001/outputs"}
	attempt := externalsession.AttemptInspection{State: "running", Current: current, AttemptSHA256: strings.Repeat("b", 64)}
	typed := mission.CurrentLoopExternalSessionJob{CurrentAttempt: &mission.CurrentLoopExternalSessionAttempt{AttemptID: current.AttemptID, AttemptSHA256: attempt.AttemptSHA256, Generation: current.Generation, Harness: current.Harness, Session: current.Session, SubmissionPath: current.SubmissionPath, SubmissionOutputs: current.SubmissionOutputs}}
	operator := &mission.CurrentLoopOperatorPackage{ExternalMemberHandoff: &mission.CurrentLoopExternalMemberHandoff{TaskContextPath: handoffPath, TaskContextSHA256: strings.Repeat("d", 64)}}
	pkg := externalSessionHarnessPackage(job, externalsession.Inspection{Job: job, JobSHA256: strings.Repeat("c", 64), State: "awaiting-submission"}, attempt, operator, typed)
	if pkg == nil || pkg.Launch == nil || pkg.Launch.Ready || pkg.Launch.Input.Path != handoffPath || pkg.Launch.Input.SHA256 != strings.Repeat("d", 64) || len(pkg.Warnings) != 1 || pkg.Return == nil {
		t.Fatalf("member handoff commit drift did not fail closed while preserving return contract: %+v", pkg)
	}
}

func TestExternalSessionHarnessPackageFailsClosedOnReviewerPromptDrift(t *testing.T) {
	caseRoot := t.TempDir()
	promptPath := filepath.Join(caseRoot, ".rekit", "reviews", "prompt.md")
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(promptPath, []byte("drifted reviewer prompt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	job := externalsession.Job{CaseRoot: caseRoot, JobID: "reviewer-job", CheckpointSHA256: strings.Repeat("a", 64), SessionKind: "reviewer", AllowedOutcomes: []string{"returned", "failed"}}
	current := &externalsession.Attempt{AttemptID: "reviewer-job-g000001", Generation: 1, Harness: "test-harness", Session: "test-session", SubmissionPath: ".rekit/external-session-attempt-inputs/reviewer-job/000001/submission.json", SubmissionResult: ".rekit/external-session-attempt-inputs/reviewer-job/000001/reviewer-result.json"}
	attempt := externalsession.AttemptInspection{State: "running", Current: current, AttemptSHA256: strings.Repeat("b", 64)}
	typed := mission.CurrentLoopExternalSessionJob{CurrentAttempt: &mission.CurrentLoopExternalSessionAttempt{AttemptID: current.AttemptID, AttemptSHA256: attempt.AttemptSHA256, Generation: current.Generation, Harness: current.Harness, Session: current.Session, SubmissionPath: current.SubmissionPath, SubmissionResult: current.SubmissionResult}}
	operator := &mission.CurrentLoopOperatorPackage{ExternalReviewerHandoff: &mission.CurrentLoopExternalReviewerHandoff{DispatchPromptPath: promptPath, DispatchPromptSHA256: strings.Repeat("d", 64)}}
	pkg := externalSessionHarnessPackage(job, externalsession.Inspection{Job: job, JobSHA256: strings.Repeat("c", 64), State: "awaiting-submission"}, attempt, operator, typed)
	if pkg == nil || pkg.Launch == nil || pkg.Launch.Ready || pkg.Launch.Input.Path != promptPath || pkg.Launch.Input.SHA256 != strings.Repeat("d", 64) || len(pkg.Warnings) != 1 || pkg.Return == nil {
		t.Fatalf("reviewer prompt drift did not fail closed while preserving return contract: %+v", pkg)
	}
}

func TestWriteCurrentLoopOperatorPackageTextIncludesStrictSubmissionTemplates(t *testing.T) {
	attemptSHA := strings.Repeat("a", 64)
	pkg := &mission.CurrentLoopOperatorPackage{ExternalSessionJob: &mission.CurrentLoopExternalSessionJob{SessionKind: "reviewer", HarnessPackage: &mission.CurrentLoopExternalSessionHarnessPackage{Return: &mission.CurrentLoopExternalSessionReturnContract{Templates: []mission.CurrentLoopExternalSessionSubmissionTemplate{{Outcome: "returned", JSON: "{\n  \"outcome\": \"returned\",\n  \"attemptSha256\": \"" + attemptSHA + "\",\n  \"reviewerSession\": \"reviewer-session\"\n}\n", RequiredWrites: []string{"reviewer-result.json", "submission.json (last)"}, RequiredReplace: []string{"<actor>"}}, {Outcome: "failed", JSON: "{\n  \"outcome\": \"failed\",\n  \"reviewerExitStatus\": \"<exit-status>\"\n}\n", RequiredWrites: []string{"submission.json (last)"}, RequiredReplace: []string{"<actor>", "<exit-status>"}}}}}}}
	var out bytes.Buffer
	if err := writeCurrentLoopOperatorPackageText(&out, "status", pkg); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"kind=reviewer", "outcome=returned", "requiredWrites=[\"reviewer-result.json\" \"submission.json (last)\"]", "requiredReplacements=[\"<actor>\"]", `json="{\n  \"outcome\": \"returned\",`, attemptSHA, `\"reviewerSession\": \"reviewer-session\"`, "outcome=failed", "requiredReplacements=[\"<actor>\" \"<exit-status>\"]", `\"reviewerExitStatus\": \"<exit-status>\"`, "submission.json (last)"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("current-loop operator text omitted strict submission template %q:\n%s", want, out.String())
		}
	}
}

func TestCurrentLoopOperatorPackagesDirectReviewerResultHandoff(t *testing.T) {
	selected := &mission.MissionCommanderDriverRequest{
		Command: "/rekit run-current-loop -Target case -ResumeCurrentLoop -ExpectedCurrentLoopCheckpointSha256 checkpoint -WhatIf -Format json",
	}
	pkg := &workstream.ReviewerDispatchOperatorPackage{
		Ready:                true,
		CurrentRunLoopStepID: "save-result-input",
		Current: &workstream.ReviewerDispatchOperatorPackageItem{
			State:                  "awaiting-reviewer-result",
			ReviewerResultDropPath: ".rekit/reviews/review/results/shard.json",
		},
	}

	handoff := statusCurrentLoopExternalReviewerHandoff(pkg, selected)
	if handoff == nil || handoff.ReviewerResultDropPathRole != "direct-reviewer-result-destination" || len(handoff.ObservationContract.Alternatives) != 2 {
		t.Fatalf("direct reviewer result handoff = %+v", handoff)
	}
	attempt := handoff.Attempt
	if attempt == nil || attempt.SchemaVersion != 1 || !strings.HasPrefix(attempt.AttemptID, "reviewer-attempt-") || len(attempt.AttemptSnapshotSHA256) != 64 || attempt.SelectedAction.Kind != "observe-reviewer-terminal-state" || attempt.DurableContinuationDriverRequest == nil || attempt.DurableContinuationDriverRequest.Command != selected.Command || attempt.ReviewerResultDropPathRole != "direct-reviewer-result-destination" {
		t.Fatalf("direct reviewer attempt package = %+v", attempt)
	}
	if actual := statusCurrentLoopReviewerAttemptSHA256(attempt); actual != attempt.AttemptSnapshotSHA256 {
		t.Fatalf("direct reviewer attempt snapshot sha = %s, want %s", attempt.AttemptSnapshotSHA256, actual)
	}
	direct := handoff.ObservationContract.Alternatives[0]
	failed := handoff.ObservationContract.Alternatives[1]
	if direct.Kind != "reviewer-result-direct-write" || direct.Transition != "external-write-then-refresh-status" || direct.PreviewCommandTemplate != selected.Command || len(direct.RequiredFlags) != 0 {
		t.Fatalf("direct reviewer result observation = %+v", direct)
	}
	if failed.Kind != "reviewer-session-failed" || !strings.Contains(failed.PreviewCommandTemplate, "-ReviewerOutcome failed") || !strings.Contains(failed.PreviewCommandTemplate, "-ReviewerExitStatus <exit-status>") {
		t.Fatalf("direct reviewer failure observation = %+v", failed)
	}
}

func TestCurrentLoopOperatorOmitsUnfocusedReviewerHandoff(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha"}, &out); err != nil {
		t.Fatal(err)
	}
	appendCurrentLoopOpenIntervention(t, caseRoot, "int-current-loop-reviewer-unfocused")
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status statusInventory
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("unfocused reviewer status did not decode: %v\n%s", err, out.String())
	}
	operator := status.MissionControlRunbook.CurrentLoopOperator
	if status.MissionControlRunbook.Scope != "case" || operator == nil || operator.ExternalReviewerHandoff != nil || operator.SourceCurrentDriverRequest == nil || driverStepCommandName(operator.SourceCurrentDriverRequest.Command) != "reconcile" {
		t.Fatalf("unfocused reviewer handoff leaked into case operator: scope=%s operator=%+v", status.MissionControlRunbook.Scope, operator)
	}
}

func TestRunCurrentLoopAppliesBoundedCaseSteps(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}

	preview := runCurrentLoopPreview(t, caseRoot, 2)
	if preview.ExpectedCurrentLoopPlanSHA256 == "" || preview.InitialRoute != "case" || preview.InitialCurrentStep == nil || preview.StopReason.Code != "ready" {
		t.Fatalf("unexpected current-loop preview: %+v", preview)
	}
	if preview.Applied || preview.AppliedSteps != 0 || len(preview.Steps) != 0 || preview.SegmentCheckpoint != nil {
		t.Fatalf("current-loop preview mutated: %+v", preview)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "runs", "current-loop-segments")); !os.IsNotExist(err) {
		t.Fatalf("current-loop WhatIf created checkpoint namespace: %v", err)
	}

	out.Reset()
	err := Run([]string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-MaxSteps", "3", "-ExpectedCurrentLoopPlanSha256", preview.ExpectedCurrentLoopPlanSHA256, "-Apply", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "expected plan sha256 mismatch") {
		t.Fatalf("maxSteps drift was not rejected: %v", err)
	}

	applied := runCurrentLoopApply(t, caseRoot, preview)
	if !applied.Applied || applied.AppliedSteps != 1 || len(applied.Steps) != 1 || applied.StopReason.Code != "no-progress" || applied.FinalStatus == nil {
		t.Fatalf("unexpected bounded current-loop apply: %+v", applied)
	}
	for index, receipt := range applied.Steps {
		if receipt.Step != index+1 || receipt.Route != "case" || receipt.ExpectedCurrentStepPlanSHA256 == "" || receipt.CurrentStepReceipt == nil || receipt.CurrentStepReceipt.State != "refreshed" {
			t.Fatalf("current-loop step receipt %d = %+v", index+1, receipt)
		}
	}
	if applied.Steps[0].RequestAfter == nil || applied.Steps[0].RequestAfter.Command != applied.Steps[0].RequestBefore.Command {
		t.Fatalf("no-progress receipt did not retain the repeated request: %+v", applied.Steps[0])
	}
	for _, forbidden := range []string{"authority.jsonl", "confirmed.jsonl"} {
		if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", forbidden)); !os.IsNotExist(err) {
			t.Fatalf("current-loop created forbidden ledger %s: %v", forbidden, err)
		}
	}
}

func TestRunCurrentLoopRecoversExactMemberDispatchPublication(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	board.Lanes[0].CurrentExecutor = "replay-member"
	board.Lanes[0].ExecutorGeneration = 1
	board.Lanes[0].UpdatedAt = "2026-08-10T01:00:00Z"
	boardData, err := json.MarshalIndent(board, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), append(boardData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	preview := runCurrentLoopPreviewWith(t, caseRoot, 2, "-Lane", "main")
	if preview.InitialCurrentStep == nil || preview.InitialCurrentStep.MemberExecution == nil {
		t.Fatalf("member dispatch preview missing: %+v", preview)
	}
	memberPlan := preview.InitialCurrentStep.MemberExecution
	published, err := memberexecution.Apply(*memberPlan, memberPlan.ExpectedPlanSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if !published.Applied || published.AlreadyApplied {
		t.Fatalf("member dispatch crash-window fixture was not freshly published: %+v", published)
	}

	recovered := runCurrentLoopApplyWith(t, caseRoot, preview, "-Lane", "main", "-ExpectedMemberExecutionPlanSha256", memberPlan.ExpectedPlanSHA256)
	if !recovered.Applied || recovered.AppliedSteps != 1 || len(recovered.Steps) != 1 || recovered.FinalStatus == nil || recovered.SegmentCheckpoint == nil {
		t.Fatalf("current-loop did not recover the exact published member dispatch: %+v", recovered)
	}
	if recovered.StopReason.Code != "external-member-handoff" || recovered.Steps[0].CurrentStepReceipt == nil || recovered.Steps[0].CurrentStepReceipt.Outcome != "current-step-applied" {
		t.Fatalf("member dispatch recovery omitted the strict receipt and external handoff stop: %+v", recovered)
	}
	var statusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	var status statusInventory
	decodeJSONStrict(t, statusOut.Bytes(), &status)
	segment := status.MissionControlRunbook.CurrentLoopSegment
	if segment == nil || !segment.Ready || segment.State != "ready" || segment.StopCode != "external-member-handoff" {
		t.Fatalf("selected-lane status did not retain the recovered member checkpoint: status=%+v written=%+v", segment, recovered.SegmentCheckpoint)
	}
}

func TestRunCurrentLoopAppliesTwoDistinctCaseStepsToLimit(t *testing.T) {
	caseRoot := currentLoopCaseWithOpenIntervention(t, "int-current-loop-limit")
	preview := runCurrentLoopPreview(t, caseRoot, 2)
	if preview.ExpectedCurrentLoopPlanSHA256 == "" || preview.InitialCurrentStep == nil || driverStepCommandName(preview.InitialCurrentStep.CurrentDriverRequest.Command) != "reconcile" {
		t.Fatalf("unexpected reconcile-first loop preview: %+v", preview)
	}
	applied := runCurrentLoopApply(t, caseRoot, preview)
	if !applied.Applied || applied.AppliedSteps != 2 || len(applied.Steps) != 2 || applied.StopReason.Code != "limit-reached" || applied.StopReason.Phase != "after-step" || applied.FinalStatus == nil {
		t.Fatalf("two-step loop did not reach its bound: %+v", applied)
	}
	if driverStepCommandName(applied.Steps[0].RequestBefore.Command) != "reconcile" || driverStepCommandName(applied.Steps[1].RequestBefore.Command) != "continue" || applied.Steps[0].ExpectedCurrentStepPlanSHA256 == applied.Steps[1].ExpectedCurrentStepPlanSHA256 {
		t.Fatalf("loop did not execute distinct reconcile and continue plans: %+v", applied.Steps)
	}
}

func TestRunCurrentLoopTerminalCheckpointProductPath(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	preview := runCurrentLoopPreview(t, caseRoot, 2)
	applied := runCurrentLoopApply(t, caseRoot, preview)
	if !applied.Applied || applied.AppliedSteps != 1 || applied.SegmentCheckpoint == nil {
		t.Fatalf("terminal fixture did not create a segment checkpoint: %+v", applied)
	}
	artifactPath := filepath.Join(caseRoot, filepath.FromSlash(applied.SegmentCheckpoint.ArtifactPath))
	artifactData, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatal(err)
	}
	var artifact struct {
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(artifactData, &artifact); err != nil {
		t.Fatal(err)
	}
	var payload currentloop.Payload
	if err := json.Unmarshal(artifact.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	payload.StatusAvailable = true
	payload.RefreshedCurrentDriverRequest = nil
	payload.RefreshedCurrentDriverRequestSHA256 = ""
	payload.Continuation = nil
	last := &payload.StepReceipts[len(payload.StepReceipts)-1]
	last.RequestAfter = nil
	last.RequestAfterSHA256 = ""
	last.CurrentStepReceipt.RefreshedCurrentDriverRequest = nil
	last.CurrentStepReceiptSHA256, _ = currentloop.ValueSHA256(last.CurrentStepReceipt)
	if err := os.Remove(artifactPath); err != nil {
		t.Fatal(err)
	}
	if _, err := currentloop.Write(repoRoot(t), caseRoot, "_template", payload); err != nil {
		t.Fatal(err)
	}
	terminal := currentloop.Inspect(repoRoot(t), caseRoot, "_template", nil)
	if terminal.Ready || terminal.State != "terminal" || terminal.Continuation != nil {
		t.Fatalf("terminal checkpoint exposed recoverable continuation: %+v", terminal)
	}

	completionEvidence := filepath.Join(caseRoot, ".rekit", "lanes", "main", "workspace", "completion-evidence.md")
	writeCompletionEvidence(t, completionEvidence, "reviewed terminal current-loop evidence")
	completionPreview := previewCompletion(t, &out, caseRoot, "main", ".rekit/lanes/main/workspace/completion-evidence.md")
	if completionPreview.Blocked || completionPreview.CompletionPlanSHA256 == "" {
		t.Fatalf("terminal current-loop completion preview is blocked: %+v", completionPreview)
	}
	completionApplied := applyCompletion(t, &out, caseRoot, completionPreview)
	if !completionApplied.Applied || completionApplied.CompletionReceipt == nil {
		t.Fatalf("terminal current-loop completion did not commit: %+v", completionApplied)
	}

	var statusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	var fresh statusInventory
	if err := json.Unmarshal(statusOut.Bytes(), &fresh); err != nil {
		t.Fatalf("terminal new-session status did not decode: %v\n%s", err, statusOut.String())
	}
	durable := fresh.MissionControlRunbook.CurrentLoopSegment
	if durable == nil || durable.Ready || durable.State != "stale-current-driver-request" || durable.Continuation != nil {
		t.Fatalf("new-session status recovered terminal budget after durable request drift: runbook=%+v checkpoint=%+v", fresh.MissionControlRunbook, durable)
	}

	var handoffOut bytes.Buffer
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, &handoffOut); err != nil {
		t.Fatal(err)
	}
	var handoff struct {
		CurrentLoopSegment *currentloop.Inspection `json:"currentLoopSegment"`
	}
	if err := json.Unmarshal(handoffOut.Bytes(), &handoff); err != nil {
		t.Fatalf("terminal drift handoff did not decode: %v\n%s", err, handoffOut.String())
	}
	if handoff.CurrentLoopSegment == nil || handoff.CurrentLoopSegment.Ready || handoff.CurrentLoopSegment.State != "stale-current-driver-request" || handoff.CurrentLoopSegment.Continuation != nil {
		t.Fatalf("new-session handoff recovered terminal budget after durable request drift: %+v", handoff.CurrentLoopSegment)
	}
}

func TestRunCurrentLoopPreservesCompletedReceiptWhenSecondStepFails(t *testing.T) {
	caseRoot := currentLoopCaseWithOpenIntervention(t, "int-current-loop-partial")
	preview := runCurrentLoopPreview(t, caseRoot, 2)
	currentLoopBeforeApplyStepHook = func(step int) error {
		if step == 2 {
			return errors.New("injected second-step failure")
		}
		return nil
	}
	t.Cleanup(func() { currentLoopBeforeApplyStepHook = nil })
	applied := runCurrentLoopApply(t, caseRoot, preview)
	if !applied.Applied || applied.AppliedSteps != 1 || len(applied.Steps) != 1 || applied.StopReason.Code != "error" || applied.StopReason.Phase != "apply-step" || !strings.Contains(applied.StopReason.Message, "step 2") || applied.FinalStatus == nil {
		t.Fatalf("second-step failure lost the completed receipt: %+v", applied)
	}
	if driverStepCommandName(applied.Steps[0].RequestBefore.Command) != "reconcile" || applied.Steps[0].CurrentStepReceipt == nil || applied.FinalStatus.MissionControlRunbook == nil || driverStepCommandName(applied.FinalStatus.MissionControlRunbook.CurrentDriverRequest.Command) != "continue" {
		t.Fatalf("partial loop result did not retain refreshed first-step state: %+v", applied)
	}
}

func TestRunCurrentLoopReportsAppliedStepWhenStatusRefreshFails(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	preview := runCurrentLoopPreview(t, caseRoot, 2)
	currentStepBeforeStatusRefreshHook = func(command string) error {
		if command != "run-driver-step" {
			t.Fatalf("unexpected nested command: %s", command)
		}
		return errors.New("injected refresh failure")
	}
	t.Cleanup(func() { currentStepBeforeStatusRefreshHook = nil })
	applied := runCurrentLoopApply(t, caseRoot, preview)
	if !applied.Applied || applied.AppliedSteps != 1 || len(applied.Steps) != 1 || applied.Steps[0].CurrentStepReceipt == nil || applied.Steps[0].CurrentStepReceipt.State != "refresh-failed" || applied.StopReason.Code != "error" || applied.StopReason.Phase != "refresh-status" || applied.FinalStatus != nil || applied.SegmentCheckpoint == nil || applied.SegmentCheckpoint.State != "status-unavailable" || applied.SegmentCheckpoint.Ready || applied.SegmentCheckpoint.Continuation != nil {
		t.Fatalf("applied step with refresh failure was not reported truthfully: %+v", applied)
	}
	var statusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	var fresh statusInventory
	if err := json.Unmarshal(statusOut.Bytes(), &fresh); err != nil {
		t.Fatalf("status-unavailable fresh status did not decode: %v\n%s", err, statusOut.String())
	}
	durable := fresh.MissionControlRunbook.CurrentLoopSegment
	if durable == nil || durable.State != "status-unavailable" || durable.Ready || durable.Continuation != nil || durable.ArtifactPath != applied.SegmentCheckpoint.ArtifactPath {
		t.Fatalf("fresh status recovered status-unavailable budget: %+v", durable)
	}
	var handoffOut bytes.Buffer
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, &handoffOut); err != nil {
		t.Fatal(err)
	}
	var handoff struct {
		CurrentLoopSegment *currentloop.Inspection `json:"currentLoopSegment"`
	}
	if err := json.Unmarshal(handoffOut.Bytes(), &handoff); err != nil {
		t.Fatalf("status-unavailable handoff did not decode: %v\n%s", err, handoffOut.String())
	}
	if handoff.CurrentLoopSegment == nil || handoff.CurrentLoopSegment.State != "status-unavailable" || handoff.CurrentLoopSegment.Ready || handoff.CurrentLoopSegment.Continuation != nil || handoff.CurrentLoopSegment.ArtifactPath != applied.SegmentCheckpoint.ArtifactPath {
		t.Fatalf("fresh handoff recovered status-unavailable budget: %+v", handoff.CurrentLoopSegment)
	}
}

func TestRunCurrentLoopStopsForExternalReviewerHandoffs(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "remote-evidence", "-Executor", "member-evidence-executor", "-Actor", "mission-commander", "-Reason", "seed exact reviewer evidence closure", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	manifestItem := currentLoopRemoteControlEvidenceItem(t, caseRoot, "feature-remote-evidence")
	out.Reset()
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Executor", "review-owner-executor", "-Actor", "mission-commander", "-Reason", "bind the durable Reviewer campaign owner", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", manifestItem, "-Lane", "feature-review"}, &out); err != nil {
		t.Fatal(err)
	}
	external := runCurrentLoopPreview(t, caseRoot, 10)
	if external.ExpectedCurrentLoopPlanSHA256 != "" || external.StopReason.Code != "external-reviewer-handoff" || external.StopReason.ExternalHandoff == nil || external.StopReason.ExternalHandoff.RunLoopStepID != "spawn-reviewer" {
		t.Fatalf("current-loop did not stop for reviewer handoff: %+v", external)
	}
	if external.ContinuationRequest == nil || external.ContinuationRequest.Kind != "current-loop-campaign-continuation" || external.ContinuationRequest.StopCode != "external-reviewer-handoff" || external.ContinuationRequest.SegmentRoute != "reviewer" || external.ContinuationRequest.ExpectedRoute != "reviewer" || external.ContinuationRequest.RemainingMaxSteps != 10 || external.ContinuationRequest.AppliedStepsInSegment != 0 || external.ContinuationRequest.WhatIfCommand != external.ResumeCommand || external.ContinuationRequest.CumulativeReceipts || external.ContinuationRequest.ObservationContract == nil || len(external.ContinuationRequest.ObservationContract.Alternatives) != 1 {
		t.Fatalf("spawn handoff omitted typed continuation request: %+v", external.ContinuationRequest)
	}
	spawnObservation := external.ContinuationRequest.ObservationContract.Alternatives[0]
	if spawnObservation.Kind != "reviewer-session-accepted" || !slices.Equal(spawnObservation.RequiredFlags, []string{"-ExpectedCurrentLoopReviewerAttemptSha256", "-ReviewerHarness", "-ReviewerSession", "-Actor"}) || !strings.Contains(spawnObservation.PreviewCommandTemplate, "-ExpectedCurrentLoopReviewerAttemptSha256 ") || spawnObservation.Transition != "refresh-status:save-result-input" {
		t.Fatalf("spawn continuation observation contract = %+v", spawnObservation)
	}
	if strings.Contains(external.ContinuationRequest.WhatIfCommand, "-ExpectedCurrentLoopReviewerAttemptSha256") {
		t.Fatalf("spawn continuation shared command carried alternative-specific attempt guard: %s", external.ContinuationRequest.WhatIfCommand)
	}
	statusOperator := external.FinalStatus.MissionControlRunbook.CurrentLoopOperator
	if statusOperator == nil || !statusOperator.Ready || statusOperator.State != "fresh-loop-review-required" || statusOperator.SelectedDriverRequest == nil || driverStepCommandName(statusOperator.SelectedDriverRequest.Command) != "run-current-loop" || statusOperator.SourceCurrentDriverRequest == nil || driverStepCommandName(statusOperator.SourceCurrentDriverRequest.Command) == "run-current-loop" || statusOperator.ExternalReviewerHandoff == nil || statusOperator.ExternalReviewerHandoff.AgentToolRequest == nil || !statusOperator.ExternalReviewerHandoff.AgentToolRequest.ReadOnly || len(statusOperator.ExternalReviewerHandoff.ObservationContract.Alternatives) != 1 {
		t.Fatalf("status current-loop operator did not package the reviewer spawn closure: %+v", statusOperator)
	}
	wave := statusOperator.ExternalReviewerHandoff.Wave
	if wave == nil || wave.SnapshotSHA256 == "" || wave.MaxParallel != 2 || wave.TotalShards != 1 || wave.ActiveSlots != 0 || wave.AvailableSlots != 2 || len(wave.SpawnWave) != 1 || len(wave.Shards) != 1 || wave.SpawnWave[0].Identity.ShardID != wave.Shards[0].Identity.ShardID || wave.SpawnWave[0].SelectedAction.AgentToolRequest == nil {
		t.Fatalf("status current-loop operator omitted reviewer wave closure: %+v", wave)
	}
	spawnAttempt := statusOperator.ExternalReviewerHandoff.Attempt
	if spawnAttempt == nil || spawnAttempt.SelectedAction.Kind != "invoke-reviewer-and-record-acceptance" || spawnAttempt.SelectedAction.Actor != "main-agent-harness" || spawnAttempt.Identity.PacketID == "" || spawnAttempt.Identity.RouteID == "" || spawnAttempt.Identity.ShardID == "" || spawnAttempt.Identity.Lane == "" || spawnAttempt.Identity.PromptSHA256 == "" || spawnAttempt.Identity.OwnerBindingMode == "" || spawnAttempt.Identity.CurrentExecutor != spawnAttempt.Identity.OwnerExecutor || spawnAttempt.Identity.CurrentGeneration != spawnAttempt.Identity.OwnerGeneration || spawnAttempt.CurrentReviewerDriverRequest == nil || spawnAttempt.DurableContinuationDriverRequest == nil || len(spawnAttempt.AttemptSnapshotSHA256) != 64 {
		t.Fatalf("status current-loop operator omitted reviewer attempt identity: %+v", spawnAttempt)
	}
	operatorSpawn := statusOperator.ExternalReviewerHandoff.ObservationContract.Alternatives[0]
	if operatorSpawn.Kind != "reviewer-session-accepted" || operatorSpawn.Transition != "refresh-status:save-result-input" || !strings.Contains(operatorSpawn.PreviewCommandTemplate, "-ExpectedCurrentLoopReviewerAttemptSha256 "+spawnAttempt.AttemptSnapshotSHA256) || !strings.Contains(operatorSpawn.PreviewCommandTemplate, "-ReviewerHarness <harness>") || !strings.Contains(operatorSpawn.PreviewCommandTemplate, "-ReviewerSession <session-id>") || !strings.Contains(operatorSpawn.PreviewCommandTemplate, "-Actor <main-agent>") {
		t.Fatalf("status current-loop operator spawn template = %+v", operatorSpawn)
	}
	if external.FinalStatus.MissionControlRunbook.Quickstart == nil || external.FinalStatus.MissionControlRunbook.Quickstart.CurrentLoopOperator == nil || external.FinalStatus.MissionControlRunbook.ReplacementExecutorTakeover == nil || external.FinalStatus.MissionControlRunbook.ReplacementExecutorTakeover.CurrentLoopOperator == nil {
		t.Fatalf("current-loop operator was not projected to quickstart and replacement takeover: %+v", external.FinalStatus.MissionControlRunbook)
	}
	handoffPreviewArgs, ok := typedMissionCommanderDriverRequestCommandCLIArgs(
		t,
		external.FinalStatus.MissionControlRunbook.HandoffPreviewDriverRequest,
	)
	if !ok {
		t.Fatal("reviewer status omitted typed handoff preview request")
	}
	var handoffPreviewOut bytes.Buffer
	if err := Run(handoffPreviewArgs, &handoffPreviewOut); err != nil {
		t.Fatal(err)
	}
	var handoffPreview workstream.HandoffResult
	if err := json.Unmarshal(handoffPreviewOut.Bytes(), &handoffPreview); err != nil {
		t.Fatalf("current-loop operator handoff preview did not decode: %v\n%s", err, handoffPreviewOut.String())
	}
	if handoffPreview.Selector != "feature-review" ||
		handoffPreview.DailyMissionControlRunbook == nil ||
		handoffPreview.DailyMissionControlRunbook.HandoffApplyDriverRequest == nil {
		t.Fatalf("handoff JSON omitted exact Reviewer target lane apply request: %+v", handoffPreview)
	}
	handoffApplyArgs, ok := typedMissionCommanderDriverRequestCommandCLIArgs(
		t,
		handoffPreview.DailyMissionControlRunbook.HandoffApplyDriverRequest,
	)
	if !ok {
		t.Fatal("reviewer handoff preview omitted typed Apply request")
	}
	var handoffApplyOut bytes.Buffer
	if err := Run(handoffApplyArgs, &handoffApplyOut); err != nil {
		t.Fatal(err)
	}
	durableHandoff, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "handovers", "feature-review-latest.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"## Current-loop operator", "selected driver request:", "reviewer attempt: id=", "snapshotSha256=", "Agent request:", "reviewer-session-accepted", "transition=refresh-status:save-result-input"} {
		if !strings.Contains(string(durableHandoff), expected) {
			t.Fatalf("durable handoff omitted current-loop operator detail %q:\n%s", expected, durableHandoff)
		}
	}
	out.Reset()
	err = Run([]string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-MaxSteps", "10", "-ExpectedCurrentLoopPlanSha256", strings.Repeat("0", 64), "-Apply", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "external or reviewed action") {
		t.Fatalf("external reviewer handoff accepted Apply: %v", err)
	}

	actor := "mission-commander"
	dispatchInputs := []string{"-ExpectedCurrentLoopReviewerAttemptSha256", spawnAttempt.AttemptSnapshotSHA256, "-ReviewerHarness", "go-cli-test-harness", "-ReviewerSession", "reviewer-session-runner", "-Actor", actor}
	staleDispatchInputs := append([]string{}, dispatchInputs...)
	staleDispatchInputs[1] = strings.Repeat("0", 64)
	out.Reset()
	err = Run(append([]string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-MaxSteps", "10", "-WhatIf", "-Format", "json"}, staleDispatchInputs...), &out)
	if err == nil || (!strings.Contains(err.Error(), "expected reviewer attempt sha256 mismatch") &&
		!strings.Contains(err.Error(), "requires a fresh current-loop reviewer attempt")) {
		t.Fatalf("stale reviewer attempt observation was not rejected before preview: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(filepath.Dir(spawnAttempt.Identity.PacketPath), "sessions", spawnAttempt.Identity.ShardID)); !os.IsNotExist(statErr) {
		t.Fatalf("stale reviewer attempt observation wrote a session receipt: %v", statErr)
	}
	dispatchPreview := runCurrentLoopPreviewWith(t, caseRoot, external.ContinuationRequest.RemainingMaxSteps, dispatchInputs...)
	if dispatchPreview.ExpectedCurrentLoopPlanSHA256 == "" || dispatchPreview.InitialRoute != "reviewer" {
		t.Fatalf("reviewer deterministic loop preview missing: %+v", dispatchPreview)
	}
	out.Reset()
	driftInputs := []string{"-ExpectedCurrentLoopReviewerAttemptSha256", spawnAttempt.AttemptSnapshotSHA256, "-ReviewerHarness", "go-cli-test-harness", "-ReviewerSession", "reviewer-session-runner", "-Actor", "replacement-commander"}
	driftArgs := []string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-MaxSteps", stringInt(dispatchPreview.MaxSteps)}
	driftArgs = append(driftArgs, driftInputs...)
	driftArgs = append(driftArgs, "-ExpectedCurrentLoopPlanSha256", dispatchPreview.ExpectedCurrentLoopPlanSHA256, "-Apply", "-Format", "json")
	err = Run(driftArgs, &out)
	if err == nil || !strings.Contains(err.Error(), "expected plan sha256 mismatch") {
		t.Fatalf("reviewer actor drift was not rejected: %v", err)
	}

	dispatchApplied := runCurrentLoopApplyWith(t, caseRoot, dispatchPreview, dispatchInputs...)
	if dispatchApplied.AppliedSteps != 1 || dispatchApplied.StopReason.Code != "external-reviewer-handoff" {
		t.Fatalf("reviewer loop did not stop after deterministic dispatch: %+v", dispatchApplied)
	}
	continuation := dispatchApplied.ContinuationRequest
	if continuation == nil || continuation.Kind != "current-loop-campaign-continuation" || continuation.StopCode != "external-reviewer-handoff" || continuation.SegmentRoute != "reviewer" || continuation.ExpectedRoute != "reviewer" || continuation.RemainingMaxSteps != dispatchPreview.MaxSteps-1 || continuation.AppliedStepsInSegment != 1 || continuation.SegmentMaxSteps != dispatchPreview.MaxSteps || continuation.CumulativeReceipts || !strings.Contains(continuation.WhatIfCommand, "-MaxSteps 9") {
		t.Fatalf("dispatch result omitted decremented continuation budget: %+v", continuation)
	}
	if dispatchApplied.StopReason.ExternalHandoff.ReviewerResultDropPathRole != "canonical-reviewer-input-destination" || dispatchApplied.StopReason.ExternalHandoff.ReviewerResultInputPath == "" || dispatchApplied.StopReason.ExternalHandoff.ReviewerResultDropPath != dispatchApplied.StopReason.ExternalHandoff.ReviewerResultInputPath {
		t.Fatalf("result handoff did not distinguish canonical input destination: %+v", dispatchApplied.StopReason.ExternalHandoff)
	}
	activeWaveOperator := dispatchApplied.FinalStatus.MissionControlRunbook.CurrentLoopOperator
	if activeWaveOperator == nil || activeWaveOperator.ExternalReviewerHandoff == nil {
		t.Fatalf("reviewer wave active status was not refreshed: %+v", activeWaveOperator)
	}
	resultWave := activeWaveOperator.ExternalReviewerHandoff.Wave
	if resultWave == nil || resultWave.ActiveSlots != 1 || resultWave.AvailableSlots != 1 || len(resultWave.Active) != 1 || len(resultWave.SpawnWave) != 0 || resultWave.Active[0].Receipt.Session != "reviewer-session-runner" || resultWave.Active[0].SelectedAction.AgentToolRequest != nil {
		t.Fatalf("reviewer wave did not project active slot after acceptance: %+v", resultWave)
	}
	if len(continuation.ObservationContract.Alternatives) != 2 || continuation.ObservationContract.Alternatives[0].Kind != "reviewer-result-returned" || continuation.ObservationContract.Alternatives[1].Kind != "reviewer-session-failed" {
		t.Fatalf("result handoff observation alternatives = %+v", continuation.ObservationContract)
	}
	returnedContinuation := continuation.ObservationContract.Alternatives[0]
	failedContinuation := continuation.ObservationContract.Alternatives[1]
	if strings.Contains(continuation.WhatIfCommand, "-ExpectedCurrentLoopReviewerAttemptSha256") || !strings.Contains(returnedContinuation.PreviewCommandTemplate, "-ExpectedCurrentLoopReviewerAttemptSha256 "+dispatchApplied.StopReason.ExpectedReviewerAttemptSHA256) || !strings.Contains(failedContinuation.PreviewCommandTemplate, "-ExpectedCurrentLoopReviewerAttemptSha256 "+dispatchApplied.StopReason.ExpectedReviewerAttemptSHA256) || failedContinuation.Transition != "refresh-status:spawn-reviewer" {
		t.Fatalf("result continuation did not separate shared command from guarded alternatives: shared=%s returned=%+v failed=%+v", continuation.WhatIfCommand, returnedContinuation, failedContinuation)
	}
	var operatorStatusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "feature-review", "-Format", "json"}, &operatorStatusOut); err != nil {
		t.Fatal(err)
	}
	var operatorStatus statusInventory
	if err := json.Unmarshal(operatorStatusOut.Bytes(), &operatorStatus); err != nil {
		t.Fatalf("reviewer operator status did not decode: %v\n%s", err, operatorStatusOut.String())
	}
	resultOperator := operatorStatus.MissionControlRunbook.CurrentLoopOperator
	if resultOperator == nil || resultOperator.State != "external-session-ready-for-attempt" || resultOperator.ResumeDriverRequest == nil || resultOperator.ExternalSessionJob == nil || resultOperator.ExternalSessionJob.AttemptRequest == nil || resultOperator.ExternalReviewerHandoff == nil || resultOperator.ExternalReviewerHandoff.ReviewerResultDropPathRole != "canonical-reviewer-input-destination" || len(resultOperator.ExternalReviewerHandoff.ObservationContract.Alternatives) != 2 {
		t.Fatalf("status current-loop operator did not package the reviewer result closure: %+v", resultOperator)
	}
	resultAttempt := resultOperator.ExternalReviewerHandoff.Attempt
	if resultAttempt == nil {
		t.Fatal("status omitted current reviewer attempt")
	}
	resultJob := resultOperator.ExternalSessionJob
	if resultJob == nil || resultJob.State != "awaiting-submission" || resultJob.SessionKind != "reviewer" || resultJob.Reviewer == nil || resultJob.Reviewer.AttemptSHA256 != resultAttempt.AttemptSnapshotSHA256 || resultJob.Reviewer.PacketID != resultAttempt.Identity.PacketID || resultJob.SubmissionResult == "" || resultJob.JobSHA256 == "" || !resultJob.SubmissionLast {
		t.Fatalf("status omitted typed external reviewer job: %+v", resultJob)
	}
	if harness := resultJob.HarnessPackage; harness == nil || harness.State != "attempt-review-required" || harness.AttemptReviewRequest == nil || harness.AttemptReviewRequest.Command != resultJob.AttemptRequest.Command || harness.Launch != nil || harness.Return != nil {
		t.Fatalf("fresh reviewer job omitted attempt-review harness package: %+v", harness)
	}
	resultJob = recordCurrentLoopExternalSessionAttempt(t, resultOperator, "go-cli-test-harness", "reviewer-session-runner", "mission-commander", "2026-08-05T00:30:00Z")
	if resultJob.AttemptRequest == nil || resultJob.AttemptRequest.State != "new-reviewer-dispatch-required" || resultJob.AttemptRequest.CommandExecutable || resultJob.AttemptRequest.Command != "" || !strings.Contains(resultJob.AttemptRequest.Guidance, "new Reviewer dispatch") {
		t.Fatalf("durable Reviewer attempt exposed same-job replacement: %+v", resultJob.AttemptRequest)
	}
	resultOperator.ExternalSessionJob = resultJob
	resultJob = acceptCurrentLoopExternalSessionLaunch(t, resultOperator, "dispatcher", "go-cli-test-harness", "reviewer-session-runner", "2026-08-05T00:30:01Z")
	if harness := resultJob.HarnessPackage; harness == nil || harness.State != "running" || harness.Launch == nil || harness.Launch.Ready || harness.Launch.Tool != "Claude Code Agent" || harness.Launch.AgentType != "read-only-reviewer" || !harness.Launch.ReadOnly || harness.Launch.Input.Role != "reviewer-dispatch-prompt" || harness.Launch.Input.Path != resultOperator.ExternalReviewerHandoff.DispatchPromptPath || harness.Launch.Input.SHA256 != resultOperator.ExternalReviewerHandoff.DispatchPromptSHA256 || harness.Launch.Attempt.AttemptSHA256 != resultJob.CurrentAttempt.AttemptSHA256 || harness.Launch.Attempt.Session != "reviewer-session-runner" || harness.Launch.ReviewerIdentity == nil || harness.Launch.ReviewerIdentity.PacketID != resultAttempt.Identity.PacketID || harness.Launch.ReviewerIdentity.RouteID != resultAttempt.Identity.RouteID || harness.Launch.ReviewerIdentity.ShardID != resultAttempt.Identity.ShardID || harness.Launch.ReviewerIdentity.DispatchID != resultAttempt.Receipt.DispatchID || harness.Launch.ReviewerIdentity.DispatchSHA256 != resultAttempt.Receipt.DispatchSHA256 || harness.Return == nil || harness.Return.SubmissionResult != resultJob.SubmissionResult || !harness.Return.SubmissionLast || len(harness.Return.Templates) != len(resultJob.AllowedOutcomes) {
		t.Fatalf("running reviewer job omitted exact harness launch/return package: %+v", harness)
	}
	if resultAttempt.AttemptID == spawnAttempt.AttemptID || resultAttempt.AttemptSnapshotSHA256 == spawnAttempt.AttemptSnapshotSHA256 || resultAttempt.RunLoopStepID != "save-result-input" || resultAttempt.SelectedAction.Kind != "observe-reviewer-terminal-state" || resultAttempt.Receipt.DispatchID == "" || resultAttempt.Receipt.Harness != "go-cli-test-harness" || resultAttempt.Receipt.Session != "reviewer-session-runner" || resultAttempt.Receipt.SessionLifecycleState == "" || resultAttempt.DurableContinuationDriverRequest == nil || !strings.Contains(resultAttempt.DurableContinuationDriverRequest.Command, dispatchApplied.SegmentCheckpoint.ArtifactSHA256) {
		t.Fatalf("checkpoint reviewer attempt did not preserve lifecycle identity: spawn=%+v result=%+v", spawnAttempt, resultAttempt)
	}
	if resultAttempt.SelectedAction.ObservationContract.Alternatives[0].Transition != "refresh-status:record-completion" || resultAttempt.SelectedAction.ObservationContract.Alternatives[1].Transition != "refresh-status:spawn-reviewer" {
		t.Fatalf("reviewer attempt transition contract = %+v", resultAttempt.SelectedAction.ObservationContract)
	}
	returnedTemplate := resultOperator.ExternalReviewerHandoff.ObservationContract.Alternatives[0].PreviewCommandTemplate
	failedTemplate := resultOperator.ExternalReviewerHandoff.ObservationContract.Alternatives[1].PreviewCommandTemplate
	if !strings.Contains(returnedTemplate, "-ResumeCurrentLoop") || !strings.Contains(returnedTemplate, dispatchApplied.SegmentCheckpoint.ArtifactSHA256) || !strings.Contains(returnedTemplate, "-ExpectedCurrentLoopReviewerAttemptSha256 "+resultAttempt.AttemptSnapshotSHA256) || !strings.Contains(returnedTemplate, "-ReviewerResultInputSourcePath <reviewer-result-source-path>") || !strings.Contains(failedTemplate, "-ReviewerOutcome failed") || !strings.Contains(failedTemplate, "-ReviewerExitStatus <exit-status>") {
		t.Fatalf("checkpoint-bound reviewer result templates are incomplete: returned=%s failed=%s", returnedTemplate, failedTemplate)
	}
	for _, alternative := range resultOperator.ExternalReviewerHandoff.ObservationContract.Alternatives {
		if !strings.Contains(alternative.ObservationEnvelopeTemplate, `"checkpointSha256": "`+dispatchApplied.SegmentCheckpoint.ArtifactSHA256+`"`) || !strings.Contains(alternative.ObservationEnvelopeTemplate, `"reviewerAttemptSha256": "`+resultAttempt.AttemptSnapshotSHA256+`"`) || !strings.Contains(alternative.ObservationPathCommand, "-CurrentLoopObservationPath") {
			t.Fatalf("checkpoint reviewer alternative omitted envelope intake: %+v", alternative)
		}
	}

	plan := decodePlanSubagentsPacket(t, dispatchApplied.FinalStatus.CaseMission.ReviewerDispatchIntakeSummary.LatestPacketPath)
	handoff := plan.ShardHandoffs[0]
	evidencePath := filepath.Join(caseRoot, "workspace", "features", "feature-login", "review-evidence.md")
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("bounded current-loop reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	reviewerSubmissionResult := reviewerResultForCLIPlan(t, plan, handoff, "accept", "accepted", "reviewer-session-runner")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(caseRoot, filepath.FromSlash(resultJob.SubmissionResult))), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, filepath.FromSlash(resultJob.SubmissionResult)), reviewerSubmissionResult, 0o600); err != nil {
		t.Fatal(err)
	}
	reviewerSubmission := map[string]any{
		"schemaVersion": 1, "kind": "current-loop-external-session-submission", "jobId": resultJob.JobID, "jobSha256": resultJob.JobSHA256,
		"outcome": "returned", "actor": "go-cli-test-harness", "observedAt": "2026-08-05T00:30:02Z", "reviewerSession": "reviewer-session-runner", "noAuthorityOrConfirmed": true, "noHeavyTool": true,
	}
	bindCurrentLoopExternalSubmissionAttempt(t, resultJob, reviewerSubmission)
	reviewerSubmissionData, _ := json.MarshalIndent(reviewerSubmission, "", "  ")
	if err := os.WriteFile(filepath.Join(caseRoot, filepath.FromSlash(resultJob.SubmissionPath)), append(reviewerSubmissionData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	var reviewerRelayStatusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "feature-review", "-Format", "json"}, &reviewerRelayStatusOut); err != nil {
		t.Fatal(err)
	}
	var reviewerRelayStatus statusInventory
	if err := json.Unmarshal(reviewerRelayStatusOut.Bytes(), &reviewerRelayStatus); err != nil {
		t.Fatal(err)
	}
	reviewerRelayOperator := reviewerRelayStatus.MissionControlRunbook.CurrentLoopOperator
	if reviewerRelayOperator.ExternalSessionJob == nil || reviewerRelayOperator.ExternalSessionJob.State != "submission-ready" || reviewerRelayOperator.SelectedDriverRequest == nil || !strings.Contains(reviewerRelayOperator.SelectedDriverRequest.Command, "-AdvanceExternalSessionResult") || reviewerRelayOperator.ExternalSessionJob.RelayPreviewRequest == nil {
		t.Fatalf("status did not select reviewer external-result turn with relay recovery: %+v", reviewerRelayOperator)
	}
	if harness := reviewerRelayOperator.ExternalSessionJob.HarnessPackage; harness == nil || harness.State != "return-review-required" || harness.Launch == nil || harness.Launch.Ready || harness.Return == nil || harness.Return.ReviewRequest == nil || harness.Return.ReviewRequest.Command != reviewerRelayOperator.SelectedDriverRequest.Command || harness.Return.RelayRecoveryRequest == nil || harness.Return.RelayRecoveryRequest.Command != reviewerRelayOperator.ExternalSessionJob.RelayPreviewRequest.Command {
		t.Fatalf("reviewer submission-ready package omitted reviewed return and relay recovery requests: %+v", harness)
	}
	reviewerCurrent := runMemberCurrentStepForLane(t, caseRoot, "feature-review", []string{"-WhatIf"})
	if reviewerCurrent.Route != "reviewer" || reviewerCurrent.ExternalSessionStep == nil || reviewerCurrent.ExternalSessionStep.Mode != "result-turn" || reviewerCurrent.ExternalSessionStep.Turn == nil || reviewerCurrent.ExpectedCurrentStepPlanSHA256 == "" {
		t.Fatalf("reviewer external result did not enter unified current-step route: %+v", reviewerCurrent)
	}
	var reviewerRelayPreview externalsession.Plan
	reviewerRelayStatusOut.Reset()
	if err := Run(rekitCommandCLIArgs(t, reviewerRelayOperator.ExternalSessionJob.RelayPreviewRequest.Command), &reviewerRelayStatusOut); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(reviewerRelayStatusOut.Bytes(), &reviewerRelayPreview); err != nil || reviewerRelayPreview.ApplyCommand == "" {
		t.Fatalf("reviewer relay preview=%+v err=%v", reviewerRelayPreview, err)
	}
	var reviewerRelayApplied currentLoopExternalSessionRelayResult
	reviewerRelayStatusOut.Reset()
	if err := Run(rekitCommandCLIArgs(t, reviewerRelayPreview.ApplyCommand), &reviewerRelayStatusOut); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(reviewerRelayStatusOut.Bytes(), &reviewerRelayApplied); err != nil || !reviewerRelayApplied.Applied || reviewerRelayApplied.RefreshedStatus == nil || reviewerRelayApplied.RefreshedStatus.MissionControlRunbook.CurrentLoopOperator.ObservationInbox.State != "ready" {
		t.Fatalf("reviewer relay apply=%+v err=%v output=%s", reviewerRelayApplied, err, reviewerRelayStatusOut.String())
	}
	if got, err := os.ReadFile(filepath.Join(caseRoot, filepath.FromSlash(resultJob.RelayResultPath))); err != nil || !bytes.Equal(got, reviewerSubmissionResult) {
		t.Fatalf("reviewer relay source bytes drifted: %v", err)
	}
	if err := os.Remove(filepath.Join(caseRoot, filepath.FromSlash(resultJob.ObservationPath))); err != nil {
		t.Fatal(err)
	}

	failedObservationPath := writeCurrentLoopObservation(t, caseRoot, "reviewer-failed", currentLoopObservationEnvelope{
		SchemaVersion: 1, Kind: "current-loop-external-session-observation", CheckpointSHA256: dispatchApplied.SegmentCheckpoint.ArtifactSHA256,
		ObservationKind: "reviewer-session-failed", Actor: actor, ReviewerAttemptSHA256: resultAttempt.AttemptSnapshotSHA256, ReviewerExitStatus: "reviewer-error",
		NoAuthorityOrConfirmed: true, NoHeavyTool: true,
	})
	failedPreview := runCurrentLoopResumePreviewWith(t, caseRoot, dispatchApplied.SegmentCheckpoint.ArtifactSHA256, "-Lane", "feature-review", "-CurrentLoopObservationPath", failedObservationPath)
	if failedPreview.ObservationSHA256 == "" || !strings.Contains(failedPreview.ApplyCommand, "-CurrentLoopObservationPath") || strings.Contains(failedPreview.ApplyCommand, "-ReviewerOutcome") || strings.Contains(failedPreview.ApplyCommand, "-Actor") {
		t.Fatalf("failed reviewer envelope preview did not preserve path-only apply: %+v", failedPreview)
	}
	failedApplied := runCurrentLoopResult(t, rekitCommandCLIArgs(t, failedPreview.ApplyCommand))
	if failedApplied.AppliedSteps != 1 || failedApplied.StopReason.Code != "external-reviewer-handoff" || failedApplied.SegmentCheckpoint == nil {
		t.Fatalf("failed reviewer attempt did not return a replacement handoff: %+v", failedApplied)
	}
	replacementOperator := failedApplied.FinalStatus.MissionControlRunbook.CurrentLoopOperator
	if replacementOperator == nil || replacementOperator.ExternalReviewerHandoff == nil || replacementOperator.ExternalReviewerHandoff.Attempt == nil {
		t.Fatalf("failed reviewer attempt omitted replacement spawn attempt: %+v", replacementOperator)
	}
	replacementSpawnAttempt := replacementOperator.ExternalReviewerHandoff.Attempt
	if replacementSpawnAttempt.RunLoopStepID != "spawn-reviewer" || replacementSpawnAttempt.Receipt.DispatchID != resultAttempt.Receipt.DispatchID || replacementSpawnAttempt.Receipt.CompletionOutcome != "failed" {
		t.Fatalf("failed reviewer attempt did not advance to fresh spawn while preserving prior receipt provenance: %+v", replacementSpawnAttempt)
	}
	replacementDispatchObservationPath := writeCurrentLoopObservation(t, caseRoot, "reviewer-accepted", currentLoopObservationEnvelope{
		SchemaVersion: 1, Kind: "current-loop-external-session-observation", CheckpointSHA256: failedApplied.SegmentCheckpoint.ArtifactSHA256,
		ObservationKind: "reviewer-session-accepted", Actor: actor, ReviewerAttemptSHA256: replacementSpawnAttempt.AttemptSnapshotSHA256, ReviewerHarness: externalsession.RemoteControlHarness, ReviewerSession: "reviewer-session-replacement",
		NoAuthorityOrConfirmed: true, NoHeavyTool: true,
	})
	replacementDispatchPreview := runCurrentLoopResumePreviewWith(t, caseRoot, failedApplied.SegmentCheckpoint.ArtifactSHA256, "-Lane", "feature-review", "-CurrentLoopObservationPath", replacementDispatchObservationPath)
	replacementDispatchApplied := runCurrentLoopResult(t, rekitCommandCLIArgs(t, replacementDispatchPreview.ApplyCommand))
	if replacementDispatchApplied.AppliedSteps != 1 || replacementDispatchApplied.StopReason.Code != "external-reviewer-handoff" || replacementDispatchApplied.SegmentCheckpoint == nil {
		t.Fatalf("replacement reviewer dispatch did not reach result handoff: %+v", replacementDispatchApplied)
	}
	replacementResultAttempt := replacementDispatchApplied.FinalStatus.MissionControlRunbook.CurrentLoopOperator.ExternalReviewerHandoff.Attempt
	if replacementResultAttempt == nil || replacementResultAttempt.RunLoopStepID != "save-result-input" || replacementResultAttempt.Receipt.Harness != externalsession.RemoteControlHarness || replacementResultAttempt.Receipt.Session != "reviewer-session-replacement" || replacementResultAttempt.Receipt.DispatchID == resultAttempt.Receipt.DispatchID {
		t.Fatalf("replacement result attempt did not bind the fresh Remote Control reviewer session: prior=%+v replacement=%+v", resultAttempt, replacementResultAttempt)
	}
	replacementDispatch, err := reviewersession.ReadDispatch(caseRoot, replacementResultAttempt.Receipt.DispatchPath, replacementResultAttempt.Receipt.DispatchSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if replacementDispatch.ReviewerHarness != externalsession.RemoteControlHarness || replacementDispatch.ReviewerSession != "reviewer-session-replacement" || !replacementDispatch.ReadOnly || !replacementDispatch.NoSpawn || !replacementDispatch.NoHeavyTool || !replacementDispatch.NoAuthority || !slices.Equal(replacementDispatch.Items, []string{manifestItem}) {
		t.Fatalf("replacement Remote Control dispatch receipt lost strict read-only evidence binding: %+v", replacementDispatch)
	}

	staleResultSource := filepath.Join(
		plan.ReviewerOrchestration.ResultRoot,
		"returned-results",
		"current-loop-stale-reviewer-result.json",
	)
	if err := os.MkdirAll(filepath.Dir(staleResultSource), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staleResultSource, reviewerResultForCLIPlan(t, plan, handoff, "accept", "accepted", "reviewer-session-runner"), 0o644); err != nil {
		t.Fatal(err)
	}
	staleResultPreview := runCurrentLoopResumePreviewWith(t, caseRoot, replacementDispatchApplied.SegmentCheckpoint.ArtifactSHA256,
		"-Lane", "feature-review",
		"-ExpectedCurrentLoopReviewerAttemptSha256", replacementResultAttempt.AttemptSnapshotSHA256,
		"-ReviewerResultInputSourcePath", staleResultSource,
		"-Actor", actor,
	)
	if staleResultPreview.ExpectedCurrentLoopPlanSHA256 != "" || staleResultPreview.StopReason.Code != "requires-review" || !strings.Contains(staleResultPreview.StopReason.Message, "current reviewer dispatch") {
		t.Fatalf("late failed-session reviewer result was not rejected before input save: %+v", staleResultPreview)
	}
	if _, err := os.Stat(replacementResultAttempt.ReviewerResultInputPath); !os.IsNotExist(err) {
		t.Fatalf("late failed-session reviewer result occupied canonical input: %v", err)
	}

	var replacementStatusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "feature-review", "-Format", "json"}, &replacementStatusOut); err != nil {
		t.Fatal(err)
	}
	var replacementStatus statusInventory
	decodeJSONStrict(t, replacementStatusOut.Bytes(), &replacementStatus)
	replacementResultOperator := replacementStatus.MissionControlRunbook.CurrentLoopOperator
	replacementResultJob := replacementResultOperator.ExternalSessionJob
	if replacementResultJob == nil || replacementResultJob.State != "awaiting-submission" || replacementResultJob.SessionKind != "reviewer" || replacementResultJob.Reviewer == nil || replacementResultJob.Reviewer.AttemptSHA256 != replacementResultAttempt.AttemptSnapshotSHA256 || replacementResultJob.Reviewer.Harness != externalsession.RemoteControlHarness || replacementResultJob.Reviewer.Session != "reviewer-session-replacement" {
		t.Fatalf("replacement Remote Control reviewer external job is invalid: %+v", replacementResultJob)
	}

	attemptInputs := []string{
		"-ExternalSessionHarness", externalsession.RemoteControlHarness,
		"-ExternalSessionId", "reviewer-session-replacement",
		"-ExternalSessionActor", actor,
		"-ExternalSessionStartedAt", "2026-08-12T10:00:00Z",
	}
	attemptPreview := runMemberCurrentStepForLane(t, caseRoot, "feature-review", append(append([]string{}, attemptInputs...), "-WhatIf"))
	if attemptPreview.Route != "reviewer" || attemptPreview.ExternalSessionStep == nil || attemptPreview.ExternalSessionStep.Mode != "attempt" || attemptPreview.ExternalSessionStep.Attempt == nil || attemptPreview.ExpectedCurrentStepPlanSHA256 == "" || attemptPreview.ExternalSessionStep.Attempt.ExpectedPlanSHA256 == "" || attemptPreview.ExternalSessionStep.Attempt.Attempt.Harness != externalsession.RemoteControlHarness || attemptPreview.ExternalSessionStep.Attempt.Attempt.Session != "reviewer-session-replacement" {
		t.Fatalf("Remote Control attempt preview omitted exact durable binding: %+v", attemptPreview)
	}
	attemptApplied := runCurrentStepApply(t, caseRoot, attemptPreview, attemptInputs...)
	if attemptApplied.ExternalSessionStep == nil || attemptApplied.ExternalSessionStep.Attempt == nil || !attemptApplied.ExternalSessionStep.Attempt.Applied || attemptApplied.RefreshedStatus == nil || attemptApplied.RefreshedStatus.MissionControlRunbook.CurrentLoopOperator.ExternalSessionJob.Dispatcher.State != "queued" {
		t.Fatalf("Remote Control attempt Apply did not reach queued dispatch: %+v", attemptApplied)
	}

	claimInputs := []string{"-ExternalSessionActor", actor, "-ExternalSessionObservedAt", "2026-08-12T10:00:01Z"}
	claimPreview := runMemberCurrentStepForLane(t, caseRoot, "feature-review", append(append([]string{}, claimInputs...), "-WhatIf"))
	if claimPreview.ExternalSessionStep == nil || claimPreview.ExternalSessionStep.Mode != "dispatch-claim" || claimPreview.ExternalSessionStep.Dispatch == nil || claimPreview.ExternalSessionStep.Dispatch.Outcome != "claimed" || claimPreview.ExternalSessionStep.Dispatch.ExpectedPlanSHA256 == "" {
		t.Fatalf("Remote Control dispatch claim preview is incomplete: %+v", claimPreview)
	}
	claimApplied := runCurrentStepApply(t, caseRoot, claimPreview, claimInputs...)
	if claimApplied.RefreshedStatus == nil || claimApplied.RefreshedStatus.MissionControlRunbook.CurrentLoopOperator.ExternalSessionJob.Dispatcher.State != "claimed" {
		t.Fatalf("Remote Control dispatch claim Apply did not reach claimed: %+v", claimApplied)
	}

	discovery := runMemberCurrentStepForLane(t, caseRoot, "feature-review", []string{"-WhatIf"})
	if discovery.ExternalSessionStep == nil || discovery.ExternalSessionStep.Mode != "remote-control-discovery-input" || discovery.ExpectedCurrentStepPlanSHA256 != "" || !slices.Contains(discovery.ExternalSessionStep.InputRequired, "run ListAgents") || discovery.ExternalSessionStep.HarnessPackage == nil {
		t.Fatalf("Remote Control discovery input stop is invalid: %+v", discovery)
	}
	endpointInputs := []string{
		"-ExternalSessionTransportEndpoint", "reviewer [opaque-ref]",
		"-ExternalSessionActor", actor,
		"-ExternalSessionObservedAt", "2026-08-12T10:00:02Z",
	}
	endpointPreview := runMemberCurrentStepForLane(t, caseRoot, "feature-review", append(append([]string{}, endpointInputs...), "-WhatIf"))
	transportEndpoint := endpointPreview.ExternalSessionStep
	if transportEndpoint == nil || transportEndpoint.Mode != "remote-control-endpoint" || transportEndpoint.Transport == nil || transportEndpoint.Transport.Endpoint == nil || endpointPreview.ExpectedCurrentStepPlanSHA256 == "" || transportEndpoint.Transport.ExpectedPlanSHA256 == "" || transportEndpoint.Transport.Endpoint.DiscoveryTool != "ListAgents" || transportEndpoint.Transport.Endpoint.Envelope.Operation != "SendMessage" || transportEndpoint.Transport.Endpoint.Envelope.Recipient != "reviewer [opaque-ref]" || !transportEndpoint.Transport.Endpoint.Envelope.NoFileTransfer || transportEndpoint.Transport.BundlePath == "" || transportEndpoint.Transport.BundleSHA256 == "" || transportEndpoint.Transport.BundleBytes == 0 || !transportEndpoint.Transport.Endpoint.NoSessionManagement || !transportEndpoint.Transport.Endpoint.NoAutomaticRetry || !transportEndpoint.Transport.Endpoint.NoHeavyTool || !transportEndpoint.Transport.Endpoint.NoAuthority || !transportEndpoint.Transport.Endpoint.NoConfirmed || strings.Contains(strings.ToLower(transportEndpoint.Transport.Endpoint.Envelope.Message), strings.ToLower(caseRoot)) {
		t.Fatalf("Remote Control endpoint preview omitted self-contained transport closure: %+v", endpointPreview)
	}
	message := transportEndpoint.Transport.Endpoint.Envelope.Message
	const bundleStart = "--- BEGIN COMMITTED EVIDENCE BUNDLE ---\n"
	const bundleEnd = "\n--- END COMMITTED EVIDENCE BUNDLE ---"
	start := strings.Index(message, bundleStart)
	end := strings.Index(message, bundleEnd)
	if start < 0 || end <= start {
		t.Fatalf("Remote Control endpoint message omitted inline evidence bundle")
	}
	var transportBundle externalsession.TransportEvidenceBundle
	if err := json.Unmarshal([]byte(message[start+len(bundleStart):end]), &transportBundle); err != nil {
		t.Fatalf("Remote Control endpoint bundle did not decode: %v", err)
	}
	promptSource, err := os.ReadFile(handoff.DispatchPromptPath)
	if err != nil {
		t.Fatal(err)
	}
	if transportBundle.Prompt.SourceSHA256 != statusSHA256Hex(promptSource) || transportBundle.Prompt.SourceBytes != len(promptSource) ||
		transportBundle.Prompt.TransportedSHA256 != statusSHA256Hex([]byte(transportBundle.Prompt.Content)) || transportBundle.Prompt.TransportedBytes != len(transportBundle.Prompt.Content) ||
		transportBundle.Prompt.SourceSHA256 == transportBundle.Prompt.TransportedSHA256 || transportBundle.Prompt.SourceBytes <= transportBundle.Prompt.TransportedBytes ||
		!strings.Contains(transportBundle.Prompt.Content, "Read-only Reviewer for a committed plan-subagents evidence closure") ||
		!strings.Contains(transportBundle.Prompt.Content, "member-task-context") || !strings.Contains(transportBundle.Prompt.Content, "recommendedVerdict=rejected") ||
		transportEndpoint.Transport.BundleBytes > 56*1024 || transportEndpoint.Transport.Endpoint.Envelope.MessageBytes > 64*1024 {
		t.Fatalf("Remote Control canonical prompt transport identity or budget drifted: prompt=%+v bundle=%d message=%d", transportBundle.Prompt, transportEndpoint.Transport.BundleBytes, transportEndpoint.Transport.Endpoint.Envelope.MessageBytes)
	}
	for _, rel := range []string{transportEndpoint.Transport.BundlePath, transportEndpoint.Transport.ArtifactPath} {
		assertFileNotExists(t, filepath.Join(caseRoot, filepath.FromSlash(rel)))
	}
	endpointApplied := runCurrentStepApply(t, caseRoot, endpointPreview, endpointInputs...)
	if endpointApplied.RefreshedStatus == nil || endpointApplied.RefreshedStatus.MissionControlRunbook.CurrentLoopOperator.State != "remote-control-delivery-required" {
		t.Fatalf("Remote Control endpoint Apply did not require exact delivery: %+v", endpointApplied)
	}
	for _, rel := range []string{transportEndpoint.Transport.BundlePath, transportEndpoint.Transport.ArtifactPath} {
		assertRegularNonSymlinkFile(t, filepath.Join(caseRoot, filepath.FromSlash(rel)))
	}

	deliveryInputs := []string{
		"-ExternalSessionDeliveryOutcome", "accepted",
		"-ExternalSessionProviderAckFingerprint", strings.Repeat("e", 64),
		"-ExternalSessionActor", actor,
		"-ExternalSessionObservedAt", "2026-08-12T10:00:03Z",
	}
	deliveryPreview := runMemberCurrentStepForLane(t, caseRoot, "feature-review", append(append([]string{}, deliveryInputs...), "-WhatIf"))
	transportDelivery := deliveryPreview.ExternalSessionStep
	if transportDelivery == nil || transportDelivery.Mode != "remote-control-delivery" || transportDelivery.Transport == nil || transportDelivery.Transport.Delivery == nil || deliveryPreview.ExpectedCurrentStepPlanSHA256 == "" || transportDelivery.Transport.Delivery.Operation != "SendMessage" || transportDelivery.Transport.Delivery.Outcome != "accepted" || transportDelivery.Transport.Delivery.EndpointSnapshotSHA256 != transportEndpoint.Transport.ArtifactSHA256 || transportDelivery.Transport.Delivery.EnvelopeSHA256 == "" || !transportDelivery.Transport.Delivery.NoSessionManagement || !transportDelivery.Transport.Delivery.NoAutomaticRetry || !transportDelivery.Transport.Delivery.NoHeavyTool || !transportDelivery.Transport.Delivery.NoAuthority || !transportDelivery.Transport.Delivery.NoConfirmed {
		t.Fatalf("Remote Control delivery preview omitted exact endpoint/message binding: %+v", deliveryPreview)
	}
	assertFileNotExists(t, filepath.Join(caseRoot, filepath.FromSlash(transportDelivery.Transport.ArtifactPath)))
	deliveryApplied := runCurrentStepApply(t, caseRoot, deliveryPreview, deliveryInputs...)
	if deliveryApplied.RefreshedStatus == nil || deliveryApplied.RefreshedStatus.MissionControlRunbook.CurrentLoopOperator.ExternalSessionJob.Transport.State != "delivery-accepted" {
		t.Fatalf("Remote Control accepted delivery Apply did not retain transport truth: %+v", deliveryApplied)
	}

	launchPreview := runMemberCurrentStepForLane(t, caseRoot, "feature-review", []string{"-WhatIf"})
	if launchPreview.ExternalSessionStep == nil || launchPreview.ExternalSessionStep.Mode != "launch-accepted" || launchPreview.ExternalSessionStep.Dispatch == nil || launchPreview.ExternalSessionStep.Dispatch.Actor != actor || launchPreview.ExternalSessionStep.Dispatch.ObservedAt != "2026-08-12T10:00:03Z" || launchPreview.ExternalSessionStep.Dispatch.ActualHarness != externalsession.RemoteControlHarness || launchPreview.ExternalSessionStep.Dispatch.ActualSession != "reviewer-session-replacement" {
		t.Fatalf("Remote Control launch preview was not derived from accepted delivery: %+v", launchPreview)
	}
	launchApplied := runCurrentStepApply(t, caseRoot, launchPreview)
	if launchApplied.RefreshedStatus == nil || launchApplied.RefreshedStatus.MissionControlRunbook.CurrentLoopOperator.State != "remote-control-reviewer-result-required" || launchApplied.RefreshedStatus.MissionControlRunbook.CurrentLoopOperator.ExternalSessionJob.Dispatcher.State != "running" || launchApplied.RefreshedStatus.MissionControlRunbook.CurrentLoopOperator.ExternalSessionJob.Transport.ReturnRequest == nil || launchApplied.RefreshedStatus.MissionControlRunbook.CurrentLoopOperator.ExternalSessionJob.Transport.ReturnRequest.CommandExecutable {
		t.Fatalf("Remote Control launch Apply did not expose a non-executable return request: %+v", launchApplied)
	}
	runningJob := launchApplied.RefreshedStatus.MissionControlRunbook.CurrentLoopOperator.ExternalSessionJob
	if runningJob.CurrentAttempt == nil {
		t.Fatalf("Remote Control running job omitted current attempt destinations: %+v", runningJob)
	}
	currentAttempt := runningJob.CurrentAttempt

	replacementResultBytes := reviewerResultForCLIPlan(t, plan, handoff, "accept", "accepted", "reviewer-session-replacement")
	replacementResultSource := filepath.Join(caseRoot, "workspace", "remote-control-reviewer-result.json")
	if err := os.WriteFile(replacementResultSource, replacementResultBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	returnInputs := []string{
		"-ExternalSessionReviewerResultSourcePath", replacementResultSource,
		"-ExternalSessionActor", actor,
		"-ExternalSessionObservedAt", "2026-08-12T10:00:04Z",
	}
	returnPreview := runMemberCurrentStepForLane(t, caseRoot, "feature-review", append(append([]string{}, returnInputs...), "-WhatIf"))
	transportReturn := returnPreview.ExternalSessionStep
	if transportReturn == nil || transportReturn.Mode != "remote-control-return" || transportReturn.TransportReturn == nil {
		t.Fatalf("Remote Control return preview omitted typed plan: %+v", returnPreview)
	}
	returnPlan := transportReturn.TransportReturn
	checks := []struct {
		name  string
		valid bool
	}{
		{"outer plan hash", returnPreview.ExpectedCurrentStepPlanSHA256 != ""},
		{"source hash", returnPlan.SourceSHA256 == statusSHA256Hex(replacementResultBytes)},
		{"result hash", returnPlan.ResultSHA256 == returnPlan.SourceSHA256},
		{"source bytes", returnPlan.SourceBytes == len(replacementResultBytes)},
		{"result bytes", returnPlan.ResultBytes == len(replacementResultBytes)},
		{"result path", returnPlan.ResultPath == currentAttempt.SubmissionResult},
		{"submission path", returnPlan.SubmissionPath == currentAttempt.SubmissionPath},
		{"generation result path", returnPlan.ResultPath != replacementResultJob.SubmissionResult},
		{"generation submission path", returnPlan.SubmissionPath != replacementResultJob.SubmissionPath},
		{"return receipt path", returnPlan.ReturnReceiptPath != ""},
		{"receipt result path", returnPlan.ReturnReceipt.ResultPath == returnPlan.ResultPath},
		{"submission receipt path", returnPlan.Submission.TransportReturnReceiptPath == returnPlan.ReturnReceiptPath},
		{"submission receipt hash", returnPlan.Submission.TransportReturnReceiptSHA256 == returnPlan.ReturnReceiptSHA256},
		{"delivery lineage", returnPlan.ReturnReceipt.DeliverySHA256 == transportDelivery.Transport.ArtifactSHA256},
		{"launch lineage", returnPlan.ReturnReceipt.LaunchReceiptSHA256 != ""},
		{"no session management", returnPlan.ReturnReceipt.NoSessionManagement},
		{"no heavy tool", returnPlan.ReturnReceipt.NoHeavyTool},
		{"no authority", returnPlan.ReturnReceipt.NoAuthority},
		{"no confirmed", returnPlan.ReturnReceipt.NoConfirmed},
		{"submission outcome", returnPlan.Submission.Outcome == "returned"},
		{"submission harness", returnPlan.Submission.ReviewerHarness == externalsession.RemoteControlHarness},
		{"submission session", returnPlan.Submission.ReviewerSession == "reviewer-session-replacement"},
		{"submission no authority", returnPlan.Submission.NoAuthorityOrConfirmed},
		{"submission no heavy tool", returnPlan.Submission.NoHeavyTool},
	}
	for _, check := range checks {
		if !check.valid {
			t.Fatalf("Remote Control return preview failed %s: return=%+v job=%+v attempt=%+v", check.name, returnPlan, replacementResultJob, currentAttempt)
		}
	}
	for _, rel := range []string{transportReturn.TransportReturn.ResultPath, transportReturn.TransportReturn.ReturnReceiptPath, transportReturn.TransportReturn.SubmissionPath} {
		assertFileNotExists(t, filepath.Join(caseRoot, filepath.FromSlash(rel)))
	}
	returnApplied := runCurrentStepApply(t, caseRoot, returnPreview, returnInputs...)
	if returnApplied.ExternalSessionStep == nil || returnApplied.ExternalSessionStep.TransportReturn == nil || !returnApplied.ExternalSessionStep.TransportReturn.Applied || returnApplied.RefreshedStatus == nil {
		t.Fatalf("Remote Control return Apply omitted deterministic receipt: %+v", returnApplied)
	}
	replacementResultJob = returnApplied.RefreshedStatus.MissionControlRunbook.CurrentLoopOperator.ExternalSessionJob
	if replacementResultJob == nil || replacementResultJob.State != "submission-ready" {
		t.Fatalf("Remote Control return Apply did not reach submission-ready: %+v", replacementResultJob)
	}
	for _, rel := range []string{transportReturn.TransportReturn.ResultPath, transportReturn.TransportReturn.ReturnReceiptPath, transportReturn.TransportReturn.SubmissionPath} {
		assertRegularNonSymlinkFile(t, filepath.Join(caseRoot, filepath.FromSlash(rel)))
	}
	if got, err := os.ReadFile(filepath.Join(caseRoot, filepath.FromSlash(transportReturn.TransportReturn.ResultPath))); err != nil || !bytes.Equal(got, replacementResultBytes) {
		t.Fatalf("Remote Control return changed inbound ReviewerResult bytes: %v", err)
	}

	replacementStatusOut.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "feature-review", "-Format", "json"}, &replacementStatusOut); err != nil {
		t.Fatal(err)
	}
	decodeJSONStrict(t, replacementStatusOut.Bytes(), &replacementStatus)
	replacementResultOperator = replacementStatus.MissionControlRunbook.CurrentLoopOperator
	if replacementResultOperator.ExternalSessionJob == nil || replacementResultOperator.ExternalSessionJob.State != "submission-ready" || replacementResultOperator.SelectedDriverRequest == nil || !strings.Contains(replacementResultOperator.SelectedDriverRequest.Command, "-AdvanceExternalSessionResult") {
		t.Fatalf("replacement reviewer result did not select the composite turn: %+v", replacementResultOperator)
	}
	var turnOut bytes.Buffer
	if err := Run(rekitCommandCLIArgs(t, replacementResultOperator.SelectedDriverRequest.Command), &turnOut); err != nil {
		t.Fatal(err)
	}
	var turn currentLoopExternalSessionTurnPlan
	decodeJSONStrict(t, turnOut.Bytes(), &turn)
	if turn.ExpectedPlanSHA256 == "" || turn.ApplyCommand == "" || turn.Relay.ReviewerResult == nil || turn.Relay.ReviewerResult.Path != replacementResultJob.RelayResultPath || turn.Relay.ReviewerResult.SHA256 != statusSHA256Hex(replacementResultBytes) || turn.Resume.ExpectedCurrentLoopPlanSHA256 == "" || turn.Resume.InitialRoute != "reviewer" || turn.Resume.InitialCurrentStep == nil || turn.Resume.InitialCurrentStep.ReviewerStep == nil || turn.Resume.InitialCurrentStep.CurrentDriverRequest.RunLoopStepID != "save-result-input" {
		t.Fatalf("reviewer external-result turn omitted exact relay and resume binding: %+v", turn)
	}
	for _, path := range []string{
		filepath.Join(caseRoot, filepath.FromSlash(replacementResultJob.RelayResultPath)),
		filepath.Join(caseRoot, filepath.FromSlash(replacementResultJob.ObservationPath)),
		replacementResultAttempt.ReviewerResultInputPath,
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("reviewer external-result WhatIf wrote %s: %v", path, err)
		}
	}
	turnOut.Reset()
	if err := Run(rekitCommandCLIArgs(t, turn.ApplyCommand), &turnOut); err != nil {
		t.Fatal(err)
	}
	var turnApplied currentLoopExternalSessionTurnPlan
	decodeJSONStrict(t, turnOut.Bytes(), &turnApplied)
	resultApplied := turnApplied.Resume
	if !turnApplied.Applied || !turnApplied.Relay.Applied || !resultApplied.Applied || resultApplied.AppliedSteps != 6 || resultApplied.StopReason.Code != "route-policy" || resultApplied.FinalStatus == nil || resultApplied.FinalStatus.MissionControlRunbook == nil || resultApplied.FinalStatus.MissionControlRunbook.Scope != "case" {
		var finalScope, finalStep string
		if resultApplied.FinalStatus != nil && resultApplied.FinalStatus.MissionControlRunbook != nil {
			finalScope = resultApplied.FinalStatus.MissionControlRunbook.Scope
			finalStep = resultApplied.FinalStatus.MissionControlRunbook.CurrentRunLoopStepID
		}
		t.Fatalf("reviewer external-result turn did not finish deterministic pipeline: turnApplied=%t relayApplied=%t steps=%d stop=%+v finalScope=%s finalStep=%s", turnApplied.Applied, turnApplied.Relay.Applied, resultApplied.AppliedSteps, resultApplied.StopReason, finalScope, finalStep)
	}
	if got, err := os.ReadFile(filepath.Join(caseRoot, filepath.FromSlash(replacementResultJob.RelayResultPath))); err != nil || !bytes.Equal(got, replacementResultBytes) {
		t.Fatalf("reviewer composite turn relay bytes drifted: %v", err)
	}
	completionPath := reviewersession.CompletionPath(replacementResultAttempt.Identity.PacketPath, replacementDispatch.ShardID, replacementDispatch.DispatchID)
	completionData, err := os.ReadFile(completionPath)
	if err != nil {
		t.Fatalf("Remote Control reviewer completion receipt is unavailable: %v", err)
	}
	completion, err := reviewersession.DecodeCompletion(completionData)
	if err != nil || completion.DispatchID != replacementDispatch.DispatchID || completion.DispatchReceiptSHA256 != replacementResultAttempt.Receipt.DispatchSHA256 || completion.ReviewerHarness != externalsession.RemoteControlHarness || completion.ReviewerSession != "reviewer-session-replacement" || completion.Outcome != "succeeded" || completion.ExitStatus != "completed" || completion.ReviewerResultInputPath != replacementResultAttempt.ReviewerResultInputPath || completion.ReviewerResultInputSHA256 != statusSHA256Hex(replacementResultBytes) || completion.ReviewerResultInputBytes != len(replacementResultBytes) || !completion.NoCollection || !completion.NoIntake || !completion.NoFacts || !completion.NoHeavyTool || !completion.NoAuthority {
		t.Fatalf("Remote Control reviewer completion receipt lost exact lineage: receipt=%+v err=%v", completion, err)
	}
	for role, path := range map[string]string{
		"reviewer input":     replacementResultAttempt.ReviewerResultInputPath,
		"reviewer source":    replacementResultAttempt.ReviewerResultSourcePath,
		"reviewer candidate": replacementResultAttempt.ReviewerResultCandidatePath,
		"reviewer result":    replacementResultAttempt.ReviewerResultPath,
	} {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, replacementResultBytes) {
			t.Fatalf("%s did not preserve exact Remote Control ReviewerResult bytes: %v", role, err)
		}
	}
	expectedSteps := []string{"save-result-input", "record-completion", "source-capture", "stage-candidate", "collect-result", "intake-results"}
	campaign := resultApplied.ContinuationRequest
	if campaign == nil || campaign.StopCode != "route-policy" || campaign.SegmentRoute != "reviewer" || campaign.ExpectedRoute != "case" || campaign.RemainingMaxSteps != turn.Resume.MaxSteps-len(expectedSteps) || campaign.ObservationContract != nil || !strings.Contains(campaign.WhatIfCommand, "-MaxSteps "+stringInt(campaign.RemainingMaxSteps)) || len(resultApplied.Steps) != len(expectedSteps) {
		t.Fatalf("fresh cross-route campaign continuation or segment receipts are invalid: %+v", resultApplied)
	}
	if resultApplied.SegmentCheckpoint == nil || !resultApplied.SegmentCheckpoint.Ready || resultApplied.SegmentCheckpoint.Continuation == nil || resultApplied.SegmentCheckpoint.SegmentRoute != "reviewer" || resultApplied.SegmentCheckpoint.ExpectedRoute != "case" || resultApplied.SegmentCheckpoint.RemainingMaxSteps != campaign.RemainingMaxSteps {
		t.Fatalf("reviewer-to-case segment checkpoint is invalid: %+v", resultApplied.SegmentCheckpoint)
	}
	var campaignStatusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", campaign.ExpectedLane, "-Format", "json"}, &campaignStatusOut); err != nil {
		t.Fatal(err)
	}
	var campaignStatus statusInventory
	if err := json.Unmarshal(campaignStatusOut.Bytes(), &campaignStatus); err != nil {
		t.Fatalf("campaign status did not decode: %v\n%s", err, campaignStatusOut.String())
	}
	durableCampaign := campaignStatus.MissionControlRunbook.CurrentLoopSegment
	if durableCampaign == nil || !durableCampaign.Ready || durableCampaign.ArtifactPath != resultApplied.SegmentCheckpoint.ArtifactPath || durableCampaign.Continuation == nil || durableCampaign.Continuation.ExpectedRoute != "case" || durableCampaign.Continuation.RemainingMaxSteps != campaign.RemainingMaxSteps {
		t.Fatalf("new-session status did not recover latest reviewer-to-case segment: %+v", durableCampaign)
	}
	for index, expected := range expectedSteps {
		if resultApplied.Steps[index].Step != index+1 || resultApplied.Steps[index].RunLoopStepID != expected {
			t.Fatalf("reviewer continuation step %d = %+v, want %s", index+1, resultApplied.Steps[index], expected)
		}
	}
	casePreview := runCurrentLoopResumePreviewWith(t, caseRoot, durableCampaign.ArtifactSHA256, "-Lane", campaign.ExpectedLane)
	if casePreview.InitialRoute != "case" || casePreview.InitialLane != campaign.ExpectedLane || casePreview.MaxSteps != campaign.RemainingMaxSteps || casePreview.ExpectedCurrentLoopPlanSHA256 == "" || casePreview.ResumeSource == nil || casePreview.ResumeSource.ArtifactSHA256 != durableCampaign.ArtifactSHA256 || casePreview.ExpectedResumeCheckpointSHA256 != durableCampaign.ArtifactSHA256 || casePreview.ApplyCommand == "" || casePreview.InitialCurrentStep == nil || casePreview.InitialCurrentStep.CurrentDriverRequest.Lane != campaign.ExpectedLane || len(casePreview.Steps) != 0 || casePreview.ContinuationRequest != nil {
		t.Fatalf("durable campaign operator did not rebuild a checkpoint-bound fresh case segment: %+v", casePreview)
	}
	out.Reset()
	err = Run([]string{
		"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template",
		"-ResumeCurrentLoop", "-ExpectedCurrentLoopCheckpointSha256", durableCampaign.ArtifactSHA256,
		"-Lane", campaign.ExpectedLane,
		"-ExpectedCurrentLoopPlanSha256", turn.Resume.ExpectedCurrentLoopPlanSHA256,
		"-ExpectedMemberExecutionPlanSha256", casePreview.InitialCurrentStep.MemberExecution.ExpectedPlanSHA256,
		"-Apply", "-Format", "json",
	}, &out)
	if err == nil || !strings.Contains(err.Error(), "expected plan sha256 mismatch") {
		t.Fatalf("previous reviewer segment hash crossed the route boundary: %v", err)
	}
	verifications, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl"))
	if err != nil || !strings.Contains(string(verifications), plan.PacketID) {
		t.Fatalf("reviewer continuation omitted verification writeback: %v\n%s", err, verifications)
	}
	decisions, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl"))
	if err != nil || !strings.Contains(string(decisions), plan.PacketID) {
		t.Fatalf("reviewer continuation omitted decision writeback: %v\n%s", err, decisions)
	}
	var revalidationOut bytes.Buffer
	if err := Run([]string{
		"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template",
		"-PacketPath", replacementResultAttempt.Identity.PacketPath, "-ReadyReviewerResults",
		"-Lane", replacementDispatch.TargetLane, "-Actor", actor,
		"-WhatIf", "-Format", "json",
	}, &revalidationOut); err != nil {
		t.Fatal(err)
	}
	var revalidation subagents.ReviewerBatchIntakeResult
	decodeJSONStrict(t, revalidationOut.Bytes(), &revalidation)
	if revalidation.Total != 1 || revalidation.Ready != 1 || revalidation.Processed != 1 || revalidation.Completed != 0 || revalidation.AlreadyComplete != 1 || revalidation.Stopped || revalidation.Partial || len(revalidation.Results) != 1 || revalidation.Results[0].WritebackStatus != "already-complete" || revalidation.Results[0].PostValidation == nil || !revalidation.Results[0].PostValidation.Valid || !revalidation.Results[0].Summary.PostValidationPresent || !revalidation.Results[0].Summary.PostValidationValid || revalidation.Results[0].Summary.PostValidationOverviewVerifications < 1 || revalidation.Results[0].Summary.PostValidationOverviewDecisions < 1 {
		t.Fatalf("completed Remote Control reviewer intake did not strictly revalidate: %+v", revalidation)
	}
	for _, forbidden := range []string{"authority.jsonl", "confirmed.jsonl"} {
		if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", forbidden)); !os.IsNotExist(err) {
			t.Fatalf("reviewer continuation created forbidden ledger %s: %v", forbidden, err)
		}
	}
}

func currentLoopRemoteControlEvidenceItem(t *testing.T, caseRoot, lane string) string {
	t.Helper()
	plan, err := memberexecution.PreviewDispatch(memberexecution.DispatchOptions{
		CaseRoot: caseRoot, Pack: "_template", Lane: lane,
		RequestSHA256: strings.Repeat("a", 64), CreatedAt: "2026-08-12T09:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := memberexecution.Apply(plan, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	accepted, err := memberexecution.PreviewObservation(memberexecution.ObservationOptions{
		CaseRoot: caseRoot, Pack: "_template", Lane: lane, AttemptID: plan.AttemptID,
		Outcome: "accepted", Actor: "member-evidence-harness", ObservedAt: "2026-08-12T09:00:01Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := memberexecution.Apply(accepted, accepted.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(plan.Inspection.OutputsRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	output := []byte("bounded current-loop Remote Control reviewer evidence\n")
	if err := os.WriteFile(filepath.Join(plan.Inspection.OutputsRoot, "review-items.json"), output, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := memberexecution.ResultManifest{
		SchemaVersion: memberexecution.SchemaVersion, Kind: memberexecution.KindManifest,
		AttemptID: plan.AttemptID, Owner: plan.Owner, Summary: "bounded Remote Control reviewer evidence",
		Outputs:           []memberexecution.Output{{Path: "review-items.json", SHA256: statusSHA256Hex(output), Bytes: int64(len(output))}},
		ReviewerItemsPath: "review-items.json", NoAuthority: true, NoConfirmed: true, NoHeavyTool: true,
	}
	manifestData, err := memberexecution.MarshalResultManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.Inspection.ManifestPath, manifestData, 0o600); err != nil {
		t.Fatal(err)
	}
	returned, err := memberexecution.PreviewObservation(memberexecution.ObservationOptions{
		CaseRoot: caseRoot, Pack: "_template", Lane: lane, AttemptID: plan.AttemptID,
		Outcome: "returned", Actor: "member-evidence-harness", ObservedAt: "2026-08-12T09:00:02Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	applied, err := memberexecution.Apply(returned, returned.ExpectedPlanSHA256)
	if err != nil || applied.Inspection.State != "intake-ready" {
		t.Fatalf("member evidence return=%+v err=%v", applied, err)
	}
	taskContextInfo, err := os.Lstat(applied.Inspection.TaskContextPath)
	if err != nil {
		t.Fatalf("member evidence task context stat failed: path=%s err=%v", applied.Inspection.TaskContextPath, err)
	}
	if !taskContextInfo.Mode().IsRegular() || taskContextInfo.Mode()&os.ModeSymlink != 0 || taskContextInfo.Size() < 1 || taskContextInfo.Size() > memberexecution.MaxReviewerEvidenceArtifactBytes {
		t.Fatalf("member evidence task context is not transport-bounded: path=%s bytes=%d mode=%v", applied.Inspection.TaskContextPath, taskContextInfo.Size(), taskContextInfo.Mode())
	}
	item, err := filepath.Rel(caseRoot, applied.Inspection.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(item)
}

func assertRegularNonSymlinkFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("expected regular non-symlink file %s: info=%v err=%v", path, info, err)
	}
}

func TestStatusCurrentLoopInspectionRequestUsesStrictConsumerCaseRequest(t *testing.T) {
	original := currentStepValidatePackMemoryConsumerTask
	t.Cleanup(func() { currentStepValidatePackMemoryConsumerTask = original })
	caseRoot := fullAttachedCase(t)
	inst, err := instance.Read(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	packRequest := missionDriverRequestForTest(`/rekit sync -SelectPackMemoryChange change -WhatIf -Format json`)
	caseRequest := missionDriverRequestForTest(`/rekit continue mission -WhatIf -Format json`)
	caseRequest.Lane = "feature-mission"
	runbook := &statusMissionControlRunbook{
		Scope:                "pack-memory",
		CurrentDriverRequest: &packRequest,
	}
	caseMission := &statusCaseMission{
		MissionCommanderActionQueue: mission.MissionCommanderActionQueue{
			CurrentDriverRequest: &caseRequest,
		},
	}
	currentStepValidatePackMemoryConsumerTask = func(repoRoot, target, pack, lane string) error {
		if repoRoot != inst.TemplateRoot || target != caseRoot || pack != inst.TemplatePack || lane != caseRequest.Lane {
			t.Fatalf("strict inspection identity drifted: repo=%q target=%q pack=%q lane=%q", repoRoot, target, pack, lane)
		}
		return nil
	}
	request := statusCurrentLoopInspectionRequest(caseRoot, caseMission, runbook)
	if request == nil || request == &caseRequest || request.RunLoopStepID != caseRequest.RunLoopStepID || request.Lane != caseRequest.Lane || !strings.Contains(request.Command, "continue") {
		t.Fatalf("checkpoint inspection retained global pack-memory request: %+v", request)
	}
	currentStepValidatePackMemoryConsumerTask = func(_, _, _, _ string) error {
		return errors.New("stale selected sync")
	}
	if request := statusCurrentLoopInspectionRequest(caseRoot, caseMission, runbook); request != &packRequest {
		t.Fatalf("stale consumer unexpectedly replaced global request: %+v", request)
	}
}

func TestCurrentLoopStatusCandidateNarrowsOnlyStrictPackMemoryConsumer(t *testing.T) {
	original := currentStepValidatePackMemoryConsumerTask
	t.Cleanup(func() { currentStepValidatePackMemoryConsumerTask = original })
	packRequest := missionDriverRequestForTest(`/rekit sync -SelectPackMemoryChange change -WhatIf -Format json`)
	caseRequest := missionDriverRequestForTest(`/rekit continue mission -WhatIf -Format json`)
	caseRequest.Lane = "feature-mission"
	status := statusInventory{
		TemplateRoot: "repo", Target: "case", Pack: "_template",
		MissionControlRunbook: &statusMissionControlRunbook{Scope: "pack-memory", CurrentDriverRequest: &packRequest},
		CaseMission:           &statusCaseMission{MissionCommanderActionQueue: mission.MissionCommanderActionQueue{CurrentDriverRequest: &caseRequest}},
	}
	currentStepValidatePackMemoryConsumerTask = func(_, _, _, lane string) error {
		if lane != "feature-mission" {
			t.Fatalf("strict consumer validation lane=%q", lane)
		}
		return nil
	}
	request, route, stop := currentLoopStatusCandidate(status)
	if stop.Code != "" || route != "case" || request == &caseRequest || request.Lane != "feature-mission" || request.Command != caseRequest.Command {
		t.Fatalf("strict consumer route request=%+v route=%s stop=%+v", request, route, stop)
	}
	currentStepValidatePackMemoryConsumerTask = func(_, _, _, _ string) error { return errors.New("stale receipt") }
	request, route, stop = currentLoopStatusCandidate(status)
	if stop.Code != "route-policy" || route != "pack-memory" || request != &packRequest {
		t.Fatalf("ordinary pack-memory route escaped policy: request=%+v route=%s stop=%+v", request, route, stop)
	}
}

func TestValidateCurrentLoopResumeStatusUsesStrictConsumerCaseRequest(t *testing.T) {
	original := currentStepValidatePackMemoryConsumerTask
	t.Cleanup(func() { currentStepValidatePackMemoryConsumerTask = original })
	packRequest := missionDriverRequestForTest(`/rekit sync -SelectPackMemoryChange change -WhatIf -Format json`)
	caseRequest := missionDriverRequestForTest(`/rekit continue mission -WhatIf -Format json`)
	caseRequest.Lane = "feature-mission"
	status := statusInventory{
		TemplateRoot: "repo", Target: "case", Pack: "_template",
		MissionControlRunbook: &statusMissionControlRunbook{Scope: "pack-memory", CurrentDriverRequest: &packRequest},
		CaseMission:           &statusCaseMission{MissionCommanderActionQueue: mission.MissionCommanderActionQueue{CurrentDriverRequest: &caseRequest}},
	}
	inspection := currentloop.Inspection{ExpectedRoute: "case", ExpectedLane: "feature-mission"}
	currentStepValidatePackMemoryConsumerTask = func(_, _, _, lane string) error {
		if lane != inspection.ExpectedLane {
			t.Fatalf("strict resume validation lane=%q", lane)
		}
		return nil
	}
	if err := validateCurrentLoopResumeStatus(status, inspection); err != nil {
		t.Fatalf("strict consumer ready checkpoint did not use normalized case route: %v", err)
	}
	currentStepValidatePackMemoryConsumerTask = func(_, _, _, _ string) error { return errors.New("stale selected sync") }
	if err := validateCurrentLoopResumeStatus(status, inspection); err == nil || !strings.Contains(err.Error(), `scope="pack-memory"`) {
		t.Fatalf("stale strict consumer checkpoint did not fail closed: %v", err)
	}
	for _, route := range []string{"case", "reviewer"} {
		request := missionDriverRequestForTest(`/rekit continue mission -WhatIf -Format json`)
		request.Lane = "feature-mission"
		ordinary := statusInventory{MissionControlRunbook: &statusMissionControlRunbook{Scope: route, CurrentDriverRequest: &request}}
		if err := validateCurrentLoopResumeStatus(ordinary, currentloop.Inspection{ExpectedRoute: route, ExpectedLane: request.Lane}); err != nil {
			t.Fatalf("ordinary %s checkpoint resume changed: %v", route, err)
		}
	}
}

func TestCurrentLoopStopsBeforeNewHumanIntervention(t *testing.T) {
	request := missionDriverRequestForTest(`/rekit reconcile main -InterventionId int-main-1 -WhatIf -Format json`)
	request.Lane = "main"
	status := statusInventory{
		MissionControlRunbook: &statusMissionControlRunbook{
			Scope:                "case",
			CurrentDriverRequest: &request,
		},
	}
	_, route, stop := currentLoopStatusCandidate(status)
	if stop.Code != "" || route != "case" {
		t.Fatalf("initial reconcile should remain previewable: route=%s stop=%+v", route, stop)
	}
	if stop := currentLoopBeforeStepPolicyStop(1, route, &request); stop.Code != "" {
		t.Fatalf("initial reconcile was incorrectly stopped: %+v", stop)
	}
	stop = currentLoopBeforeStepPolicyStop(2, route, &request)
	if stop.Code != "human-intervention" || stop.Phase != "before-step" || stop.CurrentDriverRequest != &request {
		t.Fatalf("refreshed Human-in-the-Lane reconcile was not stopped: %+v", stop)
	}
}

func TestCurrentLoopRouteDriftReturnsTypedCampaignContinuation(t *testing.T) {
	request := missionDriverRequestForTest(`/rekit continue verifier -WhatIf -Format json`)
	request.Lane = "feature-verifier"
	stop := currentLoopStopReason{
		Code:                 "route-policy",
		Phase:                "before-step",
		Message:              "refreshed route or lane changed; review a fresh loop preview",
		CurrentDriverRequest: &request,
	}
	continuation := currentLoopContinuationFor(runtime.Context{Target: `C:\cases\campaign`, Pack: "_template"}, "", 8, 3, "case", "feature-triage", "case", "", stop)
	if continuation == nil || continuation.StopCode != "route-policy" || continuation.SegmentRoute != "case" || continuation.SegmentLane != "feature-triage" || continuation.ExpectedRoute != "case" || continuation.ExpectedLane != "feature-verifier" || continuation.RemainingMaxSteps != 5 || continuation.ObservationContract != nil || continuation.CumulativeReceipts {
		t.Fatalf("lane drift continuation = %+v", continuation)
	}
	if !strings.Contains(continuation.WhatIfCommand, "-MaxSteps 5") || !continuation.FreshPreviewRequired {
		t.Fatalf("lane drift continuation command = %+v", continuation)
	}
	if got := currentLoopContinuationFor(runtime.Context{}, "", 3, 3, "case", "main", "reviewer", "", stop); got != nil {
		t.Fatalf("exhausted segment created continuation: %+v", got)
	}
	forgedExternal := stop
	forgedExternal.Code = "external-reviewer-handoff"
	if got := currentLoopContinuationFor(runtime.Context{}, "", 8, 3, "case", "main", "reviewer", "", forgedExternal); got != nil {
		t.Fatalf("external stop without a typed handoff created continuation: %+v", got)
	}
}

func TestRunCurrentLoopStopsBeforeReconcileSurfacedByFirstStepRefresh(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	preview := runCurrentLoopPreview(t, caseRoot, 2)
	driverStepAfterPreviewValidationHook = func() error {
		appendCurrentLoopOpenIntervention(t, caseRoot, "int-current-loop-refresh")
		return nil
	}
	t.Cleanup(func() { driverStepAfterPreviewValidationHook = nil })
	applied := runCurrentLoopApply(t, caseRoot, preview)
	if applied.AppliedSteps != 1 || len(applied.Steps) != 1 || applied.StopReason.Code != "human-intervention" || applied.StopReason.Phase != "before-step" || applied.FinalStatus == nil || applied.FinalStatus.MissionControlRunbook == nil {
		t.Fatalf("refreshed intervention was not stopped before reconcile: %+v", applied)
	}
	if applied.SegmentCheckpoint == nil || !applied.SegmentCheckpoint.Ready || applied.SegmentCheckpoint.State != "ready" || applied.SegmentCheckpoint.Continuation == nil || applied.SegmentCheckpoint.Continuation.RemainingMaxSteps != 1 || applied.SegmentCheckpoint.ArtifactPath == "" {
		t.Fatalf("applied Human-in-the-Lane segment omitted durable checkpoint: %+v", applied.SegmentCheckpoint)
	}
	continuation := applied.ContinuationRequest
	if continuation == nil || continuation.StopCode != "human-intervention" || continuation.State != "awaiting-fresh-segment-review" || continuation.SegmentRoute != "case" || continuation.ExpectedRoute != "case" || continuation.RemainingMaxSteps != 1 || continuation.AppliedStepsInSegment != 1 || continuation.ObservationContract != nil || !strings.Contains(continuation.WhatIfCommand, "-MaxSteps 1") {
		t.Fatalf("refreshed intervention omitted fresh review continuation: %+v", continuation)
	}
	request := applied.FinalStatus.MissionControlRunbook.CurrentDriverRequest
	if request == nil || driverStepCommandName(request.Command) != "reconcile" || !strings.Contains(request.Command, "int-current-loop-refresh") {
		t.Fatalf("final status did not retain the fresh reconcile request: %+v", request)
	}
	interventions, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "interventions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(interventions), `"resolvesEventId":"int-current-loop-refresh"`) {
		t.Fatalf("old loop authorization resolved the fresh intervention: %s", interventions)
	}
	var statusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	var newSessionStatus statusInventory
	if err := json.Unmarshal(statusOut.Bytes(), &newSessionStatus); err != nil {
		t.Fatalf("new-session status did not decode: %v\n%s", err, statusOut.String())
	}
	durable := newSessionStatus.MissionControlRunbook.CurrentLoopSegment
	if durable == nil || !durable.Ready || durable.Continuation == nil || durable.Continuation.RemainingMaxSteps != continuation.RemainingMaxSteps || durable.Continuation.WhatIfCommand != "" || durable.LegacyUnboundWhatIfCommand != continuation.WhatIfCommand || durable.ArtifactPath != applied.SegmentCheckpoint.ArtifactPath || durable.ResumeDriverRequest == nil || durable.ResumeDriverRequest.RunLoopStepID != "resume-current-loop" || !durable.ResumeDriverRequest.CommandExecutable || !strings.Contains(durable.ResumeDriverRequest.Command, "-ResumeCurrentLoop") || !strings.Contains(durable.ResumeDriverRequest.Command, durable.ArtifactSHA256) {
		t.Fatalf("new-session status did not recover exact campaign checkpoint and resume request: %+v", durable)
	}
	var handoffOut bytes.Buffer
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-WhatIf", "-Format", "json"}, &handoffOut); err != nil {
		t.Fatal(err)
	}
	var handoff struct {
		CurrentLoopSegment *currentloop.Inspection `json:"currentLoopSegment"`
	}
	if err := json.Unmarshal(handoffOut.Bytes(), &handoff); err != nil {
		t.Fatalf("handoff did not decode: %v\n%s", err, handoffOut.String())
	}
	if handoff.CurrentLoopSegment == nil || !handoff.CurrentLoopSegment.Ready || handoff.CurrentLoopSegment.ArtifactPath != durable.ArtifactPath || handoff.CurrentLoopSegment.Continuation == nil || handoff.CurrentLoopSegment.ResumeDriverRequest == nil {
		t.Fatalf("handoff did not project strict campaign checkpoint: %+v", handoff.CurrentLoopSegment)
	}
	var statusText bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "text"}, &statusText); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"current-loop resume source：artifactSha256=" + durable.ArtifactSHA256, "current-loop checkpoint-bound resume driver request", "-ResumeCurrentLoop", "legacyUnboundCommand="} {
		if !strings.Contains(statusText.String(), expected) {
			t.Fatalf("status text omitted checkpoint-bound resume detail %q:\n%s", expected, statusText.String())
		}
	}
	var handoffText bytes.Buffer
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-WhatIf", "-Format", "text"}, &handoffText); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(handoffText.String(), "current-loop checkpoint-bound resume driver request") || !strings.Contains(handoffText.String(), durable.ArtifactSHA256) {
		t.Fatalf("handoff text omitted checkpoint-bound resume detail:\n%s", handoffText.String())
	}
	artifactPath := filepath.Join(caseRoot, filepath.FromSlash(durable.ArtifactPath))
	if err := os.WriteFile(artifactPath, []byte(`{"tampered":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	statusOut.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	var tamperedStatus statusInventory
	if err := json.Unmarshal(statusOut.Bytes(), &tamperedStatus); err != nil {
		t.Fatalf("tampered status did not decode: %v\n%s", err, statusOut.String())
	}
	tampered := tamperedStatus.MissionControlRunbook.CurrentLoopSegment
	if tampered == nil || tampered.Ready || tampered.State != "invalid" || tampered.Continuation != nil || tampered.ArtifactPath != durable.ArtifactPath {
		t.Fatalf("tampered latest segment did not fail closed: %+v", tampered)
	}
	tamperedOperator := tamperedStatus.MissionControlRunbook.CurrentLoopOperator
	if tamperedOperator == nil || tamperedOperator.Ready || tamperedOperator.State != "checkpoint-invalid" || tamperedOperator.SelectedDriverRequest != nil || tamperedOperator.StartDriverRequest != nil || tamperedOperator.ResumeDriverRequest != nil || tamperedOperator.ExternalReviewerHandoff != nil {
		t.Fatalf("tampered checkpoint exposed an executable current-loop operator: %+v", tamperedOperator)
	}
	if previewErr := runCurrentLoopError(caseRoot, []string{"-ResumeCurrentLoop", "-ExpectedCurrentLoopCheckpointSha256", durable.ArtifactSHA256, "-WhatIf"}); previewErr == nil || !strings.Contains(previewErr.Error(), "state=ready") {
		t.Fatalf("tampered checkpoint resume did not fail closed: %v", previewErr)
	}
}

func TestRunCurrentLoopDurableResumeDriverRequestProductPath(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	initial := runCurrentLoopPreview(t, caseRoot, 3)
	driverStepAfterPreviewValidationHook = func() error {
		appendCurrentLoopOpenIntervention(t, caseRoot, "int-current-loop-resume")
		return nil
	}
	applied := runCurrentLoopApply(t, caseRoot, initial)
	driverStepAfterPreviewValidationHook = nil
	if applied.SegmentCheckpoint == nil || !applied.SegmentCheckpoint.Ready || applied.SegmentCheckpoint.ResumeDriverRequest == nil || applied.SegmentCheckpoint.RemainingMaxSteps != 2 {
		t.Fatalf("initial segment did not expose a durable resume request: %+v", applied.SegmentCheckpoint)
	}
	previewArgs, err := splitDriverCommand(applied.SegmentCheckpoint.ResumeDriverRequest.Command)
	if err != nil {
		t.Fatal(err)
	}
	resumedPreview := runCurrentLoopResult(t, append([]string{"-Command", previewArgs[1]}, previewArgs[2:]...))
	if resumedPreview.MaxSteps != 2 || resumedPreview.ResumeSource == nil || resumedPreview.ResumeSource.ArtifactSHA256 != applied.SegmentCheckpoint.ArtifactSHA256 || resumedPreview.ExpectedResumeCheckpointSHA256 != applied.SegmentCheckpoint.ArtifactSHA256 || driverStepCommandName(resumedPreview.InitialCurrentStep.CurrentDriverRequest.Command) != "reconcile" || resumedPreview.ApplyCommand == "" {
		t.Fatalf("resume driver request did not produce the exact fresh reconcile preview: %+v", resumedPreview)
	}
	applyArgs, err := splitDriverCommand(resumedPreview.ApplyCommand)
	if err != nil {
		t.Fatal(err)
	}
	resumedApplied := runCurrentLoopResult(t, append([]string{"-Command", applyArgs[1]}, applyArgs[2:]...))
	if !resumedApplied.Applied || resumedApplied.AppliedSteps < 1 || resumedApplied.SegmentCheckpoint == nil || resumedApplied.SegmentCheckpoint.Sequence != applied.SegmentCheckpoint.Sequence+1 || resumedApplied.SegmentCheckpoint.ResumeSourceSHA256 != applied.SegmentCheckpoint.ArtifactSHA256 {
		t.Fatalf("checkpoint-bound resume did not publish linked successor segment: %+v", resumedApplied)
	}
	if err := Run(append([]string{"-Command", applyArgs[1]}, applyArgs[2:]...), &bytes.Buffer{}); err == nil || (!strings.Contains(err.Error(), "latest checkpoint to be state=ready") && !strings.Contains(err.Error(), "consumed")) {
		t.Fatalf("stale resume apply was not rejected after successor publication: %v", err)
	}
	latest := currentloop.Inspect(repoRoot(t), caseRoot, "_template", resumedApplied.FinalStatus.MissionControlRunbook.CurrentDriverRequest)
	if latest.ArtifactSHA256 != resumedApplied.SegmentCheckpoint.ArtifactSHA256 || latest.Sequence != resumedApplied.SegmentCheckpoint.Sequence {
		t.Fatalf("stale resume apply mutated the checkpoint chain: %+v", latest)
	}
}

func TestRunCurrentLoopResumePublishesRetryCheckpointAfterZeroWriteFailure(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	initial := runCurrentLoopPreview(t, caseRoot, 3)
	driverStepAfterPreviewValidationHook = func() error {
		appendCurrentLoopOpenIntervention(t, caseRoot, "int-current-loop-retry")
		return nil
	}
	first := runCurrentLoopApply(t, caseRoot, initial)
	driverStepAfterPreviewValidationHook = nil
	if first.SegmentCheckpoint == nil || first.SegmentCheckpoint.ResumeDriverRequest == nil {
		t.Fatalf("initial segment did not expose resume request: %+v", first.SegmentCheckpoint)
	}
	previewArgs, err := splitDriverCommand(first.SegmentCheckpoint.ResumeDriverRequest.Command)
	if err != nil {
		t.Fatal(err)
	}
	preview := runCurrentLoopResult(t, append([]string{"-Command", previewArgs[1]}, previewArgs[2:]...))
	applyArgs, err := splitDriverCommand(preview.ApplyCommand)
	if err != nil {
		t.Fatal(err)
	}
	driverStepApplyBeforeMutationHook = func(string) error { return os.ErrPermission }
	t.Cleanup(func() {
		driverStepApplyBeforeMutationHook = nil
		driverStepAfterPreviewValidationHook = nil
	})
	failed := runCurrentLoopResult(t, append([]string{"-Command", applyArgs[1]}, applyArgs[2:]...))
	if failed.Applied || failed.AppliedSteps != 0 || failed.StopReason.Code != "zero-progress-retry" || failed.SegmentCheckpoint == nil || !strings.Contains(failed.StopReason.Message, "permission denied") {
		t.Fatalf("resume pre-mutation failure did not return a retry checkpoint: %+v", failed)
	}
	if !failed.SegmentCheckpoint.Ready || failed.SegmentCheckpoint.State != "ready" || failed.SegmentCheckpoint.ResumeSourceSHA256 != first.SegmentCheckpoint.ArtifactSHA256 || failed.SegmentCheckpoint.RemainingMaxSteps != first.SegmentCheckpoint.RemainingMaxSteps || failed.SegmentCheckpoint.ResumeDriverRequest == nil {
		t.Fatalf("resume zero-write retry checkpoint did not preserve the remaining budget: %+v", failed.SegmentCheckpoint)
	}
	if failed.SegmentCheckpoint.Sequence != first.SegmentCheckpoint.Sequence+1 {
		t.Fatalf("resume zero-write retry checkpoint did not append to the chain: first=%+v retry=%+v", first.SegmentCheckpoint, failed.SegmentCheckpoint)
	}
	if err := Run(append([]string{"-Command", applyArgs[1]}, applyArgs[2:]...), &bytes.Buffer{}); err == nil || (!strings.Contains(err.Error(), "expected checkpoint sha256 mismatch") && !strings.Contains(err.Error(), "latest checkpoint")) {
		t.Fatalf("original claimed resume Apply remained executable after retry checkpoint publication: %v", err)
	}
	var statusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	var retryStatus statusInventory
	if err := json.Unmarshal(statusOut.Bytes(), &retryStatus); err != nil {
		t.Fatalf("retry operator status did not decode: %v\n%s", err, statusOut.String())
	}
	retryOperator := retryStatus.MissionControlRunbook.CurrentLoopOperator
	if retryOperator == nil || !retryOperator.Ready || retryOperator.State != "checkpoint-resume-review-required" || retryOperator.SelectedDriverRequest == nil || retryOperator.ResumeDriverRequest == nil || retryOperator.RemainingMaxSteps != first.SegmentCheckpoint.RemainingMaxSteps {
		t.Fatalf("replacement status did not expose the retry checkpoint: %+v", retryOperator)
	}
	driverStepApplyBeforeMutationHook = nil
	retryPreviewArgs, err := splitDriverCommand(retryOperator.SelectedDriverRequest.Command)
	if err != nil {
		t.Fatal(err)
	}
	retryPreview := runCurrentLoopResult(t, append([]string{"-Command", retryPreviewArgs[1]}, retryPreviewArgs[2:]...))
	retryApplyArgs, err := splitDriverCommand(retryPreview.ApplyCommand)
	if err != nil {
		t.Fatal(err)
	}
	retried := runCurrentLoopResult(t, append([]string{"-Command", retryApplyArgs[1]}, retryApplyArgs[2:]...))
	if !retried.Applied || retried.AppliedSteps < 1 || retried.SegmentCheckpoint == nil || retried.SegmentCheckpoint.ResumeSourceSHA256 != failed.SegmentCheckpoint.ArtifactSHA256 {
		t.Fatalf("replacement session did not continue from the zero-write retry checkpoint: %+v", retried)
	}
}

func TestRunCurrentLoopResumeDoesNotRecoverZeroWriteFailureAfterRequestDrift(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	initial := runCurrentLoopPreview(t, caseRoot, 3)
	driverStepAfterPreviewValidationHook = func() error {
		appendCurrentLoopOpenIntervention(t, caseRoot, "int-current-loop-drift")
		return nil
	}
	first := runCurrentLoopApply(t, caseRoot, initial)
	driverStepAfterPreviewValidationHook = nil
	previewArgs, err := splitDriverCommand(first.SegmentCheckpoint.ResumeDriverRequest.Command)
	if err != nil {
		t.Fatal(err)
	}
	preview := runCurrentLoopResult(t, append([]string{"-Command", previewArgs[1]}, previewArgs[2:]...))
	applyArgs, err := splitDriverCommand(preview.ApplyCommand)
	if err != nil {
		t.Fatal(err)
	}
	driverStepApplyBeforeMutationHook = func(string) error {
		appendCurrentLoopOpenIntervention(t, caseRoot, "int-current-loop-drift-after-claim")
		return os.ErrPermission
	}
	t.Cleanup(func() {
		driverStepApplyBeforeMutationHook = nil
		driverStepAfterPreviewValidationHook = nil
	})
	failed := runCurrentLoopResult(t, append([]string{"-Command", applyArgs[1]}, applyArgs[2:]...))
	if failed.Applied || failed.AppliedSteps != 0 || failed.StopReason.Code != "error" || failed.SegmentCheckpoint != nil || !strings.Contains(strings.Join(failed.Boundary, " "), "claim remains consumed") {
		t.Fatalf("zero-write failure recovered budget after durable request drift: %+v", failed)
	}
	inspection := currentloop.Inspect(repoRoot(t), caseRoot, "_template", failed.FinalStatus.MissionControlRunbook.CurrentDriverRequest)
	if inspection.Ready || inspection.State != "consumed" || inspection.ArtifactSHA256 != first.SegmentCheckpoint.ArtifactSHA256 || inspection.ResumeDriverRequest != nil {
		t.Fatalf("request drift after claim exposed a retry checkpoint: %+v", inspection)
	}
}

func TestRunCurrentLoopResumeDoesNotRecoverAppliedMutationFailure(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	initial := runCurrentLoopPreview(t, caseRoot, 3)
	driverStepAfterPreviewValidationHook = func() error {
		appendCurrentLoopOpenIntervention(t, caseRoot, "int-current-loop-partial-resume")
		return nil
	}
	first := runCurrentLoopApply(t, caseRoot, initial)
	driverStepAfterPreviewValidationHook = nil
	if first.SegmentCheckpoint == nil || first.SegmentCheckpoint.ResumeDriverRequest == nil {
		t.Fatalf("initial segment did not expose resume request: %+v", first.SegmentCheckpoint)
	}
	previewArgs, err := splitDriverCommand(first.SegmentCheckpoint.ResumeDriverRequest.Command)
	if err != nil {
		t.Fatal(err)
	}
	preview := runCurrentLoopResult(t, append([]string{"-Command", previewArgs[1]}, previewArgs[2:]...))
	applyArgs, err := splitDriverCommand(preview.ApplyCommand)
	if err != nil {
		t.Fatal(err)
	}
	currentStepBeforeStatusRefreshHook = func(string) error { return os.ErrPermission }
	t.Cleanup(func() {
		currentStepBeforeStatusRefreshHook = nil
		driverStepAfterPreviewValidationHook = nil
	})
	failed := runCurrentLoopResult(t, append([]string{"-Command", applyArgs[1]}, applyArgs[2:]...))
	if !failed.Applied || failed.AppliedSteps != 1 || failed.StopReason.Code != "error" || failed.StopReason.Phase != "refresh-status" || failed.SegmentCheckpoint == nil || failed.SegmentCheckpoint.State != "status-unavailable" || failed.SegmentCheckpoint.Ready || failed.SegmentCheckpoint.Continuation != nil || failed.SegmentCheckpoint.ResumeSourceSHA256 != first.SegmentCheckpoint.ArtifactSHA256 {
		t.Fatalf("resume applied mutation failure recovered consumed budget: %+v", failed)
	}
	var statusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	var fresh statusInventory
	if err := json.Unmarshal(statusOut.Bytes(), &fresh); err != nil {
		t.Fatal(err)
	}
	operator := fresh.MissionControlRunbook.CurrentLoopOperator
	if operator == nil || !operator.Ready || operator.State != "fresh-loop-review-required" || operator.StartDriverRequest == nil || operator.SelectedDriverRequest == nil || operator.ResumeDriverRequest != nil || operator.RemainingMaxSteps != 0 {
		t.Fatalf("replacement status recovered prior budget instead of requiring a fresh campaign: %+v", operator)
	}
}

func TestRunCurrentLoopReturnsTypedApplyError(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	preview := runCurrentLoopPreview(t, caseRoot, 2)
	currentLoopBeforeApplyStepHook = func(step int) error {
		return os.ErrPermission
	}
	t.Cleanup(func() {
		currentLoopBeforeApplyStepHook = nil
	})
	applied := runCurrentLoopApply(t, caseRoot, preview)
	if applied.Applied || applied.AppliedSteps != 0 || len(applied.Steps) != 0 || applied.StopReason.Code != "error" || applied.StopReason.Phase != "apply-step" || !strings.Contains(applied.StopReason.Message, "permission denied") || applied.FinalStatus == nil {
		t.Fatalf("typed current-loop apply error was not returned: %+v", applied)
	}
}

func TestRunCurrentLoopValidatesOuterContract(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	cases := []struct {
		args []string
		want string
	}{
		{args: []string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, want: "requires -MaxSteps"},
		{args: []string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-MaxSteps", "21", "-WhatIf", "-Format", "json"}, want: "between 1 and 20"},
		{args: []string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-ResumeCurrentLoop", "-MaxSteps", "2", "-WhatIf", "-Format", "json"}, want: "does not accept -MaxSteps"},
		{args: []string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-ResumeCurrentLoop", "-WhatIf", "-Format", "json"}, want: "requires -ExpectedCurrentLoopCheckpointSha256"},
		{args: []string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-MaxSteps", "2", "-Lane", "unexpected", "-WhatIf", "-Format", "json"}, want: "selected current lane"},
	}
	for _, tc := range cases {
		var out bytes.Buffer
		err := Run(tc.args, &out)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("current-loop args error = %v, want %q", err, tc.want)
		}
	}
}

func currentLoopCaseWithOpenIntervention(t *testing.T, eventID string) string {
	t.Helper()
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	appendCurrentLoopOpenIntervention(t, caseRoot, eventID)
	return caseRoot
}

func appendCurrentLoopOpenIntervention(t *testing.T, caseRoot, eventID string) {
	t.Helper()
	path := filepath.Join(caseRoot, ".rekit", "facts", "interventions.jsonl")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	entry := `{"kind":"intervention","eventId":"` + eventID + `","lane":"main","subject":"current loop correction","summary":"operator redirected the current loop","action":"override","target":"current-loop","approvedBy":"lead","scope":"metadata","status":"open","batchId":"batch-current-loop"}` + "\n"
	if _, err := file.WriteString(entry); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

type currentLoopTestPlan struct {
	MaxSteps                       int                                    `json:"maxSteps"`
	InitialRoute                   string                                 `json:"initialRoute"`
	InitialLane                    string                                 `json:"initialLane"`
	Applied                        bool                                   `json:"applied"`
	AppliedSteps                   int                                    `json:"appliedSteps"`
	InitialCurrentStep             *currentStepPlan                       `json:"initialCurrentStep"`
	InitialCurrentDriverRequest    *mission.MissionCommanderDriverRequest `json:"initialCurrentDriverRequest"`
	ExpectedCurrentLoopPlanSHA256  string                                 `json:"expectedCurrentLoopPlanSha256"`
	Steps                          []currentLoopStepReceipt               `json:"steps"`
	StopReason                     currentLoopStopReason                  `json:"stopReason"`
	ResumeCommand                  string                                 `json:"resumeCommand"`
	ContinuationRequest            *currentLoopContinuationRequest        `json:"continuationRequest"`
	ResumeSource                   *currentloop.Inspection                `json:"resumeSource"`
	ExpectedResumeCheckpointSHA256 string                                 `json:"expectedResumeCheckpointSha256"`
	ObservationPath                string                                 `json:"observationPath"`
	ObservationSHA256              string                                 `json:"observationSha256"`
	ObservationKind                string                                 `json:"observationKind"`
	ObservationActor               string                                 `json:"observationActor"`
	ObservationReceipt             *currentLoopObservationReceipt         `json:"observationReceipt"`
	ApplyCommand                   string                                 `json:"applyCommand"`
	SegmentCheckpoint              *currentloop.Inspection                `json:"segmentCheckpoint"`
	FinalStatus                    *statusInventory                       `json:"finalStatus"`
	Boundary                       []string                               `json:"boundary"`
}

func runCurrentLoopPreview(t *testing.T, caseRoot string, maxSteps int) currentLoopTestPlan {
	t.Helper()
	return runCurrentLoopPreviewWith(t, caseRoot, maxSteps)
}

func setCurrentLoopLaneOwner(t *testing.T, caseRoot, laneID, executor string, generation int, updatedAt string) {
	t.Helper()
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	for index := range board.Lanes {
		if board.Lanes[index].ID != laneID {
			continue
		}
		board.Lanes[index].CurrentExecutor = executor
		board.Lanes[index].ExecutorGeneration = generation
		board.Lanes[index].UpdatedAt = updatedAt
		break
	}
	boardData, err := json.MarshalIndent(board, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	boardPath, err := projectstate.Join(caseRoot, "board.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(boardPath, append(boardData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	lanePath, err := projectstate.Join(caseRoot, "lanes", laneID, "lane.json")
	if err != nil {
		t.Fatal(err)
	}
	laneData, err := os.ReadFile(lanePath)
	if err != nil {
		t.Fatal(err)
	}
	var lane workstream.Lane
	if err := json.Unmarshal(laneData, &lane); err != nil {
		t.Fatal(err)
	}
	lane.CurrentExecutor = executor
	lane.ExecutorGeneration = generation
	lane.UpdatedAt = updatedAt
	laneData, err = json.MarshalIndent(lane, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lanePath, append(laneData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runCurrentLoopPreviewWith(t *testing.T, caseRoot string, maxSteps int, inputs ...string) currentLoopTestPlan {
	t.Helper()
	args := []string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-MaxSteps", stringInt(maxSteps)}
	args = append(args, inputs...)
	args = append(args, "-WhatIf", "-Format", "json")
	return runCurrentLoopResult(t, args)
}

func runCurrentLoopResumePreviewWith(t *testing.T, caseRoot, checkpointSHA256 string, inputs ...string) currentLoopTestPlan {
	t.Helper()
	args := []string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-ResumeCurrentLoop", "-ExpectedCurrentLoopCheckpointSha256", checkpointSHA256}
	args = append(args, inputs...)
	args = append(args, "-WhatIf", "-Format", "json")
	return runCurrentLoopResult(t, args)
}

func mustCurrentLoopObservationInboxPath(t *testing.T, caseRoot string) string {
	t.Helper()
	path, err := projectstate.Join(caseRoot, "external-session-observations", "inbox")
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func writeCurrentLoopObservation(t *testing.T, caseRoot, name string, envelope currentLoopObservationEnvelope) string {
	t.Helper()
	dir, err := projectstate.Join(caseRoot, "external-session-observations")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name+".json")
	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCurrentLoopObservationEnvelopeRejectsInvalidFilesAndFlags(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	checkpointSHA256 := strings.Repeat("a", 64)
	valid := currentLoopObservationEnvelope{
		SchemaVersion: 1, Kind: "current-loop-external-session-observation", CheckpointSHA256: checkpointSHA256,
		ObservationKind: "member-session-accepted", Actor: "harness", ObservedAt: "2026-08-03T03:01:00Z", MemberAttemptID: "g000001-a000001-0123456789abcdef",
		NoAuthorityOrConfirmed: true, NoHeavyTool: true,
	}
	validData, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	inside := writeCurrentLoopObservation(t, caseRoot, "strict-valid", valid)
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, append(validData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	unknown := filepath.Join(caseRoot, ".rekit", "external-session-observations", "unknown.json")
	unknownData := append(append([]byte{}, validData[:len(validData)-1]...), []byte(",\"unknown\":true}\n")...)
	if err := os.WriteFile(unknown, unknownData, 0o600); err != nil {
		t.Fatal(err)
	}
	trailing := filepath.Join(caseRoot, ".rekit", "external-session-observations", "trailing.json")
	if err := os.WriteFile(trailing, append(append([]byte{}, validData...), []byte("\n{}\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	oversize := filepath.Join(caseRoot, ".rekit", "external-session-observations", "oversize.json")
	if err := os.WriteFile(oversize, bytes.Repeat([]byte{'x'}, maxCurrentLoopObservationBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path string
		want string
	}{
		{outside, "escapes anchored case root"},
		{unknown, "unknown field"},
		{trailing, "exactly one JSON object"},
		{oversize, "bounded non-empty regular file"},
	} {
		if _, err := readCurrentLoopObservationEnvelope(caseRoot, tc.path); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("observation file %s error = %v, want %q", tc.path, err, tc.want)
		}
	}
	base := []string{"-ResumeCurrentLoop", "-ExpectedCurrentLoopCheckpointSha256", checkpointSHA256, "-CurrentLoopObservationPath", inside}
	for _, tc := range []struct {
		extra []string
		want  string
	}{
		{[]string{"-ExpectedCurrentLoopObservationSha256", strings.Repeat("b", 64), "-WhatIf"}, "does not accept -ExpectedCurrentLoopObservationSha256"},
		{[]string{"-Apply", "-ExpectedCurrentLoopPlanSha256", strings.Repeat("c", 64)}, "requires -ExpectedCurrentLoopObservationSha256"},
		{[]string{"-Actor", "legacy", "-WhatIf"}, "cannot be combined"},
		{[]string{"-MemberExecutionAttemptId", valid.MemberAttemptID, "-MemberExecutionOutcome", "accepted", "-MemberExecutionObservedAt", valid.ObservedAt, "-WhatIf"}, "cannot be combined"},
	} {
		if err := runCurrentLoopError(caseRoot, append(append([]string{}, base...), tc.extra...)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("observation flag contract error = %v, want %q", err, tc.want)
		}
	}
}

func TestCurrentLoopObservationInboxFailsClosedOnAmbiguityAndInvalidEntry(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	inspection := currentloop.Inspection{
		ArtifactSHA256: strings.Repeat("a", 64),
		ExpectedLane:   "main",
		Continuation: &currentloop.Continuation{
			ObservationContract:   &currentloop.ObservationContract{Alternatives: []currentloop.ObservationAlternative{{Kind: "member-session-accepted"}}},
			ExternalMemberHandoff: &mission.CurrentLoopExternalMemberHandoff{AttemptID: "g000001-a000001-deadbeefdeadbeef"},
		},
	}
	inboxDir := mustCurrentLoopObservationInboxPath(t, caseRoot)
	if err := os.MkdirAll(inboxDir, 0o700); err != nil {
		t.Fatal(err)
	}
	valid := currentLoopObservationEnvelope{
		SchemaVersion: 1, Kind: "current-loop-external-session-observation", CheckpointSHA256: inspection.ArtifactSHA256,
		ObservationKind: "member-session-accepted", Actor: "harness", ObservedAt: "2026-08-04T00:00:00Z", MemberAttemptID: inspection.Continuation.ExternalMemberHandoff.AttemptID,
		NoAuthorityOrConfirmed: true, NoHeavyTool: true,
	}
	for _, name := range []string{"one.json", "two.json"} {
		data, err := json.Marshal(valid)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(inboxDir, name), append(data, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ambiguous := inspectCurrentLoopObservationInbox(caseRoot, inspection)
	if ambiguous.State != "ambiguous" || ambiguous.MatchingCount != 2 || ambiguous.SelectedCandidate != nil || ambiguous.SelectedDriverRequest != nil {
		t.Fatalf("ambiguous inbox was not blocked: %+v", ambiguous)
	}
	if err := os.Remove(filepath.Join(inboxDir, "two.json")); err != nil {
		t.Fatal(err)
	}
	bad := valid
	bad.Actor = ""
	badData, err := json.Marshal(bad)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inboxDir, "bad.json"), append(badData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	invalid := inspectCurrentLoopObservationInbox(caseRoot, inspection)
	if invalid.State != "invalid" || invalid.InvalidCount != 1 || invalid.SelectedCandidate != nil || invalid.SelectedDriverRequest != nil {
		t.Fatalf("invalid inbox was not blocked: %+v", invalid)
	}
	if err := os.Remove(filepath.Join(inboxDir, "bad.json")); err != nil {
		t.Fatal(err)
	}
	wrongAttempt := valid
	wrongAttempt.MemberAttemptID = "g000001-a000002-deadbeefdeadbeef"
	wrongAttemptData, err := json.Marshal(wrongAttempt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inboxDir, "wrong-attempt.json"), append(wrongAttemptData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	attemptMismatch := inspectCurrentLoopObservationInbox(caseRoot, inspection)
	if attemptMismatch.State != "invalid" || attemptMismatch.InvalidCount != 1 || attemptMismatch.SelectedCandidate != nil || attemptMismatch.SelectedDriverRequest != nil {
		t.Fatalf("attempt-mismatched inbox observation was not blocked: %+v", attemptMismatch)
	}
	if err := os.Remove(filepath.Join(inboxDir, "one.json")); err != nil {
		t.Fatal(err)
	}
	staleOnly := inspectCurrentLoopObservationInbox(caseRoot, inspection)
	if staleOnly.State != "invalid" || staleOnly.InvalidCount != 1 || staleOnly.MatchingCount != 0 || staleOnly.SelectedCandidate != nil {
		t.Fatalf("attempt-mismatched observation was not invalid without another candidate: %+v", staleOnly)
	}
	if err := os.Remove(filepath.Join(inboxDir, "wrong-attempt.json")); err != nil {
		t.Fatal(err)
	}
	reviewerAttempt := strings.Repeat("b", 64)
	inspection.Continuation.ExternalMemberHandoff = nil
	inspection.Continuation.ObservationContract = &currentloop.ObservationContract{Alternatives: []currentloop.ObservationAlternative{{
		Kind:                   "reviewer-session-failed",
		PreviewCommandTemplate: "/rekit run-current-loop -ExpectedCurrentLoopReviewerAttemptSha256 " + reviewerAttempt + " -WhatIf -Format json",
	}}}
	wrongReviewer := currentLoopObservationEnvelope{
		SchemaVersion: 1, Kind: "current-loop-external-session-observation", CheckpointSHA256: inspection.ArtifactSHA256,
		ObservationKind: "reviewer-session-failed", Actor: "harness", ReviewerAttemptSHA256: strings.Repeat("c", 64), ReviewerExitStatus: "failed",
		NoAuthorityOrConfirmed: true, NoHeavyTool: true,
	}
	wrongReviewerData, err := json.Marshal(wrongReviewer)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inboxDir, "wrong-reviewer.json"), append(wrongReviewerData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	reviewerMismatch := inspectCurrentLoopObservationInbox(caseRoot, inspection)
	if reviewerMismatch.State != "invalid" || reviewerMismatch.InvalidCount != 1 || reviewerMismatch.SelectedCandidate != nil {
		t.Fatalf("reviewer-attempt-mismatched observation was not blocked: %+v", reviewerMismatch)
	}
	if err := os.Remove(filepath.Join(inboxDir, "wrong-reviewer.json")); err != nil {
		t.Fatal(err)
	}
	staleCheckpoint := valid
	staleCheckpoint.CheckpointSHA256 = strings.Repeat("d", 64)
	staleCheckpointData, err := json.Marshal(staleCheckpoint)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inboxDir, "stale-checkpoint.json"), append(staleCheckpointData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	stale := inspectCurrentLoopObservationInbox(caseRoot, inspection)
	if stale.State != "empty" || stale.StaleCount != 1 || stale.MatchingCount != 0 || stale.SelectedCandidate != nil || stale.SelectedDriverRequest != nil {
		t.Fatalf("stale checkpoint observation was selected instead of counted: %+v", stale)
	}
}

func recordCurrentLoopExternalSessionAttempt(t *testing.T, operator *mission.CurrentLoopOperatorPackage, harness, session, actor, startedAt string) *mission.CurrentLoopExternalSessionJob {
	t.Helper()
	if operator == nil || operator.ExternalSessionJob == nil || operator.ExternalSessionJob.AttemptRequest == nil {
		t.Fatalf("operator omitted external session attempt request: %+v", operator)
	}
	job := operator.ExternalSessionJob
	args := rekitCommandCLIArgs(
		t,
		job.AttemptRequest.Command,
	)
	for index := range args {
		switch strings.ToLower(args[index]) {
		case "-externalsessionharness":
			args[index+1] = harness
		case "-externalsessionid":
			args[index+1] = session
		case "-externalsessionactor":
			args[index+1] = actor
		case "-externalsessionstartedat":
			args[index+1] = startedAt
		}
	}
	var out bytes.Buffer
	if err := Run(args, &out); err != nil {
		t.Fatal(err)
	}
	var plan externalsession.AttemptPlan
	decodeJSONStrict(t, out.Bytes(), &plan)
	if plan.ApplyCommand == "" || plan.ExpectedPlanSHA256 == "" {
		t.Fatalf("attempt preview omitted apply identity: %+v", plan)
	}
	out.Reset()
	if err := Run(rekitCommandCLIArgs(t, plan.ApplyCommand), &out); err != nil {
		t.Fatal(err)
	}
	var applied currentLoopExternalSessionAttemptResult
	decodeJSONStrict(t, out.Bytes(), &applied)
	if !applied.Applied || applied.RefreshedStatus == nil || applied.AttemptSHA256 == "" || applied.AttemptSHA256 != plan.AttemptSHA256 {
		t.Fatalf("attempt apply did not return exact receipt identity and refreshed status: preview=%+v applied=%+v", plan, applied)
	}
	out.Reset()
	if err := Run(rekitCommandCLIArgs(t, plan.ApplyCommand), &out); err != nil {
		t.Fatalf("attempt committed replay failed: %v", err)
	}
	var replayed currentLoopExternalSessionAttemptResult
	decodeJSONStrict(t, out.Bytes(), &replayed)
	if !replayed.Applied || !replayed.AlreadyApplied || replayed.AttemptSHA256 != applied.AttemptSHA256 {
		t.Fatalf("attempt committed replay lost exact receipt identity: first=%+v replay=%+v", applied, replayed)
	}
	return applied.RefreshedStatus.MissionControlRunbook.CurrentLoopOperator.ExternalSessionJob
}

func recordCurrentLoopExternalSessionAttemptWithPendingRecovery(t *testing.T, operator *mission.CurrentLoopOperatorPackage, harness, session, actor, startedAt string) *mission.CurrentLoopExternalSessionJob {
	t.Helper()
	if operator == nil || operator.ExternalSessionJob == nil {
		t.Fatalf("operator omitted external session job: %+v", operator)
	}
	job := operator.ExternalSessionJob
	args := rekitCommandCLIArgs(
		t,
		job.AttemptRequest.Command,
	)
	for index := range args {
		switch strings.ToLower(args[index]) {
		case "-externalsessionharness":
			args[index+1] = harness
		case "-externalsessionid":
			args[index+1] = session
		case "-externalsessionactor":
			args[index+1] = actor
		case "-externalsessionstartedat":
			args[index+1] = startedAt
		}
	}
	var out bytes.Buffer
	if err := Run(args, &out); err != nil {
		t.Fatal(err)
	}
	var plan externalsession.AttemptPlan
	decodeJSONStrict(t, out.Bytes(), &plan)
	if plan.Dispatch == nil || plan.DispatchPath == "" || plan.DispatchSHA256 == "" || plan.ApplyCommand == "" {
		t.Fatalf("attempt preview omitted dispatch publication: %+v", plan)
	}
	if len(plan.Dispatch.Return.Templates) == 0 || !strings.Contains(plan.Dispatch.Return.Templates[0].JSON, "<attempt-receipt-sha256>") || !slices.Contains(plan.Dispatch.Return.Templates[0].RequiredReplacements, "<attempt-receipt-sha256>") {
		t.Fatalf("immutable pre-commit ticket did not defer final attempt receipt identity: %+v", plan.Dispatch.Return.Templates)
	}
	var ticketSubmission externalsession.Submission
	decodeJSONStrict(t, []byte(plan.Dispatch.Return.Templates[0].JSON), &ticketSubmission)
	for replacement, actual := range map[string]string{
		"<dispatch-claim-sha256>": ticketSubmission.DispatchClaimSHA256,
		"<launch-receipt-sha256>": ticketSubmission.LaunchReceiptSHA256,
		"<actual-harness>":        ticketSubmission.Harness,
		"<actual-session>":        ticketSubmission.Session,
	} {
		if actual != replacement || !slices.Contains(plan.Dispatch.Return.Templates[0].RequiredReplacements, replacement) {
			t.Fatalf("immutable ticket omitted accepted launch replacement %s: %+v", replacement, plan.Dispatch.Return.Templates[0])
		}
	}
	dispatchData, err := json.MarshalIndent(plan.Dispatch, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	dispatchData = append(dispatchData, '\n')
	dispatchPath := filepath.Join(operator.CaseRoot, filepath.FromSlash(plan.DispatchPath))
	if err := os.MkdirAll(filepath.Dir(dispatchPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dispatchPath, dispatchData, 0o600); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", operator.CaseRoot, "-Pack", operator.Pack, "-Lane", operator.Lane, "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var pending statusInventory
	decodeJSONStrict(t, out.Bytes(), &pending)
	pendingOperator := pending.MissionControlRunbook.CurrentLoopOperator
	if pendingOperator == nil || pendingOperator.State != "external-session-attempt-publication-pending" || pendingOperator.ExternalSessionJob == nil || pendingOperator.ExternalSessionJob.AttemptState != "attempt-publication-pending" || pendingOperator.ExternalSessionJob.Dispatcher == nil || pendingOperator.ExternalSessionJob.Dispatcher.State != "attempt-publication-pending" || pendingOperator.ExternalSessionJob.Dispatcher.Ticket == nil || pendingOperator.ExternalSessionJob.Dispatcher.Ticket.SHA256 != plan.DispatchSHA256 || pendingOperator.ExternalSessionJob.Dispatcher.Claim != nil || pendingOperator.ExternalSessionJob.Dispatcher.LaunchReceipt != nil {
		t.Fatalf("fresh status omitted exact ticket-only publication prefix: %+v", pendingOperator)
	}
	request := pendingOperator.ExternalSessionJob.AttemptRequest
	if request == nil || !request.CommandExecutable || request.State != "attempt-publication-pending" || request.Command != plan.ApplyCommand || pendingOperator.SelectedDriverRequest == nil || pendingOperator.SelectedDriverRequest.Command != request.Command {
		t.Fatalf("pending publication omitted exact recovery request: %+v", pendingOperator)
	}
	runbook := pending.MissionControlRunbook
	if runbook.CurrentDriverRequest == nil || runbook.CurrentDriverRequest.Source != "current-step-external-session" || !strings.Contains(runbook.CurrentDriverRequest.Command, "run-current-step") || runbook.CurrentRunLoopStepID != runbook.CurrentDriverRequest.RunLoopStepID || runbook.CurrentCommand != runbook.CurrentDriverRequest.Command {
		t.Fatalf("ticket-only recovery was not promoted through the unified current-step request: %+v", runbook)
	}
	quickstart := runbook.Quickstart
	takeover := runbook.ReplacementExecutorTakeover
	requestSHA256 := mission.ReplacementExecutorDriverRequestSHA256(*runbook.CurrentDriverRequest)
	if quickstart == nil || quickstart.CurrentDriverRequest == nil || quickstart.Command != runbook.CurrentDriverRequest.Command || quickstart.CurrentDriverRequest.Command != runbook.CurrentDriverRequest.Command || mission.ReplacementExecutorDriverRequestSHA256(*quickstart.CurrentDriverRequest) != requestSHA256 {
		t.Fatalf("ticket-only recovery quickstart diverged from the unified request: %+v", quickstart)
	}
	if takeover == nil || takeover.Command != runbook.CurrentDriverRequest.Command || takeover.CurrentDriverRequest.Command != runbook.CurrentDriverRequest.Command || takeover.CurrentDriverRequestSHA256 != requestSHA256 || mission.ReplacementExecutorDriverRequestSHA256(takeover.CurrentDriverRequest) != requestSHA256 {
		t.Fatalf("ticket-only recovery takeover diverged from the unified request: %+v", takeover)
	}
	unified := runMemberCurrentStep(t, operator.CaseRoot, []string{"-WhatIf"})
	if unified.ExternalSessionStep == nil || unified.ExternalSessionStep.Mode != "attempt-publication-recovery" || unified.ExternalSessionStep.Attempt == nil {
		t.Fatalf("unified current step omitted pending publication recovery: %+v", unified)
	}
	applied := runCurrentStepApply(t, operator.CaseRoot, unified)
	if applied.ExternalSessionStep == nil || applied.ExternalSessionStep.Attempt == nil || applied.ExternalSessionStep.Attempt.AlreadyApplied || applied.RefreshedStatus == nil || applied.ExternalSessionStep.Attempt.AttemptSHA256 != plan.AttemptSHA256 {
		t.Fatalf("pending publication recovery did not commit receipt last: %+v", applied)
	}
	replayed := runCurrentStepApply(t, operator.CaseRoot, unified)
	if replayed.ExternalSessionStep == nil || replayed.ExternalSessionStep.Attempt == nil || !replayed.ExternalSessionStep.Attempt.AlreadyApplied || replayed.ExternalSessionStep.Attempt.AttemptSHA256 != plan.AttemptSHA256 {
		t.Fatalf("pending publication recovery committed replay lost exact receipt: %+v", replayed)
	}
	refreshed := applied.RefreshedStatus.MissionControlRunbook.CurrentLoopOperator.ExternalSessionJob
	if refreshed == nil || refreshed.AttemptState != "committed" || refreshed.Dispatcher == nil || refreshed.Dispatcher.State != "queued" || refreshed.Dispatcher.ClaimRequest == nil {
		t.Fatalf("recovered attempt did not become a committed reservation ready for dispatch: %+v", refreshed)
	}
	return refreshed
}

func claimCurrentLoopExternalSessionDispatch(t *testing.T, operator *mission.CurrentLoopOperatorPackage, actor, observedAt string) *statusInventory {
	t.Helper()
	if operator == nil || operator.ExternalSessionJob == nil || operator.ExternalSessionJob.Dispatcher == nil || operator.ExternalSessionJob.Dispatcher.ClaimRequest == nil {
		t.Fatalf("operator omitted dispatch claim request: %+v", operator)
	}
	var out bytes.Buffer
	claimArgs := rekitCommandCLIArgs(t, operator.ExternalSessionJob.Dispatcher.ClaimRequest.Command)
	for index := range claimArgs {
		switch strings.ToLower(claimArgs[index]) {
		case "-externalsessionactor":
			claimArgs[index+1] = actor
		case "-externalsessionobservedat":
			claimArgs[index+1] = observedAt
		}
	}
	if err := Run(claimArgs, &out); err != nil {
		t.Fatal(err)
	}
	var claimPlan externalsession.DispatchPlan
	decodeJSONStrict(t, out.Bytes(), &claimPlan)
	out.Reset()
	if err := Run(rekitCommandCLIArgs(t, claimPlan.ApplyCommand), &out); err != nil {
		t.Fatal(err)
	}
	var claimed currentLoopExternalSessionDispatchResult
	decodeJSONStrict(t, out.Bytes(), &claimed)
	return claimed.RefreshedStatus
}

func recordCurrentLoopExternalSessionLaunch(t *testing.T, operator *mission.CurrentLoopOperatorPackage, actor, actualHarness, actualSession, observedAt string) *statusInventory {
	t.Helper()
	if operator == nil || operator.ExternalSessionJob == nil || operator.ExternalSessionJob.Dispatcher == nil || operator.ExternalSessionJob.Dispatcher.LaunchAcceptedRequest == nil {
		t.Fatalf("operator omitted accepted launch request: %+v", operator)
	}
	request := operator.ExternalSessionJob.Dispatcher.LaunchAcceptedRequest
	launchArgs := rekitCommandCLIArgs(t, request.Command)
	for index := range launchArgs {
		switch strings.ToLower(launchArgs[index]) {
		case "-externalsessionactor":
			launchArgs[index+1] = actor
		case "-externalsessionobservedat":
			launchArgs[index+1] = observedAt
		case "-externalsessionharness":
			launchArgs[index+1] = actualHarness
		case "-externalsessionid":
			launchArgs[index+1] = actualSession
		}
	}
	var out bytes.Buffer
	if err := Run(launchArgs, &out); err != nil {
		t.Fatal(err)
	}
	var launchPlan externalsession.DispatchPlan
	decodeJSONStrict(t, out.Bytes(), &launchPlan)
	out.Reset()
	if err := Run(rekitCommandCLIArgs(t, launchPlan.ApplyCommand), &out); err != nil {
		t.Fatal(err)
	}
	var launched currentLoopExternalSessionDispatchResult
	decodeJSONStrict(t, out.Bytes(), &launched)
	return launched.RefreshedStatus
}

func acceptCurrentLoopExternalSessionLaunch(t *testing.T, operator *mission.CurrentLoopOperatorPackage, actor, actualHarness, actualSession, observedAt string) *mission.CurrentLoopExternalSessionJob {
	t.Helper()
	claimed := claimCurrentLoopExternalSessionDispatch(t, operator, actor, observedAt)
	launched := recordCurrentLoopExternalSessionLaunch(t, claimed.MissionControlRunbook.CurrentLoopOperator, actor, actualHarness, actualSession, observedAt)
	return launched.MissionControlRunbook.CurrentLoopOperator.ExternalSessionJob
}

func bindCurrentLoopExternalSubmissionAttempt(t *testing.T, job *mission.CurrentLoopExternalSessionJob, submission map[string]any) {
	t.Helper()
	if job == nil || job.CurrentAttempt == nil || job.AttemptState != "committed" || job.Dispatcher == nil || job.Dispatcher.State != "running" {
		t.Fatalf("external session job has no accepted running launch: %+v", job)
	}
	submission["capability"] = job.Capability
	submission["attemptId"] = job.CurrentAttempt.AttemptID
	submission["attemptSha256"] = job.CurrentAttempt.AttemptSHA256
	submission["harness"] = job.CurrentAttempt.Harness
	submission["session"] = job.CurrentAttempt.Session
	if job.CurrentAttempt.LaunchControl != nil {
		submission["launchControl"] = job.CurrentAttempt.LaunchControl
	}
	if job.Dispatcher != nil && job.Dispatcher.Ticket != nil {
		if job.Dispatcher.Claim == nil || job.Dispatcher.LaunchReceipt == nil || job.Dispatcher.LaunchReceipt.State != "accepted" {
			t.Fatalf("dispatcher submission requires accepted launch lineage: %+v", job.Dispatcher)
		}
		submission["dispatchClaimSha256"] = job.Dispatcher.Claim.SHA256
		submission["launchReceiptSha256"] = job.Dispatcher.LaunchReceipt.SHA256
		submission["harness"] = job.Dispatcher.LaunchReceipt.ActualHarness
		submission["session"] = job.Dispatcher.LaunchReceipt.ActualSession
	}
}

func TestRunCurrentLoopExternalSessionInvalidReplacementRecoversTicketOnlyPrefix(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	board.Lanes[0].CurrentExecutor = "replacement-member"
	board.Lanes[0].ExecutorGeneration = 1
	board.Lanes[0].UpdatedAt = "2026-08-05T07:00:00Z"
	boardData, _ := json.MarshalIndent(board, "", "  ")
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), append(boardData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	preview := runCurrentLoopPreview(t, caseRoot, 5)
	dispatched := runCurrentLoopApplyWith(t, caseRoot, preview, "-ExpectedMemberExecutionPlanSha256", preview.InitialCurrentStep.MemberExecution.ExpectedPlanSHA256)
	if dispatched.SegmentCheckpoint == nil {
		t.Fatalf("missing external checkpoint: %+v", dispatched)
	}
	var out bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status statusInventory
	decodeJSONStrict(t, out.Bytes(), &status)
	operator := status.MissionControlRunbook.CurrentLoopOperator
	job := recordCurrentLoopExternalSessionAttempt(t, operator, "external-harness", "invalid-session", "mission-commander", "2026-08-05T07:00:00Z")
	invalidSubmission := map[string]any{
		"schemaVersion": 1, "kind": "current-loop-external-session-submission", "jobId": job.JobID, "jobSha256": job.JobSHA256,
		"outcome": "failed", "actor": "external-harness", "observedAt": "2026-08-05T07:00:01Z", "reason": "invalid lineage",
		"attemptId": job.CurrentAttempt.AttemptID, "attemptSha256": job.CurrentAttempt.AttemptSHA256,
		"harness": job.CurrentAttempt.Harness, "session": job.CurrentAttempt.Session,
		"noAuthorityOrConfirmed": true, "noHeavyTool": true,
	}
	invalidData, _ := json.MarshalIndent(invalidSubmission, "", "  ")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(caseRoot, filepath.FromSlash(job.SubmissionPath))), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, filepath.FromSlash(job.SubmissionPath)), append(invalidData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	decodeJSONStrict(t, out.Bytes(), &status)
	operator = status.MissionControlRunbook.CurrentLoopOperator
	if operator == nil || operator.State != "external-session-submission-invalid" || operator.ExternalSessionJob == nil || operator.ExternalSessionJob.CurrentAttempt == nil {
		t.Fatalf("invalid submission did not expose replacement review: %+v", operator)
	}
	replacementInput := runMemberCurrentStep(t, caseRoot, []string{"-WhatIf"})
	if replacementInput.ExternalSessionStep == nil || replacementInput.ExternalSessionStep.Mode != "attempt-input" || replacementInput.ExternalSessionStep.ReplacementRequest == nil || replacementInput.ExpectedCurrentStepPlanSHA256 != "" {
		t.Fatalf("unified invalid submission route omitted typed replacement input: current=%+v external=%+v", replacementInput, replacementInput.ExternalSessionStep)
	}
	replacementPreview := runMemberCurrentStep(t, caseRoot, []string{"-ExternalSessionHarness", "external-harness", "-ExternalSessionId", "replacement-session-unified", "-ExternalSessionActor", "mission-commander", "-ExternalSessionStartedAt", "2026-08-05T07:00:02Z", "-ExpectedExternalSessionAttemptSha256", operator.ExternalSessionJob.CurrentAttempt.AttemptSHA256, "-WhatIf"})
	if replacementPreview.ExternalSessionStep == nil || replacementPreview.ExternalSessionStep.Mode != "replacement-attempt" || replacementPreview.ExternalSessionStep.Attempt == nil || replacementPreview.ExpectedCurrentStepPlanSHA256 == "" {
		t.Fatalf("unified invalid submission route omitted exact replacement plan: %+v", replacementPreview)
	}
	recovered := recordCurrentLoopExternalSessionAttemptWithPendingRecovery(t, operator, "external-harness", "replacement-session", "mission-commander", "2026-08-05T07:00:02Z")
	if recovered.CurrentAttempt == nil || recovered.CurrentAttempt.Generation != 2 || recovered.Dispatcher == nil || recovered.Dispatcher.State != "queued" || recovered.Dispatcher.Ticket == nil || recovered.Dispatcher.Ticket.Generation != 2 {
		t.Fatalf("invalid replacement ticket-only prefix did not recover exact next generation: %+v", recovered)
	}
}

func TestRunCurrentLoopObservationInboxOutranksDispatcherStates(t *testing.T) {
	for _, dispatcherState := range []string{"queued", "claimed", "running"} {
		t.Run(dispatcherState, func(t *testing.T) {
			caseRoot := fullAttachedCase(t)
			if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &bytes.Buffer{}); err != nil {
				t.Fatal(err)
			}
			board, err := mission.ReadBoard(caseRoot)
			if err != nil {
				t.Fatal(err)
			}
			board.Lanes[0].CurrentExecutor = "observation-member"
			board.Lanes[0].ExecutorGeneration = 1
			board.Lanes[0].UpdatedAt = "2026-08-05T07:10:00Z"
			boardData, _ := json.MarshalIndent(board, "", "  ")
			if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), append(boardData, '\n'), 0o644); err != nil {
				t.Fatal(err)
			}
			preview := runCurrentLoopPreview(t, caseRoot, 5)
			memberPlan := preview.InitialCurrentStep.MemberExecution
			dispatched := runCurrentLoopApplyWith(t, caseRoot, preview, "-ExpectedMemberExecutionPlanSha256", memberPlan.ExpectedPlanSHA256)
			var out bytes.Buffer
			if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &out); err != nil {
				t.Fatal(err)
			}
			var status statusInventory
			decodeJSONStrict(t, out.Bytes(), &status)
			operator := status.MissionControlRunbook.CurrentLoopOperator
			job := recordCurrentLoopExternalSessionAttempt(t, operator, "external-harness", "observation-session", "mission-commander", "2026-08-05T07:10:00Z")
			operator.ExternalSessionJob = job
			switch dispatcherState {
			case "claimed":
				status = *claimCurrentLoopExternalSessionDispatch(t, operator, "dispatcher", "2026-08-05T07:10:01Z")
			case "running":
				claimed := claimCurrentLoopExternalSessionDispatch(t, operator, "dispatcher", "2026-08-05T07:10:01Z")
				status = *recordCurrentLoopExternalSessionLaunch(t, claimed.MissionControlRunbook.CurrentLoopOperator, "dispatcher", "actual-harness", "actual-session", "2026-08-05T07:10:02Z")
			}
			observationDir := mustCurrentLoopObservationInboxPath(t, caseRoot)
			if err := os.MkdirAll(observationDir, 0o700); err != nil {
				t.Fatal(err)
			}
			observationPath := filepath.Join(observationDir, "member-accepted.json")
			observationData, err := json.Marshal(currentLoopObservationEnvelope{
				SchemaVersion: 1, Kind: "current-loop-external-session-observation", CheckpointSHA256: dispatched.SegmentCheckpoint.ArtifactSHA256,
				ObservationKind: "member-session-accepted", Actor: "harness", ObservedAt: "2026-08-05T07:10:03Z", MemberAttemptID: memberPlan.AttemptID,
				NoAuthorityOrConfirmed: true, NoHeavyTool: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(observationPath, append(observationData, '\n'), 0o600); err != nil {
				t.Fatal(err)
			}
			out.Reset()
			if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &out); err != nil {
				t.Fatal(err)
			}
			decodeJSONStrict(t, out.Bytes(), &status)
			operator = status.MissionControlRunbook.CurrentLoopOperator
			if operator == nil || operator.ObservationInbox == nil || operator.ObservationInbox.State != "ready" || operator.SelectedDriverRequest == nil || operator.ObservationInbox.SelectedDriverRequest == nil || operator.SelectedDriverRequest.Command != operator.ObservationInbox.SelectedDriverRequest.Command || !strings.Contains(operator.SelectedDriverRequest.Command, observationPath) || operator.ExternalSessionJob == nil || operator.ExternalSessionJob.Dispatcher == nil || operator.ExternalSessionJob.Dispatcher.State != dispatcherState {
				t.Fatalf("ready observation was not authoritative over dispatcher %s: %+v", dispatcherState, operator)
			}
		})
	}
}

func TestExternalReviewerSubmissionTemplateUsesAcceptedActualIdentity(t *testing.T) {
	job, err := externalsession.NewReviewerJob(t.TempDir(), "_template", strings.Repeat("a", 64), externalsession.ReviewerIdentity{
		AttemptSHA256: strings.Repeat("b", 64), PacketID: "packet", RouteID: "route", ShardID: "shard",
		Items: []string{"item"}, OutputFields: []string{"item"}, DispatchPath: ".rekit/dispatch.json",
		DispatchSHA256: strings.Repeat("d", 64), DispatchID: "dispatch", Harness: "reserved-harness", Session: "reserved-session",
	}, []string{"accepted", "returned", "failed"})
	if err != nil {
		t.Fatal(err)
	}
	attempt := externalsession.AttemptInspection{
		Current:       &externalsession.Attempt{AttemptID: "reviewer-attempt-g000001", Harness: "reserved-harness", Session: "reserved-session"},
		AttemptSHA256: strings.Repeat("c", 64),
	}
	dispatcher := &mission.CurrentLoopExternalSessionDispatcher{
		Ticket: &mission.CurrentLoopExternalSessionDispatchTicket{},
		Claim:  &mission.CurrentLoopExternalSessionDispatchClaim{SHA256: strings.Repeat("d", 64)},
		LaunchReceipt: &mission.CurrentLoopExternalSessionLaunchReceipt{
			State: "accepted", SHA256: strings.Repeat("e", 64), ActualHarness: "actual-harness", ActualSession: "actual-session",
		},
	}
	for _, outcome := range []string{"accepted", "returned"} {
		template, _ := externalSessionSubmissionTemplate(job, strings.Repeat("f", 64), attempt, dispatcher, outcome)
		var submission externalsession.Submission
		decodeJSONStrict(t, []byte(template), &submission)
		if submission.Harness != "actual-harness" || submission.Session != "actual-session" || submission.ReviewerSession != "actual-session" || (outcome == "accepted" && submission.ReviewerHarness != "actual-harness") {
			t.Fatalf("reviewer %s template retained reservation identity: %+v", outcome, submission)
		}
	}
}

func TestRunCurrentLoopExternalSessionTurnAdvancesMemberResult(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	board.Lanes[0].CurrentExecutor = "turn-member"
	board.Lanes[0].ExecutorGeneration = 1
	board.Lanes[0].UpdatedAt = "2026-08-05T01:00:00Z"
	boardData, _ := json.MarshalIndent(board, "", "  ")
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), append(boardData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	setCurrentLoopLaneOwner(t, caseRoot, "main", "turn-member", 1, "2026-08-05T01:00:00Z")
	preview := runCurrentLoopPreview(t, caseRoot, 5)
	memberPlan := preview.InitialCurrentStep.MemberExecution
	dispatched := runCurrentLoopApplyWith(t, caseRoot, preview, "-ExpectedMemberExecutionPlanSha256", memberPlan.ExpectedPlanSHA256)
	if dispatched.SegmentCheckpoint == nil || dispatched.StopReason.Code != "external-member-handoff" {
		t.Fatalf("member dispatch did not reach external boundary: %+v", dispatched)
	}
	var statusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	var status statusInventory
	decodeJSONStrict(t, statusOut.Bytes(), &status)
	operator := status.MissionControlRunbook.CurrentLoopOperator
	firstJob := recordCurrentLoopExternalSessionAttempt(t, operator, "external-turn-harness", "member-session-a", "mission-commander", "2026-08-05T01:00:00Z")
	if firstJob == nil || firstJob.State != "awaiting-submission" || firstJob.AttemptState != "committed" || firstJob.CurrentAttempt == nil || firstJob.CurrentAttempt.Generation != 1 || firstJob.CurrentAttempt.AttemptSHA256 == "" || firstJob.SubmissionPath != firstJob.CurrentAttempt.SubmissionPath || firstJob.CurrentAttempt.SubmissionOutputs == "" {
		t.Fatalf("first member external attempt is not durably projected: %+v", firstJob)
	}
	operator.ExternalSessionJob = firstJob
	job := recordCurrentLoopExternalSessionAttempt(t, operator, "external-turn-harness", "member-session-b", "mission-commander", "2026-08-05T01:00:10Z")
	operator.ExternalSessionJob = job
	job = acceptCurrentLoopExternalSessionLaunch(t, operator, "dispatcher", "external-turn-harness", "member-session-b", "2026-08-05T01:00:11Z")
	if job == nil || job.State != "awaiting-submission" || job.AttemptState != "committed" || job.CurrentAttempt == nil || job.CurrentAttempt.Generation != 2 || job.CurrentAttempt.SupersedesSHA256 != firstJob.CurrentAttempt.AttemptSHA256 || job.CurrentAttempt.Session != "member-session-b" {
		t.Fatalf("replacement member external attempt is not current: first=%+v replacement=%+v", firstJob, job)
	}
	if harness := job.HarnessPackage; harness == nil || harness.State != "running" || harness.Launch == nil || harness.Launch.Ready || harness.Launch.Attempt.Generation != 2 || harness.Launch.Attempt.AttemptSHA256 != job.CurrentAttempt.AttemptSHA256 || harness.Launch.Attempt.AttemptSHA256 == firstJob.CurrentAttempt.AttemptSHA256 || harness.Launch.Attempt.Session != "member-session-b" || harness.Return == nil || harness.Return.SubmissionPath != job.SubmissionPath || strings.Contains(harness.Return.Templates[0].JSON, firstJob.CurrentAttempt.AttemptSHA256) {
		t.Fatalf("replacement harness package did not revoke the stale generation: first=%+v replacement=%+v", firstJob.HarnessPackage, harness)
	}
	staleSubmission := map[string]any{
		"schemaVersion": 1, "kind": "current-loop-external-session-submission", "jobId": firstJob.JobID, "jobSha256": firstJob.JobSHA256,
		"outcome": "returned", "actor": "external-turn-harness", "observedAt": "2026-08-05T01:00:20Z", "summary": "late first session result",
		"attemptId": firstJob.CurrentAttempt.AttemptID, "attemptSha256": firstJob.CurrentAttempt.AttemptSHA256,
		"harness": firstJob.CurrentAttempt.Harness, "session": firstJob.CurrentAttempt.Session,
		"noAuthorityOrConfirmed": true, "noHeavyTool": true,
	}
	staleSubmissionData, _ := json.MarshalIndent(staleSubmission, "", "  ")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(caseRoot, filepath.FromSlash(firstJob.SubmissionPath))), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, filepath.FromSlash(firstJob.SubmissionPath)), append(staleSubmissionData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	statusOut.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	status = statusInventory{}
	decodeJSONStrict(t, statusOut.Bytes(), &status)
	staleOperator := status.MissionControlRunbook.CurrentLoopOperator
	if staleOperator == nil || staleOperator.State != "external-session-running" || !staleOperator.Ready || staleOperator.ExternalSessionJob == nil || staleOperator.ExternalSessionJob.State != "awaiting-submission" || staleOperator.ExternalSessionJob.CurrentAttempt == nil || staleOperator.ExternalSessionJob.CurrentAttempt.Generation != 2 || staleOperator.ExternalSessionJob.SubmissionPath != job.SubmissionPath {
		t.Fatalf("superseded session submission affected current owner: %+v", staleOperator)
	}
	jobRoot := filepath.Join(caseRoot, filepath.Dir(filepath.FromSlash(job.SubmissionPath)))
	if err := os.MkdirAll(filepath.Join(jobRoot, "outputs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobRoot, "outputs", "result.txt"), []byte("turn member result\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	submission := map[string]any{
		"schemaVersion": 1, "kind": "current-loop-external-session-submission", "jobId": job.JobID, "jobSha256": job.JobSHA256,
		"outcome": "returned", "actor": "external-turn-harness", "observedAt": "2026-08-05T01:00:30Z", "summary": "member returned through one reviewed turn", "noAuthorityOrConfirmed": true, "noHeavyTool": true,
	}
	bindCurrentLoopExternalSubmissionAttempt(t, job, submission)
	submissionData, _ := json.MarshalIndent(submission, "", "  ")
	if err := os.WriteFile(filepath.Join(caseRoot, filepath.FromSlash(job.SubmissionPath)), append(submissionData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	statusOut.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	decodeJSONStrict(t, statusOut.Bytes(), &status)
	operator = status.MissionControlRunbook.CurrentLoopOperator
	if operator == nil || operator.SelectedDriverRequest == nil || !strings.Contains(operator.SelectedDriverRequest.Command, "-AdvanceExternalSessionResult") || operator.ExternalSessionJob.RelayPreviewRequest == nil {
		t.Fatalf("status did not select one reviewed external-result turn: %+v", operator)
	}
	before := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	current := runMemberCurrentStep(t, caseRoot, []string{"-WhatIf"})
	if current.ExternalSessionStep == nil || current.ExternalSessionStep.Mode != "result-turn" || current.ExternalSessionStep.Turn == nil || current.ExpectedCurrentStepPlanSHA256 == "" {
		t.Fatalf("unified current step omitted external-result turn: %+v", current)
	}
	turn := current.ExternalSessionStep.Turn
	if turn.ExpectedPlanSHA256 == "" || turn.ApplyCommand == "" || turn.Relay.Observation.SHA256 == "" || turn.Resume.ExpectedCurrentLoopPlanSHA256 == "" || turn.Resume.InitialCurrentStep == nil || turn.Resume.InitialCurrentStep.MemberExecution == nil || turn.Resume.InitialCurrentStep.MemberExecution.Outcome != "returned" {
		t.Fatalf("external-result turn preview omitted exact relay/resume binding: %+v", turn)
	}
	assertSnapshotEqual(t, before, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))
	current = runCurrentStepApply(t, caseRoot, current)
	turn = current.ExternalSessionStep.Turn
	if !turn.Applied || !turn.Relay.Applied || !turn.Resume.Applied || turn.Resume.AppliedSteps != 1 || turn.Resume.ObservationReceipt == nil || turn.Resume.SegmentCheckpoint == nil || turn.RefreshedStatus == nil {
		t.Fatalf("external-result turn did not relay, intake, and checkpoint in one Apply: %+v", turn)
	}
	if turn.Resume.ObservationReceipt.SourceCheckpointSHA256 != dispatched.SegmentCheckpoint.ArtifactSHA256 || !strings.EqualFold(turn.Resume.ObservationReceipt.ObservationSHA256, turn.Relay.Observation.SHA256) {
		t.Fatalf("external-result turn receipt identity drifted: %+v", turn)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, filepath.FromSlash(job.MemberManifestPath))); err != nil {
		t.Fatalf("external-result turn did not publish member manifest: %v", err)
	}
	assertFileNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertFileNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))
}

func TestRunCurrentStepUnifiesExternalSessionCampaign(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	board.Lanes[0].CurrentExecutor = "unified-member"
	board.Lanes[0].ExecutorGeneration = 1
	board.Lanes[0].UpdatedAt = "2026-08-05T09:00:00Z"
	boardData, _ := json.MarshalIndent(board, "", "  ")
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), append(boardData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	loop := runCurrentLoopPreview(t, caseRoot, 5)
	runCurrentLoopApplyWith(t, caseRoot, loop, "-ExpectedMemberExecutionPlanSha256", loop.InitialCurrentStep.MemberExecution.ExpectedPlanSHA256)

	preview := runMemberCurrentStep(t, caseRoot, []string{"-ExternalSessionHarness", "unified-harness", "-ExternalSessionId", "unified-session", "-ExternalSessionActor", "mission-commander", "-ExternalSessionStartedAt", "2026-08-05T09:00:01Z", "-WhatIf"})
	if preview.ExternalSessionStep == nil || preview.ExternalSessionStep.Mode != "attempt" || preview.ExternalSessionStep.Attempt == nil || preview.ExpectedCurrentStepPlanSHA256 == "" {
		t.Fatalf("unified current step omitted attempt plan: %+v", preview)
	}
	attemptInputs := []string{"-ExternalSessionHarness", "unified-harness", "-ExternalSessionId", "unified-session", "-ExternalSessionActor", "mission-commander", "-ExternalSessionStartedAt", "2026-08-05T09:00:01Z"}
	applied := runCurrentStepApply(t, caseRoot, preview, attemptInputs...)
	if applied.ExternalSessionStep == nil || applied.ExternalSessionStep.Attempt == nil || applied.RefreshedStatus == nil {
		t.Fatalf("unified attempt apply omitted refreshed dispatcher state: %+v", applied)
	}
	attemptReplay := runCurrentStepApply(t, caseRoot, preview, attemptInputs...)
	if attemptReplay.ExternalSessionStep == nil || attemptReplay.ExternalSessionStep.Attempt == nil || !attemptReplay.ExternalSessionStep.Attempt.AlreadyApplied {
		t.Fatalf("unified attempt committed replay lost exact receipt: %+v", attemptReplay)
	}

	claimInput := runMemberCurrentStep(t, caseRoot, []string{"-WhatIf"})
	if claimInput.ExternalSessionStep == nil || claimInput.ExternalSessionStep.Mode != "dispatch-claim-input" || claimInput.ExpectedCurrentStepPlanSHA256 != "" {
		t.Fatalf("queued dispatcher did not return typed input handoff: %+v", claimInput)
	}
	var wrongOut bytes.Buffer
	wrongClaimArgs := []string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-ExternalSessionActor", "dispatcher", "-ExternalSessionObservedAt", "2026-08-05T09:00:02Z", "-ExternalSessionLaunchOutcome", "failed", "-ExternalSessionLaunchReason", "ignored", "-WhatIf", "-Format", "json"}
	if err := Run(wrongClaimArgs, &wrongOut); err == nil {
		t.Fatalf("queued claim silently accepted launch fields: %s", wrongOut.String())
	}
	claim := runMemberCurrentStep(t, caseRoot, []string{"-ExternalSessionActor", "dispatcher", "-ExternalSessionObservedAt", "2026-08-05T09:00:02Z", "-WhatIf"})
	if claim.ExternalSessionStep == nil || claim.ExternalSessionStep.Mode != "dispatch-claim" || claim.ExternalSessionStep.Dispatch == nil {
		t.Fatalf("unified current step omitted claim plan: %+v", claim)
	}
	claimInputs := []string{"-ExternalSessionActor", "dispatcher", "-ExternalSessionObservedAt", "2026-08-05T09:00:02Z"}
	runCurrentStepApply(t, caseRoot, claim, claimInputs...)
	claimReplay := runCurrentStepApply(t, caseRoot, claim, claimInputs...)
	if claimReplay.ExternalSessionStep == nil || claimReplay.ExternalSessionStep.Dispatch == nil || !claimReplay.ExternalSessionStep.Dispatch.AlreadyApplied {
		t.Fatalf("unified claim committed replay lost exact receipt: %+v", claimReplay)
	}

	launch := runMemberCurrentStep(t, caseRoot, []string{"-ExternalSessionLaunchOutcome", "accepted", "-ExternalSessionActor", "dispatcher", "-ExternalSessionObservedAt", "2026-08-05T09:00:03Z", "-ExternalSessionHarness", "actual-harness", "-ExternalSessionId", "actual-session", "-WhatIf"})
	if launch.ExternalSessionStep == nil || launch.ExternalSessionStep.Mode != "launch-accepted" || launch.ExternalSessionStep.Dispatch == nil {
		t.Fatalf("unified current step omitted accepted launch plan: %+v", launch)
	}
	launchInputs := []string{"-ExternalSessionLaunchOutcome", "accepted", "-ExternalSessionActor", "dispatcher", "-ExternalSessionObservedAt", "2026-08-05T09:00:03Z", "-ExternalSessionHarness", "actual-harness", "-ExternalSessionId", "actual-session"}
	launched := runCurrentStepApply(t, caseRoot, launch, launchInputs...)
	launchReplay := runCurrentStepApply(t, caseRoot, launch, launchInputs...)
	if launchReplay.ExternalSessionStep == nil || launchReplay.ExternalSessionStep.Dispatch == nil || !launchReplay.ExternalSessionStep.Dispatch.AlreadyApplied {
		t.Fatalf("unified accepted launch committed replay lost exact receipt: %+v", launchReplay)
	}
	running := launched.RefreshedStatus.MissionControlRunbook
	if running.CurrentDriverRequest == nil || running.CurrentDriverRequest.Source != "current-step-external-session" || !running.CurrentDriverRequest.CommandExecutable || !strings.Contains(running.CurrentDriverRequest.Command, "run-current-step") || running.Quickstart == nil || running.ReplacementExecutorTakeover == nil || running.Quickstart.CurrentDriverRequest == nil || running.Quickstart.CurrentDriverRequest.ActionID != running.CurrentDriverRequest.ActionID || running.ReplacementExecutorTakeover.CurrentDriverRequest.ActionID != running.CurrentDriverRequest.ActionID {
		t.Fatalf("running lifecycle was not the unique unified fresh-session handoff: %+v", running)
	}
	if err := mission.ValidateMissionCommanderDriverRequest(*running.CurrentDriverRequest); err != nil {
		t.Fatalf("running external-session wrapper is not a valid typed request: %v", err)
	}
	for _, binding := range []struct {
		name    string
		command string
	}{
		{name: "command", command: running.CurrentDriverRequest.Command},
		{name: "receipt command", command: running.CurrentDriverRequest.ExpectedReceipt.Command},
	} {
		if !strings.HasPrefix(binding.command, "/rekit run-current-step ") ||
			!strings.Contains(binding.command, "-Target "+caseRoot) ||
			!strings.Contains(binding.command, "-Lane main") ||
			!strings.Contains(binding.command, "-WhatIf") ||
			!strings.Contains(binding.command, "-Format json") {
			t.Fatalf("running wrapper %s is not exact: %q", binding.name, binding.command)
		}
	}
	refresh := running.CurrentDriverRequest.ExpectedReceipt.RefreshStatusCommand
	if !strings.HasPrefix(refresh, "/rekit status ") ||
		!strings.Contains(refresh, "-Target "+caseRoot) ||
		!strings.Contains(refresh, "-Lane main") ||
		!strings.Contains(refresh, "-Format compact-json") ||
		strings.Contains(refresh, "-WhatIf") || strings.Contains(refresh, "-Apply") {
		t.Fatalf("running wrapper compact refresh is not exact and read-only: %q", refresh)
	}
	var handoffOut bytes.Buffer
	if err := Run([]string{
		"-Command", "handoff", "-Target", caseRoot,
		"-Pack", "_template", "-Lane", "main",
		"-WhatIf", "-Format", "json",
	}, &handoffOut); err != nil {
		t.Fatalf("active external-session handoff rejected typed wrapper: %v", err)
	}
	var handoff workstream.HandoffResult
	decodeJSONStrict(t, handoffOut.Bytes(), &handoff)
	published := handoff.MissionCommanderActionQueue.CurrentDriverRequest
	if published == nil || published.Command != running.CurrentDriverRequest.Command ||
		published.ExpectedReceipt.RefreshStatusCommand != refresh ||
		handoff.ReplacementExecutorTakeoverPackage == nil ||
		handoff.ReplacementExecutorTakeoverPackage.CurrentDriverRequest.Command != running.CurrentDriverRequest.Command {
		t.Fatalf("active external-session handoff drifted typed wrapper: %+v", handoff)
	}
	runningPreview := runMemberCurrentStep(t, caseRoot, []string{"-WhatIf"})
	if runningPreview.ExternalSessionStep == nil || runningPreview.ExternalSessionStep.Mode != "running-handoff" || runningPreview.ExternalSessionStep.ReplacementRequest == nil || runningPreview.ExpectedCurrentStepPlanSHA256 != "" {
		t.Fatalf("running current step did not preserve explicit replacement review: %+v", runningPreview)
	}
	currentAttemptSHA := launched.ExternalSessionStep.Dispatch.AttemptSHA256
	replacementInputs := []string{"-ExternalSessionHarness", "replacement-harness", "-ExternalSessionId", "replacement-session", "-ExternalSessionActor", "mission-commander", "-ExternalSessionStartedAt", "2026-08-05T09:00:05Z", "-ExpectedExternalSessionAttemptSha256", currentAttemptSHA}
	replacement := runMemberCurrentStep(t, caseRoot, append(append([]string{}, replacementInputs...), "-WhatIf"))
	if replacement.ExternalSessionStep == nil || replacement.ExternalSessionStep.Mode != "replacement-attempt" || replacement.ExternalSessionStep.Attempt == nil || replacement.ExpectedCurrentStepPlanSHA256 == "" {
		t.Fatalf("running current step omitted explicit replacement plan: %+v", replacement)
	}
	runCurrentStepApply(t, caseRoot, replacement, replacementInputs...)
	replacementReplay := runCurrentStepApply(t, caseRoot, replacement, replacementInputs...)
	if replacementReplay.ExternalSessionStep == nil || replacementReplay.ExternalSessionStep.Attempt == nil || !replacementReplay.ExternalSessionStep.Attempt.AlreadyApplied {
		t.Fatalf("running replacement committed replay lost exact receipt: %+v", replacementReplay)
	}
}

func TestRunCurrentStepExternalSessionFailedLaunchReplay(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	board.Lanes[0].CurrentExecutor = "failed-launch-member"
	board.Lanes[0].ExecutorGeneration = 1
	board.Lanes[0].UpdatedAt = "2026-08-05T09:10:00Z"
	boardData, _ := json.MarshalIndent(board, "", "  ")
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), append(boardData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	loop := runCurrentLoopPreview(t, caseRoot, 5)
	runCurrentLoopApplyWith(t, caseRoot, loop, "-ExpectedMemberExecutionPlanSha256", loop.InitialCurrentStep.MemberExecution.ExpectedPlanSHA256)
	attempt := runMemberCurrentStep(t, caseRoot, []string{"-ExternalSessionHarness", "failed-harness", "-ExternalSessionId", "failed-session", "-ExternalSessionActor", "mission-commander", "-ExternalSessionStartedAt", "2026-08-05T09:10:01Z", "-WhatIf"})
	runCurrentStepApply(t, caseRoot, attempt, "-ExternalSessionHarness", "failed-harness", "-ExternalSessionId", "failed-session", "-ExternalSessionActor", "mission-commander", "-ExternalSessionStartedAt", "2026-08-05T09:10:01Z")
	claim := runMemberCurrentStep(t, caseRoot, []string{"-ExternalSessionActor", "dispatcher", "-ExternalSessionObservedAt", "2026-08-05T09:10:02Z", "-WhatIf"})
	runCurrentStepApply(t, caseRoot, claim, "-ExternalSessionActor", "dispatcher", "-ExternalSessionObservedAt", "2026-08-05T09:10:02Z")
	failedInputs := []string{"-ExternalSessionLaunchOutcome", "failed", "-ExternalSessionActor", "dispatcher", "-ExternalSessionObservedAt", "2026-08-05T09:10:03Z", "-ExternalSessionLaunchReason", "harness refused launch"}
	failed := runMemberCurrentStep(t, caseRoot, append(append([]string{}, failedInputs...), "-WhatIf"))
	if failed.ExternalSessionStep == nil || failed.ExternalSessionStep.Mode != "launch-failed" || failed.ExternalSessionStep.Dispatch == nil {
		t.Fatalf("unified current step omitted failed launch plan: %+v", failed)
	}
	runCurrentStepApply(t, caseRoot, failed, failedInputs...)
	replayed := runCurrentStepApply(t, caseRoot, failed, failedInputs...)
	if replayed.ExternalSessionStep == nil || replayed.ExternalSessionStep.Dispatch == nil || !replayed.ExternalSessionStep.Dispatch.AlreadyApplied {
		t.Fatalf("unified failed launch committed replay lost exact receipt: %+v", replayed)
	}
	replacement := runMemberCurrentStep(t, caseRoot, []string{"-ExternalSessionHarness", "replacement-harness", "-ExternalSessionId", "replacement-session", "-ExternalSessionActor", "mission-commander", "-ExternalSessionStartedAt", "2026-08-05T09:10:04Z", "-ExpectedExternalSessionAttemptSha256", replayed.ExternalSessionStep.Dispatch.AttemptSHA256, "-WhatIf"})
	if replacement.ExternalSessionStep == nil || replacement.ExternalSessionStep.Mode != "replacement-attempt" || replacement.ExpectedCurrentStepPlanSHA256 == "" {
		t.Fatalf("failed launch did not route to exact replacement attempt: %+v", replacement)
	}
}

func TestRunCurrentLoopExternalSessionTurnPreservesRelayButRefusesClaimAfterHumanIntervention(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	board.Lanes[0].CurrentExecutor = "turn-member"
	board.Lanes[0].ExecutorGeneration = 1
	board.Lanes[0].UpdatedAt = "2026-08-05T01:05:00Z"
	boardData, _ := json.MarshalIndent(board, "", "  ")
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), append(boardData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	setCurrentLoopLaneOwner(t, caseRoot, "main", "turn-member", 1, "2026-08-05T01:05:00Z")
	preview := runCurrentLoopPreview(t, caseRoot, 5)
	dispatched := runCurrentLoopApplyWith(t, caseRoot, preview, "-ExpectedMemberExecutionPlanSha256", preview.InitialCurrentStep.MemberExecution.ExpectedPlanSHA256)
	if dispatched.SegmentCheckpoint == nil {
		t.Fatalf("missing external checkpoint: %+v", dispatched)
	}
	var statusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	var status statusInventory
	decodeJSONStrict(t, statusOut.Bytes(), &status)
	operator := status.MissionControlRunbook.CurrentLoopOperator
	job := recordCurrentLoopExternalSessionAttempt(t, operator, "external-turn-harness", "member-session-intervention", "mission-commander", "2026-08-05T01:05:00Z")
	operator.ExternalSessionJob = job
	job = acceptCurrentLoopExternalSessionLaunch(t, operator, "dispatcher", "external-turn-harness", "member-session-intervention", "2026-08-05T01:05:01Z")
	jobRoot := filepath.Join(caseRoot, filepath.Dir(filepath.FromSlash(job.SubmissionPath)))
	if err := os.MkdirAll(filepath.Join(jobRoot, "outputs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobRoot, "outputs", "result.txt"), []byte("intervention turn result\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	submission := map[string]any{
		"schemaVersion": 1, "kind": "current-loop-external-session-submission", "jobId": job.JobID, "jobSha256": job.JobSHA256,
		"outcome": "returned", "actor": "external-turn-harness", "observedAt": "2026-08-05T01:05:30Z", "summary": "relay before intervention", "noAuthorityOrConfirmed": true, "noHeavyTool": true,
	}
	bindCurrentLoopExternalSubmissionAttempt(t, job, submission)
	submissionData, _ := json.MarshalIndent(submission, "", "  ")
	if err := os.WriteFile(filepath.Join(caseRoot, filepath.FromSlash(job.SubmissionPath)), append(submissionData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	statusOut.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	decodeJSONStrict(t, statusOut.Bytes(), &status)
	current := runMemberCurrentStep(t, caseRoot, []string{"-WhatIf"})
	if current.ExternalSessionStep == nil || current.ExternalSessionStep.Mode != "result-turn" || current.ExternalSessionStep.Turn == nil {
		t.Fatalf("unified current step omitted intervention result turn: %+v", current)
	}
	currentLoopExternalTurnBeforeClaimHook = func() error {
		appendCurrentLoopOpenIntervention(t, caseRoot, "int-external-turn-claim")
		return nil
	}
	t.Cleanup(func() { currentLoopExternalTurnBeforeClaimHook = nil })
	var partialOut bytes.Buffer
	err = Run([]string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-ExpectedCurrentStepPlanSha256", current.ExpectedCurrentStepPlanSHA256, "-Apply", "-Format", "json"}, &partialOut)
	if err == nil || !strings.Contains(err.Error(), "Human-in-the-Lane intervention") {
		t.Fatalf("intervention before claim error=%v", err)
	}
	var partial currentStepTestPlan
	if err := json.Unmarshal(partialOut.Bytes(), &partial); err != nil {
		t.Fatalf("unified partial receipt JSON did not decode: %v\n%s", err, partialOut.String())
	}
	if !partial.Applied || partial.Receipt == nil || partial.Receipt.State != "nested-partial" || !strings.Contains(partial.Receipt.Outcome, "checkpoint-claim") || partial.ExternalSessionStep == nil || partial.ExternalSessionStep.Turn == nil || !partial.ExternalSessionStep.Turn.Relay.Applied || partial.ExternalSessionStep.Turn.FailureStage != "checkpoint-claim" {
		t.Fatalf("unified partial receipt did not preserve committed relay truth: %+v", partial)
	}
	currentLoopExternalTurnBeforeClaimHook = nil
	if got, err := os.ReadFile(filepath.Join(caseRoot, filepath.FromSlash(job.MemberManifestPath))); err != nil || len(got) == 0 {
		t.Fatalf("committed relay member manifest was not preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, filepath.FromSlash(job.ObservationPath))); err != nil {
		t.Fatalf("committed relay observation was not preserved: %v", err)
	}
	claimPath := filepath.Join(caseRoot, ".rekit", "runs", "current-loop-segment-claims", strings.ToLower(dispatched.SegmentCheckpoint.ArtifactSHA256)+".json")
	assertFileNotExists(t, claimPath)
	inspection, ok, err := memberexecution.Latest(caseRoot, "main")
	if err != nil || !ok || inspection.State != "handoff-ready" {
		t.Fatalf("nested member observation mutated after intervention: inspection=%+v ok=%v err=%v", inspection, ok, err)
	}
	statusOut.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	decodeJSONStrict(t, statusOut.Bytes(), &status)
	if status.MissionControlRunbook.CurrentDriverRequest == nil || driverStepCommandName(status.MissionControlRunbook.CurrentDriverRequest.Command) != "reconcile" {
		t.Fatalf("fresh status did not surface Human reconcile after preserved relay: %+v", status.MissionControlRunbook.CurrentDriverRequest)
	}
}

func TestRunCurrentLoopExternalSessionTurnRejectsStaleTurnHashBeforeRelay(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	board.Lanes[0].CurrentExecutor = "turn-member"
	board.Lanes[0].ExecutorGeneration = 1
	board.Lanes[0].UpdatedAt = "2026-08-05T01:10:00Z"
	boardData, _ := json.MarshalIndent(board, "", "  ")
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), append(boardData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	setCurrentLoopLaneOwner(t, caseRoot, "main", "turn-member", 1, "2026-08-05T01:10:00Z")
	preview := runCurrentLoopPreview(t, caseRoot, 5)
	dispatched := runCurrentLoopApplyWith(t, caseRoot, preview, "-ExpectedMemberExecutionPlanSha256", preview.InitialCurrentStep.MemberExecution.ExpectedPlanSHA256)
	if dispatched.SegmentCheckpoint == nil {
		t.Fatalf("missing external checkpoint: %+v", dispatched)
	}
	var statusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	var status statusInventory
	decodeJSONStrict(t, statusOut.Bytes(), &status)
	operator := status.MissionControlRunbook.CurrentLoopOperator
	job := recordCurrentLoopExternalSessionAttempt(t, operator, "external-turn-harness", "member-session-stale-turn", "mission-commander", "2026-08-05T01:10:00Z")
	operator.ExternalSessionJob = job
	job = acceptCurrentLoopExternalSessionLaunch(t, operator, "dispatcher", "external-turn-harness", "member-session-stale-turn", "2026-08-05T01:10:01Z")
	jobRoot := filepath.Join(caseRoot, filepath.Dir(filepath.FromSlash(job.SubmissionPath)))
	if err := os.MkdirAll(filepath.Join(jobRoot, "outputs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobRoot, "outputs", "result.txt"), []byte("stale turn result\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	submission := map[string]any{
		"schemaVersion": 1, "kind": "current-loop-external-session-submission", "jobId": job.JobID, "jobSha256": job.JobSHA256,
		"outcome": "returned", "actor": "external-turn-harness", "observedAt": "2026-08-05T01:10:30Z", "summary": "stale turn guard", "noAuthorityOrConfirmed": true, "noHeavyTool": true,
	}
	bindCurrentLoopExternalSubmissionAttempt(t, job, submission)
	submissionData, _ := json.MarshalIndent(submission, "", "  ")
	if err := os.WriteFile(filepath.Join(caseRoot, filepath.FromSlash(job.SubmissionPath)), append(submissionData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	statusOut.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	decodeJSONStrict(t, statusOut.Bytes(), &status)
	statusOut.Reset()
	if err := Run(rekitCommandCLIArgs(t, status.MissionControlRunbook.CurrentLoopOperator.SelectedDriverRequest.Command), &statusOut); err != nil {
		t.Fatal(err)
	}
	var turn currentLoopExternalSessionTurnPlan
	decodeJSONStrict(t, statusOut.Bytes(), &turn)
	args := rekitCommandCLIArgs(t, turn.ApplyCommand)
	for index := range args {
		if strings.EqualFold(args[index], "-ExpectedExternalSessionTurnPlanSha256") && index+1 < len(args) {
			args[index+1] = strings.Repeat("f", 64)
		}
	}
	before := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	if err := Run(args, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "turn plan sha256 mismatch") {
		t.Fatalf("stale turn hash error=%v", err)
	}
	assertSnapshotEqual(t, before, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))
	assertFileNotExists(t, filepath.Join(caseRoot, filepath.FromSlash(job.PublicationPath)))
	assertFileNotExists(t, filepath.Join(caseRoot, filepath.FromSlash(job.ObservationPath)))
}

func TestRunCurrentLoopMemberExecutionCheckpoint(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	board.Lanes[0].CurrentExecutor = "loop-member"
	board.Lanes[0].ExecutorGeneration = 1
	board.Lanes[0].UpdatedAt = "2026-08-03T03:00:00Z"
	boardData, _ := json.MarshalIndent(board, "", "  ")
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), append(boardData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	setCurrentLoopLaneOwner(t, caseRoot, "main", "loop-member", 1, "2026-08-03T03:00:00Z")
	preview := runCurrentLoopPreview(t, caseRoot, 5)
	if preview.InitialCurrentStep == nil || preview.InitialCurrentStep.MemberExecution == nil || preview.ExpectedCurrentLoopPlanSHA256 == "" {
		t.Fatalf("member loop did not bind external handoff: %+v", preview)
	}
	memberPlan := preview.InitialCurrentStep.MemberExecution
	applied := runCurrentLoopApplyWith(t, caseRoot, preview, "-ExpectedMemberExecutionPlanSha256", memberPlan.ExpectedPlanSHA256)
	if !applied.Applied || applied.AppliedSteps != 1 || applied.SegmentCheckpoint == nil || applied.Steps[0].CurrentStepReceipt.Outcome != "current-step-applied" || !strings.Contains(strings.Join(applied.Steps[0].CurrentStepReceipt.Boundary, "\n"), "member execution outcome: handoff-ready") {
		t.Fatalf("member loop did not checkpoint dispatch receipt: %+v", applied)
	}
	if applied.StopReason.Code != "external-member-handoff" || applied.StopReason.ExternalMemberHandoff == nil {
		t.Fatalf("unexpected member loop stop: %+v", applied.StopReason)
	}
	memberHandoff := applied.StopReason.ExternalMemberHandoff
	if memberHandoff.AttemptID != memberPlan.AttemptID || memberHandoff.Lane != memberPlan.Owner.Lane || memberHandoff.Executor != memberPlan.Owner.Executor || memberHandoff.ExecutorGeneration != memberPlan.Owner.ExecutorGeneration || memberHandoff.HandoffPath == "" || memberHandoff.ManifestPath != memberPlan.ExternalHandoff.ManifestPath || memberHandoff.OutputsRoot != memberPlan.ExternalHandoff.OutputsRoot || len(memberHandoff.ObservationContract.Alternatives) != 3 {
		t.Fatalf("member loop stop omitted durable external handoff identity: %+v", memberHandoff)
	}
	for _, alternative := range memberHandoff.ObservationContract.Alternatives {
		if !strings.Contains(alternative.PreviewCommandTemplate, "-MemberExecutionAttemptId "+memberPlan.AttemptID) || !strings.Contains(alternative.PreviewCommandTemplate, "-MemberExecutionOutcome ") {
			t.Fatalf("member observation alternative is not attempt bound: %+v", alternative)
		}
	}
	if !applied.SegmentCheckpoint.Ready || applied.SegmentCheckpoint.State != "ready" || applied.SegmentCheckpoint.RemainingMaxSteps != 4 || applied.SegmentCheckpoint.Continuation == nil || applied.SegmentCheckpoint.Continuation.ExternalMemberHandoff == nil || len(applied.SegmentCheckpoint.Continuation.ObservationContract.Alternatives) != 3 {
		t.Fatalf("member loop checkpoint is not resumable with durable observation handoff: %+v", applied.SegmentCheckpoint)
	}
	var statusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	var durableStatus statusInventory
	if err := json.Unmarshal(statusOut.Bytes(), &durableStatus); err != nil {
		t.Fatal(err)
	}
	operator := durableStatus.MissionControlRunbook.CurrentLoopOperator
	if operator == nil || operator.ExternalMemberHandoff == nil || operator.SelectedDriverRequest == nil || operator.SelectedDriverRequest.Command == "" {
		t.Fatalf("status omitted durable member current-loop operator handoff: %+v", operator)
	}
	if operator.ExternalMemberHandoff.HandoffSHA256 == "" {
		t.Fatalf("status omitted the commit-authenticated member handoff sha256: %+v", operator.ExternalMemberHandoff)
	}
	job := operator.ExternalSessionJob
	if job == nil {
		t.Fatalf("status omitted typed external member job: operator=%+v", operator)
	}
	if job.State != "awaiting-submission" || job.SessionKind != "member" || job.MemberAttemptID != memberPlan.AttemptID || job.MemberOwner == nil || job.MemberOwner.Executor != "loop-member" || job.MemberOwner.ExecutorGeneration != 1 || job.JobSHA256 == "" || job.SubmissionPath == "" || job.SubmissionOutputs == "" || !job.SubmissionLast {
		t.Fatalf("status omitted typed external member job: %+v", job)
	}
	if harness := job.HarnessPackage; harness == nil || harness.State != "attempt-review-required" || harness.AttemptReviewRequest == nil || harness.AttemptReviewRequest.Command != job.AttemptRequest.Command || harness.Launch != nil || harness.Return != nil {
		t.Fatalf("fresh member job omitted attempt-review harness package: %+v", harness)
	}
	job = recordCurrentLoopExternalSessionAttemptWithPendingRecovery(t, operator, "external-harness", "member-session-relay", "mission-commander", "2026-08-03T03:00:00Z")
	operator.ExternalSessionJob = job
	job = acceptCurrentLoopExternalSessionLaunch(t, operator, "dispatcher", "external-harness", "member-session-relay", "2026-08-03T03:00:01Z")
	expectedTaskContextData, err := os.ReadFile(filepath.Join(caseRoot, filepath.FromSlash(operator.ExternalMemberHandoff.TaskContextPath)))
	if err != nil {
		t.Fatal(err)
	}
	if harness := job.HarnessPackage; harness == nil || harness.State != "running" || harness.Launch == nil || harness.Launch.Ready || harness.Launch.Tool != "Claude Code session" || harness.Launch.AgentType != "durable-member-executor" || harness.Launch.ReadOnly || harness.Launch.Input.Role != "durable-member-handoff" || harness.Launch.Input.Path != operator.ExternalMemberHandoff.TaskContextPath || harness.Launch.Input.SHA256 != operator.ExternalMemberHandoff.TaskContextSHA256 || harness.Launch.Input.SHA256 != statusSHA256Hex(expectedTaskContextData) || harness.Launch.Attempt.AttemptSHA256 != job.CurrentAttempt.AttemptSHA256 || harness.Launch.Attempt.Session != "member-session-relay" || harness.Return == nil || harness.Return.SubmissionOutputs != job.SubmissionOutputs || !harness.Return.SubmissionLast || len(harness.Return.Templates) != len(job.AllowedOutcomes) {
		t.Fatalf("running member job omitted exact task-context launch/return package: harness=%+v launch=%+v return=%+v handoff=%+v", harness, harness.Launch, harness.Return, operator.ExternalMemberHandoff)
	}
	jobRoot := filepath.Join(caseRoot, filepath.Dir(filepath.FromSlash(job.SubmissionPath)))
	if err := os.MkdirAll(filepath.Join(jobRoot, "outputs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobRoot, "outputs", "result.txt"), []byte("member relay result\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	submission := map[string]any{
		"schemaVersion": 1, "kind": "current-loop-external-session-submission", "jobId": job.JobID, "jobSha256": job.JobSHA256,
		"outcome": "returned", "actor": "external-harness", "observedAt": "2026-08-03T03:00:30Z", "summary": "member returned via relay", "noAuthorityOrConfirmed": true, "noHeavyTool": true,
	}
	bindCurrentLoopExternalSubmissionAttempt(t, job, submission)
	submissionData, _ := json.MarshalIndent(submission, "", "  ")
	if err := os.WriteFile(filepath.Join(caseRoot, filepath.FromSlash(job.SubmissionPath)), append(submissionData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	var relayStatusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &relayStatusOut); err != nil {
		t.Fatal(err)
	}
	var relayStatus statusInventory
	if err := json.Unmarshal(relayStatusOut.Bytes(), &relayStatus); err != nil {
		t.Fatal(err)
	}
	relayOperator := relayStatus.MissionControlRunbook.CurrentLoopOperator
	if relayOperator.ExternalSessionJob == nil || relayOperator.ExternalSessionJob.State != "submission-ready" || relayOperator.SelectedDriverRequest == nil || !strings.Contains(relayOperator.SelectedDriverRequest.Command, "-AdvanceExternalSessionResult") || relayOperator.ExternalSessionJob.RelayPreviewRequest == nil {
		t.Fatalf("status did not select external-result turn with relay recovery: %+v", relayOperator)
	}
	if harness := relayOperator.ExternalSessionJob.HarnessPackage; harness == nil || harness.State != "return-review-required" || harness.Launch == nil || harness.Launch.Ready || harness.Return == nil || harness.Return.ReviewRequest == nil || harness.Return.ReviewRequest.Command != relayOperator.SelectedDriverRequest.Command || harness.Return.RelayRecoveryRequest == nil || harness.Return.RelayRecoveryRequest.Command != relayOperator.ExternalSessionJob.RelayPreviewRequest.Command {
		t.Fatalf("member submission-ready package omitted reviewed return and relay recovery requests: %+v", harness)
	}
	var relayPreview externalsession.Plan
	relayStatusOut.Reset()
	if err := Run(rekitCommandCLIArgs(t, relayOperator.ExternalSessionJob.RelayPreviewRequest.Command), &relayStatusOut); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(relayStatusOut.Bytes(), &relayPreview); err != nil || relayPreview.ApplyCommand == "" || relayPreview.ExpectedPlanSHA256 == "" {
		t.Fatalf("relay preview=%+v err=%v output=%s", relayPreview, err, relayStatusOut.String())
	}
	var relayApplied currentLoopExternalSessionRelayResult
	relayStatusOut.Reset()
	if err := Run(rekitCommandCLIArgs(t, relayPreview.ApplyCommand), &relayStatusOut); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(relayStatusOut.Bytes(), &relayApplied); err != nil || !relayApplied.Applied || relayApplied.RefreshedStatus == nil || relayApplied.RefreshedStatus.MissionControlRunbook.CurrentLoopOperator.ObservationInbox.State != "ready" {
		t.Fatalf("relay apply=%+v err=%v output=%s", relayApplied, err, relayStatusOut.String())
	}
	if _, err := os.Stat(filepath.Join(caseRoot, filepath.FromSlash(job.MemberManifestPath))); err != nil {
		t.Fatalf("relay did not generate member manifest: %v", err)
	}
	relayObservationPath := filepath.Join(caseRoot, filepath.FromSlash(job.ObservationPath))
	if _, err := os.Stat(relayObservationPath); err != nil {
		t.Fatalf("relay did not publish observation inbox envelope: %v", err)
	}
	if err := os.Remove(relayObservationPath); err != nil {
		t.Fatal(err)
	}
	for _, alternative := range operator.ExternalMemberHandoff.ObservationContract.Alternatives {
		if !strings.Contains(alternative.PreviewCommandTemplate, "-ResumeCurrentLoop") || !strings.Contains(alternative.PreviewCommandTemplate, applied.SegmentCheckpoint.ArtifactSHA256) || !strings.Contains(alternative.PreviewCommandTemplate, memberPlan.AttemptID) {
			t.Fatalf("status member observation alternative is not checkpoint/attempt bound: %+v", alternative)
		}
		if !strings.Contains(alternative.ObservationEnvelopeTemplate, `"checkpointSha256": "`+applied.SegmentCheckpoint.ArtifactSHA256+`"`) || !strings.Contains(alternative.ObservationEnvelopeTemplate, `"memberAttemptId": "`+memberPlan.AttemptID+`"`) || !strings.Contains(alternative.ObservationPathCommand, "-CurrentLoopObservationPath") || !strings.Contains(alternative.ObservationPathCommand, applied.SegmentCheckpoint.ArtifactSHA256) {
			t.Fatalf("status member observation alternative omitted envelope intake: %+v", alternative)
		}
	}
	var statusText bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "text"}, &statusText); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"current-loop member handoff：state=handoff-ready attempt=" + memberPlan.AttemptID, "owner=loop-member/1", "current-loop member observation：kind=member-session-accepted", "observationPathCommand=`/rekit run-current-loop", "observationEnvelopeTemplate=`{", applied.SegmentCheckpoint.ArtifactSHA256} {
		if !strings.Contains(statusText.String(), want) {
			t.Fatalf("text status omitted member handoff detail %q:\n%s", want, statusText.String())
		}
	}
	if err := runCurrentLoopError(caseRoot, []string{"-ResumeCurrentLoop", "-ExpectedCurrentLoopCheckpointSha256", applied.SegmentCheckpoint.ArtifactSHA256, "-MemberExecutionAttemptId", "g000001-a999999-deadbeefdeadbeef", "-MemberExecutionOutcome", "accepted", "-MemberExecutionObservedAt", "2026-08-03T03:00:30Z", "-Actor", "harness", "-WhatIf"}); err == nil || !strings.Contains(err.Error(), "does not match checkpoint attempt") {
		t.Fatalf("external member checkpoint accepted a different attempt observation: %v", err)
	}
	if err := runCurrentLoopError(caseRoot, []string{"-ResumeCurrentLoop", "-ExpectedCurrentLoopCheckpointSha256", applied.SegmentCheckpoint.ArtifactSHA256, "-WhatIf"}); err == nil || !strings.Contains(err.Error(), "member observation") {
		t.Fatalf("external member checkpoint resumed without a member observation: %v", err)
	}
	observationDir := filepath.Join(caseRoot, ".rekit", "external-session-observations", "inbox")
	if err := os.MkdirAll(observationDir, 0o700); err != nil {
		t.Fatal(err)
	}
	observationPath := filepath.Join(observationDir, "member-accepted.json")
	observationBytes, err := json.Marshal(currentLoopObservationEnvelope{
		SchemaVersion: 1, Kind: "current-loop-external-session-observation", CheckpointSHA256: applied.SegmentCheckpoint.ArtifactSHA256,
		ObservationKind: "member-session-accepted", Actor: "harness", ObservedAt: "2026-08-03T03:01:00Z", MemberAttemptID: memberPlan.AttemptID,
		NoAuthorityOrConfirmed: true, NoHeavyTool: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(observationPath, append(observationBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	var inboxStatusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &inboxStatusOut); err != nil {
		t.Fatal(err)
	}
	var inboxStatus statusInventory
	if err := json.Unmarshal(inboxStatusOut.Bytes(), &inboxStatus); err != nil {
		t.Fatal(err)
	}
	inboxOperator := inboxStatus.MissionControlRunbook.CurrentLoopOperator
	if inboxOperator == nil || inboxOperator.ObservationInbox == nil || inboxOperator.ObservationInbox.State != "ready" || inboxOperator.SelectedDriverRequest == nil || inboxOperator.SelectedDriverRequest.Command != inboxOperator.ObservationInbox.SelectedDriverRequest.Command || !strings.Contains(inboxOperator.SelectedDriverRequest.Command, observationPath) {
		t.Fatalf("status did not select the unique canonical inbox observation: %+v", inboxOperator)
	}
	for _, alternative := range inboxOperator.ExternalMemberHandoff.ObservationContract.Alternatives {
		if strings.Contains(alternative.PreviewCommandTemplate, "-CurrentLoopObservationPath") {
			t.Fatalf("inbox request leaked into legacy member observation template: %+v", alternative)
		}
		fields, err := splitDriverCommand(alternative.PreviewCommandTemplate)
		if err != nil || len(fields) < 2 || fields[0] != "/rekit" {
			t.Fatalf("legacy member observation template is not consumable: %q: %v", alternative.PreviewCommandTemplate, err)
		}
	}
	accepted := runCurrentLoopResult(t, rekitCommandCLIArgs(t, inboxOperator.SelectedDriverRequest.Command))
	if accepted.MaxSteps != 4 || accepted.ResumeSource == nil || accepted.InitialCurrentStep == nil || accepted.InitialCurrentStep.MemberExecution == nil || accepted.InitialCurrentStep.MemberExecution.Outcome != "accepted" {
		t.Fatalf("member observation did not resume exact checkpoint budget: %+v", accepted)
	}
	observationSHA256 := sha256Text(append(observationBytes, '\n'))
	if accepted.ObservationPath != observationPath || accepted.ObservationSHA256 != observationSHA256 {
		t.Fatalf("member observation preview omitted exact envelope identity: %+v", accepted)
	}
	for _, required := range []string{"-CurrentLoopObservationPath \"" + observationPath + "\"", "-ExpectedCurrentLoopObservationSha256 \"" + observationSHA256 + "\"", "-ExpectedMemberExecutionPlanSha256 \"" + accepted.InitialCurrentStep.MemberExecution.ExpectedPlanSHA256 + "\""} {
		if !strings.Contains(accepted.ApplyCommand, required) {
			t.Fatalf("member resume apply command omitted %q: %s", required, accepted.ApplyCommand)
		}
	}
	for _, forbidden := range []string{"-MemberExecutionAttemptId", "-MemberExecutionOutcome", "-MemberExecutionObservedAt", "-Actor"} {
		if strings.Contains(accepted.ApplyCommand, forbidden) {
			t.Fatalf("member envelope apply command leaked reconstructed flag %s: %s", forbidden, accepted.ApplyCommand)
		}
	}
	driftedBytes := append(append([]byte{}, observationBytes...), []byte(" \n")...)
	if err := os.WriteFile(observationPath, driftedBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Run(rekitCommandCLIArgs(t, accepted.ApplyCommand), &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "observation sha256 mismatch") {
		t.Fatalf("member observation byte drift was not rejected before claim: %v", err)
	}
	if err := os.WriteFile(observationPath, append(observationBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	secondPath := filepath.Join(observationDir, "member-accepted-duplicate.json")
	if err := os.WriteFile(secondPath, append(observationBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Run(rekitCommandCLIArgs(t, accepted.ApplyCommand), &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "no longer has exactly one") {
		t.Fatalf("member inbox ambiguity introduced after preview was not rejected before claim: %v", err)
	}
	if err := os.Remove(secondPath); err != nil {
		t.Fatal(err)
	}
	currentLoopBeforeApplyStepHook = func(step int) error {
		if step != 1 {
			return nil
		}
		return os.WriteFile(observationPath, append(append([]byte{}, observationBytes...), []byte(" \n")...), 0o600)
	}
	t.Cleanup(func() { currentLoopBeforeApplyStepHook = nil })
	acceptedApplied := runCurrentLoopResult(t, rekitCommandCLIArgs(t, accepted.ApplyCommand))
	currentLoopBeforeApplyStepHook = nil
	if !acceptedApplied.Applied || acceptedApplied.InitialCurrentStep == nil || acceptedApplied.InitialCurrentStep.MemberExecution == nil || acceptedApplied.InitialCurrentStep.MemberExecution.Inspection.State != "accepted" {
		t.Fatalf("member accepted resume apply command did not persist observation: %+v", acceptedApplied)
	}
	if acceptedApplied.SegmentCheckpoint == nil || acceptedApplied.SegmentCheckpoint.ArtifactSHA256 == "" {
		t.Fatalf("member accepted resume did not publish a checkpoint: checkpoint=%+v plan=%+v", acceptedApplied.SegmentCheckpoint, acceptedApplied)
	}
	if acceptedApplied.ObservationReceipt == nil || acceptedApplied.ObservationReceipt.State != "processed" || acceptedApplied.ObservationReceipt.SourceCheckpointSHA256 != applied.SegmentCheckpoint.ArtifactSHA256 || acceptedApplied.ObservationReceipt.SuccessorCheckpointSHA256 != acceptedApplied.SegmentCheckpoint.ArtifactSHA256 || acceptedApplied.ObservationReceipt.ObservationSHA256 != observationSHA256 || acceptedApplied.ObservationReceipt.ObservationKind != "member-session-accepted" {
		t.Fatalf("member accepted resume omitted one-shot processed receipt: %+v", acceptedApplied.ObservationReceipt)
	}
	if acceptedApplied.ContinuationRequest == nil || acceptedApplied.ContinuationRequest.ExternalMemberHandoff == nil || acceptedApplied.ContinuationRequest.ExternalMemberHandoff.State != "accepted" || len(acceptedApplied.ContinuationRequest.ObservationContract.Alternatives) != 2 {
		t.Fatalf("accepted member continuation did not narrow to returned/failed: %+v", acceptedApplied.ContinuationRequest)
	}
	for _, alternative := range acceptedApplied.ContinuationRequest.ObservationContract.Alternatives {
		if alternative.Kind == "member-session-accepted" {
			t.Fatalf("accepted member continuation still permits duplicate accepted: %+v", acceptedApplied.ContinuationRequest)
		}
	}
	if err := os.Remove(observationPath); err != nil {
		t.Fatal(err)
	}
	var receiptStatusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &receiptStatusOut); err != nil {
		t.Fatal(err)
	}
	var receiptStatus statusInventory
	if err := json.Unmarshal(receiptStatusOut.Bytes(), &receiptStatus); err != nil {
		t.Fatal(err)
	}
	receiptInbox := receiptStatus.MissionControlRunbook.CurrentLoopOperator.ObservationInbox
	if receiptInbox == nil || receiptInbox.LatestReceipt == nil || receiptInbox.LatestReceipt.SourceCheckpointSHA256 != applied.SegmentCheckpoint.ArtifactSHA256 || receiptInbox.LatestReceipt.SuccessorCheckpointSHA256 != acceptedApplied.SegmentCheckpoint.ArtifactSHA256 || receiptInbox.LatestReceipt.ObservationSHA256 != observationSHA256 || receiptInbox.LatestReceipt.ObservationKind != "member-session-accepted" || receiptInbox.LatestReceipt.Actor != "harness" {
		t.Fatalf("fresh status did not recover processed observation receipt: %+v", receiptInbox)
	}
	if receiptStatus.MissionControlRunbook.CurrentLoopOperator.ObservationReceipt == nil || receiptStatus.MissionControlRunbook.CurrentLoopOperator.ObservationReceipt.ObservationKind != "member-session-accepted" {
		t.Fatalf("fresh status omitted top-level processed observation receipt: %+v", receiptStatus.MissionControlRunbook.CurrentLoopOperator)
	}
	inspection := acceptedApplied.InitialCurrentStep.MemberExecution.Inspection
	output := []byte("member-result\n")
	if err := os.MkdirAll(inspection.OutputsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inspection.OutputsRoot, "result.txt"), output, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := memberexecution.ResultManifest{SchemaVersion: 1, Kind: memberexecution.KindManifest, AttemptID: memberPlan.AttemptID, Owner: memberPlan.Owner, Summary: "returned through resume", Outputs: []memberexecution.Output{{Path: "result.txt", SHA256: sha256Text(output), Bytes: int64(len(output))}}, NoAuthority: true, NoConfirmed: true, NoHeavyTool: true}
	manifestData, _ := memberexecution.MarshalResultManifest(manifest)
	if err := os.WriteFile(inspection.ManifestPath, manifestData, 0o600); err != nil {
		t.Fatal(err)
	}
	returnedSource := acceptedApplied.SegmentCheckpoint
	returnedObservationPath := filepath.Join(observationDir, "member-returned.json")
	returnedObservationBytes, err := json.Marshal(currentLoopObservationEnvelope{
		SchemaVersion: 1, Kind: "current-loop-external-session-observation", CheckpointSHA256: returnedSource.ArtifactSHA256,
		ObservationKind: "member-session-returned", Actor: "harness", ObservedAt: "2026-08-03T03:02:00Z", MemberAttemptID: memberPlan.AttemptID, Reason: "bounded result complete",
		NoAuthorityOrConfirmed: true, NoHeavyTool: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(returnedObservationPath, append(returnedObservationBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	var returnedInboxStatusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &returnedInboxStatusOut); err != nil {
		t.Fatal(err)
	}
	var returnedInboxStatus statusInventory
	if err := json.Unmarshal(returnedInboxStatusOut.Bytes(), &returnedInboxStatus); err != nil {
		t.Fatal(err)
	}
	returnedRequest := returnedInboxStatus.MissionControlRunbook.CurrentLoopOperator.SelectedDriverRequest
	if returnedRequest == nil || !strings.Contains(returnedRequest.Command, returnedObservationPath) {
		t.Fatalf("returned member observation was not selected from canonical inbox: %+v", returnedInboxStatus.MissionControlRunbook.CurrentLoopOperator)
	}
	returned := runCurrentLoopResult(t, rekitCommandCLIArgs(t, returnedRequest.Command))
	returnedApplied := runCurrentLoopResult(t, rekitCommandCLIArgs(t, returned.ApplyCommand))
	if !returnedApplied.Applied || returnedApplied.InitialCurrentStep == nil || returnedApplied.InitialCurrentStep.MemberExecution == nil || returnedApplied.InitialCurrentStep.MemberExecution.Inspection.State != "intake-ready" {
		t.Fatalf("member returned resume apply command did not persist intake: %+v", returnedApplied)
	}
	if returnedApplied.ObservationReceipt == nil || returnedApplied.ObservationReceipt.ObservationKind != "member-session-returned" || returnedApplied.ObservationReceipt.Actor != "harness" || returnedApplied.ObservationReceipt.ObservationPath != returnedObservationPath || returnedApplied.ObservationReceipt.SourceCheckpointSHA256 != returnedSource.ArtifactSHA256 {
		t.Fatalf("member returned resume omitted processed observation receipt: %+v", returnedApplied.ObservationReceipt)
	}
	if returnedApplied.AppliedSteps != 1 || returnedApplied.StopReason.Code != "member-intake-ready" || returnedApplied.SegmentCheckpoint == nil || !returnedApplied.SegmentCheckpoint.Ready || returnedApplied.SegmentCheckpoint.RemainingMaxSteps != returned.MaxSteps-returnedApplied.AppliedSteps || returnedApplied.SegmentCheckpoint.Continuation == nil || returnedApplied.SegmentCheckpoint.Continuation.ExternalMemberHandoff != nil || returnedApplied.SegmentCheckpoint.Continuation.ObservationContract != nil {
		t.Fatalf("member returned result did not publish a clean remaining-budget continuation: %+v", returnedApplied)
	}
	if returnedApplied.StopReason.ExternalMemberHandoff != nil || returnedApplied.ContinuationRequest != nil && returnedApplied.ContinuationRequest.ExternalMemberHandoff != nil {
		t.Fatalf("intake-ready member result retained a stale external handoff: stop=%+v continuation=%+v", returnedApplied.StopReason, returnedApplied.ContinuationRequest)
	}
	var returnedStatusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-Format", "json"}, &returnedStatusOut); err != nil {
		t.Fatal(err)
	}
	var returnedStatus statusInventory
	if err := json.Unmarshal(returnedStatusOut.Bytes(), &returnedStatus); err != nil {
		t.Fatal(err)
	}
	returnedOperator := returnedStatus.MissionControlRunbook.CurrentLoopOperator
	if returnedOperator == nil || returnedOperator.ExternalMemberHandoff != nil {
		t.Fatalf("intake-ready status retained a stale member handoff: %+v", returnedOperator)
	}
	if returnedOperator.ObservationReceipt == nil || returnedOperator.ObservationReceipt.ObservationKind != "member-session-returned" || returnedOperator.ObservationReceipt.Actor != "harness" || returnedOperator.ObservationReceipt.SourceCheckpointSHA256 != returnedSource.ArtifactSHA256 || returnedOperator.ObservationReceipt.SuccessorCheckpointSHA256 != returnedApplied.SegmentCheckpoint.ArtifactSHA256 {
		t.Fatalf("intake-ready status omitted terminal successor observation receipt: %+v", returnedOperator)
	}
	returnedTakeover := returnedStatus.MissionControlRunbook.ReplacementExecutorTakeover
	if returnedTakeover == nil || returnedTakeover.CurrentLoopOperator == nil || returnedTakeover.CurrentLoopOperator.ObservationReceipt == nil || returnedTakeover.CurrentLoopOperator.ObservationReceipt.ObservationKind != "member-session-returned" {
		t.Fatalf("replacement takeover omitted terminal successor observation receipt: %+v", returnedTakeover)
	}
}

func runCurrentLoopError(caseRoot string, inputs []string) error {
	args := []string{
		"-Command", "run-current-loop",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Lane", "main",
	}
	args = append(args, inputs...)
	args = append(args, "-Format", "json")
	return Run(args, &bytes.Buffer{})
}

func runCurrentLoopApply(t *testing.T, caseRoot string, preview currentLoopTestPlan) currentLoopTestPlan {
	t.Helper()
	return runCurrentLoopApplyWith(t, caseRoot, preview)
}

func runCurrentLoopApplyWith(t *testing.T, caseRoot string, preview currentLoopTestPlan, inputs ...string) currentLoopTestPlan {
	t.Helper()
	args := []string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-MaxSteps", stringInt(preview.MaxSteps)}
	args = append(args, inputs...)
	args = append(args, "-ExpectedCurrentLoopPlanSha256", preview.ExpectedCurrentLoopPlanSHA256, "-Apply", "-Format", "json")
	return runCurrentLoopResult(t, args)
}

func runCurrentLoopResult(t *testing.T, args []string) currentLoopTestPlan {
	t.Helper()
	var out bytes.Buffer
	if err := Run(args, &out); err != nil {
		t.Fatal(err)
	}
	var result currentLoopTestPlan
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("current-loop JSON did not decode: %v\n%s", err, out.String())
	}
	return result
}

func stringInt(value int) string {
	return strconv.Itoa(value)
}
