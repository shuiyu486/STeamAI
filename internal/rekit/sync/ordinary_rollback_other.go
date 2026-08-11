//go:build !windows

package sync

import (
	"fmt"
	"os"
)

func ordinaryInitRollbackCapabilityCheck() error {
	return fmt.Errorf("ordinary init create-only rollback is unsupported on this platform")
}

func ordinaryInitOpenRollbackHandle(_ string, _ bool) (*os.File, error) {
	return nil, fmt.Errorf("ordinary init rollback exact-object handles are unsupported on this platform")
}

func ordinaryInitRemoveExact(_ *os.Root, rel, _ string, _ *os.File, _ os.FileInfo, _ bool, _ func(string) error) (bool, error) {
	return false, fmt.Errorf("ordinary init rollback exact-object removal is unsupported on this platform: %s", rel)
}
