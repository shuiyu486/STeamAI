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

	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

const (
	KindDispatchTicket = "current-loop-external-session-dispatch-ticket"
	KindDispatchClaim  = "current-loop-external-session-dispatch-claim"
	KindLaunchReceipt  = "current-loop-external-session-launch-receipt"
	maxDispatchBytes   = 1 << 20
)

type DispatchInput struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Role   string `json:"role"`
}

type DispatchLaunch struct {
	Ready          bool          `json:"ready"`
	Tool           string        `json:"tool"`
	AgentType      string        `json:"agentType"`
	ReadOnly       bool          `json:"readOnly"`
	Input          DispatchInput `json:"input"`
	ExpectedOutput string        `json:"expectedOutput"`
	Boundary       []string      `json:"boundary"`
}

type DispatchSubmissionTemplate struct {
	Outcome              string   `json:"outcome"`
	JSON                 string   `json:"json"`
	RequiredWrites       []string `json:"requiredWrites"`
	RequiredReplacements []string `json:"requiredReplacements,omitempty"`
}

type DispatchReturn struct {
	SubmissionPath    string                       `json:"submissionPath"`
	SubmissionOutputs string                       `json:"submissionOutputs,omitempty"`
	SubmissionResult  string                       `json:"submissionResult,omitempty"`
	SubmissionLast    bool                         `json:"submissionLast"`
	Templates         []DispatchSubmissionTemplate `json:"templates"`
	Boundary          []string                     `json:"boundary"`
}

type DispatchTicket struct {
	SchemaVersion        int            `json:"schemaVersion"`
	Kind                 string         `json:"kind"`
	JobID                string         `json:"jobId"`
	JobSHA256            string         `json:"jobSha256"`
	CheckpointSHA256     string         `json:"checkpointSha256"`
	SessionKind          string         `json:"sessionKind"`
	Attempt              Attempt        `json:"attempt"`
	AttemptPath          string         `json:"attemptPath"`
	AttemptSHA256        string         `json:"attemptSha256"`
	Launch               DispatchLaunch `json:"launch"`
	Return               DispatchReturn `json:"return"`
	RefreshStatusCommand string         `json:"refreshStatusCommand"`
	NoSessionManagement  bool           `json:"noSessionManagement"`
	NoHeavyTool          bool           `json:"noHeavyTool"`
	NoAuthority          bool           `json:"noAuthority"`
	NoConfirmed          bool           `json:"noConfirmed"`
	Boundary             []string       `json:"boundary"`
}

type DispatchClaim struct {
	SchemaVersion       int    `json:"schemaVersion"`
	Kind                string `json:"kind"`
	DispatchSHA256      string `json:"dispatchSha256"`
	JobID               string `json:"jobId"`
	JobSHA256           string `json:"jobSha256"`
	CheckpointSHA256    string `json:"checkpointSha256"`
	AttemptID           string `json:"attemptId"`
	AttemptSHA256       string `json:"attemptSha256"`
	Generation          int    `json:"generation"`
	Harness             string `json:"harness"`
	Session             string `json:"session"`
	Actor               string `json:"actor"`
	ClaimedAt           string `json:"claimedAt"`
	NoSessionManagement bool   `json:"noSessionManagement"`
	NoHeavyTool         bool   `json:"noHeavyTool"`
	NoAuthority         bool   `json:"noAuthority"`
	NoConfirmed         bool   `json:"noConfirmed"`
}

type LaunchReceipt struct {
	SchemaVersion       int    `json:"schemaVersion"`
	Kind                string `json:"kind"`
	DispatchSHA256      string `json:"dispatchSha256"`
	ClaimSHA256         string `json:"claimSha256"`
	JobID               string `json:"jobId"`
	JobSHA256           string `json:"jobSha256"`
	CheckpointSHA256    string `json:"checkpointSha256"`
	AttemptID           string `json:"attemptId"`
	AttemptSHA256       string `json:"attemptSha256"`
	Generation          int    `json:"generation"`
	Harness             string `json:"harness"`
	Session             string `json:"session"`
	ActualHarness       string `json:"actualHarness,omitempty"`
	ActualSession       string `json:"actualSession,omitempty"`
	Actor               string `json:"actor"`
	Outcome             string `json:"outcome"`
	ObservedAt          string `json:"observedAt"`
	Reason              string `json:"reason,omitempty"`
	NoSessionManagement bool   `json:"noSessionManagement"`
	NoHeavyTool         bool   `json:"noHeavyTool"`
	NoAuthority         bool   `json:"noAuthority"`
	NoConfirmed         bool   `json:"noConfirmed"`
}

