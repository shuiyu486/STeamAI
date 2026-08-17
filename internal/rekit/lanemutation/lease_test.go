package lanemutation

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectlock"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func TestProjectRefreshLeaseAllowsAtomicInstanceReplacement(t *testing.T) {
	caseRoot := t.TempDir()
	stateRoot := filepath.Join(caseRoot, projectstate.CurrentDir)
	if err := os.Mkdir(stateRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	instancePath := filepath.Join(stateRoot, "instance.yml")
	if err := os.WriteFile(instancePath, []byte("projectName: original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lease, err := AcquireProjectRefresh(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if lease.InstancePath() != "" || lease.instanceFile != nil {
		t.Fatalf("refresh lease pinned instance identity: path=%q file=%v", lease.InstancePath(), lease.instanceFile)
	}
	replacementPath := instancePath + ".replacement"
	if err := os.WriteFile(replacementPath, []byte("projectName: refreshed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacementPath, instancePath); err != nil {
		t.Fatal(err)
	}
	if err := lease.Validate(); err != nil {
		t.Fatalf("validate after instance replacement: %v", err)
	}
	if err := lease.Unlock(); err != nil {
		t.Fatalf("unlock after instance replacement: %v", err)
	}
}

func TestProjectRefreshLeaseRejectsMetadataRootReplacement(t *testing.T) {
	caseRoot := t.TempDir()
	stateRoot := filepath.Join(caseRoot, projectstate.CurrentDir)
	if err := os.Mkdir(stateRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateRoot, "instance.yml"), []byte("projectName: original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lease, err := AcquireProjectRefresh(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	movedPath := stateRoot + "-moved"
	if err := os.Rename(stateRoot, movedPath); err != nil {
		if closeErr := lease.metadataRoot.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		lease.metadataRoot = nil
		if err := os.Rename(stateRoot, movedPath); err != nil {
			t.Fatal(err)
		}
		lease.metadataRoot, err = os.OpenRoot(movedPath)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(stateRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := lease.Validate(); err == nil || !strings.Contains(err.Error(), "metadata namespace changed") {
		t.Fatalf("metadata replacement validation error = %v", err)
	}
	if err := lease.Unlock(); err == nil || !strings.Contains(err.Error(), "metadata namespace changed") {
		t.Fatalf("metadata replacement unlock error = %v", err)
	}
}

func TestProjectRefreshLeaseUsesSameExternalProjectLock(t *testing.T) {
	caseRoot := t.TempDir()
	stateRoot := filepath.Join(caseRoot, projectstate.CurrentDir)
	if err := os.MkdirAll(filepath.Join(stateRoot, "lanes", "lane-one"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateRoot, "instance.yml"), []byte("projectName: original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateRoot, "lanes", "lane-one", "lane.json"), []byte(`{"id":"lane-one","status":"open"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	refresh, err := AcquireProjectRefresh(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	projectLock := filepath.Base(refresh.externalProjectPath)
	if err := refresh.Unlock(); err != nil {
		t.Fatal(err)
	}
	project, err := AcquireProject(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(project.externalProjectPath) != projectLock {
		t.Fatalf("refresh and project leases use different project locks")
	}
	if err := project.Unlock(); err != nil {
		t.Fatal(err)
	}
	lane, err := AcquireLane(caseRoot, "lane-one")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(lane.externalProjectPath) != projectLock {
		t.Fatalf("refresh and lane leases use different project locks")
	}
	identity, err := projectlock.CanonicalProjectIdentity(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	laneKey := sha256.Sum256([]byte(identity + "\x00" + "lane-one"))
	wantLaneLock := "lane-" + hex.EncodeToString(laneKey[:]) + ".lease"
	if got := filepath.Base(lane.externalLanePath); got != wantLaneLock {
		t.Fatalf("lane lock filename changed: got=%s want=%s", got, wantLaneLock)
	}
	if err := lane.Unlock(); err != nil {
		t.Fatal(err)
	}
}

func TestOrdinaryProjectLeaseRefusesPendingCurrentSync(t *testing.T) {
	caseRoot := t.TempDir()
	stateRoot := filepath.Join(caseRoot, projectstate.CurrentDir)
	if err := os.MkdirAll(filepath.Join(stateRoot, "maintenance", "current-sync-v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateRoot, "instance.yml"), []byte("projectName: original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateRoot, "maintenance", "current-sync-v1", "intent.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireProject(caseRoot); err == nil || !strings.Contains(err.Error(), "current sync recovery is pending") {
		t.Fatalf("ordinary project lease pending current sync error = %v", err)
	}
	refresh, err := AcquireProjectRefresh(caseRoot)
	if err != nil {
		t.Fatalf("refresh recovery lease was fenced by its own intent: %v", err)
	}
	if err := refresh.Unlock(); err != nil {
		t.Fatal(err)
	}
}

func TestAcquireProjectUsesActiveStateRoot(t *testing.T) {
	for _, stateDir := range []string{".steamai", ".rekit"} {
		t.Run(stateDir, func(t *testing.T) {
			caseRoot := t.TempDir()
			stateRoot := filepath.Join(caseRoot, stateDir)
			if err := os.Mkdir(stateRoot, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(stateRoot, "instance.yml"), []byte("templatePack: unit\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			lease, err := AcquireProject(caseRoot)
			if err != nil {
				t.Fatal(err)
			}
			if lease.metadataPath != stateRoot || lease.InstancePath() != filepath.Join(stateRoot, "instance.yml") {
				t.Fatalf("lease root = %q instance = %q, want %q", lease.metadataPath, lease.InstancePath(), stateRoot)
			}
			if err := lease.Unlock(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAcquireProjectRejectsDualStateRoots(t *testing.T) {
	caseRoot := t.TempDir()
	for _, stateDir := range []string{".steamai", ".rekit"} {
		if err := os.Mkdir(filepath.Join(caseRoot, stateDir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := AcquireProject(caseRoot); err == nil || !strings.Contains(err.Error(), "must not coexist") {
		t.Fatalf("AcquireProject error = %v, want dual-root rejection", err)
	}
}
