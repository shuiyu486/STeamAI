package adapterhost

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
)

const (
	readonlyInspectorID = "rekit-readonly-inspector"
	adapterHarness      = "rekit-adapter-host"
	maxFixtureBytes     = 1 << 20

	authorizationPhasePreExecution    = "pre-execution"
	authorizationPhasePrePublication  = "pre-publication"
	authorizationPhasePostPublication = "post-publication-pre-success"
)

type hostTestHooks struct {
	beforeReportWrite                   func() error
	afterCleanupIdentityOpen            func(string) error
	afterCleanupQuarantineIdentityCheck func(string, string) error
	runVMPIDAChild                      func(VMPIDAIndexChildOptions) ([]byte, int, error)
	afterVMPChildLaunch                 func(int) error
	beforeVMPPublication                func() error
	beforeVMPAuthorizationCurrentness   func(string) error
	beforeVMPSuccessSeal                func() error
	beforeVMPProfileRevoke              func() error
	afterVMPStageCommit                 func() error
	afterVMPOutputsPublished            func() error
	afterVMPOutputCommit                func() error
	afterVMPReceiptRecorded             func() error
	afterVMPObservation                 func() error
	beforeAuthorizationCurrentness      func(string) error
}

type Options struct {
	RepoRoot                string
	CaseRoot                string
	Pack                    string
	GateEventID             string
	ExpectedDispatchSHA256  string
	ExecutionControlBinding *executioncontrol.Binding
	testHooks               *hostTestHooks
}

type Result struct {
	SchemaVersion       int    `json:"schemaVersion"`
	Kind                string `json:"kind"`
	CaseRoot            string `json:"caseRoot"`
	Pack                string `json:"pack"`
	GateEventID         string `json:"gateEventId"`
	Lane                string `json:"lane"`
	Executor            string `json:"executor"`
	Generation          int    `json:"generation"`
	AdapterID           string `json:"adapterId"`
	AdapterHarness      string `json:"adapterHarness"`
	AdapterSession      string `json:"adapterSession"`
	ExecutableSHA256    string `json:"executableSha256"`
	DispatchPath        string `json:"dispatchPath"`
	DispatchSHA256      string `json:"dispatchSha256"`
	InputPath           string `json:"inputPath"`
	InputSHA256         string `json:"inputSha256"`
	ArtifactPath        string `json:"artifactPath"`
	ArtifactSHA256      string `json:"artifactSha256"`
	ReportPath          string `json:"reportPath"`
	ReportSHA256        string `json:"reportSha256"`
	ProcessID           int    `json:"processId"`
	StartedAt           string `json:"startedAt"`
	CompletedAt         string `json:"completedAt"`
	ReadOnlyInput       bool   `json:"readOnlyInput"`
	NoNetwork           bool   `json:"noNetwork"`
	NoNetworkBoundary   string `json:"noNetworkBoundary,omitempty"`
	NoAuthority         bool   `json:"noAuthorityOrConfirmed"`
	ExecutionStatus     string `json:"executionStatus,omitempty"`
	ExecutionExitStatus string `json:"executionExitStatus,omitempty"`
}

type inspectionArtifact struct {
	SchemaVersion    int    `json:"schemaVersion"`
	Kind             string `json:"kind"`
	AdapterID        string `json:"adapterId"`
	AdapterHarness   string `json:"adapterHarness"`
	AdapterSession   string `json:"adapterSession"`
	ExecutableSHA256 string `json:"executableSha256"`
	DispatchID       string `json:"dispatchId"`
	DispatchPath     string `json:"dispatchPath"`
	DispatchSHA256   string `json:"dispatchSha256"`
	InputPath        string `json:"inputPath"`
	InputSHA256      string `json:"inputSha256"`
	InputBytes       int64  `json:"inputBytes"`
	LineCount        int    `json:"lineCount"`
	ProcessID        int    `json:"processId"`
	StartedAt        string `json:"startedAt"`
	ReadOnlyInput    bool   `json:"readOnlyInput"`
	NoNetwork        bool   `json:"noNetwork"`
}

