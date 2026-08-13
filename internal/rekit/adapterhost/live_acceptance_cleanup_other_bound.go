//go:build !windows

package adapterhost

import (
	"fmt"
	"os"
	"path/filepath"
)

func removeEmptyLiveAcceptanceQuarantine(
	identity *liveAcceptanceCaseIdentity,
	quarantine string,
	root *os.Root,
) error {
	opened, err := root.Lstat(".")
	if err != nil || !opened.IsDir() || !os.SameFile(identity.caseInfo, opened) {
		return fmt.Errorf("adapter live quarantined case identity changed before final removal: %s", filepath.Join(identity.parentPath, quarantine))
	}
	if err := identity.validateNamedRoot(quarantine, true); err != nil {
		return err
	}
	return identity.parent.Remove(quarantine)
}
