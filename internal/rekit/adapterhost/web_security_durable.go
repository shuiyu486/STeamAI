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
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterexecution"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/websecurity"
)

const (
	webSecurityExecutionAttemptFileName = ".web-security-execution-attempt.json"
	webSecurityExecutionAttemptKind     = "web-security-execution-attempt"
	webSecurityChildLaunchFileName      = ".web-security-child-launch.json"
	webSecurityChildLaunchKind          = "web-security-child-launch"
	webSecurityOutputCommitFileName     = ".web-security-output-commit.json"
	webSecurityOutputCommitKind         = "web-security-output-commit"
	webSecuritySuccessSealFileName      = ".web-security-success-seal.json"
	webSecuritySuccessSealKind          = "web-security-success-seal"
)

type webSecurityExecutionAttempt struct {
	SchemaVersion    int                     `json:"schemaVersion"`
	Kind             string                  `json:"kind"`
	AdapterID        string                  `json:"adapterId"`
	GateEventID      string                  `json:"gateEventId"`
	DispatchID       string                  `json:"dispatchId"`
	DispatchPath     string                  `json:"dispatchPath"`
	DispatchSHA256   string                  `json:"dispatchSha256"`
	Input            websecurity.FileBinding `json:"input"`
	ExecutableSHA256 string                  `json:"executableSha256"`
	StartedAt        string                  `json:"startedAt"`
	Nonce            string                  `json:"nonce"`
}

type webSecurityChildLaunch struct {
	SchemaVersion  int    `json:"schemaVersion"`
	Kind           string `json:"kind"`
	AdapterID      string `json:"adapterId"`
	AttemptSHA256  string `json:"attemptSha256"`
	ChildProcessID int    `json:"childProcessId"`
	Boundary       string `json:"boundary"`
}

type webSecurityOutputFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type webSecurityOutputCommit struct {
	SchemaVersion     int                     `json:"schemaVersion"`
	Kind              string                  `json:"kind"`
	AdapterID         string                  `json:"adapterId"`
	GateEventID       string                  `json:"gateEventId"`
	DispatchID        string                  `json:"dispatchId"`
	DispatchPath      string                  `json:"dispatchPath"`
	DispatchSHA256    string                  `json:"dispatchSha256"`
	Input             websecurity.FileBinding `json:"input"`
	ChildLaunchSHA256 string                  `json:"childLaunchSha256"`
	Artifact          webSecurityOutputFile   `json:"artifact"`
	Report            webSecurityOutputFile   `json:"report"`
	ArtifactBytes     []byte                  `json:"artifactBytes"`
	ReportBytes       []byte                  `json:"reportBytes"`
}

type webSecuritySuccessSeal struct {
	SchemaVersion      int    `json:"schemaVersion"`
	Kind               string `json:"kind"`
	AdapterID          string `json:"adapterId"`
	GateEventID        string `json:"gateEventId"`
	DispatchID         string `json:"dispatchId"`
	DispatchSHA256     string `json:"dispatchSha256"`
	InputPath          string `json:"inputPath"`
	InputSHA256        string `json:"inputSha256"`
	ChildLaunchSHA256  string `json:"childLaunchSha256"`
	OutputCommitSHA256 string `json:"outputCommitSha256"`
}

func webSecurityExecutionAttemptPath(reportPath string) string {
	return filepath.ToSlash(filepath.Join(filepath.Dir(reportPath), webSecurityExecutionAttemptFileName))
}

func webSecurityChildLaunchPath(reportPath string) string {
	return filepath.ToSlash(filepath.Join(filepath.Dir(reportPath), webSecurityChildLaunchFileName))
}

func webSecurityOutputCommitPath(reportPath string) string {
	return filepath.ToSlash(filepath.Join(filepath.Dir(reportPath), webSecurityOutputCommitFileName))
}

func webSecuritySuccessSealPath(reportPath string) string {
	return filepath.ToSlash(filepath.Join(filepath.Dir(reportPath), webSecuritySuccessSealFileName))
}

