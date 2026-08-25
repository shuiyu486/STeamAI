package adapterhost

import (
	"bytes"
	"context"
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
	"github.com/shuiyu486/re-context-kits/internal/rekit/instructionpacket"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
	"github.com/shuiyu486/re-context-kits/internal/rekit/packidentity"
	"github.com/shuiyu486/re-context-kits/internal/rekit/processguard"
	"github.com/shuiyu486/re-context-kits/internal/rekit/websecurity"
)

const (
	webSecurityPack = "web-security"

	openAPIInventoryFileName        = "openapi-inventory.json"
	openAPIInventoryChildResultKind = "openapi-inventory-child-result"
	boundedReplayResultFileName     = "bounded-replay-result.json"
	boundedReplayChildResultKind    = "bounded-replay-child-result"

	BoundedReplayNetworkBoundary = "fixed-child-exact-loopback-no-proxy-no-redirect-no-retry"
	boundedReplayNetworkBoundary = BoundedReplayNetworkBoundary
)

type OpenAPIInventoryChildOptions struct {
	RepoRoot                   string
	CaseRoot                   string
	Pack                       string
	GateEventID                string
	ExpectedDispatchSHA256     string
	AdapterSession             string
	Executor                   string
	ExpectedExecutorGeneration int
	SourcePath                 string
	ExecutionControlBinding    *executioncontrol.Binding
	ParentLaneLeaseHandle      uintptr
	InstructionIdentity        *instructionpacket.Identity
	parentLeaseValidator       func() error
}

type OpenAPIInventoryChildResult struct {
	SchemaVersion       int                         `json:"schemaVersion"`
	Kind                string                      `json:"kind"`
	AdapterID           string                      `json:"adapterId"`
	InstructionIdentity *instructionpacket.Identity `json:"instructionIdentity,omitempty"`
	SourcePath          string                      `json:"sourcePath"`
	SourceSHA256        string                      `json:"sourceSha256"`
	InventorySHA256     string                      `json:"inventorySha256"`
	Inventory           websecurity.Inventory       `json:"inventory"`
	ReadOnlyInput       bool                        `json:"readOnlyInput"`
	NoNetwork           bool                        `json:"noNetwork"`
	NoSecrets           bool                        `json:"noSecretsPersisted"`
	NoCatalogEntry      bool                        `json:"noCatalogEntryExecution"`
	NoAuthority         bool                        `json:"noAuthorityOrConfirmed"`
}

type BoundedReplayChildOptions struct {
	RepoRoot                   string
	CaseRoot                   string
	Pack                       string
	GateEventID                string
	ExpectedDispatchSHA256     string
	AdapterSession             string
	Executor                   string
	ExpectedExecutorGeneration int
	RequestPath                string
	ExecutionControlBinding    *executioncontrol.Binding
	ParentLaneLeaseHandle      uintptr
	InstructionIdentity        *instructionpacket.Identity
	parentLeaseValidator       func() error
	beforeExecute              func() error
}

type BoundedReplayChildResult struct {
	SchemaVersion       int                         `json:"schemaVersion"`
	Kind                string                      `json:"kind"`
	AdapterID           string                      `json:"adapterId"`
	InstructionIdentity *instructionpacket.Identity `json:"instructionIdentity,omitempty"`
	RequestPath         string                      `json:"requestPath"`
	RequestSHA256       string                      `json:"requestSha256"`
	ResultSHA256        string                      `json:"resultSha256"`
	Result              websecurity.ReplayResult    `json:"result"`
	LoopbackOnly        bool                        `json:"loopbackOnly"`
	NoAmbientProxy      bool                        `json:"noAmbientProxy"`
	NoRedirects         bool                        `json:"noRedirects"`
	NoRetries           bool                        `json:"noRetries"`
	NoRequestBody       bool                        `json:"noRequestBody"`
	NoSecrets           bool                        `json:"noSecretsPersisted"`
	NoCatalogEntry      bool                        `json:"noCatalogEntryExecution"`
	NoAuthority         bool                        `json:"noAuthorityOrConfirmed"`
	NetworkBoundary     string                      `json:"networkBoundary"`
}

type webSecurityChildBinding struct {
	repoRoot     string
	caseRoot     string
	inputPath    string
	dispatchPath string
	dispatchSHA  string
	dispatch     adapterexecution.DispatchReceipt
}

