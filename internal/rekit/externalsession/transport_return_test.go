package externalsession

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoteControlReturnPublishesResultReceiptSubmissionThenRelays(t *testing.T) {
	fixture := acceptedRemoteControlFixture(t)
	sourceRel := remoteControlReviewerResultSource(t, fixture.job, "inbound/reviewer-result.json")

	plan, err := PreviewTransportReturn(
		fixture.job,
		sourceRel,
		remoteControlTestActor,
		"2026-08-12T05:05:00Z",
	)
	if err != nil {
		t.Fatal(err)
	}
	if plan.ResultPath == "" || plan.ReturnReceiptPath == "" || plan.SubmissionPath == "" ||
		plan.Submission.TransportReturnReceiptPath != plan.ReturnReceiptPath ||
		plan.Submission.ReviewerHarness != RemoteControlHarness ||
		plan.Submission.ReviewerSession != remoteControlTestSession ||
		!plan.ReturnReceipt.NoSessionManagement || !plan.ReturnReceipt.NoHeavyTool ||
		!plan.ReturnReceipt.NoAuthority || !plan.ReturnReceipt.NoConfirmed {
		t.Fatalf("return plan=%+v", plan)
	}
	applied, err := ApplyTransportReturnCurrent(plan, plan.ExpectedPlanSHA256, func() (Job, error) {
		return fixture.job, nil
	})
	if err != nil || !applied.Applied || applied.AlreadyApplied {
		t.Fatalf("return apply=%+v err=%v", applied, err)
	}
	for _, rel := range []string{plan.ResultPath, plan.ReturnReceiptPath, plan.SubmissionPath} {
		info, err := os.Lstat(filepath.Join(fixture.job.CaseRoot, filepath.FromSlash(rel)))
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("return artifact %s info=%v err=%v", rel, info, err)
		}
	}
	inspection, err := Inspect(fixture.job)
	if err != nil || inspection.State != "submission-ready" {
		t.Fatalf("inspection=%+v err=%v", inspection, err)
	}
	relay, err := Preview(fixture.job)
	if err != nil {
		t.Fatal(err)
	}
	if relay.ReviewerResult == nil || relay.ReviewerResult.SHA256 != plan.ResultSHA256 {
		t.Fatalf("relay=%+v", relay)
	}
	if _, err := ApplyCurrent(relay, relay.JobSHA256, relay.SubmissionSHA256, relay.ExpectedPlanSHA256, func() (Job, error) {
		return fixture.job, nil
	}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(
		readTestFile(t, fixture.job.CaseRoot, fixture.job.RelayResultPath),
		readTestFile(t, fixture.job.CaseRoot, plan.ResultPath),
	) {
		t.Fatal("relay changed the exact returned ReviewerResult bytes")
	}
}

func TestRemoteControlReturnApplyRecoversOnlyExactPrefix(t *testing.T) {
	for _, tt := range []struct {
		name       string
		prefix     int
		wantErr    string
		wantReplay bool
	}{
		{name: "result prefix", prefix: 1},
		{name: "result and receipt prefix", prefix: 2},
		{name: "complete replay", prefix: 3, wantReplay: true},
		{name: "non-prefix receipt", prefix: -2, wantErr: "non-prefix"},
		{name: "non-prefix submission", prefix: -3, wantErr: "non-prefix"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fixture := acceptedRemoteControlFixture(t)
			sourceRel := remoteControlReviewerResultSource(t, fixture.job, "inbound/reviewer-result.json")
			plan, err := PreviewTransportReturn(fixture.job, sourceRel, remoteControlTestActor, "2026-08-12T06:05:00Z")
			if err != nil {
				t.Fatal(err)
			}
			writes := []struct {
				path string
				data []byte
			}{
				{plan.ResultPath, plan.resultData},
				{plan.ReturnReceiptPath, plan.returnData},
				{plan.SubmissionPath, plan.submissionData},
			}
			if tt.prefix >= 0 {
				for index := 0; index < tt.prefix; index++ {
					writeTestFile(t, fixture.job.CaseRoot, writes[index].path, writes[index].data)
				}
			} else {
				index := -tt.prefix - 1
				writeTestFile(t, fixture.job.CaseRoot, writes[index].path, writes[index].data)
			}
			applied, err := ApplyTransportReturnCurrent(plan, plan.ExpectedPlanSHA256, nil)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error=%v want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || applied.AlreadyApplied != tt.wantReplay {
				t.Fatalf("applied=%+v err=%v", applied, err)
			}
		})
	}
}

