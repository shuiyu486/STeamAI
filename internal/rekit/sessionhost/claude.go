package sessionhost

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/packmemoryconsumption"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewerresult"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewersession"
	rekitruntime "github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

const (
	maxOutputBytes       = 4 * 1024 * 1024
	maxOutputs           = 64
	maxClaudeStdoutBytes = 16 * 1024 * 1024
	maxDiagnosticBytes   = 32 * 1024
)

type claudeRun struct {
	envelope         claudeEnvelope
	sessionID        string
	structuredOutput json.RawMessage
	failureCode      string
	failureDetail    string
	spawnErr         error
	waitErr          error
	startCallbackErr error
	started          bool
	recovered        bool
	exitCode         int
	timedOut         bool
	duration         time.Duration
	observedAt       string
	stdoutTail       string
	stderrTail       string
}

type claudeEnvelope struct {
	Type              string          `json:"type"`
	Subtype           string          `json:"subtype"`
	IsError           bool            `json:"is_error"`
	SessionID         string          `json:"session_id"`
	Result            string          `json:"result"`
	StructuredOutput  json.RawMessage `json:"structured_output"`
	PermissionDenials []any           `json:"permission_denials"`
}

type memberResponse struct {
	Outcome           string         `json:"outcome"`
	Summary           string         `json:"summary"`
	Reason            string         `json:"reason"`
	Outputs           []memberOutput `json:"outputs"`
	ReviewerItemsPath string         `json:"reviewerItemsPath"`
}

type memberOutput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type reviewerResponse struct {
	Outcome string          `json:"outcome"`
	Result  json.RawMessage `json:"result"`
	Reason  string          `json:"reason"`
}

type evidenceReviewResponse struct {
	Decision            string   `json:"decision"`
	Summary             string   `json:"summary"`
	Reason              string   `json:"reason"`
	EvidenceRefs        []string `json:"evidenceRefs"`
	SelectedEvidenceRef string   `json:"selectedEvidenceRef"`
	ObservationEventID  string   `json:"observationEventId"`
	ReceiptSHA256       string   `json:"receiptSha256"`
}

func runClaude(parent context.Context, opt Options, pkg mission.CurrentLoopExternalSessionHarnessPackage, sessionID string, started func() error) claudeRun {
	begin := time.Now()
	ctx, cancel := context.WithTimeout(parent, opt.Timeout)
	defer cancel()

	executionLease := opt.projectExecutionLease
	executionOwned := false
	var err error
	if executionLease == nil {
		executionLease, err = acquireSharedForCurrentProject(opt.Target)
		if err != nil {
			return claudeRun{spawnErr: err, duration: time.Since(begin)}
		}
		executionOwned = executionLease != nil
		if executionOwned {
			defer executionLease.Unlock()
		}
	} else if err := executionLease.ValidateFor(opt.Target); err != nil {
		return claudeRun{spawnErr: err, duration: time.Since(begin)}
	}
	opt.projectExecutionLease = executionLease

	prompt, schema, err := claudeRequest(opt.Target, pkg, sessionID)
	if err != nil {
		return claudeRun{spawnErr: err, duration: time.Since(begin)}
	}
	args := []string{
		"--safe-mode",
		"-p", prompt,
		"--session-id", sessionID,
		"--output-format", "json",
		"--json-schema", schema,
		"--permission-mode", "dontAsk",
		"--tools", "Read",
		"--max-budget-usd", "2.00",
	}
	if strings.TrimSpace(opt.Model) != "" {
		args = append(args, "--model", strings.TrimSpace(opt.Model))
	}
	additionalReadDirs, err := claudeAdditionalReadDirs(opt, pkg)
	if err != nil {
		return claudeRun{spawnErr: err, duration: time.Since(begin)}
	}
	for _, dir := range additionalReadDirs {
		args = append(args, "--add-dir", dir)
	}
	launchBinding, err := acquireClaudeExecutableLaunchBinding(opt)
	if err != nil {
		return claudeRun{spawnErr: err, duration: time.Since(begin)}
	}
	if launchBinding != nil {
		defer launchBinding.Close()
	}
	cmd := exec.CommandContext(ctx, opt.ClaudePath, args...)
	if err := configureTrustedClaudeCommand(cmd, launchBinding); err != nil {
		return claudeRun{spawnErr: err, duration: time.Since(begin)}
	}
	cmd.Dir = opt.Target
	cmd.Stdin = nil
	var stdout limitedBuffer
	var stderr boundedBuffer
	stdout.limit = maxClaudeStdoutBytes
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	var containment *claudeProcessContainment
	startProcess := func() error {
		if executionLease != nil {
			if err := executionLease.ValidateFor(opt.Target); err != nil {
				return fmt.Errorf("revalidate project execution lease immediately before launch: %w", err)
			}
			if recovery, err := inspectCurrentSyncRecoveryForHost(opt.Target); err != nil {
				return fmt.Errorf("recheck current project update immediately before launch: %w", err)
			} else if recovery.Pending {
				return fmt.Errorf("%s; %s", recovery.Now, recovery.Next)
			}
		}
		if launchBinding != nil {
			if err := launchBinding.Validate(); err != nil {
				return fmt.Errorf("revalidate trusted Claude namespace immediately before launch: %w", err)
			}
		}
		if err := cmd.Start(); err != nil {
			return err
		}
		var err error
		containment, err = validateContainAndResumeTrustedClaudeProcess(cmd.Process, launchBinding)
		if err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return err
		}
		return nil
	}
	if pkg.SessionKind == "member" {
		inspection, err := validateClaudeMemberLaunchInput(opt.Target, *pkg.Launch)
		if err != nil {
			return claudeRun{spawnErr: err, duration: time.Since(begin)}
		}
		ctx, err := rekitruntime.NewWithCwd(opt.Target, opt.Pack, opt.Target)
		if err != nil {
			return claudeRun{spawnErr: err, duration: time.Since(begin)}
		}
		err = packmemoryconsumption.WithCurrentConsumerAttemptLease(ctx.RepoRoot, opt.Target, ctx.Pack, inspection, startProcess)
		if err != nil {
			return claudeRun{spawnErr: err, duration: time.Since(begin), stdoutTail: stdout.String(), stderrTail: stderr.String()}
		}
	} else if err := startProcess(); err != nil {
		return claudeRun{spawnErr: err, duration: time.Since(begin), stdoutTail: stdout.String(), stderrTail: stderr.String()}
	}
	if containment != nil {
		defer containment.Close()
	}
	processStarted := true
	if started != nil {
		if err := started(); err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return claudeRun{startCallbackErr: err, started: processStarted, duration: time.Since(begin), stdoutTail: stdout.String(), stderrTail: stderr.String()}
		}
	}
	waitErr := cmd.Wait()
	run := claudeRun{
		waitErr: waitErr, started: processStarted, duration: time.Since(begin), timedOut: errors.Is(ctx.Err(), context.DeadlineExceeded),
		stdoutTail: stdout.String(), stderrTail: stderr.String(),
	}
	if cmd.ProcessState != nil {
		run.exitCode = cmd.ProcessState.ExitCode()
	}
	if stdout.exceeded {
		run.failureCode = "claude-invalid-envelope"
		run.waitErr = errors.Join(run.waitErr, fmt.Errorf("Claude Code JSON result exceeded %d bytes", maxClaudeStdoutBytes))
		return run
	}
	if err := json.Unmarshal(stdout.Bytes(), &run.envelope); err != nil {
		run.failureCode = "claude-invalid-envelope"
		if waitErr == nil {
			run.waitErr = fmt.Errorf("decode Claude Code JSON result: %w", err)
		} else {
			run.waitErr = errors.Join(waitErr, fmt.Errorf("decode Claude Code JSON result: %w", err))
		}
		return run
	}
	run.sessionID = strings.TrimSpace(run.envelope.SessionID)
	if run.sessionID == "" {
		run.failureCode = "claude-session-id-mismatch"
		run.waitErr = errors.Join(run.waitErr, fmt.Errorf("Claude Code result omitted the requested session id"))
	} else if run.sessionID != sessionID {
		run.failureCode = "claude-session-id-mismatch"
		run.waitErr = errors.Join(run.waitErr, fmt.Errorf("Claude Code session id drift: got %s want %s", run.sessionID, sessionID))
	}
	run.structuredOutput = append([]byte{}, run.envelope.StructuredOutput...)
	return run
}