func readWebSecurityExecutionAttempt(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA,
	reportPath string,
	expectedInput *websecurity.FileBinding,
) (webSecurityExecutionAttempt, string, bool, error) {
	data, err := readVMPIDAFile(caseRoot, webSecurityExecutionAttemptPath(reportPath), "web-security execution attempt", 64<<10)
	if errors.Is(err, os.ErrNotExist) {
		return webSecurityExecutionAttempt{}, "", false, nil
	}
	if err != nil {
		return webSecurityExecutionAttempt{}, "", false, err
	}
	var attempt webSecurityExecutionAttempt
	if err := decodeVMPIDAStrictJSON(data, &attempt); err != nil {
		return attempt, "", true, err
	}
	canonical, err := canonicalJSON(attempt)
	if err != nil || !bytes.Equal(data, canonical) || attempt.SchemaVersion != 1 || attempt.Kind != webSecurityExecutionAttemptKind ||
		attempt.AdapterID != dispatch.Adapter.AdapterID || attempt.GateEventID != dispatch.Gate.GateEventID || attempt.DispatchID != dispatch.DispatchID ||
		attempt.DispatchPath != dispatchPath || !strings.EqualFold(attempt.DispatchSHA256, dispatchSHA) || attempt.Input.Path != dispatch.Gate.Target ||
		(expectedInput != nil && !reflect.DeepEqual(attempt.Input, *expectedInput)) || !validSHA256(attempt.Input.SHA256) || attempt.Input.Bytes < 1 ||
		!validSHA256(attempt.ExecutableSHA256) || len(attempt.Nonce) != 32 {
		return attempt, "", true, fmt.Errorf("web-security execution attempt is not exact or canonical: %w", err)
	}
	if attempt.AdapterID == websecurity.ReplayAdapterID {
		if bindingErr := websecurity.ValidateReplayRequestBinding(attempt.Input); bindingErr != nil {
			return attempt, "", true, fmt.Errorf("web-security replay execution attempt input is invalid: %w", bindingErr)
		}
	}
	started, startedErr := time.Parse(time.RFC3339Nano, attempt.StartedAt)
	if startedErr != nil || attempt.StartedAt != started.UTC().Format(time.RFC3339Nano) {
		return attempt, "", true, fmt.Errorf("web-security execution attempt time is invalid: %w", startedErr)
	}
	if _, err := hex.DecodeString(attempt.Nonce); err != nil {
		return attempt, "", true, err
	}
	return attempt, sha256Hex(data), true, nil
}

func publishWebSecurityExecutionAttempt(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA,
	executableSHA,
	reportPath string,
	input websecurity.FileBinding,
	started time.Time,
) (webSecurityExecutionAttempt, string, error) {
	if existing, existingSHA, present, err := readWebSecurityExecutionAttempt(caseRoot, dispatch, dispatchPath, dispatchSHA, reportPath, &input); err != nil {
		return webSecurityExecutionAttempt{}, "", err
	} else if present {
		if !strings.EqualFold(existing.ExecutableSHA256, executableSHA) {
			return webSecurityExecutionAttempt{}, "", fmt.Errorf("web-security execution attempt executable identity changed")
		}
		return existing, existingSHA, nil
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return webSecurityExecutionAttempt{}, "", err
	}
	attempt := webSecurityExecutionAttempt{
		SchemaVersion: 1, Kind: webSecurityExecutionAttemptKind, AdapterID: dispatch.Adapter.AdapterID,
		GateEventID: dispatch.Gate.GateEventID, DispatchID: dispatch.DispatchID, DispatchPath: dispatchPath,
		DispatchSHA256: strings.ToLower(dispatchSHA), Input: input, ExecutableSHA256: strings.ToLower(executableSHA),
		StartedAt: started.UTC().Format(time.RFC3339Nano), Nonce: hex.EncodeToString(nonce),
	}
	data, err := canonicalJSON(attempt)
	if err != nil {
		return webSecurityExecutionAttempt{}, "", err
	}
	if _, err := rekitfs.WriteAtomicNoReplaceRegularFileAnchored(caseRoot, webSecurityExecutionAttemptPath(reportPath), "web-security execution attempt", data); err != nil {
		return webSecurityExecutionAttempt{}, "", err
	}
	return attempt, sha256Hex(data), nil
}

func publishWebSecurityChildLaunch(caseRoot, reportPath, adapterID, attemptSHA, boundary string, childPID int) (string, error) {
	launch := webSecurityChildLaunch{SchemaVersion: 1, Kind: webSecurityChildLaunchKind, AdapterID: adapterID, AttemptSHA256: strings.ToLower(attemptSHA), ChildProcessID: childPID, Boundary: boundary}
	if childPID < 1 || !validSHA256(launch.AttemptSHA256) || boundary == "" {
		return "", fmt.Errorf("web-security child launch requires an exact attempt, boundary, and process id")
	}
	data, err := canonicalJSON(launch)
	if err != nil {
		return "", err
	}
	if _, err := rekitfs.WriteAtomicNoReplaceRegularFileAnchored(caseRoot, webSecurityChildLaunchPath(reportPath), "web-security child launch", data); err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}

