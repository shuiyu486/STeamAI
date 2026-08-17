package externalsession

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewerresult"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewersession"
)

const KindTransportReturnReceipt = "current-loop-external-session-transport-return"

type TransportReturnReceipt struct {
	SchemaVersion          int              `json:"schemaVersion"`
	Kind                   string           `json:"kind"`
	Binding                TransportBinding `json:"binding"`
	BundlePath             string           `json:"bundlePath"`
	BundleSHA256           string           `json:"bundleSha256"`
	BundleBytes            int              `json:"bundleBytes"`
	EndpointSnapshotSHA256 string           `json:"endpointSnapshotSha256"`
	EnvelopeSHA256         string           `json:"envelopeSha256"`
	DeliverySHA256         string           `json:"deliverySha256"`
	LaunchReceiptSHA256    string           `json:"launchReceiptSha256"`
	SourcePath             string           `json:"sourcePath"`
	SourceSHA256           string           `json:"sourceSha256"`
	SourceBytes            int              `json:"sourceBytes"`
	ResultPath             string           `json:"resultPath"`
	ResultSHA256           string           `json:"resultSha256"`
	ResultBytes            int              `json:"resultBytes"`
	Actor                  string           `json:"actor"`
	ObservedAt             string           `json:"observedAt"`
	NoSessionManagement    bool             `json:"noSessionManagement"`
	NoHeavyTool            bool             `json:"noHeavyTool"`
	NoAuthority            bool             `json:"noAuthority"`
	NoConfirmed            bool             `json:"noConfirmed"`
}

type TransportReturnPlan struct {
	SchemaVersion        int                    `json:"schemaVersion"`
	Mode                 string                 `json:"mode"`
	Job                  Job                    `json:"job"`
	JobSHA256            string                 `json:"jobSha256"`
	AttemptSHA256        string                 `json:"attemptSha256"`
	DispatchSHA256       string                 `json:"dispatchSha256"`
	ClaimSHA256          string                 `json:"claimSha256"`
	LaunchSHA256         string                 `json:"launchSha256"`
	DeliverySHA256       string                 `json:"deliverySha256"`
	SourcePath           string                 `json:"sourcePath"`
	SourceSHA256         string                 `json:"sourceSha256"`
	SourceBytes          int                    `json:"sourceBytes"`
	ResultPath           string                 `json:"resultPath"`
	ResultSHA256         string                 `json:"resultSha256"`
	ResultBytes          int                    `json:"resultBytes"`
	ReturnReceiptPath    string                 `json:"returnReceiptPath"`
	ReturnReceiptSHA256  string                 `json:"returnReceiptSha256"`
	SubmissionPath       string                 `json:"submissionPath"`
	SubmissionSHA256     string                 `json:"submissionSha256"`
	ExpectedPlanSHA256   string                 `json:"expectedPlanSha256"`
	ReturnReceipt        TransportReturnReceipt `json:"returnReceipt"`
	Submission           Submission             `json:"submission"`
	ReviewRequired       bool                   `json:"reviewRequired"`
	RequiresConfirmation bool                   `json:"requiresConfirmation"`
	Applied              bool                   `json:"applied"`
	AlreadyApplied       bool                   `json:"alreadyApplied"`
	Boundary             []string               `json:"boundary"`
	resultData           []byte
	returnData           []byte
	submissionData       []byte
}

