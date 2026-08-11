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
)

func TestCurrentReviewerRejectionProbeAllowsMissingBootstrapBoard(t *testing.T) {
	rejected, err := currentReviewerRejectionAwaitingCorrection(t.TempDir(), liveAcceptancePack)
	if err != nil || rejected {
		t.Fatalf("missing bootstrap board rejection probe: rejected=%t err=%v", rejected, err)
	}
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
