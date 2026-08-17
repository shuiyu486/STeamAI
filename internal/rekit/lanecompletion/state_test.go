package lanecompletion

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectLegacyCompletionAndGeneratedLifecycle(t *testing.T) {
	caseRoot := t.TempDir()
	laneID := "feature-alpha"
	root := filepath.Join(caseRoot, ".rekit", "lanes", laneID)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyIntent := validCompletionIntent(laneID, 1, "")
	legacyReceipt := validCompletionReceipt(t, legacyIntent, 1, "")
	writeJSON(t, filepath.Join(root, IntentFile), legacyIntent)
	writeJSON(t, filepath.Join(root, CommitFile), legacyReceipt)

	legacy, err := Inspect(caseRoot, laneID)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.State != StateComplete || legacy.HeadSequence != 1 || legacy.CurrentCompletion == nil || len(legacy.Transitions) != 1 {
		t.Fatalf("legacy completion was not promoted to sequence 1: %+v", legacy)
	}

	reopenIntent := validReopenIntent(laneID, 2, legacy.HeadReceiptSHA256)
	reopenReceipt := validReopenReceipt(t, reopenIntent)
	writeJSON(t, IntentPath(caseRoot, laneID, 2, "reopen"), reopenIntent)
	writeJSON(t, ReceiptPath(caseRoot, laneID, 2, "reopen"), reopenReceipt)
	reopened, err := Inspect(caseRoot, laneID)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.State != StateReopened || reopened.HeadSequence != 2 || reopened.CurrentReopen == nil || reopened.CurrentCompletion != nil || len(reopened.Transitions) != 2 {
		t.Fatalf("reopen did not supersede current completion: %+v", reopened)
	}

	completeIntent := validCompletionIntent(laneID, 3, reopened.HeadReceiptSHA256)
	completeReceipt := validCompletionReceipt(t, completeIntent, 3, reopened.HeadReceiptSHA256)
	writeJSON(t, IntentPath(caseRoot, laneID, 3, "complete"), completeIntent)
	writeJSON(t, ReceiptPath(caseRoot, laneID, 3, "complete"), completeReceipt)
	completedAgain, err := Inspect(caseRoot, laneID)
	if err != nil {
		t.Fatal(err)
	}
	if completedAgain.State != StateComplete || completedAgain.HeadSequence != 3 || completedAgain.CurrentCompletion == nil || len(completedAgain.Transitions) != 3 {
		t.Fatalf("generated completion did not become current: %+v", completedAgain)
	}
}

func TestInspectRejectsPendingGapAndSymlink(t *testing.T) {
	t.Run("pending", func(t *testing.T) {
		caseRoot, laneID, head := legacyFixture(t)
		intent := validReopenIntent(laneID, 2, head)
		writeJSON(t, IntentPath(caseRoot, laneID, 2, "reopen"), intent)
		inspection, err := Inspect(caseRoot, laneID)
		if err != nil {
			t.Fatal(err)
		}
		if inspection.State != StatePending || inspection.PendingSequence != 2 || inspection.PendingKind != "reopen" {
			t.Fatalf("pending intent not classified: %+v", inspection)
		}
	})
	t.Run("gap", func(t *testing.T) {
		caseRoot, laneID, head := legacyFixture(t)
		intent := validReopenIntent(laneID, 3, head)
		receipt := validReopenReceipt(t, intent)
		writeJSON(t, IntentPath(caseRoot, laneID, 3, "reopen"), intent)
		writeJSON(t, ReceiptPath(caseRoot, laneID, 3, "reopen"), receipt)
		if _, err := Inspect(caseRoot, laneID); err == nil || !strings.Contains(err.Error(), "sequence gap") {
			t.Fatalf("sequence gap was not rejected: %v", err)
		}
	})
	t.Run("symlink", func(t *testing.T) {
		caseRoot, laneID, _ := legacyFixture(t)
		dir := filepath.Join(caseRoot, ".rekit", "lanes", laneID, LifecycleDir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(caseRoot, "outside.json")
		if err := os.WriteFile(target, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(dir, "00000000000000000002.reopen.intent.json")); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if _, err := Inspect(caseRoot, laneID); err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("symlink lifecycle entry was not rejected: %v", err)
		}
	})
}

