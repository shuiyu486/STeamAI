//go:build !windows && !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package statemigration

import (
	"fmt"
	"os"
)

func identityForRoot(*os.Root) (Identity, error) {
	return Identity{}, fmt.Errorf("state migration filesystem identity is unsupported on this platform")
}

func identityForFile(*os.File) (Identity, error) {
	return Identity{}, fmt.Errorf("state migration filesystem identity is unsupported on this platform")
}
