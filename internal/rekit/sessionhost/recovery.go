package sessionhost

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

const claudeRecoveryKind = "claude-structured-output-recovery"

type claudeRecovery struct {
	SchemaVersion          int    `json:"schemaVersion"`
	Kind                   string `json:"kind"`
	SessionKind            string `json:"sessionKind"`
	AttemptID              string `json:"attemptId"`
	AttemptSHA256          string `json:"attemptSha256"`
	SessionID              string `json:"sessionId"`
	StructuredOutputBase64 string `json:"structuredOutputBase64"`
	StructuredOutputSHA256 string `json:"structuredOutputSha256"`
}

func persistClaudeRecovery(caseRoot string, pkg mission.CurrentLoopExternalSessionHarnessPackage, run claudeRun) error {
	if !run.success() {
		return nil
	}
	recovery, rel, err := claudeRecoveryFor(pkg, run.sessionID, run.structuredOutput)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(recovery, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = rekitfs.WriteExclusiveRegularFileAnchored(caseRoot, rel, "Claude structured output recovery", data)
	return err
}

func recoverClaudeRun(caseRoot string, pkg mission.CurrentLoopExternalSessionHarnessPackage) (claudeRun, bool, error) {
	if pkg.Launch == nil {
		return claudeRun{}, false, fmt.Errorf("Claude recovery requires a launch binding")
	}
	rel := claudeRecoveryPath(pkg)
	path, err := rekitfs.SafeJoin(caseRoot, rel)
	if err != nil {
		return claudeRun{}, false, err
	}
	data, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, path, "Claude structured output recovery", maxClaudeStdoutBytes*2)
	if errors.Is(err, os.ErrNotExist) {
		return claudeRun{}, false, nil
	}
	if err != nil {
		return claudeRun{}, false, err
	}
	var recovery claudeRecovery
	if err := strictJSON(data, &recovery); err != nil {
		return claudeRun{}, false, fmt.Errorf("decode Claude structured output recovery: %w", err)
	}
	output, err := base64.StdEncoding.DecodeString(recovery.StructuredOutputBase64)
	if err != nil {
		return claudeRun{}, false, fmt.Errorf("decode Claude recovery output: %w", err)
	}
	expected, _, err := claudeRecoveryFor(pkg, pkg.Launch.Attempt.Session, output)
	if err != nil {
		return claudeRun{}, false, err
	}
	if recovery.SchemaVersion != expected.SchemaVersion || recovery.Kind != expected.Kind || recovery.SessionKind != expected.SessionKind || recovery.AttemptID != expected.AttemptID || recovery.AttemptSHA256 != expected.AttemptSHA256 || recovery.SessionID != expected.SessionID || !strings.EqualFold(recovery.StructuredOutputSHA256, expected.StructuredOutputSHA256) || recovery.StructuredOutputBase64 != expected.StructuredOutputBase64 {
		return claudeRun{}, false, fmt.Errorf("Claude structured output recovery does not match the current attempt and session")
	}
	return claudeRun{
		envelope:         claudeEnvelope{Type: "result", Subtype: "success", SessionID: recovery.SessionID},
		sessionID:        recovery.SessionID,
		structuredOutput: output,
		recovered:        true,
		exitCode:         0,
	}, true, nil
}

func claudeRecoveryFor(pkg mission.CurrentLoopExternalSessionHarnessPackage, sessionID string, output []byte) (claudeRecovery, string, error) {
	if pkg.Launch == nil || strings.TrimSpace(pkg.SessionKind) == "" || strings.TrimSpace(pkg.Launch.Attempt.AttemptID) == "" || strings.TrimSpace(pkg.Launch.Attempt.AttemptSHA256) == "" || strings.TrimSpace(sessionID) == "" || pkg.Launch.Attempt.Session != sessionID || len(output) == 0 {
		return claudeRecovery{}, "", fmt.Errorf("Claude recovery requires exact kind, attempt, session, and structured output bindings")
	}
	recovery := claudeRecovery{
		SchemaVersion:          1,
		Kind:                   claudeRecoveryKind,
		SessionKind:            pkg.SessionKind,
		AttemptID:              pkg.Launch.Attempt.AttemptID,
		AttemptSHA256:          pkg.Launch.Attempt.AttemptSHA256,
		SessionID:              sessionID,
		StructuredOutputBase64: base64.StdEncoding.EncodeToString(output),
		StructuredOutputSHA256: bytesSHA256(output),
	}
	return recovery, claudeRecoveryPath(pkg), nil
}

func claudeRecoveryPath(pkg mission.CurrentLoopExternalSessionHarnessPackage) string {
	attempt := mission.CurrentLoopExternalSessionAttempt{}
	if pkg.Launch != nil {
		attempt = pkg.Launch.Attempt
	}
	identity := strings.Join([]string{pkg.SessionKind, attempt.AttemptID, attempt.AttemptSHA256, attempt.Session}, "\n")
	return filepath.ToSlash(filepath.Join(".rekit", "session-host", "structured-results", bytesSHA256([]byte(identity))+".json"))
}
