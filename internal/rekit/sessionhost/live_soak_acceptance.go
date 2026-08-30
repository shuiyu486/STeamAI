package sessionhost

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
)

var liveSoakAcceptancePacks = []string{
	liveAcceptancePack,
	"_template",
	"web-security",
}

type LiveSoakAcceptanceOptions struct {
	Goal                           string
	Correction                     string
	Model                          string
	Actor                          string
	Timeout                        time.Duration
	MaxAttempts                    int
	ReceiptPath                    string
	InitializationSourceExecutable string
	testHooks                      *liveSoakAcceptanceTestHooks
}

type LiveSoakAcceptanceReceipt struct {
	SchemaVersion              int                        `json:"schemaVersion"`
	Kind                       string                     `json:"kind"`
	Passed                     bool                       `json:"passed"`
	ReceiptPublication         string                     `json:"receiptPublication,omitempty"`
	ReceiptError               string                     `json:"receiptError,omitempty"`
	StartedAt                  string                     `json:"startedAt"`
	CompletedAt                string                     `json:"completedAt"`
	DurationMillis             int64                      `json:"durationMillis"`
	TaskCount                  int                        `json:"taskCount"`
	SuccessfulTasks            int                        `json:"successfulTasks"`
	FailedTasks                int                        `json:"failedTasks"`
	SuccessRatePercent         int                        `json:"successRatePercent"`
	AttemptCount               int                        `json:"attemptCount"`
	SuccessfulAttempts         int                        `json:"successfulAttempts"`
	FailedAttempts             int                        `json:"failedAttempts"`
	AttemptSuccessRatePercent  int                        `json:"attemptSuccessRatePercent"`
	RetriedTasks               int                        `json:"retriedTasks"`
	HumanNaturalLanguageInputs int                        `json:"humanNaturalLanguageInputs"`
	HumanLowLevelInputs        int                        `json:"humanLowLevelInputs"`
	ManualPlaceholders         int                        `json:"manualPlaceholders"`
	ManualResultWrites         int                        `json:"manualResultWrites"`
	MemberLaunches             int                        `json:"memberLaunches"`
	MemberCompletions          int                        `json:"memberCompletions"`
	ReviewerLaunches           int                        `json:"reviewerLaunches"`
	ReviewerCompletions        int                        `json:"reviewerCompletions"`
	Replacements               int                        `json:"replacements"`
	ProcessReplacements        int                        `json:"processReplacements"`
	CleanupExpected            int                        `json:"cleanupExpected"`
	CleanupCreated             int                        `json:"cleanupCreated"`
	CleanupRemoved             int                        `json:"cleanupRemoved"`
	ProviderFailureObservation string                     `json:"providerFailureObservation"`
	FailureCounts              map[string]int             `json:"failureCounts"`
	Tasks                      []LiveSoakAcceptanceTask   `json:"tasks"`
	Recovery                   LiveSoakAcceptanceRecovery `json:"recovery"`
	Boundary                   []string                   `json:"boundary"`
}

type LiveSoakAcceptanceTask struct {
	Ordinal              int    `json:"ordinal"`
	Attempt              int    `json:"attempt"`
	Pack                 string `json:"pack"`
	Passed               bool   `json:"passed"`
	FailureCode          string `json:"failureCode,omitempty"`
	FailureDetail        string `json:"failureDetail,omitempty"`
	DurationMillis       int64  `json:"durationMillis"`
	FreshCaseVerified    bool   `json:"freshCaseVerified"`
	ExistingCaseVerified bool   `json:"existingCaseVerified"`
	HumanCorrection      bool   `json:"humanCorrection"`
	RejectedReplay       bool   `json:"rejectedReplay"`
	TerminalReplay       bool   `json:"terminalReplay"`
	ManualPlaceholders   int    `json:"manualPlaceholders"`
	ManualResultWrites   int    `json:"manualResultWrites"`
	MemberLaunches       int    `json:"memberLaunches"`
	MemberCompletions    int    `json:"memberCompletions"`
	ReviewerLaunches     int    `json:"reviewerLaunches"`
	ReviewerCompletions  int    `json:"reviewerCompletions"`
	Replacements         int    `json:"replacements"`
	ProcessReplacements  int    `json:"processReplacements"`
	CleanupExpected      int    `json:"cleanupExpected"`
	CleanupCreated       int    `json:"cleanupCreated"`
	CleanupRemoved       int    `json:"cleanupRemoved"`
}

