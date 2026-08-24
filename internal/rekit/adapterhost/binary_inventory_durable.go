package adapterhost

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	"github.com/shuiyu486/re-context-kits/internal/rekit/binaryinventory"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
)

const (
	binaryInventoryExecutionAttemptFileName = ".binary-inventory-execution-attempt.json"
	binaryInventoryExecutionAttemptKind     = "binary-inventory-execution-attempt"
	binaryInventoryChildLaunchFileName      = ".binary-inventory-child-launch.json"
	binaryInventoryChildLaunchKind          = "binary-inventory-child-launch"
	binaryInventoryOutputCommitFileName     = ".binary-inventory-output-commit.json"
	binaryInventoryOutputCommitKind         = "binary-inventory-output-commit"
	binaryInventorySuccessSealFileName      = ".binary-inventory-success-seal.json"
	binaryInventorySuccessSealKind          = "binary-inventory-success-seal"

	binaryInventoryFailureExitStatusPrefix = "binary-inventory-exit-status:"
	binaryInventoryChildLaunchSHA256Marker = ";binary-inventory-child-launch-sha256:"
)

type binaryInventoryExecutionAttempt struct {
	SchemaVersion    int                           `json:"schemaVersion"`
	Kind             string                        `json:"kind"`
	AdapterID        string                        `json:"adapterId"`
	GateEventID      string                        `json:"gateEventId"`
	DispatchID       string                        `json:"dispatchId"`
	DispatchPath     string                        `json:"dispatchPath"`
	DispatchSHA256   string                        `json:"dispatchSha256"`
	Source           binaryinventory.SourceBinding `json:"source"`
	ExecutableSHA256 string                        `json:"executableSha256"`
	StartedAt        string                        `json:"startedAt"`
	Nonce            string                        `json:"nonce"`
}

type binaryInventoryChildLaunch struct {
	SchemaVersion     int    `json:"schemaVersion"`
	Kind              string `json:"kind"`
	AdapterID         string `json:"adapterId"`
	AttemptSHA256     string `json:"attemptSha256"`
	ChildProcessID    int    `json:"childProcessId"`
	NoNetworkBoundary string `json:"noNetworkBoundary"`
}

type binaryInventoryOutputFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type binaryInventoryOutputCommit struct {
	SchemaVersion     int                           `json:"schemaVersion"`
	Kind              string                        `json:"kind"`
	AdapterID         string                        `json:"adapterId"`
	GateEventID       string                        `json:"gateEventId"`
	DispatchID        string                        `json:"dispatchId"`
	DispatchPath      string                        `json:"dispatchPath"`
	DispatchSHA256    string                        `json:"dispatchSha256"`
	Source            binaryinventory.SourceBinding `json:"source"`
	ChildLaunchSHA256 string                        `json:"childLaunchSha256"`
	Inventory         binaryInventoryOutputFile     `json:"inventory"`
	Report            binaryInventoryOutputFile     `json:"report"`
	InventoryBytes    []byte                        `json:"inventoryBytes"`
	ReportBytes       []byte                        `json:"reportBytes"`
}

type binaryInventorySuccessSeal struct {
	SchemaVersion      int    `json:"schemaVersion"`
	Kind               string `json:"kind"`
	AdapterID          string `json:"adapterId"`
	GateEventID        string `json:"gateEventId"`
	DispatchID         string `json:"dispatchId"`
	DispatchSHA256     string `json:"dispatchSha256"`
	SourcePath         string `json:"sourcePath"`
	SourceSHA256       string `json:"sourceSha256"`
	SourceBytes        int64  `json:"sourceBytes"`
	ChildLaunchSHA256  string `json:"childLaunchSha256"`
	OutputCommitSHA256 string `json:"outputCommitSha256"`
}

func binaryInventoryExecutionAttemptPath(reportPath string) string {
	return filepath.ToSlash(filepath.Join(filepath.Dir(reportPath), binaryInventoryExecutionAttemptFileName))
}

func binaryInventoryChildLaunchPath(reportPath string) string {
	return filepath.ToSlash(filepath.Join(filepath.Dir(reportPath), binaryInventoryChildLaunchFileName))
}

func binaryInventoryOutputCommitPath(reportPath string) string {
	return filepath.ToSlash(filepath.Join(filepath.Dir(reportPath), binaryInventoryOutputCommitFileName))
}

