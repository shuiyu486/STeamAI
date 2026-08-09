//go:build windows

package promote

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func rejectMemberOutputStagingPath(root, rel string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if filepath.IsAbs(rel) {
		rel, err = filepath.Rel(root, rel)
		if err != nil {
			return err
		}
	}
	rel = filepath.Clean(rel)
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("member output staging path escapes case root")
	}
	current := root
	components := []string{"."}
	if rel != "." && rel != "" {
		components = strings.FieldsFunc(rel, func(r rune) bool {
			return r == '/' || r == '\\'
		})
	}
	for _, component := range components {
		if component != "." {
			current = filepath.Join(current, component)
		}
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("member output staging path must not contain a symlink: %s", current)
		}
		pointer, err := syscall.UTF16PtrFromString(current)
		if err != nil {
			return err
		}
		attributes, err := syscall.GetFileAttributes(pointer)
		if err != nil {
			return err
		}
		if attributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return fmt.Errorf("member output staging path must not contain a reparse point: %s", current)
		}
	}
	return nil
}
