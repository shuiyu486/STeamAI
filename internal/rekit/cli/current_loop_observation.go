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
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

const (
	maxCurrentLoopObservationBytes        = 64 * 1024
	maxCurrentLoopObservationInboxEntries = 128
)

type currentLoopObservationEnvelope struct {
	SchemaVersion            int                       `json:"schemaVersion"`
	Kind                     string                    `json:"kind"`
	CheckpointSHA256         string                    `json:"checkpointSha256"`
	ObservationKind          string                    `json:"observationKind"`
	Actor                    string                    `json:"actor"`
	ObservedAt               string                    `json:"observedAt,omitempty"`
	Reason                   string                    `json:"reason,omitempty"`
	MemberAttemptID          string                    `json:"memberAttemptId,omitempty"`
	ReviewerAttemptSHA256    string                    `json:"reviewerAttemptSha256,omitempty"`
	ReviewerHarness          string                    `json:"reviewerHarness,omitempty"`
	ReviewerSession          string                    `json:"reviewerSession,omitempty"`
	ReviewerResultSourcePath string                    `json:"reviewerResultSourcePath,omitempty"`
	ReviewerExitStatus       string                    `json:"reviewerExitStatus,omitempty"`
	LaunchControl            *executioncontrol.Binding `json:"launchControl,omitempty"`
	NoAuthorityOrConfirmed   bool                      `json:"noAuthorityOrConfirmed"`
	NoHeavyTool              bool                      `json:"noHeavyTool"`
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
	var snapshot currentLoopObservationSnapshot
	var err error
	requireCanonical := true
	if opt.currentLoopObservationSnapshot != nil {
		snapshot = *opt.currentLoopObservationSnapshot
		requireCanonical = false
	} else {
		snapshot, err = readCurrentLoopObservationEnvelope(ctx.Target, path)
		if err != nil {
			return err
		}
	}
	return applyCurrentLoopObservationSnapshot(ctx, opt, inspection, snapshot, expected, requireCanonical)
}

