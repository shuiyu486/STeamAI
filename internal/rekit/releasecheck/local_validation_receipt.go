package releasecheck

import (
	"bytes"
	"context"
	"crypto/sha1"
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

	"github.com/shuiyu486/re-context-kits/internal/rekit/processguard"
)

const (
	localValidationReceiptSchemaVersion       = 3
	localValidationReceiptLegacySchemaVersion = 2
	localValidationReceiptKind                = "rekit-local-validation-receipt"
	localValidationReceiptMaxBytes            = 2 * 1024 * 1024
	localValidationArtifactMaxBytes           = 32 * 1024 * 1024

	LocalValidationReceiptSubjectNumberedBatch = "numbered-batch"
	LocalValidationReceiptSubjectActiveRoute   = "active-route"
)

type LocalValidationReceiptStep struct {
	Index    int    `json:"index"`
	Command  string `json:"command"`
	Status   string `json:"status"`
	ExitCode int    `json:"exitCode"`
	Attempts int    `json:"attempts"`
}

type LocalValidationReceiptArtifact struct {
	Path    string `json:"path"`
	State   string `json:"state"`
	Mode    string `json:"mode,omitempty"`
	SHA256  string `json:"sha256,omitempty"`
	Bytes   int64  `json:"bytes,omitempty"`
	BlobOID string `json:"blobOid,omitempty"`
}

type LocalValidationReceiptSubject struct {
	Kind           string `json:"kind"`
	BatchID        string `json:"batchId,omitempty"`
	Route          string `json:"route,omitempty"`
	CurrentBatch   string `json:"currentBatch,omitempty"`
	State          string `json:"state,omitempty"`
	ExclusiveClaim string `json:"exclusiveClaim,omitempty"`
	NextBatch      string `json:"nextBatch,omitempty"`
}

