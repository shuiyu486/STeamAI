package externalsession

import (
	"bytes"
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
)

const (
	KindAttempt           = "current-loop-external-session-attempt"
	maxAttemptBytes       = 1 << 20
	maxAttemptGenerations = 256
)

type Attempt struct {
	SchemaVersion       int      `json:"schemaVersion"`
	Kind                string   `json:"kind"`
	AttemptID           string   `json:"attemptId"`
	JobID               string   `json:"jobId"`
	JobSHA256           string   `json:"jobSha256"`
	CheckpointSHA256    string   `json:"checkpointSha256"`
	SessionKind         string   `json:"sessionKind"`
	Generation          int      `json:"generation"`
	Harness             string   `json:"harness"`
	Session             string   `json:"session"`
	Actor               string   `json:"actor"`
	StartedAt           string   `json:"startedAt"`
	SupersedesSHA256    string   `json:"supersedesSha256,omitempty"`
	SubmissionPath      string   `json:"submissionPath"`
	SubmissionOutputs   string   `json:"submissionOutputs,omitempty"`
	SubmissionResult    string   `json:"submissionResult,omitempty"`
	AllowedOutcomes     []string `json:"allowedOutcomes"`
	NoSessionManagement bool     `json:"noSessionManagement"`
	NoHeavyTool         bool     `json:"noHeavyTool"`
	NoAuthority         bool     `json:"noAuthority"`
	NoConfirmed         bool     `json:"noConfirmed"`
}

type AttemptInspection struct {
	State          string   `json:"state"`
	Current        *Attempt `json:"current,omitempty"`
	AttemptSHA256  string   `json:"attemptSha256,omitempty"`
	DispatchSHA256 string   `json:"dispatchSha256,omitempty"`
	Path           string   `json:"path,omitempty"`
	Generations    int      `json:"generations"`
	Warnings       []string `json:"warnings,omitempty"`
}

type AttemptPlan struct {
	SchemaVersion        int             `json:"schemaVersion"`
	Mode                 string          `json:"mode"`
	Job                  Job             `json:"job"`
	JobSHA256            string          `json:"jobSha256"`
	Attempt              Attempt         `json:"attempt"`
	AttemptPath          string          `json:"attemptPath"`
	AttemptSHA256        string          `json:"attemptSha256"`
	Dispatch             *DispatchTicket `json:"dispatch,omitempty"`
	DispatchPath         string          `json:"dispatchPath,omitempty"`
	DispatchSHA256       string          `json:"dispatchSha256,omitempty"`
	ExpectedPlanSHA256   string          `json:"expectedPlanSha256"`
	ApplyCommand         string          `json:"applyCommand,omitempty"`
	ReviewRequired       bool            `json:"reviewRequired"`
	RequiresConfirmation bool            `json:"requiresConfirmation"`
	Applied              bool            `json:"applied"`
	AlreadyApplied       bool            `json:"alreadyApplied"`
	Boundary             []string        `json:"boundary"`
	data                 []byte
	dispatchData         []byte
}

type attemptEnvelope struct {
	SchemaVersion  int     `json:"schemaVersion"`
	Kind           string  `json:"kind"`
	Attempt        Attempt `json:"attempt"`
	DispatchSHA256 string  `json:"dispatchSha256,omitempty"`
}

func InspectAttempt(job Job) (AttemptInspection, error) {
	jobSHA, err := jobSHA256(job)
	if err != nil {
		return AttemptInspection{}, err
	}
	root := filepath.ToSlash(filepath.Join(".rekit", "external-session-attempts", job.JobID))
	paths, err := rekitfs.ListRegularFilesAnchored(job.CaseRoot, root, "external session attempts", maxAttemptGenerations)
	if err != nil {
		return AttemptInspection{}, err
	}
	inspection := AttemptInspection{State: "ready", Generations: len(paths)}
	for index, path := range paths {
		name := filepath.Base(path)
		expectedName := fmt.Sprintf("%06d.json", index+1)
		if name != expectedName {
			return AttemptInspection{}, fmt.Errorf("external session attempt generations are not contiguous: got %s want %s", name, expectedName)
		}
		data, err := rekitfs.ReadStableRegularFileAnchored(job.CaseRoot, path, "external session attempt", maxAttemptBytes)
		if err != nil {
			return AttemptInspection{}, err
		}
		attempt, dispatchSHA, err := decodeAttemptEnvelope(data)
		if err != nil {
			return AttemptInspection{}, err
		}
		if err := validateAttempt(job, jobSHA, attempt, index+1, inspection.AttemptSHA256); err != nil {
			return AttemptInspection{}, err
		}
		inspection.Current = &attempt
		inspection.AttemptSHA256 = hash(data)
		inspection.DispatchSHA256 = dispatchSHA
		inspection.Path = filepath.ToSlash(filepath.Join(root, name))
	}
	if inspection.Current != nil {
		inspection.State = "committed"
	}
	return inspection, nil
}

