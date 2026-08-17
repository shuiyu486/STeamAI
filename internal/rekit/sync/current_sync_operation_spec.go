package sync

import (
	"fmt"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

type currentSyncExpectedObjectKind string

const (
	currentSyncExpectedFile currentSyncExpectedObjectKind = "file"
	currentSyncExpectedTree currentSyncExpectedObjectKind = "tree"
)

type currentSyncExpectedObject struct {
	Kind            currentSyncExpectedObjectKind
	BindingBasePath string
	File            *CurrentSyncBinding
	Inventory       *CurrentSyncInventory
}

type currentSyncRenameSpec struct {
	Operation       currentSyncProgressOperation
	SourcePath      string
	DestinationPath string
	Expected        currentSyncExpectedObject
}

func currentSyncRenameOperationSpec(
	intent currentSyncIntent,
	operation currentSyncProgressOperation,
	stagedReceipt *CurrentSyncBinding,
) (currentSyncRenameSpec, error) {
	if err := validateCurrentSyncIntentStructure(intent); err != nil {
		return currentSyncRenameSpec{}, err
	}
	mode, err := currentSyncModeForOperation(operation)
	if err != nil {
		return currentSyncRenameSpec{}, err
	}
	if mode != currentSyncOperationRename {
		return currentSyncRenameSpec{}, fmt.Errorf(
			"current sync operation is not a rename: %s",
			operation.Kind,
		)
	}
	spec := currentSyncRenameSpec{Operation: operation}
	switch operation.Kind {
	case "root-live-to-previous", "root-stage-to-live":
		root, ok := currentSyncIntentRootForOperation(intent, operation)
		if !ok || !root.Mutate {
			return currentSyncRenameSpec{}, fmt.Errorf(
				"current sync rename root is not bound by durable intent: %s",
				operation.Target,
			)
		}
		inventory := root.Before
		if operation.Kind == "root-live-to-previous" {
			spec.SourcePath = currentSyncStatePath(root.Name)
			spec.DestinationPath = currentSyncStatePath(root.PreviousPath)
		} else {
			inventory = root.After
			spec.SourcePath = currentSyncStatePath(root.StagePath)
			spec.DestinationPath = currentSyncStatePath(root.Name)
		}
		spec.Expected = currentSyncExpectedObject{
			Kind:            currentSyncExpectedTree,
			BindingBasePath: currentSyncStatePath(root.Name),
			Inventory:       &inventory,
		}
	case "leaf-live-to-previous",
		"leaf-stage-to-live",
		"activation-live-to-previous",
		"activation-stage-to-live":
		leaf, ok := currentSyncIntentLeafForOperation(intent, operation)
		if !ok || !leaf.Mutate {
			return currentSyncRenameSpec{}, fmt.Errorf(
				"current sync rename leaf is not bound by durable intent: %s",
				operation.Target,
			)
		}
		binding := CurrentSyncBinding{
			Path:   leaf.Path,
			Kind:   leaf.Kind,
			SHA256: leaf.BeforeSHA256,
			Size:   leaf.BeforeSize,
			Mode:   leaf.Mode,
		}
		if strings.HasSuffix(operation.Kind, "live-to-previous") {
			if !leaf.BeforeExists {
				return currentSyncRenameSpec{}, fmt.Errorf(
					"current sync rename leaf has no durable before object: %s",
					leaf.Path,
				)
			}
			spec.SourcePath = leaf.Path
			spec.DestinationPath = currentSyncStatePath(leaf.PreviousPath)
		} else {
			if !leaf.AfterExists {
				return currentSyncRenameSpec{}, fmt.Errorf(
					"current sync rename leaf has no durable after object: %s",
					leaf.Path,
				)
			}
			binding.SHA256 = leaf.AfterSHA256
			binding.Size = leaf.AfterSize
			spec.SourcePath = currentSyncStatePath(leaf.StagePath)
			spec.DestinationPath = leaf.Path
		}
		spec.Expected = currentSyncExpectedObject{
			Kind: currentSyncExpectedFile,
			File: &binding,
		}
	case "receipt-live-to-previous":
		if operation.Target != currentSyncReceiptRel ||
			intent.Plan.PreviousReceipt == nil {
			return currentSyncRenameSpec{}, fmt.Errorf(
				"current sync previous receipt rename is not bound by durable intent",
			)
		}
		binding := *intent.Plan.PreviousReceipt
		spec.SourcePath = currentSyncStatePath(currentSyncReceiptRel)
		spec.DestinationPath = currentSyncStatePath(
			currentSyncPreviousReceiptPath(intent),
		)
		spec.Expected = currentSyncExpectedObject{
			Kind: currentSyncExpectedFile,
			File: &binding,
		}
	case "receipt-stage-to-live":
		if operation.Target != currentSyncReceiptRel || stagedReceipt == nil {
			return currentSyncRenameSpec{}, fmt.Errorf(
				"current sync staged receipt rename is missing its exact binding",
			)
		}
		binding := *stagedReceipt
		if err := validateCurrentSyncBinding(
			binding,
			"current-sync-receipt",
		); err != nil || binding.Path != currentSyncStatePath(currentSyncReceiptRel) {
			return currentSyncRenameSpec{}, fmt.Errorf(
				"current sync staged receipt binding is invalid",
			)
		}
		spec.SourcePath = currentSyncStatePath(
			currentSyncStagedReceiptPath(intent),
		)
		spec.DestinationPath = currentSyncStatePath(currentSyncReceiptRel)
		spec.Expected = currentSyncExpectedObject{
			Kind: currentSyncExpectedFile,
			File: &binding,
		}
	default:
		return currentSyncRenameSpec{}, fmt.Errorf(
			"current sync rename operation is unsupported: %s",
			operation.Kind,
		)
	}
	if spec.SourcePath == spec.DestinationPath ||
		!currentSyncSafeTransactionPath(intent, spec.SourcePath) ||
		!currentSyncSafeTransactionPath(intent, spec.DestinationPath) {
		return currentSyncRenameSpec{}, fmt.Errorf(
			"current sync rename paths are invalid: %s -> %s",
			spec.SourcePath,
			spec.DestinationPath,
		)
	}
	return spec, nil
}

func currentSyncSafeTransactionPath(
	intent currentSyncIntent,
	path string,
) bool {
	clean, err := currentSyncNormalizeTargetRel(path)
	if err != nil || clean != path {
		return false
	}
	if currentSyncSafeTargetRel(clean) ||
		clean == projectstate.CurrentDir+"/instance.yml" ||
		clean == projectstate.CurrentDir+"/state.json" ||
		clean == currentSyncStatePath(currentSyncReceiptRel) {
		return true
	}
	for _, root := range currentSyncControlledRoots {
		if clean == currentSyncStatePath(root) {
			return true
		}
	}
	transactionRoot := currentSyncStatePath(intent.TransactionPath)
	return clean != transactionRoot && strings.HasPrefix(clean, transactionRoot+"/")
}

func currentSyncIntentRootForOperation(
	intent currentSyncIntent,
	operation currentSyncProgressOperation,
) (currentSyncIntentRoot, bool) {
	for _, root := range intent.Roots {
		if root.Name == operation.Target {
			return root, true
		}
	}
	return currentSyncIntentRoot{}, false
}

func currentSyncIntentLeafForOperation(
	intent currentSyncIntent,
	operation currentSyncProgressOperation,
) (currentSyncIntentLeaf, bool) {
	activation := strings.HasPrefix(operation.Kind, "activation-")
	for _, leaf := range intent.Leaves {
		if leaf.Path == operation.Target && leaf.ActivateLast == activation {
			return leaf, true
		}
	}
	return currentSyncIntentLeaf{}, false
}

func currentSyncStatePath(rel string) string {
	return projectstate.CurrentDir + "/" + rel
}

func currentSyncPreviousReceiptPath(intent currentSyncIntent) string {
	return intent.TransactionPath + "/previous/receipt.json"
}

func currentSyncStagedReceiptPath(intent currentSyncIntent) string {
	return intent.TransactionPath + "/stage/receipt.json"
}

func currentSyncReceiptBinding(
	receipt currentSyncReceipt,
) (CurrentSyncBinding, []byte, error) {
	data, err := currentSyncCanonicalData(receipt)
	if err != nil {
		return CurrentSyncBinding{}, nil, err
	}
	return currentSyncBinding(
		currentSyncStatePath(currentSyncReceiptRel),
		"current-sync-receipt",
		data,
		0o600,
	), data, nil
}
