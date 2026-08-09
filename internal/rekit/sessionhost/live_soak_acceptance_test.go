package sessionhost

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestRunLiveSoakAcceptanceAggregatesThreeTasksAndRecovery(t *testing.T) {
	calls := 0
	opt := liveSoakTestOptions(t)
	opt.testHooks = &liveSoakAcceptanceTestHooks{
		runTask: func(_ context.Context, taskOpt LiveAcceptanceOptions) (LiveAcceptanceReceipt, error) {
			calls++
			if taskOpt.Pack != liveSoakAcceptancePacks[calls-1] {
				t.Fatalf("task %d pack=%q", calls, taskOpt.Pack)
			}
			return successfulLiveSoakTaskReceipt(), nil
		},
		runRecovery: func(context.Context, LiveSupervisionAcceptanceOptions) (LiveSupervisionAcceptanceReceipt, error) {
			return successfulLiveSoakRecoveryReceipt(), nil
		},
	}

	receipt, err := RunLiveSoakAcceptance(context.Background(), opt)
	if err != nil {
		t.Fatal(err)
	}
	if !receipt.Passed || calls != 3 || receipt.TaskCount != 3 || receipt.SuccessfulTasks != 3 || receipt.FailedTasks != 0 || receipt.SuccessRatePercent != 100 || receipt.AttemptCount != 3 || receipt.SuccessfulAttempts != 3 || receipt.FailedAttempts != 0 || receipt.AttemptSuccessRatePercent != 100 || receipt.RetriedTasks != 0 {
		t.Fatalf("unexpected soak summary: %+v", receipt)
	}
	if receipt.HumanNaturalLanguageInputs != 2 || receipt.HumanLowLevelInputs != 0 || receipt.ManualPlaceholders != 0 || receipt.ManualResultWrites != 0 {
		t.Fatalf("unexpected manual input summary: %+v", receipt)
	}
	if receipt.MemberLaunches != 10 || receipt.MemberCompletions != 10 || receipt.ReviewerLaunches != 6 || receipt.ReviewerCompletions != 6 || receipt.Replacements != 3 || receipt.ProcessReplacements != 0 {
		t.Fatalf("unexpected session summary: %+v", receipt)
	}
	if receipt.CleanupExpected != 7 || receipt.CleanupCreated != 7 || receipt.CleanupRemoved != 7 || receipt.ProviderFailureObservation != "not-observed" || len(receipt.FailureCounts) != 0 {
		t.Fatalf("unexpected cleanup or failure summary: %+v", receipt)
	}
}

func TestRunLiveSoakAcceptanceCountsDurableAndProcessReplacementsSeparately(t *testing.T) {
	opt := liveSoakTestOptions(t)
	opt.testHooks = &liveSoakAcceptanceTestHooks{
		runTask: func(_ context.Context, _ LiveAcceptanceOptions) (LiveAcceptanceReceipt, error) {
			receipt := successfulLiveSoakTaskReceipt()
			receipt.Replacements = 2
			return receipt, nil
		},
		runRecovery: func(context.Context, LiveSupervisionAcceptanceOptions) (LiveSupervisionAcceptanceReceipt, error) {
			return successfulLiveSoakRecoveryReceipt(), nil
		},
	}

	receipt, err := RunLiveSoakAcceptance(context.Background(), opt)
	if err != nil || !receipt.Passed || receipt.Replacements != 3 || receipt.ProcessReplacements != 6 {
		t.Fatalf("replacement statistics drifted: receipt=%+v err=%v", receipt, err)
	}
}

