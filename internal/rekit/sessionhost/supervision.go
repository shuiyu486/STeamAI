package sessionhost

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectexecution"
	rekitruntime "github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

const (
	supervisionSpecKind     = "claude-host-run-spec"
	supervisionClaimedKind  = "claude-host-run-claimed"
	supervisionStartedKind  = "claude-host-run-started"
	supervisionFencedKind   = "claude-host-run-fenced"
	supervisionTerminalKind = "claude-host-run-terminal"
)

var (
	supervisionStartedObserver     func(supervisionStarted) error
	supervisionCutObserver         func(string) error
	supervisionObserverMu          sync.Mutex
	supervisorChildStageHook       func(string) error
	supervisorChildCommandTestHook func(*exec.Cmd)
)

func setSupervisionAcceptanceObservers(started func(supervisionStarted) error, cut func(string) error) func() {
	supervisionObserverMu.Lock()
	previousStarted := supervisionStartedObserver
	previousCut := supervisionCutObserver
	supervisionStartedObserver = started
	supervisionCutObserver = cut
	return func() {
		supervisionStartedObserver = previousStarted
		supervisionCutObserver = previousCut
		supervisionObserverMu.Unlock()
	}
}

func observeSupervisionCut(stage string) error {
	if supervisionCutObserver == nil {
		return nil
	}
	return supervisionCutObserver(stage)
}

type supervisionSpec struct {
	SchemaVersion                     int                         `json:"schemaVersion"`
	Kind                              string                      `json:"kind"`
	RunID                             string                      `json:"runId"`
	Target                            string                      `json:"target"`
	Pack                              string                      `json:"pack"`
	ClaudePath                        string                      `json:"claudePath"`
	ExpectedClaudeExecutableSHA256    string                      `json:"expectedClaudeExecutableSha256"`
	ExpectedClaudeExecutablePublisher string                      `json:"expectedClaudeExecutablePublisher"`
	Model                             string                      `json:"model,omitempty"`
	TimeoutNanos                      int64                       `json:"timeoutNanos"`
	SessionID                         string                      `json:"sessionId"`
	LaunchControl                     *claudeLaunchControlBinding `json:"launchControl,omitempty"`
	Execution                         supervisionExecution        `json:"execution"`
}

type supervisionExecution struct {
	SchemaVersion    int                      `json:"schemaVersion"`
	CaseRoot         string                   `json:"caseRoot"`
	JobID            string                   `json:"jobId,omitempty"`
	JobSHA256        string                   `json:"jobSha256,omitempty"`
	CheckpointSHA256 string                   `json:"checkpointSha256,omitempty"`
	SessionKind      string                   `json:"sessionKind"`
	Launch           supervisionLaunchBinding `json:"launch"`
	Return           supervisionReturnBinding `json:"return"`
}

type supervisionLaunchBinding struct {
	Tool             string                                              `json:"tool"`
	AgentType        string                                              `json:"agentType"`
	ReadOnly         bool                                                `json:"readOnly"`
	Input            mission.CurrentLoopExternalSessionHarnessInput      `json:"input"`
	ExpectedOutput   string                                              `json:"expectedOutput"`
	ReviewerIdentity *mission.CurrentLoopExternalSessionReviewerIdentity `json:"reviewerIdentity,omitempty"`
	Attempt          mission.CurrentLoopExternalSessionAttempt           `json:"attempt"`
}

type supervisionReturnBinding struct {
	SubmissionPath    string   `json:"submissionPath"`
	SubmissionOutputs string   `json:"submissionOutputs,omitempty"`
	SubmissionResult  string   `json:"submissionResult,omitempty"`
	SubmissionLast    bool     `json:"submissionLast"`
	AllowedOutcomes   []string `json:"allowedOutcomes,omitempty"`
}

type supervisionClaimed struct {
	SchemaVersion int    `json:"schemaVersion"`
	Kind          string `json:"kind"`
	RunID         string `json:"runId"`
	SpecSHA256    string `json:"specSha256"`
	SessionID     string `json:"sessionId"`
	ClaimedAt     string `json:"claimedAt"`
}

type supervisionFenced struct {
	SchemaVersion int    `json:"schemaVersion"`
	Kind          string `json:"kind"`
	RunID         string `json:"runId"`
	SpecSHA256    string `json:"specSha256"`
	SessionID     string `json:"sessionId"`
	Reason        string `json:"reason"`
	FencedAt      string `json:"fencedAt"`
}

type supervisionFencedError struct {
	RunID  string
	Reason string
}

var errSupervisionAdvanced = errors.New("Claude supervision advanced before fencing")

func (err *supervisionFencedError) Error() string {
	return fmt.Sprintf("Claude supervisor run %s is durably fenced: %s", err.RunID, err.Reason)
}

type supervisionStarted struct {
	SchemaVersion int    `json:"schemaVersion"`
	Kind          string `json:"kind"`
	RunID         string `json:"runId"`
	SpecSHA256    string `json:"specSha256"`
	SessionID     string `json:"sessionId"`
	StartedAt     string `json:"startedAt"`
}

