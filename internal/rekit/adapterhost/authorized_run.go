package adapterhost

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/processguard"
)

const (
	vmpIDAIndexPacketFileName      = "ida-index-packet.json"
	vmpIDAExecutionAttemptFileName = ".ida-index-execution-attempt.json"
	vmpIDAExecutionAttemptKind     = "vmp-ida-index-execution-attempt"
	vmpIDAChildLaunchFileName      = ".ida-index-child-launch.json"
	vmpIDAChildLaunchKind          = "vmp-ida-index-child-launch"
	vmpIDAOutputCommitFileName     = ".ida-index-output-commit.json"
	vmpIDAOutputCommitKind         = "vmp-ida-index-output-commit"
	vmpIDASuccessSealFileName      = ".ida-index-success-seal.json"
	vmpIDASuccessSealKind          = "vmp-ida-index-success-seal"
	vmpIDAChildResultKind          = "vmp-ida-index-inspector-child-result"
	fixedChildNoNetworkCodepath    = "fixed-child-no-network-codepath"
)

type VMPIDAIndexChildResult struct {
	SchemaVersion     int               `json:"schemaVersion"`
	Kind              string            `json:"kind"`
	AdapterID         string            `json:"adapterId"`
	RequestPath       string            `json:"requestPath"`
	RequestSHA256     string            `json:"requestSha256"`
	PacketSHA256      string            `json:"packetSha256"`
	Packet            VMPIDAIndexPacket `json:"packet"`
	ReadOnlyInput     bool              `json:"readOnlyInput"`
	NoNetwork         bool              `json:"noNetwork"`
	NoNetworkBoundary string            `json:"noNetworkBoundary"`
	NoAuthority       bool              `json:"noAuthorityOrConfirmed"`
}

// RunVMPIDAIndexChild is the private compiled-in child entrypoint. It first
// revalidates the exact immutable dispatch, current owner, and current autonomy
// profile, then performs only the fixed literal index inspection. Catalog entry,
// IDA, and network state remain unreachable here.
func RunVMPIDAIndexChild(opt VMPIDAIndexChildOptions) (VMPIDAIndexChildResult, error) {
	caseRoot, requestPath, err := validateVMPIDAIndexChildBinding(opt)
	if err != nil {
		return VMPIDAIndexChildResult{}, err
	}
	inspection, err := InspectVMPIDAIndex(caseRoot, requestPath)
	if err != nil {
		return VMPIDAIndexChildResult{}, err
	}
	return VMPIDAIndexChildResult{
		SchemaVersion:     1,
		Kind:              vmpIDAChildResultKind,
		AdapterID:         VMPIDAIndexAdapterID,
		RequestPath:       inspection.Packet.RequestPath,
		RequestSHA256:     inspection.Packet.RequestSHA256,
		PacketSHA256:      inspection.PacketSHA256,
		Packet:            inspection.Packet,
		ReadOnlyInput:     true,
		NoNetwork:         true,
		NoNetworkBoundary: fixedChildNoNetworkCodepath,
		NoAuthority:       true,
	}, nil
}

func validateVMPIDAIndexChildBinding(
	opt VMPIDAIndexChildOptions,
) (string, string, error) {
	if strings.TrimSpace(opt.Pack) != defaults.DefaultPack ||
		strings.TrimSpace(opt.GateEventID) == "" ||
		!validSHA256(opt.ExpectedDispatchSHA256) ||
		strings.TrimSpace(opt.AdapterSession) == "" ||
		strings.TrimSpace(opt.Executor) == "" ||
		opt.ExpectedExecutorGeneration < 1 {
		return "", "", fmt.Errorf(
			"private VMP IDA child requires exact pack, gate, dispatch, session, executor, and generation bindings",
		)
	}
	caseRoot, err := filepath.Abs(strings.TrimSpace(opt.CaseRoot))
	if err != nil {
		return "", "", err
	}
	repoRoot, err := filepath.Abs(strings.TrimSpace(opt.RepoRoot))
	if err != nil {
		return "", "", err
	}
	dispatch, dispatchPath, dispatchSHA, _, err :=
		gate.ReadCurrentAdapterExecutionDispatch(
			repoRoot,
			caseRoot,
			strings.TrimSpace(opt.Pack),
			strings.TrimSpace(opt.GateEventID),
		)
	if err != nil {
		return "", "", err
	}
	if !strings.EqualFold(dispatchSHA, opt.ExpectedDispatchSHA256) ||
		dispatch.Gate.GateEventID != strings.TrimSpace(opt.GateEventID) ||
		dispatch.Adapter.Pack != strings.TrimSpace(opt.Pack) ||
		dispatch.Adapter.AdapterID != VMPIDAIndexAdapterID ||
		dispatch.Owner.AdapterHarness != adapterHarness ||
		dispatch.Owner.AdapterSession != strings.TrimSpace(opt.AdapterSession) ||
		dispatch.Owner.CurrentExecutor != strings.TrimSpace(opt.Executor) ||
		dispatch.Owner.ExecutorGeneration != opt.ExpectedExecutorGeneration {
		return "", "", fmt.Errorf(
			"private VMP IDA child binding does not match the exact immutable dispatch",
		)
	}
	owner, err := laneowner.Read(caseRoot, dispatch.Owner.Lane)
	if err != nil {
		return "", "", err
	}
	if owner.CurrentExecutor != dispatch.Owner.CurrentExecutor ||
		owner.ExecutorGeneration != dispatch.Owner.ExecutorGeneration {
		return "", "", fmt.Errorf(
			"private VMP IDA child owner binding is no longer current",
		)
	}
	requestPath := cleanCaseRelative(opt.RequestPath)
	if requestPath == "" || requestPath != dispatch.Gate.Target {
		return "", "", fmt.Errorf(
			"private VMP IDA child request does not match the exact dispatch target",
		)
	}
	packetPath := filepath.ToSlash(filepath.Join(
		filepath.Dir(dispatch.ReportPath),
		vmpIDAIndexPacketFileName,
	))
	if err := validateVMPIDADispatch(
		dispatch,
		requestPath,
		dispatch.ReportPath,
		packetPath,
	); err != nil {
		return "", "", err
	}
	if err := validateCurrentAuthorization(repoRoot, caseRoot, dispatch); err != nil {
		return "", "", err
	}
	current, currentPath, currentSHA, _, err :=
		gate.ReadCurrentAdapterExecutionDispatch(
			repoRoot,
			caseRoot,
			dispatch.Adapter.Pack,
			dispatch.Gate.GateEventID,
		)
	if err != nil || currentPath != dispatchPath ||
		!strings.EqualFold(currentSHA, dispatchSHA) ||
		!adapterexecution.DispatchSemanticEqual(current, dispatch) {
		return "", "", fmt.Errorf(
			"private VMP IDA child dispatch changed during authorization validation: %w",
			err,
		)
	}
	return caseRoot, requestPath, nil
}

type AuthorizedRunOptions struct {
	RepoRoot            string
	CaseRoot            string
	Pack                string
	GateEventID         string
	ExecutionReportPath string
	AdapterSession      string
	Actor               string
	testHooks           *hostTestHooks
}

// RunAuthorizedGateProcess launches the exact adapter-host executable in its
// private authorized parent mode. The adapter parent remains the sole owner of
// child containment, execution evidence, member binding, and profile revoke.
func RunAuthorizedGateProcess(adapterPath string, opt AuthorizedRunOptions, timeout time.Duration) (AuthorizedRunResult, int, error) {
	binding, err := processguard.LockExecutable(strings.TrimSpace(adapterPath), 128<<20)
	if err != nil {
		return AuthorizedRunResult{}, 0, err
	}
	defer binding.Close()
	if timeout <= 0 {
		return AuthorizedRunResult{}, 0, fmt.Errorf("authorized VMP IDA adapter process requires a positive timeout")
	}
	args := []string{
		"-run-authorized-vmp-ida-index-inspector",
		"-repo", opt.RepoRoot,
		"-target", opt.CaseRoot,
		"-pack", opt.Pack,
		"-gate-event-id", opt.GateEventID,
		"-execution-report-path", opt.ExecutionReportPath,
		"-adapter-session", opt.AdapterSession,
		"-actor", opt.Actor,
	}
	stdout, _, processID, err := runContainedProcess(binding, args, nil, timeout)
	if err != nil {
		return AuthorizedRunResult{}, processID, err
	}
	result, err := decodeAuthorizedRunProcessResult(stdout)
	if err != nil {
		return result, processID, err
	}
	if err := binding.Validate(); err != nil {
		return result, processID, fmt.Errorf("authorized VMP IDA adapter executable changed during run: %w", err)
	}
	caseRoot, caseErr := filepath.Abs(strings.TrimSpace(opt.CaseRoot))
	if caseErr != nil || !samePath(result.CaseRoot, caseRoot) || result.Pack != strings.TrimSpace(opt.Pack) || result.GateEventID != strings.TrimSpace(opt.GateEventID) || result.AdapterSession != strings.TrimSpace(opt.AdapterSession) || result.AdapterID != VMPIDAIndexAdapterID || !result.NoNetwork || result.NoNetworkBoundary != fixedChildNoNetworkCodepath || !result.NoAuthority {
		return result, processID, fmt.Errorf("authorized VMP IDA adapter process returned inconsistent identity or boundary")
	}
	return result, processID, nil
}

func decodeAuthorizedRunProcessResult(data []byte) (AuthorizedRunResult, error) {
	var result AuthorizedRunResult
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("decode authorized VMP IDA adapter result: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return result, fmt.Errorf("authorized VMP IDA adapter stdout must contain exactly one strict JSON object")
	}
	return result, nil
}

type AuthorizedRunResult struct {
	SchemaVersion        int    `json:"schemaVersion"`
	Kind                 string `json:"kind"`
	CaseRoot             string `json:"caseRoot"`
	Pack                 string `json:"pack"`
	GateEventID          string `json:"gateEventId"`
	AdapterID            string `json:"adapterId"`
	AdapterSession       string `json:"adapterSession"`
	DispatchPath         string `json:"dispatchPath"`
	DispatchSHA256       string `json:"dispatchSha256"`
	ReportPath           string `json:"reportPath"`
	ReportSHA256         string `json:"reportSha256"`
	PacketPath           string `json:"packetPath"`
	PacketSHA256         string `json:"packetSha256"`
	ReceiptPath          string `json:"receiptPath,omitempty"`
	ReceiptSHA256        string `json:"receiptSha256,omitempty"`
	ObservationEventID   string `json:"observationEventId,omitempty"`
	TaskBindingPath      string `json:"taskBindingPath,omitempty"`
	TaskBindingSHA256    string `json:"taskBindingSha256,omitempty"`
	ProfilePath          string `json:"profilePath,omitempty"`
	ProfileSHA256        string `json:"profileSha256,omitempty"`
	ProfileRevoked       bool   `json:"profileRevoked"`
	ProfileAlreadyManual bool   `json:"profileAlreadyManual,omitempty"`
	ExecutionStatus      string `json:"executionStatus,omitempty"`
	ExecutionExitStatus  string `json:"executionExitStatus,omitempty"`
	ChildProcessID       int    `json:"childProcessId,omitempty"`
	ChildLaunched        bool   `json:"childLaunched"`
	Replay               bool   `json:"replay"`
	NoNetwork            bool   `json:"noNetwork"`
	NoNetworkBoundary    string `json:"noNetworkBoundary"`
	NoAuthority          bool   `json:"noAuthorityOrConfirmed"`
}

