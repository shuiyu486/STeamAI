//go:build !windows && !linux && !darwin

package adapterhost

import (
	"fmt"
	"os"
)

func isolateOwnedOutputNoReplace(_ *os.Root, rel, _ string) error {
	return fmt.Errorf("no-replace owned output isolation is unsupported on this platform: %s", rel)
}