func RunOpenAPIInventoryChild(opt OpenAPIInventoryChildOptions) (OpenAPIInventoryChildResult, error) {
	if err := validateAdapterInstructionBinding(opt.CaseRoot, opt.Pack, opt.InstructionIdentity); err != nil {
		return OpenAPIInventoryChildResult{}, err
	}
	binding, err := validateWebSecurityChildBinding(
		opt.RepoRoot, opt.CaseRoot, opt.Pack, opt.GateEventID,
		opt.ExpectedDispatchSHA256, opt.AdapterSession, opt.Executor,
		opt.ExpectedExecutorGeneration, opt.SourcePath,
		websecurity.InventoryAdapterID, "inspect",
	)
	if err != nil {
		return OpenAPIInventoryChildResult{}, err
	}
	guard, err := acquireAuthorizedChildControlLease(
		binding.caseRoot,
		binding.dispatch,
		opt.ExecutionControlBinding,
		opt.ParentLaneLeaseHandle,
		opt.parentLeaseValidator,
		"OpenAPI inventory private child",
	)
	if err != nil {
		return OpenAPIInventoryChildResult{}, err
	}
	defer guard.Close()
	root, err := os.OpenRoot(binding.caseRoot)
	if err != nil {
		return OpenAPIInventoryChildResult{}, err
	}
	defer root.Close()
	data, opened, err := readStableBoundedInput(root, binding.inputPath, websecurity.MaxOpenAPIBytes, "OpenAPI source")
	if err != nil {
		return OpenAPIInventoryChildResult{}, err
	}
	source, err := websecurity.BindFile(binding.inputPath, data, websecurity.MaxOpenAPIBytes)
	if err != nil {
		return OpenAPIInventoryChildResult{}, err
	}
	if err := validateAdapterInstructionBinding(opt.CaseRoot, opt.Pack, opt.InstructionIdentity); err != nil {
		return OpenAPIInventoryChildResult{}, err
	}
	inventory, err := websecurity.ImportOpenAPI(source, data)
	if err != nil {
		return OpenAPIInventoryChildResult{}, err
	}
	inventoryData, err := websecurity.CanonicalInventoryBytes(inventory)
	if err != nil {
		return OpenAPIInventoryChildResult{}, err
	}
	again, openedAgain, err := readStableBoundedInput(root, binding.inputPath, websecurity.MaxOpenAPIBytes, "OpenAPI source")
	if err != nil || !os.SameFile(opened, openedAgain) || !bytes.Equal(data, again) {
		return OpenAPIInventoryChildResult{}, fmt.Errorf("OpenAPI source changed during inventory: %w", err)
	}
	if err := validateWebSecurityChildCurrent(binding); err != nil {
		return OpenAPIInventoryChildResult{}, err
	}
	if err := requireAuthorizedChildControlAtSink(
		binding.caseRoot, guard, opt.ExecutionControlBinding, binding.dispatch,
	); err != nil {
		return OpenAPIInventoryChildResult{}, err
	}
	return OpenAPIInventoryChildResult{
		SchemaVersion:       1,
		Kind:                openAPIInventoryChildResultKind,
		AdapterID:           websecurity.InventoryAdapterID,
		InstructionIdentity: cloneAdapterInstructionIdentity(opt.InstructionIdentity),
		SourcePath:          source.Path,
		SourceSHA256:        source.SHA256, InventorySHA256: websecurity.SHA256(inventoryData),
		Inventory: inventory, ReadOnlyInput: true, NoNetwork: true, NoSecrets: true,
		NoCatalogEntry: true, NoAuthority: true,
	}, nil
}