func readWebSecurityChildLaunch(caseRoot, reportPath, adapterID, expectedSHA string) (webSecurityChildLaunch, error) {
	data, err := readVMPIDAFile(caseRoot, webSecurityChildLaunchPath(reportPath), "web-security child launch", 64<<10)
	if err != nil {
		return webSecurityChildLaunch{}, err
	}
	var launch webSecurityChildLaunch
	if err := decodeVMPIDAStrictJSON(data, &launch); err != nil {
		return launch, err
	}
	canonical, err := canonicalJSON(launch)
	if err != nil || !bytes.Equal(data, canonical) || launch.SchemaVersion != 1 || launch.Kind != webSecurityChildLaunchKind || launch.AdapterID != adapterID || !validSHA256(launch.AttemptSHA256) || launch.ChildProcessID < 1 || launch.Boundary == "" || !strings.EqualFold(sha256Hex(data), expectedSHA) {
		return launch, fmt.Errorf("web-security child launch is not exact or canonical: %w", err)
	}
	return launch, nil
}

func publishWebSecurityOutputCommit(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA string,
	input websecurity.FileBinding,
	launchSHA,
	artifactPath,
	reportPath string,
	artifactData,
	reportData []byte,
) (webSecurityOutputCommit, []byte, error) {
	commit := webSecurityOutputCommit{
		SchemaVersion: 1, Kind: webSecurityOutputCommitKind, AdapterID: dispatch.Adapter.AdapterID,
		GateEventID: dispatch.Gate.GateEventID, DispatchID: dispatch.DispatchID, DispatchPath: dispatchPath,
		DispatchSHA256: strings.ToLower(dispatchSHA), Input: input, ChildLaunchSHA256: strings.ToLower(launchSHA),
		Artifact:      webSecurityOutputFile{Path: artifactPath, SHA256: sha256Hex(artifactData), Bytes: int64(len(artifactData))},
		Report:        webSecurityOutputFile{Path: reportPath, SHA256: sha256Hex(reportData), Bytes: int64(len(reportData))},
		ArtifactBytes: append([]byte{}, artifactData...), ReportBytes: append([]byte{}, reportData...),
	}
	data, err := canonicalJSON(commit)
	if err != nil {
		return webSecurityOutputCommit{}, nil, err
	}
	if _, err := rekitfs.WriteAtomicNoReplaceRegularFileAnchored(caseRoot, webSecurityOutputCommitPath(reportPath), "web-security output commit", data); err != nil {
		return webSecurityOutputCommit{}, nil, err
	}
	return commit, data, nil
}

func readWebSecurityOutputCommit(caseRoot string, dispatch adapterexecution.DispatchReceipt, dispatchPath, dispatchSHA, reportPath, artifactPath string, input websecurity.FileBinding) (*webSecurityOutputCommit, []byte, error) {
	data, err := readVMPIDAFile(caseRoot, webSecurityOutputCommitPath(reportPath), "web-security output commit", 3<<20)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var commit webSecurityOutputCommit
	if err := decodeVMPIDAStrictJSON(data, &commit); err != nil {
		return nil, nil, err
	}
	canonical, err := canonicalJSON(commit)
	if err != nil || !bytes.Equal(data, canonical) || commit.SchemaVersion != 1 || commit.Kind != webSecurityOutputCommitKind || commit.AdapterID != dispatch.Adapter.AdapterID || commit.GateEventID != dispatch.Gate.GateEventID || commit.DispatchID != dispatch.DispatchID || commit.DispatchPath != dispatchPath || !strings.EqualFold(commit.DispatchSHA256, dispatchSHA) || !reflect.DeepEqual(commit.Input, input) || !validSHA256(commit.ChildLaunchSHA256) || commit.Artifact.Path != artifactPath || commit.Report.Path != reportPath || commit.Artifact.SHA256 != sha256Hex(commit.ArtifactBytes) || commit.Artifact.Bytes != int64(len(commit.ArtifactBytes)) || commit.Report.SHA256 != sha256Hex(commit.ReportBytes) || commit.Report.Bytes != int64(len(commit.ReportBytes)) {
		return nil, nil, fmt.Errorf("web-security output commit is not exact or canonical: %w", err)
	}
	if err := validateWebSecurityCommittedBytes(dispatch, dispatchPath, dispatchSHA, artifactPath, reportPath, commit.ArtifactBytes, commit.ReportBytes); err != nil {
		return nil, nil, err
	}
	return &commit, data, nil
}

