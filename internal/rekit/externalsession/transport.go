package externalsession

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/capabilitycontract"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

const (
	RemoteControlHarness          = "claude-code-remote-control"
	KindTransportEndpointSnapshot = "current-loop-external-session-transport-endpoint"
	KindTransportDelivery         = "current-loop-external-session-transport-delivery"
	TransportRemoteControl        = "claude-code-remote-control"
	maxTransportArtifactBytes     = 128 * 1024
	maxTransportMessageBytes      = 64 * 1024
)

type TransportBinding struct {
	Transport          string `json:"transport"`
	TransportBindingID string `json:"transportBindingId"`
	JobID              string `json:"jobId"`
	JobSHA256          string `json:"jobSha256"`
	CheckpointSHA256   string `json:"checkpointSha256"`
	AttemptID          string `json:"attemptId"`
	AttemptSHA256      string `json:"attemptSha256"`
	Generation         int    `json:"generation"`
	DispatchSHA256     string `json:"dispatchSha256"`
	ClaimSHA256        string `json:"claimSha256"`
}

type TransportMessageEnvelope struct {
	Capability        capabilitycontract.Binding `json:"capability"`
	Operation         string                     `json:"operation"`
	Recipient         string                     `json:"recipient"`
	SourceInputRole   string                     `json:"sourceInputRole"`
	SourceInputSHA256 string                     `json:"sourceInputSha256"`
	SourceInputBytes  int                        `json:"sourceInputBytes"`
	BundlePath        string                     `json:"bundlePath"`
	BundleSHA256      string                     `json:"bundleSha256"`
	BundleBytes       int                        `json:"bundleBytes"`
	Message           string                     `json:"message"`
	MessageSHA256     string                     `json:"messageSha256"`
	MessageBytes      int                        `json:"messageBytes"`
	ExpectedReply     string                     `json:"expectedReply"`
	NoFileTransfer    bool                       `json:"noFileTransfer"`
}

type TransportEndpointSnapshot struct {
	SchemaVersion       int                        `json:"schemaVersion"`
	Kind                string                     `json:"kind"`
	Binding             TransportBinding           `json:"binding"`
	DiscoveryTool       string                     `json:"discoveryTool"`
	Endpoint            string                     `json:"endpoint"`
	Actor               string                     `json:"actor"`
	ObservedAt          string                     `json:"observedAt"`
	BundlePath          string                     `json:"bundlePath"`
	BundleSHA256        string                     `json:"bundleSha256"`
	BundleBytes         int                        `json:"bundleBytes"`
	Envelope            TransportMessageEnvelope   `json:"envelope"`
	Capability          capabilitycontract.Binding `json:"capability"`
	NoSessionManagement bool                       `json:"noSessionManagement"`
	NoAutomaticRetry    bool                       `json:"noAutomaticRetry"`
	NoHeavyTool         bool                       `json:"noHeavyTool"`
	NoAuthority         bool                       `json:"noAuthority"`
	NoConfirmed         bool                       `json:"noConfirmed"`
}

type TransportDeliveryObservation struct {
	SchemaVersion          int                        `json:"schemaVersion"`
	Kind                   string                     `json:"kind"`
	Binding                TransportBinding           `json:"binding"`
	EndpointSnapshotSHA256 string                     `json:"endpointSnapshotSha256"`
	EnvelopeSHA256         string                     `json:"envelopeSha256"`
	Operation              string                     `json:"operation"`
	Outcome                string                     `json:"outcome"`
	ProviderAckFingerprint string                     `json:"providerAckFingerprint,omitempty"`
	Actor                  string                     `json:"actor"`
	ObservedAt             string                     `json:"observedAt"`
	Reason                 string                     `json:"reason,omitempty"`
	Capability             capabilitycontract.Binding `json:"capability"`
	NoSessionManagement    bool                       `json:"noSessionManagement"`
	NoAutomaticRetry       bool                       `json:"noAutomaticRetry"`
	NoHeavyTool            bool                       `json:"noHeavyTool"`
	NoAuthority            bool                       `json:"noAuthority"`
	NoConfirmed            bool                       `json:"noConfirmed"`
}

type TransportInspection struct {
	Applicable     bool                          `json:"applicable"`
	State          string                        `json:"state"`
	Binding        TransportBinding              `json:"binding"`
	BundlePath     string                        `json:"bundlePath,omitempty"`
	BundleSHA256   string                        `json:"bundleSha256,omitempty"`
	BundleBytes    int                           `json:"bundleBytes,omitempty"`
	EndpointPath   string                        `json:"endpointPath,omitempty"`
	EndpointSHA256 string                        `json:"endpointSha256,omitempty"`
	Endpoint       *TransportEndpointSnapshot    `json:"endpoint,omitempty"`
	EnvelopeSHA256 string                        `json:"envelopeSha256,omitempty"`
	DeliveryPath   string                        `json:"deliveryPath,omitempty"`
	DeliverySHA256 string                        `json:"deliverySha256,omitempty"`
	Delivery       *TransportDeliveryObservation `json:"delivery,omitempty"`
	Warnings       []string                      `json:"warnings,omitempty"`
}

