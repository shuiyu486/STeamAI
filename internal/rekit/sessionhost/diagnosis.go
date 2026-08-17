package sessionhost

import (
	"errors"
	"fmt"
	"strings"
)

const (
	failureStateTerminal    = "terminal"
	failureStateReplaceable = "replaceable"
	failureStateRecoverable = "recoverable"
)

type FailureDiagnosis struct {
	Code                string `json:"code"`
	Stage               string `json:"stage"`
	State               string `json:"state"`
	Terminal            bool   `json:"terminal"`
	Replaceable         bool   `json:"replaceable"`
	Recoverable         bool   `json:"recoverable"`
	MutationBoundary    string `json:"mutationBoundary"`
	MutationApplied     bool   `json:"mutationApplied"`
	AttemptsUsed        int    `json:"attemptsUsed,omitempty"`
	AttemptsLimit       int    `json:"attemptsLimit,omitempty"`
	NextAction          string `json:"nextAction"`
	Detail              string `json:"detail"`
	ProviderObservation string `json:"providerObservation,omitempty"`
}

type diagnosedError struct {
	code             string
	stage            string
	mutationBoundary string
	mutationApplied  bool
	nextAction       string
	cause            error
}

func (err *diagnosedError) Error() string { return err.cause.Error() }
func (err *diagnosedError) Unwrap() error { return err.cause }

func hostError(code, stage, mutationBoundary, nextAction string, mutationApplied bool, err error) error {
	if err == nil {
		return nil
	}
	return &diagnosedError{
		code: code, stage: stage, mutationBoundary: mutationBoundary,
		mutationApplied: mutationApplied, nextAction: nextAction, cause: err,
	}
}

func diagnosisForError(err error, attemptsUsed, attemptsLimit, appliedSteps int) *FailureDiagnosis {
	if err == nil {
		return nil
	}
	var diagnosed *diagnosedError
	if errors.As(err, &diagnosed) {
		state := failureStateRecoverable
		if diagnosed.code == "claude-attempt-limit-reached" {
			state = failureStateTerminal
		}
		return newFailureDiagnosis(
			diagnosed.code, diagnosed.stage, state, diagnosed.mutationBoundary,
			diagnosed.mutationApplied, attemptsUsed, attemptsLimit, diagnosed.nextAction,
			diagnosed.cause.Error(), "",
		)
	}

	text := strings.ToLower(err.Error())
	code, stage, nextAction := "claude-host-operation-failed", "host-runtime", "Refresh status, inspect the reported stage, and rerun the same host command only after the cause is corrected."
	switch {
	case strings.Contains(text, "attempt limit reached"):
		code, stage, nextAction = "claude-attempt-limit-reached", "attempt-control", "Refresh status and correct the last failure before explicitly starting another bounded host run."
	case strings.Contains(text, "claude code executable"), strings.Contains(text, "claude executable"):
		code, stage, nextAction = "claude-executable-unavailable", "executable-resolution", "Install or repair the canonical signed Claude Code executable, then rerun the same host command."
	case strings.Contains(text, "submission"), strings.Contains(text, "publish claude"):
		code, stage, nextAction = "claude-submission-failed", "submission-publication", "Refresh status, preserve any already-published result artifacts, and rerun the host to recover the exact current submission step."
	case strings.Contains(text, "intake"), strings.Contains(text, "relay committed"), strings.Contains(text, "nested resume"):
		code, stage, nextAction = "claude-intake-failed", "runtime-intake", "Refresh status and rerun the host so the deterministic runtime can resume from the committed submission boundary."
	}
	mutationApplied := appliedSteps > 0
	boundary := "none"
	if mutationApplied {
		boundary = "durable-runtime-step-may-have-committed"
	}
	state := failureStateRecoverable
	if code == "claude-attempt-limit-reached" {
		state = failureStateTerminal
	}
	return newFailureDiagnosis(code, stage, state, boundary, mutationApplied, attemptsUsed, attemptsLimit, nextAction, err.Error(), "")
}