type DispatchInspection struct {
	State        string          `json:"state"`
	TicketPath   string          `json:"ticketPath"`
	TicketSHA256 string          `json:"ticketSha256,omitempty"`
	Ticket       *DispatchTicket `json:"ticket,omitempty"`
	ClaimPath    string          `json:"claimPath"`
	ClaimSHA256  string          `json:"claimSha256,omitempty"`
	Claim        *DispatchClaim  `json:"claim,omitempty"`
	LaunchPath   string          `json:"launchPath"`
	LaunchSHA256 string          `json:"launchSha256,omitempty"`
	Launch       *LaunchReceipt  `json:"launch,omitempty"`
	Warnings     []string        `json:"warnings,omitempty"`
}

type DispatchPlan struct {
	SchemaVersion        int                `json:"schemaVersion"`
	Mode                 string             `json:"mode"`
	Outcome              string             `json:"outcome"`
	Actor                string             `json:"actor"`
	ObservedAt           string             `json:"observedAt"`
	ActualHarness        string             `json:"actualHarness,omitempty"`
	ActualSession        string             `json:"actualSession,omitempty"`
	Reason               string             `json:"reason,omitempty"`
	Job                  Job                `json:"job"`
	JobSHA256            string             `json:"jobSha256"`
	AttemptSHA256        string             `json:"attemptSha256"`
	DispatchSHA256       string             `json:"dispatchSha256"`
	ExpectedClaimSHA256  string             `json:"expectedClaimSha256,omitempty"`
	ArtifactPath         string             `json:"artifactPath"`
	ArtifactSHA256       string             `json:"artifactSha256"`
	ExpectedPlanSHA256   string             `json:"expectedPlanSha256"`
	ApplyCommand         string             `json:"applyCommand,omitempty"`
	ReviewRequired       bool               `json:"reviewRequired"`
	RequiresConfirmation bool               `json:"requiresConfirmation"`
	Applied              bool               `json:"applied"`
	AlreadyApplied       bool               `json:"alreadyApplied"`
	Inspection           DispatchInspection `json:"inspection"`
	Boundary             []string           `json:"boundary"`
	data                 []byte
}

func BindAttemptDispatch(plan AttemptPlan, ticket DispatchTicket) (AttemptPlan, error) {
	ticket.SchemaVersion = SchemaVersion
	ticket.Kind = KindDispatchTicket
	ticket.JobID = plan.Job.JobID
	ticket.JobSHA256 = plan.JobSHA256
	ticket.CheckpointSHA256 = plan.Job.CheckpointSHA256
	ticket.SessionKind = plan.Job.SessionKind
	ticket.Attempt = plan.Attempt
	ticket.AttemptPath = plan.AttemptPath
	candidateAttemptData, err := canonical(attemptEnvelope{SchemaVersion: SchemaVersion, Kind: KindAttempt, Attempt: plan.Attempt})
	if err != nil {
		return AttemptPlan{}, err
	}
	ticket.AttemptSHA256 = hash(candidateAttemptData)
	ticket.NoSessionManagement = true
	ticket.NoHeavyTool = true
	ticket.NoAuthority = true
	ticket.NoConfirmed = true
	if err := validateDispatchTicket(plan.Job, plan.Attempt, plan.AttemptPath, ticket.AttemptSHA256, ticket); err != nil {
		return AttemptPlan{}, err
	}
	data, err := canonical(ticket)
	if err != nil {
		return AttemptPlan{}, err
	}
	plan.Dispatch = &ticket
	plan.DispatchPath, err = dispatchTicketPath(plan.Job.CaseRoot, plan.Job.JobID, plan.Attempt.Generation)
	if err != nil {
		return AttemptPlan{}, err
	}
	plan.DispatchSHA256 = hash(data)
	plan.dispatchData = data
	plan.data, err = canonical(attemptEnvelope{SchemaVersion: SchemaVersion, Kind: KindAttempt, Attempt: plan.Attempt, DispatchSHA256: plan.DispatchSHA256})
	if err != nil {
		return AttemptPlan{}, err
	}
	plan.AttemptSHA256 = hash(plan.data)
	identity, err := json.Marshal(struct {
		JobSHA256      string `json:"jobSha256"`
		PreviousSHA256 string `json:"previousSha256,omitempty"`
		AttemptPath    string `json:"attemptPath"`
		AttemptSHA256  string `json:"attemptSha256"`
		DispatchPath   string `json:"dispatchPath"`
		DispatchSHA256 string `json:"dispatchSha256"`
	}{plan.JobSHA256, plan.Attempt.SupersedesSHA256, plan.AttemptPath, plan.AttemptSHA256, plan.DispatchPath, plan.DispatchSHA256})
	if err != nil {
		return AttemptPlan{}, err
	}
	plan.ExpectedPlanSHA256 = hash(identity)
	return plan, nil
}

