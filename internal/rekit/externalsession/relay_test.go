package externalsession

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewerresult"
)

const testCheckpointSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestMemberRelayPublishesManifestOutputsAndObservationLast(t *testing.T) {
	caseRoot := externalSessionTestCaseRoot(t)
	job, err := NewMemberJob(caseRoot, defaults.DefaultPack, testCheckpointSHA, "g000001-a000001-1234567890abcdef", memberexecution.Owner{Lane: "analysis", Executor: "member-a", ExecutorGeneration: 1}, ".rekit/lanes/analysis/member-executions/attempt/result/manifest.json", ".rekit/lanes/analysis/member-executions/attempt/result/outputs", []string{"accepted", "returned", "failed"})
	if err != nil {
		t.Fatal(err)
	}
	inspection, err := Inspect(job)
	if err != nil || inspection.State != "awaiting-submission" {
		t.Fatalf("inspection=%+v err=%v", inspection, err)
	}
	writeTestFile(t, caseRoot, filepath.Join(job.SubmissionOutputs, "nested", "result.txt"), []byte("result\n"))
	submission := Submission{SchemaVersion: 1, Kind: KindSubmission, JobID: job.JobID, JobSHA256: inspection.JobSHA256, Outcome: "returned", Actor: "harness-a", ObservedAt: "2026-08-04T12:00:00.0000000Z", Summary: "member returned bounded evidence", ReviewerItemsPath: "nested/result.txt", NoAuthorityOrConfirmed: true, NoHeavyTool: true}
	writeTestSubmission(t, caseRoot, job, submission)
	plan, err := Preview(job)
	if err != nil {
		t.Fatal(err)
	}
	resultSnapshot := plan.MemberResultSnapshot()
	if plan.ExpectedPlanSHA256 == "" || len(plan.Artifacts) != 4 || plan.Observation.Path != job.ObservationPath || plan.Observation.SHA256 == "" || plan.Observation.Bytes < 1 || len(plan.Observation.Data()) != int(plan.Observation.Bytes) || resultSnapshot == nil || len(resultSnapshot.ManifestData) == 0 || string(resultSnapshot.Outputs["nested/result.txt"]) != "result\n" {
		t.Fatalf("plan=%+v snapshot=%+v", plan, resultSnapshot)
	}
	applied, err := Apply(plan, plan.JobSHA256, plan.SubmissionSHA256, plan.ExpectedPlanSHA256)
	if err != nil || !applied.Applied || applied.AlreadyApplied {
		t.Fatalf("applied=%+v err=%v", applied, err)
	}
	manifestBytes := readTestFile(t, caseRoot, job.MemberManifestPath)
	var manifest memberexecution.ResultManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil || len(manifest.Outputs) != 1 || manifest.Outputs[0].Path != "nested/result.txt" || manifest.ReviewerItemsPath != "nested/result.txt" {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
	if got := string(readTestFile(t, caseRoot, filepath.Join(job.MemberOutputsRoot, "nested", "result.txt"))); got != "result\n" {
		t.Fatalf("output=%q", got)
	}
	var envelope observationEnvelope
	if err := json.Unmarshal(readTestFile(t, caseRoot, job.ObservationPath), &envelope); err != nil || envelope.ObservationKind != "member-session-returned" || envelope.MemberAttemptID != job.MemberAttemptID {
		t.Fatalf("envelope=%+v err=%v", envelope, err)
	}
	replayed, err := Apply(plan, plan.JobSHA256, plan.SubmissionSHA256, plan.ExpectedPlanSHA256)
	if err != nil || !replayed.AlreadyApplied {
		t.Fatalf("replay=%+v err=%v", replayed, err)
	}
}

func TestMemberRelayRecoversExactPublishedPrefix(t *testing.T) {
	caseRoot := externalSessionTestCaseRoot(t)
	job, err := NewMemberJob(caseRoot, defaults.DefaultPack, testCheckpointSHA, "g000001-a000001-0123456789abcdef", memberexecution.Owner{Lane: "analysis", Executor: "member-a", ExecutorGeneration: 1}, ".rekit/member/manifest.json", ".rekit/member/outputs", []string{"returned"})
	if err != nil {
		t.Fatal(err)
	}
	inspection, err := Inspect(job)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, caseRoot, filepath.Join(job.SubmissionOutputs, "result.txt"), []byte("result\n"))
	writeTestSubmission(t, caseRoot, job, Submission{SchemaVersion: 1, Kind: KindSubmission, JobID: job.JobID, JobSHA256: inspection.JobSHA256, Outcome: "returned", Actor: "harness", ObservedAt: "2026-08-05T02:00:00Z", Summary: "done", NoAuthorityOrConfirmed: true, NoHeavyTool: true})
	plan, err := Preview(job)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.writes) < 4 {
		t.Fatalf("writes=%d", len(plan.writes))
	}
	writeTestFile(t, caseRoot, plan.writes[0].rel, plan.writes[0].data)
	applied, err := Apply(plan, plan.JobSHA256, plan.SubmissionSHA256, plan.ExpectedPlanSHA256)
	if err != nil || !applied.Applied || applied.AlreadyApplied {
		t.Fatalf("applied=%+v err=%v", applied, err)
	}
	for _, artifact := range plan.Artifacts {
		data := readTestFile(t, caseRoot, artifact.Path)
		if int64(len(data)) != artifact.Bytes || hash(data) != artifact.SHA256 {
			t.Fatalf("artifact drift: %+v", artifact)
		}
	}
	replayed, err := Apply(plan, plan.JobSHA256, plan.SubmissionSHA256, plan.ExpectedPlanSHA256)
	if err != nil || !replayed.AlreadyApplied {
		t.Fatalf("replayed=%+v err=%v", replayed, err)
	}
}