func validateClaudeMemberLaunchInput(
	caseRoot string,
	launch mission.CurrentLoopExternalSessionHarnessLaunch,
) (memberexecution.Inspection, error) {
	inputPath, err := anchoredPath(caseRoot, launch.Input.Path)
	if err != nil {
		return memberexecution.Inspection{}, err
	}
	input, err := rekitfs.ReadStableRegularFileAnchored(
		caseRoot,
		inputPath,
		"Claude member task context",
		1<<20,
	)
	if err != nil {
		return memberexecution.Inspection{}, err
	}
	if !strings.EqualFold(bytesSHA256(input), launch.Input.SHA256) {
		return memberexecution.Inspection{}, fmt.Errorf(
			"Claude member task context sha256 changed before launch",
		)
	}
	var task memberexecution.TaskContext
	if err := strictJSON(input, &task); err != nil {
		return memberexecution.Inspection{}, fmt.Errorf(
			"decode Claude member task context before launch: %w",
			err,
		)
	}
	inspection, err := memberexecution.Inspect(
		caseRoot,
		task.Owner.Lane,
		task.AttemptID,
	)
	if err != nil {
		return memberexecution.Inspection{}, err
	}
	if !casePathEqual(inspection.TaskContextPath, inputPath) ||
		!strings.EqualFold(
			inspection.TaskContextSHA256,
			launch.Input.SHA256,
		) {
		return memberexecution.Inspection{}, fmt.Errorf(
			"Claude member launch input does not match the durable task context",
		)
	}
	if err := memberexecution.ValidateActionableTaskContext(
		caseRoot,
		inspection,
	); err != nil {
		return memberexecution.Inspection{}, err
	}
	return inspection, nil
}

func claudeAdditionalReadDirs(opt Options, pkg mission.CurrentLoopExternalSessionHarnessPackage) ([]string, error) {
	if pkg.SessionKind != "reviewer" {
		return nil, nil
	}
	if !casePathEqual(opt.Target, pkg.CaseRoot) {
		return nil, fmt.Errorf("Claude reviewer case root changed before launch")
	}
	ctx, err := rekitruntime.NewWithCwd(opt.Target, opt.Pack, opt.Target)
	if err != nil {
		return nil, fmt.Errorf("resolve Claude reviewer attached kit root: %w", err)
	}
	return []string{ctx.RepoRoot}, nil
}

func (run claudeRun) success() bool {
	return run.failureDetail == "" && run.spawnErr == nil && run.waitErr == nil && !run.timedOut && run.exitCode == 0 &&
		run.sessionID != "" && run.envelope.Type == "result" && run.envelope.Subtype == "success" && !run.envelope.IsError &&
		len(run.envelope.PermissionDenials) == 0 && len(bytes.TrimSpace(run.structuredOutput)) > 0
}