func binaryInventorySuccessSealPath(reportPath string) string {
	return filepath.ToSlash(filepath.Join(filepath.Dir(reportPath), binaryInventorySuccessSealFileName))
}

func readBinaryInventoryExecutionAttempt(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA,
	reportPath string,
	expectedSource *binaryinventory.SourceBinding,
) (binaryInventoryExecutionAttempt, string, bool, error) {
	data, err := readVMPIDAFile(
		caseRoot,
		binaryInventoryExecutionAttemptPath(reportPath),
		"binary inventory execution attempt",
		64<<10,
	)
	if errors.Is(err, os.ErrNotExist) {
		return binaryInventoryExecutionAttempt{}, "", false, nil
	}
	if err != nil {
		return binaryInventoryExecutionAttempt{}, "", false, err
	}
	var attempt binaryInventoryExecutionAttempt
	if err := decodeVMPIDAStrictJSON(data, &attempt); err != nil {
		return attempt, "", true, err
	}
	canonical, err := canonicalJSON(attempt)
	if err != nil || !bytes.Equal(data, canonical) ||
		attempt.SchemaVersion != 1 || attempt.Kind != binaryInventoryExecutionAttemptKind ||
		attempt.AdapterID != binaryinventory.AdapterID || attempt.GateEventID != dispatch.Gate.GateEventID ||
		attempt.DispatchID != dispatch.DispatchID || attempt.DispatchPath != dispatchPath ||
		!strings.EqualFold(attempt.DispatchSHA256, dispatchSHA) || attempt.Source.Path != dispatch.Gate.Target ||
		binaryinventory.ValidateSource(attempt.Source) != nil ||
		(expectedSource != nil && !reflect.DeepEqual(attempt.Source, *expectedSource)) ||
		!validSHA256(attempt.ExecutableSHA256) || len(attempt.Nonce) != 32 {
		return attempt, "", true, fmt.Errorf("binary inventory execution attempt is not exact or canonical: %w", err)
	}
	started, startedErr := time.Parse(time.RFC3339Nano, attempt.StartedAt)
	if startedErr != nil || !strings.HasSuffix(attempt.StartedAt, "Z") ||
		attempt.StartedAt != started.UTC().Format(time.RFC3339Nano) {
		return attempt, "", true, fmt.Errorf("binary inventory execution attempt start time is invalid or non-canonical: %w", startedErr)
	}
	if _, err := hex.DecodeString(attempt.Nonce); err != nil {
		return attempt, "", true, fmt.Errorf("binary inventory execution attempt nonce is invalid: %w", err)
	}
	return attempt, sha256Hex(data), true, nil
}

func publishBinaryInventoryExecutionAttempt(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA,
	executableSHA,
	reportPath string,
	source binaryinventory.SourceBinding,
	started time.Time,
) (binaryInventoryExecutionAttempt, string, error) {
	if existing, existingSHA, present, err := readBinaryInventoryExecutionAttempt(
		caseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		reportPath,
		&source,
	); err != nil {
		return binaryInventoryExecutionAttempt{}, "", err
	} else if present {
		if !strings.EqualFold(existing.ExecutableSHA256, executableSHA) {
			return binaryInventoryExecutionAttempt{}, "", fmt.Errorf("binary inventory execution attempt executable identity changed")
		}
		return existing, existingSHA, nil
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return binaryInventoryExecutionAttempt{}, "", err
	}
	attempt := binaryInventoryExecutionAttempt{
		SchemaVersion:    1,
		Kind:             binaryInventoryExecutionAttemptKind,
		AdapterID:        binaryinventory.AdapterID,
		GateEventID:      dispatch.Gate.GateEventID,
		DispatchID:       dispatch.DispatchID,
		DispatchPath:     dispatchPath,
		DispatchSHA256:   strings.ToLower(dispatchSHA),
		Source:           source,
		ExecutableSHA256: strings.ToLower(executableSHA),
		StartedAt:        started.UTC().Format(time.RFC3339Nano),
		Nonce:            hex.EncodeToString(nonce),
	}
	data, err := canonicalJSON(attempt)
	if err != nil {
		return binaryInventoryExecutionAttempt{}, "", err
	}
	if _, err := rekitfs.WriteAtomicNoReplaceRegularFileAnchored(
		caseRoot,
		binaryInventoryExecutionAttemptPath(reportPath),
		"binary inventory execution attempt",
		data,
	); err != nil {
		return binaryInventoryExecutionAttempt{}, "", err
	}
	return attempt, sha256Hex(data), nil
}

