package sessionhost

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectexecution"
)

func TestSupervisionSpecBindsAttemptSessionExecutableAndPackage(t *testing.T) {
	opt := recoveryOptionsForTest()
	opt.Target = t.TempDir()
	opt.ClaudePath = filepath.Join(opt.Target, "claude")
	opt.Timeout = time.Minute
	pkg := recoveryPackageForTest()

	paths, spec, data, sha, err := prepareSupervision(opt, pkg, pkg.Launch.Attempt.Session)
	if err != nil {
		t.Fatal(err)
	}
	if spec.RunID == "" || paths.runID != spec.RunID || paths.runRoot == "" || bytesSHA256(data) != sha || spec.Execution.Launch.Attempt.AttemptSHA256 != pkg.Launch.Attempt.AttemptSHA256 {
		t.Fatalf("supervision binding drifted: paths=%+v spec=%+v", paths, spec)
	}
	drifted := pkg
	drifted.Launch.Attempt.AttemptSHA256 = "drifted-attempt"
	driftedPaths, driftedSpec, _, _, err := prepareSupervision(opt, drifted, drifted.Launch.Attempt.Session)
	if err != nil {
		t.Fatal(err)
	}
	if driftedPaths.runID == paths.runID || driftedSpec.RunID == spec.RunID {
		t.Fatal("attempt drift reused the supervised run identity")
	}
	if _, _, _, _, err := prepareSupervision(opt, pkg, "different-session"); err == nil {
		t.Fatal("session drift should fail closed")
	}
}

func TestSupervisionSpecRoundTripsReviewerIdentity(t *testing.T) {
	opt := recoveryOptionsForTest()
	opt.Target = t.TempDir()
	opt.ClaudePath = filepath.Join(opt.Target, "claude")
	opt.Timeout = time.Minute
	pkg := recoveryPackageForTest()
	pkg.SessionKind = "reviewer"
	pkg.Launch.ReviewerIdentity = &mission.CurrentLoopExternalSessionReviewerIdentity{
		PacketID: "packet-exact", RouteID: "_template:lane-feature-analysis",
		ShardID: "shard-01", Items: []string{"evidence/manifest.json"},
		OutputFields: []string{"item", "decision", "candidate_path"},
		DispatchPath: ".rekit/dispatch.json", DispatchSHA256: strings.Repeat("a", 64),
		DispatchID: "dispatch-exact", ReviewerSession: pkg.Launch.Attempt.Session,
		PromptPath: ".rekit/reviewer-prompt.md", PromptSHA256: strings.Repeat("b", 64),
		NoHeavyTool: true, NoAuthority: true,
	}

	_, spec, _, _, err := prepareSupervision(opt, pkg, pkg.Launch.Attempt.Session)
	if err != nil {
		t.Fatal(err)
	}
	roundTrip := spec.Execution.packageForRun()
	if roundTrip.Launch == nil || roundTrip.Launch.ReviewerIdentity == nil {
		t.Fatalf("supervision dropped reviewer identity: got=%+v want=%+v", roundTrip.Launch, pkg.Launch)
	}
	gotIdentity, err := json.Marshal(roundTrip.Launch.ReviewerIdentity)
	if err != nil {
		t.Fatal(err)
	}
	wantIdentity, err := json.Marshal(pkg.Launch.ReviewerIdentity)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotIdentity, wantIdentity) {
		t.Fatalf("supervision changed reviewer identity: got=%s want=%s", gotIdentity, wantIdentity)
	}
}

func TestSupervisionSpecBindsResolvedPack(t *testing.T) {
	opt := recoveryOptionsForTest()
	opt.Target = t.TempDir()
	opt.Pack = "_template"
	opt.ClaudePath = filepath.Join(opt.Target, "claude")
	opt.Timeout = time.Minute
	pkg := recoveryPackageForTest()

	_, spec, _, _, err := prepareSupervision(
		opt,
		pkg,
		pkg.Launch.Attempt.Session,
	)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Pack != opt.Pack {
		t.Fatalf("supervision spec pack = %q, want %q", spec.Pack, opt.Pack)
	}
}

