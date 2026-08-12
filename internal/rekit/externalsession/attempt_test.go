package externalsession

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
)

func TestExternalSessionAttemptLifecycleAndStaleSubmission(t *testing.T) {
	caseRoot := externalSessionTestCaseRoot(t)
	job, err := NewMemberJob(caseRoot, defaults.DefaultPack, testCheckpointSHA, "g000001-a000001-attempt-lifecycle", memberexecution.Owner{Lane: "analysis", Executor: "member-a", ExecutorGeneration: 1}, ".rekit/member/manifest.json", ".rekit/member/outputs", []string{"returned", "failed"})
	if err != nil {
		t.Fatal(err)
	}
	ready, err := InspectAttempt(job)
	if err != nil || ready.State != "ready" || ready.Current != nil {
		t.Fatalf("ready=%+v err=%v", ready, err)
	}
	first, err := PreviewAttempt(job, "claude-code", "session-a", "mission-commander", "2026-08-05T03:00:00Z", "")
	if err != nil || first.ExpectedPlanSHA256 == "" || first.Attempt.Generation != 1 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	if _, err := ApplyAttempt(first, first.JobSHA256, first.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	running, err := InspectAttempt(job)
	if err != nil || running.State != "committed" || running.Current == nil || running.Current.Session != "session-a" {
		t.Fatalf("running=%+v err=%v", running, err)
	}
	if _, err := PreviewAttempt(job, "claude-code", "session-b", "replacement", "2026-08-05T03:01:00Z", strings.Repeat("0", 64)); err == nil {
		t.Fatal("replacement with stale attempt hash should fail")
	}
	replacement, err := PreviewAttempt(job, "claude-code", "session-b", "replacement", "2026-08-05T03:01:00Z", running.AttemptSHA256)
	if err != nil || replacement.Attempt.Generation != 2 || replacement.Attempt.SupersedesSHA256 != running.AttemptSHA256 {
		t.Fatalf("replacement=%+v err=%v", replacement, err)
	}
	if _, err := ApplyAttempt(replacement, replacement.JobSHA256, replacement.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	current, err := InspectAttempt(job)
	if err != nil || current.Generations != 2 || current.Current.Session != "session-b" {
		t.Fatalf("current=%+v err=%v", current, err)
	}
	stale := Submission{SchemaVersion: 1, Kind: KindSubmission, JobID: job.JobID, JobSHA256: replacement.JobSHA256, Outcome: "failed", Actor: "harness", ObservedAt: "2026-08-05T03:02:00Z", Reason: "late", AttemptID: first.Attempt.AttemptID, AttemptSHA256: first.ExpectedPlanSHA256, Harness: first.Attempt.Harness, Session: first.Attempt.Session, NoAuthorityOrConfirmed: true, NoHeavyTool: true}
	writeTestJSON(t, caseRoot, first.Attempt.SubmissionPath, stale)
	inspection, err := Inspect(job)
	if err != nil || inspection.State != "awaiting-submission" || inspection.Submission != nil {
		t.Fatalf("stale attempt-scoped submission affected current inspection=%+v err=%v", inspection, err)
	}
}

func TestExternalSessionAttemptApplyRejectsStaleJobAndReplaysCommittedReceipt(t *testing.T) {
	caseRoot := externalSessionTestCaseRoot(t)
	job, err := NewMemberJob(caseRoot, defaults.DefaultPack, testCheckpointSHA, "g000001-a000001-attempt-replay", memberexecution.Owner{Lane: "analysis", Executor: "member-a", ExecutorGeneration: 1}, ".rekit/member/manifest.json", ".rekit/member/outputs", []string{"returned"})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PreviewAttempt(job, "claude-code", "session-a", "mission-commander", "2026-08-05T03:00:00Z", "")
	if err != nil || plan.AttemptSHA256 == "" {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	stale := job
	owner := *stale.MemberOwner
	owner.ExecutorGeneration++
	stale.MemberOwner = &owner
	if _, err := ApplyAttemptCurrent(plan, plan.JobSHA256, plan.ExpectedPlanSHA256, func() (Job, error) { return stale, nil }); err == nil || !strings.Contains(err.Error(), "no longer current") {
		t.Fatalf("stale apply error=%v", err)
	}
	if inspection, err := InspectAttempt(job); err != nil || inspection.Current != nil {
		t.Fatalf("stale apply wrote attempt: %+v err=%v", inspection, err)
	}
	if _, err := ApplyAttemptCurrent(plan, plan.JobSHA256, plan.ExpectedPlanSHA256, func() (Job, error) { return job, nil }); err != nil {
		t.Fatal(err)
	}
	replay, err := ResolveAttemptApplyPlan(job, plan.Attempt.Harness, plan.Attempt.Session, plan.Attempt.Actor, plan.Attempt.StartedAt, "", plan.ExpectedPlanSHA256)
	if err != nil || replay.AttemptSHA256 != plan.AttemptSHA256 {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	applied, err := ApplyAttemptCurrent(replay, replay.JobSHA256, replay.ExpectedPlanSHA256, func() (Job, error) { return job, nil })
	if err != nil || !applied.Applied || !applied.AlreadyApplied || applied.AttemptSHA256 != plan.AttemptSHA256 {
		t.Fatalf("committed replay=%+v err=%v", applied, err)
	}
}

func TestExternalSessionAttemptRejectsReviewerDispatchSessionMismatch(t *testing.T) {
	caseRoot := externalSessionTestCaseRoot(t)
	job, err := NewReviewerJob(caseRoot, defaults.DefaultPack, testCheckpointSHA, ReviewerIdentity{AttemptSHA256: strings.Repeat("b", 64), PacketID: "packet", RouteID: "route", ShardID: "shard", Items: []string{"item"}, OutputFields: []string{"item"}, DispatchPath: ".rekit/dispatch.json", DispatchSHA256: strings.Repeat("c", 64), DispatchID: "dispatch-a", Harness: "claude-code", Session: "session-a"}, []string{"accepted", "failed"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PreviewAttempt(job, "claude-code", "session-b", "mission-commander", "2026-08-05T03:00:00Z", ""); err == nil || !strings.Contains(err.Error(), "accepted dispatch") {
		t.Fatalf("reviewer dispatch mismatch error=%v", err)
	}
	inspection, err := InspectAttempt(job)
	if err != nil || inspection.Current != nil {
		t.Fatalf("mismatched reviewer attempt wrote receipt: %+v err=%v", inspection, err)
	}
}

func TestExternalSessionAttemptRejectsDurableReviewerSameJobReplacement(t *testing.T) {
	caseRoot := externalSessionTestCaseRoot(t)
	job, err := NewReviewerJob(caseRoot, defaults.DefaultPack, testCheckpointSHA, ReviewerIdentity{AttemptSHA256: strings.Repeat("b", 64), PacketID: "packet", RouteID: "route", ShardID: "shard", Items: []string{"item"}, OutputFields: []string{"item"}, DispatchPath: ".rekit/dispatch.json", DispatchSHA256: strings.Repeat("c", 64), DispatchID: "dispatch-a", Harness: "claude-code", Session: "session-a"}, []string{"accepted", "failed"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := PreviewAttempt(job, "claude-code", "session-a", "mission-commander", "2026-08-05T03:00:00Z", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyAttempt(first, first.JobSHA256, first.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	inspection, err := InspectAttempt(job)
	if err != nil || inspection.Current == nil {
		t.Fatalf("first reviewer attempt=%+v err=%v", inspection, err)
	}
	if _, err := PreviewAttempt(job, "replacement-harness", "replacement-session", "mission-commander", "2026-08-05T03:01:00Z", inspection.AttemptSHA256); err == nil || !strings.Contains(err.Error(), "new Reviewer dispatch") {
		t.Fatalf("durable reviewer replacement error=%v", err)
	}
	current, err := InspectAttempt(job)
	if err != nil || current.Generations != 1 || current.AttemptSHA256 != inspection.AttemptSHA256 {
		t.Fatalf("durable reviewer replacement changed current attempt: %+v err=%v", current, err)
	}
}

func TestExternalSessionAttemptRejectsTamperAndSubmissionBeforeStart(t *testing.T) {
	caseRoot := externalSessionTestCaseRoot(t)
	job, err := NewReviewerJob(caseRoot, defaults.DefaultPack, testCheckpointSHA, ReviewerIdentity{AttemptSHA256: strings.Repeat("b", 64), PacketID: "packet", RouteID: "route", ShardID: "shard"}, []string{"accepted", "failed"})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PreviewAttempt(job, "harness", "session", "actor", "2026-08-05T03:00:00Z", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyAttempt(plan, plan.JobSHA256, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	writeTestJSON(t, caseRoot, plan.Attempt.SubmissionPath, Submission{SchemaVersion: 1, Kind: KindSubmission, JobID: job.JobID, JobSHA256: plan.JobSHA256, Outcome: "failed", Actor: "harness", ReviewerExitStatus: "failed", AttemptID: plan.Attempt.AttemptID, AttemptSHA256: hash(plan.data), Harness: plan.Attempt.Harness, Session: plan.Attempt.Session, NoAuthorityOrConfirmed: true, NoHeavyTool: true})
	if _, err := PreviewAttempt(job, "replacement", "session-2", "actor", "2026-08-05T03:01:00Z", hash(plan.data)); err == nil || !strings.Contains(err.Error(), "submission") {
		t.Fatalf("replacement after current submission error=%v", err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, filepath.FromSlash(plan.Attempt.SubmissionPath)), []byte("{invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	replacement, err := PreviewAttempt(job, "replacement", "session-2", "actor", "2026-08-05T03:01:00Z", hash(plan.data))
	if err != nil || replacement.Attempt.Generation != 2 {
		t.Fatalf("invalid current submission did not permit fenced replacement: %+v err=%v", replacement, err)
	}
	if err := os.Remove(filepath.Join(caseRoot, filepath.FromSlash(plan.Attempt.SubmissionPath))); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(caseRoot, filepath.FromSlash(plan.AttemptPath))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-2] ^= 1
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectAttempt(job); err == nil {
		t.Fatal("tampered attempt should fail closed")
	}
}
