//go:build windows

package adapterhost

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"syscall"
)

func validateLiveAcceptanceCleanupTree(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		pointer, err := syscall.UTF16PtrFromString(path)
		if err != nil {
			return err
		}
		attributes, err := syscall.GetFileAttributes(pointer)
		if err != nil {
			return err
		}
		if attributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return fmt.Errorf("refuse adapter live cleanup through reparse point: %s", path)
		}
		if entry.IsDir() || entry.Type().IsRegular() {
			return nil
		}
		return fmt.Errorf("refuse adapter live cleanup of a non-regular object: %s", path)
	})
}
