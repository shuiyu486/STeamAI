package sync

import "testing"

func TestCurrentSyncForwardStateUsesOnlyDurableProgress(t *testing.T) {
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

	decision, operation, err := currentSyncForwardState(progress, intent)
	if err != nil {
		t.Fatal(err)
	}
	if decision != currentSyncForwardPublishPending || operation == nil || operation.Kind != "stage-validated" {
		t.Fatalf("current sync initial forward state = %s %+v", decision, operation)
	}

	progress, err = currentSyncBeginProgressOperation(progress, intent)
	if err != nil {
		t.Fatal(err)
	}
	decision, operation, err = currentSyncForwardState(progress, intent)
	if err != nil {
		t.Fatal(err)
	}
	if decision != currentSyncForwardReconcile || operation == nil || operation.Kind != "stage-validated" {
		t.Fatalf("current sync pending forward state = %s %+v", decision, operation)
	}

	progress, err = currentSyncCompleteProgressOperation(progress, intent)
	if err != nil {
		t.Fatal(err)
	}
	decision, operation, err = currentSyncForwardState(progress, intent)
	if err != nil {
		t.Fatal(err)
	}
	expected := currentSyncExpectedProgressOperations(intent)
	if decision != currentSyncForwardPublishPending || operation == nil || !currentSyncCanonicalEqual(*operation, expected[1]) {
		t.Fatalf("current sync next forward state = %s %+v", decision, operation)
	}

	for len(progress.Completed) < len(expected) {
		progress, err = currentSyncBeginProgressOperation(progress, intent)
		if err != nil {
			t.Fatal(err)
		}
		progress, err = currentSyncCompleteProgressOperation(progress, intent)
		if err != nil {
			t.Fatal(err)
		}
	}
	decision, operation, err = currentSyncForwardState(progress, intent)
	if err != nil {
		t.Fatal(err)
	}
	if decision != currentSyncForwardTerminal || operation != nil {
		t.Fatalf("current sync terminal forward state = %s %+v", decision, operation)
	}
}

func TestCurrentSyncOperationModesAreClosed(t *testing.T) {
	for _, kind := range []string{
		"root-live-to-previous",
		"root-stage-to-live",
		"leaf-live-to-previous",
		"leaf-stage-to-live",
		"activation-live-to-previous",
		"activation-stage-to-live",
		"receipt-live-to-previous",
		"receipt-stage-to-live",
	} {
		mode, err := currentSyncModeForOperation(currentSyncProgressOperation{Kind: kind})
		if err != nil || mode != currentSyncOperationRename {
			t.Fatalf("current sync operation %s mode = %s err=%v", kind, mode, err)
		}
	}
	for _, kind := range []string{
		"stage-validated",
		"ready-to-activate",
		"activated",
		"bundle-validated",
		"receipt-committed",
	} {
		mode, err := currentSyncModeForOperation(currentSyncProgressOperation{Kind: kind})
		if err != nil || mode != currentSyncOperationValidate {
			t.Fatalf("current sync operation %s mode = %s err=%v", kind, mode, err)
		}
	}
	if _, err := currentSyncModeForOperation(currentSyncProgressOperation{Kind: "rollback"}); err == nil {
		t.Fatal("current sync accepted an operation outside the forward-only sequence")
	}
}

func TestCurrentSyncRenameClassifierAcceptsOnlyExactBeforeOrAfter(t *testing.T) {
	tests := []struct {
		name        string
		source      currentSyncExactObjectState
		destination currentSyncExactObjectState
		want        currentSyncRenameDisposition
		wantError   bool
	}{
		{
			name:        "exact-before",
			source:      currentSyncObjectExact,
			destination: currentSyncObjectAbsent,
			want:        currentSyncRenameApply,
		},
		{
			name:        "exact-after",
			source:      currentSyncObjectAbsent,
			destination: currentSyncObjectExact,
			want:        currentSyncRenameComplete,
		},
		{
			name:        "both-absent",
			source:      currentSyncObjectAbsent,
			destination: currentSyncObjectAbsent,
			wantError:   true,
		},
		{
			name:        "both-exact",
			source:      currentSyncObjectExact,
			destination: currentSyncObjectExact,
			wantError:   true,
		},
		{
			name:        "source-drift",
			source:      currentSyncObjectDrifted,
			destination: currentSyncObjectAbsent,
			wantError:   true,
		},
		{
			name:        "destination-drift",
			source:      currentSyncObjectAbsent,
			destination: currentSyncObjectDrifted,
			wantError:   true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := currentSyncClassifyRename(test.source, test.destination)
			if test.wantError {
				if err == nil {
					t.Fatalf("current sync rename classifier accepted %s/%s as %s", test.source, test.destination, got)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("current sync rename classifier = %s err=%v, want %s", got, err, test.want)
			}
		})
	}
}