func RunBoundedReplayChild(opt BoundedReplayChildOptions) (BoundedReplayChildResult, error) {
	if err := validateAdapterInstructionBinding(opt.CaseRoot, opt.Pack, opt.InstructionIdentity); err != nil {
		return BoundedReplayChildResult{}, err
	}
	binding, err := validateWebSecurityChildBinding(
		opt.RepoRoot, opt.CaseRoot, opt.Pack, opt.GateEventID,
		opt.ExpectedDispatchSHA256, opt.AdapterSession, opt.Executor,
		opt.ExpectedExecutorGeneration, opt.RequestPath,
		websecurity.ReplayAdapterID, "network",
	)
	if err != nil {
		return BoundedReplayChildResult{}, err
	}
	guard, err := acquireAuthorizedChildControlLease(
		binding.caseRoot,
		binding.dispatch,
		opt.ExecutionControlBinding,
		opt.ParentLaneLeaseHandle,
		opt.parentLeaseValidator,
		"bounded replay private child",
	)
	if err != nil {
		return BoundedReplayChildResult{}, err
	}
	defer guard.Close()
	requestData, err := readVMPIDAFile(binding.caseRoot, binding.inputPath, "bounded replay request", websecurity.MaxReplayRequestBytes)
	if err != nil {
		return BoundedReplayChildResult{}, err
	}
	request, err := websecurity.DecodeReplayRequest(requestData)
	if err != nil {
		return BoundedReplayChildResult{}, err
	}
	inventoryData, err := readVMPIDAFile(binding.caseRoot, request.Inventory.Path, "OpenAPI inventory", websecurity.MaxInventoryBytes)
	if err != nil {
		return BoundedReplayChildResult{}, err
	}
	requestBinding, err := websecurity.BindFile(binding.inputPath, requestData, websecurity.MaxReplayRequestBytes)
	if err != nil {
		return BoundedReplayChildResult{}, err
	}
	if err := websecurity.ValidateReplayRequestBinding(requestBinding); err != nil {
		return BoundedReplayChildResult{}, err
	}
	resolver := func(ref string) (string, bool) { return os.LookupEnv(ref) }
	if _, _, err := websecurity.PreflightReplay(requestBinding, requestData, inventoryData, resolver); err != nil {
		return BoundedReplayChildResult{}, err
	}
	if opt.beforeExecute != nil {
		if err := opt.beforeExecute(); err != nil {
			return BoundedReplayChildResult{}, err
		}
	}
	if err := validateWebSecurityChildCurrent(binding); err != nil {
		return BoundedReplayChildResult{}, err
	}
	if err := requireAuthorizedChildControlAtSink(
		binding.caseRoot, guard, opt.ExecutionControlBinding, binding.dispatch,
	); err != nil {
		return BoundedReplayChildResult{}, err
	}
	if err := validateAdapterInstructionBinding(opt.CaseRoot, opt.Pack, opt.InstructionIdentity); err != nil {
		return BoundedReplayChildResult{}, err
	}
	result, err := websecurity.ExecuteReplay(context.Background(), requestBinding, requestData, inventoryData, resolver)
	if err != nil {
		return BoundedReplayChildResult{}, err
	}
	resultData, err := websecurity.CanonicalReplayResultBytes(result)
	if err != nil {
		return BoundedReplayChildResult{}, err
	}
	return BoundedReplayChildResult{
		SchemaVersion:       1,
		Kind:                boundedReplayChildResultKind,
		AdapterID:           websecurity.ReplayAdapterID,
		InstructionIdentity: cloneAdapterInstructionIdentity(opt.InstructionIdentity),
		RequestPath:         binding.inputPath, RequestSHA256: requestBinding.SHA256,
		ResultSHA256: websecurity.SHA256(resultData), Result: result,
		LoopbackOnly: true, NoAmbientProxy: true, NoRedirects: true, NoRetries: true,
		NoRequestBody: true, NoSecrets: true, NoCatalogEntry: true, NoAuthority: true,
		NetworkBoundary: boundedReplayNetworkBoundary,
	}, nil
}

func validateWebSecurityChildBinding(
	repoRootValue, caseRootValue, pack, gateEventID, expectedDispatchSHA,
	adapterSession, executor string,
	expectedGeneration int,
	inputPath, adapterID, action string,
) (webSecurityChildBinding, error) {
	if err := packidentity.Validate(pack); err != nil {
		return webSecurityChildBinding{}, err
	}
	if strings.TrimSpace(pack) != webSecurityPack || strings.TrimSpace(gateEventID) == "" ||
		!validSHA256(expectedDispatchSHA) || strings.TrimSpace(adapterSession) == "" ||
		strings.TrimSpace(executor) == "" || expectedGeneration < 1 {
		return webSecurityChildBinding{}, fmt.Errorf("private web-security child requires exact pack, gate, dispatch, session, executor, and generation bindings")
	}
	caseRoot, err := filepath.Abs(strings.TrimSpace(caseRootValue))
	if err != nil {
		return webSecurityChildBinding{}, err
	}
	repoRoot, err := filepath.Abs(strings.TrimSpace(repoRootValue))
	if err != nil {
		return webSecurityChildBinding{}, err
	}
	dispatch, dispatchPath, dispatchSHA, _, err := gate.ReadCurrentAdapterExecutionDispatch(repoRoot, caseRoot, pack, gateEventID)
	if err != nil {
		return webSecurityChildBinding{}, err
	}
	inputPath = cleanCaseRelative(inputPath)
	if !strings.EqualFold(dispatchSHA, expectedDispatchSHA) || dispatch.Gate.GateEventID != gateEventID ||
		dispatch.Adapter.Pack != pack || dispatch.Adapter.AdapterID != adapterID ||
		dispatch.Adapter.Candidate.ID != adapterID || dispatch.Gate.Action != action ||
		dispatch.Owner.AdapterHarness != adapterHarness || dispatch.Owner.AdapterSession != adapterSession ||
		dispatch.Owner.CurrentExecutor != executor || dispatch.Owner.ExecutorGeneration != expectedGeneration ||
		inputPath == "" || inputPath != dispatch.Gate.Target {
		return webSecurityChildBinding{}, fmt.Errorf("private web-security child binding does not match the exact immutable dispatch")
	}
	artifactPath := webSecurityArtifactPath(dispatch)
	if dispatch.ReportPath == "" || artifactPath == "" || artifactPath == dispatch.ReportPath || artifactPath == inputPath ||
		!withinAuthorizedOutput(artifactPath, dispatch.Gate.OutputPaths) || !withinAuthorizedOutput(dispatch.ReportPath, dispatch.Gate.OutputPaths) ||
		dispatch.Gate.AuthorizedBudget.RuntimeSeconds < 1 || dispatch.Gate.AuthorizedBudget.DiskMB < 1 || dispatch.Gate.AuthorizedBudget.Requests != 1 {
		return webSecurityChildBinding{}, fmt.Errorf("web-security dispatch has invalid input, output, or budget bindings")
	}
	binding := webSecurityChildBinding{repoRoot: repoRoot, caseRoot: caseRoot, inputPath: inputPath, dispatchPath: dispatchPath, dispatchSHA: dispatchSHA, dispatch: dispatch}
	if err := validateWebSecurityChildCurrent(binding); err != nil {
		return webSecurityChildBinding{}, err
	}
	return binding, nil
}

