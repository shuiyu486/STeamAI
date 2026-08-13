package sessionhost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/cli"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanecompletion"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	"github.com/shuiyu486/re-context-kits/internal/rekit/note"
	rekitruntime "github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestRunDailyRejectsGoalAndCorrectionTogether(t *testing.T) {
	result, err := RunDaily(context.Background(), DailyOptions{Target: filepath.Join(t.TempDir(), "case"), Goal: "goal", Correction: "correction"})
	if err == nil || !strings.Contains(err.Error(), "either -goal or -correction") || result.Mode != "correction" {
		t.Fatalf("RunDaily result=%+v err=%v", result, err)
	}
	if result.Action == nil || result.Action.Code != DailyActionFailed || result.Action.RequiresInput {
		t.Fatalf("RunDaily failure action = %+v", result.Action)
	}
}

func TestRunDailyCaseReadyHookCapturesFreshOnboardingFailure(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "fresh-hook")
	calls := 0
	result, err := RunDaily(context.Background(), DailyOptions{
		Target:                            caseRoot,
		Goal:                              "capture fresh partial onboarding",
		ClaudePath:                        missingClaudePath(t),
		ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
		onCaseReady: func(root string) error {
			calls++
			if root != caseRoot {
				t.Fatalf("fresh hook root=%s want=%s", root, caseRoot)
			}
			if info, statErr := os.Lstat(root); statErr != nil || !info.IsDir() {
				t.Fatalf("fresh hook observed invalid root: info=%v err=%v", info, statErr)
			}
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "validate trusted Claude Code executable") {
		t.Fatalf("fresh hook result=%+v err=%v", result, err)
	}
	if calls != 1 {
		t.Fatalf("fresh hook calls=%d want=1", calls)
	}
}

func TestRunDailyCaseReadyHookCapturesExistingCaseBeforeMutation(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "existing-hook")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "existing hook goal", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	result, err := RunDaily(context.Background(), DailyOptions{
		Target:                            caseRoot,
		Goal:                              inspection.Identity.Goal,
		ClaudePath:                        missingClaudePath(t),
		ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
		onCaseReady: func(root string) error {
			calls++
			if root != caseRoot {
				t.Fatalf("existing hook root=%s want=%s", root, caseRoot)
			}
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "validate trusted Claude Code executable") {
		t.Fatalf("existing hook result=%+v err=%v", result, err)
	}
	if calls != 1 {
		t.Fatalf("existing hook calls=%d want=1", calls)
	}
}

func TestRunDailyRejectsUnboundCustomClaudeBeforeMutation(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "unbound")
	result, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Goal: "goal", ClaudePath: missingClaudePath(t)})
	if err == nil || !strings.Contains(err.Error(), "refuses a custom or unbound Claude executable") {
		t.Fatalf("unbound custom Claude result=%+v err=%v", result, err)
	}
	if result.OnboardingApplied || result.SessionLaunches != 0 {
		t.Fatalf("unbound custom Claude mutated case: %+v", result)
	}
	if result.Failure == nil || result.Failure.Code != "claude-executable-unavailable" || result.Failure.Stage != "executable-resolution" || !result.Failure.Recoverable || result.Failure.MutationApplied || result.Failure.MutationBoundary != "none" {
		t.Fatalf("unbound custom Claude diagnosis = %+v", result.Failure)
	}
	if _, err := os.Lstat(caseRoot); !os.IsNotExist(err) {
		t.Fatalf("unbound custom Claude created case: %v", err)
	}
}

func TestRunDailyFreshGoalCommitsAndStartsBeforeClaudeUnavailable(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "fresh")
	result, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Goal: "inspect the supplied research target", ClaudePath: missingClaudePath(t), ExpectedClaudeExecutableSHA256: strings.Repeat("0", 64), ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher})
	if err == nil || !strings.Contains(err.Error(), "validate trusted Claude Code executable") {
		t.Fatalf("RunDaily result=%+v err=%v", result, err)
	}
	if !result.OnboardingApplied || result.Pack != defaults.DefaultPack || result.Lane != "feature-mission" || result.SessionLaunches != 0 {
		t.Fatalf("fresh daily result = %+v", result)
	}
	if result.Failure == nil || result.Failure.Code != "claude-executable-unavailable" || result.Failure.Stage != "executable-resolution" || !result.Failure.Recoverable || !result.Failure.MutationApplied || result.Failure.MutationBoundary != "durable-runtime-step-may-have-committed" {
		t.Fatalf("fresh daily executable diagnosis = %+v", result.Failure)
	}
	inspection, inspectErr := missionintent.Inspect(caseRoot)
	if inspectErr != nil || !inspection.Committed || inspection.Identity.Goal != "inspect the supplied research target" {
		t.Fatalf("fresh daily inspection=%+v err=%v", inspection, inspectErr)
	}
	board, boardErr := mission.ReadBoard(caseRoot)
	if boardErr != nil {
		t.Fatal(boardErr)
	}
	lane, ok := mission.LookupBoardLane(board.Lanes, result.Lane, false)
	if !ok || lane.ExecutorGeneration != 1 || lane.CurrentExecutor == "" {
		t.Fatalf("fresh daily lane = %+v", lane)
	}
}

func TestRunDailyResumeContinuesCommittedLaneBeforeClaudeUnavailable(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "resume")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "resume goal", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	result, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, ClaudePath: missingClaudePath(t), ExpectedClaudeExecutableSHA256: strings.Repeat("0", 64), ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher})
	if err == nil || !strings.Contains(err.Error(), "validate trusted Claude Code executable") {
		t.Fatalf("resume result=%+v err=%v", result, err)
	}
	if result.Mode != "resume" || result.Pack != inspection.Identity.Pack || result.Lane != inspection.Identity.InitialLane || result.SessionLaunches != 0 || !containsDailyStep(result.DriverSteps, "overview") || !containsDailyStep(result.DriverSteps, "start") {
		t.Fatalf("resume result=%+v", result)
	}
}

func TestRunDailyMultipleLanesRequiresChoiceWithoutLaunchOrWrite(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "multiple-lanes")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "multiple lane goal", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	if _, err := workstream.StartApply(sessionhostTestRepoRoot(t), caseRoot, inspection.Identity.Pack, workstream.StartOptions{
		Name: "login", Actor: "daily-test", Executor: "session-login", TakeoverReason: "daily multi-lane choice regression",
	}); err != nil {
		t.Fatal(err)
	}
	before := snapshotDailyCaseFiles(t, caseRoot)
	memberRunCalled := false
	result, err := RunDaily(context.Background(), DailyOptions{
		Target:                            caseRoot,
		ClaudePath:                        missingClaudePath(t),
		ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
		beforeMemberRun: func(string, string, string) error {
			memberRunCalled = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("multi-lane choice should stop without executable resolution: result=%+v err=%v", result, err)
	}
	if result.Action == nil || result.Action.Code != DailyActionBlocked || !result.Action.RequiresInput || len(result.Action.Choices) < 2 {
		t.Fatalf("multi-lane choice action=%+v result=%+v", result.Action, result)
	}
	ids := make([]string, 0, len(result.Action.Choices))
	seen := map[string]bool{}
	for _, choice := range result.Action.Choices {
		if choice.ID == "" || seen[choice.ID] {
			t.Fatalf("multi-lane choices contain duplicate or empty id: %+v", result.Action.Choices)
		}
		seen[choice.ID] = true
		ids = append(ids, choice.ID)
	}
	if !slices.Contains(ids, "feature-login") {
		t.Fatalf("multi-lane choices lost started lane: %+v", result.Action.Choices)
	}
	if memberRunCalled || result.SessionLaunches != 0 || len(result.HostRuns) != 0 {
		t.Fatalf("multi-lane choice crossed the zero-launch boundary: called=%t result=%+v", memberRunCalled, result)
	}
	assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))
}