func Run(opt Options) (_ Result, retErr error) {
	started := time.Now()
	result := Result{
		SchemaVersion:  1,
		Kind:           "rekit-readonly-adapter-host-result",
		Pack:           strings.TrimSpace(opt.Pack),
		GateEventID:    strings.TrimSpace(opt.GateEventID),
		AdapterID:      readonlyInspectorID,
		AdapterHarness: adapterHarness,
		ProcessID:      os.Getpid(),
		StartedAt:      started.UTC().Format(time.RFC3339Nano),
		ReadOnlyInput:  true,
		NoNetwork:      true,
		NoAuthority:    true,
	}
	if result.Pack == "" || result.GateEventID == "" {
		return result, fmt.Errorf("adapter host requires -pack and -gate-event-id")
	}
	if !validSHA256(opt.ExpectedDispatchSHA256) {
		return result, fmt.Errorf("adapter host requires a valid -expected-dispatch-sha256")
	}
	caseRoot, err := filepath.Abs(strings.TrimSpace(opt.CaseRoot))
	if err != nil {
		return result, err
	}
	repoRoot, err := filepath.Abs(strings.TrimSpace(opt.RepoRoot))
	if err != nil {
		return result, err
	}
	result.CaseRoot = caseRoot
	executablePath, err := os.Executable()
	if err != nil {
		return result, err
	}
	executableData, err := readStableExecutable(executablePath)
	if err != nil {
		return result, err
	}
	result.ExecutableSHA256 = sha256Hex(executableData)

	dispatch, dispatchPath, dispatchSHA, _, err := gate.ReadCurrentAdapterExecutionDispatch(
		repoRoot,
		caseRoot,
		result.Pack,
		result.GateEventID,
	)
	if err != nil {
		return result, err
	}
	if !strings.EqualFold(dispatchSHA, opt.ExpectedDispatchSHA256) {
		return result, fmt.Errorf("adapter execution dispatch sha256 changed before host launch: expected %s got %s", opt.ExpectedDispatchSHA256, dispatchSHA)
	}
	if dispatch.Adapter.AdapterID == VMPIDAIndexAdapterID {
		return runVMPIDAExistingDispatch(opt, result, dispatch, dispatchPath, dispatchSHA, started)
	}
	if err := validateDispatch(dispatch); err != nil {
		return result, err
	}
	result.Lane = dispatch.Owner.Lane
	result.Executor = dispatch.Owner.CurrentExecutor
	result.Generation = dispatch.Owner.ExecutorGeneration
	result.AdapterSession = dispatch.Owner.AdapterSession
	result.DispatchPath = dispatchPath
	result.DispatchSHA256 = dispatchSHA
	result.InputPath = cleanCaseRelative(dispatch.Gate.Target)
	result.ReportPath = cleanCaseRelative(dispatch.ReportPath)
	result.ArtifactPath = filepath.ToSlash(filepath.Join(filepath.Dir(result.ReportPath), "inspection.json"))
	if result.InputPath == "" ||
		result.ReportPath == "" ||
		result.ArtifactPath == result.ReportPath ||
		result.InputPath == result.ReportPath ||
		result.InputPath == result.ArtifactPath {
		return result, fmt.Errorf("adapter host dispatch contains an invalid input or output path")
	}

	lease, err := lanemutation.AcquireOpenLane(caseRoot, result.Lane, "adapter host")
	if err != nil {
		return result, err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	current, currentPath, currentSHA, _, err := gate.ReadCurrentAdapterExecutionDispatch(
		repoRoot,
		caseRoot,
		result.Pack,
		result.GateEventID,
	)
	if err != nil {
		return result, err
	}
	if currentPath != dispatchPath || !strings.EqualFold(currentSHA, dispatchSHA) || !adapterexecution.DispatchSemanticEqual(current, dispatch) {
		return result, fmt.Errorf("adapter execution dispatch changed while acquiring lane lease")
	}
	if err := lease.Validate(); err != nil {
		return result, err
	}
	if err := validateAdapterAuthorizationPhase(
		opt,
		repoRoot,
		caseRoot,
		authorizationPhasePreExecution,
		dispatch,
		dispatchPath,
		dispatchSHA,
	); err != nil {
		return result, err
	}

	root, err := os.OpenRoot(caseRoot)
	if err != nil {
		return result, err
	}
	defer root.Close()
	input, inputInfo, err := readStableFixture(root, result.InputPath)
	if err != nil {
		return result, err
	}
	result.InputSHA256 = sha256Hex(input)
	artifact := inspectionArtifact{
		SchemaVersion:    1,
		Kind:             "rekit-readonly-inspection",
		AdapterID:        readonlyInspectorID,
		AdapterHarness:   adapterHarness,
		AdapterSession:   result.AdapterSession,
		ExecutableSHA256: result.ExecutableSHA256,
		DispatchID:       dispatch.DispatchID,
		DispatchPath:     dispatchPath,
		DispatchSHA256:   dispatchSHA,
		InputPath:        result.InputPath,
		InputSHA256:      result.InputSHA256,
		InputBytes:       int64(len(input)),
		LineCount:        lineCount(input),
		ProcessID:        result.ProcessID,
		StartedAt:        result.StartedAt,
		ReadOnlyInput:    true,
		NoNetwork:        true,
	}
	artifactData, err := canonicalJSON(artifact)
	if err != nil {
		return result, err
	}
	report := gate.AdapterReport{
		SchemaVersion: 1,
		Kind:          "adapter-execution-report",
		AdapterID:     readonlyInspectorID,
		Action:        "inspect",
		Status:        "succeeded",
		GateEventID:   result.GateEventID,
		Dispatch: &adapterexecution.ReportDispatchBinding{
			DispatchID: dispatch.DispatchID,
			Path:       dispatchPath,
			SHA256:     dispatchSHA,
		},
		ActualBudget: dispatch.Gate.AuthorizedBudget,
		OutputRefs:   []string{result.ArtifactPath},
		EvidenceRefs: []string{result.ArtifactPath},
		Summary:      "Read-only fixture inspection completed without network, debug, patch, or input mutation.",
	}
	report.ActualBudget.RuntimeSeconds = elapsedSecondsCeil(started)
	report.ActualBudget.DiskMB = 1
	report.ActualBudget.Requests = 1
	if exceedsBudget(dispatch, report) {
		return result, fmt.Errorf("adapter host minimum execution budget exceeds authorized gate budget")
	}
	reportData, err := canonicalJSON(report)
	if err != nil {
		return result, err
	}
	if int64(len(artifactData)+len(reportData)) > int64(dispatch.Gate.AuthorizedBudget.DiskMB)<<20 {
		return result, fmt.Errorf("adapter host output exceeds authorized disk budget")
	}
	if err := ensureOutputParent(root, result.ReportPath); err != nil {
		return result, err
	}
	if _, err := root.Lstat(result.ReportPath); err == nil {
		return result, fmt.Errorf("adapter host refuses an existing execution report: %s", result.ReportPath)
	} else if !os.IsNotExist(err) {
		return result, err
	}
	if _, err := root.Lstat(result.ArtifactPath); err == nil {
		return result, fmt.Errorf("adapter host refuses an existing inspection artifact: %s", result.ArtifactPath)
	} else if !os.IsNotExist(err) {
		return result, err
	}
	inputAgain, inputInfoAgain, err := readStableFixture(root, result.InputPath)
	if err != nil || !os.SameFile(inputInfo, inputInfoAgain) || !bytes.Equal(input, inputAgain) {
		return result, fmt.Errorf("adapter host input changed before output publication")
	}
	latest, latestPath, latestSHA, _, err := gate.ReadCurrentAdapterExecutionDispatch(
		repoRoot,
		caseRoot,
		result.Pack,
		result.GateEventID,
	)
	if err != nil || latestPath != dispatchPath || !strings.EqualFold(latestSHA, dispatchSHA) || !adapterexecution.DispatchSemanticEqual(latest, dispatch) {
		return result, fmt.Errorf("adapter execution dispatch changed before output publication")
	}
	if err := lease.Validate(); err != nil {
		return result, err
	}
	if err := validateAdapterAuthorizationPhase(
		opt,
		repoRoot,
		caseRoot,
		authorizationPhasePrePublication,
		dispatch,
		dispatchPath,
		dispatchSHA,
	); err != nil {
		return result, err
	}

	var artifactOwned, reportOwned *ownedOutput
	defer func() {
		if retErr == nil {
			return
		}
		var afterIdentity func(string) error
		var afterQuarantineIdentity func(string, string) error
		if opt.testHooks != nil {
			afterIdentity = opt.testHooks.afterCleanupIdentityOpen
			afterQuarantineIdentity = opt.testHooks.afterCleanupQuarantineIdentityCheck
		}
		retErr = errors.Join(retErr,
			removeOwnedOutput(root, result.ReportPath, reportOwned, afterIdentity, afterQuarantineIdentity),
			removeOwnedOutput(root, result.ArtifactPath, artifactOwned, afterIdentity, afterQuarantineIdentity),
		)
	}()
	artifactOwned, err = writeExclusive(root, result.ArtifactPath, artifactData, opt.testHooks)
	if err != nil {
		return result, err
	}
	if opt.testHooks != nil && opt.testHooks.beforeReportWrite != nil {
		if err := opt.testHooks.beforeReportWrite(); err != nil {
			return result, err
		}
	}
	report.ActualBudget.RuntimeSeconds = elapsedSecondsCeil(started)
	if exceedsBudget(dispatch, report) {
		return result, fmt.Errorf("adapter host runtime exceeded authorized gate budget")
	}
	reportData, err = canonicalJSON(report)
	if err != nil {
		return result, err
	}
	reportOwned, err = writeExclusive(root, result.ReportPath, reportData, opt.testHooks)
	if err != nil {
		return result, err
	}
	inputFinal, inputInfoFinal, err := readStableFixture(root, result.InputPath)
	if err != nil || !os.SameFile(inputInfo, inputInfoFinal) || !bytes.Equal(input, inputFinal) {
		return result, fmt.Errorf("adapter host input changed before successful completion")
	}
	if err := lease.Validate(); err != nil {
		return result, err
	}
	finalRuntime := elapsedSecondsCeil(started)
	if finalRuntime != report.ActualBudget.RuntimeSeconds {
		return result, fmt.Errorf("adapter host runtime changed during output publication")
	}
	if finalRuntime > dispatch.Gate.AuthorizedBudget.RuntimeSeconds {
		return result, fmt.Errorf("adapter host runtime exceeded authorized gate budget")
	}
	if err := validateAdapterAuthorizationPhase(
		opt,
		repoRoot,
		caseRoot,
		authorizationPhasePostPublication,
		dispatch,
		dispatchPath,
		dispatchSHA,
	); err != nil {
		return result, err
	}
	if err := validateOwnedOutput(
		root,
		result.ArtifactPath,
		artifactOwned,
		artifactData,
	); err != nil {
		return result, err
	}
	if err := validateOwnedOutput(
		root,
		result.ReportPath,
		reportOwned,
		reportData,
	); err != nil {
		return result, err
	}
	result.ArtifactSHA256 = sha256Hex(artifactData)
	result.ReportSHA256 = sha256Hex(reportData)
	result.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
	artifactOwned = nil
	reportOwned = nil
	return result, nil
}

func validateAdapterAuthorizationPhase(
	opt Options,
	repoRoot,
	caseRoot,
	phase string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA string,
) error {
	if opt.testHooks != nil &&
		opt.testHooks.beforeAuthorizationCurrentness != nil {
		if err := opt.testHooks.beforeAuthorizationCurrentness(phase); err != nil {
			return err
		}
	}
	if _, err := gate.ValidateAdapterExecutionCurrentness(
		repoRoot,
		caseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
	); err != nil {
		return fmt.Errorf(
			"adapter execution authorization is not current at %s: %w",
			phase,
			err,
		)
	}
	return nil
}

func validateDispatch(dispatch adapterexecution.DispatchReceipt) error {
	if dispatch.Gate.Action != "inspect" || dispatch.Adapter.AdapterID != readonlyInspectorID {
		return fmt.Errorf("adapter host accepts only the %s inspect dispatch", readonlyInspectorID)
	}
	if dispatch.Owner.AdapterHarness != adapterHarness {
		return fmt.Errorf("adapter host dispatch harness must be %s", adapterHarness)
	}
	if dispatch.Gate.Target == "" || dispatch.Owner.AdapterSession == "" {
		return fmt.Errorf("adapter host dispatch omits target or session binding")
	}
	if dispatch.Gate.AuthorizedBudget.RuntimeSeconds < 1 || dispatch.Gate.AuthorizedBudget.DiskMB < 1 || dispatch.Gate.AuthorizedBudget.Requests != 1 {
		return fmt.Errorf("adapter host dispatch budget is not bounded for one read-only request")
	}
	return nil
}

func exceedsBudget(dispatch adapterexecution.DispatchReceipt, report gate.AdapterReport) bool {
	allowed := dispatch.Gate.AuthorizedBudget
	actual := report.ActualBudget
	return actual.RuntimeSeconds > allowed.RuntimeSeconds || actual.DiskMB > allowed.DiskMB || actual.Requests > allowed.Requests
}

func elapsedSecondsCeil(started time.Time) int {
	elapsed := time.Since(started)
	seconds := int((elapsed + time.Second - 1) / time.Second)
	if seconds < 1 {
		return 1
	}
	return seconds
}

func cleanCaseRelative(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || filepath.IsAbs(value) {
		return ""
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return ""
	}
	return clean
}

func readStableExecutable(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > 128<<20 {
		return nil, fmt.Errorf("adapter executable must be a bounded regular non-symlink file: %s", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	opened, statErr := file.Stat()
	data, readErr := io.ReadAll(io.LimitReader(file, (128<<20)+1))
	closeErr := file.Close()
	after, afterErr := os.Lstat(path)
	if statErr != nil || readErr != nil || closeErr != nil || afterErr != nil || !opened.Mode().IsRegular() ||
		!os.SameFile(info, opened) || !os.SameFile(opened, after) || int64(len(data)) != opened.Size() || len(data) > 128<<20 {
		return nil, fmt.Errorf("adapter executable changed while reading: %s", path)
	}
	return data, nil
}

func readStableFixture(root *os.Root, rel string) ([]byte, os.FileInfo, error) {
	info, err := root.Lstat(rel)
	if err != nil {
		return nil, nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > maxFixtureBytes {
		return nil, nil, fmt.Errorf("adapter host input must be a bounded regular non-symlink file: %s", rel)
	}
	file, err := root.Open(rel)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, nil, fmt.Errorf("adapter host input changed while opening: %s", rel)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxFixtureBytes+1))
	if err != nil || len(data) > maxFixtureBytes {
		return nil, nil, fmt.Errorf("adapter host input exceeds read limit: %s", rel)
	}
	post, err := root.Lstat(rel)
	if err != nil || !os.SameFile(opened, post) || post.Size() != int64(len(data)) {
		return nil, nil, fmt.Errorf("adapter host input changed while reading: %s", rel)
	}
	return data, opened, nil
}

func ensureOutputParent(root *os.Root, rel string) error {
	parent := filepath.Dir(filepath.FromSlash(rel))
	if parent == "." || parent == "" {
		return nil
	}
	return root.MkdirAll(parent, 0o755)
}

func writeExclusive(root *os.Root, rel string, data []byte, hooks *hostTestHooks) (_ *ownedOutput, retErr error) {
	file, err := root.OpenFile(filepath.FromSlash(rel), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	owned, err := captureOwnedOutput(file)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	defer func() {
		if file != nil {
			retErr = errors.Join(retErr, file.Close())
		}
		if retErr != nil {
			var afterIdentity func(string) error
			var afterQuarantineIdentity func(string, string) error
			if hooks != nil {
				afterIdentity = hooks.afterCleanupIdentityOpen
				afterQuarantineIdentity = hooks.afterCleanupQuarantineIdentityCheck
			}
			retErr = errors.Join(retErr, removeOwnedOutput(
				root,
				rel,
				owned,
				afterIdentity,
				afterQuarantineIdentity,
			))
		}
	}()
	if _, err := file.Write(data); err != nil {
		return nil, err
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	file = nil
	return owned, nil
}

func canonicalJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func lineCount(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	count := bytes.Count(data, []byte{'\n'})
	if data[len(data)-1] != '\n' {
		count++
	}
	return count
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validSHA256(value string) bool {
	data, err := hex.DecodeString(strings.TrimSpace(value))
	return err == nil && len(data) == sha256.Size
}