type TransportPlan struct {
	SchemaVersion        int                           `json:"schemaVersion"`
	Mode                 string                        `json:"mode"`
	Job                  Job                           `json:"job"`
	JobSHA256            string                        `json:"jobSha256"`
	AttemptSHA256        string                        `json:"attemptSha256"`
	DispatchSHA256       string                        `json:"dispatchSha256"`
	ClaimSHA256          string                        `json:"claimSha256"`
	ArtifactPath         string                        `json:"artifactPath"`
	ArtifactSHA256       string                        `json:"artifactSha256"`
	ExpectedPlanSHA256   string                        `json:"expectedPlanSha256"`
	BundlePath           string                        `json:"bundlePath,omitempty"`
	BundleSHA256         string                        `json:"bundleSha256,omitempty"`
	BundleBytes          int                           `json:"bundleBytes,omitempty"`
	Endpoint             *TransportEndpointSnapshot    `json:"endpoint,omitempty"`
	Delivery             *TransportDeliveryObservation `json:"delivery,omitempty"`
	ReviewRequired       bool                          `json:"reviewRequired"`
	RequiresConfirmation bool                          `json:"requiresConfirmation"`
	Applied              bool                          `json:"applied"`
	AlreadyApplied       bool                          `json:"alreadyApplied"`
	Boundary             []string                      `json:"boundary"`
	data                 []byte
	bundleData           []byte
}

func transportCapability() capabilitycontract.Binding {
	binding, err := capabilitycontract.Bind(capabilitycontract.Transport())
	if err != nil {
		panic(err)
	}
	return binding
}

func IsRemoteControlAttempt(attempt *Attempt) bool {
	return attempt != nil && attempt.Harness == RemoteControlHarness
}

func InspectTransport(job Job, attempt AttemptInspection, dispatch DispatchInspection) (TransportInspection, error) {
	if !IsRemoteControlAttempt(attempt.Current) {
		return TransportInspection{State: "not-applicable"}, nil
	}
	inspection := TransportInspection{Applicable: true, State: "dispatch-required"}
	if job.SessionKind != "reviewer" || job.Reviewer == nil || job.Reviewer.Harness != RemoteControlHarness || attempt.Current.Harness != job.Reviewer.Harness || attempt.Current.Session != job.Reviewer.Session {
		return inspection, fmt.Errorf("Remote Control transport requires an explicit matching durable reviewer dispatch binding")
	}
	if dispatch.Ticket == nil {
		return inspection, nil
	}
	if !dispatch.Ticket.Launch.ReadOnly {
		return inspection, fmt.Errorf("Remote Control transport requires a read-only dispatch ticket")
	}
	if dispatch.Claim == nil {
		inspection.State = "claim-required"
		return inspection, nil
	}
	binding, err := transportBinding(job, attempt, dispatch)
	if err != nil {
		return inspection, err
	}
	inspection.Binding = binding
	inspection.EndpointPath, err = transportEndpointPath(job.CaseRoot, job.JobID, attempt.Current.Generation)
	if err != nil {
		return inspection, err
	}
	inspection.DeliveryPath, err = transportDeliveryPath(job.CaseRoot, job.JobID, attempt.Current.Generation)
	if err != nil {
		return inspection, err
	}
	bundlePath, err := transportBundlePath(job.CaseRoot, job.JobID, attempt.Current.Generation)
	if err != nil {
		return inspection, err
	}
	bundleData, err := readOptionalTransport(job.CaseRoot, bundlePath, "external session transport evidence bundle")
	if err != nil {
		return inspection, err
	}
	inspection.BundlePath = bundlePath
	if bundleData != nil {
		inspection.BundleSHA256 = hash(bundleData)
		inspection.BundleBytes = len(bundleData)
	}
	endpointData, err := readOptionalTransport(job.CaseRoot, inspection.EndpointPath, "external session transport endpoint")
	if err != nil {
		return inspection, err
	}
	if endpointData == nil {
		inspection.State = "endpoint-required"
		if deliveryData, readErr := readOptionalTransport(job.CaseRoot, inspection.DeliveryPath, "external session transport delivery"); readErr != nil {
			return inspection, readErr
		} else if deliveryData != nil {
			return inspection, fmt.Errorf("external session transport delivery exists without its endpoint snapshot")
		}
		if bundleData != nil {
			if _, err := validateTransportEvidenceBundle(job, *dispatch.Ticket, binding, bundleData); err != nil {
				return inspection, fmt.Errorf("invalid pending external session transport evidence bundle: %w", err)
			}
		}
		return inspection, nil
	}
	if bundleData == nil {
		return inspection, fmt.Errorf("external session transport endpoint exists without its evidence bundle")
	}
	if _, err := validateTransportEvidenceBundle(job, *dispatch.Ticket, binding, bundleData); err != nil {
		return inspection, fmt.Errorf("invalid external session transport evidence bundle: %w", err)
	}
	var endpoint TransportEndpointSnapshot
	if err := decodeCanonicalTransport(endpointData, &endpoint); err != nil {
		return inspection, fmt.Errorf("invalid external session transport endpoint: %w", err)
	}
	if err := validateTransportEndpoint(job, attempt, dispatch, endpoint); err != nil {
		return inspection, err
	}
	inspection.State = "delivery-required"
	inspection.Endpoint = &endpoint
	inspection.EndpointSHA256 = hash(endpointData)
	envelopeData, err := canonical(endpoint.Envelope)
	if err != nil {
		return inspection, err
	}
	inspection.EnvelopeSHA256 = hash(envelopeData)
	deliveryData, err := readOptionalTransport(job.CaseRoot, inspection.DeliveryPath, "external session transport delivery")
	if err != nil {
		return inspection, err
	}
	if deliveryData == nil {
		return inspection, nil
	}
	var delivery TransportDeliveryObservation
	if err := decodeCanonicalTransport(deliveryData, &delivery); err != nil {
		return inspection, fmt.Errorf("invalid external session transport delivery: %w", err)
	}
	if err := validateTransportDelivery(job, attempt, dispatch, endpoint, inspection.EndpointSHA256, inspection.EnvelopeSHA256, delivery); err != nil {
		return inspection, err
	}
	inspection.Delivery = &delivery
	inspection.DeliverySHA256 = hash(deliveryData)
	inspection.State = "delivery-" + delivery.Outcome
	if delivery.Outcome == "uncertain" {
		inspection.Warnings = append(inspection.Warnings, "delivery is uncertain; do not resend automatically or claim launch acceptance")
	}
	return inspection, nil
}

