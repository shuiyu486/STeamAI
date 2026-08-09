//go:build !windows

package sessionhost

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func validateLiveAcceptanceCleanupTree(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refuse live acceptance cleanup through symlink: %s", path)
		}
		if info.IsDir() || info.Mode().IsRegular() {
			return nil
		}
		return fmt.Errorf("refuse live acceptance cleanup of a non-regular object: %s", path)
	})
}