func publishBinaryInventoryChildLaunch(caseRoot, reportPath, attemptSHA string, childPID int) (string, error) {
	launch := binaryInventoryChildLaunch{
		SchemaVersion:     1,
		Kind:              binaryInventoryChildLaunchKind,
		AdapterID:         binaryinventory.AdapterID,
		AttemptSHA256:     strings.ToLower(attemptSHA),
		ChildProcessID:    childPID,
		NoNetworkBoundary: fixedChildNoNetworkCodepath,
	}
	if childPID < 1 || !validSHA256(launch.AttemptSHA256) {
		return "", fmt.Errorf("binary inventory child launch proof requires an exact attempt and positive process id")
	}
	data, err := canonicalJSON(launch)
	if err != nil {
		return "", err
	}
	if _, err := rekitfs.WriteAtomicNoReplaceRegularFileAnchored(
		caseRoot,
		binaryInventoryChildLaunchPath(reportPath),
		"binary inventory child launch proof",
		data,
	); err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}

func readBinaryInventoryChildLaunch(caseRoot, reportPath, expectedSHA string) (binaryInventoryChildLaunch, error) {
	data, err := readVMPIDAFile(
		caseRoot,
		binaryInventoryChildLaunchPath(reportPath),
		"binary inventory child launch proof",
		64<<10,
	)
	if err != nil {
		return binaryInventoryChildLaunch{}, err
	}
	var launch binaryInventoryChildLaunch
	if err := decodeVMPIDAStrictJSON(data, &launch); err != nil {
		return launch, err
	}
	canonical, err := canonicalJSON(launch)
	if err != nil || !bytes.Equal(data, canonical) ||
		launch.SchemaVersion != 1 || launch.Kind != binaryInventoryChildLaunchKind ||
		launch.AdapterID != binaryinventory.AdapterID || !validSHA256(launch.AttemptSHA256) ||
		launch.ChildProcessID < 1 || launch.NoNetworkBoundary != fixedChildNoNetworkCodepath ||
		!strings.EqualFold(sha256Hex(data), expectedSHA) {
		return launch, fmt.Errorf("binary inventory child launch proof is not exact or canonical: %w", err)
	}
	return launch, nil
}

func validateBinaryInventoryChildLaunchAttempt(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA,
	reportPath,
	launchSHA string,
	source binaryinventory.SourceBinding,
) error {
	launch, err := readBinaryInventoryChildLaunch(caseRoot, reportPath, launchSHA)
	if err != nil {
		return err
	}
	_, attemptSHA, present, err := readBinaryInventoryExecutionAttempt(
		caseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		reportPath,
		&source,
	)
	if err != nil || !present || !strings.EqualFold(launch.AttemptSHA256, attemptSHA) {
		return fmt.Errorf("binary inventory child launch proof does not bind the exact parent-owned execution attempt: %w", err)
	}
	return nil
}

func buildBinaryInventoryOutputCommit(
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA,
	reportPath,
	inventoryPath,
	launchSHA string,
	source binaryinventory.SourceBinding,
	reportData,
	inventoryData []byte,
) (binaryInventoryOutputCommit, []byte, error) {
	commit := binaryInventoryOutputCommit{
		SchemaVersion:     1,
		Kind:              binaryInventoryOutputCommitKind,
		AdapterID:         binaryinventory.AdapterID,
		GateEventID:       dispatch.Gate.GateEventID,
		DispatchID:        dispatch.DispatchID,
		DispatchPath:      dispatchPath,
		DispatchSHA256:    strings.ToLower(dispatchSHA),
		Source:            source,
		ChildLaunchSHA256: strings.ToLower(launchSHA),
		Inventory:         binaryInventoryOutputFile{Path: inventoryPath, SHA256: sha256Hex(inventoryData), Bytes: int64(len(inventoryData))},
		Report:            binaryInventoryOutputFile{Path: reportPath, SHA256: sha256Hex(reportData), Bytes: int64(len(reportData))},
		InventoryBytes:    append([]byte{}, inventoryData...),
		ReportBytes:       append([]byte{}, reportData...),
	}
	if err := validateBinaryInventoryOutputCommit(commit, dispatch, dispatchPath, dispatchSHA, reportPath, inventoryPath, source); err != nil {
		return binaryInventoryOutputCommit{}, nil, err
	}
	data, err := canonicalJSON(commit)
	if err != nil {
		return binaryInventoryOutputCommit{}, nil, err
	}
	return commit, data, nil
}

