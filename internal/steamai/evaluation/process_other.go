//go:build !windows

package evaluation

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

type armProcessControl struct{}

func configureArmProcess(cmd *exec.Cmd) (*armProcessControl, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return &armProcessControl{}, nil
}

func (*armProcessControl) attach(*exec.Cmd) error { return nil }

func (*armProcessControl) cancel(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return os.ErrProcessDone
	}
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}

func (*armProcessControl) close() error { return nil }