func (run claudeRun) failureReason() string {
	parts := []string{}
	if strings.TrimSpace(run.failureDetail) != "" {
		parts = append(parts, strings.TrimSpace(run.failureDetail))
	}
	for _, value := range []string{errorText(run.spawnErr), errorText(run.waitErr)} {
		if value != "" {
			parts = append(parts, value)
		}
	}
	if run.timedOut {
		parts = append(parts, "Claude Code timed out")
	}
	if run.stderrTail != "" {
		parts = append(parts, "stderr: "+oneLine(run.stderrTail))
	}
	if len(parts) == 0 {
		parts = append(parts, "Claude Code did not return a successful structured result")
	}
	return truncate(strings.Join(parts, "; "), 1024)
}

func sessionResult(run claudeRun, attemptGeneration, launchOrdinal int, reservationID, kind, outcome string, attemptsLimit int) Session {
	diagnostics := []string{}
	if detail := run.failureReason(); !run.success() && detail != "" {
		diagnostics = append(diagnostics, "failure: "+truncate(oneLine(detail), 1024))
	}
	if run.stdoutTail != "" && !run.success() {
		diagnostics = append(diagnostics, "stdout: "+truncate(oneLine(run.stdoutTail), 1024))
	}
	if run.stderrTail != "" {
		diagnostics = append(diagnostics, "stderr: "+truncate(oneLine(run.stderrTail), 1024))
	}
	attemptsUsed := claudeRunAttemptsUsed(attemptGeneration, launchOrdinal)
	failure := diagnosisForClaudeRun(run, outcome, kind, attemptsUsed, attemptsLimit)
	if failure == nil && run.failureDetail != "" {
		failure = diagnosisForStructuredResult(kind, errors.New(run.failureDetail), attemptsUsed, attemptsLimit)
	}
	if failure == nil && (outcome == "failed" || outcome == "failed-recovered") {
		failure = diagnosisForReportedFailure(kind, claudeReportedFailureReason(kind, run), attemptsUsed, attemptsLimit)
	}
	return Session{
		Started: run.started, Recovered: run.recovered, AttemptGeneration: attemptGeneration, RunLaunchOrdinal: launchOrdinal,
		ReservationID: reservationID, SessionID: run.sessionID, SessionKind: kind,
		Outcome: outcome, ExitCode: run.exitCode, TimedOut: run.timedOut,
		ResultSubtype: run.envelope.Subtype, ResultIsError: run.envelope.IsError,
		DurationMillis: run.duration.Milliseconds(), PermissionDenials: run.envelope.PermissionDenials,
		Failure: failure, Diagnostics: diagnostics,
	}
}

func claudeReportedFailureReason(kind string, run claudeRun) string {
	if !run.success() {
		return run.failureReason()
	}
	switch kind {
	case "member":
		var response memberResponse
		if strictJSON(run.structuredOutput, &response) == nil && response.Outcome == "failed" {
			return strings.TrimSpace(response.Reason)
		}
	case "reviewer":
		var response reviewerResponse
		if strictJSON(run.structuredOutput, &response) == nil && response.Outcome == "failed" {
			return strings.TrimSpace(response.Reason)
		}
	}
	return run.failureReason()
}

