package externalsession

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
	"slices"
	"strings"
	"time"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewerresult"
)

const (
	maxSubmissionBytes = 64 * 1024
	maxOutputBytes     = 4 * 1024 * 1024
	maxOutputs         = 64
)

type observationEnvelope struct {
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

type publicationReceipt struct {
	SchemaVersion       int        `json:"schemaVersion"`
	Kind                string     `json:"kind"`
	JobID               string     `json:"jobId"`
	JobSHA256           string     `json:"jobSha256"`
	CheckpointSHA256    string     `json:"checkpointSha256"`
	SessionKind         string     `json:"sessionKind"`
	AttemptID           string     `json:"attemptId"`
	AttemptSHA256       string     `json:"attemptSha256"`
	Harness             string     `json:"harness"`
	Session             string     `json:"session"`
	Outcome             string     `json:"outcome"`
	Actor               string     `json:"actor"`
	SubmissionPath      string     `json:"submissionPath"`
	SubmissionSHA256    string     `json:"submissionSha256"`
	Artifacts           []Artifact `json:"artifacts"`
	ObservationPath     string     `json:"observationPath"`
	ObservationSHA256   string     `json:"observationSha256"`
	NoSessionManagement bool       `json:"noSessionManagement"`
	NoHeavyTool         bool       `json:"noHeavyTool"`
	NoAuthority         bool       `json:"noAuthority"`
	NoConfirmed         bool       `json:"noConfirmed"`
}

func NewMemberJob(caseRoot, pack, checkpointSHA256, attemptID string, owner memberexecution.Owner, manifestPath, outputsRoot string, allowedOutcomes []string) (Job, error) {
	job, err := baseJob(caseRoot, pack, checkpointSHA256, "member", attemptID, allowedOutcomes)
	if err != nil {
		return Job{}, err
	}
	job.MemberAttemptID = strings.TrimSpace(attemptID)
	job.MemberOwner = &owner
	job.MemberManifestPath = cleanRelative(manifestPath)
	job.MemberOutputsRoot = cleanRelative(outputsRoot)
	job.SubmissionOutputs = filepath.ToSlash(filepath.Join(".rekit", "external-session-jobs", job.JobID, "outputs"))
	if job.MemberAttemptID == "" || owner.Lane == "" || owner.Executor == "" || owner.ExecutorGeneration < 1 || job.MemberManifestPath == "" || job.MemberOutputsRoot == "" {
		return Job{}, fmt.Errorf("external member job requires exact attempt, owner, manifest, and outputs identity")
	}
	return job, nil
}

func NewReviewerJob(caseRoot, pack, checkpointSHA256 string, reviewer ReviewerIdentity, allowedOutcomes []string) (Job, error) {
	job, err := baseJob(caseRoot, pack, checkpointSHA256, "reviewer", reviewer.AttemptSHA256, allowedOutcomes)
	if err != nil {
		return Job{}, err
	}
	job.Reviewer = &reviewer
	job.SubmissionResult = filepath.ToSlash(filepath.Join(".rekit", "external-session-jobs", job.JobID, "reviewer-result.json"))
	job.RelayResultPath = filepath.ToSlash(filepath.Join(".rekit", "external-session-relays", job.JobID, "reviewer-result.json"))
	if !validSHA(reviewer.AttemptSHA256) || reviewer.PacketID == "" || reviewer.RouteID == "" || reviewer.ShardID == "" {
		return Job{}, fmt.Errorf("external reviewer job requires exact attempt, packet, route, and shard identity")
	}
	if !((reviewer.DispatchID == "" && reviewer.Harness == "" && reviewer.Session == "") || (reviewer.DispatchID != "" && reviewer.Harness != "" && reviewer.Session != "")) {
		return Job{}, fmt.Errorf("external reviewer job dispatch identity must include dispatch, harness, and session together")
	}
	return job, nil
}

func baseJob(caseRoot, pack, checkpointSHA256, sessionKind, identity string, allowedOutcomes []string) (Job, error) {
	caseRoot, err := filepath.Abs(caseRoot)
	if err != nil {
		return Job{}, err
	}
	if !validSHA(checkpointSHA256) || strings.TrimSpace(pack) == "" || strings.TrimSpace(identity) == "" || !slices.Contains([]string{"member", "reviewer"}, sessionKind) {
		return Job{}, fmt.Errorf("invalid external session job identity")
	}
	allowedOutcomes = uniqueAllowedOutcomes(allowedOutcomes)
	if len(allowedOutcomes) == 0 {
		return Job{}, fmt.Errorf("external session job requires at least one allowed outcome")
	}
	idSum := hash([]byte(sessionKind + "\x00" + strings.ToLower(checkpointSHA256) + "\x00" + identity))
	jobID := sessionKind + "-" + idSum[:24]
	base := filepath.Join(".rekit", "external-session-jobs", jobID)
	relay := filepath.Join(".rekit", "external-session-relays", jobID)
	return Job{
		SchemaVersion: SchemaVersion, Kind: KindJob, JobID: jobID, CaseRoot: caseRoot, Pack: strings.TrimSpace(pack),
		CheckpointSHA256: strings.ToLower(checkpointSHA256), SessionKind: sessionKind, AllowedOutcomes: allowedOutcomes,
		SubmissionPath:  filepath.ToSlash(filepath.Join(base, "submission.json")),
		PublicationPath: filepath.ToSlash(filepath.Join(relay, "publication.json")),
		ObservationPath: filepath.ToSlash(filepath.Join(".rekit", "external-session-observations", "inbox", jobID+".json")),
		SubmissionLast:  true, NoSessionManagement: true, NoHeavyTool: true, NoAuthority: true, NoConfirmed: true,
	}, nil
}

func Inspect(job Job) (Inspection, error) {
	jobBytes, err := canonical(job)
	if err != nil {
		return Inspection{}, err
	}
	inspection := Inspection{Job: job, JobSHA256: hash(jobBytes), State: "awaiting-submission"}
	attempt, err := InspectAttempt(job)
	if err != nil {
		return Inspection{}, err
	}
	if attempt.Current == nil {
		return inspection, nil
	}
	data, err := rekitfs.ReadStableRegularFileAnchored(job.CaseRoot, filepath.Join(job.CaseRoot, filepath.FromSlash(attempt.Current.SubmissionPath)), "external session submission", maxSubmissionBytes)
	if os.IsNotExist(err) {
		return inspection, nil
	}
	if err != nil {
		inspection.State = "invalid"
		inspection.Warnings = []string{err.Error()}
		return inspection, nil
	}
	submission, err := decodeSubmission(data)
	if err != nil {
		inspection.State = "invalid"
		inspection.Warnings = []string{err.Error()}
		return inspection, nil
	}
	inspection.Submission = &submission
	inspection.SubmissionSHA256 = hash(data)
	if err := validateSubmission(job, inspection.JobSHA256, submission); err != nil {
		inspection.State = "invalid"
		inspection.Warnings = []string{err.Error()}
		return inspection, nil
	}
	if _, err := Preview(job); err != nil {
		inspection.State = "invalid"
		inspection.Warnings = []string{err.Error()}
		return inspection, nil
	}
	inspection.State = "submission-ready"
	return inspection, nil
}

func Preview(job Job) (Plan, error) {
	jobBytes, err := canonical(job)
	if err != nil {
		return Plan{}, err
	}
	jobSHA := hash(jobBytes)
	attempt, err := InspectAttempt(job)
	if err != nil {
		return Plan{}, err
	}
	if attempt.Current == nil {
		return Plan{}, fmt.Errorf("external session relay requires a current durable harness attempt")
	}
	submissionPath := filepath.Join(job.CaseRoot, filepath.FromSlash(attempt.Current.SubmissionPath))
	submissionBytes, err := rekitfs.ReadStableRegularFileAnchored(job.CaseRoot, submissionPath, "external session submission", maxSubmissionBytes)
	if err != nil {
		return Plan{}, err
	}
	submission, err := decodeSubmission(submissionBytes)
	if err != nil {
		return Plan{}, err
	}
	if err := validateSubmission(job, jobSHA, submission); err != nil {
		return Plan{}, err
	}
	plan := Plan{
		SchemaVersion: SchemaVersion, Mode: "relay", Job: job, JobSHA256: jobSHA, Submission: submission,
		SubmissionSHA256: hash(submissionBytes), ReviewRequired: true, RequiresConfirmation: true,
		Boundary: []string{
			"relay preview binds the exact current job, submission bytes, source artifacts, destination bytes, and final observation envelope",
			"Apply revalidates every source and publishes with exclusive no-overwrite exact-prefix recovery; the inbox envelope is the final commit point",
			"relay does not claim or consume the checkpoint, manage a session, execute a heavy tool, or write authority/confirmed state",
		},
	}
	envelope := observationEnvelope{
		SchemaVersion: 1, Kind: "current-loop-external-session-observation", CheckpointSHA256: job.CheckpointSHA256,
		Actor: submission.Actor, NoAuthorityOrConfirmed: true, NoHeavyTool: true,
	}
	writes := []plannedWrite{}
	artifacts := []Artifact{}
	if job.SessionKind == "member" {
		envelope.ObservationKind = "member-session-" + submission.Outcome
		envelope.MemberAttemptID = job.MemberAttemptID
		envelope.ObservedAt = submission.ObservedAt
		envelope.Reason = submission.Reason
		if submission.Outcome == "returned" {
			outputWrites, manifestWrite, produced, err := memberResultWrites(job, submission)
			if err != nil {
				return Plan{}, err
			}
			writes = append(writes, outputWrites...)
			writes = append(writes, manifestWrite)
			artifacts = append(artifacts, produced...)
			artifacts = append(artifacts, artifactFor(manifestWrite.rel, manifestWrite.data))
			plan.memberResult = &memberexecution.ResultSnapshot{
				ManifestPath: filepath.Join(job.CaseRoot, filepath.FromSlash(job.MemberManifestPath)),
				ManifestData: append([]byte{}, manifestWrite.data...),
				OutputsRoot:  filepath.Join(job.CaseRoot, filepath.FromSlash(job.MemberOutputsRoot)),
				Outputs:      map[string][]byte{},
			}
			for _, write := range outputWrites {
				rel, relErr := filepath.Rel(filepath.FromSlash(job.MemberOutputsRoot), filepath.FromSlash(write.rel))
				if relErr != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					return Plan{}, fmt.Errorf("external member relay output escapes destination root")
				}
				plan.memberResult.Outputs[strings.ToLower(filepath.ToSlash(rel))] = append([]byte{}, write.data...)
			}
		}
	} else {
		switch submission.Outcome {
		case "accepted":
			envelope.ObservationKind = "reviewer-session-accepted"
			envelope.ReviewerAttemptSHA256 = job.Reviewer.AttemptSHA256
			envelope.ReviewerHarness = submission.ReviewerHarness
			envelope.ReviewerSession = submission.ReviewerSession
		case "returned":
			envelope.ObservationKind = "reviewer-result-returned"
			envelope.ReviewerAttemptSHA256 = job.Reviewer.AttemptSHA256
			resultPath := filepath.Join(job.CaseRoot, filepath.FromSlash(attempt.Current.SubmissionResult))
			resultBytes, err := rekitfs.ReadStableRegularFileAnchored(job.CaseRoot, resultPath, "external reviewer result", reviewerresult.MaxBytes)
			if err != nil {
				return Plan{}, err
			}
			result, err := reviewerresult.Decode(resultBytes)
			if err != nil {
				return Plan{}, err
			}
			if result.PacketID != job.Reviewer.PacketID || result.RouteID != job.Reviewer.RouteID || result.ShardID != job.Reviewer.ShardID || result.ReviewerSession != submission.ReviewerSession {
				return Plan{}, fmt.Errorf("external reviewer result does not match the job packet, route, shard, and session")
			}
			write := plannedWrite{rel: job.RelayResultPath, data: resultBytes}
			writes = append(writes, write)
			artifacts = append(artifacts, artifactFor(write.rel, write.data))
			plan.ReviewerResult = &ReviewerResultBinding{Path: write.rel, SHA256: hash(write.data), Bytes: int64(len(write.data)), data: append([]byte{}, write.data...)}
			envelope.ReviewerResultSourcePath = filepath.Join(job.CaseRoot, filepath.FromSlash(job.RelayResultPath))
		case "failed":
			envelope.ObservationKind = "reviewer-session-failed"
			envelope.ReviewerAttemptSHA256 = job.Reviewer.AttemptSHA256
			envelope.ReviewerExitStatus = submission.ReviewerExitStatus
		}
	}
	envelopeBytes, err := canonical(envelope)
	if err != nil {
		return Plan{}, err
	}
	receipt := publicationReceipt{
		SchemaVersion: SchemaVersion, Kind: KindReceipt, JobID: job.JobID, JobSHA256: jobSHA,
		CheckpointSHA256: job.CheckpointSHA256, SessionKind: job.SessionKind,
		AttemptID: submission.AttemptID, AttemptSHA256: submission.AttemptSHA256, Harness: submission.Harness, Session: submission.Session,
		Outcome: submission.Outcome, Actor: submission.Actor,
		SubmissionPath: attempt.Current.SubmissionPath, SubmissionSHA256: plan.SubmissionSHA256, Artifacts: artifacts,
		ObservationPath: job.ObservationPath, ObservationSHA256: hash(envelopeBytes),
		NoSessionManagement: true, NoHeavyTool: true, NoAuthority: true, NoConfirmed: true,
	}
	receiptBytes, err := canonical(receipt)
	if err != nil {
		return Plan{}, err
	}
	writes = append(writes, plannedWrite{rel: job.PublicationPath, data: receiptBytes})
	writes = append(writes, plannedWrite{rel: job.ObservationPath, data: envelopeBytes})
	plan.Artifacts = append(artifacts, artifactFor(job.PublicationPath, receiptBytes), artifactFor(job.ObservationPath, envelopeBytes))
	plan.Observation = ObservationBinding{
		Path:   job.ObservationPath,
		SHA256: hash(envelopeBytes),
		Bytes:  int64(len(envelopeBytes)),
		data:   append([]byte{}, envelopeBytes...),
	}
	identity := struct {
		SchemaVersion    int        `json:"schemaVersion"`
		JobSHA256        string     `json:"jobSha256"`
		SubmissionSHA256 string     `json:"submissionSha256"`
		Artifacts        []Artifact `json:"artifacts"`
	}{SchemaVersion, jobSHA, plan.SubmissionSHA256, plan.Artifacts}
	identityBytes, err := json.Marshal(identity)
	if err != nil {
		return Plan{}, err
	}
	plan.ExpectedPlanSHA256 = hash(identityBytes)
	plan.writes = writes
	return plan, nil
}