func InspectDispatch(job Job, attempt AttemptInspection) (DispatchInspection, error) {
	committed, err := InspectCurrentDispatch(job, attempt)
	if err != nil {
		return DispatchInspection{}, err
	}
	pendingGeneration := attempt.Generations + 1
	pendingPath, err := dispatchTicketPath(job.CaseRoot, job.JobID, pendingGeneration)
	if err != nil {
		return DispatchInspection{}, err
	}
	pendingData, err := readOptionalDispatch(job.CaseRoot, pendingPath, "external session pending dispatch ticket")
	if err != nil {
		return DispatchInspection{}, err
	}
	if pendingData != nil {
		return inspectPendingDispatch(job, attempt, pendingGeneration, pendingPath, pendingData)
	}
	return committed, nil
}

func InspectCurrentDispatch(job Job, attempt AttemptInspection) (DispatchInspection, error) {
	if attempt.Current == nil {
		return DispatchInspection{State: "absent"}, nil
	}
	ticketPath, err := dispatchTicketPath(job.CaseRoot, job.JobID, attempt.Current.Generation)
	if err != nil {
		return DispatchInspection{}, err
	}
	claimPath, err := dispatchClaimPath(job.CaseRoot, job.JobID, attempt.Current.Generation)
	if err != nil {
		return DispatchInspection{}, err
	}
	launchPath, err := launchReceiptPath(job.CaseRoot, job.JobID, attempt.Current.Generation)
	if err != nil {
		return DispatchInspection{}, err
	}
	out := DispatchInspection{State: "missing", TicketPath: ticketPath, ClaimPath: claimPath, LaunchPath: launchPath}
	ticketData, err := readOptionalDispatch(job.CaseRoot, out.TicketPath, "external session dispatch ticket")
	if err != nil {
		return out, err
	}
	if ticketData == nil {
		if job.DispatchRequired || attempt.DispatchSHA256 != "" {
			return out, fmt.Errorf("external session committed attempt is missing its bound dispatch ticket")
		}
		return out, nil
	}
	var ticket DispatchTicket
	if err := decodeCanonicalDispatch(ticketData, &ticket); err != nil {
		return out, fmt.Errorf("invalid external session dispatch ticket: %w", err)
	}
	candidateAttemptData, candidateErr := canonical(attemptEnvelope{SchemaVersion: SchemaVersion, Kind: KindAttempt, Attempt: *attempt.Current})
	if candidateErr != nil || validateDispatchTicket(job, *attempt.Current, attempt.Path, hash(candidateAttemptData), ticket) != nil {
		return out, fmt.Errorf("invalid external session dispatch ticket")
	}
	out.State, out.Ticket, out.TicketSHA256 = "queued", &ticket, hash(ticketData)
	if attempt.DispatchSHA256 == "" || !strings.EqualFold(attempt.DispatchSHA256, out.TicketSHA256) {
		return out, fmt.Errorf("external session attempt does not bind the exact dispatch ticket")
	}
	claimData, err := readOptionalDispatch(job.CaseRoot, out.ClaimPath, "external session dispatch claim")
	if err != nil {
		return out, err
	}
	if claimData == nil {
		return out, nil
	}
	var claim DispatchClaim
	if err := decodeCanonicalDispatch(claimData, &claim); err != nil || validateDispatchClaim(ticket, out.TicketSHA256, attempt.AttemptSHA256, claim) != nil {
		return out, fmt.Errorf("invalid external session dispatch claim")
	}
	out.State, out.Claim, out.ClaimSHA256 = "claimed", &claim, hash(claimData)
	launchData, err := readOptionalDispatch(job.CaseRoot, out.LaunchPath, "external session launch receipt")
	if err != nil {
		return out, err
	}
	if launchData == nil {
		return out, nil
	}
	var launch LaunchReceipt
	if err := decodeCanonicalDispatch(launchData, &launch); err != nil || validateLaunchReceipt(ticket, out.TicketSHA256, attempt.AttemptSHA256, claim, out.ClaimSHA256, launch) != nil {
		return out, fmt.Errorf("invalid external session launch receipt")
	}
	out.Launch, out.LaunchSHA256 = &launch, hash(launchData)
	if launch.Outcome == "accepted" {
		out.State = "running"
	} else {
		out.State = "launch-failed"
	}
	return out, nil
}

