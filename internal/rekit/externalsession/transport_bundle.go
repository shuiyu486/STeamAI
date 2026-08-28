package externalsession

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewersession"
)

const (
	KindTransportEvidenceBundle = "current-loop-external-session-transport-evidence-bundle"
	maxTransportBundleItems     = 16
	maxTransportBundleArtifacts = 64
	maxTransportBundleRawBytes  = 48 * 1024
	maxTransportBundleBytes     = 56 * 1024
)

type TransportReviewerBinding struct {
	PacketID       string   `json:"packetId"`
	RouteID        string   `json:"routeId"`
	ShardID        string   `json:"shardId"`
	Items          []string `json:"items"`
	OutputFields   []string `json:"outputFields"`
	DispatchID     string   `json:"dispatchId"`
	DispatchSHA256 string   `json:"dispatchSha256"`
	Harness        string   `json:"harness"`
	Session        string   `json:"session"`
}

type TransportPromptArtifact struct {
	Path              string `json:"path"`
	Role              string `json:"role"`
	SourceSHA256      string `json:"sourceSha256"`
	SourceBytes       int    `json:"sourceBytes"`
	TransportedSHA256 string `json:"transportedSha256"`
	TransportedBytes  int    `json:"transportedBytes"`
	Content           string `json:"content"`
	CaseRootTokenized bool   `json:"caseRootTokenized"`
}

type TransportEvidenceArtifact struct {
	Path              string `json:"path"`
	Role              string `json:"role"`
	SourceSHA256      string `json:"sourceSha256"`
	SourceBytes       int64  `json:"sourceBytes"`
	TransportedSHA256 string `json:"transportedSha256"`
	TransportedBytes  int64  `json:"transportedBytes"`
	Content           string `json:"content"`
	Compacted         bool   `json:"compacted"`
}

type TransportEvidenceClosure struct {
	Item      string                      `json:"item"`
	AttemptID string                      `json:"attemptId"`
	Owner     memberexecution.Owner       `json:"owner"`
	Artifacts []TransportEvidenceArtifact `json:"artifacts"`
}

type TransportEvidenceBundle struct {
	SchemaVersion  int                        `json:"schemaVersion"`
	Kind           string                     `json:"kind"`
	Binding        TransportBinding           `json:"binding"`
	Reviewer       TransportReviewerBinding   `json:"reviewer"`
	Prompt         TransportPromptArtifact    `json:"prompt"`
	Closures       []TransportEvidenceClosure `json:"closures"`
	ArtifactCount  int                        `json:"artifactCount"`
	RawBytes       int                        `json:"rawBytes"`
	NoFileTransfer bool                       `json:"noFileTransfer"`
	NoHeavyTool    bool                       `json:"noHeavyTool"`
	NoAuthority    bool                       `json:"noAuthority"`
	NoConfirmed    bool                       `json:"noConfirmed"`
}

