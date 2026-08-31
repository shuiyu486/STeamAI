package sessionhost

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewersession"
)

func TestClaudeLaunchCapabilityPolicySeparatesMemberAndReadOnlyReview(t *testing.T) {
	member := mission.CurrentLoopExternalSessionHarnessPackage{
		SessionKind: "member",
		Launch: &mission.CurrentLoopExternalSessionHarnessLaunch{
			Capability: transportCapabilityForTest(),
		},
	}
	if err := validateClaudeCapabilityPolicy(member); err != nil {
		t.Fatal(err)
	}
	member.Launch.Capability = readOnlyCapabilityForTest()
	if err := validateClaudeCapabilityPolicy(member); err == nil || !strings.Contains(err.Error(), "policy class mismatch") {
		t.Fatalf("member launch accepted read-only capability: %v", err)
	}
	member.Launch.Capability = transportCapabilityForTest()
	member.Launch.ReadOnly = true
	if err := validateClaudeCapabilityPolicy(member); err == nil || !strings.Contains(err.Error(), "cannot be projected as read-only") {
		t.Fatalf("member launch accepted read-only projection: %v", err)
	}

	review := mission.CurrentLoopExternalSessionHarnessPackage{
		SessionKind: "mission-commander-evidence-review",
		Launch: &mission.CurrentLoopExternalSessionHarnessLaunch{
			ReadOnly:   true,
			Capability: readOnlyCapabilityForTest(),
		},
	}
	if err := validateClaudeCapabilityPolicy(review); err != nil {
		t.Fatal(err)
	}
	review.Launch.Capability = transportCapabilityForTest()
	if err := validateClaudeCapabilityPolicy(review); err == nil || !strings.Contains(err.Error(), "policy class mismatch") {
		t.Fatalf("evidence review accepted transport capability: %v", err)
	}
	review.Launch.Capability = readOnlyCapabilityForTest()
	review.Launch.ReadOnly = false
	if err := validateClaudeCapabilityPolicy(review); err == nil || !strings.Contains(err.Error(), "must be projected as read-only") {
		t.Fatalf("evidence review accepted writable projection: %v", err)
	}
}

func TestPublishValidatedMemberOutputsCopiesBoundedTransportBytes(t *testing.T) {
	caseRoot := t.TempDir()
	outputs := []validatedMemberOutput{
		{path: ".rekit/host/outputs/analysis/result.txt", data: []byte("opaque transport bytes\n")},
		{path: ".rekit/host/outputs/index.txt", data: []byte("opaque-index\n")},
	}
	if err := publishValidatedMemberOutputs(caseRoot, outputs); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		"analysis/result.txt": "opaque transport bytes\n",
		"index.txt":           "opaque-index\n",
	} {
		got, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "host", "outputs", filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Fatalf("output %s = %q, want exact bounded bytes %q", path, string(got), want)
		}
	}
}

func TestPublishMemberOutputsRejectsUnsafeAndPartialResults(t *testing.T) {
	tests := []memberResponse{
		{},
		{Outputs: []memberOutput{{Path: "../escape.txt", Content: "x"}}},
		{Outputs: []memberOutput{{Path: "C:/escape.txt", Content: "x"}}},
		{Outputs: []memberOutput{{Path: "result.txt", Content: ""}}},
		{Outputs: []memberOutput{{Path: "result.txt", Content: "x"}}, ReviewerItemsPath: "missing.txt"},
	}
	for index, response := range tests {
		if err := publishMemberOutputs(t.TempDir(), ".rekit/host/outputs", response); err == nil {
			t.Fatalf("unsafe response %d was accepted: %+v", index, response)
		}
	}
}

func TestReviewerDispatchReadyRequiresExactBootstrapState(t *testing.T) {
	plan := currentStepPlan{ReviewerStep: &reviewerStep{ExternalHandoff: &reviewerExternalHandoff{
		State: "ready-for-reviewer-dispatch", RunLoopStepID: "spawn-reviewer",
	}}}
	if !reviewerDispatchReady(plan) {
		t.Fatal("exact reviewer bootstrap state was not recognized")
	}
	plan.ReviewerStep.ExternalHandoff.RunLoopStepID = "save-result-input"
	if reviewerDispatchReady(plan) {
		t.Fatal("non-bootstrap reviewer handoff was accepted")
	}
}

