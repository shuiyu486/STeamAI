package processguard

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

var ErrTreeContainmentUnsupported = errors.New("process tree containment is unsupported on this platform")

// RunTree starts cmd inside the platform process-tree containment primitive.
// The command must not have been started and must not carry caller-owned
// platform process attributes.
func RunTree(ctx context.Context, cmd *exec.Cmd) error {
	return runTreeObserved(ctx, cmd, nil)
}

// RunTreeObserved is RunTree with one callback after the child has started and
// platform containment is established. On Windows the child is still suspended.
func RunTreeObserved(ctx context.Context, cmd *exec.Cmd, afterStart func() error) error {
	if afterStart == nil {
		return fmt.Errorf("process tree start observer is missing")
	}
	return runTreeObserved(ctx, cmd, afterStart)
}

func runTreeObserved(ctx context.Context, cmd *exec.Cmd, afterStart func() error) error {
	if ctx == nil || cmd == nil {
		return fmt.Errorf("process tree context or command is missing")
	}
	if cmd.Process != nil || cmd.ProcessState != nil {
		return fmt.Errorf("process tree command has already started")
	}
	if cmd.SysProcAttr != nil {
		return fmt.Errorf("process tree command must not set platform process attributes")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !directProcessFile(cmd.Stdin) ||
		!directProcessFile(cmd.Stdout) ||
		!directProcessFile(cmd.Stderr) {
		return fmt.Errorf("process tree stdio must use direct files")
	}
	if cmd.WaitDelay != 0 {
		return fmt.Errorf("process tree command must not set WaitDelay")
	}
	return runTree(ctx, cmd, afterStart)
}

func directProcessFile(value any) bool {
	if value == nil {
		return true
	}
	_, ok := value.(*os.File)
	return ok
}

func treeContextError(ctx context.Context) error {
	if cause := context.Cause(ctx); cause != nil {
		return cause
	}
	return ctx.Err()
}
