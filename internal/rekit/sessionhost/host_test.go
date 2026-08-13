package sessionhost

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestCurrentReviewerRejectionProbeAllowsMissingBootstrapBoard(t *testing.T) {
	rejected, err := currentReviewerRejectionAwaitingCorrection(t.TempDir(), liveAcceptancePack)
	if err != nil || rejected {
		t.Fatalf("missing bootstrap board rejection probe: rejected=%t err=%v", rejected, err)
	}
}

func TestRunPublicHostRequiresFreshCurrentDriverRequestBeforeClaudeResolution(t *testing.T) {
	opt := Options{Target: t.TempDir(), ClaudePath: missingClaudePath(t), MaxAttempts: 1}
	opt.RequireCurrentDriverRequest()
	result, err := Run(context.Background(), opt)
	if err == nil || !strings.Contains(err.Error(), "exact current driver request SHA-256") {
		t.Fatalf("missing current driver request binding: result=%+v err=%v", result, err)
	}
	if result.Failure == nil || result.Failure.Code != "current-driver-request-required" || result.Failure.Stage != "driver-request-currentness" || result.Failure.MutationApplied || result.Failure.MutationBoundary != "none" {
		t.Fatalf("missing request binding diagnosis=%+v", result.Failure)
	}
	if result.SessionLaunches != 0 || len(result.Sessions) != 0 {
		t.Fatalf("missing request binding launched Claude: %+v", result)
	}
}

func TestRunPublicHostRejectsMalformedCurrentDriverRequestBeforeClaudeResolution(t *testing.T) {
	opt := Options{
		Target: t.TempDir(), ClaudePath: missingClaudePath(t), MaxAttempts: 1,
		ExpectedCurrentDriverRequestSHA256: strings.Repeat("z", 64),
	}
	opt.RequireCurrentDriverRequest()
	result, err := Run(context.Background(), opt)
	if err == nil || result.Failure == nil || result.Failure.Code != "current-driver-request-required" || result.Failure.Stage != "driver-request-currentness" || result.SessionLaunches != 0 {
		t.Fatalf("malformed request binding: result=%+v err=%v", result, err)
	}
}

func TestRunPublicHostRejectsStaleCurrentDriverRequestWithoutMutation(t *testing.T) {
	repo := sessionhostTestRepoRoot(t)
	caseRoot := provisionSessionhostAttachedCase(t, repo, "_template")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	if _, err := applyDailyOnboarding(caseRoot, "bind a stale host request", "host-request-test", &bootstrap); err != nil {
		t.Fatal(err)
	}
	before := snapshotDailyCaseFiles(t, caseRoot)
	opt := Options{
		Target: caseRoot, Pack: "_template", SelectedLane: bootstrap.Lane,
		ClaudePath: missingClaudePath(t), MaxAttempts: 1,
		ExpectedCurrentDriverRequestSHA256: strings.Repeat("0", 64),
	}
	opt.RequireCurrentDriverRequest()
	result, err := Run(context.Background(), opt)
	if err == nil || result.Failure == nil || result.Failure.Code != "current-driver-request-stale" || result.Failure.Stage != "driver-request-currentness" {
		t.Fatalf("stale current request: result=%+v err=%v", result, err)
	}
	if result.Failure.MutationApplied || result.SessionLaunches != 0 || len(result.Sessions) != 0 {
		t.Fatalf("stale current request crossed zero-write boundary: %+v", result)
	}
	assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))
}