func TestRunLiveSoakAcceptanceRetriesSemanticFailureAndRetainsAttempt(t *testing.T) {
	calls := 0
	opt := liveSoakTestOptions(t)
	opt.testHooks = &liveSoakAcceptanceTestHooks{
		runTask: func(_ context.Context, _ LiveAcceptanceOptions) (LiveAcceptanceReceipt, error) {
			calls++
			receipt := successfulLiveSoakTaskReceipt()
			if calls == 1 {
				receipt.Passed = false
				return receipt, errors.New("replacement reviewer rejected the bounded current output")
			}
			return receipt, nil
		},
		runRecovery: func(context.Context, LiveSupervisionAcceptanceOptions) (LiveSupervisionAcceptanceReceipt, error) {
			return successfulLiveSoakRecoveryReceipt(), nil
		},
	}

	receipt, err := RunLiveSoakAcceptance(context.Background(), opt)
	if err != nil || !receipt.Passed || calls != 4 || receipt.SuccessfulTasks != 3 || receipt.FailedTasks != 0 || receipt.RetriedTasks != 1 {
		t.Fatalf("bounded semantic retry failed: receipt=%+v err=%v", receipt, err)
	}
	if receipt.AttemptCount != 4 || receipt.SuccessfulAttempts != 3 || receipt.FailedAttempts != 1 || receipt.AttemptSuccessRatePercent != 75 || len(receipt.Tasks) != 4 {
		t.Fatalf("semantic retry attempts were not retained: %+v", receipt)
	}
	if receipt.Tasks[0].Passed || receipt.Tasks[0].Attempt != 1 || receipt.Tasks[1].Attempt != 2 || receipt.FailureCounts["reviewer-semantic-or-lineage"] != 1 {
		t.Fatalf("semantic retry lineage drifted: %+v", receipt)
	}
	if receipt.CleanupExpected != 9 || receipt.CleanupCreated != 9 || receipt.CleanupRemoved != 9 {
		t.Fatalf("semantic retry cleanup totals drifted: %+v", receipt)
	}
}

func TestRunLiveSoakAcceptanceDoesNotRetryTypedReviewerIntakeFailure(t *testing.T) {
	calls := 0
	opt := liveSoakTestOptions(t)
	opt.testHooks = &liveSoakAcceptanceTestHooks{
		runTask: func(_ context.Context, _ LiveAcceptanceOptions) (LiveAcceptanceReceipt, error) {
			calls++
			receipt := successfulLiveSoakTaskReceipt()
			if calls == 1 {
				receipt.Passed = false
				receipt.Failure = &FailureDiagnosis{
					Code:   "claude-intake-failed",
					Stage:  "runtime-intake",
					Detail: "replacement reviewer runtime intake contract failed",
				}
				return receipt, errors.New("daily correction and reviewer sessions: runtime intake failed")
			}
			return receipt, nil
		},
		runRecovery: func(context.Context, LiveSupervisionAcceptanceOptions) (LiveSupervisionAcceptanceReceipt, error) {
			return successfulLiveSoakRecoveryReceipt(), nil
		},
	}

	receipt, err := RunLiveSoakAcceptance(context.Background(), opt)
	if err == nil || receipt.Passed || calls != 3 || receipt.RetriedTasks != 0 || receipt.AttemptCount != 3 || receipt.FailedTasks != 1 {
		t.Fatalf("typed reviewer intake failure was retried: receipt=%+v err=%v", receipt, err)
	}
	if receipt.Tasks[0].FailureCode != "strict-contract-or-intake" || receipt.FailureCounts["strict-contract-or-intake"] != 1 {
		t.Fatalf("typed reviewer intake classification drifted: %+v", receipt)
	}
}

func TestRunLiveSoakAcceptanceRejectsWeakTaskReceipt(t *testing.T) {
	opt := liveSoakTestOptions(t)
	opt.testHooks = &liveSoakAcceptanceTestHooks{
		runTask: func(_ context.Context, _ LiveAcceptanceOptions) (LiveAcceptanceReceipt, error) {
			receipt := successfulLiveSoakTaskReceipt()
			receipt.RejectedReplay.SessionLaunches = 1
			return receipt, nil
		},
		runRecovery: func(context.Context, LiveSupervisionAcceptanceOptions) (LiveSupervisionAcceptanceReceipt, error) {
			return successfulLiveSoakRecoveryReceipt(), nil
		},
	}

	receipt, err := RunLiveSoakAcceptance(context.Background(), opt)
	if err == nil || receipt.Passed || receipt.SuccessfulTasks != 0 || receipt.FailedTasks != 3 {
		t.Fatalf("weak task receipts passed: receipt=%+v err=%v", receipt, err)
	}
}