func PreviewAttempt(job Job, harness, session, actor, startedAt, supersedesSHA256 string) (AttemptPlan, error) {
	inspection, err := InspectAttempt(job)
	if err != nil {
		return AttemptPlan{}, err
	}
	jobSHA, err := jobSHA256(job)
	if err != nil {
		return AttemptPlan{}, err
	}
	harness = strings.TrimSpace(harness)
	session = strings.TrimSpace(session)
	actor = strings.TrimSpace(actor)
	startedAt = strings.TrimSpace(startedAt)
	supersedesSHA256 = strings.ToLower(strings.TrimSpace(supersedesSHA256))
	if !validSingleLine(harness) || !validSingleLine(session) || !validSingleLine(actor) {
		return AttemptPlan{}, fmt.Errorf("external session attempt requires single-line harness, session, and actor")
	}
	if parsed, err := time.Parse(time.RFC3339Nano, startedAt); err != nil || parsed.Format(time.RFC3339Nano) != startedAt {
		return AttemptPlan{}, fmt.Errorf("external session attempt startedAt must be canonical RFC3339Nano")
	}
	generation := inspection.Generations + 1
	if generation > maxAttemptGenerations {
		return AttemptPlan{}, fmt.Errorf("external session attempt generation limit reached")
	}
	if generation == 1 && supersedesSHA256 != "" {
		return AttemptPlan{}, fmt.Errorf("first external session attempt cannot supersede another attempt")
	}
	if generation > 1 && !strings.EqualFold(supersedesSHA256, inspection.AttemptSHA256) {
		return AttemptPlan{}, fmt.Errorf("replacement external session attempt must supersede the exact current attempt")
	}
	if job.Reviewer != nil && job.Reviewer.DispatchID != "" && (job.Reviewer.Harness != harness || job.Reviewer.Session != session) {
		return AttemptPlan{}, fmt.Errorf("external reviewer result attempt must match the accepted dispatch harness and session")
	}
	if inspection.Current != nil && inspection.Current.Harness == harness && inspection.Current.Session == session {
		return AttemptPlan{}, fmt.Errorf("replacement external session attempt requires a distinct harness or session")
	}
	if inspection.Current != nil {
		if _, err := os.Lstat(filepath.Join(job.CaseRoot, filepath.FromSlash(inspection.Current.SubmissionPath))); err == nil {
			current, inspectErr := Inspect(job)
			if inspectErr != nil {
				return AttemptPlan{}, inspectErr
			}
			if current.State != "invalid" {
				return AttemptPlan{}, fmt.Errorf("external session attempt cannot replace after a valid current submission exists")
			}
		} else if !os.IsNotExist(err) {
			return AttemptPlan{}, err
		}
	}
	if _, err := os.Lstat(filepath.Join(job.CaseRoot, filepath.FromSlash(job.PublicationPath))); err == nil {
		return AttemptPlan{}, fmt.Errorf("external session attempt cannot start after terminal publication exists")
	} else if !os.IsNotExist(err) {
		return AttemptPlan{}, err
	}
	attemptID := fmt.Sprintf("%s-g%06d", job.JobID, generation)
	attemptRoot := filepath.Join(".rekit", "external-session-attempt-inputs", job.JobID, fmt.Sprintf("%06d", generation))
	attempt := Attempt{
		SchemaVersion: SchemaVersion, Kind: KindAttempt, AttemptID: attemptID,
		JobID: job.JobID, JobSHA256: jobSHA, CheckpointSHA256: job.CheckpointSHA256,
		SessionKind: job.SessionKind, Generation: generation, Harness: harness, Session: session,
		Actor: actor, StartedAt: startedAt, SupersedesSHA256: supersedesSHA256,
		SubmissionPath: filepath.ToSlash(filepath.Join(attemptRoot, "submission.json")), AllowedOutcomes: append([]string{}, job.AllowedOutcomes...),
		NoSessionManagement: true, NoHeavyTool: true, NoAuthority: true, NoConfirmed: true,
	}
	if job.SessionKind == "member" {
		attempt.SubmissionOutputs = filepath.ToSlash(filepath.Join(attemptRoot, "outputs"))
	} else {
		attempt.SubmissionResult = filepath.ToSlash(filepath.Join(attemptRoot, "reviewer-result.json"))
	}
	envelopeBytes, err := canonical(attemptEnvelope{SchemaVersion: SchemaVersion, Kind: KindAttempt, Attempt: attempt})
	if err != nil {
		return AttemptPlan{}, err
	}
	path := filepath.ToSlash(filepath.Join(".rekit", "external-session-attempts", job.JobID, fmt.Sprintf("%06d.json", generation)))
	identityBytes, err := json.Marshal(struct {
		JobSHA256      string `json:"jobSha256"`
		PreviousSHA256 string `json:"previousSha256,omitempty"`
		AttemptPath    string `json:"attemptPath"`
		AttemptSHA256  string `json:"attemptSha256"`
	}{jobSHA, inspection.AttemptSHA256, path, hash(envelopeBytes)})
	if err != nil {
		return AttemptPlan{}, err
	}
	return AttemptPlan{
		SchemaVersion: SchemaVersion, Mode: "external-session-attempt", Job: job, JobSHA256: jobSHA,
		Attempt: attempt, AttemptPath: path, AttemptSHA256: hash(envelopeBytes), ExpectedPlanSHA256: hash(identityBytes),
		ReviewRequired: true, RequiresConfirmation: true,
		Boundary: []string{
			"attempt Apply records external harness/session ownership but does not start, poll, stop, or observe the session",
			"replacement requires the exact current attempt hash and creates an immutable next generation",
			"submission must bind the current attempt identity; superseded session output fails closed",
			"attempt receipt does not execute heavy tools or write authority/confirmed state",
		},
		data: envelopeBytes,
	}, nil
}

