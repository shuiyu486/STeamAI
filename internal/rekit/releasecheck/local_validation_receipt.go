package releasecheck

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

const (
	localValidationReceiptSchemaVersion = 1
	localValidationReceiptKind          = "rekit-local-validation-receipt"
	localValidationReceiptMaxBytes      = 2 * 1024 * 1024
	localValidationArtifactMaxBytes     = 32 * 1024 * 1024
)

type LocalValidationReceiptStep struct {
	Index    int    `json:"index"`
	Command  string `json:"command"`
	Status   string `json:"status"`
	ExitCode int    `json:"exitCode"`
	Attempts int    `json:"attempts"`
}

type LocalValidationReceiptArtifact struct {
	Path   string `json:"path"`
	State  string `json:"state"`
	Mode   string `json:"mode,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
	Bytes  int64  `json:"bytes,omitempty"`
}

type LocalValidationReceipt struct {
	SchemaVersion     int                              `json:"schemaVersion"`
	Kind              string                           `json:"kind"`
	BatchID           string                           `json:"batchId"`
	BaselineHead      string                           `json:"baselineHead"`
	GateProfile       string                           `json:"gateProfile"`
	StepCount         int                              `json:"stepCount"`
	Passed            int                              `json:"passed"`
	Failed            int                              `json:"failed"`
	Skipped           int                              `json:"skipped"`
	ReleaseCheckReady bool                             `json:"releaseCheckReady"`
	CreatedAt         string                           `json:"createdAt"`
	Steps             []LocalValidationReceiptStep     `json:"steps"`
	Artifacts         []LocalValidationReceiptArtifact `json:"artifacts"`
	Boundary          []string                         `json:"boundary"`
}

type LocalValidationReceiptInput struct {
	BatchID           string
	GateProfile       string
	Passed            int
	Failed            int
	Skipped           int
	ReleaseCheckReady bool
	Steps             []LocalValidationReceiptStep
	Snapshot          LocalValidationSnapshot
}

type LocalValidationSnapshot struct {
	BaselineHead string
	Artifacts    []LocalValidationReceiptArtifact
}

type LocalValidationReceiptInspection struct {
	Present       bool                    `json:"present"`
	Ready         bool                    `json:"ready"`
	State         string                  `json:"state"`
	Path          string                  `json:"path,omitempty"`
	SHA256        string                  `json:"sha256,omitempty"`
	ValidatedHead string                  `json:"validatedHead,omitempty"`
	Receipt       *LocalValidationReceipt `json:"receipt,omitempty"`
	Evidence      []string                `json:"evidence,omitempty"`
	Boundary      []string                `json:"boundary"`
	Warnings      []string                `json:"warnings,omitempty"`
}

func CaptureLocalValidationSnapshot(repo string) (LocalValidationSnapshot, error) {
	baseline, err := localValidationGit(repo, "rev-parse", "HEAD")
	if err != nil || !validLocalValidationCommit(baseline) {
		return LocalValidationSnapshot{}, fmt.Errorf("read local validation baseline HEAD: %w", err)
	}
	artifacts, err := localValidationWorkingArtifacts(repo)
	if err != nil {
		return LocalValidationSnapshot{}, err
	}
	paths := localValidationArtifactPaths(artifacts)
	if !slices.Contains(paths, "docs/batch-plan.md") || !slices.Contains(paths, "CHANGELOG.md") || !releaseHandoffHasImplementationPath(paths) {
		return LocalValidationSnapshot{}, fmt.Errorf("local validation receipt requires batch plan, changelog, and product implementation changes")
	}
	return LocalValidationSnapshot{BaselineHead: baseline, Artifacts: artifacts}, nil
}

func PublishLocalValidationReceipt(repo string, input LocalValidationReceiptInput) (LocalValidationReceiptInspection, error) {
	inspection := localValidationReceiptInspectionBase()
	batchID := strings.TrimSpace(input.BatchID)
	if batchID == "" {
		return inspection, fmt.Errorf("local validation receipt requires the latest batch id")
	}
	if strings.TrimSpace(input.GateProfile) == "" || !input.ReleaseCheckReady || input.Failed != 0 || input.Skipped != 0 || input.Passed != len(input.Steps) || len(input.Steps) == 0 {
		return inspection, fmt.Errorf("local validation receipt requires a complete successful release run")
	}
	for index, step := range input.Steps {
		if step.Index != index+1 || strings.TrimSpace(step.Command) == "" || step.Status != "passed" || step.ExitCode != 0 || step.Attempts < 1 {
			return inspection, fmt.Errorf("local validation receipt step %d is not a canonical success", index+1)
		}
	}
	baseline := strings.TrimSpace(input.Snapshot.BaselineHead)
	if !validLocalValidationCommit(baseline) || len(input.Snapshot.Artifacts) == 0 {
		return inspection, fmt.Errorf("local validation receipt requires a pre-run artifact snapshot")
	}
	artifacts := append([]LocalValidationReceiptArtifact{}, input.Snapshot.Artifacts...)
	current, err := CaptureLocalValidationSnapshot(repo)
	if err != nil {
		return inspection, err
	}
	if !strings.EqualFold(current.BaselineHead, baseline) || !slices.Equal(current.Artifacts, artifacts) {
		return inspection, fmt.Errorf("local validation artifact snapshot changed while release-run was executing")
	}
	receipt := LocalValidationReceipt{
		SchemaVersion:     localValidationReceiptSchemaVersion,
		Kind:              localValidationReceiptKind,
		BatchID:           batchID,
		BaselineHead:      baseline,
		GateProfile:       strings.TrimSpace(input.GateProfile),
		StepCount:         len(input.Steps),
		Passed:            input.Passed,
		Failed:            input.Failed,
		Skipped:           input.Skipped,
		ReleaseCheckReady: true,
		CreatedAt:         time.Now().UTC().Format(time.RFC3339Nano),
		Steps:             append([]LocalValidationReceiptStep{}, input.Steps...),
		Artifacts:         artifacts,
		Boundary: []string{
			"receipt binds the successful local gate profile to the exact validated working-tree artifacts",
			"receipt is repo-local git metadata and does not claim remote CI green",
			"post-push acceptance requires one direct implementation commit whose exact artifact set and bytes match this receipt",
		},
	}
	data, err := canonicalLocalValidationReceipt(receipt)
	if err != nil {
		return inspection, err
	}
	path, err := localValidationReceiptPath(repo)
	if err != nil {
		return inspection, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return inspection, err
	}
	if info, err := os.Lstat(path); err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		return inspection, fmt.Errorf("existing local validation receipt must be a regular non-symlink file")
	} else if err != nil && !os.IsNotExist(err) {
		return inspection, err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".local-validation-*.tmp")
	if err != nil {
		return inspection, err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return inspection, err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return inspection, err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return inspection, err
	}
	if err := temp.Close(); err != nil {
		return inspection, err
	}
	if err := replaceLocalValidationReceipt(tempPath, path); err != nil {
		return inspection, err
	}
	inspection.Present = true
	inspection.State = "recorded-for-implementation-commit"
	inspection.Path = localValidationReceiptDisplayPath(repo, path)
	inspection.SHA256 = localValidationHash(data)
	inspection.Receipt = &receipt
	inspection.Evidence = []string{"successful local release-run receipt recorded", "exact pre-commit artifact snapshot recorded"}
	return inspection, nil
}

func InspectLocalValidationReceipt(repo string, latest ReleaseHandoffLatestBatch) LocalValidationReceiptInspection {
	inspection := localValidationReceiptInspectionBase()
	path, err := localValidationReceiptPath(repo)
	if err != nil {
		inspection.State = "git-path-unavailable"
		inspection.Warnings = append(inspection.Warnings, err.Error())
		return inspection
	}
	inspection.Path = localValidationReceiptDisplayPath(repo, path)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		inspection.State = "not-recorded"
		return inspection
	}
	inspection.Present = true
	if err != nil {
		inspection.State = "unreadable"
		inspection.Warnings = append(inspection.Warnings, err.Error())
		return inspection
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > localValidationReceiptMaxBytes {
		inspection.State = "invalid-artifact"
		inspection.Warnings = append(inspection.Warnings, "local validation receipt must be a bounded regular non-symlink file")
		return inspection
	}
	data, err := os.ReadFile(path)
	if err != nil {
		inspection.State = "unreadable"
		inspection.Warnings = append(inspection.Warnings, err.Error())
		return inspection
	}
	inspection.SHA256 = localValidationHash(data)
	var receipt LocalValidationReceipt
	if err := strictCanonicalLocalValidationReceipt(data, &receipt); err != nil {
		inspection.State = "invalid-contract"
		inspection.Warnings = append(inspection.Warnings, err.Error())
		return inspection
	}
	inspection.Receipt = &receipt
	if err := validateLocalValidationReceiptContract(repo, receipt, latest); err != nil {
		inspection.State = "stale-or-invalid"
		inspection.Warnings = append(inspection.Warnings, err.Error())
		return inspection
	}
	head, err := localValidationGit(repo, "rev-parse", "HEAD")
	if err != nil || !validLocalValidationCommit(head) {
		inspection.State = "git-state-unavailable"
		inspection.Warnings = append(inspection.Warnings, fmt.Sprintf("read current HEAD: %v", err))
		return inspection
	}
	parents, err := localValidationGit(repo, "rev-list", "--parents", "-n", "1", head)
	if err != nil {
		inspection.State = "git-state-unavailable"
		inspection.Warnings = append(inspection.Warnings, err.Error())
		return inspection
	}
	parentFields := strings.Fields(parents)
	if len(parentFields) != 2 || !strings.EqualFold(parentFields[1], receipt.BaselineHead) {
		inspection.State = "non-direct-implementation-commit"
		inspection.Warnings = append(inspection.Warnings, "current HEAD is not the one direct implementation commit after the validated baseline")
		return inspection
	}
	changedText, err := localValidationGitRaw(repo, "diff", "--name-only", "-z", receipt.BaselineHead, head, "--")
	if err != nil {
		inspection.State = "git-state-unavailable"
		inspection.Warnings = append(inspection.Warnings, err.Error())
		return inspection
	}
	changed := splitLocalValidationNUL(changedText)
	sort.Strings(changed)
	if !slices.Equal(changed, localValidationArtifactPaths(receipt.Artifacts)) {
		inspection.State = "artifact-set-mismatch"
		inspection.Warnings = append(inspection.Warnings, "implementation commit changed paths differ from the validated artifact set")
		return inspection
	}
	for _, artifact := range receipt.Artifacts {
		if err := validateLocalValidationCommittedArtifact(repo, receipt.BaselineHead, head, artifact); err != nil {
			inspection.State = "artifact-content-mismatch"
			inspection.Warnings = append(inspection.Warnings, err.Error())
			return inspection
		}
	}
	finalHead, err := localValidationGit(repo, "rev-parse", "HEAD")
	if err != nil || !strings.EqualFold(finalHead, head) {
		inspection.State = "stale-repository-snapshot"
		inspection.Warnings = append(inspection.Warnings, "HEAD changed while validating the local validation receipt")
		return inspection
	}
	inspection.Ready = true
	inspection.State = "validated-implementation-commit"
	inspection.ValidatedHead = head
	inspection.Evidence = []string{
		"machine-readable local release-run receipt validated",
		"validated implementation commit exactly matches the pre-commit artifact snapshot",
		"release-check step and complete local gate profile are bound by the receipt",
	}
	return inspection
}

func localValidationReceiptInspectionBase() LocalValidationReceiptInspection {
	return LocalValidationReceiptInspection{
		State: "unavailable",
		Boundary: []string{
			"local validation receipt is repo-local git metadata and never claims remote CI green",
			"missing, malformed, stale, multi-commit, path-drifted, or byte-drifted receipts fail closed",
		},
	}
}

func validateLocalValidationReceiptContract(repo string, receipt LocalValidationReceipt, latest ReleaseHandoffLatestBatch) error {
	if receipt.SchemaVersion != localValidationReceiptSchemaVersion || receipt.Kind != localValidationReceiptKind || receipt.BatchID == "" || receipt.BatchID != latest.BatchID || !validLocalValidationCommit(receipt.BaselineHead) || receipt.GateProfile == "" || !receipt.ReleaseCheckReady || receipt.StepCount < 1 || receipt.StepCount != len(receipt.Steps) || receipt.Passed != receipt.StepCount || receipt.Failed != 0 || receipt.Skipped != 0 || len(receipt.Artifacts) == 0 {
		return fmt.Errorf("local validation receipt identity or success summary is invalid")
	}
	created, err := time.Parse(time.RFC3339Nano, receipt.CreatedAt)
	if err != nil || created.Format(time.RFC3339Nano) != receipt.CreatedAt {
		return fmt.Errorf("local validation receipt createdAt is not canonical RFC3339Nano")
	}
	profile, err := localValidationExpectedProfile(repo, receipt.GateProfile)
	if err != nil || len(profile.Steps) != len(receipt.Steps) {
		return fmt.Errorf("local validation receipt gate profile is unavailable or incomplete")
	}
	for index, step := range receipt.Steps {
		expected := profile.Steps[index]
		if step.Index != index+1 || step.Command != expected.Command || !expected.Required || !expected.Resolved || step.Status != "passed" || step.ExitCode != 0 || step.Attempts < 1 {
			return fmt.Errorf("local validation receipt step %d is invalid", index+1)
		}
	}
	paths := localValidationArtifactPaths(receipt.Artifacts)
	if len(paths) != len(receipt.Artifacts) || !sort.StringsAreSorted(paths) || !slices.Contains(paths, "docs/batch-plan.md") || !slices.Contains(paths, "CHANGELOG.md") || !releaseHandoffHasImplementationPath(paths) {
		return fmt.Errorf("local validation receipt artifact set is invalid")
	}
	for _, artifact := range receipt.Artifacts {
		if !validLocalValidationPath(artifact.Path) || (artifact.State != "present" && artifact.State != "deleted") {
			return fmt.Errorf("local validation receipt artifact is invalid: %s", artifact.Path)
		}
		if artifact.State == "present" && (artifact.Mode != "100644" && artifact.Mode != "100755" || !validLocalValidationSHA(artifact.SHA256) || artifact.Bytes < 0) {
			return fmt.Errorf("local validation receipt artifact content is invalid: %s", artifact.Path)
		}
		if artifact.State == "deleted" && (artifact.Mode != "" || artifact.SHA256 != "" || artifact.Bytes != 0) {
			return fmt.Errorf("deleted local validation artifact carries content: %s", artifact.Path)
		}
	}
	return nil
}

func localValidationExpectedProfile(repo, name string) (GateProfile, error) {
	cat, err := loadCatalog(repo)
	if err != nil {
		return GateProfile{}, err
	}
	profile := gateProfile(catalogGateSteps(repo, cat.RecommendedMinimum))
	if profile.Name != strings.TrimSpace(name) || !profile.Ready {
		return GateProfile{}, fmt.Errorf("unexpected local validation gate profile")
	}
	return profile, nil
}

func localValidationWorkingArtifacts(repo string) ([]LocalValidationReceiptArtifact, error) {
	changedData, err := localValidationGitRaw(repo, "diff", "--name-only", "-z", "HEAD", "--")
	if err != nil {
		return nil, err
	}
	untrackedData, err := localValidationGitRaw(repo, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	paths := append(splitLocalValidationNUL(changedData), splitLocalValidationNUL(untrackedData)...)
	sort.Strings(paths)
	paths = slices.Compact(paths)
	artifacts := make([]LocalValidationReceiptArtifact, 0, len(paths))
	for _, rel := range paths {
		if !validLocalValidationPath(rel) {
			return nil, fmt.Errorf("invalid local validation artifact path: %s", rel)
		}
		path := filepath.Join(repo, filepath.FromSlash(rel))
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			artifacts = append(artifacts, LocalValidationReceiptArtifact{Path: rel, State: "deleted"})
			continue
		}
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > localValidationArtifactMaxBytes {
			return nil, fmt.Errorf("local validation artifact must be a bounded regular non-symlink file: %s", rel)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, LocalValidationReceiptArtifact{Path: rel, State: "present", Mode: localValidationFileMode(info.Mode()), SHA256: localValidationHash(data), Bytes: int64(len(data))})
	}
	return artifacts, nil
}

func validateLocalValidationCommittedArtifact(repo, baseline, head string, artifact LocalValidationReceiptArtifact) error {
	status, err := localValidationGit(repo, "diff", "--name-status", baseline, head, "--", artifact.Path)
	if err != nil {
		return err
	}
	fields := strings.Fields(status)
	if len(fields) < 2 || fields[len(fields)-1] != artifact.Path {
		return fmt.Errorf("validated artifact status is unavailable: %s", artifact.Path)
	}
	if artifact.State == "deleted" {
		if fields[0] != "D" {
			return fmt.Errorf("validated artifact deletion status differs: %s", artifact.Path)
		}
		return nil
	}
	if fields[0] != "A" && fields[0] != "M" {
		return fmt.Errorf("validated artifact status differs: %s", artifact.Path)
	}
	modeLine, err := localValidationGit(repo, "ls-tree", head, "--", artifact.Path)
	if err != nil {
		return err
	}
	modeFields := strings.Fields(modeLine)
	if len(modeFields) < 4 || modeFields[0] != artifact.Mode || modeFields[len(modeFields)-1] != artifact.Path {
		return fmt.Errorf("validated artifact mode differs: %s", artifact.Path)
	}
	data, err := localValidationGitRaw(repo, "show", head+":"+artifact.Path)
	if err != nil {
		return fmt.Errorf("read validated artifact from implementation commit %s: %w", artifact.Path, err)
	}
	if int64(len(data)) != artifact.Bytes || localValidationHash([]byte(data)) != artifact.SHA256 {
		return fmt.Errorf("validated artifact bytes differ in implementation commit: %s", artifact.Path)
	}
	return nil
}

func localValidationFileMode(mode os.FileMode) string {
	if mode.Perm()&0o111 != 0 {
		return "100755"
	}
	return "100644"
}

func localValidationReceiptPath(repo string) (string, error) {
	value, err := localValidationGit(repo, "rev-parse", "--git-path", "rekit/local-validation-v1.json")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(repo, filepath.FromSlash(value))
	}
	return filepath.Clean(value), nil
}

func localValidationReceiptDisplayPath(repo, path string) string {
	rel, err := filepath.Rel(repo, path)
	if err == nil && rel != "" && rel != "." && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

func localValidationGit(repo string, args ...string) (string, error) {
	value, err := localValidationGitRaw(repo, args...)
	return strings.TrimSpace(value), err
}

func localValidationGitRaw(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(output), nil
}

func splitLocalValidationNUL(value string) []string {
	items := []string{}
	for item := range strings.SplitSeq(value, "\x00") {
		item = filepath.ToSlash(strings.TrimSpace(item))
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

func localValidationArtifactPaths(artifacts []LocalValidationReceiptArtifact) []string {
	paths := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		paths = append(paths, artifact.Path)
	}
	return paths
}

func validLocalValidationPath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || filepath.IsAbs(value) || strings.ContainsRune(value, '\x00') || strings.Contains(value, "\\") {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	return clean == value && clean != "." && clean != ".." && !strings.HasPrefix(clean, "../") && !strings.HasPrefix(clean, ".git/")
}

func validLocalValidationCommit(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validLocalValidationSHA(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func localValidationHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func canonicalLocalValidationReceipt(receipt LocalValidationReceipt) ([]byte, error) {
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func strictCanonicalLocalValidationReceipt(data []byte, receipt *LocalValidationReceipt) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(receipt); err != nil {
		return err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("local validation receipt must contain exactly one JSON object")
	}
	canonical, err := canonicalLocalValidationReceipt(*receipt)
	if err != nil {
		return err
	}
	if !bytes.Equal(data, canonical) {
		return fmt.Errorf("local validation receipt is not canonical JSON")
	}
	return nil
}