func inspectPendingDispatch(job Job, attempt AttemptInspection, generation int, path string, data []byte) (DispatchInspection, error) {
	var ticket DispatchTicket
	if err := decodeCanonicalDispatch(data, &ticket); err != nil {
		return DispatchInspection{}, fmt.Errorf("invalid pending external session dispatch ticket: %w", err)
	}
	expectedAttemptPath, err := projectstate.Rel(job.CaseRoot, "external-session-attempts", job.JobID, fmt.Sprintf("%06d.json", generation))
	if err != nil {
		return DispatchInspection{}, err
	}
	if ticket.JobID != job.JobID || ticket.CheckpointSHA256 != job.CheckpointSHA256 || ticket.Attempt.Generation != generation || ticket.Attempt.SupersedesSHA256 != attempt.AttemptSHA256 || ticket.AttemptPath != expectedAttemptPath {
		return DispatchInspection{}, fmt.Errorf("pending external session dispatch ticket does not bind the next exact generation")
	}
	attemptBytes, err := canonical(attemptEnvelope{SchemaVersion: SchemaVersion, Kind: KindAttempt, Attempt: ticket.Attempt})
	if err != nil || !strings.EqualFold(hash(attemptBytes), ticket.AttemptSHA256) {
		return DispatchInspection{}, fmt.Errorf("pending external session dispatch ticket attempt identity is invalid")
	}
	jobSHA, err := jobSHA256(job)
	if err != nil || !strings.EqualFold(ticket.JobSHA256, jobSHA) || !strings.EqualFold(ticket.Attempt.JobSHA256, jobSHA) {
		return DispatchInspection{}, fmt.Errorf("pending external session dispatch ticket job identity is invalid")
	}
	if err := validateDispatchTicket(job, ticket.Attempt, ticket.AttemptPath, ticket.AttemptSHA256, ticket); err != nil {
		return DispatchInspection{}, err
	}
	claimPath, err := dispatchClaimPath(job.CaseRoot, job.JobID, generation)
	if err != nil {
		return DispatchInspection{}, err
	}
	launchPath, err := launchReceiptPath(job.CaseRoot, job.JobID, generation)
	if err != nil {
		return DispatchInspection{}, err
	}
	return DispatchInspection{
		State: "attempt-publication-pending", TicketPath: path, TicketSHA256: hash(data), Ticket: &ticket,
		ClaimPath: claimPath, LaunchPath: launchPath,
	}, nil
}

func PendingAttemptPlan(job Job, inspection DispatchInspection) (AttemptPlan, error) {
	if inspection.State != "attempt-publication-pending" || inspection.Ticket == nil {
		return AttemptPlan{}, fmt.Errorf("external session dispatch has no pending attempt publication")
	}
	candidateAttemptBytes, err := canonical(attemptEnvelope{SchemaVersion: SchemaVersion, Kind: KindAttempt, Attempt: inspection.Ticket.Attempt})
	if err != nil || !strings.EqualFold(hash(candidateAttemptBytes), inspection.Ticket.AttemptSHA256) {
		return AttemptPlan{}, fmt.Errorf("pending external session dispatch ticket attempt identity is invalid")
	}
	attemptBytes, err := canonical(attemptEnvelope{SchemaVersion: SchemaVersion, Kind: KindAttempt, Attempt: inspection.Ticket.Attempt, DispatchSHA256: inspection.TicketSHA256})
	if err != nil {
		return AttemptPlan{}, err
	}
	plan := AttemptPlan{
		SchemaVersion: SchemaVersion, Mode: "external-session-attempt", Job: job,
		JobSHA256: inspection.Ticket.JobSHA256, Attempt: inspection.Ticket.Attempt,
		AttemptPath: inspection.Ticket.AttemptPath, AttemptSHA256: hash(attemptBytes),
		ReviewRequired: true, RequiresConfirmation: true, data: attemptBytes,
	}
	return BindAttemptDispatch(plan, *inspection.Ticket)
}

