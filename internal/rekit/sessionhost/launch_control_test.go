package sessionhost

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewersession"
)

func TestClaudeLaunchControlRequiredOnlyForCurrentProject(t *testing.T) {
	currentRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(currentRoot, projectstate.CurrentDir), 0o755); err != nil {
		t.Fatal(err)
	}
	legacyRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(legacyRoot, projectstate.LegacyDir), 0o755); err != nil {
		t.Fatal(err)
	}
	ordinaryRoot := t.TempDir()

	for _, test := range []struct {
		name     string
		caseRoot string
		want     bool
	}{
		{name: "current", caseRoot: currentRoot, want: true},
		{name: "legacy", caseRoot: legacyRoot, want: false},
		{name: "ordinary", caseRoot: ordinaryRoot, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			required, err := claudeLaunchControlRequired(test.caseRoot)
			if err != nil || required != test.want {
				t.Fatalf("control required=%t err=%v, want %t", required, err, test.want)
			}
		})
	}
}

func TestClaudeLaunchControlRejectsStaleStateBeforeLaunch(t *testing.T) {
	for _, fixture := range []struct {
		name   string
		mutate func(*testing.T, string, string)
		want   string
	}{
		{
			name: "paused",
			mutate: func(t *testing.T, caseRoot, lane string) {
				applyClaudeLaunchControlForTest(t, caseRoot, lane, executioncontrol.ActionPause, "2026-08-18T13:00:00Z")
			},
			want: "control state is paused",
		},
		{
			name: "stopped",
			mutate: func(t *testing.T, caseRoot, lane string) {
				applyClaudeLaunchControlForTest(t, caseRoot, lane, executioncontrol.ActionStop, "2026-08-18T13:00:00Z")
			},
			want: "control state is stopped",
		},
		{
			name: "running-head-drift",
			mutate: func(t *testing.T, caseRoot, lane string) {
				applyClaudeLaunchControlForTest(t, caseRoot, lane, executioncontrol.ActionPause, "2026-08-18T13:00:00Z")
				applyClaudeLaunchControlForTest(t, caseRoot, lane, executioncontrol.ActionResume, "2026-08-18T13:01:00Z")
			},
			want: "control head changed",
		},
		{
			name:   "pending",
			mutate: publishPendingClaudeLaunchControlForTest,
			want:   "has pending execution control",
		},
		{
			name:   "owner-drift",
			mutate: driftClaudeLaunchOwnerForTest,
			want:   "owner is stale",
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			caseRoot, opt, pkg, _, _ := projectExecutionLaunchFixture(t)
			bound, err := ensureClaudeLaunchControlBinding(opt, pkg)
			if err != nil {
				t.Fatal(err)
			}
			if bound.launchControlBinding == nil {
				t.Fatal("current project launch omitted its expected control binding")
			}
			binding := cloneClaudeLaunchControlBinding(bound.launchControlBinding)
			fixture.mutate(t, caseRoot, binding.Lane)

			launches := 0
			err = withClaudeLaunchControl(caseRoot, binding, pkg, func() error {
				launches++
				return nil
			})
			if err == nil || !strings.Contains(err.Error(), fixture.want) {
				t.Fatalf("stale launch error = %v, want %q", err, fixture.want)
			}
			if launches != 0 {
				t.Fatalf("stale launch invoked process boundary %d times", launches)
			}
		})
	}
}