func TestRunPublicHostRequiresTypedLaneChoiceForMultipleCurrentLanes(t *testing.T) {
	repo := sessionhostTestRepoRoot(t)
	caseRoot := provisionSessionhostAttachedCase(t, repo, "_template")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "bind an ambiguous host request", "host-request-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	if _, err := workstream.StartApply(repo, caseRoot, inspection.Identity.Pack, workstream.StartOptions{
		Name: "login", Actor: "host-request-test", Executor: "session-login", TakeoverReason: "ordinary host lane choice regression",
	}); err != nil {
		t.Fatal(err)
	}
	status, err := runStatus(Options{Target: caseRoot, Pack: inspection.Identity.Pack})
	if err != nil {
		t.Fatal(err)
	}
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentDriverRequest != nil || status.MissionControlRunbook.CurrentDriverRequestSHA256 != "" || len(publicCaseMissionLaneChoices(status.CaseMission)) < 2 {
		t.Fatalf("multi-lane fixture did not require a typed lane choice: %+v", status)
	}
	before := snapshotDailyCaseFiles(t, caseRoot)
	opt := Options{
		Target: caseRoot, Pack: inspection.Identity.Pack,
		ClaudePath: missingClaudePath(t), MaxAttempts: 1,
		ExpectedCurrentDriverRequestSHA256: strings.Repeat("0", 64),
	}
	opt.RequireCurrentDriverRequest()
	result, err := Run(context.Background(), opt)
	if err == nil || result.Failure == nil || result.Failure.Code != "current-driver-request-lane-required" || result.Failure.Stage != "driver-request-currentness" {
		t.Fatalf("ambiguous current request: result=%+v err=%v", result, err)
	}
	if result.Failure.MutationApplied || result.Failure.MutationBoundary != "none" || result.SessionLaunches != 0 || len(result.Sessions) != 0 {
		t.Fatalf("ambiguous current request crossed zero-write boundary: %+v", result)
	}
	assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))
}

func TestRunPublicHostAcceptsFreshRequestForExplicitLane(t *testing.T) {
	repo := sessionhostTestRepoRoot(t)
	caseRoot := provisionSessionhostAttachedCase(t, repo, "_template")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "bind an explicit host lane", "host-request-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	if _, err := workstream.StartApply(repo, caseRoot, inspection.Identity.Pack, workstream.StartOptions{
		Name: "login", Actor: "host-request-test", Executor: "session-login", TakeoverReason: "ordinary host explicit lane regression",
	}); err != nil {
		t.Fatal(err)
	}
	selected := "feature-login"
	status, err := runStatus(Options{Target: caseRoot, Pack: inspection.Identity.Pack, SelectedLane: selected})
	if err != nil {
		t.Fatal(err)
	}
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentDriverRequest == nil || status.MissionControlRunbook.CurrentDriverRequest.Lane != selected {
		t.Fatalf("selected-lane status omitted its request: %+v", status)
	}
	before := snapshotDailyCaseFiles(t, caseRoot)
	opt := Options{
		Target: caseRoot, Pack: inspection.Identity.Pack, SelectedLane: selected,
		ClaudePath: missingClaudePath(t), MaxAttempts: 1,
		ExpectedCurrentDriverRequestSHA256: status.MissionControlRunbook.CurrentDriverRequestSHA256,
	}
	opt.RequireCurrentDriverRequest()
	result, err := Run(context.Background(), opt)
	if err == nil || result.Failure == nil || result.Failure.Code != "claude-executable-unavailable" || result.Failure.Stage != "executable-resolution" {
		t.Fatalf("fresh selected-lane request did not reach executable resolution: result=%+v err=%v", result, err)
	}
	if result.Failure.MutationApplied || result.SessionLaunches != 0 || len(result.Sessions) != 0 {
		t.Fatalf("selected-lane executable failure crossed zero-write boundary: %+v", result)
	}
	assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))
}