type LiveSoakAcceptanceRecovery struct {
	Passed               bool   `json:"passed"`
	FailureCode          string `json:"failureCode,omitempty"`
	FailureDetail        string `json:"failureDetail,omitempty"`
	DurationMillis       int64  `json:"durationMillis"`
	CutPoint             string `json:"cutPoint"`
	InterruptedCutPoints int    `json:"interruptedCutPoints"`
	FreshHostLaunches    int    `json:"freshHostLaunches"`
	FreshCompletions     int    `json:"freshCompletions"`
	TotalStartedReceipts int    `json:"totalStartedReceipts"`
	OutputPublications   int    `json:"outputPublications"`
	ManualPlaceholders   int    `json:"manualPlaceholders"`
	ManualResultWrites   int    `json:"manualResultWrites"`
	CleanupExpected      int    `json:"cleanupExpected"`
	CleanupCreated       int    `json:"cleanupCreated"`
	CleanupRemoved       int    `json:"cleanupRemoved"`
}

type liveSoakAcceptanceTestHooks struct {
	now         func() time.Time
	runTask     func(context.Context, LiveAcceptanceOptions) (LiveAcceptanceReceipt, error)
	runRecovery func(context.Context, LiveSupervisionAcceptanceOptions) (LiveSupervisionAcceptanceReceipt, error)
}