type LocalValidationReceipt struct {
	SchemaVersion     int                              `json:"schemaVersion"`
	Kind              string                           `json:"kind"`
	BatchID           string                           `json:"batchId,omitempty"`
	Subject           *LocalValidationReceiptSubject   `json:"subject,omitempty"`
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
	Subject           LocalValidationReceiptSubject
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

func LocalValidationReceiptSubjectFor(handoff ReleaseHandoff) (LocalValidationReceiptSubject, bool) {
	if route := handoff.ActiveRoute; route.Present {
		if !route.Ready || !route.ProjectionConsistent || route.State != "completed" {
			return LocalValidationReceiptSubject{}, false
		}
		subject, err := normalizeLocalValidationReceiptSubject(LocalValidationReceiptSubject{
			Kind:           LocalValidationReceiptSubjectActiveRoute,
			Route:          route.Route,
			CurrentBatch:   route.CurrentBatch,
			State:          route.State,
			ExclusiveClaim: route.ExclusiveClaim,
			NextBatch:      route.NextBatch,
		})
		return subject, err == nil
	}
	latest := handoff.LatestBatch
	if !latest.Handoff.Completed || latestBatchIDNumber(latest.BatchID) < 817 || latest.Handoff.ReleaseInspectionCadence.State != "implementation-pending" {
		return LocalValidationReceiptSubject{}, false
	}
	subject, err := normalizeLocalValidationReceiptSubject(LocalValidationReceiptSubject{
		Kind:    LocalValidationReceiptSubjectNumberedBatch,
		BatchID: latest.BatchID,
	})
	return subject, err == nil
}

func normalizeLocalValidationReceiptSubject(subject LocalValidationReceiptSubject) (LocalValidationReceiptSubject, error) {
	subject.Kind = strings.TrimSpace(subject.Kind)
	subject.BatchID = strings.TrimSpace(subject.BatchID)
	subject.Route = strings.TrimSpace(subject.Route)
	subject.CurrentBatch = strings.TrimSpace(subject.CurrentBatch)
	subject.State = strings.ToLower(strings.TrimSpace(subject.State))
	subject.ExclusiveClaim = strings.TrimSpace(subject.ExclusiveClaim)
	subject.NextBatch = strings.TrimSpace(subject.NextBatch)
	switch subject.Kind {
	case LocalValidationReceiptSubjectNumberedBatch:
		if subject.BatchID == "" || subject.Route != "" || subject.CurrentBatch != "" || subject.State != "" || subject.ExclusiveClaim != "" || subject.NextBatch != "" {
			return LocalValidationReceiptSubject{}, fmt.Errorf("numbered-batch local validation receipt subject is invalid")
		}
	case LocalValidationReceiptSubjectActiveRoute:
		if subject.BatchID != "" || subject.Route == "" || subject.CurrentBatch == "" || subject.State != "completed" || subject.ExclusiveClaim == "" || subject.NextBatch == "" {
			return LocalValidationReceiptSubject{}, fmt.Errorf("active-route local validation receipt subject is invalid")
		}
	default:
		return LocalValidationReceiptSubject{}, fmt.Errorf("local validation receipt requires a typed subject")
	}
	return subject, nil
}

func PublishLocalValidationReceipt(repo string, input LocalValidationReceiptInput) (LocalValidationReceiptInspection, error) {
	inspection := localValidationReceiptInspectionBase()
	subject, err := normalizeLocalValidationReceiptSubject(input.Subject)
	if err != nil {
		return inspection, err
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
		Subject:           &subject,
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
			"receipt binds the successful local gate profile to the exact validated working-tree artifacts and Git blob identities",
			"receipt is repo-local git metadata and does not claim remote CI green",
			"post-push acceptance requires one direct implementation commit whose exact artifact set, committed blobs, and raw or Git line-ending checkout-equivalent bytes match this receipt",
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

func InspectLocalValidationReceipt(repo string, subject LocalValidationReceiptSubject) LocalValidationReceiptInspection {
	inspection := localValidationReceiptInspectionBase()
	expectedSubject, subjectErr := normalizeLocalValidationReceiptSubject(subject)
	if subjectErr != nil {
		inspection.State = "invalid-subject"
		inspection.Warnings = append(inspection.Warnings, subjectErr.Error())
		return inspection
	}
	path, err := localValidationReceiptPath(repo)
	if err != nil {
		inspection.State = "git-path-unavailable"
		inspection.Warnings = append(inspection.Warnings, err.Error())
		return inspection
	}
	legacyPathSelected := false
	inspection.Path = localValidationReceiptDisplayPath(repo, path)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		legacyPath, legacyErr := localValidationLegacyReceiptPath(repo)
		if legacyErr != nil {
			inspection.State = "git-path-unavailable"
			inspection.Warnings = append(inspection.Warnings, legacyErr.Error())
			return inspection
		}
		legacyInfo, legacyErr := os.Lstat(legacyPath)
		if os.IsNotExist(legacyErr) {
			inspection.State = "not-recorded"
			return inspection
		}
		path = legacyPath
		info = legacyInfo
		err = legacyErr
		legacyPathSelected = true
		inspection.Path = localValidationReceiptDisplayPath(repo, path)
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
	receipt, err := decodeLocalValidationReceipt(data)
	if err != nil {
		inspection.State = "invalid-contract"
		inspection.Warnings = append(inspection.Warnings, err.Error())
		return inspection
	}
	if legacyPathSelected && receipt.SchemaVersion != localValidationReceiptLegacySchemaVersion || !legacyPathSelected && receipt.SchemaVersion != localValidationReceiptSchemaVersion {
		inspection.State = "invalid-contract"
		inspection.Warnings = append(inspection.Warnings, "local validation receipt schema does not match its versioned Git metadata path")
		return inspection
	}
	inspection.Receipt = &receipt
	if err := validateLocalValidationReceiptContract(repo, receipt, expectedSubject); err != nil {
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
	if strings.EqualFold(head, receipt.BaselineHead) {
		current, err := CaptureLocalValidationSnapshot(repo)
		if err != nil || !strings.EqualFold(current.BaselineHead, receipt.BaselineHead) || !slices.Equal(current.Artifacts, receipt.Artifacts) {
			inspection.State = "artifact-content-mismatch"
			inspection.Warnings = append(inspection.Warnings, "validated working-tree artifact snapshot changed before the implementation commit")
			return inspection
		}
		inspection.State = "recorded-for-implementation-commit"
		inspection.Evidence = append([]string{
			"machine-readable local release-run receipt recorded for the exact current working-tree snapshot",
			"create one direct implementation commit from the validated artifact set before post-push inspection",
		}, localValidationReceiptStepEvidence(receipt.Steps)...)
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
	if err := validateLocalValidationCommittedArtifacts(repo, receipt.BaselineHead, head, receipt.Artifacts); err != nil {
		inspection.State = "artifact-content-mismatch"
		inspection.Warnings = append(inspection.Warnings, err.Error())
		return inspection
	}
	checkoutEligible, err := localValidationLineEndingCheckoutEligibility(repo, head, receipt.Artifacts)
	if err != nil {
		inspection.State = "git-state-unavailable"
		inspection.Warnings = append(inspection.Warnings, err.Error())
		return inspection
	}
	worktreeBefore, matchesReceipt, err := localValidationReceiptWorktreeArtifacts(repo, receipt.Artifacts, checkoutEligible)
	if err != nil || !matchesReceipt {
		inspection.State = "artifact-content-mismatch"
		inspection.Warnings = append(inspection.Warnings, "validated working-tree artifact bytes changed after the release run")
		return inspection
	}
	finalHead, err := localValidationGit(repo, "rev-parse", "HEAD")
	if err != nil || !strings.EqualFold(finalHead, head) {
		inspection.State = "stale-repository-snapshot"
		inspection.Warnings = append(inspection.Warnings, "HEAD changed while validating the local validation receipt")
		return inspection
	}
	worktreeAfter, _, err := localValidationReceiptWorktreeArtifacts(repo, receipt.Artifacts, checkoutEligible)
	if err != nil || !slices.Equal(worktreeAfter, worktreeBefore) {
		inspection.State = "stale-repository-snapshot"
		inspection.Warnings = append(inspection.Warnings, "validated working-tree artifacts changed while validating the local validation receipt")
		return inspection
	}
	inspection.Ready = true
	inspection.State = "validated-implementation-commit"
	inspection.ValidatedHead = head
	inspection.Evidence = append([]string{
		"machine-readable local release-run receipt validated",
		"validated implementation commit exactly matches the pre-commit artifact snapshot or its Git line-ending checkout",
		"release-check step and complete local gate profile are bound by the receipt",
	}, localValidationReceiptStepEvidence(receipt.Steps)...)
	return inspection
}

func localValidationReceiptStepEvidence(steps []LocalValidationReceiptStep) []string {
	labels := map[string]string{
		"go run ./cmd/rekit -- -Command release-check -Format json": "release-check -Format json recorded",
		"go run ./cmd/rekit -- -Command status":                     "status handoff recorded",
		"go run ./cmd/rekit -- -Command packs":                      "packs inventory recorded",
		"go run ./cmd/rekit -- -Command doctor":                     "doctor validation recorded",
		CanonicalGoTestCommand:                                      "go test ./... recorded",
		"go vet ./...":                                              "go vet ./... recorded",
		"git diff --check":                                          "git diff --check recorded",
	}
	evidence := make([]string, 0, len(steps))
	for _, step := range steps {
		if label := labels[step.Command]; label != "" {
			evidence = append(evidence, label)
		}
	}
	return evidence
}

func localValidationReceiptInspectionBase() LocalValidationReceiptInspection {
	return LocalValidationReceiptInspection{
		State: "unavailable",
		Boundary: []string{
			"local validation receipt is repo-local git metadata and never claims remote CI green",
			"missing, malformed, stale, multi-commit, path-drifted, or non-checkout-equivalent byte-drifted receipts fail closed",
		},
	}
}

func localValidationReceiptSubjectForReceipt(receipt LocalValidationReceipt) (LocalValidationReceiptSubject, error) {
	switch receipt.SchemaVersion {
	case localValidationReceiptLegacySchemaVersion:
		if receipt.Subject != nil {
			return LocalValidationReceiptSubject{}, fmt.Errorf("legacy local validation receipt must not carry a typed subject")
		}
		return normalizeLocalValidationReceiptSubject(LocalValidationReceiptSubject{
			Kind:    LocalValidationReceiptSubjectNumberedBatch,
			BatchID: receipt.BatchID,
		})
	case localValidationReceiptSchemaVersion:
		if receipt.BatchID != "" || receipt.Subject == nil {
			return LocalValidationReceiptSubject{}, fmt.Errorf("typed local validation receipt has an invalid subject envelope")
		}
		return normalizeLocalValidationReceiptSubject(*receipt.Subject)
	default:
		return LocalValidationReceiptSubject{}, fmt.Errorf("unsupported local validation receipt schema version: %d", receipt.SchemaVersion)
	}
}

func validateLocalValidationReceiptContract(repo string, receipt LocalValidationReceipt, expectedSubject LocalValidationReceiptSubject) error {
	if receipt.Kind != localValidationReceiptKind || !validLocalValidationCommit(receipt.BaselineHead) || receipt.GateProfile == "" || !receipt.ReleaseCheckReady || receipt.StepCount < 1 || receipt.StepCount != len(receipt.Steps) || receipt.Passed != receipt.StepCount || receipt.Failed != 0 || receipt.Skipped != 0 || len(receipt.Artifacts) == 0 {
		return fmt.Errorf("local validation receipt identity or success summary is invalid")
	}
	actualSubject, err := localValidationReceiptSubjectForReceipt(receipt)
	if err != nil || actualSubject != expectedSubject {
		return fmt.Errorf("local validation receipt subject does not match the current validation subject")
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
		if artifact.State == "present" && (artifact.Mode != "100644" && artifact.Mode != "100755" || !validLocalValidationSHA(artifact.SHA256) || artifact.Bytes < 0 || !validLocalValidationObjectID(artifact.BlobOID)) {
			return fmt.Errorf("local validation receipt artifact content is invalid: %s", artifact.Path)
		}
		if artifact.State == "deleted" && (artifact.Mode != "" || artifact.SHA256 != "" || artifact.Bytes != 0 || artifact.BlobOID != "") {
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
	profile := gateProfile(requiredGateSteps(repo, requiredCommands, cat.RecommendedMinimum))
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
		data, err := readStableLocalValidationArtifact(path, info)
		if err != nil {
			return nil, err
		}
		blobOID, err := localValidationGitInput(repo, data, "hash-object", "--path="+rel, "--stdin")
		if err != nil || !validLocalValidationObjectID(blobOID) {
			return nil, fmt.Errorf("hash local validation artifact through Git clean filters %s: %w", rel, err)
		}
		gitMode, err := localValidationExpectedGitMode(repo, rel, info.Mode())
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, LocalValidationReceiptArtifact{Path: rel, State: "present", Mode: gitMode, SHA256: localValidationHash(data), Bytes: int64(len(data)), BlobOID: blobOID})
	}
	return artifacts, nil
}

func localValidationIndexArtifacts(repo string) (map[string]LocalValidationReceiptArtifact, error) {
	data, err := localValidationGitRaw(repo, "ls-files", "--stage", "-z")
	if err != nil {
		return nil, err
	}
	artifacts := map[string]LocalValidationReceiptArtifact{}
	for _, record := range splitLocalValidationNULRaw(data) {
		metadata, path, ok := strings.Cut(record, "\t")
		fields := strings.Fields(metadata)
		path = filepath.ToSlash(path)
		if !ok || len(fields) != 3 || !validLocalValidationPath(path) || !validLocalValidationObjectID(fields[1]) {
			return nil, fmt.Errorf("local validation artifact index entry is invalid: %s", path)
		}
		if fields[2] != "0" {
			artifacts[path] = LocalValidationReceiptArtifact{Path: path, State: "present", Mode: "unmerged", BlobOID: fields[1]}
			continue
		}
		artifacts[path] = LocalValidationReceiptArtifact{Path: path, State: "present", Mode: fields[0], BlobOID: fields[1]}
	}
	return artifacts, nil
}

func localValidationLineEndingCheckoutEligibility(repo, head string, expected []LocalValidationReceiptArtifact) (map[string]bool, error) {
	paths := make([]string, 0, len(expected))
	for _, artifact := range expected {
		if artifact.State == "present" {
			paths = append(paths, artifact.Path)
		}
	}
	input := []byte{}
	for _, path := range paths {
		input = append(input, path...)
		input = append(input, 0)
	}
	output, err := localValidationGitOutput(repo, input, "check-attr", "-z", "--stdin", "--source="+head, "text", "eol")
	if err != nil {
		return nil, err
	}
	records := splitLocalValidationNULRaw(output)
	if len(records) != len(paths)*6 {
		return nil, fmt.Errorf("local validation line-ending attribute count differs: got %d want %d", len(records), len(paths)*6)
	}
	autocrlf, err := localValidationGit(repo, "config", "--get", "--default=false", "core.autocrlf")
	if err != nil {
		return nil, err
	}
	eligible := make(map[string]bool, len(paths))
	for index, path := range paths {
		record := records[index*6 : index*6+6]
		if record[0] != path || record[1] != "text" || record[3] != path || record[4] != "eol" {
			return nil, fmt.Errorf("local validation line-ending attributes are invalid: %s", path)
		}
		text, eol := record[2], record[5]
		eligible[path] = text != "unset" && eol != "lf" && (eol == "crlf" || eol == "unspecified" && strings.EqualFold(autocrlf, "true"))
	}
	return eligible, nil
}

func localValidationReceiptWorktreeArtifacts(repo string, expected []LocalValidationReceiptArtifact, checkoutEligible map[string]bool) ([]LocalValidationReceiptArtifact, bool, error) {
	indexArtifacts, err := localValidationIndexArtifacts(repo)
	if err != nil {
		return nil, false, err
	}
	artifacts := make([]LocalValidationReceiptArtifact, 0, len(expected))
	matchesReceipt := true
	for _, artifact := range expected {
		path := filepath.Join(repo, filepath.FromSlash(artifact.Path))
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			current := LocalValidationReceiptArtifact{Path: artifact.Path, State: "deleted"}
			artifacts = append(artifacts, current)
			_, indexed := indexArtifacts[artifact.Path]
			matchesReceipt = matchesReceipt && current == artifact && !indexed
			continue
		}
		if err != nil {
			return nil, false, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > localValidationArtifactMaxBytes {
			return nil, false, fmt.Errorf("local validation artifact must remain a bounded regular non-symlink file: %s", artifact.Path)
		}
		data, err := readStableLocalValidationArtifact(path, info)
		if err != nil {
			return nil, false, err
		}
		indexed, indexedOK := indexArtifacts[artifact.Path]
		current := LocalValidationReceiptArtifact{Path: artifact.Path, State: "present", Mode: indexed.Mode, SHA256: localValidationHash(data), Bytes: int64(len(data)), BlobOID: indexed.BlobOID}
		artifacts = append(artifacts, current)
		indexMatches := indexedOK && artifact.State == "present" && indexed.Mode == artifact.Mode && indexed.BlobOID == artifact.BlobOID
		if indexMatches && localValidationArtifactRawIdentityMatches(data, artifact) {
			continue
		}
		if !indexMatches || !checkoutEligible[artifact.Path] || !localValidationArtifactMatchesLineEndingCheckout(data, artifact) {
			matchesReceipt = false
		}
	}
	return artifacts, matchesReceipt, nil
}

func localValidationArtifactMatchesLineEndingCheckout(data []byte, artifact LocalValidationReceiptArtifact) bool {
	if bytes.IndexByte(data, 0) >= 0 {
		return false
	}
	blobData := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	if bytes.IndexByte(blobData, '\r') >= 0 || localValidationBlobOID(blobData, artifact.BlobOID) != artifact.BlobOID {
		return false
	}
	checkoutData := bytes.ReplaceAll(blobData, []byte("\n"), []byte("\r\n"))
	if bytes.Equal(checkoutData, blobData) || int64(len(checkoutData)) > localValidationArtifactMaxBytes {
		return false
	}
	receiptMatches := localValidationArtifactRawIdentityMatches(blobData, artifact) || localValidationArtifactRawIdentityMatches(checkoutData, artifact)
	return receiptMatches && (bytes.Equal(data, blobData) || bytes.Equal(data, checkoutData))
}

func localValidationArtifactRawIdentityMatches(data []byte, artifact LocalValidationReceiptArtifact) bool {
	return int64(len(data)) == artifact.Bytes && localValidationHash(data) == artifact.SHA256
}

func localValidationBlobOID(data []byte, expected string) string {
	header := fmt.Appendf(nil, "blob %d\x00", len(data))
	switch len(expected) {
	case sha1.Size * 2:
		hash := sha1.New()
		_, _ = hash.Write(header)
		_, _ = hash.Write(data)
		return hex.EncodeToString(hash.Sum(nil))
	case sha256.Size * 2:
		hash := sha256.New()
		_, _ = hash.Write(header)
		_, _ = hash.Write(data)
		return hex.EncodeToString(hash.Sum(nil))
	default:
		return ""
	}
}

func readStableLocalValidationArtifact(path string, before os.FileInfo) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	opened, statErr := file.Stat()
	data, readErr := io.ReadAll(io.LimitReader(file, localValidationArtifactMaxBytes+1))
	closeErr := file.Close()
	after, afterErr := os.Lstat(path)
	if statErr != nil || readErr != nil || closeErr != nil || afterErr != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) || after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() || !os.SameFile(opened, after) || int64(len(data)) != opened.Size() || int64(len(data)) > localValidationArtifactMaxBytes {
		return nil, fmt.Errorf("local validation artifact changed while reading: %s", path)
	}
	return data, nil
}

func localValidationExpectedGitMode(repo, path string, mode os.FileMode) (string, error) {
	indexData, err := localValidationGitRaw(repo, "ls-files", "--stage", "-z", "--", path)
	if err != nil {
		return "", err
	}
	records := splitLocalValidationNULRaw(indexData)
	if len(records) == 0 {
		return localValidationFileMode(mode), nil
	}
	if len(records) != 1 {
		return "", fmt.Errorf("local validation artifact index entry is ambiguous: %s", path)
	}
	metadata, recordPath, ok := strings.Cut(records[0], "\t")
	fields := strings.Fields(metadata)
	if !ok || recordPath != path || len(fields) != 3 || (fields[0] != "100644" && fields[0] != "100755") || fields[2] != "0" {
		return "", fmt.Errorf("local validation artifact index entry is invalid: %s", path)
	}
	return fields[0], nil
}

func validateLocalValidationCommittedArtifacts(repo, baseline, head string, artifacts []LocalValidationReceiptArtifact) error {
	baselineTree, err := localValidationTreeArtifacts(repo, baseline, artifacts)
	if err != nil {
		return err
	}
	headTree, err := localValidationTreeArtifacts(repo, head, artifacts)
	if err != nil {
		return err
	}
	for _, artifact := range artifacts {
		before, existedBefore := baselineTree[artifact.Path]
		after, existsAfter := headTree[artifact.Path]
		if artifact.State == "deleted" {
			if !existedBefore || existsAfter {
				return fmt.Errorf("validated artifact deletion status differs: %s", artifact.Path)
			}
			continue
		}
		if !existsAfter || after.Mode != artifact.Mode {
			return fmt.Errorf("validated artifact mode differs: %s", artifact.Path)
		}
		if !validLocalValidationObjectID(artifact.BlobOID) || after.BlobOID != artifact.BlobOID {
			return fmt.Errorf("validated artifact blob differs in implementation commit: %s", artifact.Path)
		}
		if existedBefore && (before.Mode != "100644" && before.Mode != "100755" || before == after) {
			return fmt.Errorf("validated artifact status differs: %s", artifact.Path)
		}
	}
	return nil
}

func localValidationTreeArtifacts(repo, revision string, expected []LocalValidationReceiptArtifact) (map[string]LocalValidationReceiptArtifact, error) {
	data, err := localValidationGitRaw(repo, "ls-tree", "-r", "-z", revision)
	if err != nil {
		return nil, err
	}
	wanted := make(map[string]struct{}, len(expected))
	for _, artifact := range expected {
		wanted[artifact.Path] = struct{}{}
	}
	artifacts := map[string]LocalValidationReceiptArtifact{}
	for _, record := range splitLocalValidationNULRaw(data) {
		metadata, path, ok := strings.Cut(record, "\t")
		path = filepath.ToSlash(path)
		if _, relevant := wanted[path]; !relevant {
			continue
		}
		fields := strings.Fields(metadata)
		if !ok || len(fields) != 3 || !validLocalValidationPath(path) || (fields[1] != "blob" && fields[1] != "commit") || !validLocalValidationObjectID(fields[2]) {
			return nil, fmt.Errorf("local validation tree entry is invalid: %s", path)
		}
		artifacts[path] = LocalValidationReceiptArtifact{Path: path, State: "present", Mode: fields[0], BlobOID: fields[2]}
	}
	return artifacts, nil
}

func localValidationFileMode(mode os.FileMode) string {
	if mode.Perm()&0o111 != 0 {
		return "100755"
	}
	return "100644"
}

func localValidationReceiptPath(repo string) (string, error) {
	return localValidationReceiptPathFor(repo, "rekit/local-validation-v3.json")
}

func localValidationLegacyReceiptPath(repo string) (string, error) {
	return localValidationReceiptPathFor(repo, "rekit/local-validation-v2.json")
}

func localValidationReceiptPathFor(repo, gitPath string) (string, error) {
	value, err := localValidationGit(repo, "rev-parse", "--git-path", gitPath)
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
	return localValidationGitOutput(repo, nil, args...)
}

func localValidationGitInput(repo string, input []byte, args ...string) (string, error) {
	value, err := localValidationGitOutput(repo, input, args...)
	return strings.TrimSpace(value), err
}

func localValidationGitOutput(repo string, input []byte, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	stdout, stderr, err := processguard.RunTreeOutputs(
		ctx,
		cmd,
		input,
		64<<20,
	)
	if err != nil {
		return string(stdout), fmt.Errorf(
			"git %s: %w: %s",
			strings.Join(args, " "),
			err,
			strings.TrimSpace(string(stderr)),
		)
	}
	return string(stdout), nil
}

func splitLocalValidationNUL(value string) []string {
	items := splitLocalValidationNULRaw(value)
	for index, item := range items {
		items[index] = filepath.ToSlash(item)
	}
	return items
}

func splitLocalValidationNULRaw(value string) []string {
	items := []string{}
	for item := range strings.SplitSeq(value, "\x00") {
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

func validLocalValidationObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
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

type localValidationReceiptVersion struct {
	SchemaVersion int `json:"schemaVersion"`
}

type localValidationReceiptV2 struct {
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

func decodeLocalValidationReceipt(data []byte) (LocalValidationReceipt, error) {
	var version localValidationReceiptVersion
	if err := json.Unmarshal(data, &version); err != nil {
		return LocalValidationReceipt{}, err
	}
	switch version.SchemaVersion {
	case localValidationReceiptLegacySchemaVersion:
		var legacy localValidationReceiptV2
		if err := strictCanonicalLocalValidationJSON(data, &legacy); err != nil {
			return LocalValidationReceipt{}, err
		}
		return LocalValidationReceipt{
			SchemaVersion: legacy.SchemaVersion, Kind: legacy.Kind, BatchID: legacy.BatchID,
			BaselineHead: legacy.BaselineHead, GateProfile: legacy.GateProfile,
			StepCount: legacy.StepCount, Passed: legacy.Passed, Failed: legacy.Failed,
			Skipped: legacy.Skipped, ReleaseCheckReady: legacy.ReleaseCheckReady,
			CreatedAt: legacy.CreatedAt, Steps: legacy.Steps, Artifacts: legacy.Artifacts,
			Boundary: legacy.Boundary,
		}, nil
	case localValidationReceiptSchemaVersion:
		var receipt LocalValidationReceipt
		if err := strictCanonicalLocalValidationJSON(data, &receipt); err != nil {
			return LocalValidationReceipt{}, err
		}
		return receipt, nil
	default:
		return LocalValidationReceipt{}, fmt.Errorf("unsupported local validation receipt schema version: %d", version.SchemaVersion)
	}
}

func strictCanonicalLocalValidationJSON(data []byte, destination any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("local validation receipt must contain exactly one JSON object")
	}
	canonical, err := json.MarshalIndent(destination, "", "  ")
	if err != nil {
		return err
	}
	canonical = append(canonical, '\n')
	if !bytes.Equal(data, canonical) {
		return fmt.Errorf("local validation receipt is not canonical JSON")
	}
	return nil
}
