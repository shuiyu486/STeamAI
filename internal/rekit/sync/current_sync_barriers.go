package sync

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
)

func executeCurrentSyncValidateOperation(
	caseRoot string,
	intent currentSyncIntent,
	progress currentSyncProgress,
	operation currentSyncProgressOperation,
) error {
	mode, err := currentSyncModeForOperation(operation)
	if err != nil {
		return err
	}
	if mode != currentSyncOperationValidate {
		return fmt.Errorf(
			"current sync operation is not a validation barrier: %s",
			operation.Kind,
		)
	}
	if progress.Pending == nil ||
		!currentSyncCanonicalEqual(*progress.Pending, operation) {
		return fmt.Errorf(
			"current sync validation barrier is not the exact pending operation",
		)
	}
	switch operation.Kind {
	case "stage-validated":
		return validateCurrentSyncStagedState(caseRoot, intent)
	case "ready-to-activate":
		return validateCurrentSyncPreActivationState(caseRoot, intent)
	case "activated":
		return validateCurrentSyncActivatedState(caseRoot, intent)
	case "bundle-validated":
		return validateCurrentSyncLiveNextState(caseRoot, intent)
	case "receipt-committed":
		return validateCurrentSyncReceiptCommit(
			caseRoot,
			intent,
			progress,
		)
	default:
		return fmt.Errorf(
			"current sync validation operation is unsupported: %s",
			operation.Kind,
		)
	}
}

func validateCurrentSyncPreActivationState(
	caseRoot string,
	intent currentSyncIntent,
) (retErr error) {
	root, err := rekitfs.OpenAnchoredRoot(caseRoot)
	if err != nil {
		return err
	}
	defer func() {
		retErr = errors.Join(retErr, root.Close())
	}()
	for _, binding := range intent.Roots {
		inventory := binding.After
		expected := currentSyncExpectedObject{
			Kind:            currentSyncExpectedTree,
			BindingBasePath: currentSyncStatePath(binding.Name),
			Inventory:       &inventory,
		}
		if err := requireCurrentSyncExactObject(
			root,
			currentSyncStatePath(binding.Name),
			expected,
			"pre-activation controlled root",
		); err != nil {
			return err
		}
	}
	for _, leaf := range intent.Leaves {
		before := leaf.ActivateLast && leaf.Mutate
		if err := requireCurrentSyncLeafState(
			root,
			leaf,
			before,
			"pre-activation leaf",
		); err != nil {
			return err
		}
	}
	return root.Validate()
}

func validateCurrentSyncActivatedState(
	caseRoot string,
	intent currentSyncIntent,
) error {
	stateRoot := caseRootStateRoot(caseRoot)
	if err := validateCurrentSyncActiveInstance(
		caseRoot,
		stateRoot,
		currentSyncReceipt{
			Pack:        intent.Plan.Pack,
			ProjectName: intent.Plan.ProjectName,
			Manifest:    intent.Plan.NextManifest,
		},
	); err != nil {
		return err
	}
	for _, leaf := range intent.Leaves {
		if !leaf.ActivateLast {
			continue
		}
		root, err := rekitfs.OpenAnchoredRoot(caseRoot)
		if err != nil {
			return err
		}
		requireErr := requireCurrentSyncLeafState(
			root,
			leaf,
			false,
			"activated leaf",
		)
		closeErr := root.Close()
		if requireErr != nil || closeErr != nil {
			return errors.Join(requireErr, closeErr)
		}
	}
	return nil
}

func validateCurrentSyncLiveNextState(
	caseRoot string,
	intent currentSyncIntent,
) error {
	stateRoot := caseRootStateRoot(caseRoot)
	if err := validateCurrentSyncActivatedState(caseRoot, intent); err != nil {
		return err
	}
	if _, err := runtimebundle.Validate(
		stateRoot,
		runtimebundle.ManifestRel,
		intent.Plan.NextManifest.SHA256,
		intent.Plan.Pack,
	); err != nil {
		return fmt.Errorf("current sync live bundle is invalid: %w", err)
	}
	controlled, err := currentSyncLiveControlledInventory(
		stateRoot,
		intent.Plan.NextControlled,
	)
	if err != nil {
		return err
	}
	if !currentSyncCanonicalEqual(controlled, intent.Plan.NextControlled) {
		return fmt.Errorf(
			"current sync live controlled inventory differs from durable intent",
		)
	}
	targets, err := currentSyncLiveTargetInventory(
		caseRoot,
		intent,
		intent.Plan.NextTargets,
	)
	if err != nil {
		return err
	}
	if !currentSyncCanonicalEqual(targets, intent.Plan.NextTargets) {
		return fmt.Errorf(
			"current sync live target inventory differs from durable intent",
		)
	}
	return nil
}

