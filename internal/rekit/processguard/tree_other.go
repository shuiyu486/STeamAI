//go:build !windows && !darwin && !dragonfly && !freebsd && !illumos && !linux && !netbsd && !openbsd

package processguard

import (
	"context"
	"os/exec"
)

func runTree(context.Context, *exec.Cmd, func() error) error {
	return ErrTreeContainmentUnsupported
}
