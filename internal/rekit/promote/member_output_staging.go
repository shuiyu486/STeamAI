package promote

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/kitmutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/review"
)

const maxMemberOutputStagingBytes = 1 << 20

type MemberOutputStagingOptions struct {
	Lane               string
	AttemptID          string
	OutputPath         string
	ManagedTargetPath  string
	ExpectedPlanSHA256 string
	WhatIf             bool
}

type MemberOutputStagingResult struct {
	SchemaVersion      int                   `json:"schemaVersion"`
	Command            string                `json:"command"`
	Kind               string                `json:"kind"`
	Mode               string                `json:"mode"`
	CaseRoot           string                `json:"caseRoot"`
	RepoRoot           string                `json:"repoRoot"`
	Pack               string                `json:"pack"`
	Lane               string                `json:"lane"`
	AttemptID          string                `json:"attemptId"`
	Owner              memberexecution.Owner `json:"owner"`
	TaskContextPath    string                `json:"taskContextPath"`
	TaskContextSHA256  string                `json:"taskContextSha256"`
	ManifestPath       string                `json:"manifestPath"`
	ManifestSHA256     string                `json:"manifestSha256"`
	OutputPath         string                `json:"outputPath"`
	OutputSHA256       string                `json:"outputSha256"`
	OutputBytes        int64                 `json:"outputBytes"`
	ManagedTargetPath  string                `json:"managedTargetPath"`
	TargetBeforeSHA256 string                `json:"targetBeforeSha256"`
	SanitizedSHA256    string                `json:"sanitizedSha256"`
	SanitizedBytes     int64                 `json:"sanitizedBytes"`
	ReplacementCounts  map[string]int        `json:"replacementCounts"`
	DenyViolations     []string              `json:"denyViolations,omitempty"`
	PlanSHA256         string                `json:"planSha256"`
	IntentPath         string                `json:"intentPath"`
	IntentSHA256       string                `json:"intentSha256"`
	ReceiptPath        string                `json:"receiptPath"`
	ReceiptSHA256      string                `json:"receiptSha256,omitempty"`
	IsMutation         bool                  `json:"isMutation"`
	Applied            bool                  `json:"applied"`
	Replay             bool                  `json:"replay,omitempty"`
	ReviewPending      bool                  `json:"reviewPending"`
	RequiresReview     bool                  `json:"requiresReview"`
	ApplyCommand       string                `json:"applyCommand,omitempty"`
	NextSteps          []string              `json:"nextSteps"`
	Boundary           []string              `json:"boundary"`
}

type memberOutputStagingPlan struct {
	SchemaVersion      int                   `json:"schemaVersion"`
	Kind               string                `json:"kind"`
	CaseRoot           string                `json:"caseRoot"`
	RepoRoot           string                `json:"repoRoot"`
	Pack               string                `json:"pack"`
	Lane               string                `json:"lane"`
	AttemptID          string                `json:"attemptId"`
	Owner              memberexecution.Owner `json:"owner"`
	TaskContextPath    string                `json:"taskContextPath"`
	TaskContextSHA256  string                `json:"taskContextSha256"`
	ManifestPath       string                `json:"manifestPath"`
	ManifestSHA256     string                `json:"manifestSha256"`
	OutputPath         string                `json:"outputPath"`
	OutputSHA256       string                `json:"outputSha256"`
	OutputBytes        int64                 `json:"outputBytes"`
	ManagedTargetPath  string                `json:"managedTargetPath"`
	TargetBeforeSHA256 string                `json:"targetBeforeSha256"`
	SanitizedSHA256    string                `json:"sanitizedSha256"`
	SanitizedBytes     int64                 `json:"sanitizedBytes"`
	ReplacementCounts  map[string]int        `json:"replacementCounts"`
	DenyViolations     []string              `json:"denyViolations,omitempty"`
	NoAuthority        bool                  `json:"noAuthority"`
	NoConfirmed        bool                  `json:"noConfirmed"`
	NoHeavyTool        bool                  `json:"noHeavyTool"`
	ReviewPending      bool                  `json:"reviewPending"`
}

type memberOutputStagingIntent struct {
	SchemaVersion int                     `json:"schemaVersion"`
	Kind          string                  `json:"kind"`
	Plan          memberOutputStagingPlan `json:"plan"`
	PlanSHA256    string                  `json:"planSha256"`
	SanitizedText string                  `json:"sanitizedText"`
}

