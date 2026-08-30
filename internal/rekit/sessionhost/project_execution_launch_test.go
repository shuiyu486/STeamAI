package sessionhost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterhost"
	"github.com/shuiyu486/re-context-kits/internal/rekit/capabilitycontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectlock"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
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
	projectExecutionHelperPromptEnv    = "REKIT_SESSIONHOST_EXECUTION_HELPER_PROMPT"
)

func TestMain(m *testing.M) {
	if handled, code := adapterhost.RunEmbeddedPrivate(os.Args[1:], os.Stdout, os.Stderr); handled {
		os.Exit(code)
	}
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
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	restoreExecutableSource := runtimebundle.SetExecutableSourceForTest(executable)
	restoreRuntimeBuilders := syncreview.SetRuntimeBundleBuildersForTest(runtimebundle.BuildWithExecutable)
	code := m.Run()
	restoreRuntimeBuilders()
	restoreExecutableSource()
	os.Exit(code)
}

func mustMarshalProjectExecutionTest(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
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
	promptPath := filepath.Join(t.TempDir(), "stdin-prompt.txt")
	t.Setenv(projectExecutionHelperRoleEnv, "claude")
	t.Setenv(projectExecutionHelperReadyEnv, readyPath)
	t.Setenv(projectExecutionHelperReleaseEnv, releasePath)
	t.Setenv(projectExecutionHelperPromptEnv, promptPath)
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
		prompt, err := os.ReadFile(promptPath)
		if err != nil {
			t.Fatal(err)
		}
		input, err := os.ReadFile(pkg.Launch.Input.Path)
		if err != nil {
			t.Fatal(err)
		}
		for _, expected := range []string{
			`boundInput={"role":"member-task-context","sha256":"` + pkg.Launch.Input.SHA256 + `"`,
			`"content":` + string(mustMarshalProjectExecutionTest(t, string(input))),
			"Do not ask for, reconstruct, concatenate, or Read an input file path",
		} {
			if !strings.Contains(string(prompt), expected) {
				t.Fatalf("stdin prompt omitted %q: %s", expected, prompt)
			}
		}
		if strings.Contains(string(prompt), pkg.Launch.Input.Path) || strings.Contains(string(prompt), caseRoot) {
			t.Fatalf("stdin prompt leaked bound input path: %s", prompt)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Claude launch did not finish after its helper was released")
	}
}

