package mutationfence

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRefusePendingCurrentSync(t *testing.T) {
	for _, rel := range []string{currentSyncOwnerRel, currentSyncIntentRel} {
		t.Run(filepath.Base(rel), func(t *testing.T) {
			caseRoot := t.TempDir()
			stateRoot := filepath.Join(caseRoot, ".steamai")
			if err := os.MkdirAll(filepath.Join(stateRoot, "maintenance", "current-sync-v1"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(stateRoot, "instance.yml"), []byte("schemaVersion: 2\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := RefusePendingCurrentSync(caseRoot, "test mutation"); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(stateRoot, filepath.FromSlash(rel))
			if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := RefusePendingCurrentSync(caseRoot, "test mutation"); err == nil || !strings.Contains(err.Error(), "recovery is pending") {
				t.Fatalf("pending current sync fence error = %v", err)
			}
		})
	}
}

func TestRefusePendingCurrentSyncRejectsInvalidFence(t *testing.T) {
	for _, rel := range []string{currentSyncOwnerRel, currentSyncIntentRel} {
		t.Run(filepath.Base(rel), func(t *testing.T) {
			caseRoot := t.TempDir()
			stateRoot := filepath.Join(caseRoot, ".steamai")
			path := filepath.Join(stateRoot, filepath.FromSlash(rel))
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(stateRoot, "instance.yml"), []byte("schemaVersion: 2\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := RefusePendingCurrentSync(caseRoot, "test mutation"); err == nil || !strings.Contains(err.Error(), "invalid pending current sync fence") {
				t.Fatalf("invalid current sync fence error = %v", err)
			}
		})
	}
}