func RunLiveSoakAcceptance(parent context.Context, opt LiveSoakAcceptanceOptions) (LiveSoakAcceptanceReceipt, error) {
	now := time.Now
	runTask := RunLiveAcceptance
	runRecovery := RunLiveSupervisionAcceptance
	if opt.testHooks != nil {
		if opt.testHooks.now != nil {
			now = opt.testHooks.now
		}
		if opt.testHooks.runTask != nil {
			runTask = opt.testHooks.runTask
		}
		if opt.testHooks.runRecovery != nil {
			runRecovery = opt.testHooks.runRecovery
		}
	}
	started := now().UTC()
	receipt := LiveSoakAcceptanceReceipt{
		SchemaVersion:              1,
		Kind:                       "rekit-windows-live-soak-acceptance-receipt",
		StartedAt:                  started.Format(time.RFC3339Nano),
		TaskCount:                  len(liveSoakAcceptancePacks),
		HumanNaturalLanguageInputs: 2,
		HumanLowLevelInputs:        0,
		ProviderFailureObservation: "not-observed",
		FailureCounts:              map[string]int{},
		Boundary: []string{
			"the campaign sequentially runs three bounded real-Claude daily tasks through the existing exact-pack live acceptance gate",
			"only reviewer-semantic-or-lineage failure may receive one fresh-case retry; the failed first attempt remains in tasks, failure counts, session totals, duration, and cleanup totals",
			"the campaign separately runs the existing real process-start recovery gate and retains every failed result in the aggregate receipt",
			"member output and ReviewerResult bytes are never generated or rewritten by the soak orchestrator",
			"all disposable fresh, attached, and recovery cases must be removed before the campaign can pass",
			"no authority/confirmed state or heavy-tool execution is permitted",
		},
	}
	goal := strings.TrimSpace(opt.Goal)
	correction := strings.TrimSpace(opt.Correction)
	if goal == "" || correction == "" {
		return receipt, fmt.Errorf("live soak acceptance requires non-empty natural-language goal and human correction")
	}
	if err := validateLiveSoakReceiptPath(opt.ReceiptPath); err != nil {
		return receipt, err
	}

	for index, pack := range liveSoakAcceptancePacks {
		for attempt := 1; attempt <= 2; attempt++ {
			taskStarted := now()
			child, childErr := runTask(parent, LiveAcceptanceOptions{
				Pack:                           pack,
				Goal:                           fmt.Sprintf("%s This is bounded Windows soak task %d of %d for pack %s, fresh attempt %d.", goal, index+1, len(liveSoakAcceptancePacks), pack, attempt),
				Correction:                     fmt.Sprintf("%s Apply this correction only to Windows soak task %d of %d, fresh attempt %d.", correction, index+1, len(liveSoakAcceptancePacks), attempt),
				Model:                          opt.Model,
				Actor:                          opt.Actor,
				Timeout:                        opt.Timeout,
				MaxAttempts:                    opt.MaxAttempts,
				InitializationSourceExecutable: opt.InitializationSourceExecutable,
			})
			task := summarizeLiveSoakTask(index+1, attempt, pack, child, childErr, now().Sub(taskStarted))
			receipt.Tasks = append(receipt.Tasks, task)
			addLiveSoakTaskAttempt(&receipt, task)
			if task.Passed || task.FailureCode != "reviewer-semantic-or-lineage" || attempt == 2 {
				addLiveSoakTaskOutcome(&receipt, task)
				break
			}
			receipt.RetriedTasks++
		}
	}

	recoveryStarted := now()
	recoveryReceipt, recoveryErr := runRecovery(parent, LiveSupervisionAcceptanceOptions{
		Goal:                           goal + " Run the bounded Windows recovery exercise without external effects.",
		Model:                          opt.Model,
		Actor:                          opt.Actor,
		Timeout:                        opt.Timeout,
		MaxAttempts:                    opt.MaxAttempts,
		InitializationSourceExecutable: opt.InitializationSourceExecutable,
	})
	receipt.Recovery = summarizeLiveSoakRecovery(recoveryReceipt, recoveryErr, now().Sub(recoveryStarted))
	addLiveSoakRecovery(&receipt, receipt.Recovery)

	completed := now().UTC()
	receipt.CompletedAt = completed.Format(time.RFC3339Nano)
	receipt.DurationMillis = completed.Sub(started).Milliseconds()
	if receipt.TaskCount > 0 {
		receipt.SuccessRatePercent = receipt.SuccessfulTasks * 100 / receipt.TaskCount
	}
	if receipt.AttemptCount > 0 {
		receipt.AttemptSuccessRatePercent = receipt.SuccessfulAttempts * 100 / receipt.AttemptCount
	}
	receipt.Passed = receipt.SuccessfulTasks == receipt.TaskCount &&
		receipt.FailedTasks == 0 && receipt.Recovery.Passed &&
		receipt.ManualPlaceholders == 0 && receipt.ManualResultWrites == 0 &&
		receipt.CleanupExpected > 0 && receipt.CleanupCreated == receipt.CleanupExpected && receipt.CleanupRemoved == receipt.CleanupExpected
	if !receipt.Passed {
		return receipt, fmt.Errorf("Windows live soak acceptance failed: tasks=%d/%d recovery=%t cleanupCreated=%d cleanupRemoved=%d/%d", receipt.SuccessfulTasks, receipt.TaskCount, receipt.Recovery.Passed, receipt.CleanupCreated, receipt.CleanupRemoved, receipt.CleanupExpected)
	}
	return receipt, nil
}