func PreviewTransportEndpoint(job Job, attempt AttemptInspection, dispatch DispatchInspection, endpoint, actor, observedAt string) (TransportPlan, error) {
	inspection, err := InspectTransport(job, attempt, dispatch)
	if err != nil {
		return TransportPlan{}, err
	}
	if !inspection.Applicable || dispatch.Claim == nil || dispatch.Ticket == nil {
		return TransportPlan{}, fmt.Errorf("Remote Control endpoint requires the exact claimed transport dispatch")
	}
	endpoint = strings.TrimSpace(endpoint)
	actor = strings.TrimSpace(actor)
	observedAt = strings.TrimSpace(observedAt)
	if !validBoundedSingleLine(endpoint, 512) {
		return TransportPlan{}, fmt.Errorf("Remote Control endpoint must be a bounded non-empty opaque ListAgents route")
	}
	if actor != dispatch.Claim.Actor {
		return TransportPlan{}, fmt.Errorf("Remote Control endpoint actor must match the exact dispatch claim owner")
	}
	observed, err := canonicalTransportTime(observedAt, "endpoint observedAt")
	if err != nil {
		return TransportPlan{}, err
	}
	claimed, _ := time.Parse(time.RFC3339Nano, dispatch.Claim.ClaimedAt)
	if observed.Before(claimed) {
		return TransportPlan{}, fmt.Errorf("Remote Control endpoint observation predates the dispatch claim")
	}
	_, bundleData, err := buildTransportEvidenceBundle(job, *dispatch.Ticket, inspection.Binding)
	if err != nil {
		return TransportPlan{}, err
	}
	bundlePath, err := transportBundlePath(job.CaseRoot, job.JobID, attempt.Current.Generation)
	if err != nil {
		return TransportPlan{}, err
	}
	bundleSHA := hash(bundleData)
	envelope, err := buildTransportEnvelope(job, *dispatch.Ticket, inspection.Binding, endpoint, bundlePath, bundleData)
	if err != nil {
		return TransportPlan{}, err
	}
	snapshot := TransportEndpointSnapshot{
		SchemaVersion: SchemaVersion, Kind: KindTransportEndpointSnapshot,
		Binding: inspection.Binding, DiscoveryTool: "ListAgents", Endpoint: endpoint,
		Actor: actor, ObservedAt: observedAt, BundlePath: bundlePath, BundleSHA256: bundleSHA, BundleBytes: len(bundleData), Envelope: envelope,
		Capability: job.Capability, NoSessionManagement: true, NoAutomaticRetry: true, NoHeavyTool: true, NoAuthority: true, NoConfirmed: true,
	}
	data, err := canonical(snapshot)
	if err != nil {
		return TransportPlan{}, err
	}
	if inspection.Endpoint != nil && !bytes.Equal(data, mustCanonical(*inspection.Endpoint)) {
		return TransportPlan{}, fmt.Errorf("Remote Control endpoint snapshot already exists with different immutable bytes")
	}
	plan, err := newTransportPlan("remote-control-endpoint", job, attempt, dispatch, inspection.EndpointPath, data, &snapshot, nil)
	if err != nil {
		return TransportPlan{}, err
	}
	plan.BundlePath, plan.BundleSHA256, plan.BundleBytes = bundlePath, bundleSHA, len(bundleData)
	plan.bundleData = append([]byte{}, bundleData...)
	return plan, nil
}

