package projectexecution

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectlock"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

type Lease struct {
	caseRoot  *os.Root
	stateRoot *os.Root
	lockFile  *os.File
	casePath  string
	statePath string
	lockPath  string
	caseInfo  os.FileInfo
	stateInfo os.FileInfo
	lockInfo  os.FileInfo
	locked    bool
}

func AcquireShared(caseRoot string) (*Lease, error) {
	return acquire(caseRoot, false)
}

func AcquireExclusive(caseRoot string) (*Lease, error) {
	return acquire(caseRoot, true)
}

func acquire(caseRoot string, exclusive bool) (*Lease, error) {
	casePath, err := filepath.Abs(caseRoot)
	if err != nil {
		return nil, err
	}
	casePath = filepath.Clean(casePath)
	caseInfo, err := rekitfs.ValidateNonReparseDirectory(casePath, "project execution case root")
	if err != nil {
		return nil, err
	}
	state, err := projectstate.Resolve(casePath)
	if err != nil {
		return nil, err
	}
	if !state.Existing || state.Legacy || state.Dir != projectstate.CurrentDir {
		return nil, fmt.Errorf("project execution lease requires one existing .steamai-only project")
	}
	stateInfo, err := rekitfs.ValidateNonReparseDirectory(state.Path, "project execution state root")
	if err != nil {
		return nil, err
	}
	lease := &Lease{
		casePath:  casePath,
		statePath: state.Path,
		caseInfo:  caseInfo,
		stateInfo: stateInfo,
	}
	fail := func(cause error) (*Lease, error) {
		return nil, errors.Join(cause, lease.Unlock())
	}
	lease.caseRoot, err = os.OpenRoot(casePath)
	if err != nil {
		return fail(err)
	}
	lease.stateRoot, err = os.OpenRoot(state.Path)
	if err != nil {
		return fail(err)
	}
	lockRootPath, err := projectlock.WorkstreamRoot()
	if err != nil {
		return fail(err)
	}
	if err := os.MkdirAll(lockRootPath, 0o700); err != nil {
		return fail(err)
	}
	if _, err := rekitfs.ValidateNonReparseDirectory(lockRootPath, "project execution lock root"); err != nil {
		return fail(err)
	}
	lockRoot, err := os.OpenRoot(lockRootPath)
	if err != nil {
		return fail(err)
	}
	key, err := projectlock.CanonicalProjectKey(casePath)
	if err != nil {
		return fail(errors.Join(err, lockRoot.Close()))
	}
	name := "case-" + key + ".execution-v1.lease"
	lease.lockFile, err = lockRoot.OpenFile(name, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fail(errors.Join(err, lockRoot.Close()))
	}
	lease.lockPath = filepath.Join(lockRootPath, name)
	lease.lockInfo, err = lease.lockFile.Stat()
	if err != nil {
		return fail(errors.Join(err, lockRoot.Close()))
	}
	if err := lockRoot.Close(); err != nil {
		return fail(err)
	}
	if !lease.lockInfo.Mode().IsRegular() || lease.lockInfo.Mode()&os.ModeSymlink != 0 {
		return fail(fmt.Errorf("project execution lock must be a regular non-symlink file: %s", lease.lockPath))
	}
	if err := projectlock.Lock(lease.lockFile.Fd(), exclusive); err != nil {
		return fail(err)
	}
	lease.locked = true
	if err := lease.ValidateFor(casePath); err != nil {
		return fail(err)
	}
	if exclusive {
		if _, _, err := CancelPendingHandoff(casePath); err != nil {
			return fail(fmt.Errorf("cancel pending project execution supervisor handoff: %w", err))
		}
		if err := lease.ValidateFor(casePath); err != nil {
			return fail(err)
		}
	}
	return lease, nil
}

func (lease *Lease) Validate() error {
	if lease == nil {
		return fmt.Errorf("project execution lease is not held")
	}
	return lease.ValidateFor(lease.casePath)
}

func (lease *Lease) ValidateFor(caseRoot string) error {
	if lease == nil || lease.caseRoot == nil || lease.stateRoot == nil || lease.lockFile == nil {
		return fmt.Errorf("project execution lease is not held")
	}
	casePath, err := filepath.Abs(caseRoot)
	if err != nil {
		return err
	}
	same, err := rekitfs.SameExistingPath(lease.casePath, casePath)
	if err != nil || !same {
		return fmt.Errorf("project execution lease target changed: %s", casePath)
	}
	currentCase, caseErr := os.Lstat(lease.casePath)
	currentState, stateErr := os.Lstat(lease.statePath)
	currentLock, lockErr := os.Lstat(lease.lockPath)
	openedCase, openedCaseErr := lease.caseRoot.Stat(".")
	openedState, openedStateErr := lease.stateRoot.Stat(".")
	if caseErr != nil || stateErr != nil || lockErr != nil || openedCaseErr != nil || openedStateErr != nil ||
		!os.SameFile(lease.caseInfo, currentCase) || !os.SameFile(lease.caseInfo, openedCase) ||
		!os.SameFile(lease.stateInfo, currentState) || !os.SameFile(lease.stateInfo, openedState) ||
		!os.SameFile(lease.lockInfo, currentLock) {
		return fmt.Errorf("project execution namespace changed while lease is held: %s", lease.casePath)
	}
	state, err := projectstate.Resolve(lease.casePath)
	if err != nil || !state.Existing || state.Legacy || state.Dir != projectstate.CurrentDir || state.Path != lease.statePath {
		return fmt.Errorf("project execution state root changed while lease is held: %s", lease.statePath)
	}
	return nil
}

func (lease *Lease) Unlock() error {
	if lease == nil {
		return nil
	}
	var errs []error
	if lease.lockFile != nil {
		if lease.locked {
			errs = append(errs, projectlock.Unlock(lease.lockFile.Fd()))
			lease.locked = false
		}
		errs = append(errs, lease.lockFile.Close())
		lease.lockFile = nil
	}
	if lease.stateRoot != nil {
		errs = append(errs, lease.stateRoot.Close())
		lease.stateRoot = nil
	}
	if lease.caseRoot != nil {
		errs = append(errs, lease.caseRoot.Close())
		lease.caseRoot = nil
	}
	return errors.Join(errs...)
}