func claudeRequest(caseRoot string, pkg mission.CurrentLoopExternalSessionHarnessPackage, sessionID string) (string, string, error) {
	if pkg.Launch == nil || !pkg.Launch.Ready {
		return "", "", fmt.Errorf("Claude launch package is not ready")
	}
	if pkg.Launch.Attempt.Session != sessionID {
		return "", "", fmt.Errorf("Claude launch reservation does not match the durable attempt")
	}
	inputPath, err := anchoredPath(caseRoot, pkg.Launch.Input.Path)
	if err != nil {
		return "", "", err
	}
	input, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, inputPath, "Claude host input", 1<<20)
	if err != nil {
		return "", "", err
	}
	if !strings.EqualFold(bytesSHA256(input), pkg.Launch.Input.SHA256) {
		return "", "", fmt.Errorf("Claude host input sha256 mismatch")
	}
	common := fmt.Sprintf("Read the immutable task input at the exact absolute path %q using the Read tool before answering. Follow it exactly within its no-authority/no-heavy-tool boundary. Resolve any case-relative evidence paths inside that input from the current case root %q. Your actual Claude Code session ID is %s. Return only the requested structured output through the schema. Use Read only for the immutable input and the minimum listed evidence needed for the verdict; do not explore unrelated files or repeat reads. After those bounded reads, immediately return the structured output and never end the response with another Read call. Do not write external-session result or submission files; the host will persist your real returned bytes.", inputPath, caseRoot, sessionID)
	if pkg.SessionKind == "member" {
		schema := `{"type":"object","properties":{"outcome":{"type":"string","enum":["returned","failed"]},"summary":{"type":"string"},"reason":{"type":"string"},"outputs":{"type":"array","maxItems":64,"items":{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"],"additionalProperties":false}},"reviewerItemsPath":{"type":"string"}},"required":["outcome","summary","reason","outputs","reviewerItemsPath"],"additionalProperties":false}`
		return common + " For outcome=returned provide a non-empty summary and at least one bounded output path/content pair. Read the task context outputContract.fields and make the returned analysis explicitly address every field using the currently inspected evidence; use a clear unknown, none, or not-applicable value when the evidence supports no stronger claim, and never invent a value. If the task context contains a correction and historical Reviewer rejection, treat them as instructions and provenance for replacing the old result: cite the current correction evidence path and report the corrected current analysis rather than repeating the historical gap as current. The independent Reviewer is a later runtime-owned segment: do not require a Reviewer result inside the member output, do not defer merely because that later review has not happened, and do not claim that it has happened. Judge only whether the member evidence and analysis you can produce now support the requested factual conclusion; the later Reviewer will independently accept or reject that output. An unmet semantic acceptance requirement or missing bounded evidence is not a process failure: return a bounded output that states the concrete gap so an independent Reviewer can reject it; use outcome=failed only when you cannot return any bounded output. If the TaskContext binding kind is vmp-ida-index-evidence, read its exact packet, report, dispatch, and receipt paths and put the exact selected row (preserving TSV text or its exact JSON string escaping), selected evidence ref, packet path, receipt path, and observation event ID in the returned reviewerItemsPath output; a query-term-only echo is invalid. Set reviewerItemsPath to one returned output containing one non-empty review item per line when practical; otherwise use an empty string and the manifest will be reviewed. For outcome=failed provide a non-empty reason and no outputs.", schema, nil
	}
	if pkg.SessionKind == "mission-commander-evidence-review" {
		schema := `{"type":"object","properties":{"decision":{"type":"string","enum":["accepted","rejected"]},"summary":{"type":"string"},"reason":{"type":"string"},"evidenceRefs":{"type":"array","minItems":2,"maxItems":8,"items":{"type":"string"}},"selectedEvidenceRef":{"type":"string"},"observationEventId":{"type":"string"},"receiptSha256":{"type":"string","pattern":"^[0-9a-f]{64}$"}},"required":["decision","summary","reason","evidenceRefs","selectedEvidenceRef","observationEventId","receiptSha256"],"additionalProperties":false}`
		return common + " This is an independent Mission Commander evidence review, not a member or ReviewerResult session. Verify the immutable review input against the exact packet, request sources, report, dispatch, receipt, and observation paths it names. Accept only if the selected row is an exact source line, its matched term and evidence ref are exact, and every supplied SHA-256 and lineage identity agrees. Reject on any missing, unreadable, ambiguous, or drifted binding. Return the exact selectedEvidenceRef, observationEventId, receiptSha256, and the evidenceRefs listed by the review input; do not write files or ledger state.", schema, nil
	}
	if pkg.SessionKind != "reviewer" {
		return "", "", fmt.Errorf("unsupported Claude session kind %q", pkg.SessionKind)
	}
	schema := `{"type":"object","properties":{"outcome":{"type":"string","enum":["returned","failed"]},"result":{"type":["object","null"]},"reason":{"type":"string"}},"required":["outcome","result","reason"],"additionalProperties":false}`
	receipt, fields, err := validateReviewerLaunchIdentity(caseRoot, *pkg.Launch)
	if err != nil {
		return "", "", err
	}
	identity := reviewerExpectedOutput(receipt, fields)
	return common + " " + identity + " For outcome=returned, result must be exactly one ReviewerResult object and reviewerSession must equal the actual session ID above. Judge only the current reviewed manifest, its current output bytes, and currently accessible bounded evidence. This session is the independent Reviewer for the current attempt: do not reject merely because the member correctly said no Reviewer had run yet, and do not require the member output to contain this later session's result. Decide whether the current member evidence and analysis satisfy the factual acceptance requirements; use the current manifest binding as proof that this review is independent and current. A historical rejection embedded in the replacement TaskContext is correction provenance, not evidence that the current replacement still has the old defect; reject only if the current output fails to address it. Route fields with evidence-supported unknown, none, or not-applicable values satisfy presence but must not be upgraded into unsupported positive claims. Put only readable case-relative file references (optionally with a # fragment) or the exact packet ID in result.evidenceRefs; put non-file evidence labels, selected-row refs such as ida-index:..., observation event IDs, and other lineage identifiers in result.summary or result.routeOutput instead. If the current member reviewerItemsPath contains vmp-ida-index-evidence rows, copy the first complete ida-index:... evidenceRef verbatim into result.summary and the exact observationEventId verbatim into result.routeOutput.request_id; do not shorten, paraphrase, or replace either value with a packet ID. Keep result.routeOutput.evidence bound to an inspectable top-level evidenceRefs value, and include the exact case-relative packetPath and receiptPath from those rows in result.evidenceRefs. In result.routeOutput set tool_scope exactly to read-only and next_action exactly to main-agent review when those fields are required; do not request writes, heavy tools, authority/confirmed, or external effects. For outcome=failed, result must be null and reason must be non-empty.", schema, nil
}

func validateReviewerLaunchIdentity(
	caseRoot string,
	launch mission.CurrentLoopExternalSessionHarnessLaunch,
) (reviewersession.DispatchReceipt, []string, error) {
	identity := launch.ReviewerIdentity
	if identity == nil || strings.TrimSpace(identity.DispatchPath) == "" ||
		strings.TrimSpace(identity.DispatchSHA256) == "" {
		return reviewersession.DispatchReceipt{}, nil, fmt.Errorf(
			"Claude reviewer launch package omitted its exact durable dispatch identity",
		)
	}
	receipt, err := reviewersession.ReadDispatch(caseRoot, identity.DispatchPath, identity.DispatchSHA256)
	if err != nil {
		return reviewersession.DispatchReceipt{}, nil, err
	}
	fields, err := reviewersession.OutputContractFields(caseRoot, receipt)
	if err != nil {
		return reviewersession.DispatchReceipt{}, nil, err
	}
	if receipt.PacketID != identity.PacketID || receipt.RouteID != identity.RouteID ||
		receipt.ShardID != identity.ShardID || !slices.Equal(receipt.Items, identity.Items) ||
		receipt.DispatchID != identity.DispatchID ||
		receipt.ReviewerSession != identity.ReviewerSession ||
		!casePathEqual(receipt.PromptPath, identity.PromptPath) ||
		!strings.EqualFold(receipt.PromptSHA256, identity.PromptSHA256) ||
		receipt.NoHeavyTool != identity.NoHeavyTool ||
		receipt.NoAuthority != identity.NoAuthority ||
		launch.Attempt.Session != receipt.ReviewerSession ||
		!casePathEqual(launch.Input.Path, receipt.PromptPath) ||
		!strings.EqualFold(launch.Input.SHA256, receipt.PromptSHA256) ||
		!slices.Equal(fields, identity.OutputFields) ||
		!launch.ReadOnly || !receipt.ReadOnly || !receipt.NoHeavyTool || !receipt.NoAuthority {
		return reviewersession.DispatchReceipt{}, nil, fmt.Errorf(
			"Claude reviewer launch package does not match its exact durable dispatch identity",
		)
	}
	return receipt, fields, nil
}

