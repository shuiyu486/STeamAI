package sessionhost

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func TestRunHoldsRecoveredResultAcrossRestartWhenPausedBeforeStatus(t *testing.T) {
	opt, running := runningSessionhostAttemptFixture(t, 1)
	pkg := *running.ExternalSessionStep.HarnessPackage
	bindTrustedSupervisionOptionsForTest(t, &opt, 3)
	bound, err := ensureClaudeLaunchControlBinding(opt, pkg)
	if err != nil {
		t.Fatal(err)
	}
	run := claudeRun{
		launchControlBinding: cloneClaudeLaunchControlBinding(bound.launchControlBinding),
		envelope: claudeEnvelope{
			Type:      "result",
			Subtype:   "success",
			SessionID: pkg.Launch.Attempt.Session,
		},
		sessionID: pkg.Launch.Attempt.Session,
		structuredOutput: json.RawMessage(
			`{"outcome":"returned","summary":"bounded restart result","reason":"","outputs":[{"path":"restart-result.txt","content":"opaque restart result\n"}],"reviewerItemsPath":""}`,
		),
		started:    true,
		exitCode:   0,
		observedAt: "2026-08-18T17:00:00Z",
	}
	paths, _, specData, _, err := prepareSupervision(bound, pkg, pkg.Launch.Attempt.Session)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(paths.root) })
	if err := os.WriteFile(paths.spec, specData, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := persistClaudeRecoveryForCase(opt.Target, bound, pkg, run); err != nil {
		t.Fatal(err)
	}
	recoveryRoot, err := claudeRecoveryRoot(opt.Target)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(recoveryRoot) })
	applyClaudeLaunchControlForTest(
		t,
		opt.Target,
		bound.launchControlBinding.Lane,
		executioncontrol.ActionPause,
		"2026-08-18T17:01:00Z",
	)

	first, err := Run(context.Background(), opt)
	if err != nil {
		t.Fatalf("restart pause preflight result=%+v err=%v", first, err)
	}
	if first.FinalMode != executioncontrol.ResultDispositionHeldWhilePaused ||
		first.SessionLaunches != 0 || first.SessionCompletions != 0 ||
		first.AppliedSteps != 0 || first.Replacements != 0 || len(first.Sessions) != 1 ||
		first.Sessions[0].Outcome != executioncontrol.ResultDispositionHeldWhilePaused {
		t.Fatalf("restart pause preflight crossed host progression: %+v", first)
	}
	for _, path := range []string{
		filepath.Join(opt.Target, filepath.FromSlash(pkg.Return.SubmissionOutputs), "restart-result.txt"),
		filepath.Join(opt.Target, filepath.FromSlash(pkg.Return.SubmissionPath)),
		paths.claimed,
		paths.started,
		paths.terminal,
	} {
		if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
			t.Fatalf("restart pause preflight published or launched %s: %v", path, statErr)
		}
	}
	recoveryPath := filepath.Join(
		recoveryRoot,
		filepath.FromSlash(claudeRecoveryPath(pkg, run.launchControlBinding)),
	)
	if _, statErr := os.Lstat(recoveryPath); statErr != nil {
		t.Fatalf("restart pause preflight did not retain recovery %s: %v", recoveryPath, statErr)
	}
	heldRoot := filepath.Join(
		opt.Target,
		projectstate.CurrentDir,
		"lanes",
		bound.launchControlBinding.Lane,
		"execution-control",
		"held-results",
	)
	entries, err := os.ReadDir(heldRoot)
	if err != nil || len(entries) != 1 {
		t.Fatalf("restart pause held receipts=%d err=%v", len(entries), err)
	}
	receiptPath := filepath.Join(heldRoot, entries[0].Name())
	beforeReplay, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}

	replayed, err := Run(context.Background(), opt)
	if err != nil {
		t.Fatalf("restart pause replay result=%+v err=%v", replayed, err)
	}
	afterReplay, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.FinalMode != executioncontrol.ResultDispositionHeldWhilePaused ||
		replayed.SessionLaunches != 0 || replayed.SessionCompletions != 0 ||
		replayed.AppliedSteps != 0 || len(replayed.Sessions) != 1 ||
		string(afterReplay) != string(beforeReplay) {
		t.Fatalf("restart pause replay changed sticky held result: %+v", replayed)
	}
	entries, err = os.ReadDir(heldRoot)
	if err != nil || len(entries) != 1 {
		t.Fatalf("restart pause replay held receipts=%d err=%v", len(entries), err)
	}
}