type supervisionTerminal struct {
	SchemaVersion                  int            `json:"schemaVersion"`
	Kind                           string         `json:"kind"`
	RunID                          string         `json:"runId"`
	SpecSHA256                     string         `json:"specSha256"`
	SessionID                      string         `json:"sessionId,omitempty"`
	Envelope                       claudeEnvelope `json:"envelope"`
	StructuredOutputBase64         string         `json:"structuredOutputBase64,omitempty"`
	StructuredOutputSHA256         string         `json:"structuredOutputSha256,omitempty"`
	FailureCode                    string         `json:"failureCode,omitempty"`
	FailureDetail                  string         `json:"failureDetail,omitempty"`
	SpawnError                     string         `json:"spawnError,omitempty"`
	WaitError                      string         `json:"waitError,omitempty"`
	StartError                     string         `json:"startError,omitempty"`
	Started                        bool           `json:"started"`
	ExitCode                       int            `json:"exitCode"`
	TimedOut                       bool           `json:"timedOut"`
	DurationNanos                  int64          `json:"durationNanos"`
	StdoutTail                     string         `json:"stdoutTail,omitempty"`
	StderrTail                     string         `json:"stderrTail,omitempty"`
	StopActuationRequestPath       string         `json:"stopActuationRequestPath,omitempty"`
	StopActuationObservationPath   string         `json:"stopActuationObservationPath,omitempty"`
	StopActuationObservationSHA256 string         `json:"stopActuationObservationSha256,omitempty"`
	StopActuationError             string         `json:"stopActuationError,omitempty"`
	ObservedAt                     string         `json:"observedAt"`
	rawResultRef                   string
	rawResultSHA256                string
	rawResultBytes                 int64
}

type supervisionPaths struct {
	root     string
	runID    string
	runRoot  string
	spec     string
	claimed  string
	started  string
	fenced   string
	terminal string
	owner    string
}