func buildTransportEvidenceBundle(job Job, ticket DispatchTicket, binding TransportBinding) (TransportEvidenceBundle, []byte, error) {
	if job.SessionKind != "reviewer" || job.Reviewer == nil || job.Reviewer.Harness != RemoteControlHarness || job.Reviewer.Session != binding.TransportBindingID {
		return TransportEvidenceBundle{}, nil, fmt.Errorf("Remote Control transport requires an explicit durable reviewer dispatch binding")
	}
	if !ticket.Launch.ReadOnly || ticket.Launch.AgentType != "read-only-reviewer" {
		return TransportEvidenceBundle{}, nil, fmt.Errorf("Remote Control transport v1 supports only the read-only Reviewer vertical slice")
	}
	if err := validateDispatchInput(job, ticket); err != nil {
		return TransportEvidenceBundle{}, nil, err
	}
	receipt, err := reviewersession.ReadDispatch(job.CaseRoot, job.Reviewer.DispatchPath, job.Reviewer.DispatchSHA256)
	if err != nil {
		return TransportEvidenceBundle{}, nil, err
	}
	fields, err := reviewersession.OutputContractFields(job.CaseRoot, receipt)
	if err != nil {
		return TransportEvidenceBundle{}, nil, err
	}
	if receipt.DispatchID != job.Reviewer.DispatchID || receipt.PacketID != job.Reviewer.PacketID || receipt.RouteID != job.Reviewer.RouteID ||
		receipt.ShardID != job.Reviewer.ShardID || !slices.Equal(receipt.Items, job.Reviewer.Items) || !slices.Equal(fields, job.Reviewer.OutputFields) ||
		receipt.ReviewerHarness != RemoteControlHarness || receipt.ReviewerHarness != job.Reviewer.Harness || receipt.ReviewerSession != job.Reviewer.Session ||
		!receipt.ReadOnly || receipt.AgentType != "read-only-reviewer" {
		return TransportEvidenceBundle{}, nil, fmt.Errorf("Remote Control transport reviewer identity does not match the exact durable dispatch receipt")
	}
	if len(receipt.Items) == 0 || len(receipt.Items) > maxTransportBundleItems {
		return TransportEvidenceBundle{}, nil, fmt.Errorf("Remote Control transport requires 1..%d reviewer evidence items", maxTransportBundleItems)
	}
	promptPath, err := transportAnchoredPath(job.CaseRoot, ticket.Launch.Input.Path)
	if err != nil {
		return TransportEvidenceBundle{}, nil, err
	}
	promptSource, err := rekitfs.ReadStableRegularFileAnchored(job.CaseRoot, promptPath, "Remote Control reviewer prompt", memberexecution.MaxReviewerEvidenceArtifactBytes)
	if err != nil {
		return TransportEvidenceBundle{}, nil, err
	}
	if !utf8.Valid(promptSource) || bytes.IndexByte(promptSource, 0) >= 0 {
		return TransportEvidenceBundle{}, nil, fmt.Errorf("Remote Control reviewer prompt must be UTF-8 text")
	}
	tokenizedPromptText := redactTransportCaseRoot(string(promptSource), job.CaseRoot)
	promptText := tokenizedPromptText
	canonicalPrompt, canonicalPromptOK, err := reviewersession.CanonicalDispatchPrompt(job.CaseRoot, receipt)
	if err != nil {
		return TransportEvidenceBundle{}, nil, err
	}
	if canonicalPromptOK && canonicalPrompt == string(promptSource) {
		promptText, _ = compactTransportReviewerPrompt(tokenizedPromptText, receipt, fields)
	}
	promptBytes := []byte(promptText)
	if transportContainsCaseRoot(promptBytes, job.CaseRoot) {
		return TransportEvidenceBundle{}, nil, fmt.Errorf("Remote Control reviewer prompt still contains the local case root after tokenization")
	}
	promptRel, err := transportRelativePath(job.CaseRoot, promptPath)
	if err != nil {
		return TransportEvidenceBundle{}, nil, err
	}
	bundle := TransportEvidenceBundle{
		SchemaVersion: SchemaVersion,
		Kind:          KindTransportEvidenceBundle,
		Binding:       binding,
		Reviewer: TransportReviewerBinding{
			PacketID: receipt.PacketID, RouteID: receipt.RouteID, ShardID: receipt.ShardID,
			Items: append([]string{}, receipt.Items...), OutputFields: append([]string{}, fields...),
			DispatchID: receipt.DispatchID, DispatchSHA256: job.Reviewer.DispatchSHA256,
			Harness: receipt.ReviewerHarness, Session: receipt.ReviewerSession,
		},
		Prompt: TransportPromptArtifact{
			Path: promptRel, Role: ticket.Launch.Input.Role,
			SourceSHA256: ticket.Launch.Input.SHA256, SourceBytes: len(promptSource),
			TransportedSHA256: hash(promptBytes), TransportedBytes: len(promptBytes), Content: promptText,
			CaseRootTokenized: string(promptSource) != tokenizedPromptText,
		},
		NoFileTransfer: true, NoHeavyTool: true, NoAuthority: true, NoConfirmed: true,
	}
	seenItems := map[string]bool{}
	seenArtifacts := map[string]bool{strings.ToLower(promptRel): true}
	rawBytes := len(promptBytes)
	artifactCount := 1
	for _, item := range receipt.Items {
		key := strings.ToLower(filepath.ToSlash(item))
		if seenItems[key] {
			return TransportEvidenceBundle{}, nil, fmt.Errorf("Remote Control reviewer evidence item is duplicated: %s", item)
		}
		seenItems[key] = true
		sourceClosure, err := memberexecution.SnapshotReviewerEvidenceClosure(job.CaseRoot, job.Pack, item)
		if err != nil {
			return TransportEvidenceBundle{}, nil, err
		}
		closure := TransportEvidenceClosure{
			Item: sourceClosure.Item, AttemptID: sourceClosure.AttemptID, Owner: sourceClosure.Owner,
			Artifacts: make([]TransportEvidenceArtifact, 0, len(sourceClosure.Artifacts)),
		}
		for _, sourceArtifact := range sourceClosure.Artifacts {
			artifactKey := strings.ToLower(sourceArtifact.Path)
			if seenArtifacts[artifactKey] {
				return TransportEvidenceBundle{}, nil, fmt.Errorf("Remote Control reviewer evidence artifact path is duplicated: %s", sourceArtifact.Path)
			}
			seenArtifacts[artifactKey] = true
			source := []byte(sourceArtifact.Content)
			if int64(len(source)) != sourceArtifact.Bytes || !strings.EqualFold(hash(source), sourceArtifact.SHA256) {
				return TransportEvidenceBundle{}, nil, fmt.Errorf("Remote Control reviewer evidence artifact binding drift: %s", sourceArtifact.Path)
			}
			transported := source
			compacted := false
			if sourceArtifact.Role == "member-task-context" {
				transported, err = compactTransportTaskContext(source)
				if err != nil {
					return TransportEvidenceBundle{}, nil, fmt.Errorf("compact Remote Control member task context: %w", err)
				}
				compacted = !bytes.Equal(transported, source)
			}
			if transportContainsCaseRoot(transported, job.CaseRoot) {
				return TransportEvidenceBundle{}, nil, fmt.Errorf("Remote Control reviewer evidence contains the local case root: %s", sourceArtifact.Path)
			}
			closure.Artifacts = append(closure.Artifacts, TransportEvidenceArtifact{
				Path: sourceArtifact.Path, Role: sourceArtifact.Role,
				SourceSHA256: sourceArtifact.SHA256, SourceBytes: sourceArtifact.Bytes,
				TransportedSHA256: hash(transported), TransportedBytes: int64(len(transported)),
				Content: string(transported), Compacted: compacted,
			})
			rawBytes += len(transported)
			artifactCount++
		}
		bundle.Closures = append(bundle.Closures, closure)
	}
	if artifactCount > maxTransportBundleArtifacts {
		return TransportEvidenceBundle{}, nil, fmt.Errorf("Remote Control evidence bundle exceeds %d artifacts", maxTransportBundleArtifacts)
	}
	if rawBytes > maxTransportBundleRawBytes {
		return TransportEvidenceBundle{}, nil, fmt.Errorf("Remote Control evidence bundle exceeds %d raw bytes: got %d", maxTransportBundleRawBytes, rawBytes)
	}
	bundle.ArtifactCount = artifactCount
	bundle.RawBytes = rawBytes
	data, err := canonical(bundle)
	if err != nil {
		return TransportEvidenceBundle{}, nil, err
	}
	if len(data) > maxTransportBundleBytes {
		return TransportEvidenceBundle{}, nil, fmt.Errorf("Remote Control canonical evidence bundle exceeds %d bytes: got %d", maxTransportBundleBytes, len(data))
	}
	if transportContainsCaseRoot(data, job.CaseRoot) {
		return TransportEvidenceBundle{}, nil, fmt.Errorf("Remote Control canonical evidence bundle contains the local case root")
	}
	return bundle, data, nil
}