func TestRunDailyCorrectionMultipleLanesRequiresChoiceWithoutWrite(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "multiple-lane-correction")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "multiple lane correction goal", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	if _, err := workstream.StartApply(sessionhostTestRepoRoot(t), caseRoot, inspection.Identity.Pack, workstream.StartOptions{
		Name: "login", Actor: "daily-test", Executor: "session-login", TakeoverReason: "daily correction multi-lane choice regression",
	}); err != nil {
		t.Fatal(err)
	}

	before := snapshotDailyCaseFiles(t, caseRoot)
	result, err := RunDaily(context.Background(), DailyOptions{
		Target:                            caseRoot,
		Correction:                        "apply this only after I select the exact lane",
		ClaudePath:                        missingClaudePath(t),
		ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
	})
	if err != nil {
		t.Fatalf("multi-lane correction should stop before lineage or executable resolution: result=%+v err=%v", result, err)
	}
	if result.Action == nil || result.Action.Code != DailyActionBlocked || !result.Action.RequiresInput || len(result.Action.Choices) < 2 {
		t.Fatalf("multi-lane correction choice action=%+v result=%+v", result.Action, result)
	}
	ids := make([]string, 0, len(result.Action.Choices))
	for _, choice := range result.Action.Choices {
		ids = append(ids, choice.ID)
	}
	if !slices.Contains(ids, "feature-login") || result.SessionLaunches != 0 || len(result.HostRuns) != 0 || len(result.DriverSteps) != 0 {
		t.Fatalf("multi-lane correction crossed the zero-write/launch boundary: choices=%+v result=%+v", result.Action.Choices, result)
	}
	assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))
}

func TestRunDailyResolvedLaneBindsMemberPreparation(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "resolved-lane-binding")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "resolved lane binding goal", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	lanes := make([]mission.BoardLane, 0, 1)
	for _, lane := range board.Lanes {
		if lane.ID == inspection.Identity.InitialLane {
			lanes = append(lanes, lane)
		}
	}
	if len(lanes) != 1 {
		t.Fatalf("initial lane missing from board: %+v", board.Lanes)
	}
	board.Lanes = lanes
	writeDailyTestBoard(t, caseRoot, board)
	if _, err := runPublicStatus(caseRoot, inspection.Identity.Pack, inspection.Identity.InitialLane); err != nil {
		global, globalErr := runPublicStatus(caseRoot, inspection.Identity.Pack, "")
		var request *publicDriverRequest
		if global.MissionControlRunbook != nil {
			request = global.MissionControlRunbook.CurrentDriverRequest
		}
		t.Fatalf("resolved lane fixture is not selectable: %v globalRequest=%+v globalErr=%v", err, request, globalErr)
	}

	preparedLane := ""
	result, err := RunDaily(context.Background(), DailyOptions{
		Target:                            caseRoot,
		ClaudePath:                        missingClaudePath(t),
		ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
		beforeMemberRun: func(_, _, lane string) error {
			preparedLane = lane
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "validate trusted Claude Code executable") {
		t.Fatalf("resolved lane binding action=%+v result=%+v err=%v", result.Action, result, err)
	}
	if result.Lane != inspection.Identity.InitialLane || preparedLane != result.Lane {
		t.Fatalf("resolved lane was not bound through member preparation: prepared=%q result=%+v", preparedLane, result)
	}
}

func TestRunDailyInvokesMemberPreparationBeforeExecutableResolution(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "member-preparation")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "member preparation goal", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	result, err := RunDaily(context.Background(), DailyOptions{
		Target:                            caseRoot,
		ClaudePath:                        missingClaudePath(t),
		ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
		beforeMemberRun: func(root, pack, lane string) error {
			called = true
			if root != caseRoot || pack != inspection.Identity.Pack || lane != inspection.Identity.InitialLane {
				t.Fatalf("member preparation binding drifted: root=%s pack=%s lane=%s", root, pack, lane)
			}
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "validate trusted Claude Code executable") {
		t.Fatalf("RunDaily result=%+v err=%v", result, err)
	}
	if !called || result.SessionLaunches != 0 {
		t.Fatalf("member preparation was not invoked before launch: called=%t result=%+v", called, result)
	}
}

func TestRunDailyRejectsConflictingCommittedGoalWithoutLaunchingClaude(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "goal-conflict")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	if _, err := applyDailyOnboarding(caseRoot, "immutable goal", "daily-test", &bootstrap); err != nil {
		t.Fatal(err)
	}
	result, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Goal: "different goal", ClaudePath: missingClaudePath(t), ExpectedClaudeExecutableSHA256: strings.Repeat("0", 64), ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher})
	if err == nil || !strings.Contains(err.Error(), "differs from the immutable committed mission intent") {
		t.Fatalf("conflicting goal result=%+v err=%v", result, err)
	}
	if result.SessionLaunches != 0 || len(result.HostRuns) != 0 {
		t.Fatalf("conflicting goal launched a session: %+v", result)
	}
}

func TestRunDailyAdoptsExistingAttachedCaseBeforeClaudeUnavailable(t *testing.T) {
	repo := sessionhostTestRepoRoot(t)
	caseRoot := provisionSessionhostAttachedCase(t, repo, "_template")
	result, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Goal: "analyze the attached case", ClaudePath: missingClaudePath(t), ExpectedClaudeExecutableSHA256: strings.Repeat("0", 64), ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher})
	if err == nil || !strings.Contains(err.Error(), "validate trusted Claude Code executable") {
		t.Fatalf("RunDaily result=%+v err=%v", result, err)
	}
	if !result.OnboardingApplied || result.Pack != "_template" || result.Lane != "feature-mission" || result.SessionLaunches != 0 {
		t.Fatalf("attached daily result = %+v", result)
	}
	inspection, inspectErr := missionintent.Inspect(caseRoot)
	if inspectErr != nil || !inspection.Committed || inspection.Recovery.Mode != "attached-adoption" {
		t.Fatalf("attached daily inspection=%+v err=%v", inspection, inspectErr)
	}
	content, readErr := os.ReadFile(filepath.Join(caseRoot, "case-local.txt"))
	if readErr != nil || string(content) != "preserve me\n" {
		t.Fatalf("ordinary attached content changed: %q err=%v", content, readErr)
	}
}