func PreviewTransportReturn(job Job, sourcePath, actor, observedAt string) (TransportReturnPlan, error) {
	jobSHA, err := jobSHA256(job)
	if err != nil {
		return TransportReturnPlan{}, err
	}
	attempt, err := InspectAttempt(job)
	if err != nil || attempt.Current == nil || !IsRemoteControlAttempt(attempt.Current) {
		return TransportReturnPlan{}, fmt.Errorf("Remote Control return requires the exact current Remote Control attempt")
	}
	dispatch, err := InspectCurrentDispatch(job, attempt)
	if err != nil || dispatch.State != "running" || dispatch.Ticket == nil || dispatch.Claim == nil || dispatch.Launch == nil {
		return TransportReturnPlan{}, fmt.Errorf("Remote Control return requires the exact accepted launch lineage")
	}
	transport, err := InspectTransport(job, attempt, dispatch)
	if err != nil {
		return TransportReturnPlan{}, err
	}
	if transport.State != "delivery-accepted" || transport.Delivery == nil || transport.Endpoint == nil {
		return TransportReturnPlan{}, fmt.Errorf("Remote Control return requires an accepted immutable delivery observation")
	}
	if err := ValidateRemoteControlLaunchTransition(
		transport,
		dispatch.Launch.Outcome,
		dispatch.Launch.Actor,
		dispatch.Launch.ObservedAt,
		dispatch.Launch.ActualHarness,
		dispatch.Launch.ActualSession,
		dispatch.Launch.Reason,
	); err != nil {
		return TransportReturnPlan{}, err
	}
	actor = strings.TrimSpace(actor)
	observedAt = strings.TrimSpace(observedAt)
	if !validBoundedSingleLine(actor, 512) || actor != dispatch.Claim.Actor {
		return TransportReturnPlan{}, fmt.Errorf("Remote Control return actor must match the exact dispatch claim owner")
	}
	observed, err := canonicalTransportTime(observedAt, "return observedAt")
	if err != nil {
		return TransportReturnPlan{}, err
	}
	launched, _ := time.Parse(time.RFC3339Nano, dispatch.Launch.ObservedAt)
	if observed.Before(launched) {
		return TransportReturnPlan{}, fmt.Errorf("Remote Control return predates the accepted launch receipt")
	}
	sourceFull, err := transportAnchoredPath(job.CaseRoot, sourcePath)
	if err != nil {
		return TransportReturnPlan{}, err
	}
	sourceRel, err := transportRelativePath(job.CaseRoot, sourceFull)
	if err != nil {
		return TransportReturnPlan{}, err
	}
	resultData, err := rekitfs.ReadStableRegularFileAnchored(job.CaseRoot, sourceFull, "Remote Control inbound ReviewerResult", reviewerresult.MaxBytes)
	if err != nil {
		return TransportReturnPlan{}, err
	}
	if _, err := validateReviewerResultForJob(job, resultData); err != nil {
		return TransportReturnPlan{}, err
	}
	resultPath := attempt.Current.SubmissionResult
	returnPath, err := TransportReturnReceiptPathForCase(job.CaseRoot, job.JobID, attempt.Current.Generation)
	if err != nil {
		return TransportReturnPlan{}, err
	}
	if sourceRel == resultPath || sourceRel == returnPath || sourceRel == attempt.Current.SubmissionPath {
		return TransportReturnPlan{}, fmt.Errorf("Remote Control ReviewerResult source path conflicts with return publication paths")
	}
	resultSHA := hash(resultData)
	receipt := TransportReturnReceipt{
		SchemaVersion:          SchemaVersion,
		Kind:                   KindTransportReturnReceipt,
		Binding:                transport.Binding,
		BundlePath:             transport.BundlePath,
		BundleSHA256:           transport.BundleSHA256,
		BundleBytes:            transport.BundleBytes,
		EndpointSnapshotSHA256: transport.EndpointSHA256,
		EnvelopeSHA256:         transport.EnvelopeSHA256,
		DeliverySHA256:         transport.DeliverySHA256,
		LaunchReceiptSHA256:    dispatch.LaunchSHA256,
		SourcePath:             sourceRel,
		SourceSHA256:           resultSHA,
		SourceBytes:            len(resultData),
		ResultPath:             resultPath,
		ResultSHA256:           resultSHA,
		ResultBytes:            len(resultData),
		Actor:                  actor, ObservedAt: observedAt,
		NoSessionManagement: true, NoHeavyTool: true, NoAuthority: true, NoConfirmed: true,
	}
	returnData, err := canonical(receipt)
	if err != nil {
		return TransportReturnPlan{}, err
	}
	submission := Submission{
		SchemaVersion:                SchemaVersion,
		Kind:                         KindSubmission,
		JobID:                        job.JobID,
		JobSHA256:                    jobSHA,
		Outcome:                      "returned",
		Actor:                        actor,
		ReviewerHarness:              RemoteControlHarness,
		ReviewerSession:              transport.Binding.TransportBindingID,
		AttemptID:                    attempt.Current.AttemptID,
		AttemptSHA256:                attempt.AttemptSHA256,
		DispatchClaimSHA256:          dispatch.ClaimSHA256,
		LaunchReceiptSHA256:          dispatch.LaunchSHA256,
		TransportReturnReceiptPath:   returnPath,
		TransportReturnReceiptSHA256: hash(returnData),
		Harness:                      dispatch.Launch.ActualHarness,
		Session:                      dispatch.Launch.ActualSession,
		NoAuthorityOrConfirmed:       true,
		NoHeavyTool:                  true,
	}
	submissionData, err := canonical(submission)
	if err != nil {
		return TransportReturnPlan{}, err
	}
	identityData, err := json.Marshal(struct {
		JobSHA256           string `json:"jobSha256"`
		AttemptSHA256       string `json:"attemptSha256"`
		DispatchSHA256      string `json:"dispatchSha256"`
		ClaimSHA256         string `json:"claimSha256"`
		LaunchSHA256        string `json:"launchSha256"`
		DeliverySHA256      string `json:"deliverySha256"`
		SourceSHA256        string `json:"sourceSha256"`
		ResultPath          string `json:"resultPath"`
		ResultSHA256        string `json:"resultSha256"`
		ReturnReceiptPath   string `json:"returnReceiptPath"`
		ReturnReceiptSHA256 string `json:"returnReceiptSha256"`
		SubmissionPath      string `json:"submissionPath"`
		SubmissionSHA256    string `json:"submissionSha256"`
	}{
		jobSHA, attempt.AttemptSHA256, dispatch.TicketSHA256, dispatch.ClaimSHA256,
		dispatch.LaunchSHA256, transport.DeliverySHA256, resultSHA, resultPath, resultSHA,
		returnPath, hash(returnData), attempt.Current.SubmissionPath, hash(submissionData),
	})
	if err != nil {
		return TransportReturnPlan{}, err
	}
	return TransportReturnPlan{
		SchemaVersion: SchemaVersion, Mode: "remote-control-return", Job: job, JobSHA256: jobSHA,
		AttemptSHA256: attempt.AttemptSHA256, DispatchSHA256: dispatch.TicketSHA256,
		ClaimSHA256: dispatch.ClaimSHA256, LaunchSHA256: dispatch.LaunchSHA256, DeliverySHA256: transport.DeliverySHA256,
		SourcePath: sourceRel, SourceSHA256: resultSHA, SourceBytes: len(resultData),
		ResultPath: resultPath, ResultSHA256: resultSHA, ResultBytes: len(resultData),
		ReturnReceiptPath: returnPath, ReturnReceiptSHA256: hash(returnData),
		SubmissionPath: attempt.Current.SubmissionPath, SubmissionSHA256: hash(submissionData),
		ExpectedPlanSHA256: hash(identityData), ReturnReceipt: receipt, Submission: submission,
		ReviewRequired: true, RequiresConfirmation: true,
		Boundary: []string{
			"the inbound source is a case-local bounded ReviewerResult; no transport message grants authority or confirmed state",
			"Apply revalidates current delivery and launch lineage, then publishes result first, return receipt second, and submission last",
			"the existing relay and strict Reviewer intake remain the only canonical result consumption path",
			"no Remote Control resend, session management, heavy tool, authority, or confirmed write is performed",
		},
		resultData: append([]byte{}, resultData...), returnData: returnData, submissionData: submissionData,
	}, nil
}

