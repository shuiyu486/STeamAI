package adapterhost

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/websecurity"
)

const (
	webSecurityFailureExitStatusPrefix = "web-security-exit-status:"
	webSecurityChildLaunchSHA256Marker = ";web-security-child-launch-sha256:"
)

func readTerminalWebSecurityReport(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA,
	artifactPath string,
) (gate.AdapterReport, []byte, []byte, bool, error) {
	reportData, err := readVMPIDAFile(caseRoot, dispatch.ReportPath, "web-security terminal report", websecurity.MaxReplayResultBytes+64<<10)
	if errors.Is(err, os.ErrNotExist) {
		return gate.AdapterReport{}, nil, nil, false, nil
	}
	if err != nil {
		return gate.AdapterReport{}, nil, nil, false, err
	}
	attempt, attemptSHA, present, err := readWebSecurityExecutionAttempt(
		caseRoot, dispatch, dispatchPath, dispatchSHA, dispatch.ReportPath, nil,
	)
	if err != nil || !present {
		if err == nil {
			err = fmt.Errorf("web-security terminal report lacks its exact execution attempt")
		}
		return gate.AdapterReport{}, nil, nil, false, err
	}
	launchData, err := readVMPIDAFile(caseRoot, webSecurityChildLaunchPath(dispatch.ReportPath), "web-security terminal child launch", 64<<10)
	if err != nil {
		return gate.AdapterReport{}, nil, nil, false, err
	}
	launchSHA := sha256Hex(launchData)
	launch, err := readWebSecurityChildLaunch(caseRoot, dispatch.ReportPath, dispatch.Adapter.AdapterID, launchSHA)
	if err != nil || !strings.EqualFold(launch.AttemptSHA256, attemptSHA) || launch.Boundary != webSecurityChildBoundary(dispatch.Adapter.AdapterID) {
		return gate.AdapterReport{}, nil, nil, false, fmt.Errorf("web-security terminal child launch does not bind its exact attempt: %w", err)
	}
	var report gate.AdapterReport
	if err := decodeVMPIDAStrictJSON(reportData, &report); err != nil {
		return gate.AdapterReport{}, nil, nil, false, err
	}
	canonical, err := canonicalJSON(report)
	if err != nil || !bytes.Equal(reportData, canonical) {
		return report, nil, nil, false, fmt.Errorf("web-security terminal report is not canonical: %w", err)
	}
	if len(report.OutputRefs) == 0 && len(report.EvidenceRefs) == 0 {
		exitStatus, proofSHA, err := terminalWebSecurityFailureBinding(report)
		if err != nil || !strings.EqualFold(proofSHA, launchSHA) {
			return report, nil, nil, false, fmt.Errorf("web-security terminal report does not bind its exact child launch: %w", err)
		}
		if err := validateWebSecurityInterruptedStatus(dispatch.Adapter.AdapterID, report.Status, exitStatus); err != nil {
			return report, nil, nil, false, err
		}
		if _, artifactErr := readVMPIDAFile(caseRoot, artifactPath, "web-security interrupted artifact", webSecurityArtifactLimit(dispatch.Adapter.AdapterID)); !errors.Is(artifactErr, os.ErrNotExist) {
			if artifactErr == nil {
				artifactErr = fmt.Errorf("unexpected artifact exists")
			}
			return report, nil, nil, false, artifactErr
		}
		commit, _, commitErr := readWebSecurityOutputCommit(
			caseRoot, dispatch, dispatchPath, dispatchSHA, dispatch.ReportPath, artifactPath, attempt.Input,
		)
		if commitErr != nil || commit != nil {
			if commitErr == nil {
				commitErr = fmt.Errorf("unexpected output commit exists")
			}
			return report, nil, nil, false, commitErr
		}
		return report, reportData, nil, true, nil
	}
	if !reflect.DeepEqual(report.OutputRefs, []string{artifactPath}) || !reflect.DeepEqual(report.EvidenceRefs, []string{artifactPath}) {
		return report, nil, nil, false, fmt.Errorf("web-security terminal report has unexpected artifact references")
	}
	commit, commitData, err := readWebSecurityOutputCommit(
		caseRoot, dispatch, dispatchPath, dispatchSHA, dispatch.ReportPath, artifactPath, attempt.Input,
	)
	if err != nil || commit == nil {
		return report, nil, nil, false, fmt.Errorf("web-security terminal report requires its exact output commit: %w", err)
	}
	if !strings.EqualFold(commit.ChildLaunchSHA256, launchSHA) {
		return report, nil, nil, false, fmt.Errorf("web-security output commit does not bind its exact child launch")
	}
	commitSHA := sha256Hex(commitData)
	if err := readWebSecuritySuccessSeal(caseRoot, dispatch, dispatchSHA, dispatch.ReportPath, attempt.Input, launchSHA, commitSHA); errors.Is(err, os.ErrNotExist) {
		return report, nil, nil, false, nil
	} else if err != nil {
		return report, nil, nil, false, err
	}
	artifactData, err := readVMPIDAFile(caseRoot, artifactPath, "web-security terminal artifact", webSecurityArtifactLimit(dispatch.Adapter.AdapterID))
	if err != nil || !bytes.Equal(artifactData, commit.ArtifactBytes) || !bytes.Equal(reportData, commit.ReportBytes) {
		return report, nil, nil, false, fmt.Errorf("web-security terminal outputs differ from their sealed commit: %w", err)
	}
	if _, err := webSecurityExitStatus(dispatch.Adapter.AdapterID, artifactData, report); err != nil {
		return report, nil, nil, false, err
	}
	return report, reportData, artifactData, true, nil
}