func PreviewDispatchTransition(job Job, attempt AttemptInspection, outcome, actor, observedAt, actualHarness, actualSession, reason string) (DispatchPlan, error) {
	inspection, err := InspectDispatch(job, attempt)
	if err != nil {
		return DispatchPlan{}, err
	}
	if attempt.Current == nil || inspection.Ticket == nil {
		return DispatchPlan{}, fmt.Errorf("external session dispatch requires the current immutable ticket")
	}
	if err := validateDispatchInput(job, *inspection.Ticket); err != nil {
		return DispatchPlan{}, err
	}
	outcome = strings.ToLower(strings.TrimSpace(outcome))
	actor = strings.TrimSpace(actor)
	observedAt = strings.TrimSpace(observedAt)
	actualHarness = strings.TrimSpace(actualHarness)
	actualSession = strings.TrimSpace(actualSession)
	reason = strings.TrimSpace(reason)
	if !validSingleLine(actor) {
		return DispatchPlan{}, fmt.Errorf("external session dispatch actor must be a non-empty single line")
	}
	if parsed, err := time.Parse(time.RFC3339Nano, observedAt); err != nil || parsed.Format(time.RFC3339Nano) != observedAt {
		return DispatchPlan{}, fmt.Errorf("external session dispatch observedAt must be canonical RFC3339Nano")
	}
	plan := DispatchPlan{
		SchemaVersion: SchemaVersion, Mode: "external-session-dispatch", Outcome: outcome,
		Actor: actor, ObservedAt: observedAt, ActualHarness: actualHarness, ActualSession: actualSession, Reason: reason,
		Job: job, JobSHA256: inspection.Ticket.JobSHA256, AttemptSHA256: attempt.AttemptSHA256,
		DispatchSHA256: inspection.TicketSHA256, Inspection: inspection,
		ReviewRequired: true, RequiresConfirmation: true,
		Boundary: []string{
			"dispatch transition records only durable claim or launch truth; the external harness owns actual session lifecycle",
			"every transition binds the exact current job, attempt generation, ticket, claim, harness, and session",
			"dispatch transition does not execute heavy tools or write authority/confirmed state",
		},
	}
	switch outcome {
	case "claimed":
		claim := DispatchClaim{
			SchemaVersion: SchemaVersion, Kind: KindDispatchClaim, DispatchSHA256: inspection.TicketSHA256,
			JobID: job.JobID, JobSHA256: inspection.Ticket.JobSHA256, CheckpointSHA256: job.CheckpointSHA256,
			AttemptID: attempt.Current.AttemptID, AttemptSHA256: attempt.AttemptSHA256, Generation: attempt.Current.Generation,
			Harness: attempt.Current.Harness, Session: attempt.Current.Session, Actor: actor, ClaimedAt: observedAt,
			NoSessionManagement: true, NoHeavyTool: true, NoAuthority: true, NoConfirmed: true,
		}
		if err := validateDispatchClaim(*inspection.Ticket, inspection.TicketSHA256, attempt.AttemptSHA256, claim); err != nil {
			return DispatchPlan{}, err
		}
		plan.ArtifactPath = inspection.ClaimPath
		plan.data, err = canonical(claim)
	case "accepted", "failed":
		if inspection.Claim == nil {
			return DispatchPlan{}, fmt.Errorf("external session launch receipt requires the exact durable claim")
		}
		if outcome == "accepted" && (reason != "" || !validSingleLine(actualHarness) || !validSingleLine(actualSession)) {
			return DispatchPlan{}, fmt.Errorf("accepted external session launch requires actual harness/session and cannot include a reason")
		}
		if outcome == "failed" && (reason == "" || actualHarness != "" || actualSession != "") {
			return DispatchPlan{}, fmt.Errorf("failed external session launch requires a reason and cannot claim an actual session")
		}
		if IsRemoteControlAttempt(attempt.Current) {
			transport, transportErr := InspectTransport(job, attempt, inspection)
			if transportErr != nil {
				return DispatchPlan{}, transportErr
			}
			if err := ValidateRemoteControlLaunchTransition(transport, outcome, actor, observedAt, actualHarness, actualSession, reason); err != nil {
				return DispatchPlan{}, err
			}
		}
		launch := LaunchReceipt{
			SchemaVersion: SchemaVersion, Kind: KindLaunchReceipt, DispatchSHA256: inspection.TicketSHA256, ClaimSHA256: inspection.ClaimSHA256,
			JobID: job.JobID, JobSHA256: inspection.Ticket.JobSHA256, CheckpointSHA256: job.CheckpointSHA256,
			AttemptID: attempt.Current.AttemptID, AttemptSHA256: attempt.AttemptSHA256, Generation: attempt.Current.Generation,
			Harness: attempt.Current.Harness, Session: attempt.Current.Session,
			ActualHarness: actualHarness, ActualSession: actualSession, Actor: actor,
			Outcome: outcome, ObservedAt: observedAt, Reason: reason,
			NoSessionManagement: true, NoHeavyTool: true, NoAuthority: true, NoConfirmed: true,
		}
		if err := validateLaunchReceipt(*inspection.Ticket, inspection.TicketSHA256, attempt.AttemptSHA256, *inspection.Claim, inspection.ClaimSHA256, launch); err != nil {
			return DispatchPlan{}, err
		}
		plan.ExpectedClaimSHA256 = inspection.ClaimSHA256
		plan.ArtifactPath = inspection.LaunchPath
		plan.data, err = canonical(launch)
	default:
		return DispatchPlan{}, fmt.Errorf("external session dispatch outcome must be claimed, accepted, or failed")
	}
	if err != nil {
		return DispatchPlan{}, err
	}
	plannedSHA := hash(plan.data)
	if outcome == "claimed" && inspection.Claim != nil && !strings.EqualFold(inspection.ClaimSHA256, plannedSHA) {
		return DispatchPlan{}, fmt.Errorf("external session dispatch is already claimed by a different immutable receipt")
	}
	if outcome != "claimed" && inspection.Launch != nil && !strings.EqualFold(inspection.LaunchSHA256, plannedSHA) {
		return DispatchPlan{}, fmt.Errorf("external session launch already has a different immutable receipt")
	}
	plan.ArtifactSHA256 = plannedSHA
	identity, err := json.Marshal(struct {
		JobSHA256           string `json:"jobSha256"`
		AttemptSHA256       string `json:"attemptSha256"`
		DispatchSHA256      string `json:"dispatchSha256"`
		ExpectedClaimSHA256 string `json:"expectedClaimSha256,omitempty"`
		Outcome             string `json:"outcome"`
		ArtifactPath        string `json:"artifactPath"`
		ArtifactSHA256      string `json:"artifactSha256"`
	}{plan.JobSHA256, plan.AttemptSHA256, plan.DispatchSHA256, plan.ExpectedClaimSHA256, plan.Outcome, plan.ArtifactPath, plan.ArtifactSHA256})
	if err != nil {
		return DispatchPlan{}, err
	}
	plan.ExpectedPlanSHA256 = hash(identity)
	return plan, nil
}