func validateReviewerResultIdentity(
	result reviewerresult.Result,
	receipt reviewersession.DispatchReceipt,
	fields []string,
) error {
	if result.PacketID != receipt.PacketID || result.RouteID != receipt.RouteID ||
		result.ShardID != receipt.ShardID || !slices.Equal(result.Items, receipt.Items) ||
		result.ReviewerSession != receipt.ReviewerSession {
		return fmt.Errorf(
			"real Claude ReviewerResult does not match exact dispatch packet/route/shard/items/session identity",
		)
	}
	return reviewersession.ValidateRouteOutput(fields, result.RouteOutput)
}

func reviewerLaunchIdentity(
	receipt reviewersession.DispatchReceipt,
	fields []string,
	dispatchPath,
	dispatchSHA256 string,
) *mission.CurrentLoopExternalSessionReviewerIdentity {
	return &mission.CurrentLoopExternalSessionReviewerIdentity{
		PacketID: receipt.PacketID, RouteID: receipt.RouteID,
		ShardID: receipt.ShardID, Items: append([]string{}, receipt.Items...),
		OutputFields: append([]string{}, fields...),
		DispatchPath: dispatchPath, DispatchSHA256: dispatchSHA256,
		DispatchID: receipt.DispatchID, ReviewerSession: receipt.ReviewerSession,
		PromptPath: receipt.PromptPath, PromptSHA256: receipt.PromptSHA256,
		NoHeavyTool: receipt.NoHeavyTool, NoAuthority: receipt.NoAuthority,
	}
}

func reviewerExpectedOutput(receipt reviewersession.DispatchReceipt, fields []string) string {
	items, err := json.Marshal(receipt.Items)
	if err != nil {
		panic(err)
	}
	outputFields, err := json.Marshal(fields)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf(
		"Copy these immutable dispatch values exactly into result: packetId=%q, routeId=%q, shardId=%q, and items=%s. Set result.routeOutput to exactly these fields: %s; every listed field must be a non-empty string and no other routeOutput fields are allowed. Do not copy placeholder text such as packet.packetId from the prompt template.",
		receipt.PacketID,
		receipt.RouteID,
		receipt.ShardID,
		items,
		outputFields,
	)
}

func reviewerClaudePackage(caseRoot string, handoff reviewerExternalHandoff) (mission.CurrentLoopExternalSessionHarnessPackage, reviewersession.DispatchReceipt, error) {
	if handoff.RunLoopStepID != "save-result-input" || handoff.State != "reviewer-session-running-unknown" {
		return mission.CurrentLoopExternalSessionHarnessPackage{}, reviewersession.DispatchReceipt{}, fmt.Errorf("reviewer handoff is not waiting for a real session result")
	}
	if strings.TrimSpace(handoff.ReviewerDispatchReceiptPath) == "" || strings.TrimSpace(handoff.ReviewerDispatchReceiptSHA256) == "" {
		return mission.CurrentLoopExternalSessionHarnessPackage{}, reviewersession.DispatchReceipt{}, fmt.Errorf("reviewer handoff omitted the dispatch receipt binding")
	}
	dispatchPath, err := anchoredPath(caseRoot, handoff.ReviewerDispatchReceiptPath)
	if err != nil {
		return mission.CurrentLoopExternalSessionHarnessPackage{}, reviewersession.DispatchReceipt{}, err
	}
	data, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, dispatchPath, "reviewer dispatch receipt", 1<<20)
	if err != nil {
		return mission.CurrentLoopExternalSessionHarnessPackage{}, reviewersession.DispatchReceipt{}, err
	}
	if !strings.EqualFold(bytesSHA256(data), handoff.ReviewerDispatchReceiptSHA256) {
		return mission.CurrentLoopExternalSessionHarnessPackage{}, reviewersession.DispatchReceipt{}, fmt.Errorf("reviewer dispatch receipt sha256 mismatch")
	}
	receipt, err := reviewersession.DecodeDispatch(data)
	if err != nil {
		return mission.CurrentLoopExternalSessionHarnessPackage{}, reviewersession.DispatchReceipt{}, err
	}
	if receipt.DispatchID != handoff.ReviewerDispatchID || receipt.ReviewerHarness != handoff.ReviewerHarness || receipt.ReviewerSession != handoff.ReviewerSession ||
		!casePathEqual(receipt.PromptPath, handoff.DispatchPromptPath) || !strings.EqualFold(receipt.PromptSHA256, handoff.DispatchPromptSHA256) {
		return mission.CurrentLoopExternalSessionHarnessPackage{}, reviewersession.DispatchReceipt{}, fmt.Errorf("reviewer handoff does not match its durable dispatch receipt")
	}
	if receipt.ReviewerHarness != defaultHarness {
		return mission.CurrentLoopExternalSessionHarnessPackage{}, reviewersession.DispatchReceipt{}, fmt.Errorf("reviewer dispatch harness %q is not supported by this host", receipt.ReviewerHarness)
	}
	inputPath, err := anchoredPath(caseRoot, receipt.PromptPath)
	if err != nil {
		return mission.CurrentLoopExternalSessionHarnessPackage{}, reviewersession.DispatchReceipt{}, err
	}
	input, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, inputPath, "reviewer dispatch prompt", 1<<20)
	if err != nil {
		return mission.CurrentLoopExternalSessionHarnessPackage{}, reviewersession.DispatchReceipt{}, err
	}
	if !strings.EqualFold(bytesSHA256(input), receipt.PromptSHA256) {
		return mission.CurrentLoopExternalSessionHarnessPackage{}, reviewersession.DispatchReceipt{}, fmt.Errorf("reviewer dispatch prompt sha256 mismatch")
	}
	fields, err := reviewersession.OutputContractFields(caseRoot, receipt)
	if err != nil {
		return mission.CurrentLoopExternalSessionHarnessPackage{}, reviewersession.DispatchReceipt{}, err
	}
	return mission.CurrentLoopExternalSessionHarnessPackage{
		SchemaVersion: 1,
		State:         "launch-ready",
		CaseRoot:      caseRoot,
		SessionKind:   "reviewer",
		Launch: &mission.CurrentLoopExternalSessionHarnessLaunch{
			Ready: true, Tool: "Claude Code Agent", AgentType: receipt.AgentType, ReadOnly: receipt.ReadOnly,
			Input:            mission.CurrentLoopExternalSessionHarnessInput{Path: receipt.PromptPath, SHA256: receipt.PromptSHA256, Role: "reviewer-dispatch-prompt"},
			ExpectedOutput:   reviewerExpectedOutput(receipt, fields),
			ReviewerIdentity: reviewerLaunchIdentity(receipt, fields, handoff.ReviewerDispatchReceiptPath, handoff.ReviewerDispatchReceiptSHA256),
			Attempt:          mission.CurrentLoopExternalSessionAttempt{AttemptID: receipt.DispatchID, AttemptSHA256: handoff.ReviewerDispatchReceiptSHA256, Generation: 1, Harness: receipt.ReviewerHarness, Session: receipt.ReviewerSession, Actor: receipt.Actor, StartedAt: receipt.RecordedAt},
		},
	}, receipt, nil
}