func PreviewTransportDelivery(job Job, attempt AttemptInspection, dispatch DispatchInspection, outcome, ackFingerprint, actor, observedAt, reason string) (TransportPlan, error) {
	inspection, err := InspectTransport(job, attempt, dispatch)
	if err != nil {
		return TransportPlan{}, err
	}
	if inspection.Endpoint == nil || inspection.EndpointSHA256 == "" {
		return TransportPlan{}, fmt.Errorf("Remote Control delivery requires the exact endpoint snapshot and message envelope")
	}
	outcome = strings.ToLower(strings.TrimSpace(outcome))
	ackFingerprint = strings.ToLower(strings.TrimSpace(ackFingerprint))
	actor = strings.TrimSpace(actor)
	observedAt = strings.TrimSpace(observedAt)
	reason = strings.TrimSpace(reason)
	if actor != dispatch.Claim.Actor || actor != inspection.Endpoint.Actor {
		return TransportPlan{}, fmt.Errorf("Remote Control delivery actor must match the endpoint and dispatch claim owner")
	}
	observed, err := canonicalTransportTime(observedAt, "delivery observedAt")
	if err != nil {
		return TransportPlan{}, err
	}
	endpointObserved, _ := time.Parse(time.RFC3339Nano, inspection.Endpoint.ObservedAt)
	if observed.Before(endpointObserved) {
		return TransportPlan{}, fmt.Errorf("Remote Control delivery observation predates endpoint discovery")
	}
	if ackFingerprint != "" && !validSHA(ackFingerprint) {
		return TransportPlan{}, fmt.Errorf("Remote Control provider acknowledgement fingerprint must be SHA-256")
	}
	switch outcome {
	case "accepted":
		if reason != "" {
			return TransportPlan{}, fmt.Errorf("accepted Remote Control delivery cannot include a reason")
		}
	case "rejected", "uncertain":
		if !validBoundedSingleLine(reason, 2048) {
			return TransportPlan{}, fmt.Errorf("%s Remote Control delivery requires a bounded single-line reason", outcome)
		}
	default:
		return TransportPlan{}, fmt.Errorf("Remote Control delivery outcome must be accepted, rejected, or uncertain")
	}
	delivery := TransportDeliveryObservation{
		SchemaVersion: SchemaVersion, Kind: KindTransportDelivery, Binding: inspection.Binding,
		EndpointSnapshotSHA256: inspection.EndpointSHA256, EnvelopeSHA256: inspection.EnvelopeSHA256,
		Operation: "SendMessage", Outcome: outcome, ProviderAckFingerprint: ackFingerprint,
		Actor: actor, ObservedAt: observedAt, Reason: reason,
		Capability: job.Capability, NoSessionManagement: true, NoAutomaticRetry: true, NoHeavyTool: true, NoAuthority: true, NoConfirmed: true,
	}
	data, err := canonical(delivery)
	if err != nil {
		return TransportPlan{}, err
	}
	if inspection.Delivery != nil && !bytes.Equal(data, mustCanonical(*inspection.Delivery)) {
		return TransportPlan{}, fmt.Errorf("Remote Control delivery observation already exists with different immutable bytes")
	}
	plan, err := newTransportPlan("remote-control-delivery", job, attempt, dispatch, inspection.DeliveryPath, data, nil, &delivery)
	if err != nil {
		return TransportPlan{}, err
	}
	plan.BundlePath, plan.BundleSHA256, plan.BundleBytes = inspection.BundlePath, inspection.BundleSHA256, inspection.BundleBytes
	return plan, nil
}

