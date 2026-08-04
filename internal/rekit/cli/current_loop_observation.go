package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/currentloop"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

const maxCurrentLoopObservationBytes = 64 * 1024

type currentLoopObservationEnvelope struct {
	SchemaVersion            int    `json:"schemaVersion"`
	Kind                     string `json:"kind"`
	CheckpointSHA256         string `json:"checkpointSha256"`
	ObservationKind          string `json:"observationKind"`
	Actor                    string `json:"actor"`
	ObservedAt               string `json:"observedAt,omitempty"`
	Reason                   string `json:"reason,omitempty"`
	MemberAttemptID          string `json:"memberAttemptId,omitempty"`
	ReviewerAttemptSHA256    string `json:"reviewerAttemptSha256,omitempty"`
	ReviewerHarness          string `json:"reviewerHarness,omitempty"`
	ReviewerSession          string `json:"reviewerSession,omitempty"`
	ReviewerResultSourcePath string `json:"reviewerResultSourcePath,omitempty"`
	ReviewerExitStatus       string `json:"reviewerExitStatus,omitempty"`
	NoAuthorityOrConfirmed   bool   `json:"noAuthorityOrConfirmed"`
	NoHeavyTool              bool   `json:"noHeavyTool"`
}

type currentLoopObservationSnapshot struct {
	Path     string
	Bytes    []byte
	SHA256   string
	Envelope currentLoopObservationEnvelope
}

func applyCurrentLoopObservationEnvelope(ctx runtime.Context, opt *Options, inspection currentloop.Inspection) error {
	path := strings.TrimSpace(opt.CurrentLoopObservationPath)
	expected := strings.ToLower(strings.TrimSpace(opt.ExpectedCurrentLoopObservationSHA256))
	if path == "" {
		if expected != "" {
			return fmt.Errorf("run-current-loop -ExpectedCurrentLoopObservationSha256 requires -CurrentLoopObservationPath")
		}
		return nil
	}
	snapshot, err := readCurrentLoopObservationEnvelope(ctx.Target, path)
	if err != nil {
		return err
	}
	if expected != "" && snapshot.SHA256 != expected {
		return fmt.Errorf("run-current-loop observation sha256 mismatch: got %s want %s", snapshot.SHA256, expected)
	}
	if !strings.EqualFold(snapshot.Envelope.CheckpointSHA256, inspection.ArtifactSHA256) {
		return fmt.Errorf("run-current-loop observation checkpoint sha256 mismatch: got %s want %s", snapshot.Envelope.CheckpointSHA256, inspection.ArtifactSHA256)
	}
	if err := qualifyCurrentLoopObservation(&snapshot.Envelope, inspection); err != nil {
		return err
	}
	opt.CurrentLoopObservationPath = snapshot.Path
	opt.ExpectedCurrentLoopObservationSHA256 = snapshot.SHA256
	observation := snapshot.Envelope
	opt.Start.Actor = observation.Actor
	opt.Note.Actor = observation.Actor
	switch observation.ObservationKind {
	case "member-session-accepted", "member-session-returned", "member-session-failed":
		opt.MemberExecutionAttemptID = observation.MemberAttemptID
		opt.MemberExecutionOutcome = strings.TrimPrefix(observation.ObservationKind, "member-session-")
		opt.MemberExecutionReason = observation.Reason
		opt.MemberExecutionObservedAt = observation.ObservedAt
	case "reviewer-session-accepted":
		opt.ExpectedCurrentLoopReviewerAttemptSHA256 = observation.ReviewerAttemptSHA256
		opt.ReviewerHarness = observation.ReviewerHarness
		opt.ReviewerSession = observation.ReviewerSession
	case "reviewer-result-returned":
		opt.ExpectedCurrentLoopReviewerAttemptSHA256 = observation.ReviewerAttemptSHA256
		opt.ReviewerResultInputSourcePath = observation.ReviewerResultSourcePath
	case "reviewer-session-failed":
		opt.ExpectedCurrentLoopReviewerAttemptSHA256 = observation.ReviewerAttemptSHA256
		opt.ReviewerOutcome = "failed"
		opt.ReviewerExitStatus = observation.ReviewerExitStatus
	default:
		return fmt.Errorf("run-current-loop observation kind %q is unsupported", observation.ObservationKind)
	}
	return nil
}