func webSecurityArtifactLimit(adapterID string) int64 {
	if adapterID == websecurity.InventoryAdapterID {
		return websecurity.MaxInventoryBytes
	}
	return websecurity.MaxReplayResultBytes
}

func terminalWebSecurityFailureBinding(report gate.AdapterReport) (string, string, error) {
	marker := strings.TrimSpace(report.Escalation)
	if !strings.HasPrefix(marker, webSecurityFailureExitStatusPrefix) {
		return "", "", fmt.Errorf("web-security interrupted report is missing its exact execution exit status")
	}
	binding := strings.TrimPrefix(marker, webSecurityFailureExitStatusPrefix)
	separator := strings.Index(binding, webSecurityChildLaunchSHA256Marker)
	if separator < 1 {
		return "", "", fmt.Errorf("web-security interrupted report is missing its parent-owned child launch proof")
	}
	exitStatus := binding[:separator]
	launchSHA := binding[separator+len(webSecurityChildLaunchSHA256Marker):]
	if !validWebSecurityInterruptedExitStatus(exitStatus) || !validSHA256(launchSHA) || strings.ToLower(launchSHA) != launchSHA {
		return "", "", fmt.Errorf("web-security interrupted report has an invalid execution status or child launch proof")
	}
	return exitStatus, launchSHA, nil
}

func validWebSecurityInterruptedExitStatus(value string) bool {
	switch value {
	case "parent-interrupted", "delivery-uncertain", "source-drift", "source-drift-after-delivery":
		return true
	default:
		return false
	}
}

func validateWebSecurityInterruptedStatus(adapterID, reportStatus, exitStatus string) error {
	status := strings.ToLower(strings.TrimSpace(reportStatus))
	expected := "aborted"
	if adapterID == websecurity.InventoryAdapterID && exitStatus == "source-drift" {
		expected = "failed"
	}
	if status != expected {
		return fmt.Errorf("web-security interrupted report status does not match execution exit status: %s/%s", status, exitStatus)
	}
	return nil
}

