//go:build !windows

package steamai

import (
	"io"
)

type nativePlatform struct{}

func (nativePlatform) Supported() bool                      { return false }
func (nativePlatform) CanonicalSource() (string, error)     { return "", errUnsupportedPlatform }
func (nativePlatform) ActiveExecutable() (string, error)    { return "", errUnsupportedPlatform }
func (nativePlatform) Install(string, string, string) error { return errUnsupportedPlatform }
func (nativePlatform) ActivateUpdate(updateInstall) (updateResult, error) {
	return updateResult{}, errUnsupportedPlatform
}
func (nativePlatform) Uninstall(string) (uninstallResult, error) {
	return uninstallResult{}, errUnsupportedPlatform
}
func (nativePlatform) CaseIdentity(string) (string, error) {
	return "", errUnsupportedPlatform
}
func (nativePlatform) AcquireCommander(string) (commanderLease, error) {
	return commanderLease{}, errUnsupportedPlatform
}
func (nativePlatform) AcquireCanonicalMutation(string) (commanderLease, error) {
	return commanderLease{}, errUnsupportedPlatform
}
func (nativePlatform) RunAttached(processSpec, io.Reader, io.Writer, io.Writer) error {
	return errUnsupportedPlatform
}
func (nativePlatform) OpenVisible(processSpec) error { return errUnsupportedPlatform }
func rejectReparse(string) error                     { return nil }