func TestSupervisionExecutionIgnoresAcceptedProjectionDrift(t *testing.T) {
	opt := recoveryOptionsForTest()
	opt.Target = t.TempDir()
	opt.ClaudePath = filepath.Join(opt.Target, "claude")
	opt.Timeout = time.Minute
	launchReady := recoveryPackageForTest()
	launchReady.Launch.Ready = true
	launchReady.Return = &mission.CurrentLoopExternalSessionReturnContract{Templates: []mission.CurrentLoopExternalSessionSubmissionTemplate{{
		Outcome: "returned",
		JSON:    `{"dispatchClaimSha256":"<dispatch-claim-sha256>","launchReceiptSha256":"<launch-receipt-sha256>"}`,
	}}}

	_, launchSpec, launchData, launchSHA, err := prepareSupervision(opt, launchReady, launchReady.Launch.Attempt.Session)
	if err != nil {
		t.Fatal(err)
	}
	running := launchReady
	running.State = "running"
	running.Launch = &mission.CurrentLoopExternalSessionHarnessLaunch{}
	*running.Launch = *launchReady.Launch
	running.Launch.Ready = false
	running.Return = &mission.CurrentLoopExternalSessionReturnContract{}
	*running.Return = *launchReady.Return
	running.Return.Templates = append([]mission.CurrentLoopExternalSessionSubmissionTemplate{}, launchReady.Return.Templates...)
	running.Return.Templates[0].JSON = `{"dispatchClaimSha256":"accepted-claim","launchReceiptSha256":"accepted-receipt"}`
	_, runningSpec, runningData, runningSHA, err := prepareSupervision(opt, running, running.Launch.Attempt.Session)
	if err != nil {
		t.Fatal(err)
	}
	if launchSHA != runningSHA || !bytes.Equal(launchData, runningData) {
		t.Fatalf("accepted projection drift changed immutable supervision binding: launch=%+v running=%+v", launchSpec.Execution, runningSpec.Execution)
	}
}

func TestSupervisionTerminalRoundTripsExactClaudeRun(t *testing.T) {
	output := json.RawMessage(`{"outcome":"returned","summary":"exact","reason":"","outputs":[{"path":"out.txt","content":"exact"}],"reviewerItemsPath":""}`)
	receipt := supervisionTerminal{
		SchemaVersion:          1,
		Kind:                   supervisionTerminalKind,
		RunID:                  strings.Repeat("a", 64),
		SpecSHA256:             strings.Repeat("b", 64),
		SessionID:              "session-id",
		Envelope:               claudeEnvelope{Type: "result", Subtype: "success", SessionID: "session-id"},
		StructuredOutputBase64: base64.StdEncoding.EncodeToString(output),
		StructuredOutputSHA256: bytesSHA256(output),
		Started:                true,
		ExitCode:               0,
		DurationNanos:          int64(time.Second),
		ObservedAt:             "2026-08-07T00:00:00Z",
	}
	run := claudeRunFromTerminal(receipt, nil, true)
	if !run.success() || !run.recovered || run.sessionID != receipt.SessionID || run.duration != time.Second || run.observedAt != receipt.ObservedAt || !bytes.Equal(run.structuredOutput, output) {
		t.Fatalf("terminal reconstruction drifted: %+v", run)
	}
}

func TestSupervisionRejectsTerminalBindingDrift(t *testing.T) {
	opt := recoveryOptionsForTest()
	opt.Target = t.TempDir()
	opt.ClaudePath = filepath.Join(opt.Target, "claude")
	opt.Timeout = time.Minute
	pkg := recoveryPackageForTest()
	paths, spec, data, sha, err := prepareSupervision(opt, pkg, pkg.Launch.Attempt.Session)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.spec, data, 0o600); err != nil {
		t.Fatal(err)
	}
	receipt := supervisionTerminal{SchemaVersion: 1, Kind: supervisionTerminalKind, RunID: spec.RunID, SpecSHA256: "wrong", ObservedAt: "2026-08-07T00:00:00Z"}
	if err := writeSupervisionJSON(paths.runRoot, "terminal.json", "test terminal", receipt); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := readSupervisionTerminal(paths, spec, sha); err == nil || ok {
		t.Fatalf("terminal binding drift ok=%t err=%v", ok, err)
	}
}

