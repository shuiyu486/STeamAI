//go:build linux

package adapterhost

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func isolateOwnedOutputNoReplace(root *os.Root, rel, quarantine string) error {
	parent := filepath.Dir(filepath.FromSlash(rel))
	quarantineParent := filepath.Dir(filepath.FromSlash(quarantine))
	if parent != quarantineParent {
		return os.ErrInvalid
	}
	dir, err := root.Open(parent)
	if err != nil {
		return err
	}
	defer dir.Close()
	return unix.Renameat2(
		int(dir.Fd()),
		filepath.Base(filepath.FromSlash(rel)),
		int(dir.Fd()),
		filepath.Base(filepath.FromSlash(quarantine)),
		unix.RENAME_NOREPLACE,
	)
}
