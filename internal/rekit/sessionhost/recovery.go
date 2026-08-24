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
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/capabilitycontract"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instructionpacket"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

const claudeRecoveryKind = "claude-structured-output-recovery"

type claudeRecovery struct {
	SchemaVersion             int                         `json:"schemaVersion"`
	Kind                      string                      `json:"kind"`
	SessionKind               string                      `json:"sessionKind"`
	AttemptID                 string                      `json:"attemptId"`
	AttemptSHA256             string                      `json:"attemptSha256"`
	SessionID                 string                      `json:"sessionId"`
	Pack                      string                      `json:"pack,omitempty"`
	InstructionIdentity       *instructionpacket.Identity `json:"instructionIdentity,omitempty"`
	Capability                capabilitycontract.Binding  `json:"capability"`
	LaunchControl             *claudeLaunchControlBinding `json:"launchControl,omitempty"`
	ObservedAt                string                      `json:"observedAt,omitempty"`
	ClaudeExecutableSHA256    string                      `json:"claudeExecutableSha256"`
	ClaudeExecutablePublisher string                      `json:"claudeExecutablePublisher"`
	StructuredOutputBase64    string                      `json:"structuredOutputBase64"`
	StructuredOutputSHA256    string                      `json:"structuredOutputSha256"`
}

