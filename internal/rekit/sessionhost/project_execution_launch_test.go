package sessionhost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectlock"
)

func requireDurableHandoffForSessionhostTest(t *testing.T) {
	t.Helper()
	if !projectexecution.DurableHandoffSupported() {
		t.Skip("durable supervisor handoff requires handle-bound exact filesystem mutation")
	}
}

const (
	projectExecutionHelperRoleEnv      = "REKIT_SESSIONHOST_EXECUTION_HELPER_ROLE"
	projectExecutionHelperReadyEnv     = "REKIT_SESSIONHOST_EXECUTION_HELPER_READY"
	projectExecutionHelperReleaseEnv   = "REKIT_SESSIONHOST_EXECUTION_HELPER_RELEASE"
	projectExecutionHelperSpecEnv      = "REKIT_SESSIONHOST_EXECUTION_HELPER_SPEC"
	projectExecutionHelperSpecSHAEnv   = "REKIT_SESSIONHOST_EXECUTION_HELPER_SPEC_SHA256"
	projectExecutionHelperBeforeEnv    = "REKIT_SESSIONHOST_EXECUTION_HELPER_BEFORE_ACQUIRE"
	projectExecutionHelperChildDoneEnv = "REKIT_SESSIONHOST_EXECUTION_HELPER_CHILD_DONE"
)