func summarizeLiveSoakTask(ordinal, attempt int, pack string, receipt LiveAcceptanceReceipt, runErr error, duration time.Duration) LiveSoakAcceptanceTask {
	task := LiveSoakAcceptanceTask{
		Ordinal: ordinal, Attempt: attempt, Pack: pack, DurationMillis: duration.Milliseconds(),
		FreshCaseVerified:    receipt.FirstMember.AttemptID != "" && receipt.FinalAcceptance != nil,
		ExistingCaseVerified: receipt.AttachedCase.Verified && receipt.AttachedCase.MemberCutpointVerified && receipt.AttachedCase.ReviewerCutpointVerified && receipt.AttachedCase.CompletionRecoveryVerified && receipt.AttachedCase.TerminalReplay && receipt.AttachedCase.ReplayLaunches == 0,
		HumanCorrection:      receipt.CorrectionEventID != "" && receipt.ReplacementMember.Generation > receipt.FirstMember.Generation && receipt.ReplacementMember.CorrectionSourceID == receipt.CorrectionEventID,
		RejectedReplay:       receipt.RejectedReplay.Verified && receipt.RejectedReplay.SessionLaunches == 0 && receipt.RejectedReplay.SessionCompletions == 0,
		TerminalReplay:       receipt.TerminalReplay.Verified && receipt.TerminalReplay.SessionLaunches == 0 && receipt.TerminalReplay.SessionCompletions == 0,
		ManualPlaceholders:   receipt.ManualPlaceholders, ManualResultWrites: receipt.ManualResultWrites,
		MemberLaunches: receipt.MemberLaunches, MemberCompletions: receipt.MemberCompletions,
		ReviewerLaunches: receipt.ReviewerLaunches, ReviewerCompletions: receipt.ReviewerCompletions,
		ProcessReplacements: receipt.Replacements,
	}
	if task.HumanCorrection {
		task.Replacements = receipt.ReplacementMember.Generation - receipt.FirstMember.Generation
	}
	if strings.TrimSpace(receipt.CaseRoot) != "" {
		task.CleanupExpected++
		if receipt.CaseCreated {
			task.CleanupCreated++
			if receipt.Cleanup == "removed" {
				task.CleanupRemoved++
			}
		}
	}
	if strings.TrimSpace(receipt.AttachedCase.CaseRoot) != "" {
		task.CleanupExpected++
		if receipt.AttachedCase.CaseCreated {
			task.CleanupCreated++
			if receipt.AttachedCase.Cleanup == "removed" {
				task.CleanupRemoved++
			}
		}
	}
	task.Passed = runErr == nil && receipt.Passed && task.FreshCaseVerified && task.ExistingCaseVerified &&
		task.HumanCorrection && task.RejectedReplay && task.TerminalReplay && task.ManualPlaceholders == 0 &&
		task.ManualResultWrites == 0 && task.MemberLaunches >= 3 && task.MemberCompletions >= 3 &&
		task.ReviewerLaunches >= 2 && task.ReviewerCompletions >= 2 && task.Replacements >= 1 &&
		task.CleanupExpected == 2 && task.CleanupCreated == 2 && task.CleanupRemoved == 2
	if !task.Passed {
		task.FailureCode, task.FailureDetail = classifyLiveSoakTaskFailure(receipt, runErr)
	}
	return task
}

func summarizeLiveSoakRecovery(receipt LiveSupervisionAcceptanceReceipt, runErr error, duration time.Duration) LiveSoakAcceptanceRecovery {
	recovery := LiveSoakAcceptanceRecovery{
		DurationMillis: duration.Milliseconds(), CutPoint: receipt.CutPoint,
		InterruptedCutPoints: len(receipt.InterruptedCutPoints), FreshHostLaunches: receipt.FreshHostLaunches,
		FreshCompletions: receipt.FreshCompletions, TotalStartedReceipts: receipt.TotalStartedReceipts,
		OutputPublications: receipt.OutputPublications, ManualPlaceholders: receipt.ManualPlaceholders,
		ManualResultWrites: receipt.ManualResultWrites,
	}
	if strings.TrimSpace(receipt.CaseRoot) != "" {
		recovery.CleanupExpected = 1
		if receipt.CaseCreated {
			recovery.CleanupCreated = 1
			if receipt.Cleanup == "removed" {
				recovery.CleanupRemoved = 1
			}
		}
	}
	recovery.Passed = runErr == nil && receipt.Passed && receipt.FirstHostInterrupted &&
		receipt.CutPoint == "process-start" && sameLiveAcceptanceStrings(receipt.InterruptedCutPoints, []string{"process-start", "output-returned", "result-first", "submission", "intake"}) &&
		receipt.FreshHostLaunches == 0 && receipt.FreshCompletions == 1 &&
		receipt.TotalStartedReceipts == 1 && receipt.OutputPublications == 1 &&
		receipt.RunID != "" && receipt.SessionID != "" && receipt.AttemptSHA256 != "" &&
		receipt.ManualPlaceholders == 0 &&
		receipt.ManualResultWrites == 0 && recovery.CleanupExpected == 1 && recovery.CleanupCreated == 1 && recovery.CleanupRemoved == 1
	if !recovery.Passed {
		recovery.FailureCode, recovery.FailureDetail = classifyLiveSoakFailure(runErr, "soak-recovery-invariant-failed")
	}
	return recovery
}