func persistClaudeRecoveryForCase(caseRoot string, opt Options, pkg mission.CurrentLoopExternalSessionHarnessPackage, run claudeRun) error {
	if !run.success() || !trustedRecoveryProvenance(opt) {
		return nil
	}
	required, err := claudeLaunchControlRequired(caseRoot)
	if err != nil {
		return err
	}
	if required && run.launchControlBinding == nil {
		return fmt.Errorf("Claude recovery omitted the result birth control binding")
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
	opt.launchControlBinding = cloneClaudeLaunchControlBinding(run.launchControlBinding)
	observedAt := strings.TrimSpace(run.observedAt)
	if observedAt == "" {
		observedAt = nowRFC3339Nano()
	}
	recovery, rel, err := claudeRecoveryFor(opt, pkg, run.sessionID, run.structuredOutput, observedAt)
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

func removeClaudeRecoveryForCase(caseRoot string, opt Options, pkg mission.CurrentLoopExternalSessionHarnessPackage, run claudeRun) error {
	if !trustedRecoveryProvenance(opt) {
		return nil
	}
	opt.launchControlBinding = cloneClaudeLaunchControlBinding(run.launchControlBinding)
	rootPath, err := claudeRecoveryRoot(caseRoot)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.Remove(claudeRecoveryPath(pkg, opt.launchControlBinding)); err != nil && !os.IsNotExist(err) {
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
	required, err := claudeLaunchControlRequired(caseRoot)
	if err != nil {
		return claudeRun{}, false, err
	}
	if required && opt.launchControlBinding == nil {
		return discoverClaudeRunForCase(caseRoot, root, opt, pkg)
	}
	run, data, path, recovered, err := recoverClaudeRunAt(root, opt, pkg, claudeRecoveryPath(pkg, opt.launchControlBinding))
	if err != nil || !recovered {
		return run, recovered, err
	}
	run, err = bindClaudeRawResultArtifact(run, path, data)
	if err != nil {
		return claudeRun{}, false, err
	}
	return run, true, nil
}

func recoverClaudeRun(root string, opt Options, pkg mission.CurrentLoopExternalSessionHarnessPackage) (claudeRun, bool, error) {
	rel := claudeRecoveryPath(pkg, opt.launchControlBinding)
	run, _, _, recovered, err := recoverClaudeRunAt(root, opt, pkg, rel)
	if err != nil || recovered {
		return run, recovered, err
	}
	stale, staleErr := hasClaudeRecoveryForOtherControl(root, pkg, rel, opt.launchControlBinding != nil)
	if staleErr != nil {
		return claudeRun{}, false, staleErr
	}
	if stale {
		return claudeRun{}, false, fmt.Errorf("Claude structured output recovery exists for a different execution control head")
	}
	return claudeRun{}, false, nil
}

func recoverClaudeRunAt(
	root string,
	opt Options,
	pkg mission.CurrentLoopExternalSessionHarnessPackage,
	rel string,
) (claudeRun, []byte, string, bool, error) {
	if pkg.Launch == nil {
		return claudeRun{}, nil, "", false, fmt.Errorf("Claude recovery requires a launch binding")
	}
	path, err := rekitfs.SafeJoin(root, rel)
	if err != nil {
		return claudeRun{}, nil, "", false, err
	}
	data, err := rekitfs.ReadStableRegularFileAnchored(root, path, "Claude structured output recovery", maxClaudeRawArtifactBytes)
	if errors.Is(err, os.ErrNotExist) {
		return claudeRun{}, nil, path, false, nil
	}
	if err != nil {
		return claudeRun{}, nil, "", false, err
	}
	var recovery claudeRecovery
	if err := strictJSON(data, &recovery); err != nil {
		return claudeRun{}, nil, "", false, fmt.Errorf("decode Claude structured output recovery: %w", err)
	}
	output, err := base64.StdEncoding.DecodeString(recovery.StructuredOutputBase64)
	if err != nil {
		return claudeRun{}, nil, "", false, fmt.Errorf("decode Claude recovery output: %w", err)
	}
	expected, expectedRel, err := claudeRecoveryFor(opt, pkg, pkg.Launch.Attempt.Session, output, recovery.ObservedAt)
	if err != nil {
		return claudeRun{}, nil, "", false, err
	}
	if filepath.Clean(expectedRel) != filepath.Clean(rel) || !claudeRecoveryEqual(recovery, expected) {
		return claudeRun{}, nil, "", false, fmt.Errorf("Claude structured output recovery does not match its exact attempt, session, control head, observation time, path, and trusted executable")
	}
	return claudeRun{
		launchControlBinding: cloneClaudeLaunchControlBinding(recovery.LaunchControl),
		envelope:             claudeEnvelope{Type: "result", Subtype: "success", SessionID: recovery.SessionID},
		sessionID:            recovery.SessionID,
		structuredOutput:     output,
		recovered:            true,
		exitCode:             0,
		observedAt:           recovery.ObservedAt,
	}, data, path, true, nil
}

func discoverClaudeRunForCase(
	caseRoot,
	root string,
	opt Options,
	pkg mission.CurrentLoopExternalSessionHarnessPackage,
) (claudeRun, bool, error) {
	owner, err := claudeLaunchOwner(caseRoot, pkg)
	if err != nil {
		return claudeRun{}, false, err
	}
	attemptRel := claudeRecoveryAttemptPath(pkg)
	attemptPath, err := rekitfs.SafeJoin(root, attemptRel)
	if err != nil {
		return claudeRun{}, false, err
	}
	entries, err := os.ReadDir(attemptPath)
	if os.IsNotExist(err) {
		legacyPath, joinErr := rekitfs.SafeJoin(root, attemptRel+".json")
		if joinErr != nil {
			return claudeRun{}, false, joinErr
		}
		if _, statErr := os.Lstat(legacyPath); statErr == nil {
			return claudeRun{}, false, fmt.Errorf("attached project Claude recovery omitted its execution control birth binding")
		} else if !os.IsNotExist(statErr) {
			return claudeRun{}, false, statErr
		}
		return claudeRun{}, false, nil
	}
	if err != nil {
		return claudeRun{}, false, err
	}
	if len(entries) > 64 {
		return claudeRun{}, false, fmt.Errorf("Claude recovery attempt contains too many control-head results")
	}
	var recovered claudeRun
	found := false
	for _, entry := range entries {
		name := entry.Name()
		if entry.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(name, ".json") || !validClaudeLaunchSHA256(strings.TrimSuffix(name, ".json")) {
			return claudeRun{}, false, fmt.Errorf("Claude recovery attempt contains an invalid control-head artifact: %s", name)
		}
		info, infoErr := entry.Info()
		if infoErr != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return claudeRun{}, false, errors.Join(infoErr, fmt.Errorf("Claude recovery candidate must be a regular non-symlink file: %s", name))
		}
		rel := filepath.Join(attemptRel, name)
		path, joinErr := rekitfs.SafeJoin(root, rel)
		if joinErr != nil {
			return claudeRun{}, false, joinErr
		}
		data, readErr := rekitfs.ReadStableRegularFileAnchored(root, path, "Claude structured output recovery", maxClaudeRawArtifactBytes)
		if readErr != nil {
			return claudeRun{}, false, readErr
		}
		var candidate claudeRecovery
		if decodeErr := strictJSON(data, &candidate); decodeErr != nil {
			return claudeRun{}, false, fmt.Errorf("decode Claude structured output recovery: %w", decodeErr)
		}
		if candidate.LaunchControl == nil || candidate.LaunchControl.Owner != owner {
			return claudeRun{}, false, fmt.Errorf("Claude recovery control-head artifact does not match the immutable launch owner")
		}
		candidateOpt := opt
		candidateOpt.Target = caseRoot
		candidateOpt.launchControlBinding = cloneClaudeLaunchControlBinding(candidate.LaunchControl)
		run, exactData, exactPath, ok, recoverErr := recoverClaudeRunAt(root, candidateOpt, pkg, rel)
		if recoverErr != nil || !ok {
			return claudeRun{}, false, errors.Join(recoverErr, fmt.Errorf("Claude recovery control-head artifact could not be recovered exactly"))
		}
		if found {
			return claudeRun{}, false, fmt.Errorf("Claude recovery attempt contains multiple valid control-head results")
		}
		recovered, err = bindClaudeRawResultArtifact(run, exactPath, exactData)
		if err != nil {
			return claudeRun{}, false, err
		}
		found = true
	}
	return recovered, found, nil
}

func claudeRecoveryFor(opt Options, pkg mission.CurrentLoopExternalSessionHarnessPackage, sessionID string, output []byte, observedAt string) (claudeRecovery, string, error) {
	hash := strings.ToLower(strings.TrimSpace(opt.ExpectedClaudeExecutableSHA256))
	publisher := strings.TrimSpace(opt.ExpectedClaudeExecutablePublisher)
	observedAt = strings.TrimSpace(observedAt)
	if pkg.Launch == nil || strings.TrimSpace(pkg.SessionKind) == "" || strings.TrimSpace(pkg.Launch.Attempt.AttemptID) == "" || strings.TrimSpace(pkg.Launch.Attempt.AttemptSHA256) == "" || strings.TrimSpace(sessionID) == "" || pkg.Launch.Attempt.Session != sessionID || len(output) == 0 || len(hash) != 64 || !strings.EqualFold(publisher, liveAcceptanceClaudePublisher) {
		return claudeRecovery{}, "", fmt.Errorf("Claude recovery requires exact kind, attempt, session, output, and trusted executable bindings")
	}
	if err := validateClaudeCapabilityPolicy(pkg); err != nil {
		return claudeRecovery{}, "", err
	}
	if err := validateProductionInstructionBirth(pkg.Pack, pkg.Launch.InstructionIdentity); err != nil {
		return claudeRecovery{}, "", err
	}
	if observedAt != "" {
		parsed, err := time.Parse(time.RFC3339Nano, observedAt)
		if err != nil || parsed.Format(time.RFC3339Nano) != observedAt {
			return claudeRecovery{}, "", fmt.Errorf("Claude recovery observedAt must be canonical RFC3339Nano")
		}
	}
	if opt.launchControlBinding != nil {
		if err := validateClaudeLaunchControlBinding(*opt.launchControlBinding); err != nil {
			return claudeRecovery{}, "", err
		}
		if observedAt == "" {
			return claudeRecovery{}, "", fmt.Errorf("Claude recovery with an execution control binding requires observedAt")
		}
	}
	schemaVersion := 1
	if opt.launchControlBinding != nil {
		schemaVersion = 2
	}
	recovery := claudeRecovery{
		SchemaVersion:             schemaVersion,
		Kind:                      claudeRecoveryKind,
		SessionKind:               pkg.SessionKind,
		AttemptID:                 pkg.Launch.Attempt.AttemptID,
		AttemptSHA256:             pkg.Launch.Attempt.AttemptSHA256,
		SessionID:                 sessionID,
		Pack:                      pkg.Pack,
		InstructionIdentity:       cloneProductionInstructionIdentityPointer(pkg.Launch.InstructionIdentity),
		Capability:                pkg.Launch.Capability,
		LaunchControl:             cloneClaudeLaunchControlBinding(opt.launchControlBinding),
		ObservedAt:                observedAt,
		ClaudeExecutableSHA256:    hash,
		ClaudeExecutablePublisher: publisher,
		StructuredOutputBase64:    base64.StdEncoding.EncodeToString(output),
		StructuredOutputSHA256:    bytesSHA256(output),
	}
	return recovery, claudeRecoveryPath(pkg, opt.launchControlBinding), nil
}

func trustedRecoveryProvenance(opt Options) bool {
	return len(strings.TrimSpace(opt.ExpectedClaudeExecutableSHA256)) == 64 &&
		strings.EqualFold(strings.TrimSpace(opt.ExpectedClaudeExecutablePublisher), liveAcceptanceClaudePublisher)
}

func claudeRecoveryRoot(caseRoot string) (string, error) {
	root, cacheRoot, relRoot, caseSHA, err := claudeRecoveryRootIdentity(caseRoot)
	if err != nil {
		return "", err
	}
	marker := []byte("rekit Claude recovery root v1\ncaseSha256=" + caseSHA + "\n")
	if _, err := rekitfs.WriteExclusiveRegularFileAnchored(cacheRoot, filepath.Join(relRoot, ".binding"), "Claude recovery root binding", marker); err != nil {
		return "", err
	}
	return root, nil
}

func claudeRecoveryRootPath(caseRoot string) (string, error) {
	root, _, _, _, err := claudeRecoveryRootIdentity(caseRoot)
	return root, err
}

func claudeRecoveryRootIdentity(caseRoot string) (root, cacheRoot, relRoot, caseSHA string, err error) {
	caseRoot, err = filepath.Abs(caseRoot)
	if err != nil {
		return "", "", "", "", err
	}
	cacheRoot, err = os.UserCacheDir()
	if err != nil {
		return "", "", "", "", fmt.Errorf("resolve host-owned Claude recovery root: %w", err)
	}
	cacheRoot, err = filepath.Abs(cacheRoot)
	if err != nil {
		return "", "", "", "", err
	}
	identityPath := filepath.Clean(caseRoot)
	if runtime.GOOS == "windows" {
		identityPath = strings.ToLower(identityPath)
	}
	caseSHA = bytesSHA256([]byte(identityPath))
	relRoot = filepath.Join("rekit", "session-host", "v1", "cases", caseSHA, "structured-results")
	root = filepath.Join(cacheRoot, relRoot)
	if pathsOverlap(caseRoot, root) {
		return "", "", "", "", fmt.Errorf("host-owned Claude recovery root must be outside the attached case: %s", caseRoot)
	}
	return root, cacheRoot, relRoot, caseSHA, nil
}

func pathsOverlap(left, right string) bool {
	inside := func(root, path string) bool {
		rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
		return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))))
	}
	return inside(left, right) || inside(right, left)
}

