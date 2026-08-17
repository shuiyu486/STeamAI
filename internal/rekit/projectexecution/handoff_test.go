package projectexecution

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func handoffFixture(t *testing.T, caseRoot string) Handoff {
	t.Helper()
	handoff, err := NewHandoff(
		caseRoot,
		strings.Repeat("a", 64),
		strings.Repeat("b", 64),
		"supervisor-session",
	)
	if err != nil {
		t.Fatal(err)
	}
	return handoff
}

func requireDurableHandoffForTest(t *testing.T) {
	t.Helper()
	if !DurableHandoffSupported() {
		t.Skip("durable supervisor handoff requires handle-bound exact filesystem mutation")
	}
}

func TestDurableHandoffRejectsUnsupportedMutationBeforePublicationOrCancellation(t *testing.T) {
	caseRoot := currentProjectFixture(t)
	handoff := handoffFixture(t, caseRoot)
	previousSupport := handoffHandleBoundExactMutationSupported
	t.Cleanup(func() { handoffHandleBoundExactMutationSupported = previousSupport })

	handoffHandleBoundExactMutationSupported = func() bool { return false }
	if err := PublishHandoff(caseRoot, handoff); err == nil ||
		!strings.Contains(err.Error(), "requires handle-bound exact filesystem mutation support") {
		t.Fatalf("unsupported handoff publication error=%v", err)
	}
	rootPath, key, err := handoffRoot(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	pendingPath := filepath.Join(rootPath, handoffPendingName(key))
	if _, err := os.Lstat(pendingPath); !os.IsNotExist(err) {
		t.Fatalf("unsupported publication wrote pending handoff: %v", err)
	}

	pendingBefore, err := handoffData(handoff)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pendingPath, pendingBefore, 0o600); err != nil {
		t.Fatal(err)
	}
	pendingBefore, err = os.ReadFile(pendingPath)
	if err != nil {
		t.Fatal(err)
	}
	handoffHandleBoundExactMutationSupported = func() bool { return false }
	for name, operation := range map[string]func() error{
		"claim":  func() error { return ClaimHandoff(caseRoot, handoff) },
		"cancel": func() error { return CancelHandoff(caseRoot, handoff) },
		"cancel-pending": func() error {
			_, _, err := CancelPendingHandoff(caseRoot)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := operation()
			if err == nil || !strings.Contains(err.Error(), "requires handle-bound exact filesystem mutation support") {
				t.Fatalf("unsupported handoff %s error=%v", name, err)
			}
			pendingAfter, readErr := os.ReadFile(pendingPath)
			if readErr != nil || string(pendingAfter) != string(pendingBefore) {
				t.Fatalf("unsupported handoff %s changed pending marker: %q err=%v", name, pendingAfter, readErr)
			}
			if _, statErr := os.Lstat(filepath.Join(rootPath, handoffCancellationName(handoff))); !os.IsNotExist(statErr) {
				t.Fatalf("unsupported handoff %s published cancellation: %v", name, statErr)
			}
		})
	}

	if err := os.Remove(pendingPath); err != nil {
		t.Fatal(err)
	}
}

func TestDurableHandoffMissingRemainsReadOnlyWhenMutationIsUnsupported(t *testing.T) {
	caseRoot := currentProjectFixture(t)
	handoff := handoffFixture(t, caseRoot)
	previousSupport := handoffHandleBoundExactMutationSupported
	t.Cleanup(func() { handoffHandleBoundExactMutationSupported = previousSupport })
	handoffHandleBoundExactMutationSupported = func() bool { return false }

	rootPath, key, err := handoffRoot(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	pendingPath := filepath.Join(rootPath, handoffPendingName(key))
	cancellationPath := filepath.Join(rootPath, handoffCancellationName(handoff))

	if err := ClaimHandoff(caseRoot, handoff); err == nil ||
		!strings.Contains(err.Error(), "no longer pending") ||
		strings.Contains(err.Error(), "requires handle-bound exact filesystem mutation support") {
		t.Fatalf("unsupported missing handoff claim error=%v", err)
	}
	if err := CancelHandoff(caseRoot, handoff); err != nil {
		t.Fatalf("unsupported missing handoff cancel error=%v", err)
	}
	if canceled, found, err := CancelPendingHandoff(caseRoot); err != nil ||
		found || canceled != (Handoff{}) {
		t.Fatalf(
			"unsupported missing pending cancel handoff=%+v found=%t err=%v",
			canceled,
			found,
			err,
		)
	}
	for label, path := range map[string]string{
		"pending marker":       pendingPath,
		"cancellation receipt": cancellationPath,
	} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("unsupported missing handoff wrote %s: %v", label, err)
		}
	}
}