func TestMemberRelayRejectsNonPrefixPublicationBeforeWriting(t *testing.T) {
	for _, existingIndex := range []int{1, 2, 3} {
		t.Run(fmt.Sprintf("existing-%d", existingIndex), func(t *testing.T) {
			caseRoot := externalSessionTestCaseRoot(t)
			job, err := NewMemberJob(caseRoot, defaults.DefaultPack, testCheckpointSHA, "g000001-a000001-abcdef0123456789", memberexecution.Owner{Lane: "analysis", Executor: "member-a", ExecutorGeneration: 1}, ".rekit/member/manifest.json", ".rekit/member/outputs", []string{"returned"})
			if err != nil {
				t.Fatal(err)
			}
			inspection, err := Inspect(job)
			if err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, caseRoot, filepath.Join(job.SubmissionOutputs, "result.txt"), []byte("result\n"))
			writeTestSubmission(t, caseRoot, job, Submission{SchemaVersion: 1, Kind: KindSubmission, JobID: job.JobID, JobSHA256: inspection.JobSHA256, Outcome: "returned", Actor: "harness", ObservedAt: "2026-08-05T02:05:00Z", Summary: "done", NoAuthorityOrConfirmed: true, NoHeavyTool: true})
			plan, err := Preview(job)
			if err != nil {
				t.Fatal(err)
			}
			if existingIndex >= len(plan.writes) {
				t.Fatalf("write index %d exceeds %d", existingIndex, len(plan.writes))
			}
			writeTestFile(t, caseRoot, plan.writes[existingIndex].rel, plan.writes[existingIndex].data)
			if _, err := Apply(plan, plan.JobSHA256, plan.SubmissionSHA256, plan.ExpectedPlanSHA256); err == nil || !strings.Contains(err.Error(), "non-prefix") {
				t.Fatalf("non-prefix relay error=%v", err)
			}
			if _, err := os.Lstat(filepath.Join(caseRoot, filepath.FromSlash(plan.writes[0].rel))); !os.IsNotExist(err) {
				t.Fatalf("non-prefix preflight wrote earlier artifact: %v", err)
			}
		})
	}
}