type memberOutputStagingReceipt struct {
	SchemaVersion int                     `json:"schemaVersion"`
	Kind          string                  `json:"kind"`
	Plan          memberOutputStagingPlan `json:"plan"`
	PlanSHA256    string                  `json:"planSha256"`
	IntentPath    string                  `json:"intentPath"`
	IntentSHA256  string                  `json:"intentSha256"`
	TargetSHA256  string                  `json:"targetSha256"`
	ReviewPending bool                    `json:"reviewPending"`
	NoAuthority   bool                    `json:"noAuthority"`
	NoConfirmed   bool                    `json:"noConfirmed"`
	NoHeavyTool   bool                    `json:"noHeavyTool"`
}

type preparedMemberOutputStaging struct {
	result       MemberOutputStagingResult
	plan         memberOutputStagingPlan
	targetBefore []byte
	sanitized    []byte
	intent       memberOutputStagingIntent
	intentBytes  []byte
	receipt      memberOutputStagingReceipt
	receiptBytes []byte
}

func StageMemberOutput(repoRoot, caseRoot, pack string, opt MemberOutputStagingOptions) (_ MemberOutputStagingResult, retErr error) {
	if opt.WhatIf && strings.TrimSpace(opt.ExpectedPlanSHA256) != "" {
		return MemberOutputStagingResult{}, fmt.Errorf("member output staging WhatIf does not accept -ExpectedMemberOutputStagingPlanSha256")
	}
	if !opt.WhatIf && !validMemberOutputStagingSHA(opt.ExpectedPlanSHA256) {
		return MemberOutputStagingResult{}, fmt.Errorf("member output staging Apply requires a valid -ExpectedMemberOutputStagingPlanSha256 from WhatIf")
	}
	prepared, err := prepareMemberOutputStaging(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return MemberOutputStagingResult{}, err
	}
	if len(prepared.plan.DenyViolations) > 0 {
		return MemberOutputStagingResult{}, fmt.Errorf("member output remains blocked after sanitization: %s", strings.Join(prepared.plan.DenyViolations, ", "))
	}
	if opt.WhatIf {
		prepared.result.ApplyCommand = memberOutputStagingApplyCommand(prepared.result)
		return prepared.result, nil
	}
	if !strings.EqualFold(strings.TrimSpace(opt.ExpectedPlanSHA256), prepared.result.PlanSHA256) {
		return MemberOutputStagingResult{}, fmt.Errorf("member output staging changed after preview")
	}
	lease, err := kitmutation.Acquire(prepared.result.CaseRoot)
	if err != nil {
		return MemberOutputStagingResult{}, err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	laneLease, err := lanemutation.AcquireLane(prepared.result.CaseRoot, prepared.result.Lane)
	if err != nil {
		return MemberOutputStagingResult{}, err
	}
	defer func() { retErr = errors.Join(retErr, laneLease.Unlock()) }()
	prepared, err = prepareMemberOutputStaging(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return MemberOutputStagingResult{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(opt.ExpectedPlanSHA256), prepared.result.PlanSHA256) {
		return MemberOutputStagingResult{}, fmt.Errorf("member output staging changed after preview")
	}

	intentCurrent, intentExists, err := readMemberOutputStagingArtifact(prepared.result.CaseRoot, prepared.result.IntentPath, "member output staging intent", maxMemberOutputStagingBytes)
	if err != nil {
		return MemberOutputStagingResult{}, err
	}
	if intentExists && !bytes.Equal(intentCurrent, prepared.intentBytes) {
		return MemberOutputStagingResult{}, fmt.Errorf("member output staging intent has different bindings")
	}
	receiptCurrent, receiptExists, err := readMemberOutputStagingArtifact(prepared.result.CaseRoot, prepared.result.ReceiptPath, "member output staging receipt", maxMemberOutputStagingBytes)
	if err != nil {
		return MemberOutputStagingResult{}, err
	}
	if receiptExists && !bytes.Equal(receiptCurrent, prepared.receiptBytes) {
		return MemberOutputStagingResult{}, fmt.Errorf("member output staging receipt has different bindings")
	}
	if !intentExists {
		if err := publishMemberOutputStagingExclusive(prepared.result.CaseRoot, prepared.result.IntentPath, prepared.intentBytes, "member output staging intent"); err != nil {
			return MemberOutputStagingResult{}, err
		}
	}

	targetPath, err := refsf.SafeJoin(prepared.result.CaseRoot, prepared.result.ManagedTargetPath)
	if err != nil {
		return MemberOutputStagingResult{}, err
	}
	targetCurrent, targetExists, err := readMemberOutputStagingArtifact(prepared.result.CaseRoot, targetPath, "member output staging managed target", maxMemberOutputStagingBytes)
	if err != nil {
		return MemberOutputStagingResult{}, err
	}
	if !targetExists {
		return MemberOutputStagingResult{}, fmt.Errorf("member output staging managed target disappeared after preview")
	}
	if !bytes.Equal(targetCurrent, prepared.sanitized) {
		if !bytes.Equal(targetCurrent, prepared.targetBefore) {
			return MemberOutputStagingResult{}, fmt.Errorf("member output staging managed target predecessor changed after preview")
		}
		if err := replaceMemberOutputStagingExact(prepared.result.CaseRoot, targetPath, prepared.targetBefore, prepared.sanitized, "member output staging managed target"); err != nil {
			return MemberOutputStagingResult{}, err
		}
	}

	if !receiptExists {
		if err := publishMemberOutputStagingExclusive(prepared.result.CaseRoot, prepared.result.ReceiptPath, prepared.receiptBytes, "member output staging receipt"); err != nil {
			return MemberOutputStagingResult{}, err
		}
		receiptCurrent = prepared.receiptBytes
	}
	prepared.result.Mode = "staged-review-pending"
	prepared.result.IsMutation = true
	prepared.result.Applied = true
	prepared.result.Replay = receiptExists
	prepared.result.ReceiptSHA256 = memberOutputStagingSHA(receiptCurrent)
	prepared.result.ApplyCommand = ""
	prepared.result.NextSteps = []string{"run promote -CreateCandidates -WhatIf and review the exact sanitized managed-doc candidate", "do not accept or merge the candidate without an explicit review-first decision"}
	return prepared.result, nil
}

func prepareMemberOutputStaging(repoRoot, caseRoot, pack string, opt MemberOutputStagingOptions) (preparedMemberOutputStaging, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return preparedMemberOutputStaging{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return preparedMemberOutputStaging{}, err
	}
	if err := m.ValidateSchema(); err != nil {
		return preparedMemberOutputStaging{}, err
	}
	lane := strings.TrimSpace(opt.Lane)
	attemptID := strings.TrimSpace(opt.AttemptID)
	outputPath := filepath.ToSlash(strings.TrimSpace(opt.OutputPath))
	targetRel := filepath.ToSlash(strings.TrimSpace(opt.ManagedTargetPath))
	if lane == "" || attemptID == "" || outputPath == "" || targetRel == "" {
		return preparedMemberOutputStaging{}, fmt.Errorf("member output staging requires -Lane, -MemberExecutionAttemptId, -MemberOutputPath, and -ManagedTargetPath")
	}
	if !memberOutputStagingManifestTarget(m, targetRel) {
		return preparedMemberOutputStaging{}, fmt.Errorf("member output staging target must be declared by both managedFiles and promoteFiles: %s", targetRel)
	}
	packTarget, err := m.SourcePath(targetRel)
	if err != nil {
		return preparedMemberOutputStaging{}, err
	}
	packBefore, exists, err := readMemberOutputStagingArtifact(m.PackRoot, packTarget, "member output staging pack target", maxMemberOutputStagingBytes)
	if err != nil {
		return preparedMemberOutputStaging{}, err
	}
	if !exists {
		return preparedMemberOutputStaging{}, fmt.Errorf("member output staging requires an existing pack predecessor: %s", targetRel)
	}
	caseTarget, err := refsf.SafeJoin(inst.CaseRoot, targetRel)
	if err != nil {
		return preparedMemberOutputStaging{}, err
	}
	targetBefore, targetBeforeExists, err := readMemberOutputStagingArtifact(inst.CaseRoot, caseTarget, "member output staging managed target", maxMemberOutputStagingBytes)
	if err != nil {
		return preparedMemberOutputStaging{}, err
	}
	if !targetBeforeExists {
		return preparedMemberOutputStaging{}, fmt.Errorf("member output staging requires an existing case managed target: %s", targetRel)
	}

	inspection, err := memberexecution.Inspect(inst.CaseRoot, lane, attemptID)
	if err != nil {
		return preparedMemberOutputStaging{}, err
	}
	if inspection.State != "intake-ready" || inspection.Manifest == nil || inspection.Intent == nil || inspection.TaskContext == nil || inspection.Owner.Lane != lane || inspection.AttemptID != attemptID || !strings.EqualFold(inspection.Intent.Pack, pack) {
		return preparedMemberOutputStaging{}, fmt.Errorf("member output staging requires a strict intake-ready member result")
	}
	if err := memberexecution.ValidateActionableTaskContext(inst.CaseRoot, inspection); err != nil {
		return preparedMemberOutputStaging{}, err
	}
	matches, err := memberexecution.CurrentOwnerMatches(inst.CaseRoot, pack, inspection.Owner)
	if err != nil || !matches {
		return preparedMemberOutputStaging{}, errors.Join(err, fmt.Errorf("member output staging owner generation is stale"))
	}
	output, ok := memberOutputStagingOutput(inspection.Manifest.Outputs, outputPath)
	if !ok {
		return preparedMemberOutputStaging{}, fmt.Errorf("member output staging output is not declared by the strict result manifest: %s", outputPath)
	}
	sourcePath, err := refsf.SafeJoin(inspection.OutputsRoot, output.Path)
	if err != nil {
		return preparedMemberOutputStaging{}, err
	}
	raw, err := refsf.ReadStableRegularFileAnchored(inst.CaseRoot, sourcePath, "member output staging source", maxMemberOutputStagingBytes)
	if err != nil || int64(len(raw)) != output.Bytes || !strings.EqualFold(memberOutputStagingSHA(raw), output.SHA256) {
		return preparedMemberOutputStaging{}, errors.Join(err, fmt.Errorf("member output staging source does not match the strict result manifest"))
	}
	if !utf8.Valid(raw) {
		return preparedMemberOutputStaging{}, fmt.Errorf("member output staging source must be UTF-8 text")
	}
	sanitizedText, counts := sanitizeToolingCandidate(string(raw), inst.CaseRoot)
	sanitized := []byte(sanitizedText)
	if len(bytes.TrimSpace(sanitized)) == 0 || len(sanitized) > maxMemberOutputStagingBytes {
		return preparedMemberOutputStaging{}, fmt.Errorf("member output staging sanitized text must be bounded and non-empty")
	}
	denyPatterns := append([]string{}, m.PromoteDenyPatterns...)
	denyPatterns = append(denyPatterns, caseSpecificPatterns(inst.CaseRoot)...)
	violations := review.MatchAny(sanitizedText, denyPatterns)
	plan := memberOutputStagingPlan{
		SchemaVersion: 1, Kind: "pack-memory-member-output-staging-plan", CaseRoot: inst.CaseRoot, RepoRoot: m.RepoRoot, Pack: m.Pack,
		Lane: lane, AttemptID: attemptID, Owner: inspection.Owner, TaskContextPath: inspection.TaskContextPath, TaskContextSHA256: inspection.TaskContextSHA256,
		ManifestPath: inspection.ManifestPath, ManifestSHA256: inspection.ManifestSHA256, OutputPath: output.Path, OutputSHA256: output.SHA256, OutputBytes: output.Bytes,
		ManagedTargetPath: targetRel, TargetBeforeSHA256: memberOutputStagingSHA(packBefore), SanitizedSHA256: memberOutputStagingSHA(sanitized), SanitizedBytes: int64(len(sanitized)), ReplacementCounts: counts,
		DenyViolations: violations, NoAuthority: true, NoConfirmed: true, NoHeavyTool: true, ReviewPending: true,
	}
	planBytes, err := memberOutputStagingCanonical(plan)
	if err != nil {
		return preparedMemberOutputStaging{}, err
	}
	planSHA := memberOutputStagingSHA(planBytes)
	artifactRoot, err := memberOutputStagingRoot(inst.CaseRoot)
	if err != nil {
		return preparedMemberOutputStaging{}, err
	}
	intentPath := filepath.Join(artifactRoot, planSHA+".intent.json")
	receiptPath := filepath.Join(artifactRoot, planSHA+".receipt.json")
	intent := memberOutputStagingIntent{SchemaVersion: 1, Kind: "pack-memory-member-output-staging-intent", Plan: plan, PlanSHA256: planSHA, SanitizedText: sanitizedText}
	intentBytes, err := memberOutputStagingCanonical(intent)
	if err != nil {
		return preparedMemberOutputStaging{}, err
	}
	receipt := memberOutputStagingReceipt{SchemaVersion: 1, Kind: "pack-memory-member-output-staging-receipt", Plan: plan, PlanSHA256: planSHA, IntentPath: intentPath, IntentSHA256: memberOutputStagingSHA(intentBytes), TargetSHA256: plan.SanitizedSHA256, ReviewPending: true, NoAuthority: true, NoConfirmed: true, NoHeavyTool: true}
	receiptBytes, err := memberOutputStagingCanonical(receipt)
	if err != nil {
		return preparedMemberOutputStaging{}, err
	}
	result := MemberOutputStagingResult{
		SchemaVersion: 1, Command: "promote", Kind: "pack-memory-member-output-staging", Mode: "preview-review-pending",
		CaseRoot: inst.CaseRoot, RepoRoot: m.RepoRoot, Pack: m.Pack, Lane: lane, AttemptID: attemptID, Owner: inspection.Owner,
		TaskContextPath: inspection.TaskContextPath, TaskContextSHA256: inspection.TaskContextSHA256, ManifestPath: inspection.ManifestPath, ManifestSHA256: inspection.ManifestSHA256,
		OutputPath: output.Path, OutputSHA256: output.SHA256, OutputBytes: output.Bytes, ManagedTargetPath: targetRel,
		TargetBeforeSHA256: plan.TargetBeforeSHA256, SanitizedSHA256: plan.SanitizedSHA256, SanitizedBytes: plan.SanitizedBytes, ReplacementCounts: counts, DenyViolations: violations,
		PlanSHA256: planSHA, IntentPath: intentPath, IntentSHA256: receipt.IntentSHA256, ReceiptPath: receiptPath,
		ReviewPending: true, RequiresReview: true,
		NextSteps: []string{"inspect source/output/manifest lineage, replacement counts, residual deny violations, sanitized SHA-256, and target path", "run only the returned expected-hash Apply command after review"},
		Boundary:  memberOutputStagingBoundary(),
	}
	if !bytes.Equal(targetBefore, packBefore) && !bytes.Equal(targetBefore, sanitized) {
		return preparedMemberOutputStaging{}, fmt.Errorf("member output staging managed target differs from both the pack predecessor and the planned sanitized bytes")
	}
	return preparedMemberOutputStaging{result: result, plan: plan, targetBefore: packBefore, sanitized: sanitized, intent: intent, intentBytes: intentBytes, receipt: receipt, receiptBytes: receiptBytes}, nil
}

func memberOutputStagingRoot(caseRoot string) (string, error) {
	return projectstate.Join(caseRoot, "pack-memory-staging")
}

func memberOutputStagingManifestTarget(m *manifest.Manifest, target string) bool {
	managed := false
	for _, item := range m.ManagedFiles {
		managed = managed || filepath.ToSlash(item) == target
	}
	promoted := false
	for _, item := range m.PromoteFiles {
		promoted = promoted || filepath.ToSlash(item) == target
	}
	return managed && promoted
}

func memberOutputStagingOutput(outputs []memberexecution.Output, path string) (memberexecution.Output, bool) {
	for _, output := range outputs {
		if filepath.ToSlash(output.Path) == path {
			return output, true
		}
	}
	return memberexecution.Output{}, false
}

func publishMemberOutputStagingExclusive(caseRoot, path string, data []byte, label string) error {
	if len(data) == 0 || len(data) > maxMemberOutputStagingBytes {
		return fmt.Errorf("%s bytes must be bounded and non-empty", label)
	}
	rel, err := memberOutputStagingRelativePath(caseRoot, path, label)
	if err != nil {
		return err
	}
	root, rootInfo, err := openMemberOutputStagingRoot(caseRoot, label)
	if err != nil {
		return err
	}
	defer root.Close()
	parentRel := filepath.Dir(rel)
	artifactRel, err := projectstate.Rel(caseRoot, "pack-memory-staging")
	if err != nil {
		return err
	}
	if parentRel == filepath.FromSlash(artifactRel) {
		if err := root.Mkdir(parentRel, 0o700); err != nil && !os.IsExist(err) {
			return err
		}
	}
	if err := rejectMemberOutputStagingPath(caseRoot, parentRel); err != nil {
		return err
	}
	file, err := root.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	written, writeErr := file.Write(data)
	syncErr := file.Sync()
	opened, statErr := file.Stat()
	closeErr := file.Close()
	after, afterErr := root.Lstat(rel)
	if writeErr != nil || written != len(data) || syncErr != nil || statErr != nil || closeErr != nil || afterErr != nil || !after.Mode().IsRegular() || after.Mode()&os.ModeSymlink != 0 || !os.SameFile(opened, after) || opened.Size() != int64(len(data)) {
		_ = root.Remove(rel)
		return fmt.Errorf("%s exclusive publication failed: %w", label, errors.Join(writeErr, syncErr, statErr, closeErr, afterErr))
	}
	if err := revalidateMemberOutputStagingRoot(caseRoot, rootInfo, label); err != nil {
		_ = root.Remove(rel)
		return err
	}
	return rejectMemberOutputStagingPath(caseRoot, rel)
}

func memberOutputStagingRelativePath(caseRoot, path, label string) (string, error) {
	root, err := filepath.Abs(caseRoot)
	if err != nil {
		return "", err
	}
	full, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, full)
	if err != nil || filepath.IsAbs(rel) || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s path escapes case root", label)
	}
	return filepath.Clean(rel), nil
}