func TestRunDailyReturnsOrdinaryDirectoryAdoptionWithoutMutation(t *testing.T) {
	caseRoot := t.TempDir()
	path := filepath.Join(caseRoot, "user.txt")
	original := []byte("keep\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	readyCalls := 0
	result, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot,
		Goal:   "goal",
		onCaseReady: func(string) error {
			readyCalls++
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Blocked || result.Action == nil || result.Action.Code != DailyActionDirectoryAdoptionRequired || !result.Action.RequiresInput || len(result.Action.Choices) != 2 || result.Action.Choices[0].ID != "initialize-in-place" || result.Action.Choices[1].ID != "cancel" {
		t.Fatalf("ordinary directory action = %+v", result)
	}
	if readyCalls != 0 || result.SessionLaunches != 0 || len(result.HostRuns) != 0 {
		t.Fatalf("ordinary directory crossed the zero-write admission boundary: %+v calls=%d", result, readyCalls)
	}
	if content, readErr := os.ReadFile(path); readErr != nil || !bytes.Equal(content, original) {
		t.Fatalf("ordinary directory content changed: %q err=%v", content, readErr)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, filepath.FromSlash(missionintent.IntentRel))); !os.IsNotExist(err) {
		t.Fatalf("ordinary directory adoption preview wrote intent: %v", err)
	}
}

func TestClassifyDailyTargetRejectsPartialCaseState(t *testing.T) {
	for _, rel := range []string{".rekit/state.json", ".claude/skills/rekit/SKILL.md"} {
		t.Run(rel, func(t *testing.T) {
			caseRoot := t.TempDir()
			path := filepath.Join(caseRoot, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("partial\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			classified, err := classifyDailyTarget(caseRoot)
			if err == nil || classified.Kind != dailyTargetInvalid {
				t.Fatalf("partial target classification = %+v err=%v", classified, err)
			}
		})
	}
}

func TestClassifyDailyTargetCoversFiveAdmissionClasses(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	ordinary := t.TempDir()
	if target, err := classifyDailyTarget(missing); err != nil || target.Kind != dailyTargetMissing {
		t.Fatalf("missing target = %+v err=%v", target, err)
	}
	if target, err := classifyDailyTarget(ordinary); err != nil || target.Kind != dailyTargetOrdinary {
		t.Fatalf("ordinary target = %+v err=%v", target, err)
	}

	repoRoot, err := currentRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	attached := filepath.Join(t.TempDir(), "attached")
	preview, err := syncreview.InitPreview(repoRoot, attached, defaults.DefaultPack, syncreview.ApplyOptions{ProjectName: "attached", CreateLocalFiles: true, Command: "init"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := syncreview.Apply(repoRoot, attached, defaults.DefaultPack, syncreview.ApplyOptions{ProjectName: "attached", CreateLocalFiles: true, Command: "init", ExpectedPlanSHA256: preview.ExpectedPlanSHA256}); err != nil {
		t.Fatal(err)
	}
	if target, err := classifyDailyTarget(attached); err != nil || target.Kind != dailyTargetAttached {
		t.Fatalf("attached target = %+v err=%v", target, err)
	}
	bootstrap := DailyResult{CaseRoot: attached}
	if _, err := applyDailyOnboarding(attached, "classification goal", "daily-test", &bootstrap); err != nil {
		t.Fatal(err)
	}
	if target, err := classifyDailyTarget(attached); err != nil || target.Kind != dailyTargetMission {
		t.Fatalf("mission target = %+v err=%v", target, err)
	}

	writeText := func(path, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	invalid := filepath.Join(t.TempDir(), "invalid")
	writeText(filepath.Join(invalid, ".rekit", "instance.yml"), "templateRoot: "+repoRoot+"\ntemplatePack: missing-pack\nprojectName: invalid\nprojectRoot: "+invalid+"\n")
	if target, err := classifyDailyTarget(invalid); err == nil || target.Kind != dailyTargetInvalid {
		t.Fatalf("invalid target = %+v err=%v", target, err)
	}
}

func TestRunDailyRejectsInvalidBindingBeforeCaseReadyHook(t *testing.T) {
	repoRoot, err := currentRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	caseRoot := filepath.Join(t.TempDir(), "wrong-binding")
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	metadata := "templateRoot: " + repoRoot + "\ntemplatePack: missing-pack\nprojectName: invalid\nprojectRoot: " + caseRoot + "\n"
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "instance.yml"), []byte(metadata), 0o600); err != nil {
		t.Fatal(err)
	}
	readyCalls := 0
	result, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Goal: "goal", onCaseReady: func(string) error {
		readyCalls++
		return nil
	}})
	if err == nil || readyCalls != 0 || result.SessionLaunches != 0 {
		t.Fatalf("invalid binding crossed admission boundary: result=%+v calls=%d err=%v", result, readyCalls, err)
	}
}

func TestRunDailyReplaysCommittedClosedLaneBeforeOpenLaneProbe(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "committed-terminal")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "committed terminal replay goal", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	evidenceRef := filepath.ToSlash(filepath.Join(".rekit", "lanes", bootstrap.Lane, "workspace", "completion-evidence.md"))
	evidencePath := filepath.Join(caseRoot, filepath.FromSlash(evidenceRef))
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("reviewed terminal replay evidence\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	completeOpt := workstream.CompleteOptions{
		Selector:     bootstrap.Lane,
		Actor:        "daily-test",
		Reason:       "publish a valid fail-closed completion for terminal replay",
		EvidenceRefs: evidenceRef,
	}
	preview, err := workstream.CompletePreview(sessionhostTestRepoRoot(t), caseRoot, inspection.Identity.Pack, completeOpt)
	if err != nil || preview.Blocked || len(preview.CompletionPlanSHA256) != 64 {
		t.Fatalf("completion preview=%+v err=%v", preview, err)
	}
	completeOpt.ExpectedPreviewSHA256 = preview.CompletionPlanSHA256
	completed, err := workstream.CompleteApply(sessionhostTestRepoRoot(t), caseRoot, inspection.Identity.Pack, completeOpt)
	if err != nil || !completed.Applied || completed.Lane.Status != "closed" || completed.CompletionReceipt == nil {
		t.Fatalf("completion result=%+v err=%v", completed, err)
	}

	before := snapshotDailyCaseFiles(t, caseRoot)
	for _, replay := range []DailyOptions{
		{Target: caseRoot, Goal: inspection.Identity.Goal},
		{Target: caseRoot},
	} {
		memberRunCalled := false
		replay.beforeMemberRun = func(string, string, string) error {
			memberRunCalled = true
			return nil
		}
		result, err := RunDaily(context.Background(), replay)
		if err != nil {
			t.Fatalf("terminal replay result=%+v err=%v", result, err)
		}
		if !result.Replay || result.FinalState != "lane-closed" || result.Lane != bootstrap.Lane || result.ExecutorGeneration != completed.Lane.ExecutorGeneration || result.SessionLaunches != 0 || result.SessionCompletions != 0 || len(result.HostRuns) != 0 || memberRunCalled {
			t.Fatalf("terminal replay crossed its zero-launch boundary: called=%t result=%+v", memberRunCalled, result)
		}
		assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))
	}
}

func TestRunDailyCorrectionReopensCommittedClosedLaneBeforeNewMember(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "terminal-correction")
	actor := "daily-test"
	correction := "post-completion review found one more bounded verification step"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "terminal correction goal", actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	evidenceRef := filepath.ToSlash(filepath.Join(".rekit", "lanes", bootstrap.Lane, "workspace", "completion-evidence.md"))
	evidencePath := filepath.Join(caseRoot, filepath.FromSlash(evidenceRef))
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("reviewed completion evidence before terminal correction\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	completeOpt := workstream.CompleteOptions{Selector: bootstrap.Lane, Actor: actor, Reason: "publish terminal correction fixture", EvidenceRefs: evidenceRef}
	preview, err := workstream.CompletePreview(sessionhostTestRepoRoot(t), caseRoot, inspection.Identity.Pack, completeOpt)
	if err != nil || preview.Blocked || len(preview.CompletionPlanSHA256) != 64 {
		t.Fatalf("completion preview=%+v err=%v", preview, err)
	}
	completeOpt.ExpectedPreviewSHA256 = preview.CompletionPlanSHA256
	completed, err := workstream.CompleteApply(sessionhostTestRepoRoot(t), caseRoot, inspection.Identity.Pack, completeOpt)
	if err != nil || !completed.Applied || completed.Lane.Status != "closed" || completed.CompletionReceipt == nil {
		t.Fatalf("completion result=%+v err=%v", completed, err)
	}

	memberRunCalled := false
	result, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot, Correction: correction, Actor: actor,
		beforeMemberRun: func(_, _, _ string) error { memberRunCalled = true; return nil },
	})
	if err != nil {
		t.Fatalf("terminal correction result=%+v err=%v", result, err)
	}
	if memberRunCalled || result.Lane != bootstrap.Lane || result.FinalState != "terminal-correction-reopened" || result.Action == nil || result.Action.Code != DailyActionReadyToContinue || result.ReopenOperationID == "" || result.ReopenOperationCommit == nil || result.ReopenOperationCommit.State != "committed" || result.ReopenOperationCommit.Reason != correction || result.SessionLaunches != 0 || len(result.HostRuns) != 0 || !containsDailyStep(result.DriverSteps, "reopen") {
		t.Fatalf("terminal correction did not stop at committed review-first reopen: memberRunCalled=%t result=%+v", memberRunCalled, result)
	}
	if result.CurrentDriverRequest == nil || result.CurrentDriverRequestSHA256 == "" {
		t.Fatalf("terminal correction omitted the fresh typed continuation request: %+v", result)
	}
	if identity, err := mission.MissionCommanderDriverRequestSHA256(*result.CurrentDriverRequest); err != nil || identity != result.CurrentDriverRequestSHA256 {
		t.Fatalf("terminal correction current driver request identity=%s err=%v result=%+v", identity, err, result)
	}
	if !result.ReopenOperationCommit.NoAuthority || !result.ReopenOperationCommit.NoConfirmed || !result.ReopenOperationCommit.NoHeavyTool || !result.ReopenOperationCommit.NoAutoResume {
		t.Fatalf("terminal correction crossed reopen boundaries: %+v", result.ReopenOperationCommit)
	}
	lifecycle, err := lanecompletion.Inspect(caseRoot, bootstrap.Lane)
	if err != nil || lifecycle.State != lanecompletion.StateReopened || lifecycle.CurrentReopen == nil || lifecycle.CurrentReopen.OperationID != result.ReopenOperationID {
		t.Fatalf("terminal correction lifecycle=%+v err=%v", lifecycle, err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	lane, ok := mission.LookupBoardLane(board.Lanes, bootstrap.Lane, false)
	if !ok || lane.Status != "open" || lane.ExecutorGeneration != completed.Lane.ExecutorGeneration+1 || lane.CurrentExecutor != "" {
		t.Fatalf("terminal correction did not fence the completed owner before fresh execution: %+v", lane)
	}
}

func TestRunDailyCommittedTerminalCorrectionDoesNotClaimDifferentExplicitLane(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "terminal-correction-explicit-lane")
	actor := "daily-test"
	correction := "reuse this bounded correction on the selected lane"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "explicit terminal correction goal", actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	if _, err := workstream.StartApply(sessionhostTestRepoRoot(t), caseRoot, inspection.Identity.Pack, workstream.StartOptions{
		Name: "login", Actor: actor, Executor: "session-login", TakeoverReason: "explicit terminal correction regression",
	}); err != nil {
		t.Fatal(err)
	}
	closed := []string{bootstrap.Lane, "feature-login"}
	for _, lane := range closed {
		evidenceRef := filepath.ToSlash(filepath.Join(".rekit", "lanes", lane, "workspace", "completion-evidence.md"))
		evidencePath := filepath.Join(caseRoot, filepath.FromSlash(evidenceRef))
		if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(evidencePath, []byte("reviewed completion evidence for "+lane+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		completeOpt := workstream.CompleteOptions{Selector: lane, Actor: actor, Reason: "close lane for explicit terminal correction", EvidenceRefs: evidenceRef}
		preview, err := workstream.CompletePreview(sessionhostTestRepoRoot(t), caseRoot, inspection.Identity.Pack, completeOpt)
		if err != nil {
			t.Fatal(err)
		}
		completeOpt.ExpectedPreviewSHA256 = preview.CompletionPlanSHA256
		if completed, err := workstream.CompleteApply(sessionhostTestRepoRoot(t), caseRoot, inspection.Identity.Pack, completeOpt); err != nil || !completed.Applied {
			t.Fatalf("complete %s: result=%+v err=%v", lane, completed, err)
		}
	}

	first, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Correction: correction, SelectedLane: bootstrap.Lane, Actor: actor})
	if err != nil {
		t.Fatalf("first terminal correction: result=%+v err=%v", first, err)
	}
	if first.Lane != bootstrap.Lane || first.FinalState != "terminal-correction-reopened" || first.ReopenOperationID == "" || first.SessionLaunches != 0 || len(first.HostRuns) != 0 {
		t.Fatalf("first explicit terminal correction crossed its boundary: %+v", first)
	}

	second, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Correction: correction, SelectedLane: "feature-login", Actor: actor})
	if err != nil {
		t.Fatalf("second terminal correction: result=%+v err=%v", second, err)
	}
	if second.Lane != "feature-login" || second.FinalState != "terminal-correction-reopened" || second.ReopenOperationID == "" || second.ReopenOperationID == first.ReopenOperationID || second.Replay || second.SessionLaunches != 0 || len(second.HostRuns) != 0 {
		t.Fatalf("different explicit lane did not create its own terminal correction: first=%+v second=%+v", first, second)
	}
	operations, err := lanecompletion.InspectOperations(caseRoot)
	if err != nil || operations.LatestSequence != 2 || len(operations.Commits) != 2 {
		t.Fatalf("different explicit lane operation history=%+v err=%v", operations, err)
	}

	beforeReplay := snapshotDailyCaseFiles(t, caseRoot)
	replayed, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Correction: correction, SelectedLane: "feature-login", Actor: actor})
	if err != nil {
		t.Fatalf("same explicit lane replay: result=%+v err=%v", replayed, err)
	}
	if !replayed.Replay || replayed.Lane != "feature-login" || replayed.ReopenOperationID != second.ReopenOperationID || replayed.SessionLaunches != 0 || len(replayed.HostRuns) != 0 {
		t.Fatalf("same explicit lane did not replay its exact committed operation: %+v", replayed)
	}
	assertDailyCaseFilesEqual(t, beforeReplay, snapshotDailyCaseFiles(t, caseRoot))

	beforeHistoricalReplay := snapshotDailyCaseFiles(t, caseRoot)
	historical, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Correction: correction, SelectedLane: bootstrap.Lane, Actor: actor})
	if err != nil {
		t.Fatalf("historical explicit lane replay: result=%+v err=%v", historical, err)
	}
	if !historical.Replay || historical.Lane != bootstrap.Lane || historical.ReopenOperationID != first.ReopenOperationID || historical.SessionLaunches != 0 || len(historical.HostRuns) != 0 {
		t.Fatalf("historical explicit lane did not replay operation 1: %+v", historical)
	}
	assertDailyCaseFilesEqual(t, beforeHistoricalReplay, snapshotDailyCaseFiles(t, caseRoot))
}