func TestDurableHandoffDifferentBindingRemainsReadOnlyWhenMutationIsUnsupported(t *testing.T) {
	caseRoot := currentProjectFixture(t)
	handoff := handoffFixture(t, caseRoot)
	previousSupport := handoffHandleBoundExactMutationSupported
	t.Cleanup(func() { handoffHandleBoundExactMutationSupported = previousSupport })
	handoffHandleBoundExactMutationSupported = func() bool { return false }

	rootPath, key, err := handoffRoot(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	pendingPath := filepath.Join(rootPath, handoffPendingName(key))
	pendingBefore, err := handoffData(handoff)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pendingPath, pendingBefore, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(pendingPath) })

	drifted := handoff
	drifted.SessionID = "different-session"
	if err := ClaimHandoff(caseRoot, drifted); err == nil ||
		!strings.Contains(err.Error(), "intent changed") ||
		strings.Contains(err.Error(), "requires handle-bound exact filesystem mutation support") {
		t.Fatalf("unsupported different-binding claim error=%v", err)
	}
	if err := CancelHandoff(caseRoot, drifted); err != nil {
		t.Fatalf("unsupported different-binding cancel error=%v", err)
	}
	pendingAfter, err := os.ReadFile(pendingPath)
	if err != nil || string(pendingAfter) != string(pendingBefore) {
		t.Fatalf(
			"unsupported different-binding operation changed pending marker: %q err=%v",
			pendingAfter,
			err,
		)
	}
	for label, candidate := range map[string]Handoff{
		"actual":  handoff,
		"drifted": drifted,
	} {
		if _, err := os.Lstat(filepath.Join(
			rootPath,
			handoffCancellationName(candidate),
		)); !os.IsNotExist(err) {
			t.Fatalf(
				"unsupported different-binding operation wrote %s cancellation: %v",
				label,
				err,
			)
		}
	}
}