func diagnosisForClaudeRun(run claudeRun, outcome, sessionKind string, attemptsUsed, attemptsLimit int) *FailureDiagnosis {
	if run.success() {
		return nil
	}
	code, stage, nextAction, providerObservation := run.failureCode, "process-result", "Refresh status and rerun the host so it can create the next bounded replacement attempt.", ""
	text := strings.ToLower(strings.Join([]string{run.stderrTail, run.stdoutTail, run.envelope.Result, errorText(run.spawnErr), errorText(run.waitErr)}, "\n"))

	switch {
	case run.failureDetail != "":
		structured := diagnosisForStructuredResult(sessionKind, errors.New(run.failureDetail), attemptsUsed, attemptsLimit)
		structured.MutationApplied = run.started
		return structured
	case len(run.envelope.PermissionDenials) > 0:
		code, stage, nextAction, providerObservation = "claude-permission-denied", "tool-permission", "Review the denied read-only access and task inputs without bypassing permissions, then rerun the host.", "observed"
	case code == "claude-invalid-envelope":
		stage, nextAction = "envelope-validation", "Refresh status and rerun the host so the malformed process result is replaced within the attempt limit."
	case code == "claude-session-id-mismatch":
		stage, nextAction = "session-validation", "Refresh status and rerun the host so the mismatched session is replaced within the attempt limit."
	case code == "claude-supervision-fenced":
		stage, nextAction = "supervision-fence", "Refresh status and rerun the host so the durable fence is handled within the current attempt limit."
	case run.timedOut:
		code, stage, nextAction = "claude-timeout", "process-wait", "Increase -timeout or narrow the goal, then rerun the host from refreshed status."
	case run.spawnErr != nil:
		code, stage, nextAction = "claude-spawn-failed", "process-spawn", "Repair executable access or process launch policy, then rerun the same host command."
	case containsAny(text, "not logged in", "please log in", "authentication", "unauthorized", "invalid api key", "oauth"):
		code, stage, nextAction, providerObservation = "claude-authentication-failed", "provider-authentication", "Run Claude Code interactively to restore authentication, then rerun the same host command.", "observed"
	case containsAny(text, "quota", "usage limit", "rate limit", "credit balance", "billing"):
		code, stage, nextAction, providerObservation = "claude-quota-unavailable", "provider-availability", "Resolve the account quota or billing limit, then rerun the same host command.", "observed"
	case containsAny(text, "model not found", "model is not available", "invalid model", "unsupported model", "does not have access to model"):
		code, stage, nextAction, providerObservation = "claude-model-unavailable", "provider-availability", "Choose an available -model or omit the model override, then rerun the same host command.", "observed"
	case run.exitCode != 0 || run.waitErr != nil:
		code, stage, nextAction = "claude-nonzero-exit", "process-exit", "Inspect the bounded process diagnostics, correct the cause, and rerun the host from refreshed status."
	case code == "":
		code, stage, nextAction = "claude-result-envelope-failed", "envelope-validation", "Refresh status and rerun the host so the unsuccessful result envelope is replaced within the attempt limit."
	}
	state := failureStateReplaceable
	mutationBoundary := "session-launch-recorded"
	mutationApplied := run.started
	if outcome == "launch-failed" {
		mutationBoundary = "durable-launch-failure-recorded"
		mutationApplied = true
		if attemptsUsed < attemptsLimit {
			state = failureStateRecoverable
			nextAction = "Rerun the same host command; the deterministic runtime has recorded the failed launch and can continue with the next bounded attempt."
		}
	}
	if attemptsLimit > 0 && attemptsUsed >= attemptsLimit {
		state = failureStateTerminal
		nextAction = "Refresh status and correct the reported cause before explicitly starting another bounded host run."
	}
	return newFailureDiagnosis(code, stage, state, mutationBoundary, mutationApplied, attemptsUsed, attemptsLimit, nextAction, run.failureReason(), providerObservation)
}

func diagnosisForReportedFailure(pkgKind, reason string, attemptsUsed, attemptsLimit int) *FailureDiagnosis {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "Claude " + pkgKind + " reported failure without a reason"
	}
	return newFailureDiagnosis(
		"claude-reported-failure",
		"structured-result",
		failureStateTerminal,
		"failed-submission-recorded",
		true,
		attemptsUsed,
		attemptsLimit,
		"Refresh status and review the reported reason before explicitly starting another bounded host run.",
		reason,
		"reported",
	)
}

func diagnosisForStructuredResult(pkgKind string, validationErr error, attemptsUsed, attemptsLimit int) *FailureDiagnosis {
	if validationErr == nil {
		return nil
	}
	code := "claude-invalid-structured-output"
	stage := "structured-output-validation"
	detail := validationErr.Error()
	if strings.Contains(strings.ToLower(detail), "session mismatch") {
		code, stage = "claude-session-id-mismatch", "session-validation"
	}
	state := failureStateReplaceable
	nextAction := fmt.Sprintf("Refresh status and rerun the host so the invalid %s result is replaced within the attempt limit.", pkgKind)
	if attemptsLimit > 0 && attemptsUsed >= attemptsLimit {
		state = failureStateTerminal
		nextAction = "Refresh status and correct the result contract failure before explicitly starting another bounded host run."
	}
	return newFailureDiagnosis(code, stage, state, "session-launch-recorded", true, attemptsUsed, attemptsLimit, nextAction, detail, "")
}

func newFailureDiagnosis(code, stage, state, mutationBoundary string, mutationApplied bool, attemptsUsed, attemptsLimit int, nextAction, detail, providerObservation string) *FailureDiagnosis {
	return &FailureDiagnosis{
		Code: code, Stage: stage, State: state,
		Terminal: state == failureStateTerminal, Replaceable: state == failureStateReplaceable, Recoverable: state == failureStateRecoverable,
		MutationBoundary: mutationBoundary, MutationApplied: mutationApplied,
		AttemptsUsed: attemptsUsed, AttemptsLimit: attemptsLimit,
		NextAction: nextAction, Detail: truncate(oneLine(detail), 1024), ProviderObservation: providerObservation,
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