func qualifyCurrentLoopObservation(observation *currentLoopObservationEnvelope, inspection currentloop.Inspection) error {
	if observation.SchemaVersion != 1 || observation.Kind != "current-loop-external-session-observation" || strings.TrimSpace(observation.Actor) == "" || !observation.NoAuthorityOrConfirmed || !observation.NoHeavyTool {
		return fmt.Errorf("run-current-loop observation envelope boundary or identity is invalid")
	}
	allowed := false
	if inspection.Continuation != nil && inspection.Continuation.ObservationContract != nil {
		for _, alternative := range inspection.Continuation.ObservationContract.Alternatives {
			if alternative.Kind == observation.ObservationKind {
				allowed = true
				break
			}
		}
	}
	if !allowed {
		return fmt.Errorf("run-current-loop observation kind %q is not allowed by checkpoint continuation", observation.ObservationKind)
	}
	memberKind := strings.HasPrefix(observation.ObservationKind, "member-session-")
	if memberKind {
		if observation.MemberAttemptID == "" || observation.ObservedAt == "" || observation.ReviewerAttemptSHA256 != "" || observation.ReviewerHarness != "" || observation.ReviewerSession != "" || observation.ReviewerResultSourcePath != "" || observation.ReviewerExitStatus != "" {
			return fmt.Errorf("run-current-loop member observation envelope fields are invalid")
		}
		return nil
	}
	if observation.MemberAttemptID != "" || observation.ObservedAt != "" || observation.Reason != "" || observation.ReviewerAttemptSHA256 == "" {
		return fmt.Errorf("run-current-loop reviewer observation envelope fields are invalid")
	}
	switch observation.ObservationKind {
	case "reviewer-session-accepted":
		if observation.ReviewerHarness == "" || observation.ReviewerSession == "" || observation.ReviewerResultSourcePath != "" || observation.ReviewerExitStatus != "" {
			return fmt.Errorf("run-current-loop accepted reviewer observation envelope fields are invalid")
		}
	case "reviewer-result-returned":
		if observation.ReviewerResultSourcePath == "" || observation.ReviewerHarness != "" || observation.ReviewerSession != "" || observation.ReviewerExitStatus != "" {
			return fmt.Errorf("run-current-loop returned reviewer observation envelope fields are invalid")
		}
	case "reviewer-session-failed":
		if observation.ReviewerExitStatus == "" || observation.ReviewerHarness != "" || observation.ReviewerSession != "" || observation.ReviewerResultSourcePath != "" {
			return fmt.Errorf("run-current-loop failed reviewer observation envelope fields are invalid")
		}
	}
	return nil
}

func materializeCurrentLoopObservationEnvelopes(caseRoot string, inspection currentloop.Inspection, contract *mission.CurrentLoopObservationContract) {
	if contract == nil || inspection.ArtifactSHA256 == "" {
		return
	}
	for idx := range contract.Alternatives {
		alternative := &contract.Alternatives[idx]
		if alternative.Kind == "reviewer-result-direct-write" {
			continue
		}
		envelope := currentLoopObservationEnvelope{
			SchemaVersion:          1,
			Kind:                   "current-loop-external-session-observation",
			CheckpointSHA256:       inspection.ArtifactSHA256,
			ObservationKind:        alternative.Kind,
			Actor:                  "<harness>",
			NoAuthorityOrConfirmed: true,
			NoHeavyTool:            true,
		}
		switch alternative.Kind {
		case "member-session-accepted", "member-session-returned", "member-session-failed":
			if inspection.Continuation == nil || inspection.Continuation.ExternalMemberHandoff == nil {
				continue
			}
			envelope.MemberAttemptID = inspection.Continuation.ExternalMemberHandoff.AttemptID
			envelope.ObservedAt = "<RFC3339Nano>"
			if alternative.Kind != "member-session-accepted" {
				envelope.Reason = "<reason>"
			}
		case "reviewer-session-accepted":
			envelope.ReviewerAttemptSHA256 = currentLoopObservationTemplateValue(alternative.PreviewCommandTemplate, "-ExpectedCurrentLoopReviewerAttemptSha256")
			envelope.ReviewerHarness = "<harness>"
			envelope.ReviewerSession = "<session-id>"
		case "reviewer-result-returned":
			envelope.ReviewerAttemptSHA256 = currentLoopObservationTemplateValue(alternative.PreviewCommandTemplate, "-ExpectedCurrentLoopReviewerAttemptSha256")
			envelope.ReviewerResultSourcePath = "<reviewer-result-source-path>"
		case "reviewer-session-failed":
			envelope.ReviewerAttemptSHA256 = currentLoopObservationTemplateValue(alternative.PreviewCommandTemplate, "-ExpectedCurrentLoopReviewerAttemptSha256")
			envelope.ReviewerExitStatus = "<exit-status>"
		default:
			continue
		}
		data, err := json.MarshalIndent(envelope, "", "  ")
		if err != nil {
			continue
		}
		alternative.ObservationEnvelopeTemplate = string(data)
		alternative.ObservationPathCommand = currentLoopObservationPathCommand(caseRoot, inspection.ArtifactSHA256)
	}
}

func currentLoopObservationPathCommand(caseRoot, checkpointSHA256 string) string {
	return joinDriverCommand([]string{"/rekit", "run-current-loop", "-Target", caseRoot, "-ResumeCurrentLoop", "-ExpectedCurrentLoopCheckpointSha256", checkpointSHA256, "-CurrentLoopObservationPath", "<case-local-observation.json>", "-WhatIf", "-Format", "json"})
}

func currentLoopObservationTemplateValue(command, flag string) string {
	fields, err := splitDriverCommand(command)
	if err != nil {
		return ""
	}
	for idx := 0; idx+1 < len(fields); idx++ {
		if strings.EqualFold(fields[idx], flag) {
			return fields[idx+1]
		}
	}
	return ""
}

func readCurrentLoopObservationEnvelope(caseRoot, requested string) (currentLoopObservationSnapshot, error) {
	path := strings.TrimSpace(requested)
	if !filepath.IsAbs(path) {
		path = filepath.Join(caseRoot, path)
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return currentLoopObservationSnapshot{}, err
	}
	data, err := refsf.ReadStableRegularFileAnchored(caseRoot, path, "current-loop observation file", maxCurrentLoopObservationBytes)
	if err != nil {
		return currentLoopObservationSnapshot{}, err
	}
	var envelope currentLoopObservationEnvelope
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return currentLoopObservationSnapshot{}, fmt.Errorf("decode current-loop observation envelope: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return currentLoopObservationSnapshot{}, fmt.Errorf("current-loop observation file must contain exactly one JSON object")
	}
	sum := sha256.Sum256(data)
	return currentLoopObservationSnapshot{Path: path, Bytes: data, SHA256: hex.EncodeToString(sum[:]), Envelope: envelope}, nil
}
