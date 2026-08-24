package externalsession

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/capabilitycontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instructionpacket"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func TestDispatchedAttemptPublishesTicketBeforeCommitAndRecoversPrefix(t *testing.T) {
	job := dispatchTestJob(t)
	plan := dispatchTestAttemptPlan(t, job)
	if _, err := ApplyAttempt(plan, plan.JobSHA256, strings.Repeat("0", 64)); err == nil {
		t.Fatal("mismatched plan hash should reject before publication")
	}
	if _, err := os.Stat(filepath.Join(job.CaseRoot, filepath.FromSlash(plan.DispatchPath))); !os.IsNotExist(err) {
		t.Fatalf("rejected apply wrote ticket: %v", err)
	}
	writeTestFile(t, job.CaseRoot, plan.DispatchPath, plan.dispatchData)
	beforeCommit, err := InspectDispatch(job, AttemptInspection{State: "ready"})
	if err != nil || beforeCommit.State != "attempt-publication-pending" || beforeCommit.TicketSHA256 != plan.DispatchSHA256 {
		t.Fatalf("pending dispatch=%+v err=%v", beforeCommit, err)
	}
	recovered, err := PendingAttemptPlan(job, beforeCommit)
	if err != nil || recovered.ExpectedPlanSHA256 != plan.ExpectedPlanSHA256 || !bytes.Equal(recovered.data, plan.data) {
		t.Fatalf("pending recovery=%+v err=%v", recovered, err)
	}
	applied, err := ApplyAttempt(recovered, recovered.JobSHA256, recovered.ExpectedPlanSHA256)
	if err != nil || !applied.Applied || applied.AlreadyApplied {
		t.Fatalf("recover ticket-only prefix=%+v err=%v", applied, err)
	}
	attempt, err := InspectAttempt(job)
	if err != nil || attempt.Current == nil {
		t.Fatalf("committed attempt=%+v err=%v", attempt, err)
	}
	dispatch, err := InspectDispatch(job, attempt)
	if err != nil || dispatch.State != "queued" || dispatch.TicketSHA256 != plan.DispatchSHA256 {
		t.Fatalf("dispatch=%+v err=%v", dispatch, err)
	}
	replayPlan, err := ResolveAttemptApplyPlanUnbound(job, plan.Attempt.Harness, plan.Attempt.Session, plan.Attempt.Actor, plan.Attempt.StartedAt, "")
	if err != nil {
		t.Fatal(err)
	}
	replayPlan, err = BindAttemptDispatch(replayPlan, *plan.Dispatch)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := ApplyAttempt(replayPlan, replayPlan.JobSHA256, replayPlan.ExpectedPlanSHA256)
	if err != nil || !replay.AlreadyApplied {
		t.Fatalf("committed replay=%+v err=%v", replay, err)
	}
}

func TestBindAttemptDispatchRejectsCapabilityPolicyDrift(t *testing.T) {
	job := dispatchTestJob(t)
	plan := dispatchTestAttemptPlan(t, job)

	readOnly, err := capabilitycontract.Bind(capabilitycontract.ReadOnly())
	if err != nil {
		t.Fatal(err)
	}
	ticket := *plan.Dispatch
	ticket.Launch.Capability = readOnly
	if _, err := BindAttemptDispatch(plan, ticket); err == nil || !strings.Contains(err.Error(), "capability") {
		t.Fatalf("member dispatch accepted read-only capability drift: %v", err)
	}

	ticket = *plan.Dispatch
	ticket.Launch.ReadOnly = true
	if _, err := BindAttemptDispatch(plan, ticket); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("member dispatch accepted read-only projection drift: %v", err)
	}
}

func TestBindAttemptDispatchRejectsMissingProductionInstructionIdentity(t *testing.T) {
	caseRoot := externalSessionTestCaseRootWithStateDir(t, projectstate.CurrentDir)
	job, err := NewMemberJob(caseRoot, "web-security", testCheckpointSHA, "g000001-a000001-production", memberexecution.Owner{Lane: "analysis", Executor: "member-a", ExecutorGeneration: 1}, projectstate.CurrentRel("member", "manifest.json"), projectstate.CurrentRel("member", "outputs"), []string{"returned"})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateDispatchInstructionIdentity(job, nil); err == nil || !strings.Contains(err.Error(), "omitted its durable instruction identity") {
		t.Fatalf("current production dispatch without instruction identity was accepted: %v", err)
	}
}