func ApplyDispatchTransitionCurrent(plan DispatchPlan, expectedJobSHA256, expectedDispatchSHA256, expectedClaimSHA256, expectedPlanSHA256 string, current func() (Job, error)) (_ DispatchPlan, retErr error) {
	if !strings.EqualFold(strings.TrimSpace(expectedJobSHA256), plan.JobSHA256) || !strings.EqualFold(strings.TrimSpace(expectedDispatchSHA256), plan.DispatchSHA256) || !strings.EqualFold(strings.TrimSpace(expectedClaimSHA256), plan.ExpectedClaimSHA256) || !strings.EqualFold(strings.TrimSpace(expectedPlanSHA256), plan.ExpectedPlanSHA256) {
		return DispatchPlan{}, fmt.Errorf("external session dispatch identity or plan sha256 mismatch; rerun WhatIf")
	}
	lease, err := lanemutation.AcquireProject(plan.Job.CaseRoot)
	if err != nil {
		return DispatchPlan{}, err
	}
	defer func() { retErr = errorsJoin(retErr, lease.Unlock()) }()
	if current != nil {
		live, err := current()
		if err != nil {
			return DispatchPlan{}, err
		}
		liveSHA, err := jobSHA256(live)
		if err != nil || !strings.EqualFold(liveSHA, plan.JobSHA256) {
			return DispatchPlan{}, fmt.Errorf("external session dispatch job is no longer current")
		}
	}
	attempt, err := InspectAttempt(plan.Job)
	if err != nil || attempt.Current == nil || !strings.EqualFold(attempt.AttemptSHA256, plan.AttemptSHA256) {
		return DispatchPlan{}, fmt.Errorf("external session dispatch attempt is no longer current")
	}
	actor, actualHarness, actualSession := dispatchPlanActors(plan)
	fresh, err := PreviewDispatchTransition(plan.Job, attempt, plan.Outcome, actor, dispatchPlanObservedAt(plan), actualHarness, actualSession, dispatchPlanReason(plan))
	if err != nil {
		return DispatchPlan{}, err
	}
	if fresh.ExpectedPlanSHA256 != plan.ExpectedPlanSHA256 || !bytes.Equal(fresh.data, plan.data) {
		return DispatchPlan{}, fmt.Errorf("external session dispatch changed after preview; rerun WhatIf")
	}
	alreadyCommitted := (plan.Outcome == "claimed" && fresh.Inspection.Claim != nil) ||
		(plan.Outcome != "claimed" && fresh.Inspection.Launch != nil)
	if !alreadyCommitted && attempt.Current.LaunchControl != nil {
		if err := executioncontrol.RequireCurrentBindingWithProjectLease(
			plan.Job.CaseRoot,
			lease,
			*attempt.Current.LaunchControl,
		); err != nil {
			return DispatchPlan{}, err
		}
	}
	replayed, err := rekitfs.WriteExclusiveRegularFileAnchored(plan.Job.CaseRoot, plan.ArtifactPath, "external session dispatch transition", plan.data)
	if err != nil {
		return DispatchPlan{}, err
	}
	plan.Applied, plan.AlreadyApplied, plan.ReviewRequired, plan.RequiresConfirmation = true, replayed, false, false
	plan.data = nil
	return plan, nil
}

func dispatchPlanActors(plan DispatchPlan) (string, string, string) {
	if plan.Outcome == "claimed" {
		var claim DispatchClaim
		_ = decodeCanonicalDispatch(plan.data, &claim)
		return claim.Actor, "", ""
	}
	var launch LaunchReceipt
	_ = decodeCanonicalDispatch(plan.data, &launch)
	return launch.Actor, launch.ActualHarness, launch.ActualSession
}