func Apply(plan Plan, expectedJobSHA256, expectedSubmissionSHA256, expectedPlanSHA256 string) (_ Plan, retErr error) {
	return ApplyCurrent(plan, expectedJobSHA256, expectedSubmissionSHA256, expectedPlanSHA256, nil)
}

func ApplyCurrent(plan Plan, expectedJobSHA256, expectedSubmissionSHA256, expectedPlanSHA256 string, current func() (Job, error)) (_ Plan, retErr error) {
	lease, err := lanemutation.AcquireProject(plan.Job.CaseRoot)
	if err != nil {
		return Plan{}, err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	if current != nil {
		live, err := current()
		if err != nil {
			return Plan{}, err
		}
		liveSHA, err := jobSHA256(live)
		if err != nil || !strings.EqualFold(liveSHA, plan.JobSHA256) {
			return Plan{}, fmt.Errorf("external session relay job is no longer current")
		}
	}
	fresh, err := Preview(plan.Job)
	if err != nil {
		return Plan{}, err
	}
	for name, pair := range map[string][2]string{
		"job": {expectedJobSHA256, fresh.JobSHA256}, "submission": {expectedSubmissionSHA256, fresh.SubmissionSHA256}, "relay plan": {expectedPlanSHA256, fresh.ExpectedPlanSHA256},
	} {
		if strings.TrimSpace(pair[0]) == "" || !strings.EqualFold(strings.TrimSpace(pair[0]), pair[1]) {
			return Plan{}, fmt.Errorf("external session %s sha256 mismatch: got %s want %s", name, strings.TrimSpace(pair[0]), pair[1])
		}
	}
	firstMissing := len(fresh.writes)
	for index, write := range fresh.writes {
		path := filepath.Join(fresh.Job.CaseRoot, filepath.FromSlash(write.rel))
		info, statErr := os.Lstat(path)
		if os.IsNotExist(statErr) {
			if firstMissing == len(fresh.writes) {
				firstMissing = index
			}
			continue
		}
		if statErr != nil {
			return Plan{}, statErr
		}
		if firstMissing != len(fresh.writes) {
			return Plan{}, fmt.Errorf("external session relay publication is non-prefix at %s", write.rel)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return Plan{}, fmt.Errorf("external session relay existing artifact must be a regular non-symlink file: %s", write.rel)
		}
		current, readErr := rekitfs.ReadStableRegularFileAnchored(fresh.Job.CaseRoot, path, "external session relay artifact", int64(len(write.data))+1)
		if readErr != nil || !bytes.Equal(current, write.data) {
			return Plan{}, fmt.Errorf("external session relay existing artifact differs: %s", write.rel)
		}
	}
	allReplayed := firstMissing == len(fresh.writes)
	for index := firstMissing; index < len(fresh.writes); index++ {
		write := fresh.writes[index]
		replayed, err := rekitfs.WriteExclusiveRegularFileAnchored(fresh.Job.CaseRoot, write.rel, "external session relay artifact", write.data)
		if err != nil {
			return Plan{}, err
		}
		allReplayed = allReplayed && replayed
	}
	fresh.Applied = true
	fresh.AlreadyApplied = allReplayed
	fresh.ReviewRequired = false
	fresh.RequiresConfirmation = false
	fresh.writes = nil
	return fresh, nil
}

func memberResultWrites(job Job, submission Submission) ([]plannedWrite, plannedWrite, []Artifact, error) {
	attempt, err := InspectAttempt(job)
	if err != nil || attempt.Current == nil {
		return nil, plannedWrite{}, nil, fmt.Errorf("external member outputs require a current durable harness attempt")
	}
	sourceRoot := filepath.Join(job.CaseRoot, filepath.FromSlash(attempt.Current.SubmissionOutputs))
	paths, err := rekitfs.WalkRegularFilesAnchored(job.CaseRoot, attempt.Current.SubmissionOutputs, "external member submission outputs", maxOutputs)
	if err != nil {
		return nil, plannedWrite{}, nil, err
	}
	if len(paths) == 0 {
		return nil, plannedWrite{}, nil, fmt.Errorf("returned external member submission requires at least one output")
	}
	outputs := make([]memberexecution.Output, 0, len(paths))
	writes := make([]plannedWrite, 0, len(paths))
	artifacts := make([]Artifact, 0, len(paths))
	for _, path := range paths {
		data, err := rekitfs.ReadStableRegularFileAnchored(job.CaseRoot, path, "external member submission output", maxOutputBytes)
		if err != nil {
			return nil, plannedWrite{}, nil, err
		}
		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, plannedWrite{}, nil, fmt.Errorf("external member output escapes submission root")
		}
		rel = filepath.ToSlash(rel)
		output := memberexecution.Output{Path: rel, SHA256: hash(data), Bytes: int64(len(data))}
		outputs = append(outputs, output)
		write := plannedWrite{rel: filepath.ToSlash(filepath.Join(filepath.FromSlash(job.MemberOutputsRoot), filepath.FromSlash(rel))), data: data}
		writes = append(writes, write)
		artifacts = append(artifacts, artifactFor(write.rel, data))
	}
	if submission.ReviewerItemsPath != "" && !slices.ContainsFunc(outputs, func(output memberexecution.Output) bool {
		return strings.EqualFold(output.Path, submission.ReviewerItemsPath)
	}) {
		return nil, plannedWrite{}, nil, fmt.Errorf("reviewerItemsPath must name a submitted output")
	}
	manifest := memberexecution.ResultManifest{
		SchemaVersion: memberexecution.SchemaVersion, Kind: memberexecution.KindManifest, AttemptID: job.MemberAttemptID,
		Owner: *job.MemberOwner, Summary: submission.Summary, Outputs: outputs, ReviewerItemsPath: submission.ReviewerItemsPath,
		NoAuthority: true, NoConfirmed: true, NoHeavyTool: true,
	}
	manifestBytes, err := canonical(manifest)
	if err != nil {
		return nil, plannedWrite{}, nil, err
	}
	return writes, plannedWrite{rel: job.MemberManifestPath, data: manifestBytes}, artifacts, nil
}