func reviewerResultBytes(
	pkg mission.CurrentLoopExternalSessionHarnessPackage,
	run claudeRun,
) ([]byte, string, error) {
	if !run.success() {
		return nil, run.failureReason(), nil
	}
	var response reviewerResponse
	if err := strictJSON(run.structuredOutput, &response); err != nil {
		return nil, "", fmt.Errorf("invalid Claude reviewer structured output: %w", err)
	}
	if response.Outcome != "returned" {
		reason := strings.TrimSpace(response.Reason)
		if reason == "" {
			reason = "Claude reviewer reported failure without a reason"
		}
		return nil, reason, nil
	}
	if len(bytes.TrimSpace(response.Result)) == 0 || bytes.Equal(bytes.TrimSpace(response.Result), []byte("null")) {
		return nil, "", fmt.Errorf("Claude reviewer returned no ReviewerResult")
	}
	result, err := reviewerresult.Decode(response.Result)
	if err != nil {
		return nil, "", fmt.Errorf("validate real Claude ReviewerResult: %w", err)
	}
	receipt, fields, err := validateReviewerLaunchIdentity(pkg.CaseRoot, *pkg.Launch)
	if err != nil {
		return nil, "", err
	}
	if err := validateReviewerResultIdentity(result, receipt, fields); err != nil {
		return nil, "", err
	}
	return canonicalJSON(response.Result), "", nil
}

func casePathEqual(left, right string) bool {
	leftPath, leftErr := filepath.Abs(left)
	rightPath, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && strings.EqualFold(filepath.Clean(leftPath), filepath.Clean(rightPath))
}

func validateEvidenceReviewResponse(response evidenceReviewResponse) error {
	if response.Decision != "accepted" && response.Decision != "rejected" {
		return fmt.Errorf("Claude evidence review returned invalid decision %q", response.Decision)
	}
	receiptHash, receiptHashErr := hex.DecodeString(strings.TrimSpace(response.ReceiptSHA256))
	if strings.TrimSpace(response.Summary) == "" || strings.TrimSpace(response.Reason) == "" || len(response.EvidenceRefs) < 2 || len(response.EvidenceRefs) > 8 || strings.TrimSpace(response.SelectedEvidenceRef) == "" || strings.TrimSpace(response.ObservationEventID) == "" || receiptHashErr != nil || len(receiptHash) != sha256.Size {
		return fmt.Errorf("Claude evidence review returned incomplete decision bindings")
	}
	for _, ref := range response.EvidenceRefs {
		if strings.TrimSpace(ref) == "" || strings.ContainsAny(ref, "\r\n") {
			return fmt.Errorf("Claude evidence review returned invalid evidence ref")
		}
	}
	return nil
}