func ApplyTransportReturnCurrent(plan TransportReturnPlan, expectedPlanSHA256 string, current func() (Job, error)) (_ TransportReturnPlan, retErr error) {
	if !strings.EqualFold(strings.TrimSpace(expectedPlanSHA256), plan.ExpectedPlanSHA256) {
		return TransportReturnPlan{}, fmt.Errorf("external session transport return plan sha256 mismatch; rerun WhatIf")
	}
	lease, err := lanemutation.AcquireProject(plan.Job.CaseRoot)
	if err != nil {
		return TransportReturnPlan{}, err
	}
	defer func() { retErr = errorsJoin(retErr, lease.Unlock()) }()
	if current != nil {
		live, err := current()
		if err != nil {
			return TransportReturnPlan{}, err
		}
		liveSHA, err := jobSHA256(live)
		if err != nil || !strings.EqualFold(liveSHA, plan.JobSHA256) {
			return TransportReturnPlan{}, fmt.Errorf("external session transport return job is no longer current")
		}
	}
	fresh, err := PreviewTransportReturn(plan.Job, plan.SourcePath, plan.ReturnReceipt.Actor, plan.ReturnReceipt.ObservedAt)
	if err != nil {
		return TransportReturnPlan{}, err
	}
	if fresh.ExpectedPlanSHA256 != plan.ExpectedPlanSHA256 ||
		!bytes.Equal(fresh.resultData, plan.resultData) ||
		!bytes.Equal(fresh.returnData, plan.returnData) ||
		!bytes.Equal(fresh.submissionData, plan.submissionData) {
		return TransportReturnPlan{}, fmt.Errorf("external session transport return changed after preview; rerun WhatIf")
	}
	writes := []struct {
		path  string
		label string
		data  []byte
	}{
		{fresh.ResultPath, "Remote Control reviewer result", fresh.resultData},
		{fresh.ReturnReceiptPath, "Remote Control return receipt", fresh.returnData},
		{fresh.SubmissionPath, "Remote Control external session submission", fresh.submissionData},
	}
	firstMissing := len(writes)
	for index, write := range writes {
		full := filepath.Join(fresh.Job.CaseRoot, filepath.FromSlash(write.path))
		info, statErr := os.Lstat(full)
		if os.IsNotExist(statErr) {
			if firstMissing == len(writes) {
				firstMissing = index
			}
			continue
		}
		if statErr != nil {
			return TransportReturnPlan{}, statErr
		}
		if firstMissing != len(writes) {
			return TransportReturnPlan{}, fmt.Errorf("Remote Control return publication is non-prefix at %s", write.path)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return TransportReturnPlan{}, fmt.Errorf("Remote Control return artifact must be a regular non-symlink file: %s", write.path)
		}
		data, readErr := rekitfs.ReadStableRegularFileAnchored(fresh.Job.CaseRoot, full, write.label, int64(len(write.data))+1)
		if readErr != nil || !bytes.Equal(data, write.data) {
			return TransportReturnPlan{}, fmt.Errorf("Remote Control existing return artifact differs: %s", write.path)
		}
	}
	alreadyApplied := firstMissing == len(writes)
	for index := firstMissing; index < len(writes); index++ {
		write := writes[index]
		if _, err := rekitfs.WriteExclusiveRegularFileAnchored(fresh.Job.CaseRoot, write.path, write.label, write.data); err != nil {
			return TransportReturnPlan{}, err
		}
	}
	fresh.Applied = true
	fresh.AlreadyApplied = alreadyApplied
	fresh.ReviewRequired = false
	fresh.RequiresConfirmation = false
	fresh.resultData = nil
	fresh.returnData = nil
	fresh.submissionData = nil
	return fresh, nil
}