func TestRemoteControlReturnRejectsIdentitySourceAndLineageDrift(t *testing.T) {
	fixture := acceptedRemoteControlFixture(t)
	sourceRel := remoteControlReviewerResultSource(t, fixture.job, "inbound/reviewer-result.json")
	if _, err := PreviewTransportReturn(fixture.job, sourceRel, "other-actor", "2026-08-12T07:05:00Z"); err == nil || !strings.Contains(err.Error(), "claim owner") {
		t.Fatalf("actor error=%v", err)
	}
	if _, err := PreviewTransportReturn(fixture.job, sourceRel, remoteControlTestActor, "2026-08-12T05:02:59Z"); err == nil || !strings.Contains(err.Error(), "predates") {
		t.Fatalf("time error=%v", err)
	}
	badResult := []byte(`{"packetId":"packet-a","routeId":"route-a","shardId":"shard-a","items":["wrong-item"],"reviewerSession":"remote-reviewer-binding-a","decision":"accept","confidence":"high","summary":"reviewed","evidenceRefs":["evidence:a"],"risks":[],"conflicts":[],"recommendedVerdict":"accept","routeOutput":{"item":"item-a","decision":"accept","candidate_path":"bounded-candidate"}}`)
	writeTestFile(t, fixture.job.CaseRoot, "inbound/wrong-result.json", badResult)
	if _, err := PreviewTransportReturn(fixture.job, "inbound/wrong-result.json", remoteControlTestActor, "2026-08-12T07:05:00Z"); err == nil || !strings.Contains(err.Error(), "exact dispatch") {
		t.Fatalf("result identity error=%v", err)
	}
	if _, err := PreviewTransportReturn(fixture.job, filepath.Join(fixture.job.CaseRoot, "..", "outside.json"), remoteControlTestActor, "2026-08-12T07:05:00Z"); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("source escape error=%v", err)
	}
	writeTestFile(t, fixture.job.CaseRoot, fixture.attempt.Current.SubmissionResult, readTestFile(t, fixture.job.CaseRoot, sourceRel))
	if _, err := PreviewTransportReturn(fixture.job, fixture.attempt.Current.SubmissionResult, remoteControlTestActor, "2026-08-12T07:05:00Z"); err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("canonical result source error=%v", err)
	}
}

func TestRemoteControlReturnRejectsSourceAndCurrentnessDriftAfterPreview(t *testing.T) {
	fixture := acceptedRemoteControlFixture(t)
	sourceRel := remoteControlReviewerResultSource(t, fixture.job, "inbound/reviewer-result.json")
	plan, err := PreviewTransportReturn(fixture.job, sourceRel, remoteControlTestActor, "2026-08-12T08:05:00Z")
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, fixture.job.CaseRoot, sourceRel, []byte("tampered\n"))
	if _, err := ApplyTransportReturnCurrent(plan, plan.ExpectedPlanSHA256, nil); err == nil {
		t.Fatal("return apply accepted a source changed after preview")
	}
	for _, rel := range []string{plan.ResultPath, plan.ReturnReceiptPath, plan.SubmissionPath} {
		if _, err := os.Lstat(filepath.Join(fixture.job.CaseRoot, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Fatalf("source drift published %s: %v", rel, err)
		}
	}

	fixture = acceptedRemoteControlFixture(t)
	sourceRel = remoteControlReviewerResultSource(t, fixture.job, "inbound/reviewer-result.json")
	plan, err = PreviewTransportReturn(fixture.job, sourceRel, remoteControlTestActor, "2026-08-12T08:05:00Z")
	if err != nil {
		t.Fatal(err)
	}
	stale := fixture.job
	stale.CheckpointSHA256 = strings.Repeat("9", 64)
	if _, err := ApplyTransportReturnCurrent(plan, plan.ExpectedPlanSHA256, func() (Job, error) {
		return stale, nil
	}); err == nil || !strings.Contains(err.Error(), "job is no longer current") {
		t.Fatalf("stale return job error=%v", err)
	}
}

func TestRemoteControlReturnRejectsSymlinkSource(t *testing.T) {
	fixture := acceptedRemoteControlFixture(t)
	realRel := remoteControlReviewerResultSource(t, fixture.job, "inbound/real-result.json")
	linkRel := "inbound/linked-result.json"
	if err := os.Symlink(
		filepath.Join(fixture.job.CaseRoot, filepath.FromSlash(realRel)),
		filepath.Join(fixture.job.CaseRoot, filepath.FromSlash(linkRel)),
	); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := PreviewTransportReturn(fixture.job, linkRel, remoteControlTestActor, "2026-08-12T08:05:00Z"); err == nil || (!strings.Contains(err.Error(), "regular") && !strings.Contains(err.Error(), "symlink") && !strings.Contains(err.Error(), "reparse")) {
		t.Fatalf("symlink return source error=%v", err)
	}
}

func TestRemoteControlRelayRejectsReturnReceiptAndSourceTamper(t *testing.T) {
	for _, tt := range []struct {
		name   string
		tamper func(t *testing.T, fixture remoteControlFixture, plan TransportReturnPlan)
		want   string
	}{
		{
			name: "receipt bytes",
			tamper: func(t *testing.T, fixture remoteControlFixture, plan TransportReturnPlan) {
				writeTestFile(t, fixture.job.CaseRoot, plan.ReturnReceiptPath, []byte("tampered\n"))
			},
			want: "return receipt",
		},
		{
			name: "source bytes",
			tamper: func(t *testing.T, fixture remoteControlFixture, plan TransportReturnPlan) {
				writeTestFile(t, fixture.job.CaseRoot, plan.SourcePath, []byte("tampered\n"))
			},
			want: "source bytes",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fixture := acceptedRemoteControlFixture(t)
			sourceRel := remoteControlReviewerResultSource(t, fixture.job, "inbound/reviewer-result.json")
			plan, err := PreviewTransportReturn(fixture.job, sourceRel, remoteControlTestActor, "2026-08-12T08:05:00Z")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ApplyTransportReturnCurrent(plan, plan.ExpectedPlanSHA256, nil); err != nil {
				t.Fatal(err)
			}
			tt.tamper(t, fixture, plan)
			if _, err := Preview(fixture.job); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("relay error=%v want %q", err, tt.want)
			}
		})
	}
}

