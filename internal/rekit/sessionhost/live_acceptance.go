package sessionhost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/note"
	"github.com/shuiyu486/re-context-kits/internal/rekit/onboarding"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

const (
	liveAcceptancePack = defaults.DefaultPack
	liveAcceptanceLane = "feature-analysis-live-acceptance"
)

type LiveAcceptanceOptions struct {
	CaseRoot    string
	Goal        string
	Correction  string
	ClaudePath  string
	Model       string
	Actor       string
	Timeout     time.Duration
	MaxAttempts int
	KeepCase    bool
	ReceiptPath string
}

type LiveAcceptanceReceipt struct {
	SchemaVersion       int                       `json:"schemaVersion"`
	Kind                string                    `json:"kind"`
	Passed              bool                      `json:"passed"`
	Pack                string                    `json:"pack"`
	CaseRoot            string                    `json:"caseRoot"`
	NaturalLanguageGoal string                    `json:"naturalLanguageGoal"`
	HumanCorrection     string                    `json:"humanCorrection"`
	ExplicitOperations  int                       `json:"explicitOperations"`
	ManualPlaceholders  int                       `json:"manualPlaceholders"`
	ManualResultWrites  int                       `json:"manualResultWrites"`
	MemberLaunches      int                       `json:"memberLaunches"`
	MemberCompletions   int                       `json:"memberCompletions"`
	ReviewerLaunches    int                       `json:"reviewerLaunches"`
	ReviewerCompletions int                       `json:"reviewerCompletions"`
	Replacements        int                       `json:"replacements"`
	MemberSessions      []LiveAcceptanceSession   `json:"memberSessions"`
	ReviewerSessions    []LiveAcceptanceSession   `json:"reviewerSessions"`
	FirstMember         LiveAcceptanceMember      `json:"firstMember"`
	ReplacementMember   LiveAcceptanceMember      `json:"replacementMember"`
	CorrectionEventID   string                    `json:"correctionEventId"`
	Completion          *LiveAcceptanceCompletion `json:"completion,omitempty"`
	LLMSource           string                    `json:"llmSource"`
	Cleanup             string                    `json:"cleanup"`
	Boundary            []string                  `json:"boundary"`
}

type LiveAcceptanceSession struct {
	Started           bool     `json:"started"`
	Recovered         bool     `json:"recovered,omitempty"`
	AttemptGeneration int      `json:"attemptGeneration,omitempty"`
	HostRun           int      `json:"hostRun,omitempty"`
	RunLaunchOrdinal  int      `json:"runLaunchOrdinal,omitempty"`
	OwnerGeneration   int      `json:"ownerGeneration,omitempty"`
	SessionID         string   `json:"sessionId"`
	Kind              string   `json:"kind"`
	Outcome           string   `json:"outcome"`
	Diagnostics       []string `json:"diagnostics,omitempty"`
}

type LiveAcceptanceMember struct {
	AttemptID          string `json:"attemptId"`
	Executor           string `json:"executor"`
	Generation         int    `json:"generation"`
	TaskContextPath    string `json:"taskContextPath"`
	TaskContextSHA256  string `json:"taskContextSha256"`
	ManifestPath       string `json:"manifestPath"`
	ManifestSHA256     string `json:"manifestSha256"`
	GoalSource         string `json:"goalSource"`
	CorrectionSourceID string `json:"correctionSourceId,omitempty"`
}

type LiveAcceptanceCompletion struct {
	Scope         string `json:"scope"`
	Lane          string `json:"lane"`
	Status        string `json:"status"`
	Sequence      int    `json:"sequence"`
	PreviewSHA256 string `json:"previewSha256"`
	EvidenceCount int    `json:"evidenceCount"`
	NoAuthority   bool   `json:"noAuthority"`
	NoConfirmed   bool   `json:"noConfirmed"`
	NoHeavyTool   bool   `json:"noHeavyTool"`
}

