package memberexecution

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

const MaxReviewerEvidenceArtifactBytes = 48 * 1024

type ReviewerEvidenceArtifact struct {
	Path    string `json:"path"`
	Role    string `json:"role"`
	SHA256  string `json:"sha256"`
	Bytes   int64  `json:"bytes"`
	Content string `json:"content"`
}

type ReviewerEvidenceClosure struct {
	Item      string                     `json:"item"`
	AttemptID string                     `json:"attemptId"`
	Owner     Owner                      `json:"owner"`
	Artifacts []ReviewerEvidenceArtifact `json:"artifacts"`
}

func SnapshotReviewerEvidenceClosure(caseRoot, pack, item string) (ReviewerEvidenceClosure, error) {
	rawItem := strings.TrimSpace(item)
	item = filepath.ToSlash(filepath.Clean(filepath.FromSlash(rawItem)))
	if rawItem != item {
		return ReviewerEvidenceClosure{}, fmt.Errorf("Remote Control reviewer item must use its exact canonical path")
	}
	view, err := projectstate.ResolveMissionView(caseRoot)
	if err != nil {
		return ReviewerEvidenceClosure{}, err
	}
	prefix, err := view.Rel("lanes")
	if err != nil {
		return ReviewerEvidenceClosure{}, err
	}
	prefix = filepath.ToSlash(prefix) + "/"
	const marker = "/member-executions/"
	const suffix = "/evidence/manifest.json"
	if item == "." || filepath.IsAbs(filepath.FromSlash(item)) || !strings.HasPrefix(item, prefix) || !strings.HasSuffix(item, suffix) {
		return ReviewerEvidenceClosure{}, fmt.Errorf("Remote Control reviewer item must be a canonical member evidence manifest path")
	}
	middle := strings.TrimSuffix(strings.TrimPrefix(item, prefix), suffix)
	parts := strings.Split(middle, marker)
	if len(parts) != 2 || !segment.MatchString(parts[0]) || !segment.MatchString(parts[1]) {
		return ReviewerEvidenceClosure{}, fmt.Errorf("Remote Control reviewer item has invalid lane or attempt identity")
	}
	lane, attemptID := parts[0], parts[1]
	inspection, err := Inspect(caseRoot, lane, attemptID)
	if err != nil {
		return ReviewerEvidenceClosure{}, err
	}
	if inspection.State != "intake-ready" || inspection.Manifest == nil || inspection.TaskContext == nil {
		return ReviewerEvidenceClosure{}, fmt.Errorf("Remote Control reviewer evidence is not intake-ready")
	}
	if err := ValidateActionableTaskContext(caseRoot, inspection); err != nil {
		return ReviewerEvidenceClosure{}, err
	}
	current, err := CurrentOwnerMatches(caseRoot, pack, inspection.Owner)
	if err != nil {
		return ReviewerEvidenceClosure{}, err
	}
	if !current {
		return ReviewerEvidenceClosure{}, fmt.Errorf("Remote Control reviewer evidence belongs to a stale member owner generation")
	}
	manifestRel, err := filepath.Rel(caseRoot, inspection.ManifestPath)
	if err != nil || filepath.ToSlash(manifestRel) != item {
		return ReviewerEvidenceClosure{}, fmt.Errorf("Remote Control reviewer item does not match the canonical member evidence manifest")
	}
	artifacts := make([]ReviewerEvidenceArtifact, 0, len(inspection.Manifest.Outputs)+2)
	appendArtifact := func(path, role string, expectedSHA string, expectedBytes int64) error {
		data, readErr := rekitfs.ReadStableRegularFileAnchored(caseRoot, path, "Remote Control reviewer evidence "+role, MaxReviewerEvidenceArtifactBytes)
		if readErr != nil {
			return readErr
		}
		if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			return fmt.Errorf("Remote Control reviewer evidence %s must be UTF-8 text", role)
		}
		if expectedSHA != "" && (!strings.EqualFold(hash(data), expectedSHA) || (expectedBytes >= 0 && int64(len(data)) != expectedBytes)) {
			return fmt.Errorf("Remote Control reviewer evidence %s hash or size drift", role)
		}
		rel, relErr := filepath.Rel(caseRoot, path)
		if relErr != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("Remote Control reviewer evidence %s escapes the case root", role)
		}
		artifacts = append(artifacts, ReviewerEvidenceArtifact{
			Path: filepath.ToSlash(rel), Role: role, SHA256: hash(data), Bytes: int64(len(data)), Content: string(data),
		})
		return nil
	}
	if err := appendArtifact(inspection.TaskContextPath, "member-task-context", inspection.TaskContextSHA256, -1); err != nil {
		return ReviewerEvidenceClosure{}, err
	}
	if err := appendArtifact(inspection.ManifestPath, "member-evidence-manifest", inspection.ManifestSHA256, -1); err != nil {
		return ReviewerEvidenceClosure{}, err
	}
	for _, output := range inspection.Manifest.Outputs {
		path := filepath.Join(inspection.OutputsRoot, filepath.FromSlash(output.Path))
		if err := appendArtifact(path, "member-evidence-output", output.SHA256, output.Bytes); err != nil {
			return ReviewerEvidenceClosure{}, err
		}
	}
	return ReviewerEvidenceClosure{Item: item, AttemptID: inspection.AttemptID, Owner: inspection.Owner, Artifacts: artifacts}, nil
}