func openMemberOutputStagingRoot(caseRoot, label string) (*os.Root, os.FileInfo, error) {
	rootPath, err := filepath.Abs(caseRoot)
	if err != nil {
		return nil, nil, err
	}
	before, err := os.Lstat(rootPath)
	if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return nil, nil, fmt.Errorf("%s case root is invalid", label)
	}
	if err := rejectMemberOutputStagingPath(rootPath, "."); err != nil {
		return nil, nil, err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, nil, err
	}
	opened, err := root.Lstat(".")
	if err != nil || !opened.IsDir() || !os.SameFile(before, opened) {
		root.Close()
		return nil, nil, fmt.Errorf("%s case root changed while opening", label)
	}
	return root, opened, nil
}

func revalidateMemberOutputStagingRoot(caseRoot string, expected os.FileInfo, label string) error {
	current, err := os.Lstat(caseRoot)
	if err != nil || expected == nil || !os.SameFile(expected, current) {
		return fmt.Errorf("%s case root changed during publication: %w", label, err)
	}
	return rejectMemberOutputStagingPath(caseRoot, ".")
}

func readMemberOutputStagingArtifact(root, path, label string, limit int64) ([]byte, bool, error) {
	_, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	data, err := refsf.ReadStableRegularFileAnchored(root, path, label, limit)
	return data, err == nil, err
}