func validateClaudeStructuredResult(pkg mission.CurrentLoopExternalSessionHarnessPackage, run claudeRun) error {
	if !run.success() {
		return nil
	}
	switch pkg.SessionKind {
	case "member":
		var response memberResponse
		if err := strictJSON(run.structuredOutput, &response); err != nil {
			return fmt.Errorf("invalid Claude member structured output: %w", err)
		}
		if response.Outcome != "returned" {
			return nil
		}
		if strings.TrimSpace(response.Summary) == "" {
			return fmt.Errorf("Claude member returned an empty summary")
		}
		if _, err := validateMemberOutputs(pkg.Return.SubmissionOutputs, response); err != nil {
			return err
		}
	case "reviewer":
		receipt, fields, err := validateReviewerLaunchIdentity(pkg.CaseRoot, *pkg.Launch)
		if err != nil {
			return err
		}
		var response reviewerResponse
		if err := strictJSON(run.structuredOutput, &response); err != nil {
			return fmt.Errorf("invalid Claude reviewer structured output: %w", err)
		}
		if response.Outcome != "returned" {
			return nil
		}
		if len(bytes.TrimSpace(response.Result)) == 0 || bytes.Equal(bytes.TrimSpace(response.Result), []byte("null")) {
			return fmt.Errorf("Claude reviewer returned no ReviewerResult")
		}
		result, err := reviewerresult.Decode(response.Result)
		if err != nil {
			return fmt.Errorf("validate real Claude ReviewerResult: %w", err)
		}
		if err := validateReviewerResultIdentity(result, receipt, fields); err != nil {
			return err
		}
	case "mission-commander-evidence-review":
		var response evidenceReviewResponse
		if err := strictJSON(run.structuredOutput, &response); err != nil {
			return fmt.Errorf("invalid Claude evidence review structured output: %w", err)
		}
		if err := validateEvidenceReviewResponse(response); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported Claude session kind %q", pkg.SessionKind)
	}
	return nil
}

func publishClaudeResult(opt Options, plan currentStepPlan, run claudeRun) (string, error) {
	if plan.ExternalSessionStep == nil || plan.ExternalSessionStep.Mode != "running-handoff" || plan.ExternalSessionStep.HarnessPackage == nil {
		return "host-failed", fmt.Errorf("fresh current step is not the accepted running handoff")
	}
	pkg := plan.ExternalSessionStep.HarnessPackage
	if pkg.Return == nil || pkg.Launch == nil {
		return "host-failed", fmt.Errorf("running handoff omitted return contract")
	}
	outcome := "failed"
	summary, reason, reviewerItemsPath := "", run.failureReason(), ""
	if run.success() {
		switch pkg.SessionKind {
		case "member":
			var response memberResponse
			if err := strictJSON(run.structuredOutput, &response); err != nil {
				reason = "invalid Claude member structured output: " + err.Error()
				break
			}
			if response.Outcome == "returned" {
				summary = strings.TrimSpace(response.Summary)
				if summary == "" {
					reason = "Claude member returned an empty summary"
					break
				}
				if _, err := validateMemberOutputs(pkg.Return.SubmissionOutputs, response); err != nil {
					reason = err.Error()
					break
				}
				if err := publishMemberOutputs(opt.Target, pkg.Return.SubmissionOutputs, response); err != nil {
					return "host-failed", err
				}
				outcome, reason, reviewerItemsPath = "returned", "", strings.TrimSpace(response.ReviewerItemsPath)
			} else {
				reason = strings.TrimSpace(response.Reason)
				if reason == "" {
					reason = "Claude member reported failure without a reason"
				}
			}
		case "reviewer":
			receipt, fields, err := validateReviewerLaunchIdentity(pkg.CaseRoot, *pkg.Launch)
			if err != nil {
				reason = err.Error()
				break
			}
			var response reviewerResponse
			if err := strictJSON(run.structuredOutput, &response); err != nil {
				reason = "invalid Claude reviewer structured output: " + err.Error()
				break
			}
			if response.Outcome == "returned" {
				if len(bytes.TrimSpace(response.Result)) == 0 || bytes.Equal(bytes.TrimSpace(response.Result), []byte("null")) {
					reason = "Claude reviewer returned no ReviewerResult"
					break
				}
				reviewerResult, err := reviewerresult.Decode(response.Result)
				if err != nil {
					reason = "validate real Claude ReviewerResult: " + err.Error()
					break
				}
				if err := validateReviewerResultIdentity(reviewerResult, receipt, fields); err != nil {
					reason = err.Error()
					break
				}
				if _, err := rekitfs.WriteExclusiveRegularFileAnchored(opt.Target, pkg.Return.SubmissionResult, "Claude reviewer result", canonicalJSON(response.Result)); err != nil {
					return "host-failed", err
				}
				outcome, reason = "returned", ""
			} else {
				reason = strings.TrimSpace(response.Reason)
				if reason == "" {
					reason = "Claude reviewer reported failure without a reason"
				}
			}
		default:
			return "host-failed", fmt.Errorf("unsupported Claude session kind %q", pkg.SessionKind)
		}
	}
	template, err := submissionTemplate(pkg, outcome)
	if err != nil {
		return "host-failed", err
	}
	var submission map[string]any
	if err := json.Unmarshal([]byte(template.JSON), &submission); err != nil {
		return "host-failed", err
	}
	submission["actor"] = opt.Actor
	if pkg.SessionKind == "member" {
		submission["observedAt"] = nowRFC3339Nano()
		if outcome == "returned" {
			submission["summary"] = summary
			if reviewerItemsPath != "" {
				submission["reviewerItemsPath"] = reviewerItemsPath
			}
		} else {
			submission["reason"] = truncate(reason, 4096)
		}
	} else if outcome == "failed" {
		submission["reviewerExitStatus"] = truncate(reason, 1024)
	}
	data, err := json.MarshalIndent(submission, "", "  ")
	if err != nil {
		return "host-failed", err
	}
	data = append(data, '\n')
	if _, err := rekitfs.WriteExclusiveRegularFileAnchored(opt.Target, pkg.Return.SubmissionPath, "Claude external session submission", data); err != nil {
		return "host-failed", err
	}
	return outcome, nil
}

func publishMemberOutputs(caseRoot, root string, response memberResponse) error {
	validated, err := validateMemberOutputs(root, response)
	if err != nil {
		return err
	}
	return publishValidatedMemberOutputs(caseRoot, validated)
}