func supervisedClaudeRun(parent context.Context, opt Options, pkg mission.CurrentLoopExternalSessionHarnessPackage, sessionID string, started func() error) (claudeRun, bool, error) {
	boundOpt, err := ensureClaudeLaunchControlBinding(opt, pkg)
	if err != nil {
		return claudeRun{}, false, err
	}
	opt = boundOpt
	if !trustedRecoveryProvenance(opt) {
		return runClaude(parent, opt, pkg, sessionID, started), true, nil
	}
	handoffRequired, err := supervisionHandoffRequired(opt)
	if err != nil {
		return claudeRun{}, false, err
	}
	if handoffRequired && !projectexecution.DurableHandoffSupported() {
		return claudeRun{}, false, fmt.Errorf(
			"current Claude supervision requires handle-bound exact filesystem mutation support",
		)
	}
	paths, spec, specData, specSHA, err := prepareSupervision(opt, pkg, sessionID)
	if err != nil {
		return claudeRun{}, false, err
	}
	replayed, err := rekitfs.WriteExclusiveRegularFileAnchored(paths.runRoot, "spec.json", "Claude host-run supervision spec", specData)
	if err != nil {
		return claudeRun{}, false, err
	}
	launched := false
	if fenced, ok, err := readSupervisionFenced(paths, spec, specSHA); err != nil {
		return claudeRun{}, false, err
	} else if ok {
		return claudeRun{}, false, &supervisionFencedError{RunID: fenced.RunID, Reason: fenced.Reason}
	}
	if terminal, ok, err := readSupervisionTerminal(paths, spec, specSHA); err != nil {
		return claudeRun{}, false, err
	} else if ok {
		run := claudeRunFromTerminal(terminal, spec.LaunchControl, true)
		if terminal.Started && supervisionStartedObserver != nil {
			startedReceipt := supervisionStarted{SchemaVersion: 1, Kind: supervisionStartedKind, RunID: spec.RunID, SpecSHA256: specSHA, SessionID: spec.SessionID}
			if err := supervisionStartedObserver(startedReceipt); err != nil {
				return claudeRun{}, false, err
			}
		}
		if terminal.Started && started != nil {
			if err := started(); err != nil {
				run.startCallbackErr = err
				run.started = true
			}
		}
		if err := observeSupervisionCut("output-returned"); err != nil {
			return claudeRun{}, false, err
		}
		return run, false, nil
	}
	if !replayed {
		var handoff projectexecution.Handoff
		if handoffRequired {
			handoff, err = projectexecution.NewHandoff(
				spec.Target,
				spec.RunID,
				specSHA,
				spec.SessionID,
			)
			if err != nil {
				return claudeRun{}, false, err
			}
			if err := projectexecution.PublishHandoff(spec.Target, handoff); err != nil {
				return claudeRun{}, false, err
			}
		}
		if err := startSupervisorChild(paths.spec, specSHA); err != nil {
			if handoffRequired {
				err = errors.Join(
					err,
					projectexecution.CancelHandoff(spec.Target, handoff),
				)
			}
			return claudeRun{}, false, err
		}
		launched = true
	}

	deadline := time.Now().Add(opt.Timeout)
	startupDeadline := time.Now().Add(10 * time.Second)
	claimedRecorded := false
	startedRecorded := false
	for {
		if terminal, ok, err := readSupervisionTerminal(paths, spec, specSHA); err != nil {
			return claudeRun{}, launched, err
		} else if ok {
			run := claudeRunFromTerminal(terminal, spec.LaunchControl, !launched)
			if launched {
				run.started = terminal.Started
			}
			if terminal.Started && !startedRecorded {
				startedReceipt := supervisionStarted{SchemaVersion: 1, Kind: supervisionStartedKind, RunID: spec.RunID, SpecSHA256: specSHA, SessionID: spec.SessionID}
				if supervisionStartedObserver != nil {
					if err := supervisionStartedObserver(startedReceipt); err != nil {
						return claudeRun{}, launched, err
					}
				}
				if started != nil {
					if err := started(); err != nil {
						run.startCallbackErr = err
						run.started = true
					}
				}
			}
			if err := observeSupervisionCut("output-returned"); err != nil {
				return claudeRun{}, launched, err
			}
			return run, launched, nil
		}
		if !claimedRecorded {
			if _, ok, err := readSupervisionClaimed(paths, spec, specSHA); err != nil {
				return claudeRun{}, launched, err
			} else if ok {
				claimedRecorded = true
			}
		}
		if !startedRecorded {
			if _, ok, err := readSupervisionStarted(paths, spec, specSHA); err != nil {
				return claudeRun{}, launched, err
			} else if ok {
				startedRecorded = true
				startedReceipt, _, _ := readSupervisionStarted(paths, spec, specSHA)
				if supervisionStartedObserver != nil {
					if err := supervisionStartedObserver(startedReceipt); err != nil {
						return claudeRun{}, launched, err
					}
				}
				if started != nil {
					if err := started(); err != nil {
						return claudeRun{started: true, startCallbackErr: err}, launched, nil
					}
				}
			}
		}
		busy, err := supervisionOwnerBusy(paths.owner)
		if err != nil {
			return claudeRun{}, launched, err
		}
		if !busy && !claimedRecorded && time.Now().After(startupDeadline) {
			reason := "supervisor child did not claim the exact run before the startup deadline"
			if err := fenceSupervision(paths, spec, specSHA, reason, handoffRequired); !errors.Is(err, errSupervisionAdvanced) {
				return claudeRun{}, launched, err
			}
		}
		if !busy && claimedRecorded {
			if recovered, ok, recoverErr := recoverClaudeRunForCase(opt.Target, opt, pkg); recoverErr != nil {
				return claudeRun{}, launched, recoverErr
			} else if ok {
				recovered.recovered = !launched
				recovered.started = launched
				return recovered, launched, nil
			}
			reason := "exact supervisor ownership ended without a terminal result or exact structured-output recovery"
			if err := fenceSupervision(paths, spec, specSHA, reason, handoffRequired); !errors.Is(err, errSupervisionAdvanced) {
				return claudeRun{}, launched, err
			}
		}
		if err := parent.Err(); err != nil {
			return claudeRun{}, launched, err
		}
		if time.Now().After(deadline) {
			return claudeRun{}, launched, fmt.Errorf("timed out collecting exact Claude supervisor run %s", paths.runID)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func fenceSupervision(
	paths supervisionPaths,
	spec supervisionSpec,
	specSHA,
	reason string,
	handoffRequired bool,
) error {
	if fenced, ok, err := readSupervisionFenced(paths, spec, specSHA); err != nil {
		return err
	} else if ok {
		return &supervisionFencedError{RunID: fenced.RunID, Reason: fenced.Reason}
	}
	lease, busy, err := acquireSupervisionOwner(paths.owner, true)
	if err != nil {
		return err
	}
	if busy {
		return fmt.Errorf("%w: ownership became active for run %s", errSupervisionAdvanced, paths.runID)
	}
	defer lease.Close()
	if handoffRequired {
		handoff, err := projectexecution.NewHandoff(
			spec.Target,
			spec.RunID,
			specSHA,
			spec.SessionID,
		)
		if err != nil {
			return err
		}
		if err := projectexecution.CancelHandoff(spec.Target, handoff); err != nil {
			return err
		}
	}
	if terminal, ok, err := readSupervisionTerminal(paths, spec, specSHA); err != nil {
		return err
	} else if ok {
		return fmt.Errorf("%w: terminal result appeared for run %s", errSupervisionAdvanced, terminal.RunID)
	}
	if fenced, ok, err := readSupervisionFenced(paths, spec, specSHA); err != nil {
		return err
	} else if ok {
		return &supervisionFencedError{RunID: fenced.RunID, Reason: fenced.Reason}
	}
	fenced := supervisionFenced{SchemaVersion: 1, Kind: supervisionFencedKind, RunID: spec.RunID, SpecSHA256: specSHA, SessionID: spec.SessionID, Reason: reason, FencedAt: nowRFC3339Nano()}
	if err := writeSupervisionJSON(paths.runRoot, "fenced.json", "Claude supervision fenced receipt", fenced); err != nil {
		return err
	}
	return &supervisionFencedError{RunID: paths.runID, Reason: reason}
}

func prepareSupervision(opt Options, pkg mission.CurrentLoopExternalSessionHarnessPackage, sessionID string) (supervisionPaths, supervisionSpec, []byte, string, error) {
	var err error
	opt, err = ensureClaudeLaunchControlBinding(opt, pkg)
	if err != nil {
		return supervisionPaths{}, supervisionSpec{}, nil, "", err
	}
	root, err := supervisionRoot(opt.Target)
	if err != nil {
		return supervisionPaths{}, supervisionSpec{}, nil, "", err
	}
	paths, spec, data, specSHA, err := supervisionSpecForRoot(root, opt, pkg, sessionID)
	if err != nil {
		return supervisionPaths{}, supervisionSpec{}, nil, "", err
	}
	if err := os.MkdirAll(paths.runRoot, 0o700); err != nil {
		return supervisionPaths{}, supervisionSpec{}, nil, "", err
	}
	if info, err := os.Lstat(paths.runRoot); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return supervisionPaths{}, supervisionSpec{}, nil, "", fmt.Errorf("Claude supervision run root is not a stable directory: %s", paths.runRoot)
	}
	return paths, spec, data, specSHA, nil
}

func supervisionSpecForRoot(
	root string,
	opt Options,
	pkg mission.CurrentLoopExternalSessionHarnessPackage,
	sessionID string,
) (supervisionPaths, supervisionSpec, []byte, string, error) {
	if pkg.Launch == nil || strings.TrimSpace(pkg.Launch.Attempt.AttemptID) == "" ||
		strings.TrimSpace(pkg.Launch.Attempt.AttemptSHA256) == "" || pkg.Launch.Attempt.Session != sessionID {
		return supervisionPaths{}, supervisionSpec{}, nil, "", fmt.Errorf("Claude supervision requires exact attempt and session bindings")
	}
	if opt.launchControlBinding != nil {
		if err := validateClaudeLaunchControlBinding(*opt.launchControlBinding); err != nil {
			return supervisionPaths{}, supervisionSpec{}, nil, "", err
		}
	}
	runID, err := supervisionRunID(pkg, opt.launchControlBinding)
	if err != nil {
		return supervisionPaths{}, supervisionSpec{}, nil, "", err
	}
	paths := supervisionPathsForRun(root, runID)
	spec := supervisionSpec{
		SchemaVersion: 1, Kind: supervisionSpecKind, RunID: runID, Target: opt.Target, Pack: opt.Pack,
		ClaudePath: opt.ClaudePath, ExpectedClaudeExecutableSHA256: strings.ToLower(strings.TrimSpace(opt.ExpectedClaudeExecutableSHA256)),
		ExpectedClaudeExecutablePublisher: strings.TrimSpace(opt.ExpectedClaudeExecutablePublisher), Model: strings.TrimSpace(opt.Model),
		TimeoutNanos: int64(opt.Timeout), SessionID: sessionID,
		LaunchControl: cloneClaudeLaunchControlBinding(opt.launchControlBinding), Execution: supervisionExecutionFor(pkg),
	}
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return supervisionPaths{}, supervisionSpec{}, nil, "", err
	}
	data = append(data, '\n')
	return paths, spec, data, bytesSHA256(data), nil
}

func supervisionRunID(pkg mission.CurrentLoopExternalSessionHarnessPackage, binding *claudeLaunchControlBinding) (string, error) {
	if pkg.Launch == nil {
		return "", fmt.Errorf("Claude supervision run identity requires a launch binding")
	}
	identity, err := json.Marshal(struct {
		SessionKind   string                      `json:"sessionKind"`
		AttemptID     string                      `json:"attemptId"`
		AttemptSHA256 string                      `json:"attemptSha256"`
		SessionID     string                      `json:"sessionId"`
		LaunchControl *claudeLaunchControlBinding `json:"launchControl,omitempty"`
	}{
		SessionKind:   pkg.SessionKind,
		AttemptID:     pkg.Launch.Attempt.AttemptID,
		AttemptSHA256: pkg.Launch.Attempt.AttemptSHA256,
		SessionID:     pkg.Launch.Attempt.Session,
		LaunchControl: cloneClaudeLaunchControlBinding(binding),
	})
	if err != nil {
		return "", err
	}
	return bytesSHA256(identity), nil
}

func supervisionExecutionFor(pkg mission.CurrentLoopExternalSessionHarnessPackage) supervisionExecution {
	execution := supervisionExecution{
		SchemaVersion:    pkg.SchemaVersion,
		CaseRoot:         pkg.CaseRoot,
		JobID:            pkg.JobID,
		JobSHA256:        pkg.JobSHA256,
		CheckpointSHA256: pkg.CheckpointSHA256,
		SessionKind:      pkg.SessionKind,
	}
	if pkg.Launch != nil {
		execution.Launch = supervisionLaunchBinding{
			Tool:             pkg.Launch.Tool,
			AgentType:        pkg.Launch.AgentType,
			ReadOnly:         pkg.Launch.ReadOnly,
			Input:            pkg.Launch.Input,
			ExpectedOutput:   pkg.Launch.ExpectedOutput,
			ReviewerIdentity: pkg.Launch.ReviewerIdentity,
			Attempt:          pkg.Launch.Attempt,
		}
	}
	if pkg.Return != nil {
		execution.Return = supervisionReturnBinding{
			SubmissionPath:    pkg.Return.SubmissionPath,
			SubmissionOutputs: pkg.Return.SubmissionOutputs,
			SubmissionResult:  pkg.Return.SubmissionResult,
			SubmissionLast:    pkg.Return.SubmissionLast,
		}
		for _, template := range pkg.Return.Templates {
			execution.Return.AllowedOutcomes = append(execution.Return.AllowedOutcomes, template.Outcome)
		}
	}
	return execution
}

func (execution supervisionExecution) packageForRun() mission.CurrentLoopExternalSessionHarnessPackage {
	pkg := mission.CurrentLoopExternalSessionHarnessPackage{
		SchemaVersion:    execution.SchemaVersion,
		CaseRoot:         execution.CaseRoot,
		JobID:            execution.JobID,
		JobSHA256:        execution.JobSHA256,
		CheckpointSHA256: execution.CheckpointSHA256,
		SessionKind:      execution.SessionKind,
		Launch: &mission.CurrentLoopExternalSessionHarnessLaunch{
			Ready:            true,
			Tool:             execution.Launch.Tool,
			AgentType:        execution.Launch.AgentType,
			ReadOnly:         execution.Launch.ReadOnly,
			Input:            execution.Launch.Input,
			ExpectedOutput:   execution.Launch.ExpectedOutput,
			ReviewerIdentity: execution.Launch.ReviewerIdentity,
			Attempt:          execution.Launch.Attempt,
		},
	}
	if execution.Return.SubmissionPath != "" || execution.Return.SubmissionOutputs != "" || execution.Return.SubmissionResult != "" || len(execution.Return.AllowedOutcomes) > 0 {
		pkg.Return = &mission.CurrentLoopExternalSessionReturnContract{
			SubmissionPath:    execution.Return.SubmissionPath,
			SubmissionOutputs: execution.Return.SubmissionOutputs,
			SubmissionResult:  execution.Return.SubmissionResult,
			SubmissionLast:    execution.Return.SubmissionLast,
		}
		for _, outcome := range execution.Return.AllowedOutcomes {
			pkg.Return.Templates = append(pkg.Return.Templates, mission.CurrentLoopExternalSessionSubmissionTemplate{Outcome: outcome})
		}
	}
	return pkg
}

func acquireSupervisionControl(parent context.Context, caseRoot string) (*supervisionOwnerLease, error) {
	root, err := supervisionRoot(caseRoot)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(root, "control.lease")
	for {
		lease, busy, err := acquireSupervisionOwner(path, true)
		if err != nil {
			return nil, fmt.Errorf("acquire Claude host control lease: %w", err)
		}
		if !busy {
			return lease, nil
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-parent.Done():
			timer.Stop()
			return nil, parent.Err()
		case <-timer.C:
		}
	}
}

func supervisionRoot(caseRoot string) (string, error) {
	root, caseSHA, err := supervisionRootIdentity(caseRoot)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	marker := []byte("rekit Claude supervision root v2\ncaseSha256=" + caseSHA + "\n")
	if _, err := rekitfs.WriteExclusiveRegularFileAnchored(root, "binding", "Claude supervision root binding", marker); err != nil {
		return "", err
	}
	return root, nil
}

func supervisionRootPath(caseRoot string) (string, error) {
	root, _, err := supervisionRootIdentity(caseRoot)
	return root, err
}

func supervisionRootIdentity(caseRoot string) (root, caseSHA string, err error) {
	caseRoot, err = filepath.Abs(caseRoot)
	if err != nil {
		return "", "", err
	}
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return "", "", fmt.Errorf("resolve host-owned Claude supervision root: %w", err)
	}
	identity := filepath.Clean(caseRoot)
	if runtime.GOOS == "windows" {
		identity = strings.ToLower(identity)
	}
	caseSHA = bytesSHA256([]byte(identity))
	root = filepath.Join(cacheRoot, "rekit", "session-host", "v2", "cases", caseSHA)
	if pathsOverlap(caseRoot, root) {
		return "", "", fmt.Errorf("host-owned Claude supervision root must be outside the attached case: %s", caseRoot)
	}
	return root, caseSHA, nil
}