func TestValidateClaudeMemberLaunchInputKeepsAttemptNamespacesDistinct(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "case")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	onboarding, err := applyDailyOnboarding(
		caseRoot,
		"inspect a bounded target",
		"member-launch-input-test",
		&bootstrap,
	)
	if err != nil {
		t.Fatal(err)
	}
	pack := onboarding.Identity.Pack
	bootstrap.Lane = onboarding.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	owner, err := laneowner.Read(caseRoot, bootstrap.Lane)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := memberexecution.WorkspaceInventoryTaskBinding(caseRoot, ".")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := memberexecution.WriteTaskBindingForOwner(
		caseRoot, bootstrap.Lane, owner.CurrentExecutor, owner.ExecutorGeneration, binding,
	); err != nil {
		t.Fatal(err)
	}
	plan, err := memberexecution.PreviewDispatch(
		memberexecution.DispatchOptions{
			CaseRoot:      caseRoot,
			Pack:          pack,
			Lane:          bootstrap.Lane,
			RequestSHA256: strings.Repeat("a", 64),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	published, err := memberexecution.Apply(
		plan,
		plan.ExpectedPlanSHA256,
	)
	if err != nil {
		t.Fatal(err)
	}
	inspection := published.Plan.Inspection
	input, err := os.ReadFile(inspection.TaskContextPath)
	if err != nil {
		t.Fatal(err)
	}
	launch := mission.CurrentLoopExternalSessionHarnessLaunch{
		Input: mission.CurrentLoopExternalSessionHarnessInput{
			Path:   inspection.Handoff.TaskContextPath,
			SHA256: bytesSHA256(input),
		},
		Attempt: mission.CurrentLoopExternalSessionAttempt{
			AttemptID: "member-external-job-g000001",
		},
	}
	validated, err := validateClaudeMemberLaunchInput(caseRoot, launch)
	if err != nil {
		t.Fatal(err)
	}
	if validated.AttemptID != inspection.AttemptID ||
		validated.AttemptID == launch.Attempt.AttemptID {
		t.Fatalf(
			"member/external attempt namespaces collapsed: member=%q external=%q",
			validated.AttemptID,
			launch.Attempt.AttemptID,
		)
	}

	launch.Attempt.SubmissionOutputs = ".steamai/external-session-attempt-inputs/member-job/000001/outputs"
	canonicalOutput, err := filepath.Rel(
		caseRoot,
		filepath.Join(inspection.AttemptRoot, "evidence", "outputs", "member-output", "result.json"),
	)
	if err != nil {
		t.Fatal(err)
	}
	canonicalOutput = filepath.ToSlash(canonicalOutput)
	response := memberResponse{
		Outcome:           "returned",
		Summary:           "bounded member result",
		Outputs:           []memberOutput{{Path: canonicalOutput, Content: "bounded result\n"}},
		ReviewerItemsPath: canonicalOutput,
	}
	normalized, err := normalizeMemberResponseOutputPaths(caseRoot, launch, response)
	if err != nil {
		t.Fatal(err)
	}
	if got := normalized.Outputs[0].Path; got != "member-output/result.json" {
		t.Fatalf("normalized output path=%q", got)
	}
	if normalized.ReviewerItemsPath != "member-output/result.json" {
		t.Fatalf("normalized reviewerItemsPath=%q", normalized.ReviewerItemsPath)
	}
	validatedOutputs, err := validateMemberOutputs(launch.Attempt.SubmissionOutputs, normalized)
	if err != nil {
		t.Fatal(err)
	}
	wantPublished := filepath.ToSlash(filepath.Join(
		filepath.FromSlash(launch.Attempt.SubmissionOutputs),
		"member-output",
		"result.json",
	))
	if len(validatedOutputs) != 1 || validatedOutputs[0].path != wantPublished {
		t.Fatalf("validated publication paths=%+v want=%q", validatedOutputs, wantPublished)
	}
	response.Outputs[0].Path = ".steamai/lanes/unrelated/result.json"
	response.ReviewerItemsPath = response.Outputs[0].Path
	if _, err := normalizeMemberResponseOutputPaths(caseRoot, launch, response); err == nil ||
		!strings.Contains(err.Error(), "submission output root") {
		t.Fatalf("unrelated state-root path error=%v", err)
	}

	launch.Ready = true
	launch.Attempt.Session = "member-session"
	pkg := mission.CurrentLoopExternalSessionHarnessPackage{
		CaseRoot:    caseRoot,
		SessionKind: "member",
		Launch:      &launch,
		Return: &mission.CurrentLoopExternalSessionReturnContract{
			SubmissionOutputs: launch.Attempt.SubmissionOutputs,
		},
	}
	prompt, _, err := claudeRequest(caseRoot, pkg, launch.Attempt.Session)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, launch.Attempt.SubmissionOutputs) ||
		!strings.Contains(prompt, "never return a case-relative .steamai or .rekit path") {
		t.Fatalf("member prompt omitted exact output path contract: %s", prompt)
	}
}

func TestMemberTaskBindingPolicyBoundsTypedInputs(t *testing.T) {
	artifact := memberexecution.TaskContext{Binding: &memberexecution.TaskBinding{
		Kind: memberexecution.TaskBindingArtifactAnalysis,
		Values: map[string]string{
			"artifact-path":   "inputs/sample.bin",
			"artifact-sha256": strings.Repeat("a", 64),
			"artifact-bytes":  "16",
		},
	}}
	artifactInput, err := json.Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	artifactPolicy := memberTaskBindingPolicy(artifactInput)
	if !strings.Contains(artifactPolicy, "exact case-relative artifact-path") ||
		!strings.Contains(artifactPolicy, "do not substitute another file") {
		t.Fatalf("artifact-analysis policy=%q", artifactPolicy)
	}

	inventory := memberexecution.TaskContext{Binding: &memberexecution.TaskBinding{
		Kind:   memberexecution.TaskBindingWorkspaceInventory,
		Values: map[string]string{"workspace-scope": "inputs"},
	}}
	inventoryInput, err := json.Marshal(inventory)
	if err != nil {
		t.Fatal(err)
	}
	inventoryPolicy := memberTaskBindingPolicy(inventoryInput)
	if !strings.Contains(inventoryPolicy, "empty directory is a valid inventory result") ||
		!strings.Contains(inventoryPolicy, "Do not choose a candidate artifact") {
		t.Fatalf("workspace-inventory policy=%q", inventoryPolicy)
	}
}