func addLiveSoakTaskAttempt(receipt *LiveSoakAcceptanceReceipt, task LiveSoakAcceptanceTask) {
	receipt.AttemptCount++
	if task.Passed {
		receipt.SuccessfulAttempts++
	} else {
		receipt.FailedAttempts++
		receipt.FailureCounts[task.FailureCode]++
		observeLiveSoakProviderFailure(receipt, task.FailureCode)
	}
	receipt.ManualPlaceholders += task.ManualPlaceholders
	receipt.ManualResultWrites += task.ManualResultWrites
	receipt.MemberLaunches += task.MemberLaunches
	receipt.MemberCompletions += task.MemberCompletions
	receipt.ReviewerLaunches += task.ReviewerLaunches
	receipt.ReviewerCompletions += task.ReviewerCompletions
	receipt.Replacements += task.Replacements
	receipt.ProcessReplacements += task.ProcessReplacements
	receipt.CleanupExpected += task.CleanupExpected
	receipt.CleanupCreated += task.CleanupCreated
	receipt.CleanupRemoved += task.CleanupRemoved
}

func addLiveSoakTaskOutcome(receipt *LiveSoakAcceptanceReceipt, task LiveSoakAcceptanceTask) {
	if task.Passed {
		receipt.SuccessfulTasks++
	} else {
		receipt.FailedTasks++
	}
}

func addLiveSoakRecovery(receipt *LiveSoakAcceptanceReceipt, recovery LiveSoakAcceptanceRecovery) {
	if !recovery.Passed {
		receipt.FailureCounts[recovery.FailureCode]++
		observeLiveSoakProviderFailure(receipt, recovery.FailureCode)
	}
	receipt.ManualPlaceholders += recovery.ManualPlaceholders
	receipt.ManualResultWrites += recovery.ManualResultWrites
	receipt.MemberLaunches += recovery.TotalStartedReceipts
	receipt.MemberCompletions += recovery.FreshCompletions
	receipt.CleanupExpected += recovery.CleanupExpected
	receipt.CleanupCreated += recovery.CleanupCreated
	receipt.CleanupRemoved += recovery.CleanupRemoved
}

func classifyLiveSoakTaskFailure(receipt LiveAcceptanceReceipt, err error) (string, string) {
	if receipt.Failure != nil && strings.TrimSpace(receipt.Failure.Code) != "" {
		code := strings.TrimSpace(receipt.Failure.Code)
		detail := truncate(oneLine(receipt.Failure.Detail), 1024)
		if detail == "" {
			detail = code
		}
		return liveSoakTypedFailureClass(*receipt.Failure), detail
	}
	var firstCode, firstDetail string
	for _, sessions := range [][]LiveAcceptanceSession{receipt.MemberSessions, receipt.ReviewerSessions} {
		for _, session := range sessions {
			if session.Failure == nil || strings.TrimSpace(session.Failure.Code) == "" {
				continue
			}
			code := strings.TrimSpace(session.Failure.Code)
			detail := truncate(oneLine(session.Failure.Detail), 1024)
			if detail == "" {
				detail = code
			}
			if liveSoakProviderFailureObserved(*session.Failure) {
				return code, detail
			}
			if firstCode == "" {
				firstCode, firstDetail = code, detail
			}
		}
	}
	if firstCode != "" {
		return firstCode, firstDetail
	}
	return classifyLiveSoakFailure(err, "soak-task-invariant-failed")
}