func TestBindAttemptDispatchRejectsInvalidInstructionIdentity(t *testing.T) {
	job := dispatchTestJob(t)
	writeTestFile(t, job.CaseRoot, ".rekit/member/handoff.json", []byte("dispatch test handoff\n"))
	plan, err := PreviewAttempt(job, "claude-code", "owner-session-a", "mission-commander", "2026-08-05T04:00:00Z", "")
	if err != nil {
		t.Fatal(err)
	}
	templates := make([]DispatchSubmissionTemplate, 0, len(plan.Attempt.AllowedOutcomes))
	for _, outcome := range plan.Attempt.AllowedOutcomes {
		templates = append(templates, DispatchSubmissionTemplate{Outcome: outcome, JSON: "{\"outcome\":\"" + outcome + "\"}\n", RequiredWrites: []string{plan.Attempt.SubmissionPath + " (last)"}})
	}
	identity := instructionpacket.Identity{SchemaVersion: instructionpacket.SchemaVersion, Pack: job.Pack, Mode: instructionpacket.ModePolicyOnly, ReceiptKind: "fixture-result", SHA256: strings.Repeat("a", 64)}
	_, err = BindAttemptDispatch(plan, DispatchTicket{
		Launch:               DispatchLaunch{Ready: true, Tool: "Claude Code session", AgentType: "durable-member-executor", Capability: job.Capability, Input: DispatchInput{Path: ".rekit/member/handoff.json", SHA256: hash([]byte("dispatch test handoff\n")), Role: "durable-member-handoff"}, ExpectedOutput: "bounded result", InstructionIdentity: &identity, Boundary: []string{"claim first"}},
		Return:               DispatchReturn{SubmissionPath: plan.Attempt.SubmissionPath, SubmissionOutputs: plan.Attempt.SubmissionOutputs, SubmissionLast: true, Templates: templates, Boundary: []string{"submission last"}},
		RefreshStatusCommand: "/rekit status", Boundary: []string{"no heavy tool"},
	})
	if err == nil || !strings.Contains(err.Error(), "instruction identity") {
		t.Fatalf("invalid instruction identity was accepted: %v", err)
	}
}

func TestDispatchedAttemptRejectsReceiptWithoutTicketAndTicketTamper(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(t *testing.T, job Job, plan AttemptPlan)
	}{
		{name: "receipt-without-ticket", mutate: func(t *testing.T, job Job, plan AttemptPlan) {
			writeTestFile(t, job.CaseRoot, plan.AttemptPath, plan.data)
		}},
		{name: "different-ticket", mutate: func(t *testing.T, job Job, plan AttemptPlan) {
			writeTestFile(t, job.CaseRoot, plan.DispatchPath, []byte("{}\n"))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			job := dispatchTestJob(t)
			plan := dispatchTestAttemptPlan(t, job)
			test.mutate(t, job, plan)
			if _, err := ApplyAttempt(plan, plan.JobSHA256, plan.ExpectedPlanSHA256); err == nil {
				t.Fatal("non-prefix or different ticket should fail closed")
			}
		})
	}
}

func TestCommittedDispatchAttemptRejectsMissingOrRewrittenTicket(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(t *testing.T, job Job, plan AttemptPlan)
	}{
		{name: "missing", mutate: func(t *testing.T, job Job, plan AttemptPlan) {
			if err := os.Remove(filepath.Join(job.CaseRoot, filepath.FromSlash(plan.DispatchPath))); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "canonical-rewrite", mutate: func(t *testing.T, job Job, plan AttemptPlan) {
			rewritten := *plan.Dispatch
			rewritten.Boundary = append(append([]string{}, rewritten.Boundary...), "rewritten after commit")
			data, err := canonical(rewritten)
			if err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, job.CaseRoot, plan.DispatchPath, data)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			job := dispatchTestJob(t)
			plan := dispatchTestAttemptPlan(t, job)
			if _, err := ApplyAttempt(plan, plan.JobSHA256, plan.ExpectedPlanSHA256); err != nil {
				t.Fatal(err)
			}
			attempt, err := InspectAttempt(job)
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(t, job, plan)
			if _, err := InspectDispatch(job, attempt); err == nil {
				t.Fatal("committed dispatcher attempt accepted a missing or rewritten ticket")
			}
		})
	}
}

