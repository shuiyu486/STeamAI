package projectlock

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"runtime"
	"strings"
)

// WorkstreamRoot returns the stable per-user directory shared by project and
// lane lock files. Callers own creation and namespace validation.
func WorkstreamRoot() (string, error) {
	return stableWorkstreamLockRoot()
}

// CanonicalProjectIdentity returns the existing physical project path identity
// used by established project and lane lock namespaces.
func CanonicalProjectIdentity(caseRoot string) (string, error) {
	casePath, err := filepath.Abs(caseRoot)
	if err != nil {
		return "", err
	}
	identity, err := filepath.EvalSymlinks(casePath)
	if err != nil {
		return "", err
	}
	identity = filepath.Clean(identity)
	if runtime.GOOS == "windows" {
		identity = strings.ToLower(identity)
	}
	return identity, nil
}

// CanonicalProjectKey returns the existing physical project path key used by
// project-scoped lock namespaces.
func CanonicalProjectKey(caseRoot string) (string, error) {
	identity, err := CanonicalProjectIdentity(caseRoot)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(sum[:]), nil
}

// Lock acquires one blocking kernel lock on the selected byte range.
func Lock(fd uintptr, exclusive bool) error {
	return lockFile(fd, exclusive)
}

// Unlock releases a lock acquired by Lock.
func Unlock(fd uintptr) error {
	return unlockFile(fd)
}