func TestClaudeAdditionalReadDirsAreReviewerOnlyAndAttachedKitBound(t *testing.T) {
	repo := sessionhostTestRepoRoot(t)
	caseRoot := provisionSessionhostAttachedCase(t, repo, "_template")
	reviewer := mission.CurrentLoopExternalSessionHarnessPackage{CaseRoot: caseRoot, SessionKind: "reviewer"}
	dirs, err := claudeAdditionalReadDirs(Options{Target: caseRoot, Pack: "_template"}, reviewer)
	if err != nil || len(dirs) != 1 || !casePathEqual(dirs[0], filepath.Join(caseRoot, projectstate.CurrentDir)) {
		t.Fatalf("reviewer additional dirs=%v err=%v", dirs, err)
	}
	member := reviewer
	member.SessionKind = "member"
	if dirs, err := claudeAdditionalReadDirs(Options{Target: caseRoot, Pack: "_template"}, member); err != nil || len(dirs) != 0 {
		t.Fatalf("member additional dirs=%v err=%v", dirs, err)
	}
	reviewer.CaseRoot = t.TempDir()
	if _, err := claudeAdditionalReadDirs(Options{Target: caseRoot, Pack: "_template"}, reviewer); err == nil || !strings.Contains(err.Error(), "case root changed") {
		t.Fatalf("reviewer case drift error=%v", err)
	}
}

func TestClaudeSuccessRequiresReturnedSessionIdentityAndNoPermissionDenial(t *testing.T) {
	run := claudeRun{
		envelope: claudeEnvelope{Type: "result", Subtype: "success"},
		exitCode: 0, structuredOutput: json.RawMessage(`{"opaque":"non-empty"}`),
	}
	if run.success() {
		t.Fatal("Claude success accepted a missing durable session identity")
	}
	run.sessionID = "session-id"
	if !run.success() {
		t.Fatal("Claude success rejected a bound durable session identity")
	}
	run.envelope.PermissionDenials = []any{"Read denied"}
	if run.success() {
		t.Fatal("Claude success accepted a permission denial")
	}
}

func TestClaudeFailureDiagnosisMatrix(t *testing.T) {
	tests := []struct {
		name  string
		run   claudeRun
		code  string
		stage string
	}{
		{name: "auth", run: claudeRun{started: true, exitCode: 1, stderrTail: "Please log in to Claude Code"}, code: "claude-authentication-failed", stage: "provider-authentication"},
		{name: "quota", run: claudeRun{started: true, exitCode: 1, stderrTail: "Usage limit reached"}, code: "claude-quota-unavailable", stage: "provider-availability"},
		{name: "model", run: claudeRun{started: true, exitCode: 1, stderrTail: "model is not available"}, code: "claude-model-unavailable", stage: "provider-availability"},
		{name: "spawn", run: claudeRun{spawnErr: errors.New("CreateProcess failed")}, code: "claude-spawn-failed", stage: "process-spawn"},
		{name: "timeout", run: claudeRun{started: true, timedOut: true, waitErr: context.DeadlineExceeded}, code: "claude-timeout", stage: "process-wait"},
		{name: "permission", run: claudeRun{started: true, envelope: claudeEnvelope{PermissionDenials: []any{"Read denied"}}}, code: "claude-permission-denied", stage: "tool-permission"},
		{name: "nonzero", run: claudeRun{started: true, exitCode: 7, waitErr: errors.New("exit status 7")}, code: "claude-nonzero-exit", stage: "process-exit"},
		{name: "envelope", run: claudeRun{started: true, failureCode: "claude-invalid-envelope", waitErr: errors.New("decode result")}, code: "claude-invalid-envelope", stage: "envelope-validation"},
		{name: "session", run: claudeRun{started: true, failureCode: "claude-session-id-mismatch", waitErr: errors.New("session drift")}, code: "claude-session-id-mismatch", stage: "session-validation"},
		{name: "structured", run: claudeRun{started: true, failureDetail: "unknown structured output field"}, code: "claude-invalid-structured-output", stage: "structured-output-validation"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnosis := diagnosisForClaudeRun(test.run, "replacement-requested", "member", 1, 3)
			if diagnosis == nil || diagnosis.Code != test.code || diagnosis.Stage != test.stage || diagnosis.State != failureStateReplaceable || !diagnosis.Replaceable || diagnosis.NextAction == "" {
				t.Fatalf("diagnosis = %+v", diagnosis)
			}
		})
	}
}

func TestClaudeFailureDiagnosisPrefersStructuredEvidence(t *testing.T) {
	tests := []struct {
		name string
		run  claudeRun
		code string
	}{
		{name: "invalid envelope before provider prose", run: claudeRun{started: true, failureCode: "claude-invalid-envelope", waitErr: errors.New("decode result"), stdoutTail: "authentication quota model not found"}, code: "claude-invalid-envelope"},
		{name: "session mismatch before provider prose", run: claudeRun{started: true, failureCode: "claude-session-id-mismatch", waitErr: errors.New("session drift"), stderrTail: "please log in"}, code: "claude-session-id-mismatch"},
		{name: "spawn permission is process failure", run: claudeRun{spawnErr: errors.New("permission denied")}, code: "claude-spawn-failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnosis := diagnosisForClaudeRun(test.run, "replacement-requested", "member", 1, 3)
			if diagnosis == nil || diagnosis.Code != test.code {
				t.Fatalf("diagnosis = %+v", diagnosis)
			}
		})
	}
}

