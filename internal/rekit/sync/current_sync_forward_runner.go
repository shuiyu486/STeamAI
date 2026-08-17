package sync

import "fmt"

type currentSyncOperationEffect func(
	currentSyncProgress,
	currentSyncProgressOperation,
) error

func runCurrentSyncForward(
	caseRoot string,
	intent currentSyncIntent,
	progress currentSyncProgress,
	effect currentSyncOperationEffect,
) (currentSyncProgress, error) {
	if effect == nil {
		return currentSyncProgress{}, fmt.Errorf(
			"current sync forward operation effect is missing",
		)
	}
	for {
		decision, operation, err := currentSyncForwardState(progress, intent)
		if err != nil {
			return currentSyncProgress{}, err
		}
		switch decision {
		case currentSyncForwardTerminal:
			return progress, nil
		case currentSyncForwardPublishPending:
			next, err := currentSyncBeginProgressOperation(progress, intent)
			if err != nil {
				return currentSyncProgress{}, err
			}
			if _, err := publishCurrentSyncProgress(
				caseRoot,
				intent,
				next,
			); err != nil {
				return currentSyncProgress{}, err
			}
			progress = next
		case currentSyncForwardReconcile:
			if operation == nil || progress.Pending == nil ||
				!currentSyncCanonicalEqual(*operation, *progress.Pending) {
				return currentSyncProgress{}, fmt.Errorf(
					"current sync forward pending operation is ambiguous",
				)
			}
			if err := effect(progress, *operation); err != nil {
				return currentSyncProgress{}, fmt.Errorf(
					"current sync operation %d %s failed: %w",
					operation.Sequence,
					operation.Kind,
					err,
				)
			}
			if err := runCurrentSyncApplyTransitionHook(
				"after-operation-effect:"+operation.Kind,
				intent.Plan,
			); err != nil {
				return currentSyncProgress{}, err
			}
			next, err := currentSyncCompleteProgressOperation(progress, intent)
			if err != nil {
				return currentSyncProgress{}, err
			}
			if _, err := publishCurrentSyncProgress(
				caseRoot,
				intent,
				next,
			); err != nil {
				return currentSyncProgress{}, err
			}
			progress = next
		default:
			return currentSyncProgress{}, fmt.Errorf(
				"current sync forward decision is invalid: %s",
				decision,
			)
		}
	}
}
