package sessionhost

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/packmemoryconsumption"
	"github.com/shuiyu486/re-context-kits/internal/rekit/promote"
	"github.com/shuiyu486/re-context-kits/internal/rekit/releasecheck"
	"github.com/shuiyu486/re-context-kits/internal/rekit/subagents"
	rekitsync "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

const (
	packMemoryAcceptanceProducerOutput = "pack-memory-candidate.md"
	packMemoryAcceptanceManagedTarget  = "references/" + liveAcceptancePack + "/progressive-disclosure.md"
)

func runPackMemoryLiveAcceptanceLifecycle(
	parent context.Context,
	spec PackMemoryLiveAcceptanceChildSpec,
) (result PackMemoryLiveAcceptanceChildResult, retErr error) {
	result = PackMemoryLiveAcceptanceChildResult{
		SchemaVersion: 1,
		Kind:          packMemoryLiveAcceptanceChildKind,
		Pack:          liveAcceptancePack,
		Cleanup:       "pending",
		Boundary: []string{
			"producer, ReviewerResult, and consumer-use bytes must come from spawned Claude Code processes",
			"all candidate, verification, selected sync, and reconsume mutations remain inside the isolated kit",
			"no authority/confirmed state or heavy-tool execution is permitted",
		},
	}
	casesRoot := filepath.Join(spec.IsolatedKitRoot, ".rh07-cases")
	producerCase := filepath.Join(casesRoot, "producer")
	consumerCase := filepath.Join(casesRoot, "consumer")
	result.ProducerCase = producerCase
	result.ConsumerCase = consumerCase
	result.Cleanup = "owned-by-isolated-kit"
	if err := requirePackMemoryAcceptanceAbsent(casesRoot, producerCase, consumerCase); err != nil {
		return result, err
	}
	if err := os.Mkdir(casesRoot, 0o700); err != nil {
		return result, fmt.Errorf("create isolated pack-memory case root: %w", err)
	}

	consumerGoal := strings.Join([]string{
		"Consume the selected promoted pack memory that the runtime syncs before this member starts.",
		"Read references/" + liveAcceptancePack + "/progressive-disclosure.md and apply one exact checklist item added by the selected promoted change—not text already present in the predecessor—to explain a bounded reverse-engineering analysis approach.",
		"Write only consumer-use.json as a strict single JSON object with schemaVersion=1, kind=pack-memory-consumer-use-statement, the task-bound changeId and sourceSha256, an exact quote of at least 8 bytes from the accepted delta, a concrete non-empty appliedAs explanation, and noAuthority/noConfirmed/noHeavyTool all true.",
		"Do not write authority, confirmed state, pack sources, or heavy-tool output.",
	}, " ")
	consumerOnboarding := DailyResult{}
	consumerIntent, err := applyDailyOnboarding(consumerCase, consumerGoal, spec.Actor, &consumerOnboarding)
	if err != nil || !consumerIntent.Committed || !consumerOnboarding.OnboardingApplied {
		return result, fmt.Errorf("pre-onboard fresh pack-memory consumer before promoted generation: %w", err)
	}

	producerGoal := strings.Join([]string{
		strings.TrimSpace(spec.Goal),
		"Read the current case-local references/" + liveAcceptancePack + "/progressive-disclosure.md and produce a compatible complete successor.",
		"Preserve every existing pack-specific policy-registry route, bounded-review delegation gate, shard constraint, deny pattern, Stop hook rule, maintenance trigger, context-budget rule, and new-session recovery instruction.",
		"Add one generic, sanitized, cross-case reusable reverse-engineering checklist without weakening, deleting, or replacing those existing constraints.",
		"Write the complete successor Markdown document only to the strict member output path pack-memory-candidate.md.",
		"Do not include samples, traces, dumps, payloads, flags, customer data, absolute paths, addresses, case IDs, task IDs, or case-specific progress beyond the existing sanitized pack routing terms that must be preserved.",
		"Return the ordinary strict member result manifest; do not write pack files, authority, confirmed state, or heavy-tool output.",
	}, " ")
	producerDaily, err := RunDaily(parent, DailyOptions{
		Target:                            producerCase,
		Goal:                              producerGoal,
		Actor:                             spec.Actor,
		ClaudePath:                        spec.ClaudePath,
		ExpectedClaudeExecutableSHA256:    spec.ClaudeSHA256,
		ExpectedClaudeExecutablePublisher: spec.ClaudePublisher,
		Model:                             spec.Model,
		Timeout:                           time.Duration(spec.TimeoutNanos),
		MaxAttempts:                       spec.MaxAttempts,
		beforeMemberRun: func(caseRoot, pack, _ string) error {
			plan, err := rekitsync.Plan(spec.IsolatedKitRoot, caseRoot, pack)
			if err != nil {
				return err
			}
			if plan.Direction != "kit-to-case" || len(plan.Items) == 0 {
				return fmt.Errorf("producer managed-file sync preview is incomplete")
			}
			applied, err := rekitsync.Apply(spec.IsolatedKitRoot, caseRoot, pack, rekitsync.ApplyOptions{Command: "sync"})
			if err != nil {
				return err
			}
			managedPath := filepath.Join(caseRoot, filepath.FromSlash(packMemoryAcceptanceManagedTarget))
			if !applied.Applied || !rekitfs.SamePath(applied.CaseRoot, caseRoot) {
				return fmt.Errorf("producer managed-file sync did not apply to exact case")
			}
			_, err = rekitfs.ReadStableRegularFileAnchored(caseRoot, managedPath, "producer current managed pack memory", 1<<20)
			return err
		},
	})
	result.ProducerLaunches = producerDaily.SessionLaunches
	result.Failures = appendPackMemoryAcceptanceDailyFailures(
		result.Failures,
		"producer",
		producerDaily,
		spec,
	)
	if err != nil {
		return result, fmt.Errorf("run real pack-memory producer: %w", err)
	}
	if !producerDaily.OnboardingApplied || producerDaily.Pack != liveAcceptancePack || producerDaily.Lane == "" || producerDaily.FinalState != "member-intake-ready" || producerDaily.SessionLaunches < 1 || producerDaily.SessionCompletions < 1 {
		return result, fmt.Errorf("real pack-memory producer did not reach strict member intake: %+v", producerDaily)
	}
	inspection, output, err := packMemoryAcceptanceStrictOutput(producerCase, producerDaily.Lane, packMemoryAcceptanceProducerOutput)
	if err != nil {
		return result, err
	}
	rawPath, err := rekitfs.SafeJoin(inspection.OutputsRoot, output.Path)
	if err != nil {
		return result, err
	}
	rawOutput, err := rekitfs.ReadStableRegularFileAnchored(producerCase, rawPath, "pack-memory producer raw output", 1<<20)
	if err != nil {
		return result, err
	}
	predecessorPath := filepath.Join(spec.IsolatedKitRoot, "packs", liveAcceptancePack, filepath.FromSlash(packMemoryAcceptanceManagedTarget))
	predecessor, err := rekitfs.ReadStableRegularFileAnchored(spec.IsolatedKitRoot, predecessorPath, "pack-memory producer canonical predecessor", 1<<20)
	if err != nil {
		return result, err
	}
	if err := validatePackMemoryAcceptanceSanitization(predecessor, rawOutput); err != nil {
		return result, fmt.Errorf("real producer output introduced non-predecessor sanitization: %w", err)
	}
	result.Producer = PackMemoryLiveAcceptanceProducerProof{
		Lane:              producerDaily.Lane,
		AttemptID:         inspection.AttemptID,
		TaskContextSHA256: inspection.TaskContextSHA256,
		ManifestSHA256:    inspection.ManifestSHA256,
		OutputPath:        output.Path,
		OutputSHA256:      output.SHA256,
		ManagedTargetPath: packMemoryAcceptanceManagedTarget,
	}

	stagePreview, err := promote.StageMemberOutput(spec.IsolatedKitRoot, producerCase, liveAcceptancePack, promote.MemberOutputStagingOptions{
		Lane: producerDaily.Lane, AttemptID: inspection.AttemptID, OutputPath: output.Path,
		ManagedTargetPath: packMemoryAcceptanceManagedTarget, WhatIf: true,
	})
	if err != nil {
		return result, fmt.Errorf("preview real producer staging: %w", err)
	}
	staged, err := promote.StageMemberOutput(spec.IsolatedKitRoot, producerCase, liveAcceptancePack, promote.MemberOutputStagingOptions{
		Lane: producerDaily.Lane, AttemptID: inspection.AttemptID, OutputPath: output.Path,
		ManagedTargetPath: packMemoryAcceptanceManagedTarget, ExpectedPlanSHA256: stagePreview.PlanSHA256,
	})
	if err != nil {
		return result, fmt.Errorf("apply real producer staging: %w", err)
	}
	if !staged.Applied || staged.Mode != "staged-review-pending" || !staged.ReviewPending || !staged.RequiresReview || len(staged.DenyViolations) != 0 || !strings.EqualFold(staged.OutputSHA256, output.SHA256) || !packMemoryAcceptanceExpectedSanitization(staged.ReplacementCounts) {
		return result, fmt.Errorf("real producer staging did not preserve the strict raw-to-sanitized lineage: %+v", staged)
	}
	result.Producer.StagingPlanSHA256 = staged.PlanSHA256
	result.Producer.StagingReceiptSHA256 = staged.ReceiptSHA256
	result.Producer.SanitizedSHA256 = staged.SanitizedSHA256

	created, err := promote.CreateCandidates(spec.IsolatedKitRoot, producerCase, liveAcceptancePack, promote.CandidateOptions{})
	if err != nil {
		return result, fmt.Errorf("create staged pack-memory candidate: %w", err)
	}
	candidate, err := packMemoryAcceptanceCandidate(created, packMemoryAcceptanceManagedTarget)
	if err != nil {
		return result, err
	}
	candidateData, err := rekitfs.ReadStableRegularFileAnchored(created.CandidateRoot, candidate.CandidatePath, "pack-memory managed candidate", 1<<20)
	if err != nil {
		return result, err
	}
	candidateSHA256 := packMemoryAcceptanceBytesSHA256(candidateData)
	if !strings.EqualFold(candidateSHA256, staged.SanitizedSHA256) {
		return result, fmt.Errorf("sanitized and candidate sha256 do not match the strict producer lineage")
	}
	result.Producer.CandidateSHA256 = candidateSHA256

	created, err = promote.WriteCandidateReviewWorkspace(created, promote.CandidateArtifactOptions{
		ReviewOutputDir: filepath.Join(producerCase, ".rekit", "reviews", "rh07-pack-memory-candidate"),
	})
	if err != nil || created.ReviewWorkspace == nil {
		return result, fmt.Errorf("write candidate review workspace: %w", err)
	}
	reviewerEvidenceRef, err := filepath.Rel(producerCase, created.ReviewWorkspace.CombinedDiffPath)
	if err != nil || filepath.IsAbs(reviewerEvidenceRef) || reviewerEvidenceRef == "." || reviewerEvidenceRef == ".." || strings.HasPrefix(reviewerEvidenceRef, ".."+string(filepath.Separator)) {
		return result, fmt.Errorf("candidate review diff is not case-relative: %s", created.ReviewWorkspace.CombinedDiffPath)
	}
	reviewerPlan, err := subagents.WritePlan(spec.IsolatedKitRoot, producerCase, liveAcceptancePack, subagents.Options{
		TaskType: "feature-analysis", Items: filepath.ToSlash(reviewerEvidenceRef),
		ItemsPerAgent: 1, MaxParallel: 1, Lane: producerDaily.Lane,
	})
	if err != nil {
		return result, fmt.Errorf("plan independent real candidate reviewer: %w", err)
	}
	if reviewerPlan.ShardCount != 1 || len(reviewerPlan.ShardHandoffs) != 1 || reviewerPlan.PacketPath == "" {
		return result, fmt.Errorf("candidate reviewer plan is not one-shard and packet-bound: %+v", reviewerPlan)
	}
	reviewerHandoff := reviewerPlan.ShardHandoffs[0]
	if reviewerHandoff.ShardID == "" || reviewerHandoff.DispatchPromptPath == "" || reviewerHandoff.DispatchPromptSHA256 == "" {
		return result, fmt.Errorf("candidate reviewer plan omitted exact shard or dispatch prompt bindings: %+v", reviewerHandoff)
	}
	packetData, err := rekitfs.ReadStableRegularFileAnchored(filepath.Dir(reviewerPlan.PacketPath), reviewerPlan.PacketPath, "pack-memory reviewer packet", 1<<20)
	if err != nil {
		return result, err
	}
	var reviewerPacket struct {
		PacketID string `json:"packetId"`
	}
	if err := json.Unmarshal(packetData, &reviewerPacket); err != nil {
		return result, fmt.Errorf("decode candidate reviewer packet identity: %w", err)
	}
	if strings.TrimSpace(reviewerPacket.PacketID) == "" {
		return result, fmt.Errorf("candidate reviewer packet omitted packetId")
	}
	result.Reviewer.PacketSHA256 = packMemoryAcceptanceBytesSHA256(packetData)
	result.Reviewer.CandidateSHA256 = candidateSHA256

	reviewerHost, err := Run(parent, Options{
		Target: producerCase, Pack: liveAcceptancePack, Actor: spec.Actor,
		ClaudePath: spec.ClaudePath, ExpectedClaudeExecutableSHA256: spec.ClaudeSHA256,
		ExpectedClaudeExecutablePublisher: spec.ClaudePublisher, Model: spec.Model,
		Timeout: time.Duration(spec.TimeoutNanos), MaxAttempts: spec.MaxAttempts,
		reviewerBinding: &reviewerBinding{
			PacketID:             reviewerPacket.PacketID,
			PacketPath:           reviewerPlan.PacketPath,
			PacketSHA256:         result.Reviewer.PacketSHA256,
			Lane:                 reviewerPlan.TargetLane,
			ShardID:              reviewerHandoff.ShardID,
			DispatchPromptPath:   reviewerHandoff.DispatchPromptPath,
			DispatchPromptSHA256: reviewerHandoff.DispatchPromptSHA256,
		},
	})
	result.ReviewerLaunches = packMemoryAcceptanceReviewerLaunches(reviewerHost)
	result.Failures = appendPackMemoryAcceptanceHostFailures(
		result.Failures,
		"reviewer",
		reviewerHost,
		spec,
	)
	if err != nil {
		return result, fmt.Errorf("run independent real candidate reviewer: %w", err)
	}
	if result.ReviewerLaunches != 1 || reviewerHost.SessionCompletions < 1 {
		return result, fmt.Errorf("independent real candidate reviewer did not complete exactly one reviewer launch: %+v", reviewerHost)
	}
	intake, err := subagents.IntakeReadyReviewerResults(spec.IsolatedKitRoot, producerCase, liveAcceptancePack, subagents.ReviewerBatchIntakeOptions{
		PacketPath: reviewerPlan.PacketPath, Lane: producerDaily.Lane, Actor: spec.Actor, WhatIf: true,
	})
	if err != nil {
		return result, fmt.Errorf("revalidate real reviewer intake: %w", err)
	}
	if intake.Total != 1 || len(intake.Results) != 1 || intake.Stopped || intake.Partial {
		return result, fmt.Errorf("real reviewer intake is not complete and singular: %+v", intake)
	}
	reviewed := intake.Results[0]
	if reviewed.ReviewerResult.Decision != "accept" || reviewed.VerificationVerdict != "accepted" || reviewed.MainDecision != "accept" || reviewed.WritebackStatus != "complete" && reviewed.WritebackStatus != "already-complete" || reviewed.ReviewerSession == "" {
		return result, fmt.Errorf("real reviewer did not accept the exact candidate: %+v", reviewed)
	}
	reviewerResultData, err := rekitfs.ReadStableRegularFileAnchored(filepath.Dir(reviewed.ReviewerResultPath), reviewed.ReviewerResultPath, "pack-memory reviewer result", 1<<20)
	if err != nil {
		return result, err
	}
	result.Reviewer.ReviewerResultSHA256 = packMemoryAcceptanceBytesSHA256(reviewerResultData)
	result.Reviewer.ReviewerSession = reviewed.ReviewerSession
	result.Reviewer.Decision = reviewed.ReviewerResult.Decision
	result.Reviewer.VerificationVerdict = reviewed.VerificationVerdict
	result.Reviewer.MainDecision = reviewed.MainDecision
	result.Reviewer.WritebackStatus = reviewed.WritebackStatus

	decisionPath := filepath.Join(producerCase, ".rekit", "reviews", "rh07-pack-memory-candidate", "decisions.json")
	decisionEvidence := strings.Join([]string{created.ReviewWorkspace.CombinedDiffPath, reviewed.ReviewerResultPath}, ",")
	draftPreview, err := promote.DraftCandidateDecisions(spec.IsolatedKitRoot, producerCase, liveAcceptancePack, promote.CandidateDecisionDraftOptions{
		PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, Decision: "accept-managed-reject-tooling",
		Reason: "independent real reviewer accepted exact bounded reusable pack memory", Actor: spec.Actor,
		EvidenceRefs: decisionEvidence, WhatIf: true,
	})
	if err != nil {
		return result, fmt.Errorf("preview reviewed candidate decision draft: %w", err)
	}
	draftApplied, err := promote.DraftCandidateDecisions(spec.IsolatedKitRoot, producerCase, liveAcceptancePack, promote.CandidateDecisionDraftOptions{
		PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, Decision: "accept-managed-reject-tooling",
		Reason: "independent real reviewer accepted exact bounded reusable pack memory", Actor: spec.Actor,
		EvidenceRefs: decisionEvidence, ExpectedDecisionSHA256: draftPreview.DecisionSHA256,
	})
	if err != nil || !draftApplied.Applied || draftApplied.Accepted != 1 {
		return result, fmt.Errorf("apply reviewed candidate decision draft: %w", err)
	}
	result.Promotion.DecisionSHA256 = draftApplied.DecisionSHA256
	decisionPreview, err := promote.ApplyCandidateDecisions(spec.IsolatedKitRoot, producerCase, liveAcceptancePack, promote.CandidateDecisionOptions{
		PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, WhatIf: true,
	})
	if err != nil || decisionPreview.Accepted != 1 || decisionPreview.Applied {
		return result, fmt.Errorf("preview reviewed candidate decision: %w", err)
	}
	decisionApplied, err := promote.ApplyCandidateDecisions(spec.IsolatedKitRoot, producerCase, liveAcceptancePack, promote.CandidateDecisionOptions{
		PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath,
	})
	if err != nil || !decisionApplied.Applied || decisionApplied.Accepted != 1 || decisionApplied.Receipt == nil || !decisionApplied.Receipt.VerificationPending || decisionApplied.Receipt.VerificationWorkspaceRoot == "" {
		return result, fmt.Errorf("apply reviewed candidate decision: %w", err)
	}
	decisionReceiptData, err := rekitfs.ReadStableRegularFileAnchored(filepath.Dir(decisionApplied.ReceiptPath), decisionApplied.ReceiptPath, "pack-memory decision receipt", 1<<20)
	if err != nil {
		return result, err
	}
	result.Promotion.DecisionReceiptSHA256 = packMemoryAcceptanceBytesSHA256(decisionReceiptData)
	promotedData, err := rekitfs.ReadStableRegularFileAnchored(spec.IsolatedKitRoot, candidate.PackTarget, "promoted pack-memory source", 1<<20)
	if err != nil {
		return result, err
	}
	result.Promotion.PromotedSourceSHA256 = packMemoryAcceptanceBytesSHA256(promotedData)
	if !strings.EqualFold(result.Promotion.PromotedSourceSHA256, candidateSHA256) {
		return result, fmt.Errorf("reviewed candidate and promoted source sha256 do not match")
	}

	workspace := decisionApplied.Receipt.VerificationWorkspaceRoot
	freshRoot := filepath.Join(workspace, "fresh")
	attachedRoot := filepath.Join(workspace, "attached")
	provisionPreview, err := promote.ProvisionCandidateVerificationCases(spec.IsolatedKitRoot, producerCase, liveAcceptancePack, promote.CandidateVerificationProvisionOptions{
		PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath,
		FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, WhatIf: true,
	})
	if err != nil {
		return result, fmt.Errorf("preview candidate verification cases: %w", err)
	}
	provisionApplied, err := promote.ProvisionCandidateVerificationCases(spec.IsolatedKitRoot, producerCase, liveAcceptancePack, promote.CandidateVerificationProvisionOptions{
		PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath,
		FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: provisionPreview.ProvisionSHA256,
	})
	if err != nil || !provisionApplied.Applied || provisionApplied.Mode != "provisioned" || len(provisionApplied.Cases) != 2 || provisionApplied.Cases[0].DoctorRows == 0 || provisionApplied.Cases[1].DoctorRows == 0 {
		return result, fmt.Errorf("apply candidate verification cases: %w", err)
	}
	verificationPreview, err := promote.VerifyCandidateDecision(spec.IsolatedKitRoot, producerCase, liveAcceptancePack, promote.CandidateDecisionVerificationOptions{
		PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath,
		FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, WhatIf: true,
	})
	if err != nil || !verificationPreview.Ready || verificationPreview.Applied {
		return result, fmt.Errorf("preview candidate verification: %w", err)
	}
	verificationApplied, err := promote.VerifyCandidateDecision(spec.IsolatedKitRoot, producerCase, liveAcceptancePack, promote.CandidateDecisionVerificationOptions{
		PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath,
		FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot,
	})
	if err != nil || !verificationApplied.Applied || !verificationApplied.Ready || verificationApplied.VerificationProofPath == "" {
		return result, fmt.Errorf("apply candidate verification: %w", err)
	}
	verificationData, err := rekitfs.ReadStableRegularFileAnchored(spec.IsolatedKitRoot, verificationApplied.VerificationProofPath, "candidate verification proof", 1<<20)
	if err != nil {
		return result, err
	}
	result.Promotion.VerificationProofSHA256 = packMemoryAcceptanceBytesSHA256(verificationData)
	retirementPreview, err := promote.RetireCandidateVerificationWorkspace(spec.IsolatedKitRoot, producerCase, liveAcceptancePack, promote.CandidateVerificationRetirementOptions{
		PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, WhatIf: true,
	})
	if err != nil {
		return result, fmt.Errorf("preview candidate verification retirement: %w", err)
	}
	retirementApplied, err := promote.RetireCandidateVerificationWorkspace(spec.IsolatedKitRoot, producerCase, liveAcceptancePack, promote.CandidateVerificationRetirementOptions{
		PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath,
		ExpectedRetirementSHA256: retirementPreview.RetirementSHA256,
	})
	if err != nil || !retirementApplied.Applied || retirementApplied.Mode != "retired" || retirementApplied.Replay {
		return result, fmt.Errorf("apply candidate verification retirement: %w", err)
	}
	retirementIntentData, err := rekitfs.ReadStableRegularFileAnchored(spec.IsolatedKitRoot, retirementApplied.RetirementIntentPath, "candidate retirement intent", 1<<20)
	if err != nil {
		return result, err
	}
	retirementReceiptData, err := rekitfs.ReadStableRegularFileAnchored(spec.IsolatedKitRoot, retirementApplied.RetirementReceiptPath, "candidate retirement receipt", 1<<20)
	if err != nil {
		return result, err
	}
	result.Promotion.RetirementIntentSHA256 = packMemoryAcceptanceBytesSHA256(retirementIntentData)
	result.Promotion.RetirementReceiptSHA256 = packMemoryAcceptanceBytesSHA256(retirementReceiptData)

	proofRoot := filepath.Join(created.CandidateRoot, "review-artifacts")
	cleanupProof, err := packMemoryAcceptanceCleanupProof(spec.IsolatedKitRoot, producerCase, created.ReviewWorkspace.PacketPath, decisionPath, candidate.CandidatePath, proofRoot, spec.Actor)
	if err != nil {
		return result, err
	}
	result.Promotion.CleanupProofSHA256 = cleanupProof.ProofSHA256
	for _, proofType := range []string{
		"pack-doctor-output",
		"fresh-case-reconsume-proof",
		"attached-case-reconsume-proof",
	} {
		proof, err := packMemoryAcceptanceLifecycleProof(spec.IsolatedKitRoot, producerCase, created.ReviewWorkspace.PacketPath, candidate.CandidatePath, "", proofType, verificationApplied.VerificationProofPath, spec.Actor)
		if err != nil {
			return result, err
		}
		switch proofType {
		case "pack-doctor-output":
			result.Promotion.PackDoctorProofSHA256 = proof.ProofSHA256
		case "fresh-case-reconsume-proof":
			result.Promotion.FreshProofSHA256 = proof.ProofSHA256
		case "attached-case-reconsume-proof":
			result.Promotion.AttachedProofSHA256 = proof.ProofSHA256
		}
	}

	catalog, err := releasecheck.BuildCompletedPackMemoryChangeCatalog(spec.IsolatedKitRoot, liveAcceptancePack)
	if err != nil {
		return result, fmt.Errorf("build completed pack-memory change catalog: %w", err)
	}
	change, err := packMemoryAcceptanceCompletedChange(catalog, packMemoryAcceptanceManagedTarget, candidateSHA256)
	if err != nil {
		return result, err
	}
	result.Promotion.ChangeID = change.ChangeID
	result.Promotion.AuthoritySHA256 = change.AuthoritySHA256

	var consumptionPlanSHA256, consumptionReceiptSHA256, consumerBindingSHA256 string
	consumerDaily, err := RunDaily(parent, DailyOptions{
		Target: consumerCase, Goal: consumerGoal, Actor: spec.Actor,
		ClaudePath: spec.ClaudePath, ExpectedClaudeExecutableSHA256: spec.ClaudeSHA256,
		ExpectedClaudeExecutablePublisher: spec.ClaudePublisher, Model: spec.Model,
		Timeout: time.Duration(spec.TimeoutNanos), MaxAttempts: spec.MaxAttempts,
		beforeMemberRun: func(caseRoot, pack, lane string) error {
			preview, err := packmemoryconsumption.Preview(spec.IsolatedKitRoot, caseRoot, pack, change.ChangeID)
			if err != nil {
				return err
			}
			applied, err := packmemoryconsumption.Apply(spec.IsolatedKitRoot, caseRoot, pack, change.ChangeID, preview.ExpectedPlanSHA256)
			if err != nil {
				return err
			}
			consumptionPlanSHA256 = applied.Receipt.PlanSHA256
			receiptData, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, applied.Plan.ReceiptPath, "pack-memory consumer receipt", 1<<20)
			if err != nil {
				return err
			}
			consumptionReceiptSHA256 = packMemoryAcceptanceBytesSHA256(receiptData)
			_, consumerBindingSHA256, err = packmemoryconsumption.BindConsumerTask(caseRoot, lane, change.ChangeID)
			return err
		},
	})
	result.ConsumerLaunches = consumerDaily.SessionLaunches
	result.Failures = appendPackMemoryAcceptanceDailyFailures(
		result.Failures,
		"consumer",
		consumerDaily,
		spec,
	)
	if err != nil || consumerDaily.FinalState != "member-intake-ready" || consumerDaily.Lane == "" || consumerDaily.SessionLaunches < 1 || consumerDaily.SessionCompletions < 1 || consumerBindingSHA256 == "" {
		return result, fmt.Errorf("run real pack-memory consumer: %w", err)
	}
	consumerInspection, consumerOutput, err := packMemoryAcceptanceStrictOutput(consumerCase, consumerDaily.Lane, "consumer-use.json")
	if err != nil {
		return result, err
	}
	consumerProof, err := packmemoryconsumption.VerifyConsumerUse(spec.IsolatedKitRoot, consumerCase, liveAcceptancePack, packmemoryconsumption.ConsumerUseOptions{
		ChangeID: change.ChangeID, Lane: consumerDaily.Lane, AttemptID: consumerInspection.AttemptID, OutputPath: consumerOutput.Path,
	})
	if err != nil || !consumerProof.Verified || !consumerProof.ReadOnly || !consumerProof.NoAuthority || !consumerProof.NoConfirmed || !consumerProof.NoHeavyTool {
		return result, fmt.Errorf("verify real pack-memory consumer use: %w", err)
	}
	result.Consumer = PackMemoryLiveAcceptanceConsumerProof{
		Lane: consumerDaily.Lane, AttemptID: consumerInspection.AttemptID, BindingSHA256: consumerBindingSHA256,
		ConsumptionPlanSHA256: consumptionPlanSHA256, ConsumptionReceiptSHA256: consumptionReceiptSHA256,
		ManifestSHA256: consumerInspection.ManifestSHA256, OutputSHA256: consumerOutput.SHA256,
		QuoteSHA256: consumerProof.QuoteSHA256, AppliedAsSHA256: packMemoryAcceptanceBytesSHA256([]byte(consumerProof.AppliedAs)), ProofSHA256: consumerProof.ProofSHA256,
	}
	if !strings.EqualFold(consumerProof.SourceSHA256, candidateSHA256) || consumerProof.ChangeID != change.ChangeID || consumerProof.ConsumptionPlanSHA256 != consumptionPlanSHA256 || !strings.EqualFold(consumerProof.ConsumptionReceiptSHA256, consumptionReceiptSHA256) {
		return result, fmt.Errorf("real consumer proof is not bound to the promoted change and selected sync receipt")
	}
	result.Passed = true
	return result, nil
}