func claudeRecoveryPath(pkg mission.CurrentLoopExternalSessionHarnessPackage, bindings ...*claudeLaunchControlBinding) string {
	var binding *claudeLaunchControlBinding
	if len(bindings) > 0 {
		binding = bindings[0]
	}
	attemptPath := claudeRecoveryAttemptPath(pkg)
	if binding == nil {
		return attemptPath + ".json"
	}
	controlIdentity, _ := json.Marshal(binding)
	return filepath.Join(attemptPath, bytesSHA256(controlIdentity)+".json")
}

func claudeRecoveryAttemptPath(pkg mission.CurrentLoopExternalSessionHarnessPackage) string {
	attempt := mission.CurrentLoopExternalSessionAttempt{}
	if pkg.Launch != nil {
		attempt = pkg.Launch.Attempt
	}
	identity := strings.Join([]string{pkg.SessionKind, attempt.AttemptID, attempt.AttemptSHA256, attempt.Session}, "\n")
	return bytesSHA256([]byte(identity))
}

func hasClaudeRecoveryForOtherControl(root string, pkg mission.CurrentLoopExternalSessionHarnessPackage, currentRel string, checkLegacy bool) (bool, error) {
	attemptRel := claudeRecoveryAttemptPath(pkg)
	attemptPath, err := rekitfs.SafeJoin(root, attemptRel)
	if err != nil {
		return false, err
	}
	entries, err := os.ReadDir(attemptPath)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if len(entries) > 64 {
		return false, fmt.Errorf("Claude recovery attempt contains too many control-head results")
	}
	currentName := filepath.Base(currentRel)
	for _, entry := range entries {
		if entry.Name() == currentName {
			continue
		}
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".json") {
			return true, nil
		}
	}
	if !checkLegacy {
		return false, nil
	}
	legacyPath, err := rekitfs.SafeJoin(root, attemptRel+".json")
	if err != nil {
		return false, err
	}
	info, err := os.Lstat(legacyPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0, nil
}

func claudeRecoveryEqual(left, right claudeRecovery) bool {
	if left.Capability != right.Capability ||
		!sameClaudeLaunchControlBinding(left.LaunchControl, right.LaunchControl) ||
		!equalProductionInstructionIdentityPointers(left.InstructionIdentity, right.InstructionIdentity) {
		return false
	}
	left.Capability = capabilitycontract.Binding{}
	right.Capability = capabilitycontract.Binding{}
	left.LaunchControl = nil
	right.LaunchControl = nil
	left.InstructionIdentity = nil
	right.InstructionIdentity = nil
	return left == right
}