func publishValidatedMemberOutputs(caseRoot string, outputs []validatedMemberOutput) error {
	if len(outputs) == 0 || len(outputs) > maxOutputs {
		return fmt.Errorf("Claude member output publication requires 1..%d validated outputs", maxOutputs)
	}
	for _, output := range outputs {
		if strings.TrimSpace(output.path) == "" || len(output.data) == 0 || len(output.data) > maxOutputBytes {
			return fmt.Errorf("Claude member output publication received invalid bounded transport bytes")
		}
		if _, err := rekitfs.WriteExclusiveRegularFileAnchored(caseRoot, output.path, "Claude member output", output.data); err != nil {
			return err
		}
	}
	return nil
}

type validatedMemberOutput struct {
	path string
	data []byte
}

func validateMemberOutputs(root string, response memberResponse) ([]validatedMemberOutput, error) {
	if len(response.Outputs) == 0 || len(response.Outputs) > maxOutputs {
		return nil, fmt.Errorf("Claude member returned %d outputs; expected 1..%d", len(response.Outputs), maxOutputs)
	}
	seen := map[string]bool{}
	validated := make([]validatedMemberOutput, 0, len(response.Outputs))
	for _, output := range response.Outputs {
		rawPath := strings.TrimSpace(output.Path)
		if rawPath == "" || strings.HasPrefix(rawPath, "/") || strings.HasPrefix(rawPath, "\\") || strings.Contains(rawPath, ":") {
			return nil, fmt.Errorf("Claude member output path is invalid: %q", output.Path)
		}
		path := filepath.ToSlash(filepath.Clean(filepath.FromSlash(rawPath)))
		if path == "." || filepath.IsAbs(path) || path == ".." || strings.HasPrefix(path, "../") {
			return nil, fmt.Errorf("Claude member output path is invalid: %q", output.Path)
		}
		key := strings.ToLower(path)
		data := []byte(output.Content)
		if seen[key] || len(data) == 0 || len(data) > maxOutputBytes {
			return nil, fmt.Errorf("Claude member output is duplicate, empty, or too large: %s", path)
		}
		seen[key] = true
		validated = append(validated, validatedMemberOutput{path: filepath.ToSlash(filepath.Join(filepath.FromSlash(root), filepath.FromSlash(path))), data: data})
	}
	if response.ReviewerItemsPath != "" {
		rawReviewerPath := strings.TrimSpace(response.ReviewerItemsPath)
		if strings.HasPrefix(rawReviewerPath, "/") || strings.HasPrefix(rawReviewerPath, "\\") || strings.Contains(rawReviewerPath, ":") {
			return nil, fmt.Errorf("reviewerItemsPath is invalid")
		}
		reviewerPath := filepath.ToSlash(filepath.Clean(filepath.FromSlash(rawReviewerPath)))
		if reviewerPath == "." || reviewerPath == ".." || strings.HasPrefix(reviewerPath, "../") || !seen[strings.ToLower(reviewerPath)] {
			return nil, fmt.Errorf("reviewerItemsPath does not name a returned output")
		}
	}
	return validated, nil
}

func submissionTemplate(pkg *mission.CurrentLoopExternalSessionHarnessPackage, outcome string) (mission.CurrentLoopExternalSessionSubmissionTemplate, error) {
	for _, template := range pkg.Return.Templates {
		if template.Outcome == outcome {
			return template, nil
		}
	}
	return mission.CurrentLoopExternalSessionSubmissionTemplate{}, fmt.Errorf("external session outcome %q is not allowed", outcome)
}

func strictJSON(data []byte, target any) error {
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("structured output must contain exactly one JSON object")
	}
	return nil
}

func canonicalJSON(data []byte) []byte {
	var value any
	if json.Unmarshal(data, &value) != nil {
		return append([]byte{}, data...)
	}
	out, _ := json.MarshalIndent(value, "", "  ")
	return append(out, '\n')
}

func anchoredPath(caseRoot, path string) (string, error) {
	if filepath.IsAbs(path) {
		full, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(caseRoot, full)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("Claude host input escapes case root: %s", path)
		}
		return full, nil
	}
	return rekitfs.SafeJoin(caseRoot, path)
}

func newUUID() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	data[6] = (data[6] & 0x0f) | 0x40
	data[8] = (data[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", data[0:4], data[4:6], data[6:8], data[8:10], data[10:16]), nil
}

type limitedBuffer struct {
	mu       sync.Mutex
	data     []byte
	limit    int
	exceeded bool
}

func (buffer *limitedBuffer) Write(data []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	if buffer.exceeded {
		return len(data), nil
	}
	if len(buffer.data)+len(data) > buffer.limit {
		remaining := buffer.limit - len(buffer.data)
		if remaining > 0 {
			buffer.data = append(buffer.data, data[:remaining]...)
		}
		buffer.exceeded = true
		return len(data), nil
	}
	buffer.data = append(buffer.data, data...)
	return len(data), nil
}

func (buffer *limitedBuffer) Bytes() []byte {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return append([]byte{}, buffer.data...)
}

func (buffer *limitedBuffer) String() string {
	return string(buffer.Bytes())
}

type boundedBuffer struct {
	mu   sync.Mutex
	data []byte
}

func (buffer *boundedBuffer) Write(data []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	buffer.data = append(buffer.data, data...)
	if len(buffer.data) > maxDiagnosticBytes {
		buffer.data = append([]byte{}, buffer.data[len(buffer.data)-maxDiagnosticBytes:]...)
	}
	return len(data), nil
}

func (buffer *boundedBuffer) Bytes() []byte {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return append([]byte{}, buffer.data...)
}

func (buffer *boundedBuffer) String() string {
	return string(buffer.Bytes())
}

func bytesSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func oneLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
