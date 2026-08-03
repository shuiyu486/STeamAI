//go:build windows

package memberexecution

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
		return fmt.Errorf("member execution path must not be a reparse point: %s", path)
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
		if err := rejectReparsePath(current); err != nil {
			return err
		}
	}
	return nil
}

func splitPathComponents(path string) []string {
	components := []string{}
	for path != "." && path != "" {
		dir, leaf := filepath.Split(path)
		if leaf != "" {
			components = append([]string{leaf}, components...)
		}
		path = filepath.Clean(dir)
		path = filepath.Clean(path)
		if path == string(filepath.Separator) {
			break
		}
	}
	return components
}