func TestSupervisionTerminalRejectsSessionDrift(t *testing.T) {
	opt := recoveryOptionsForTest()
	opt.Target = t.TempDir()
	opt.ClaudePath = filepath.Join(opt.Target, "claude")
	opt.Timeout = time.Minute
	pkg := recoveryPackageForTest()
	paths, spec, data, sha, err := prepareSupervision(opt, pkg, pkg.Launch.Attempt.Session)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.spec, data, 0o600); err != nil {
		t.Fatal(err)
	}
	output := json.RawMessage(`{"opaque":"wrong-session"}`)
	receipt := supervisionTerminal{
		SchemaVersion: 1, Kind: supervisionTerminalKind, RunID: spec.RunID, SpecSHA256: sha,
		SessionID: "different-session", Envelope: claudeEnvelope{Type: "result", Subtype: "success", SessionID: "different-session"},
		StructuredOutputBase64: base64.StdEncoding.EncodeToString(output), StructuredOutputSHA256: bytesSHA256(output), ObservedAt: "2026-08-07T00:00:00Z",
	}
	if err := writeSupervisionJSON(paths.runRoot, "terminal.json", "test terminal", receipt); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := readSupervisionTerminal(paths, spec, sha); err == nil || ok {
		t.Fatalf("wrong session terminal ok=%t err=%v", ok, err)
	}
}

func TestSupervisionClaimReceiptBindsExactOwnerRun(t *testing.T) {
	opt := recoveryOptionsForTest()
	opt.Target = t.TempDir()
	opt.ClaudePath = filepath.Join(opt.Target, "claude")
	opt.Timeout = time.Minute
	pkg := recoveryPackageForTest()
	paths, spec, data, sha, err := prepareSupervision(opt, pkg, pkg.Launch.Attempt.Session)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.spec, data, 0o600); err != nil {
		t.Fatal(err)
	}
	claim := supervisionClaimed{SchemaVersion: 1, Kind: supervisionClaimedKind, RunID: spec.RunID, SpecSHA256: sha, SessionID: spec.SessionID, ClaimedAt: "2026-08-07T00:00:00Z"}
	if err := writeSupervisionJSON(paths.runRoot, "claimed.json", "test claim", claim); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := readSupervisionClaimed(paths, spec, sha); err != nil || !ok {
		t.Fatalf("exact owner claim ok=%t err=%v", ok, err)
	}
	claim.SpecSHA256 = "drifted"
	if err := os.Remove(paths.claimed); err != nil {
		t.Fatal(err)
	}
	if err := writeSupervisionJSON(paths.runRoot, "claimed.json", "test drifted claim", claim); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := readSupervisionClaimed(paths, spec, sha); err == nil || ok {
		t.Fatalf("drifted owner claim ok=%t err=%v", ok, err)
	}
}

func TestSupervisionStartupFailurePublishesPermanentFence(t *testing.T) {
	requireDurableHandoffForSessionhostTest(t)
	opt := recoveryOptionsForTest()
	opt.Target = t.TempDir()
	opt.ClaudePath = filepath.Join(opt.Target, "claude")
	opt.Timeout = time.Minute
	pkg := recoveryPackageForTest()
	paths, spec, data, sha, err := prepareSupervision(opt, pkg, pkg.Launch.Attempt.Session)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.spec, data, 0o600); err != nil {
		t.Fatal(err)
	}
	handoff, err := projectexecution.NewHandoff(
		spec.Target,
		spec.RunID,
		sha,
		spec.SessionID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := projectexecution.PublishHandoff(spec.Target, handoff); err != nil {
		t.Fatal(err)
	}
	firstErr := fenceSupervision(paths, spec, sha, "child exited before claim", true)
	secondErr := fenceSupervision(paths, spec, sha, "different reason must not replace first", true)
	var firstFence, secondFence *supervisionFencedError
	if !errors.As(firstErr, &firstFence) || !errors.As(secondErr, &secondFence) || firstFence.Reason != secondFence.Reason || firstFence.Reason != "child exited before claim" {
		t.Fatalf("startup fence was not immutable: first=%v second=%v", firstErr, secondErr)
	}
}

