package sessionhost

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestClaudeRecoveryTreatsMissingArtifactAsFreshLaunch(t *testing.T) {
	caseRoot := t.TempDir()
	pkg := recoveryPackageForTest()

	recovered, ok, err := recoverClaudeRun(caseRoot, pkg)
	if err != nil || ok {
		t.Fatalf("missing recovery ok=%t err=%v run=%+v", ok, err, recovered)
	}
}

func TestClaudeRecoveryRoundTripsExactAttemptSessionAndBytes(t *testing.T) {
	caseRoot := t.TempDir()
	pkg := recoveryPackageForTest()
	output := json.RawMessage(`{"outcome":"returned","summary":"fixture","reason":"","outputs":[{"path":"result.txt","content":"fixture bytes"}],"reviewerItemsPath":""}`)
	run := claudeRun{
		envelope:         claudeEnvelope{Type: "result", Subtype: "success", SessionID: "session-id"},
		sessionID:        "session-id",
		structuredOutput: output,
		started:          true,
		exitCode:         0,
	}
	if err := persistClaudeRecovery(caseRoot, pkg, run); err != nil {
		t.Fatal(err)
	}
	recovered, ok, err := recoverClaudeRun(caseRoot, pkg)
	if err != nil || !ok {
		t.Fatalf("recovery ok=%t err=%v", ok, err)
	}
	if recovered.sessionID != run.sessionID || !recovered.success() || !recovered.recovered || recovered.started || !bytes.Equal(recovered.structuredOutput, output) {
		t.Fatalf("recovered run drifted: %+v", recovered)
	}
}

func TestClaudeRecoveryRejectsAttemptDrift(t *testing.T) {
	caseRoot := t.TempDir()
	pkg := recoveryPackageForTest()
	output := json.RawMessage(`{"outcome":"failed","summary":"","reason":"fixture","outputs":[],"reviewerItemsPath":""}`)
	run := claudeRun{envelope: claudeEnvelope{Type: "result", Subtype: "success", SessionID: "session-id"}, sessionID: "session-id", structuredOutput: output, started: true, exitCode: 0}
	if err := persistClaudeRecovery(caseRoot, pkg, run); err != nil {
		t.Fatal(err)
	}
	original := claudeRecoveryPath(pkg)
	pkg.Launch.Attempt.AttemptSHA256 = "different-attempt-sha"
	drifted := claudeRecoveryPath(pkg)
	originalPath := filepath.Join(caseRoot, filepath.FromSlash(original))
	driftedPath := filepath.Join(caseRoot, filepath.FromSlash(drifted))
	if err := os.MkdirAll(filepath.Dir(driftedPath), 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(driftedPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := recoverClaudeRun(caseRoot, pkg); err == nil || ok {
		t.Fatalf("attempt drift recovery ok=%t err=%v", ok, err)
	}
}

func TestClaudeReviewerRecoveryUsesDispatchAndSessionBinding(t *testing.T) {
	caseRoot := t.TempDir()
	pkg := recoveryPackageForTest()
	pkg.SessionKind = "reviewer"
	pkg.Launch.Attempt.AttemptID = "reviewer-dispatch-id"
	output := json.RawMessage(`{"outcome":"returned","result":{"reviewerSession":"session-id"},"reason":""}`)
	run := claudeRun{envelope: claudeEnvelope{Type: "result", Subtype: "success", SessionID: "session-id"}, sessionID: "session-id", structuredOutput: output, started: true, exitCode: 0}
	if err := persistClaudeRecovery(caseRoot, pkg, run); err != nil {
		t.Fatal(err)
	}
	recovered, ok, err := recoverClaudeRun(caseRoot, pkg)
	if err != nil || !ok || !bytes.Equal(recovered.structuredOutput, output) {
		t.Fatalf("reviewer recovery ok=%t err=%v run=%+v", ok, err, recovered)
	}
	originalPath := filepath.Join(caseRoot, filepath.FromSlash(claudeRecoveryPath(pkg)))
	data, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	pkg.Launch.Attempt.Session = "replacement-session"
	driftedPath := filepath.Join(caseRoot, filepath.FromSlash(claudeRecoveryPath(pkg)))
	if err := os.WriteFile(driftedPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := recoverClaudeRun(caseRoot, pkg); err == nil || ok {
		t.Fatalf("reviewer session drift recovery ok=%t err=%v", ok, err)
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
