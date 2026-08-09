package adapterhost

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/processguard"
)

func runContainedProcess(binding *processguard.ExecutableBinding, args, env []string, timeout time.Duration) ([]byte, []byte, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binding.Path(), args...)
	if env != nil {
		cmd.Env = env
	}
	if err := processguard.ConfigureSuspended(cmd, binding); err != nil {
		return nil, nil, 0, err
	}
	cmd.WaitDelay = time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, nil, 0, fmt.Errorf("start suspended adapter live process: %w", err)
	}
	childPID := cmd.Process.Pid
	containment, err := processguard.ValidateContainAndResume(cmd.Process, binding)
	if err != nil {
		_ = cmd.Process.Kill()
		waitErr := cmd.Wait()
		return stdout.Bytes(), stderr.Bytes(), childPID, errors.Join(fmt.Errorf("validate suspended adapter live process: %w", err), waitErr)
	}
	var closeOnce sync.Once
	var containmentErr error
	closeContainment := func() {
		closeOnce.Do(func() { containmentErr = containment.Close() })
	}
	watchDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			closeContainment()
		case <-watchDone:
		}
	}()
	waitErr := cmd.Wait()
	close(watchDone)
	closeContainment()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return stdout.Bytes(), stderr.Bytes(), childPID, errors.Join(fmt.Errorf("adapter live process exceeded dispatch runtime budget"), waitErr, containmentErr)
	}
	if waitErr != nil {
		return stdout.Bytes(), stderr.Bytes(), childPID, errors.Join(fmt.Errorf("adapter live process failed: %w: %s", waitErr, strings.TrimSpace(stderr.String())), containmentErr)
	}
	if containmentErr != nil {
		return stdout.Bytes(), stderr.Bytes(), childPID, fmt.Errorf("close adapter process containment: %w", containmentErr)
	}
	return stdout.Bytes(), stderr.Bytes(), childPID, nil
}