func TestRunLiveSoakAcceptanceRejectsWeakRecoveryReceipt(t *testing.T) {
	opt := liveSoakTestOptions(t)
	opt.testHooks = &liveSoakAcceptanceTestHooks{
		runTask: func(_ context.Context, _ LiveAcceptanceOptions) (LiveAcceptanceReceipt, error) {
			return successfulLiveSoakTaskReceipt(), nil
		},
		runRecovery: func(context.Context, LiveSupervisionAcceptanceOptions) (LiveSupervisionAcceptanceReceipt, error) {
			receipt := successfulLiveSoakRecoveryReceipt()
			receipt.InterruptedCutPoints[0], receipt.InterruptedCutPoints[1] = receipt.InterruptedCutPoints[1], receipt.InterruptedCutPoints[0]
			return receipt, nil
		},
	}

	receipt, err := RunLiveSoakAcceptance(context.Background(), opt)
	if err == nil || receipt.Passed || receipt.Recovery.Passed || receipt.Recovery.FailureCode != "soak-recovery-invariant-failed" {
		t.Fatalf("weak recovery receipt passed: receipt=%+v err=%v", receipt, err)
	}
}

func TestRunLiveSoakAcceptanceRetainsFailureAndContinues(t *testing.T) {
	calls := 0
	opt := liveSoakTestOptions(t)
	opt.testHooks = &liveSoakAcceptanceTestHooks{
		runTask: func(_ context.Context, _ LiveAcceptanceOptions) (LiveAcceptanceReceipt, error) {
			calls++
			receipt := successfulLiveSoakTaskReceipt()
			if calls == 2 {
				receipt.Passed = false
				return receipt, errors.New("Claude Code timed out while waiting for bounded output")
			}
			return receipt, nil
		},
		runRecovery: func(context.Context, LiveSupervisionAcceptanceOptions) (LiveSupervisionAcceptanceReceipt, error) {
			return successfulLiveSoakRecoveryReceipt(), nil
		},
	}

	receipt, err := RunLiveSoakAcceptance(context.Background(), opt)
	if err == nil || receipt.Passed || calls != 3 || receipt.SuccessfulTasks != 2 || receipt.FailedTasks != 1 || receipt.SuccessRatePercent != 66 {
		t.Fatalf("failed task was not retained: receipt=%+v err=%v", receipt, err)
	}
	if len(receipt.Tasks) != 3 || receipt.Tasks[1].FailureCode != "process-timeout" || receipt.FailureCounts["process-timeout"] != 1 {
		t.Fatalf("failed task classification drifted: %+v", receipt)
	}
	if receipt.CleanupExpected != 7 || receipt.CleanupCreated != 7 || receipt.CleanupRemoved != 7 {
		t.Fatalf("failure cleanup was not retained: %+v", receipt)
	}
}

func TestRunLiveSoakAcceptanceRetainsRecoveryFailure(t *testing.T) {
	calls := 0
	opt := liveSoakTestOptions(t)
	opt.testHooks = &liveSoakAcceptanceTestHooks{
		runTask: func(_ context.Context, _ LiveAcceptanceOptions) (LiveAcceptanceReceipt, error) {
			calls++
			return successfulLiveSoakTaskReceipt(), nil
		},
		runRecovery: func(context.Context, LiveSupervisionAcceptanceOptions) (LiveSupervisionAcceptanceReceipt, error) {
			receipt := successfulLiveSoakRecoveryReceipt()
			receipt.Passed = false
			return receipt, errors.New("recovery output publication failed")
		},
	}

	receipt, err := RunLiveSoakAcceptance(context.Background(), opt)
	if err == nil || receipt.Passed || calls != 3 || receipt.SuccessfulTasks != 3 || receipt.FailedTasks != 0 || receipt.Recovery.Passed {
		t.Fatalf("recovery failure was not retained: receipt=%+v err=%v", receipt, err)
	}
	if receipt.Recovery.FailureCode != "host-runtime" || receipt.FailureCounts["host-runtime"] != 1 {
		t.Fatalf("recovery failure classification drifted: %+v", receipt)
	}
	if receipt.CleanupExpected != 7 || receipt.CleanupCreated != 7 || receipt.CleanupRemoved != 7 {
		t.Fatalf("recovery failure cleanup was not retained: %+v", receipt)
	}
}