func validateWebSecurityFailureClosureArtifacts(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA,
	reportPath,
	expectedReportSHA string,
) error {
	reportData, err := readVMPIDAFile(caseRoot, reportPath, "web-security terminal failure report", websecurity.MaxReplayResultBytes+64<<10)
	if err != nil || !strings.EqualFold(sha256Hex(reportData), expectedReportSHA) {
		return fmt.Errorf("web-security terminal failure report changed: %w", err)
	}
	var report gate.AdapterReport
	if err := decodeVMPIDAStrictJSON(reportData, &report); err != nil {
		return err
	}
	exitStatus, launchSHA, err := terminalWebSecurityFailureBinding(report)
	if err != nil {
		return err
	}
	if err := validateWebSecurityInterruptedStatus(dispatch.Adapter.AdapterID, report.Status, exitStatus); err != nil {
		return err
	}
	_, attemptSHA, present, err := readWebSecurityExecutionAttempt(
		caseRoot, dispatch, dispatchPath, dispatchSHA, reportPath, nil,
	)
	if err != nil || !present {
		return fmt.Errorf("web-security terminal failure lacks its exact execution attempt: %w", err)
	}
	launch, err := readWebSecurityChildLaunch(caseRoot, reportPath, dispatch.Adapter.AdapterID, launchSHA)
	if err != nil || !strings.EqualFold(launch.AttemptSHA256, attemptSHA) || launch.Boundary != webSecurityChildBoundary(dispatch.Adapter.AdapterID) {
		return fmt.Errorf("web-security terminal failure child launch drifted: %w", err)
	}
	return nil
}