func replaceMemberOutputStagingExact(caseRoot, path string, before, after []byte, label string) error {
	rel, err := memberOutputStagingRelativePath(caseRoot, path, label)
	if err != nil {
		return err
	}
	if err := rejectMemberOutputStagingPath(caseRoot, rel); err != nil {
		return err
	}
	root, rootInfo, err := openMemberOutputStagingRoot(caseRoot, label)
	if err != nil {
		return err
	}
	defer root.Close()
	parentRel := filepath.Dir(rel)
	parent, err := root.OpenRoot(parentRel)
	if err != nil {
		return err
	}
	defer parent.Close()
	openedParent, err := parent.Lstat(".")
	if err != nil || !openedParent.IsDir() {
		return fmt.Errorf("%s parent is invalid", label)
	}
	name := filepath.Base(rel)
	current, err := readMemberOutputStagingParentFile(parent, name, label)
	if err != nil {
		return err
	}
	if bytes.Equal(current, after) {
		return revalidateMemberOutputStagingRoot(caseRoot, rootInfo, label)
	}
	if !bytes.Equal(current, before) {
		return fmt.Errorf("%s predecessor differs from durable intent", label)
	}
	tempPath, err := projectstate.Join(caseRoot, "pack-memory-staging", memberOutputStagingSHA(after)+".target.tmp")
	if err != nil {
		return err
	}
	tempRel, err := memberOutputStagingRelativePath(caseRoot, tempPath, label+" temporary file")
	if err != nil {
		return err
	}
	if tempCurrent, exists, err := readMemberOutputStagingArtifact(caseRoot, tempPath, label+" temporary file", maxMemberOutputStagingBytes); err != nil {
		return err
	} else if exists && !bytes.Equal(tempCurrent, after) {
		return fmt.Errorf("%s temporary file differs", label)
	} else if !exists {
		if err := publishMemberOutputStagingExclusive(caseRoot, tempPath, after, label+" temporary file"); err != nil {
			return err
		}
	}
	if !memberOutputStagingParentCurrent(root, parentRel, openedParent) {
		return fmt.Errorf("%s parent changed before replacement", label)
	}
	if err := root.Rename(tempRel, rel); err != nil {
		return err
	}
	published, err := readMemberOutputStagingParentFile(parent, name, label)
	if err != nil || !bytes.Equal(published, after) {
		return fmt.Errorf("%s differs after atomic replacement: %w", label, err)
	}
	if !memberOutputStagingParentCurrent(root, parentRel, openedParent) {
		return fmt.Errorf("%s parent changed during replacement", label)
	}
	if err := revalidateMemberOutputStagingRoot(caseRoot, rootInfo, label); err != nil {
		return err
	}
	return rejectMemberOutputStagingPath(caseRoot, rel)
}

