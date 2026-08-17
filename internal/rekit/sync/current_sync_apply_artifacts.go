package sync

import (
	"fmt"
	"path/filepath"
	"strings"
)

type currentSyncApplySnapshot struct {
	Route    currentSyncApplyRoute
	Intent   currentSyncIntent
	Progress currentSyncProgress
	Receipt  currentSyncReceipt
}

func readCurrentSyncApplySnapshot(
	caseRoot,
	stateRoot,
	expectedPlanSHA256 string,
) (currentSyncApplySnapshot, error) {
	if !validCurrentSyncSHA(expectedPlanSHA256) {
		return currentSyncApplySnapshot{}, fmt.Errorf(
			"current sync Apply requires the exact reviewed plan SHA-256",
		)
	}
	pending, err := inspectCurrentSyncPendingOwnership(caseRoot, stateRoot)
	if err != nil {
		return currentSyncApplySnapshot{}, err
	}
	owner, ownerExists, err := readCurrentSyncOwner(caseRoot, stateRoot)
	if err != nil {
		return currentSyncApplySnapshot{}, err
	}
	active, activeExists, err := readCurrentSyncIntent(caseRoot, stateRoot)
	if err != nil {
		return currentSyncApplySnapshot{}, err
	}
	if activeExists && !strings.EqualFold(
		active.PlanSHA256,
		expectedPlanSHA256,
	) {
		return currentSyncApplySnapshot{}, fmt.Errorf(
			"current sync active intent does not match the expected reviewed plan",
		)
	}
	if ownerExists && !strings.EqualFold(
		owner.PlanSHA256,
		expectedPlanSHA256,
	) {
		return currentSyncApplySnapshot{}, fmt.Errorf(
			"current sync transaction owner does not match the expected reviewed plan",
		)
	}
	if pending.Exists && !strings.EqualFold(
		pending.Intent.PlanSHA256,
		expectedPlanSHA256,
	) {
		return currentSyncApplySnapshot{}, fmt.Errorf(
			"current sync pending transaction does not match the expected reviewed plan",
		)
	}
	transactionPath := currentSyncTransactionPath(expectedPlanSHA256)
	archived, archivedExists, err := readOptionalCurrentSyncArchivedIntent(
		stateRoot,
		transactionPath,
	)
	if err != nil {
		return currentSyncApplySnapshot{}, err
	}
	if activeExists && archivedExists &&
		!currentSyncCanonicalEqual(active, archived) {
		return currentSyncApplySnapshot{}, fmt.Errorf(
			"current sync active intent differs from archived transaction",
		)
	}
	if ownerExists && activeExists &&
		!currentSyncCanonicalEqual(owner.Intent, active) {
		return currentSyncApplySnapshot{}, fmt.Errorf(
			"current sync transaction owner differs from active intent",
		)
	}
	if ownerExists && archivedExists &&
		!currentSyncCanonicalEqual(owner.Intent, archived) {
		return currentSyncApplySnapshot{}, fmt.Errorf(
			"current sync transaction owner differs from archived transaction",
		)
	}
	if !archivedExists && (ownerExists || activeExists) {
		intent := active
		if ownerExists {
			intent = owner.Intent
		}
		return currentSyncApplySnapshot{
			Route:  currentSyncApplyRestoreActive,
			Intent: intent,
		}, nil
	}
	intent := archived
	if activeExists {
		intent = active
	} else if ownerExists {
		intent = owner.Intent
	}
	var progress currentSyncProgress
	progressExists := false
	progressTerminal := false
	if archivedExists {
		progress, progressExists, err = readCurrentSyncProgress(
			stateRoot,
			archived,
		)
		if err != nil {
			return currentSyncApplySnapshot{}, err
		}
		if progressExists {
			progressTerminal = currentSyncProgressTerminal(progress, archived)
		}
	}
	receipt, receiptExists, err := readCurrentSyncReceipt(stateRoot)
	if err != nil {
		return currentSyncApplySnapshot{}, err
	}
	matchingReceipt := receiptExists &&
		strings.EqualFold(receipt.PlanSHA256, expectedPlanSHA256) &&
		archivedExists &&
		strings.EqualFold(
			receipt.TransactionSHA256,
			archived.TransactionSHA256,
		)
	predecessorReceipt := receiptExists && archivedExists &&
		currentSyncReceiptMatchesBinding(
			receipt,
			archived.Plan.PreviousReceipt,
		)
	if receiptExists && !matchingReceipt && !predecessorReceipt &&
		(activeExists || archivedExists || progressExists) {
		return currentSyncApplySnapshot{}, fmt.Errorf(
			"current sync durable receipt conflicts with the expected transaction",
		)
	}
	route, err := currentSyncSelectApplyRoute(currentSyncApplyArtifacts{
		ActiveIntent:     activeExists || ownerExists,
		ArchivedIntent:   archivedExists,
		Progress:         progressExists,
		ProgressTerminal: progressTerminal,
		MatchingReceipt:  matchingReceipt,
	})
	if err != nil {
		return currentSyncApplySnapshot{}, err
	}
	return currentSyncApplySnapshot{
		Route:    route,
		Intent:   intent,
		Progress: progress,
		Receipt:  receipt,
	}, nil
}

func currentSyncReceiptMatchesBinding(
	receipt currentSyncReceipt,
	binding *CurrentSyncBinding,
) bool {
	if binding == nil {
		return false
	}
	data, err := currentSyncCanonicalData(receipt)
	return err == nil &&
		binding.Path == currentSyncStatePath(currentSyncReceiptRel) &&
		binding.Kind == "current-sync-receipt" &&
		strings.EqualFold(binding.SHA256, currentSyncSHA(data)) &&
		binding.Size == int64(len(data)) &&
		binding.Mode == 0o600
}

func readOptionalCurrentSyncArchivedIntent(
	stateRoot,
	transactionPath string,
) (currentSyncIntent, bool, error) {
	clean, err := currentSyncNormalizeTargetRel(transactionPath)
	if err != nil || clean != transactionPath ||
		!strings.HasPrefix(transactionPath, currentSyncTransactionsRel+"/") {
		return currentSyncIntent{}, false, fmt.Errorf(
			"current sync archived intent transaction path is invalid",
		)
	}
	path := filepath.Join(
		stateRoot,
		filepath.FromSlash(transactionPath),
		currentSyncArchivedIntent,
	)
	data, exists, err := currentSyncReadOptional(
		stateRoot,
		path,
		"current sync archived intent",
		currentSyncMaxTransactionBytes,
	)
	if err != nil || !exists {
		return currentSyncIntent{}, exists, err
	}
	var intent currentSyncIntent
	if err := decodeCurrentSyncCanonical(
		data,
		&intent,
		"archived intent",
	); err != nil {
		return currentSyncIntent{}, false, err
	}
	if err := validateCurrentSyncIntentStructure(intent); err != nil {
		return currentSyncIntent{}, false, err
	}
	if intent.TransactionPath != transactionPath {
		return currentSyncIntent{}, false, fmt.Errorf(
			"current sync archived intent transaction binding is invalid",
		)
	}
	return intent, true, nil
}

func currentSyncProgressTerminal(
	progress currentSyncProgress,
	intent currentSyncIntent,
) bool {
	return progress.Pending == nil &&
		len(progress.Completed) == len(currentSyncExpectedProgressOperations(intent)) &&
		progress.Phase == "receipt-committed"
}
