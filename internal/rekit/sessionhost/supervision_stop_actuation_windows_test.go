//go:build windows

package sessionhost

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
)

const (
	supervisionStopActuationHelperMode   = "STEAMAI_SESSIONHOST_STOP_ACTUATION_HELPER"
	supervisionStopActuationHelperMarker = "STEAMAI_SESSIONHOST_STOP_ACTUATION_MARKER"
)

func TestSupervisionStopActuationWindowsHelper(t *testing.T) {
	if os.Getenv(supervisionStopActuationHelperMode) != "1" {
		return
	}
	if err := os.WriteFile(os.Getenv(supervisionStopActuationHelperMarker), []byte("started\n"), 0o600); err != nil {
		os.Exit(91)
	}
	time.Sleep(30 * time.Second)
	os.Exit(92)
}

func startSupervisionStopActuationTestProcess(cmd *exec.Cmd) (*claudeProcessContainment, error) {
	if cmd == nil {
		return nil, fmt.Errorf("stop actuation test command is missing")
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000004}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	containment, err := containAndResumeSupervisionStopActuationTestProcess(cmd.Process)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, err
	}
	return containment, nil
}

func containAndResumeSupervisionStopActuationTestProcess(process *os.Process) (*claudeProcessContainment, error) {
	if process == nil {
		return nil, fmt.Errorf("stop actuation test process is missing")
	}
	const (
		processSuspendResume = 0x0800
		processSetQuota      = 0x0100
		processTerminate     = 0x0001
	)
	processHandle, err := syscall.OpenProcess(
		processSuspendResume|processSetQuota|processTerminate,
		false,
		uint32(process.Pid),
	)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(processHandle)
	job, _, callErr := createJobObjectW.Call(0, 0)
	if job == 0 {
		return nil, fmt.Errorf("create stop actuation test containment job: %v", callErr)
	}
	containment := &claudeProcessContainment{handle: syscall.Handle(job)}
	limits := jobObjectExtendedLimitInformation{}
	limits.BasicLimitInformation.LimitFlags = 0x00002000
	set, _, callErr := setInformationJobObject.Call(
		job,
		9,
		uintptr(unsafe.Pointer(&limits)),
		unsafe.Sizeof(limits),
	)
	if set == 0 {
		_ = containment.Close()
		return nil, fmt.Errorf("set stop actuation test containment limits: %v", callErr)
	}
	assigned, _, callErr := assignProcessToJobObject.Call(job, uintptr(processHandle))
	if assigned == 0 {
		_ = containment.Close()
		return nil, fmt.Errorf("assign stop actuation test process to containment: %v", callErr)
	}
	status, _, callErr := ntResumeProcess.Call(uintptr(processHandle))
	if int32(status) < 0 {
		_ = containment.Close()
		return nil, fmt.Errorf("resume stop actuation test process: NTSTATUS 0x%x: %v", uint32(status), callErr)
	}
	return containment, nil
}

func TestSupervisionStopActuationClosesExactWindowsJob(t *testing.T) {
	caseRoot, scope := supervisionStopActuationFixture(t)
	current, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "started.txt")
	cmd := exec.Command(current, "-test.run=^TestSupervisionStopActuationWindowsHelper$")
	cmd.Env = append(os.Environ(),
		supervisionStopActuationHelperMode+"=1",
		supervisionStopActuationHelperMarker+"="+marker,
	)
	containment, err := startSupervisionStopActuationTestProcess(cmd)
	if err != nil {
		t.Fatal(err)
	}
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()
	waited := false
	defer func() {
		_ = containment.Close()
		if waited {
			return
		}
		select {
		case <-waitDone:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-waitDone
		}
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatal("contained helper did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}

	resultCh := make(chan supervisionStopActuationResult, 1)
	go func() {
		resultCh <- watchSupervisionStopActuation(
			context.Background(),
			scope,
			containment.Close,
		)
	}()
	stopped := applyClaudeLaunchControlForTest(
		t,
		caseRoot,
		scope.spec.LaunchControl.Lane,
		executioncontrol.ActionStop,
		"2026-08-19T01:03:00Z",
	)
	result := waitForSupervisionStopActuation(t, resultCh)
	if result.Err != nil || !validClaudeLaunchSHA256(result.ObservationSHA256) {
		t.Fatalf("exact Windows Job stop actuation result=%+v", result)
	}
	select {
	case <-waitDone:
		waited = true
	case <-time.After(5 * time.Second):
		t.Fatal("contained helper survived exact Windows Job close")
	}
	if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
		t.Fatalf("contained helper has no terminal process state: %+v", cmd.ProcessState)
	}

	_, observationPath := supervisionStopActuationArtifactPaths(stopped.ControlGeneration)
	observation, _, ok, err := readSupervisionStopActuationObservation(scope, observationPath)
	if err != nil || !ok || observation.Outcome != "owned-containment-closed" ||
		!observation.ContainmentCloseAttempted || !observation.ContainmentCloseSucceeded ||
		!observation.NoProcessTerminationClaim {
		t.Fatalf("exact Windows Job observation ok=%t err=%v value=%+v", ok, err, observation)
	}
	inspection, err := executioncontrol.Inspect(caseRoot, stopped.Lane)
	if err != nil || inspection.State != executioncontrol.StateStopped ||
		inspection.CurrentGeneration != stopped.ControlGeneration {
		t.Fatalf("exact Windows Job actuation changed stopped head: inspection=%+v err=%v", inspection, err)
	}
}
