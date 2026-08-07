package sessionhost

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

const claudeRecoveryKind = "claude-structured-output-recovery"

type claudeRecovery struct {
	SchemaVersion             int    `json:"schemaVersion"`
	Kind                      string `json:"kind"`
	SessionKind               string `json:"sessionKind"`
	AttemptID                 string `json:"attemptId"`
	AttemptSHA256             string `json:"attemptSha256"`
	SessionID                 string `json:"sessionId"`
	ClaudeExecutableSHA256    string `json:"claudeExecutableSha256"`
	ClaudeExecutablePublisher string `json:"claudeExecutablePublisher"`
	StructuredOutputBase64    string `json:"structuredOutputBase64"`
	StructuredOutputSHA256    string `json:"structuredOutputSha256"`
}

func persistClaudeRecoveryForCase(caseRoot string, opt Options, pkg mission.CurrentLoopExternalSessionHarnessPackage, run claudeRun) error {
	if !run.success() || !trustedRecoveryProvenance(opt) {
		return nil
	}
	root, err := claudeRecoveryRoot(caseRoot)
	if err != nil {
		return err
	}
	return persistClaudeRecovery(root, opt, pkg, run)
}

func persistClaudeRecovery(root string, opt Options, pkg mission.CurrentLoopExternalSessionHarnessPackage, run claudeRun) error {
	if !run.success() {
		return nil
	}
	recovery, rel, err := claudeRecoveryFor(opt, pkg, run.sessionID, run.structuredOutput)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(recovery, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = rekitfs.WriteExclusiveRegularFileAnchored(root, rel, "Claude structured output recovery", data)
	return err
}

func removeClaudeRecoveryForCase(caseRoot string, opt Options, pkg mission.CurrentLoopExternalSessionHarnessPackage) error {
	if !trustedRecoveryProvenance(opt) {
		return nil
	}
	rootPath, err := claudeRecoveryRoot(caseRoot)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.Remove(claudeRecoveryPath(pkg)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove consumed Claude structured output recovery: %w", err)
	}
	return nil
}

func recoverClaudeRunForCase(caseRoot string, opt Options, pkg mission.CurrentLoopExternalSessionHarnessPackage) (claudeRun, bool, error) {
	if !trustedRecoveryProvenance(opt) {
		return claudeRun{}, false, nil
	}
	root, err := claudeRecoveryRoot(caseRoot)
	if err != nil {
		return claudeRun{}, false, err
	}
	return recoverClaudeRun(root, opt, pkg)
}

func recoverClaudeRun(root string, opt Options, pkg mission.CurrentLoopExternalSessionHarnessPackage) (claudeRun, bool, error) {
	if pkg.Launch == nil {
		return claudeRun{}, false, fmt.Errorf("Claude recovery requires a launch binding")
	}
	rel := claudeRecoveryPath(pkg)
	path, err := rekitfs.SafeJoin(root, rel)
	if err != nil {
		return claudeRun{}, false, err
	}
	data, err := rekitfs.ReadStableRegularFileAnchored(root, path, "Claude structured output recovery", maxClaudeStdoutBytes*2)
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
	expected, _, err := claudeRecoveryFor(opt, pkg, pkg.Launch.Attempt.Session, output)
	if err != nil {
		return claudeRun{}, false, err
	}
	if recovery != expected {
		return claudeRun{}, false, fmt.Errorf("Claude structured output recovery does not match the current attempt, session, and trusted executable")
	}
	return claudeRun{
		envelope:         claudeEnvelope{Type: "result", Subtype: "success", SessionID: recovery.SessionID},
		sessionID:        recovery.SessionID,
		structuredOutput: output,
		recovered:        true,
		exitCode:         0,
	}, true, nil
}

func claudeRecoveryFor(opt Options, pkg mission.CurrentLoopExternalSessionHarnessPackage, sessionID string, output []byte) (claudeRecovery, string, error) {
	hash := strings.ToLower(strings.TrimSpace(opt.ExpectedClaudeExecutableSHA256))
	publisher := strings.TrimSpace(opt.ExpectedClaudeExecutablePublisher)
	if pkg.Launch == nil || strings.TrimSpace(pkg.SessionKind) == "" || strings.TrimSpace(pkg.Launch.Attempt.AttemptID) == "" || strings.TrimSpace(pkg.Launch.Attempt.AttemptSHA256) == "" || strings.TrimSpace(sessionID) == "" || pkg.Launch.Attempt.Session != sessionID || len(output) == 0 || len(hash) != 64 || !strings.EqualFold(publisher, liveAcceptanceClaudePublisher) {
		return claudeRecovery{}, "", fmt.Errorf("Claude recovery requires exact kind, attempt, session, output, and trusted executable bindings")
	}
	recovery := claudeRecovery{
		SchemaVersion:             1,
		Kind:                      claudeRecoveryKind,
		SessionKind:               pkg.SessionKind,
		AttemptID:                 pkg.Launch.Attempt.AttemptID,
		AttemptSHA256:             pkg.Launch.Attempt.AttemptSHA256,
		SessionID:                 sessionID,
		ClaudeExecutableSHA256:    hash,
		ClaudeExecutablePublisher: publisher,
		StructuredOutputBase64:    base64.StdEncoding.EncodeToString(output),
		StructuredOutputSHA256:    bytesSHA256(output),
	}
	return recovery, claudeRecoveryPath(pkg), nil
}

func trustedRecoveryProvenance(opt Options) bool {
	return len(strings.TrimSpace(opt.ExpectedClaudeExecutableSHA256)) == 64 &&
		strings.EqualFold(strings.TrimSpace(opt.ExpectedClaudeExecutablePublisher), liveAcceptanceClaudePublisher)
}

func claudeRecoveryRoot(caseRoot string) (string, error) {
	caseRoot, err := filepath.Abs(caseRoot)
	if err != nil {
		return "", err
	}
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve host-owned Claude recovery root: %w", err)
	}
	cacheRoot, err = filepath.Abs(cacheRoot)
	if err != nil {
		return "", err
	}
	identityPath := filepath.Clean(caseRoot)
	if runtime.GOOS == "windows" {
		identityPath = strings.ToLower(identityPath)
	}
	caseSHA := bytesSHA256([]byte(identityPath))
	relRoot := filepath.Join("rekit", "session-host", "v1", "cases", caseSHA, "structured-results")
	root := filepath.Join(cacheRoot, relRoot)
	if pathsOverlap(caseRoot, root) {
		return "", fmt.Errorf("host-owned Claude recovery root must be outside the attached case: %s", caseRoot)
	}
	marker := []byte("rekit Claude recovery root v1\ncaseSha256=" + caseSHA + "\n")
	if _, err := rekitfs.WriteExclusiveRegularFileAnchored(cacheRoot, filepath.Join(relRoot, ".binding"), "Claude recovery root binding", marker); err != nil {
		return "", err
	}
	return root, nil
}

func pathsOverlap(left, right string) bool {
	inside := func(root, path string) bool {
		rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
		return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))))
	}
	return inside(left, right) || inside(right, left)
}

func claudeRecoveryPath(pkg mission.CurrentLoopExternalSessionHarnessPackage) string {
	attempt := mission.CurrentLoopExternalSessionAttempt{}
	if pkg.Launch != nil {
		attempt = pkg.Launch.Attempt
	}
	identity := strings.Join([]string{pkg.SessionKind, attempt.AttemptID, attempt.AttemptSHA256, attempt.Session}, "\n")
	return bytesSHA256([]byte(identity)) + ".json"
}