func RunLiveAcceptance(parent context.Context, opt LiveAcceptanceOptions) (receipt LiveAcceptanceReceipt, retErr error) {
	goal := strings.TrimSpace(opt.Goal)
	correction := strings.TrimSpace(opt.Correction)
	if goal == "" || correction == "" {
		return receipt, fmt.Errorf("live acceptance requires non-empty natural-language goal and human correction")
	}
	caseRoot, err := liveAcceptanceCaseRoot(opt.CaseRoot)
	if err != nil {
		return receipt, err
	}
	if !opt.KeepCase && strings.TrimSpace(opt.ReceiptPath) != "" {
		receiptPath, err := filepath.Abs(strings.TrimSpace(opt.ReceiptPath))
		if err != nil {
			return receipt, err
		}
		if liveAcceptancePathWithin(caseRoot, receiptPath) {
			return receipt, fmt.Errorf("live acceptance receipt must be outside the disposable case root: %s", receiptPath)
		}
	}
	actor := strings.TrimSpace(opt.Actor)
	if actor == "" {
		actor = "rekit-live-acceptance"
	}
	receipt = LiveAcceptanceReceipt{
		SchemaVersion:       1,
		Kind:                "rekit-" + liveAcceptancePack + "-live-acceptance-receipt",
		Pack:                liveAcceptancePack,
		CaseRoot:            caseRoot,
		NaturalLanguageGoal: goal,
		HumanCorrection:     correction,
		ManualPlaceholders:  0,
		ManualResultWrites:  0,
		LLMSource:           "member outputs and ReviewerResult bytes are accepted only from spawned Claude Code JSON envelopes with exact session_id matching",
		Cleanup:             "pending",
		Boundary: []string{
			"this gate is explicit opt-in and is never run by ordinary go test ./...",
			"the gate does not fabricate member output or ReviewerResult bytes",
			"completion scope is the feature lane; the authority main lane remains outside this acceptance claim",
			"no authority/confirmed state or heavy-tool execution is permitted",
		},
	}
	defer func() {
		if opt.KeepCase {
			receipt.Cleanup = "retained-by-request"
			return
		}
		if err := os.RemoveAll(caseRoot); err != nil {
			receipt.Passed = false
			receipt.Cleanup = "failed: " + err.Error()
			retErr = errors.Join(retErr, fmt.Errorf("clean live acceptance case: %w", err))
			return
		}
		receipt.Cleanup = "removed"
	}()

	ctx, err := runtime.New(caseRoot, liveAcceptancePack)
	if err != nil {
		return receipt, err
	}
	onboardOpt := onboarding.Options{Target: caseRoot, Pack: liveAcceptancePack, ProjectName: "rekit-live-acceptance", Goal: goal, Actor: actor, Executor: "live-member-generation-1", InitialLane: liveAcceptanceLane}
	onboardPlan, err := onboarding.Preview(ctx.RepoRoot, onboardOpt)
	if err != nil {
		return receipt, err
	}
	receipt.ExplicitOperations++
	onboardOpt.PublicationStamp = onboardPlan.PublicationStamp
	onboardOpt.ExpectedOnboardingPlanSHA256 = onboardPlan.OnboardingPlanSHA256
	if _, err := onboarding.Apply(ctx.RepoRoot, onboardOpt); err != nil {
		return receipt, err
	}
	receipt.ExplicitOperations++

	startPlan, err := workstream.StartPreview(ctx.RepoRoot, caseRoot, liveAcceptancePack, workstream.StartOptions{Selector: "analysis-live-acceptance", Executor: "live-member-generation-1", Actor: actor})
	if err != nil {
		return receipt, err
	}
	receipt.ExplicitOperations++
	startPreviewBytes, err := json.Marshal(startPlan)
	if err != nil {
		return receipt, err
	}
	startOpt := workstream.StartOptions{Selector: "analysis-live-acceptance", Executor: "live-member-generation-1", Actor: actor, ExpectedPreviewSHA256: liveAcceptanceSHA(startPreviewBytes)}
	if _, err := workstream.StartApply(ctx.RepoRoot, caseRoot, liveAcceptancePack, startOpt); err != nil {
		return receipt, err
	}
	receipt.ExplicitOperations++

	hostOpt := Options{Target: caseRoot, Pack: liveAcceptancePack, Actor: actor, ClaudePath: opt.ClaudePath, Model: opt.Model, Timeout: opt.Timeout, MaxAttempts: opt.MaxAttempts, StopAfterMemberIntake: true}
	firstRun, err := Run(parent, hostOpt)
	addLiveAcceptanceSessions(&receipt, firstRun, 1)
	if err != nil {
		return receipt, fmt.Errorf("first real member session: %w", err)
	}
	receipt.ExplicitOperations++
	first, ok, err := memberexecution.Latest(caseRoot, liveAcceptanceLane)
	if err != nil || !ok || first.State != "intake-ready" || first.Owner.ExecutorGeneration != 1 || first.TaskContext == nil || first.Manifest == nil {
		return receipt, fmt.Errorf("first real member was not durably intake-ready: found=%t state=%s err=%v", ok, first.State, err)
	}
	bindLiveAcceptanceOwnerGeneration(receipt.MemberSessions, first)
	receipt.FirstMember = liveAcceptanceMember(first)

	correctionID, err := newUUID()
	if err != nil {
		return receipt, err
	}
	correctionID = "live-correction-" + strings.ReplaceAll(correctionID, "-", "")
	createdAt := nowRFC3339Nano()
	noteOpt := note.Options{Kind: "intervention", Lane: liveAcceptanceLane, Subject: "human correction for live acceptance", Summary: correction, Actor: actor, Action: "override", Status: "open", EventID: correctionID, CreatedAt: createdAt, Target: first.ManifestPath}
	notePreview, err := note.Append(ctx.RepoRoot, caseRoot, liveAcceptancePack, noteOpt, true)
	if err != nil {
		return receipt, err
	}
	receipt.ExplicitOperations++
	noteOpt.ExpectedEventSHA256 = notePreview.EventSHA256
	if applied, err := note.Append(ctx.RepoRoot, caseRoot, liveAcceptancePack, noteOpt, false); err != nil || !applied.Applied {
		return receipt, fmt.Errorf("record human correction: applied=%t err=%v", applied.Applied, err)
	}
	receipt.ExplicitOperations++
	receipt.CorrectionEventID = correctionID

	reconcileOpt := workstream.ReconcileOptions{Selector: "analysis-live-acceptance", InterventionID: correctionID, Actor: actor, Executor: "live-member-generation-2", Reason: "apply the explicit human correction and replace the prior Claude member"}
	if _, err := workstream.ReconcilePreview(ctx.RepoRoot, caseRoot, liveAcceptancePack, reconcileOpt); err != nil {
		return receipt, err
	}
	receipt.ExplicitOperations++
	reconciled, err := workstream.ReconcileApply(ctx.RepoRoot, caseRoot, liveAcceptancePack, reconcileOpt)
	if err != nil || !reconciled.Applied || reconciled.ExecutorGeneration != 2 {
		return receipt, fmt.Errorf("reconcile human correction: generation=%d applied=%t err=%v", reconciled.ExecutorGeneration, reconciled.Applied, err)
	}
	receipt.ExplicitOperations++

	hostOpt.StopAfterMemberIntake = false
	secondRun, err := Run(parent, hostOpt)
	addLiveAcceptanceSessions(&receipt, secondRun, 2)
	if err != nil {
		return receipt, fmt.Errorf("replacement member and reviewer sessions: %w", err)
	}
	receipt.ExplicitOperations++
	second, ok, err := memberexecution.Latest(caseRoot, liveAcceptanceLane)
	if err != nil || !ok || second.State != "intake-ready" || second.Owner.ExecutorGeneration != 2 || second.TaskContext == nil || second.TaskContext.Correction == nil || second.Manifest == nil {
		return receipt, fmt.Errorf("replacement member was not correction-bound and intake-ready: found=%t state=%s err=%v", ok, second.State, err)
	}
	if second.TaskContext.Goal != goal || second.TaskContext.GoalSource != "committed-mission-intent" || second.TaskContext.Correction.SourceEventID != correctionID {
		return receipt, fmt.Errorf("replacement task context omitted the exact goal or correction")
	}
	bindLiveAcceptanceOwnerGeneration(receipt.MemberSessions, second)
	receipt.ReplacementMember = liveAcceptanceMember(second)
	if receipt.ReviewerCompletions < 1 {
		return receipt, fmt.Errorf("no real Reviewer Claude session completed")
	}

	completeOpt := workstream.CompleteOptions{Selector: "analysis-live-acceptance", Actor: actor, Reason: "accepted real Reviewer lineage completed the corrected member result", EvidenceRefs: relativeLiveAcceptancePath(caseRoot, second.ManifestPath)}
	completionPlan, err := workstream.CompletePreview(ctx.RepoRoot, caseRoot, liveAcceptancePack, completeOpt)
	if err != nil || completionPlan.Blocked || completionPlan.CompletionPlanSHA256 == "" {
		return receipt, fmt.Errorf("feature completion preview is not accepted-lineage ready: blocked=%t err=%v", completionPlan.Blocked, err)
	}
	receipt.ExplicitOperations++
	completeOpt.ExpectedPreviewSHA256 = completionPlan.CompletionPlanSHA256
	completed, err := workstream.CompleteApply(ctx.RepoRoot, caseRoot, liveAcceptancePack, completeOpt)
	if err != nil || !completed.Applied || completed.CompletionReceipt == nil || completed.Lane.Status != "closed" {
		return receipt, fmt.Errorf("feature completion failed: applied=%t status=%s err=%v", completed.Applied, completed.Lane.Status, err)
	}
	receipt.ExplicitOperations++
	verified, err := workstream.InspectLaneCompletion(caseRoot, liveAcceptanceLane)
	if err != nil {
		return receipt, err
	}
	receipt.Completion = &LiveAcceptanceCompletion{Scope: "feature-lane", Lane: verified.Lane, Status: "closed", Sequence: verified.Sequence, PreviewSHA256: verified.PreviewSHA256, EvidenceCount: len(verified.Evidence), NoAuthority: verified.NoAuthority, NoConfirmed: verified.NoConfirmed, NoHeavyTool: verified.NoHeavyTool}
	if !verified.NoAuthority || !verified.NoConfirmed || !verified.NoHeavyTool {
		return receipt, fmt.Errorf("feature completion boundary is not fail-closed")
	}
	receipt.Passed = true

	return receipt, nil
}