func TestLaunchFailedDiagnosisReportsDurableRuntimeMutation(t *testing.T) {
	run := claudeRun{spawnErr: errors.New("CreateProcess failed")}
	diagnosis := diagnosisForClaudeRun(run, "launch-failed", "member", 1, 3)
	if diagnosis == nil || !diagnosis.Recoverable || !diagnosis.MutationApplied || diagnosis.MutationBoundary != "durable-launch-failure-recorded" {
		t.Fatalf("launch-failed diagnosis = %+v", diagnosis)
	}
}

func TestClaudeAttemptLimitIsTerminalAndDoesNotRetry(t *testing.T) {
	run := claudeRun{started: true, exitCode: 1, waitErr: errors.New("exit status 1")}
	diagnosis := diagnosisForClaudeRun(run, "failed", "member", 3, 3)
	if diagnosis == nil || diagnosis.State != failureStateTerminal || !diagnosis.Terminal || diagnosis.Replaceable || diagnosis.Recoverable {
		t.Fatalf("attempt limit diagnosis = %+v", diagnosis)
	}
	if diagnosis.AttemptsUsed != 3 || diagnosis.AttemptsLimit != 3 || diagnosis.NextAction == "" {
		t.Fatalf("attempt limit evidence = %+v", diagnosis)
	}
}

func TestHostOperationFailureDiagnosisPreservesMutationTruth(t *testing.T) {
	diagnosis := diagnosisForError(errors.New("external session turn relay committed but observation intake failed"), 1, 3, 4)
	if diagnosis == nil || diagnosis.Code != "claude-intake-failed" || diagnosis.State != failureStateRecoverable || !diagnosis.MutationApplied || diagnosis.MutationBoundary != "durable-runtime-step-may-have-committed" {
		t.Fatalf("intake diagnosis = %+v", diagnosis)
	}
	publicationErr := hostError("claude-submission-failed", "submission-publication", "result-artifact-publication-may-have-committed", "refresh status", true, errors.New("write submission failed"))
	diagnosis = diagnosisForError(publicationErr, 1, 3, 2)
	if diagnosis == nil || diagnosis.Code != "claude-submission-failed" || !diagnosis.Recoverable || diagnosis.Replaceable || !diagnosis.MutationApplied || diagnosis.MutationBoundary != "result-artifact-publication-may-have-committed" || diagnosis.NextAction != "refresh status" {
		t.Fatalf("submission diagnosis = %+v", diagnosis)
	}
}

func TestLimitedBufferFailsClosedWithoutTruncationSuccess(t *testing.T) {
	buffer := limitedBuffer{limit: 8}
	if _, err := buffer.Write([]byte("1234567890")); err != nil {
		t.Fatal(err)
	}
	if !buffer.exceeded || string(buffer.Bytes()) != "12345678" {
		t.Fatalf("limited buffer = exceeded:%t bytes:%q", buffer.exceeded, string(buffer.Bytes()))
	}
	if _, err := buffer.Write([]byte("ignored")); err != nil {
		t.Fatal(err)
	}
	if string(buffer.Bytes()) != "12345678" {
		t.Fatalf("limited buffer changed after overflow: %q", string(buffer.Bytes()))
	}
}

