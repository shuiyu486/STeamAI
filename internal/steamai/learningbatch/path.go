package learningbatch

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func pathWithin(path, root string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func requirePlainPath(root, path string, directory bool) error {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if !pathWithin(path, root) {
		return errors.New("路径越出允许根目录")
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	current := root
	if err := requirePlainDirectory(current); err != nil {
		return err
	}
	parts := strings.Split(rel, string(filepath.Separator))
	for index, part := range parts {
		if part == "." || part == "" {
			continue
		}
		current = filepath.Join(current, part)
		last := index == len(parts)-1
		if last && !directory {
			if err := requirePlainFile(current); err != nil {
				return err
			}
			continue
		}
		if err := requirePlainDirectory(current); err != nil {
			return err
		}
	}
	return nil
}

func requirePlainDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("不是普通目录")
	}
	return rejectReparse(path)
}

func requirePlainFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("不是普通文件")
	}
	return rejectReparse(path)
}