func TestMemberRelayRejectsSourceDriftAndExistingDifferentDestination(t *testing.T) {
	caseRoot := externalSessionTestCaseRoot(t)
	job, err := NewMemberJob(caseRoot, defaults.DefaultPack, testCheckpointSHA, "g000001-a000001-fedcba0987654321", memberexecution.Owner{Lane: "analysis", Executor: "member-a", ExecutorGeneration: 1}, ".rekit/member/manifest.json", ".rekit/member/outputs", []string{"returned"})
	if err != nil {
		t.Fatal(err)
	}
	inspection, _ := Inspect(job)
	writeTestFile(t, caseRoot, filepath.Join(job.SubmissionOutputs, "result.txt"), []byte("before\n"))
	writeTestSubmission(t, caseRoot, job, Submission{SchemaVersion: 1, Kind: KindSubmission, JobID: job.JobID, JobSHA256: inspection.JobSHA256, Outcome: "returned", Actor: "harness", ObservedAt: "2026-08-04T12:00:00Z", Summary: "done", NoAuthorityOrConfirmed: true, NoHeavyTool: true})
	plan, err := Preview(job)
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := InspectAttempt(job)
	if err != nil || attempt.Current == nil {
		t.Fatalf("inspect source drift attempt: %+v err=%v", attempt, err)
	}
	writeTestFile(t, caseRoot, filepath.Join(attempt.Current.SubmissionOutputs, "result.txt"), []byte("after\n"))
	if _, err := Apply(plan, plan.JobSHA256, plan.SubmissionSHA256, plan.ExpectedPlanSHA256); err == nil || !strings.Contains(err.Error(), "relay plan sha256 mismatch") {
		t.Fatalf("source drift err=%v", err)
	}
	os.Remove(filepath.Join(caseRoot, filepath.FromSlash(job.SubmissionPath)))
	os.Remove(filepath.Join(caseRoot, filepath.FromSlash(job.SubmissionOutputs), "result.txt"))
}

