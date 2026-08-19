package sessionhost

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
)

func TestSupervisionStopActuationClosesOnlyAfterDurableStopRequest(t *testing.T) {
	caseRoot, scope := supervisionStopActuationFixture(t)
	resultCh := make(chan supervisionStopActuationResult, 1)
	var calls atomic.Int32
	go func() {
		resultCh <- watchSupervisionStopActuation(
			context.Background(),
			scope,
			func() error {
				calls.Add(1)
				inspection, err := executioncontrol.Inspect(
					caseRoot,
					scope.spec.LaunchControl.Lane,
				)
				if err != nil {
					return err
				}
				if inspection.Pending ||
					inspection.State != executioncontrol.StateStopped ||
					inspection.CurrentGeneration <= scope.spec.LaunchControl.ControlGeneration {
					return errors.New("owned containment was closed before durable stopped became current")
				}
				requestPath, _ := supervisionStopActuationArtifactPaths(
					inspection.CurrentGeneration,
				)
				if _, err := os.Lstat(filepath.Join(scope.paths.runRoot, filepath.FromSlash(requestPath))); err != nil {
					return errors.New("owned containment was closed before its durable actuation request")
				}
				if _, err := os.Lstat(filepath.Join(caseRoot, filepath.FromSlash(inspection.CurrentReceiptPath))); err != nil {
					return errors.New("owned containment was closed before its durable stop receipt")
				}
				return nil
			},
		)
	}()

	stopped := applyClaudeLaunchControlForTest(
		t,
		caseRoot,
		scope.spec.LaunchControl.Lane,
		executioncontrol.ActionStop,
		"2026-08-19T01:00:00Z",
	)
	result := waitForSupervisionStopActuation(t, resultCh)
	if result.Err != nil || calls.Load() != 1 {
		t.Fatalf("stop actuation result=%+v calls=%d", result, calls.Load())
	}
	requestPath, observationPath := supervisionStopActuationArtifactPaths(stopped.ControlGeneration)
	if result.RequestPath != requestPath || result.ObservationPath != observationPath ||
		!validClaudeLaunchSHA256(result.ObservationSHA256) {
		t.Fatalf("stop actuation artifact result=%+v", result)
	}
	observation, _, ok, err := readSupervisionStopActuationObservation(scope, observationPath)
	if err != nil || !ok || observation.Outcome != "owned-containment-closed" ||
		!observation.RequestPublished || !observation.ContainmentCloseAttempted ||
		!observation.ContainmentCloseSucceeded || !observation.NoProcessTerminationClaim {
		t.Fatalf("stop actuation observation ok=%t err=%v value=%+v", ok, err, observation)
	}
	inspection, err := executioncontrol.Inspect(caseRoot, stopped.Lane)
	if err != nil || inspection.State != executioncontrol.StateStopped ||
		inspection.CurrentGeneration != stopped.ControlGeneration {
		t.Fatalf("stop actuation changed durable stopped head: inspection=%+v err=%v", inspection, err)
	}
}

func TestSupervisionStopActuationFailureDoesNotRollBackStopped(t *testing.T) {
	caseRoot, scope := supervisionStopActuationFixture(t)
	stopped := applyClaudeLaunchControlForTest(
		t,
		caseRoot,
		scope.spec.LaunchControl.Lane,
		executioncontrol.ActionStop,
		"2026-08-19T01:01:00Z",
	)
	actuationErr := errors.New("bounded owned containment close failed")
	result := watchSupervisionStopActuation(
		context.Background(),
		scope,
		func() error { return actuationErr },
	)
	if !errors.Is(result.Err, actuationErr) {
		t.Fatalf("stop actuation failure result=%+v", result)
	}
	_, observationPath := supervisionStopActuationArtifactPaths(stopped.ControlGeneration)
	observation, _, ok, err := readSupervisionStopActuationObservation(scope, observationPath)
	if err != nil || !ok || observation.Outcome != "actuation-failed" ||
		!observation.RequestPublished || !observation.ContainmentCloseAttempted ||
		observation.ContainmentCloseSucceeded ||
		!strings.Contains(observation.Error, actuationErr.Error()) {
		t.Fatalf("failed stop actuation observation ok=%t err=%v value=%+v", ok, err, observation)
	}
	inspection, err := executioncontrol.Inspect(caseRoot, stopped.Lane)
	if err != nil || inspection.State != executioncontrol.StateStopped ||
		inspection.CurrentGeneration != stopped.ControlGeneration || inspection.Pending {
		t.Fatalf("failed actuation rolled back durable stop: inspection=%+v err=%v", inspection, err)
	}
}