func startSupervisorChild(specPath, specSHA string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(executable, "-internal-supervisor", specPath, "-internal-supervisor-sha256", specSHA)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	configureSupervisorCommand(cmd)
	if supervisorChildCommandTestHook != nil {
		supervisorChildCommandTestHook(cmd)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start Claude supervisor child: %w", err)
	}
	return cmd.Process.Release()
}

// ValidateSupervisorProjectRoot binds a project-local internal child to the
// exact target encoded by its strict host-owned supervision spec.
func ValidateSupervisorProjectRoot(specPath, expectedSHA, projectRoot string) error {
	specPath, err := filepath.Abs(strings.TrimSpace(specPath))
	if err != nil {
		return err
	}
	data, err := os.ReadFile(specPath)
	if err != nil {
		return err
	}
	if len(data) == 0 || len(data) > 2*1024*1024 ||
		!strings.EqualFold(bytesSHA256(data), strings.TrimSpace(expectedSHA)) {
		return fmt.Errorf("Claude supervisor spec sha256 mismatch")
	}
	var spec supervisionSpec
	if err := strictJSON(data, &spec); err != nil {
		return fmt.Errorf("decode Claude supervisor spec: %w", err)
	}
	if spec.SchemaVersion != 1 || spec.Kind != supervisionSpecKind ||
		spec.RunID == "" || spec.SessionID == "" || spec.TimeoutNanos <= 0 {
		return fmt.Errorf("Claude supervisor spec is incomplete")
	}
	resolved, err := rekitruntime.ResolveProjectLocalTarget(
		projectRoot,
		spec.Target,
		"",
	)
	if err != nil {
		return err
	}
	if !rekitfs.SamePath(resolved, spec.Execution.CaseRoot) {
		return fmt.Errorf("Claude supervisor execution case root differs from the project-local executable owner")
	}
	return nil
}