func TestClaudeReviewerRequestBindsExactDispatchIdentity(t *testing.T) {
	caseRoot, _, memberPackage, _, _ := projectExecutionLaunchFixture(t)
	pack := memberPackage.Pack
	reviewRoot := filepath.Join(caseRoot, projectstate.CurrentDir, "reviews", "review-exact")
	inputRel := filepath.ToSlash(filepath.Join(projectstate.CurrentDir, "reviews", "review-exact", "prompts", "shard-01.md"))
	input := []byte("review bounded evidence\n")
	inputPath := filepath.Join(caseRoot, filepath.FromSlash(inputRel))
	if err := os.MkdirAll(filepath.Dir(inputPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputPath, input, 0o600); err != nil {
		t.Fatal(err)
	}
	packetRel := filepath.ToSlash(filepath.Join(projectstate.CurrentDir, "reviews", "review-exact", "packet.json"))
	fields := []string{"item", "decision", "candidate_path"}
	packet := []byte(`{"packetId":"packet-exact","route":{"id":"_template:lane-feature-analysis","outputContract":"item,decision,candidate_path"},"shards":[{"id":"shard-01","items":["evidence/manifest.json"]}],"outputContract":"item,decision,candidate_path"}`)
	if err := os.WriteFile(filepath.Join(caseRoot, filepath.FromSlash(packetRel)), packet, 0o600); err != nil {
		t.Fatal(err)
	}
	dispatchPath := reviewersession.DispatchPath(filepath.Join(reviewRoot, "packet.json"), "shard-01", "dispatch-exact")
	dispatchRelPath, err := filepath.Rel(caseRoot, dispatchPath)
	if err != nil {
		t.Fatal(err)
	}
	dispatchRel := filepath.ToSlash(dispatchRelPath)
	if err := os.MkdirAll(filepath.Dir(dispatchPath), 0o700); err != nil {
		t.Fatal(err)
	}
	receipt := reviewersession.DispatchReceipt{
		SchemaVersion: 1, Kind: "reviewer-session-dispatch",
		DispatchID: "dispatch-exact", PacketID: "packet-exact",
		PacketPath: packetRel, PacketSHA256: bytesSHA256(packet),
		RouteID: "_template:lane-feature-analysis", ShardID: "shard-01",
		Items: []string{"evidence/manifest.json"}, PromptPath: inputPath,
		PromptSHA256: bytesSHA256(input), AgentType: "read-only-reviewer",
		ReadOnly: true, TargetLane: "feature-mission",
		PacketOwner:     reviewersession.Owner{CurrentExecutor: "member", ExecutorGeneration: 1, BindingMode: "current-executor-generation"},
		EffectiveOwner:  reviewersession.Owner{CurrentExecutor: "member", ExecutorGeneration: 1, BindingMode: "current-executor-generation"},
		ReviewerHarness: defaultHarness, ReviewerSession: "session-id",
		Actor: "test", RecordedAt: "2026-08-09T00:00:00Z", Capability: reviewersession.ReadOnlyCapability(),
		NoSpawn: true, NoHeavyTool: true, NoAuthority: true,
	}
	dispatch, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	dispatch = append(dispatch, '\n')
	if err := os.WriteFile(dispatchPath, dispatch, 0o600); err != nil {
		t.Fatal(err)
	}
	handoff := reviewerExternalHandoff{
		State:                         "reviewer-session-running-unknown",
		RunLoopStepID:                 "save-result-input",
		DispatchPromptPath:            receipt.PromptPath,
		DispatchPromptSHA256:          receipt.PromptSHA256,
		ReviewerDispatchID:            receipt.DispatchID,
		ReviewerDispatchReceiptPath:   dispatchRel,
		ReviewerDispatchReceiptSHA256: bytesSHA256(dispatch),
		ReviewerHarness:               receipt.ReviewerHarness,
		ReviewerSession:               receipt.ReviewerSession,
	}
	pkg, _, err := reviewerClaudePackage(caseRoot, pack, handoff)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Pack != pack || pkg.Launch == nil || pkg.Launch.InstructionIdentity == nil ||
		pkg.Launch.Capability != receipt.Capability || pkg.Launch.ReviewerIdentity == nil ||
		pkg.Launch.ReviewerIdentity.Capability != receipt.Capability ||
		pkg.Launch.ExpectedOutput != reviewerExpectedOutput(receipt, fields) {
		t.Fatalf("reviewer package omitted its production instruction binding: %+v", pkg)
	}
	productionInstructions, err := inlineProductionInstructions(caseRoot, pkg)
	if err != nil {
		t.Fatal(err)
	}
	prompt, _, err := claudeRequest(caseRoot, pkg, "session-id")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`packetId="packet-exact"`,
		`routeId="_template:lane-feature-analysis"`,
		`shardId="shard-01"`,
		`items=["evidence/manifest.json"]`,
		`exactly these fields: ["item","decision","candidate_path"]`,
		productionInstructions,
		"Packet SHA-256: `" + pkg.Launch.InstructionIdentity.SHA256 + "`",
		"Do not copy placeholder text such as packet.packetId",
		"Judge only the current reviewed manifest",
		"This session is the independent Reviewer for the current attempt",
		"do not require the member output to contain this later session's result",
		"historical rejection embedded in the replacement TaskContext is correction provenance",
		"Put only readable case-relative file references",
		"copy the first complete ida-index:... evidenceRef verbatim into result.summary",
		"exact observationEventId verbatim",
		"Keep result.routeOutput.evidence bound to an inspectable top-level evidenceRefs value",
		"exact case-relative packetPath and receiptPath",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("reviewer prompt omitted exact dispatch identity %q: %s", expected, prompt)
		}
	}
	placeholderResponse := json.RawMessage(`{
		"outcome":"returned",
		"result":{
			"packetId":"packet.packetId",
			"routeId":"_template:lane-feature-analysis",
			"shardId":"shard-01",
			"items":["evidence/manifest.json"],
			"reviewerSession":"session-id",
			"decision":"reject",
			"confidence":"high",
			"summary":"bounded rejection",
			"evidenceRefs":["evidence/manifest.json"],
			"risks":[],
			"conflicts":[],
			"recommendedVerdict":"rejected",
			"routeOutput":{"item":"evidence/manifest.json","decision":"reject","candidate_path":"bounded-candidate"}
		},
		"reason":""
	}`)
	placeholderRun := claudeRun{
		envelope: claudeEnvelope{Type: "result", Subtype: "success"},
		exitCode: 0, sessionID: "session-id", structuredOutput: placeholderResponse,
	}
	if err := validateClaudeStructuredResult(pkg, placeholderRun); err == nil || !strings.Contains(err.Error(), "exact dispatch") {
		t.Fatalf("pre-publication validation accepted placeholder reviewer identity: %v", err)
	}
	drifted := *pkg.Launch.ReviewerIdentity
	drifted.Capability = transportCapabilityForTest()
	pkg.Launch.ReviewerIdentity = &drifted
	if err := validateClaudeCapabilityBinding(caseRoot, pkg); err == nil || !strings.Contains(err.Error(), "durable dispatch lineage") {
		t.Fatalf("reviewer launch accepted capability drift: %v", err)
	}
	pkg.Launch.ReviewerIdentity = nil
	if _, _, err := claudeRequest(caseRoot, pkg, "session-id"); err == nil || !strings.Contains(err.Error(), "exact durable dispatch identity") {
		t.Fatalf("reviewer request accepted missing exact durable identity: %v", err)
	}
}