// RunAuthorizedGate owns the product lifecycle after a durable authorized gate
// exists. It records execution evidence, binds it for the current member, then
// revokes only the exact generated preauthorized profile. It never creates a
// gate, executes a catalog entry, or acknowledges the resulting evidence.
func RunAuthorizedGate(opt AuthorizedRunOptions) (AuthorizedRunResult, error) {
	result, err := runAuthorizedGateExecution(opt)
	if err != nil {
		return result, err
	}
	if result.ObservationEventID == "" ||
		(result.ExecutionStatus == "succeeded" && result.TaskBindingSHA256 == "") ||
		((result.ExecutionStatus == "failed" || result.ExecutionStatus == "aborted") && result.TaskBindingSHA256 != "") {
		return result, fmt.Errorf("authorized VMP IDA run cannot revoke profile before terminal observation and status-appropriate member evidence binding")
	}
	dispatch, _, _, present, err := readVMPIDADispatchArtifact(result.CaseRoot, authorizedGateLane(opt.RepoRoot, result.CaseRoot, result.Pack, result.GateEventID), result.GateEventID)
	if err != nil || !present {
		return result, fmt.Errorf("read VMP IDA dispatch before profile revoke: %w", err)
	}
	lane := dispatch.Owner.Lane
	profile, profilePath, exists, err := autonomy.Read(result.CaseRoot, lane)
	if err != nil || !exists {
		return result, fmt.Errorf("read VMP IDA profile before revoke: %w", err)
	}
	profileSHA := autonomy.FileHash(profilePath)
	if profile.Mode == autonomy.ModeManualGate {
		if !reflect.DeepEqual(profile, autonomy.DefaultProfile(lane)) {
			return result, fmt.Errorf("VMP IDA profile is manual but not the exact generated default")
		}
		result.ProfilePath = autonomy.RelPath(lane)
		result.ProfileSHA256 = profileSHA
		result.ProfileAlreadyManual = true
		return result, nil
	}
	if !strings.EqualFold(profileSHA, dispatch.Gate.Authorization.ProfileHash) {
		return result, fmt.Errorf("VMP IDA profile changed after execution observation; exact revoke is required")
	}
	if opt.testHooks != nil && opt.testHooks.beforeVMPProfileRevoke != nil {
		if err := opt.testHooks.beforeVMPProfileRevoke(); err != nil {
			return result, err
		}
	}
	if !strings.EqualFold(autonomy.FileHash(profilePath), dispatch.Gate.Authorization.ProfileHash) {
		return result, fmt.Errorf("VMP IDA profile changed immediately before exact revoke")
	}
	plan, err := autonomy.PreviewRevoke(autonomy.ProfileRevokeOptions{
		RepoRoot: opt.RepoRoot, CaseRoot: result.CaseRoot, Pack: result.Pack, Lane: lane,
	})
	if err != nil {
		return result, err
	}
	revoked, err := autonomy.ApplyProfilePlan(plan, plan.ExpectedPlanSHA256)
	if err != nil {
		return result, err
	}
	result.ProfilePath = revoked.Plan.ProfilePath
	result.ProfileSHA256 = revoked.ProfileSHA256
	result.ProfileRevoked = revoked.Applied
	result.ProfileAlreadyManual = revoked.AlreadyApplied
	return result, nil
}