func TestRunLiveSoakAcceptanceDoesNotCountUncreatedCaseAsRemoved(t *testing.T) {
	calls := 0
	opt := liveSoakTestOptions(t)
	opt.testHooks = &liveSoakAcceptanceTestHooks{
		runTask: func(_ context.Context, _ LiveAcceptanceOptions) (LiveAcceptanceReceipt, error) {
			calls++
			receipt := successfulLiveSoakTaskReceipt()
			if calls == 1 {
				receipt.Passed = false
				receipt.AttachedCase.CaseCreated = false
				receipt.AttachedCase.Cleanup = "not-created"
				return receipt, errors.New("failed before attached case creation")
			}
			return receipt, nil
		},
		runRecovery: func(context.Context, LiveSupervisionAcceptanceOptions) (LiveSupervisionAcceptanceReceipt, error) {
			return successfulLiveSoakRecoveryReceipt(), nil
		},
	}

	receipt, err := RunLiveSoakAcceptance(context.Background(), opt)
	if err == nil || receipt.CleanupExpected != 7 || receipt.CleanupCreated != 6 || receipt.CleanupRemoved != 6 {
		t.Fatalf("uncreated case cleanup was misreported: receipt=%+v err=%v", receipt, err)
	}
	if receipt.Tasks[0].CleanupExpected != 2 || receipt.Tasks[0].CleanupCreated != 1 || receipt.Tasks[0].CleanupRemoved != 1 {
		t.Fatalf("task cleanup truth drifted: %+v", receipt.Tasks[0])
	}
}

func TestLiveAcceptanceSessionPreservesTypedFailure(t *testing.T) {
	receipt := LiveAcceptanceReceipt{}
	addLiveAcceptanceSessions(&receipt, Result{Sessions: []Session{{
		Started: true, SessionKind: "member", Outcome: "failed",
		Failure: &FailureDiagnosis{Code: "claude-quota-unavailable", ProviderObservation: "observed", Detail: "bounded failure"},
	}}}, 1)
	if len(receipt.MemberSessions) != 1 || receipt.MemberSessions[0].Failure == nil || receipt.MemberSessions[0].Failure.Code != "claude-quota-unavailable" {
		t.Fatalf("typed failure was not preserved: %+v", receipt.MemberSessions)
	}
}

func TestLiveAcceptanceCleanupStatusPreservesMixedOutcomes(t *testing.T) {
	cleanupErr := errors.New("bounded cleanup failure")
	if got := liveAcceptanceCleanupStatus(true, nil); got != "removed" {
		t.Fatalf("successful cleanup status=%q", got)
	}
	if got := liveAcceptanceCleanupStatus(true, cleanupErr); got != "failed" {
		t.Fatalf("failed cleanup status=%q", got)
	}
	if got := liveAcceptanceCleanupStatus(false, nil); got != "not-created" {
		t.Fatalf("uncreated cleanup status=%q", got)
	}
	if err := wrapLiveAcceptanceCleanupError("fresh", nil); err != nil {
		t.Fatalf("successful cleanup produced error: %v", err)
	}
	if err := wrapLiveAcceptanceCleanupError("attached", cleanupErr); err == nil || err.Error() != "clean attached live acceptance case: bounded cleanup failure" {
		t.Fatalf("failed cleanup error=%v", err)
	}
}

func TestRunLiveSoakAcceptanceRecordsObservedTypedProviderFailure(t *testing.T) {
	calls := 0
	opt := liveSoakTestOptions(t)
	opt.testHooks = &liveSoakAcceptanceTestHooks{
		runTask: func(_ context.Context, _ LiveAcceptanceOptions) (LiveAcceptanceReceipt, error) {
			calls++
			receipt := successfulLiveSoakTaskReceipt()
			if calls == 1 {
				receipt.Passed = false
				receipt.MemberSessions = []LiveAcceptanceSession{
					{
						Started: true, Kind: "member", Outcome: "failed",
						Failure: &FailureDiagnosis{Code: "claude-timeout", Detail: "earlier bounded timeout"},
					},
					{
						Started: true, Kind: "member", Outcome: "failed",
						Failure: &FailureDiagnosis{Code: "claude-quota-unavailable", ProviderObservation: "observed", Detail: "bounded provider availability failure"},
					},
				}
				return receipt, errors.New("external session attempt limit reached")
			}
			return receipt, nil
		},
		runRecovery: func(context.Context, LiveSupervisionAcceptanceOptions) (LiveSupervisionAcceptanceReceipt, error) {
			return successfulLiveSoakRecoveryReceipt(), nil
		},
	}

	receipt, err := RunLiveSoakAcceptance(context.Background(), opt)
	if err == nil || receipt.ProviderFailureObservation != "observed" || receipt.FailureCounts["claude-quota-unavailable"] != 1 {
		t.Fatalf("typed provider failure observation drifted: receipt=%+v err=%v", receipt, err)
	}
}