func TestRunStopsForExecutionEvidenceReviewWithoutLaunchingClaude(t *testing.T) {
	repo := sessionhostTestRepoRoot(t)
	caseRoot := provisionSessionhostAttachedCase(t, repo, "_template")
	board := mission.Board{
		SchemaVersion:        1,
		CaseRoot:             filepath.ToSlash(caseRoot),
		RepoRoot:             filepath.ToSlash(repo),
		Pack:                 "_template",
		DefaultAuthorityLane: "main",
		Lanes: []mission.BoardLane{{
			ID: "main", Type: "main", Title: "Main", Status: "open",
			Authority: true, Workspace: "workspace/main/main",
			CurrentExecutor: "evidence-review-member", ExecutorGeneration: 1,
			UpdatedAt: "2026-08-11T00:00:00Z",
		}},
		FactsRoot: ".rekit/facts",
	}
	boardData, err := json.Marshal(board)
	if err != nil {
		t.Fatal(err)
	}
	writeSessionhostTestFile(t, caseRoot, ".rekit/board.json", append(boardData, '\n'))
	writeSessionhostTestFile(t, caseRoot, ".rekit/lanes/main/lane.json", []byte(`{"schemaVersion":1,"id":"main","type":"main","title":"Main","status":"open","authority":true,"workspace":"workspace/main/main","laneRoot":".rekit/lanes/main","currentExecutor":"evidence-review-member","executorGeneration":1,"lastTakeoverAt":"2026-08-11T00:00:00Z","lastTakeoverBy":"test","lastTakeoverReason":"fixture"}`))
	writeSessionhostTestFile(t, caseRoot, ".rekit/facts/observations.jsonl", []byte(`{"kind":"observation","eventId":"obs-host-stop","lane":"main","subject":"bounded adapter output","summary":"preauthorized adapter output ready for review","status":"complete","target":"target-alpha","evidenceRefs":["evidence/debug.json"],"execution":{"gateEventId":"gate-host-stop","authorization":"preauthorized","status":"complete","outputRefs":["workspace/main/debug/out.txt"]},"gate":{"action":"debug","authorization":{"decision":"preauthorized"}}}`+"\n"))
	for _, rel := range []string{"requests.jsonl", "candidates.jsonl", "decisions.jsonl", "interventions.jsonl", "verifications.jsonl"} {
		writeSessionhostTestFile(t, caseRoot, ".rekit/facts/"+rel, nil)
	}
	before := snapshotDailyCaseFiles(t, caseRoot)
	result, err := Run(context.Background(), Options{
		Target: caseRoot, Pack: "_template", SelectedLane: "main",
		ClaudePath: missingClaudePath(t), MaxAttempts: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.FinalMode != DailyActionReadyForEvidenceReview || result.SessionLaunches != 0 || result.SessionCompletions != 0 || result.AppliedSteps != 0 || len(result.Sessions) != 0 {
		t.Fatalf("evidence review stop launched or mutated host work: %+v", result)
	}
	assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))
}

func writeSessionhostTestFile(t *testing.T, caseRoot, rel string, data []byte) {
	t.Helper()
	path := filepath.Join(caseRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestResolveClaudePathReportsTypedExecutableFailure(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-claude-executable")
	if runtime.GOOS == "windows" {
		missing += ".exe"
	}
	result, err := Run(context.Background(), Options{Target: t.TempDir(), ClaudePath: missing, MaxAttempts: 3})
	if err == nil {
		t.Fatal("missing Claude executable was accepted")
	}
	if result.Failure == nil || result.Failure.Code != "claude-executable-unavailable" || result.Failure.Stage != "executable-resolution" || result.Failure.State != failureStateRecoverable {
		t.Fatalf("typed executable diagnosis = %+v", result.Failure)
	}
	if result.Failure.MutationApplied || result.Failure.MutationBoundary != "none" || result.Failure.NextAction == "" {
		t.Fatalf("executable mutation boundary or next action = %+v", result.Failure)
	}
}

func TestRunResolvesOmittedPackFromAttachedCaseMetadata(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit"), 0o700); err != nil {
		t.Fatal(err)
	}
	metadata := []byte("templatePack: _template\n")
	if err := os.WriteFile(
		filepath.Join(caseRoot, ".rekit", "instance.yml"),
		metadata,
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(caseRoot, "missing-claude-executable")
	if runtime.GOOS == "windows" {
		missing += ".exe"
	}
	result, err := Run(context.Background(), Options{
		Target:      caseRoot,
		ClaudePath:  missing,
		MaxAttempts: 3,
	})
	if err == nil {
		t.Fatal("missing Claude executable was accepted")
	}
	if result.Pack != "_template" {
		t.Fatalf("omitted host pack was not resolved from case metadata: %+v", result)
	}
}

func TestResolveClaudePathRejectsWindowsCommandScripts(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows executable resolution only")
	}
	path := filepath.Join(t.TempDir(), "claude.cmd")
	if err := os.WriteFile(path, []byte("@exit /b 0\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveClaudePath(path); err == nil || !strings.Contains(err.Error(), "native claude.exe") {
		t.Fatalf("command script resolution error=%v", err)
	}
}