func TestDispatchRequiredJobCannotDowngradeToLegacy(t *testing.T) {
	job := dispatchTestJob(t)
	job.DispatchRequired = true
	plan := dispatchTestAttemptPlan(t, job)
	if _, err := ApplyAttempt(plan, plan.JobSHA256, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(job.CaseRoot, filepath.FromSlash(plan.DispatchPath))); err != nil {
		t.Fatal(err)
	}
	legacyData, err := canonical(attemptEnvelope{SchemaVersion: SchemaVersion, Kind: KindAttempt, Attempt: plan.Attempt})
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, job.CaseRoot, plan.AttemptPath, legacyData)
	attempt, err := InspectAttempt(job)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := InspectDispatch(job, attempt); err == nil || !strings.Contains(err.Error(), "missing its bound dispatch ticket") {
		t.Fatalf("dispatch-required job downgraded to legacy: %v", err)
	}
}

func TestPendingReplacementDoesNotMaskCommittedTicketDamage(t *testing.T) {
	for _, mutate := range []func(t *testing.T, job Job, plan AttemptPlan){
		func(t *testing.T, job Job, plan AttemptPlan) {
			if err := os.Remove(filepath.Join(job.CaseRoot, filepath.FromSlash(plan.DispatchPath))); err != nil {
				t.Fatal(err)
			}
		},
		func(t *testing.T, job Job, plan AttemptPlan) {
			rewritten := *plan.Dispatch
			rewritten.Boundary = append(append([]string{}, rewritten.Boundary...), "rewritten")
			data, err := canonical(rewritten)
			if err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, job.CaseRoot, plan.DispatchPath, data)
		},
	} {
		job := dispatchTestJob(t)
		first := dispatchTestAttemptPlan(t, job)
		if _, err := ApplyAttempt(first, first.JobSHA256, first.ExpectedPlanSHA256); err != nil {
			t.Fatal(err)
		}
		attempt, err := InspectAttempt(job)
		if err != nil {
			t.Fatal(err)
		}
		replacement, err := PreviewAttempt(job, "claude-code", "owner-session-b", "replacement", "2026-08-05T04:00:01Z", attempt.AttemptSHA256)
		if err != nil {
			t.Fatal(err)
		}
		replacement = bindDispatchTestPlan(t, replacement)
		writeTestFile(t, job.CaseRoot, replacement.DispatchPath, replacement.dispatchData)
		mutate(t, job, first)
		if _, err := InspectDispatch(job, attempt); err == nil {
			t.Fatal("pending replacement masked committed ticket damage")
		}
	}
}