func packMemoryAcceptanceExpectedSanitization(counts map[string]int) bool {
	for kind, count := range counts {
		if kind == "capturesPath" {
			if count != 2 {
				return false
			}
			continue
		}
		if count != 0 {
			return false
		}
	}
	return counts["capturesPath"] == 2
}

var packMemoryAcceptanceCapturesPath = regexp.MustCompile(`(?i)captures[\\/][^` + "`" + `\r\n|，。；;\)\] ]+`)

func validatePackMemoryAcceptanceSanitization(predecessor, raw []byte) error {
	expected := packMemoryAcceptanceCapturesPath.FindAll(predecessor, -1)
	actual := packMemoryAcceptanceCapturesPath.FindAll(raw, -1)
	if len(expected) != 2 || len(actual) != len(expected) {
		return fmt.Errorf("capturesPath matches predecessor=%d raw=%d, want exact inherited pair", len(expected), len(actual))
	}
	for index := range expected {
		if !strings.EqualFold(string(expected[index]), string(actual[index])) {
			return fmt.Errorf("capturesPath match %d changed from canonical predecessor", index+1)
		}
	}
	return nil
}

func packMemoryAcceptanceCleanupProof(repoRoot, caseRoot, packetPath, decisionPath, candidatePath, proofRoot, actor string) (promote.CandidateReviewProofDraftResult, error) {
	proofPath := filepath.Join(proofRoot, "rh07.candidate-cleanup-proof.json")
	preview, err := promote.DraftCandidateReviewProof(repoRoot, caseRoot, liveAcceptancePack, promote.CandidateReviewProofDraftOptions{
		PacketPath: packetPath, DecisionPath: decisionPath, ProofPath: proofPath,
		ProofType: "candidate-cleanup-proof", CandidatePath: candidatePath,
		Reason: "reviewed candidate cleanup and index removal verified", Actor: actor, WhatIf: true,
	})
	if err != nil {
		return promote.CandidateReviewProofDraftResult{}, fmt.Errorf("preview candidate cleanup proof: %w", err)
	}
	applied, err := promote.DraftCandidateReviewProof(repoRoot, caseRoot, liveAcceptancePack, promote.CandidateReviewProofDraftOptions{
		PacketPath: packetPath, DecisionPath: decisionPath, ProofPath: proofPath,
		ProofType: "candidate-cleanup-proof", CandidatePath: candidatePath,
		Reason: "reviewed candidate cleanup and index removal verified", Actor: actor,
		ExpectedProofSHA256: preview.ProofSHA256,
	})
	if err != nil || !applied.Applied || applied.Proof.Cleanup == nil || !applied.Proof.Cleanup.CandidateAbsent || !applied.Proof.Cleanup.IndexEntryAbsent {
		return promote.CandidateReviewProofDraftResult{}, fmt.Errorf("apply candidate cleanup proof: %w", err)
	}
	return applied, nil
}