func TestRunDailyHoldsRecoveredResultAcrossRestartBeforeStatus(t *testing.T) {
	identity, err := discoverTrustedClaudeExecutable()
	if err != nil {
		t.Skipf("signed canonical Claude Code installation unavailable: %v", err)
	}
	opt, running := runningSessionhostAttemptFixture(t, 1)
	pkg := *running.ExternalSessionStep.HarnessPackage
	bindTrustedSupervisionOptionsForTest(t, &opt, 3)
	opt.ClaudePath = identity.Path
	opt.ExpectedClaudeExecutableSHA256 = identity.SHA256
	opt.ExpectedClaudeExecutablePublisher = identity.Publisher
	bound, err := ensureClaudeLaunchControlBinding(opt, pkg)
	if err != nil {
		t.Fatal(err)
	}
	run := claudeRun{
		launchControlBinding: cloneClaudeLaunchControlBinding(bound.launchControlBinding),
		envelope: claudeEnvelope{
			Type:      "result",
			Subtype:   "success",
			SessionID: pkg.Launch.Attempt.Session,
		},
		sessionID: pkg.Launch.Attempt.Session,
		structuredOutput: json.RawMessage(
			`{"outcome":"returned","summary":"bounded daily restart result","reason":"","outputs":[{"path":"daily-restart-result.txt","content":"opaque daily restart result\n"}],"reviewerItemsPath":""}`,
		),
		started:    true,
		exitCode:   0,
		observedAt: "2026-08-18T18:00:00Z",
	}
	paths, _, specData, _, err := prepareSupervision(bound, pkg, pkg.Launch.Attempt.Session)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(paths.root) })
	if err := os.WriteFile(paths.spec, specData, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := persistClaudeRecoveryForCase(opt.Target, bound, pkg, run); err != nil {
		t.Fatal(err)
	}
	recoveryRoot, err := claudeRecoveryRoot(opt.Target)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(recoveryRoot) })
	applyClaudeLaunchControlForTest(
		t,
		opt.Target,
		bound.launchControlBinding.Lane,
		executioncontrol.ActionPause,
		"2026-08-18T18:01:00Z",
	)

	result, err := RunDaily(context.Background(), DailyOptions{
		Target:                            opt.Target,
		SelectedLane:                      bound.launchControlBinding.Lane,
		Actor:                             "daily-recovery-preflight-test",
		ClaudePath:                        identity.Path,
		ExpectedClaudeExecutableSHA256:    identity.SHA256,
		ExpectedClaudeExecutablePublisher: identity.Publisher,
		MaxAttempts:                       3,
	})
	if err != nil {
		t.Fatalf("daily restart pause preflight result=%+v err=%v", result, err)
	}
	if result.FinalState != executioncontrol.ResultDispositionHeldWhilePaused || !result.Blocked || !result.Replay ||
		result.Lane != bound.launchControlBinding.Lane || result.SessionLaunches != 0 || result.SessionCompletions != 0 ||
		result.Replacements != 0 || len(result.DriverSteps) != 0 || len(result.HostRuns) != 1 ||
		result.Action == nil || result.Action.Code != DailyActionBlocked || result.Action.RequiresInput ||
		result.HostRuns[0].AppliedSteps != 0 || len(result.HostRuns[0].Sessions) != 1 {
		t.Fatalf("daily restart pause preflight crossed progression: %+v", result)
	}
	for _, path := range []string{
		filepath.Join(opt.Target, filepath.FromSlash(pkg.Return.SubmissionOutputs), "daily-restart-result.txt"),
		filepath.Join(opt.Target, filepath.FromSlash(pkg.Return.SubmissionPath)),
		paths.claimed,
		paths.started,
		paths.terminal,
	} {
		if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
			t.Fatalf("daily restart pause preflight published or launched %s: %v", path, statErr)
		}
	}
}

