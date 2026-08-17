package sync

import (
	"strings"
	"testing"
)

func TestCurrentSyncRenameOperationSpecsAreExactAndComplete(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	plan, err := CurrentSyncPreview(
		fixture.repoRoot,
		fixture.caseRoot,
		fixture.pack,
		CurrentSyncOptions{SourceExecutable: fixture.sourceExecutable},
	)
	if err != nil {
		t.Fatal(err)
	}
	intent, err := buildCurrentSyncIntent(plan)
	if err != nil {
		t.Fatal(err)
	}
	progress, err := newCurrentSyncProgress(intent)
	if err != nil {
		t.Fatal(err)
	}
	for !currentSyncReceiptCommitPending(progress) {
		if progress.Pending == nil {
			progress, err = currentSyncBeginProgressOperation(progress, intent)
		} else {
			progress, err = currentSyncCompleteProgressOperation(progress, intent)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	receipt, err := buildCurrentSyncReceipt(intent, progress)
	if err != nil {
		t.Fatal(err)
	}
	receiptBinding, receiptData, err := currentSyncReceiptBinding(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if currentSyncSHA(receiptData) != receiptBinding.SHA256 ||
		int64(len(receiptData)) != receiptBinding.Size ||
		receiptBinding.Path != currentSyncStatePath(currentSyncReceiptRel) {
		t.Fatalf("current sync staged receipt binding = %+v", receiptBinding)
	}

	renameCount := 0
	for _, operation := range currentSyncExpectedProgressOperations(intent) {
		mode, err := currentSyncModeForOperation(operation)
		if err != nil {
			t.Fatal(err)
		}
		if mode != currentSyncOperationRename {
			continue
		}
		renameCount++
		var stagedReceipt *CurrentSyncBinding
		if operation.Kind == "receipt-stage-to-live" {
			stagedReceipt = &receiptBinding
		}
		spec, err := currentSyncRenameOperationSpec(
			intent,
			operation,
			stagedReceipt,
		)
		if err != nil {
			t.Fatalf("current sync operation %+v has no exact spec: %v", operation, err)
		}
		if !currentSyncCanonicalEqual(spec.Operation, operation) ||
			spec.SourcePath == spec.DestinationPath ||
			!currentSyncSafeTransactionPath(intent, spec.SourcePath) ||
			!currentSyncSafeTransactionPath(intent, spec.DestinationPath) {
			t.Fatalf("current sync operation spec is invalid: %+v", spec)
		}
		switch spec.Expected.Kind {
		case currentSyncExpectedFile:
			if spec.Expected.File == nil || spec.Expected.Inventory != nil ||
				!validCurrentSyncSHA(spec.Expected.File.SHA256) ||
				spec.Expected.File.Mode == 0 {
				t.Fatalf("current sync file operation spec is invalid: %+v", spec)
			}
		case currentSyncExpectedTree:
			if spec.Expected.Inventory == nil || spec.Expected.File != nil ||
				spec.Expected.BindingBasePath == "" {
				t.Fatalf("current sync tree operation spec is invalid: %+v", spec)
			}
			if err := validateCurrentSyncInventory(*spec.Expected.Inventory); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatalf("current sync operation spec has unknown object kind: %+v", spec)
		}
	}
	if renameCount == 0 {
		t.Fatal("current sync fixture produced no rename operation specs")
	}
}

func TestCurrentSyncRenameOperationSpecRejectsUnboundInputs(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	plan, err := CurrentSyncPreview(
		fixture.repoRoot,
		fixture.caseRoot,
		fixture.pack,
		CurrentSyncOptions{SourceExecutable: fixture.sourceExecutable},
	)
	if err != nil {
		t.Fatal(err)
	}
	intent, err := buildCurrentSyncIntent(plan)
	if err != nil {
		t.Fatal(err)
	}
	tests := []currentSyncProgressOperation{
		{Kind: "root-live-to-previous", Target: "unknown"},
		{Kind: "leaf-live-to-previous", Target: "unknown"},
		{Kind: "activation-stage-to-live", Target: ".steamai/state.json"},
		{Kind: "receipt-live-to-previous", Target: currentSyncReceiptRel},
		{Kind: "receipt-stage-to-live", Target: currentSyncReceiptRel},
		{Kind: "rollback", Target: "runtime"},
	}
	for _, operation := range tests {
		t.Run(operation.Kind+"-"+strings.ReplaceAll(operation.Target, "/", "-"), func(t *testing.T) {
			if _, err := currentSyncRenameOperationSpec(intent, operation, nil); err == nil {
				t.Fatalf("current sync accepted unbound operation: %+v", operation)
			}
		})
	}
}