func packMemoryAcceptanceLifecycleProof(repoRoot, caseRoot, packetPath, candidatePath, proofPath, proofType, evidencePath, actor string) (promote.CandidateLifecycleProofDraftResult, error) {
	reason := "reviewed candidate lifecycle evidence verifies " + proofType
	preview, err := promote.DraftCandidateLifecycleProof(repoRoot, caseRoot, liveAcceptancePack, promote.CandidateReviewProofDraftOptions{
		PacketPath: packetPath, ProofPath: proofPath, ProofType: proofType,
		CandidatePath: candidatePath, Reason: reason, Actor: actor,
		EvidenceRefs: evidencePath, WhatIf: true,
	})
	if err != nil {
		return promote.CandidateLifecycleProofDraftResult{}, fmt.Errorf("preview %s: %w", proofType, err)
	}
	applied, err := promote.DraftCandidateLifecycleProof(repoRoot, caseRoot, liveAcceptancePack, promote.CandidateReviewProofDraftOptions{
		PacketPath: packetPath, ProofPath: proofPath, ProofType: proofType,
		CandidatePath: candidatePath, Reason: reason, Actor: actor,
		EvidenceRefs: evidencePath, ExpectedProofSHA256: preview.ProofSHA256,
	})
	if err != nil || !applied.Applied {
		return promote.CandidateLifecycleProofDraftResult{}, fmt.Errorf("apply %s: %w", proofType, err)
	}
	return applied, nil
}

