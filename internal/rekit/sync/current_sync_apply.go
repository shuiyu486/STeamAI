package sync

import (
	"errors"
	"fmt"
	"strings"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

var (
	currentSyncApplyTransitionHook               func(string, CurrentSyncPlan) error
	currentSyncHandleBoundExactMutationSupported = rekitfs.HandleBoundExactMutationSupported
	currentSyncAcquireRefreshLease               = acquireCurrentSyncRefreshLease
)

// SetCurrentSyncApplyTransitionHookForTest installs a deterministic package-test seam.
func SetCurrentSyncApplyTransitionHookForTest(
	hook func(string, CurrentSyncPlan) error,
) func() {
	previous := currentSyncApplyTransitionHook
	currentSyncApplyTransitionHook = hook
	return func() { currentSyncApplyTransitionHook = previous }
}

func runCurrentSyncApplyTransitionHook(stage string, plan CurrentSyncPlan) error {
	if currentSyncApplyTransitionHook == nil {
		return nil
	}
	return currentSyncApplyTransitionHook(stage, plan)
}

// CurrentSyncApply resumes or starts one exact reviewed current project refresh.
// It must be invoked by the external maintenance executable named by the plan.
func CurrentSyncApply(
	caseRoot,
	pack string,
	opt CurrentSyncApplyOptions,
) (_ CurrentSyncResult, retErr error) {
	caseFull, stateRoot, err := currentSyncRoots(caseRoot)
	if err != nil {
		return CurrentSyncResult{}, err
	}
	if strings.TrimSpace(opt.SourceRepoRoot) == "" ||
		!validCurrentSyncSHA(opt.ExpectedPlanSHA256) {
		return CurrentSyncResult{}, fmt.Errorf(
			"current sync Apply requires the external source repository and exact reviewed plan SHA-256",
		)
	}
	if !currentSyncHandleBoundExactMutationSupported() {
		return CurrentSyncResult{}, fmt.Errorf(
			"current sync Apply requires handle-bound exact filesystem mutation support",
		)
	}
	lease, err := currentSyncAcquireRefreshLease(caseFull)
	if err != nil {
		return CurrentSyncResult{}, err
	}
	defer func() {
		retErr = errors.Join(retErr, lease.Unlock())
	}()
	if err := lease.Validate(); err != nil {
		return CurrentSyncResult{}, err
	}

	snapshot, err := readCurrentSyncApplySnapshot(
		caseFull,
		stateRoot.Path,
		opt.ExpectedPlanSHA256,
	)
	if err != nil {
		return CurrentSyncResult{}, err
	}
	if snapshot.Route != currentSyncApplyFresh {
		if err := validateCurrentSyncApplyIdentity(
			snapshot.Intent,
			opt.SourceRepoRoot,
			caseFull,
			pack,
			opt.ExpectedPlanSHA256,
			opt.CurrentSyncOptions,
		); err != nil {
			return CurrentSyncResult{}, err
		}
		if err := validateCurrentSyncIntentSource(snapshot.Intent); err != nil {
			return CurrentSyncResult{}, err
		}
		if err := currentSyncValidateDurableMaintenanceProcess(
			snapshot.Intent.Plan,
		); err != nil {
			return CurrentSyncResult{}, err
		}
	}
	if snapshot.Route == currentSyncApplyReplay {
		return currentSyncReplayResultLocked(
			caseFull,
			stateRoot.Path,
			opt.ExpectedPlanSHA256,
			lease,
		)
	}
	if snapshot.Route == currentSyncApplyCleanup {
		result, err := currentSyncCleanupCommittedResult(
			caseFull,
			stateRoot.Path,
			snapshot,
			lease,
		)
		if err != nil {
			return CurrentSyncResult{}, err
		}
		result.Status = "refreshed"
		result.IsMutation = true
		result.Applied = true
		result.Replay = false
		result.AlreadyCurrent = false
		return result, nil
	}

	intent := snapshot.Intent
	progress := snapshot.Progress
	switch snapshot.Route {
	case currentSyncApplyFresh:
		fresh, err := buildCurrentSyncPlan(
			opt.SourceRepoRoot,
			caseFull,
			pack,
			opt.CurrentSyncOptions,
		)
		if err != nil {
			return CurrentSyncResult{}, err
		}
		if fresh.AlreadyCurrent || !strings.EqualFold(
			fresh.ExpectedPlanSHA256,
			opt.ExpectedPlanSHA256,
		) {
			return CurrentSyncResult{}, fmt.Errorf(
				"current sync plan changed after preview; rerun sync preview",
			)
		}
		intent, err = buildCurrentSyncIntent(fresh)
		if err != nil {
			return CurrentSyncResult{}, err
		}
		if err := currentSyncValidateDurableMaintenanceProcess(
			intent.Plan,
		); err != nil {
			return CurrentSyncResult{}, err
		}
		if _, err := publishCurrentSyncIntent(caseFull, intent); err != nil {
			return CurrentSyncResult{}, err
		}
		if err := runCurrentSyncApplyTransitionHook(
			"after-intent-publication",
			fresh,
		); err != nil {
			return CurrentSyncResult{}, err
		}
		if err := stageCurrentSyncMaterial(caseFull, intent, fresh); err != nil {
			return CurrentSyncResult{}, err
		}
		progress, err = newCurrentSyncProgress(intent)
		if err != nil {
			return CurrentSyncResult{}, err
		}
		if _, err := publishCurrentSyncProgress(
			caseFull,
			intent,
			progress,
		); err != nil {
			return CurrentSyncResult{}, err
		}
		if err := runCurrentSyncApplyTransitionHook(
			"after-initial-progress-publication",
			fresh,
		); err != nil {
			return CurrentSyncResult{}, err
		}
	case currentSyncApplyRestoreActive:
		if err := validateCurrentSyncApplyIdentity(
			intent,
			opt.SourceRepoRoot,
			caseFull,
			pack,
			opt.ExpectedPlanSHA256,
			opt.CurrentSyncOptions,
		); err != nil {
			return CurrentSyncResult{}, err
		}
		if _, err := publishCurrentSyncIntent(caseFull, intent); err != nil {
			return CurrentSyncResult{}, err
		}
		fresh, err := rebuildCurrentSyncPlanForStaging(
			opt.SourceRepoRoot,
			caseFull,
			pack,
			opt.ExpectedPlanSHA256,
			opt.CurrentSyncOptions,
			intent,
		)
		if err != nil {
			return CurrentSyncResult{}, err
		}
		if err := stageCurrentSyncMaterial(caseFull, intent, fresh); err != nil {
			return CurrentSyncResult{}, err
		}
		progress, err = newCurrentSyncProgress(intent)
		if err != nil {
			return CurrentSyncResult{}, err
		}
		if _, err := publishCurrentSyncProgress(
			caseFull,
			intent,
			progress,
		); err != nil {
			return CurrentSyncResult{}, err
		}
	case currentSyncApplyResume:
		if err := validateCurrentSyncApplyIdentity(
			intent,
			opt.SourceRepoRoot,
			caseFull,
			pack,
			opt.ExpectedPlanSHA256,
			opt.CurrentSyncOptions,
		); err != nil {
			return CurrentSyncResult{}, err
		}
		if progress.Generation == 0 {
			return CurrentSyncResult{}, fmt.Errorf(
				"current sync active transaction has no durable progress",
			)
		}
	default:
		return CurrentSyncResult{}, fmt.Errorf(
			"current sync Apply route is unsupported: %s",
			snapshot.Route,
		)
	}

	if err := lease.Validate(); err != nil {
		return CurrentSyncResult{}, err
	}
	progress, err = runCurrentSyncForward(
		caseFull,
		intent,
		progress,
		currentSyncOperationEffectFor(caseFull, intent),
	)
	if err != nil {
		return CurrentSyncResult{}, err
	}
	if !currentSyncProgressTerminal(progress, intent) {
		return CurrentSyncResult{}, fmt.Errorf(
			"current sync forward journal did not reach terminal state",
		)
	}
	receipt, exists, err := readCurrentSyncReceipt(stateRoot.Path)
	if err != nil || !exists {
		return CurrentSyncResult{}, errors.Join(
			err,
			fmt.Errorf("current sync committed receipt is unavailable"),
		)
	}
	if _, _, err := validateCurrentSyncReplayState(
		caseFull,
		stateRoot.Path,
		receipt,
	); err != nil {
		return CurrentSyncResult{}, err
	}
	if err := runCurrentSyncApplyTransitionHook(
		"before-terminal-cleanup",
		intent.Plan,
	); err != nil {
		return CurrentSyncResult{}, err
	}
	return currentSyncCleanupCommittedResult(
		caseFull,
		stateRoot.Path,
		currentSyncApplySnapshot{
			Route:    currentSyncApplyCleanup,
			Intent:   intent,
			Progress: progress,
			Receipt:  receipt,
		},
		lease,
	)
}

func currentSyncCleanupCommittedResult(
	caseRoot,
	stateRoot string,
	snapshot currentSyncApplySnapshot,
	lease *currentSyncRefreshLease,
) (CurrentSyncResult, error) {
	if snapshot.Route != currentSyncApplyCleanup ||
		!currentSyncProgressTerminal(snapshot.Progress, snapshot.Intent) ||
		!strings.EqualFold(
			snapshot.Receipt.PlanSHA256,
			snapshot.Intent.PlanSHA256,
		) {
		return CurrentSyncResult{}, fmt.Errorf(
			"current sync terminal cleanup artifacts are invalid",
		)
	}
	if err := lease.Validate(); err != nil {
		return CurrentSyncResult{}, err
	}
	if _, _, err := validateCurrentSyncReplayState(
		caseRoot,
		stateRoot,
		snapshot.Receipt,
	); err != nil {
		return CurrentSyncResult{}, err
	}
	intentData, err := currentSyncCanonicalData(snapshot.Intent)
	if err != nil {
		return CurrentSyncResult{}, err
	}
	ownerData, err := currentSyncCanonicalData(
		currentSyncOwnerFor(snapshot.Intent),
	)
	if err != nil {
		return CurrentSyncResult{}, err
	}
	root, err := rekitfs.OpenAnchoredRoot(stateRoot)
	if err != nil {
		return CurrentSyncResult{}, err
	}
	removeIntentErr := root.RemoveExactFile(
		currentSyncIntentRel,
		intentData,
		0o600,
	)
	removeOwnerErr := root.RemoveExactFile(
		currentSyncOwnerRel,
		ownerData,
		0o600,
	)
	validateErr := root.Validate()
	closeErr := root.Close()
	if removeIntentErr != nil || removeOwnerErr != nil ||
		validateErr != nil || closeErr != nil {
		return CurrentSyncResult{}, errors.Join(
			removeIntentErr,
			removeOwnerErr,
			validateErr,
			closeErr,
		)
	}
	if err := lease.Validate(); err != nil {
		return CurrentSyncResult{}, err
	}
	result, err := currentSyncReplayResultLocked(
		caseRoot,
		stateRoot,
		snapshot.Intent.PlanSHA256,
		lease,
	)
	if err != nil {
		return CurrentSyncResult{}, err
	}
	result.Status = "refreshed"
	result.IsMutation = true
	result.Applied = true
	result.Replay = false
	result.AlreadyCurrent = false
	return result, nil
}

func currentSyncReplayResultLocked(
	caseRoot,
	stateRoot,
	expectedPlanSHA256 string,
	lease *currentSyncRefreshLease,
) (CurrentSyncResult, error) {
	if err := lease.Validate(); err != nil {
		return CurrentSyncResult{}, err
	}
	receipt, exists, err := readCurrentSyncReceipt(stateRoot)
	if err != nil || !exists {
		return CurrentSyncResult{}, errors.Join(
			err,
			fmt.Errorf("current sync durable receipt is unavailable"),
		)
	}
	if !validCurrentSyncSHA(expectedPlanSHA256) ||
		!strings.EqualFold(receipt.PlanSHA256, expectedPlanSHA256) {
		return CurrentSyncResult{}, fmt.Errorf(
			"current sync durable receipt does not match the expected reviewed plan",
		)
	}
	if _, _, err := validateCurrentSyncReplayState(
		caseRoot,
		stateRoot,
		receipt,
	); err != nil {
		return CurrentSyncResult{}, err
	}
	if _, activeExists, err := readCurrentSyncIntent(
		caseRoot,
		stateRoot,
	); err != nil || activeExists {
		return CurrentSyncResult{}, fmt.Errorf(
			"current sync active intent changed before strict replay return: %w",
			err,
		)
	}
	if _, ownerExists, err := readCurrentSyncOwner(
		caseRoot,
		stateRoot,
	); err != nil || ownerExists {
		return CurrentSyncResult{}, fmt.Errorf(
			"current sync transaction owner changed before strict replay return: %w",
			err,
		)
	}
	if err := lease.Validate(); err != nil {
		return CurrentSyncResult{}, err
	}
	currentReceipt, receiptExists, err := readCurrentSyncReceipt(stateRoot)
	if err != nil || !receiptExists ||
		!currentSyncCanonicalEqual(currentReceipt, receipt) {
		return CurrentSyncResult{}, fmt.Errorf(
			"current sync durable receipt changed before strict replay return: %w",
			err,
		)
	}
	return CurrentSyncResult{
		SchemaVersion:     currentSyncSchemaVersion,
		Kind:              currentSyncResultKind,
		Command:           receipt.Command,
		Status:            "already-current",
		CaseRoot:          caseRoot,
		Pack:              receipt.Pack,
		IsMutation:        false,
		Applied:           false,
		Replay:            true,
		AlreadyCurrent:    true,
		PlanSHA256:        receipt.PlanSHA256,
		TransactionSHA256: receipt.TransactionSHA256,
		ReceiptPath:       projectstate.CurrentDir + "/" + currentSyncReceiptRel,
		Receipt:           &receipt,
	}, nil
}