func runAuthorizedGateExecution(opt AuthorizedRunOptions) (AuthorizedRunResult, error) {
	result := AuthorizedRunResult{
		SchemaVersion:     1,
		Kind:              "vmp-ida-index-authorized-run",
		Pack:              strings.TrimSpace(opt.Pack),
		GateEventID:       strings.TrimSpace(opt.GateEventID),
		AdapterID:         VMPIDAIndexAdapterID,
		AdapterSession:    strings.TrimSpace(opt.AdapterSession),
		NoNetwork:         true,
		NoNetworkBoundary: fixedChildNoNetworkCodepath,
		NoAuthority:       true,
	}
	if result.Pack != defaults.DefaultPack || result.GateEventID == "" || result.AdapterSession == "" || strings.TrimSpace(opt.Actor) == "" {
		return result, fmt.Errorf("authorized VMP IDA run requires pack=%s, gate event id, adapter session, and actor", defaults.DefaultPack)
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
	lane := authorizedGateLane(repoRoot, caseRoot, result.Pack, result.GateEventID)
	owner, err := laneowner.Read(caseRoot, lane)
	if err != nil {
		return result, err
	}
	reportPath := cleanCaseRelative(opt.ExecutionReportPath)
	if reportPath == "" {
		return result, fmt.Errorf("authorized VMP IDA run requires a case-relative execution report path")
	}

	dispatch, dispatchPath, dispatchSHA, dispatchPresent, err := readVMPIDADispatchArtifact(caseRoot, lane, result.GateEventID)
	if err != nil {
		return result, err
	}
	if dispatchPresent {
		if dispatch.ReportPath != reportPath || dispatch.Owner.AdapterSession != result.AdapterSession || dispatch.Owner.CurrentExecutor != owner.CurrentExecutor || dispatch.Owner.ExecutorGeneration != owner.ExecutorGeneration {
			return result, fmt.Errorf("authorized VMP IDA run does not match the immutable existing dispatch")
		}
		result.DispatchPath, result.DispatchSHA256 = dispatchPath, dispatchSHA
		result.ReportPath = dispatch.ReportPath
		result.PacketPath = filepath.ToSlash(filepath.Join(filepath.Dir(dispatch.ReportPath), vmpIDAIndexPacketFileName))
		report, reportData, packetData, terminal, terminalErr := readTerminalVMPReport(caseRoot, dispatch, dispatchPath, dispatchSHA, result.PacketPath)
		if terminalErr != nil {
			return result, terminalErr
		}
		if terminal {
			result.Replay = true
			result.ExecutionStatus = report.Status
			result.ExecutionExitStatus, err = terminalVMPExecutionExitStatus(report)
			if err != nil {
				return result, err
			}
			result.ReportSHA256 = sha256Hex(reportData)
			if len(packetData) > 0 {
				result.PacketSHA256 = sha256Hex(packetData)
			} else {
				result.PacketPath = ""
			}
			return completeVMPIDAEvidenceLifecycle(
				opt,
				result,
				lane,
				dispatch,
				dispatchPath,
				dispatchSHA,
			)
		}
	} else {
		dispatchOpt := gate.Options{
			GateEventID: result.GateEventID, ExecutionReportPath: reportPath,
			AdapterID: VMPIDAIndexAdapterID, Executor: owner.CurrentExecutor,
			ExpectedExecutorGeneration: owner.ExecutorGeneration, AdapterHarness: adapterHarness,
			AdapterSession: result.AdapterSession, Actor: strings.TrimSpace(opt.Actor),
		}
		preview, previewErr := gate.RecordAdapterExecutionDispatch(repoRoot, caseRoot, result.Pack, dispatchOpt)
		if previewErr != nil {
			return result, previewErr
		}
		dispatchOpt.ExpectedAdapterExecutionDispatchBindingSHA256 = preview.BindingSHA256
		if _, err := gate.RecordAdapterExecutionDispatch(repoRoot, caseRoot, result.Pack, dispatchOpt); err != nil {
			return result, err
		}
		dispatch, dispatchPath, dispatchSHA, _, err = gate.ReadCurrentAdapterExecutionDispatch(repoRoot, caseRoot, result.Pack, result.GateEventID)
		if err != nil {
			return result, err
		}
		result.DispatchPath, result.DispatchSHA256 = dispatchPath, dispatchSHA
		result.ReportPath = dispatch.ReportPath
		result.PacketPath = filepath.ToSlash(filepath.Join(filepath.Dir(dispatch.ReportPath), vmpIDAIndexPacketFileName))
	}

	hostResult, err := Run(Options{
		RepoRoot: repoRoot, CaseRoot: caseRoot, Pack: result.Pack,
		GateEventID: result.GateEventID, ExpectedDispatchSHA256: dispatchSHA,
		testHooks: opt.testHooks,
	})
	if err != nil {
		return result, err
	}
	result.ChildLaunched = hostResult.ProcessID > 0
	result.ChildProcessID = hostResult.ProcessID
	result.Replay = !result.ChildLaunched
	result.ExecutionStatus = hostResult.ExecutionStatus
	result.ExecutionExitStatus = hostResult.ExecutionExitStatus
	result.ReportPath, result.ReportSHA256 = hostResult.ReportPath, hostResult.ReportSHA256
	result.PacketPath, result.PacketSHA256 = hostResult.ArtifactPath, hostResult.ArtifactSHA256
	if opt.testHooks != nil && opt.testHooks.afterVMPOutputsPublished != nil {
		if err := opt.testHooks.afterVMPOutputsPublished(); err != nil {
			return result, err
		}
	}

	return completeVMPIDAEvidenceLifecycle(
		opt,
		result,
		lane,
		dispatch,
		dispatchPath,
		dispatchSHA,
	)
}

const vmpIDAFailureExitStatusPrefix = "vmp-ida-exit-status:"

func terminalVMPExecutionExitStatus(report gate.AdapterReport) (string, error) {
	status := strings.ToLower(strings.TrimSpace(report.Status))
	if status == "succeeded" {
		if strings.TrimSpace(report.Escalation) != "" {
			return "", fmt.Errorf("succeeded VMP IDA terminal report must not carry a failure exit status")
		}
		return "completed", nil
	}
	if status != "failed" && status != "aborted" {
		return "", fmt.Errorf("unsupported VMP IDA terminal report status: %s", status)
	}
	exitStatus, _, err := terminalVMPFailureBinding(report)
	if err != nil {
		return "", err
	}
	aborted := exitStatus == "child-timeout" || exitStatus == "runtime-budget-exceeded"
	if (status == "aborted") != aborted {
		return "", fmt.Errorf("VMP IDA terminal report status does not match execution exit status: %s/%s", status, exitStatus)
	}
	return exitStatus, nil
}

func validVMPIDAFailureExitStatus(value string) bool {
	switch value {
	case "child-timeout", "child-failed", "child-invalid-stdout", "child-invalid-packet", "source-drift", "output-budget-exceeded", "runtime-budget-exceeded":
		return true
	}
	const prefix = "child-exit-"
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	codeText := strings.TrimPrefix(value, prefix)
	code, err := strconv.Atoi(codeText)
	return err == nil && code >= 0 && strconv.Itoa(code) == codeText
}

const vmpIDAChildLaunchSHA256Marker = ";vmp-ida-child-launch-sha256:"

func vmpIDAFailureEscalation(exitStatus, launchSHA string) string {
	return vmpIDAFailureExitStatusPrefix + exitStatus +
		vmpIDAChildLaunchSHA256Marker + strings.ToLower(launchSHA)
}

func terminalVMPFailureBinding(report gate.AdapterReport) (string, string, error) {
	marker := strings.TrimSpace(report.Escalation)
	if !strings.HasPrefix(marker, vmpIDAFailureExitStatusPrefix) {
		return "", "", fmt.Errorf("failed VMP IDA terminal report is missing its exact execution exit status")
	}
	binding := strings.TrimPrefix(marker, vmpIDAFailureExitStatusPrefix)
	separator := strings.Index(binding, vmpIDAChildLaunchSHA256Marker)
	if separator < 1 {
		return "", "", fmt.Errorf("failed VMP IDA terminal report is missing its parent-owned child launch proof")
	}
	exitStatus := binding[:separator]
	launchSHA := binding[separator+len(vmpIDAChildLaunchSHA256Marker):]
	if !validVMPIDAFailureExitStatus(exitStatus) {
		return "", "", fmt.Errorf("failed VMP IDA terminal report has invalid execution exit status: %s", exitStatus)
	}
	if !validSHA256(launchSHA) || strings.ToLower(launchSHA) != launchSHA {
		return "", "", fmt.Errorf("failed VMP IDA terminal report has an invalid child launch proof hash")
	}
	return exitStatus, launchSHA, nil
}

func completeVMPIDAEvidenceLifecycle(
	opt AuthorizedRunOptions,
	result AuthorizedRunResult,
	lane string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA string,
) (AuthorizedRunResult, error) {
	repoRoot := strings.TrimSpace(opt.RepoRoot)
	caseRoot := result.CaseRoot
	validation, err := gate.ValidateAdapterExecutionReport(
		repoRoot,
		caseRoot,
		result.Pack,
		gate.Options{
			GateEventID:         result.GateEventID,
			ExecutionReportPath: result.ReportPath,
		},
	)
	if validation.Report != nil {
		result.ExecutionStatus = validation.Report.Status
	}
	if err != nil {
		return result, err
	}
	if validation.Report == nil {
		return result, fmt.Errorf("VMP IDA terminal report is missing from validation")
	}
	reportedExitStatus, exitErr := terminalVMPExecutionExitStatus(*validation.Report)
	if exitErr != nil {
		return result, exitErr
	}
	if result.ExecutionExitStatus != "" && result.ExecutionExitStatus != reportedExitStatus {
		return result, fmt.Errorf("VMP IDA execution exit status differs from the terminal report")
	}
	result.ExecutionExitStatus = reportedExitStatus
	if !validation.ReceiptPresent {
		receiptOpt := gate.Options{
			GateEventID:                result.GateEventID,
			ExecutionReportPath:        result.ReportPath,
			AdapterID:                  VMPIDAIndexAdapterID,
			Executor:                   dispatch.Owner.CurrentExecutor,
			ExpectedExecutorGeneration: dispatch.Owner.ExecutorGeneration,
			AdapterHarness:             adapterHarness,
			AdapterSession:             dispatch.Owner.AdapterSession,
			ExecutionExitStatus:        result.ExecutionExitStatus,
			Actor:                      strings.TrimSpace(opt.Actor),
		}
		preview, previewErr := gate.RecordAdapterExecutionReceipt(
			repoRoot,
			caseRoot,
			result.Pack,
			receiptOpt,
		)
		if previewErr != nil {
			return result, previewErr
		}
		receiptOpt.ExpectedAdapterExecutionBindingSHA256 = preview.BindingSHA256
		if _, applyErr := gate.RecordAdapterExecutionReceipt(
			repoRoot,
			caseRoot,
			result.Pack,
			receiptOpt,
		); applyErr != nil {
			return result, applyErr
		}
		validation, err = gate.ValidateAdapterExecutionReport(
			repoRoot,
			caseRoot,
			result.Pack,
			gate.Options{
				GateEventID:         result.GateEventID,
				ExecutionReportPath: result.ReportPath,
			},
		)
		if err != nil {
			return result, err
		}
	}
	if !validation.Valid || !validation.ProvenanceValid ||
		validation.AdapterExecution == nil ||
		!validation.ReceiptPresent {
		return result, fmt.Errorf(
			"validate VMP IDA adapter report and receipt: %s",
			strings.TrimSpace(validation.Error),
		)
	}
	result.ReceiptPath = validation.AdapterExecutionReceiptPath
	result.ReceiptSHA256 = validation.AdapterExecutionReceiptSHA256
	receiptExecution := validation.AdapterExecution.Execution
	if receiptExecution.Outcome != validation.Report.Status ||
		receiptExecution.ExitStatus != reportedExitStatus ||
		receiptExecution.Escalation != validation.Report.Escalation ||
		!reflect.DeepEqual(receiptExecution.BoundaryHits, validation.Report.BoundaryHits) {
		return result, fmt.Errorf(
			"VMP IDA receipt outcome, exit status, escalation, or launch proof differs from the terminal report",
		)
	}
	result.ExecutionStatus = validation.Report.Status
	result.ExecutionExitStatus = reportedExitStatus
	if opt.testHooks != nil && opt.testHooks.afterVMPReceiptRecorded != nil {
		if err := opt.testHooks.afterVMPReceiptRecorded(); err != nil {
			return result, err
		}
	}
	if err := validateVMPIDAReceiptArtifacts(
		caseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		result,
		*validation.AdapterExecution,
	); err != nil {
		return result, err
	}
	observationEventID, observationErr :=
		terminalVMPObservationEventID(caseRoot, lane, result.GateEventID)
	if observationErr != nil {
		observation, recordErr := gate.RecordExecution(
			repoRoot,
			caseRoot,
			result.Pack,
			gate.Options{
				GateEventID:                           result.GateEventID,
				Actor:                                 strings.TrimSpace(opt.Actor),
				ExecutionReportPath:                   validation.ReportPath,
				ExpectedExecutionReportSHA256:         validation.RecordExpectedReportSHA256,
				AdapterExecutionReceiptPath:           validation.AdapterExecutionReceiptPath,
				ExpectedAdapterExecutionReceiptSHA256: validation.AdapterExecutionReceiptSHA256,
				Executor:                              dispatch.Owner.CurrentExecutor,
				ExpectedExecutorGeneration:            dispatch.Owner.ExecutorGeneration,
			},
		)
		if recordErr != nil ||
			(!observation.Applied && observation.Reason != "duplicate eventId") {
			return result, fmt.Errorf(
				"record VMP IDA execution observation: %w",
				recordErr,
			)
		}
		observationEventID = observation.EventID
	}
	result.ObservationEventID = observationEventID
	if opt.testHooks != nil && opt.testHooks.afterVMPObservation != nil {
		if err := opt.testHooks.afterVMPObservation(); err != nil {
			return result, err
		}
	}
	if result.ExecutionStatus == "succeeded" {
		owner, ownerErr := laneowner.Read(caseRoot, lane)
		if ownerErr != nil ||
			owner.CurrentExecutor != dispatch.Owner.CurrentExecutor ||
			owner.ExecutorGeneration != dispatch.Owner.ExecutorGeneration {
			return result, fmt.Errorf(
				"VMP IDA evidence owner changed before member task binding: %w",
				ownerErr,
			)
		}
		result.TaskBindingPath, result.TaskBindingSHA256, err =
			bindVMPIDAEvidence(
				caseRoot,
				lane,
				dispatch,
				dispatchPath,
				dispatchSHA,
				result,
				*validation.AdapterExecution,
			)
		return result, err
	}
	if result.ExecutionStatus != "failed" && result.ExecutionStatus != "aborted" {
		return result, fmt.Errorf("VMP IDA terminal execution status is unsupported: %s", result.ExecutionStatus)
	}
	return result, nil
}

func readVMPIDADispatchArtifact(caseRoot, lane, gateEventID string) (adapterexecution.DispatchReceipt, string, string, bool, error) {
	lane = strings.TrimSpace(lane)
	gateEventID = strings.TrimSpace(gateEventID)
	if lane == "" || gateEventID == "" {
		return adapterexecution.DispatchReceipt{}, "", "", false, nil
	}
	rel := filepath.ToSlash(filepath.Join(".rekit", "lanes", lane, "adapter-executions", gateEventID, "dispatch.json"))
	data, err := readVMPIDAFile(caseRoot, rel, "VMP IDA immutable dispatch", 1<<20)
	if errors.Is(err, os.ErrNotExist) {
		return adapterexecution.DispatchReceipt{}, rel, "", false, nil
	}
	if err != nil {
		return adapterexecution.DispatchReceipt{}, rel, "", false, err
	}
	dispatch, err := adapterexecution.DecodeDispatch(data)
	if err != nil {
		return dispatch, rel, sha256Hex(data), true, err
	}
	if dispatch.Gate.GateEventID != gateEventID || dispatch.Owner.Lane != lane || dispatch.Adapter.Pack != defaults.DefaultPack || dispatch.Adapter.AdapterID != VMPIDAIndexAdapterID {
		return dispatch, rel, sha256Hex(data), true, fmt.Errorf("VMP IDA immutable dispatch identity is invalid")
	}
	return dispatch, rel, sha256Hex(data), true, nil
}

func terminalVMPObservationEventID(caseRoot, lane, gateEventID string) (string, error) {
	observations, err := mission.ReadStrictFact(caseRoot, "observation")
	if err != nil {
		return "", err
	}
	for _, observation := range observations {
		item, ok := mission.ExecutionEvidenceReviewItemFromObservation(observation, lane, nil)
		if ok && item.GateEventID == gateEventID && item.EventID != "" {
			return item.EventID, nil
		}
	}
	return "", fmt.Errorf("terminal VMP IDA report has no exact execution observation")
}

func validateVMPIDAReceiptArtifacts(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA string,
	result AuthorizedRunResult,
	receipt adapterexecution.Receipt,
) error {
	dispatchData, err := readVMPIDAFile(
		caseRoot,
		dispatchPath,
		"VMP IDA receipt-bound dispatch",
		1<<20,
	)
	if err != nil {
		return err
	}
	if receipt.Dispatch.DispatchID != dispatch.DispatchID ||
		receipt.Dispatch.Path != dispatchPath ||
		!strings.EqualFold(receipt.Dispatch.SHA256, dispatchSHA) ||
		receipt.Dispatch.Bytes != int64(len(dispatchData)) ||
		receipt.Gate.GateEventID != result.GateEventID ||
		receipt.Owner != dispatch.Owner ||
		receipt.Adapter.Pack != result.Pack ||
		receipt.Adapter.AdapterID != VMPIDAIndexAdapterID {
		return fmt.Errorf(
			"VMP IDA receipt does not bind the exact dispatch, gate, adapter, and owner",
		)
	}
	reportData, err := readVMPIDAFile(
		caseRoot,
		result.ReportPath,
		"VMP IDA receipt-bound report",
		1<<20,
	)
	if err != nil || receipt.Report.Path != result.ReportPath ||
		!strings.EqualFold(receipt.Report.SHA256, sha256Hex(reportData)) ||
		receipt.Report.Bytes != int64(len(reportData)) {
		return fmt.Errorf("VMP IDA current report does not match its receipt: %w", err)
	}
	if receipt.Execution.Outcome != result.ExecutionStatus ||
		receipt.Execution.ExitStatus != result.ExecutionExitStatus {
		return fmt.Errorf("VMP IDA receipt outcome or exit status does not match the terminal report")
	}
	if result.ExecutionStatus == "failed" || result.ExecutionStatus == "aborted" {
		if len(receipt.Artifacts) != 0 || result.PacketPath != "" || result.PacketSHA256 != "" {
			return fmt.Errorf("failed VMP IDA receipt must not bind a success packet")
		}
		return nil
	}
	if result.ExecutionStatus != "succeeded" {
		return fmt.Errorf("VMP IDA receipt has unsupported terminal outcome: %s", result.ExecutionStatus)
	}
	packetData, err := readVMPIDAFile(
		caseRoot,
		result.PacketPath,
		"VMP IDA receipt-bound packet",
		VMPIDAIndexMaxPacketBytes,
	)
	if err != nil {
		return err
	}
	var packet VMPIDAIndexPacket
	if err := decodeVMPIDAStrictJSON(packetData, &packet); err != nil {
		return err
	}
	canonicalPacket, err := canonicalJSON(packet)
	if err != nil || !bytes.Equal(packetData, canonicalPacket) ||
		packet.RequestPath != dispatch.Gate.Target ||
		packet.RequestSHA256 != strings.TrimSuffix(
			filepath.Base(dispatch.Gate.Target),
			filepath.Ext(dispatch.Gate.Target),
		) {
		return fmt.Errorf(
			"VMP IDA current packet is not canonical or request-bound: %w",
			err,
		)
	}
	if len(receipt.Artifacts) != 1 {
		return fmt.Errorf("VMP IDA receipt must bind exactly one packet artifact")
	}
	artifact := receipt.Artifacts[0]
	if artifact.Path != result.PacketPath ||
		!reflect.DeepEqual(artifact.Roles, []string{"evidence", "output"}) ||
		!strings.EqualFold(artifact.SHA256, sha256Hex(packetData)) ||
		artifact.Bytes != int64(len(packetData)) {
		return fmt.Errorf("VMP IDA current packet does not match its receipt")
	}
	return nil
}

func bindVMPIDAEvidence(
	caseRoot,
	lane string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA string,
	result AuthorizedRunResult,
	receipt adapterexecution.Receipt,
) (string, string, error) {
	if err := validateVMPIDAReceiptArtifacts(
		caseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		result,
		receipt,
	); err != nil {
		return "", "", err
	}
	artifact := receipt.Artifacts[0]
	binding := memberexecution.TaskBinding{
		Kind: "vmp-ida-index-evidence",
		Values: map[string]string{
			"gate-event-id":        receipt.Gate.GateEventID,
			"profile-hash":         receipt.Gate.Authorization.ProfileHash,
			"request-path":         receipt.Gate.Target,
			"request-sha256":       strings.TrimSuffix(filepath.Base(receipt.Gate.Target), filepath.Ext(receipt.Gate.Target)),
			"packet-path":          artifact.Path,
			"packet-sha256":        artifact.SHA256,
			"report-path":          receipt.Report.Path,
			"report-sha256":        receipt.Report.SHA256,
			"dispatch-path":        receipt.Dispatch.Path,
			"dispatch-sha256":      receipt.Dispatch.SHA256,
			"receipt-path":         result.ReceiptPath,
			"receipt-sha256":       result.ReceiptSHA256,
			"observation-event-id": result.ObservationEventID,
		},
	}
	return memberexecution.WriteTaskBindingForOwner(
		caseRoot,
		lane,
		dispatch.Owner.CurrentExecutor,
		dispatch.Owner.ExecutorGeneration,
		binding,
	)
}

func authorizedGateLane(repoRoot, caseRoot, pack, gateEventID string) string {
	dispatch, _, _, _, err := gate.ReadCurrentAdapterExecutionDispatch(repoRoot, caseRoot, pack, gateEventID)
	if err == nil {
		return dispatch.Owner.Lane
	}
	// The dispatch does not exist yet. Derive the lane from the strict authorized
	// request ledger; RecordAdapterExecutionDispatch remains the canonical semantic
	// validator and rejects stale or malformed authorization.
	items, readErr := mission.ReadStrictFact(caseRoot, "request")
	if readErr != nil {
		return ""
	}
	for _, item := range items {
		if mission.Value(item, "eventId") == gateEventID && mission.IsAuthorizedGateRequest(item) {
			return mission.Value(item, "lane")
		}
	}
	return ""
}

func runVMPIDAExistingDispatch(opt Options, result Result, dispatch adapterexecution.DispatchReceipt, dispatchPath, dispatchSHA string, started time.Time) (_ Result, retErr error) {
	result.AdapterID = VMPIDAIndexAdapterID
	result.Lane = dispatch.Owner.Lane
	result.Executor = dispatch.Owner.CurrentExecutor
	result.Generation = dispatch.Owner.ExecutorGeneration
	result.AdapterSession = dispatch.Owner.AdapterSession
	result.DispatchPath = dispatchPath
	result.DispatchSHA256 = dispatchSHA
	result.InputPath = cleanCaseRelative(dispatch.Gate.Target)
	result.ReportPath = cleanCaseRelative(dispatch.ReportPath)
	result.ArtifactPath = filepath.ToSlash(filepath.Join(filepath.Dir(result.ReportPath), vmpIDAIndexPacketFileName))
	result.NoNetwork = true
	result.NoNetworkBoundary = fixedChildNoNetworkCodepath
	if err := validateVMPIDADispatch(dispatch, result.InputPath, result.ReportPath, result.ArtifactPath); err != nil {
		return result, err
	}
	lease, err := lanemutation.AcquireOpenLane(result.CaseRoot, result.Lane, "VMP IDA adapter host")
	if err != nil {
		return result, err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	current, currentPath, currentSHA, _, err := gate.ReadCurrentAdapterExecutionDispatch(opt.RepoRoot, result.CaseRoot, result.Pack, result.GateEventID)
	if err != nil || currentPath != dispatchPath || !strings.EqualFold(currentSHA, dispatchSHA) || !adapterexecution.DispatchSemanticEqual(current, dispatch) {
		return result, fmt.Errorf("VMP IDA adapter dispatch changed while acquiring lane lease: %w", err)
	}
	requestArtifact, err := readVMPIDAIndexRequestArtifact(result.CaseRoot, result.InputPath)
	if err != nil {
		return result, err
	}
	result.InputSHA256 = requestArtifact.RequestSHA256
	root, err := os.OpenRoot(result.CaseRoot)
	if err != nil {
		return result, err
	}
	defer root.Close()
	if err := ensureOutputParent(root, result.ReportPath); err != nil {
		return result, err
	}
	existing, err := readRecoverableVMPOutputs(
		result.CaseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		requestArtifact.RequestSHA256,
		result.ReportPath,
		result.ArtifactPath,
	)
	if err != nil {
		return result, err
	}
	if existing != nil {
		if len(existing.commitData) == 0 || !validSHA256(existing.launchSHA) {
			return result, fmt.Errorf("VMP IDA recoverable outputs lack exact commit or launch proof")
		}
		if existing.sealed {
			if err := publishVMPIDACommittedOutputsFromBytes(
				result.CaseRoot,
				result.ArtifactPath,
				result.ReportPath,
				existing.packetData,
				existing.reportData,
			); err != nil {
				return result, err
			}
			result.ProcessID = 0
			result.ExecutionStatus = "succeeded"
			result.ExecutionExitStatus = "completed"
			result.ArtifactSHA256 = sha256Hex(existing.packetData)
			result.ReportSHA256 = sha256Hex(existing.reportData)
			result.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
			return result, nil
		}
		var attemptStarted time.Time
		if !existing.sealed {
			attempt, _, present, attemptErr := readVMPIDAExecutionAttempt(
				result.CaseRoot,
				dispatch,
				dispatchPath,
				dispatchSHA,
				requestArtifact.RequestSHA256,
				result.ReportPath,
			)
			if attemptErr != nil || !present {
				return result, fmt.Errorf("VMP IDA unsealed committed outputs lack their exact execution attempt: %w", attemptErr)
			}
			attemptStarted, attemptErr = time.Parse(time.RFC3339Nano, attempt.StartedAt)
			if attemptErr != nil {
				return result, attemptErr
			}
		}
		if !existing.sealed {
			requestCurrent, currentErr := ReadVMPIDAIndexRequest(result.CaseRoot, result.InputPath)
			if currentErr != nil || requestCurrent.RequestSHA256 != requestArtifact.RequestSHA256 ||
				!bytes.Equal(requestCurrent.CanonicalBytes, requestArtifact.CanonicalBytes) {
				if err := removeExactVMPIDAPublicOutputs(
					result.CaseRoot,
					result.ArtifactPath,
					result.ReportPath,
					existing.packetData,
					existing.reportData,
				); err != nil {
					return result, err
				}
				return publishVMPIDAFailureReport(
					result,
					dispatch,
					dispatchPath,
					dispatchSHA,
					attemptStarted,
					"failed",
					"source-drift",
					existing.launchSHA,
				)
			}
			if elapsedSecondsCeil(attemptStarted) > dispatch.Gate.AuthorizedBudget.RuntimeSeconds {
				if err := removeExactVMPIDAPublicOutputs(result.CaseRoot, result.ArtifactPath, result.ReportPath, existing.packetData, existing.reportData); err != nil {
					return result, err
				}
				return publishVMPIDAFailureReport(
					result,
					dispatch,
					dispatchPath,
					dispatchSHA,
					attemptStarted,
					"aborted",
					"runtime-budget-exceeded",
					existing.launchSHA,
				)
			}
		}
		if err := publishVMPIDACommittedOutputsFromBytes(
			result.CaseRoot,
			result.ArtifactPath,
			result.ReportPath,
			existing.packetData,
			existing.reportData,
		); err != nil {
			return result, err
		}
		if !existing.sealed {
			requestCurrent, currentErr := ReadVMPIDAIndexRequest(result.CaseRoot, result.InputPath)
			if currentErr != nil || requestCurrent.RequestSHA256 != requestArtifact.RequestSHA256 ||
				!bytes.Equal(requestCurrent.CanonicalBytes, requestArtifact.CanonicalBytes) {
				if err := removeExactVMPIDAPublicOutputs(
					result.CaseRoot,
					result.ArtifactPath,
					result.ReportPath,
					existing.packetData,
					existing.reportData,
				); err != nil {
					return result, err
				}
				return publishVMPIDAFailureReport(
					result,
					dispatch,
					dispatchPath,
					dispatchSHA,
					attemptStarted,
					"failed",
					"source-drift",
					existing.launchSHA,
				)
			}
			if opt.testHooks != nil && opt.testHooks.beforeVMPSuccessSeal != nil {
				if err := opt.testHooks.beforeVMPSuccessSeal(); err != nil {
					return result, err
				}
			}
			requestCurrent, currentErr = ReadVMPIDAIndexRequest(result.CaseRoot, result.InputPath)
			if currentErr != nil || requestCurrent.RequestSHA256 != requestArtifact.RequestSHA256 ||
				!bytes.Equal(requestCurrent.CanonicalBytes, requestArtifact.CanonicalBytes) {
				if err := removeExactVMPIDAPublicOutputs(
					result.CaseRoot,
					result.ArtifactPath,
					result.ReportPath,
					existing.packetData,
					existing.reportData,
				); err != nil {
					return result, err
				}
				return publishVMPIDAFailureReport(
					result,
					dispatch,
					dispatchPath,
					dispatchSHA,
					attemptStarted,
					"failed",
					"source-drift",
					existing.launchSHA,
				)
			}
			if err := lease.Validate(); err != nil {
				if cleanupErr := removeExactVMPIDAPublicOutputs(result.CaseRoot, result.ArtifactPath, result.ReportPath, existing.packetData, existing.reportData); cleanupErr != nil {
					return result, errors.Join(err, cleanupErr)
				}
				return result, err
			}
			if elapsedSecondsCeil(attemptStarted) > dispatch.Gate.AuthorizedBudget.RuntimeSeconds {
				if err := removeExactVMPIDAPublicOutputs(result.CaseRoot, result.ArtifactPath, result.ReportPath, existing.packetData, existing.reportData); err != nil {
					return result, err
				}
				return publishVMPIDAFailureReport(
					result,
					dispatch,
					dispatchPath,
					dispatchSHA,
					attemptStarted,
					"aborted",
					"runtime-budget-exceeded",
					existing.launchSHA,
				)
			}
			if err := publishVMPIDASuccessSeal(
				result.CaseRoot,
				dispatch,
				dispatchSHA,
				requestArtifact.RequestSHA256,
				existing.launchSHA,
				result.ReportPath,
				existing.commitData,
			); err != nil {
				return result, err
			}
		}
		result.ProcessID = 0
		result.ExecutionStatus = "succeeded"
		result.ExecutionExitStatus = "completed"
		result.ArtifactSHA256 = sha256Hex(existing.packetData)
		result.ReportSHA256 = sha256Hex(existing.reportData)
		result.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
		return result, nil
	}
	if err := validateCurrentAuthorization(opt.RepoRoot, result.CaseRoot, dispatch); err != nil {
		return result, err
	}
	if _, err := ReadVMPIDAIndexRequest(result.CaseRoot, result.InputPath); err != nil {
		return result, err
	}
	if _, launchErr := readVMPIDAFile(
		result.CaseRoot,
		vmpIDAChildLaunchPath(result.ReportPath),
		"VMP IDA prior child launch proof",
		64<<10,
	); launchErr == nil {
		return result, fmt.Errorf("VMP IDA dispatch has a child launch proof without a terminal outcome; a distinct authorized gate and dispatch are required")
	} else if !errors.Is(launchErr, os.ErrNotExist) {
		return result, launchErr
	}
	attempt, attemptSHA, err := publishVMPIDAExecutionAttempt(
		result.CaseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		requestArtifact.RequestSHA256,
		result.ExecutableSHA256,
		result.ReportPath,
		started,
	)
	if err != nil {
		return result, err
	}
	started, err = time.Parse(time.RFC3339Nano, attempt.StartedAt)
	if err != nil {
		return result, err
	}
	remaining := time.Until(
		started.Add(
			time.Duration(dispatch.Gate.AuthorizedBudget.RuntimeSeconds) * time.Second,
		),
	)
	if remaining <= 0 {
		return result, fmt.Errorf("VMP IDA execution attempt exhausted its runtime budget before child launch; a distinct authorized gate and dispatch are required")
	}
	launchSHA := ""
	launchHookFailed := false
	var errVMPIDAChildLaunchIncomplete = errors.New("VMP IDA child launch was recorded without a terminal outcome")
	afterLaunch := func(childPID int) error {
		var launchErr error
		launchSHA, launchErr = publishVMPIDAChildLaunch(
			result.CaseRoot,
			result.ReportPath,
			attemptSHA,
			childPID,
		)
		if launchErr == nil && opt.testHooks != nil && opt.testHooks.afterVMPChildLaunch != nil {
			launchErr = opt.testHooks.afterVMPChildLaunch(childPID)
			launchHookFailed = launchErr != nil
		}
		return launchErr
	}
	stdout, childPID, err := runVMPIDAChild(
		opt,
		dispatch,
		dispatchSHA,
		result.InputPath,
		remaining,
		result.ExecutableSHA256,
		afterLaunch,
	)
	result.ProcessID = childPID
	if err != nil {
		if childPID <= 0 {
			return result, err
		}
		if launchHookFailed {
			return result, errors.Join(errVMPIDAChildLaunchIncomplete, err)
		}
		status := "failed"
		exitStatus := "child-failed"
		if errors.Is(err, errContainedProcessTimeout) {
			status = "aborted"
			exitStatus = "child-timeout"
		} else {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && exitErr.ExitCode() >= 0 {
				exitStatus = fmt.Sprintf("child-exit-%d", exitErr.ExitCode())
			}
		}
		return publishVMPIDAFailureReport(
			result,
			dispatch,
			dispatchPath,
			dispatchSHA,
			started,
			status,
			exitStatus,
			launchSHA,
		)
	}
	child, err := decodeVMPIDAChildResult(stdout, result.InputPath)
	if err != nil {
		if childPID <= 0 {
			return result, err
		}
		exitStatus := "child-invalid-stdout"
		if errors.Is(err, errVMPIDAChildInvalidPacket) {
			exitStatus = "child-invalid-packet"
		}
		return publishVMPIDAFailureReport(
			result,
			dispatch,
			dispatchPath,
			dispatchSHA,
			started,
			"failed",
			exitStatus,
			launchSHA,
		)
	}
	packetData, err := canonicalJSON(child.Packet)
	if err != nil || !strings.EqualFold(sha256Hex(packetData), child.PacketSHA256) {
		return publishVMPIDAFailureReport(
			result,
			dispatch,
			dispatchPath,
			dispatchSHA,
			started,
			"failed",
			"child-invalid-packet",
			launchSHA,
		)
	}
	if opt.testHooks != nil && opt.testHooks.beforeVMPPublication != nil {
		if err := opt.testHooks.beforeVMPPublication(); err != nil {
			return result, err
		}
	}
	requestAfter, err := ReadVMPIDAIndexRequest(result.CaseRoot, result.InputPath)
	if err != nil || requestAfter.RequestSHA256 != requestArtifact.RequestSHA256 || !bytes.Equal(requestAfter.CanonicalBytes, requestArtifact.CanonicalBytes) {
		return publishVMPIDAFailureReport(
			result,
			dispatch,
			dispatchPath,
			dispatchSHA,
			started,
			"failed",
			"source-drift",
			launchSHA,
		)
	}
	latest, latestPath, latestSHA, _, err := gate.ReadCurrentAdapterExecutionDispatch(opt.RepoRoot, result.CaseRoot, result.Pack, result.GateEventID)
	if err != nil || latestPath != dispatchPath || !strings.EqualFold(latestSHA, dispatchSHA) || !adapterexecution.DispatchSemanticEqual(latest, dispatch) {
		return result, fmt.Errorf("VMP IDA adapter dispatch changed before output publication: %w", err)
	}
	if err := validateCurrentAuthorization(opt.RepoRoot, result.CaseRoot, dispatch); err != nil {
		return result, err
	}
	report := gate.AdapterReport{
		SchemaVersion: 1, Kind: "adapter-execution-report", AdapterID: VMPIDAIndexAdapterID,
		Action: "inspect", Status: "succeeded", GateEventID: result.GateEventID,
		Dispatch:     &adapterexecution.ReportDispatchBinding{DispatchID: dispatch.DispatchID, Path: dispatchPath, SHA256: dispatchSHA},
		ActualBudget: autonomy.Budget{RuntimeSeconds: elapsedSecondsCeil(started), DiskMB: 1, Requests: 1},
		OutputRefs:   []string{result.ArtifactPath}, EvidenceRefs: []string{result.ArtifactPath},
		Summary: "Bounded literal inspection of existing IDA TSV exports completed through a fixed compiled-in child code path.",
	}
	if report.ActualBudget.RuntimeSeconds > dispatch.Gate.AuthorizedBudget.RuntimeSeconds {
		return publishVMPIDAFailureReport(
			result,
			dispatch,
			dispatchPath,
			dispatchSHA,
			started,
			"aborted",
			"runtime-budget-exceeded",
			launchSHA,
		)
	}
	if report.ActualBudget.DiskMB > dispatch.Gate.AuthorizedBudget.DiskMB ||
		report.ActualBudget.Requests > dispatch.Gate.AuthorizedBudget.Requests ||
		int64(len(packetData)) > int64(dispatch.Gate.AuthorizedBudget.DiskMB)<<20 {
		return publishVMPIDAFailureReport(
			result,
			dispatch,
			dispatchPath,
			dispatchSHA,
			started,
			"failed",
			"output-budget-exceeded",
			launchSHA,
		)
	}
	reportData, err := canonicalJSON(report)
	if err != nil {
		return result, err
	}
	commit, commitData, commitPath, err := buildVMPIDAOutputCommit(
		dispatch,
		dispatchPath,
		dispatchSHA,
		requestArtifact.RequestSHA256,
		result.ReportPath,
		result.ArtifactPath,
		reportData,
		packetData,
	)
	if err != nil {
		return result, err
	}
	if _, err := rekitfs.WriteExclusiveRegularFileAnchored(
		result.CaseRoot,
		commitPath,
		"VMP IDA output commit",
		commitData,
	); err != nil {
		return result, err
	}
	if opt.testHooks != nil && opt.testHooks.afterVMPStageCommit != nil {
		if err := opt.testHooks.afterVMPStageCommit(); err != nil {
			return result, err
		}
	}
	requestFinal, err := ReadVMPIDAIndexRequest(result.CaseRoot, result.InputPath)
	if err != nil || requestFinal.RequestSHA256 != requestArtifact.RequestSHA256 ||
		!bytes.Equal(requestFinal.CanonicalBytes, requestArtifact.CanonicalBytes) {
		return publishVMPIDAFailureReport(
			result,
			dispatch,
			dispatchPath,
			dispatchSHA,
			started,
			"failed",
			"source-drift",
			launchSHA,
		)
	}
	if err := lease.Validate(); err != nil {
		return result, err
	}
	if err := publishVMPIDACommittedOutputs(result.CaseRoot, commit, opt.testHooks); err != nil {
		return result, err
	}
	if opt.testHooks != nil && opt.testHooks.beforeVMPSuccessSeal != nil {
		if err := opt.testHooks.beforeVMPSuccessSeal(); err != nil {
			return result, err
		}
	}
	requestFinal, err = ReadVMPIDAIndexRequest(result.CaseRoot, result.InputPath)
	if err != nil || requestFinal.RequestSHA256 != requestArtifact.RequestSHA256 ||
		!bytes.Equal(requestFinal.CanonicalBytes, requestArtifact.CanonicalBytes) {
		if cleanupErr := removeExactVMPIDAPublicOutputs(
			result.CaseRoot,
			result.ArtifactPath,
			result.ReportPath,
			packetData,
			reportData,
		); cleanupErr != nil {
			return result, cleanupErr
		}
		return publishVMPIDAFailureReport(
			result,
			dispatch,
			dispatchPath,
			dispatchSHA,
			started,
			"failed",
			"source-drift",
			launchSHA,
		)
	}
	if err := lease.Validate(); err != nil {
		if cleanupErr := removeExactVMPIDAPublicOutputs(result.CaseRoot, result.ArtifactPath, result.ReportPath, packetData, reportData); cleanupErr != nil {
			return result, errors.Join(err, cleanupErr)
		}
		return result, err
	}
	if elapsedSecondsCeil(started) > dispatch.Gate.AuthorizedBudget.RuntimeSeconds {
		if err := removeExactVMPIDAPublicOutputs(
			result.CaseRoot,
			result.ArtifactPath,
			result.ReportPath,
			packetData,
			reportData,
		); err != nil {
			return result, err
		}
		return publishVMPIDAFailureReport(
			result,
			dispatch,
			dispatchPath,
			dispatchSHA,
			started,
			"aborted",
			"runtime-budget-exceeded",
			launchSHA,
		)
	}
	if err := publishVMPIDASuccessSeal(
		result.CaseRoot,
		dispatch,
		dispatchSHA,
		requestArtifact.RequestSHA256,
		launchSHA,
		result.ReportPath,
		commitData,
	); err != nil {
		return result, err
	}
	result.ArtifactSHA256 = commit.Packet.SHA256
	result.ReportSHA256 = commit.Report.SHA256
	result.ExecutionStatus = "succeeded"
	result.ExecutionExitStatus = "completed"
	result.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if opt.testHooks != nil && opt.testHooks.afterVMPOutputCommit != nil {
		if err := opt.testHooks.afterVMPOutputCommit(); err != nil {
			return result, err
		}
	}
	return result, nil
}

func publishVMPIDAFailureReport(
	result Result,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA string,
	started time.Time,
	status,
	exitStatus,
	launchSHA string,
) (Result, error) {
	if !validVMPIDAFailureExitStatus(exitStatus) {
		return result, fmt.Errorf("invalid VMP IDA failure exit status: %s", exitStatus)
	}
	if !validSHA256(launchSHA) {
		return result, fmt.Errorf("VMP IDA failure report requires the exact parent-owned child launch proof")
	}
	if err := validateVMPIDAChildLaunchAttempt(
		result.CaseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		result.InputSHA256,
		result.ReportPath,
		launchSHA,
	); err != nil {
		return result, err
	}
	report := gate.AdapterReport{
		SchemaVersion: 1,
		Kind:          "adapter-execution-report",
		AdapterID:     VMPIDAIndexAdapterID,
		Action:        "inspect",
		Status:        status,
		GateEventID:   result.GateEventID,
		Dispatch: &adapterexecution.ReportDispatchBinding{
			DispatchID: dispatch.DispatchID,
			Path:       dispatchPath,
			SHA256:     dispatchSHA,
		},
		ActualBudget: autonomy.Budget{
			RuntimeSeconds: elapsedSecondsCeil(started),
			Requests:       1,
		},
		Escalation: vmpIDAFailureEscalation(exitStatus, launchSHA),
		Summary:    "The fixed compiled-in IDA index child reached a terminal failure; no packet was published and this dispatch must not be replayed.",
	}
	if exceedsBudget(dispatch, report) {
		report.ActualBudget.RuntimeSeconds = dispatch.Gate.AuthorizedBudget.RuntimeSeconds
	}
	reportData, err := canonicalJSON(report)
	if err != nil {
		return result, err
	}
	if _, err := rekitfs.WriteExclusiveRegularFileAnchored(
		result.CaseRoot,
		result.ReportPath,
		"VMP IDA terminal failure report",
		reportData,
	); err != nil {
		return result, err
	}
	result.ArtifactPath = ""
	result.ArtifactSHA256 = ""
	result.ReportSHA256 = sha256Hex(reportData)
	result.ExecutionStatus = status
	result.ExecutionExitStatus = exitStatus
	result.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return result, nil
}

func validateVMPIDADispatch(dispatch adapterexecution.DispatchReceipt, requestPath, reportPath, packetPath string) error {
	if dispatch.Adapter.Pack != defaults.DefaultPack || dispatch.Adapter.AdapterID != VMPIDAIndexAdapterID || dispatch.Adapter.Candidate.ID != VMPIDAIndexAdapterID || dispatch.Gate.Action != "inspect" || dispatch.Owner.AdapterHarness != adapterHarness {
		return fmt.Errorf("VMP IDA adapter accepts only pack=%s action=inspect harness=%s and compiled-in candidate=%s", defaults.DefaultPack, adapterHarness, VMPIDAIndexAdapterID)
	}
	if requestPath == "" || requestPath != dispatch.Gate.Target || reportPath == "" || packetPath == reportPath || packetPath == requestPath || reportPath == requestPath {
		return fmt.Errorf("VMP IDA adapter dispatch has invalid request or output bindings")
	}
	if !withinAuthorizedOutput(packetPath, dispatch.Gate.OutputPaths) || !withinAuthorizedOutput(reportPath, dispatch.Gate.OutputPaths) {
		return fmt.Errorf("VMP IDA adapter packet and report must stay within exact authorized output paths")
	}
	if dispatch.Gate.AuthorizedBudget.RuntimeSeconds < 1 || dispatch.Gate.AuthorizedBudget.DiskMB < 1 || dispatch.Gate.AuthorizedBudget.Requests != 1 {
		return fmt.Errorf("VMP IDA adapter dispatch budget is not bounded for one request")
	}
	return nil
}

func validateCurrentAuthorization(repoRoot, caseRoot string, dispatch adapterexecution.DispatchReceipt) error {
	m, err := manifest.Load(repoRoot, dispatch.Adapter.Pack)
	if err != nil {
		return err
	}
	profile, profilePath, exists, err := autonomy.Read(caseRoot, dispatch.Owner.Lane)
	if err != nil || !exists {
		return fmt.Errorf("read current VMP IDA autonomy profile: %w", err)
	}
	profileHash := autonomy.FileHash(profilePath)
	fresh, _, freshExists, err := autonomy.Read(caseRoot, dispatch.Owner.Lane)
	if err != nil || !freshExists || profileHash == "" || autonomy.FileHash(profilePath) != profileHash || !reflect.DeepEqual(profile, fresh) {
		return fmt.Errorf("VMP IDA autonomy profile changed while reading")
	}
	profile = fresh
	if err := autonomy.Validate(profile, dispatch.Owner.Lane, m, caseRoot); err != nil {
		return err
	}
	if profile.Mode != autonomy.ModePreauthorized || autonomy.IsExpired(profile, time.Now().UTC()) {
		return fmt.Errorf("VMP IDA autonomy profile is not current preauthorized or has expired")
	}
	authorization := dispatch.Gate.Authorization
	if authorization.Decision != autonomy.DecisionPreauthorized || authorization.RequiresConfirmation || !strings.EqualFold(authorization.ProfileHash, profileHash) || authorization.ProfilePath != autonomy.RelPath(dispatch.Owner.Lane) {
		return fmt.Errorf("VMP IDA authorization profile hash, path, or decision drifted")
	}
	request := autonomy.Request{
		Lane: dispatch.Owner.Lane, Action: dispatch.Gate.Action, Target: dispatch.Gate.Target,
		Budget: dispatch.Gate.AuthorizedBudget, StopConditions: append([]string{}, dispatch.Gate.StopConditions...),
		OutputPaths: append([]string{}, dispatch.Gate.OutputPaths...),
	}
	freshDecision := autonomy.Evaluate(profile, autonomy.RelPath(dispatch.Owner.Lane), true, profileHash, request, time.Now().UTC())
	if !authorizationDecisionEqual(freshDecision, authorization) {
		return fmt.Errorf("VMP IDA authorization decision drifted from the current profile")
	}
	if !reflect.DeepEqual(profile.Budget, dispatch.Gate.AuthorizedBudget) {
		return fmt.Errorf("VMP IDA authorization budget drifted from the current profile")
	}
	if !reflect.DeepEqual(profile.OutputPaths, dispatch.Gate.OutputPaths) {
		return fmt.Errorf("VMP IDA authorization output paths drifted from the current profile")
	}
	if !reflect.DeepEqual(profile.StopConditions, dispatch.Gate.StopConditions) {
		return fmt.Errorf("VMP IDA authorization stop conditions drifted from the current profile")
	}
	expectedDeniedActions := make([]string, 0, len(m.HeavyToolGates))
	for _, action := range m.HeavyToolGateIDs() {
		if action != "inspect" {
			expectedDeniedActions = append(expectedDeniedActions, action)
		}
	}
	if !reflect.DeepEqual(profile.AllowedActions, []string{"inspect"}) || !reflect.DeepEqual(profile.DeniedActions, expectedDeniedActions) || len(profile.TargetScope) != 1 || profile.TargetScope[0] != (autonomy.Target{Match: "exact", Value: dispatch.Gate.Target}) {
		return fmt.Errorf("VMP IDA authorization action or target drifted from the current profile")
	}
	return nil
}

func authorizationDecisionEqual(left, right autonomy.Decision) bool {
	leftData, leftErr := json.Marshal(left)
	rightData, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftData, rightData)
}

func runVMPIDAChild(
	opt Options,
	dispatch adapterexecution.DispatchReceipt,
	dispatchSHA,
	requestPath string,
	timeout time.Duration,
	executableSHA string,
	afterLaunch func(int) error,
) ([]byte, int, error) {
	childOpt := VMPIDAIndexChildOptions{
		RepoRoot:                   opt.RepoRoot,
		CaseRoot:                   opt.CaseRoot,
		Pack:                       opt.Pack,
		GateEventID:                opt.GateEventID,
		ExpectedDispatchSHA256:     dispatchSHA,
		AdapterSession:             dispatch.Owner.AdapterSession,
		Executor:                   dispatch.Owner.CurrentExecutor,
		ExpectedExecutorGeneration: dispatch.Owner.ExecutorGeneration,
		RequestPath:                requestPath,
	}
	if opt.testHooks != nil && opt.testHooks.runVMPIDAChild != nil {
		stdout, childPID, err := opt.testHooks.runVMPIDAChild(childOpt)
		if childPID > 0 && afterLaunch != nil {
			if launchErr := afterLaunch(childPID); launchErr != nil {
				return stdout, childPID, errors.Join(err, launchErr)
			}
		}
		return stdout, childPID, err
	}
	executablePath, err := os.Executable()
	if err != nil {
		return nil, 0, err
	}
	binding, err := processguard.LockExecutable(executablePath, 128<<20)
	if err != nil {
		return nil, 0, err
	}
	defer binding.Close()
	if !strings.EqualFold(binding.SHA256(), executableSHA) {
		return nil, 0, fmt.Errorf("adapter executable identity changed before child launch")
	}
	args := []string{
		"-child-vmp-ida-index-inspector",
		"-repo", childOpt.RepoRoot,
		"-target", childOpt.CaseRoot,
		"-pack", childOpt.Pack,
		"-gate-event-id", childOpt.GateEventID,
		"-expected-dispatch-sha256", childOpt.ExpectedDispatchSHA256,
		"-adapter-session", childOpt.AdapterSession,
		"-executor", childOpt.Executor,
		"-expected-executor-generation", fmt.Sprintf(
			"%d",
			childOpt.ExpectedExecutorGeneration,
		),
		"-child-request-path", childOpt.RequestPath,
	}
	stdout, _, childPID, err := runContainedProcessObserved(
		binding,
		args,
		fixedChildEnvironment(),
		timeout,
		afterLaunch,
	)
	return stdout, childPID, err
}

func fixedChildEnvironment() []string {
	keys := []string{"SystemRoot", "WINDIR", "COMSPEC", "PATHEXT", "TEMP", "TMP"}
	env := make([]string, 0, len(keys)+1)
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
			env = append(env, key+"="+value)
		}
	}
	env = append(env, "REKIT_CHILD_BOUNDARY="+fixedChildNoNetworkCodepath)
	if runtime.GOOS != "windows" {
		env = append(env, "PATH=/usr/bin:/bin")
	}
	return env
}

