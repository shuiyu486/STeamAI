//go:build !windows

package sessionhost

import (
	"fmt"
	"os"
	"os/exec"
)

func trustedClaudeInstallationCandidates() ([]string, error) {
	return nil, fmt.Errorf("explicit live acceptance trusted Claude provenance is currently supported only on Windows")
}

func validateClaudeExecutablePathComponents(path string) error {
	return fmt.Errorf("trusted Claude path component validation is unavailable on this platform: %s", path)
}

func configureTrustedClaudeCommand(_ *exec.Cmd, binding *claudeExecutableLock) error {
	if binding != nil {
		return fmt.Errorf("trusted Claude suspended launch binding is unavailable on this platform")
	}
	return nil
}

func validateAndResumeTrustedClaudeProcess(_ *os.Process, binding *claudeExecutableLock) error {
	if binding != nil {
		return fmt.Errorf("trusted Claude suspended process validation is unavailable on this platform")
	}
	return nil
}

func trustedClaudeVersion(locked *os.File) (string, error) {
	return "", fmt.Errorf("trusted Claude version metadata is unavailable on this platform: %s", locked.Name())
}

func nativeClaudeExecutablePath(locked *os.File) (string, error) {
	return "", fmt.Errorf("trusted Claude native executable path is unavailable on this platform: %s", locked.Name())
}

func lockClaudeExecutablePathNamespace(path string, _ *os.File) ([]*os.File, error) {
	return nil, fmt.Errorf("trusted Claude namespace locking is unavailable on this platform: %s", path)
}

func validateClaudeExecutableLaunchNamespace(path string, _ *os.File, _ []*os.File) error {
	return fmt.Errorf("trusted Claude namespace validation is unavailable on this platform: %s", path)
}

func openClaudeExecutableReadLock(path string) (*os.File, error) {
	return os.Open(path)
}

func verifyClaudeAuthenticodePublisher(locked *os.File) (string, error) {
	return "", fmt.Errorf("Authenticode provenance is unavailable on this platform: %s", locked.Name())
}