func RunSupervisorChild(parent context.Context, specPath, expectedSHA string) error {
	specPath, err := filepath.Abs(strings.TrimSpace(specPath))
	if err != nil {
		return err
	}
	data, err := os.ReadFile(specPath)
	if err != nil {
		return err
	}
	if len(data) == 0 || len(data) > 2*1024*1024 || !strings.EqualFold(bytesSHA256(data), strings.TrimSpace(expectedSHA)) {
		return fmt.Errorf("Claude supervisor spec sha256 mismatch")
	}
	var spec supervisionSpec
	if err := strictJSON(data, &spec); err != nil {
		return fmt.Errorf("decode Claude supervisor spec: %w", err)
	}
	if spec.SchemaVersion != 1 || spec.Kind != supervisionSpecKind || spec.RunID == "" || spec.SessionID == "" || spec.TimeoutNanos <= 0 {
		return fmt.Errorf("Claude supervisor spec is incomplete")
	}
	opt := Options{
		Target: spec.Target, Pack: spec.Pack, ClaudePath: spec.ClaudePath, ExpectedClaudeExecutableSHA256: spec.ExpectedClaudeExecutableSHA256,
		ExpectedClaudeExecutablePublisher: spec.ExpectedClaudeExecutablePublisher, Model: spec.Model, Timeout: time.Duration(spec.TimeoutNanos),
		launchControlBinding: cloneClaudeLaunchControlBinding(spec.LaunchControl),
	}
	if supervisorChildStageHook != nil {
		if err := supervisorChildStageHook("before-execution-acquire"); err != nil {
			return err
		}
	}
	executionLease, err := acquireSharedForCurrentProject(spec.Target)
	if err != nil {
		return err
	}
	if executionLease != nil {
		defer executionLease.Unlock()
		if err := executionLease.ValidateFor(spec.Target); err != nil {
			return err
		}
		handoff, err := projectexecution.NewHandoff(
			spec.Target,
			spec.RunID,
			expectedSHA,
			spec.SessionID,
		)
		if err != nil {
			return err
		}
		if err := projectexecution.ClaimHandoff(spec.Target, handoff); err != nil {
			return err
		}
		if err := executionLease.ValidateFor(spec.Target); err != nil {
			return err
		}
		if recovery, err := inspectCurrentSyncRecoveryForHost(spec.Target); err != nil {
			return err
		} else if recovery.Pending {
			return fmt.Errorf("%s; %s", recovery.Now, recovery.Next)
		}
	}
	opt.projectExecutionLease = executionLease
	runPackage := spec.Execution.packageForRun()
	paths, expectedSpec, expectedData, specSHA, err := prepareSupervision(opt, runPackage, spec.SessionID)
	if err != nil {
		return err
	}
	if spec.LaunchControl != nil {
		opt.stopActuation = &supervisionStopActuationContext{
			paths:      paths,
			spec:       expectedSpec,
			specSHA256: specSHA,
		}
	}
	expectedCanonical, err := json.Marshal(expectedSpec)
	if err != nil {
		return err
	}
	actualCanonical, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	if paths.spec != specPath || !strings.EqualFold(specSHA, expectedSHA) || !strings.EqualFold(bytesSHA256(expectedData), expectedSHA) || string(actualCanonical) != string(expectedCanonical) {
		return fmt.Errorf("Claude supervisor spec does not match its exact host-owned run binding")
	}
	if executionLease != nil {
		if err := executionLease.ValidateFor(spec.Target); err != nil {
			return err
		}
	}
	lease, busy, err := acquireSupervisionOwner(paths.owner, false)
	if err != nil {
		return err
	}
	if busy {
		return fmt.Errorf("Claude supervisor owner is already active for run %s", spec.RunID)
	}
	defer lease.Close()
	if fenced, ok, err := readSupervisionFenced(paths, spec, specSHA); err != nil {
		return err
	} else if ok {
		return &supervisionFencedError{RunID: fenced.RunID, Reason: fenced.Reason}
	}
	claimed := supervisionClaimed{SchemaVersion: 1, Kind: supervisionClaimedKind, RunID: spec.RunID, SpecSHA256: specSHA, SessionID: spec.SessionID, ClaimedAt: nowRFC3339Nano()}
	if err := writeSupervisionJSON(paths.runRoot, "claimed.json", "Claude supervision claimed receipt", claimed); err != nil {
		return err
	}
	if _, ok, err := readSupervisionTerminal(paths, spec, specSHA); err != nil {
		return err
	} else if ok {
		return nil
	}
	run := runClaude(parent, opt, runPackage, spec.SessionID, func() error {
		started := supervisionStarted{SchemaVersion: 1, Kind: supervisionStartedKind, RunID: spec.RunID, SpecSHA256: specSHA, SessionID: spec.SessionID, StartedAt: nowRFC3339Nano()}
		return writeSupervisionJSON(paths.runRoot, "started.json", "Claude supervision started receipt", started)
	})
	if run.success() {
		if err := validateClaudeStructuredResult(runPackage, run); err != nil {
			run.failureDetail = err.Error()
		} else if err := persistClaudeRecoveryForCase(opt.Target, opt, runPackage, run); err != nil {
			run.failureDetail = "persist exact Claude structured output recovery: " + err.Error()
		}
	}
	durableEnvelope := run.envelope
	durableEnvelope.StructuredOutput = nil
	terminal := supervisionTerminal{
		SchemaVersion: 1, Kind: supervisionTerminalKind, RunID: spec.RunID, SpecSHA256: specSHA,
		SessionID: spec.SessionID, Envelope: durableEnvelope,
		StructuredOutputBase64: base64.StdEncoding.EncodeToString(run.structuredOutput), StructuredOutputSHA256: bytesSHA256(run.structuredOutput),
		FailureCode: run.failureCode, FailureDetail: run.failureDetail, SpawnError: errorText(run.spawnErr), WaitError: errorText(run.waitErr),
		StartError: errorText(run.startCallbackErr), Started: run.started, ExitCode: run.exitCode, TimedOut: run.timedOut,
		DurationNanos: int64(run.duration), StdoutTail: run.stdoutTail, StderrTail: run.stderrTail,
		StopActuationError: errorText(run.stopActuation.Err), ObservedAt: nowRFC3339Nano(),
	}
	if validClaudeLaunchSHA256(run.stopActuation.ObservationSHA256) {
		terminal.StopActuationRequestPath = run.stopActuation.RequestPath
		terminal.StopActuationObservationPath = run.stopActuation.ObservationPath
		terminal.StopActuationObservationSHA256 = run.stopActuation.ObservationSHA256
	}
	return writeSupervisionJSON(paths.runRoot, "terminal.json", "Claude supervision terminal receipt", terminal)
}