func TransportReturnReceiptPath(jobID string, generation int) string {
	return filepath.ToSlash(filepath.Join(projectstate.LegacyDir, "external-session-transport", "returns", jobID, fmt.Sprintf("%06d.json", generation)))
}

func TransportReturnReceiptPathForCase(caseRoot, jobID string, generation int) (string, error) {
	return projectstate.Rel(caseRoot, "external-session-transport", "returns", jobID, fmt.Sprintf("%06d.json", generation))
}

func validateReviewerResultForJob(job Job, data []byte) (reviewerresult.Result, error) {
	if job.SessionKind != "reviewer" || job.Reviewer == nil {
		return reviewerresult.Result{}, fmt.Errorf("external ReviewerResult requires an exact reviewer job")
	}
	result, err := reviewerresult.Decode(data)
	if err != nil {
		return reviewerresult.Result{}, err
	}
	receipt, err := reviewersession.ReadDispatch(job.CaseRoot, job.Reviewer.DispatchPath, job.Reviewer.DispatchSHA256)
	if err != nil {
		return reviewerresult.Result{}, err
	}
	fields, err := reviewersession.OutputContractFields(job.CaseRoot, receipt)
	if err != nil {
		return reviewerresult.Result{}, err
	}
	if receipt.PacketID != job.Reviewer.PacketID || receipt.RouteID != job.Reviewer.RouteID ||
		receipt.ShardID != job.Reviewer.ShardID || !slices.Equal(receipt.Items, job.Reviewer.Items) ||
		receipt.DispatchID != job.Reviewer.DispatchID || receipt.ReviewerHarness != job.Reviewer.Harness ||
		receipt.ReviewerSession != job.Reviewer.Session || !slices.Equal(fields, job.Reviewer.OutputFields) {
		return reviewerresult.Result{}, fmt.Errorf("external reviewer job does not match exact durable dispatch and output contract")
	}
	if result.PacketID != receipt.PacketID || result.RouteID != receipt.RouteID || result.ShardID != receipt.ShardID ||
		!slices.Equal(result.Items, receipt.Items) || result.ReviewerSession != receipt.ReviewerSession {
		return reviewerresult.Result{}, fmt.Errorf("external reviewer result does not match exact dispatch packet, route, shard, items, and session")
	}
	if err := reviewersession.ValidateRouteOutput(fields, result.RouteOutput); err != nil {
		return reviewerresult.Result{}, err
	}
	return result, nil
}

