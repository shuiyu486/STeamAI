//go:build !windows

package fs

import (
	"fmt"
	"os"
	"path/filepath"
)

func rejectReparsePath(string) error { return nil }

func validateNoReparsePath(path string) error {
	volume := filepath.VolumeName(path)
	current := volume + string(filepath.Separator)
	rel, err := filepath.Rel(current, path)
	if err != nil {
		return err
	}
	for _, component := range splitPathComponents(rel) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("path must not traverse symlink: %s", current)
		}
	}
	return nil
}

func rejectReparseAncestors(path string) error {
	volume := filepath.VolumeName(path)
	current := volume + string(filepath.Separator)
	rel, err := filepath.Rel(current, path)
	if err != nil {
		return err
	}
	for _, component := range splitPathComponents(rel) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("anchored path ancestor must not be a symlink: %s", current)
		}
	}
	return nil
}