func liveAcceptancePathWithin(root, path string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func liveAcceptanceCaseRoot(requested string) (string, error) {
	if strings.TrimSpace(requested) != "" {
		root, err := filepath.Abs(strings.TrimSpace(requested))
		if err != nil {
			return "", err
		}
		if _, err := os.Lstat(root); err == nil {
			return "", fmt.Errorf("live acceptance requires a non-existing fresh case root: %s", root)
		} else if !os.IsNotExist(err) {
			return "", err
		}
		return root, nil
	}
	base, err := os.MkdirTemp("", "rekit-"+liveAcceptancePack+"-live-acceptance-")
	if err != nil {
		return "", err
	}
	if err := os.Remove(base); err != nil {
		return "", err
	}
	return base, nil
}

func addLiveAcceptanceSessions(receipt *LiveAcceptanceReceipt, result Result, hostRun int) {
	receipt.Replacements += result.Replacements
	for _, item := range result.Sessions {
		if !item.Started && !item.Recovered {
			continue
		}
		session := LiveAcceptanceSession{
			Started: item.Started, Recovered: item.Recovered, AttemptGeneration: item.AttemptGeneration,
			HostRun: hostRun, RunLaunchOrdinal: item.RunLaunchOrdinal,
			SessionID: item.SessionID, Kind: item.SessionKind, Outcome: item.Outcome, Diagnostics: append([]string{}, item.Diagnostics...),
		}
		completed := item.Outcome == "returned" || item.Outcome == "returned-recovered"
		switch item.SessionKind {
		case "member":
			if item.Started {
				receipt.MemberLaunches++
			}
			if completed {
				receipt.MemberCompletions++
			}
			receipt.MemberSessions = append(receipt.MemberSessions, session)
		case "reviewer":
			if item.Started {
				receipt.ReviewerLaunches++
			}
			if completed {
				receipt.ReviewerCompletions++
			}
			receipt.ReviewerSessions = append(receipt.ReviewerSessions, session)
		}
	}
}

func bindLiveAcceptanceOwnerGeneration(sessions []LiveAcceptanceSession, inspection memberexecution.Inspection) {
	for index := len(sessions) - 1; index >= 0; index-- {
		if sessions[index].Kind == "member" && sessions[index].OwnerGeneration == 0 {
			sessions[index].OwnerGeneration = inspection.Owner.ExecutorGeneration
			return
		}
	}
}

func liveAcceptanceMember(inspection memberexecution.Inspection) LiveAcceptanceMember {
	member := LiveAcceptanceMember{AttemptID: inspection.AttemptID, Executor: inspection.Owner.Executor, Generation: inspection.Owner.ExecutorGeneration, TaskContextPath: relativeLiveAcceptancePath(inspection.Intent.CaseRoot, inspection.TaskContextPath), TaskContextSHA256: inspection.TaskContextSHA256, ManifestPath: relativeLiveAcceptancePath(inspection.Intent.CaseRoot, inspection.ManifestPath), ManifestSHA256: inspection.ManifestSHA256}
	if inspection.TaskContext != nil {
		member.GoalSource = inspection.TaskContext.GoalSource
		if inspection.TaskContext.Correction != nil {
			member.CorrectionSourceID = inspection.TaskContext.Correction.SourceEventID
		}
	}
	return member
}

func relativeLiveAcceptancePath(caseRoot, path string) string {
	rel, err := filepath.Rel(caseRoot, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func WriteLiveAcceptanceReceipt(path string, receipt LiveAcceptanceReceipt) error {
	path, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if info, err := os.Lstat(path); err == nil && (!info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
		return fmt.Errorf("live acceptance receipt target must be a regular non-symlink file")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func liveAcceptanceSHA(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