func readSupervisionFenced(paths supervisionPaths, spec supervisionSpec, specSHA string) (supervisionFenced, bool, error) {
	var receipt supervisionFenced
	ok, err := readSupervisionJSON(paths.runRoot, "fenced.json", "Claude supervision fenced receipt", &receipt)
	if err != nil || !ok {
		return receipt, ok, err
	}
	if receipt.SchemaVersion != 1 || receipt.Kind != supervisionFencedKind || receipt.RunID != spec.RunID || receipt.SpecSHA256 != specSHA || receipt.SessionID != spec.SessionID || strings.TrimSpace(receipt.Reason) == "" || strings.TrimSpace(receipt.FencedAt) == "" {
		return receipt, false, fmt.Errorf("Claude supervision fenced receipt does not match the exact run")
	}
	return receipt, true, nil
}

func readSupervisionClaimed(paths supervisionPaths, spec supervisionSpec, specSHA string) (supervisionClaimed, bool, error) {
	var receipt supervisionClaimed
	ok, err := readSupervisionJSON(paths.runRoot, "claimed.json", "Claude supervision claimed receipt", &receipt)
	if err != nil || !ok {
		return receipt, ok, err
	}
	if receipt.SchemaVersion != 1 || receipt.Kind != supervisionClaimedKind || receipt.RunID != spec.RunID || receipt.SpecSHA256 != specSHA || receipt.SessionID != spec.SessionID || strings.TrimSpace(receipt.ClaimedAt) == "" {
		return receipt, false, fmt.Errorf("Claude supervision claimed receipt does not match the exact run")
	}
	return receipt, true, nil
}

