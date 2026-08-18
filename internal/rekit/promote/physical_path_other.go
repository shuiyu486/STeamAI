//go:build !windows

package promote

import (
	"path/filepath"
)

func promotePhysicalPath(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	return filepath.Abs(resolved)
}