func TestClaudeEvidenceReviewRequestBindsExactReadOnlyDecision(t *testing.T) {
	caseRoot := t.TempDir()
	inputRel := ".rekit/evidence-review-input.json"
	input := []byte("{}\n")
	inputPath := filepath.Join(caseRoot, filepath.FromSlash(inputRel))
	if err := os.MkdirAll(filepath.Dir(inputPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputPath, input, 0o600); err != nil {
		t.Fatal(err)
	}
	pkg := hostPackageForTest(caseRoot, inputRel, input)
	pkg.SessionKind = "mission-commander-evidence-review"
	pkg.Launch.ReadOnly = true
	prompt, schema, err := claudeRequest(caseRoot, pkg, "session-id")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"independent Mission Commander evidence review", "exact packet, request sources, report, dispatch, receipt, and observation", "Reject on any missing, unreadable, ambiguous, or drifted binding", "do not write files or ledger state"} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("evidence review prompt omitted %q: %s", expected, prompt)
		}
	}
	for _, expected := range []string{`"enum":["accepted","rejected"]`, `"selectedEvidenceRef"`, `"observationEventId"`, `"receiptSha256"`, `"additionalProperties":false`} {
		if !strings.Contains(schema, expected) {
			t.Fatalf("evidence review schema omitted %q: %s", expected, schema)
		}
	}
}

func TestClaudeBinaryInventoryEvidenceReviewRequestUsesTypedPolicy(t *testing.T) {
	caseRoot := t.TempDir()
	inputRel := ".rekit/binary-inventory-evidence-review-input.json"
	input := []byte(`{"schemaVersion":1,"kind":"binary-inventory-evidence-review"}` + "\n")
	inputPath := filepath.Join(caseRoot, filepath.FromSlash(inputRel))
	if err := os.MkdirAll(filepath.Dir(inputPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputPath, input, 0o600); err != nil {
		t.Fatal(err)
	}
	pkg := hostPackageForTest(caseRoot, inputRel, input)
	pkg.SessionKind = "mission-commander-evidence-review"
	pkg.Launch.ReadOnly = true
	pkg.Launch.Input.Role = "mission-commander-binary-inventory-evidence-review-input"
	prompt, schema, err := claudeRequest(caseRoot, pkg, "session-id")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"binary inventory evidence review",
		"exact source, canonical inventory, report, dispatch, receipt, and observation",
		"all five safety boundaries",
		"inventory path is the exact selectedEvidenceRef",
		`"content":"{\"schemaVersion\":1,\"kind\":\"binary-inventory-evidence-review\"}\n"`,
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("binary inventory evidence review prompt omitted %q: %s", expected, prompt)
		}
	}
	for _, forbidden := range []string{"selected row is an exact source line", "matched term and evidence ref are exact"} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("binary inventory evidence review inherited VMP policy %q: %s", forbidden, prompt)
		}
	}
	if !strings.Contains(schema, `"selectedEvidenceRef"`) || !strings.Contains(schema, `"additionalProperties":false`) {
		t.Fatalf("binary inventory evidence review schema drifted: %s", schema)
	}

	pkg.Launch.Input.Role = "unknown-evidence-review-input"
	if _, _, err := claudeRequest(caseRoot, pkg, "session-id"); err == nil || !strings.Contains(err.Error(), "unsupported Mission Commander evidence review input role") {
		t.Fatalf("unknown evidence review role was accepted: %v", err)
	}
}

func TestRunTrustedEvidenceReviewClaudeRejectsUnboundExecutableBeforeLaunch(t *testing.T) {
	caseRoot := t.TempDir()
	run := runTrustedEvidenceReviewClaude(
		context.Background(),
		Options{
			Target:                            caseRoot,
			ClaudePath:                        filepath.Join(caseRoot, "missing-claude.exe"),
			ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
			ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
		},
		mission.CurrentLoopExternalSessionHarnessPackage{},
		"evidence-review-test",
		nil,
	)
	if run.started || run.spawnErr == nil || !strings.Contains(run.spawnErr.Error(), "bind trusted Claude for independent evidence review") ||
		strings.Contains(run.failureReason(), "exec: no command") {
		t.Fatalf("trusted evidence review did not fail at executable binding: %+v", run)
	}
}