func validateRemoteControlReturnLineage(job Job, attempt AttemptInspection, dispatch DispatchInspection, submission Submission, resultData []byte) error {
	if attempt.Current == nil || !IsRemoteControlAttempt(attempt.Current) || dispatch.State != "running" || dispatch.Launch == nil {
		return fmt.Errorf("Remote Control submission requires the exact current accepted launch lineage")
	}
	if submission.Outcome != "returned" {
		return fmt.Errorf("Remote Control transport v1 accepts only a returned ReviewerResult submission")
	}
	transport, err := InspectTransport(job, attempt, dispatch)
	if err != nil {
		return err
	}
	if transport.State != "delivery-accepted" || transport.Delivery == nil || transport.Endpoint == nil {
		return fmt.Errorf("Remote Control submission requires the exact accepted delivery lineage")
	}
	if err := ValidateRemoteControlLaunchTransition(transport, dispatch.Launch.Outcome, dispatch.Launch.Actor, dispatch.Launch.ObservedAt, dispatch.Launch.ActualHarness, dispatch.Launch.ActualSession, dispatch.Launch.Reason); err != nil {
		return err
	}
	expectedReturnPath, err := TransportReturnReceiptPathForCase(job.CaseRoot, job.JobID, attempt.Current.Generation)
	if err != nil {
		return err
	}
	if submission.TransportReturnReceiptPath != expectedReturnPath ||
		!validSHA(submission.TransportReturnReceiptSHA256) {
		return fmt.Errorf("Remote Control submission requires the exact transport return receipt binding")
	}
	returnData, err := readOptionalTransport(job.CaseRoot, submission.TransportReturnReceiptPath, "external session transport return receipt")
	if err != nil || returnData == nil || !strings.EqualFold(hash(returnData), submission.TransportReturnReceiptSHA256) {
		return fmt.Errorf("Remote Control transport return receipt is unavailable or changed")
	}
	var receipt TransportReturnReceipt
	if err := decodeCanonicalTransport(returnData, &receipt); err != nil {
		return fmt.Errorf("invalid Remote Control transport return receipt: %w", err)
	}
	if receipt.SchemaVersion != SchemaVersion || receipt.Kind != KindTransportReturnReceipt ||
		receipt.Binding != transport.Binding || receipt.BundlePath != transport.BundlePath ||
		!strings.EqualFold(receipt.BundleSHA256, transport.BundleSHA256) || receipt.BundleBytes != transport.BundleBytes ||
		!strings.EqualFold(receipt.EndpointSnapshotSHA256, transport.EndpointSHA256) ||
		!strings.EqualFold(receipt.EnvelopeSHA256, transport.EnvelopeSHA256) ||
		!strings.EqualFold(receipt.DeliverySHA256, transport.DeliverySHA256) ||
		!strings.EqualFold(receipt.LaunchReceiptSHA256, dispatch.LaunchSHA256) ||
		receipt.ResultPath != attempt.Current.SubmissionResult || !strings.EqualFold(receipt.ResultSHA256, hash(resultData)) ||
		receipt.ResultBytes != len(resultData) || receipt.Actor != submission.Actor ||
		!receipt.NoSessionManagement || !receipt.NoHeavyTool || !receipt.NoAuthority || !receipt.NoConfirmed {
		return fmt.Errorf("Remote Control transport return receipt does not match the exact delivery, launch, result, and submission lineage")
	}
	observed, err := canonicalTransportTime(receipt.ObservedAt, "return observedAt")
	if err != nil {
		return err
	}
	launched, _ := time.Parse(time.RFC3339Nano, dispatch.Launch.ObservedAt)
	if observed.Before(launched) {
		return fmt.Errorf("Remote Control transport return predates the accepted launch receipt")
	}
	if receipt.SourcePath == "" || !validSHA(receipt.SourceSHA256) || receipt.SourceBytes != receipt.ResultBytes || !strings.EqualFold(receipt.SourceSHA256, receipt.ResultSHA256) {
		return fmt.Errorf("Remote Control transport return source binding is invalid")
	}
	sourceFull, err := transportAnchoredPath(job.CaseRoot, receipt.SourcePath)
	if err != nil {
		return fmt.Errorf("Remote Control transport return source binding is invalid: %w", err)
	}
	sourceRel, err := transportRelativePath(job.CaseRoot, sourceFull)
	if err != nil || sourceRel != receipt.SourcePath || sourceRel == attempt.Current.SubmissionResult || sourceRel == submission.TransportReturnReceiptPath || sourceRel == attempt.Current.SubmissionPath {
		return fmt.Errorf("Remote Control transport return source path is not an exact independent case-local path")
	}
	sourceData, err := rekitfs.ReadStableRegularFileAnchored(job.CaseRoot, sourceFull, "Remote Control inbound ReviewerResult source", reviewerresult.MaxBytes)
	if err != nil || len(sourceData) != receipt.SourceBytes || !strings.EqualFold(hash(sourceData), receipt.SourceSHA256) || !bytes.Equal(sourceData, resultData) {
		return fmt.Errorf("Remote Control transport return source bytes are unavailable or changed")
	}
	if submission.ReviewerHarness != RemoteControlHarness || submission.ReviewerSession != transport.Binding.TransportBindingID {
		return fmt.Errorf("Remote Control submission reviewer identity does not match the durable transport binding")
	}
	if _, err := validateReviewerResultForJob(job, resultData); err != nil {
		return err
	}
	return nil
}
