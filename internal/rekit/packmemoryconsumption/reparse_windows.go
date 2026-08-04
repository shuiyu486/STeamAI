//go:build windows

package packmemoryconsumption

import (
	"fmt"
	"path/filepath"
	"syscall"
)

func rejectReparsePath(path string) error {
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attributes, err := syscall.GetFileAttributes(pointer)
	if err != nil {
		return err
	}
	if attributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("pack-memory consumption anchored path must not be a reparse point: %s", path)
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
	for rel != "." && rel != "" {
		dir, leaf := filepath.Split(rel)
		components := []string{leaf}
		for filepath.Clean(dir) != "." && filepath.Clean(dir) != string(filepath.Separator) && dir != "" {
			dir, leaf = filepath.Split(filepath.Clean(dir))
			components = append([]string{leaf}, components...)
		}
		for _, component := range components {
			if component == "" {
				continue
			}
			current = filepath.Join(current, component)
			if err := rejectReparsePath(current); err != nil {
				return err
			}
		}
		break
	}
	return nil
}