func compactTransportTaskContext(source []byte) ([]byte, error) {
	var task memberexecution.TaskContext
	decoder := json.NewDecoder(bytes.NewReader(source))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&task); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("member task context must contain exactly one JSON object")
	}
	canonicalSource, err := canonical(task)
	if err != nil || !bytes.Equal(canonicalSource, source) {
		return nil, fmt.Errorf("member task context is not canonical")
	}
	task.Resume.Content = ""
	task.Checkpoint.Content = ""
	return canonical(task)
}

func validateTransportEvidenceBundle(job Job, ticket DispatchTicket, binding TransportBinding, data []byte) (TransportEvidenceBundle, error) {
	if len(data) == 0 || len(data) > maxTransportBundleBytes {
		return TransportEvidenceBundle{}, fmt.Errorf("Remote Control evidence bundle must be bounded canonical JSON")
	}
	var bundle TransportEvidenceBundle
	if err := decodeCanonicalTransport(data, &bundle); err != nil {
		return TransportEvidenceBundle{}, err
	}
	expected, expectedData, err := buildTransportEvidenceBundle(job, ticket, binding)
	if err != nil {
		return TransportEvidenceBundle{}, err
	}
	if !bytes.Equal(data, expectedData) || bundle.Kind != KindTransportEvidenceBundle || bundle.Binding != binding || bundle.ArtifactCount != expected.ArtifactCount || bundle.RawBytes != expected.RawBytes {
		return TransportEvidenceBundle{}, fmt.Errorf("Remote Control evidence bundle does not match the exact current reviewer evidence closure")
	}
	return bundle, nil
}

func transportBundlePath(caseRoot, jobID string, generation int) (string, error) {
	return projectstate.Rel(caseRoot, "external-session-transport", "bundles", jobID, fmt.Sprintf("%06d.json", generation))
}

func transportAnchoredPath(caseRoot, path string) (string, error) {
	if !filepath.IsAbs(path) {
		return rekitfs.SafeJoin(caseRoot, path)
	}
	full, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if _, err := transportRelativePath(caseRoot, full); err != nil {
		return "", err
	}
	return full, nil
}

func transportRelativePath(caseRoot, path string) (string, error) {
	root, err := filepath.Abs(caseRoot)
	if err != nil {
		return "", err
	}
	full, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("Remote Control transport path escapes the case root: %s", path)
	}
	return filepath.ToSlash(rel), nil
}

func transportContainsCaseRoot(data []byte, caseRoot string) bool {
	text := string(data)
	root := filepath.Clean(caseRoot)
	return containsFold(text, root) || containsFold(text, filepath.ToSlash(root)) || containsFold(text, strings.ReplaceAll(root, "/", "\\"))
}