func validateWebSecurityChildCurrent(binding webSecurityChildBinding) error {
	owner, err := laneowner.Read(binding.caseRoot, binding.dispatch.Owner.Lane)
	if err != nil {
		return err
	}
	if owner.CurrentExecutor != binding.dispatch.Owner.CurrentExecutor || owner.ExecutorGeneration != binding.dispatch.Owner.ExecutorGeneration {
		return fmt.Errorf("private web-security child owner binding is no longer current")
	}
	if err := validateCurrentAuthorization(binding.repoRoot, binding.caseRoot, binding.dispatch, binding.dispatchPath, binding.dispatchSHA); err != nil {
		return err
	}
	current, currentPath, currentSHA, _, err := gate.ReadCurrentAdapterExecutionDispatch(binding.repoRoot, binding.caseRoot, binding.dispatch.Adapter.Pack, binding.dispatch.Gate.GateEventID)
	if err != nil || currentPath != binding.dispatchPath || !strings.EqualFold(currentSHA, binding.dispatchSHA) || !adapterexecution.DispatchSemanticEqual(current, binding.dispatch) {
		return fmt.Errorf("private web-security child dispatch changed during validation: %w", err)
	}
	return nil
}

func runWebSecurityExistingDispatch(
	opt Options,
	result Result,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA string,
	started time.Time,
) (_ Result, retErr error) {
	result.AdapterID = dispatch.Adapter.AdapterID
	result.Lane = dispatch.Owner.Lane
	result.Executor = dispatch.Owner.CurrentExecutor
	result.Generation = dispatch.Owner.ExecutorGeneration
	result.AdapterSession = dispatch.Owner.AdapterSession
	result.DispatchPath = dispatchPath
	result.DispatchSHA256 = dispatchSHA
	result.InputPath = cleanCaseRelative(dispatch.Gate.Target)
	result.ReportPath = cleanCaseRelative(dispatch.ReportPath)
	result.ArtifactPath = webSecurityArtifactPath(dispatch)
	result.NoAuthority = true
	result.NoNetwork = dispatch.Adapter.AdapterID == websecurity.InventoryAdapterID
	if result.NoNetwork {
		result.NoNetworkBoundary = fixedChildNoNetworkCodepath
	} else {
		result.NoNetworkBoundary = boundedReplayNetworkBoundary
	}

	lease, err := lanemutation.AcquireOpenLane(result.CaseRoot, result.Lane, "web-security adapter host")
	if err != nil {
		return result, err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	if err := requireAuthorizedAdapterControlWithLease(result.CaseRoot, lease, opt.ExecutionControlBinding, dispatch); err != nil {
		return result, fmt.Errorf("web-security execution control is stale: %w", err)
	}
	root, err := os.OpenRoot(result.CaseRoot)
	if err != nil {
		return result, err
	}
	defer root.Close()
	if recovered, found, recoverErr := recoverWebSecurityLaunchedAttempt(result, dispatch, dispatchPath, dispatchSHA, root); recoverErr != nil || found {
		return recovered, recoverErr
	}
	if err := validateAdapterAuthorizationPhase(opt, opt.RepoRoot, result.CaseRoot, authorizationPhasePreExecution, dispatch, dispatchPath, dispatchSHA); err != nil {
		return result, err
	}
	inputLimit := webSecurityInputLimit(dispatch.Adapter.AdapterID)
	inputData, inputInfo, err := readStableBoundedInput(root, result.InputPath, inputLimit, "web-security adapter input")
	if err != nil {
		return result, err
	}
	inputBinding, err := websecurity.BindFile(result.InputPath, inputData, int(inputLimit))
	if err != nil {
		return result, err
	}
	if dispatch.Adapter.AdapterID == websecurity.ReplayAdapterID {
		if err := websecurity.ValidateReplayRequestBinding(inputBinding); err != nil {
			return result, err
		}
	}
	result.InputSHA256 = inputBinding.SHA256
	replayAuthRef := ""
	if dispatch.Adapter.AdapterID == websecurity.ReplayAdapterID {
		request, decodeErr := websecurity.DecodeReplayRequest(inputData)
		if decodeErr != nil {
			return result, decodeErr
		}
		inventoryData, readErr := readVMPIDAFile(result.CaseRoot, request.Inventory.Path, "OpenAPI inventory", websecurity.MaxInventoryBytes)
		if readErr != nil {
			return result, readErr
		}
		if _, _, preflightErr := websecurity.PreflightReplay(inputBinding, inputData, inventoryData, func(ref string) (string, bool) { return os.LookupEnv(ref) }); preflightErr != nil {
			return result, preflightErr
		}
		if request.Auth != nil {
			replayAuthRef = request.Auth.AuthRef
		}
	}
	for _, path := range []string{result.ArtifactPath, result.ReportPath} {
		if _, statErr := root.Lstat(filepath.FromSlash(path)); statErr == nil {
			return result, fmt.Errorf("web-security adapter refuses existing public output: %s", path)
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return result, statErr
		}
	}
	executablePath, err := os.Executable()
	if err != nil {
		return result, err
	}
	executableData, err := readStableExecutable(executablePath)
	if err != nil {
		return result, err
	}
	result.ExecutableSHA256 = sha256Hex(executableData)
	attempt, attemptSHA, err := publishWebSecurityExecutionAttempt(result.CaseRoot, dispatch, dispatchPath, dispatchSHA, result.ExecutableSHA256, result.ReportPath, inputBinding, started)
	if err != nil {
		return result, err
	}
	_ = attempt
	timeout := time.Duration(dispatch.Gate.AuthorizedBudget.RuntimeSeconds) * time.Second
	launchSHA := ""
	var launchCutErr error
	afterLaunch := func(childPID int) error {
		sha, launchErr := publishWebSecurityChildLaunch(result.CaseRoot, result.ReportPath, dispatch.Adapter.AdapterID, attemptSHA, webSecurityChildBoundary(dispatch.Adapter.AdapterID), childPID)
		if launchErr != nil {
			return launchErr
		}
		launchSHA = sha
		if opt.testHooks != nil && opt.testHooks.afterWebSecurityChildLaunch != nil {
			launchCutErr = opt.testHooks.afterWebSecurityChildLaunch(childPID)
			return launchCutErr
		}
		return nil
	}
	var stdout []byte
	var childPID int
	if dispatch.Adapter.AdapterID == websecurity.InventoryAdapterID {
		stdout, childPID, err = runOpenAPIInventoryChildProcess(opt, dispatch, dispatchSHA, result.InputPath, timeout, result.ExecutableSHA256, lease, afterLaunch)
	} else {
		stdout, childPID, err = runBoundedReplayChildProcess(opt, dispatch, dispatchSHA, result.InputPath, replayAuthRef, timeout, result.ExecutableSHA256, lease, afterLaunch)
	}
	result.ProcessID = childPID
	if err != nil {
		if launchSHA == "" || launchCutErr != nil {
			return result, err
		}
		return publishWebSecurityInterruptedReport(result, dispatch, dispatchPath, dispatchSHA, started, launchSHA, webSecurityInterruptedExitStatus(dispatch.Adapter.AdapterID))
	}
	if launchSHA == "" {
		return result, fmt.Errorf("web-security child completed without a durable launch proof")
	}

	var artifactData []byte
	var reportStatus, summary, escalation string
	boundaryHits := []string{}
	if dispatch.Adapter.AdapterID == websecurity.InventoryAdapterID {
		child, err := decodeOpenAPIInventoryChildResult(stdout, result.InputPath, opt.InstructionIdentity)
		if err != nil {
			return result, err
		}
		artifactData, err = websecurity.CanonicalInventoryBytes(child.Inventory)
		if err != nil || !strings.EqualFold(child.InventorySHA256, websecurity.SHA256(artifactData)) {
			return result, fmt.Errorf("OpenAPI inventory child result binding is invalid: %w", err)
		}
		reportStatus = "succeeded"
		summary = "Bounded OpenAPI 3 JSON endpoint/auth inventory completed without network access, secret-value persistence, or catalog entry execution."
	} else {
		child, err := decodeBoundedReplayChildResult(stdout, result.InputPath, opt.InstructionIdentity)
		if err != nil {
			return result, err
		}
		artifactData, err = websecurity.CanonicalReplayResultBytes(child.Result)
		if err != nil || !strings.EqualFold(child.ResultSHA256, websecurity.SHA256(artifactData)) {
			return result, fmt.Errorf("bounded replay child result binding is invalid: %w", err)
		}
		switch child.Result.Status {
		case "matched", "different":
			reportStatus = "succeeded"
			summary = "One exact inventoried loopback request completed with a deterministic digest-only response diff."
		case "failed-before-delivery":
			reportStatus = "failed"
			summary = "Bounded replay failed before delivery; no request was sent."
		case "delivery-uncertain":
			reportStatus = "aborted"
			boundaryHits = []string{"delivery-uncertain"}
			escalation = "delivery-uncertain; do not retry or replace this request"
			summary = "Bounded replay delivery is uncertain and is terminal without retry."
		case "aborted-after-delivery":
			reportStatus = "aborted"
			boundaryHits = []string{child.Result.Delivery.ErrorCode}
			escalation = child.Result.Delivery.ErrorCode + "; request was delivered and must not be retried"
			summary = "Bounded replay stopped after one delivered request due to a response boundary."
		default:
			return result, fmt.Errorf("bounded replay child returned unsupported terminal status")
		}
	}

	report := gate.AdapterReport{
		SchemaVersion: 1, Kind: "adapter-execution-report", AdapterID: dispatch.Adapter.AdapterID,
		Action: dispatch.Gate.Action, Status: reportStatus, GateEventID: dispatch.Gate.GateEventID,
		Dispatch:     &adapterexecution.ReportDispatchBinding{DispatchID: dispatch.DispatchID, Path: dispatchPath, SHA256: dispatchSHA},
		ActualBudget: dispatch.Gate.AuthorizedBudget, OutputRefs: []string{result.ArtifactPath}, EvidenceRefs: []string{result.ArtifactPath},
		BoundaryHits: boundaryHits, Escalation: escalation, Summary: summary,
	}
	report.ActualBudget.RuntimeSeconds = elapsedSecondsCeil(started)
	report.ActualBudget.DiskMB = 1
	report.ActualBudget.Requests = 1
	if exceedsBudget(dispatch, report) || int64(len(artifactData)) > int64(dispatch.Gate.AuthorizedBudget.DiskMB)<<20 {
		return result, fmt.Errorf("web-security adapter exceeded authorized budget")
	}
	reportData, err := canonicalJSON(report)
	if err != nil {
		return result, err
	}
	inputAgain, inputInfoAgain, err := readStableBoundedInput(root, result.InputPath, inputLimit, "web-security adapter input")
	if err != nil || !os.SameFile(inputInfo, inputInfoAgain) || !bytes.Equal(inputData, inputAgain) {
		return publishWebSecurityInterruptedReport(result, dispatch, dispatchPath, dispatchSHA, started, launchSHA, webSecuritySourceDriftExitStatus(dispatch.Adapter.AdapterID))
	}
	commit, commitData, err := publishWebSecurityOutputCommit(
		result.CaseRoot, dispatch, dispatchPath, dispatchSHA, inputBinding, launchSHA,
		result.ArtifactPath, result.ReportPath, artifactData, reportData,
	)
	if err != nil {
		return result, err
	}
	if opt.testHooks != nil && opt.testHooks.afterWebSecurityOutputCommit != nil {
		if err := opt.testHooks.afterWebSecurityOutputCommit(); err != nil {
			return result, err
		}
	}
	if err := lease.Validate(); err != nil {
		return result, err
	}
	if err := publishWebSecurityOutputs(result.CaseRoot, commit); err != nil {
		return result, err
	}
	if opt.testHooks != nil && opt.testHooks.beforeWebSecuritySuccessSeal != nil {
		if err := opt.testHooks.beforeWebSecuritySuccessSeal(); err != nil {
			return result, err
		}
	}
	commitSHA := sha256Hex(commitData)
	if err := publishWebSecuritySuccessSeal(result.CaseRoot, dispatch, dispatchSHA, result.ReportPath, inputBinding, launchSHA, commitSHA); err != nil {
		return result, err
	}
	if err := readWebSecuritySuccessSeal(result.CaseRoot, dispatch, dispatchSHA, result.ReportPath, inputBinding, launchSHA, commitSHA); err != nil {
		return result, err
	}
	if err := lease.Validate(); err != nil {
		return result, err
	}
	return webSecurityResultFromCommit(result, commit)
}

func runOpenAPIInventoryChildProcess(
	opt Options,
	dispatch adapterexecution.DispatchReceipt,
	dispatchSHA,
	sourcePath string,
	timeout time.Duration,
	executableSHA string,
	lease *lanemutation.Lease,
	afterLaunch func(int) error,
) ([]byte, int, error) {
	child := OpenAPIInventoryChildOptions{
		RepoRoot:                   opt.RepoRoot,
		CaseRoot:                   opt.CaseRoot,
		Pack:                       opt.Pack,
		GateEventID:                opt.GateEventID,
		ExpectedDispatchSHA256:     dispatchSHA,
		AdapterSession:             dispatch.Owner.AdapterSession,
		Executor:                   dispatch.Owner.CurrentExecutor,
		ExpectedExecutorGeneration: dispatch.Owner.ExecutorGeneration,
		SourcePath:                 sourcePath,
		ExecutionControlBinding:    executioncontrol.CloneBinding(opt.ExecutionControlBinding),
		InstructionIdentity:        cloneAdapterInstructionIdentity(opt.InstructionIdentity),
	}
	if lease == nil {
		return nil, 0, fmt.Errorf("OpenAPI inventory child launch requires the parent lane mutation lease")
	}
	if err := lease.ValidateLaneFor(child.CaseRoot, dispatch.Owner.Lane); err != nil {
		return nil, 0, err
	}
	if opt.testHooks != nil && opt.testHooks.runOpenAPIInventoryChild != nil {
		child.parentLeaseValidator = func() error {
			return lease.ValidateLaneFor(child.CaseRoot, dispatch.Owner.Lane)
		}
		data, pid, err := opt.testHooks.runOpenAPIInventoryChild(child)
		if pid > 0 && afterLaunch != nil {
			err = errors.Join(err, afterLaunch(pid))
		}
		return data, pid, err
	}
	args := []string{privateChildOpenAPIInventoryFlag, "-repo", child.RepoRoot, "-target", child.CaseRoot, "-pack", child.Pack, "-gate-event-id", child.GateEventID, "-expected-dispatch-sha256", child.ExpectedDispatchSHA256, "-adapter-session", child.AdapterSession, "-executor", child.Executor, "-expected-executor-generation", fmt.Sprintf("%d", child.ExpectedExecutorGeneration), "-child-source-path", child.SourcePath}
	args, err := appendAdapterInstructionIdentityArg(args, child.InstructionIdentity)
	if err != nil {
		return nil, 0, err
	}
	return runWebSecurityPrivateChild(args, child.ExecutionControlBinding, lease, timeout, executableSHA, nil, afterLaunch)
}

func runBoundedReplayChildProcess(
	opt Options,
	dispatch adapterexecution.DispatchReceipt,
	dispatchSHA,
	requestPath,
	authRef string,
	timeout time.Duration,
	executableSHA string,
	lease *lanemutation.Lease,
	afterLaunch func(int) error,
) ([]byte, int, error) {
	child := BoundedReplayChildOptions{
		RepoRoot:                   opt.RepoRoot,
		CaseRoot:                   opt.CaseRoot,
		Pack:                       opt.Pack,
		GateEventID:                opt.GateEventID,
		ExpectedDispatchSHA256:     dispatchSHA,
		AdapterSession:             dispatch.Owner.AdapterSession,
		Executor:                   dispatch.Owner.CurrentExecutor,
		ExpectedExecutorGeneration: dispatch.Owner.ExecutorGeneration,
		RequestPath:                requestPath,
		ExecutionControlBinding:    executioncontrol.CloneBinding(opt.ExecutionControlBinding),
		InstructionIdentity:        cloneAdapterInstructionIdentity(opt.InstructionIdentity),
	}
	if lease == nil {
		return nil, 0, fmt.Errorf("bounded replay child launch requires the parent lane mutation lease")
	}
	if err := lease.ValidateLaneFor(child.CaseRoot, dispatch.Owner.Lane); err != nil {
		return nil, 0, err
	}
	if opt.testHooks != nil && opt.testHooks.runBoundedReplayChild != nil {
		child.parentLeaseValidator = func() error {
			return lease.ValidateLaneFor(child.CaseRoot, dispatch.Owner.Lane)
		}
		data, pid, err := opt.testHooks.runBoundedReplayChild(child)
		if pid > 0 && afterLaunch != nil {
			err = errors.Join(err, afterLaunch(pid))
		}
		return data, pid, err
	}
	args := []string{privateChildBoundedReplayFlag, "-repo", child.RepoRoot, "-target", child.CaseRoot, "-pack", child.Pack, "-gate-event-id", child.GateEventID, "-expected-dispatch-sha256", child.ExpectedDispatchSHA256, "-adapter-session", child.AdapterSession, "-executor", child.Executor, "-expected-executor-generation", fmt.Sprintf("%d", child.ExpectedExecutorGeneration), "-child-request-path", child.RequestPath}
	args, err := appendAdapterInstructionIdentityArg(args, child.InstructionIdentity)
	if err != nil {
		return nil, 0, err
	}
	return runWebSecurityPrivateChild(args, child.ExecutionControlBinding, lease, timeout, executableSHA, replayAuthEnvironment(authRef), afterLaunch)
}

func runWebSecurityPrivateChild(
	args []string,
	controlBinding *executioncontrol.Binding,
	lease *lanemutation.Lease,
	timeout time.Duration,
	executableSHA string,
	environment []string,
	afterLaunch func(int) error,
) ([]byte, int, error) {
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
		return nil, 0, fmt.Errorf("adapter executable identity changed before web-security child launch")
	}
	parentLaneLeaseFile, err := lease.DuplicateLaneLockForChild()
	if err != nil {
		return nil, 0, err
	}
	defer parentLaneLeaseFile.Close()
	args, err = appendAuthorizedChildControlArgs(args, controlBinding, parentLaneLeaseFile.Fd())
	if err != nil {
		return nil, 0, err
	}
	stdout, _, pid, err := runContainedProcessObservedWithInheritedFiles(
		binding,
		args,
		environment,
		timeout,
		[]*os.File{parentLaneLeaseFile},
		afterLaunch,
	)
	return stdout, pid, err
}