var errVMPIDAChildInvalidPacket = errors.New("VMP IDA child returned an invalid packet binding")

func decodeVMPIDAChildResult(data []byte, requestPath string) (VMPIDAIndexChildResult, error) {
	var result VMPIDAIndexChildResult
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("decode VMP IDA child result: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return result, fmt.Errorf("VMP IDA child stdout must contain exactly one strict JSON object")
	}
	if result.SchemaVersion != 1 || result.Kind != vmpIDAChildResultKind ||
		result.AdapterID != VMPIDAIndexAdapterID || result.RequestPath != requestPath ||
		!result.ReadOnlyInput || !result.NoNetwork ||
		result.NoNetworkBoundary != fixedChildNoNetworkCodepath || !result.NoAuthority {
		return result, fmt.Errorf("VMP IDA child returned an invalid strict result envelope")
	}
	packetData, err := canonicalJSON(result.Packet)
	if err != nil || result.Packet.SchemaVersion != vmpIDAIndexPacketSchemaVersion ||
		result.Packet.Kind != vmpIDAIndexPacketKind ||
		result.Packet.AdapterID != VMPIDAIndexAdapterID ||
		result.Packet.RequestPath != requestPath ||
		result.RequestSHA256 != result.Packet.RequestSHA256 ||
		!validSHA256(result.RequestSHA256) ||
		!strings.EqualFold(result.PacketSHA256, sha256Hex(packetData)) {
		return result, fmt.Errorf("%w: %v", errVMPIDAChildInvalidPacket, err)
	}
	return result, nil
}