func decodeSubmission(data []byte) (Submission, error) {
	if len(data) == 0 || len(data) > maxSubmissionBytes {
		return Submission{}, fmt.Errorf("external session submission must be a bounded non-empty JSON file")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var submission Submission
	if err := dec.Decode(&submission); err != nil {
		return Submission{}, fmt.Errorf("decode external session submission: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return Submission{}, fmt.Errorf("external session submission must contain exactly one JSON object")
	}
	return submission, nil
}

func validateSubmission(job Job, jobSHA string, submission Submission) error {
	if submission.SchemaVersion != SchemaVersion || submission.Kind != KindSubmission || submission.JobID != job.JobID || !strings.EqualFold(submission.JobSHA256, jobSHA) || !slices.Contains(job.AllowedOutcomes, submission.Outcome) {
		return fmt.Errorf("external session submission does not match the current job and allowed outcomes")
	}
	attempt, err := InspectAttempt(job)
	if err != nil || attempt.Current == nil || attempt.State != "committed" || submission.AttemptID != attempt.Current.AttemptID || !strings.EqualFold(submission.AttemptSHA256, attempt.AttemptSHA256) {
		return fmt.Errorf("external session submission does not match the current durable harness attempt")
	}
	dispatch, dispatchErr := InspectCurrentDispatch(job, attempt)
	if dispatchErr != nil {
		return dispatchErr
	}
	if dispatch.Ticket != nil {
		if err := validateDispatchInput(job, *dispatch.Ticket); err != nil {
			return err
		}
		if dispatch.State != "running" || dispatch.Claim == nil || dispatch.Launch == nil || !strings.EqualFold(submission.DispatchClaimSHA256, dispatch.ClaimSHA256) || !strings.EqualFold(submission.LaunchReceiptSHA256, dispatch.LaunchSHA256) || submission.Harness != dispatch.Launch.ActualHarness || submission.Session != dispatch.Launch.ActualSession {
			return fmt.Errorf("external session submission does not match the exact accepted launch lineage")
		}
	} else {
		if job.Reviewer != nil && job.Reviewer.DispatchID != "" && (submission.Harness != job.Reviewer.Harness || submission.Session != job.Reviewer.Session) {
			return fmt.Errorf("external reviewer submission does not match the accepted dispatch harness and session")
		}
		if submission.Harness != attempt.Current.Harness || submission.Session != attempt.Current.Session {
			return fmt.Errorf("external session submission does not match the current durable harness attempt")
		}
		if submission.DispatchClaimSHA256 != "" || submission.LaunchReceiptSHA256 != "" {
			return fmt.Errorf("legacy external session submission cannot claim dispatch lineage")
		}
	}
	if strings.TrimSpace(submission.Actor) == "" || strings.ContainsAny(submission.Actor, "\r\n") || !submission.NoAuthorityOrConfirmed || !submission.NoHeavyTool {
		return fmt.Errorf("external session submission requires a single-line actor and strict no-authority/no-heavy-tool boundaries")
	}
	if submission.ObservedAt != "" {
		if _, err := time.Parse(time.RFC3339Nano, submission.ObservedAt); err != nil {
			return fmt.Errorf("external session submission observedAt must be RFC3339Nano")
		}
	}
	if job.SessionKind == "member" {
		if submission.ObservedAt == "" || (submission.Outcome == "returned" && strings.TrimSpace(submission.Summary) == "") || (submission.Outcome == "failed" && strings.TrimSpace(submission.Reason) == "") {
			return fmt.Errorf("external member submission requires observedAt, returned summary, and failed reason")
		}
		if submission.ReviewerHarness != "" || submission.ReviewerSession != "" || submission.ReviewerExitStatus != "" {
			return fmt.Errorf("external member submission contains reviewer-only fields")
		}
		return nil
	}
	if submission.Summary != "" || submission.ReviewerItemsPath != "" || submission.ObservedAt != "" || submission.Reason != "" {
		return fmt.Errorf("external reviewer submission contains member-only fields")
	}
	switch submission.Outcome {
	case "accepted":
		if strings.TrimSpace(submission.ReviewerHarness) == "" || strings.TrimSpace(submission.ReviewerSession) == "" || submission.ReviewerHarness != submission.Harness || submission.ReviewerSession != submission.Session || submission.ReviewerExitStatus != "" {
			return fmt.Errorf("accepted reviewer submission requires the exact attempt harness and session")
		}
	case "returned":
		if strings.TrimSpace(submission.ReviewerSession) == "" || submission.ReviewerSession != submission.Session || submission.ReviewerExitStatus != "" {
			return fmt.Errorf("returned reviewer submission requires the exact attempt session")
		}
	case "failed":
		if strings.TrimSpace(submission.ReviewerExitStatus) == "" || submission.ReviewerSession != "" {
			return fmt.Errorf("failed reviewer submission requires exit status")
		}
	}
	return nil
}

func uniqueAllowedOutcomes(values []string) []string {
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if slices.Contains([]string{"accepted", "returned", "failed"}, value) && !slices.Contains(out, value) {
			out = append(out, value)
		}
	}
	return out
}

func cleanRelative(value string) string {
	value = filepath.Clean(filepath.FromSlash(strings.TrimSpace(value)))
	if value == "." || filepath.IsAbs(value) || value == ".." || strings.HasPrefix(value, ".."+string(filepath.Separator)) {
		return ""
	}
	return filepath.ToSlash(value)
}

func artifactFor(path string, data []byte) Artifact {
	return Artifact{Path: filepath.ToSlash(path), SHA256: hash(data), Bytes: int64(len(data))}
}

func canonical(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validSHA(value string) bool {
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	return err == nil && len(decoded) == sha256.Size
}
