package sessionhost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	iofs "io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

const liveAcceptancePack = defaults.DefaultPack

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
	ReceiptPublication  string                    `json:"receiptPublication,omitempty"`
	ReceiptError        string                    `json:"receiptError,omitempty"`
	Pack                string                    `json:"pack"`
	CaseRoot            string                    `json:"caseRoot"`
	NaturalLanguageGoal string                    `json:"naturalLanguageGoal"`
	HumanCorrection     string                    `json:"humanCorrection"`
	Claude              LiveAcceptanceClaude      `json:"claude"`
	ExplicitOperations  int                       `json:"explicitOperations"`
	PublicPreviews      int                       `json:"publicPreviews"`
	PublicMutations     int                       `json:"publicMutations"`
	PackageMutations    int                       `json:"packageMutations"`
	PublicRoute         string                    `json:"publicRoute"`
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
	TerminalReplay      LiveAcceptanceReplay      `json:"terminalReplay"`
	AttachedCase        LiveAcceptanceAttached    `json:"attachedCase"`
	LLMSource           string                    `json:"llmSource"`
	Cleanup             string                    `json:"cleanup"`
	Boundary            []string                  `json:"boundary"`
}

type LiveAcceptanceClaude struct {
	Path      string `json:"path"`
	Publisher string `json:"publisher"`
	Version   string `json:"version"`
	SHA256    string `json:"sha256"`
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

type LiveAcceptanceReplay struct {
	Verified           bool   `json:"verified"`
	FinalState         string `json:"finalState"`
	SessionLaunches    int    `json:"sessionLaunches"`
	SessionCompletions int    `json:"sessionCompletions"`
	MutationSHA256     string `json:"mutationSha256"`
}

type LiveAcceptanceAttached struct {
	Verified            bool                 `json:"verified"`
	CaseRoot            string               `json:"caseRoot"`
	Pack                string               `json:"pack"`
	Lane                string               `json:"lane"`
	OnboardingMode      string               `json:"onboardingMode"`
	SessionLaunches     int                  `json:"sessionLaunches"`
	SessionCompletions  int                  `json:"sessionCompletions"`
	Member              LiveAcceptanceMember `json:"member"`
	TerminalReplay      bool                 `json:"terminalReplay"`
	ReplayLaunches      int                  `json:"replayLaunches"`
	PreservedCaseSHA256 string               `json:"preservedCaseSha256"`
	ReplayTreeSHA256    string               `json:"replayTreeSha256"`
	Cleanup             string               `json:"cleanup"`
}

type liveAcceptanceCaseIdentity struct {
	parent     *os.Root
	parentPath string
	name       string
	parentInfo os.FileInfo
	caseInfo   os.FileInfo
}

func (identity *liveAcceptanceCaseIdentity) Close() error {
	if identity == nil || identity.parent == nil {
		return nil
	}
	err := identity.parent.Close()
	identity.parent = nil
	return err
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
	attachedRoot, err := liveAcceptanceAttachedCaseRoot(caseRoot)
	if err != nil {
		return receipt, err
	}
	if !opt.KeepCase && strings.TrimSpace(opt.ReceiptPath) != "" {
		receiptPath, err := filepath.Abs(strings.TrimSpace(opt.ReceiptPath))
		if err != nil {
			return receipt, err
		}
		if liveAcceptancePathWithin(caseRoot, receiptPath) || liveAcceptancePathWithin(attachedRoot, receiptPath) {
			return receipt, fmt.Errorf("live acceptance receipt must be outside the disposable case roots: %s", receiptPath)
		}
	}
	claude, err := resolveLiveAcceptanceClaude(opt.ClaudePath)
	if err != nil {
		return receipt, err
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
		Claude:              claude,
		PublicRoute:         "RunDaily goal + correction + exact terminal replay; RunDaily attached goal + exact goal replay",
		ManualPlaceholders:  0,
		ManualResultWrites:  0,
		LLMSource:           "member outputs and ReviewerResult bytes are accepted only from spawned Claude Code JSON envelopes with exact session_id matching",
		Cleanup:             "pending",
		AttachedCase: LiveAcceptanceAttached{
			CaseRoot: attachedRoot,
			Cleanup:  "pending",
		},
		Boundary: []string{
			"this gate is explicit opt-in and is never run by ordinary go test ./...",
			"the gate calls the same Go-owned daily front door used by ordinary operation",
			"the gate does not fabricate member output or ReviewerResult bytes",
			"completion scope is the feature lane; the authority main lane remains outside this acceptance claim",
			"no authority/confirmed state or heavy-tool execution is permitted",
		},
	}
	var freshCaseIdentity, attachedCaseIdentity liveAcceptanceCaseIdentity
	defer func() {
		defer freshCaseIdentity.Close()
		defer attachedCaseIdentity.Close()
		if opt.KeepCase {
			receipt.Cleanup = "retained-by-request"
			receipt.AttachedCase.Cleanup = "retained-by-request"
			return
		}
		var cleanupErr error
		if err := removeLiveAcceptanceCase(caseRoot, &freshCaseIdentity); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("clean fresh live acceptance case: %w", err))
		}
		if err := removeLiveAcceptanceCase(attachedRoot, &attachedCaseIdentity); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("clean attached live acceptance case: %w", err))
		}
		if cleanupErr != nil {
			receipt.Passed = false
			receipt.Cleanup = "failed"
			receipt.AttachedCase.Cleanup = "failed"
			retErr = errors.Join(retErr, cleanupErr)
			return
		}
		receipt.Cleanup = "removed"
		receipt.AttachedCase.Cleanup = "removed"
	}()

	dailyOpt := DailyOptions{
		Target:                            caseRoot,
		Goal:                              goal,
		Actor:                             actor,
		ClaudePath:                        claude.Path,
		ExpectedClaudeExecutableSHA256:    claude.SHA256,
		ExpectedClaudeExecutablePublisher: claude.Publisher,
		Model:                             opt.Model,
		Timeout:                           opt.Timeout,
		MaxAttempts:                       opt.MaxAttempts,
		onCaseReady: func(root string) error {
			return captureLiveAcceptanceCaseRoot(root, &freshCaseIdentity)
		},
	}
	firstResult, err := RunDaily(parent, dailyOpt)
	addLiveAcceptanceDailyResult(&receipt, firstResult)
	if err != nil {
		return receipt, fmt.Errorf("fresh daily goal and first real member session: %w", err)
	}
	if !firstResult.OnboardingApplied || firstResult.Pack != liveAcceptancePack || firstResult.Lane == "" || firstResult.SessionLaunches < 1 || firstResult.SessionCompletions < 1 {
		return receipt, fmt.Errorf("fresh daily goal did not onboard and collect a real member: %+v", firstResult)
	}
	lane := firstResult.Lane
	first, ok, err := memberexecution.Latest(caseRoot, lane)
	if err != nil || !ok || first.State != "intake-ready" || first.Owner.ExecutorGeneration != 1 || first.TaskContext == nil || first.Manifest == nil {
		return receipt, fmt.Errorf("first real member was not durably intake-ready: found=%t state=%s err=%v", ok, first.State, err)
	}
	bindLiveAcceptanceOwnerGeneration(receipt.MemberSessions, first)
	receipt.FirstMember = liveAcceptanceMember(first)

	dailyOpt.Goal = ""
	dailyOpt.Correction = correction
	correctedResult, err := RunDaily(parent, dailyOpt)
	addLiveAcceptanceDailyResult(&receipt, correctedResult)
	if err != nil {
		return receipt, fmt.Errorf("daily correction, replacement member, and reviewer sessions: %w", err)
	}
	if correctedResult.CorrectionEventID == "" || correctedResult.ExecutorGeneration != 2 || correctedResult.SessionLaunches < 2 || correctedResult.SessionCompletions < 2 || correctedResult.Completion == nil || correctedResult.Completion.Lane.Status != "closed" {
		return receipt, fmt.Errorf("daily correction did not replace, review, and close the lane: %+v", correctedResult)
	}
	receipt.CorrectionEventID = correctedResult.CorrectionEventID
	second, ok, err := memberexecution.Latest(caseRoot, lane)
	if err != nil || !ok || second.State != "intake-ready" || second.Owner.ExecutorGeneration != 2 || second.TaskContext == nil || second.TaskContext.Correction == nil || second.Manifest == nil {
		return receipt, fmt.Errorf("replacement member was not correction-bound and intake-ready: found=%t state=%s err=%v", ok, second.State, err)
	}
	if second.TaskContext.Goal != goal || second.TaskContext.GoalSource != "committed-mission-intent" || second.TaskContext.Correction.SourceEventID != correctedResult.CorrectionEventID {
		return receipt, fmt.Errorf("replacement task context omitted the exact goal or correction")
	}
	bindLiveAcceptanceOwnerGeneration(receipt.MemberSessions, second)
	receipt.ReplacementMember = liveAcceptanceMember(second)
	if receipt.ReviewerCompletions < 1 {
		return receipt, fmt.Errorf("no real Reviewer Claude session completed")
	}

	verified, err := workstream.InspectLaneCompletion(caseRoot, lane)
	if err != nil {
		return receipt, err
	}
	receipt.Completion = &LiveAcceptanceCompletion{Scope: "feature-lane", Lane: verified.Lane, Status: "closed", Sequence: verified.Sequence, PreviewSHA256: verified.PreviewSHA256, EvidenceCount: len(verified.Evidence), NoAuthority: verified.NoAuthority, NoConfirmed: verified.NoConfirmed, NoHeavyTool: verified.NoHeavyTool}
	if !verified.NoAuthority || !verified.NoConfirmed || !verified.NoHeavyTool {
		return receipt, fmt.Errorf("feature completion boundary is not fail-closed")
	}
	beforeReplay, err := liveAcceptanceTreeSHA256(caseRoot)
	if err != nil {
		return receipt, err
	}
	replayResult, err := RunDaily(parent, dailyOpt)
	receipt.ExplicitOperations++
	if err != nil {
		return receipt, fmt.Errorf("daily terminal replay: %w", err)
	}
	afterReplay, err := liveAcceptanceTreeSHA256(caseRoot)
	if err != nil {
		return receipt, err
	}
	if !replayResult.Replay || replayResult.FinalState != "lane-closed" || replayResult.SessionLaunches != 0 || replayResult.SessionCompletions != 0 || len(replayResult.HostRuns) != 0 || replayResult.CorrectionEventID != correctedResult.CorrectionEventID || beforeReplay != afterReplay {
		return receipt, fmt.Errorf("daily terminal replay was not zero-launch and mutation-free: %+v", replayResult)
	}
	receipt.TerminalReplay = LiveAcceptanceReplay{Verified: true, FinalState: replayResult.FinalState, SessionLaunches: replayResult.SessionLaunches, SessionCompletions: replayResult.SessionCompletions, MutationSHA256: afterReplay}

	attached, err := runLiveAcceptanceAttached(parent, attachedRoot, goal, actor, claude, opt, &attachedCaseIdentity)
	if err != nil {
		return receipt, err
	}
	receipt.AttachedCase = attached.Receipt
	receipt.PublicPreviews++
	receipt.PublicMutations++
	addLiveAcceptanceDailyResult(&receipt, attached.First)
	receipt.ExplicitOperations += 3
	if attached.Replay.SessionLaunches != 0 || !attached.Replay.Replay {
		return receipt, fmt.Errorf("attached daily goal replay relaunched Claude: %+v", attached.Replay)
	}
	receipt.Passed = true
	return receipt, nil
}