func validateBinaryInventoryOutputCommit(
	commit binaryInventoryOutputCommit,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA,
	reportPath,
	inventoryPath string,
	source binaryinventory.SourceBinding,
) error {
	if commit.SchemaVersion != 1 || commit.Kind != binaryInventoryOutputCommitKind ||
		commit.AdapterID != binaryinventory.AdapterID || commit.GateEventID != dispatch.Gate.GateEventID ||
		commit.DispatchID != dispatch.DispatchID || commit.DispatchPath != dispatchPath ||
		!strings.EqualFold(commit.DispatchSHA256, dispatchSHA) || !reflect.DeepEqual(commit.Source, source) ||
		!validSHA256(commit.ChildLaunchSHA256) || commit.Inventory.Path != inventoryPath || commit.Report.Path != reportPath ||
		commit.Inventory.Bytes != int64(len(commit.InventoryBytes)) || commit.Report.Bytes != int64(len(commit.ReportBytes)) ||
		commit.Inventory.Bytes < 1 || commit.Inventory.Bytes > binaryinventory.MaxOutputBytes ||
		commit.Report.Bytes < 1 || commit.Report.Bytes > binaryinventory.MaxOutputBytes ||
		!strings.EqualFold(commit.Inventory.SHA256, sha256Hex(commit.InventoryBytes)) ||
		!strings.EqualFold(commit.Report.SHA256, sha256Hex(commit.ReportBytes)) {
		return fmt.Errorf("binary inventory output commit does not match the exact dispatch, source, sidecar, and report")
	}
	return validateBinaryInventoryOutputPair(
		commit.InventoryBytes,
		commit.ReportBytes,
		dispatch,
		dispatchPath,
		dispatchSHA,
		inventoryPath,
		source,
	)
}

func readBinaryInventoryOutputCommit(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA,
	reportPath,
	inventoryPath string,
	source binaryinventory.SourceBinding,
) (*binaryInventoryOutputCommit, []byte, error) {
	data, err := readVMPIDAFile(
		caseRoot,
		binaryInventoryOutputCommitPath(reportPath),
		"binary inventory output commit",
		(2*binaryinventory.MaxOutputBytes)+(64<<10),
	)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var commit binaryInventoryOutputCommit
	if err := decodeVMPIDAStrictJSON(data, &commit); err != nil {
		return nil, nil, fmt.Errorf("decode binary inventory output commit: %w", err)
	}
	canonical, err := canonicalJSON(commit)
	if err != nil || !bytes.Equal(data, canonical) {
		return nil, nil, fmt.Errorf("binary inventory output commit is not canonical: %w", err)
	}
	if err := validateBinaryInventoryOutputCommit(commit, dispatch, dispatchPath, dispatchSHA, reportPath, inventoryPath, source); err != nil {
		return nil, nil, err
	}
	return &commit, data, nil
}

func publishBinaryInventorySuccessSeal(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchSHA,
	reportPath string,
	source binaryinventory.SourceBinding,
	launchSHA string,
	commitData []byte,
) error {
	seal := binaryInventorySuccessSeal{
		SchemaVersion:      1,
		Kind:               binaryInventorySuccessSealKind,
		AdapterID:          binaryinventory.AdapterID,
		GateEventID:        dispatch.Gate.GateEventID,
		DispatchID:         dispatch.DispatchID,
		DispatchSHA256:     strings.ToLower(dispatchSHA),
		SourcePath:         source.Path,
		SourceSHA256:       source.SHA256,
		SourceBytes:        source.Bytes,
		ChildLaunchSHA256:  strings.ToLower(launchSHA),
		OutputCommitSHA256: sha256Hex(commitData),
	}
	data, err := canonicalJSON(seal)
	if err != nil {
		return err
	}
	_, err = rekitfs.WriteAtomicNoReplaceRegularFileAnchored(
		caseRoot,
		binaryInventorySuccessSealPath(reportPath),
		"binary inventory success seal",
		data,
	)
	return err
}