type recoverableVMPOutputs struct {
	reportData []byte
	packetData []byte
	commitData []byte
	launchSHA  string
	sealed     bool
}

type vmpIDAExecutionAttempt struct {
	SchemaVersion    int    `json:"schemaVersion"`
	Kind             string `json:"kind"`
	AdapterID        string `json:"adapterId"`
	GateEventID      string `json:"gateEventId"`
	DispatchID       string `json:"dispatchId"`
	DispatchPath     string `json:"dispatchPath"`
	DispatchSHA256   string `json:"dispatchSha256"`
	RequestPath      string `json:"requestPath"`
	RequestSHA256    string `json:"requestSha256"`
	ExecutableSHA256 string `json:"executableSha256"`
	StartedAt        string `json:"startedAt"`
	Nonce            string `json:"nonce"`
}

type vmpIDAChildLaunch struct {
	SchemaVersion     int    `json:"schemaVersion"`
	Kind              string `json:"kind"`
	AdapterID         string `json:"adapterId"`
	AttemptSHA256     string `json:"attemptSha256"`
	ChildProcessID    int    `json:"childProcessId"`
	NoNetworkBoundary string `json:"noNetworkBoundary"`
}

type vmpIDASuccessSeal struct {
	SchemaVersion      int    `json:"schemaVersion"`
	Kind               string `json:"kind"`
	AdapterID          string `json:"adapterId"`
	GateEventID        string `json:"gateEventId"`
	DispatchID         string `json:"dispatchId"`
	DispatchSHA256     string `json:"dispatchSha256"`
	RequestPath        string `json:"requestPath"`
	RequestSHA256      string `json:"requestSha256"`
	ChildLaunchSHA256  string `json:"childLaunchSha256"`
	OutputCommitSHA256 string `json:"outputCommitSha256"`
}

type vmpIDAOutputCommitFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

func vmpIDAExecutionAttemptPath(reportPath string) string {
	return filepath.ToSlash(filepath.Join(
		filepath.Dir(reportPath),
		vmpIDAExecutionAttemptFileName,
	))
}

func vmpIDAChildLaunchPath(reportPath string) string {
	return filepath.ToSlash(filepath.Join(
		filepath.Dir(reportPath),
		vmpIDAChildLaunchFileName,
	))
}

func vmpIDASuccessSealPath(reportPath string) string {
	return filepath.ToSlash(filepath.Join(
		filepath.Dir(reportPath),
		vmpIDASuccessSealFileName,
	))
}

func readVMPIDAExecutionAttempt(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA,
	requestSHA,
	reportPath string,
) (vmpIDAExecutionAttempt, string, bool, error) {
	data, err := readVMPIDAFile(
		caseRoot,
		vmpIDAExecutionAttemptPath(reportPath),
		"VMP IDA execution attempt",
		64<<10,
	)
	if errors.Is(err, os.ErrNotExist) {
		return vmpIDAExecutionAttempt{}, "", false, nil
	}
	if err != nil {
		return vmpIDAExecutionAttempt{}, "", false, err
	}
	var attempt vmpIDAExecutionAttempt
	if err := decodeVMPIDAStrictJSON(data, &attempt); err != nil {
		return attempt, "", true, err
	}
	canonical, err := canonicalJSON(attempt)
	if err != nil || !bytes.Equal(data, canonical) ||
		attempt.SchemaVersion != 1 || attempt.Kind != vmpIDAExecutionAttemptKind ||
		attempt.AdapterID != VMPIDAIndexAdapterID ||
		attempt.GateEventID != dispatch.Gate.GateEventID ||
		attempt.DispatchID != dispatch.DispatchID ||
		attempt.DispatchPath != dispatchPath ||
		!strings.EqualFold(attempt.DispatchSHA256, dispatchSHA) ||
		attempt.RequestPath != dispatch.Gate.Target ||
		!strings.EqualFold(attempt.RequestSHA256, requestSHA) ||
		!validSHA256(attempt.ExecutableSHA256) ||
		len(attempt.Nonce) != 32 {
		return attempt, "", true, fmt.Errorf("VMP IDA execution attempt is not exact or canonical: %w", err)
	}
	started, startedErr := time.Parse(time.RFC3339Nano, attempt.StartedAt)
	if startedErr != nil || !strings.HasSuffix(attempt.StartedAt, "Z") ||
		attempt.StartedAt != started.UTC().Format(time.RFC3339Nano) {
		return attempt, "", true, fmt.Errorf("VMP IDA execution attempt start time is invalid or non-canonical: %w", startedErr)
	}
	if _, err := hex.DecodeString(attempt.Nonce); err != nil {
		return attempt, "", true, fmt.Errorf("VMP IDA execution attempt nonce is invalid: %w", err)
	}
	return attempt, sha256Hex(data), true, nil
}

func publishVMPIDAExecutionAttempt(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA,
	requestSHA,
	executableSHA,
	reportPath string,
	started time.Time,
) (vmpIDAExecutionAttempt, string, error) {
	if existing, existingSHA, present, err := readVMPIDAExecutionAttempt(
		caseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		requestSHA,
		reportPath,
	); err != nil {
		return vmpIDAExecutionAttempt{}, "", err
	} else if present {
		if !strings.EqualFold(existing.ExecutableSHA256, executableSHA) {
			return vmpIDAExecutionAttempt{}, "", fmt.Errorf("VMP IDA execution attempt executable identity changed")
		}
		return existing, existingSHA, nil
	}
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return vmpIDAExecutionAttempt{}, "", err
	}
	attempt := vmpIDAExecutionAttempt{
		SchemaVersion:    1,
		Kind:             vmpIDAExecutionAttemptKind,
		AdapterID:        VMPIDAIndexAdapterID,
		GateEventID:      dispatch.Gate.GateEventID,
		DispatchID:       dispatch.DispatchID,
		DispatchPath:     dispatchPath,
		DispatchSHA256:   strings.ToLower(dispatchSHA),
		RequestPath:      dispatch.Gate.Target,
		RequestSHA256:    strings.ToLower(requestSHA),
		ExecutableSHA256: strings.ToLower(executableSHA),
		StartedAt:        started.UTC().Format(time.RFC3339Nano),
		Nonce:            hex.EncodeToString(nonceBytes),
	}
	data, err := canonicalJSON(attempt)
	if err != nil {
		return vmpIDAExecutionAttempt{}, "", err
	}
	if _, err := rekitfs.WriteExclusiveRegularFileAnchored(
		caseRoot,
		vmpIDAExecutionAttemptPath(reportPath),
		"VMP IDA execution attempt",
		data,
	); err != nil {
		return vmpIDAExecutionAttempt{}, "", err
	}
	return attempt, sha256Hex(data), nil
}