func completeWebSecurityEvidenceLifecycle(
	opt AuthorizedRunOptions,
	result AuthorizedRunResult,
	lane string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA string,
) (AuthorizedRunResult, error) {
	report, reportData, artifactData, terminal, err := readTerminalWebSecurityReport(
		result.CaseRoot, dispatch, dispatchPath, dispatchSHA, authorizedAdapterArtifactPath(dispatch),
	)
	if err != nil || !terminal {
		if err == nil {
			err = fmt.Errorf("web-security evidence lifecycle requires an exact terminal report")
		}
		return result, err
	}
	result.ReportSHA256 = sha256Hex(reportData)
	if len(artifactData) == 0 {
		exitStatus, launchSHA, err := terminalWebSecurityFailureBinding(report)
		if err != nil {
			return result, err
		}
		closure, err := gate.RecordAdapterExecutionTerminalClosure(
			opt.RepoRoot,
			result.CaseRoot,
			result.Pack,
			gate.AdapterExecutionTerminalClosureOptions{
				GateEventID:                   result.GateEventID,
				DispatchPath:                  dispatchPath,
				ExpectedDispatchSHA256:        dispatchSHA,
				ExecutionReportPath:           result.ReportPath,
				ExpectedExecutionReportSHA256: result.ReportSHA256,
				ExecutionExitStatus:           exitStatus,
				RecoveryProofPath:             webSecurityChildLaunchPath(result.ReportPath),
				ExpectedRecoveryProofSHA256:   launchSHA,
				Actor:                         strings.TrimSpace(opt.Actor),
				ExecutionControlBinding:       executioncontrol.CloneBinding(opt.ExecutionControlBinding),
				ValidateExactArtifacts: func() error {
					if opt.testHooks != nil && opt.testHooks.beforeWebSecurityFailureClosureValidation != nil {
						if err := opt.testHooks.beforeWebSecurityFailureClosureValidation(); err != nil {
							return err
						}
					}
					return validateWebSecurityFailureClosureArtifacts(
						result.CaseRoot, dispatch, dispatchPath, dispatchSHA, result.ReportPath, result.ReportSHA256,
					)
				},
			},
		)
		if err != nil {
			return result, err
		}
		result.PacketPath = ""
		result.PacketSHA256 = ""
		result.ReceiptPath = closure.ReceiptPath
		result.ReceiptSHA256 = closure.ReceiptSHA256
		result.ObservationEventID = closure.ObservationEventID
		result.ExecutionStatus = report.Status
		result.ExecutionExitStatus = exitStatus
		return result, nil
	}

	exitStatus, err := webSecurityExitStatus(result.AdapterID, artifactData, report)
	if err != nil {
		return result, err
	}
	if result.ExecutionExitStatus != "" && result.ExecutionExitStatus != exitStatus {
		return result, fmt.Errorf("web-security adapter result exit status differs from its terminal artifact")
	}
	result.ExecutionStatus = report.Status
	result.ExecutionExitStatus = exitStatus
	result.PacketPath = authorizedAdapterArtifactPath(dispatch)
	result.PacketSHA256 = sha256Hex(artifactData)
	validation, err := gate.ValidateAdapterExecutionReport(
		opt.RepoRoot, result.CaseRoot, result.Pack,
		gate.Options{GateEventID: result.GateEventID, ExecutionReportPath: result.ReportPath},
	)
	if err != nil {
		return result, err
	}
	if validation.Report == nil || validation.AdapterContext == nil || validation.AdapterContext.Selected == nil ||
		validation.AdapterContext.Selected.ID != result.AdapterID {
		return result, fmt.Errorf("web-security terminal report validation omitted the exact compiled-in adapter")
	}
	if !validation.ReceiptPresent {
		receiptOpt := gate.Options{
			GateEventID: result.GateEventID, ExecutionReportPath: result.ReportPath,
			AdapterID: result.AdapterID, Executor: dispatch.Owner.CurrentExecutor,
			ExpectedExecutorGeneration: dispatch.Owner.ExecutorGeneration,
			AdapterHarness:             adapterHarness, AdapterSession: dispatch.Owner.AdapterSession,
			ExecutionExitStatus: exitStatus, Actor: strings.TrimSpace(opt.Actor),
			ExecutionControlBinding: executioncontrol.CloneBinding(opt.ExecutionControlBinding),
		}
		preview, previewErr := gate.RecordAdapterExecutionReceipt(opt.RepoRoot, result.CaseRoot, result.Pack, receiptOpt)
		if previewErr != nil {
			return result, previewErr
		}
		receiptOpt.ExpectedAdapterExecutionBindingSHA256 = preview.BindingSHA256
		if _, err := gate.RecordAdapterExecutionReceipt(opt.RepoRoot, result.CaseRoot, result.Pack, receiptOpt); err != nil {
			return result, err
		}
		validation, err = gate.ValidateAdapterExecutionReport(
			opt.RepoRoot, result.CaseRoot, result.Pack,
			gate.Options{GateEventID: result.GateEventID, ExecutionReportPath: result.ReportPath},
		)
		if err != nil {
			return result, err
		}
	}
	if !validation.Valid || !validation.ProvenanceValid || validation.AdapterExecution == nil || !validation.ReceiptPresent {
		return result, fmt.Errorf("validate web-security report and receipt: %s", strings.TrimSpace(validation.Error))
	}
	result.ReceiptPath = validation.AdapterExecutionReceiptPath
	result.ReceiptSHA256 = validation.AdapterExecutionReceiptSHA256
	if err := ValidateWebSecurityReceiptArtifacts(
		result.CaseRoot, dispatch, dispatchPath, dispatchSHA, result, *validation.AdapterExecution,
	); err != nil {
		return result, err
	}
	if opt.testHooks != nil && opt.testHooks.afterWebSecurityReceiptRecorded != nil {
		if err := opt.testHooks.afterWebSecurityReceiptRecorded(); err != nil {
			return result, err
		}
	}
	observationEventID, observationErr := terminalVMPObservationEventID(result.CaseRoot, lane, result.GateEventID)
	if observationErr != nil {
		observation, recordErr := gate.RecordExecution(
			opt.RepoRoot,
			result.CaseRoot,
			result.Pack,
			gate.Options{
				GateEventID: result.GateEventID, Actor: strings.TrimSpace(opt.Actor),
				ExecutionReportPath:                   validation.ReportPath,
				ExpectedExecutionReportSHA256:         validation.RecordExpectedReportSHA256,
				AdapterExecutionReceiptPath:           validation.AdapterExecutionReceiptPath,
				ExpectedAdapterExecutionReceiptSHA256: validation.AdapterExecutionReceiptSHA256,
				Executor:                              dispatch.Owner.CurrentExecutor,
				ExpectedExecutorGeneration:            dispatch.Owner.ExecutorGeneration,
				ExecutionControlBinding:               executioncontrol.CloneBinding(opt.ExecutionControlBinding),
			},
		)
		if recordErr != nil || (!observation.Applied && observation.Reason != "duplicate eventId") {
			return result, fmt.Errorf("record web-security execution observation: %w", recordErr)
		}
		observationEventID = observation.EventID
	}
	result.ObservationEventID = observationEventID
	if opt.testHooks != nil && opt.testHooks.afterWebSecurityObservation != nil {
		if err := opt.testHooks.afterWebSecurityObservation(); err != nil {
			return result, err
		}
	}
	if result.ExecutionStatus != "succeeded" {
		return result, nil
	}
	if opt.DeferSuccessfulTaskBinding {
		return result, nil
	}
	owner, err := laneowner.Read(result.CaseRoot, lane)
	if err != nil || owner.CurrentExecutor != dispatch.Owner.CurrentExecutor || owner.ExecutorGeneration != dispatch.Owner.ExecutorGeneration {
		return result, fmt.Errorf("web-security evidence owner changed before member task binding: %w", err)
	}
	result.TaskBindingPath, result.TaskBindingSHA256, err = bindWebSecurityEvidence(
		result.CaseRoot, lane, dispatch, result, *validation.AdapterExecution,
		executioncontrol.CloneBinding(opt.ExecutionControlBinding),
	)
	return result, err
}