type liveAcceptanceAttachedResult struct {
	Receipt LiveAcceptanceAttached
	First   DailyResult
	Replay  DailyResult
}

func runLiveAcceptanceAttached(parent context.Context, caseRoot, goal, actor string, claude LiveAcceptanceClaude, opt LiveAcceptanceOptions, identity *liveAcceptanceCaseIdentity) (liveAcceptanceAttachedResult, error) {
	result := liveAcceptanceAttachedResult{
		Receipt: LiveAcceptanceAttached{
			CaseRoot: caseRoot,
			Pack:     liveAcceptancePack,
			Cleanup:  "pending",
		},
	}
	initArgs := []string{"-Command", "init", "-Target", caseRoot, "-Pack", liveAcceptancePack, "-ProjectName", "rekit-live-attached-acceptance", "-WhatIf", "-Format", "json"}
	var preview syncreview.InitPlan
	if err := runPublicCLI(initArgs, &preview); err != nil {
		return result, fmt.Errorf("public attached case init preview: %w", err)
	}
	if preview.IsMutation || !preview.ReviewRequired || !preview.RequiresConfirmation || len(preview.Writes) == 0 {
		return result, fmt.Errorf("public attached case init preview omitted review-first writes")
	}
	initArgs[len(initArgs)-3] = "-Apply"
	var applied syncreview.ApplyResult
	applyErr := runPublicCLI(initArgs, &applied)
	if identity != nil {
		if bindErr := captureLiveAcceptanceCaseRoot(caseRoot, identity); bindErr != nil {
			return result, errors.Join(applyErr, fmt.Errorf("bind attached live acceptance case root: %w", bindErr))
		}
	}
	if applyErr != nil {
		return result, fmt.Errorf("public attached case init Apply: %w", applyErr)
	}
	if !applied.Applied || !applied.IsMutation || applied.Pack != liveAcceptancePack {
		return result, fmt.Errorf("public attached case init did not apply: %+v", applied)
	}
	preserved := []byte("preserve attached case content\n")
	preservedPath := filepath.Join(caseRoot, "case-local.txt")
	if err := os.WriteFile(preservedPath, preserved, 0o600); err != nil {
		return result, err
	}
	preservedSum := sha256.Sum256(preserved)
	result.Receipt.PreservedCaseSHA256 = hex.EncodeToString(preservedSum[:])

	dailyOpt := DailyOptions{
		Target:                            caseRoot,
		Goal:                              goal,
		Actor:                             actor,
		ClaudePath:                        claude.Path,
		ExpectedClaudeExecutableSHA256:    claude.SHA256,
		ExpectedClaudeExecutablePublisher: claude.Publisher,
		Model:                             opt.Model,
		Timeout:                           opt.Timeout,
		MaxAttempts:                       opt.MaxAttempts,
		onCaseReady: func(root string) error {
			return captureLiveAcceptanceCaseRoot(root, identity)
		},
	}
	first, err := RunDaily(parent, dailyOpt)
	if err != nil {
		return result, fmt.Errorf("attached daily goal and real member session: %w", err)
	}
	result.First = first
	inspection, err := missionintent.Inspect(caseRoot)
	if err != nil || !inspection.Committed || inspection.Recovery.Mode != "attached-adoption" {
		return result, fmt.Errorf("attached daily onboarding did not commit strict adoption: state=%s mode=%s err=%v", inspection.State, inspection.Recovery.Mode, err)
	}
	if !first.OnboardingApplied || first.Pack != liveAcceptancePack || first.Lane == "" || first.SessionLaunches < 1 || first.SessionCompletions < 1 {
		return result, fmt.Errorf("attached daily goal did not launch and collect a real member: %+v", first)
	}
	member, ok, err := memberexecution.Latest(caseRoot, first.Lane)
	if err != nil || !ok || member.State != "intake-ready" || member.TaskContext == nil || member.TaskContext.Goal != goal || member.TaskContext.GoalSource != "committed-mission-intent" || member.TaskContext.Correction != nil || member.Manifest == nil {
		return result, fmt.Errorf("attached daily member was not durably intake-ready: found=%t state=%s err=%v", ok, member.State, err)
	}
	actual, err := os.ReadFile(preservedPath)
	if err != nil || string(actual) != string(preserved) {
		return result, fmt.Errorf("attached daily goal changed ordinary case content: err=%v", err)
	}

	beforeReplay, err := liveAcceptanceTreeSHA256(caseRoot)
	if err != nil {
		return result, err
	}
	replay, err := RunDaily(parent, dailyOpt)
	if err != nil {
		return result, fmt.Errorf("attached daily exact goal replay: %w", err)
	}
	result.Replay = replay
	afterReplay, err := liveAcceptanceTreeSHA256(caseRoot)
	if err != nil {
		return result, err
	}
	if !replay.Replay || replay.FinalState != "member-intake-ready" || replay.SessionLaunches != 0 || replay.SessionCompletions != 0 || len(replay.HostRuns) != 0 || beforeReplay != afterReplay {
		return result, fmt.Errorf("attached daily exact goal replay was not zero-launch and mutation-free: %+v", replay)
	}
	actual, err = os.ReadFile(preservedPath)
	if err != nil || string(actual) != string(preserved) {
		return result, fmt.Errorf("attached daily replay changed ordinary case content: err=%v", err)
	}
	result.Receipt.Verified = true
	result.Receipt.Lane = first.Lane
	result.Receipt.OnboardingMode = inspection.Recovery.Mode
	result.Receipt.SessionLaunches = first.SessionLaunches
	result.Receipt.SessionCompletions = first.SessionCompletions
	result.Receipt.Member = liveAcceptanceMember(member)
	result.Receipt.TerminalReplay = replay.Replay
	result.Receipt.ReplayLaunches = replay.SessionLaunches
	result.Receipt.ReplayTreeSHA256 = afterReplay
	return result, nil
}

