package sessionhost

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

const claudeLaunchControlSchemaVersion = executioncontrol.BindingSchemaVersion

type claudeLaunchControlBinding = executioncontrol.Binding

func cloneClaudeLaunchControlBinding(binding *claudeLaunchControlBinding) *claudeLaunchControlBinding {
	return executioncontrol.CloneBinding(binding)
}

func sameClaudeLaunchControlBinding(left, right *claudeLaunchControlBinding) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func ensureClaudeLaunchControlBinding(
	opt Options,
	pkg mission.CurrentLoopExternalSessionHarnessPackage,
) (Options, error) {
	required, err := claudeLaunchControlRequired(opt.Target)
	if err != nil {
		return Options{}, err
	}
	if !required {
		if opt.launchControlBinding != nil {
			return Options{}, fmt.Errorf("Claude launch control binding is present outside an attached project")
		}
		return opt, nil
	}
	owner, err := claudeLaunchOwner(opt.Target, pkg)
	if err != nil {
		return Options{}, err
	}
	frozen := pkg.Launch.Attempt.LaunchControl
	if frozen != nil {
		if err := validateClaudeLaunchControlBinding(*frozen); err != nil {
			return Options{}, err
		}
		if frozen.Owner != owner {
			return Options{}, fmt.Errorf("Claude launch attempt execution control binding does not match its immutable owner")
		}
		if opt.launchControlBinding == nil {
			opt.launchControlBinding = cloneClaudeLaunchControlBinding(frozen)
		} else if !sameClaudeLaunchControlBinding(opt.launchControlBinding, frozen) {
			return Options{}, fmt.Errorf("Claude launch control binding differs from its immutable attempt")
		}
		return opt, nil
	}
	if opt.launchControlBinding == nil {
		binding, err := captureClaudeLaunchControlBinding(opt.Target, owner)
		if err != nil {
			return Options{}, err
		}
		opt.launchControlBinding = binding
		return opt, nil
	}
	if err := validateClaudeLaunchControlBinding(*opt.launchControlBinding); err != nil {
		return Options{}, err
	}
	if opt.launchControlBinding.Owner != owner {
		return Options{}, fmt.Errorf("Claude launch immutable input owner does not match its expected lane control binding")
	}
	return opt, nil
}

func claudeLaunchControlRequired(caseRoot string) (bool, error) {
	state, err := projectstate.Resolve(strings.TrimSpace(caseRoot))
	if err != nil {
		return false, err
	}
	return state.Existing && !state.Legacy, nil
}

func claudeLaunchOwner(
	caseRoot string,
	pkg mission.CurrentLoopExternalSessionHarnessPackage,
) (laneowner.Snapshot, error) {
	if pkg.Launch == nil {
		return laneowner.Snapshot{}, fmt.Errorf("Claude launch package omitted its launch binding")
	}
	if strings.TrimSpace(pkg.CaseRoot) == "" || !casePathEqual(caseRoot, pkg.CaseRoot) {
		return laneowner.Snapshot{}, fmt.Errorf("Claude launch package case root changed before launch")
	}
	switch pkg.SessionKind {
	case "member":
		inspection, err := validateClaudeMemberLaunchInput(caseRoot, *pkg.Launch)
		if err != nil {
			return laneowner.Snapshot{}, err
		}
		if inspection.TaskContext == nil {
			return laneowner.Snapshot{}, fmt.Errorf("Claude member launch omitted its immutable task owner")
		}
		owner := inspection.TaskContext.Owner
		return validateClaudeLaunchOwner(laneowner.Snapshot{
			Lane:               owner.Lane,
			CurrentExecutor:    owner.Executor,
			ExecutorGeneration: owner.ExecutorGeneration,
		})
	case "reviewer":
		receipt, _, err := validateReviewerLaunchIdentity(caseRoot, *pkg.Launch)
		if err != nil {
			return laneowner.Snapshot{}, err
		}
		return validateClaudeLaunchOwner(laneowner.Snapshot{
			Lane:               receipt.TargetLane,
			CurrentExecutor:    receipt.EffectiveOwner.CurrentExecutor,
			ExecutorGeneration: receipt.EffectiveOwner.ExecutorGeneration,
		})
	case "mission-commander-evidence-review":
		return missionCommanderEvidenceReviewLaunchOwner(caseRoot, *pkg.Launch)
	default:
		return laneowner.Snapshot{}, fmt.Errorf("unsupported Claude session kind %q", pkg.SessionKind)
	}
}

