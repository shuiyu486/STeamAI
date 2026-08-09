//go:build !windows

package adapterhost

import (
	"fmt"
	"os"
	"path/filepath"
)

type ownedOutput struct {
	info os.FileInfo
}

func captureOwnedOutput(file *os.File) (*ownedOutput, error) {
	if file == nil {
		return nil, fmt.Errorf("owned output file is missing")
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("owned output must be a regular file")
	}
	return &ownedOutput{info: info}, nil
}

func removeOwnedOutput(root *os.Root, rel string, owned *ownedOutput, afterIdentity func(string) error) error {
	if root == nil || owned == nil {
		return nil
	}
	current, err := root.Lstat(filepath.FromSlash(rel))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(owned.info, current) {
		return fmt.Errorf("refuse cleanup because owned output identity changed: %s", rel)
	}
	if afterIdentity != nil {
		if err := afterIdentity(rel); err != nil {
			return err
		}
	}
	return root.Remove(filepath.FromSlash(rel))
}
