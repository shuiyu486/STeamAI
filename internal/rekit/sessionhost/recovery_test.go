package sessionhost

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestClaudeRecoveryTreatsMissingArtifactAsFreshLaunch(t *testing.T) {
	root := t.TempDir()
	pkg := recoveryPackageForTest()
	opt := recoveryOptionsForTest()

	recovered, ok, err := recoverClaudeRun(root, opt, pkg)
	if err != nil || ok {
		t.Fatalf("missing recovery ok=%t err=%v run=%+v", ok, err, recovered)
	}
}

func TestClaudeRecoveryRoundTripsExactAttemptSessionExecutableAndBytes(t *testing.T) {
	root := t.TempDir()
	pkg := recoveryPackageForTest()
	opt := recoveryOptionsForTest()
	output := json.RawMessage(`{"opaque":"bounded-bytes-1"}`)
	run := claudeRun{
		envelope:         claudeEnvelope{Type: "result", Subtype: "success", SessionID: "session-id"},
		sessionID:        "session-id",
		structuredOutput: output,
		started:          true,
		exitCode:         0,
	}
	if err := persistClaudeRecovery(root, opt, pkg, run); err != nil {
		t.Fatal(err)
	}
	recovered, ok, err := recoverClaudeRun(root, opt, pkg)
	if err != nil || !ok {
		t.Fatalf("recovery ok=%t err=%v", ok, err)
	}
	if recovered.sessionID != run.sessionID || !recovered.success() || !recovered.recovered || recovered.started || !bytes.Equal(recovered.structuredOutput, output) {
		t.Fatalf("recovered run drifted: %+v", recovered)
	}
	caseRoot := t.TempDir()
	caseRecoveryRoot, err := claudeRecoveryRoot(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := persistClaudeRecovery(caseRecoveryRoot, opt, pkg, run); err != nil {
		t.Fatal(err)
	}
	if err := removeClaudeRecoveryForCase(caseRoot, opt, pkg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(caseRecoveryRoot, claudeRecoveryPath(pkg))); !os.IsNotExist(err) {
		t.Fatalf("consumed recovery remains: %v", err)
	}
}

func TestClaudeRecoveryRejectsAttemptDrift(t *testing.T) {
	root := t.TempDir()
	pkg := recoveryPackageForTest()
	opt := recoveryOptionsForTest()
	output := json.RawMessage(`{"opaque":"bounded-bytes-2"}`)
	run := claudeRun{envelope: claudeEnvelope{Type: "result", Subtype: "success", SessionID: "session-id"}, sessionID: "session-id", structuredOutput: output, started: true, exitCode: 0}
	if err := persistClaudeRecovery(root, opt, pkg, run); err != nil {
		t.Fatal(err)
	}
	original := claudeRecoveryPath(pkg)
	pkg.Launch.Attempt.AttemptSHA256 = "different-attempt-sha"
	drifted := claudeRecoveryPath(pkg)
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(original)))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(drifted)), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := recoverClaudeRun(root, opt, pkg); err == nil || ok {
		t.Fatalf("attempt drift recovery ok=%t err=%v", ok, err)
	}
}

func TestClaudeRecoveryRejectsExecutableDrift(t *testing.T) {
	root := t.TempDir()
	pkg := recoveryPackageForTest()
	opt := recoveryOptionsForTest()
	output := json.RawMessage(`{"opaque":"bounded-bytes-3"}`)
	run := claudeRun{envelope: claudeEnvelope{Type: "result", Subtype: "success", SessionID: "session-id"}, sessionID: "session-id", structuredOutput: output, started: true, exitCode: 0}
	if err := persistClaudeRecovery(root, opt, pkg, run); err != nil {
		t.Fatal(err)
	}
	drifted := opt
	drifted.ExpectedClaudeExecutableSHA256 = strings.Repeat("b", 64)
	if _, ok, err := recoverClaudeRun(root, drifted, pkg); err == nil || ok {
		t.Fatalf("executable drift recovery ok=%t err=%v", ok, err)
	}
}

func TestClaudeReviewerRecoveryUsesDispatchAndSessionBinding(t *testing.T) {
	root := t.TempDir()
	pkg := recoveryPackageForTest()
	opt := recoveryOptionsForTest()
	pkg.SessionKind = "reviewer"
	pkg.Launch.Attempt.AttemptID = "reviewer-dispatch-id"
	output := json.RawMessage(`{"opaque":"bounded-bytes-4"}`)
	run := claudeRun{envelope: claudeEnvelope{Type: "result", Subtype: "success", SessionID: "session-id"}, sessionID: "session-id", structuredOutput: output, started: true, exitCode: 0}
	if err := persistClaudeRecovery(root, opt, pkg, run); err != nil {
		t.Fatal(err)
	}
	recovered, ok, err := recoverClaudeRun(root, opt, pkg)
	if err != nil || !ok || !bytes.Equal(recovered.structuredOutput, output) {
		t.Fatalf("reviewer recovery ok=%t err=%v run=%+v", ok, err, recovered)
	}
	originalPath := filepath.Join(root, filepath.FromSlash(claudeRecoveryPath(pkg)))
	data, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	pkg.Launch.Attempt.Session = "replacement-session"
	driftedPath := filepath.Join(root, filepath.FromSlash(claudeRecoveryPath(pkg)))
	if err := os.WriteFile(driftedPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := recoverClaudeRun(root, opt, pkg); err == nil || ok {
		t.Fatalf("reviewer session drift recovery ok=%t err=%v", ok, err)
	}
}

func TestClaudeRecoveryIgnoresCaseLocalForgery(t *testing.T) {
	caseRoot := t.TempDir()
	pkg := recoveryPackageForTest()
	opt := recoveryOptionsForTest()
	output := json.RawMessage(`{"opaque":"case-local-forgery"}`)
	recovery, _, err := claudeRecoveryFor(opt, pkg, pkg.Launch.Attempt.Session, output)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(recovery)
	if err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(caseRoot, ".rekit", "session-host", "structured-results", claudeRecoveryPath(pkg))
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := recoverClaudeRunForCase(caseRoot, opt, pkg); err != nil || ok {
		t.Fatalf("case-local forgery recovery ok=%t err=%v", ok, err)
	}
}

func recoveryOptionsForTest() Options {
	return Options{
		ExpectedClaudeExecutableSHA256:    strings.Repeat("a", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
	}
}

func recoveryPackageForTest() mission.CurrentLoopExternalSessionHarnessPackage {
	return mission.CurrentLoopExternalSessionHarnessPackage{
		SessionKind: "member",
		Launch: &mission.CurrentLoopExternalSessionHarnessLaunch{Attempt: mission.CurrentLoopExternalSessionAttempt{
			AttemptID: "attempt-id", AttemptSHA256: "attempt-sha", Session: "session-id",
		}},
	}
}