func ApplyTransportCurrent(plan TransportPlan, expectedPlanSHA256 string, current func() (Job, error)) (_ TransportPlan, retErr error) {
	if !strings.EqualFold(strings.TrimSpace(expectedPlanSHA256), plan.ExpectedPlanSHA256) {
		return TransportPlan{}, fmt.Errorf("external session transport plan sha256 mismatch; rerun WhatIf")
	}
	lease, err := lanemutation.AcquireProject(plan.Job.CaseRoot)
	if err != nil {
		return TransportPlan{}, err
	}
	defer func() { retErr = errorsJoin(retErr, lease.Unlock()) }()
	if current != nil {
		live, err := current()
		if err != nil {
			return TransportPlan{}, err
		}
		liveSHA, err := jobSHA256(live)
		if err != nil || !strings.EqualFold(liveSHA, plan.JobSHA256) {
			return TransportPlan{}, fmt.Errorf("external session transport job is no longer current")
		}
	}
	attempt, err := InspectAttempt(plan.Job)
	if err != nil || attempt.Current == nil || !strings.EqualFold(attempt.AttemptSHA256, plan.AttemptSHA256) {
		return TransportPlan{}, fmt.Errorf("external session transport attempt is no longer current")
	}
	dispatch, err := InspectCurrentDispatch(plan.Job, attempt)
	if err != nil || dispatch.Claim == nil || !strings.EqualFold(dispatch.TicketSHA256, plan.DispatchSHA256) || !strings.EqualFold(dispatch.ClaimSHA256, plan.ClaimSHA256) {
		return TransportPlan{}, fmt.Errorf("external session transport claim is no longer current")
	}
	var fresh TransportPlan
	switch plan.Mode {
	case "remote-control-endpoint":
		if plan.Endpoint == nil {
			return TransportPlan{}, fmt.Errorf("external session transport endpoint plan is incomplete")
		}
		fresh, err = PreviewTransportEndpoint(plan.Job, attempt, dispatch, plan.Endpoint.Endpoint, plan.Endpoint.Actor, plan.Endpoint.ObservedAt)
	case "remote-control-delivery":
		if plan.Delivery == nil {
			return TransportPlan{}, fmt.Errorf("external session transport delivery plan is incomplete")
		}
		fresh, err = PreviewTransportDelivery(plan.Job, attempt, dispatch, plan.Delivery.Outcome, plan.Delivery.ProviderAckFingerprint, plan.Delivery.Actor, plan.Delivery.ObservedAt, plan.Delivery.Reason)
	default:
		return TransportPlan{}, fmt.Errorf("external session transport mode %q is unsupported", plan.Mode)
	}
	if err != nil {
		return TransportPlan{}, err
	}
	if fresh.ExpectedPlanSHA256 != plan.ExpectedPlanSHA256 || !bytes.Equal(fresh.data, plan.data) || fresh.BundlePath != plan.BundlePath || fresh.BundleSHA256 != plan.BundleSHA256 || fresh.BundleBytes != plan.BundleBytes {
		return TransportPlan{}, fmt.Errorf("external session transport changed after preview; rerun WhatIf")
	}
	bundleReplayed := true
	if plan.Mode == "remote-control-endpoint" {
		if !bytes.Equal(fresh.bundleData, plan.bundleData) || hash(plan.bundleData) != plan.BundleSHA256 || len(plan.bundleData) != plan.BundleBytes {
			return TransportPlan{}, fmt.Errorf("external session transport evidence bundle changed after preview; rerun WhatIf")
		}
		bundleReplayed, err = rekitfs.WriteExclusiveRegularFileAnchored(plan.Job.CaseRoot, plan.BundlePath, "external session transport evidence bundle", plan.bundleData)
		if err != nil {
			return TransportPlan{}, err
		}
	}
	replayed, err := rekitfs.WriteExclusiveRegularFileAnchored(plan.Job.CaseRoot, plan.ArtifactPath, "external session transport artifact", plan.data)
	if err != nil {
		return TransportPlan{}, err
	}
	plan.Applied, plan.AlreadyApplied, plan.ReviewRequired, plan.RequiresConfirmation = true, bundleReplayed && replayed, false, false
	plan.data = nil
	plan.bundleData = nil
	return plan, nil
}

func TransportLaunchTransition(inspection TransportInspection) (outcome, actor, observedAt, actualHarness, actualSession, reason string, err error) {
	if inspection.Delivery == nil {
		err = fmt.Errorf("Remote Control launch transition requires a delivery observation")
		return
	}
	if capErr := capabilitycontract.RequireBindingPolicy(inspection.Delivery.Capability, capabilitycontract.PolicyClassTransport); capErr != nil || inspection.Delivery.Capability != transportCapability() {
		err = fmt.Errorf("Remote Control delivery capability contract cannot become launch truth")
		return
	}
	actor = inspection.Delivery.Actor
	observedAt = inspection.Delivery.ObservedAt
	switch inspection.Delivery.Outcome {
	case "accepted":
		outcome = "accepted"
		actualHarness = RemoteControlHarness
		actualSession = inspection.Binding.TransportBindingID
	case "rejected":
		outcome = "failed"
		reason = remoteControlLaunchFailureReason(*inspection.Delivery)
	case "uncertain":
		err = fmt.Errorf("uncertain Remote Control delivery cannot become launch truth; create a new durable Reviewer dispatch and job without automatic resend")
	default:
		err = fmt.Errorf("unsupported Remote Control delivery outcome %q", inspection.Delivery.Outcome)
	}
	return
}