func dispatchPlanObservedAt(plan DispatchPlan) string {
	if plan.Outcome == "claimed" {
		var claim DispatchClaim
		_ = decodeCanonicalDispatch(plan.data, &claim)
		return claim.ClaimedAt
	}
	var launch LaunchReceipt
	_ = decodeCanonicalDispatch(plan.data, &launch)
	return launch.ObservedAt
}

func dispatchPlanReason(plan DispatchPlan) string {
	if plan.Outcome == "claimed" {
		return ""
	}
	var launch LaunchReceipt
	_ = decodeCanonicalDispatch(plan.data, &launch)
	return launch.Reason
}

func equalAttempt(left, right Attempt) bool {
	return left.SchemaVersion == right.SchemaVersion &&
		left.Kind == right.Kind &&
		left.AttemptID == right.AttemptID &&
		left.JobID == right.JobID &&
		strings.EqualFold(left.JobSHA256, right.JobSHA256) &&
		left.CheckpointSHA256 == right.CheckpointSHA256 &&
		left.SessionKind == right.SessionKind &&
		left.Generation == right.Generation &&
		left.Harness == right.Harness &&
		left.Session == right.Session &&
		left.Actor == right.Actor &&
		left.StartedAt == right.StartedAt &&
		strings.EqualFold(left.SupersedesSHA256, right.SupersedesSHA256) &&
		left.SubmissionPath == right.SubmissionPath &&
		left.SubmissionOutputs == right.SubmissionOutputs &&
		left.SubmissionResult == right.SubmissionResult &&
		sameAttemptLaunchControl(left.LaunchControl, right.LaunchControl) &&
		slices.Equal(left.AllowedOutcomes, right.AllowedOutcomes) &&
		left.NoSessionManagement == right.NoSessionManagement &&
		left.NoHeavyTool == right.NoHeavyTool &&
		left.NoAuthority == right.NoAuthority &&
		left.NoConfirmed == right.NoConfirmed
}

func validateDispatchInput(job Job, ticket DispatchTicket) error {
	path := ticket.Launch.Input.Path
	var err error
	if filepath.IsAbs(path) {
		root, rootErr := filepath.Abs(job.CaseRoot)
		input, inputErr := filepath.Abs(path)
		if rootErr != nil || inputErr != nil {
			return fmt.Errorf("external session dispatch input path is invalid")
		}
		rel, relErr := filepath.Rel(root, input)
		if relErr != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("external session dispatch input path escapes the case root")
		}
		path = input
	} else {
		path, err = rekitfs.SafeJoin(job.CaseRoot, path)
		if err != nil {
			return fmt.Errorf("external session dispatch input path is invalid: %w", err)
		}
	}
	data, err := rekitfs.ReadStableRegularFileAnchored(job.CaseRoot, path, "external session dispatch input", maxDispatchBytes)
	if err != nil {
		return err
	}
	if !strings.EqualFold(hash(data), ticket.Launch.Input.SHA256) {
		return fmt.Errorf("external session dispatch input no longer matches its immutable sha256")
	}
	return nil
}

func validateDispatchTicket(job Job, attempt Attempt, attemptPath, attemptSHA string, ticket DispatchTicket) error {
	if ticket.SchemaVersion != SchemaVersion || ticket.Kind != KindDispatchTicket || ticket.JobID != job.JobID || !strings.EqualFold(ticket.JobSHA256, attempt.JobSHA256) || ticket.CheckpointSHA256 != job.CheckpointSHA256 || ticket.SessionKind != job.SessionKind || !equalAttempt(ticket.Attempt, attempt) || ticket.AttemptPath != attemptPath || !strings.EqualFold(ticket.AttemptSHA256, attemptSHA) || !ticket.Launch.Ready || ticket.Launch.Input.Path == "" || !isSHA(ticket.Launch.Input.SHA256) || ticket.Return.SubmissionPath != attempt.SubmissionPath || ticket.Return.SubmissionOutputs != attempt.SubmissionOutputs || ticket.Return.SubmissionResult != attempt.SubmissionResult || !ticket.Return.SubmissionLast || len(ticket.Return.Templates) != len(attempt.AllowedOutcomes) || ticket.RefreshStatusCommand == "" || !ticket.NoSessionManagement || !ticket.NoHeavyTool || !ticket.NoAuthority || !ticket.NoConfirmed {
		return fmt.Errorf("external session dispatch ticket contract is invalid")
	}
	for idx, template := range ticket.Return.Templates {
		if template.Outcome != attempt.AllowedOutcomes[idx] || strings.TrimSpace(template.JSON) == "" || len(template.RequiredWrites) == 0 {
			return fmt.Errorf("external session dispatch return template is invalid")
		}
	}
	return nil
}