func ResolveAttemptApplyPlan(job Job, harness, session, actor, startedAt, supersedesSHA256, expectedPlanSHA256 string) (AttemptPlan, error) {
	return resolveAttemptApplyPlan(job, harness, session, actor, startedAt, supersedesSHA256, expectedPlanSHA256, true)
}

func ResolveAttemptApplyPlanUnbound(job Job, harness, session, actor, startedAt, supersedesSHA256 string) (AttemptPlan, error) {
	return resolveAttemptApplyPlan(job, harness, session, actor, startedAt, supersedesSHA256, "", false)
}

func resolveAttemptApplyPlan(job Job, harness, session, actor, startedAt, supersedesSHA256, expectedPlanSHA256 string, validateExpected bool) (AttemptPlan, error) {
	plan, err := PreviewAttempt(job, harness, session, actor, startedAt, supersedesSHA256)
	if err == nil {
		return plan, nil
	}
	inspection, inspectErr := InspectAttempt(job)
	if inspectErr != nil || inspection.Current == nil {
		return AttemptPlan{}, err
	}
	attempt := *inspection.Current
	if attempt.Harness != strings.TrimSpace(harness) || attempt.Session != strings.TrimSpace(session) || attempt.Actor != strings.TrimSpace(actor) || attempt.StartedAt != strings.TrimSpace(startedAt) || !strings.EqualFold(attempt.SupersedesSHA256, strings.TrimSpace(supersedesSHA256)) {
		return AttemptPlan{}, err
	}
	data, readErr := rekitfs.ReadStableRegularFileAnchored(job.CaseRoot, filepath.Join(job.CaseRoot, filepath.FromSlash(inspection.Path)), "external session attempt", maxAttemptBytes)
	if readErr != nil || !strings.EqualFold(hash(data), inspection.AttemptSHA256) {
		return AttemptPlan{}, fmt.Errorf("external session committed attempt receipt is unavailable or changed")
	}
	jobSHA, hashErr := jobSHA256(job)
	if hashErr != nil {
		return AttemptPlan{}, hashErr
	}
	identityBytes, marshalErr := json.Marshal(struct {
		JobSHA256      string `json:"jobSha256"`
		PreviousSHA256 string `json:"previousSha256,omitempty"`
		AttemptPath    string `json:"attemptPath"`
		AttemptSHA256  string `json:"attemptSha256"`
	}{jobSHA, attempt.SupersedesSHA256, inspection.Path, inspection.AttemptSHA256})
	if marshalErr != nil {
		return AttemptPlan{}, marshalErr
	}
	plan = AttemptPlan{
		SchemaVersion: SchemaVersion, Mode: "external-session-attempt", Job: job, JobSHA256: jobSHA,
		Attempt: attempt, AttemptPath: inspection.Path, AttemptSHA256: inspection.AttemptSHA256,
		ExpectedPlanSHA256: hash(identityBytes), ReviewRequired: true, RequiresConfirmation: true, data: data,
	}
	if validateExpected && !strings.EqualFold(strings.TrimSpace(expectedPlanSHA256), plan.ExpectedPlanSHA256) {
		return AttemptPlan{}, err
	}
	return plan, nil
}