func TestInspectRejectsLifecycleNamespaceObstructions(t *testing.T) {
	t.Run("lifecycle root symlink", func(t *testing.T) {
		caseRoot, laneID, _ := legacyFixture(t)
		outside := t.TempDir()
		link := filepath.Join(caseRoot, ".rekit", "lanes", laneID, LifecycleDir)
		if err := os.Symlink(outside, link); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if _, err := Inspect(caseRoot, laneID); err == nil || !(strings.Contains(err.Error(), "non-symlink directory") || strings.Contains(err.Error(), "reparse point")) {
			t.Fatalf("lifecycle root symlink was not rejected: %v", err)
		}
	})
	t.Run("intermediate lane symlink", func(t *testing.T) {
		caseRoot := t.TempDir()
		outside := t.TempDir()
		lanes := filepath.Join(caseRoot, ".rekit", "lanes")
		if err := os.MkdirAll(lanes, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(lanes, "main")); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if _, err := Inspect(caseRoot, "main"); err == nil || !(strings.Contains(err.Error(), "non-symlink directory") || strings.Contains(err.Error(), "reparse point")) {
			t.Fatalf("intermediate lane symlink was not rejected: %v", err)
		}
	})
	t.Run("non-directory lifecycle root", func(t *testing.T) {
		caseRoot, laneID, _ := legacyFixture(t)
		path := filepath.Join(caseRoot, ".rekit", "lanes", laneID, LifecycleDir)
		if err := os.WriteFile(path, []byte("obstruction\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Inspect(caseRoot, laneID); err == nil || !(strings.Contains(err.Error(), "non-symlink directory") || strings.Contains(err.Error(), "reparse point")) {
			t.Fatalf("non-directory lifecycle root was not rejected: %v", err)
		}
	})
}

func TestPathsAndInspectionUseCurrentStateRoot(t *testing.T) {
	caseRoot := t.TempDir()
	laneID := "main"
	laneRoot := filepath.Join(caseRoot, ".steamai", "lanes", laneID)
	if err := os.MkdirAll(laneRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	intent := validCompletionIntent(laneID, 1, "")
	receipt := validCompletionReceipt(t, intent, 1, "")
	writeJSON(t, filepath.Join(laneRoot, IntentFile), intent)
	writeJSON(t, filepath.Join(laneRoot, CommitFile), receipt)

	intentPath, err := IntentPathE(caseRoot, laneID, 1, "complete")
	if err != nil {
		t.Fatal(err)
	}
	receiptPath, err := ReceiptPathE(caseRoot, laneID, 1, "complete")
	if err != nil {
		t.Fatal(err)
	}
	if intentPath != filepath.Join(laneRoot, IntentFile) || receiptPath != filepath.Join(laneRoot, CommitFile) {
		t.Fatalf("current lifecycle paths = %s, %s", intentPath, receiptPath)
	}
	inspection, err := Inspect(caseRoot, laneID)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.State != StateComplete || inspection.HeadSequence != 1 {
		t.Fatalf("current completion was not inspected: %+v", inspection)
	}
}

func TestPathsAndInspectionRejectConflictingStateRoots(t *testing.T) {
	caseRoot := t.TempDir()
	for _, root := range []string{".steamai", ".rekit"} {
		if err := os.MkdirAll(filepath.Join(caseRoot, root), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := IntentPathE(caseRoot, "main", 1, "complete"); err == nil {
		t.Fatal("completion intent path accepted conflicting state roots")
	}
	if _, err := OperationIntentPathE(caseRoot, 1); err == nil {
		t.Fatal("operation intent path accepted conflicting state roots")
	}
	if _, err := Inspect(caseRoot, "main"); err == nil {
		t.Fatal("completion inspection accepted conflicting state roots")
	}
	if _, err := InspectOperations(caseRoot); err == nil {
		t.Fatal("operation inspection accepted conflicting state roots")
	}
	for name, path := range map[string]func() string{
		"completion intent":  func() string { return IntentPath(caseRoot, "main", 1, "complete") },
		"completion receipt": func() string { return ReceiptPath(caseRoot, "main", 1, "complete") },
		"operation intent":   func() string { return OperationIntentPath(caseRoot, 1) },
		"operation commit":   func() string { return OperationCommitPath(caseRoot, 1) },
	} {
		t.Run(name+" compatibility panic", func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered == nil {
					t.Fatal("compatibility path helper swallowed conflicting state roots")
				}
			}()
			_ = path()
		})
	}
}

func TestInspectOperationsRejectsNamespaceObstructions(t *testing.T) {
	for _, tc := range []struct {
		name string
		make func(t *testing.T, caseRoot, outside string)
	}{
		{name: "root symlink", make: func(t *testing.T, caseRoot, outside string) {
			if err := os.Symlink(outside, filepath.Join(caseRoot, ".rekit", OperationsDir)); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
		}},
		{name: "sequence symlink", make: func(t *testing.T, caseRoot, outside string) {
			root := filepath.Join(caseRoot, ".rekit", OperationsDir)
			if err := os.MkdirAll(root, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, filepath.Join(root, "00000000000000000001")); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
		}},
		{name: "root file", make: func(t *testing.T, caseRoot, outside string) {
			if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", OperationsDir), []byte("obstruction\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			caseRoot := t.TempDir()
			if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit"), 0o755); err != nil {
				t.Fatal(err)
			}
			outside := t.TempDir()
			tc.make(t, caseRoot, outside)
			if _, err := InspectOperations(caseRoot); err == nil {
				t.Fatalf("operation namespace obstruction %s was not rejected", tc.name)
			}
		})
	}
}

func legacyFixture(t *testing.T) (string, string, string) {
	t.Helper()
	caseRoot := t.TempDir()
	laneID := "main"
	root := filepath.Join(caseRoot, ".rekit", "lanes", laneID)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	intent := validCompletionIntent(laneID, 1, "")
	receipt := validCompletionReceipt(t, intent, 1, "")
	writeJSON(t, filepath.Join(root, IntentFile), intent)
	writeJSON(t, filepath.Join(root, CommitFile), receipt)
	inspection, err := Inspect(caseRoot, laneID)
	if err != nil {
		t.Fatal(err)
	}
	return caseRoot, laneID, inspection.HeadReceiptSHA256
}

func validCompletionIntent(lane string, sequence int, previous string) CompletionIntent {
	return CompletionIntent{SchemaVersion: 1, Kind: "lane-completion-intent", Sequence: sequence, PreviousReceiptSHA: previous, Lane: lane, Label: lane, PreviousStatus: "open", Actor: "main-agent", Reason: "reviewed completion", EvidenceRefs: []string{"evidence.md"}, Evidence: []Evidence{{Ref: "evidence.md", SHA256: strings.Repeat("a", 64), Bytes: 1}}, CreatedAt: "2026-08-03T00:00:00Z", EventID: "event-complete-" + lane, PreviewSHA256: strings.Repeat("b", 64), NoAuthority: true, NoConfirmed: true, NoHeavyTool: true}
}

func validCompletionReceipt(t *testing.T, intent CompletionIntent, sequence int, previous string) CompletionReceipt {
	t.Helper()
	intentSHA, err := canonicalSHA(intent)
	if err != nil {
		t.Fatal(err)
	}
	return CompletionReceipt{SchemaVersion: 1, Kind: "lane-completion", State: "committed", Sequence: sequence, PreviousReceiptSHA: previous, Lane: intent.Lane, Label: intent.Label, PreviousStatus: intent.PreviousStatus, Actor: intent.Actor, Reason: intent.Reason, EvidenceRefs: intent.EvidenceRefs, Evidence: intent.Evidence, CompletedAt: intent.CreatedAt, EventID: intent.EventID, PreviewSHA256: intent.PreviewSHA256, IntentSHA256: intentSHA, LaneSHA256: strings.Repeat("c", 64), BoardLaneSHA256: strings.Repeat("d", 64), ResumeSHA256: strings.Repeat("e", 64), CheckpointSHA256: strings.Repeat("f", 64), NoAuthority: true, NoConfirmed: true, NoHeavyTool: true}
}

func validReopenIntent(lane string, sequence int, previous string) ReopenIntent {
	return ReopenIntent{SchemaVersion: 1, Kind: "lane-reopen-intent", OperationID: "reopen-operation-1", Sequence: sequence, PreviousReceiptSHA: previous, SupersededCompletionSequence: sequence - 1, SupersededCompletionSHA256: previous, Lane: lane, Label: lane, PreviousStatus: "closed", Actor: "main-agent", Reason: "new work invalidates completion", EvidenceRefs: []string{"reopen.md"}, Evidence: []Evidence{{Ref: "reopen.md", SHA256: strings.Repeat("1", 64), Bytes: 1}}, PreviousExecutorGeneration: 1, ResultingExecutorGeneration: 2, CreatedAt: "2026-08-03T00:01:00Z", EventID: "event-reopen-" + lane, PreviewSHA256: strings.Repeat("2", 64), NoAuthority: true, NoConfirmed: true, NoHeavyTool: true, NoAutoResume: true}
}

func validReopenReceipt(t *testing.T, intent ReopenIntent) ReopenReceipt {
	t.Helper()
	intentSHA, err := canonicalSHA(intent)
	if err != nil {
		t.Fatal(err)
	}
	return ReopenReceipt{SchemaVersion: 1, Kind: "lane-reopen", State: "committed", OperationID: intent.OperationID, Sequence: intent.Sequence, PreviousReceiptSHA: intent.PreviousReceiptSHA, SupersededCompletionSequence: intent.SupersededCompletionSequence, SupersededCompletionSHA256: intent.SupersededCompletionSHA256, Lane: intent.Lane, Label: intent.Label, PreviousStatus: intent.PreviousStatus, Actor: intent.Actor, Reason: intent.Reason, EvidenceRefs: intent.EvidenceRefs, Evidence: intent.Evidence, PreviousExecutorGeneration: intent.PreviousExecutorGeneration, ResultingExecutorGeneration: intent.ResultingExecutorGeneration, ReopenedAt: intent.CreatedAt, EventID: intent.EventID, PreviewSHA256: intent.PreviewSHA256, IntentSHA256: intentSHA, LaneSHA256: strings.Repeat("3", 64), BoardLaneSHA256: strings.Repeat("4", 64), ResumeSHA256: strings.Repeat("5", 64), CheckpointSHA256: strings.Repeat("6", 64), NoAuthority: true, NoConfirmed: true, NoHeavyTool: true, NoAutoResume: true}
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}
