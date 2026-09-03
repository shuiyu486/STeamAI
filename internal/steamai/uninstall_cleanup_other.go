//go:build !windows

package steamai

func runUninstallCleanup([]string) error {
	return errUnsupportedPlatform
}