func readMemberOutputStagingParentFile(parent *os.Root, name, label string) ([]byte, error) {
	before, err := parent.Lstat(name)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() < 1 || before.Size() > maxMemberOutputStagingBytes {
		return nil, fmt.Errorf("%s must be a bounded regular file: %w", label, err)
	}
	file, err := parent.Open(name)
	if err != nil {
		return nil, err
	}
	opened, statErr := file.Stat()
	data, readErr := io.ReadAll(io.LimitReader(file, maxMemberOutputStagingBytes+1))
	closeErr := file.Close()
	after, afterErr := parent.Lstat(name)
	if statErr != nil || readErr != nil || closeErr != nil || afterErr != nil || !os.SameFile(before, opened) || !os.SameFile(opened, after) || int64(len(data)) != opened.Size() || int64(len(data)) > maxMemberOutputStagingBytes {
		return nil, fmt.Errorf("%s changed while reading: %w", label, errors.Join(statErr, readErr, closeErr, afterErr))
	}
	return data, nil
}

func memberOutputStagingParentCurrent(root *os.Root, rel string, expected os.FileInfo) bool {
	current, err := root.OpenRoot(rel)
	if err != nil {
		return false
	}
	defer current.Close()
	info, err := current.Lstat(".")
	return err == nil && expected != nil && os.SameFile(expected, info)
}