func packMemoryAcceptanceCompletedChange(catalog releasecheck.CompletedPackMemoryChangeCatalog, managedPath, sourceSHA256 string) (releasecheck.CompletedPackMemoryChange, error) {
	var matched []releasecheck.CompletedPackMemoryChange
	for _, change := range catalog.Changes {
		if filepath.ToSlash(change.ManagedPath) == filepath.ToSlash(managedPath) && strings.EqualFold(change.SourceSHA256, sourceSHA256) {
			matched = append(matched, change)
		}
	}
	if len(matched) != 1 {
		return releasecheck.CompletedPackMemoryChange{}, fmt.Errorf("completed catalog requires exactly one RH-07 change, found %d", len(matched))
	}
	change := matched[0]
	if change.ChangeID == "" || len(change.AuthoritySHA256) != 64 || change.CleanupProof.SHA256 == "" || change.PackDoctorProof.SHA256 == "" || change.FreshReconsumeProof.SHA256 == "" || change.AttachedReconsumeProof.SHA256 == "" {
		return releasecheck.CompletedPackMemoryChange{}, fmt.Errorf("completed RH-07 change omitted strict authority or lifecycle proofs")
	}
	return change, nil
}

func packMemoryAcceptanceStrictOutput(caseRoot, lane, expectedPath string) (memberexecution.Inspection, memberexecution.Output, error) {
	inspection, ok, err := memberexecution.Latest(caseRoot, lane)
	if err != nil {
		return memberexecution.Inspection{}, memberexecution.Output{}, err
	}
	if !ok || inspection.State != "intake-ready" || inspection.Intent == nil || inspection.TaskContext == nil || inspection.Manifest == nil || !inspection.Manifest.NoAuthority || !inspection.Manifest.NoConfirmed || !inspection.Manifest.NoHeavyTool {
		return memberexecution.Inspection{}, memberexecution.Output{}, fmt.Errorf("member result is not strict intake-ready")
	}
	expectedPath = filepath.ToSlash(strings.TrimSpace(expectedPath))
	for _, output := range inspection.Manifest.Outputs {
		if filepath.ToSlash(output.Path) == expectedPath {
			return inspection, output, nil
		}
	}
	return memberexecution.Inspection{}, memberexecution.Output{}, fmt.Errorf("strict result manifest omitted exact output %q", expectedPath)
}

