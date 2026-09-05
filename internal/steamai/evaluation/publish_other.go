//go:build !windows && !linux

package evaluation

import (
	"errors"
	"os"
	"path/filepath"
)

func publishDirectoryNoReplace(staging, target string) error {
	claim := target + ".claim"
	if err := os.Link(filepath.Join(staging, "manifest.json"), claim); err != nil {
		if os.IsExist(err) {
			return errors.New("evaluation run 已存在或正在发布")
		}
		return err
	}
	defer os.Remove(claim)
	if _, err := os.Lstat(target); err == nil {
		return errors.New("evaluation run 已存在")
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Rename(staging, target)
}
