package processguard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

var (
	ErrTreeOutputLimit = errors.New("process tree output limit exceeded")
	ErrTreeOutputDrain = errors.New("process tree output did not close after containment ended")
)

var treeOutputDrainTimeout = 5 * time.Second

// RunTreeOutput runs cmd with optional input and returns bounded combined
// stdout/stderr. The caller owns command construction and the context budget.
func RunTreeOutput(
	ctx context.Context,
	cmd *exec.Cmd,
	input []byte,
	maxOutputBytes int,
) ([]byte, error) {
	stdout, stderr, err := runTreeOutputs(
		ctx,
		cmd,
		input,
		maxOutputBytes,
		true,
	)
	return append(stdout, stderr...), err
}

// RunTreeOutputs is RunTreeOutput with distinct bounded stdout and stderr.
// maxOutputBytes is one shared limit across both streams.
func RunTreeOutputs(
	ctx context.Context,
	cmd *exec.Cmd,
	input []byte,
	maxOutputBytes int,
) ([]byte, []byte, error) {
	return runTreeOutputs(ctx, cmd, input, maxOutputBytes, false)
}

func runTreeOutputs(
	ctx context.Context,
	cmd *exec.Cmd,
	input []byte,
	maxOutputBytes int,
	combined bool,
) ([]byte, []byte, error) {
	if ctx == nil || cmd == nil {
		return nil, nil, fmt.Errorf("process tree context or command is missing")
	}
	if maxOutputBytes < 1 {
		return nil, nil, fmt.Errorf("process tree output limit must be positive")
	}
	if cmd.Stdin != nil || cmd.Stdout != nil || cmd.Stderr != nil {
		return nil, nil, fmt.Errorf("process tree output command must not set stdio")
	}

	inputFile, inputPath, err := treeInputFile(input)
	if err != nil {
		return nil, nil, err
	}
	if inputFile != nil {
		defer os.Remove(inputPath)
		defer inputFile.Close()
		cmd.Stdin = inputFile
	}

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return nil, nil, fmt.Errorf("create process tree stdout pipe: %w", err)
	}
	var stderrReader *os.File
	var stderrWriter *os.File
	if combined {
		stderrWriter = stdoutWriter
	} else {
		stderrReader, stderrWriter, err = os.Pipe()
		if err != nil {
			_ = stdoutReader.Close()
			_ = stdoutWriter.Close()
			return nil, nil, fmt.Errorf("create process tree stderr pipe: %w", err)
		}
	}

	runCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	budget := &treeOutputBudget{limit: maxOutputBytes, cancel: cancel}
	stdoutCollector := &treeOutputCollector{budget: budget}
	stderrCollector := &treeOutputCollector{budget: budget}
	copyCount := 1
	copyDone := make(chan error, 2)
	go func() { copyDone <- stdoutCollector.copyFrom(stdoutReader) }()
	if !combined {
		copyCount++
		go func() { copyDone <- stderrCollector.copyFrom(stderrReader) }()
	}
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter
	runErr := RunTree(runCtx, cmd)
	stdoutCloseErr := stdoutWriter.Close()
	var stderrCloseErr error
	if !combined {
		stderrCloseErr = stderrWriter.Close()
	}
	if cmd.Process == nil {
		_ = stdoutReader.Close()
		if stderrReader != nil {
			_ = stderrReader.Close()
		}
	}
	copyErr := waitTreeOutputCopies(
		copyDone,
		copyCount,
		stdoutReader,
		stderrReader,
		treeOutputDrainTimeout,
	)
	stdout := stdoutCollector.result()
	stderr := stderrCollector.result()
	limitErr := budget.err()
	return stdout, stderr, treeOutputError(
		runErr,
		stdoutCloseErr,
		stderrCloseErr,
		copyErr,
		limitErr,
	)
}

func waitTreeOutputCopies(
	copyDone <-chan error,
	copyCount int,
	stdoutReader,
	stderrReader *os.File,
	timeout time.Duration,
) error {
	var copyErr error
	remaining := copyCount
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for remaining > 0 {
		select {
		case err := <-copyDone:
			copyErr = errors.Join(copyErr, err)
			remaining--
		case <-timer.C:
			stdoutCloseErr := stdoutReader.Close()
			var stderrCloseErr error
			if stderrReader != nil {
				stderrCloseErr = stderrReader.Close()
			}
			copyErr = errors.Join(
				copyErr,
				stdoutCloseErr,
				stderrCloseErr,
				fmt.Errorf("%w: timeout=%s", ErrTreeOutputDrain, timeout),
			)
			for remaining > 0 {
				copyErr = errors.Join(copyErr, <-copyDone)
				remaining--
			}
		}
	}
	return copyErr
}

func treeOutputError(runErr, stdoutCloseErr, stderrCloseErr, copyErr, limitErr error) error {
	if limitErr != nil && errors.Is(runErr, ErrTreeOutputLimit) {
		limitErr = nil
	}
	return errors.Join(runErr, stdoutCloseErr, stderrCloseErr, copyErr, limitErr)
}

func treeInputFile(input []byte) (*os.File, string, error) {
	if input == nil {
		return nil, "", nil
	}
	file, err := os.CreateTemp("", "steamai-process-input-*.tmp")
	if err != nil {
		return nil, "", fmt.Errorf("create process tree input: %w", err)
	}
	path := file.Name()
	fail := func(cause error) (*os.File, string, error) {
		return nil, "", errors.Join(cause, file.Close(), os.Remove(path))
	}
	if err := file.Chmod(0o600); err != nil {
		return fail(fmt.Errorf("protect process tree input: %w", err))
	}
	if _, err := file.Write(input); err != nil {
		return fail(fmt.Errorf("write process tree input: %w", err))
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fail(fmt.Errorf("rewind process tree input: %w", err))
	}
	return file, path, nil
}

type treeOutputBudget struct {
	mu       sync.Mutex
	used     int
	limit    int
	exceeded bool
	cancel   context.CancelCauseFunc
}

func (budget *treeOutputBudget) append(target *[]byte, data []byte) {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	remaining := budget.limit - budget.used
	if remaining > 0 {
		count := min(remaining, len(data))
		*target = append(*target, data[:count]...)
		budget.used += count
	}
	if len(data) > remaining && !budget.exceeded {
		budget.exceeded = true
		budget.cancel(ErrTreeOutputLimit)
	}
}

func (budget *treeOutputBudget) err() error {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	if !budget.exceeded {
		return nil
	}
	return fmt.Errorf("%w: limit=%d", ErrTreeOutputLimit, budget.limit)
}

type treeOutputCollector struct {
	budget *treeOutputBudget
	data   []byte
}

func (collector *treeOutputCollector) copyFrom(reader *os.File) error {
	if reader == nil {
		return nil
	}
	defer reader.Close()
	buffer := make([]byte, 32<<10)
	for {
		count, err := reader.Read(buffer)
		if count > 0 {
			collector.budget.append(&collector.data, buffer[:count])
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func (collector *treeOutputCollector) result() []byte {
	collector.budget.mu.Lock()
	defer collector.budget.mu.Unlock()
	return append([]byte(nil), collector.data...)
}