func liveSoakTypedFailureClass(failure FailureDiagnosis) string {
	code := strings.ToLower(strings.TrimSpace(failure.Code))
	stage := strings.ToLower(strings.TrimSpace(failure.Stage))
	detail := strings.ToLower(strings.TrimSpace(failure.Detail))
	switch {
	case liveSoakProviderFailureObserved(failure):
		return strings.TrimSpace(failure.Code)
	case strings.Contains(code, "intake"), strings.Contains(stage, "intake"), strings.Contains(code, "structured-output"), containsAny(detail, "contract", "manifest", "intake"):
		return "strict-contract-or-intake"
	case strings.Contains(code, "timeout"), strings.Contains(stage, "timeout"):
		return "process-timeout"
	case strings.Contains(code, "executable"), strings.Contains(stage, "executable"):
		return "executable-resolution"
	case strings.Contains(code, "cleanup"), strings.Contains(stage, "cleanup"):
		return "cleanup-failed"
	case containsAny(detail, "reviewer", "rejection", "accepted"):
		return "reviewer-semantic-or-lineage"
	default:
		return code
	}
}

func classifyLiveSoakFailure(err error, fallback string) (string, string) {
	if err == nil {
		return fallback, fallback
	}
	detail := truncate(oneLine(err.Error()), 1024)
	text := strings.ToLower(detail)
	switch {
	case containsAny(text, "not logged in", "authentication", "unauthorized", "invalid api key", "oauth"):
		return "provider-authentication", detail
	case containsAny(text, "quota", "usage limit", "rate limit", "credit balance", "billing"):
		return "provider-quota", detail
	case containsAny(text, "model not found", "model is not available", "invalid model", "unsupported model"):
		return "provider-model", detail
	case strings.Contains(text, "timeout") || strings.Contains(text, "timed out"):
		return "process-timeout", detail
	case strings.Contains(text, "executable"):
		return "executable-resolution", detail
	case strings.Contains(text, "cleanup") || strings.Contains(text, "clean "):
		return "cleanup-failed", detail
	case strings.Contains(text, "contract") || strings.Contains(text, "manifest") || strings.Contains(text, "intake"):
		return "strict-contract-or-intake", detail
	case strings.Contains(text, "reviewer") || strings.Contains(text, "rejection") || strings.Contains(text, "accepted"):
		return "reviewer-semantic-or-lineage", detail
	default:
		return "host-runtime", detail
	}
}

func liveSoakProviderFailureCode(code string) bool {
	code = strings.ToLower(strings.TrimSpace(code))
	return strings.HasPrefix(code, "provider-") || code == "claude-authentication-failed" || code == "claude-quota-unavailable" || code == "claude-model-unavailable"
}

func liveSoakProviderFailureObserved(failure FailureDiagnosis) bool {
	return strings.EqualFold(strings.TrimSpace(failure.ProviderObservation), "observed") || liveSoakProviderFailureCode(failure.Code)
}

func observeLiveSoakProviderFailure(receipt *LiveSoakAcceptanceReceipt, code string) {
	if liveSoakProviderFailureCode(code) {
		receipt.ProviderFailureObservation = "observed"
	}
}

func validateLiveSoakReceiptPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("live soak acceptance requires -receipt outside the repository")
	}
	absolute, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return err
	}
	repoRoot, err := currentRepoRoot()
	if err != nil {
		return err
	}
	if liveAcceptancePathWithin(repoRoot, absolute) {
		return fmt.Errorf("live soak acceptance receipt must be outside the repository: %s", absolute)
	}
	return nil
}

func WriteLiveSoakAcceptanceReceipt(path string, receipt LiveSoakAcceptanceReceipt) error {
	if err := validateLiveSoakReceiptPath(path); err != nil {
		return err
	}
	path, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return err
	}
	anchorPath := filepath.VolumeName(path) + string(filepath.Separator)
	if anchorPath == "" {
		anchorPath = string(filepath.Separator)
	}
	rel, err := filepath.Rel(anchorPath, path)
	if err != nil || filepath.IsAbs(rel) || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("live soak acceptance receipt path escapes its volume root: %s", path)
	}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := rekitfs.WriteNewExclusiveRegularFileAnchored(anchorPath, filepath.ToSlash(rel), "live soak acceptance receipt", data); err != nil {
		return fmt.Errorf("publish live soak acceptance receipt %s: %w", path, err)
	}
	return nil
}
