package projectexecution

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func currentProjectFixture(t *testing.T) string {
	t.Helper()
	caseRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(caseRoot, ".steamai"), 0o755); err != nil {
		t.Fatal(err)
	}
	return caseRoot
}

func TestSharedLeasesCanCoexist(t *testing.T) {
	caseRoot := currentProjectFixture(t)
	first, err := AcquireShared(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Unlock()

	second, err := AcquireShared(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.ValidateFor(caseRoot); err != nil {
		t.Fatal(err)
	}
	if err := second.Unlock(); err != nil {
		t.Fatal(err)
	}
}

func TestSharedLeaseBlocksExclusive(t *testing.T) {
	caseRoot := currentProjectFixture(t)
	shared, err := AcquireShared(caseRoot)
	if err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	done := make(chan struct {
		lease *Lease
		err   error
	}, 1)
	go func() {
		close(started)
		lease, err := AcquireExclusive(caseRoot)
		done <- struct {
			lease *Lease
			err   error
		}{lease, err}
	}()
	<-started
	select {
	case acquired := <-done:
		if acquired.lease != nil {
			_ = acquired.lease.Unlock()
		}
		_ = shared.Unlock()
		t.Fatalf("exclusive lease acquired while shared lease was held: %v", acquired.err)
	case <-time.After(150 * time.Millisecond):
	}
	if err := shared.Unlock(); err != nil {
		t.Fatal(err)
	}
	select {
	case acquired := <-done:
		if acquired.err != nil {
			t.Fatal(acquired.err)
		}
		if err := acquired.lease.Unlock(); err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("exclusive lease did not acquire after shared lease released")
	}
}

func TestExclusiveLeaseBlocksShared(t *testing.T) {
	caseRoot := currentProjectFixture(t)
	exclusive, err := AcquireExclusive(caseRoot)
	if err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	done := make(chan struct {
		lease *Lease
		err   error
	}, 1)
	go func() {
		close(started)
		lease, err := AcquireShared(caseRoot)
		done <- struct {
			lease *Lease
			err   error
		}{lease, err}
	}()
	<-started
	select {
	case acquired := <-done:
		if acquired.lease != nil {
			_ = acquired.lease.Unlock()
		}
		_ = exclusive.Unlock()
		t.Fatalf("shared lease acquired while exclusive lease was held: %v", acquired.err)
	case <-time.After(150 * time.Millisecond):
	}
	if err := exclusive.Unlock(); err != nil {
		t.Fatal(err)
	}
	select {
	case acquired := <-done:
		if acquired.err != nil {
			t.Fatal(acquired.err)
		}
		if err := acquired.lease.Unlock(); err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shared lease did not acquire after exclusive lease released")
	}
}

func TestProjectExecutionLeaseRejectsDualRoots(t *testing.T) {
	caseRoot := currentProjectFixture(t)
	if err := os.Mkdir(filepath.Join(caseRoot, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireShared(caseRoot); err == nil || !strings.Contains(err.Error(), "must not coexist") {
		t.Fatalf("dual-root lease error = %v", err)
	}
}

func TestProjectExecutionLeaseRejectsSymlinkAlias(t *testing.T) {
	caseRoot := currentProjectFixture(t)
	alias := filepath.Join(t.TempDir(), "project-alias")
	if err := os.Symlink(caseRoot, alias); err != nil {
		t.Skipf("project symlink unavailable: %v", err)
	}
	if lease, err := AcquireShared(alias); err == nil {
		_ = lease.Unlock()
		t.Fatal("project execution lease accepted a symlink alias")
	}
}

func TestProjectExecutionLeaseDetectsStateRootRebind(t *testing.T) {
	caseRoot := currentProjectFixture(t)
	lease, err := AcquireShared(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Unlock()

	stateRoot := filepath.Join(caseRoot, ".steamai")
	movedRoot := filepath.Join(caseRoot, ".steamai-moved")
	if err := os.Rename(stateRoot, movedRoot); err != nil {
		t.Skipf("state root cannot be rebound while pinned on this filesystem: %v", err)
	}
	if err := os.Mkdir(stateRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := lease.ValidateFor(caseRoot); err == nil || !strings.Contains(err.Error(), "namespace changed") {
		t.Fatalf("state-root rebind validation error = %v", err)
	}
}

func TestProjectExecutionLeaseReleasesAfterProcessKill(t *testing.T) {
	caseRoot := currentProjectFixture(t)
	readyPath := filepath.Join(t.TempDir(), "ready")
	cmd := exec.Command(os.Args[0], "-test.run=^TestProjectExecutionLeaseHelperProcess$")
	cmd.Env = append(os.Environ(),
		"REKIT_PROJECT_EXECUTION_HELPER=1",
		"REKIT_PROJECT_EXECUTION_CASE_ROOT="+caseRoot,
		"REKIT_PROJECT_EXECUTION_READY_PATH="+readyPath,
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Lstat(readyPath); err == nil {
			break
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatal("project execution helper did not acquire its lease")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("killed project execution helper exited successfully")
	}

	lease, err := AcquireExclusive(caseRoot)
	if err != nil {
		t.Fatalf("process kill did not release project execution lease: %v", err)
	}
	if err := lease.Unlock(); err != nil {
		t.Fatal(err)
	}
}

func TestProjectExecutionLeaseHelperProcess(t *testing.T) {
	if os.Getenv("REKIT_PROJECT_EXECUTION_HELPER") != "1" {
		return
	}
	lease, err := AcquireExclusive(os.Getenv("REKIT_PROJECT_EXECUTION_CASE_ROOT"))
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Unlock()
	if err := os.WriteFile(os.Getenv("REKIT_PROJECT_EXECUTION_READY_PATH"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for {
		time.Sleep(time.Second)
	}
}
