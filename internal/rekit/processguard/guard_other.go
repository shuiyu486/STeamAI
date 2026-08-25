//go:build !windows

package processguard

import (
	"fmt"
	"os"
	"os/exec"
)

type ExecutableBinding struct{}

type Containment struct{}

func LockExecutable(path string, maxBytes int64) (*ExecutableBinding, error) {
	return nil, fmt.Errorf("verified suspended executable launch is available only on Windows: %s", path)
}

func (binding *ExecutableBinding) Path() string { return "" }

func (binding *ExecutableBinding) SHA256() string { return "" }

func (binding *ExecutableBinding) Validate() error {
	return fmt.Errorf("verified suspended executable launch is available only on Windows")
}

func (binding *ExecutableBinding) Close() error { return nil }

func (containment *Containment) Close() error { return nil }

func ConfigureSuspended(cmd *exec.Cmd, binding *ExecutableBinding) error {
	return fmt.Errorf("verified suspended executable launch is available only on Windows")
}

func ConfigureInheritedFiles(cmd *exec.Cmd, files []*os.File) error {
	if len(files) == 0 {
		return nil
	}
	return fmt.Errorf("inherited child file handles are available only on Windows")
}

func ValidateContainAndResume(process *os.Process, binding *ExecutableBinding) (*Containment, error) {
	return nil, fmt.Errorf("verified suspended executable launch is available only on Windows")
}

func ValidateContainAndResumeObserved(process *os.Process, binding *ExecutableBinding, beforeResume func() error) (*Containment, error) {
	return nil, fmt.Errorf("verified suspended executable launch is available only on Windows")
}

func ValidateContainAndResumeAllowBreakaway(process *os.Process, binding *ExecutableBinding) (*Containment, error) {
	return nil, fmt.Errorf("verified suspended executable launch is available only on Windows")
}