func TestRunHoldsRecoveredResultWhenPausedBeforePublication(t *testing.T) {
	opt, running := runningSessionhostAttemptFixture(t, 1)
	pkg := *running.ExternalSessionStep.HarnessPackage
	bindTrustedSupervisionOptionsForTest(t, &opt, 3)
	bound, err := ensureClaudeLaunchControlBinding(opt, pkg)
	if err != nil {
		t.Fatal(err)
	}
	run := claudeRun{
		launchControlBinding: cloneClaudeLaunchControlBinding(bound.launchControlBinding),
		envelope: claudeEnvelope{
			Type:      "result",
			Subtype:   "success",
			SessionID: pkg.Launch.Attempt.Session,
		},
		sessionID: pkg.Launch.Attempt.Session,
		structuredOutput: json.RawMessage(
			`{"outcome":"returned","summary":"bounded paused result","reason":"","outputs":[{"path":"pause-result.txt","content":"opaque paused result\n"}],"reviewerItemsPath":""}`,
		),
		started:    true,
		exitCode:   0,
		observedAt: "2026-08-18T16:00:00Z",
	}
	if !run.success() {
		t.Fatal("paused result fixture should have a successful process envelope")
	}
	if err := persistClaudeRecoveryForCase(opt.Target, bound, pkg, run); err != nil {
		t.Fatal(err)
	}
	recoveryRoot, err := claudeRecoveryRoot(opt.Target)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(recoveryRoot) })

	restore := setSupervisionAcceptanceObservers(nil, func(stage string) error {
		if stage == "result-first" {
			applyClaudeLaunchControlForTest(
				t,
				opt.Target,
				bound.launchControlBinding.Lane,
				executioncontrol.ActionPause,
				"2026-08-18T16:01:00Z",
			)
		}
		return nil
	})
	defer restore()

	result, err := Run(context.Background(), opt)
	if err != nil {
		t.Fatalf("pause-before-publication result=%+v err=%v", result, err)
	}
	if result.FinalMode != executioncontrol.ResultDispositionHeldWhilePaused ||
		result.SessionLaunches != 0 || result.SessionCompletions != 0 ||
		result.AppliedSteps != 0 || len(result.Sessions) != 1 ||
		result.Sessions[0].Outcome != executioncontrol.ResultDispositionHeldWhilePaused {
		t.Fatalf("paused result crossed host progression: %+v", result)
	}
	for _, path := range []string{
		filepath.Join(opt.Target, filepath.FromSlash(pkg.Return.SubmissionOutputs), "pause-result.txt"),
		filepath.Join(opt.Target, filepath.FromSlash(pkg.Return.SubmissionPath)),
	} {
		if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
			t.Fatalf("paused result published canonical artifact %s: %v", path, statErr)
		}
	}
	recoveryPath := filepath.Join(
		recoveryRoot,
		filepath.FromSlash(claudeRecoveryPath(pkg, run.launchControlBinding)),
	)
	if _, statErr := os.Lstat(recoveryPath); statErr != nil {
		t.Fatalf("paused result did not retain raw recovery %s: %v", recoveryPath, statErr)
	}
	heldRoot := filepath.Join(
		opt.Target,
		projectstate.CurrentDir,
		"lanes",
		bound.launchControlBinding.Lane,
		"execution-control",
		"held-results",
	)
	entries, err := os.ReadDir(heldRoot)
	if err != nil || len(entries) != 1 {
		t.Fatalf("paused result held receipts=%d err=%v", len(entries), err)
	}
	data, err := os.ReadFile(filepath.Join(heldRoot, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	var receipt executioncontrol.HeldResultReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Disposition != executioncontrol.ResultDispositionHeldWhilePaused ||
		receipt.CanonicalPublication || receipt.Advanced ||
		!receipt.NoAutoResume || !receipt.NoAuthority || !receipt.NoConfirmed ||
		!receipt.NoHeavyTool {
		t.Fatalf("paused result receipt crossed boundary: %+v", receipt)
	}
}