func readSupervisionStarted(paths supervisionPaths, spec supervisionSpec, specSHA string) (supervisionStarted, bool, error) {
	var receipt supervisionStarted
	ok, err := readSupervisionJSON(paths.runRoot, "started.json", "Claude supervision started receipt", &receipt)
	if err != nil || !ok {
		return receipt, ok, err
	}
	if receipt.SchemaVersion != 1 || receipt.Kind != supervisionStartedKind || receipt.RunID != spec.RunID || receipt.SpecSHA256 != specSHA || receipt.SessionID != spec.SessionID || strings.TrimSpace(receipt.StartedAt) == "" {
		return receipt, false, fmt.Errorf("Claude supervision started receipt does not match the exact run")
	}
	return receipt, true, nil
}

func readSupervisionTerminal(paths supervisionPaths, spec supervisionSpec, specSHA string) (supervisionTerminal, bool, error) {
	var receipt supervisionTerminal
	data, err := rekitfs.ReadStableRegularFileAnchored(paths.runRoot, paths.terminal, "Claude supervision terminal receipt", maxClaudeRawArtifactBytes)
	if errors.Is(err, os.ErrNotExist) {
		return receipt, false, nil
	}
	if err != nil {
		return receipt, false, err
	}
	if err := strictJSON(data, &receipt); err != nil {
		return receipt, false, fmt.Errorf("decode Claude supervision terminal receipt: %w", err)
	}
	if receipt.SchemaVersion != 1 || receipt.Kind != supervisionTerminalKind || receipt.RunID != spec.RunID || receipt.SpecSHA256 != specSHA || strings.TrimSpace(receipt.SessionID) == "" || receipt.SessionID != spec.SessionID || strings.TrimSpace(receipt.ObservedAt) == "" {
		return receipt, false, fmt.Errorf("Claude supervision terminal receipt does not match the exact run and requested session")
	}
	if err := validateTerminalStopActuation(paths, spec, specSHA, receipt); err != nil {
		return receipt, false, err
	}
	if receipt.Envelope.SessionID != "" && receipt.Envelope.SessionID != spec.SessionID {
		return receipt, false, fmt.Errorf("Claude supervision terminal envelope does not match the requested session")
	}
	output, err := base64.StdEncoding.DecodeString(receipt.StructuredOutputBase64)
	if err != nil || (len(output) > 0 && !strings.EqualFold(bytesSHA256(output), receipt.StructuredOutputSHA256)) || (len(output) == 0 && receipt.StructuredOutputSHA256 != bytesSHA256(nil)) {
		return receipt, false, fmt.Errorf("Claude supervision terminal structured output binding is invalid")
	}
	receipt.rawResultRef, receipt.rawResultSHA256, receipt.rawResultBytes, err = hostCacheArtifactIdentity(paths.terminal, data)
	if err != nil {
		return receipt, false, err
	}
	return receipt, true, nil
}