func validateDispatchClaim(ticket DispatchTicket, ticketSHA, attemptReceiptSHA string, claim DispatchClaim) error {
	attempt := ticket.Attempt
	if claim.SchemaVersion != SchemaVersion || claim.Kind != KindDispatchClaim || !strings.EqualFold(claim.DispatchSHA256, ticketSHA) || claim.JobID != ticket.JobID || !strings.EqualFold(claim.JobSHA256, ticket.JobSHA256) || claim.CheckpointSHA256 != ticket.CheckpointSHA256 || claim.AttemptID != attempt.AttemptID || !strings.EqualFold(claim.AttemptSHA256, attemptReceiptSHA) || claim.Generation != attempt.Generation || claim.Harness != attempt.Harness || claim.Session != attempt.Session || !validSingleLine(claim.Actor) || !claim.NoSessionManagement || !claim.NoHeavyTool || !claim.NoAuthority || !claim.NoConfirmed {
		return fmt.Errorf("external session dispatch claim contract is invalid")
	}
	parsed, err := time.Parse(time.RFC3339Nano, claim.ClaimedAt)
	if err != nil || parsed.Format(time.RFC3339Nano) != claim.ClaimedAt {
		return fmt.Errorf("external session dispatch claim time is invalid")
	}
	return nil
}

func validateLaunchReceipt(ticket DispatchTicket, ticketSHA, attemptReceiptSHA string, claim DispatchClaim, claimSHA string, launch LaunchReceipt) error {
	attempt := ticket.Attempt
	if claim.Harness != attempt.Harness || claim.Session != attempt.Session || !strings.EqualFold(claim.AttemptSHA256, attemptReceiptSHA) {
		return fmt.Errorf("external session launch claim owner is invalid")
	}
	if launch.SchemaVersion != SchemaVersion || launch.Kind != KindLaunchReceipt || !strings.EqualFold(launch.DispatchSHA256, ticketSHA) || !strings.EqualFold(launch.ClaimSHA256, claimSHA) || launch.JobID != ticket.JobID || !strings.EqualFold(launch.JobSHA256, ticket.JobSHA256) || launch.CheckpointSHA256 != ticket.CheckpointSHA256 || launch.AttemptID != attempt.AttemptID || !strings.EqualFold(launch.AttemptSHA256, attemptReceiptSHA) || launch.Generation != attempt.Generation || launch.Harness != attempt.Harness || launch.Session != attempt.Session || launch.Actor != claim.Actor || !validSingleLine(launch.Actor) || !launch.NoSessionManagement || !launch.NoHeavyTool || !launch.NoAuthority || !launch.NoConfirmed {
		return fmt.Errorf("external session launch receipt contract is invalid")
	}
	parsed, err := time.Parse(time.RFC3339Nano, launch.ObservedAt)
	if err != nil || parsed.Format(time.RFC3339Nano) != launch.ObservedAt {
		return fmt.Errorf("external session launch receipt time is invalid")
	}
	if launch.Outcome == "accepted" && launch.Reason == "" && validSingleLine(launch.ActualHarness) && validSingleLine(launch.ActualSession) {
		return nil
	}
	if launch.Outcome == "failed" && strings.TrimSpace(launch.Reason) != "" && launch.ActualHarness == "" && launch.ActualSession == "" {
		return nil
	}
	return fmt.Errorf("external session launch receipt outcome or reason is invalid")
}

func dispatchTicketPath(caseRoot, jobID string, generation int) (string, error) {
	return projectstate.Rel(caseRoot, "external-session-dispatch", "inbox", jobID, fmt.Sprintf("%06d.json", generation))
}

func dispatchClaimPath(caseRoot, jobID string, generation int) (string, error) {
	return projectstate.Rel(caseRoot, "external-session-dispatch", "claims", jobID, fmt.Sprintf("%06d.json", generation))
}

func launchReceiptPath(caseRoot, jobID string, generation int) (string, error) {
	return projectstate.Rel(caseRoot, "external-session-dispatch", "launch-receipts", jobID, fmt.Sprintf("%06d.json", generation))
}

func readOptionalDispatch(caseRoot, rel, label string) ([]byte, error) {
	path, err := rekitfs.SafeJoin(caseRoot, rel)
	if err != nil {
		return nil, err
	}
	data, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, path, label, maxDispatchBytes)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}

func decodeCanonicalDispatch(data []byte, value any) error {
	if len(data) == 0 || len(data) > maxDispatchBytes {
		return fmt.Errorf("external session dispatch artifact must be bounded non-empty JSON")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(value); err != nil {
		return err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("external session dispatch artifact must contain exactly one JSON object")
	}
	canonicalData, err := canonical(value)
	if err != nil || !bytes.Equal(canonicalData, data) {
		return fmt.Errorf("external session dispatch artifact is not canonical")
	}
	return nil
}

func isSHA(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if !strings.ContainsRune("0123456789abcdefABCDEF", char) {
			return false
		}
	}
	return true
}