func ValidateWebSecurityReceiptArtifacts(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA string,
	result AuthorizedRunResult,
	receipt adapterexecution.Receipt,
) error {
	if receipt.Dispatch.DispatchID != dispatch.DispatchID || receipt.Dispatch.Path != dispatchPath ||
		!strings.EqualFold(receipt.Dispatch.SHA256, dispatchSHA) || receipt.Gate.GateEventID != result.GateEventID ||
		receipt.Gate.Target != dispatch.Gate.Target || receipt.Adapter.Pack != webSecurityPack ||
		receipt.Adapter.AdapterID != result.AdapterID || receipt.Owner != dispatch.Owner ||
		receipt.Report.Path != result.ReportPath || !strings.EqualFold(receipt.Report.SHA256, result.ReportSHA256) ||
		receipt.Execution.Outcome != result.ExecutionStatus || receipt.Execution.ExitStatus != result.ExecutionExitStatus ||
		len(receipt.Artifacts) != 1 || receipt.Artifacts[0].Path != result.PacketPath ||
		!strings.EqualFold(receipt.Artifacts[0].SHA256, result.PacketSHA256) ||
		!reflect.DeepEqual(receipt.Artifacts[0].Roles, []string{"evidence", "output"}) {
		return fmt.Errorf("web-security receipt does not bind the exact dispatch, owner, report, and artifact")
	}
	report, reportData, artifactData, terminal, err := readTerminalWebSecurityReport(
		caseRoot, dispatch, dispatchPath, dispatchSHA, result.PacketPath,
	)
	if err != nil || !terminal || len(artifactData) == 0 || report.Status != result.ExecutionStatus ||
		!strings.EqualFold(sha256Hex(reportData), result.ReportSHA256) || !strings.EqualFold(sha256Hex(artifactData), result.PacketSHA256) {
		return fmt.Errorf("web-security receipt-bound terminal bytes are invalid: %w", err)
	}
	return nil
}