func TestClaudeLaunchControlFreshResumeBindingAllowsLaunch(t *testing.T) {
	caseRoot, opt, pkg, _, _ := projectExecutionLaunchFixture(t)
	owner, err := claudeLaunchOwner(caseRoot, pkg)
	if err != nil {
		t.Fatal(err)
	}
	applyClaudeLaunchControlForTest(t, caseRoot, owner.Lane, executioncontrol.ActionPause, "2026-08-18T13:00:00Z")
	applyClaudeLaunchControlForTest(t, caseRoot, owner.Lane, executioncontrol.ActionResume, "2026-08-18T13:01:00Z")

	bound, err := ensureClaudeLaunchControlBinding(opt, pkg)
	if err != nil {
		t.Fatal(err)
	}
	if bound.launchControlBinding == nil || bound.launchControlBinding.ControlGeneration != 2 ||
		bound.launchControlBinding.ControlReceiptSHA256 == "" || bound.launchControlBinding.Owner != owner {
		t.Fatalf("fresh resume launch binding = %+v", bound.launchControlBinding)
	}
	launches := 0
	if err := withClaudeLaunchControl(caseRoot, bound.launchControlBinding, pkg, func() error {
		launches++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if launches != 1 {
		t.Fatalf("fresh resume launch boundary calls = %d, want 1", launches)
	}
}

func TestSupervisionSpecBindsControlHeadIntoRunIdentity(t *testing.T) {
	caseRoot, opt, pkg, _, _ := projectExecutionLaunchFixture(t)
	paths, first, firstData, firstSHA, err := prepareSupervision(opt, pkg, pkg.Launch.Attempt.Session)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(paths.root) })
	if first.LaunchControl == nil || first.LaunchControl.ControlGeneration != 0 ||
		first.LaunchControl.ControlReceiptSHA256 != "" || first.LaunchControl.Owner.Lane == "" ||
		bytesSHA256(firstData) != firstSHA {
		t.Fatalf("initial supervision control binding = %+v", first.LaunchControl)
	}

	applyClaudeLaunchControlForTest(t, caseRoot, first.LaunchControl.Lane, executioncontrol.ActionPause, "2026-08-18T13:00:00Z")
	applyClaudeLaunchControlForTest(t, caseRoot, first.LaunchControl.Lane, executioncontrol.ActionResume, "2026-08-18T13:01:00Z")
	_, resumed, resumedData, resumedSHA, err := prepareSupervision(opt, pkg, pkg.Launch.Attempt.Session)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.LaunchControl == nil || resumed.LaunchControl.ControlGeneration != 2 ||
		resumed.LaunchControl.ControlReceiptSHA256 == "" || resumed.RunID == first.RunID ||
		resumedSHA == firstSHA || string(resumedData) == string(firstData) {
		t.Fatalf("resumed supervision did not bind a new control head: first=%+v resumed=%+v", first.LaunchControl, resumed.LaunchControl)
	}
}

func TestSupervisorChildRejectsPausedControlBeforeProcessStart(t *testing.T) {
	caseRoot, opt, pkg, readyPath, _ := projectExecutionLaunchFixture(t)
	paths, spec, specData, specSHA, err := prepareSupervision(opt, pkg, pkg.Launch.Attempt.Session)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(paths.root) })
	if err := os.WriteFile(paths.spec, specData, 0o600); err != nil {
		t.Fatal(err)
	}
	handoff, err := projectexecution.NewHandoff(caseRoot, spec.RunID, specSHA, spec.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if err := projectexecution.PublishHandoff(caseRoot, handoff); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = projectexecution.CancelHandoff(caseRoot, handoff) })
	applyClaudeLaunchControlForTest(t, caseRoot, spec.LaunchControl.Lane, executioncontrol.ActionPause, "2026-08-18T13:00:00Z")

	if err := RunSupervisorChild(context.Background(), paths.spec, specSHA); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(readyPath); !os.IsNotExist(err) {
		t.Fatalf("paused supervisor child reached process helper: %v", err)
	}
	terminal, ok, err := readSupervisionTerminal(paths, spec, specSHA)
	if err != nil || !ok || terminal.Started || !strings.Contains(terminal.SpawnError, "control state is paused") {
		t.Fatalf("paused supervisor terminal ok=%t err=%v receipt=%+v", ok, err, terminal)
	}
	if _, err := os.Lstat(paths.started); !os.IsNotExist(err) {
		t.Fatalf("paused supervisor child published started receipt: %v", err)
	}
}