func TestClaudeWebSecurityEvidenceReviewRequestsUseTypedPolicies(t *testing.T) {
	tests := []struct {
		name      string
		role      string
		kind      string
		expected  []string
		forbidden []string
	}{
		{
			name: "OpenAPI inventory", role: webSecurityOpenAPIReviewInputRole, kind: webSecurityOpenAPIReviewInputKind,
			expected: []string{
				"web-security OpenAPI inventory evidence review",
				"exact source, canonical inventory, report, dispatch, receipt, and observation",
				"Do not make a network request, construct or execute a replay",
				"use ambient credentials", "output a secret",
			},
			forbidden: []string{"selected row is an exact source line", "matched term and evidence ref are exact"},
		},
		{
			name: "bounded replay", role: webSecurityReplayReviewInputRole, kind: webSecurityReplayReviewInputKind,
			expected: []string{
				"web-security bounded replay evidence review",
				"exact redacted replay result, inventory, report, dispatch, receipt, and observation",
				"Do not make or repeat any network request", "do not retry or replace delivery-uncertain",
				"do not read an authRef environment value", "do not infer response body content from a digest",
				"Treat delivery-uncertain as terminal evidence", "do not output a secret",
			},
			forbidden: []string{"selected row is an exact source line", "matched term and evidence ref are exact"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caseRoot := t.TempDir()
			inputRel := ".rekit/web-security-evidence-review-input.json"
			input := []byte(`{"schemaVersion":1,"kind":"` + tt.kind + `"}` + "\n")
			inputPath := filepath.Join(caseRoot, filepath.FromSlash(inputRel))
			if err := os.MkdirAll(filepath.Dir(inputPath), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(inputPath, input, 0o600); err != nil {
				t.Fatal(err)
			}
			pkg := hostPackageForTest(caseRoot, inputRel, input)
			pkg.SessionKind = "mission-commander-evidence-review"
			pkg.Launch.ReadOnly = true
			pkg.Launch.Input.Role = tt.role
			prompt, schema, err := claudeRequest(caseRoot, pkg, "session-id")
			if err != nil {
				t.Fatal(err)
			}
			for _, expected := range tt.expected {
				if !strings.Contains(prompt, expected) {
					t.Fatalf("web-security evidence review prompt omitted %q: %s", expected, prompt)
				}
			}
			for _, forbidden := range tt.forbidden {
				if strings.Contains(prompt, forbidden) {
					t.Fatalf("web-security evidence review inherited VMP policy %q: %s", forbidden, prompt)
				}
			}
			if !strings.Contains(prompt, `"role":"`+tt.role+`"`) || !strings.Contains(prompt, `"content":"{\"schemaVersion\":1`) {
				t.Fatalf("web-security evidence review prompt omitted bound role/content: %s", prompt)
			}
			if !strings.Contains(schema, `"selectedEvidenceRef"`) || !strings.Contains(schema, `"additionalProperties":false`) {
				t.Fatalf("web-security evidence review schema drifted: %s", schema)
			}
		})
	}
}

func TestClaudeWebSecurityMemberBindingPolicies(t *testing.T) {
	tests := []struct {
		kind     string
		expected []string
	}{
		{
			kind:     webSecurityOpenAPIMemberBindingKind,
			expected: []string{"web-security-openapi-inventory-evidence", "Do not make a network request", "construct or execute a replay", "output any secret"},
		},
		{
			kind:     webSecurityReplayMemberBindingKind,
			expected: []string{"web-security-bounded-replay-evidence", "Do not make or repeat any network request", "do not retry or replace a delivery-uncertain", "do not read an authRef environment value", "do not infer response body content from a digest", "Treat delivery-uncertain as terminal evidence", "do not output any secret"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			caseRoot := t.TempDir()
			inputRel := ".rekit/member-task-context.json"
			input := []byte(`{"schemaVersion":2,"kind":"member-lane-execution-task-context","binding":{"kind":"` + tt.kind + `","values":{"artifact-path":"evidence.json"}}}` + "\n")
			inputPath := filepath.Join(caseRoot, filepath.FromSlash(inputRel))
			if err := os.MkdirAll(filepath.Dir(inputPath), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(inputPath, input, 0o600); err != nil {
				t.Fatal(err)
			}
			pkg := hostPackageForTest(caseRoot, inputRel, input)
			prompt, _, err := claudeRequest(caseRoot, pkg, "session-id")
			if err != nil {
				t.Fatal(err)
			}
			for _, expected := range tt.expected {
				if !strings.Contains(prompt, expected) {
					t.Fatalf("web-security member prompt omitted %q: %s", expected, prompt)
				}
			}
		})
	}
}

func TestValidateEvidenceReviewResponseFailsClosed(t *testing.T) {
	valid := evidenceReviewResponse{
		Decision: "accepted", Summary: "exact lineage", Reason: "all bindings agree",
		EvidenceRefs: []string{"packet.json", "receipt.json"}, SelectedEvidenceRef: "ida-index:function_index.tsv#L2",
		ObservationEventID: "obs-exact", ReceiptSHA256: strings.Repeat("a", 64),
	}
	if err := validateEvidenceReviewResponse(valid); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*evidenceReviewResponse){
		"decision":          func(value *evidenceReviewResponse) { value.Decision = "defer" },
		"summary":           func(value *evidenceReviewResponse) { value.Summary = "" },
		"reason":            func(value *evidenceReviewResponse) { value.Reason = "" },
		"refs":              func(value *evidenceReviewResponse) { value.EvidenceRefs = []string{"packet.json"} },
		"empty ref":         func(value *evidenceReviewResponse) { value.EvidenceRefs[1] = "" },
		"selected":          func(value *evidenceReviewResponse) { value.SelectedEvidenceRef = "" },
		"observation":       func(value *evidenceReviewResponse) { value.ObservationEventID = "" },
		"receipt malformed": func(value *evidenceReviewResponse) { value.ReceiptSHA256 = strings.Repeat("z", 64) },
	} {
		t.Run(name, func(t *testing.T) {
			copy := valid
			copy.EvidenceRefs = append([]string{}, valid.EvidenceRefs...)
			mutate(&copy)
			if err := validateEvidenceReviewResponse(copy); err == nil {
				t.Fatalf("invalid evidence review response was accepted: %+v", copy)
			}
		})
	}
	pkg := mission.CurrentLoopExternalSessionHarnessPackage{SessionKind: "mission-commander-evidence-review"}
	run := claudeRun{envelope: claudeEnvelope{Type: "result", Subtype: "success"}, exitCode: 0, sessionID: "session-id", structuredOutput: json.RawMessage(`{"decision":"accepted","summary":"x","reason":"y","evidenceRefs":["a","b"],"selectedEvidenceRef":"ref","observationEventId":"obs","receiptSha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","extra":true}`)}
	if err := validateClaudeStructuredResult(pkg, run); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("evidence review accepted extra structured field: %v", err)
	}
}

