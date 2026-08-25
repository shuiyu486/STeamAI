package adapterhost

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	"github.com/shuiyu486/re-context-kits/internal/rekit/binaryinventory"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instructionpacket"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/packidentity"
	"github.com/shuiyu486/re-context-kits/internal/rekit/processguard"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

const (
	binaryInventoryFileName        = "binary-inventory.json"
	binaryInventoryChildResultKind = "binary-inventory-child-result"
)

type BinaryInventoryChildOptions struct {
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

type BinaryInventoryChildResult struct {
	SchemaVersion       int                         `json:"schemaVersion"`
	Kind                string                      `json:"kind"`
	AdapterID           string                      `json:"adapterId"`
	InstructionIdentity *instructionpacket.Identity `json:"instructionIdentity,omitempty"`
	SourcePath          string                      `json:"sourcePath"`
	SourceSHA256        string                      `json:"sourceSha256"`
	InventorySHA256     string                      `json:"inventorySha256"`
	Inventory           binaryinventory.Sidecar     `json:"inventory"`
	ReadOnlyInput       bool                        `json:"readOnlyInput"`
	NoSampleExecution   bool                        `json:"noSampleExecution"`
	NoNetwork           bool                        `json:"noNetwork"`
	NoNetworkBoundary   string                      `json:"noNetworkBoundary"`
	NoCatalogEntry      bool                        `json:"noCatalogEntryExecution"`
	NoAuthority         bool                        `json:"noAuthorityOrConfirmed"`
}

type binaryInventoryChildBinding struct {
	repoRoot     string
	caseRoot     string
	sourcePath   string
	dispatchPath string
	dispatchSHA  string
	dispatch     adapterexecution.DispatchReceipt
}

// RunBinaryInventoryChild is the private compiled-in PE/ELF parser entrypoint.
// It reads one bounded case-local file and never executes the sample, a catalog
// entry, a shell command, a plugin, or a network client.
func RunBinaryInventoryChild(opt BinaryInventoryChildOptions) (BinaryInventoryChildResult, error) {
	if err := validateAdapterInstructionBinding(opt.CaseRoot, opt.Pack, opt.InstructionIdentity); err != nil {
		return BinaryInventoryChildResult{}, err
	}
	binding, err := validateBinaryInventoryChildBinding(opt)
	if err != nil {
		return BinaryInventoryChildResult{}, err
	}
	guard, err := acquireAuthorizedChildControlLease(
		binding.caseRoot,
		binding.dispatch,
		opt.ExecutionControlBinding,
		opt.ParentLaneLeaseHandle,
		opt.parentLeaseValidator,
		"binary inventory private child",
	)
	if err != nil {
		return BinaryInventoryChildResult{}, err
	}
	defer guard.Close()
	root, err := os.OpenRoot(binding.caseRoot)
	if err != nil {
		return BinaryInventoryChildResult{}, err
	}
	defer root.Close()
	data, opened, err := readStableBoundedInput(
		root,
		binding.sourcePath,
		binaryinventory.MaxInputBytes,
		"binary inventory input",
	)
	if err != nil {
		return BinaryInventoryChildResult{}, err
	}
	source, err := binaryinventory.BindSource(binding.sourcePath, data)
	if err != nil {
		return BinaryInventoryChildResult{}, err
	}
	if err := requireAuthorizedChildControlAtSink(
		binding.caseRoot, guard, opt.ExecutionControlBinding, binding.dispatch,
	); err != nil {
		return BinaryInventoryChildResult{}, err
	}
	inventory, err := binaryinventory.Inspect(source, data)
	if err != nil {
		return BinaryInventoryChildResult{}, err
	}
	if err := requireAuthorizedChildControlAtSink(
		binding.caseRoot, guard, opt.ExecutionControlBinding, binding.dispatch,
	); err != nil {
		return BinaryInventoryChildResult{}, err
	}
	inventoryData, err := binaryinventory.CanonicalBytes(inventory)
	if err != nil {
		return BinaryInventoryChildResult{}, err
	}
	dataAgain, openedAgain, err := readStableBoundedInput(
		root,
		binding.sourcePath,
		binaryinventory.MaxInputBytes,
		"binary inventory input",
	)
	if err != nil || !os.SameFile(opened, openedAgain) || !bytes.Equal(data, dataAgain) {
		return BinaryInventoryChildResult{}, fmt.Errorf("binary inventory input changed during inspection: %w", err)
	}
	if err := validateBinaryInventoryChildCurrent(binding); err != nil {
		return BinaryInventoryChildResult{}, err
	}
	return BinaryInventoryChildResult{
		SchemaVersion:       1,
		Kind:                binaryInventoryChildResultKind,
		AdapterID:           binaryinventory.AdapterID,
		InstructionIdentity: cloneAdapterInstructionIdentity(opt.InstructionIdentity),
		SourcePath:          source.Path,
		SourceSHA256:        source.SHA256,
		InventorySHA256:     binaryinventory.SHA256(inventoryData),
		Inventory:           inventory,
		ReadOnlyInput:       true,
		NoSampleExecution:   true,
		NoNetwork:           true,
		NoNetworkBoundary:   fixedChildNoNetworkCodepath,
		NoCatalogEntry:      true,
		NoAuthority:         true,
	}, nil
}

func validateBinaryInventoryChildBinding(opt BinaryInventoryChildOptions) (binaryInventoryChildBinding, error) {
	if err := packidentity.Validate(opt.Pack); err != nil {
		return binaryInventoryChildBinding{}, err
	}
	if strings.TrimSpace(opt.Pack) != defaults.DefaultPack ||
		strings.TrimSpace(opt.GateEventID) == "" ||
		!validSHA256(opt.ExpectedDispatchSHA256) ||
		strings.TrimSpace(opt.AdapterSession) == "" ||
		strings.TrimSpace(opt.Executor) == "" ||
		opt.ExpectedExecutorGeneration < 1 {
		return binaryInventoryChildBinding{}, fmt.Errorf("private binary inventory child requires exact pack, gate, dispatch, session, executor, and generation bindings")
	}
	caseRoot, err := filepath.Abs(strings.TrimSpace(opt.CaseRoot))
	if err != nil {
		return binaryInventoryChildBinding{}, err
	}
	repoRoot, err := filepath.Abs(strings.TrimSpace(opt.RepoRoot))
	if err != nil {
		return binaryInventoryChildBinding{}, err
	}
	dispatch, dispatchPath, dispatchSHA, _, err := gate.ReadCurrentAdapterExecutionDispatch(
		repoRoot,
		caseRoot,
		strings.TrimSpace(opt.Pack),
		strings.TrimSpace(opt.GateEventID),
	)
	if err != nil {
		return binaryInventoryChildBinding{}, err
	}
	sourcePath := cleanCaseRelative(opt.SourcePath)
	inventoryPath := filepath.ToSlash(filepath.Join(filepath.Dir(dispatch.ReportPath), binaryInventoryFileName))
	if !strings.EqualFold(dispatchSHA, opt.ExpectedDispatchSHA256) ||
		dispatch.Gate.GateEventID != strings.TrimSpace(opt.GateEventID) ||
		dispatch.Adapter.Pack != strings.TrimSpace(opt.Pack) ||
		dispatch.Adapter.AdapterID != binaryinventory.AdapterID ||
		dispatch.Owner.AdapterHarness != adapterHarness ||
		dispatch.Owner.AdapterSession != strings.TrimSpace(opt.AdapterSession) ||
		dispatch.Owner.CurrentExecutor != strings.TrimSpace(opt.Executor) ||
		dispatch.Owner.ExecutorGeneration != opt.ExpectedExecutorGeneration ||
		sourcePath == "" || sourcePath != opt.SourcePath || sourcePath != dispatch.Gate.Target {
		return binaryInventoryChildBinding{}, fmt.Errorf("private binary inventory child binding does not match the exact immutable dispatch")
	}
	if err := validateBinaryInventoryDispatch(dispatch, sourcePath, dispatch.ReportPath, inventoryPath); err != nil {
		return binaryInventoryChildBinding{}, err
	}
	binding := binaryInventoryChildBinding{
		repoRoot: repoRoot, caseRoot: caseRoot, sourcePath: sourcePath,
		dispatchPath: dispatchPath, dispatchSHA: dispatchSHA, dispatch: dispatch,
	}
	if err := validateBinaryInventoryChildCurrent(binding); err != nil {
		return binaryInventoryChildBinding{}, err
	}
	return binding, nil
}

func validateBinaryInventoryChildCurrent(binding binaryInventoryChildBinding) error {
	owner, err := laneowner.Read(binding.caseRoot, binding.dispatch.Owner.Lane)
	if err != nil {
		return err
	}
	if owner.CurrentExecutor != binding.dispatch.Owner.CurrentExecutor ||
		owner.ExecutorGeneration != binding.dispatch.Owner.ExecutorGeneration {
		return fmt.Errorf("private binary inventory child owner binding is no longer current")
	}
	if err := validateCurrentAuthorization(
		binding.repoRoot,
		binding.caseRoot,
		binding.dispatch,
		binding.dispatchPath,
		binding.dispatchSHA,
	); err != nil {
		return err
	}
	current, currentPath, currentSHA, _, err := gate.ReadCurrentAdapterExecutionDispatch(
		binding.repoRoot,
		binding.caseRoot,
		binding.dispatch.Adapter.Pack,
		binding.dispatch.Gate.GateEventID,
	)
	if err != nil || currentPath != binding.dispatchPath ||
		!strings.EqualFold(currentSHA, binding.dispatchSHA) ||
		!adapterexecution.DispatchSemanticEqual(current, binding.dispatch) {
		return fmt.Errorf("private binary inventory child dispatch changed during validation: %w", err)
	}
	return nil
}

func validateBinaryInventoryDispatch(
	dispatch adapterexecution.DispatchReceipt,
	sourcePath,
	reportPath,
	inventoryPath string,
) error {
	if err := packidentity.Validate(dispatch.Adapter.Pack); err != nil {
		return err
	}
	if dispatch.Adapter.Pack != defaults.DefaultPack ||
		dispatch.Adapter.AdapterID != binaryinventory.AdapterID ||
		dispatch.Adapter.Candidate.ID != binaryinventory.AdapterID ||
		dispatch.Gate.Action != "inspect" ||
		dispatch.Owner.AdapterHarness != adapterHarness {
		return fmt.Errorf("binary inventory adapter accepts only pack=%s action=inspect harness=%s and compiled-in candidate=%s", defaults.DefaultPack, adapterHarness, binaryinventory.AdapterID)
	}
	if sourcePath == "" || sourcePath != dispatch.Gate.Target || reportPath == "" ||
		inventoryPath == "" || inventoryPath == reportPath || inventoryPath == sourcePath || reportPath == sourcePath {
		return fmt.Errorf("binary inventory adapter dispatch has invalid input or output bindings")
	}
	if !withinAuthorizedOutput(inventoryPath, dispatch.Gate.OutputPaths) ||
		!withinAuthorizedOutput(reportPath, dispatch.Gate.OutputPaths) {
		return fmt.Errorf("binary inventory and report must stay within exact authorized output paths")
	}
	if dispatch.Gate.AuthorizedBudget.RuntimeSeconds < 1 ||
		dispatch.Gate.AuthorizedBudget.DiskMB < 1 ||
		dispatch.Gate.AuthorizedBudget.Requests != 1 {
		return fmt.Errorf("binary inventory dispatch budget is not bounded for one request")
	}
	return nil
}

func runBinaryInventoryChild(
	opt Options,
	dispatch adapterexecution.DispatchReceipt,
	dispatchSHA,
	sourcePath string,
	timeout time.Duration,
	executableSHA string,
	lease *lanemutation.Lease,
	afterLaunch func(int) error,
) ([]byte, int, error) {
	childOpt := BinaryInventoryChildOptions{
		RepoRoot: opt.RepoRoot, CaseRoot: opt.CaseRoot, Pack: opt.Pack,
		GateEventID: opt.GateEventID, ExpectedDispatchSHA256: dispatchSHA,
		AdapterSession:             dispatch.Owner.AdapterSession,
		Executor:                   dispatch.Owner.CurrentExecutor,
		ExpectedExecutorGeneration: dispatch.Owner.ExecutorGeneration,
		SourcePath:                 sourcePath,
		ExecutionControlBinding:    executioncontrol.CloneBinding(opt.ExecutionControlBinding),
		InstructionIdentity:        cloneAdapterInstructionIdentity(opt.InstructionIdentity),
	}
	if lease == nil {
		return nil, 0, fmt.Errorf("binary inventory child launch requires the parent lane mutation lease")
	}
	if err := lease.ValidateLaneFor(childOpt.CaseRoot, dispatch.Owner.Lane); err != nil {
		return nil, 0, err
	}
	if opt.testHooks != nil && opt.testHooks.runBinaryInventoryChild != nil {
		childOpt.parentLeaseValidator = func() error {
			return lease.ValidateLaneFor(childOpt.CaseRoot, dispatch.Owner.Lane)
		}
		stdout, childPID, err := opt.testHooks.runBinaryInventoryChild(childOpt)
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
		return nil, 0, fmt.Errorf("adapter executable identity changed before binary inventory child launch")
	}
	parentLaneLeaseFile, err := lease.DuplicateLaneLockForChild()
	if err != nil {
		return nil, 0, err
	}
	defer parentLaneLeaseFile.Close()
	childOpt.ParentLaneLeaseHandle = parentLaneLeaseFile.Fd()
	args := []string{
		privateChildBinaryInventoryFlag,
		"-repo", childOpt.RepoRoot,
		"-target", childOpt.CaseRoot,
		"-pack", childOpt.Pack,
		"-gate-event-id", childOpt.GateEventID,
		"-expected-dispatch-sha256", childOpt.ExpectedDispatchSHA256,
		"-adapter-session", childOpt.AdapterSession,
		"-executor", childOpt.Executor,
		"-expected-executor-generation", fmt.Sprintf("%d", childOpt.ExpectedExecutorGeneration),
		"-child-source-path", childOpt.SourcePath,
	}
	args, err = appendAdapterInstructionIdentityArg(args, childOpt.InstructionIdentity)
	if err != nil {
		return nil, 0, err
	}
	args, err = appendAuthorizedChildControlArgs(
		args, childOpt.ExecutionControlBinding, childOpt.ParentLaneLeaseHandle,
	)
	if err != nil {
		return nil, 0, err
	}
	stdout, _, childPID, err := runContainedProcessObservedWithInheritedFiles(
		binding,
		args,
		fixedChildEnvironment(),
		timeout,
		[]*os.File{parentLaneLeaseFile},
		afterLaunch,
	)
	return stdout, childPID, err
}

func decodeBinaryInventoryChildResult(data []byte, sourcePath string, expectedInstructionIdentity *instructionpacket.Identity) (BinaryInventoryChildResult, error) {
	var result BinaryInventoryChildResult
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("decode binary inventory child result: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return result, fmt.Errorf("binary inventory child stdout must contain exactly one strict JSON object")
	}
	if result.SchemaVersion != 1 || result.Kind != binaryInventoryChildResultKind ||
		result.AdapterID != binaryinventory.AdapterID || result.SourcePath != sourcePath ||
		!result.ReadOnlyInput || !result.NoSampleExecution || !result.NoNetwork ||
		result.NoNetworkBoundary != fixedChildNoNetworkCodepath || !result.NoCatalogEntry || !result.NoAuthority {
		return result, fmt.Errorf("binary inventory child returned an invalid strict result envelope")
	}
	if err := validateAdapterChildInstructionIdentity(expectedInstructionIdentity, result.InstructionIdentity); err != nil {
		return result, err
	}
	inventoryData, err := binaryinventory.CanonicalBytes(result.Inventory)
	if err != nil || result.Inventory.Source.Path != sourcePath ||
		result.SourceSHA256 != result.Inventory.Source.SHA256 ||
		!validSHA256(result.SourceSHA256) ||
		!strings.EqualFold(result.InventorySHA256, binaryinventory.SHA256(inventoryData)) {
		return result, fmt.Errorf("binary inventory child returned an invalid sidecar binding: %w", err)
	}
	return result, nil
}

func binaryInventorySourceBindingCurrent(root *os.Root, source binaryinventory.SourceBinding) bool {
	data, _, err := readStableBoundedInput(
		root,
		source.Path,
		binaryinventory.MaxInputBytes,
		"binary inventory input",
	)
	return err == nil && int64(len(data)) == source.Bytes && strings.EqualFold(binaryinventory.SHA256(data), source.SHA256)
}

func runBinaryInventoryExistingDispatch(
	opt Options,
	result Result,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA string,
	started time.Time,
) (_ Result, retErr error) {
	result.AdapterID = binaryinventory.AdapterID
	result.Lane = dispatch.Owner.Lane
	result.Executor = dispatch.Owner.CurrentExecutor
	result.Generation = dispatch.Owner.ExecutorGeneration
	result.AdapterSession = dispatch.Owner.AdapterSession
	result.DispatchPath = dispatchPath
	result.DispatchSHA256 = dispatchSHA
	result.InputPath = cleanCaseRelative(dispatch.Gate.Target)
	result.ReportPath = cleanCaseRelative(dispatch.ReportPath)
	result.ArtifactPath = filepath.ToSlash(filepath.Join(filepath.Dir(result.ReportPath), binaryInventoryFileName))
	result.NoNetwork = true
	result.NoNetworkBoundary = fixedChildNoNetworkCodepath
	if err := validateBinaryInventoryDispatch(dispatch, result.InputPath, result.ReportPath, result.ArtifactPath); err != nil {
		return result, err
	}
	lease, err := lanemutation.AcquireOpenLane(result.CaseRoot, result.Lane, "binary inventory adapter host")
	if err != nil {
		return result, err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	stateRoot, err := projectstate.Resolve(result.CaseRoot)
	if err != nil {
		return result, err
	}
	if stateRoot.Existing && !stateRoot.Legacy {
		current, currentPath, currentSHA, _, currentErr := gate.ReadCurrentAdapterExecutionDispatch(
			opt.RepoRoot,
			result.CaseRoot,
			result.Pack,
			result.GateEventID,
		)
		if currentErr != nil || currentPath != dispatchPath || !strings.EqualFold(currentSHA, dispatchSHA) ||
			!adapterexecution.DispatchSemanticEqual(current, dispatch) {
			return result, fmt.Errorf("binary inventory dispatch changed before recovery: %w", currentErr)
		}
		if err := requireAuthorizedAdapterControlWithLease(result.CaseRoot, lease, opt.ExecutionControlBinding, dispatch); err != nil {
			return result, fmt.Errorf("binary inventory recovery execution control is stale: %w", err)
		}
	}
	root, err := os.OpenRoot(result.CaseRoot)
	if err != nil {
		return result, err
	}
	defer root.Close()

	attempt, attemptSHA, attemptPresent, err := readBinaryInventoryExecutionAttempt(
		result.CaseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		result.ReportPath,
		nil,
	)
	if err != nil {
		return result, err
	}
	launchData, launchErr := readVMPIDAFile(
		result.CaseRoot,
		binaryInventoryChildLaunchPath(result.ReportPath),
		"binary inventory prior child launch proof",
		64<<10,
	)
	if launchErr == nil {
		if !attemptPresent {
			return result, fmt.Errorf("binary inventory child launch lacks its exact execution attempt")
		}
		source := attempt.Source
		result.InputSHA256 = source.SHA256
		launchSHA := sha256Hex(launchData)
		launch, err := readBinaryInventoryChildLaunch(result.CaseRoot, result.ReportPath, launchSHA)
		if err != nil || !strings.EqualFold(launch.AttemptSHA256, attemptSHA) {
			return result, fmt.Errorf("binary inventory child launch proof does not bind the exact parent-owned execution attempt: %w", err)
		}
		attemptStarted, err := time.Parse(time.RFC3339Nano, attempt.StartedAt)
		if err != nil {
			return result, err
		}
		commit, commitData, err := readBinaryInventoryOutputCommit(
			result.CaseRoot,
			dispatch,
			dispatchPath,
			dispatchSHA,
			result.ReportPath,
			result.ArtifactPath,
			source,
		)
		if err != nil {
			return result, err
		}
		if commit == nil {
			for _, path := range []string{result.ArtifactPath, result.ReportPath} {
				if _, statErr := root.Lstat(filepath.FromSlash(path)); statErr == nil {
					return result, fmt.Errorf("binary inventory interrupted launch has an uncommitted public output: %s", path)
				} else if !errors.Is(statErr, os.ErrNotExist) {
					return result, statErr
				}
			}
			if !binaryInventorySourceBindingCurrent(root, source) {
				return publishBinaryInventoryFailureReport(
					result, dispatch, dispatchPath, dispatchSHA, source, attemptStarted, "failed", "source-drift", launchSHA,
				)
			}
			if elapsedSecondsCeil(attemptStarted) > dispatch.Gate.AuthorizedBudget.RuntimeSeconds {
				return publishBinaryInventoryFailureReport(
					result, dispatch, dispatchPath, dispatchSHA, source, attemptStarted, "aborted", "runtime-budget-exceeded", launchSHA,
				)
			}
			if err := validateAdapterAuthorizationPhase(
				opt,
				opt.RepoRoot,
				result.CaseRoot,
				authorizationPhasePrePublication,
				dispatch,
				dispatchPath,
				dispatchSHA,
			); err != nil {
				return publishBinaryInventoryFailureReport(
					result, dispatch, dispatchPath, dispatchSHA, source, attemptStarted, "failed", "authorization-drift", launchSHA,
				)
			}
			return publishBinaryInventoryFailureReport(
				result, dispatch, dispatchPath, dispatchSHA, source, attemptStarted, "aborted", "parent-interrupted", launchSHA,
			)
		}
		if !strings.EqualFold(commit.ChildLaunchSHA256, launchSHA) {
			return result, fmt.Errorf("binary inventory output commit does not bind the exact child launch proof")
		}
		sealed := false
		seal, sealErr := readBinaryInventorySuccessSeal(
			result.CaseRoot,
			dispatch,
			dispatchSHA,
			result.ReportPath,
			source,
			commitData,
		)
		if sealErr == nil {
			if !strings.EqualFold(seal.ChildLaunchSHA256, launchSHA) {
				return result, fmt.Errorf("binary inventory success seal does not bind the exact child launch proof")
			}
			sealed = true
		} else if !errors.Is(sealErr, os.ErrNotExist) {
			return result, sealErr
		}
		if !sealed {
			if !binaryInventorySourceBindingCurrent(root, source) {
				return publishBinaryInventoryFailureReport(
					result, dispatch, dispatchPath, dispatchSHA, source, attemptStarted, "failed", "source-drift", launchSHA,
				)
			}
			if elapsedSecondsCeil(attemptStarted) > dispatch.Gate.AuthorizedBudget.RuntimeSeconds {
				return publishBinaryInventoryFailureReport(
					result, dispatch, dispatchPath, dispatchSHA, source, attemptStarted, "aborted", "runtime-budget-exceeded", launchSHA,
				)
			}
			if err := validateAdapterAuthorizationPhase(
				opt,
				opt.RepoRoot,
				result.CaseRoot,
				authorizationPhasePrePublication,
				dispatch,
				dispatchPath,
				dispatchSHA,
			); err != nil {
				return publishBinaryInventoryFailureReport(
					result, dispatch, dispatchPath, dispatchSHA, source, attemptStarted, "failed", "authorization-drift", launchSHA,
				)
			}
		}
		ownedOutputs, err := publishBinaryInventoryCommittedOutputs(root, *commit, opt.testHooks)
		if err != nil {
			cleanupErr := error(nil)
			if ownedOutputs.inventory != nil || ownedOutputs.report != nil {
				cleanupErr = removeOwnedBinaryInventoryPublicOutputs(root, ownedOutputs, *commit, opt.testHooks)
			}
			return result, errors.Join(err, cleanupErr)
		}
		if !sealed {
			if !binaryInventorySourceBindingCurrent(root, source) {
				if cleanupErr := removeOwnedBinaryInventoryPublicOutputs(root, ownedOutputs, *commit, opt.testHooks); cleanupErr != nil {
					return result, cleanupErr
				}
				return publishBinaryInventoryFailureReport(
					result, dispatch, dispatchPath, dispatchSHA, source, attemptStarted, "failed", "source-drift", launchSHA,
				)
			}
			if err := lease.Validate(); err != nil {
				if cleanupErr := removeOwnedBinaryInventoryPublicOutputs(root, ownedOutputs, *commit, opt.testHooks); cleanupErr != nil {
					return result, errors.Join(err, cleanupErr)
				}
				return publishBinaryInventoryFailureReport(
					result, dispatch, dispatchPath, dispatchSHA, source, attemptStarted, "failed", "authorization-drift", launchSHA,
				)
			}
			if err := validateAdapterAuthorizationPhase(
				opt,
				opt.RepoRoot,
				result.CaseRoot,
				authorizationPhasePostPublication,
				dispatch,
				dispatchPath,
				dispatchSHA,
			); err != nil {
				if cleanupErr := removeOwnedBinaryInventoryPublicOutputs(root, ownedOutputs, *commit, opt.testHooks); cleanupErr != nil {
					return result, errors.Join(err, cleanupErr)
				}
				return publishBinaryInventoryFailureReport(
					result, dispatch, dispatchPath, dispatchSHA, source, attemptStarted, "failed", "authorization-drift", launchSHA,
				)
			}
			if opt.testHooks != nil && opt.testHooks.beforeBinaryInventorySuccessSeal != nil {
				if err := opt.testHooks.beforeBinaryInventorySuccessSeal(); err != nil {
					return result, errors.Join(err, removeOwnedBinaryInventoryPublicOutputs(root, ownedOutputs, *commit, opt.testHooks))
				}
			}
			if err := publishBinaryInventorySuccessSeal(
				result.CaseRoot,
				dispatch,
				dispatchSHA,
				result.ReportPath,
				source,
				launchSHA,
				commitData,
			); err != nil {
				return result, errors.Join(err, removeOwnedBinaryInventoryPublicOutputs(root, ownedOutputs, *commit, opt.testHooks))
			}
		}
		result.ProcessID = 0
		result.ArtifactSHA256 = commit.Inventory.SHA256
		result.ReportSHA256 = commit.Report.SHA256
		result.ExecutionStatus = "succeeded"
		result.ExecutionExitStatus = "completed"
		result.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
		return result, nil
	}
	if !errors.Is(launchErr, os.ErrNotExist) {
		return result, launchErr
	}
	if attemptPresent {
		return result, fmt.Errorf("binary inventory execution attempt exists without a durable child launch; use a distinct authorized gate and dispatch")
	}

	current, currentPath, currentSHA, _, err := gate.ReadCurrentAdapterExecutionDispatch(
		opt.RepoRoot,
		result.CaseRoot,
		result.Pack,
		result.GateEventID,
	)
	if err != nil || currentPath != dispatchPath || !strings.EqualFold(currentSHA, dispatchSHA) ||
		!adapterexecution.DispatchSemanticEqual(current, dispatch) {
		return result, fmt.Errorf("binary inventory dispatch changed while acquiring lane lease: %w", err)
	}
	if err := requireAuthorizedAdapterControlWithLease(result.CaseRoot, lease, opt.ExecutionControlBinding, dispatch); err != nil {
		return result, fmt.Errorf("binary inventory execution control is stale: %w", err)
	}
	if err := validateAdapterAuthorizationPhase(
		opt,
		opt.RepoRoot,
		result.CaseRoot,
		authorizationPhasePreExecution,
		dispatch,
		dispatchPath,
		dispatchSHA,
	); err != nil {
		return result, err
	}
	input, inputInfo, err := readStableBoundedInput(
		root,
		result.InputPath,
		binaryinventory.MaxInputBytes,
		"binary inventory input",
	)
	if err != nil {
		return result, err
	}
	result.InputSHA256 = binaryinventory.SHA256(input)
	source, err := binaryinventory.BindSource(result.InputPath, input)
	if err != nil {
		return result, err
	}
	if err := ensureOutputParent(root, result.ReportPath); err != nil {
		return result, err
	}
	for path, label := range map[string]string{
		result.ArtifactPath: "binary inventory",
		result.ReportPath:   "binary inventory execution report",
	} {
		if _, err := root.Lstat(filepath.FromSlash(path)); err == nil {
			return result, fmt.Errorf("adapter host refuses an existing %s: %s", label, path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return result, err
		}
	}
	attempt, attemptSHA, err = publishBinaryInventoryExecutionAttempt(
		result.CaseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		result.ExecutableSHA256,
		result.ReportPath,
		source,
		started,
	)
	if err != nil {
		return result, err
	}
	started, err = time.Parse(time.RFC3339Nano, attempt.StartedAt)
	if err != nil {
		return result, err
	}
	remaining := time.Until(started.Add(time.Duration(dispatch.Gate.AuthorizedBudget.RuntimeSeconds) * time.Second))
	if remaining <= 0 {
		return result, fmt.Errorf("binary inventory runtime budget was exhausted before child launch")
	}
	launchSHA := ""
	var launchCutpointErr error
	afterLaunch := func(childPID int) error {
		var launchErr error
		launchSHA, launchErr = publishBinaryInventoryChildLaunch(
			result.CaseRoot,
			result.ReportPath,
			attemptSHA,
			childPID,
		)
		if launchErr == nil && opt.testHooks != nil && opt.testHooks.afterBinaryInventoryChildLaunch != nil {
			launchErr = opt.testHooks.afterBinaryInventoryChildLaunch(childPID)
			launchCutpointErr = launchErr
		}
		return launchErr
	}
	stdout, childPID, err := runBinaryInventoryChild(
		opt,
		dispatch,
		dispatchSHA,
		result.InputPath,
		remaining,
		result.ExecutableSHA256,
		lease,
		afterLaunch,
	)
	result.ProcessID = childPID
	if launchCutpointErr != nil {
		return result, launchCutpointErr
	}
	if err != nil {
		if childPID <= 0 || !validSHA256(launchSHA) {
			return result, err
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
		return publishBinaryInventoryFailureReport(
			result, dispatch, dispatchPath, dispatchSHA, source, started, status, exitStatus, launchSHA,
		)
	}
	if !validSHA256(launchSHA) {
		return result, fmt.Errorf("binary inventory child completed without its durable launch proof")
	}
	child, err := decodeBinaryInventoryChildResult(stdout, result.InputPath, opt.InstructionIdentity)
	if err != nil {
		return publishBinaryInventoryFailureReport(
			result, dispatch, dispatchPath, dispatchSHA, source, started, "failed", "child-invalid-stdout", launchSHA,
		)
	}
	if !strings.EqualFold(child.SourceSHA256, result.InputSHA256) || child.Inventory.Source.Bytes != int64(len(input)) {
		return publishBinaryInventoryFailureReport(
			result, dispatch, dispatchPath, dispatchSHA, source, started, "failed", "child-invalid-inventory", launchSHA,
		)
	}
	inventoryData, err := binaryinventory.CanonicalBytes(child.Inventory)
	if err != nil || !strings.EqualFold(binaryinventory.SHA256(inventoryData), child.InventorySHA256) {
		return publishBinaryInventoryFailureReport(
			result, dispatch, dispatchPath, dispatchSHA, source, started, "failed", "child-invalid-inventory", launchSHA,
		)
	}
	inputAgain, inputInfoAgain, err := readStableBoundedInput(
		root,
		result.InputPath,
		binaryinventory.MaxInputBytes,
		"binary inventory input",
	)
	if err != nil || !os.SameFile(inputInfo, inputInfoAgain) || !bytes.Equal(input, inputAgain) {
		return publishBinaryInventoryFailureReport(
			result, dispatch, dispatchPath, dispatchSHA, source, started, "failed", "source-drift", launchSHA,
		)
	}
	latest, latestPath, latestSHA, _, err := gate.ReadCurrentAdapterExecutionDispatch(
		opt.RepoRoot,
		result.CaseRoot,
		result.Pack,
		result.GateEventID,
	)
	if err != nil || latestPath != dispatchPath || !strings.EqualFold(latestSHA, dispatchSHA) ||
		!adapterexecution.DispatchSemanticEqual(latest, dispatch) {
		return publishBinaryInventoryFailureReport(
			result, dispatch, dispatchPath, dispatchSHA, source, started, "failed", "authorization-drift", launchSHA,
		)
	}
	if err := lease.Validate(); err != nil {
		return publishBinaryInventoryFailureReport(
			result, dispatch, dispatchPath, dispatchSHA, source, started, "failed", "authorization-drift", launchSHA,
		)
	}
	if err := validateAdapterAuthorizationPhase(
		opt,
		opt.RepoRoot,
		result.CaseRoot,
		authorizationPhasePrePublication,
		dispatch,
		dispatchPath,
		dispatchSHA,
	); err != nil {
		return publishBinaryInventoryFailureReport(
			result, dispatch, dispatchPath, dispatchSHA, source, started, "failed", "authorization-drift", launchSHA,
		)
	}
	report := binaryInventoryReport(result, dispatch, dispatchPath, dispatchSHA, started)
	if exceedsBudget(dispatch, report) {
		return publishBinaryInventoryFailureReport(
			result, dispatch, dispatchPath, dispatchSHA, source, started, "aborted", "runtime-budget-exceeded", launchSHA,
		)
	}
	reportData, err := canonicalJSON(report)
	if err != nil {
		return result, err
	}
	if int64(len(inventoryData)+len(reportData)) > int64(dispatch.Gate.AuthorizedBudget.DiskMB)<<20 {
		return publishBinaryInventoryFailureReport(
			result, dispatch, dispatchPath, dispatchSHA, source, started, "failed", "output-budget-exceeded", launchSHA,
		)
	}
	commit, commitData, err := buildBinaryInventoryOutputCommit(
		dispatch,
		dispatchPath,
		dispatchSHA,
		result.ReportPath,
		result.ArtifactPath,
		launchSHA,
		source,
		reportData,
		inventoryData,
	)
	if err != nil {
		return result, err
	}
	if _, err := rekitfs.WriteAtomicNoReplaceRegularFileAnchored(
		result.CaseRoot,
		binaryInventoryOutputCommitPath(result.ReportPath),
		"binary inventory output commit",
		commitData,
	); err != nil {
		return result, err
	}
	if opt.testHooks != nil && opt.testHooks.afterBinaryInventoryOutputCommit != nil {
		if err := opt.testHooks.afterBinaryInventoryOutputCommit(); err != nil {
			return result, err
		}
	}
	inputFinal, inputInfoFinal, err := readStableBoundedInput(
		root,
		result.InputPath,
		binaryinventory.MaxInputBytes,
		"binary inventory input",
	)
	if err != nil || !os.SameFile(inputInfo, inputInfoFinal) || !bytes.Equal(input, inputFinal) {
		return publishBinaryInventoryFailureReport(
			result, dispatch, dispatchPath, dispatchSHA, source, started, "failed", "source-drift", launchSHA,
		)
	}
	if err := lease.Validate(); err != nil {
		return publishBinaryInventoryFailureReport(
			result, dispatch, dispatchPath, dispatchSHA, source, started, "failed", "authorization-drift", launchSHA,
		)
	}
	if err := validateAdapterAuthorizationPhase(
		opt,
		opt.RepoRoot,
		result.CaseRoot,
		authorizationPhasePrePublication,
		dispatch,
		dispatchPath,
		dispatchSHA,
	); err != nil {
		return publishBinaryInventoryFailureReport(
			result, dispatch, dispatchPath, dispatchSHA, source, started, "failed", "authorization-drift", launchSHA,
		)
	}
	ownedOutputs, err := publishBinaryInventoryCommittedOutputs(root, commit, opt.testHooks)
	if err != nil {
		cleanupErr := error(nil)
		if ownedOutputs.inventory != nil || ownedOutputs.report != nil {
			cleanupErr = removeOwnedBinaryInventoryPublicOutputs(root, ownedOutputs, commit, opt.testHooks)
		}
		return result, errors.Join(err, cleanupErr)
	}
	cleanup := func(cause error) (Result, error) {
		cleanupErr := removeOwnedBinaryInventoryPublicOutputs(root, ownedOutputs, commit, opt.testHooks)
		return result, errors.Join(cause, cleanupErr)
	}
	inputFinal, inputInfoFinal, err = readStableBoundedInput(
		root,
		result.InputPath,
		binaryinventory.MaxInputBytes,
		"binary inventory input",
	)
	if err != nil || !os.SameFile(inputInfo, inputInfoFinal) || !bytes.Equal(input, inputFinal) {
		return cleanup(fmt.Errorf("binary inventory input changed after output publication: %w", err))
	}
	if err := lease.Validate(); err != nil {
		return cleanup(err)
	}
	if err := validateAdapterAuthorizationPhase(
		opt,
		opt.RepoRoot,
		result.CaseRoot,
		authorizationPhasePostPublication,
		dispatch,
		dispatchPath,
		dispatchSHA,
	); err != nil {
		return cleanup(err)
	}
	if elapsedSecondsCeil(started) > dispatch.Gate.AuthorizedBudget.RuntimeSeconds {
		return cleanup(fmt.Errorf("binary inventory runtime exceeded authorized gate budget after output publication"))
	}
	if opt.testHooks != nil && opt.testHooks.beforeBinaryInventorySuccessSeal != nil {
		if err := opt.testHooks.beforeBinaryInventorySuccessSeal(); err != nil {
			return cleanup(err)
		}
	}
	if err := publishBinaryInventorySuccessSeal(
		result.CaseRoot,
		dispatch,
		dispatchSHA,
		result.ReportPath,
		source,
		launchSHA,
		commitData,
	); err != nil {
		return cleanup(err)
	}
	result.ArtifactSHA256 = commit.Inventory.SHA256
	result.ReportSHA256 = commit.Report.SHA256
	result.ExecutionStatus = "succeeded"
	result.ExecutionExitStatus = "completed"
	result.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return result, nil
}

func binaryREAdapterArtifactPath(dispatch adapterexecution.DispatchReceipt) string {
	name := vmpIDAIndexPacketFileName
	if dispatch.Adapter.AdapterID == binaryinventory.AdapterID {
		name = binaryInventoryFileName
	}
	return filepath.ToSlash(filepath.Join(filepath.Dir(dispatch.ReportPath), name))
}

func readTerminalBinaryREReport(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA,
	artifactPath string,
) (gate.AdapterReport, []byte, []byte, bool, error) {
	if dispatch.Adapter.AdapterID == VMPIDAIndexAdapterID {
		return readTerminalVMPReport(caseRoot, dispatch, dispatchPath, dispatchSHA, artifactPath)
	}
	reportData, err := readVMPIDAFile(caseRoot, dispatch.ReportPath, "binary inventory terminal report", binaryinventory.MaxOutputBytes)
	if errors.Is(err, os.ErrNotExist) {
		return gate.AdapterReport{}, nil, nil, false, nil
	}
	if err != nil {
		return gate.AdapterReport{}, nil, nil, false, err
	}
	attempt, _, present, err := readBinaryInventoryExecutionAttempt(
		caseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		dispatch.ReportPath,
		nil,
	)
	if err != nil || !present {
		if err == nil {
			err = fmt.Errorf("binary inventory terminal report requires its exact execution attempt")
		}
		return gate.AdapterReport{}, nil, nil, false, err
	}
	source := attempt.Source
	var report gate.AdapterReport
	if err := decodeVMPIDAStrictJSON(reportData, &report); err != nil {
		return gate.AdapterReport{}, nil, nil, false, err
	}
	if report.Status == "failed" || report.Status == "aborted" {
		if len(report.OutputRefs) != 0 || len(report.EvidenceRefs) != 0 {
			return report, nil, nil, false, fmt.Errorf("failed binary inventory terminal report must not claim sidecar evidence")
		}
		_, launchSHA, bindingErr := terminalBinaryInventoryFailureBinding(report)
		if bindingErr != nil {
			return report, nil, nil, false, bindingErr
		}
		if err := validateBinaryInventoryChildLaunchAttempt(
			caseRoot,
			dispatch,
			dispatchPath,
			dispatchSHA,
			dispatch.ReportPath,
			launchSHA,
			source,
		); err != nil {
			return report, nil, nil, false, err
		}
		if _, err := terminalBinaryInventoryExecutionExitStatus(report); err != nil {
			return report, nil, nil, false, err
		}
		if _, artifactErr := readVMPIDAFile(caseRoot, artifactPath, "failed binary inventory sidecar", binaryinventory.MaxOutputBytes); !errors.Is(artifactErr, os.ErrNotExist) {
			if artifactErr == nil {
				artifactErr = fmt.Errorf("unexpected sidecar exists")
			}
			return report, nil, nil, false, artifactErr
		}
		return report, reportData, nil, true, nil
	}
	if report.Status != "succeeded" {
		return report, nil, nil, false, fmt.Errorf("unsupported binary inventory terminal report status: %s", report.Status)
	}
	commit, commitData, err := readBinaryInventoryOutputCommit(
		caseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		dispatch.ReportPath,
		artifactPath,
		source,
	)
	if err != nil || commit == nil {
		return report, nil, nil, false, fmt.Errorf("succeeded binary inventory report requires the exact output commit: %w", err)
	}
	seal, err := readBinaryInventorySuccessSeal(
		caseRoot,
		dispatch,
		dispatchSHA,
		dispatch.ReportPath,
		source,
		commitData,
	)
	if errors.Is(err, os.ErrNotExist) {
		return report, nil, nil, false, nil
	}
	if err != nil {
		return report, nil, nil, false, err
	}
	if err := validateBinaryInventoryChildLaunchAttempt(
		caseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		dispatch.ReportPath,
		seal.ChildLaunchSHA256,
		source,
	); err != nil {
		return report, nil, nil, false, err
	}
	inventoryData, err := readVMPIDAFile(caseRoot, artifactPath, "binary inventory terminal sidecar", binaryinventory.MaxOutputBytes)
	if err != nil || !bytes.Equal(inventoryData, commit.InventoryBytes) || !bytes.Equal(reportData, commit.ReportBytes) {
		return report, nil, nil, false, fmt.Errorf("binary inventory terminal outputs differ from their sealed commit: %w", err)
	}
	return report, reportData, inventoryData, true, nil
}

func validateBinaryInventoryFailureClosureArtifacts(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA,
	reportPath,
	expectedReportSHA string,
) error {
	reportData, err := readVMPIDAFile(
		caseRoot,
		reportPath,
		"binary inventory terminal failure report",
		binaryinventory.MaxOutputBytes,
	)
	if err != nil || !strings.EqualFold(sha256Hex(reportData), expectedReportSHA) {
		return fmt.Errorf("binary inventory terminal failure report changed: %w", err)
	}
	var report gate.AdapterReport
	if err := decodeVMPIDAStrictJSON(reportData, &report); err != nil {
		return err
	}
	exitStatus, launchSHA, err := terminalBinaryInventoryFailureBinding(report)
	if err != nil {
		return err
	}
	if terminalStatus, err := terminalBinaryInventoryExecutionExitStatus(report); err != nil || terminalStatus != exitStatus {
		return fmt.Errorf("binary inventory terminal exit status is invalid: %w", err)
	}
	attempt, _, present, err := readBinaryInventoryExecutionAttempt(
		caseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		reportPath,
		nil,
	)
	if err != nil || !present {
		return fmt.Errorf("binary inventory terminal failure lacks its exact execution attempt: %w", err)
	}
	return validateBinaryInventoryChildLaunchAttempt(
		caseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		reportPath,
		launchSHA,
		attempt.Source,
	)
}

func completeBinaryInventoryEvidenceLifecycle(
	opt AuthorizedRunOptions,
	result AuthorizedRunResult,
	lane string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA string,
) (AuthorizedRunResult, error) {
	if result.ExecutionStatus == "failed" || result.ExecutionStatus == "aborted" {
		reportData, err := readVMPIDAFile(
			result.CaseRoot,
			result.ReportPath,
			"binary inventory terminal failure report",
			binaryinventory.MaxOutputBytes,
		)
		if err != nil {
			return result, err
		}
		var report gate.AdapterReport
		if err := decodeVMPIDAStrictJSON(reportData, &report); err != nil {
			return result, err
		}
		exitStatus, launchSHA, err := terminalBinaryInventoryFailureBinding(report)
		if err != nil {
			return result, err
		}
		reportSHA := sha256Hex(reportData)
		if !strings.EqualFold(result.ReportSHA256, reportSHA) {
			return result, fmt.Errorf("binary inventory terminal failure report differs from the exact adapter result")
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
				ExpectedExecutionReportSHA256: reportSHA,
				ExecutionExitStatus:           exitStatus,
				RecoveryProofPath:             binaryInventoryChildLaunchPath(result.ReportPath),
				ExpectedRecoveryProofSHA256:   launchSHA,
				Actor:                         strings.TrimSpace(opt.Actor),
				ExecutionControlBinding:       executioncontrol.CloneBinding(opt.ExecutionControlBinding),
				ValidateExactArtifacts: func() error {
					if opt.testHooks != nil && opt.testHooks.beforeBinaryInventoryFailureClosureValidation != nil {
						if err := opt.testHooks.beforeBinaryInventoryFailureClosureValidation(); err != nil {
							return err
						}
					}
					return validateBinaryInventoryFailureClosureArtifacts(
						result.CaseRoot,
						dispatch,
						dispatchPath,
						dispatchSHA,
						result.ReportPath,
						reportSHA,
					)
				},
			},
		)
		if err != nil {
			return result, err
		}
		result.ReportSHA256 = sha256Hex(reportData)
		result.PacketPath = ""
		result.PacketSHA256 = ""
		result.ReceiptPath = closure.ReceiptPath
		result.ReceiptSHA256 = closure.ReceiptSHA256
		result.ObservationEventID = closure.ObservationEventID
		result.ExecutionStatus = report.Status
		result.ExecutionExitStatus = exitStatus
		return result, nil
	}
	validation, err := gate.ValidateAdapterExecutionReport(
		opt.RepoRoot,
		result.CaseRoot,
		result.Pack,
		gate.Options{GateEventID: result.GateEventID, ExecutionReportPath: result.ReportPath},
	)
	if err != nil {
		return result, err
	}
	if validation.Report == nil || validation.Report.Status != "succeeded" ||
		validation.AdapterContext == nil || validation.AdapterContext.Selected == nil ||
		validation.AdapterContext.Selected.ID != binaryinventory.AdapterID {
		return result, fmt.Errorf("binary inventory terminal report validation omitted the exact compiled-in adapter")
	}
	if !validation.ReceiptPresent {
		receiptOpt := gate.Options{
			GateEventID: result.GateEventID, ExecutionReportPath: result.ReportPath,
			AdapterID:                  binaryinventory.AdapterID,
			Executor:                   dispatch.Owner.CurrentExecutor,
			ExpectedExecutorGeneration: dispatch.Owner.ExecutorGeneration,
			AdapterHarness:             adapterHarness, AdapterSession: dispatch.Owner.AdapterSession,
			ExecutionExitStatus: "completed", Actor: strings.TrimSpace(opt.Actor),
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
			opt.RepoRoot,
			result.CaseRoot,
			result.Pack,
			gate.Options{GateEventID: result.GateEventID, ExecutionReportPath: result.ReportPath},
		)
		if err != nil {
			return result, err
		}
	}
	if !validation.Valid || !validation.ProvenanceValid || validation.AdapterExecution == nil || !validation.ReceiptPresent {
		return result, fmt.Errorf("validate binary inventory report and receipt: %s", strings.TrimSpace(validation.Error))
	}
	result.ReceiptPath = validation.AdapterExecutionReceiptPath
	result.ReceiptSHA256 = validation.AdapterExecutionReceiptSHA256
	if err := ValidateBinaryInventoryReceiptArtifacts(
		result.CaseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		result,
		*validation.AdapterExecution,
	); err != nil {
		return result, err
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
			return result, fmt.Errorf("record binary inventory execution observation: %w", recordErr)
		}
		observationEventID = observation.EventID
	}
	result.ObservationEventID = observationEventID
	if opt.DeferSuccessfulTaskBinding {
		return result, nil
	}
	owner, err := laneowner.Read(result.CaseRoot, lane)
	if err != nil || owner.CurrentExecutor != dispatch.Owner.CurrentExecutor ||
		owner.ExecutorGeneration != dispatch.Owner.ExecutorGeneration {
		return result, fmt.Errorf("binary inventory evidence owner changed before member task binding: %w", err)
	}
	binding := memberexecution.TaskBinding{
		Kind: "binary-inventory-evidence",
		Values: map[string]string{
			"gate-event-id":        result.GateEventID,
			"source-path":          dispatch.Gate.Target,
			"inventory-path":       result.PacketPath,
			"inventory-sha256":     result.PacketSHA256,
			"report-path":          result.ReportPath,
			"report-sha256":        result.ReportSHA256,
			"dispatch-path":        dispatchPath,
			"dispatch-sha256":      dispatchSHA,
			"receipt-path":         result.ReceiptPath,
			"receipt-sha256":       result.ReceiptSHA256,
			"observation-event-id": result.ObservationEventID,
		},
	}
	result.TaskBindingPath, result.TaskBindingSHA256, err = writeAuthorizedTaskBindingForOwner(
		result.CaseRoot,
		lane,
		dispatch.Owner.CurrentExecutor,
		dispatch.Owner.ExecutorGeneration,
		executioncontrol.CloneBinding(opt.ExecutionControlBinding),
		binding,
	)
	return result, err
}

func ValidateBinaryInventoryReceiptArtifacts(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA string,
	result AuthorizedRunResult,
	receipt adapterexecution.Receipt,
) error {
	if receipt.Dispatch.DispatchID != dispatch.DispatchID || receipt.Dispatch.Path != dispatchPath ||
		!strings.EqualFold(receipt.Dispatch.SHA256, dispatchSHA) || receipt.Gate.GateEventID != result.GateEventID ||
		receipt.Gate.Target != dispatch.Gate.Target || receipt.Adapter.AdapterID != binaryinventory.AdapterID ||
		receipt.Owner != dispatch.Owner || receipt.Report.Path != result.ReportPath ||
		!strings.EqualFold(receipt.Report.SHA256, result.ReportSHA256) || receipt.Execution.Outcome != "succeeded" ||
		receipt.Execution.ExitStatus != "completed" || len(receipt.Artifacts) != 1 ||
		receipt.Artifacts[0].Path != result.PacketPath ||
		!strings.EqualFold(receipt.Artifacts[0].SHA256, result.PacketSHA256) ||
		!reflect.DeepEqual(receipt.Artifacts[0].Roles, []string{"evidence", "output"}) {
		return fmt.Errorf("binary inventory receipt does not bind the exact dispatch, owner, report, and sidecar")
	}
	return ValidateBinaryInventoryReportArtifacts(
		caseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		result.ReportPath,
		result.PacketPath,
	)
}

func validateBinaryInventoryOutputPair(
	inventoryData,
	reportData []byte,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA,
	inventoryPath string,
	source binaryinventory.SourceBinding,
) error {
	inventory, err := binaryinventory.Decode(inventoryData)
	if err != nil || !reflect.DeepEqual(inventory.Source, source) || inventory.AdapterID != binaryinventory.AdapterID {
		return fmt.Errorf("binary inventory committed sidecar is invalid or source-drifted: %w", err)
	}
	var report gate.AdapterReport
	if err := decodeVMPIDAStrictJSON(reportData, &report); err != nil {
		return err
	}
	canonicalReport, err := canonicalJSON(report)
	if err != nil || !bytes.Equal(reportData, canonicalReport) ||
		report.SchemaVersion != 1 || report.Kind != "adapter-execution-report" ||
		report.AdapterID != binaryinventory.AdapterID || report.Action != "inspect" || report.Status != "succeeded" ||
		report.GateEventID != dispatch.Gate.GateEventID || report.Dispatch == nil ||
		report.Dispatch.DispatchID != dispatch.DispatchID || report.Dispatch.Path != dispatchPath ||
		!strings.EqualFold(report.Dispatch.SHA256, dispatchSHA) ||
		!reflect.DeepEqual(report.OutputRefs, []string{inventoryPath}) ||
		!reflect.DeepEqual(report.EvidenceRefs, []string{inventoryPath}) {
		return fmt.Errorf("binary inventory committed report does not match the exact dispatch and sidecar")
	}
	return nil
}

func ValidateBinaryInventoryReportArtifacts(
	caseRoot string,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA,
	reportPath,
	inventoryPath string,
) error {
	if err := validateBinaryInventoryDispatch(dispatch, dispatch.Gate.Target, reportPath, inventoryPath); err != nil {
		return err
	}
	reportData, err := readVMPIDAFile(caseRoot, reportPath, "binary inventory terminal report", binaryinventory.MaxOutputBytes)
	if err != nil {
		return err
	}
	var report gate.AdapterReport
	if err := decodeVMPIDAStrictJSON(reportData, &report); err != nil {
		return err
	}
	canonicalReport, err := canonicalJSON(report)
	if err != nil || !bytes.Equal(reportData, canonicalReport) ||
		report.SchemaVersion != 1 || report.Kind != "adapter-execution-report" ||
		report.AdapterID != binaryinventory.AdapterID || report.Action != "inspect" || report.Status != "succeeded" ||
		report.GateEventID != dispatch.Gate.GateEventID || report.Dispatch == nil ||
		report.Dispatch.DispatchID != dispatch.DispatchID || report.Dispatch.Path != dispatchPath ||
		!strings.EqualFold(report.Dispatch.SHA256, dispatchSHA) ||
		!reflect.DeepEqual(report.OutputRefs, []string{inventoryPath}) ||
		!reflect.DeepEqual(report.EvidenceRefs, []string{inventoryPath}) {
		return fmt.Errorf("binary inventory report does not match the exact dispatch and inventory")
	}
	inventoryData, err := readVMPIDAFile(caseRoot, inventoryPath, "binary inventory sidecar", binaryinventory.MaxOutputBytes)
	if err != nil {
		return err
	}
	inventory, err := binaryinventory.Decode(inventoryData)
	if err != nil || inventory.AdapterID != binaryinventory.AdapterID ||
		inventory.Source.Path != dispatch.Gate.Target || inventory.Source.Bytes < 1 {
		return fmt.Errorf("binary inventory sidecar does not match the exact dispatch source: %w", err)
	}
	return nil
}

func binaryInventoryReport(
	result Result,
	dispatch adapterexecution.DispatchReceipt,
	dispatchPath,
	dispatchSHA string,
	started time.Time,
) gate.AdapterReport {
	return gate.AdapterReport{
		SchemaVersion: 1,
		Kind:          "adapter-execution-report",
		AdapterID:     binaryinventory.AdapterID,
		Action:        "inspect",
		Status:        "succeeded",
		GateEventID:   result.GateEventID,
		Dispatch: &adapterexecution.ReportDispatchBinding{
			DispatchID: dispatch.DispatchID,
			Path:       dispatchPath,
			SHA256:     dispatchSHA,
		},
		ActualBudget: autonomy.Budget{
			RuntimeSeconds: elapsedSecondsCeil(started),
			DiskMB:         1,
			Requests:       1,
		},
		OutputRefs:   []string{result.ArtifactPath},
		EvidenceRefs: []string{result.ArtifactPath},
		Summary:      "Bounded PE/ELF inventory completed through a fixed compiled-in child without sample execution, network access, or catalog entry execution.",
	}
}