func TestSupervisionFencePermanentlyRejectsOldRun(t *testing.T) {
	opt := recoveryOptionsForTest()
	opt.Target = t.TempDir()
	opt.ClaudePath = filepath.Join(opt.Target, "claude")
	opt.Timeout = time.Minute
	pkg := recoveryPackageForTest()
	paths, spec, data, sha, err := prepareSupervision(opt, pkg, pkg.Launch.Attempt.Session)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.spec, data, 0o600); err != nil {
		t.Fatal(err)
	}
	fenced := supervisionFenced{SchemaVersion: 1, Kind: supervisionFencedKind, RunID: spec.RunID, SpecSHA256: sha, SessionID: spec.SessionID, Reason: "owner exited without terminal", FencedAt: "2026-08-07T00:00:00Z"}
	if err := writeSupervisionJSON(paths.runRoot, "fenced.json", "test fence", fenced); err != nil {
		t.Fatal(err)
	}
	run, launched, err := supervisedClaudeRun(context.Background(), opt, pkg, pkg.Launch.Attempt.Session, nil)
	var fencedErr *supervisionFencedError
	if !errors.As(err, &fencedErr) || launched || run.started || fencedErr.RunID != spec.RunID {
		t.Fatalf("fenced run was not permanently rejected: launched=%t run=%+v err=%v", launched, run, err)
	}
}