func TestMain(m *testing.M) {
	if specPath, specSHA, ok := projectExecutionInternalSupervisorArgs(os.Args); ok {
		supervisorChildStageHook = projectExecutionSupervisorChildHook
		_ = os.Setenv(projectExecutionHelperRoleEnv, "claude")
		err := RunSupervisorChild(context.Background(), specPath, specSHA)
		if done := os.Getenv(projectExecutionHelperChildDoneEnv); done != "" {
			_ = os.WriteFile(done, []byte(errorText(err)), 0o600)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		os.Exit(0)
	}
	if os.Getenv(projectExecutionHelperRoleEnv) == "claude" {
		os.Exit(runProjectExecutionClaudeHelper())
	}
	os.Exit(m.Run())
}

func projectExecutionInternalSupervisorArgs(args []string) (string, string, bool) {
	var specPath, specSHA string
	for index := range len(args) {
		switch args[index] {
		case "-internal-supervisor":
			if index+1 < len(args) {
				specPath = args[index+1]
			}
		case "-internal-supervisor-sha256":
			if index+1 < len(args) {
				specSHA = args[index+1]
			}
		}
	}
	return specPath, specSHA, specPath != "" && specSHA != ""
}

func projectExecutionSupervisorChildHook(stage string) error {
	if stage != "before-execution-acquire" {
		return nil
	}
	readyPath := os.Getenv(projectExecutionHelperBeforeEnv)
	releasePath := os.Getenv(projectExecutionHelperReleaseEnv)
	if readyPath == "" || releasePath == "" {
		return fmt.Errorf("project execution supervisor child hook binding is incomplete")
	}
	if err := os.WriteFile(readyPath, fmt.Appendf(nil, "%d\n", os.Getpid()), 0o600); err != nil {
		return err
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := os.Lstat(releasePath); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting to release supervisor child before execution acquire")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRunClaudeHoldsSharedExecutionLeaseAcrossLaunch(t *testing.T) {
	caseRoot, opt, pkg, readyPath, releasePath := projectExecutionLaunchFixture(t)
	t.Setenv(projectExecutionHelperRoleEnv, "claude")
	t.Setenv(projectExecutionHelperReadyEnv, readyPath)
	t.Setenv(projectExecutionHelperReleaseEnv, releasePath)
	t.Cleanup(func() { _ = os.WriteFile(releasePath, nil, 0o600) })

	runDone := make(chan claudeRun, 1)
	go func() {
		runDone <- runClaude(
			context.Background(),
			opt,
			pkg,
			pkg.Launch.Attempt.Session,
			nil,
		)
	}()
	waitForProjectExecutionFile(t, readyPath)

	exclusiveDone := acquireProjectExecutionExclusive(caseRoot)
	assertProjectExecutionExclusiveBlocked(t, exclusiveDone)
	if err := os.WriteFile(releasePath, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	lease := waitForProjectExecutionExclusive(t, exclusiveDone)
	if err := lease.Unlock(); err != nil {
		t.Fatal(err)
	}
	select {
	case run := <-runDone:
		if !run.success() || !run.started || run.sessionID != pkg.Launch.Attempt.Session {
			t.Fatalf("Claude launch did not complete under the shared execution lease: %+v", run)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Claude launch did not finish after its helper was released")
	}
}

func TestDetachedSupervisorRejectsLateChildAfterParentHardKill(t *testing.T) {
	requireDurableHandoffForSessionhostTest(t)
	caseRoot, opt, pkg, _, childReleasePath := projectExecutionLaunchFixture(t)
	opt.ExpectedClaudeExecutableSHA256 = strings.Repeat("e", 64)
	opt.ExpectedClaudeExecutablePublisher = liveAcceptanceClaudePublisher
	paths, _, specData, _, err := prepareSupervision(opt, pkg, pkg.Launch.Attempt.Session)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(paths.root) })
	signals := t.TempDir()
	parentInputPath := filepath.Join(signals, "parent-spec.json")
	if err := os.WriteFile(parentInputPath, specData, 0o600); err != nil {
		t.Fatal(err)
	}
	beforeAcquirePath := filepath.Join(signals, "before-acquire")
	childDonePath := filepath.Join(signals, "child-done")

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	parent := exec.Command(
		executable,
		"-test.run=^TestProjectExecutionSupervisedParentHelperProcess$",
		"-test.count=1",
	)
	parent.Env = append(
		os.Environ(),
		projectExecutionHelperRoleEnv+"=parent",
		projectExecutionHelperSpecEnv+"="+parentInputPath,
		projectExecutionHelperBeforeEnv+"="+beforeAcquirePath,
		projectExecutionHelperChildDoneEnv+"="+childDonePath,
		projectExecutionHelperReleaseEnv+"="+childReleasePath,
	)
	var stdout, stderr bytes.Buffer
	parent.Stdout = &stdout
	parent.Stderr = &stderr
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	parentDone := false
	t.Cleanup(func() {
		_ = os.WriteFile(childReleasePath, nil, 0o600)
		if !parentDone && parent.Process != nil {
			_ = parent.Process.Kill()
			_ = parent.Wait()
		}
	})
	waitForProjectExecutionFile(t, beforeAcquirePath)
	if _, err := os.Lstat(paths.claimed); !os.IsNotExist(err) {
		t.Fatalf("supervisor child claimed before the controlled acquire boundary: %v", err)
	}

	if err := parent.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := parent.Wait(); err == nil {
		t.Fatal("hard-killed parent exited successfully")
	}
	parentDone = true

	exclusive, err := projectexecution.AcquireExclusive(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := exclusive.Unlock(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(childReleasePath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	waitForProjectExecutionFile(t, childDonePath)
	childResult, err := os.ReadFile(childDonePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(childResult), "permanently canceled") {
		t.Fatalf("late supervisor child result=%q\nparent stdout:\n%s\nparent stderr:\n%s", childResult, stdout.String(), stderr.String())
	}
	if _, err := os.Lstat(paths.claimed); !os.IsNotExist(err) {
		t.Fatalf("late supervisor child published a claim after maintenance cancellation: %v", err)
	}
	if _, err := os.Lstat(paths.started); !os.IsNotExist(err) {
		t.Fatalf("late supervisor child launched Claude after maintenance cancellation: %v", err)
	}
	if _, err := os.Lstat(paths.terminal); !os.IsNotExist(err) {
		t.Fatalf("late supervisor child published terminal state after maintenance cancellation: %v", err)
	}
}

func TestLegacyDetachedSupervisorRunsDoNotPublishProjectExecutionHandoff(t *testing.T) {
	previousCommandHook := supervisorChildCommandTestHook
	supervisorChildCommandTestHook = configureContainedSupervisorCommandForTest
	t.Cleanup(func() { supervisorChildCommandTestHook = previousCommandHook })
	caseRoot, opt, first, beforeAcquirePath, releasePath :=
		projectExecutionLaunchFixture(t)
	if err := os.Rename(
		filepath.Join(caseRoot, ".steamai"),
		filepath.Join(caseRoot, ".rekit"),
	); err != nil {
		t.Fatal(err)
	}
	first.Launch.Input.Path = ".rekit/execution-lease-input.json"
	opt.ExpectedClaudeExecutableSHA256 = strings.Repeat("e", 64)
	opt.ExpectedClaudeExecutablePublisher = liveAcceptanceClaudePublisher
	t.Setenv(projectExecutionHelperBeforeEnv, beforeAcquirePath)
	t.Setenv(projectExecutionHelperReleaseEnv, releasePath)
	t.Setenv(projectExecutionHelperChildDoneEnv, "")
	if err := os.WriteFile(releasePath, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	supervisionRootPath, err := supervisionRoot(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(supervisionRootPath) })
	lockRoot, err := projectlock.WorkstreamRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectKey, err := projectlock.CanonicalProjectKey(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	pendingHandoff := filepath.Join(
		lockRoot,
		"case-"+projectKey+".execution-v1.handoff.json",
	)
	assertNoPendingHandoff := func(stage string) {
		t.Helper()
		if _, err := os.Lstat(pendingHandoff); err == nil {
			t.Fatalf("%s left a legacy project execution handoff pending", stage)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	run := func(stage string, pkg mission.CurrentLoopExternalSessionHarnessPackage) {
		t.Helper()
		paths, _, _, _, err := prepareSupervision(
			opt,
			pkg,
			pkg.Launch.Attempt.Session,
		)
		if err != nil {
			t.Fatal(err)
		}
		_, launched, err := supervisedClaudeRun(
			context.Background(),
			opt,
			pkg,
			pkg.Launch.Attempt.Session,
			nil,
		)
		if err != nil || !launched {
			t.Fatalf("%s legacy supervised run launched=%t err=%v", stage, launched, err)
		}
		if _, err := os.Lstat(paths.terminal); err != nil {
			t.Fatalf("%s legacy supervised run did not reach terminal state: %v", stage, err)
		}
		assertNoPendingHandoff(stage)
	}

	run("first", first)
	second := first
	secondLaunch := *first.Launch
	second.Launch = &secondLaunch
	second.Launch.Attempt.AttemptID = "execution-lease-attempt-2"
	second.Launch.Attempt.AttemptSHA256 = strings.Repeat("f", 64)
	second.Launch.Attempt.Generation = 2
	second.Launch.Attempt.Session = "execution-lease-session-2"
	run("second", second)
}

func TestProjectExecutionSupervisedParentHelperProcess(t *testing.T) {
	if os.Getenv(projectExecutionHelperRoleEnv) != "parent" {
		return
	}
	supervisorChildCommandTestHook = configureContainedSupervisorCommandForTest
	data, err := os.ReadFile(os.Getenv(projectExecutionHelperSpecEnv))
	if err != nil {
		t.Fatal(err)
	}
	var spec supervisionSpec
	if err := strictJSON(data, &spec); err != nil {
		t.Fatal(err)
	}
	pkg := spec.Execution.packageForRun()
	opt := Options{
		Target:                            spec.Target,
		Pack:                              spec.Pack,
		ClaudePath:                        spec.ClaudePath,
		ExpectedClaudeExecutableSHA256:    spec.ExpectedClaudeExecutableSHA256,
		ExpectedClaudeExecutablePublisher: spec.ExpectedClaudeExecutablePublisher,
		Timeout:                           time.Duration(spec.TimeoutNanos),
	}
	lease, err := projectexecution.AcquireShared(spec.Target)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Unlock()
	opt.projectExecutionLease = lease
	_, launched, err := supervisedClaudeRun(
		context.Background(),
		opt,
		pkg,
		pkg.Launch.Attempt.Session,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !launched {
		t.Fatal("supervised parent helper did not launch the production child")
	}
}

func TestSupervisorChildKeepsSharedExecutionLeaseAfterParentRelease(t *testing.T) {
	requireDurableHandoffForSessionhostTest(t)
	caseRoot, opt, pkg, readyPath, releasePath := projectExecutionLaunchFixture(t)
	paths, _, specData, specSHA, err := prepareSupervision(
		opt,
		pkg,
		pkg.Launch.Attempt.Session,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(paths.root) })
	if err := os.WriteFile(paths.spec, specData, 0o600); err != nil {
		t.Fatal(err)
	}
	handoff, err := projectexecution.NewHandoff(
		caseRoot,
		paths.runID,
		specSHA,
		pkg.Launch.Attempt.Session,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := projectexecution.PublishHandoff(caseRoot, handoff); err != nil {
		t.Fatal(err)
	}
	defer projectexecution.CancelHandoff(caseRoot, handoff)

	parentLease, err := projectexecution.AcquireShared(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	parentHeld := true
	t.Cleanup(func() {
		if parentHeld {
			_ = parentLease.Unlock()
		}
		_ = os.WriteFile(releasePath, nil, 0o600)
	})

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(
		executable,
		"-test.run=^TestProjectExecutionSupervisorHelperProcess$",
		"-test.count=1",
	)
	cmd.Env = append(
		os.Environ(),
		projectExecutionHelperRoleEnv+"=supervisor",
		projectExecutionHelperReadyEnv+"="+readyPath,
		projectExecutionHelperReleaseEnv+"="+releasePath,
		projectExecutionHelperSpecEnv+"="+paths.spec,
		projectExecutionHelperSpecSHAEnv+"="+specSHA,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	processDone := false
	t.Cleanup(func() {
		if !processDone && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	waitForProjectExecutionFile(t, readyPath)
	waitForProjectExecutionFile(t, paths.claimed)

	if err := parentLease.Unlock(); err != nil {
		t.Fatal(err)
	}
	parentHeld = false
	exclusiveDone := acquireProjectExecutionExclusive(caseRoot)
	assertProjectExecutionExclusiveBlocked(t, exclusiveDone)

	if err := os.WriteFile(releasePath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()
	select {
	case err := <-waitDone:
		processDone = true
		if err != nil {
			t.Fatalf(
				"supervisor helper failed after release: %v\nstdout:\n%s\nstderr:\n%s",
				err,
				stdout.String(),
				stderr.String(),
			)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("supervisor helper did not finish after its Claude child was released")
	}

	lease := waitForProjectExecutionExclusive(t, exclusiveDone)
	if err := lease.Unlock(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(paths.terminal); err != nil {
		t.Fatalf("supervisor child did not publish its terminal receipt: %v", err)
	}
}

func TestProjectExecutionSupervisorHelperProcess(t *testing.T) {
	if os.Getenv(projectExecutionHelperRoleEnv) != "supervisor" {
		return
	}
	if err := os.Setenv(projectExecutionHelperRoleEnv, "claude"); err != nil {
		t.Fatal(err)
	}
	if err := RunSupervisorChild(
		context.Background(),
		os.Getenv(projectExecutionHelperSpecEnv),
		os.Getenv(projectExecutionHelperSpecSHAEnv),
	); err != nil {
		t.Fatal(err)
	}
}

func projectExecutionLaunchFixture(t *testing.T) (
	string,
	Options,
	mission.CurrentLoopExternalSessionHarnessPackage,
	string,
	string,
) {
	t.Helper()
	caseRoot := t.TempDir()
	stateRoot := filepath.Join(caseRoot, ".steamai")
	if err := os.Mkdir(stateRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	inputRel := ".steamai/execution-lease-input.json"
	input := []byte("{}\n")
	if err := os.WriteFile(
		filepath.Join(caseRoot, filepath.FromSlash(inputRel)),
		input,
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	signals := t.TempDir()
	readyPath := filepath.Join(signals, "ready")
	releasePath := filepath.Join(signals, "release")
	opt := Options{
		Target:     caseRoot,
		Pack:       "_template",
		ClaudePath: executable,
		Timeout:    20 * time.Second,
	}
	pkg := mission.CurrentLoopExternalSessionHarnessPackage{
		SchemaVersion:    1,
		CaseRoot:         caseRoot,
		JobID:            "execution-lease-job",
		JobSHA256:        strings.Repeat("a", 64),
		CheckpointSHA256: strings.Repeat("b", 64),
		SessionKind:      "mission-commander-evidence-review",
		Launch: &mission.CurrentLoopExternalSessionHarnessLaunch{
			Ready:    true,
			ReadOnly: true,
			Input: mission.CurrentLoopExternalSessionHarnessInput{
				Path:   inputRel,
				SHA256: bytesSHA256(input),
			},
			Attempt: mission.CurrentLoopExternalSessionAttempt{
				AttemptID:     "execution-lease-attempt",
				AttemptSHA256: strings.Repeat("c", 64),
				Generation:    1,
				Session:       "execution-lease-session",
			},
		},
	}
	return caseRoot, opt, pkg, readyPath, releasePath
}

func runProjectExecutionClaudeHelper() int {
	sessionID := ""
	for index := 0; index+1 < len(os.Args); index++ {
		if os.Args[index] == "--session-id" {
			sessionID = os.Args[index+1]
			break
		}
	}
	readyPath := os.Getenv(projectExecutionHelperReadyEnv)
	releasePath := os.Getenv(projectExecutionHelperReleaseEnv)
	if sessionID == "" || readyPath == "" || releasePath == "" {
		fmt.Fprintln(os.Stderr, "project execution Claude helper binding is incomplete")
		return 2
	}
	if err := os.WriteFile(readyPath, nil, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := os.Lstat(releasePath); err == nil {
			break
		} else if !os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		if time.Now().After(deadline) {
			fmt.Fprintln(os.Stderr, "timed out waiting for project execution helper release")
			return 2
		}
		time.Sleep(10 * time.Millisecond)
	}
	output := map[string]any{
		"decision":            "accepted",
		"summary":             "exact execution lease fixture",
		"reason":              "all bounded bindings agree",
		"evidenceRefs":        []string{"packet.json", "receipt.json"},
		"selectedEvidenceRef": "receipt.json",
		"observationEventId":  "execution-lease-observation",
		"receiptSha256":       strings.Repeat("d", 64),
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]any{
		"type":               "result",
		"subtype":            "success",
		"session_id":         sessionID,
		"structured_output":  output,
		"permission_denials": []any{},
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	return 0
}

func acquireProjectExecutionExclusive(
	caseRoot string,
) <-chan struct {
	lease *projectexecution.Lease
	err   error
} {
	done := make(chan struct {
		lease *projectexecution.Lease
		err   error
	}, 1)
	go func() {
		lease, err := projectexecution.AcquireExclusive(caseRoot)
		done <- struct {
			lease *projectexecution.Lease
			err   error
		}{lease: lease, err: err}
	}()
	return done
}

func assertProjectExecutionExclusiveBlocked(
	t *testing.T,
	done <-chan struct {
		lease *projectexecution.Lease
		err   error
	},
) {
	t.Helper()
	select {
	case acquired := <-done:
		if acquired.lease != nil {
			_ = acquired.lease.Unlock()
		}
		t.Fatalf("exclusive execution lease acquired while Claude execution was active: %v", acquired.err)
	case <-time.After(150 * time.Millisecond):
	}
}

func waitForProjectExecutionExclusive(
	t *testing.T,
	done <-chan struct {
		lease *projectexecution.Lease
		err   error
	},
) *projectexecution.Lease {
	t.Helper()
	select {
	case acquired := <-done:
		if acquired.err != nil {
			t.Fatal(acquired.err)
		}
		if acquired.lease == nil {
			t.Fatal("exclusive execution lease acquisition returned no lease")
		}
		return acquired.lease
	case <-time.After(5 * time.Second):
		t.Fatal("exclusive execution lease did not acquire after Claude execution ended")
		return nil
	}
}

func waitForProjectExecutionFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Lstat(path); err == nil {
			return
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for project execution marker: %s", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