func readSupervisionJSON(root, rel, label string, target any) (bool, error) {
	path := filepath.Join(root, rel)
	data, err := rekitfs.ReadStableRegularFileAnchored(root, path, label, 32*1024*1024)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := strictJSON(data, target); err != nil {
		return false, fmt.Errorf("decode %s: %w", label, err)
	}
	return true, nil
}

func writeSupervisionJSON(root, rel, label string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = rekitfs.WriteExclusiveRegularFileAnchored(root, rel, label, data)
	return err
}

func claudeRunFromTerminal(receipt supervisionTerminal, binding *claudeLaunchControlBinding, recovered bool) claudeRun {
	output, err := base64.StdEncoding.DecodeString(receipt.StructuredOutputBase64)
	if err != nil || (len(output) > 0 && !strings.EqualFold(bytesSHA256(output), receipt.StructuredOutputSHA256)) {
		return claudeRun{
			launchControlBinding: cloneClaudeLaunchControlBinding(binding),
			failureCode:          "claude-invalid-envelope",
			failureDetail:        "Claude supervision terminal structured output binding is invalid",
			recovered:            recovered,
		}
	}
	run := claudeRun{
		launchControlBinding: cloneClaudeLaunchControlBinding(binding),
		rawResultRef:         receipt.rawResultRef,
		rawResultSHA256:      receipt.rawResultSHA256,
		rawResultBytes:       receipt.rawResultBytes,
		envelope:             receipt.Envelope, sessionID: receipt.SessionID, structuredOutput: append(json.RawMessage{}, output...),
		failureCode: receipt.FailureCode, failureDetail: receipt.FailureDetail, recovered: recovered, exitCode: receipt.ExitCode,
		timedOut: receipt.TimedOut, duration: time.Duration(receipt.DurationNanos), stdoutTail: receipt.StdoutTail, stderrTail: receipt.StderrTail,
		observedAt: receipt.ObservedAt,
		stopActuation: supervisionStopActuationResult{
			RequestPath: receipt.StopActuationRequestPath, ObservationPath: receipt.StopActuationObservationPath,
			ObservationSHA256: receipt.StopActuationObservationSHA256,
		},
	}
	if receipt.StopActuationError != "" {
		run.stopActuation.Err = errors.New(receipt.StopActuationError)
	}
	if receipt.SpawnError != "" {
		run.spawnErr = errors.New(receipt.SpawnError)
	}
	if receipt.WaitError != "" {
		run.waitErr = errors.New(receipt.WaitError)
	}
	if receipt.StartError != "" {
		run.startCallbackErr = errors.New(receipt.StartError)
	}
	return run
}