func ApplyAttempt(plan AttemptPlan, expectedJobSHA256, expectedPlanSHA256 string) (_ AttemptPlan, retErr error) {
	return ApplyAttemptCurrent(plan, expectedJobSHA256, expectedPlanSHA256, nil)
}

func ApplyAttemptCurrent(plan AttemptPlan, expectedJobSHA256, expectedPlanSHA256 string, current func() (Job, error)) (_ AttemptPlan, retErr error) {
	if !strings.EqualFold(strings.TrimSpace(expectedJobSHA256), plan.JobSHA256) || !strings.EqualFold(strings.TrimSpace(expectedPlanSHA256), plan.ExpectedPlanSHA256) {
		return AttemptPlan{}, fmt.Errorf("external session attempt job or plan sha256 mismatch; rerun WhatIf")
	}
	lease, err := lanemutation.AcquireProject(plan.Job.CaseRoot)
	if err != nil {
		return AttemptPlan{}, err
	}
	defer func() { retErr = errorsJoin(retErr, lease.Unlock()) }()
	if current != nil {
		live, err := current()
		if err != nil {
			return AttemptPlan{}, err
		}
		liveSHA, err := jobSHA256(live)
		if err != nil || !strings.EqualFold(liveSHA, plan.JobSHA256) {
			return AttemptPlan{}, fmt.Errorf("external session attempt job is no longer current")
		}
	}
	dispatched := plan.Dispatch != nil || len(plan.dispatchData) != 0 || plan.DispatchPath != "" || plan.DispatchSHA256 != ""
	if dispatched && (plan.Dispatch == nil || len(plan.dispatchData) == 0 || plan.DispatchPath == "" || !isSHA(plan.DispatchSHA256)) {
		return AttemptPlan{}, fmt.Errorf("external session attempt requires an exact immutable dispatch ticket")
	}
	fresh, err := PreviewAttempt(plan.Job, plan.Attempt.Harness, plan.Attempt.Session, plan.Attempt.Actor, plan.Attempt.StartedAt, plan.Attempt.SupersedesSHA256)
	if err != nil {
		if inspection, inspectErr := InspectAttempt(plan.Job); inspectErr == nil && inspection.Current != nil && inspection.AttemptSHA256 == hash(plan.data) {
			if dispatched {
				dispatch, dispatchErr := InspectDispatch(plan.Job, inspection)
				if dispatchErr != nil || dispatch.Ticket == nil || !strings.EqualFold(dispatch.TicketSHA256, plan.DispatchSHA256) {
					return AttemptPlan{}, fmt.Errorf("external session committed attempt is missing its exact dispatch ticket")
				}
			}
			plan.Applied = true
			plan.AlreadyApplied = true
			plan.ReviewRequired = false
			plan.RequiresConfirmation = false
			plan.data = nil
			plan.dispatchData = nil
			return plan, nil
		}
		return AttemptPlan{}, err
	}
	if dispatched {
		fresh, err = BindAttemptDispatch(fresh, *plan.Dispatch)
		if err != nil {
			return AttemptPlan{}, err
		}
	}
	if fresh.ExpectedPlanSHA256 != plan.ExpectedPlanSHA256 || !bytes.Equal(fresh.data, plan.data) || (dispatched && !bytes.Equal(fresh.dispatchData, plan.dispatchData)) {
		return AttemptPlan{}, fmt.Errorf("external session attempt or dispatch changed after preview; rerun WhatIf")
	}
	if prior, err := InspectAttempt(plan.Job); err != nil {
		return AttemptPlan{}, err
	} else if prior.Current != nil {
		if _, err := os.Lstat(filepath.Join(plan.Job.CaseRoot, filepath.FromSlash(prior.Current.SubmissionPath))); err == nil {
			current, inspectErr := Inspect(plan.Job)
			if inspectErr != nil {
				return AttemptPlan{}, inspectErr
			}
			if current.State != "invalid" {
				return AttemptPlan{}, fmt.Errorf("external session attempt cannot replace after a valid current submission exists")
			}
		} else if !os.IsNotExist(err) {
			return AttemptPlan{}, err
		}
	}
	dispatchReplayed := true
	if dispatched {
		dispatchReplayed, err = rekitfs.WriteExclusiveRegularFileAnchored(plan.Job.CaseRoot, plan.DispatchPath, "external session dispatch ticket", fresh.dispatchData)
		if err != nil {
			return AttemptPlan{}, err
		}
	}
	receiptReplayed, err := rekitfs.WriteExclusiveRegularFileAnchored(plan.Job.CaseRoot, plan.AttemptPath, "external session attempt", fresh.data)
	if err != nil {
		return AttemptPlan{}, err
	}
	plan.Applied = true
	plan.AlreadyApplied = receiptReplayed && (!dispatched || dispatchReplayed)
	plan.ReviewRequired = false
	plan.RequiresConfirmation = false
	plan.data = nil
	plan.dispatchData = nil
	return plan, nil
}