func TestRunDailyTerminalCorrectionMultipleClosedLanesRequiresChoiceWithoutWrite(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "multiple-terminal-correction")
	actor := "daily-test"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "multiple terminal correction goal", actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	if _, err := workstream.StartApply(sessionhostTestRepoRoot(t), caseRoot, inspection.Identity.Pack, workstream.StartOptions{
		Name: "login", Actor: actor, Executor: "session-login", TakeoverReason: "multiple terminal correction choice regression",
	}); err != nil {
		t.Fatal(err)
	}
	closed := []string{bootstrap.Lane, "feature-login"}
	for _, lane := range closed {
		evidenceRef := filepath.ToSlash(filepath.Join(".rekit", "lanes", lane, "workspace", "completion-evidence.md"))
		evidencePath := filepath.Join(caseRoot, filepath.FromSlash(evidenceRef))
		if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(evidencePath, []byte("reviewed completion evidence for "+lane+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		completeOpt := workstream.CompleteOptions{Selector: lane, Actor: actor, Reason: "close lane for terminal correction choice", EvidenceRefs: evidenceRef}
		preview, err := workstream.CompletePreview(sessionhostTestRepoRoot(t), caseRoot, inspection.Identity.Pack, completeOpt)
		if err != nil {
			t.Fatal(err)
		}
		completeOpt.ExpectedPreviewSHA256 = preview.CompletionPlanSHA256
		if completed, err := workstream.CompleteApply(sessionhostTestRepoRoot(t), caseRoot, inspection.Identity.Pack, completeOpt); err != nil || !completed.Applied {
			t.Fatalf("complete %s: result=%+v err=%v", lane, completed, err)
		}
	}

	before := snapshotDailyCaseFiles(t, caseRoot)
	result, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Correction: "reopen only the lane I select", Actor: actor})
	if err != nil {
		t.Fatalf("multiple terminal correction choices: result=%+v err=%v", result, err)
	}
	if result.Action == nil || result.Action.Code != DailyActionBlocked || !result.Action.RequiresInput || len(result.Action.Choices) != len(closed) || result.SessionLaunches != 0 || len(result.HostRuns) != 0 || len(result.DriverSteps) != 0 {
		t.Fatalf("multiple terminal correction did not stop at typed choice: %+v", result)
	}
	ids := make([]string, 0, len(result.Action.Choices))
	for _, choice := range result.Action.Choices {
		ids = append(ids, choice.ID)
	}
	for _, lane := range closed {
		if !slices.Contains(ids, lane) {
			t.Fatalf("multiple terminal correction omitted %s: %+v", lane, result.Action.Choices)
		}
	}
	assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))
}

