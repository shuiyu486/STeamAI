package sessionhost

import (
	"encoding/json"
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"runtime"
	"strings"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
)

const (
	hostCacheArtifactPrefix   = "host-cache://"
	maxClaudeRawArtifactBytes = 32 * 1024 * 1024
)

func ensureClaudeRawResultArtifact(caseRoot string, run claudeRun) (claudeRun, error) {
	if run.rawResultRef != "" || run.rawResultSHA256 != "" || run.rawResultBytes != 0 {
		if err := validateClaudeRawResultArtifact(caseRoot, run); err != nil {
			return claudeRun{}, err
		}
		return run, nil
	}
	data, err := claudeRawResultBytes(run)
	if err != nil {
		return claudeRun{}, err
	}
	root, err := claudeRawResultRoot(caseRoot)
	if err != nil {
		return claudeRun{}, err
	}
	rel := bytesSHA256(data) + ".json"
	if _, err := rekitfs.WriteExclusiveRegularFileAnchored(root, rel, "host-owned Claude raw result", data); err != nil {
		return claudeRun{}, err
	}
	return bindClaudeRawResultArtifact(run, filepath.Join(root, rel), data)
}

func claudeRawResultBytes(run claudeRun) ([]byte, error) {
	if len(run.structuredOutput) > 0 {
		return append([]byte{}, run.structuredOutput...), nil
	}
	raw := struct {
		SchemaVersion int            `json:"schemaVersion"`
		Kind          string         `json:"kind"`
		Envelope      claudeEnvelope `json:"envelope"`
		SessionID     string         `json:"sessionId,omitempty"`
		FailureCode   string         `json:"failureCode,omitempty"`
		FailureDetail string         `json:"failureDetail,omitempty"`
		SpawnError    string         `json:"spawnError,omitempty"`
		WaitError     string         `json:"waitError,omitempty"`
		StartError    string         `json:"startError,omitempty"`
		Started       bool           `json:"started"`
		ExitCode      int            `json:"exitCode"`
		TimedOut      bool           `json:"timedOut"`
		DurationNanos int64          `json:"durationNanos"`
		ObservedAt    string         `json:"observedAt,omitempty"`
		StdoutTail    string         `json:"stdoutTail,omitempty"`
		StderrTail    string         `json:"stderrTail,omitempty"`
	}{
		SchemaVersion: 1,
		Kind:          "claude-host-raw-terminal-truth",
		Envelope:      run.envelope,
		SessionID:     run.sessionID,
		FailureCode:   run.failureCode,
		FailureDetail: run.failureDetail,
		SpawnError:    errorText(run.spawnErr),
		WaitError:     errorText(run.waitErr),
		StartError:    errorText(run.startCallbackErr),
		Started:       run.started,
		ExitCode:      run.exitCode,
		TimedOut:      run.timedOut,
		DurationNanos: int64(run.duration),
		ObservedAt:    run.observedAt,
		StdoutTail:    run.stdoutTail,
		StderrTail:    run.stderrTail,
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("encode host-owned Claude raw terminal truth: %w", err)
	}
	return data, nil
}

func bindClaudeRawResultArtifact(run claudeRun, artifactPath string, data []byte) (claudeRun, error) {
	ref, sha, size, err := hostCacheArtifactIdentity(artifactPath, data)
	if err != nil {
		return claudeRun{}, err
	}
	if run.rawResultRef != "" || run.rawResultSHA256 != "" || run.rawResultBytes != 0 {
		if run.rawResultRef != ref || !strings.EqualFold(run.rawResultSHA256, sha) || run.rawResultBytes != size {
			return claudeRun{}, fmt.Errorf("Claude raw result already carries a different host-owned artifact identity")
		}
		return run, nil
	}
	run.rawResultRef = ref
	run.rawResultSHA256 = sha
	run.rawResultBytes = size
	return run, nil
}