func packMemoryAcceptanceCandidate(result promote.CandidateResult, managedTarget string) (promote.CandidateReviewItem, error) {
	for _, candidate := range result.ReviewPlan.ReviewItems {
		if candidate.Kind == "managed-doc" && candidate.ReviewDecision == "pending-review" && filepath.ToSlash(candidate.Path) == filepath.ToSlash(managedTarget) {
			return candidate, nil
		}
	}
	return promote.CandidateReviewItem{}, fmt.Errorf("created candidates omitted staged managed target %q", managedTarget)
}

func appendPackMemoryAcceptanceDailyFailures(
	failures []PackMemoryLiveAcceptanceFailure,
	phase string,
	daily DailyResult,
	spec PackMemoryLiveAcceptanceChildSpec,
) []PackMemoryLiveAcceptanceFailure {
	for _, host := range daily.HostRuns {
		failures = appendPackMemoryAcceptanceHostFailures(
			failures,
			phase,
			host,
			spec,
		)
	}
	return failures
}

func appendPackMemoryAcceptanceHostFailures(
	failures []PackMemoryLiveAcceptanceFailure,
	phase string,
	host Result,
	spec PackMemoryLiveAcceptanceChildSpec,
) []PackMemoryLiveAcceptanceFailure {
	for _, session := range host.Sessions {
		if session.Failure == nil && len(session.Diagnostics) == 0 {
			continue
		}
		failure := session.Failure
		if failure != nil {
			copy := *failure
			copy.Detail = truncate(
				redactPackMemoryAcceptanceDiagnostic(
					copy.Detail,
					spec,
				),
				1024,
			)
			failure = &copy
		}
		diagnostics := make([]string, 0, len(session.Diagnostics))
		for _, diagnostic := range session.Diagnostics {
			diagnostics = append(
				diagnostics,
				truncate(
					redactPackMemoryAcceptanceDiagnostic(
						diagnostic,
						spec,
					),
					1024,
				),
			)
		}
		failures = append(failures, PackMemoryLiveAcceptanceFailure{
			Phase:             phase,
			AttemptGeneration: session.AttemptGeneration,
			Outcome:           session.Outcome,
			Failure:           failure,
			Diagnostics:       diagnostics,
		})
	}
	return failures
}

