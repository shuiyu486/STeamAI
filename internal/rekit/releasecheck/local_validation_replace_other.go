//go:build !windows

package releasecheck

import "os"

func replaceLocalValidationReceipt(tempPath, path string) error {
	return os.Rename(tempPath, path)
}
