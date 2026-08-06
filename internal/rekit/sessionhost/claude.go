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
	"strings"
	"sync"
	"time"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewerresult"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewersession"
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
	spawnErr         error
	waitErr          error
	startCallbackErr error
	started          bool
	recovered        bool
	exitCode         int
	timedOut         bool
	duration         time.Duration
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

func runClaude(parent context.Context, opt Options, pkg mission.CurrentLoopExternalSessionHarnessPackage, sessionID string, started func() error) claudeRun {
	begin := time.Now()
	ctx, cancel := context.WithTimeout(parent, opt.Timeout)
	defer cancel()

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
		"--tools", "Read,Glob,Grep",
		"--max-budget-usd", "2.00",
	}
	if strings.TrimSpace(opt.Model) != "" {
		args = append(args, "--model", strings.TrimSpace(opt.Model))
	}
	cmd := exec.CommandContext(ctx, opt.ClaudePath, args...)
	cmd.Dir = opt.Target
	cmd.Stdin = nil
	var stdout limitedBuffer
	var stderr boundedBuffer
	stdout.limit = maxClaudeStdoutBytes
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Start(); err != nil {
		return claudeRun{spawnErr: err, duration: time.Since(begin), stdoutTail: stdout.String(), stderrTail: stderr.String()}
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
		run.waitErr = errors.Join(run.waitErr, fmt.Errorf("Claude Code JSON result exceeded %d bytes", maxClaudeStdoutBytes))
		return run
	}
	if err := json.Unmarshal(stdout.Bytes(), &run.envelope); err != nil {
		if waitErr == nil {
			run.waitErr = fmt.Errorf("decode Claude Code JSON result: %w", err)
		} else {
			run.waitErr = errors.Join(waitErr, fmt.Errorf("decode Claude Code JSON result: %w", err))
		}
		return run
	}
	run.sessionID = strings.TrimSpace(run.envelope.SessionID)
	if run.sessionID == "" {
		run.waitErr = errors.Join(run.waitErr, fmt.Errorf("Claude Code result omitted the requested session id"))
	} else if run.sessionID != sessionID {
		run.waitErr = errors.Join(run.waitErr, fmt.Errorf("Claude Code session id drift: got %s want %s", run.sessionID, sessionID))
	}
	run.structuredOutput = append([]byte{}, run.envelope.StructuredOutput...)
	return run
}

func (run claudeRun) success() bool {
	return run.spawnErr == nil && run.waitErr == nil && !run.timedOut && run.exitCode == 0 &&
		run.sessionID != "" && run.envelope.Type == "result" && run.envelope.Subtype == "success" && !run.envelope.IsError &&
		len(bytes.TrimSpace(run.structuredOutput)) > 0
}

func (run claudeRun) failureReason() string {
	parts := []string{}
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

func sessionResult(run claudeRun, attemptGeneration, launchOrdinal int, reservationID, kind, outcome string) Session {
	diagnostics := []string{}
	if run.stdoutTail != "" && !run.success() {
		diagnostics = append(diagnostics, "stdout: "+truncate(oneLine(run.stdoutTail), 1024))
	}
	if run.stderrTail != "" {
		diagnostics = append(diagnostics, "stderr: "+truncate(oneLine(run.stderrTail), 1024))
	}
	return Session{
		Started: run.started, Recovered: run.recovered, AttemptGeneration: attemptGeneration, RunLaunchOrdinal: launchOrdinal,
		ReservationID: reservationID, SessionID: run.sessionID, SessionKind: kind,
		Outcome: outcome, ExitCode: run.exitCode, TimedOut: run.timedOut,
		ResultSubtype: run.envelope.Subtype, ResultIsError: run.envelope.IsError,
		DurationMillis: run.duration.Milliseconds(), PermissionDenials: run.envelope.PermissionDenials,
		Diagnostics: diagnostics,
	}
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
	common := fmt.Sprintf("Read the immutable task input at the exact absolute path %q using the Read tool before answering. Follow it exactly within its no-authority/no-heavy-tool boundary. Resolve any case-relative evidence paths inside that input from the current case root %q. Your actual Claude Code session ID is %s. Return only the requested structured output through the schema. Do not write external-session result or submission files; the host will persist your real returned bytes.", inputPath, caseRoot, sessionID)
	if pkg.SessionKind == "member" {
		schema := `{"type":"object","properties":{"outcome":{"type":"string","enum":["returned","failed"]},"summary":{"type":"string"},"reason":{"type":"string"},"outputs":{"type":"array","maxItems":64,"items":{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"],"additionalProperties":false}},"reviewerItemsPath":{"type":"string"}},"required":["outcome","summary","reason","outputs","reviewerItemsPath"],"additionalProperties":false}`
		return common + " For outcome=returned provide a non-empty summary and at least one bounded output path/content pair. Set reviewerItemsPath to one returned output containing one non-empty review item per line when practical; otherwise use an empty string and the manifest will be reviewed. For outcome=failed provide a non-empty reason and no outputs.", schema, nil
	}
	schema := `{"type":"object","properties":{"outcome":{"type":"string","enum":["returned","failed"]},"result":{"type":["object","null"]},"reason":{"type":"string"}},"required":["outcome","result","reason"],"additionalProperties":false}`
	return common + " For outcome=returned, result must be exactly one ReviewerResult object and reviewerSession must equal the actual session ID above. For outcome=failed, result must be null and reason must be non-empty.", schema, nil
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
	return mission.CurrentLoopExternalSessionHarnessPackage{
		SchemaVersion: 1,
		State:         "launch-ready",
		CaseRoot:      caseRoot,
		SessionKind:   "reviewer",
		Launch: &mission.CurrentLoopExternalSessionHarnessLaunch{
			Ready: true, Tool: "Claude Code Agent", AgentType: receipt.AgentType, ReadOnly: receipt.ReadOnly,
			Input:          mission.CurrentLoopExternalSessionHarnessInput{Path: receipt.PromptPath, SHA256: receipt.PromptSHA256, Role: "reviewer-dispatch-prompt"},
			ExpectedOutput: "exactly one ReviewerResult JSON object; no Markdown fence or surrounding prose",
			Attempt:        mission.CurrentLoopExternalSessionAttempt{AttemptID: receipt.DispatchID, AttemptSHA256: handoff.ReviewerDispatchReceiptSHA256, Generation: 1, Harness: receipt.ReviewerHarness, Session: receipt.ReviewerSession, Actor: receipt.Actor, StartedAt: receipt.RecordedAt},
		},
	}, receipt, nil
}

func reviewerResultBytes(run claudeRun) ([]byte, string, error) {
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
	if result.ReviewerSession != run.sessionID {
		return nil, "", fmt.Errorf("real Claude ReviewerResult session mismatch")
	}
	return canonicalJSON(response.Result), "", nil
}

func casePathEqual(left, right string) bool {
	leftPath, leftErr := filepath.Abs(left)
	rightPath, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && strings.EqualFold(filepath.Clean(leftPath), filepath.Clean(rightPath))
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
				if reviewerResult.ReviewerSession != run.sessionID {
					reason = "real Claude ReviewerResult session mismatch"
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
	for _, output := range validated {
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