func applyCurrentLoopObservationSnapshot(ctx runtime.Context, opt *Options, inspection currentloop.Inspection, snapshot currentLoopObservationSnapshot, expected string, requireCanonical bool) error {
	if expected != "" && snapshot.SHA256 != expected {
		return fmt.Errorf("run-current-loop observation sha256 mismatch: got %s want %s", snapshot.SHA256, expected)
	}
	if !strings.EqualFold(snapshot.Envelope.CheckpointSHA256, inspection.ArtifactSHA256) {
		return fmt.Errorf("run-current-loop observation checkpoint sha256 mismatch: got %s want %s", snapshot.Envelope.CheckpointSHA256, inspection.ArtifactSHA256)
	}
	if err := qualifyCurrentLoopObservation(&snapshot.Envelope, inspection); err != nil {
		return err
	}
	expectedControl := executioncontrol.CloneBinding(snapshot.Envelope.LaunchControl)
	if strings.HasPrefix(snapshot.Envelope.ObservationKind, "member-session-") {
		expectedControl = currentLoopObservationLaunchControl(inspection, snapshot.Envelope.ObservationKind)
		controlRequired, err := currentMemberExecutionControlRequired(ctx.Target)
		if err != nil {
			return err
		}
		if controlRequired && !executioncontrol.SameBinding(expectedControl, snapshot.Envelope.LaunchControl) {
			return fmt.Errorf("run-current-loop observation execution control binding does not match checkpoint birth lineage")
		}
		if !controlRequired {
			expectedControl = executioncontrol.CloneBinding(snapshot.Envelope.LaunchControl)
		}
	}
	if expectedControl != nil {
		if err := executioncontrol.ValidateBinding(*expectedControl); err != nil {
			return fmt.Errorf("run-current-loop observation durable execution control binding is invalid: %w", err)
		}
		if expectedControl.Lane != strings.TrimSpace(inspection.ExpectedLane) {
			return fmt.Errorf("run-current-loop observation durable execution control lane does not match checkpoint continuation")
		}
	}
	if opt.currentLoopExecutionControlBinding != nil && !executioncontrol.SameBinding(opt.currentLoopExecutionControlBinding, expectedControl) {
		return fmt.Errorf("run-current-loop observation execution control binding changed within the result turn")
	}
	opt.currentLoopExecutionControlBinding = executioncontrol.CloneBinding(expectedControl)
	if requireCanonical {
		inCanonicalInbox, err := currentLoopObservationInCanonicalInbox(ctx.Target, snapshot.Path)
		if err != nil {
			return err
		}
		if inCanonicalInbox {
			inbox := inspectCurrentLoopObservationInbox(ctx.Target, inspection)
			if inbox.State != "ready" || inbox.SelectedCandidate == nil || !refsf.SamePath(inbox.SelectedCandidate.Path, snapshot.Path) || !strings.EqualFold(inbox.SelectedCandidate.SHA256, snapshot.SHA256) {
				return fmt.Errorf("run-current-loop canonical observation inbox no longer has exactly one matching strict candidate; refresh status")
			}
		}
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

func currentLoopObservationLaunchControl(inspection currentloop.Inspection, observationKind string) *executioncontrol.Binding {
	if strings.HasPrefix(observationKind, "member-session-") && inspection.Continuation != nil && inspection.Continuation.ExternalMemberHandoff != nil {
		return executioncontrol.CloneBinding(inspection.Continuation.ExternalMemberHandoff.LaunchControl)
	}
	return nil
}

func currentLoopObservationKind(opt Options) string {
	if strings.TrimSpace(opt.CurrentLoopObservationPath) == "" {
		return ""
	}
	if outcome := strings.ToLower(strings.TrimSpace(opt.MemberExecutionOutcome)); outcome != "" {
		return "member-session-" + outcome
	}
	if strings.TrimSpace(opt.ReviewerResultInputSourcePath) != "" {
		return "reviewer-result-returned"
	}
	if strings.TrimSpace(opt.ReviewerOutcome) == "failed" {
		return "reviewer-session-failed"
	}
	if strings.TrimSpace(opt.ReviewerHarness) != "" || strings.TrimSpace(opt.ReviewerSession) != "" {
		return "reviewer-session-accepted"
	}
	return ""
}

func qualifyCurrentLoopObservation(observation *currentLoopObservationEnvelope, inspection currentloop.Inspection) error {
	if err := validateCurrentLoopObservationEnvelopeShape(observation); err != nil {
		return err
	}
	var matched *currentloop.ObservationAlternative
	if inspection.Continuation != nil && inspection.Continuation.ObservationContract != nil {
		for idx := range inspection.Continuation.ObservationContract.Alternatives {
			alternative := &inspection.Continuation.ObservationContract.Alternatives[idx]
			if alternative.Kind == observation.ObservationKind {
				matched = alternative
				break
			}
		}
	}
	if matched == nil {
		return fmt.Errorf("run-current-loop observation kind %q is not allowed by checkpoint continuation", observation.ObservationKind)
	}
	if strings.HasPrefix(observation.ObservationKind, "member-session-") {
		if inspection.Continuation.ExternalMemberHandoff == nil || observation.MemberAttemptID != inspection.Continuation.ExternalMemberHandoff.AttemptID {
			return fmt.Errorf("run-current-loop observation member attempt does not match checkpoint continuation")
		}
		return nil
	}
	expectedReviewerAttempt := strings.TrimSpace(matched.ExpectedReviewerAttemptSHA256)
	if expectedReviewerAttempt == "" {
		expectedReviewerAttempt = currentLoopObservationTemplateValue(matched.PreviewCommandTemplate, "-ExpectedCurrentLoopReviewerAttemptSha256")
	}
	if expectedReviewerAttempt == "" || !strings.EqualFold(observation.ReviewerAttemptSHA256, expectedReviewerAttempt) {
		return fmt.Errorf("run-current-loop observation reviewer attempt does not match checkpoint continuation")
	}
	return nil
}

func validateCurrentLoopObservationEnvelopeShape(observation *currentLoopObservationEnvelope) error {
	if observation.SchemaVersion != 1 || observation.Kind != "current-loop-external-session-observation" || strings.TrimSpace(observation.CheckpointSHA256) == "" || strings.TrimSpace(observation.Actor) == "" || !observation.NoAuthorityOrConfirmed || !observation.NoHeavyTool {
		return fmt.Errorf("run-current-loop observation envelope boundary or identity is invalid")
	}
	switch observation.ObservationKind {
	case "member-session-accepted", "member-session-returned", "member-session-failed", "reviewer-session-accepted", "reviewer-result-returned", "reviewer-session-failed":
	default:
		return fmt.Errorf("run-current-loop observation kind %q is unsupported", observation.ObservationKind)
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

func currentLoopObservationReceiptFromInspection(inspection currentloop.Inspection) *mission.CurrentLoopObservationReceipt {
	if inspection.ResumeSourceSHA256 == "" || inspection.ObservationSHA256 == "" {
		return nil
	}
	return &mission.CurrentLoopObservationReceipt{
		State:                     "processed",
		SourceCheckpointSHA256:    inspection.ResumeSourceSHA256,
		SuccessorCheckpointSHA256: inspection.ArtifactSHA256,
		ObservationPath:           inspection.ObservationPath,
		ObservationSHA256:         inspection.ObservationSHA256,
		ObservationKind:           inspection.ObservationKind,
		Actor:                     inspection.ObservationActor,
		Boundary: []string{
			"receipt is recovered from the strict successor checkpoint and exact observation identity",
			"older compatible checkpoints may omit observationKind and actor; replay remains blocked by the one-shot source checkpoint claim",
		},
	}
}

func currentLoopObservationInboxRel(caseRoot string) (string, error) {
	return projectstate.Rel(caseRoot, "external-session-observations", "inbox")
}

func inspectCurrentLoopObservationInbox(caseRoot string, inspection currentloop.Inspection) *mission.CurrentLoopObservationInbox {
	inboxRel, relErr := currentLoopObservationInboxRel(caseRoot)
	inbox := &mission.CurrentLoopObservationInbox{
		State:         "empty",
		Path:          inboxRel,
		LatestReceipt: currentLoopObservationReceiptFromInspection(inspection),
		Boundary: []string{
			"status discovery is read-only and selects only one strict envelope bound to the latest ready checkpoint",
			"multiple matching candidates, invalid entries, or namespace errors block selection; the runtime never guesses by filename or time",
			"processing remains WhatIf then exact hash-bound Apply; discovery does not claim the checkpoint or mutate the observation file",
		},
	}
	if relErr != nil {
		inbox.State = "invalid"
		inbox.InvalidCount = 1
		inbox.Warnings = []string{relErr.Error()}
		return inbox
	}
	paths, err := refsf.ListRegularFilesAnchored(caseRoot, inboxRel, "current-loop observation inbox", maxCurrentLoopObservationInboxEntries)
	if err != nil {
		inbox.State = "invalid"
		inbox.InvalidCount = 1
		inbox.Warnings = []string{err.Error()}
		return inbox
	}
	inbox.CandidateCount = len(paths)
	matches := []currentLoopObservationSnapshot{}
	for _, path := range paths {
		if !strings.EqualFold(filepath.Ext(path), ".json") {
			inbox.InvalidCount++
			inbox.Warnings = append(inbox.Warnings, "observation inbox entry must use .json: "+filepath.Base(path))
			continue
		}
		snapshot, err := readCurrentLoopObservationEnvelope(caseRoot, path)
		if err != nil {
			inbox.InvalidCount++
			inbox.Warnings = append(inbox.Warnings, filepath.Base(path)+": "+err.Error())
			continue
		}
		if err := validateCurrentLoopObservationEnvelopeShape(&snapshot.Envelope); err != nil {
			inbox.InvalidCount++
			inbox.Warnings = append(inbox.Warnings, filepath.Base(path)+": "+err.Error())
			continue
		}
		if !strings.EqualFold(snapshot.Envelope.CheckpointSHA256, inspection.ArtifactSHA256) {
			inbox.StaleCount++
			continue
		}
		if err := qualifyCurrentLoopObservation(&snapshot.Envelope, inspection); err != nil {
			inbox.InvalidCount++
			inbox.Warnings = append(inbox.Warnings, filepath.Base(path)+": "+err.Error())
			continue
		}
		if strings.HasPrefix(snapshot.Envelope.ObservationKind, "member-session-") {
			controlRequired, controlErr := currentMemberExecutionControlRequired(caseRoot)
			if controlErr != nil {
				inbox.InvalidCount++
				inbox.Warnings = append(inbox.Warnings, filepath.Base(path)+": "+controlErr.Error())
				continue
			}
			expectedControl := currentLoopObservationLaunchControl(inspection, snapshot.Envelope.ObservationKind)
			if controlRequired && !executioncontrol.SameBinding(expectedControl, snapshot.Envelope.LaunchControl) {
				inbox.InvalidCount++
				inbox.Warnings = append(inbox.Warnings, filepath.Base(path)+": observation execution control binding does not match checkpoint birth lineage")
				continue
			}
		}
		matches = append(matches, snapshot)
	}
	inbox.MatchingCount = len(matches)
	if inbox.InvalidCount > 0 {
		inbox.State = "invalid"
		return inbox
	}
	if len(matches) == 0 {
		return inbox
	}
	if len(matches) > 1 {
		inbox.State = "ambiguous"
		inbox.Warnings = append(inbox.Warnings, fmt.Sprintf("%d strict observations match checkpoint %s", len(matches), inspection.ArtifactSHA256))
		return inbox
	}
	selected := matches[0]
	refresh, err := statusMissionControlRefreshCommand(caseRoot)
	if err != nil {
		inbox.State = "invalid"
		inbox.InvalidCount++
		inbox.Warnings = append(inbox.Warnings, err.Error())
		return inbox
	}
	inbox.State = "ready"
	inbox.SelectedCandidate = &mission.CurrentLoopObservationInboxCandidate{
		Path:            selected.Path,
		SHA256:          selected.SHA256,
		ObservationKind: selected.Envelope.ObservationKind,
		Actor:           selected.Envelope.Actor,
	}
	command := currentLoopObservationPreviewCommand(
		caseRoot,
		inspection.ArtifactSHA256,
		selected.Path,
	)
	request, err := mission.MissionCommanderDriverRequestWithTypedCommand(
		mission.MissionCommanderDriverRequest{
			Kind:              "preview-command",
			RunLoopStepID:     "preview-current-loop-inbox-observation",
			Actor:             "main-agent",
			State:             "observation-inbox-ready",
			Source:            "missionControlRunbook.currentLoopOperator.observationInbox.selectedDriverRequest",
			Lane:              inspection.ExpectedLane,
			Label:             "preview-current-loop-inbox-observation",
			ActionID:          "preview-current-loop-inbox-observation-" + selected.SHA256[:12],
			Command:           command,
			CommandExecutable: true,
			RequiresReview:    true,
			ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
				State:                "previewed",
				RefreshStatusCommand: refresh,
				Description:          "review the exact inbox observation preview and execute only its returned hash-bound Apply command",
				Boundary: []string{
					"preview binds the selected path, exact bytes SHA, source checkpoint, current attempt, and nested plan",
					"Apply rereads the same bytes before the one-shot checkpoint claim",
				},
			},
			Boundary: append([]string{}, inbox.Boundary...),
		},
	)
	if err != nil {
		inbox.State = "invalid"
		inbox.InvalidCount++
		inbox.Warnings = append(inbox.Warnings, err.Error())
		return inbox
	}
	inbox.SelectedDriverRequest = &request
	return inbox
}

func currentLoopObservationInCanonicalInbox(caseRoot, path string) (bool, error) {
	inboxPath, err := projectstate.Join(caseRoot, "external-session-observations", "inbox")
	if err != nil {
		return false, err
	}
	parent, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return false, err
	}
	return refsf.SamePath(parent, inboxPath), nil
}

func currentLoopObservationPreviewCommand(caseRoot, checkpointSHA256, path string) string {
	return joinDriverCommand([]string{"/rekit", "run-current-loop", "-Target", caseRoot, "-ResumeCurrentLoop", "-ExpectedCurrentLoopCheckpointSha256", checkpointSHA256, "-CurrentLoopObservationPath", path, "-WhatIf", "-Format", "json"})
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
			envelope.LaunchControl = executioncontrol.CloneBinding(inspection.Continuation.ExternalMemberHandoff.LaunchControl)
			envelope.ObservedAt = "<RFC3339Nano>"
			if alternative.Kind != "member-session-accepted" {
				envelope.Reason = "<reason>"
			}
		case "reviewer-session-accepted":
			envelope.ReviewerAttemptSHA256 = alternative.ExpectedReviewerAttemptSHA256
			if envelope.ReviewerAttemptSHA256 == "" {
				envelope.ReviewerAttemptSHA256 = currentLoopObservationTemplateValue(alternative.PreviewCommandTemplate, "-ExpectedCurrentLoopReviewerAttemptSha256")
			}
			envelope.ReviewerHarness = "<harness>"
			envelope.ReviewerSession = "<session-id>"
		case "reviewer-result-returned":
			envelope.ReviewerAttemptSHA256 = alternative.ExpectedReviewerAttemptSHA256
			if envelope.ReviewerAttemptSHA256 == "" {
				envelope.ReviewerAttemptSHA256 = currentLoopObservationTemplateValue(alternative.PreviewCommandTemplate, "-ExpectedCurrentLoopReviewerAttemptSha256")
			}
			envelope.ReviewerResultSourcePath = "<reviewer-result-source-path>"
		case "reviewer-session-failed":
			envelope.ReviewerAttemptSHA256 = alternative.ExpectedReviewerAttemptSHA256
			if envelope.ReviewerAttemptSHA256 == "" {
				envelope.ReviewerAttemptSHA256 = currentLoopObservationTemplateValue(alternative.PreviewCommandTemplate, "-ExpectedCurrentLoopReviewerAttemptSha256")
			}
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
	if _, err := projectstate.Resolve(caseRoot); err != nil {
		return currentLoopObservationSnapshot{}, err
	}
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
	return decodeCurrentLoopObservationSnapshot(path, data)
}

func decodeCurrentLoopObservationSnapshot(path string, data []byte) (currentLoopObservationSnapshot, error) {
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
	return currentLoopObservationSnapshot{Path: path, Bytes: append([]byte{}, data...), SHA256: hex.EncodeToString(sum[:]), Envelope: envelope}, nil
}