func validateClaudeRawResultArtifact(caseRoot string, run claudeRun) error {
	if strings.TrimSpace(run.rawResultRef) == "" || len(strings.TrimSpace(run.rawResultSHA256)) != 64 || run.rawResultBytes < 1 {
		return fmt.Errorf("Claude raw result requires an exact host-owned artifact reference, sha256, and byte count")
	}
	caseSHA, err := hostCacheCaseSHA(caseRoot)
	if err != nil {
		return err
	}
	rel, err := hostCacheArtifactRel(run.rawResultRef)
	if err != nil {
		return err
	}
	parts := strings.Split(rel, "/")
	if len(parts) < 7 || parts[0] != "rekit" || parts[1] != "session-host" || parts[3] != "cases" || !strings.EqualFold(parts[4], caseSHA) {
		return fmt.Errorf("Claude raw result artifact does not belong to the attached case host-cache namespace")
	}
	data, err := readHostCacheArtifact(rel)
	if err != nil {
		return err
	}
	if int64(len(data)) != run.rawResultBytes || !strings.EqualFold(bytesSHA256(data), run.rawResultSHA256) {
		return fmt.Errorf("Claude raw result artifact no longer matches its exact sha256 and byte count")
	}
	return nil
}

func hostCacheArtifactIdentity(artifactPath string, data []byte) (string, string, int64, error) {
	if len(data) < 1 || len(data) > maxClaudeRawArtifactBytes {
		return "", "", 0, fmt.Errorf("host-owned Claude raw result artifact must contain 1 through %d bytes", maxClaudeRawArtifactBytes)
	}
	cacheRoot, err := hostCacheRoot()
	if err != nil {
		return "", "", 0, err
	}
	artifactPath, err = filepath.Abs(artifactPath)
	if err != nil {
		return "", "", 0, err
	}
	rel, err := filepath.Rel(cacheRoot, artifactPath)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", "", 0, fmt.Errorf("host-owned Claude raw result artifact is outside the user cache root")
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	if pathpkg.Clean(rel) != rel {
		return "", "", 0, fmt.Errorf("host-owned Claude raw result artifact reference is not canonical")
	}
	return hostCacheArtifactPrefix + rel, bytesSHA256(data), int64(len(data)), nil
}

func hostCacheArtifactRel(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if !strings.HasPrefix(ref, hostCacheArtifactPrefix) {
		return "", fmt.Errorf("Claude raw result reference is not a host-cache artifact")
	}
	rel := strings.TrimPrefix(ref, hostCacheArtifactPrefix)
	if rel == "" || strings.ContainsAny(rel, "\\\r\n") || pathpkg.IsAbs(rel) || pathpkg.Clean(rel) != rel || rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("Claude raw result host-cache reference is invalid")
	}
	return rel, nil
}

func readHostCacheArtifact(rel string) ([]byte, error) {
	cacheRoot, err := hostCacheRoot()
	if err != nil {
		return nil, err
	}
	path, err := rekitfs.SafeJoin(cacheRoot, filepath.FromSlash(rel))
	if err != nil {
		return nil, err
	}
	return rekitfs.ReadStableRegularFileAnchored(cacheRoot, path, "host-owned Claude raw result artifact", maxClaudeRawArtifactBytes)
}

func claudeRawResultRoot(caseRoot string) (string, error) {
	cacheRoot, err := hostCacheRoot()
	if err != nil {
		return "", err
	}
	caseSHA, err := hostCacheCaseSHA(caseRoot)
	if err != nil {
		return "", err
	}
	relRoot := filepath.Join("rekit", "session-host", "v3", "cases", caseSHA, "raw-results")
	root := filepath.Join(cacheRoot, relRoot)
	if pathsOverlap(caseRoot, root) {
		return "", fmt.Errorf("host-owned Claude raw result root must be outside the attached case: %s", caseRoot)
	}
	marker := []byte("rekit Claude raw result root v3\ncaseSha256=" + caseSHA + "\n")
	if _, err := rekitfs.WriteExclusiveRegularFileAnchored(cacheRoot, filepath.Join(relRoot, ".binding"), "Claude raw result root binding", marker); err != nil {
		return "", err
	}
	return root, nil
}

func hostCacheRoot() (string, error) {
	root, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve host-owned Claude cache root: %w", err)
	}
	return filepath.Abs(root)
}

func hostCacheCaseSHA(caseRoot string) (string, error) {
	identity, err := filepath.Abs(caseRoot)
	if err != nil {
		return "", err
	}
	identity = filepath.Clean(identity)
	if runtime.GOOS == "windows" {
		identity = strings.ToLower(identity)
	}
	return bytesSHA256([]byte(identity)), nil
}