func TestRunDailyTerminalCorrectionRecoversExactPendingReopen(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "terminal-correction-recovery")
	actor := "daily-test"
	correction := "resume the exact terminal correction after interruption"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "terminal correction recovery goal", actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	evidenceRef := filepath.ToSlash(filepath.Join(".rekit", "lanes", bootstrap.Lane, "workspace", "completion-evidence.md"))
	evidencePath := filepath.Join(caseRoot, filepath.FromSlash(evidenceRef))
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("reviewed completion evidence before recoverable reopen\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	completeOpt := workstream.CompleteOptions{Selector: bootstrap.Lane, Actor: actor, Reason: "publish terminal correction recovery fixture", EvidenceRefs: evidenceRef}
	preview, err := workstream.CompletePreview(sessionhostTestRepoRoot(t), caseRoot, inspection.Identity.Pack, completeOpt)
	if err != nil {
		t.Fatal(err)
	}
	completeOpt.ExpectedPreviewSHA256 = preview.CompletionPlanSHA256
	if completed, err := workstream.CompleteApply(sessionhostTestRepoRoot(t), caseRoot, inspection.Identity.Pack, completeOpt); err != nil || !completed.Applied {
		t.Fatalf("complete recovery fixture: result=%+v err=%v", completed, err)
	}

	restore := workstream.SetReopenAfterOperationIntentHookForTest(func() error { return errors.New("stop after daily reopen intent") })
	interrupted, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Correction: correction, Actor: actor})
	restore()
	if err == nil || !strings.Contains(err.Error(), "stop after daily reopen intent") {
		t.Fatalf("terminal correction interruption result=%+v err=%v", interrupted, err)
	}
	if !containsDailyStep(interrupted.DriverSteps, "reopen") || interrupted.Failure == nil || !interrupted.Failure.MutationApplied || interrupted.Failure.MutationBoundary != "durable-runtime-step-may-have-committed" {
		t.Fatalf("terminal correction interruption omitted its durable mutation boundary: %+v", interrupted)
	}
	operations, err := lanecompletion.InspectOperations(caseRoot)
	if err != nil || !operations.Pending {
		t.Fatalf("interrupted correction did not leave an exact pending operation: %+v err=%v", operations, err)
	}
	beforeMismatch := snapshotDailyCaseFiles(t, caseRoot)
	mismatch, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Correction: "a different terminal correction", Actor: actor})
	if err == nil || !strings.Contains(err.Error(), "exact original correction") {
		t.Fatalf("mismatched pending recovery result=%+v err=%v", mismatch, err)
	}
	assertDailyCaseFilesEqual(t, beforeMismatch, snapshotDailyCaseFiles(t, caseRoot))

	recovered, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Correction: correction, Actor: actor})
	if err != nil {
		t.Fatalf("recover pending terminal correction: result=%+v err=%v", recovered, err)
	}
	if recovered.FinalState != "terminal-correction-reopened" || recovered.Action == nil || recovered.Action.Code != DailyActionReadyToContinue || recovered.ReopenOperationCommit == nil || recovered.ReopenOperationCommit.State != "committed" || recovered.SessionLaunches != 0 || len(recovered.HostRuns) != 0 || !containsDailyStep(recovered.DriverSteps, "reopen") {
		t.Fatalf("pending terminal correction recovery crossed its boundary: %+v", recovered)
	}
	operations, err = lanecompletion.InspectOperations(caseRoot)
	if err != nil || operations.Pending || operations.LatestSequence != 1 {
		t.Fatalf("pending terminal correction did not commit exactly once: %+v err=%v", operations, err)
	}
	replayed, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Correction: correction, Actor: actor})
	if err != nil || !replayed.Replay || replayed.ReopenOperationID != recovered.ReopenOperationID || replayed.FinalState != "terminal-correction-reopened" || replayed.SessionLaunches != 0 {
		t.Fatalf("committed terminal correction did not replay exactly: result=%+v err=%v", replayed, err)
	}
	operations, err = lanecompletion.InspectOperations(caseRoot)
	if err != nil || operations.LatestSequence != 1 || len(operations.Commits) != 1 {
		t.Fatalf("committed replay changed operation history: %+v err=%v", operations, err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	for index := range board.Lanes {
		if board.Lanes[index].ID == bootstrap.Lane {
			board.Lanes[index].CurrentExecutor = "post-reopen-session"
		}
	}
	writeDailyTestBoard(t, caseRoot, board)
	stale, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Correction: correction, Actor: actor})
	if err == nil || !strings.Contains(err.Error(), "current board lane") {
		t.Fatalf("advanced committed correction replay result=%+v err=%v", stale, err)
	}
	if len(stale.DriverSteps) != 0 || stale.Failure == nil || stale.Failure.MutationApplied {
		t.Fatalf("advanced committed correction replay was reported as a new mutation: %+v", stale)
	}
}

func TestRunDailyTerminalCorrectionRejectsStaleCompletionAfterPreview(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "terminal-correction-stale")
	actor := "daily-test"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "terminal correction stale goal", actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	evidenceRef := filepath.ToSlash(filepath.Join(".rekit", "lanes", bootstrap.Lane, "workspace", "completion-evidence.md"))
	evidencePath := filepath.Join(caseRoot, filepath.FromSlash(evidenceRef))
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("completion evidence before stale preview\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	completeOpt := workstream.CompleteOptions{Selector: bootstrap.Lane, Actor: actor, Reason: "publish stale terminal correction fixture", EvidenceRefs: evidenceRef}
	preview, err := workstream.CompletePreview(sessionhostTestRepoRoot(t), caseRoot, inspection.Identity.Pack, completeOpt)
	if err != nil {
		t.Fatal(err)
	}
	completeOpt.ExpectedPreviewSHA256 = preview.CompletionPlanSHA256
	if completed, err := workstream.CompleteApply(sessionhostTestRepoRoot(t), caseRoot, inspection.Identity.Pack, completeOpt); err != nil || !completed.Applied {
		t.Fatalf("complete stale fixture: result=%+v err=%v", completed, err)
	}

	previous := dailyTerminalCorrectionAfterPreviewHook
	dailyTerminalCorrectionAfterPreviewHook = func() error {
		board, err := mission.ReadBoard(caseRoot)
		if err != nil {
			return err
		}
		board.UpdatedAt = "2026-08-12T23:59:59Z"
		writeDailyTestBoard(t, caseRoot, board)
		return nil
	}
	t.Cleanup(func() { dailyTerminalCorrectionAfterPreviewHook = previous })
	result, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Correction: "reopen only if the reviewed terminal state stays current", Actor: actor})
	if err == nil || (!strings.Contains(err.Error(), "changed") && !strings.Contains(err.Error(), "mismatch")) {
		t.Fatalf("stale terminal correction result=%+v err=%v", result, err)
	}
	if result.ReopenOperationID != "" || result.ReopenOperationCommit != nil || containsDailyStep(result.DriverSteps, "reopen") || result.SessionLaunches != 0 {
		t.Fatalf("stale terminal correction crossed zero-commit boundary: %+v", result)
	}
	lifecycle, err := lanecompletion.Inspect(caseRoot, bootstrap.Lane)
	if err != nil || lifecycle.State != lanecompletion.StateComplete || lifecycle.CurrentCompletion == nil {
		t.Fatalf("stale terminal correction changed lifecycle: %+v err=%v", lifecycle, err)
	}
}