func readBinaryInventorySuccessSeal(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchSHA,
	reportPath string,
	source binaryinventory.SourceBinding,
	commitData []byte,
) (binaryInventorySuccessSeal, error) {
	data, err := readVMPIDAFile(
		caseRoot,
		binaryInventorySuccessSealPath(reportPath),
		"binary inventory success seal",
		64<<10,
	)
	if err != nil {
		return binaryInventorySuccessSeal{}, err
	}
	var seal binaryInventorySuccessSeal
	if err := decodeVMPIDAStrictJSON(data, &seal); err != nil {
		return seal, err
	}
	canonical, err := canonicalJSON(seal)
	if err != nil || !bytes.Equal(data, canonical) ||
		seal.SchemaVersion != 1 || seal.Kind != binaryInventorySuccessSealKind ||
		seal.AdapterID != binaryinventory.AdapterID || seal.GateEventID != dispatch.Gate.GateEventID ||
		seal.DispatchID != dispatch.DispatchID || !strings.EqualFold(seal.DispatchSHA256, dispatchSHA) ||
		seal.SourcePath != source.Path || !strings.EqualFold(seal.SourceSHA256, source.SHA256) ||
		seal.SourceBytes != source.Bytes || !validSHA256(seal.ChildLaunchSHA256) ||
		!strings.EqualFold(seal.OutputCommitSHA256, sha256Hex(commitData)) {
		return seal, fmt.Errorf("binary inventory success seal is not exact or canonical: %w", err)
	}
	return seal, nil
}

type binaryInventoryOwnedOutputs struct {
	inventory *ownedOutput
	report    *ownedOutput
}

func publishBinaryInventoryCommittedOutputs(
	root *os.Root,
	commit binaryInventoryOutputCommit,
	hooks *hostTestHooks,
) (binaryInventoryOwnedOutputs, error) {
	var owned binaryInventoryOwnedOutputs
	if root == nil || len(commit.InventoryBytes) == 0 || len(commit.ReportBytes) == 0 {
		return owned, fmt.Errorf("binary inventory committed output root or bytes are missing")
	}
	var err error
	owned.inventory, err = publishVMPIDAOutput(
		root,
		commit.Inventory.Path,
		"binary inventory committed sidecar",
		commit.InventoryBytes,
	)
	if err != nil {
		return owned, err
	}
	if hooks != nil && hooks.beforeReportWrite != nil {
		if err := hooks.beforeReportWrite(); err != nil {
			return owned, err
		}
	}
	owned.report, err = publishVMPIDAOutput(
		root,
		commit.Report.Path,
		"binary inventory committed report",
		commit.ReportBytes,
	)
	if err != nil && owned.inventory != nil {
		cleanupErr := removeOwnedBinaryInventoryPublicOutputs(root, owned, commit, hooks)
		if cleanupErr == nil {
			owned.inventory = nil
		}
		return owned, errors.Join(err, cleanupErr)
	}
	return owned, err
}