func TestSupervisionControlLeaseSerializesFreshHosts(t *testing.T) {
	caseRoot := t.TempDir()
	first, err := acquireSupervisionControl(context.Background(), caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := acquireSupervisionControl(ctx, caseRoot); !errors.Is(err, context.Canceled) {
		t.Fatalf("contending fresh host did not honor cancellation: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := acquireSupervisionControl(context.Background(), caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSupervisionFenceCannotRaceActiveOwner(t *testing.T) {
	opt := recoveryOptionsForTest()
	opt.Target = t.TempDir()
	opt.ClaudePath = filepath.Join(opt.Target, "claude")
	opt.Timeout = time.Minute
	pkg := recoveryPackageForTest()
	paths, spec, data, sha, err := prepareSupervision(opt, pkg, pkg.Launch.Attempt.Session)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.spec, data, 0o600); err != nil {
		t.Fatal(err)
	}
	owner, busy, err := acquireSupervisionOwner(paths.owner, true)
	if err != nil || busy {
		t.Fatalf("owner acquisition busy=%t err=%v", busy, err)
	}
	if err := fenceSupervision(paths, spec, sha, "must not win", false); !errors.Is(err, errSupervisionAdvanced) || !strings.Contains(err.Error(), "ownership became active") {
		t.Fatalf("active owner did not advance collection instead of fencing: %v", err)
	}
	if _, err := os.Lstat(paths.fenced); !os.IsNotExist(err) {
		t.Fatalf("active owner race published fence: %v", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSupervisionFenceContinuesWhenTerminalAppears(t *testing.T) {
	opt := recoveryOptionsForTest()
	opt.Target = t.TempDir()
	opt.ClaudePath = filepath.Join(opt.Target, "claude")
	opt.Timeout = time.Minute
	pkg := recoveryPackageForTest()
	paths, spec, data, sha, err := prepareSupervision(opt, pkg, pkg.Launch.Attempt.Session)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.spec, data, 0o600); err != nil {
		t.Fatal(err)
	}
	terminal := supervisionTerminal{
		SchemaVersion: 1, Kind: supervisionTerminalKind, RunID: spec.RunID, SpecSHA256: sha,
		SessionID: spec.SessionID, Envelope: claudeEnvelope{SessionID: spec.SessionID},
		StructuredOutputSHA256: bytesSHA256(nil), ObservedAt: "2026-08-07T00:00:00Z",
	}
	if err := writeSupervisionJSON(paths.runRoot, "terminal.json", "test terminal", terminal); err != nil {
		t.Fatal(err)
	}
	if err := fenceSupervision(paths, spec, sha, "must not win", false); !errors.Is(err, errSupervisionAdvanced) || !strings.Contains(err.Error(), "terminal result appeared") {
		t.Fatalf("terminal publication did not advance collection instead of fencing: %v", err)
	}
	if _, err := os.Lstat(paths.fenced); !os.IsNotExist(err) {
		t.Fatalf("terminal publication race wrote fence: %v", err)
	}
}

func TestSupervisorChildRejectsFenceBeforeClaim(t *testing.T) {
	requireDurableHandoffForSessionhostTest(t)
	opt := recoveryOptionsForTest()
	opt.Target = t.TempDir()
	opt.ClaudePath = filepath.Join(opt.Target, "claude")
	opt.Timeout = time.Minute
	pkg := recoveryPackageForTest()
	paths, spec, data, sha, err := prepareSupervision(opt, pkg, pkg.Launch.Attempt.Session)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.spec, data, 0o600); err != nil {
		t.Fatal(err)
	}
	handoff, err := projectexecution.NewHandoff(
		spec.Target,
		spec.RunID,
		sha,
		spec.SessionID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := projectexecution.PublishHandoff(spec.Target, handoff); err != nil {
		t.Fatal(err)
	}
	if err := fenceSupervision(paths, spec, sha, "startup deadline", true); err == nil {
		t.Fatal("fence publication did not return typed fence")
	}
	err = RunSupervisorChild(context.Background(), paths.spec, sha)
	var fencedErr *supervisionFencedError
	if !errors.As(err, &fencedErr) || fencedErr.RunID != spec.RunID {
		t.Fatalf("late child did not reject exact fence: %v", err)
	}
	if _, err := os.Lstat(paths.claimed); !os.IsNotExist(err) {
		t.Fatalf("late child wrote claim after fence: %v", err)
	}
}

func TestSupervisionOwnerLeaseIsProcessScoped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owner.lease")
	lease, busy, err := acquireSupervisionOwner(path, false)
	if err != nil || busy {
		t.Fatalf("acquire owner lease busy=%t err=%v", busy, err)
	}
	if busy, err := supervisionOwnerBusy(path); err != nil || !busy {
		t.Fatalf("owner lease probe busy=%t err=%v", busy, err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if busy, err := supervisionOwnerBusy(path); err != nil || busy {
		t.Fatalf("released owner lease probe busy=%t err=%v", busy, err)
	}
}

func TestSupervisedClaudeRunCollectsExistingTerminalWithoutLaunch(t *testing.T) {
	opt := recoveryOptionsForTest()
	opt.Target = t.TempDir()
	opt.ClaudePath = filepath.Join(opt.Target, "claude")
	opt.Timeout = time.Minute
	pkg := recoveryPackageForTest()
	paths, spec, specData, specSHA, err := prepareSupervision(opt, pkg, pkg.Launch.Attempt.Session)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.spec, specData, 0o600); err != nil {
		t.Fatal(err)
	}
	output := json.RawMessage(`{"opaque":"exact"}`)
	receipt := supervisionTerminal{
		SchemaVersion: 1, Kind: supervisionTerminalKind, RunID: spec.RunID, SpecSHA256: specSHA,
		SessionID: pkg.Launch.Attempt.Session, Envelope: claudeEnvelope{Type: "result", Subtype: "success", SessionID: pkg.Launch.Attempt.Session},
		StructuredOutputBase64: base64.StdEncoding.EncodeToString(output), StructuredOutputSHA256: bytesSHA256(output),
		Started: true, ExitCode: 0, ObservedAt: "2026-08-07T00:00:00Z",
	}
	if err := writeSupervisionJSON(paths.runRoot, "terminal.json", "test terminal", receipt); err != nil {
		t.Fatal(err)
	}
	startedCalls := 0
	run, launched, err := supervisedClaudeRun(context.Background(), opt, pkg, pkg.Launch.Attempt.Session, func() error {
		startedCalls++
		return nil
	})
	if err != nil || launched || !run.success() || run.started || !run.recovered || startedCalls != 1 {
		t.Fatalf("existing terminal collection launched=%t startedCalls=%d err=%v run=%+v", launched, startedCalls, err, run)
	}
}
