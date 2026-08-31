package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestBuildStatusTypedInputRequiredPreservesCurrentDriverRequest(t *testing.T) {
	caseRoot := attachedCaseWithPack(t, "binary-re")
	root := repoRoot(t)
	if _, err := casebind.WriteLegacyMetadata(caseRoot, root, "binary-re"); err != nil {
		t.Fatal(err)
	}
	if _, err := casebind.WriteInitialState(caseRoot, root, "binary-re"); err != nil {
		t.Fatal(err)
	}
	copyRepoFile(t, root, "rekit/templates/case-shim/SKILL.md", caseRoot, ".claude/skills/rekit/SKILL.md")
	for _, rel := range []string{
		"references/binary-re/README.md",
		"references/binary-re/agent-driven-re.md",
		"references/binary-re/workflow-template.md",
		"references/binary-re/progressive-disclosure.md",
		"references/binary-re/toolchain-router.md",
		"references/binary-re/singleton-handler-review.md",
		"references/binary-re/lane-collaboration.md",
		"references/binary-re/general-analysis.md",
		"references/binary-re/general-agent-team.md",
		"references/binary-re/general-workflow.md",
		"references/binary-re/general-toolchain-router.md",
	} {
		copyRepoFile(t, root, "packs/binary-re/"+rel, caseRoot, rel)
	}
	copyRepoFile(t, root, "packs/binary-re/CLAUDE.local.snippet.md", caseRoot, "CLAUDE.local.md")
	template, err := os.ReadFile(filepath.Join(root, "packs", "binary-re", "references", "binary-re", "task-handoff.template.md"))
	if err != nil {
		t.Fatal(err)
	}
	writeCaseFile(t, caseRoot, "references/binary-re/task-handoff.md", strings.ReplaceAll(strings.ReplaceAll(string(template), "<PROJECT_NAME>", "demo"), "<PROJECT_ROOT>", caseRoot))
	var out strings.Builder
	if err := Run([]string{
		"-Command", "onboard", "-Target", caseRoot, "-Pack", "binary-re",
		"-ProjectName", "typed-input-status", "-Goal", "inspect a bounded target",
		"-Actor", "mission-commander", "-Executor", "typed-input-executor",
		"-InitialLane", "binary-analysis-initial", "-WhatIf", "-Format", "json",
	}, &out); err != nil {
		t.Fatal(err)
	}
	var onboarding onboardCLIPlan
	if err := json.Unmarshal([]byte(out.String()), &onboarding); err != nil || len(onboarding.ApplyArgs) == 0 {
		t.Fatalf("onboarding preview=%+v err=%v", onboarding, err)
	}
	out.Reset()
	if err := Run(onboarding.ApplyArgs, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{
		"-Command", "overview", "-Target", caseRoot, "-Pack", "binary-re", "-Format", "json",
	}, &out); err != nil {
		t.Fatal(err)
	}
	var overview struct {
		Lanes []struct {
			ID                 string `json:"id"`
			CurrentExecutor    string `json:"currentExecutor"`
			ExecutorGeneration int    `json:"executorGeneration"`
		} `json:"lanes"`
	}
	if err := json.Unmarshal([]byte(out.String()), &overview); err != nil || len(overview.Lanes) == 0 {
		t.Fatalf("overview=%+v err=%v\n%s", overview, err, out.String())
	}
	lane := ""
	for _, candidate := range overview.Lanes {
		if candidate.CurrentExecutor != "" && candidate.ExecutorGeneration > 0 {
			lane = candidate.ID
			break
		}
	}
	if lane == "" {
		out.Reset()
		if err := Run([]string{
			"-Command", "start", "-Target", caseRoot, "-Pack", "binary-re",
			"-Name", "initial", "-Executor", "typed-input-executor", "-Actor", "mission-commander",
			"-Reason", "establish typed input status owner", "-Apply", "-Format", "json",
		}, &out); err != nil {
			t.Fatal(err)
		}
		var started struct {
			Lane struct {
				ID string `json:"id"`
			} `json:"lane"`
		}
		if err := json.Unmarshal([]byte(out.String()), &started); err != nil || started.Lane.ID == "" {
			t.Fatalf("start=%+v err=%v\n%s", started, err, out.String())
		}
		lane = started.Lane.ID
	}
	out.Reset()
	if err := Run([]string{
		"-Command", "status", "-Target", caseRoot, "-Pack", "binary-re",
		"-Lane", lane, "-Format", "json",
	}, &out); err != nil {
		t.Fatal(err)
	}
	var status statusInventory
	if err := json.Unmarshal([]byte(out.String()), &status); err != nil {
		t.Fatalf("status JSON did not decode: %v\n%s", err, out.String())
	}
	if status.MemberExecution == nil || status.MemberExecution.State != "input-required" ||
		status.MemberExecution.Ready || status.MemberExecution.InputReadiness == nil ||
		status.MemberExecution.InputReadiness.State != "input-required" {
		t.Fatalf("status omitted typed input readiness: %+v", status.MemberExecution)
	}
	request := status.MissionControlRunbook.CurrentDriverRequest
	if request == nil || request.Blocked || !request.CommandExecutable || request.Invocation == nil ||
		request.RunLoopStepID != "preview-current" || strings.TrimSpace(request.Command) == "" {
		t.Fatalf("typed readiness rewrote the authoritative current request: %+v", request)
	}
	if current := status.CaseMission.MissionCommanderActionQueue.CurrentAction; current == nil || current.State == "input-required" {
		t.Fatalf("typed readiness rewrote the Mission Control action queue: %+v", current)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "binary-re", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var aggregate statusInventory
	if err := json.Unmarshal([]byte(out.String()), &aggregate); err != nil {
		t.Fatalf("aggregate status did not decode: %v\n%s", err, out.String())
	}
	if aggregate.MemberExecution == nil || aggregate.MemberExecution.InputReadiness == nil ||
		aggregate.MemberExecution.InputReadiness.State != "input-required" {
		t.Fatalf("aggregate status omitted typed input readiness: member=%+v runbook=%+v controls=%+v", aggregate.MemberExecution, aggregate.MissionControlRunbook, aggregate.ExecutionControls)
	}

	var publicOut strings.Builder
	if err := RunPublic([]string{"status", "--target", caseRoot}, &publicOut, ""); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"typed 输入", "没有启动 member 或 Reviewer", "artifact-analysis", "workspace-inventory"} {
		if !strings.Contains(publicOut.String(), expected) {
			t.Fatalf("public typed input summary omitted %q: %s", expected, publicOut.String())
		}
	}
	for _, forbidden := range []string{lane, caseRoot} {
		if strings.Contains(publicOut.String(), forbidden) {
			t.Fatalf("public typed input summary leaked %q: %s", forbidden, publicOut.String())
		}
	}
}