func removeOwnedBinaryInventoryPublicOutputs(
	root *os.Root,
	owned binaryInventoryOwnedOutputs,
	commit binaryInventoryOutputCommit,
	hooks *hostTestHooks,
) error {
	if root == nil {
		return fmt.Errorf("binary inventory output cleanup root is missing")
	}
	var afterIdentity func(string) error
	var afterQuarantineIdentity func(string, string) error
	if hooks != nil {
		afterIdentity = hooks.afterCleanupIdentityOpen
		afterQuarantineIdentity = hooks.afterCleanupQuarantineIdentityCheck
	}
	var cleanupErr error
	for _, output := range []struct {
		path     string
		owned    *ownedOutput
		expected []byte
	}{
		{path: commit.Inventory.Path, owned: owned.inventory, expected: commit.InventoryBytes},
		{path: commit.Report.Path, owned: owned.report, expected: commit.ReportBytes},
	} {
		if output.owned == nil {
			if _, err := root.Lstat(filepath.FromSlash(output.path)); errors.Is(err, os.ErrNotExist) {
				continue
			} else if err != nil {
				cleanupErr = errors.Join(cleanupErr, err)
				continue
			}
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf(
				"refuse cleanup because binary inventory output ownership was not captured at publication: %s",
				output.path,
			))
			continue
		}
		if err := validateOwnedOutput(root, output.path, output.owned, output.expected); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		if err := removeOwnedOutput(
			root,
			output.path,
			output.owned,
			afterIdentity,
			afterQuarantineIdentity,
		); err != nil && !isOwnedOutputIsolation(err) {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	return cleanupErr
}

func publishBinaryInventoryFailureReport(
	result Result,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA string,
	source binaryinventory.SourceBinding,
	started time.Time,
	status,
	exitStatus,
	launchSHA string,
) (Result, error) {
	if !validBinaryInventoryFailureExitStatus(exitStatus) {
		return result, fmt.Errorf("invalid binary inventory failure exit status: %s", exitStatus)
	}
	if err := validateBinaryInventoryChildLaunchAttempt(
		result.CaseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		result.ReportPath,
		launchSHA,
		source,
	); err != nil {
		return result, err
	}
	report := gate.AdapterReport{
		SchemaVersion: 1,
		Kind:          "adapter-execution-report",
		AdapterID:     binaryinventory.AdapterID,
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
		Escalation: binaryInventoryFailureEscalation(exitStatus, launchSHA),
		Summary:    "The fixed compiled-in binary inventory child reached a terminal failure; no sidecar was published and this dispatch must not be replayed.",
	}
	if exceedsBudget(dispatch, report) {
		report.ActualBudget.RuntimeSeconds = dispatch.Gate.AuthorizedBudget.RuntimeSeconds
	}
	reportData, err := canonicalJSON(report)
	if err != nil {
		return result, err
	}
	if _, err := rekitfs.WriteAtomicNoReplaceRegularFileAnchored(
		result.CaseRoot,
		result.ReportPath,
		"binary inventory terminal failure report",
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

func validBinaryInventoryFailureExitStatus(value string) bool {
	switch value {
	case "child-timeout", "child-failed", "child-invalid-stdout", "child-invalid-inventory", "source-drift", "authorization-drift", "output-budget-exceeded", "runtime-budget-exceeded", "parent-interrupted":
		return true
	}
	const prefix = "child-exit-"
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	code := strings.TrimPrefix(value, prefix)
	if code == "" {
		return false
	}
	for _, ch := range code {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return code == "0" || code[0] != '0'
}

func binaryInventoryFailureEscalation(exitStatus, launchSHA string) string {
	return binaryInventoryFailureExitStatusPrefix + exitStatus +
		binaryInventoryChildLaunchSHA256Marker + strings.ToLower(launchSHA)
}

func terminalBinaryInventoryFailureBinding(report gate.AdapterReport) (string, string, error) {
	marker := strings.TrimSpace(report.Escalation)
	if !strings.HasPrefix(marker, binaryInventoryFailureExitStatusPrefix) {
		return "", "", fmt.Errorf("failed binary inventory terminal report is missing its exact execution exit status")
	}
	binding := strings.TrimPrefix(marker, binaryInventoryFailureExitStatusPrefix)
	separator := strings.Index(binding, binaryInventoryChildLaunchSHA256Marker)
	if separator < 1 {
		return "", "", fmt.Errorf("failed binary inventory terminal report is missing its parent-owned child launch proof")
	}
	exitStatus := binding[:separator]
	launchSHA := binding[separator+len(binaryInventoryChildLaunchSHA256Marker):]
	if !validBinaryInventoryFailureExitStatus(exitStatus) || !validSHA256(launchSHA) || strings.ToLower(launchSHA) != launchSHA {
		return "", "", fmt.Errorf("failed binary inventory terminal report has an invalid execution status or child launch proof")
	}
	return exitStatus, launchSHA, nil
}

func terminalBinaryInventoryExecutionExitStatus(report gate.AdapterReport) (string, error) {
	status := strings.ToLower(strings.TrimSpace(report.Status))
	if status == "succeeded" {
		if strings.TrimSpace(report.Escalation) != "" {
			return "", fmt.Errorf("succeeded binary inventory report must not carry a failure exit status")
		}
		return "completed", nil
	}
	if status != "failed" && status != "aborted" {
		return "", fmt.Errorf("unsupported binary inventory terminal report status: %s", status)
	}
	exitStatus, _, err := terminalBinaryInventoryFailureBinding(report)
	if err != nil {
		return "", err
	}
	aborted := exitStatus == "child-timeout" || exitStatus == "runtime-budget-exceeded" || exitStatus == "parent-interrupted"
	if (status == "aborted") != aborted {
		return "", fmt.Errorf("binary inventory terminal report status does not match execution exit status: %s/%s", status, exitStatus)
	}
	return exitStatus, nil
}
