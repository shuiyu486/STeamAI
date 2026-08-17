//go:build windows

package processguard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRunTreeOutputHelper(t *testing.T) {
	mode := os.Getenv("REKIT_PROCESS_TREE_OUTPUT_HELPER")
	if mode == "" {
		return
	}
	switch mode {
	case "separate":
		_, _ = fmt.Fprint(os.Stdout, "stdout sentinel\n")
		_, _ = fmt.Fprint(os.Stderr, "stderr sentinel\n")
	case "input":
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			os.Exit(91)
		}
		_, _ = os.Stdout.Write(data)
	case "overflow":
		pidPath := os.Getenv("REKIT_PROCESS_TREE_OUTPUT_PID")
		cmd := exec.Command(
			"powershell.exe",
			"-NoProfile",
			"-NonInteractive",
			"-Command",
			"Start-Sleep -Seconds 30",
		)
		if err := cmd.Start(); err != nil {
			os.Exit(92)
		}
		if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
			_ = cmd.Process.Kill()
			os.Exit(93)
		}
		_, _ = os.Stdout.Write([]byte(strings.Repeat("x", 4096)))
		for {
			time.Sleep(time.Hour)
		}
	default:
		os.Exit(94)
	}
}

func TestRunTreeOutputsSeparatesStreams(t *testing.T) {
	cmd := processTreeOutputHelperCommand("separate")
	stdout, stderr, err := RunTreeOutputs(
		context.Background(),
		cmd,
		nil,
		1<<20,
	)
	if err != nil || !strings.HasPrefix(string(stdout), "stdout sentinel\n") ||
		string(stderr) != "stderr sentinel\n" {
		t.Fatalf("separate output: stdout=%q stderr=%q err=%v", stdout, stderr, err)
	}
}

func TestRunTreeOutputPassesExactInput(t *testing.T) {
	input := []byte("exact input\x00bytes\n")
	cmd := processTreeOutputHelperCommand("input")
	stdout, stderr, err := RunTreeOutputs(
		context.Background(),
		cmd,
		input,
		1<<20,
	)
	if err != nil || string(stdout) != string(input)+"PASS\n" || len(stderr) != 0 {
		t.Fatalf("input output: stdout=%q stderr=%q err=%v", stdout, stderr, err)
	}
}

func TestRunTreeOutputLimitTerminatesDescendants(t *testing.T) {
	pidPath := filepath.Join(t.TempDir(), "descendant.pid")
	cmd := processTreeOutputHelperCommand("overflow")
	cmd.Env = append(cmd.Env, "REKIT_PROCESS_TREE_OUTPUT_PID="+pidPath)
	started := time.Now()
	output, err := RunTreeOutput(context.Background(), cmd, nil, 1024)
	if !errors.Is(err, ErrTreeOutputLimit) || len(output) != 1024 {
		t.Fatalf("bounded output: bytes=%d err=%v", len(output), err)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("output overflow did not terminate tree promptly: %s", elapsed)
	}
	requireRecordedProcessDead(t, pidPath)
}

func TestTreeOutputErrorRetainsContainmentFailures(t *testing.T) {
	containmentErr := errors.New("containment cleanup failed")
	err := treeOutputError(
		errors.Join(ErrTreeOutputLimit, containmentErr),
		nil,
		nil,
		nil,
		fmt.Errorf("%w: limit=1024", ErrTreeOutputLimit),
	)
	if !errors.Is(err, ErrTreeOutputLimit) || !errors.Is(err, containmentErr) {
		t.Fatalf("tree output error lost a cause: %v", err)
	}
}

func TestWaitTreeOutputCopiesClosesReadersAtDeadline(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	copyDone := make(chan error, 1)
	go func() {
		buffer := make([]byte, 1)
		_, err := reader.Read(buffer)
		copyDone <- err
	}()
	started := time.Now()
	runErr := waitTreeOutputCopies(copyDone, 1, reader, nil, 25*time.Millisecond)
	if !errors.Is(runErr, ErrTreeOutputDrain) {
		t.Fatalf("escaped output writer did not report bounded drain: %v", runErr)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("bounded output drain returned too slowly: %s", elapsed)
	}
}

func processTreeOutputHelperCommand(mode string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=TestRunTreeOutputHelper")
	cmd.Env = append(
		os.Environ(),
		"REKIT_PROCESS_TREE_OUTPUT_HELPER="+mode,
	)
	return cmd
}