func TestRelayApplyRejectsStaleLiveJobBeforePublication(t *testing.T) {
	caseRoot := externalSessionTestCaseRoot(t)
	job, err := NewReviewerJob(caseRoot, defaults.DefaultPack, testCheckpointSHA, ReviewerIdentity{AttemptSHA256: strings.Repeat("b", 64), PacketID: "packet-a", RouteID: "route-a", ShardID: "shard-a"}, []string{"accepted"})
	if err != nil {
		t.Fatal(err)
	}
	inspection, _ := Inspect(job)
	writeTestSubmission(t, caseRoot, job, Submission{SchemaVersion: 1, Kind: KindSubmission, JobID: job.JobID, JobSHA256: inspection.JobSHA256, Outcome: "accepted", Actor: "harness", ReviewerHarness: "harness", ReviewerSession: "session-test", NoAuthorityOrConfirmed: true, NoHeavyTool: true})
	plan, err := Preview(job)
	if err != nil {
		t.Fatal(err)
	}
	stale := job
	stale.Reviewer = &ReviewerIdentity{AttemptSHA256: strings.Repeat("c", 64), PacketID: "packet-a", RouteID: "route-a", ShardID: "shard-a"}
	if _, err := ApplyCurrent(plan, plan.JobSHA256, plan.SubmissionSHA256, plan.ExpectedPlanSHA256, func() (Job, error) { return stale, nil }); err == nil || !strings.Contains(err.Error(), "no longer current") {
		t.Fatalf("stale relay error=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, filepath.FromSlash(job.PublicationPath))); !os.IsNotExist(err) {
		t.Fatalf("stale relay published terminal receipt: %v", err)
	}
}

func TestReviewerRelayRejectsAcceptedDispatchSessionMismatch(t *testing.T) {
	caseRoot := externalSessionTestCaseRoot(t)
	job, err := NewReviewerJob(caseRoot, defaults.DefaultPack, testCheckpointSHA, ReviewerIdentity{AttemptSHA256: strings.Repeat("b", 64), PacketID: "packet-a", RouteID: "route-a", ShardID: "shard-a", DispatchID: "dispatch-a", Harness: "claude-code", Session: "session-a"}, []string{"returned"})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PreviewAttempt(job, "claude-code", "session-a", "harness", "2026-08-05T01:00:00Z", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyAttempt(plan, plan.JobSHA256, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	attempt, _ := InspectAttempt(job)
	writeTestJSON(t, caseRoot, attempt.Current.SubmissionPath, Submission{SchemaVersion: 1, Kind: KindSubmission, JobID: job.JobID, JobSHA256: plan.JobSHA256, Outcome: "returned", Actor: "harness", ReviewerSession: "session-b", AttemptID: attempt.Current.AttemptID, AttemptSHA256: attempt.AttemptSHA256, Harness: "claude-code", Session: "session-b", NoAuthorityOrConfirmed: true, NoHeavyTool: true})
	inspection, err := Inspect(job)
	if err != nil || inspection.State != "invalid" {
		t.Fatalf("dispatch mismatch inspection=%+v err=%v", inspection, err)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, filepath.FromSlash(job.PublicationPath))); !os.IsNotExist(err) {
		t.Fatalf("dispatch mismatch published terminal receipt: %v", err)
	}
}

func TestReviewerRelayValidatesIdentityAndPublishesCanonicalSource(t *testing.T) {
	caseRoot := externalSessionTestCaseRoot(t)
	reviewer := ReviewerIdentity{AttemptSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", PacketID: "packet-a", RouteID: "route-a", ShardID: "shard-a"}
	job, err := NewReviewerJob(caseRoot, defaults.DefaultPack, testCheckpointSHA, reviewer, []string{"accepted", "returned", "failed"})
	if err != nil {
		t.Fatal(err)
	}
	inspection, _ := Inspect(job)
	resultBytes := []byte(`{"packetId":"packet-a","routeId":"route-a","shardId":"shard-a","items":["item-a"],"reviewerSession":"session-test","decision":"accept","confidence":"high","summary":"reviewed","evidenceRefs":["evidence:a"],"risks":[],"conflicts":[],"recommendedVerdict":"accept","routeOutput":{}}`)
	if decoded, err := reviewerresult.Decode(resultBytes); err != nil || decoded.PacketID != reviewer.PacketID || decoded.RouteID != reviewer.RouteID || decoded.ShardID != reviewer.ShardID {
		t.Fatalf("fixture decode=%+v err=%v", decoded, err)
	}
	writeTestFile(t, caseRoot, job.SubmissionResult, resultBytes)
	writeTestSubmission(t, caseRoot, job, Submission{SchemaVersion: 1, Kind: KindSubmission, JobID: job.JobID, JobSHA256: inspection.JobSHA256, Outcome: "returned", Actor: "harness", ReviewerSession: "session-test", NoAuthorityOrConfirmed: true, NoHeavyTool: true})
	plan, err := Preview(job)
	if err != nil {
		t.Fatal(err)
	}
	if plan.ReviewerResult == nil || plan.ReviewerResult.Path != job.RelayResultPath || plan.ReviewerResult.SHA256 != hash(resultBytes) || plan.ReviewerResult.Bytes != int64(len(resultBytes)) || !bytes.Equal(plan.ReviewerResult.Data(), resultBytes) {
		t.Fatalf("reviewer result binding=%+v", plan.ReviewerResult)
	}
	copied := plan.ReviewerResult.Data()
	copied[0] ^= 0xff
	if !bytes.Equal(plan.ReviewerResult.Data(), resultBytes) {
		t.Fatal("reviewer result binding did not return a defensive copy")
	}
	if _, err := Apply(plan, plan.JobSHA256, plan.SubmissionSHA256, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, caseRoot, job.RelayResultPath); string(got) != string(readTestFile(t, caseRoot, job.SubmissionResult)) {
		t.Fatal("relay result bytes differ from submission result")
	}
	var envelope observationEnvelope
	if err := json.Unmarshal(readTestFile(t, caseRoot, job.ObservationPath), &envelope); err != nil || envelope.ObservationKind != "reviewer-result-returned" || envelope.ReviewerResultSourcePath != filepath.Join(caseRoot, filepath.FromSlash(job.RelayResultPath)) {
		t.Fatalf("envelope=%+v err=%v", envelope, err)
	}
}

func TestInspectRejectsUnknownSubmissionField(t *testing.T) {
	caseRoot := externalSessionTestCaseRoot(t)
	job, err := NewReviewerJob(caseRoot, defaults.DefaultPack, testCheckpointSHA, ReviewerIdentity{AttemptSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", PacketID: "packet-a", RouteID: "route-a", ShardID: "shard-a"}, []string{"accepted"})
	if err != nil {
		t.Fatal(err)
	}
	inspection, _ := Inspect(job)
	plan, err := PreviewAttempt(job, "claude", "session-test", "harness", "2026-08-05T01:00:00Z", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyAttempt(plan, plan.JobSHA256, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"schemaVersion":1,"kind":"current-loop-external-session-submission","jobId":"` + job.JobID + `","jobSha256":"` + inspection.JobSHA256 + `","outcome":"accepted","actor":"harness","attemptId":"` + plan.Attempt.AttemptID + `","attemptSha256":"` + hash(plan.data) + `","harness":"claude","session":"session-test","reviewerHarness":"claude","reviewerSession":"session-test","noAuthorityOrConfirmed":true,"noHeavyTool":true,"unexpected":true}`)
	writeTestFile(t, caseRoot, plan.Attempt.SubmissionPath, data)
	inspection, err = Inspect(job)
	if err != nil || inspection.State != "invalid" || len(inspection.Warnings) == 0 {
		t.Fatalf("inspection=%+v err=%v", inspection, err)
	}
}

func bindTestSubmissionAttempt(t *testing.T, job Job, submission *Submission) {
	t.Helper()
	inspection, err := InspectAttempt(job)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Current == nil {
		plan, err := PreviewAttempt(job, submission.Actor, "session-test", submission.Actor, "2026-08-05T01:00:00Z", "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ApplyAttempt(plan, plan.JobSHA256, plan.ExpectedPlanSHA256); err != nil {
			t.Fatal(err)
		}
		inspection, err = InspectAttempt(job)
		if err != nil {
			t.Fatal(err)
		}
	}
	submission.AttemptID = inspection.Current.AttemptID
	submission.AttemptSHA256 = inspection.AttemptSHA256
	submission.Harness = inspection.Current.Harness
	submission.Session = inspection.Current.Session
	if job.SessionKind == "member" {
		if paths, _ := rekitfs.WalkRegularFilesAnchored(job.CaseRoot, job.SubmissionOutputs, "test submission outputs", maxOutputs); len(paths) > 0 {
			for _, source := range paths {
				rel, _ := filepath.Rel(filepath.Join(job.CaseRoot, filepath.FromSlash(job.SubmissionOutputs)), source)
				writeTestFile(t, job.CaseRoot, filepath.Join(inspection.Current.SubmissionOutputs, rel), readTestFile(t, job.CaseRoot, filepath.ToSlash(strings.TrimPrefix(source, job.CaseRoot+string(filepath.Separator)))))
			}
		}
	} else if job.SubmissionResult != "" {
		if data, err := os.ReadFile(filepath.Join(job.CaseRoot, filepath.FromSlash(job.SubmissionResult))); err == nil {
			writeTestFile(t, job.CaseRoot, inspection.Current.SubmissionResult, data)
		}
	}
	if job.SessionKind == "reviewer" && submission.ReviewerSession != "" {
		submission.Session = submission.ReviewerSession
		if submission.Session != inspection.Current.Session {
			t.Fatalf("reviewer test session %s differs from attempt session %s", submission.Session, inspection.Current.Session)
		}
	}
}

func writeTestSubmission(t *testing.T, caseRoot string, job Job, submission Submission) {
	t.Helper()
	bindTestSubmissionAttempt(t, job, &submission)
	inspection, err := InspectAttempt(job)
	if err != nil || inspection.Current == nil {
		t.Fatalf("inspect test submission attempt: %+v err=%v", inspection, err)
	}
	writeTestJSON(t, caseRoot, inspection.Current.SubmissionPath, submission)
}

func externalSessionTestCaseRoot(t *testing.T) string {
	t.Helper()
	caseRoot := t.TempDir()
	writeTestFile(t, caseRoot, ".re-template.yml", []byte("template: true\n"))
	writeTestFile(t, caseRoot, ".rekit/instance.yml", []byte("projectName: test\n"))
	return caseRoot
}

func writeTestJSON(t *testing.T, caseRoot, rel string, value any) {
	t.Helper()
	data, err := canonical(value)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, caseRoot, rel, data)
}

func writeTestFile(t *testing.T, caseRoot, rel string, data []byte) {
	t.Helper()
	path := filepath.Join(caseRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, caseRoot, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(caseRoot, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