func TestRejectedVMPIDAEvidenceReviewStopsBeforeAcknowledgement(t *testing.T) {
	decision := evidenceReviewResponse{
		Decision: "rejected", Summary: "lineage rejected", Reason: "receipt drifted",
		EvidenceRefs: []string{"packet.json", "receipt.json"}, SelectedEvidenceRef: "ida-index:function_index.tsv#L2",
		ObservationEventID: "obs-exact", ReceiptSHA256: strings.Repeat("a", 64),
	}
	if err := requireAcceptedLiveAcceptanceVMPIDAEvidenceReview(decision); err == nil || !strings.Contains(err.Error(), "receipt drifted") {
		t.Fatalf("rejected evidence review did not stop before acknowledgement: %v", err)
	}
	decision.Decision = "accepted"
	if err := requireAcceptedLiveAcceptanceVMPIDAEvidenceReview(decision); err != nil {
		t.Fatal(err)
	}
}

func TestClaudeRequestRejectsUnknownSessionKind(t *testing.T) {
	caseRoot := t.TempDir()
	inputRel := ".rekit/input.json"
	input := []byte("{}\n")
	inputPath := filepath.Join(caseRoot, filepath.FromSlash(inputRel))
	if err := os.MkdirAll(filepath.Dir(inputPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputPath, input, 0o600); err != nil {
		t.Fatal(err)
	}
	pkg := hostPackageForTest(caseRoot, inputRel, input)
	pkg.SessionKind = "unknown"
	if _, _, err := claudeRequest(caseRoot, pkg, "session-id"); err == nil || !strings.Contains(err.Error(), "unsupported Claude session kind") {
		t.Fatalf("Claude request accepted unknown session kind: %v", err)
	}
}

func TestClaudeRequestRejectsNonUTF8BoundInput(t *testing.T) {
	caseRoot := t.TempDir()
	inputRel := ".rekit/non-utf8-input.bin"
	input := []byte{0xff, 0xfe, 0xfd}
	inputPath := filepath.Join(caseRoot, filepath.FromSlash(inputRel))
	if err := os.MkdirAll(filepath.Dir(inputPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputPath, input, 0o600); err != nil {
		t.Fatal(err)
	}
	pkg := hostPackageForTest(caseRoot, inputRel, input)
	if _, _, err := claudeRequest(caseRoot, pkg, "session-id"); err == nil || !strings.Contains(err.Error(), "valid UTF-8") {
		t.Fatalf("non-UTF-8 bound input error=%v", err)
	}
}

func TestClaudeFailureReasonPreservesStructuredValidationDetail(t *testing.T) {
	run := claudeRun{failureDetail: `validate real Claude ReviewerResult: packetId "packet.packetId" does not match current packet`}
	if reason := run.failureReason(); !strings.Contains(reason, "packet.packetId") || strings.Contains(reason, "did not return a successful structured result") {
		t.Fatalf("structured validation detail was hidden: %q", reason)
	}
}

func TestClaudeSchemasBindImmutableInputAndDurableSession(t *testing.T) {
	caseRoot := t.TempDir()
	inputRel := ".rekit/handoff.md"
	input := []byte("bounded task\n")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(caseRoot, filepath.FromSlash(inputRel))), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, filepath.FromSlash(inputRel)), input, 0o600); err != nil {
		t.Fatal(err)
	}
	pkg := hostPackageForTest(caseRoot, inputRel, input)
	prompt, schema, err := claudeRequest(caseRoot, pkg, "session-id")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`boundInput={"sha256":"` + bytesSHA256(input) + `","bytes":13,"content":"bounded task\n"}`,
		"Do not ask for, reconstruct, concatenate, or Read an input file path",
		"boundInput.content is the complete task instruction",
		"never end the response with another Read call",
		"missing bounded evidence is not a process failure",
		"independent Reviewer can reject it",
		"independent Reviewer is a later runtime-owned segment",
		"do not defer merely because that later review has not happened",
		"explicitly address every field",
		"historical Reviewer rejection",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("member Claude request omitted %q: %s", expected, prompt)
		}
	}
	for _, forbidden := range []string{
		filepath.Join(caseRoot, filepath.FromSlash(inputRel)),
		caseRoot,
		"exact absolute path",
		"using the Read tool before answering",
	} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("member Claude request leaked hand-copied input path %q: %s", forbidden, prompt)
		}
	}
	if !strings.Contains(schema, `"outputs"`) {
		t.Fatalf("member Claude schema omitted outputs: %s", schema)
	}
	pkg.Launch.Attempt.Session = "different"
	if _, _, err := claudeRequest(caseRoot, pkg, "session-id"); err == nil {
		t.Fatal("Claude request accepted a session different from the durable attempt")
	}
}

func hostPackageForTest(caseRoot, inputRel string, input []byte) mission.CurrentLoopExternalSessionHarnessPackage {
	return mission.CurrentLoopExternalSessionHarnessPackage{
		CaseRoot:    caseRoot,
		SessionKind: "member",
		Launch: &mission.CurrentLoopExternalSessionHarnessLaunch{
			Ready: true,
			Input: mission.CurrentLoopExternalSessionHarnessInput{Path: inputRel, SHA256: bytesSHA256(input)},
			Attempt: mission.CurrentLoopExternalSessionAttempt{
				Session:           "session-id",
				SubmissionOutputs: ".rekit/external-session-attempt-inputs/member-job/000001/outputs",
			},
		},
		Return: &mission.CurrentLoopExternalSessionReturnContract{
			SubmissionOutputs: ".rekit/external-session-attempt-inputs/member-job/000001/outputs",
		},
	}
}