func TestRunClaudeBinaryInventoryMemberConsumesTypedPolicyThroughStdin(t *testing.T) {
	binding := memberexecution.TaskBinding{
		Kind: "binary-inventory-evidence",
		Values: map[string]string{
			"source-path":           "inputs/synthetic-pe.bin",
			"source-sha256":         strings.Repeat("1", 64),
			"inventory-path":        "workspace/main/inventory/session-1/binary-inventory.json",
			"inventory-sha256":      strings.Repeat("2", 64),
			"report-path":           "workspace/main/inventory/session-1/adapter-report.json",
			"dispatch-path":         ".steamai/lanes/main/adapter-executions/gate-a/dispatch.json",
			"receipt-path":          ".steamai/lanes/main/adapter-executions/gate-a/receipt.json",
			"selected-evidence-ref": "workspace/main/inventory/session-1/binary-inventory.json",
			"observation-event-id":  "evt-binary-inventory",
			"format-family":         "pe",
			"section-count":         "1",
			"import-count":          "0",
			"export-count":          "0",
		},
	}
	caseRoot, opt, pkg, readyPath, releasePath := projectExecutionLaunchFixtureWithBinding(t, &binding)
	inputPath := pkg.Launch.Input.Path
	if _, err := os.ReadFile(inputPath); err != nil {
		t.Fatal(err)
	}
	productionInstructions, err := inlineProductionInstructions(caseRoot, pkg)
	if err != nil {
		t.Fatal(err)
	}
	promptPath := filepath.Join(t.TempDir(), "binary-inventory-member-stdin.txt")
	t.Setenv(projectExecutionHelperRoleEnv, "claude")
	t.Setenv(projectExecutionHelperReadyEnv, readyPath)
	t.Setenv(projectExecutionHelperReleaseEnv, releasePath)
	t.Setenv(projectExecutionHelperPromptEnv, promptPath)
	t.Cleanup(func() { _ = os.WriteFile(releasePath, nil, 0o600) })

	runDone := make(chan claudeRun, 1)
	go func() {
		runDone <- runClaude(context.Background(), opt, pkg, pkg.Launch.Attempt.Session, nil)
	}()
	waitForProjectExecutionFile(t, readyPath)
	if err := os.WriteFile(releasePath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case run := <-runDone:
		if !run.success() || !run.started {
			t.Fatalf("binary inventory member helper launch failed: %+v", run)
		}
		prompt, err := os.ReadFile(promptPath)
		if err != nil {
			t.Fatal(err)
		}
		for _, expected := range []string{
			`boundInput={"role":"member-task-context","sha256":"` + pkg.Launch.Input.SHA256 + `"`,
			`\"kind\": \"binary-inventory-evidence\"`,
			productionInstructions,
			"Packet SHA-256: `" + pkg.Launch.InstructionIdentity.SHA256 + "`",
			"Receipt kind: `" + pkg.Launch.InstructionIdentity.ReceiptKind + "`",
			"read its exact inventory, report, dispatch, and receipt paths",
			"canonical PE/ELF inventory fields",
			"source/inventory/report/dispatch/receipt hashes",
			"format family, section/import/export counts",
			"selected evidence ref, and observation event ID",
		} {
			if !strings.Contains(string(prompt), expected) {
				t.Fatalf("binary inventory member stdin omitted %q: %s", expected, prompt)
			}
		}
		for _, forbidden := range []string{inputPath, caseRoot, "exact selected row", "query-term-only echo"} {
			if strings.Contains(string(prompt), forbidden) {
				t.Fatalf("binary inventory member stdin leaked or inherited %q: %s", forbidden, prompt)
			}
		}
	case <-time.After(5 * time.Second):
		t.Fatal("binary inventory member helper did not finish")
	}
}

