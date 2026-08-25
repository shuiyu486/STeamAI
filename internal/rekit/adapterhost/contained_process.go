package adapterhost

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/processguard"
)

const (
	maxContainedStdoutBytes = 1 << 20
	maxContainedStderrBytes = 64 << 10
)

var errContainedProcessTimeout = errors.New("adapter live process exceeded dispatch runtime budget")

type limitedProcessBuffer struct {
	mu       sync.Mutex
	data     []byte
	limit    int
	exceeded bool
}

func (buffer *limitedProcessBuffer) Write(data []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	if !buffer.exceeded {
		remaining := min(buffer.limit-len(buffer.data), len(data))
		if remaining > 0 {
			buffer.data = append(buffer.data, data[:remaining]...)
		}
		buffer.exceeded = remaining < len(data)
	}
	return len(data), nil
}

func (buffer *limitedProcessBuffer) Bytes() []byte {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return append([]byte{}, buffer.data...)
}

func (buffer *limitedProcessBuffer) String() string {
	return string(buffer.Bytes())
}

func runContainedProcess(binding *processguard.ExecutableBinding, args, env []string, timeout time.Duration) ([]byte, []byte, int, error) {
	return runContainedProcessObserved(binding, args, env, timeout, nil)
}

func runContainedProcessObserved(
	binding *processguard.ExecutableBinding,
	args, env []string,
	timeout time.Duration,
	afterLaunch func(int) error,
) ([]byte, []byte, int, error) {
	return runContainedProcessObservedWithInheritedFiles(
		binding,
		args,
		env,
		timeout,
		nil,
		afterLaunch,
	)
}

func runContainedProcessObservedWithInheritedFiles(
	binding *processguard.ExecutableBinding,
	args, env []string,
	timeout time.Duration,
	inheritedFiles []*os.File,
	afterLaunch func(int) error,
) ([]byte, []byte, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binding.Path(), args...)
	if env != nil {
		cmd.Env = env
	}
	if err := processguard.ConfigureInheritedFiles(cmd, inheritedFiles); err != nil {
		return nil, nil, 0, err
	}
	if err := processguard.ConfigureSuspended(cmd, binding); err != nil {
		return nil, nil, 0, err
	}
	cmd.WaitDelay = time.Second
	stdout := limitedProcessBuffer{limit: maxContainedStdoutBytes}
	stderr := limitedProcessBuffer{limit: maxContainedStderrBytes}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, nil, 0, fmt.Errorf("start suspended adapter live process: %w", err)
	}
	childPID := cmd.Process.Pid
	var containment *processguard.Containment
	var err error
	if afterLaunch == nil {
		containment, err = processguard.ValidateContainAndResume(cmd.Process, binding)
	} else {
		containment, err = processguard.ValidateContainAndResumeObserved(
			cmd.Process,
			binding,
			func() error { return afterLaunch(childPID) },
		)
	}
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
		return stdout.Bytes(), stderr.Bytes(), childPID, errors.Join(errContainedProcessTimeout, waitErr, containmentErr)
	}
	if stdout.exceeded || stderr.exceeded {
		return stdout.Bytes(), stderr.Bytes(), childPID, errors.Join(fmt.Errorf("adapter live process exceeded bounded stdout or stderr"), waitErr, containmentErr)
	}
	if waitErr != nil {
		return stdout.Bytes(), stderr.Bytes(), childPID, errors.Join(fmt.Errorf("adapter live process failed: %w: %s", waitErr, strings.TrimSpace(stderr.String())), containmentErr)
	}
	if containmentErr != nil {
		return stdout.Bytes(), stderr.Bytes(), childPID, fmt.Errorf("close adapter process containment: %w", containmentErr)
	}
	return stdout.Bytes(), stderr.Bytes(), childPID, nil
}
