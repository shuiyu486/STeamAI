//go:build !windows && !darwin && !dragonfly && !freebsd && !illumos && !linux && !netbsd && !openbsd

package sessionhost

import (
	"fmt"
	"os/exec"
	"runtime"
)

type supervisionOwnerLease struct{}

func configureSupervisorCommand(cmd *exec.Cmd) {
	if cmd != nil && cmd.Err == nil {
		cmd.Err = unsupportedSupervisionOwnerError()
	}
}

func acquireSupervisionOwner(_ string, _ bool) (*supervisionOwnerLease, bool, error) {
	return nil, false, unsupportedSupervisionOwnerError()
}

func supervisionOwnerBusy(_ string) (bool, error) {
	return false, unsupportedSupervisionOwnerError()
}

func unsupportedSupervisionOwnerError() error {
	return fmt.Errorf("Claude supervision owner leases are unsupported on %s/%s", runtime.GOOS, runtime.GOARCH)
}

func (*supervisionOwnerLease) Close() error {
	return nil
}