func TestPendingReplacementDoesNotInvalidateCurrentSubmissionLineage(t *testing.T) {
	job := dispatchTestJob(t)
	job.DispatchRequired = true
	first := dispatchTestAttemptPlan(t, job)
	if _, err := ApplyAttempt(first, first.JobSHA256, first.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	attempt, err := InspectAttempt(job)
	if err != nil {
		t.Fatal(err)
	}
	claim, err := PreviewDispatchTransition(job, attempt, "claimed", "dispatcher", "2026-08-05T04:00:00Z", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyDispatchTransitionCurrent(claim, claim.JobSHA256, claim.DispatchSHA256, "", claim.ExpectedPlanSHA256, nil); err != nil {
		t.Fatal(err)
	}
	launch, err := PreviewDispatchTransition(job, attempt, "accepted", "dispatcher", "2026-08-05T04:00:01Z", "actual-harness", "actual-session", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyDispatchTransitionCurrent(launch, launch.JobSHA256, launch.DispatchSHA256, launch.ExpectedClaimSHA256, launch.ExpectedPlanSHA256, nil); err != nil {
		t.Fatal(err)
	}
	current, err := InspectCurrentDispatch(job, attempt)
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := PreviewAttempt(job, "claude-code", "owner-session-b", "replacement", "2026-08-05T04:00:02Z", attempt.AttemptSHA256)
	if err != nil {
		t.Fatal(err)
	}
	replacement = bindDispatchTestPlan(t, replacement)
	writeTestFile(t, job.CaseRoot, replacement.DispatchPath, replacement.dispatchData)
	submission := Submission{
		SchemaVersion: SchemaVersion, Kind: KindSubmission, JobID: job.JobID, JobSHA256: first.JobSHA256,
		Capability: job.Capability, Outcome: "failed", Actor: "actual-harness", ObservedAt: "2026-08-05T04:00:03Z", Reason: "bounded failure",
		AttemptID: attempt.Current.AttemptID, AttemptSHA256: attempt.AttemptSHA256,
		DispatchClaimSHA256: current.ClaimSHA256, LaunchReceiptSHA256: current.LaunchSHA256,
		Harness: "actual-harness", Session: "actual-session", NoAuthorityOrConfirmed: true, NoHeavyTool: true,
	}
	writeTestJSON(t, job.CaseRoot, attempt.Current.SubmissionPath, submission)
	inspection, err := Inspect(job)
	if err != nil || inspection.State != "submission-ready" {
		t.Fatalf("pending replacement invalidated current submission: %+v err=%v", inspection, err)
	}
}

func TestDispatchClaimIsExclusiveUnderConcurrency(t *testing.T) {
	job := dispatchTestJob(t)
	plan := dispatchTestAttemptPlan(t, job)
	if _, err := ApplyAttempt(plan, plan.JobSHA256, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	attempt, err := InspectAttempt(job)
	if err != nil {
		t.Fatal(err)
	}
	plans := make([]DispatchPlan, 2)
	for index, actor := range []string{"dispatcher-a", "dispatcher-b"} {
		plans[index], err = PreviewDispatchTransition(job, attempt, "claimed", actor, "2026-08-05T04:00:00Z", "", "", "")
		if err != nil {
			t.Fatal(err)
		}
	}
	var wait sync.WaitGroup
	errs := make([]error, len(plans))
	for index := range plans {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			candidate := plans[index]
			_, errs[index] = ApplyDispatchTransitionCurrent(candidate, candidate.JobSHA256, candidate.DispatchSHA256, "", candidate.ExpectedPlanSHA256, nil)
		}(index)
	}
	wait.Wait()
	succeeded := 0
	for _, applyErr := range errs {
		if applyErr == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("exclusive concurrent claim successes=%d errors=%v", succeeded, errs)
	}
	claimed, err := InspectDispatch(job, attempt)
	if err != nil || claimed.State != "claimed" || claimed.Claim == nil {
		t.Fatalf("claimed=%+v err=%v", claimed, err)
	}
	winner := plans[0]
	if claimed.Claim.Actor == plans[1].Actor {
		winner = plans[1]
	}
	replayed, err := ApplyDispatchTransitionCurrent(winner, winner.JobSHA256, winner.DispatchSHA256, "", winner.ExpectedPlanSHA256, nil)
	if err != nil || !replayed.AlreadyApplied {
		t.Fatalf("winner replay=%+v err=%v", replayed, err)
	}
}

func TestDispatchReplayRejectsDependencyDrift(t *testing.T) {
	job := dispatchTestJob(t)
	plan := dispatchTestAttemptPlan(t, job)
	if _, err := ApplyAttempt(plan, plan.JobSHA256, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	attempt, err := InspectAttempt(job)
	if err != nil {
		t.Fatal(err)
	}
	claim, err := PreviewDispatchTransition(job, attempt, "claimed", "dispatcher", "2026-08-05T04:00:00Z", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyDispatchTransitionCurrent(claim, claim.JobSHA256, claim.DispatchSHA256, "", claim.ExpectedPlanSHA256, nil); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, job.CaseRoot, plan.Dispatch.Launch.Input.Path, []byte("drifted after claim\n"))
	if _, err := ApplyDispatchTransitionCurrent(claim, claim.JobSHA256, claim.DispatchSHA256, "", claim.ExpectedPlanSHA256, nil); err == nil || !strings.Contains(err.Error(), "immutable sha256") {
		t.Fatalf("claim replay masked input drift: %v", err)
	}
}

func TestDispatchRejectsInputDriftAndArtifactTamper(t *testing.T) {
	job := dispatchTestJob(t)
	plan := dispatchTestAttemptPlan(t, job)
	if _, err := ApplyAttempt(plan, plan.JobSHA256, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	attempt, _ := InspectAttempt(job)
	writeTestFile(t, job.CaseRoot, plan.Dispatch.Launch.Input.Path, []byte("changed after ticket publication\n"))
	if _, err := PreviewDispatchTransition(job, attempt, "claimed", "dispatcher", "2026-08-05T04:00:00Z", "", "", ""); err == nil || !strings.Contains(err.Error(), "immutable sha256") {
		t.Fatalf("input drift error=%v", err)
	}

	writeTestFile(t, job.CaseRoot, plan.Dispatch.Launch.Input.Path, []byte("dispatch test handoff\n"))
	claim, err := PreviewDispatchTransition(job, attempt, "claimed", "dispatcher", "2026-08-05T04:00:00Z", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyDispatchTransitionCurrent(claim, claim.JobSHA256, claim.DispatchSHA256, "", claim.ExpectedPlanSHA256, nil); err != nil {
		t.Fatal(err)
	}
	claimPath := filepath.Join(job.CaseRoot, filepath.FromSlash(claim.ArtifactPath))
	claimData, err := os.ReadFile(claimPath)
	if err != nil {
		t.Fatal(err)
	}
	claimData[len(claimData)-2] ^= 1
	if err := os.WriteFile(claimPath, claimData, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectDispatch(job, attempt); err == nil || !strings.Contains(err.Error(), "invalid external session dispatch claim") {
		t.Fatalf("tampered claim error=%v", err)
	}
}

func TestDispatchRejectsNonCanonicalAndUnsafeArtifacts(t *testing.T) {
	for _, test := range []struct {
		name string
		data []byte
	}{
		{name: "unknown-field", data: []byte("{\"unknown\":true}\n")},
		{name: "trailing-object", data: []byte("{}\n{}\n")},
		{name: "non-canonical", data: []byte("{}")},
	} {
		t.Run(test.name, func(t *testing.T) {
			job := dispatchTestJob(t)
			attempt, err := InspectAttempt(job)
			if err != nil {
				t.Fatal(err)
			}
			path, err := dispatchTicketPath(job.CaseRoot, job.JobID, 1)
			if err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, job.CaseRoot, path, test.data)
			if _, err := InspectDispatch(job, attempt); err == nil {
				t.Fatal("malformed dispatch ticket should fail closed")
			}
		})
	}
	if runtime.GOOS == "windows" {
		t.Skip("non-admin Windows symlink creation is not guaranteed")
	}
	job := dispatchTestJob(t)
	attempt, err := InspectAttempt(job)
	if err != nil {
		t.Fatal(err)
	}
	ticketRel, err := dispatchTicketPath(job.CaseRoot, job.JobID, 1)
	if err != nil {
		t.Fatal(err)
	}
	ticketPath := filepath.Join(job.CaseRoot, filepath.FromSlash(ticketRel))
	if err := os.MkdirAll(filepath.Dir(ticketPath), 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(job.CaseRoot, "dispatch-target.json")
	if err := os.WriteFile(target, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, ticketPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := InspectDispatch(job, attempt); err == nil {
		t.Fatal("symlinked dispatch ticket should fail closed")
	}
}

func TestDispatchClaimLaunchTruthAndReplacementFence(t *testing.T) {
	job := dispatchTestJob(t)
	first := dispatchTestAttemptPlan(t, job)
	if _, err := ApplyAttempt(first, first.JobSHA256, first.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	attempt, _ := InspectAttempt(job)
	claim, err := PreviewDispatchTransition(job, attempt, "claimed", "dispatcher-a", "2026-08-05T04:01:00Z", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyDispatchTransitionCurrent(claim, claim.JobSHA256, claim.DispatchSHA256, "", claim.ExpectedPlanSHA256, func() (Job, error) { return job, nil }); err != nil {
		t.Fatal(err)
	}
	claimed, err := InspectDispatch(job, attempt)
	if err != nil || claimed.State != "claimed" || claimed.Claim == nil || claimed.Claim.Actor != "dispatcher-a" {
		t.Fatalf("claimed=%+v err=%v", claimed, err)
	}
	if _, err := PreviewDispatchTransition(job, attempt, "claimed", "dispatcher-b", "2026-08-05T04:01:01Z", "", "", ""); err == nil {
		t.Fatal("different claimant must not replace current claim")
	}
	if _, err := PreviewDispatchTransition(job, attempt, "accepted", "dispatcher-b", "2026-08-05T04:01:02Z", "claude-code", "wrong-owner-session", ""); err == nil {
		t.Fatal("actor other than the exclusive claim owner must not record launch truth")
	}
	accepted, err := PreviewDispatchTransition(job, attempt, "accepted", "dispatcher-a", "2026-08-05T04:02:00Z", "claude-code", "actual-session-a", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyDispatchTransitionCurrent(accepted, accepted.JobSHA256, accepted.DispatchSHA256, accepted.ExpectedClaimSHA256, accepted.ExpectedPlanSHA256, func() (Job, error) { return job, nil }); err != nil {
		t.Fatal(err)
	}
	running, err := InspectDispatch(job, attempt)
	if err != nil || running.State != "running" || running.Launch == nil || running.Launch.ActualSession != "actual-session-a" {
		t.Fatalf("running=%+v err=%v", running, err)
	}
	replacement, err := PreviewAttempt(job, "claude-code", "owner-session-b", "replacement", "2026-08-05T04:03:00Z", attempt.AttemptSHA256)
	if err != nil {
		t.Fatal(err)
	}
	replacement = bindDispatchTestPlan(t, replacement)
	if _, err := ApplyAttempt(replacement, replacement.JobSHA256, replacement.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyDispatchTransitionCurrent(accepted, accepted.JobSHA256, accepted.DispatchSHA256, accepted.ExpectedClaimSHA256, accepted.ExpectedPlanSHA256, func() (Job, error) { return job, nil }); err == nil || !strings.Contains(err.Error(), "no longer current") {
		t.Fatalf("stale generation launch replay error=%v", err)
	}
}

func TestDispatchedSubmissionUsesAcceptedActualIdentity(t *testing.T) {
	job := dispatchTestJob(t)
	plan := dispatchTestAttemptPlan(t, job)
	if _, err := ApplyAttempt(plan, plan.JobSHA256, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	attempt, err := InspectAttempt(job)
	if err != nil || attempt.Current == nil {
		t.Fatalf("attempt=%+v err=%v", attempt, err)
	}
	claim, err := PreviewDispatchTransition(job, attempt, "claimed", "dispatcher", "2026-08-05T04:10:00Z", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyDispatchTransitionCurrent(claim, claim.JobSHA256, claim.DispatchSHA256, "", claim.ExpectedPlanSHA256, nil); err != nil {
		t.Fatal(err)
	}
	accepted, err := PreviewDispatchTransition(job, attempt, "accepted", "dispatcher", "2026-08-05T04:11:00Z", "actual-harness", "actual-session", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyDispatchTransitionCurrent(accepted, accepted.JobSHA256, accepted.DispatchSHA256, accepted.ExpectedClaimSHA256, accepted.ExpectedPlanSHA256, nil); err != nil {
		t.Fatal(err)
	}
	dispatch, err := InspectDispatch(job, attempt)
	if err != nil || dispatch.State != "running" {
		t.Fatalf("dispatch=%+v err=%v", dispatch, err)
	}
	submission := Submission{
		SchemaVersion: SchemaVersion, Kind: KindSubmission, JobID: job.JobID, JobSHA256: plan.JobSHA256,
		Capability: job.Capability, Outcome: "failed", Actor: "actual-executor", ObservedAt: "2026-08-05T04:12:00Z", Reason: "bounded failure",
		AttemptID: attempt.Current.AttemptID, AttemptSHA256: attempt.AttemptSHA256,
		DispatchClaimSHA256: dispatch.ClaimSHA256, LaunchReceiptSHA256: dispatch.LaunchSHA256,
		Harness: "actual-harness", Session: "actual-session", NoAuthorityOrConfirmed: true, NoHeavyTool: true,
	}
	writeTestJSON(t, job.CaseRoot, attempt.Current.SubmissionPath, submission)
	inspection, err := Inspect(job)
	if err != nil || inspection.State != "submission-ready" {
		t.Fatalf("actual launch submission=%+v err=%v", inspection, err)
	}

	submission.Harness = plan.Attempt.Harness
	submission.Session = plan.Attempt.Session
	writeTestJSON(t, job.CaseRoot, attempt.Current.SubmissionPath, submission)
	inspection, err = Inspect(job)
	if err != nil || inspection.State != "invalid" || !strings.Contains(strings.Join(inspection.Warnings, " "), "accepted launch lineage") {
		t.Fatalf("reservation identity submission=%+v err=%v", inspection, err)
	}
}

func TestDispatchedRelayReplayRejectsLaunchInputDrift(t *testing.T) {
	job := dispatchTestJob(t)
	job.DispatchRequired = true
	plan := dispatchTestAttemptPlan(t, job)
	if _, err := ApplyAttempt(plan, plan.JobSHA256, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	attempt, err := InspectAttempt(job)
	if err != nil || attempt.Current == nil {
		t.Fatalf("attempt=%+v err=%v", attempt, err)
	}
	claim, err := PreviewDispatchTransition(job, attempt, "claimed", "dispatcher", "2026-08-05T04:20:00Z", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyDispatchTransitionCurrent(claim, claim.JobSHA256, claim.DispatchSHA256, "", claim.ExpectedPlanSHA256, nil); err != nil {
		t.Fatal(err)
	}
	accepted, err := PreviewDispatchTransition(job, attempt, "accepted", "dispatcher", "2026-08-05T04:20:01Z", "actual-harness", "actual-session", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyDispatchTransitionCurrent(accepted, accepted.JobSHA256, accepted.DispatchSHA256, accepted.ExpectedClaimSHA256, accepted.ExpectedPlanSHA256, nil); err != nil {
		t.Fatal(err)
	}
	dispatch, err := InspectCurrentDispatch(job, attempt)
	if err != nil || dispatch.State != "running" {
		t.Fatalf("dispatch=%+v err=%v", dispatch, err)
	}
	writeTestJSON(t, job.CaseRoot, attempt.Current.SubmissionPath, Submission{
		SchemaVersion: SchemaVersion, Kind: KindSubmission, JobID: job.JobID, JobSHA256: plan.JobSHA256,
		Capability: job.Capability, Outcome: "failed", Actor: "actual-executor", ObservedAt: "2026-08-05T04:20:02Z", Reason: "bounded failure",
		AttemptID: attempt.Current.AttemptID, AttemptSHA256: attempt.AttemptSHA256,
		DispatchClaimSHA256: dispatch.ClaimSHA256, LaunchReceiptSHA256: dispatch.LaunchSHA256,
		Harness: "actual-harness", Session: "actual-session", NoAuthorityOrConfirmed: true, NoHeavyTool: true,
	})
	relay, err := Preview(job)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyCurrent(relay, relay.JobSHA256, relay.SubmissionSHA256, relay.ExpectedPlanSHA256, func() (Job, error) { return job, nil }); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, job.CaseRoot, plan.Dispatch.Launch.Input.Path, []byte("drifted after relay publication\n"))
	if _, err := ApplyCurrent(relay, relay.JobSHA256, relay.SubmissionSHA256, relay.ExpectedPlanSHA256, func() (Job, error) { return job, nil }); err == nil || !strings.Contains(err.Error(), "immutable sha256") {
		t.Fatalf("committed relay replay masked launch input drift: %v", err)
	}
}

func TestDispatchLaunchOutcomeShape(t *testing.T) {
	job := dispatchTestJob(t)
	plan := dispatchTestAttemptPlan(t, job)
	if _, err := ApplyAttempt(plan, plan.JobSHA256, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	attempt, _ := InspectAttempt(job)
	claim, _ := PreviewDispatchTransition(job, attempt, "claimed", "dispatcher", "2026-08-05T05:00:00Z", "", "", "")
	if _, err := ApplyDispatchTransitionCurrent(claim, claim.JobSHA256, claim.DispatchSHA256, "", claim.ExpectedPlanSHA256, nil); err != nil {
		t.Fatal(err)
	}
	invalid := []struct {
		outcome, harness, session, reason string
	}{
		{"accepted", "", "", ""},
		{"accepted", "h", "s", "reason"},
		{"failed", "h", "s", "failed"},
		{"failed", "", "", ""},
	}
	for _, input := range invalid {
		if _, err := PreviewDispatchTransition(job, attempt, input.outcome, "dispatcher", "2026-08-05T05:01:00Z", input.harness, input.session, input.reason); err == nil {
			t.Fatalf("invalid launch shape accepted: %+v", input)
		}
	}
	failed, err := PreviewDispatchTransition(job, attempt, "failed", "dispatcher", "2026-08-05T05:01:00Z", "", "", "launch rejected")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyDispatchTransitionCurrent(failed, failed.JobSHA256, failed.DispatchSHA256, failed.ExpectedClaimSHA256, failed.ExpectedPlanSHA256, nil); err != nil {
		t.Fatal(err)
	}
	inspection, err := InspectDispatch(job, attempt)
	if err != nil || inspection.State != "launch-failed" {
		t.Fatalf("failed inspection=%+v err=%v", inspection, err)
	}
}

func dispatchTestJob(t *testing.T) Job {
	t.Helper()
	job, err := NewMemberJob(externalSessionTestCaseRoot(t), defaults.DefaultPack, testCheckpointSHA, "g000001-a000001-dispatch", memberexecution.Owner{Lane: "analysis", Executor: "member-a", ExecutorGeneration: 1}, ".rekit/member/manifest.json", ".rekit/member/outputs", []string{"returned", "failed"})
	if err != nil {
		t.Fatal(err)
	}
	return job
}

func dispatchTestAttemptPlan(t *testing.T, job Job) AttemptPlan {
	t.Helper()
	writeTestFile(t, job.CaseRoot, ".rekit/member/handoff.json", []byte("dispatch test handoff\n"))
	plan, err := PreviewAttempt(job, "claude-code", "owner-session-a", "mission-commander", "2026-08-05T04:00:00Z", "")
	if err != nil {
		t.Fatal(err)
	}
	return bindDispatchTestPlan(t, plan)
}

func bindDispatchTestPlan(t *testing.T, plan AttemptPlan) AttemptPlan {
	t.Helper()
	templates := make([]DispatchSubmissionTemplate, 0, len(plan.Attempt.AllowedOutcomes))
	for _, outcome := range plan.Attempt.AllowedOutcomes {
		templates = append(templates, DispatchSubmissionTemplate{Outcome: outcome, JSON: "{\"outcome\":\"" + outcome + "\"}\n", RequiredWrites: []string{plan.Attempt.SubmissionPath + " (last)"}})
	}
	bound, err := BindAttemptDispatch(plan, DispatchTicket{
		Launch:               DispatchLaunch{Ready: true, Tool: "Claude Code session", AgentType: "durable-member-executor", Capability: plan.Job.Capability, Input: DispatchInput{Path: ".rekit/member/handoff.json", SHA256: hash([]byte("dispatch test handoff\n")), Role: "durable-member-handoff"}, ExpectedOutput: "bounded result", Boundary: []string{"claim first"}},
		Return:               DispatchReturn{SubmissionPath: plan.Attempt.SubmissionPath, SubmissionOutputs: plan.Attempt.SubmissionOutputs, SubmissionLast: true, Templates: templates, Boundary: []string{"submission last"}},
		RefreshStatusCommand: "/rekit status", Boundary: []string{"no heavy tool"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return bound
}