func TestRunLiveSoakAcceptanceRecordsObservedProviderFailure(t *testing.T) {
	calls := 0
	opt := liveSoakTestOptions(t)
	opt.testHooks = &liveSoakAcceptanceTestHooks{
		runTask: func(_ context.Context, _ LiveAcceptanceOptions) (LiveAcceptanceReceipt, error) {
			calls++
			receipt := successfulLiveSoakTaskReceipt()
			if calls == 1 {
				receipt.Passed = false
				return receipt, errors.New("Claude usage limit reached")
			}
			return receipt, nil
		},
		runRecovery: func(context.Context, LiveSupervisionAcceptanceOptions) (LiveSupervisionAcceptanceReceipt, error) {
			return successfulLiveSoakRecoveryReceipt(), nil
		},
	}

	receipt, err := RunLiveSoakAcceptance(context.Background(), opt)
	if err == nil || receipt.ProviderFailureObservation != "observed" || receipt.FailureCounts["provider-quota"] != 1 {
		t.Fatalf("provider failure observation drifted: receipt=%+v err=%v", receipt, err)
	}
}

func TestRunLiveSoakAcceptanceRejectsReceiptInsideRepository(t *testing.T) {
	repoRoot, err := currentRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	opt := LiveSoakAcceptanceOptions{
		Goal:        "Inspect one harmless bounded case-local feature note.",
		Correction:  "Use only the newly published bounded note.",
		ReceiptPath: filepath.Join(repoRoot, "forbidden-soak-receipt.json"),
	}
	if _, err := RunLiveSoakAcceptance(context.Background(), opt); err == nil {
		t.Fatal("repository-contained soak receipt was accepted")
	}
}

func liveSoakTestOptions(t *testing.T) LiveSoakAcceptanceOptions {
	t.Helper()
	return LiveSoakAcceptanceOptions{
		Goal:        "Inspect one harmless bounded case-local feature note.",
		Correction:  "Use only the newly published bounded note.",
		ReceiptPath: filepath.Join(t.TempDir(), "receipt.json"),
		Timeout:     time.Minute,
		MaxAttempts: 3,
	}
}

func successfulLiveSoakTaskReceipt() LiveAcceptanceReceipt {
	return LiveAcceptanceReceipt{
		Passed:            true,
		CaseRoot:          filepath.Join("C:\\temp", "fresh"),
		CaseCreated:       true,
		Cleanup:           "removed",
		FinalAcceptance:   &LiveAcceptanceAcceptance{PacketID: "packet"},
		CorrectionEventID: "correction",
		FirstMember: LiveAcceptanceMember{
			AttemptID:  "member-attempt",
			Generation: 1,
		},
		ReplacementMember: LiveAcceptanceMember{
			AttemptID:          "replacement-attempt",
			Generation:         2,
			CorrectionSourceID: "correction",
		},
		RejectedReplay: LiveAcceptanceReplay{
			Verified: true,
		},
		TerminalReplay: LiveAcceptanceReplay{
			Verified: true,
		},
		AttachedCase: LiveAcceptanceAttached{
			CaseRoot:       filepath.Join("C:\\temp", "attached"),
			CaseCreated:    true,
			Verified:       true,
			TerminalReplay: true,
			ReplayLaunches: 0,
			Cleanup:        "removed",
		},
		MemberLaunches:      3,
		MemberCompletions:   3,
		ReviewerLaunches:    2,
		ReviewerCompletions: 2,
		Replacements:        0,
	}
}

func successfulLiveSoakRecoveryReceipt() LiveSupervisionAcceptanceReceipt {
	return LiveSupervisionAcceptanceReceipt{
		Passed:               true,
		CaseRoot:             filepath.Join("C:\\temp", "recovery"),
		CaseCreated:          true,
		CutPoint:             "process-start",
		RunID:                "run-1",
		SessionID:            "session-1",
		AttemptSHA256:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		FirstHostInterrupted: true,
		InterruptedCutPoints: []string{"process-start", "output-returned", "result-first", "submission", "intake"},
		FreshHostLaunches:    0,
		FreshCompletions:     1,
		TotalStartedReceipts: 1,
		OutputPublications:   1,
		Cleanup:              "removed",
	}
}