func memberOutputStagingApplyCommand(result MemberOutputStagingResult) string {
	quote := func(value string) string { return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"` }
	return strings.Join([]string{
		"/rekit promote -Target " + quote(result.CaseRoot),
		"-Pack " + quote(result.Pack),
		"-StageMemberOutput",
		"-Lane " + quote(result.Lane),
		"-MemberExecutionAttemptId " + quote(result.AttemptID),
		"-MemberOutputPath " + quote(result.OutputPath),
		"-ManagedTargetPath " + quote(result.ManagedTargetPath),
		"-ExpectedMemberOutputStagingPlanSha256 " + quote(result.PlanSHA256),
		"-Apply -Format json",
	}, " ")
}

func memberOutputStagingBoundary() []string {
	return []string{
		"source bytes must come from one strict intake-ready member ResultManifest output and its current owner generation",
		"sanitization and deny-pattern checks run before the case managed target is replaced",
		"Apply replaces only the exact manifest-declared managed/promote predecessor and writes case-local intent and receipt",
		"staging remains reviewPending and does not create candidates, merge pack sources, or grant accept authority",
		"no authority/confirmed writes and no heavy-tool execution",
	}
}

func memberOutputStagingCanonical(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func memberOutputStagingSHA(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validMemberOutputStagingSHA(value string) bool {
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	return err == nil && len(decoded) == sha256.Size
}