func missionCommanderEvidenceReviewLaunchOwner(
	caseRoot string,
	launch mission.CurrentLoopExternalSessionHarnessLaunch,
) (laneowner.Snapshot, error) {
	inputPath, err := anchoredPath(caseRoot, launch.Input.Path)
	if err != nil {
		return laneowner.Snapshot{}, err
	}
	inputData, err := rekitfs.ReadStableRegularFileAnchored(
		caseRoot,
		inputPath,
		"Mission Commander evidence review input",
		1<<20,
	)
	if err != nil {
		return laneowner.Snapshot{}, err
	}
	if !strings.EqualFold(bytesSHA256(inputData), launch.Input.SHA256) {
		return laneowner.Snapshot{}, fmt.Errorf("Mission Commander evidence review input sha256 changed before launch")
	}
	var input liveAcceptanceVMPIDAEvidenceReviewInput
	if err := strictJSON(inputData, &input); err != nil {
		return laneowner.Snapshot{}, fmt.Errorf("decode Mission Commander evidence review input: %w", err)
	}
	if input.SchemaVersion != 1 || input.Kind != "vmp-ida-index-evidence-review" ||
		strings.TrimSpace(input.GateEventID) == "" || !input.NoAuthority || !input.NoHeavyTool ||
		strings.TrimSpace(input.ReceiptPath) == "" || !validClaudeLaunchSHA256(input.ReceiptSHA256) {
		return laneowner.Snapshot{}, fmt.Errorf("Mission Commander evidence review input has invalid lane receipt bindings")
	}
	receiptPath, err := anchoredPath(caseRoot, input.ReceiptPath)
	if err != nil {
		return laneowner.Snapshot{}, err
	}
	receiptData, err := rekitfs.ReadStableRegularFileAnchored(
		caseRoot,
		receiptPath,
		"Mission Commander evidence review adapter execution receipt",
		1<<20,
	)
	if err != nil {
		return laneowner.Snapshot{}, err
	}
	if !strings.EqualFold(bytesSHA256(receiptData), input.ReceiptSHA256) {
		return laneowner.Snapshot{}, fmt.Errorf("Mission Commander evidence review receipt sha256 changed before launch")
	}
	receipt, err := adapterexecution.Decode(receiptData)
	if err != nil {
		return laneowner.Snapshot{}, err
	}
	if receipt.Gate.GateEventID != input.GateEventID ||
		!strings.EqualFold(receipt.Report.SHA256, input.ReportSHA256) ||
		!strings.EqualFold(receipt.Dispatch.SHA256, input.DispatchSHA256) {
		return laneowner.Snapshot{}, fmt.Errorf("Mission Commander evidence review receipt lineage changed before launch")
	}
	return validateClaudeLaunchOwner(laneowner.Snapshot{
		Lane:               receipt.Owner.Lane,
		CurrentExecutor:    receipt.Owner.CurrentExecutor,
		ExecutorGeneration: receipt.Owner.ExecutorGeneration,
	})
}

func validateClaudeLaunchOwner(owner laneowner.Snapshot) (laneowner.Snapshot, error) {
	owner.Lane = strings.TrimSpace(owner.Lane)
	owner.CurrentExecutor = strings.TrimSpace(owner.CurrentExecutor)
	if owner.Lane == "" || owner.CurrentExecutor == "" || owner.ExecutorGeneration <= 0 {
		return laneowner.Snapshot{}, fmt.Errorf("Claude launch immutable input has no exact durable lane owner")
	}
	return owner, nil
}

