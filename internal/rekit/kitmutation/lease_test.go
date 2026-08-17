package kitmutation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAcquireProjectRefreshBypassesOnlyPendingCurrentSyncFence(t *testing.T) {
	caseRoot := t.TempDir()
	stateRoot := filepath.Join(caseRoot, ".steamai")
	if err := os.MkdirAll(filepath.Join(stateRoot, "maintenance", "current-sync-v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateRoot, "instance.yml"), []byte("schemaVersion: 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateRoot, "maintenance", "current-sync-v1", "intent.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lease, err := AcquireProjectRefresh(caseRoot)
	if err != nil {
		t.Fatalf("project refresh lease was fenced by its own transaction: %v", err)
	}
	if err := lease.Unlock(); err != nil {
		t.Fatal(err)
	}
}

func TestAcquireRefusesPendingCurrentSync(t *testing.T) {
	caseRoot := t.TempDir()
	stateRoot := filepath.Join(caseRoot, ".steamai")
	if err := os.MkdirAll(filepath.Join(stateRoot, "maintenance", "current-sync-v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateRoot, "instance.yml"), []byte("schemaVersion: 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateRoot, "maintenance", "current-sync-v1", "intent.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(caseRoot); err == nil || !strings.Contains(err.Error(), "current sync recovery is pending") {
		t.Fatalf("kit mutation pending current sync error = %v", err)
	}
}