func ValidateRemoteControlLaunchTransition(inspection TransportInspection, outcome, actor, observedAt, actualHarness, actualSession, reason string) error {
	expectedOutcome, expectedActor, expectedObservedAt, expectedHarness, expectedSession, expectedReason, err := TransportLaunchTransition(inspection)
	if err != nil {
		return err
	}
	if outcome != expectedOutcome || actor != expectedActor || observedAt != expectedObservedAt || actualHarness != expectedHarness || actualSession != expectedSession || reason != expectedReason {
		return fmt.Errorf("Remote Control launch receipt must derive exactly from the immutable delivery observation")
	}
	return nil
}

func newTransportPlan(mode string, job Job, attempt AttemptInspection, dispatch DispatchInspection, path string, data []byte, endpoint *TransportEndpointSnapshot, delivery *TransportDeliveryObservation) (TransportPlan, error) {
	jobSHA, err := jobSHA256(job)
	if err != nil {
		return TransportPlan{}, err
	}
	artifactSHA := hash(data)
	identity, err := json.Marshal(struct {
		JobSHA256      string `json:"jobSha256"`
		AttemptSHA256  string `json:"attemptSha256"`
		DispatchSHA256 string `json:"dispatchSha256"`
		ClaimSHA256    string `json:"claimSha256"`
		Mode           string `json:"mode"`
		ArtifactPath   string `json:"artifactPath"`
		ArtifactSHA256 string `json:"artifactSha256"`
	}{jobSHA, attempt.AttemptSHA256, dispatch.TicketSHA256, dispatch.ClaimSHA256, mode, path, artifactSHA})
	if err != nil {
		return TransportPlan{}, err
	}
	return TransportPlan{
		SchemaVersion: SchemaVersion, Mode: mode, Job: job, JobSHA256: jobSHA,
		AttemptSHA256: attempt.AttemptSHA256, DispatchSHA256: dispatch.TicketSHA256, ClaimSHA256: dispatch.ClaimSHA256,
		ArtifactPath: path, ArtifactSHA256: artifactSHA, ExpectedPlanSHA256: hash(identity),
		Endpoint: endpoint, Delivery: delivery, ReviewRequired: true, RequiresConfirmation: true,
		Boundary: []string{
			"transport artifacts bind the exact job, attempt, dispatch, claim, opaque endpoint snapshot, and message envelope",
			"delivery accepted is launch truth only after the existing launch receipt is derived; it is never completion or authority",
			"uncertain delivery blocks automatic resend and same-job replacement; recovery requires a new durable Reviewer dispatch, session, and job",
			"Remote Control transport does not write result artifacts, authority/confirmed state, or heavy-tool authorization",
		},
		data: data,
	}, nil
}

func transportBinding(job Job, attempt AttemptInspection, dispatch DispatchInspection) (TransportBinding, error) {
	if attempt.Current == nil || dispatch.Ticket == nil || dispatch.Claim == nil || !IsRemoteControlAttempt(attempt.Current) {
		return TransportBinding{}, fmt.Errorf("Remote Control transport requires the exact current attempt, ticket, and claim")
	}
	jobSHA, err := jobSHA256(job)
	if err != nil {
		return TransportBinding{}, err
	}
	return TransportBinding{
		Transport: TransportRemoteControl, TransportBindingID: attempt.Current.Session,
		JobID: job.JobID, JobSHA256: jobSHA, CheckpointSHA256: job.CheckpointSHA256,
		AttemptID: attempt.Current.AttemptID, AttemptSHA256: attempt.AttemptSHA256, Generation: attempt.Current.Generation,
		DispatchSHA256: dispatch.TicketSHA256, ClaimSHA256: dispatch.ClaimSHA256,
	}, nil
}