func publishVMPIDAChildLaunch(
	caseRoot,
	reportPath,
	attemptSHA string,
	childPID int,
) (string, error) {
	launch := vmpIDAChildLaunch{
		SchemaVersion:     1,
		Kind:              vmpIDAChildLaunchKind,
		AdapterID:         VMPIDAIndexAdapterID,
		AttemptSHA256:     strings.ToLower(attemptSHA),
		ChildProcessID:    childPID,
		NoNetworkBoundary: fixedChildNoNetworkCodepath,
	}
	if childPID < 1 || !validSHA256(launch.AttemptSHA256) {
		return "", fmt.Errorf("VMP IDA child launch proof requires an exact attempt and positive process id")
	}
	data, err := canonicalJSON(launch)
	if err != nil {
		return "", err
	}
	if _, err := rekitfs.WriteExclusiveRegularFileAnchoredWriteThrough(
		caseRoot,
		vmpIDAChildLaunchPath(reportPath),
		"VMP IDA child launch proof",
		data,
	); err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}

func readVMPIDAChildLaunch(
	caseRoot,
	reportPath,
	expectedSHA string,
) (vmpIDAChildLaunch, error) {
	data, err := readVMPIDAFile(
		caseRoot,
		vmpIDAChildLaunchPath(reportPath),
		"VMP IDA child launch proof",
		64<<10,
	)
	if err != nil {
		return vmpIDAChildLaunch{}, err
	}
	var launch vmpIDAChildLaunch
	if err := decodeVMPIDAStrictJSON(data, &launch); err != nil {
		return launch, err
	}
	canonical, err := canonicalJSON(launch)
	if err != nil || !bytes.Equal(data, canonical) ||
		launch.SchemaVersion != 1 || launch.Kind != vmpIDAChildLaunchKind ||
		launch.AdapterID != VMPIDAIndexAdapterID ||
		!validSHA256(launch.AttemptSHA256) || launch.ChildProcessID < 1 ||
		launch.NoNetworkBoundary != fixedChildNoNetworkCodepath ||
		!strings.EqualFold(sha256Hex(data), expectedSHA) {
		return launch, fmt.Errorf("VMP IDA child launch proof is not exact or canonical: %w", err)
	}
	return launch, nil
}

func validateVMPIDAChildLaunchAttempt(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA,
	requestSHA,
	reportPath,
	launchSHA string,
) error {
	launch, err := readVMPIDAChildLaunch(caseRoot, reportPath, launchSHA)
	if err != nil {
		return err
	}
	_, attemptSHA, present, err := readVMPIDAExecutionAttempt(
		caseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		requestSHA,
		reportPath,
	)
	if err != nil || !present || !strings.EqualFold(launch.AttemptSHA256, attemptSHA) {
		return fmt.Errorf("VMP IDA child launch proof does not bind the exact parent-owned execution attempt: %w", err)
	}
	return nil
}

type vmpIDAOutputCommit struct {
	SchemaVersion  int                    `json:"schemaVersion"`
	Kind           string                 `json:"kind"`
	AdapterID      string                 `json:"adapterId"`
	GateEventID    string                 `json:"gateEventId"`
	DispatchID     string                 `json:"dispatchId"`
	DispatchPath   string                 `json:"dispatchPath"`
	DispatchSHA256 string                 `json:"dispatchSha256"`
	RequestPath    string                 `json:"requestPath"`
	RequestSHA256  string                 `json:"requestSha256"`
	Packet         vmpIDAOutputCommitFile `json:"packet"`
	Report         vmpIDAOutputCommitFile `json:"report"`
	PacketBytes    []byte                 `json:"packetBytes"`
	ReportBytes    []byte                 `json:"reportBytes"`
}

func buildVMPIDAOutputCommit(
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA,
	requestSHA,
	reportPath,
	packetPath string,
	reportData,
	packetData []byte,
) (vmpIDAOutputCommit, []byte, string, error) {
	commit := vmpIDAOutputCommit{
		SchemaVersion:  1,
		Kind:           vmpIDAOutputCommitKind,
		AdapterID:      VMPIDAIndexAdapterID,
		GateEventID:    dispatch.Gate.GateEventID,
		DispatchID:     dispatch.DispatchID,
		DispatchPath:   dispatchPath,
		DispatchSHA256: strings.ToLower(dispatchSHA),
		RequestPath:    dispatch.Gate.Target,
		RequestSHA256:  strings.ToLower(requestSHA),
		Packet: vmpIDAOutputCommitFile{
			Path:   packetPath,
			SHA256: sha256Hex(packetData),
			Bytes:  int64(len(packetData)),
		},
		Report: vmpIDAOutputCommitFile{
			Path:   reportPath,
			SHA256: sha256Hex(reportData),
			Bytes:  int64(len(reportData)),
		},
		PacketBytes: append([]byte{}, packetData...),
		ReportBytes: append([]byte{}, reportData...),
	}
	if err := validateVMPIDAOutputCommit(
		commit,
		dispatch,
		dispatchPath,
		dispatchSHA,
		requestSHA,
		reportPath,
		packetPath,
	); err != nil {
		return vmpIDAOutputCommit{}, nil, "", err
	}
	data, err := canonicalJSON(commit)
	if err != nil {
		return vmpIDAOutputCommit{}, nil, "", err
	}
	commitPath := filepath.ToSlash(filepath.Join(
		filepath.Dir(reportPath),
		vmpIDAOutputCommitFileName,
	))
	return commit, data, commitPath, nil
}

func validateVMPIDAOutputCommit(
	commit vmpIDAOutputCommit,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA,
	requestSHA,
	reportPath,
	packetPath string,
) error {
	if commit.SchemaVersion != 1 ||
		commit.Kind != vmpIDAOutputCommitKind ||
		commit.AdapterID != VMPIDAIndexAdapterID ||
		commit.GateEventID != dispatch.Gate.GateEventID ||
		commit.DispatchID != dispatch.DispatchID ||
		commit.DispatchPath != dispatchPath ||
		!strings.EqualFold(commit.DispatchSHA256, dispatchSHA) ||
		commit.RequestPath != dispatch.Gate.Target ||
		!strings.EqualFold(commit.RequestSHA256, requestSHA) ||
		commit.Packet.Path != packetPath ||
		commit.Report.Path != reportPath ||
		commit.Packet.Bytes != int64(len(commit.PacketBytes)) ||
		commit.Report.Bytes != int64(len(commit.ReportBytes)) ||
		commit.Packet.Bytes < 1 ||
		commit.Packet.Bytes > VMPIDAIndexMaxPacketBytes ||
		commit.Report.Bytes < 1 ||
		commit.Report.Bytes > 1<<20 ||
		!strings.EqualFold(commit.Packet.SHA256, sha256Hex(commit.PacketBytes)) ||
		!strings.EqualFold(commit.Report.SHA256, sha256Hex(commit.ReportBytes)) {
		return fmt.Errorf("VMP IDA output commit does not match the exact dispatch, request, packet, and report")
	}
	return validateVMPIDAOutputPair(
		commit.PacketBytes,
		commit.ReportBytes,
		dispatch,
		dispatchPath,
		dispatchSHA,
		packetPath,
	)
}

func readVMPIDAOutputCommit(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA,
	requestSHA,
	reportPath,
	packetPath string,
) (*vmpIDAOutputCommit, error) {
	commitPath := filepath.ToSlash(filepath.Join(
		filepath.Dir(reportPath),
		vmpIDAOutputCommitFileName,
	))
	data, err := readVMPIDAFile(
		caseRoot,
		commitPath,
		"VMP IDA output commit",
		VMPIDAIndexMaxPacketBytes+(1<<20)+(64<<10),
	)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var commit vmpIDAOutputCommit
	if err := decodeVMPIDAStrictJSON(data, &commit); err != nil {
		return nil, fmt.Errorf("decode VMP IDA output commit: %w", err)
	}
	canonical, err := canonicalJSON(commit)
	if err != nil || !bytes.Equal(data, canonical) {
		return nil, fmt.Errorf("VMP IDA output commit is not canonical: %w", err)
	}
	if err := validateVMPIDAOutputCommit(
		commit,
		dispatch,
		dispatchPath,
		dispatchSHA,
		requestSHA,
		reportPath,
		packetPath,
	); err != nil {
		return nil, err
	}
	return &commit, nil
}

func publishVMPIDASuccessSeal(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchSHA,
	requestSHA,
	launchSHA,
	reportPath string,
	commitData []byte,
) error {
	seal := vmpIDASuccessSeal{
		SchemaVersion:      1,
		Kind:               vmpIDASuccessSealKind,
		AdapterID:          VMPIDAIndexAdapterID,
		GateEventID:        dispatch.Gate.GateEventID,
		DispatchID:         dispatch.DispatchID,
		DispatchSHA256:     strings.ToLower(dispatchSHA),
		RequestPath:        dispatch.Gate.Target,
		RequestSHA256:      strings.ToLower(requestSHA),
		ChildLaunchSHA256:  strings.ToLower(launchSHA),
		OutputCommitSHA256: sha256Hex(commitData),
	}
	data, err := canonicalJSON(seal)
	if err != nil {
		return err
	}
	_, err = rekitfs.WriteExclusiveRegularFileAnchored(
		caseRoot,
		vmpIDASuccessSealPath(reportPath),
		"VMP IDA success seal",
		data,
	)
	return err
}

func readVMPIDASuccessSeal(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchSHA,
	requestSHA,
	reportPath string,
	commitData []byte,
) (vmpIDASuccessSeal, error) {
	data, err := readVMPIDAFile(
		caseRoot,
		vmpIDASuccessSealPath(reportPath),
		"VMP IDA success seal",
		64<<10,
	)
	if err != nil {
		return vmpIDASuccessSeal{}, err
	}
	var seal vmpIDASuccessSeal
	if err := decodeVMPIDAStrictJSON(data, &seal); err != nil {
		return seal, err
	}
	canonical, err := canonicalJSON(seal)
	if err != nil || !bytes.Equal(data, canonical) ||
		seal.SchemaVersion != 1 || seal.Kind != vmpIDASuccessSealKind ||
		seal.AdapterID != VMPIDAIndexAdapterID ||
		seal.GateEventID != dispatch.Gate.GateEventID ||
		seal.DispatchID != dispatch.DispatchID ||
		!strings.EqualFold(seal.DispatchSHA256, dispatchSHA) ||
		seal.RequestPath != dispatch.Gate.Target ||
		!strings.EqualFold(seal.RequestSHA256, requestSHA) ||
		!validSHA256(seal.ChildLaunchSHA256) ||
		!strings.EqualFold(seal.OutputCommitSHA256, sha256Hex(commitData)) {
		return seal, fmt.Errorf("VMP IDA success seal is not exact or canonical: %w", err)
	}
	return seal, nil
}

func publishVMPIDACommittedOutputsFromBytes(
	caseRoot,
	packetPath,
	reportPath string,
	packetData,
	reportData []byte,
) error {
	if len(packetData) == 0 || len(reportData) == 0 {
		return fmt.Errorf("VMP IDA committed output bytes are missing")
	}
	if _, err := rekitfs.WriteExclusiveRegularFileAnchored(
		caseRoot,
		packetPath,
		"VMP IDA committed packet",
		packetData,
	); err != nil {
		return err
	}
	_, err := rekitfs.WriteExclusiveRegularFileAnchored(
		caseRoot,
		reportPath,
		"VMP IDA committed report",
		reportData,
	)
	return err
}

