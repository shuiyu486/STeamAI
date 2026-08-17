//go:build !windows

package sessionhost

import "os/exec"

func configureContainedSupervisorCommandForTest(_ *exec.Cmd) {}