func TestNonRemoteSubmissionCannotClaimTransportReturnReceipt(t *testing.T) {
	job := dispatchTestJob(t)
	plan := dispatchTestAttemptPlan(t, job)
	if _, err := ApplyAttempt(plan, plan.JobSHA256, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	attempt, err := InspectAttempt(job)
	if err != nil {
		t.Fatal(err)
	}
	claim, err := PreviewDispatchTransition(job, attempt, "claimed", "dispatcher", "2026-08-12T09:01:00Z", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyDispatchTransitionCurrent(claim, claim.JobSHA256, claim.DispatchSHA256, "", claim.ExpectedPlanSHA256, nil); err != nil {
		t.Fatal(err)
	}
	accepted, err := PreviewDispatchTransition(job, attempt, "accepted", "dispatcher", "2026-08-12T09:02:00Z", "claude-code", "local-session", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyDispatchTransitionCurrent(accepted, accepted.JobSHA256, accepted.DispatchSHA256, accepted.ExpectedClaimSHA256, accepted.ExpectedPlanSHA256, nil); err != nil {
		t.Fatal(err)
	}
	dispatch, err := InspectCurrentDispatch(job, attempt)
	if err != nil {
		t.Fatal(err)
	}
	writeTestJSON(t, job.CaseRoot, attempt.Current.SubmissionPath, Submission{
		SchemaVersion: SchemaVersion, Kind: KindSubmission, JobID: job.JobID, JobSHA256: plan.JobSHA256,
		Outcome: "failed", Actor: "executor", ObservedAt: "2026-08-12T09:03:00Z", Reason: "bounded failure",
		AttemptID: attempt.Current.AttemptID, AttemptSHA256: attempt.AttemptSHA256,
		DispatchClaimSHA256: dispatch.ClaimSHA256, LaunchReceiptSHA256: dispatch.LaunchSHA256,
		TransportReturnReceiptPath: ".rekit/fake-return.json", TransportReturnReceiptSHA256: strings.Repeat("f", 64),
		Harness: "claude-code", Session: "local-session", NoAuthorityOrConfirmed: true, NoHeavyTool: true,
	})
	inspection, err := Inspect(job)
	if err != nil || inspection.State != "invalid" || !strings.Contains(strings.Join(inspection.Warnings, " "), "cannot claim") {
		t.Fatalf("inspection=%+v err=%v", inspection, err)
	}
}

func acceptedRemoteControlFixture(t *testing.T) remoteControlFixture {
	t.Helper()
	fixture := newRemoteControlFixture(t)
	applyRemoteControlEndpoint(t, fixture)
	delivery, err := PreviewTransportDelivery(
		fixture.job,
		fixture.attempt,
		fixture.dispatch,
		"accepted",
		strings.Repeat("e", 64),
		remoteControlTestActor,
		"2026-08-12T05:03:00Z",
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyTransportCurrent(delivery, delivery.ExpectedPlanSHA256, nil); err != nil {
		t.Fatal(err)
	}
	transport, err := InspectTransport(fixture.job, fixture.attempt, fixture.dispatch)
	if err != nil {
		t.Fatal(err)
	}
	outcome, actor, observedAt, harness, session, reason, err := TransportLaunchTransition(transport)
	if err != nil {
		t.Fatal(err)
	}
	launch, err := PreviewDispatchTransition(fixture.job, fixture.attempt, outcome, actor, observedAt, harness, session, reason)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyDispatchTransitionCurrent(launch, launch.JobSHA256, launch.DispatchSHA256, launch.ExpectedClaimSHA256, launch.ExpectedPlanSHA256, nil); err != nil {
		t.Fatal(err)
	}
	dispatch, err := InspectCurrentDispatch(fixture.job, fixture.attempt)
	if err != nil || dispatch.State != "running" {
		t.Fatalf("dispatch=%+v err=%v", dispatch, err)
	}
	fixture.dispatch = dispatch
	return fixture
}

func remoteControlReviewerResultSource(t *testing.T, job Job, rel string) string {
	t.Helper()
	result := []byte(`{"packetId":"packet-a","routeId":"route-a","shardId":"shard-a","items":["` + job.Reviewer.Items[0] + `"],"reviewerSession":"` + remoteControlTestSession + `","decision":"accept","confidence":"high","summary":"reviewed","evidenceRefs":["evidence:a"],"risks":[],"conflicts":[],"recommendedVerdict":"accept","routeOutput":{"item":"item-a","decision":"accept","candidate_path":"bounded-candidate"}}`)
	writeTestFile(t, job.CaseRoot, rel, result)
	return rel
}