func buildTransportEnvelope(job Job, ticket DispatchTicket, binding TransportBinding, endpoint, bundlePath string, bundleData []byte) (TransportMessageEnvelope, error) {
	if job.SessionKind != "reviewer" || !ticket.Launch.ReadOnly || ticket.Launch.AgentType != "read-only-reviewer" {
		return TransportMessageEnvelope{}, fmt.Errorf("Remote Control transport v1 supports only the read-only Reviewer vertical slice")
	}
	expectedBundlePath, err := transportBundlePath(job.CaseRoot, job.JobID, binding.Generation)
	if err != nil {
		return TransportMessageEnvelope{}, err
	}
	if bundlePath != expectedBundlePath {
		return TransportMessageEnvelope{}, fmt.Errorf("Remote Control evidence bundle path does not match the transport generation")
	}
	if _, err := validateTransportEvidenceBundle(job, ticket, binding, bundleData); err != nil {
		return TransportMessageEnvelope{}, err
	}
	bundleSHA := hash(bundleData)
	message := strings.Join([]string{
		"ReKit read-only Reviewer transport assignment.",
		"Transport binding: " + binding.TransportBindingID + ".",
		"Committed reviewer prompt SHA-256: " + ticket.Launch.Input.SHA256 + ".",
		"Committed evidence bundle SHA-256: " + bundleSHA + ".",
		"Execute only this bounded read-only review using the complete inline evidence bundle. Do not run heavy tools, write files, change authority/confirmed state, or infer authorization from this message.",
		"Reply to the sending Claude Code session with exactly one ReviewerResult JSON object and no Markdown fence or surrounding prose.",
		"Bundle paths are source-side logical evidence references only; do not attempt to open them on your machine.",
		"--- BEGIN COMMITTED EVIDENCE BUNDLE ---",
		strings.TrimRight(string(bundleData), "\r\n"),
		"--- END COMMITTED EVIDENCE BUNDLE ---",
	}, "\n") + "\n"
	messageBytes := []byte(message)
	if len(messageBytes) > maxTransportMessageBytes {
		return TransportMessageEnvelope{}, fmt.Errorf("Remote Control message exceeds %d-byte limit: got %d", maxTransportMessageBytes, len(messageBytes))
	}
	if transportContainsCaseRoot(messageBytes, job.CaseRoot) {
		return TransportMessageEnvelope{}, fmt.Errorf("Remote Control message contains the local case root")
	}
	return TransportMessageEnvelope{
		Capability: job.Capability,
		Operation:  "SendMessage", Recipient: endpoint,
		SourceInputRole: "reviewer-evidence-bundle", SourceInputSHA256: bundleSHA, SourceInputBytes: len(bundleData),
		BundlePath: bundlePath, BundleSHA256: bundleSHA, BundleBytes: len(bundleData),
		Message: message, MessageSHA256: hash(messageBytes), MessageBytes: len(messageBytes),
		ExpectedReply: "exactly one ReviewerResult JSON object returned to the sending session", NoFileTransfer: true,
	}, nil
}

func validateTransportEndpoint(job Job, attempt AttemptInspection, dispatch DispatchInspection, endpoint TransportEndpointSnapshot) error {
	binding, err := transportBinding(job, attempt, dispatch)
	if err != nil {
		return err
	}
	bundlePath, err := transportBundlePath(job.CaseRoot, job.JobID, attempt.Current.Generation)
	if err != nil {
		return err
	}
	bundleData, err := readOptionalTransport(job.CaseRoot, bundlePath, "external session transport evidence bundle")
	if err != nil || bundleData == nil {
		return fmt.Errorf("external session transport endpoint evidence bundle is unavailable")
	}
	if _, err := validateTransportEvidenceBundle(job, *dispatch.Ticket, binding, bundleData); err != nil {
		return err
	}
	if endpoint.SchemaVersion != SchemaVersion || endpoint.Kind != KindTransportEndpointSnapshot || endpoint.Binding != binding || endpoint.DiscoveryTool != "ListAgents" || !validBoundedSingleLine(endpoint.Endpoint, 512) || endpoint.Actor != dispatch.Claim.Actor || endpoint.BundlePath != bundlePath || !strings.EqualFold(endpoint.BundleSHA256, hash(bundleData)) || endpoint.BundleBytes != len(bundleData) || !endpoint.NoSessionManagement || !endpoint.NoAutomaticRetry || !endpoint.NoHeavyTool || !endpoint.NoAuthority || !endpoint.NoConfirmed {
		return fmt.Errorf("external session transport endpoint contract is invalid")
	}
	if err := capabilitycontract.RequireBindingPolicy(endpoint.Capability, capabilitycontract.PolicyClassTransport); err != nil || endpoint.Capability != job.Capability || endpoint.Envelope.Capability != endpoint.Capability {
		return fmt.Errorf("external session transport endpoint capability contract is invalid")
	}
	observed, err := canonicalTransportTime(endpoint.ObservedAt, "endpoint observedAt")
	if err != nil {
		return err
	}
	claimed, _ := time.Parse(time.RFC3339Nano, dispatch.Claim.ClaimedAt)
	if observed.Before(claimed) {
		return fmt.Errorf("external session transport endpoint predates the dispatch claim")
	}
	expected, err := buildTransportEnvelope(job, *dispatch.Ticket, binding, endpoint.Endpoint, bundlePath, bundleData)
	if err != nil || endpoint.Envelope != expected {
		return fmt.Errorf("external session transport message envelope does not match the exact dispatch input")
	}
	return nil
}