func replayAuthEnvironment(authRef string) []string {
	env := fixedChildEnvironment()
	authRef = strings.TrimSpace(authRef)
	if authRef == "" {
		return env
	}
	if value, ok := os.LookupEnv(authRef); ok && value != "" {
		env = append(env, authRef+"="+value)
	}
	return env
}

func decodeOpenAPIInventoryChildResult(data []byte, sourcePath string, expectedInstructionIdentity *instructionpacket.Identity) (OpenAPIInventoryChildResult, error) {
	var result OpenAPIInventoryChildResult
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return result, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return result, fmt.Errorf("OpenAPI inventory child stdout must contain one JSON object")
	}
	inventoryData, err := websecurity.CanonicalInventoryBytes(result.Inventory)
	if err != nil || result.SchemaVersion != 1 || result.Kind != openAPIInventoryChildResultKind || result.AdapterID != websecurity.InventoryAdapterID || result.SourcePath != sourcePath || result.SourceSHA256 != result.Inventory.Source.SHA256 || !strings.EqualFold(result.InventorySHA256, websecurity.SHA256(inventoryData)) || !result.ReadOnlyInput || !result.NoNetwork || !result.NoSecrets || !result.NoCatalogEntry || !result.NoAuthority {
		return result, fmt.Errorf("OpenAPI inventory child returned an invalid result: %w", err)
	}
	if err := validateAdapterChildInstructionIdentity(expectedInstructionIdentity, result.InstructionIdentity); err != nil {
		return result, err
	}
	return result, nil
}