func TestRunClaudeProductionInstructionSourceDriftBlocksProcessStart(t *testing.T) {
	caseRoot, opt, pkg, _, _ := projectExecutionLaunchFixture(t)
	if pkg.Launch == nil || pkg.Launch.InstructionIdentity == nil || len(pkg.Launch.InstructionIdentity.Sources) == 0 {
		t.Fatal("production launch fixture omitted instruction identity")
	}
	source := pkg.Launch.InstructionIdentity.Sources[0]
	sourcePath := filepath.Join(caseRoot, ".steamai", filepath.FromSlash(source.Path))
	original, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath, append(original, []byte("\nsource drift\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	started := false
	run := runClaude(context.Background(), opt, pkg, pkg.Launch.Attempt.Session, func() error {
		started = true
		return nil
	})
	if run.started || started || run.spawnErr == nil || !strings.Contains(run.spawnErr.Error(), "instruction") {
		t.Fatalf("drifted production source was not blocked before process start: %+v callback=%t", run, started)
	}
}

func TestRunClaudeBinaryInventoryReviewConsumesTypedPolicyThroughStdin(t *testing.T) {
	caseRoot, opt, pkg, readyPath, releasePath := legacyProjectExecutionLaunchFixture(t)
	input := []byte(`{"schemaVersion":1,"kind":"binary-inventory-evidence-review","selectedEvidenceRef":"workspace/main/inventory/binary-inventory.json"}` + "\n")
	inputPath := filepath.Join(caseRoot, filepath.FromSlash(pkg.Launch.Input.Path))
	if err := os.WriteFile(inputPath, input, 0o600); err != nil {
		t.Fatal(err)
	}
	pkg.Launch.Input.SHA256 = bytesSHA256(input)
	pkg.Launch.Input.Role = "mission-commander-binary-inventory-evidence-review-input"
	promptPath := filepath.Join(t.TempDir(), "binary-inventory-review-stdin.txt")
	t.Setenv(projectExecutionHelperRoleEnv, "claude")
	t.Setenv(projectExecutionHelperReadyEnv, readyPath)
	t.Setenv(projectExecutionHelperReleaseEnv, releasePath)
	t.Setenv(projectExecutionHelperPromptEnv, promptPath)
	t.Cleanup(func() { _ = os.WriteFile(releasePath, nil, 0o600) })

	runDone := make(chan claudeRun, 1)
	go func() {
		runDone <- runClaude(context.Background(), opt, pkg, pkg.Launch.Attempt.Session, nil)
	}()
	waitForProjectExecutionFile(t, readyPath)
	if err := os.WriteFile(releasePath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case run := <-runDone:
		if !run.success() || !run.started || run.sessionID != pkg.Launch.Attempt.Session {
			t.Fatalf("binary inventory evidence review helper launch failed: %+v", run)
		}
		prompt, err := os.ReadFile(promptPath)
		if err != nil {
			t.Fatal(err)
		}
		for _, expected := range []string{
			`boundInput={"role":"mission-commander-binary-inventory-evidence-review-input","sha256":"` + pkg.Launch.Input.SHA256 + `"`,
			`"content":` + string(mustMarshalProjectExecutionTest(t, string(input))),
			"exact source, canonical inventory, report, dispatch, receipt, and observation",
			"all five safety boundaries",
			"inventory path is the exact selectedEvidenceRef",
		} {
			if !strings.Contains(string(prompt), expected) {
				t.Fatalf("binary inventory review stdin omitted %q: %s", expected, prompt)
			}
		}
		for _, forbidden := range []string{inputPath, caseRoot, "selected row is an exact source line", "matched term and evidence ref are exact"} {
			if strings.Contains(string(prompt), forbidden) {
				t.Fatalf("binary inventory review stdin leaked or inherited %q: %s", forbidden, prompt)
			}
		}
	case <-time.After(5 * time.Second):
		t.Fatal("binary inventory evidence review helper did not finish")
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
		legacyProjectExecutionLaunchFixture(t)
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

func legacyProjectExecutionLaunchFixture(t *testing.T) (
	string,
	Options,
	mission.CurrentLoopExternalSessionHarnessPackage,
	string,
	string,
) {
	t.Helper()
	caseRoot := t.TempDir()
	stateRoot := filepath.Join(caseRoot, ".rekit")
	if err := os.Mkdir(stateRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	inputRel := ".rekit/execution-lease-input.json"
	input := []byte("{}\n")
	if err := os.WriteFile(filepath.Join(caseRoot, filepath.FromSlash(inputRel)), input, 0o600); err != nil {
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
			Ready:      true,
			ReadOnly:   true,
			Capability: readOnlyCapabilityForTest(),
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

func projectExecutionLaunchFixture(t *testing.T) (
	string,
	Options,
	mission.CurrentLoopExternalSessionHarnessPackage,
	string,
	string,
) {
	return projectExecutionLaunchFixtureWithBinding(t, nil)
}

func projectExecutionLaunchFixtureWithBinding(t *testing.T, binding *memberexecution.TaskBinding) (
	string,
	Options,
	mission.CurrentLoopExternalSessionHarnessPackage,
	string,
	string,
) {
	t.Helper()
	caseRoot := filepath.Join(t.TempDir(), "case")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	intent, err := applyDailyOnboarding(
		caseRoot,
		"exercise project execution launch ownership",
		"project-execution-launch-test",
		&bootstrap,
	)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = intent.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, intent.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	if binding != nil {
		owner, err := laneowner.Read(caseRoot, bootstrap.Lane)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := memberexecution.WriteTaskBindingForOwner(
			caseRoot,
			bootstrap.Lane,
			owner.CurrentExecutor,
			owner.ExecutorGeneration,
			*binding,
		); err != nil {
			t.Fatal(err)
		}
	}
	memberPlan, err := memberexecution.PreviewDispatch(memberexecution.DispatchOptions{
		CaseRoot:      caseRoot,
		Pack:          intent.Identity.Pack,
		Lane:          bootstrap.Lane,
		RequestSHA256: strings.Repeat("a", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	memberApplied, err := memberexecution.Apply(memberPlan, memberPlan.ExpectedPlanSHA256)
	if err != nil {
		t.Fatal(err)
	}
	member := memberApplied.Plan.Inspection
	if member.TaskContext == nil {
		t.Fatal("current member launch fixture omitted its immutable task context")
	}
	owner := laneowner.Snapshot{
		Lane:               member.TaskContext.Owner.Lane,
		CurrentExecutor:    member.TaskContext.Owner.Executor,
		ExecutorGeneration: member.TaskContext.Owner.ExecutorGeneration,
	}
	capability, err := capabilitycontract.Bind(capabilitycontract.Transport())
	if err != nil {
		t.Fatal(err)
	}
	launchControl, err := executioncontrol.CaptureBinding(caseRoot, owner, capability)
	if err != nil {
		t.Fatal(err)
	}
	instructionIdentity, err := currentProductionInstructionIdentity(caseRoot, intent.Identity.Pack)
	if err != nil {
		t.Fatal(err)
	}
	input, err := os.ReadFile(member.TaskContextPath)
	if err != nil {
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
		Pack:       intent.Identity.Pack,
		ClaudePath: executable,
		Timeout:    20 * time.Second,
	}
	pkg := mission.CurrentLoopExternalSessionHarnessPackage{
		SchemaVersion:    1,
		CaseRoot:         caseRoot,
		Pack:             intent.Identity.Pack,
		JobID:            "execution-lease-job",
		JobSHA256:        strings.Repeat("b", 64),
		CheckpointSHA256: strings.Repeat("c", 64),
		SessionKind:      "member",
		Launch: &mission.CurrentLoopExternalSessionHarnessLaunch{
			Ready:               true,
			Capability:          transportCapabilityForTest(),
			InstructionIdentity: &instructionIdentity,
			Input: mission.CurrentLoopExternalSessionHarnessInput{
				Path:   member.TaskContextPath,
				SHA256: bytesSHA256(input),
				Role:   "member-task-context",
			},
			Attempt: mission.CurrentLoopExternalSessionAttempt{
				AttemptID:     "execution-lease-attempt",
				AttemptSHA256: strings.Repeat("d", 64),
				Generation:    1,
				Session:       "execution-lease-session",
				LaunchControl: executioncontrol.CloneBinding(&launchControl),
			},
		},
	}
	return caseRoot, opt, pkg, readyPath, releasePath
}

func runProjectExecutionClaudeHelper() int {
	sessionID := ""
	schema := ""
	printFromStdin := false
	for index := 0; index < len(os.Args); index++ {
		switch os.Args[index] {
		case "-p", "--print":
			printFromStdin = index+1 == len(os.Args) || strings.HasPrefix(os.Args[index+1], "-")
		case "--session-id":
			if index+1 < len(os.Args) {
				sessionID = os.Args[index+1]
			}
		case "--json-schema":
			if index+1 < len(os.Args) {
				schema = os.Args[index+1]
			}
		}
	}
	if !printFromStdin {
		fmt.Fprintln(os.Stderr, "project execution Claude helper received prompt as a command-line argument")
		return 2
	}
	readyPath := os.Getenv(projectExecutionHelperReadyEnv)
	releasePath := os.Getenv(projectExecutionHelperReleaseEnv)
	promptPath := os.Getenv(projectExecutionHelperPromptEnv)
	if sessionID == "" || readyPath == "" || releasePath == "" {
		fmt.Fprintln(os.Stderr, "project execution Claude helper binding is incomplete")
		return 2
	}
	prompt, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if promptPath != "" {
		if err := os.WriteFile(promptPath, prompt, 0o600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
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
		"outcome":           "failed",
		"summary":           "",
		"reason":            "bounded process-launch fixture",
		"outputs":           []any{},
		"reviewerItemsPath": "",
	}
	if strings.Contains(schema, `"decision"`) {
		output = map[string]any{
			"decision":            "accepted",
			"summary":             "exact execution lease fixture",
			"reason":              "all bounded bindings agree",
			"evidenceRefs":        []string{"packet.json", "receipt.json"},
			"selectedEvidenceRef": "receipt.json",
			"observationEventId":  "execution-lease-observation",
			"receiptSha256":       strings.Repeat("d", 64),
		}
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