func TestExclusiveLeaseConsumesPendingSupervisorHandoff(t *testing.T) {
	requireDurableHandoffForTest(t)
	caseRoot := currentProjectFixture(t)
	handoff := handoffFixture(t, caseRoot)
	if err := PublishHandoff(caseRoot, handoff); err != nil {
		t.Fatal(err)
	}

	exclusive, err := AcquireExclusive(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer exclusive.Unlock()
	if err := ClaimHandoff(caseRoot, handoff); err == nil ||
		!strings.Contains(err.Error(), "permanently canceled") {
		t.Fatalf("canceled child claim error = %v", err)
	}
	if err := PublishHandoff(caseRoot, handoff); err == nil ||
		!strings.Contains(err.Error(), "permanently canceled") {
		t.Fatalf("canceled handoff ABA replay error = %v", err)
	}
}

func TestCancelHandoffReplaysCanceledReceiptBeforePendingCleanup(t *testing.T) {
	requireDurableHandoffForTest(t)
	caseRoot := currentProjectFixture(t)
	handoff := handoffFixture(t, caseRoot)
	if err := PublishHandoff(caseRoot, handoff); err != nil {
		t.Fatal(err)
	}
	rootPath, key, err := handoffRoot(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := publishHandoffCancellation(rootPath, handoff); err != nil {
		t.Fatal(err)
	}
	if err := CancelHandoff(caseRoot, handoff); err != nil {
		t.Fatal(err)
	}
	if err := CancelHandoff(caseRoot, handoff); err != nil {
		t.Fatalf("canceled handoff cleanup replay failed: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(rootPath, handoffPendingName(key))); !os.IsNotExist(err) {
		t.Fatalf("canceled handoff cleanup replay left pending intent: %v", err)
	}
}

func TestExclusiveLeaseReplaysCanceledReceiptBeforePendingCleanup(t *testing.T) {
	requireDurableHandoffForTest(t)
	caseRoot := currentProjectFixture(t)
	handoff := handoffFixture(t, caseRoot)
	if err := PublishHandoff(caseRoot, handoff); err != nil {
		t.Fatal(err)
	}
	rootPath, key, err := handoffRoot(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := publishHandoffCancellation(rootPath, handoff); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(rootPath, handoffPendingName(key))); err != nil {
		t.Fatal(err)
	}

	exclusive, err := AcquireExclusive(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer exclusive.Unlock()
	if _, err := os.Lstat(filepath.Join(rootPath, handoffPendingName(key))); !os.IsNotExist(err) {
		t.Fatalf("exclusive replay left canceled pending handoff: %v", err)
	}
	if err := ClaimHandoff(caseRoot, handoff); err == nil ||
		!strings.Contains(err.Error(), "permanently canceled") {
		t.Fatalf("canceled receipt did not permanently reject child: %v", err)
	}
}

func TestSupervisorHandoffCancellationCapacityFailsClosed(t *testing.T) {
	requireDurableHandoffForTest(t)
	caseRoot := currentProjectFixture(t)
	handoff := handoffFixture(t, caseRoot)
	rootPath, _, err := handoffRoot(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Mkdir(handoffCancellationDir(handoff.ProjectKey), 0o700); err != nil {
		_ = root.Close()
		t.Fatal(err)
	}
	directory, err := root.OpenRoot(handoffCancellationDir(handoff.ProjectKey))
	if err != nil {
		_ = root.Close()
		t.Fatal(err)
	}
	for index := range handoffCancellationMax {
		file, err := directory.OpenFile(
			fmt.Sprintf("%064x.json", index),
			os.O_CREATE|os.O_EXCL|os.O_WRONLY,
			0o600,
		)
		if err != nil {
			_ = directory.Close()
			_ = root.Close()
			t.Fatal(err)
		}
		if _, err := file.Write([]byte("{}\n")); err != nil {
			_ = file.Close()
			_ = directory.Close()
			_ = root.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			_ = directory.Close()
			_ = root.Close()
			t.Fatal(err)
		}
	}
	if err := directory.Close(); err != nil {
		_ = root.Close()
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if err := PublishHandoff(caseRoot, handoff); err == nil ||
		!strings.Contains(err.Error(), "fail-closed limit") {
		t.Fatalf("full cancellation history publish error = %v", err)
	}
}

func TestSharedLeaseCanClaimExactSupervisorHandoff(t *testing.T) {
	requireDurableHandoffForTest(t)
	caseRoot := currentProjectFixture(t)
	handoff := handoffFixture(t, caseRoot)
	if err := PublishHandoff(caseRoot, handoff); err != nil {
		t.Fatal(err)
	}

	shared, err := AcquireShared(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer shared.Unlock()
	if err := ClaimHandoff(caseRoot, handoff); err != nil {
		t.Fatal(err)
	}
	if err := ClaimHandoff(caseRoot, handoff); err == nil ||
		!strings.Contains(err.Error(), "no longer pending") {
		t.Fatalf("replayed child claim error = %v", err)
	}
}

func TestSupervisorHandoffRejectsDifferentBinding(t *testing.T) {
	requireDurableHandoffForTest(t)
	caseRoot := currentProjectFixture(t)
	handoff := handoffFixture(t, caseRoot)
	if err := PublishHandoff(caseRoot, handoff); err != nil {
		t.Fatal(err)
	}
	defer CancelHandoff(caseRoot, handoff)

	drifted := handoff
	drifted.SessionID = "different-session"
	if err := ClaimHandoff(caseRoot, drifted); err == nil ||
		!strings.Contains(err.Error(), "intent changed") {
		t.Fatalf("drifted child claim error = %v", err)
	}
	if err := ClaimHandoff(caseRoot, handoff); err != nil {
		t.Fatalf("drifted claim consumed exact pending handoff: %v", err)
	}
}

func TestExclusiveLeaseFailsClosedOnInvalidSupervisorHandoff(t *testing.T) {
	caseRoot := currentProjectFixture(t)
	rootPath, key, err := handoffRoot(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(rootPath, handoffPendingName(key))
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	if lease, err := AcquireExclusive(caseRoot); err == nil {
		_ = lease.Unlock()
		t.Fatal("exclusive execution lease accepted invalid pending handoff")
	}
}