func captureClaudeLaunchControlBinding(
	caseRoot string,
	owner laneowner.Snapshot,
) (binding *claudeLaunchControlBinding, retErr error) {
	owner, err := validateClaudeLaunchOwner(owner)
	if err != nil {
		return nil, err
	}
	lease, err := lanemutation.AcquireLane(caseRoot, owner.Lane)
	if err != nil {
		return nil, err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	if err := lease.Validate(); err != nil {
		return nil, err
	}
	inspection, err := executioncontrol.Inspect(caseRoot, owner.Lane)
	if err != nil {
		return nil, err
	}
	if err := requireRunnableClaudeLaunchControl(inspection); err != nil {
		return nil, err
	}
	currentOwner, err := laneowner.Read(caseRoot, owner.Lane)
	if err != nil {
		return nil, err
	}
	if currentOwner != owner {
		return nil, fmt.Errorf("Claude launch immutable input owner is stale for lane %s", owner.Lane)
	}
	if err := lease.Validate(); err != nil {
		return nil, err
	}
	binding = &claudeLaunchControlBinding{
		SchemaVersion:        claudeLaunchControlSchemaVersion,
		Lane:                 owner.Lane,
		ControlGeneration:    inspection.CurrentGeneration,
		ControlReceiptSHA256: inspection.CurrentReceiptSHA256,
		Owner:                owner,
	}
	if err := validateClaudeLaunchControlBinding(*binding); err != nil {
		return nil, err
	}
	return binding, nil
}

func withClaudeLaunchControl(
	caseRoot string,
	binding *claudeLaunchControlBinding,
	pkg mission.CurrentLoopExternalSessionHarnessPackage,
	launch func() error,
) (retErr error) {
	if binding == nil {
		return launch()
	}
	if err := validateClaudeLaunchControlBinding(*binding); err != nil {
		return err
	}
	lease, err := lanemutation.AcquireLane(caseRoot, binding.Lane)
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	if err := lease.Validate(); err != nil {
		return err
	}
	owner, err := claudeLaunchOwner(caseRoot, pkg)
	if err != nil {
		return err
	}
	if owner != binding.Owner {
		return fmt.Errorf("Claude launch immutable input owner changed before process start")
	}
	inspection, err := executioncontrol.Inspect(caseRoot, binding.Lane)
	if err != nil {
		return err
	}
	if err := requireRunnableClaudeLaunchControl(inspection); err != nil {
		return err
	}
	if inspection.CurrentGeneration != binding.ControlGeneration ||
		!strings.EqualFold(inspection.CurrentReceiptSHA256, binding.ControlReceiptSHA256) {
		return fmt.Errorf(
			"Claude launch lane %s control head changed before process start",
			binding.Lane,
		)
	}
	currentOwner, err := laneowner.Read(caseRoot, binding.Lane)
	if err != nil {
		return err
	}
	if currentOwner != binding.Owner {
		return fmt.Errorf("Claude launch lane %s durable executor owner changed before process start", binding.Lane)
	}
	if err := lease.Validate(); err != nil {
		return err
	}
	return launch()
}

func requireRunnableClaudeLaunchControl(inspection executioncontrol.Inspection) error {
	if inspection.Pending {
		return fmt.Errorf(
			"Claude launch lane %s has pending execution control generation %d",
			inspection.Lane,
			inspection.PendingGeneration,
		)
	}
	if inspection.State != executioncontrol.StateRunning {
		return fmt.Errorf(
			"Claude launch lane %s execution control state is %s",
			inspection.Lane,
			inspection.State,
		)
	}
	return nil
}

func validateClaudeLaunchControlBinding(binding claudeLaunchControlBinding) error {
	if err := executioncontrol.ValidateBinding(binding); err != nil {
		return fmt.Errorf("Claude launch control binding is invalid: %w", err)
	}
	return nil
}

func validClaudeLaunchSHA256(value string) bool {
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	return err == nil && len(decoded) == sha256.Size
}