func validateWebSecurityCommittedBytes(dispatch adapterexecution.DispatchReceipt, dispatchPath, dispatchSHA, artifactPath, reportPath string, artifactData, reportData []byte) error {
	if dispatch.Adapter.AdapterID == websecurity.InventoryAdapterID {
		inventory, err := websecurity.DecodeInventory(artifactData)
		if err != nil || inventory.Source.Path != dispatch.Gate.Target {
			return fmt.Errorf("committed OpenAPI inventory is invalid: %w", err)
		}
	} else {
		result, err := websecurity.DecodeReplayResult(artifactData)
		if err != nil || result.Request.Path != dispatch.Gate.Target {
			return fmt.Errorf("committed bounded replay result is invalid: %w", err)
		}
	}
	var report gate.AdapterReport
	decoder := json.NewDecoder(bytes.NewReader(reportData))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("committed web-security report must contain one JSON object")
	}
	canonical, err := canonicalJSON(report)
	if err != nil || !bytes.Equal(reportData, canonical) || report.SchemaVersion != 1 || report.Kind != "adapter-execution-report" || report.AdapterID != dispatch.Adapter.AdapterID || report.Action != dispatch.Gate.Action || report.GateEventID != dispatch.Gate.GateEventID || report.Dispatch == nil || report.Dispatch.DispatchID != dispatch.DispatchID || report.Dispatch.Path != dispatchPath || !strings.EqualFold(report.Dispatch.SHA256, dispatchSHA) || !reflect.DeepEqual(report.OutputRefs, []string{artifactPath}) || !reflect.DeepEqual(report.EvidenceRefs, []string{artifactPath}) || reportPath != dispatch.ReportPath {
		return fmt.Errorf("committed web-security report is invalid or dispatch-drifted: %w", err)
	}
	return nil
}

func publishWebSecurityOutputs(caseRoot string, commit webSecurityOutputCommit) error {
	if _, err := rekitfs.WriteAtomicNoReplaceRegularFileAnchored(caseRoot, commit.Artifact.Path, "web-security artifact", commit.ArtifactBytes); err != nil {
		return err
	}
	if _, err := rekitfs.WriteAtomicNoReplaceRegularFileAnchored(caseRoot, commit.Report.Path, "web-security report", commit.ReportBytes); err != nil {
		return err
	}
	return nil
}

func publishWebSecuritySuccessSeal(caseRoot string, dispatch adapterexecution.DispatchReceipt, dispatchSHA, reportPath string, input websecurity.FileBinding, launchSHA, commitSHA string) error {
	seal := webSecuritySuccessSeal{SchemaVersion: 1, Kind: webSecuritySuccessSealKind, AdapterID: dispatch.Adapter.AdapterID, GateEventID: dispatch.Gate.GateEventID, DispatchID: dispatch.DispatchID, DispatchSHA256: strings.ToLower(dispatchSHA), InputPath: input.Path, InputSHA256: input.SHA256, ChildLaunchSHA256: strings.ToLower(launchSHA), OutputCommitSHA256: strings.ToLower(commitSHA)}
	data, err := canonicalJSON(seal)
	if err != nil {
		return err
	}
	_, err = rekitfs.WriteAtomicNoReplaceRegularFileAnchored(caseRoot, webSecuritySuccessSealPath(reportPath), "web-security success seal", data)
	return err
}

func readWebSecuritySuccessSeal(caseRoot string, dispatch adapterexecution.DispatchReceipt, dispatchSHA, reportPath string, input websecurity.FileBinding, launchSHA, commitSHA string) error {
	data, err := readVMPIDAFile(caseRoot, webSecuritySuccessSealPath(reportPath), "web-security success seal", 64<<10)
	if err != nil {
		return err
	}
	var seal webSecuritySuccessSeal
	if err := decodeVMPIDAStrictJSON(data, &seal); err != nil {
		return err
	}
	canonical, err := canonicalJSON(seal)
	if err != nil || !bytes.Equal(data, canonical) || seal.SchemaVersion != 1 || seal.Kind != webSecuritySuccessSealKind || seal.AdapterID != dispatch.Adapter.AdapterID || seal.GateEventID != dispatch.Gate.GateEventID || seal.DispatchID != dispatch.DispatchID || !strings.EqualFold(seal.DispatchSHA256, dispatchSHA) || seal.InputPath != input.Path || !strings.EqualFold(seal.InputSHA256, input.SHA256) || !strings.EqualFold(seal.ChildLaunchSHA256, launchSHA) || !strings.EqualFold(seal.OutputCommitSHA256, commitSHA) {
		return fmt.Errorf("web-security success seal is invalid: %w", err)
	}
	return nil
}