func addLiveAcceptanceDailyResult(receipt *LiveAcceptanceReceipt, result DailyResult) {
	receipt.ExplicitOperations++
	baseHostRun := 0
	for _, session := range append(append([]LiveAcceptanceSession{}, receipt.MemberSessions...), receipt.ReviewerSessions...) {
		baseHostRun = max(baseHostRun, session.HostRun)
	}
	for _, step := range result.DriverSteps {
		switch step {
		case "overview", "note-intervention":
			receipt.PublicMutations++
		case "start", "reconcile", "complete":
			receipt.PublicPreviews++
			receipt.PublicMutations++
		}
	}
	if result.OnboardingApplied {
		receipt.PublicPreviews++
		receipt.PublicMutations++
	}
	for index, hostRun := range result.HostRuns {
		addLiveAcceptanceSessions(receipt, hostRun, baseHostRun+index+1)
	}
}

func liveAcceptanceTreeSHA256(root string) (string, error) {
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry iofs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("live acceptance case tree contains a symlink: %s", rel)
		}
		if entry.IsDir() {
			_, _ = hash.Write([]byte("d\x00" + rel + "\x00"))
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("live acceptance case tree contains a non-regular file: %s", rel)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, _ = hash.Write([]byte("f\x00" + rel + "\x00"))
		_, _ = hash.Write(data)
		_, _ = hash.Write([]byte{0})
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func captureLiveAcceptanceCaseRoot(path string, identity *liveAcceptanceCaseIdentity) error {
	if identity == nil {
		return fmt.Errorf("live acceptance case identity target is missing")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	parentPath := filepath.Dir(path)
	name := filepath.Base(path)
	if identity.parent != nil {
		current, err := identity.parent.Lstat(identity.name)
		if err != nil || !current.IsDir() || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(identity.caseInfo, current) {
			return fmt.Errorf("live acceptance case root identity changed: %s", path)
		}
		return nil
	}
	parentInfo, err := os.Lstat(parentPath)
	if err != nil || !parentInfo.IsDir() || parentInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("live acceptance case parent must be a non-symlink directory: %s: %w", parentPath, err)
	}
	parent, err := os.OpenRoot(parentPath)
	if err != nil {
		return err
	}
	openedParent, err := parent.Lstat(".")
	if err != nil || !openedParent.IsDir() || !os.SameFile(parentInfo, openedParent) {
		parent.Close()
		return fmt.Errorf("live acceptance case parent changed while opening: %s", parentPath)
	}
	caseInfo, err := parent.Lstat(name)
	if err != nil || !caseInfo.IsDir() || caseInfo.Mode()&os.ModeSymlink != 0 {
		parent.Close()
		return fmt.Errorf("live acceptance case root must be a non-symlink directory: %s: %w", path, err)
	}
	identity.parent = parent
	identity.parentPath = parentPath
	identity.name = name
	identity.parentInfo = openedParent
	identity.caseInfo = caseInfo
	return nil
}

func removeLiveAcceptanceCase(path string, identity *liveAcceptanceCaseIdentity) error {
	if identity == nil || identity.parent == nil {
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("live acceptance cleanup has no captured case root identity: %s", path)
	}
	info, err := identity.parent.Lstat(identity.name)
	if os.IsNotExist(err) {
		return fmt.Errorf("live acceptance case root disappeared before identity-bound cleanup: %s", path)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !os.SameFile(identity.caseInfo, info) {
		return fmt.Errorf("live acceptance cleanup refuses a replaced case root: %s", path)
	}
	quarantine := identity.name + ".cleanup"
	if _, err := identity.parent.Lstat(quarantine); err == nil {
		return fmt.Errorf("live acceptance cleanup quarantine already exists: %s", filepath.Join(identity.parentPath, quarantine))
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := identity.parent.Rename(identity.name, quarantine); err != nil {
		return err
	}
	moved, err := identity.parent.Lstat(quarantine)
	if err != nil || !moved.IsDir() || moved.Mode()&os.ModeSymlink != 0 || !os.SameFile(identity.caseInfo, moved) {
		return fmt.Errorf("live acceptance case root identity changed while quarantining cleanup: %s", path)
	}
	if current, err := identity.parent.Lstat(identity.name); err == nil {
		return fmt.Errorf("live acceptance cleanup refuses a replacement created at the case root: %s mode=%s", path, current.Mode())
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := identity.parent.RemoveAll(quarantine); err != nil {
		return err
	}
	currentParent, err := os.Lstat(identity.parentPath)
	if err != nil || !os.SameFile(identity.parentInfo, currentParent) {
		return fmt.Errorf("live acceptance case parent changed during cleanup: %s", identity.parentPath)
	}
	return nil
}

func liveAcceptanceAttachedCaseRoot(freshRoot string) (string, error) {
	parent := filepath.Dir(freshRoot)
	base := filepath.Base(freshRoot)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return "", fmt.Errorf("invalid live acceptance case root: %s", freshRoot)
	}
	return filepath.Join(parent, base+"-attached"), nil
}

func resolveLiveAcceptanceClaude(requested string) (LiveAcceptanceClaude, error) {
	if strings.TrimSpace(requested) != "" {
		return LiveAcceptanceClaude{}, fmt.Errorf("live acceptance refuses a custom Claude executable; omit -claude so the canonical signed Claude Code installation is discovered independently of PATH")
	}
	identity, err := discoverTrustedClaudeExecutable()
	if err != nil {
		return LiveAcceptanceClaude{}, err
	}
	return LiveAcceptanceClaude{
		Path:      identity.Path,
		Publisher: identity.Publisher,
		Version:   identity.Version,
		SHA256:    identity.SHA256,
	}, nil
}

func liveAcceptancePathWithin(root, path string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func liveAcceptanceCaseRoot(requested string) (string, error) {
	var root string
	if strings.TrimSpace(requested) != "" {
		resolved, err := filepath.Abs(strings.TrimSpace(requested))
		if err != nil {
			return "", err
		}
		root = resolved
	} else {
		base, err := os.MkdirTemp("", "rekit-"+liveAcceptancePack+"-live-acceptance-")
		if err != nil {
			return "", err
		}
		if err := os.Remove(base); err != nil {
			return "", err
		}
		root = base
	}
	attached, err := liveAcceptanceAttachedCaseRoot(root)
	if err != nil {
		return "", err
	}
	for _, candidate := range []string{root, attached} {
		if _, err := os.Lstat(candidate); err == nil {
			return "", fmt.Errorf("live acceptance requires non-existing fresh and attached case roots: %s", candidate)
		} else if !os.IsNotExist(err) {
			return "", err
		}
	}
	return root, nil
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
	anchorPath := filepath.VolumeName(path) + string(filepath.Separator)
	if anchorPath == "" {
		anchorPath = string(filepath.Separator)
	}
	rel, err := filepath.Rel(anchorPath, path)
	if err != nil || filepath.IsAbs(rel) || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("live acceptance receipt path escapes its volume root: %s", path)
	}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := rekitfs.WriteNewExclusiveRegularFileAnchored(anchorPath, filepath.ToSlash(rel), "live acceptance receipt", data); err != nil {
		return fmt.Errorf("publish live acceptance receipt %s: %w", path, err)
	}
	return nil
}