func TestSupervisionPauseNeverActuatesOwnedContainment(t *testing.T) {
	caseRoot, scope := supervisionStopActuationFixture(t)
	paused := applyClaudeLaunchControlForTest(
		t,
		caseRoot,
		scope.spec.LaunchControl.Lane,
		executioncontrol.ActionPause,
		"2026-08-19T01:02:00Z",
	)
	request, ready, err := inspectSupervisionStopActuationRequest(scope)
	if err != nil || ready || request.Kind != "" {
		t.Fatalf("paused control produced stop actuation request: ready=%t request=%+v err=%v", ready, request, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var calls atomic.Int32
	result := watchSupervisionStopActuation(ctx, scope, func() error {
		calls.Add(1)
		return nil
	})
	if result.Err != nil || calls.Load() != 0 || result.RequestPath != "" || result.ObservationPath != "" {
		t.Fatalf("paused control actuated a process: result=%+v calls=%d", result, calls.Load())
	}
	requestPath, observationPath := supervisionStopActuationArtifactPaths(paused.ControlGeneration)
	for _, rel := range []string{requestPath, observationPath} {
		if _, err := os.Lstat(filepath.Join(scope.paths.runRoot, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Fatalf("paused control published stop actuation artifact %s: %v", rel, err)
		}
	}
}

func TestSupervisionTerminalReadsDurableStopActuationLineage(t *testing.T) {
	caseRoot, scope := supervisionStopActuationFixture(t)
	stopped := applyClaudeLaunchControlForTest(
		t,
		caseRoot,
		scope.spec.LaunchControl.Lane,
		executioncontrol.ActionStop,
		"2026-08-19T01:04:00Z",
	)
	result := watchSupervisionStopActuation(context.Background(), scope, func() error { return nil })
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	terminal := supervisionStopActuationTerminalForTest(scope)
	terminal.StopActuationRequestPath = result.RequestPath
	terminal.StopActuationObservationPath = result.ObservationPath
	terminal.StopActuationObservationSHA256 = result.ObservationSHA256
	if err := writeSupervisionJSON(scope.paths.runRoot, "terminal.json", "test terminal", terminal); err != nil {
		t.Fatal(err)
	}
	actual, ok, err := readSupervisionTerminal(scope.paths, scope.spec, scope.specSHA256)
	if err != nil || !ok || actual.StopActuationObservationSHA256 != result.ObservationSHA256 {
		t.Fatalf("durable stop terminal ok=%t err=%v value=%+v", ok, err, actual)
	}
	observation, _, ok, err := readSupervisionStopActuationObservation(scope, result.ObservationPath)
	if err != nil || !ok || observation.StopControlGeneration != stopped.ControlGeneration {
		t.Fatalf("durable stop observation ok=%t err=%v value=%+v", ok, err, observation)
	}
}

func TestSupervisionStopActuationFinalInspectionObservesConcurrentStop(t *testing.T) {
	caseRoot, scope := supervisionStopActuationFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int32
	resultCh := make(chan supervisionStopActuationResult, 1)
	go func() {
		resultCh <- watchSupervisionStopActuation(ctx, scope, func() error {
			calls.Add(1)
			return nil
		})
	}()
	applyClaudeLaunchControlForTest(
		t,
		caseRoot,
		scope.spec.LaunchControl.Lane,
		executioncontrol.ActionStop,
		"2026-08-19T01:04:30Z",
	)
	cancel()
	result := waitForSupervisionStopActuation(t, resultCh)
	if result.Err != nil || calls.Load() != 1 || !validClaudeLaunchSHA256(result.ObservationSHA256) {
		t.Fatalf("final stop inspection result=%+v calls=%d", result, calls.Load())
	}
}

func TestSupervisionTerminalKeepsRawTruthWithoutActuationObservation(t *testing.T) {
	_, scope := supervisionStopActuationFixture(t)
	terminal := supervisionStopActuationTerminalForTest(scope)
	terminal.StopActuationError = "publish stop actuation observation: bounded test failure"
	if err := writeSupervisionJSON(scope.paths.runRoot, "terminal.json", "test terminal", terminal); err != nil {
		t.Fatal(err)
	}
	actual, ok, err := readSupervisionTerminal(scope.paths, scope.spec, scope.specSHA256)
	if err != nil || !ok || actual.StopActuationError != terminal.StopActuationError || actual.rawResultRef == "" {
		t.Fatalf("error-only stop actuation terminal ok=%t err=%v value=%+v", ok, err, actual)
	}
	run := claudeRunFromTerminal(actual, scope.spec.LaunchControl, true)
	if run.stopActuation.Err == nil || run.stopActuation.RequestPath != "" || run.rawResultRef == "" {
		t.Fatalf("error-only stop actuation recovery=%+v", run.stopActuation)
	}
}

func supervisionStopActuationTerminalForTest(scope supervisionStopActuationContext) supervisionTerminal {
	return supervisionTerminal{
		SchemaVersion:          1,
		Kind:                   supervisionTerminalKind,
		RunID:                  scope.spec.RunID,
		SpecSHA256:             scope.specSHA256,
		SessionID:              scope.spec.SessionID,
		StructuredOutputSHA256: bytesSHA256(nil),
		ObservedAt:             "2026-08-19T01:05:00Z",
	}
}

func supervisionStopActuationFixture(t *testing.T) (string, supervisionStopActuationContext) {
	t.Helper()
	caseRoot, opt, pkg, _, _ := projectExecutionLaunchFixture(t)
	bound, err := ensureClaudeLaunchControlBinding(opt, pkg)
	if err != nil {
		t.Fatal(err)
	}
	paths, spec, data, specSHA, err := prepareSupervision(
		bound,
		pkg,
		pkg.Launch.Attempt.Session,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(paths.root) })
	if _, err := os.Lstat(paths.spec); os.IsNotExist(err) {
		if err := os.WriteFile(paths.spec, data, 0o600); err != nil {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	return caseRoot, supervisionStopActuationContext{
		paths:      paths,
		spec:       spec,
		specSHA256: specSHA,
	}
}

func waitForSupervisionStopActuation(
	t *testing.T,
	result <-chan supervisionStopActuationResult,
) supervisionStopActuationResult {
	t.Helper()
	select {
	case value := <-result:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for exact supervisor stop actuation")
		return supervisionStopActuationResult{}
	}
}