func redactPackMemoryAcceptanceDiagnostic(
	value string,
	spec PackMemoryLiveAcceptanceChildSpec,
) string {
	paths := []struct {
		path        string
		replacement string
	}{
		{strings.TrimSpace(spec.ChildHostPath), "<child-host>"},
		{strings.TrimSpace(spec.ClaudePath), "<claude-executable>"},
		{strings.TrimSpace(spec.IsolatedKitRoot), "<isolated-kit>"},
	}
	sort.SliceStable(paths, func(left, right int) bool {
		return len(paths[left].path) > len(paths[right].path)
	})
	for _, item := range paths {
		if item.path == "" {
			continue
		}
		parts := strings.FieldsFunc(
			item.path,
			func(r rune) bool { return r == '\\' || r == '/' },
		)
		for index := range parts {
			parts[index] = regexp.QuoteMeta(parts[index])
		}
		pattern := `(?i)` + strings.Join(parts, `[/\\]+`)
		value = regexp.MustCompile(pattern).ReplaceAllString(
			value,
			item.replacement,
		)
	}
	return value
}

func packMemoryAcceptanceReviewerLaunches(result Result) int {
	count := 0
	for _, session := range result.Sessions {
		if session.Started && session.SessionKind == "reviewer" && session.SessionID != "" && session.RunLaunchOrdinal > 0 {
			count++
		}
	}
	return count
}

func requirePackMemoryAcceptanceAbsent(paths ...string) error {
	for _, path := range paths {
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("pack-memory live acceptance disposable case already exists: %s", path)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
