package sync

import "fmt"

type currentSyncForwardDecision string

const (
	currentSyncForwardPublishPending currentSyncForwardDecision = "publish-pending"
	currentSyncForwardReconcile      currentSyncForwardDecision = "reconcile-pending"
	currentSyncForwardTerminal       currentSyncForwardDecision = "terminal"
)

type currentSyncOperationMode string

const (
	currentSyncOperationValidate currentSyncOperationMode = "validate"
	currentSyncOperationRename   currentSyncOperationMode = "rename-no-replace"
)

type currentSyncExactObjectState string

const (
	currentSyncObjectAbsent  currentSyncExactObjectState = "absent"
	currentSyncObjectExact   currentSyncExactObjectState = "exact"
	currentSyncObjectDrifted currentSyncExactObjectState = "drifted"
)

type currentSyncRenameDisposition string

const (
	currentSyncRenameApply    currentSyncRenameDisposition = "apply"
	currentSyncRenameComplete currentSyncRenameDisposition = "complete"
)

func currentSyncForwardState(
	progress currentSyncProgress,
	intent currentSyncIntent,
) (currentSyncForwardDecision, *currentSyncProgressOperation, error) {
	if err := validateCurrentSyncProgress(progress, intent); err != nil {
		return "", nil, err
	}
	expected := currentSyncExpectedProgressOperations(intent)
	if progress.Pending != nil {
		operation := *progress.Pending
		return currentSyncForwardReconcile, &operation, nil
	}
	if len(progress.Completed) == len(expected) {
		return currentSyncForwardTerminal, nil, nil
	}
	if len(progress.Completed) > len(expected) {
		return "", nil, fmt.Errorf("current sync durable progress exceeds the forward operation sequence")
	}
	operation := expected[len(progress.Completed)]
	return currentSyncForwardPublishPending, &operation, nil
}

func currentSyncModeForOperation(
	operation currentSyncProgressOperation,
) (currentSyncOperationMode, error) {
	switch operation.Kind {
	case "root-live-to-previous",
		"root-stage-to-live",
		"leaf-live-to-previous",
		"leaf-stage-to-live",
		"activation-live-to-previous",
		"activation-stage-to-live",
		"receipt-live-to-previous",
		"receipt-stage-to-live":
		return currentSyncOperationRename, nil
	case "stage-validated",
		"ready-to-activate",
		"activated",
		"bundle-validated",
		"receipt-committed":
		return currentSyncOperationValidate, nil
	default:
		return "", fmt.Errorf(
			"current sync operation kind is unsupported: %s",
			operation.Kind,
		)
	}
}

func currentSyncClassifyRename(
	source,
	destination currentSyncExactObjectState,
) (currentSyncRenameDisposition, error) {
	if err := validateCurrentSyncExactObjectState(source); err != nil {
		return "", err
	}
	if err := validateCurrentSyncExactObjectState(destination); err != nil {
		return "", err
	}
	switch {
	case source == currentSyncObjectExact && destination == currentSyncObjectAbsent:
		return currentSyncRenameApply, nil
	case source == currentSyncObjectAbsent && destination == currentSyncObjectExact:
		return currentSyncRenameComplete, nil
	default:
		return "", fmt.Errorf(
			"current sync rename state is neither exact before nor exact after: source=%s destination=%s",
			source,
			destination,
		)
	}
}

func validateCurrentSyncExactObjectState(state currentSyncExactObjectState) error {
	switch state {
	case currentSyncObjectAbsent, currentSyncObjectExact, currentSyncObjectDrifted:
		return nil
	default:
		return fmt.Errorf("current sync exact object state is invalid: %s", state)
	}
}