func decodeBoundedReplayChildResult(data []byte, requestPath string, expectedInstructionIdentity *instructionpacket.Identity) (BoundedReplayChildResult, error) {
	var result BoundedReplayChildResult
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return result, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return result, fmt.Errorf("bounded replay child stdout must contain one JSON object")
	}
	resultData, err := websecurity.CanonicalReplayResultBytes(result.Result)
	if err != nil || result.SchemaVersion != 1 || result.Kind != boundedReplayChildResultKind || result.AdapterID != websecurity.ReplayAdapterID || result.RequestPath != requestPath || result.RequestSHA256 != result.Result.Request.SHA256 || !strings.EqualFold(result.ResultSHA256, websecurity.SHA256(resultData)) || !result.LoopbackOnly || !result.NoAmbientProxy || !result.NoRedirects || !result.NoRetries || !result.NoRequestBody || !result.NoSecrets || !result.NoCatalogEntry || !result.NoAuthority || result.NetworkBoundary != boundedReplayNetworkBoundary {
		return result, fmt.Errorf("bounded replay child returned an invalid result: %w", err)
	}
	if err := validateAdapterChildInstructionIdentity(expectedInstructionIdentity, result.InstructionIdentity); err != nil {
		return result, err
	}
	return result, nil
}

func webSecurityArtifactPath(dispatch adapterexecution.DispatchReceipt) string {
	name := openAPIInventoryFileName
	if dispatch.Adapter.AdapterID == websecurity.ReplayAdapterID {
		name = boundedReplayResultFileName
	}
	return filepath.ToSlash(filepath.Join(filepath.Dir(dispatch.ReportPath), name))
}