func webSecurityInputLimit(adapterID string) int64 {
	if adapterID == websecurity.InventoryAdapterID {
		return websecurity.MaxOpenAPIBytes
	}
	return websecurity.MaxReplayRequestBytes
}

func webSecurityChildBoundary(adapterID string) string {
	if adapterID == websecurity.InventoryAdapterID {
		return fixedChildNoNetworkCodepath
	}
	return boundedReplayNetworkBoundary
}

func webSecurityInterruptedExitStatus(adapterID string) string {
	if adapterID == websecurity.ReplayAdapterID {
		return "delivery-uncertain"
	}
	return "parent-interrupted"
}

func webSecuritySourceDriftExitStatus(adapterID string) string {
	if adapterID == websecurity.ReplayAdapterID {
		return "source-drift-after-delivery"
	}
	return "source-drift"
}

func recoverWebSecurityLaunchedAttempt(
	result Result,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA string,
	root *os.Root,
) (Result, bool, error) {
	attempt, attemptSHA, attemptPresent, err := readWebSecurityExecutionAttempt(
		result.CaseRoot, dispatch, dispatchPath, dispatchSHA, result.ReportPath, nil,
	)
	if err != nil {
		return result, true, err
	}
	launchData, launchErr := readVMPIDAFile(result.CaseRoot, webSecurityChildLaunchPath(result.ReportPath), "web-security prior child launch", 64<<10)
	if errors.Is(launchErr, os.ErrNotExist) {
		return result, false, nil
	}
	if launchErr != nil {
		return result, true, launchErr
	}
	if !attemptPresent {
		return result, true, fmt.Errorf("web-security child launch lacks its exact execution attempt")
	}
	launchSHA := sha256Hex(launchData)
	launch, err := readWebSecurityChildLaunch(result.CaseRoot, result.ReportPath, dispatch.Adapter.AdapterID, launchSHA)
	if err != nil || !strings.EqualFold(launch.AttemptSHA256, attemptSHA) || launch.Boundary != webSecurityChildBoundary(dispatch.Adapter.AdapterID) {
		return result, true, fmt.Errorf("web-security child launch does not bind the exact attempt: %w", err)
	}
	result.ProcessID = launch.ChildProcessID
	result.InputSHA256 = attempt.Input.SHA256
	result.ExecutableSHA256 = attempt.ExecutableSHA256
	commit, commitData, err := readWebSecurityOutputCommit(
		result.CaseRoot, dispatch, dispatchPath, dispatchSHA, result.ReportPath,
		result.ArtifactPath, attempt.Input,
	)
	if err != nil {
		return result, true, err
	}
	if commit == nil {
		for _, rel := range []string{result.ArtifactPath, result.ReportPath} {
			if _, statErr := root.Lstat(filepath.FromSlash(rel)); statErr == nil {
				return result, true, fmt.Errorf("web-security launched attempt has an uncommitted public output: %s", rel)
			} else if !errors.Is(statErr, os.ErrNotExist) {
				return result, true, statErr
			}
		}
		started, parseErr := time.Parse(time.RFC3339Nano, attempt.StartedAt)
		if parseErr != nil {
			return result, true, parseErr
		}
		closed, closeErr := publishWebSecurityInterruptedReport(
			result, dispatch, dispatchPath, dispatchSHA, started, launchSHA,
			webSecurityInterruptedExitStatus(dispatch.Adapter.AdapterID),
		)
		return closed, true, closeErr
	}
	if !strings.EqualFold(commit.ChildLaunchSHA256, launchSHA) {
		return result, true, fmt.Errorf("web-security output commit does not bind the exact child launch")
	}
	if err := publishWebSecurityOutputs(result.CaseRoot, *commit); err != nil {
		return result, true, err
	}
	commitSHA := sha256Hex(commitData)
	if err := readWebSecuritySuccessSeal(result.CaseRoot, dispatch, dispatchSHA, result.ReportPath, attempt.Input, launchSHA, commitSHA); errors.Is(err, os.ErrNotExist) {
		if err := publishWebSecuritySuccessSeal(result.CaseRoot, dispatch, dispatchSHA, result.ReportPath, attempt.Input, launchSHA, commitSHA); err != nil {
			return result, true, err
		}
	} else if err != nil {
		return result, true, err
	}
	if err := readWebSecuritySuccessSeal(result.CaseRoot, dispatch, dispatchSHA, result.ReportPath, attempt.Input, launchSHA, commitSHA); err != nil {
		return result, true, err
	}
	recovered, err := webSecurityResultFromCommit(result, *commit)
	return recovered, true, err
}