func bindWebSecurityEvidence(
	caseRoot,
	lane string,
	dispatch adapterexecution.DispatchReceipt,
	result AuthorizedRunResult,
	receipt adapterexecution.Receipt,
	controlBinding *executioncontrol.Binding,
) (string, string, error) {
	artifact := receipt.Artifacts[0]
	kind := "web-security-openapi-inventory-evidence"
	if result.AdapterID == websecurity.ReplayAdapterID {
		kind = "web-security-bounded-replay-evidence"
	}
	binding := memberexecution.TaskBinding{
		Kind: kind,
		Values: map[string]string{
			"gate-event-id":        receipt.Gate.GateEventID,
			"profile-hash":         receipt.Gate.Authorization.ProfileHash,
			"input-path":           receipt.Gate.Target,
			"artifact-path":        artifact.Path,
			"artifact-sha256":      artifact.SHA256,
			"report-path":          receipt.Report.Path,
			"report-sha256":        receipt.Report.SHA256,
			"dispatch-path":        receipt.Dispatch.Path,
			"dispatch-sha256":      receipt.Dispatch.SHA256,
			"receipt-path":         result.ReceiptPath,
			"receipt-sha256":       result.ReceiptSHA256,
			"observation-event-id": result.ObservationEventID,
		},
	}
	return writeAuthorizedTaskBindingForOwner(
		caseRoot,
		lane,
		dispatch.Owner.CurrentExecutor,
		dispatch.Owner.ExecutorGeneration,
		controlBinding,
		binding,
	)
}

func recoverWebSecurityInterruptedAttempt(
	opt AuthorizedRunOptions,
	caseRoot,
	pack string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA string,
) (Result, bool, error) {
	result := Result{
		SchemaVersion:     1,
		Kind:              "rekit-readonly-adapter-host-result",
		CaseRoot:          caseRoot,
		Pack:              pack,
		GateEventID:       dispatch.Gate.GateEventID,
		Lane:              dispatch.Owner.Lane,
		Executor:          dispatch.Owner.CurrentExecutor,
		Generation:        dispatch.Owner.ExecutorGeneration,
		AdapterID:         dispatch.Adapter.AdapterID,
		AdapterHarness:    dispatch.Owner.AdapterHarness,
		AdapterSession:    dispatch.Owner.AdapterSession,
		DispatchPath:      dispatchPath,
		DispatchSHA256:    dispatchSHA,
		InputPath:         cleanCaseRelative(dispatch.Gate.Target),
		ReportPath:        cleanCaseRelative(dispatch.ReportPath),
		ArtifactPath:      webSecurityArtifactPath(dispatch),
		ReadOnlyInput:     true,
		NoNetwork:         authorizedAdapterNoNetwork(dispatch.Adapter.AdapterID),
		NoNetworkBoundary: authorizedAdapterBoundary(dispatch.Adapter.AdapterID),
		NoAuthority:       true,
	}
	if _, err := readVMPIDAFile(caseRoot, webSecurityChildLaunchPath(result.ReportPath), "web-security prior child launch proof", 64<<10); errors.Is(err, os.ErrNotExist) {
		return Result{}, false, nil
	} else if err != nil {
		return result, false, err
	}
	result, err := runWebSecurityExistingDispatch(
		Options{
			RepoRoot: opt.RepoRoot, CaseRoot: caseRoot, Pack: pack,
			GateEventID: dispatch.Gate.GateEventID, ExpectedDispatchSHA256: dispatchSHA,
			ExecutionControlBinding: executioncontrol.CloneBinding(opt.ExecutionControlBinding),
			testHooks:               opt.testHooks,
		},
		result,
		dispatch,
		dispatchPath,
		dispatchSHA,
		time.Now(),
	)
	if err == nil {
		result.ProcessID = 0
	}
	return result, true, err
}