func validateCurrentSyncReceiptCommit(
	caseRoot string,
	intent currentSyncIntent,
	progress currentSyncProgress,
) error {
	if progress.Pending == nil ||
		progress.Pending.Kind != "receipt-committed" {
		return fmt.Errorf(
			"current sync receipt commit is not the pending barrier",
		)
	}
	stateRoot := caseRootStateRoot(caseRoot)
	receipt, exists, err := readCurrentSyncReceipt(stateRoot)
	if err != nil || !exists {
		return errors.Join(
			err,
			fmt.Errorf("current sync committed receipt is unavailable"),
		)
	}
	history, err := readCurrentSyncProgressHistory(stateRoot, intent)
	if err != nil {
		return err
	}
	var receiptPending currentSyncProgress
	matches := 0
	for _, item := range history {
		if item.Pending == nil || item.Pending.Kind != "receipt-stage-to-live" ||
			!strings.EqualFold(
				item.ProgressSHA256,
				receipt.ProgressSHA256,
			) {
			continue
		}
		receiptPending = item
		matches++
	}
	if matches != 1 {
		return fmt.Errorf(
			"current sync receipt progress lineage is missing or ambiguous",
		)
	}
	if err := validateCurrentSyncReceipt(
		receipt,
		intent,
		receiptPending,
	); err != nil {
		return err
	}
	binding, _, err := currentSyncReceiptBinding(receipt)
	if err != nil {
		return err
	}
	root, err := rekitfs.OpenAnchoredRoot(caseRoot)
	if err != nil {
		return err
	}
	receiptStateErr := requireCurrentSyncExactObject(
		root,
		currentSyncStatePath(currentSyncReceiptRel),
		currentSyncExpectedObject{
			Kind: currentSyncExpectedFile,
			File: &binding,
		},
		"committed receipt",
	)
	closeErr := root.Close()
	if receiptStateErr != nil || closeErr != nil {
		return errors.Join(receiptStateErr, closeErr)
	}
	if err := validateCurrentSyncLiveReceiptState(
		caseRoot,
		stateRoot,
		intent,
		receipt,
	); err != nil {
		return err
	}
	return nil
}

func requireCurrentSyncLeafState(
	root *rekitfs.AnchoredRoot,
	leaf currentSyncIntentLeaf,
	before bool,
	label string,
) error {
	exists := leaf.AfterExists
	sha := leaf.AfterSHA256
	size := leaf.AfterSize
	if before {
		exists = leaf.BeforeExists
		sha = leaf.BeforeSHA256
		size = leaf.BeforeSize
	}
	if !exists {
		if _, err := root.Lstat(leaf.Path); errors.Is(err, fs.ErrNotExist) {
			return nil
		} else if err != nil {
			return err
		}
		return fmt.Errorf("current sync %s must be absent: %s", label, leaf.Path)
	}
	binding := CurrentSyncBinding{
		Path:   leaf.Path,
		Kind:   leaf.Kind,
		SHA256: sha,
		Size:   size,
		Mode:   leaf.Mode,
	}
	return requireCurrentSyncExactObject(
		root,
		leaf.Path,
		currentSyncExpectedObject{
			Kind: currentSyncExpectedFile,
			File: &binding,
		},
		label,
	)
}

func requireCurrentSyncExactObject(
	root *rekitfs.AnchoredRoot,
	path string,
	expected currentSyncExpectedObject,
	label string,
) error {
	observed, err := inspectCurrentSyncExpectedObject(root, path, expected)
	if err != nil {
		return err
	}
	if observed.State != currentSyncObjectExact {
		return fmt.Errorf(
			"current sync %s is not exact: %s: %s",
			label,
			path,
			observed.State,
		)
	}
	return nil
}