func publishWebSecurityInterruptedReport(
	result Result,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA string,
	started time.Time,
	launchSHA,
	exitStatus string,
) (Result, error) {
	if !validWebSecurityInterruptedExitStatus(exitStatus) {
		return result, fmt.Errorf("invalid web-security interrupted exit status: %s", exitStatus)
	}
	boundaryHits := []string{}
	if exitStatus == "delivery-uncertain" {
		boundaryHits = []string{"delivery-uncertain"}
	}
	status := "aborted"
	if dispatch.Adapter.AdapterID == websecurity.InventoryAdapterID && exitStatus == "source-drift" {
		status = "failed"
	}
	report := gate.AdapterReport{
		SchemaVersion: 1,
		Kind:          "adapter-execution-report",
		AdapterID:     dispatch.Adapter.AdapterID,
		Action:        dispatch.Gate.Action,
		Status:        status,
		GateEventID:   dispatch.Gate.GateEventID,
		Dispatch: &adapterexecution.ReportDispatchBinding{
			DispatchID: dispatch.DispatchID,
			Path:       dispatchPath,
			SHA256:     dispatchSHA,
		},
		ActualBudget: dispatch.Gate.AuthorizedBudget,
		OutputRefs:   []string{},
		EvidenceRefs: []string{},
		BoundaryHits: boundaryHits,
		Escalation:   webSecurityFailureExitStatusPrefix + exitStatus + webSecurityChildLaunchSHA256Marker + strings.ToLower(launchSHA),
		Summary:      "The fixed web-security child was durably launched, but no exact output commit exists; the attempt is terminal and must not be rerun.",
	}
	report.ActualBudget.DiskMB = 0
	report.ActualBudget.Requests = 1
	data, err := canonicalJSON(report)
	if err != nil {
		return result, err
	}
	if _, err := rekitfs.WriteAtomicNoReplaceRegularFileAnchored(result.CaseRoot, result.ReportPath, "web-security interrupted report", data); err != nil {
		return result, err
	}
	result.ArtifactPath = ""
	result.ArtifactSHA256 = ""
	result.ReportSHA256 = sha256Hex(data)
	result.ExecutionStatus = status
	result.ExecutionExitStatus = exitStatus
	result.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
	_ = started
	return result, nil
}

func webSecurityResultFromCommit(result Result, commit webSecurityOutputCommit) (Result, error) {
	var report gate.AdapterReport
	decoder := json.NewDecoder(bytes.NewReader(commit.ReportBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return result, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return result, fmt.Errorf("web-security committed report must contain one object")
	}
	result.ArtifactPath = commit.Artifact.Path
	result.ArtifactSHA256 = commit.Artifact.SHA256
	result.ReportPath = commit.Report.Path
	result.ReportSHA256 = commit.Report.SHA256
	result.ExecutionStatus = report.Status
	if dispatchResult, err := webSecurityExitStatus(commit.AdapterID, commit.ArtifactBytes, report); err != nil {
		return result, err
	} else {
		result.ExecutionExitStatus = dispatchResult
	}
	result.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return result, nil
}

func webSecurityExitStatus(adapterID string, artifactData []byte, report gate.AdapterReport) (string, error) {
	if adapterID == websecurity.InventoryAdapterID {
		if report.Status != "succeeded" {
			return "", fmt.Errorf("OpenAPI inventory committed report is not succeeded")
		}
		return "completed", nil
	}
	result, err := websecurity.DecodeReplayResult(artifactData)
	if err != nil {
		return "", err
	}
	switch result.Status {
	case "matched", "different":
		if report.Status != "succeeded" {
			return "", fmt.Errorf("bounded replay completed result status drifted from report")
		}
		return result.Status, nil
	case "failed-before-delivery", "aborted-after-delivery":
		return result.Delivery.ErrorCode, nil
	case "delivery-uncertain":
		return result.Status, nil
	default:
		return "", fmt.Errorf("bounded replay result has unsupported terminal status")
	}
}
