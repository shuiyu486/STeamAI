package sessionhost

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const liveAcceptanceClaudePublisher = "Anthropic, PBC"

type trustedClaudeExecutable struct {
	Path      string
	Publisher string
	Version   string
	SHA256    string
}

type claudeExecutableLock struct {
	sourcePath string
	nativePath string
	file       *os.File
	namespace  []*os.File
}

func (lock *claudeExecutableLock) Validate() error {
	if lock == nil || lock.file == nil || strings.TrimSpace(lock.nativePath) == "" {
		return fmt.Errorf("trusted Claude executable launch binding is missing")
	}
	return validateClaudeExecutableLaunchNamespace(lock.sourcePath, lock.file, lock.namespace)
}

func (lock *claudeExecutableLock) Close() error {
	if lock == nil {
		return nil
	}
	errs := []error{}
	if lock.file != nil {
		errs = append(errs, lock.file.Close())
	}
	for i := len(lock.namespace) - 1; i >= 0; i-- {
		errs = append(errs, lock.namespace[i].Close())
	}
	return errors.Join(errs...)
}

func discoverTrustedClaudeExecutable() (trustedClaudeExecutable, error) {
	candidates, err := trustedClaudeInstallationCandidates()
	if err != nil {
		return trustedClaudeExecutable{}, err
	}
	failures := []error{}
	for _, candidate := range candidates {
		identity, err := inspectTrustedClaudeExecutable(candidate, true)
		if err == nil {
			return identity, nil
		}
		if !os.IsNotExist(err) {
			failures = append(failures, fmt.Errorf("%s: %w", candidate, err))
		}
	}
	if len(failures) > 0 {
		return trustedClaudeExecutable{}, fmt.Errorf("no trusted installed Claude Code executable passed provenance validation: %w", errors.Join(failures...))
	}
	return trustedClaudeExecutable{}, fmt.Errorf("trusted installed Claude Code executable not found in canonical installation locations")
}

func lockTrustedClaudeExecutable(path string) (*claudeExecutableLock, error) {
	if err := validateClaudeExecutablePathComponents(path); err != nil {
		return nil, err
	}
	locked, err := openClaudeExecutableReadLock(path)
	if err != nil {
		return nil, err
	}
	binding := &claudeExecutableLock{sourcePath: path, file: locked}
	fail := func(err error) (*claudeExecutableLock, error) {
		_ = binding.Close()
		return nil, err
	}
	if err := validateLockedClaudeExecutablePath(path, locked); err != nil {
		return fail(err)
	}
	binding.nativePath, err = nativeClaudeExecutablePath(locked)
	if err != nil {
		return fail(err)
	}
	binding.namespace, err = lockClaudeExecutablePathNamespace(path, locked)
	if err != nil {
		return fail(err)
	}
	return binding, nil
}

func inspectTrustedClaudeExecutable(path string, includeVersion bool) (trustedClaudeExecutable, error) {
	full, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return trustedClaudeExecutable{}, err
	}
	full = filepath.Clean(full)
	binding, err := lockTrustedClaudeExecutable(full)
	if err != nil {
		return trustedClaudeExecutable{}, err
	}
	defer binding.Close()
	publisher, err := verifyClaudeAuthenticodePublisher(binding.file)
	if err != nil {
		return trustedClaudeExecutable{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(publisher), liveAcceptanceClaudePublisher) {
		return trustedClaudeExecutable{}, fmt.Errorf("Claude Code Authenticode signer is not %q: %q", liveAcceptanceClaudePublisher, publisher)
	}
	hash, err := hashLockedClaudeExecutable(binding.file)
	if err != nil {
		return trustedClaudeExecutable{}, err
	}
	identity := trustedClaudeExecutable{
		Path:      full,
		Publisher: strings.TrimSpace(publisher),
		SHA256:    hash,
	}
	if includeVersion {
		identity.Version, err = trustedClaudeVersion(binding.file)
		if err != nil {
			return trustedClaudeExecutable{}, err
		}
		if err := binding.Validate(); err != nil {
			return trustedClaudeExecutable{}, err
		}
	}
	return identity, nil
}

func acquireClaudeExecutableLaunchBinding(opt Options) (*claudeExecutableLock, error) {
	expectedHash := strings.ToLower(strings.TrimSpace(opt.ExpectedClaudeExecutableSHA256))
	expectedPublisher := strings.TrimSpace(opt.ExpectedClaudeExecutablePublisher)
	if expectedHash == "" && expectedPublisher == "" {
		return nil, nil
	}
	if len(expectedHash) != 64 || expectedPublisher == "" {
		return nil, fmt.Errorf("trusted Claude launch binding requires an exact SHA-256 and publisher")
	}
	if err := validateClaudeExecutablePathComponents(opt.ClaudePath); err != nil {
		return nil, err
	}
	locked, err := openClaudeExecutableReadLock(opt.ClaudePath)
	if err != nil {
		return nil, fmt.Errorf("lock trusted Claude executable for launch: %w", err)
	}
	binding := &claudeExecutableLock{sourcePath: opt.ClaudePath, file: locked}
	fail := func(err error) (*claudeExecutableLock, error) {
		_ = binding.Close()
		return nil, err
	}
	if err := validateLockedClaudeExecutablePath(opt.ClaudePath, locked); err != nil {
		return fail(err)
	}
	binding.nativePath, err = nativeClaudeExecutablePath(locked)
	if err != nil {
		return fail(err)
	}
	publisher, err := verifyClaudeAuthenticodePublisher(binding.file)
	if err != nil {
		return fail(fmt.Errorf("revalidate trusted Claude Authenticode signature before launch: %w", err))
	}
	if !strings.EqualFold(strings.TrimSpace(publisher), expectedPublisher) || !strings.EqualFold(expectedPublisher, liveAcceptanceClaudePublisher) {
		return fail(fmt.Errorf("trusted Claude publisher drift before launch: got %q want %q", publisher, expectedPublisher))
	}
	hash, err := hashLockedClaudeExecutable(locked)
	if err != nil {
		return fail(fmt.Errorf("rehash trusted Claude executable before launch: %w", err))
	}
	if hash != expectedHash {
		return fail(fmt.Errorf("trusted Claude executable SHA-256 drift before launch: got %s want %s", hash, expectedHash))
	}
	if err := validateLockedClaudeExecutablePath(opt.ClaudePath, locked); err != nil {
		return fail(err)
	}
	binding.namespace, err = lockClaudeExecutablePathNamespace(opt.ClaudePath, locked)
	if err != nil {
		return fail(err)
	}
	return binding, nil
}

func validateLockedClaudeExecutablePath(path string, locked *os.File) error {
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !pathInfo.Mode().IsRegular() || pathInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("trusted Claude executable must be a regular non-symlink file: %s", path)
	}
	lockedInfo, err := locked.Stat()
	if err != nil {
		return err
	}
	if !lockedInfo.Mode().IsRegular() || !os.SameFile(pathInfo, lockedInfo) {
		return fmt.Errorf("trusted Claude executable path changed while acquiring its launch lock: %s", path)
	}
	return nil
}

func hashLockedClaudeExecutable(locked *os.File) (string, error) {
	if _, err := locked.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, locked); err != nil {
		return "", err
	}
	if _, err := locked.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