func decodeAttemptEnvelope(data []byte) (Attempt, string, error) {
	if len(data) == 0 || len(data) > maxAttemptBytes {
		return Attempt{}, "", fmt.Errorf("external session attempt must be bounded non-empty JSON")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var envelope attemptEnvelope
	if err := dec.Decode(&envelope); err != nil {
		return Attempt{}, "", err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return Attempt{}, "", fmt.Errorf("external session attempt must contain exactly one JSON object")
	}
	canonicalBytes, err := canonical(envelope)
	if err != nil || !bytes.Equal(canonicalBytes, data) || envelope.SchemaVersion != SchemaVersion || envelope.Kind != KindAttempt || (envelope.DispatchSHA256 != "" && !isSHA(envelope.DispatchSHA256)) {
		return Attempt{}, "", fmt.Errorf("external session attempt envelope is not canonical")
	}
	return envelope.Attempt, envelope.DispatchSHA256, nil
}

func validateAttempt(job Job, jobSHA string, attempt Attempt, generation int, supersedesSHA string) error {
	root := filepath.Join(".rekit", "external-session-attempt-inputs", job.JobID, fmt.Sprintf("%06d", generation))
	expectedSubmission := filepath.ToSlash(filepath.Join(root, "submission.json"))
	expectedOutputs := ""
	expectedResult := ""
	if job.SessionKind == "member" {
		expectedOutputs = filepath.ToSlash(filepath.Join(root, "outputs"))
	} else {
		expectedResult = filepath.ToSlash(filepath.Join(root, "reviewer-result.json"))
	}
	if attempt.SchemaVersion != SchemaVersion || attempt.Kind != KindAttempt || attempt.JobID != job.JobID || !strings.EqualFold(attempt.JobSHA256, jobSHA) || attempt.CheckpointSHA256 != job.CheckpointSHA256 || attempt.SessionKind != job.SessionKind || attempt.Generation != generation || attempt.AttemptID != fmt.Sprintf("%s-g%06d", job.JobID, generation) || !strings.EqualFold(attempt.SupersedesSHA256, supersedesSHA) || attempt.SubmissionPath != expectedSubmission || attempt.SubmissionOutputs != expectedOutputs || attempt.SubmissionResult != expectedResult || !slices.Equal(attempt.AllowedOutcomes, job.AllowedOutcomes) || !validSingleLine(attempt.Harness) || !validSingleLine(attempt.Session) || !validSingleLine(attempt.Actor) || !attempt.NoSessionManagement || !attempt.NoHeavyTool || !attempt.NoAuthority || !attempt.NoConfirmed {
		return fmt.Errorf("external session attempt contract is invalid")
	}
	if parsed, err := time.Parse(time.RFC3339Nano, attempt.StartedAt); err != nil || parsed.Format(time.RFC3339Nano) != attempt.StartedAt {
		return fmt.Errorf("external session attempt startedAt is invalid")
	}
	return nil
}

func jobSHA256(job Job) (string, error) {
	data, err := canonical(job)
	if err != nil {
		return "", err
	}
	return hash(data), nil
}

func validSingleLine(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && !strings.ContainsAny(value, "\r\n")
}

func errorsJoin(left, right error) error {
	return errors.Join(left, right)
}
