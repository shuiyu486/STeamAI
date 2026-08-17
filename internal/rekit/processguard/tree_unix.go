//go:build darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd

package processguard

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

func runTree(ctx context.Context, cmd *exec.Cmd, afterStart func() error) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
	if err := cmd.Start(); err != nil {
		return err
	}
	// Setpgid runs in the child before exec, and Start reports any setup error.
	// The returned PID is therefore the group ID even if a short-lived root has
	// already exited before the parent can probe it.
	pgid := cmd.Process.Pid
	if pgid <= 1 {
		killErr := cmd.Process.Kill()
		waitErr := cmd.Wait()
		return errors.Join(
			errors.New("process tree group ID is not safe to signal"),
			killErr,
			waitErr,
		)
	}
	if err := treeContextError(ctx); err != nil {
		killErr := unix.Kill(-pgid, unix.SIGKILL)
		if errors.Is(killErr, unix.ESRCH) {
			killErr = nil
		}
		return errors.Join(err, killErr, cmd.Wait())
	}
	if afterStart != nil {
		if err := afterStart(); err != nil {
			killErr := unix.Kill(-pgid, unix.SIGKILL)
			if errors.Is(killErr, unix.ESRCH) {
				killErr = nil
			}
			return errors.Join(
				fmt.Errorf("observe contained process before wait: %w", err),
				killErr,
				cmd.Wait(),
			)
		}
	}

	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()
	rootExited := false
	var waitErr error
	var contextErr error
	select {
	case waitErr = <-waitDone:
		rootExited = true
	case <-ctx.Done():
		contextErr = treeContextError(ctx)
	}
	killErr := unix.Kill(-pgid, unix.SIGKILL)
	if errors.Is(killErr, unix.ESRCH) {
		killErr = nil
	}
	if !rootExited {
		waitErr = <-waitDone
	}
	if errors.Is(waitErr, exec.ErrWaitDelay) {
		waitErr = nil
	}
	return errors.Join(waitErr, contextErr, killErr)
}