func TestBindStatusHistoricalAttemptRequiresInputForCurrentOwnerGeneration(t *testing.T) {
	caseRoot := attachedCaseWithPack(t, "binary-re")
	stateRoot := filepath.Join(caseRoot, ".rekit")
	lane := "feature-analysis"
	board := mission.Board{
		SchemaVersion: 1,
		CaseRoot:      caseRoot,
		Pack:          "binary-re",
		Lanes: []mission.BoardLane{{
			ID:                 lane,
			Status:             "open",
			CurrentExecutor:    "executor-a",
			ExecutorGeneration: 1,
		}},
	}
	writeBoard := func() {
		data, err := json.Marshal(board)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stateRoot, "board.json"), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeBoard()
	laneRoot := filepath.Join(stateRoot, "lanes", lane)
	for path, data := range map[string][]byte{
		filepath.Join(laneRoot, "prompts", "RESUME.md"):       []byte("# Resume\n\nContinue the bounded task.\n"),
		filepath.Join(laneRoot, "checkpoints", "latest.json"): []byte("{\"schemaVersion\":1,\"lane\":\"feature-analysis\",\"status\":\"active\"}\n"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	dispatch, err := memberexecution.PreviewDispatch(memberexecution.DispatchOptions{
		CaseRoot: caseRoot, Pack: board.Pack, Lane: lane,
		RequestSHA256: strings.Repeat("a", 64), CreatedAt: "2026-08-31T01:02:03Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := memberexecution.Apply(dispatch, dispatch.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	board.Lanes[0].CurrentExecutor = "executor-b"
	board.Lanes[0].ExecutorGeneration = 2
	writeBoard()

	invocation, err := commands.NewPublicInvocation(commands.Continue, "-Target", caseRoot, "-Pack", board.Pack, "-Lane", lane, "-WhatIf", "-Format", "json")
	if err != nil {
		t.Fatal(err)
	}
	request := mission.MissionCommanderDriverRequest{
		Source: mission.MemberContinuationSource, State: mission.MemberContinuationState,
		Lane: lane, Invocation: &invocation,
	}
	status := statusInventory{
		Target: caseRoot, Pack: board.Pack, Mode: "case",
		MissionControlRunbook: &statusMissionControlRunbook{
			Scope: "case", CurrentDriverRequest: &request,
		},
	}
	bindStatusMemberExecution(&status)
	if status.MemberExecution == nil || status.MemberExecution.State != "input-required" ||
		status.MemberExecution.Ready || status.MemberExecution.InputReadiness == nil ||
		status.MemberExecution.InputReadiness.State != "input-required" {
		t.Fatalf("historical attempt masked current typed input readiness: %+v", status.MemberExecution)
	}
	if status.MissionControlRunbook.CurrentDriverRequest != &request || status.MissionControlRunbook.CurrentDriverRequest.Lane != lane {
		t.Fatalf("typed readiness rewrote current driver request: %+v", status.MissionControlRunbook.CurrentDriverRequest)
	}
}

func TestBindStatusMemberExecutionIgnoresNonMemberRoutes(t *testing.T) {
	caseRoot := t.TempDir()
	stateRoot := filepath.Join(caseRoot, ".rekit")
	if err := os.MkdirAll(stateRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	board := mission.Board{Lanes: []mission.BoardLane{{
		ID: "feature-analysis", Status: "open",
		CurrentExecutor: "executor-a", ExecutorGeneration: 1,
	}}}
	data, err := json.Marshal(board)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateRoot, "board.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, runbook := range []*statusMissionControlRunbook{
		{Scope: "reviewer"},
		{Scope: "case", CurrentDriverRequest: &mission.MissionCommanderDriverRequest{
			Lane: "feature-analysis", Source: "executionEvidenceReview.pending",
		}},
	} {
		status := statusInventory{Target: caseRoot, Pack: "binary-re", Mode: "case", MissionControlRunbook: runbook}
		bindStatusMemberExecution(&status)
		if status.MemberExecution != nil {
			t.Fatalf("non-member route projected typed input readiness: scope=%q request=%+v member=%+v", runbook.Scope, runbook.CurrentDriverRequest, status.MemberExecution)
		}
	}
}

func TestBindStatusSingleLaneInputRequiredIgnoresAuthorityLane(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	board := mission.Board{Lanes: []mission.BoardLane{{
		ID: "devirt-main", Status: "open", Authority: true,
		CurrentExecutor: "authority-owner", ExecutorGeneration: 1,
	}}}
	data, err := json.Marshal(board)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	status := statusInventory{Target: caseRoot, Pack: "binary-re", Mode: "case"}
	bindStatusSingleLaneInputRequired(&status)
	if status.MemberExecution != nil {
		t.Fatalf("authority lane was projected as typed member input: %+v", status.MemberExecution)
	}
}

func TestBindStatusReviewerCorrectionPreservesOpenReconcileAction(t *testing.T) {
	reconcile := mission.MissionCommanderNextActionItem{
		Lane:           "feature-mission",
		Label:          "Feature Mission",
		ActionID:       "reconcile:daily-correction-current",
		State:          "needs-reconcile",
		Command:        `/rekit reconcile "Feature Mission" -InterventionEventId daily-correction-current -WhatIf -Format json`,
		Source:         "missionCommanderActions",
		RequiresReview: true,
	}
	queue := mission.MissionCommanderActionQueueFor([]mission.MissionCommanderNextActionItem{reconcile})
	status := reviewerRejectedStatusForTest(queue)

	bindStatusReviewerCorrection(&status)

	current := status.CaseMission.MissionCommanderActionQueue.CurrentAction
	if current == nil || current.ActionID != reconcile.ActionID || !strings.Contains(current.Command, "/rekit reconcile") {
		t.Fatalf("current action = %+v", current)
	}
	request := status.MissionControlRunbook.CurrentDriverRequest
	if request == nil || !strings.Contains(request.Command, "/rekit reconcile") {
		t.Fatalf("current driver request = %+v", request)
	}
	if !strings.Contains(strings.Join(status.MemberExecution.Boundary, "\n"), "already recorded") {
		t.Fatalf("member boundary = %v", status.MemberExecution.Boundary)
	}
}

func TestBindStatusReviewerCorrectionProjectsCorrectionBeforeIntervention(t *testing.T) {
	status := reviewerRejectedStatusForTest(mission.MissionCommanderActionQueue{})

	bindStatusReviewerCorrection(&status)

	current := status.CaseMission.MissionCommanderActionQueue.CurrentAction
	if current == nil || current.State != "reviewer-rejected-awaiting-correction" || !strings.HasPrefix(current.Command, "rekit-host -daily") {
		t.Fatalf("current action = %+v", current)
	}
}

func reviewerRejectedStatusForTest(queue mission.MissionCommanderActionQueue) statusInventory {
	caseMission := &statusCaseMission{
		MissionCommanderActionQueue: queue,
		DailyMissionControlRunbook: workstream.DailyMissionControlRunbookFor(
			`C:\case`,
			"case",
			queue,
			"",
			"",
		),
	}
	return statusInventory{
		Target:      `C:\case`,
		CaseMission: caseMission,
		MemberExecution: &memberExecutionStatus{
			State:             "reviewer-rejected-awaiting-correction",
			Lane:              "feature-mission",
			CorrectionCommand: `rekit-host -daily -target "C:\case" -correction "<human-correction>" -actor "<actor>"`,
			ReviewerRejection: &workstream.MemberReviewerRejection{
				PacketID:            "packet-1",
				DecisionEventID:     "decision-1",
				VerificationEventID: "verification-1",
				ReviewerSession:     "reviewer-session-1",
				OwnerGeneration:     1,
				Summary:             "required acceptance condition is missing",
			},
		},
		MissionControlRunbook: buildStatusMissionControlRunbook(`C:\case`, caseMission, nil),
	}
}
