package lanemutation

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
)

var safeLaneIDSegment = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$`)

type Lease struct {
	metadataRoot        *os.Root
	stableCaseFile      *os.File
	externalProjectFile *os.File
	externalLaneFile    *os.File
	instanceFile        *os.File
	canonicalLaneFile   *os.File
	metadataPath        string
	stableCasePath      string
	externalProjectPath string
	externalLanePath    string
	instancePath        string
	canonicalLanePath   string
	metadataInfo        os.FileInfo
	stableCaseInfo      os.FileInfo
	externalProjectInfo os.FileInfo
	externalLaneInfo    os.FileInfo
	instanceInfo        os.FileInfo
	canonicalLaneInfo   os.FileInfo
	unlockFile          func(uintptr) error
}

func AcquireLane(caseRoot, laneID string) (*Lease, error) {
	if !safeLaneIDSegment.MatchString(laneID) {
		return nil, fmt.Errorf("invalid lane id for mutation lock: %q", laneID)
	}
	return acquire(caseRoot, laneID)
}

func AcquireProject(caseRoot string) (*Lease, error) {
	return acquire(caseRoot, "")
}

func acquire(caseRoot, laneID string) (*Lease, error) {
	casePath, err := filepath.Abs(caseRoot)
	if err != nil {
		return nil, err
	}
	metadataPath, err := refsf.SafeJoin(casePath, ".rekit")
	if err != nil {
		return nil, err
	}
	if st, err := os.Lstat(metadataPath); err != nil {
		return nil, err
	} else if st.Mode()&os.ModeSymlink != 0 || !st.IsDir() {
		return nil, fmt.Errorf("workstream metadata root must be a directory and not a symlink: %s", metadataPath)
	}
	metadataRoot, err := os.OpenRoot(metadataPath)
	if err != nil {
		return nil, err
	}
	metadataInfo, err := os.Stat(metadataPath)
	if err != nil {
		return nil, errors.Join(err, metadataRoot.Close())
	}
	lease := &Lease{metadataRoot: metadataRoot, metadataPath: metadataPath, metadataInfo: metadataInfo, unlockFile: unlockLaneLeaseFile}
	casePathRoot, err := os.OpenRoot(casePath)
	if err != nil {
		return nil, errors.Join(err, metadataRoot.Close())
	}
	stableCaseRel := ".re-template.yml"
	stableCaseBefore, err := casePathRoot.Lstat(stableCaseRel)
	if err == nil {
		if stableCaseBefore.Mode()&os.ModeSymlink != 0 || !stableCaseBefore.Mode().IsRegular() {
			return nil, errors.Join(fmt.Errorf("stable workstream case identity must be a regular file and not a symlink: %s", filepath.Join(casePath, stableCaseRel)), casePathRoot.Close(), metadataRoot.Close())
		}
		lease.stableCaseFile, err = casePathRoot.Open(stableCaseRel)
		if err != nil {
			return nil, errors.Join(err, casePathRoot.Close(), metadataRoot.Close())
		}
		lease.stableCaseInfo, err = lease.stableCaseFile.Stat()
		if err != nil {
			return nil, errors.Join(err, lease.stableCaseFile.Close(), casePathRoot.Close(), metadataRoot.Close())
		}
		lease.stableCasePath = filepath.Join(casePath, stableCaseRel)
		if !os.SameFile(stableCaseBefore, lease.stableCaseInfo) {
			return nil, errors.Join(fmt.Errorf("stable workstream case identity changed while opening: %s", lease.stableCasePath), lease.stableCaseFile.Close(), casePathRoot.Close(), metadataRoot.Close())
		}
	} else if !os.IsNotExist(err) {
		return nil, errors.Join(err, casePathRoot.Close(), metadataRoot.Close())
	}
	if err := casePathRoot.Close(); err != nil {
		if lease.stableCaseFile != nil {
			return nil, errors.Join(err, lease.stableCaseFile.Close(), metadataRoot.Close())
		}
		return nil, errors.Join(err, metadataRoot.Close())
	}
	if lease.stableCaseFile != nil {
		if err := lockLaneLeaseFile(lease.stableCaseFile.Fd(), true); err != nil {
			return nil, errors.Join(err, lease.stableCaseFile.Close(), metadataRoot.Close())
		}
	}
	fail := func(cause error) (*Lease, error) {
		return nil, errors.Join(cause, lease.Unlock())
	}

	laneRel := ""
	laneExists := false
	if laneID != "" {
		laneRel = filepath.Join("lanes", laneID, "lane.json")
		laneInfo, laneErr := metadataRoot.Lstat(laneRel)
		switch {
		case laneErr == nil:
			if laneInfo.Mode()&os.ModeSymlink != 0 || !laneInfo.Mode().IsRegular() {
				return fail(fmt.Errorf("canonical workstream lane must be a regular file and not a symlink: %s", filepath.Join(metadataPath, laneRel)))
			}
			laneExists = true
		case os.IsNotExist(laneErr):
		default:
			return fail(laneErr)
		}
	}
	exclusiveProject := true

	lockRootPath, err := stableWorkstreamLockRoot()
	if err != nil {
		return fail(err)
	}
	if err := os.MkdirAll(lockRootPath, 0o700); err != nil {
		return fail(err)
	}
	if st, err := os.Lstat(lockRootPath); err != nil {
		return fail(err)
	} else if st.Mode()&os.ModeSymlink != 0 || !st.IsDir() {
		return fail(fmt.Errorf("workstream lock root must be a directory and not a symlink: %s", lockRootPath))
	}
	lockRoot, err := os.OpenRoot(lockRootPath)
	if err != nil {
		return fail(err)
	}
	closeLockRoot := func(cause error) (*Lease, error) {
		return fail(errors.Join(cause, lockRoot.Close()))
	}
	caseIdentity, err := filepath.EvalSymlinks(casePath)
	if err != nil {
		return closeLockRoot(fmt.Errorf("resolve canonical workstream case identity: %w", err))
	}
	caseIdentity = filepath.Clean(caseIdentity)
	if runtime.GOOS == "windows" {
		caseIdentity = strings.ToLower(caseIdentity)
	}
	caseKey := sha256.Sum256([]byte(caseIdentity))
	projectName := "case-" + hex.EncodeToString(caseKey[:]) + ".lease"
	lease.externalProjectFile, err = lockRoot.OpenFile(projectName, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return closeLockRoot(err)
	}
	lease.externalProjectPath = filepath.Join(lockRootPath, projectName)
	lease.externalProjectInfo, err = lease.externalProjectFile.Stat()
	if err != nil {
		return closeLockRoot(err)
	}
	if err := lockLaneLeaseFile(lease.externalProjectFile.Fd(), exclusiveProject); err != nil {
		return closeLockRoot(err)
	}
	if laneExists {
		laneKey := sha256.Sum256([]byte(caseIdentity + "\x00" + laneID))
		laneName := "lane-" + hex.EncodeToString(laneKey[:]) + ".lease"
		lease.externalLaneFile, err = lockRoot.OpenFile(laneName, os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			return closeLockRoot(err)
		}
		lease.externalLanePath = filepath.Join(lockRootPath, laneName)
		lease.externalLaneInfo, err = lease.externalLaneFile.Stat()
		if err != nil {
			return closeLockRoot(err)
		}
		if err := lockLaneLeaseFile(lease.externalLaneFile.Fd(), true); err != nil {
			return closeLockRoot(err)
		}
	}
	if err := lockRoot.Close(); err != nil {
		return fail(err)
	}
	instanceRel := "instance.yml"
	instanceRoot := metadataRoot
	instanceBase := metadataPath
	var legacyInstanceRoot *os.Root
	instanceBefore, err := metadataRoot.Lstat(instanceRel)
	if os.IsNotExist(err) {
		instanceRel = ".re-template.yml"
		instanceBase = casePath
		legacyInstanceRoot, err = os.OpenRoot(casePath)
		if err != nil {
			return fail(err)
		}
		instanceRoot = legacyInstanceRoot
		instanceBefore, err = instanceRoot.Lstat(instanceRel)
	}
	if err != nil {
		return fail(err)
	}
	if instanceBefore.Mode()&os.ModeSymlink != 0 || !instanceBefore.Mode().IsRegular() {
		if legacyInstanceRoot != nil {
			_ = legacyInstanceRoot.Close()
		}
		return fail(fmt.Errorf("canonical workstream instance must be a regular file and not a symlink: %s", filepath.Join(instanceBase, instanceRel)))
	}
	lease.instanceFile, err = instanceRoot.Open(instanceRel)
	if legacyInstanceRoot != nil {
		err = errors.Join(err, legacyInstanceRoot.Close())
	}
	if err != nil {
		return fail(err)
	}
	lease.instanceInfo, err = lease.instanceFile.Stat()
	if err != nil {
		return fail(err)
	}
	lease.instancePath = filepath.Join(instanceBase, instanceRel)
	if !os.SameFile(instanceBefore, lease.instanceInfo) {
		return fail(fmt.Errorf("canonical workstream instance changed while opening: %s", lease.instancePath))
	}
	if lease.stableCaseFile != nil && os.SameFile(lease.stableCaseInfo, lease.instanceInfo) {
		if err := lease.instanceFile.Close(); err != nil {
			return fail(err)
		}
		lease.instanceFile = nil
	} else if err := lockLaneLeaseFile(lease.instanceFile.Fd(), exclusiveProject); err != nil {
		return fail(err)
	}
	if laneExists {
		laneBefore, err := metadataRoot.Lstat(laneRel)
		if err != nil {
			return fail(err)
		}
		if laneBefore.Mode()&os.ModeSymlink != 0 || !laneBefore.Mode().IsRegular() {
			return fail(fmt.Errorf("canonical workstream lane must be a regular file and not a symlink: %s", filepath.Join(metadataPath, laneRel)))
		}
		lease.canonicalLaneFile, err = metadataRoot.Open(laneRel)
		if err != nil {
			return fail(err)
		}
		lease.canonicalLaneInfo, err = lease.canonicalLaneFile.Stat()
		if err != nil {
			return fail(err)
		}
		lease.canonicalLanePath = filepath.Join(metadataPath, laneRel)
		if !os.SameFile(laneBefore, lease.canonicalLaneInfo) {
			return fail(fmt.Errorf("canonical workstream lane changed while opening: %s", lease.canonicalLanePath))
		}
		if err := lockLaneLeaseFile(lease.canonicalLaneFile.Fd(), true); err != nil {
			return fail(err)
		}
	}
	if err := lease.Validate(); err != nil {
		return fail(err)
	}
	return lease, nil
}

func (lease *Lease) validateIdentity() error {
	if lease.stableCaseFile != nil {
		currentStable, err := os.Lstat(lease.stableCasePath)
		if err != nil || !os.SameFile(lease.stableCaseInfo, currentStable) {
			return fmt.Errorf("workstream stable case namespace changed while mutation lease is held: %s", lease.stableCasePath)
		}
	}
	currentMetadata, err := os.Lstat(lease.metadataPath)
	if err != nil || !os.SameFile(lease.metadataInfo, currentMetadata) {
		return fmt.Errorf("workstream metadata namespace changed while mutation lease is held: %s", lease.metadataPath)
	}
	return nil
}

func (lease *Lease) InstancePath() string {
	if lease == nil {
		return ""
	}
	return lease.instancePath
}

func (lease *Lease) CanonicalLanePath() string {
	if lease == nil {
		return ""
	}
	return lease.canonicalLanePath
}

func (lease *Lease) SetUnlockFileForTest(unlock func(uintptr) error) {
	if lease != nil && unlock != nil {
		lease.unlockFile = unlock
	}
}

func (lease *Lease) Validate() error {
	if lease == nil || lease.metadataRoot == nil {
		return fmt.Errorf("workstream mutation lease is not held")
	}
	if err := lease.validateIdentity(); err != nil {
		return err
	}
	checks := []struct {
		file *os.File
		path string
		info os.FileInfo
		kind string
	}{
		{lease.stableCaseFile, lease.stableCasePath, lease.stableCaseInfo, "stable case"},
		{lease.externalProjectFile, lease.externalProjectPath, lease.externalProjectInfo, "project lease"},
		{lease.externalLaneFile, lease.externalLanePath, lease.externalLaneInfo, "lane lease"},
		{lease.instanceFile, lease.instancePath, lease.instanceInfo, "canonical instance"},
		{lease.canonicalLaneFile, lease.canonicalLanePath, lease.canonicalLaneInfo, "canonical lane"},
	}
	for _, check := range checks {
		if check.file == nil {
			continue
		}
		current, err := os.Stat(check.path)
		if err != nil || !os.SameFile(check.info, current) {
			return fmt.Errorf("workstream %s namespace changed while mutation lease is held: %s", check.kind, check.path)
		}
	}
	return nil
}

func (lease *Lease) Unlock() error {
	if lease == nil || lease.metadataRoot == nil {
		return nil
	}
	var errs []error
	if err := lease.Validate(); err != nil {
		errs = append(errs, err)
	}
	files := []*os.File{lease.canonicalLaneFile, lease.instanceFile, lease.externalLaneFile, lease.externalProjectFile, lease.stableCaseFile}
	for _, file := range files {
		if file == nil {
			continue
		}
		if err := lease.unlockFile(file.Fd()); err != nil {
			errs = append(errs, err)
		}
		if err := file.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := lease.metadataRoot.Close(); err != nil {
		errs = append(errs, err)
	}
	lease.metadataRoot = nil
	return errors.Join(errs...)
}