func validateTransportDelivery(job Job, attempt AttemptInspection, dispatch DispatchInspection, endpoint TransportEndpointSnapshot, endpointSHA, envelopeSHA string, delivery TransportDeliveryObservation) error {
	binding, err := transportBinding(job, attempt, dispatch)
	if err != nil {
		return err
	}
	if delivery.SchemaVersion != SchemaVersion || delivery.Kind != KindTransportDelivery || delivery.Binding != binding || !strings.EqualFold(delivery.EndpointSnapshotSHA256, endpointSHA) || !strings.EqualFold(delivery.EnvelopeSHA256, envelopeSHA) || delivery.Operation != "SendMessage" || delivery.Actor != dispatch.Claim.Actor || delivery.Actor != endpoint.Actor || !delivery.NoSessionManagement || !delivery.NoAutomaticRetry || !delivery.NoHeavyTool || !delivery.NoAuthority || !delivery.NoConfirmed {
		return fmt.Errorf("external session transport delivery contract is invalid")
	}
	if err := capabilitycontract.RequireBindingPolicy(delivery.Capability, capabilitycontract.PolicyClassTransport); err != nil || delivery.Capability != endpoint.Capability || delivery.Capability != endpoint.Envelope.Capability || delivery.Capability != job.Capability {
		return fmt.Errorf("external session transport delivery capability contract is invalid")
	}
	observed, err := canonicalTransportTime(delivery.ObservedAt, "delivery observedAt")
	if err != nil {
		return err
	}
	endpointObserved, _ := time.Parse(time.RFC3339Nano, endpoint.ObservedAt)
	if observed.Before(endpointObserved) {
		return fmt.Errorf("external session transport delivery predates endpoint discovery")
	}
	if delivery.ProviderAckFingerprint != "" && !validSHA(delivery.ProviderAckFingerprint) {
		return fmt.Errorf("external session transport provider acknowledgement fingerprint is invalid")
	}
	switch delivery.Outcome {
	case "accepted":
		if delivery.Reason != "" {
			return fmt.Errorf("accepted external session transport delivery cannot include a reason")
		}
	case "rejected", "uncertain":
		if !validBoundedSingleLine(delivery.Reason, 2048) {
			return fmt.Errorf("external session transport %s delivery requires a reason", delivery.Outcome)
		}
	default:
		return fmt.Errorf("external session transport delivery outcome is invalid")
	}
	return nil
}

func remoteControlLaunchFailureReason(delivery TransportDeliveryObservation) string {
	return "Remote Control SendMessage rejected: " + delivery.Reason
}

func transportEndpointPath(caseRoot, jobID string, generation int) (string, error) {
	return projectstate.Rel(caseRoot, "external-session-transport", "endpoints", jobID, fmt.Sprintf("%06d.json", generation))
}

func transportDeliveryPath(caseRoot, jobID string, generation int) (string, error) {
	return projectstate.Rel(caseRoot, "external-session-transport", "deliveries", jobID, fmt.Sprintf("%06d.json", generation))
}

func readOptionalTransport(caseRoot, rel, label string) ([]byte, error) {
	path, err := rekitfs.SafeJoin(caseRoot, rel)
	if err != nil {
		return nil, err
	}
	data, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, path, label, maxTransportArtifactBytes)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}

func decodeCanonicalTransport(data []byte, value any) error {
	if len(data) == 0 || len(data) > maxTransportArtifactBytes {
		return fmt.Errorf("external session transport artifact must be bounded non-empty JSON")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(value); err != nil {
		return err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("external session transport artifact must contain exactly one JSON object")
	}
	canonicalBytes, err := canonical(value)
	if err != nil || !bytes.Equal(canonicalBytes, data) {
		return fmt.Errorf("external session transport artifact is not canonical")
	}
	return nil
}

func canonicalTransportTime(value, label string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || parsed.Format(time.RFC3339Nano) != value {
		return time.Time{}, fmt.Errorf("Remote Control transport %s must be canonical RFC3339Nano", label)
	}
	return parsed, nil
}

func validBoundedSingleLine(value string, limit int) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= limit && !strings.ContainsAny(value, "\r\n\x00")
}

func redactTransportCaseRoot(value, caseRoot string) string {
	variants := []string{
		filepath.Clean(caseRoot),
		filepath.ToSlash(filepath.Clean(caseRoot)),
		strings.ReplaceAll(filepath.Clean(caseRoot), "/", "\\"),
	}
	for _, variant := range variants {
		value = replaceAllFold(value, variant, "<case-root>")
	}
	return value
}

func replaceAllFold(value, old, replacement string) string {
	if old == "" {
		return value
	}
	for {
		index := strings.Index(strings.ToLower(value), strings.ToLower(old))
		if index < 0 {
			return value
		}
		value = value[:index] + replacement + value[index+len(old):]
	}
}

func containsFold(value, needle string) bool {
	return needle != "" && strings.Contains(strings.ToLower(value), strings.ToLower(needle))
}

func mustCanonical(value any) []byte {
	data, _ := canonical(value)
	return data
}