func TestRunDailyRefusesPendingCompletionAsTerminalReplay(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "pending-completion")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "pending completion goal", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	boardPath := filepath.Join(caseRoot, ".rekit", "board.json")
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	lane, ok := mission.LookupBoardLane(board.Lanes, bootstrap.Lane, false)
	if !ok {
		t.Fatalf("started lane missing from board: %+v", board.Lanes)
	}
	for index := range board.Lanes {
		if board.Lanes[index].ID == bootstrap.Lane {
			board.Lanes[index].Status = "closed"
		}
	}
	boardBytes, err := json.MarshalIndent(board, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(boardPath, append(boardBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	intent := lanecompletion.CompletionIntent{
		SchemaVersion:      1,
		Kind:               "lane-completion-intent",
		Sequence:           1,
		Lane:               bootstrap.Lane,
		Label:              "mission",
		PreviousStatus:     "active",
		Actor:              "daily-test",
		Reason:             "simulate interrupted publication without LLM output",
		EvidenceRefs:       []string{},
		Evidence:           []lanecompletion.Evidence{},
		CurrentExecutor:    lane.CurrentExecutor,
		ExecutorGeneration: lane.ExecutorGeneration,
		CreatedAt:          "2026-08-07T01:02:03Z",
		EventID:            "lane-completed-pending-test",
		PreviewSHA256:      strings.Repeat("a", 64),
		NoAuthority:        true,
		NoConfirmed:        true,
		NoHeavyTool:        true,
	}
	intentBytes, err := json.MarshalIndent(intent, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	intentPath := lanecompletion.IntentPath(caseRoot, bootstrap.Lane, 1, "complete")
	if err := os.WriteFile(intentPath, append(intentBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := RunDaily(context.Background(), DailyOptions{
		Target:                            caseRoot,
		Goal:                              inspection.Identity.Goal,
		ClaudePath:                        missingClaudePath(t),
		ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
	})
	if err == nil || (!strings.Contains(err.Error(), "pending lane completion publication") && !strings.Contains(err.Error(), "completion publication is incomplete")) {
		t.Fatalf("pending completion result=%+v err=%v", result, err)
	}
	if result.Replay || result.SessionLaunches != 0 || result.SessionCompletions != 0 {
		t.Fatalf("pending completion was treated as terminal replay: %+v", result)
	}
	if _, err := os.Lstat(lanecompletion.ReceiptPath(caseRoot, bootstrap.Lane, 1, "complete")); !os.IsNotExist(err) {
		t.Fatalf("daily replay wrote pending completion commit: %v", err)
	}
}

func TestRunDailyRefusesArchivedLaneWithoutDurableArchiveTransition(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "archived-lane")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "archived lane goal", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	for index := range board.Lanes {
		if board.Lanes[index].ID == bootstrap.Lane {
			board.Lanes[index].Status = "archived"
		}
	}
	boardBytes, err := json.MarshalIndent(board, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), append(boardBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Goal: inspection.Identity.Goal})
	if err == nil || !strings.Contains(err.Error(), "no durable archive transition is supported") {
		t.Fatalf("archived lane result=%+v err=%v", result, err)
	}
	if result.Replay || result.SessionLaunches != 0 {
		t.Fatalf("archived lane was treated as replay: %+v", result)
	}
}

func TestRunDailyCorrectionRefusesPendingCompletionAsTerminalReplay(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "pending-correction-completion")
	actor := "daily-test"
	correction := "retry the durable correction without launching another member"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "pending correction completion goal", actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	lane, ok := mission.LookupBoardLane(board.Lanes, bootstrap.Lane, false)
	if !ok {
		t.Fatalf("started lane missing from board: %+v", board.Lanes)
	}
	eventID := dailyCorrectionEventID(dailyCorrectionScope(caseRoot, inspection), bootstrap.Lane, correction)
	correctionEvent := map[string]any{
		"schemaVersion": 1, "eventId": eventID, "kind": "intervention", "lane": bootstrap.Lane,
		"subject": "daily human correction", "summary": correction, "actor": actor,
		"action": "override", "status": "open", "target": missionintent.MissionIntentRel,
	}
	resolutionTime := "2026-08-07T01:02:03Z"
	resolutionEvent := map[string]any{
		"schemaVersion": 1, "eventId": "daily-correction-resolution-pending-test", "kind": "intervention", "lane": bootstrap.Lane,
		"subject": "reconcile pending correction", "summary": "deterministic lifecycle fixture without LLM output",
		"action": "reconcile", "status": "resolved", "resolvesEventId": eventID,
		"target": missionintent.MissionIntentRel, "actor": actor, "executor": lane.CurrentExecutor,
		"reason": "simulate interrupted completion publication", "time": resolutionTime,
	}
	_, interventionPath, err := mission.FactPath(caseRoot, "intervention")
	if err != nil {
		t.Fatal(err)
	}
	if err := mission.AppendJSONLine(interventionPath, correctionEvent); err != nil {
		t.Fatal(err)
	}
	if err := mission.AppendJSONLine(interventionPath, resolutionEvent); err != nil {
		t.Fatal(err)
	}
	for index := range board.Lanes {
		if board.Lanes[index].ID == bootstrap.Lane {
			board.Lanes[index].Status = "closed"
			board.Lanes[index].LastReconciledIntervention = eventID
			board.Lanes[index].LastReconcileAt = resolutionTime
		}
	}
	boardBytes, err := json.MarshalIndent(board, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), append(boardBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	intent := lanecompletion.CompletionIntent{
		SchemaVersion: 1, Kind: "lane-completion-intent", Sequence: 1, Lane: bootstrap.Lane, Label: "mission",
		PreviousStatus: "active", Actor: actor, Reason: "simulate interrupted completion publication without LLM output",
		EvidenceRefs: []string{}, Evidence: []lanecompletion.Evidence{}, CurrentExecutor: lane.CurrentExecutor,
		ExecutorGeneration: lane.ExecutorGeneration, CreatedAt: resolutionTime, EventID: "lane-completed-pending-correction-test",
		PreviewSHA256: strings.Repeat("a", 64), NoAuthority: true, NoConfirmed: true, NoHeavyTool: true,
	}
	intentBytes, err := json.MarshalIndent(intent, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lanecompletion.IntentPath(caseRoot, bootstrap.Lane, 1, "complete"), append(intentBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot, Correction: correction, SelectedLane: inspection.Identity.InitialLane, Actor: actor, ClaudePath: missingClaudePath(t),
		ExpectedClaudeExecutableSHA256: strings.Repeat("0", 64), ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
	})
	if err == nil || (!strings.Contains(err.Error(), "pending lane completion publication") && !strings.Contains(err.Error(), "completion publication is incomplete")) {
		t.Fatalf("pending correction completion result=%+v err=%v", result, err)
	}
	if result.Replay || result.SessionLaunches != 0 || result.SessionCompletions != 0 {
		t.Fatalf("pending correction completion was treated as terminal replay: %+v", result)
	}
}

func TestRunDailyExplicitReviewerRejectionReplayPreservesLaneWithoutMutation(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "explicit-reviewer-rejection")
	actor := "daily-test"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "explicit reviewer rejection replay goal", actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	lane := inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap, lane); err != nil {
		t.Fatal(err)
	}
	manifestRef := writeDailyReviewerRejectionFixture(t, caseRoot, inspection.Identity.Pack, lane, actor)
	if _, rejected, err := workstream.CurrentMemberManifestReviewerRejection(caseRoot, lane, manifestRef); err != nil || !rejected {
		t.Fatalf("reviewer rejection fixture is not canonical: rejected=%t err=%v", rejected, err)
	}

	before := snapshotDailyCaseFiles(t, caseRoot)
	result, err := RunDaily(context.Background(), DailyOptions{
		Target:                            caseRoot,
		SelectedLane:                      lane,
		ClaudePath:                        missingClaudePath(t),
		ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
		beforeMemberRun: func(_, _, _ string) error {
			t.Fatal("reviewer rejection replay reached member execution")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("explicit reviewer rejection replay: result=%+v err=%v", result, err)
	}
	if result.Lane != lane || result.FinalState != "reviewer-rejected-awaiting-correction" || !result.Blocked || !result.Replay || result.Action == nil || result.Action.Code != DailyActionWaitingForCorrection || result.SessionLaunches != 0 || result.SessionCompletions != 0 || result.Replacements != 0 || len(result.HostRuns) != 0 || len(result.DriverSteps) != 0 {
		t.Fatalf("explicit reviewer rejection replay lost its canonical projection: %+v", result)
	}
	assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))
}

func writeDailyReviewerRejectionFixture(t *testing.T, caseRoot, pack, lane, actor string) string {
	t.Helper()
	status, err := runPublicStatus(caseRoot, pack, lane)
	if err != nil || status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentDriverRequest == nil {
		t.Fatalf("read member dispatch request: status=%+v err=%v", status, err)
	}
	requestSHA256, err := mission.MissionCommanderDriverRequestSHA256(*status.MissionControlRunbook.CurrentDriverRequest)
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err := memberexecution.PreviewDispatch(memberexecution.DispatchOptions{CaseRoot: caseRoot, Pack: pack, Lane: lane, RequestSHA256: requestSHA256, CreatedAt: "2026-08-12T01:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := memberexecution.Apply(dispatch, dispatch.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	accepted, err := memberexecution.PreviewObservation(memberexecution.ObservationOptions{CaseRoot: caseRoot, Pack: pack, Lane: lane, AttemptID: dispatch.AttemptID, Outcome: "accepted", Actor: actor, ObservedAt: "2026-08-12T01:01:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := memberexecution.Apply(accepted, accepted.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}

	output := []byte("bounded member result for reviewer rejection\n")
	manifest := memberexecution.ResultManifest{
		SchemaVersion: 1, Kind: memberexecution.KindManifest, AttemptID: dispatch.AttemptID, Owner: dispatch.Owner,
		Summary: "bounded member result", Outputs: []memberexecution.Output{{Path: "review-items.txt", SHA256: bytesSHA256(output), Bytes: int64(len(output))}},
		ReviewerItemsPath: "review-items.txt", NoAuthority: true, NoConfirmed: true, NoHeavyTool: true,
	}
	manifestData, err := memberexecution.MarshalResultManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dispatch.Inspection.OutputsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dispatch.Inspection.OutputsRoot, "review-items.txt"), output, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dispatch.Inspection.ManifestPath, manifestData, 0o600); err != nil {
		t.Fatal(err)
	}
	returned, err := memberexecution.PreviewObservation(memberexecution.ObservationOptions{
		CaseRoot: caseRoot, Pack: pack, Lane: lane, AttemptID: dispatch.AttemptID, Outcome: "returned", Actor: actor, ObservedAt: "2026-08-12T01:02:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := memberexecution.Apply(returned, returned.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	latest, found, err := memberexecution.Latest(caseRoot, lane)
	if err != nil || !found || latest.State != "intake-ready" || latest.Manifest == nil {
		t.Fatalf("member rejection fixture is not intake-ready: latest=%+v found=%t err=%v", latest, found, err)
	}
	manifestRef := relativeLiveAcceptancePath(caseRoot, latest.ManifestPath)

	planCommand := "/rekit plan-subagents -Target " + quoteDailyCommandArg(caseRoot) + " -Pack " + pack + " -TaskType feature-analysis -Items " + quoteDailyCommandArg(manifestRef) + " -Lane " + lane + " -Format json"
	var reviewerPlan struct {
		PacketID      string `json:"packetId"`
		PacketPath    string `json:"packetPath"`
		TargetLane    string `json:"targetLane"`
		ShardHandoffs []struct {
			ShardID string `json:"shardId"`
		} `json:"shardHandoffs"`
	}
	if err := runPublicCommandJSON(planCommand, &reviewerPlan); err != nil {
		t.Fatal(err)
	}
	if reviewerPlan.PacketPath == "" || reviewerPlan.TargetLane != lane || len(reviewerPlan.ShardHandoffs) != 1 {
		t.Fatalf("reviewer rejection plan identity is incomplete: %+v", reviewerPlan)
	}

	hostOpt := Options{Target: caseRoot, Pack: pack, SelectedLane: lane, Actor: actor}
	spawn, err := runCurrentStep(hostOpt, []string{"-ReviewerHarness", "daily-test-harness", "-ReviewerSession", "daily-test-reviewer", "-Actor", actor}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyCurrentStep(hostOpt, spawn, []string{"-ReviewerHarness", "daily-test-harness", "-ReviewerSession", "daily-test-reviewer", "-Actor", actor}); err != nil {
		t.Fatal(err)
	}
	status, err = runPublicStatus(caseRoot, pack, lane)
	if err != nil {
		t.Fatal(err)
	}
	operator := dailyReviewerOperatorPackage(t, caseRoot, pack, lane)
	if operator.PacketID == "" || operator.RouteID == "" || operator.TargetLane != lane || operator.Current == nil || operator.Current.ShardID != reviewerPlan.ShardHandoffs[0].ShardID {
		t.Fatalf("reviewer rejection operator identity is incomplete: %+v", operator)
	}
	resultData, err := json.Marshal(map[string]any{
		"packetId": operator.PacketID, "routeId": operator.RouteID, "shardId": reviewerPlan.ShardHandoffs[0].ShardID, "items": []string{manifestRef},
		"reviewerSession": "daily-test-reviewer", "decision": "reject", "confidence": "high", "summary": "bounded member result requires correction", "evidenceRefs": []string{manifestRef}, "risks": []string{"missing bounded acceptance detail"}, "conflicts": []string{}, "recommendedVerdict": "rejected",
		"routeOutput": map[string]any{"item": manifestRef, "decision": "reject", "confidence": "high", "evidence": manifestRef, "risk": "missing bounded acceptance detail", "next_action": "main-agent-writeback", "tier_used": "light", "tool_scope": "read-only", "feature": "mission", "request_id": "n/a", "candidate_path": "n/a", "defer_reason": "n/a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	resultSource := filepath.Join(caseRoot, "workspace", "daily-reviewer-rejection.json")
	if err := os.MkdirAll(filepath.Dir(resultSource), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resultSource, resultData, 0o600); err != nil {
		t.Fatal(err)
	}
	input, err := runCurrentStep(hostOpt, []string{"-ReviewerResultInputSourcePath", resultSource, "-Actor", actor}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyCurrentStep(hostOpt, input, []string{"-ReviewerResultInputSourcePath", resultSource, "-Actor", actor}); err != nil {
		t.Fatal(err)
	}
	for _, stepID := range []string{"record-completion", "source-capture", "stage-candidate", "collect-result", "intake-results"} {
		operator = dailyReviewerOperatorPackage(t, caseRoot, pack, lane)
		if operator.CurrentRunLoopStepID != stepID {
			t.Fatalf("reviewer rejection pipeline step=%s want=%s operator=%+v", operator.CurrentRunLoopStepID, stepID, operator)
		}
		step, err := runCurrentStep(hostOpt, []string{"-Actor", actor}, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := applyCurrentStep(hostOpt, step, []string{"-Actor", actor}); err != nil {
			t.Fatal(err)
		}
	}
	return manifestRef
}

func dailyReviewerOperatorPackage(t *testing.T, caseRoot, pack, lane string) *workstream.ReviewerDispatchOperatorPackage {
	t.Helper()
	var status struct {
		CaseMission *struct {
			ReviewerDispatchIntakeSummary workstream.ReviewerDispatchIntakeSummary `json:"reviewerDispatchIntakeSummary"`
		} `json:"caseMission"`
	}
	if err := runPublicCLI([]string{"-Command", "status", "-Target", caseRoot, "-Pack", pack, "-Lane", lane, "-Format", "json"}, &status); err != nil {
		t.Fatal(err)
	}
	if status.CaseMission == nil || status.CaseMission.ReviewerDispatchIntakeSummary.OperatorPackage == nil {
		t.Fatalf("reviewer operator package is missing: %+v", status)
	}
	return status.CaseMission.ReviewerDispatchIntakeSummary.OperatorPackage
}

func runPublicCommandJSON(command string, target any) error {
	args, err := cli.SplitPublicCommand(command)
	if err != nil {
		return err
	}
	return runPublicCLI(args, target)
}

func quoteDailyCommandArg(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func TestFinishDailyCompletionPreservesReviewerRejectionState(t *testing.T) {
	result := DailyResult{FinalState: "reviewer-rejected-awaiting-correction"}
	if err := finishDailyCompletion(t.TempDir(), liveAcceptancePack, &result); err != nil {
		t.Fatal(err)
	}
	if result.FinalState != "reviewer-rejected-awaiting-correction" || !result.Blocked {
		t.Fatalf("typed reviewer rejection was not preserved: %+v", result)
	}
}

func TestExistingDailyCorrectionByIDDistinguishesRepeatedTextAcrossRejections(t *testing.T) {
	caseRoot := t.TempDir()
	lane := "feature-mission"
	actor := "daily-test"
	correction := "apply the same correction text to the current rejection"
	firstID := "daily-correction-first-rejection"
	secondID := "daily-correction-second-rejection"
	_, interventionPath, err := mission.FactPath(caseRoot, "intervention")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(interventionPath), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, eventID := range []string{firstID, secondID} {
		event := map[string]any{
			"schemaVersion": 1, "eventId": eventID, "kind": "intervention", "lane": lane,
			"subject": "daily human correction", "summary": correction, "actor": actor,
			"action": "override", "status": "open",
			"reviewerVerificationEventId": eventID + "-verification",
			"reviewerDecisionEventId":     eventID + "-decision",
		}
		if err := mission.AppendJSONLine(interventionPath, event); err != nil {
			t.Fatal(err)
		}
	}
	first, err := existingDailyCorrectionByID(caseRoot, firstID, lane, correction, actor)
	if err != nil || mission.Value(first, "eventId") != firstID {
		t.Fatalf("first correction event=%v err=%v", first, err)
	}
	second, err := existingDailyCorrectionByID(caseRoot, secondID, lane, correction, actor)
	if err != nil || mission.Value(second, "eventId") != secondID {
		t.Fatalf("second correction event=%v err=%v", second, err)
	}
}

func TestDailyCorrectionLaneFindsResolvedRequestOnOpenReplacementGeneration(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "correction-replacement")
	actor := "daily-source-actor"
	goal := "inspect the supplied research target"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, goal, actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}

	correction := "apply the rejection-bound correction"
	eventID := "daily-correction-rejection-bound"
	_, interventionPath, err := mission.FactPath(caseRoot, "intervention")
	if err != nil {
		t.Fatal(err)
	}
	event := map[string]any{
		"schemaVersion": 1, "eventId": eventID, "kind": "intervention", "lane": inspection.Identity.InitialLane,
		"subject": "daily human correction", "summary": correction, "actor": actor, "action": "override", "status": "open",
		"reviewerVerificationEventId": "verification", "reviewerDecisionEventId": "decision",
	}
	if err := mission.AppendJSONLine(interventionPath, event); err != nil {
		t.Fatal(err)
	}

	lane, boardLane, existing, err := dailyCorrectionLane(caseRoot, inspection, correction, actor)
	if err != nil || lane != inspection.Identity.InitialLane || boardLane.ID != lane || mission.Value(existing, "eventId") != eventID {
		t.Fatalf("open replacement correction lookup: lane=%s board=%+v existing=%v err=%v", lane, boardLane, existing, err)
	}
}

func TestDailyCorrectionLaneUsesLatestReconciledIdentityForRepeatedText(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "repeated-correction-recovery")
	actor := "daily-source-actor"
	goal := "inspect the supplied research target"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, goal, actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}

	correction := "apply the repeated rejection-bound correction"
	firstID := "daily-correction-generation-1"
	secondID := "daily-correction-generation-2"
	firstTime := "2026-08-07T01:02:03Z"
	secondTime := "2026-08-07T02:03:04Z"
	_, interventionPath, err := mission.FactPath(caseRoot, "intervention")
	if err != nil {
		t.Fatal(err)
	}
	for generation, eventID := range []string{firstID, secondID} {
		event := map[string]any{
			"schemaVersion": 1, "eventId": eventID, "kind": "intervention", "lane": inspection.Identity.InitialLane,
			"subject": "daily human correction", "summary": correction, "actor": actor, "action": "override", "status": "open",
			"target":                      ".rekit/member-generation-" + string(rune('1'+generation)) + "/evidence/manifest.json",
			"reviewerVerificationEventId": "verification-" + eventID, "reviewerDecisionEventId": "decision-" + eventID,
		}
		if err := mission.AppendJSONLine(interventionPath, event); err != nil {
			t.Fatal(err)
		}
	}
	for _, resolution := range []map[string]any{
		{
			"schemaVersion": 1, "eventId": "resolution-generation-1", "kind": "intervention", "lane": inspection.Identity.InitialLane,
			"subject": "reconcile first correction", "summary": "first replacement generation", "actor": actor,
			"action": "reconcile", "status": "resolved", "resolvesEventId": firstID,
			"executor": "replacement-generation-2", "time": firstTime,
		},
		{
			"schemaVersion": 1, "eventId": "resolution-generation-2", "kind": "intervention", "lane": inspection.Identity.InitialLane,
			"subject": "reconcile second correction", "summary": "second replacement generation", "actor": actor,
			"action": "reconcile", "status": "resolved", "resolvesEventId": secondID,
			"executor": "replacement-generation-3", "time": secondTime,
		},
	} {
		if err := mission.AppendJSONLine(interventionPath, resolution); err != nil {
			t.Fatal(err)
		}
	}

	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	for index := range board.Lanes {
		if board.Lanes[index].ID == inspection.Identity.InitialLane {
			board.Lanes[index].CurrentExecutor = "replacement-generation-3"
			board.Lanes[index].ExecutorGeneration = 3
			board.Lanes[index].LastReconciledIntervention = secondID
			board.Lanes[index].LastReconcileAt = secondTime
		}
	}
	writeDailyTestBoard(t, caseRoot, board)

	lane, boardLane, existing, err := dailyCorrectionLane(caseRoot, inspection, correction, actor)
	if err != nil || lane != inspection.Identity.InitialLane || boardLane.ID != lane || mission.Value(existing, "eventId") != secondID {
		t.Fatalf("latest reconciled correction lookup: lane=%s board=%+v existing=%v err=%v", lane, boardLane, existing, err)
	}
	resolution, resolved, err := inspectDailyCorrectionResolution(caseRoot, lane, secondID)
	if err != nil || !resolved || resolution.EventID != "resolution-generation-2" || resolution.Time != secondTime || resolution.Executor != "replacement-generation-3" || resolution.ExecutorGeneration != 3 {
		t.Fatalf("latest correction resolution: resolution=%+v resolved=%t err=%v", resolution, resolved, err)
	}

	for _, mutate := range []struct {
		name string
		set  func(*mission.BoardLane)
	}{
		{name: "event-id", set: func(lane *mission.BoardLane) { lane.LastReconciledIntervention = firstID }},
		{name: "resolution-time", set: func(lane *mission.BoardLane) { lane.LastReconcileAt = firstTime }},
		{name: "executor", set: func(lane *mission.BoardLane) { lane.CurrentExecutor = "stale-generation" }},
		{name: "generation", set: func(lane *mission.BoardLane) { lane.ExecutorGeneration = 0 }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			tampered := board
			tampered.Lanes = append([]mission.BoardLane{}, board.Lanes...)
			for index := range tampered.Lanes {
				if tampered.Lanes[index].ID == lane {
					mutate.set(&tampered.Lanes[index])
				}
			}
			writeDailyTestBoard(t, caseRoot, tampered)
			if _, ok, err := inspectDailyCorrectionResolution(caseRoot, lane, secondID); err == nil || ok {
				t.Fatalf("tampered resolution accepted: ok=%t err=%v", ok, err)
			}
			writeDailyTestBoard(t, caseRoot, board)
		})
	}
}

func writeDailyTestBoard(t *testing.T, caseRoot string, board mission.Board) {
	t.Helper()
	data, err := json.MarshalIndent(board, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRunDailyStopsForExecutionEvidenceReviewWithoutTrustedClaude(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "evidence-review-stop")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "review bounded execution evidence", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap, bootstrap.Lane); err != nil {
		t.Fatal(err)
	}
	observation := `{"kind":"observation","eventId":"obs-daily-host-stop","lane":"` + bootstrap.Lane + `","subject":"bounded adapter output","summary":"preauthorized adapter output ready for review","status":"complete","target":"target-alpha","evidenceRefs":["evidence/debug.json"],"execution":{"gateEventId":"gate-daily-host-stop","authorization":"preauthorized","status":"complete","outputRefs":["workspace/output.txt"]},"gate":{"action":"inspect","authorization":{"decision":"preauthorized"}}}` + "\n"
	writeSessionhostTestFile(t, caseRoot, ".rekit/facts/observations.jsonl", []byte(observation))

	before := snapshotDailyCaseFiles(t, caseRoot)
	result, err := RunDaily(context.Background(), DailyOptions{
		Target:                            caseRoot,
		SelectedLane:                      bootstrap.Lane,
		ClaudePath:                        missingClaudePath(t),
		ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
		MaxAttempts:                       1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.FinalState != DailyActionReadyForEvidenceReview || result.Action == nil || result.Action.Code != DailyActionReadyForEvidenceReview || result.SessionLaunches != 0 || result.SessionCompletions != 0 || result.Replacements != 0 || len(result.HostRuns) != 1 || result.HostRuns[0].AppliedSteps != 0 {
		t.Fatalf("daily evidence review stop launched or mutated host work: %+v", result)
	}
	assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))
}

func TestRunDailyPrewrittenCorrectionCannotBypassMemberReadiness(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "correction-retry")
	actor := "daily-source-actor"
	goal := "inspect the supplied research target"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, goal, actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	pack := inspection.Identity.Pack
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, pack, &bootstrap); err != nil {
		t.Fatal(err)
	}

	correction := "prioritize the recovered control-flow evidence"
	eventID := dailyCorrectionEventID(dailyCorrectionScope(caseRoot, inspection), inspection.Identity.InitialLane, correction)
	args := dailyCorrectionArgs(caseRoot, pack, inspection.Identity.InitialLane, correction, actor, eventID, "2026-08-07T01:02:03Z", missionintent.MissionIntentRel)
	var preview note.AppendResult
	if err := runPublicCLI(args, &preview); err != nil {
		t.Fatal(err)
	}
	var recorded note.AppendResult
	if err := runPublicCLI(preview.RecordArgs, &recorded); err != nil || !recorded.Applied {
		t.Fatalf("record correction: applied=%t err=%v", recorded.Applied, err)
	}

	opt := DailyOptions{Target: caseRoot, Correction: correction, SelectedLane: inspection.Identity.InitialLane, Actor: actor, ClaudePath: missingClaudePath(t), ExpectedClaudeExecutableSHA256: strings.Repeat("0", 64), ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher}
	first, firstErr := RunDaily(context.Background(), opt)
	if firstErr == nil || !strings.Contains(firstErr.Error(), "current real member result") {
		t.Fatalf("prewritten correction result=%+v err=%v", first, firstErr)
	}
	if first.ExecutorGeneration != 0 || containsDailyStep(first.DriverSteps, "reconcile") || first.SessionLaunches != 0 {
		t.Fatalf("prewritten correction bypassed member readiness: %+v", first)
	}
	if _, ok, inspectErr := inspectDailyCorrectionResolution(caseRoot, inspection.Identity.InitialLane, eventID); inspectErr != nil || ok {
		t.Fatalf("prewritten correction created resolution: ok=%t err=%v", ok, inspectErr)
	}
}

func provisionSessionhostAttachedCase(t *testing.T, repo, pack string) string {
	t.Helper()
	caseRoot := filepath.Join(t.TempDir(), "attached")
	plan, err := syncreview.PlanExclusiveInit(repo, caseRoot, pack, syncreview.ExclusiveInitOptions{ProjectName: "attached-demo", ProvisionID: "sessionhost-attached-fixture", Role: "sessionhost-attached-fixture", CreatedAt: time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC), SkipVerificationMarker: true, DefaultPublicationPhase: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := syncreview.ApplyExclusiveInit(plan); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, "case-local.txt"), []byte("preserve me\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return caseRoot
}

func sessionhostTestRepoRoot(t *testing.T) string {
	t.Helper()
	ctx, err := rekitruntime.New("", "_template")
	if err != nil {
		t.Fatal(err)
	}
	return ctx.RepoRoot
}

func snapshotDailyCaseFiles(t *testing.T, caseRoot string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(caseRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(caseRoot, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func assertDailyCaseFilesEqual(t *testing.T, want, got map[string]string) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("daily case file count changed: before=%d after=%d", len(want), len(got))
	}
	for path, before := range want {
		if after, ok := got[path]; !ok {
			t.Fatalf("daily case file disappeared: %s", path)
		} else if after != before {
			t.Fatalf("daily case file changed: %s", path)
		}
	}
}

func missingClaudePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "missing-claude.exe")
}

func containsDailyStep(steps []string, want string) bool {
	return slices.Contains(steps, want)
}