func removeExactVMPIDAPublicOutputs(
	caseRoot,
	packetPath,
	reportPath string,
	expectedPacket,
	expectedReport []byte,
) error {
	root, err := os.OpenRoot(caseRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, output := range []struct {
		path     string
		label    string
		expected []byte
		limit    int64
	}{
		{path: packetPath, label: "VMP IDA committed packet", expected: expectedPacket, limit: VMPIDAIndexMaxPacketBytes},
		{path: reportPath, label: "VMP IDA committed report", expected: expectedReport, limit: 1 << 20},
	} {
		file, openErr := root.Open(filepath.FromSlash(output.path))
		if errors.Is(openErr, os.ErrNotExist) {
			continue
		}
		if openErr != nil {
			return openErr
		}
		info, statErr := file.Stat()
		data, readErr := io.ReadAll(io.LimitReader(file, output.limit+1))
		owned, identityErr := captureOwnedOutput(file)
		closeErr := file.Close()
		if statErr != nil || readErr != nil || identityErr != nil || closeErr != nil ||
			!info.Mode().IsRegular() || int64(len(data)) != info.Size() ||
			int64(len(data)) > output.limit || !bytes.Equal(data, output.expected) {
			return fmt.Errorf(
				"refuse cleanup because %s differs from its exact output commit: %w",
				output.label,
				errors.Join(statErr, readErr, identityErr, closeErr),
			)
		}
		if err := removeOwnedOutput(root, output.path, owned, nil); err != nil {
			return err
		}
	}
	return nil
}

func publishVMPIDACommittedOutputs(
	caseRoot string,
	commit vmpIDAOutputCommit,
	hooks *hostTestHooks,
) error {
	if _, err := rekitfs.WriteExclusiveRegularFileAnchored(
		caseRoot,
		commit.Packet.Path,
		"VMP IDA committed packet",
		commit.PacketBytes,
	); err != nil {
		return err
	}
	if hooks != nil && hooks.beforeReportWrite != nil {
		if err := hooks.beforeReportWrite(); err != nil {
			return err
		}
	}
	return publishVMPIDACommittedOutputsFromBytes(
		caseRoot,
		commit.Packet.Path,
		commit.Report.Path,
		commit.PacketBytes,
		commit.ReportBytes,
	)
}

func validateVMPIDAOutputPair(
	packetData,
	reportData []byte,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA,
	packetPath string,
) error {
	var packet VMPIDAIndexPacket
	if err := decodeVMPIDAStrictJSON(packetData, &packet); err != nil {
		return err
	}
	canonicalPacket, err := canonicalJSON(packet)
	if err != nil || !bytes.Equal(packetData, canonicalPacket) ||
		packet.AdapterID != VMPIDAIndexAdapterID ||
		packet.RequestPath != dispatch.Gate.Target {
		return fmt.Errorf(
			"existing VMP IDA packet is not the exact canonical dispatch target: %w",
			err,
		)
	}
	var report gate.AdapterReport
	if err := decodeVMPIDAStrictJSON(reportData, &report); err != nil {
		return err
	}
	canonicalReport, err := canonicalJSON(report)
	if err != nil || !bytes.Equal(reportData, canonicalReport) ||
		report.SchemaVersion != 1 ||
		report.Kind != "adapter-execution-report" ||
		report.AdapterID != VMPIDAIndexAdapterID ||
		report.Action != "inspect" ||
		report.Status != "succeeded" ||
		report.GateEventID != dispatch.Gate.GateEventID ||
		report.Dispatch == nil ||
		report.Dispatch.DispatchID != dispatch.DispatchID ||
		report.Dispatch.Path != dispatchPath ||
		!strings.EqualFold(report.Dispatch.SHA256, dispatchSHA) ||
		!reflect.DeepEqual(report.OutputRefs, []string{packetPath}) ||
		!reflect.DeepEqual(report.EvidenceRefs, []string{packetPath}) {
		return fmt.Errorf(
			"existing VMP IDA report is not the exact canonical dispatch result: %w",
			err,
		)
	}
	return nil
}

func readRecoverableVMPOutputs(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA,
	requestSHA,
	reportPath,
	packetPath string,
) (*recoverableVMPOutputs, error) {
	commit, err := readVMPIDAOutputCommit(
		caseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		requestSHA,
		reportPath,
		packetPath,
	)
	if err != nil {
		return nil, err
	}
	packetData, packetErr := readVMPIDAFile(
		caseRoot,
		packetPath,
		"VMP IDA recoverable packet",
		VMPIDAIndexMaxPacketBytes,
	)
	reportData, reportErr := readVMPIDAFile(
		caseRoot,
		reportPath,
		"VMP IDA recoverable report",
		1<<20,
	)
	packetMissing := errors.Is(packetErr, os.ErrNotExist)
	reportMissing := errors.Is(reportErr, os.ErrNotExist)
	if commit == nil {
		if packetMissing && reportMissing {
			return nil, nil
		}
		if packetErr != nil && !packetMissing {
			return nil, packetErr
		}
		if reportErr != nil && !reportMissing {
			return nil, reportErr
		}
		return nil, fmt.Errorf(
			"VMP IDA output publication is partial and has no exact committed stage",
		)
	}
	if packetErr != nil && !packetMissing {
		return nil, packetErr
	}
	if reportErr != nil && !reportMissing {
		return nil, reportErr
	}
	if !packetMissing && !bytes.Equal(packetData, commit.PacketBytes) {
		return nil, fmt.Errorf("existing VMP IDA packet differs from the exact output commit")
	}
	if !reportMissing && !bytes.Equal(reportData, commit.ReportBytes) {
		return nil, fmt.Errorf("existing VMP IDA report differs from the exact output commit")
	}
	packetData = commit.PacketBytes
	reportData = commit.ReportBytes
	if err := validateVMPIDAOutputPair(
		packetData,
		reportData,
		dispatch,
		dispatchPath,
		dispatchSHA,
		packetPath,
	); err != nil {
		return nil, err
	}
	commitData, err := canonicalJSON(*commit)
	if err != nil {
		return nil, err
	}
	launchData, launchErr := readVMPIDAFile(
		caseRoot,
		vmpIDAChildLaunchPath(reportPath),
		"VMP IDA recoverable child launch proof",
		64<<10,
	)
	if launchErr != nil {
		return nil, fmt.Errorf("VMP IDA committed outputs do not bind a child launch proof: %w", launchErr)
	}
	launchSHA := sha256Hex(launchData)
	if err := validateVMPIDAChildLaunchAttempt(
		caseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		requestSHA,
		reportPath,
		launchSHA,
	); err != nil {
		return nil, err
	}
	recoverable := &recoverableVMPOutputs{
		reportData: reportData,
		packetData: packetData,
		commitData: commitData,
		launchSHA:  launchSHA,
	}
	seal, err := readVMPIDASuccessSeal(
		caseRoot,
		dispatch,
		dispatchSHA,
		requestSHA,
		reportPath,
		commitData,
	)
	if errors.Is(err, os.ErrNotExist) {
		return recoverable, nil
	}
	if err != nil {
		return nil, fmt.Errorf("VMP IDA committed outputs have an invalid success seal: %w", err)
	}
	if !strings.EqualFold(seal.ChildLaunchSHA256, launchSHA) {
		return nil, fmt.Errorf("VMP IDA success seal does not bind the exact child launch proof")
	}
	recoverable.sealed = true
	return recoverable, nil
}

func withinAuthorizedOutput(rel string, roots []string) bool {
	rel = cleanCaseRelative(rel)
	for _, root := range roots {
		root = cleanCaseRelative(root)
		if root != "" && (rel == root || strings.HasPrefix(rel, root+"/")) {
			return true
		}
	}
	return false
}

func readTerminalVMPReport(caseRoot string, dispatch adapterexecution.DispatchReceipt, dispatchPath, dispatchSHA, packetPath string) (gate.AdapterReport, []byte, []byte, bool, error) {
	reportData, err := readVMPIDAFile(caseRoot, dispatch.ReportPath, "VMP IDA terminal report", 1<<20)
	if errors.Is(err, os.ErrNotExist) {
		return gate.AdapterReport{}, nil, nil, false, nil
	}
	if err != nil {
		return gate.AdapterReport{}, nil, nil, false, err
	}
	var report gate.AdapterReport
	if err := decodeVMPIDAStrictJSON(reportData, &report); err != nil {
		return report, nil, nil, false, err
	}
	canonicalReport, err := canonicalJSON(report)
	if err != nil || !bytes.Equal(reportData, canonicalReport) ||
		report.SchemaVersion != 1 || report.Kind != "adapter-execution-report" ||
		report.AdapterID != VMPIDAIndexAdapterID || report.Action != "inspect" ||
		report.GateEventID != dispatch.Gate.GateEventID || report.Dispatch == nil ||
		report.Dispatch.DispatchID != dispatch.DispatchID || report.Dispatch.Path != dispatchPath ||
		!strings.EqualFold(report.Dispatch.SHA256, dispatchSHA) {
		return report, nil, nil, false, fmt.Errorf("existing VMP IDA terminal report does not match the exact dispatch")
	}
	if report.Status == "failed" || report.Status == "aborted" {
		if len(report.OutputRefs) != 0 || len(report.EvidenceRefs) != 0 {
			return report, nil, nil, false, fmt.Errorf("failed VMP IDA terminal report must not claim packet evidence")
		}
		_, launchSHA, bindingErr := terminalVMPFailureBinding(report)
		if bindingErr != nil {
			return report, nil, nil, false, bindingErr
		}
		requestSHA := strings.TrimSuffix(
			filepath.Base(dispatch.Gate.Target),
			filepath.Ext(dispatch.Gate.Target),
		)
		if launchErr := validateVMPIDAChildLaunchAttempt(
			caseRoot,
			dispatch,
			dispatchPath,
			dispatchSHA,
			requestSHA,
			dispatch.ReportPath,
			launchSHA,
		); launchErr != nil {
			return report, nil, nil, false, fmt.Errorf("failed VMP IDA terminal report does not bind a valid child launch proof: %w", launchErr)
		}
		if _, exitErr := terminalVMPExecutionExitStatus(report); exitErr != nil {
			return report, nil, nil, false, exitErr
		}
		if _, packetErr := readVMPIDAFile(caseRoot, packetPath, "VMP IDA failed terminal packet", VMPIDAIndexMaxPacketBytes); !errors.Is(packetErr, os.ErrNotExist) {
			if packetErr == nil {
				packetErr = fmt.Errorf("unexpected packet exists")
			}
			return report, nil, nil, false, fmt.Errorf("failed VMP IDA terminal report must not have a packet: %w", packetErr)
		}
		return report, reportData, nil, true, nil
	}
	if report.Status != "succeeded" ||
		!reflect.DeepEqual(report.OutputRefs, []string{packetPath}) ||
		!reflect.DeepEqual(report.EvidenceRefs, []string{packetPath}) {
		return report, nil, nil, false, fmt.Errorf("existing VMP IDA terminal report has unsupported status or packet bindings")
	}
	requestSHA := strings.TrimSuffix(
		filepath.Base(dispatch.Gate.Target),
		filepath.Ext(dispatch.Gate.Target),
	)
	commit, err := readVMPIDAOutputCommit(
		caseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		requestSHA,
		dispatch.ReportPath,
		packetPath,
	)
	if err != nil || commit == nil {
		return report, nil, nil, false, fmt.Errorf("succeeded VMP IDA terminal report requires the exact output commit: %w", err)
	}
	commitData, err := canonicalJSON(*commit)
	if err != nil {
		return report, nil, nil, false, err
	}
	seal, err := readVMPIDASuccessSeal(
		caseRoot,
		dispatch,
		dispatchSHA,
		requestSHA,
		dispatch.ReportPath,
		commitData,
	)
	if errors.Is(err, os.ErrNotExist) {
		return report, nil, nil, false, nil
	}
	if err != nil {
		return report, nil, nil, false, fmt.Errorf("succeeded VMP IDA terminal report has an invalid success seal: %w", err)
	}
	if err := validateVMPIDAChildLaunchAttempt(
		caseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		requestSHA,
		dispatch.ReportPath,
		seal.ChildLaunchSHA256,
	); err != nil {
		return report, nil, nil, false, fmt.Errorf("succeeded VMP IDA terminal report does not bind a valid child launch proof: %w", err)
	}
	packetData, err := readVMPIDAFile(caseRoot, packetPath, "VMP IDA terminal packet", VMPIDAIndexMaxPacketBytes)
	if err != nil {
		return report, nil, nil, false, err
	}
	var packet VMPIDAIndexPacket
	if err := decodeVMPIDAStrictJSON(packetData, &packet); err != nil ||
		packet.AdapterID != VMPIDAIndexAdapterID ||
		packet.RequestPath != dispatch.Gate.Target ||
		!bytes.Equal(packetData, commit.PacketBytes) ||
		!bytes.Equal(reportData, commit.ReportBytes) {
		return report, nil, nil, false, fmt.Errorf("existing VMP IDA terminal output does not match its exact sealed receipt binding: %v", err)
	}
	return report, reportData, packetData, true, nil
}