func TestClaudeReviewerLaunchControlUsesEffectiveDispatchOwner(t *testing.T) {
	caseRoot, opt, memberPackage, _, _ := projectExecutionLaunchFixture(t)
	owner, err := claudeLaunchOwner(caseRoot, memberPackage)
	if err != nil {
		t.Fatal(err)
	}
	reviewRoot := mustProjectStatePathForLaunchTest(t, caseRoot, "reviews", "launch-control-review")
	packetRel, err := projectstate.Rel(caseRoot, "reviews", "launch-control-review", "packet.json")
	if err != nil {
		t.Fatal(err)
	}
	promptRel, err := projectstate.Rel(caseRoot, "reviews", "launch-control-review", "prompts", "shard-01.md")
	if err != nil {
		t.Fatal(err)
	}
	packet := []byte(`{"packetId":"launch-control-packet","route":{"id":"binary-re:launch-control","outputContract":"item,decision"},"shards":[{"id":"shard-01","items":["bounded-item"]}],"outputContract":"item,decision"}`)
	prompt := []byte("review the bounded launch-control item\n")
	for path, data := range map[string][]byte{
		filepath.Join(caseRoot, filepath.FromSlash(packetRel)): packet,
		filepath.Join(caseRoot, filepath.FromSlash(promptRel)): prompt,
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	dispatchPath := reviewersession.DispatchPath(filepath.Join(reviewRoot, "packet.json"), "shard-01", "launch-control-dispatch")
	dispatchRel, err := filepath.Rel(caseRoot, dispatchPath)
	if err != nil {
		t.Fatal(err)
	}
	dispatchRel = filepath.ToSlash(dispatchRel)
	reviewOwner := reviewersession.Owner{
		CurrentExecutor: owner.CurrentExecutor, ExecutorGeneration: owner.ExecutorGeneration,
		BindingMode: "current-executor-generation",
	}
	receipt := reviewersession.DispatchReceipt{
		SchemaVersion: 1, Kind: "reviewer-session-dispatch", DispatchID: "launch-control-dispatch",
		PacketID: "launch-control-packet", PacketPath: packetRel, PacketSHA256: bytesSHA256(packet),
		RouteID: "binary-re:launch-control", ShardID: "shard-01", Items: []string{"bounded-item"},
		PromptPath: promptRel, PromptSHA256: bytesSHA256(prompt), AgentType: "read-only-reviewer", ReadOnly: true,
		TargetLane: owner.Lane, PacketOwner: reviewOwner, EffectiveOwner: reviewOwner,
		ReviewerHarness: defaultHarness, ReviewerSession: "launch-control-review-session",
		Actor: "launch-control-test", RecordedAt: "2026-08-18T13:00:00Z",
		NoSpawn: true, NoHeavyTool: true, NoAuthority: true,
	}
	dispatch, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	dispatch = append(dispatch, '\n')
	if err := os.MkdirAll(filepath.Dir(dispatchPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dispatchPath, dispatch, 0o600); err != nil {
		t.Fatal(err)
	}
	fields := []string{"item", "decision"}
	reviewerPackage := mission.CurrentLoopExternalSessionHarnessPackage{
		SchemaVersion: 1, State: "launch-ready", CaseRoot: caseRoot, SessionKind: "reviewer",
		Launch: &mission.CurrentLoopExternalSessionHarnessLaunch{
			Ready: true, Tool: "Claude Code Agent", AgentType: receipt.AgentType, ReadOnly: true,
			Input:            mission.CurrentLoopExternalSessionHarnessInput{Path: promptRel, SHA256: bytesSHA256(prompt), Role: "reviewer-dispatch-prompt"},
			ExpectedOutput:   reviewerExpectedOutput(receipt, fields),
			ReviewerIdentity: reviewerLaunchIdentity(receipt, fields, dispatchRel, bytesSHA256(dispatch)),
			Attempt:          mission.CurrentLoopExternalSessionAttempt{AttemptID: receipt.DispatchID, AttemptSHA256: bytesSHA256(dispatch), Generation: 1, Harness: defaultHarness, Session: receipt.ReviewerSession},
		},
	}
	bound, err := ensureClaudeLaunchControlBinding(opt, reviewerPackage)
	if err != nil {
		t.Fatal(err)
	}
	if bound.launchControlBinding == nil || bound.launchControlBinding.Owner != owner || bound.launchControlBinding.Lane != receipt.TargetLane {
		t.Fatalf("reviewer launch control binding = %+v, owner=%+v", bound.launchControlBinding, owner)
	}
}

func applyClaudeLaunchControlForTest(t *testing.T, caseRoot, lane, action, stamp string) executioncontrol.Plan {
	t.Helper()
	preview, err := executioncontrol.Preview(caseRoot, executioncontrol.Options{
		Lane: lane, Action: action, Actor: "launch-control-test",
		Reason: "exercise exact launch control head", PublicationStamp: stamp,
	})
	if err != nil {
		t.Fatal(err)
	}
	applied, err := executioncontrol.Apply(caseRoot, executioncontrol.Options{
		Lane: preview.Lane, Action: preview.Action, Actor: preview.Actor, Reason: preview.Reason,
		PublicationStamp: preview.PublicationStamp, ExpectedPlanSHA256: preview.ExpectedPlanSHA256,
	})
	if err != nil {
		t.Fatal(err)
	}
	return applied
}

func publishPendingClaudeLaunchControlForTest(t *testing.T, caseRoot, lane string) {
	t.Helper()
	preview, err := executioncontrol.Preview(caseRoot, executioncontrol.Options{
		Lane: lane, Action: executioncontrol.ActionPause, Actor: "launch-control-test",
		Reason: "leave exact launch control intent pending", PublicationStamp: "2026-08-18T13:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	intent := executioncontrol.Intent{
		SchemaVersion: executioncontrol.SchemaVersion, Kind: "lane-execution-control-intent",
		Lane: preview.Lane, Action: preview.Action, PreviousState: preview.PreviousState, State: preview.State,
		ControlGeneration: preview.ControlGeneration, PreviousReceiptSHA: preview.PreviousReceiptSHA,
		Owner: preview.Owner, Actor: preview.Actor, Reason: preview.Reason, PublicationStamp: preview.PublicationStamp,
		IntentPath: preview.IntentPath, ReceiptPath: preview.ReceiptPath, PlanSHA256: preview.ExpectedPlanSHA256,
		NoAuthority: true, NoConfirmed: true, NoHeavyTool: true, NoAutoResume: true,
	}
	data, err := json.MarshalIndent(intent, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	path := filepath.Join(caseRoot, filepath.FromSlash(intent.IntentPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	inspection, err := executioncontrol.Inspect(caseRoot, lane)
	if err != nil {
		t.Fatal(err)
	}
	if !inspection.Pending || inspection.PendingGeneration != preview.ControlGeneration {
		t.Fatalf("pending launch control fixture = %+v", inspection)
	}
}

func driftClaudeLaunchOwnerForTest(t *testing.T, caseRoot, lane string) {
	t.Helper()
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for index := range board.Lanes {
		if board.Lanes[index].ID != lane {
			continue
		}
		board.Lanes[index].CurrentExecutor += "-replacement"
		board.Lanes[index].ExecutorGeneration++
		found = true
		break
	}
	if !found {
		t.Fatalf("owner drift fixture omitted lane %s", lane)
	}
	writeClaudeLaunchJSONForTest(t, mustProjectStatePathForLaunchTest(t, caseRoot, "board.json"), board)

	lanePath := mustProjectStatePathForLaunchTest(t, caseRoot, "lanes", lane, "lane.json")
	data, err := os.ReadFile(lanePath)
	if err != nil {
		t.Fatal(err)
	}
	var laneDocument map[string]any
	if err := json.Unmarshal(data, &laneDocument); err != nil {
		t.Fatal(err)
	}
	for _, boardLane := range board.Lanes {
		if boardLane.ID == lane {
			laneDocument["currentExecutor"] = boardLane.CurrentExecutor
			laneDocument["executorGeneration"] = boardLane.ExecutorGeneration
			break
		}
	}
	writeClaudeLaunchJSONForTest(t, lanePath, laneDocument)
}

func mustProjectStatePathForLaunchTest(t *testing.T, caseRoot string, elements ...string) string {
	t.Helper()
	path, err := projectstate.Join(caseRoot, elements...)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func writeClaudeLaunchJSONForTest(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}
