package sessionhost

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	iofs "io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterhost"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanecompletion"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	"github.com/shuiyu486/re-context-kits/internal/rekit/processguard"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

const (
	liveAcceptancePack         = defaults.DefaultPack
	liveAcceptanceEvidencePath = "fixtures/live-acceptance/feature-note.txt"
	liveAcceptanceEvidenceText = "Feature: bounded-live-acceptance\nRequest ID: rh08-harmless-feature-note\nCandidate path: none\nEndpoint: local-only\nMethod: read-only\nImpact: none\nObservation: this harmless case-local note exists solely for bounded feature analysis.\nRequired next action: report the inspected evidence without external effects.\n"
)

var crossPackLiveAcceptancePacks = map[string]bool{
	"_template":    true,
	"web-security": true,
}

type LiveAcceptanceOptions struct {
	CaseRoot    string
	Pack        string
	Goal        string
	Correction  string
	ClaudePath  string
	Model       string
	Actor       string
	Timeout     time.Duration
	MaxAttempts int
	KeepCase    bool
	ReceiptPath string
	AdapterPath string
}

type LiveAcceptanceReceipt struct {
	SchemaVersion       int                       `json:"schemaVersion"`
	Kind                string                    `json:"kind"`
	Passed              bool                      `json:"passed"`
	ReceiptPublication  string                    `json:"receiptPublication,omitempty"`
	ReceiptError        string                    `json:"receiptError,omitempty"`
	Pack                string                    `json:"pack"`
	CaseRoot            string                    `json:"caseRoot"`
	CaseCreated         bool                      `json:"caseCreated"`
	NaturalLanguageGoal string                    `json:"naturalLanguageGoal"`
	HumanCorrection     string                    `json:"humanCorrection"`
	CorrectionEvidence  LiveAcceptanceEvidence    `json:"correctionEvidence"`
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
	Failure             *FailureDiagnosis         `json:"failure,omitempty"`
	FirstMember         LiveAcceptanceMember      `json:"firstMember"`
	FirstRejection      *LiveAcceptanceRejection  `json:"firstRejection,omitempty"`
	RejectedReplay      LiveAcceptanceReplay      `json:"rejectedReplay"`
	ReplacementMember   LiveAcceptanceMember      `json:"replacementMember"`
	FinalAcceptance     *LiveAcceptanceAcceptance `json:"finalAcceptance,omitempty"`
	CorrectionEventID   string                    `json:"correctionEventId"`
	Completion          *LiveAcceptanceCompletion `json:"completion,omitempty"`
	VMPIDA              *LiveAcceptanceVMPIDA     `json:"vmpIda,omitempty"`
	TerminalReplay      LiveAcceptanceReplay      `json:"terminalReplay"`
	AttachedCase        LiveAcceptanceAttached    `json:"attachedCase"`
	LLMSource           string                    `json:"llmSource"`
	Cleanup             string                    `json:"cleanup"`
	Boundary            []string                  `json:"boundary"`
}

type LiveAcceptanceEvidence struct {
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Bytes   int    `json:"bytes"`
	Publish string `json:"publication"`
}

type LiveAcceptanceClaude struct {
	Path      string `json:"path"`
	Publisher string `json:"publisher"`
	Version   string `json:"version"`
	SHA256    string `json:"sha256"`
}

type LiveAcceptanceSession struct {
	Started           bool              `json:"started"`
	Recovered         bool              `json:"recovered,omitempty"`
	AttemptGeneration int               `json:"attemptGeneration,omitempty"`
	HostRun           int               `json:"hostRun,omitempty"`
	RunLaunchOrdinal  int               `json:"runLaunchOrdinal,omitempty"`
	OwnerGeneration   int               `json:"ownerGeneration,omitempty"`
	SessionID         string            `json:"sessionId"`
	Kind              string            `json:"kind"`
	Outcome           string            `json:"outcome"`
	Failure           *FailureDiagnosis `json:"failure,omitempty"`
	Diagnostics       []string          `json:"diagnostics,omitempty"`
}

type LiveAcceptanceMember struct {
	AttemptID          string                          `json:"attemptId"`
	OutputContract     *memberexecution.OutputContract `json:"outputContract,omitempty"`
	Executor           string                          `json:"executor"`
	Generation         int                             `json:"generation"`
	TaskContextPath    string                          `json:"taskContextPath"`
	TaskContextSHA256  string                          `json:"taskContextSha256"`
	ManifestPath       string                          `json:"manifestPath"`
	ManifestSHA256     string                          `json:"manifestSha256"`
	GoalSource         string                          `json:"goalSource"`
	CorrectionSourceID string                          `json:"correctionSourceId,omitempty"`
}

type LiveAcceptanceRejection struct {
	ManifestPath              string   `json:"manifestPath"`
	ManifestSHA256            string   `json:"manifestSha256"`
	PacketID                  string   `json:"packetId"`
	RouteID                   string   `json:"routeId"`
	ShardID                   string   `json:"shardId"`
	ReviewerResultInputSHA256 string   `json:"reviewerResultInputSha256"`
	ReviewerSession           string   `json:"reviewerSession"`
	VerificationEventID       string   `json:"verificationEventId"`
	DecisionEventID           string   `json:"decisionEventId"`
	Summary                   string   `json:"summary"`
	EvidenceRefs              []string `json:"evidenceRefs"`
	OwnerExecutor             string   `json:"ownerExecutor"`
	OwnerGeneration           int      `json:"ownerGeneration"`
}

type LiveAcceptanceAcceptance struct {
	ManifestPath              string `json:"manifestPath"`
	ManifestSHA256            string `json:"manifestSha256"`
	PacketID                  string `json:"packetId"`
	RouteID                   string `json:"routeId"`
	ShardID                   string `json:"shardId"`
	ReviewerResultInputSHA256 string `json:"reviewerResultInputSha256"`
	ReviewerSession           string `json:"reviewerSession"`
	VerificationEventID       string `json:"verificationEventId"`
	DecisionEventID           string `json:"decisionEventId"`
	OwnerExecutor             string `json:"ownerExecutor"`
	OwnerGeneration           int    `json:"ownerGeneration"`
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

type LiveAcceptanceVMPIDA struct {
	Verified                bool                            `json:"verified"`
	AdapterPath             string                          `json:"adapterPath"`
	AdapterSHA256           string                          `json:"adapterSha256"`
	AdapterProcessID        int                             `json:"adapterProcessId"`
	RequestPath             string                          `json:"requestPath"`
	RequestSHA256           string                          `json:"requestSha256"`
	ProfilePreviewSHA256    string                          `json:"profilePreviewSha256"`
	ProfileSHA256           string                          `json:"profileSha256"`
	DeniedActions           []string                        `json:"deniedActions"`
	GateEventID             string                          `json:"gateEventId"`
	Authorization           string                          `json:"authorization"`
	Lifecycle               *BinaryREAdapterLifecycleResult `json:"ordinaryLifecycle,omitempty"`
	Run                     adapterhost.AuthorizedRunResult `json:"run"`
	AcknowledgementEventID  string                          `json:"acknowledgementEventId"`
	EvidenceReviewSessionID string                          `json:"evidenceReviewSessionId"`
	EvidenceReviewDecision  string                          `json:"evidenceReviewDecision"`
	SelectedEvidenceRef     string                          `json:"selectedEvidenceRef"`
	EvidenceReviewCleared   bool                            `json:"evidenceReviewCleared"`
	MemberBindingVerified   bool                            `json:"memberBindingVerified"`
	ReviewerLineageVerified bool                            `json:"reviewerLineageVerified"`
	TerminalReplayNoChild   bool                            `json:"terminalReplayNoChild"`
	TerminalReplayNoClaude  bool                            `json:"terminalReplayNoClaude"`
	NoAuthorityOrConfirmed  bool                            `json:"noAuthorityOrConfirmed"`
	NoNetworkBoundary       string                          `json:"noNetworkBoundary"`
	AcknowledgementBoundary string                          `json:"acknowledgementBoundary"`
}

type LiveAcceptanceReplay struct {
	Verified           bool   `json:"verified"`
	FinalState         string `json:"finalState"`
	SessionLaunches    int    `json:"sessionLaunches"`
	SessionCompletions int    `json:"sessionCompletions"`
	MutationSHA256     string `json:"mutationSha256"`
}

type LiveAcceptanceAttached struct {
	Verified                   bool                   `json:"verified"`
	CaseRoot                   string                 `json:"caseRoot"`
	CaseCreated                bool                   `json:"caseCreated"`
	Pack                       string                 `json:"pack"`
	Lane                       string                 `json:"lane"`
	OnboardingMode             string                 `json:"onboardingMode"`
	SessionLaunches            int                    `json:"sessionLaunches"`
	SessionCompletions         int                    `json:"sessionCompletions"`
	MemberCutpointVerified     bool                   `json:"memberCutpointVerified"`
	ReviewerCutpointVerified   bool                   `json:"reviewerCutpointVerified"`
	CompletionRecoveryVerified bool                   `json:"completionRecoveryVerified"`
	Evidence                   LiveAcceptanceEvidence `json:"evidence"`
	Member                     LiveAcceptanceMember   `json:"member"`
	TerminalReplay             bool                   `json:"terminalReplay"`
	ReplayLaunches             int                    `json:"replayLaunches"`
	PreservedCaseSHA256        string                 `json:"preservedCaseSha256"`
	ReplayTreeSHA256           string                 `json:"replayTreeSha256"`
	Cleanup                    string                 `json:"cleanup"`
}

type liveAcceptanceCaseIdentity struct {
	parent     *os.Root
	parentPath string
	name       string
	parentInfo os.FileInfo
	caseInfo   os.FileInfo
	markerName string
	marker     []byte
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
	pack := strings.ToLower(strings.TrimSpace(opt.Pack))
	if pack == "" {
		pack = liveAcceptancePack
	}
	if pack != liveAcceptancePack && !crossPackLiveAcceptancePacks[pack] {
		return receipt, fmt.Errorf("live acceptance pack %q is outside the explicit cross-pack allowlist", pack)
	}
	goalInput := strings.TrimSpace(opt.Goal)
	correctionInput := strings.TrimSpace(opt.Correction)
	if goalInput == "" || correctionInput == "" {
		return receipt, fmt.Errorf("live acceptance requires non-empty natural-language goal and human correction")
	}
	goal := liveAcceptanceBoundGoalForPack(pack, goalInput)
	correction := liveAcceptanceBoundCorrectionForPack(pack, correctionInput)
	caseRoot, err := liveAcceptanceCaseRoot(opt.CaseRoot, pack)
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
		Kind:                "rekit-" + pack + "-live-acceptance-receipt",
		Pack:                pack,
		CaseRoot:            caseRoot,
		NaturalLanguageGoal: goalInput,
		HumanCorrection:     correctionInput,
		Claude:              claude,
		PublicRoute:         "one RunDaily exact-pack goal operation with isolated member and Reviewer segments + intentional bounded evidence gap + reviewer rejection + rejection stop replay + correction-time bounded evidence publication + replacement review + exact terminal replay; default VMP pack additionally proves public profile preview/Apply, canonical authorized-gate, real fixed adapter parent and contained child, evidence review acknowledgement, exact profile revoke, and child-free replay; one attached exact-pack goal resumes across member and Reviewer cutpoints into zero-Claude completion recovery and exact terminal replay",
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
			"the gate calls the same Go-owned daily front door used by ordinary operation and binds one exact allowlisted pack",
			"the gate does not fabricate member output or ReviewerResult bytes",
			"bounded correction input is published only after the first canonical rejection; it is never member output or ReviewerResult content",
			"default VMP pack alone executes the exact authorized inspect adapter; debug, patch, dump, network, and all other heavy actions remain denied",
			"the fixed adapter child has no network codepath, but this claim is not OS-enforced socket isolation",
			"completion scope is the feature lane; the authority main lane remains outside this acceptance claim",
			"no authority or confirmed state is written; daily completion itself executes no heavy tool",
		},
	}
	var freshCaseIdentity, attachedCaseIdentity liveAcceptanceCaseIdentity
	defer func() {
		defer freshCaseIdentity.Close()
		defer attachedCaseIdentity.Close()
		receipt.CaseCreated = freshCaseIdentity.parent != nil
		receipt.AttachedCase.CaseCreated = attachedCaseIdentity.parent != nil
		if opt.KeepCase {
			if receipt.CaseCreated {
				receipt.Cleanup = "retained-by-request"
			} else {
				receipt.Cleanup = "not-created"
			}
			if receipt.AttachedCase.CaseCreated {
				receipt.AttachedCase.Cleanup = "retained-by-request"
			} else {
				receipt.AttachedCase.Cleanup = "not-created"
			}
			return
		}
		var freshCleanupErr, attachedCleanupErr error
		if receipt.CaseCreated {
			freshCleanupErr = removeLiveAcceptanceCase(caseRoot, &freshCaseIdentity)
		}
		receipt.Cleanup = liveAcceptanceCleanupStatus(receipt.CaseCreated, freshCleanupErr)
		if receipt.AttachedCase.CaseCreated {
			attachedCleanupErr = removeLiveAcceptanceCase(attachedRoot, &attachedCaseIdentity)
		}
		receipt.AttachedCase.Cleanup = liveAcceptanceCleanupStatus(receipt.AttachedCase.CaseCreated, attachedCleanupErr)
		cleanupErr := errors.Join(
			wrapLiveAcceptanceCleanupError("fresh", freshCleanupErr),
			wrapLiveAcceptanceCleanupError("attached", attachedCleanupErr),
		)
		if cleanupErr != nil {
			receipt.Passed = false
			retErr = errors.Join(retErr, cleanupErr)
		}
	}()

	if !strings.EqualFold(pack, liveAcceptancePack) {
		if err := initLiveAcceptanceCase(caseRoot, pack, "rekit-live-cross-pack-acceptance", &freshCaseIdentity); err != nil {
			return receipt, err
		}
		receipt.CaseCreated = true
	} else {
		adapterPath := strings.TrimSpace(opt.AdapterPath)
		if adapterPath == "" {
			return receipt, fmt.Errorf("default VMP pack live acceptance requires the built rekit-adapter-host executable")
		}
		adapterBinding, err := processguard.LockExecutable(adapterPath, 128<<20)
		if err != nil {
			return receipt, fmt.Errorf("bind default VMP pack live acceptance adapter executable: %w", err)
		}
		receipt.VMPIDA = &LiveAcceptanceVMPIDA{
			AdapterPath:             adapterBinding.Path(),
			AdapterSHA256:           adapterBinding.SHA256(),
			NoAuthorityOrConfirmed:  true,
			NoNetworkBoundary:       "fixed-child-no-network-codepath",
			AcknowledgementBoundary: "independent trusted Claude reviews exact packet/source/receipt/observation lineage before hash-bound tool-review verification; the adapter runner never acknowledges its own evidence",
		}
		if err := adapterBinding.Close(); err != nil {
			return receipt, err
		}
	}
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
			if err := captureLiveAcceptanceCaseRoot(root, &freshCaseIdentity); err != nil {
				return err
			}
			receipt.CaseCreated = true
			return nil
		},
	}
	if strings.EqualFold(pack, liveAcceptancePack) {
		dailyOpt.binaryREAdapterPath = receipt.VMPIDA.AdapterPath
		dailyOpt.beforeMemberRun = func(root, currentPack, lane string) error {
			if strings.TrimSpace(dailyOpt.Correction) == "" {
				return nil
			}
			return prepareLiveAcceptanceVMPIDA(root, currentPack, lane, receipt.VMPIDA)
		}
	}
	firstResult, err := RunDaily(parent, dailyOpt)
	addLiveAcceptanceDailyResult(&receipt, firstResult)
	if err != nil {
		return receipt, fmt.Errorf("fresh daily goal and first real member session: %w", err)
	}
	if !firstResult.OnboardingApplied || firstResult.Pack != pack || firstResult.Lane == "" || firstResult.SessionLaunches < 1 || firstResult.SessionCompletions < 1 {
		return receipt, fmt.Errorf("fresh daily goal did not onboard and collect a real member: %+v", firstResult)
	}
	lane := firstResult.Lane
	dailyOpt.SelectedLane = lane
	first, ok, err := memberexecution.Latest(caseRoot, lane)
	if err != nil || !ok || first.State != "intake-ready" || first.Owner.ExecutorGeneration != 1 || first.TaskContext == nil || first.Manifest == nil {
		return receipt, fmt.Errorf("first real member was not durably intake-ready: found=%t state=%s err=%v", ok, first.State, err)
	}
	if err := validateLiveAcceptanceOutputContract(pack, first); err != nil {
		return receipt, err
	}
	bindLiveAcceptanceOwnerGeneration(receipt.MemberSessions, first)
	receipt.FirstMember = liveAcceptanceMember(first)

	if err := validateLiveAcceptanceReviewerRejectionStop(
		firstResult,
		dailyOpt.MaxAttempts,
	); err != nil {
		return receipt, err
	}
	dailyOpt.Goal = ""
	manifestRef := relativeLiveAcceptancePath(caseRoot, first.ManifestPath)
	rejection, rejected, err := workstream.CurrentMemberManifestReviewerRejection(caseRoot, lane, manifestRef)
	if err != nil || !rejected {
		return receipt, fmt.Errorf("first reviewer did not produce a canonical rejection: rejected=%t err=%v", rejected, err)
	}
	if rejection.ManifestSHA256 != first.ManifestSHA256 || rejection.RouteID != first.TaskContext.OutputContract.RouteID || rejection.OwnerExecutor != first.Owner.Executor || rejection.OwnerGeneration != first.Owner.ExecutorGeneration || rejection.ReviewerSession == "" || rejection.VerificationEventID == "" || rejection.DecisionEventID == "" || rejection.Summary == "" || len(rejection.EvidenceRefs) == 0 {
		return receipt, fmt.Errorf("canonical rejection omitted current manifest, route, owner, session, event, or evidence lineage: %+v", rejection)
	}
	firstReviewerSession := rejection.ReviewerSession
	receipt.FirstRejection = liveAcceptanceRejection(rejection)

	beforeRejectedReplay, err := liveAcceptanceTreeSHA256(caseRoot)
	if err != nil {
		return receipt, err
	}
	trustedClaudePath := dailyOpt.ClaudePath
	dailyOpt.ClaudePath = filepath.Join(caseRoot, "missing-claude-for-durable-replay.exe")
	rejectedReplay, err := RunDaily(parent, dailyOpt)
	dailyOpt.ClaudePath = trustedClaudePath
	receipt.ExplicitOperations++
	if err != nil {
		return receipt, fmt.Errorf("rejected manifest stop replay: %w", err)
	}
	afterRejectedReplay, err := liveAcceptanceTreeSHA256(caseRoot)
	if err != nil {
		return receipt, err
	}
	if !rejectedReplay.Replay || !rejectedReplay.Blocked || rejectedReplay.FinalState != "reviewer-rejected-awaiting-correction" || rejectedReplay.SessionLaunches != 0 || rejectedReplay.SessionCompletions != 0 || len(rejectedReplay.HostRuns) != 0 || beforeRejectedReplay != afterRejectedReplay {
		return receipt, fmt.Errorf("rejected manifest replay relaunched a reviewer or mutated the case: %+v", rejectedReplay)
	}
	receipt.RejectedReplay = LiveAcceptanceReplay{Verified: true, FinalState: rejectedReplay.FinalState, SessionLaunches: rejectedReplay.SessionLaunches, SessionCompletions: rejectedReplay.SessionCompletions, MutationSHA256: afterRejectedReplay}

	if !strings.EqualFold(pack, liveAcceptancePack) {
		evidence, err := publishLiveAcceptanceEvidence(caseRoot)
		if err != nil {
			return receipt, fmt.Errorf("publish bounded correction evidence: %w", err)
		}
		receipt.CorrectionEvidence = evidence
		receipt.PackageMutations++
	}

	dailyOpt.Correction = correction
	correctedResult, err := RunDaily(parent, dailyOpt)
	addLiveAcceptanceDailyResult(&receipt, correctedResult)
	if err != nil {
		return receipt, fmt.Errorf("daily correction, replacement member, and reviewer sessions: %w", err)
	}
	if correctedResult.CorrectionEventID == "" || correctedResult.ExecutorGeneration != 2 || correctedResult.SessionLaunches < 2 || correctedResult.SessionCompletions < 2 || correctedResult.Completion == nil || correctedResult.Completion.Lane.Status != "closed" {
		return receipt, fmt.Errorf("daily correction did not replace, review, and close the lane: %+v", correctedResult)
	}
	if strings.EqualFold(pack, liveAcceptancePack) {
		if err := projectLiveAcceptanceVMPIDALifecycle(caseRoot, lane, correctedResult.BinaryREAdapter, receipt.VMPIDA); err != nil {
			return receipt, err
		}
	}
	receipt.CorrectionEventID = correctedResult.CorrectionEventID
	second, ok, err := memberexecution.Latest(caseRoot, lane)
	if err != nil || !ok || second.State != "intake-ready" || second.Owner.ExecutorGeneration != 2 || second.TaskContext == nil || second.TaskContext.Correction == nil || second.TaskContext.Correction.ReviewerRejection == nil || second.Manifest == nil {
		return receipt, fmt.Errorf("replacement member was not correction-bound and intake-ready: found=%t state=%s err=%v", ok, second.State, err)
	}
	if err := validateLiveAcceptanceOutputContract(pack, second); err != nil {
		return receipt, err
	}
	replacementRejection := second.TaskContext.Correction.ReviewerRejection
	if second.TaskContext.Goal != goal || second.TaskContext.GoalSource != "committed-mission-intent" || second.TaskContext.Correction.SourceEventID != correctedResult.CorrectionEventID || second.TaskContext.Correction.SourceSummary != correction || replacementRejection.ManifestRef != rejection.ManifestRef || replacementRejection.ManifestSHA256 != rejection.ManifestSHA256 || replacementRejection.PacketID != rejection.PacketID || replacementRejection.RouteID != rejection.RouteID || replacementRejection.ShardID != rejection.ShardID || replacementRejection.ReviewerResultInputSHA256 != rejection.ReviewerResultInputSHA256 || replacementRejection.ReviewerSession != rejection.ReviewerSession || replacementRejection.VerificationEventID != rejection.VerificationEventID || replacementRejection.DecisionEventID != rejection.DecisionEventID || replacementRejection.OwnerExecutor != rejection.OwnerExecutor || replacementRejection.OwnerGeneration != rejection.OwnerGeneration || replacementRejection.Summary != rejection.Summary || !sameLiveAcceptanceStrings(replacementRejection.EvidenceRefs, rejection.EvidenceRefs) {
		return receipt, fmt.Errorf("replacement task context omitted the exact goal, correction, or canonical rejection lineage")
	}
	bindLiveAcceptanceOwnerGeneration(receipt.MemberSessions, second)
	receipt.ReplacementMember = liveAcceptanceMember(second)
	if second.ManifestSHA256 == first.ManifestSHA256 {
		return receipt, fmt.Errorf("replacement member reused the rejected manifest sha256")
	}
	if strings.EqualFold(pack, liveAcceptancePack) {
		if err := validateLiveAcceptanceVMPIDAMember(caseRoot, second, receipt.VMPIDA); err != nil {
			return receipt, err
		}
	} else if err := validateLiveAcceptanceEvidence(caseRoot, receipt.CorrectionEvidence); err != nil {
		return receipt, err
	}
	if receipt.ReviewerCompletions < 2 || len(receipt.ReviewerSessions) < 2 {
		return receipt, fmt.Errorf("two real Reviewer Claude sessions did not complete")
	}
	secondReviewerSession := receipt.ReviewerSessions[len(receipt.ReviewerSessions)-1].SessionID
	if secondReviewerSession == "" || secondReviewerSession == firstReviewerSession {
		return receipt, fmt.Errorf("replacement review did not use an independent reviewer session")
	}
	secondManifestRef := relativeLiveAcceptancePath(caseRoot, second.ManifestPath)
	acceptance, accepted, err := workstream.CurrentMemberManifestReviewerAcceptance(caseRoot, lane, secondManifestRef)
	if err != nil || !accepted {
		return receipt, fmt.Errorf("replacement manifest lacks canonical accepted reviewer lineage: accepted=%t err=%v", accepted, err)
	}
	if acceptance.ManifestSHA256 != second.ManifestSHA256 || acceptance.RouteID != second.TaskContext.OutputContract.RouteID || acceptance.PacketID == rejection.PacketID || acceptance.ReviewerSession != secondReviewerSession || acceptance.ReviewerSession == firstReviewerSession || acceptance.OwnerExecutor != second.Owner.Executor || acceptance.OwnerGeneration != second.Owner.ExecutorGeneration {
		return receipt, fmt.Errorf("final acceptance did not bind the replacement manifest, route, new packet, independent reviewer session, and current owner")
	}
	receipt.FinalAcceptance = liveAcceptanceAcceptance(acceptance)
	if strings.EqualFold(pack, liveAcceptancePack) {
		if err := validateLiveAcceptanceVMPIDAReviewer(caseRoot, second, acceptance, receipt.VMPIDA); err != nil {
			return receipt, err
		}
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
	if strings.EqualFold(pack, liveAcceptancePack) {
		if receipt.VMPIDA == nil || receipt.VMPIDA.Lifecycle == nil || replayResult.BinaryREAdapter != nil {
			return receipt, fmt.Errorf("terminal VMP IDA replay unexpectedly re-entered the ordinary adapter lifecycle: %+v", replayResult.BinaryREAdapter)
		}
		receipt.VMPIDA.TerminalReplayNoChild = true
		receipt.VMPIDA.TerminalReplayNoClaude = replayResult.SessionLaunches == 0
		receipt.VMPIDA.Verified = true
	}

	attached, err := runLiveAcceptanceAttached(parent, attachedRoot, pack, liveAcceptanceAttachedGoal(), actor, claude, opt, &attachedCaseIdentity)
	receipt.AttachedCase = attached.Receipt
	if err != nil {
		return receipt, err
	}
	receipt.PublicPreviews++
	receipt.PublicMutations++
	receipt.PackageMutations++
	addLiveAcceptanceDailyResult(&receipt, attached.MemberCutpoint)
	addLiveAcceptanceDailyResult(&receipt, attached.ReviewerCutpoint)
	addLiveAcceptanceDailyResult(&receipt, attached.CompletionRecovery)
	receipt.ExplicitOperations++
	if attached.Replay.SessionLaunches != 0 || !attached.Replay.Replay {
		return receipt, fmt.Errorf("attached daily goal replay relaunched Claude: %+v", attached.Replay)
	}
	receipt.Passed = true
	return receipt, nil
}

type liveAcceptanceAttachedResult struct {
	Receipt            LiveAcceptanceAttached
	MemberCutpoint     DailyResult
	ReviewerCutpoint   DailyResult
	CompletionRecovery DailyResult
	Replay             DailyResult
}

func liveAcceptanceCleanupStatus(created bool, err error) string {
	if !created {
		return "not-created"
	}
	if err != nil {
		return "failed"
	}
	return "removed"
}

func wrapLiveAcceptanceCleanupError(label string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("clean %s live acceptance case: %w", label, err)
}

func liveAcceptanceBoundGoal(goal string) string {
	return strings.TrimSpace(goal) + " Acceptance requires reading and citing the exact bounded feature note at " + liveAcceptanceEvidencePath + ". Always return a bounded analysis output. If that file is absent or unreadable, do not invent content: state that this mandatory acceptance requirement is unmet so the Reviewer can reject it."
}

func liveAcceptanceBoundGoalForPack(pack, goal string) string {
	if strings.EqualFold(strings.TrimSpace(pack), liveAcceptancePack) {
		return strings.TrimSpace(goal) + " Acceptance requires a bounded literal inspection of the synthetic existing IDA TSV indexes. The required immutable task binding and adapter packet are intentionally absent before correction. Do not invent index findings: return a bounded gap analysis so the independent Reviewer can reject it."
	}
	return strings.TrimSpace(goal) + " Acceptance requires reading and citing the exact bounded feature note at " + liveAcceptanceEvidencePath + ". Always return a bounded analysis output. If that file is absent or unreadable, do not invent content: state that this mandatory acceptance requirement is unmet so the Reviewer can reject it."
}

func liveAcceptanceBoundCorrection(correction string) string {
	return strings.TrimSpace(correction) + " Read and cite the newly published bounded case-local evidence at " + liveAcceptanceEvidencePath + "."
}

func liveAcceptanceBoundCorrectionForPack(pack, correction string) string {
	if strings.EqualFold(strings.TrimSpace(pack), liveAcceptancePack) {
		return strings.TrimSpace(correction) + " Read the exact vmp-ida-index-evidence task binding, then read its immutable packet, report, dispatch, and receipt paths. In the reviewer-items output cite the exact selected row text and evidenceRef plus the packet path, receipt path, and observation event ID. Report only literal matches supported by that exact lineage and preserve the no-authority/no-confirmed boundary."
	}
	return strings.TrimSpace(correction) + " Read and cite the newly published bounded case-local evidence at " + liveAcceptanceEvidencePath + "."
}

func liveAcceptanceAttachedGoal() string {
	return "The bounded case-local evidence is already published at " + liveAcceptanceEvidencePath + ". Inspect that exact feature note and report only its observed feature, request ID, candidate path, endpoint, method, impact, observation, and required next action. Cite that exact file and do not infer external facts or perform external effects."
}

func publishLiveAcceptanceEvidence(caseRoot string) (LiveAcceptanceEvidence, error) {
	data := []byte(liveAcceptanceEvidenceText)
	if err := rekitfs.WriteNewExclusiveRegularFileAnchored(caseRoot, liveAcceptanceEvidencePath, "live acceptance feature evidence", data); err != nil {
		return LiveAcceptanceEvidence{}, err
	}
	sum := sha256.Sum256(data)
	return LiveAcceptanceEvidence{
		Path: liveAcceptanceEvidencePath, SHA256: hex.EncodeToString(sum[:]), Bytes: len(data), Publish: "exclusive-case-local",
	}, nil
}

func validateLiveAcceptanceEvidence(caseRoot string, evidence LiveAcceptanceEvidence) error {
	if evidence.Path != liveAcceptanceEvidencePath || evidence.Bytes != len(liveAcceptanceEvidenceText) || evidence.Publish != "exclusive-case-local" {
		return fmt.Errorf("live acceptance evidence receipt is invalid: %+v", evidence)
	}
	path, err := rekitfs.SafeJoin(caseRoot, evidence.Path)
	if err != nil {
		return err
	}
	data, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, path, "live acceptance feature evidence", int64(len(liveAcceptanceEvidenceText)))
	if err != nil || string(data) != liveAcceptanceEvidenceText {
		return fmt.Errorf("live acceptance feature evidence changed: %w", err)
	}
	sum := sha256.Sum256(data)
	if !strings.EqualFold(evidence.SHA256, hex.EncodeToString(sum[:])) {
		return fmt.Errorf("live acceptance feature evidence sha256 changed")
	}
	return nil
}

func initLiveAcceptanceCase(caseRoot, pack, projectName string, identity *liveAcceptanceCaseIdentity) error {
	initArgs := []string{"-Command", "init", "-Target", caseRoot, "-Pack", pack, "-ProjectName", projectName, "-WhatIf", "-Format", "json"}
	var preview syncreview.InitPlan
	if err := runPublicCLI(initArgs, &preview); err != nil {
		return fmt.Errorf("public live acceptance case init preview: %w", err)
	}
	if preview.IsMutation || !preview.ReviewRequired || !preview.RequiresConfirmation || len(preview.Writes) == 0 ||
		preview.ExpectedPlanSHA256 == "" || len(preview.ApplyArgs) == 0 {
		return fmt.Errorf("public live acceptance case init preview omitted review-first writes or exact Apply request")
	}
	var applied syncreview.ApplyResult
	applyErr := runPublicCLI(preview.ApplyArgs, &applied)
	if identity != nil {
		if bindErr := captureLiveAcceptanceCaseRoot(caseRoot, identity); bindErr != nil {
			return errors.Join(applyErr, fmt.Errorf("bind live acceptance case root: %w", bindErr))
		}
	}
	if applyErr != nil {
		return fmt.Errorf("public live acceptance case init Apply: %w", applyErr)
	}
	if !applied.Applied || !applied.IsMutation || !strings.EqualFold(applied.Pack, pack) {
		return fmt.Errorf("public live acceptance case init did not apply: %+v", applied)
	}
	return nil
}

func runLiveAcceptanceAttached(parent context.Context, caseRoot, pack, goal, actor string, claude LiveAcceptanceClaude, opt LiveAcceptanceOptions, identity *liveAcceptanceCaseIdentity) (liveAcceptanceAttachedResult, error) {
	result := liveAcceptanceAttachedResult{
		Receipt: LiveAcceptanceAttached{
			CaseRoot: caseRoot,
			Pack:     pack,
			Cleanup:  "pending",
		},
	}
	if err := initLiveAcceptanceCase(caseRoot, pack, "rekit-live-attached-acceptance", identity); err != nil {
		return result, err
	}
	result.Receipt.CaseCreated = true
	evidence, err := publishLiveAcceptanceEvidence(caseRoot)
	if err != nil {
		return result, fmt.Errorf("publish attached bounded feature evidence: %w", err)
	}
	result.Receipt.Evidence = evidence
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
		stopAfterMemberSegment: true,
	}
	memberCutpoint, err := RunDaily(parent, dailyOpt)
	if err != nil {
		return result, fmt.Errorf("attached daily member cutpoint: %w", err)
	}
	result.MemberCutpoint = memberCutpoint
	inspection, err := missionintent.Inspect(caseRoot)
	if err != nil || !inspection.Committed || inspection.Recovery.Mode != "attached-adoption" {
		return result, fmt.Errorf("attached daily onboarding did not commit strict adoption: state=%s mode=%s err=%v", inspection.State, inspection.Recovery.Mode, err)
	}
	if !memberCutpoint.OnboardingApplied || memberCutpoint.Pack != pack || memberCutpoint.Lane == "" || memberCutpoint.FinalState != "reviewer-ready" || memberCutpoint.SessionLaunches != 1 || memberCutpoint.SessionCompletions != 1 || len(memberCutpoint.HostRuns) != 1 || memberCutpoint.HostRuns[0].FinalMode != "reviewer-ready" || memberCutpoint.HostRuns[0].SessionLaunches != 1 || memberCutpoint.HostRuns[0].SessionCompletions != 1 {
		return result, fmt.Errorf("attached daily member cutpoint crossed the Reviewer launch boundary: %+v", memberCutpoint)
	}
	member, ok, err := memberexecution.Latest(caseRoot, memberCutpoint.Lane)
	if err != nil || !ok || member.State != "intake-ready" || member.TaskContext == nil || member.TaskContext.Goal != goal || member.TaskContext.GoalSource != "committed-mission-intent" || member.TaskContext.Correction != nil || member.Manifest == nil {
		return result, fmt.Errorf("attached daily member was not durably intake-ready: found=%t state=%s err=%v", ok, member.State, err)
	}
	if err := validateLiveAcceptanceOutputContract(pack, member); err != nil {
		return result, err
	}
	facts, err := mission.ReadStrictLedgerFacts(caseRoot)
	if err != nil {
		return result, err
	}
	handoffs, err := workstream.ReviewerDispatchIntakeHandoffs(caseRoot, facts, memberCutpoint.Lane)
	if err != nil || len(handoffs) != 1 || handoffs[0].Paused || handoffs[0].State != "ready-for-reviewer-dispatch" || handoffs[0].PacketPath == "" || handoffs[0].DispatchPromptPath == "" || len(handoffs[0].DispatchPromptSHA256) != 64 || !handoffs[0].DispatchPromptCurrent || handoffs[0].ReviewerSession != "" {
		return result, fmt.Errorf("attached member cutpoint did not materialize one current Reviewer packet without launching it: handoffs=%+v err=%v", handoffs, err)
	}
	result.Receipt.MemberCutpointVerified = true

	dailyOpt.SelectedLane = memberCutpoint.Lane
	dailyOpt.stopAfterMemberSegment = false
	cutErr := errors.New("live acceptance reviewer intake cutpoint")
	cutObserved := false
	restoreObservers := setSupervisionAcceptanceObservers(nil, func(stage string) error {
		if stage != "reviewer-intake" {
			return nil
		}
		cutObserved = true
		return cutErr
	})
	restored := false
	restore := func() {
		if restored {
			return
		}
		restored = true
		restoreObservers()
	}
	defer restore()
	reviewerCutpoint, reviewerErr := RunDaily(parent, dailyOpt)
	restore()
	if !errors.Is(reviewerErr, cutErr) || !cutObserved {
		return result, fmt.Errorf("attached daily Reviewer intake cutpoint was not observed: result=%+v err=%v", reviewerCutpoint, reviewerErr)
	}
	reviewerCutpoint.Failure = nil
	result.ReviewerCutpoint = reviewerCutpoint
	if reviewerCutpoint.SessionLaunches != 1 || reviewerCutpoint.SessionCompletions != 1 || len(reviewerCutpoint.HostRuns) != 2 || reviewerCutpoint.HostRuns[0].SessionLaunches != 0 || reviewerCutpoint.HostRuns[0].SessionCompletions != 0 || reviewerCutpoint.HostRuns[0].FinalMode != "reviewer-ready" || reviewerCutpoint.HostRuns[1].SessionLaunches != 1 || reviewerCutpoint.HostRuns[1].SessionCompletions != 1 {
		return result, fmt.Errorf("attached daily Reviewer cutpoint did not recover with zero member launch and one Reviewer launch: %+v", reviewerCutpoint)
	}
	manifestRef := relativeLiveAcceptancePath(caseRoot, member.ManifestPath)
	acceptance, accepted, err := workstream.CurrentMemberManifestReviewerAcceptance(caseRoot, memberCutpoint.Lane, manifestRef)
	if err != nil || !accepted || acceptance.OwnerExecutor != member.Owner.Executor || acceptance.OwnerGeneration != member.Owner.ExecutorGeneration || acceptance.ReviewerSession == "" {
		return result, fmt.Errorf("attached Reviewer cutpoint omitted current accepted lineage: accepted=%t acceptance=%+v err=%v", accepted, acceptance, err)
	}
	lifecycle, err := lanecompletion.Inspect(caseRoot, memberCutpoint.Lane)
	if err != nil || lifecycle.State != lanecompletion.StateNone {
		return result, fmt.Errorf("attached Reviewer cutpoint crossed the completion boundary: state=%s err=%v", lifecycle.State, err)
	}
	status, err := runPublicStatus(caseRoot, pack, memberCutpoint.Lane)
	if err != nil || !dailyCompletionOwnerRequest(status, memberCutpoint.Lane) {
		return result, fmt.Errorf("attached Reviewer cutpoint did not expose the exact accepted-lineage completion owner: err=%v", err)
	}
	result.Receipt.ReviewerCutpointVerified = true

	completionRecovery, err := RunDaily(parent, dailyOpt)
	if err != nil {
		return result, fmt.Errorf("attached daily completion recovery: %w", err)
	}
	result.CompletionRecovery = completionRecovery
	if completionRecovery.Mode != "goal" || !completionRecovery.OnboardingReplay || completionRecovery.FinalState != "lane-closed" || completionRecovery.SessionLaunches != 0 || completionRecovery.SessionCompletions != 0 || len(completionRecovery.HostRuns) != 1 || completionRecovery.HostRuns[0].SessionLaunches != 0 || completionRecovery.HostRuns[0].SessionCompletions != 0 || completionRecovery.Completion == nil || completionRecovery.Completion.Lane.Status != "closed" || len(completionRecovery.DriverSteps) != 1 || completionRecovery.DriverSteps[0] != "complete" {
		return result, fmt.Errorf("attached completion recovery relaunched Claude or did not apply only canonical completion: %+v", completionRecovery)
	}
	result.Receipt.CompletionRecoveryVerified = true

	if err := validateLiveAcceptanceEvidence(caseRoot, result.Receipt.Evidence); err != nil {
		return result, err
	}
	actual, err := os.ReadFile(preservedPath)
	if err != nil || string(actual) != string(preserved) {
		return result, fmt.Errorf("attached daily recovery changed ordinary case content: err=%v", err)
	}
	beforeReplay, err := liveAcceptanceTreeSHA256(caseRoot)
	if err != nil {
		return result, err
	}
	dailyOpt.Goal = goal
	replay, err := RunDaily(parent, dailyOpt)
	if err != nil {
		return result, fmt.Errorf("attached daily exact goal replay: %w", err)
	}
	result.Replay = replay
	afterReplay, err := liveAcceptanceTreeSHA256(caseRoot)
	if err != nil {
		return result, err
	}
	if !replay.Replay || replay.FinalState != "lane-closed" || replay.SessionLaunches != 0 || replay.SessionCompletions != 0 || len(replay.HostRuns) != 0 || beforeReplay != afterReplay {
		return result, fmt.Errorf("attached daily exact terminal replay was not zero-launch and mutation-free: %+v", replay)
	}
	actual, err = os.ReadFile(preservedPath)
	if err != nil || string(actual) != string(preserved) {
		return result, fmt.Errorf("attached daily replay changed ordinary case content: err=%v", err)
	}
	result.Receipt.Verified = true
	result.Receipt.Lane = memberCutpoint.Lane
	result.Receipt.OnboardingMode = inspection.Recovery.Mode
	result.Receipt.SessionLaunches = memberCutpoint.SessionLaunches + reviewerCutpoint.SessionLaunches
	result.Receipt.SessionCompletions = memberCutpoint.SessionCompletions + reviewerCutpoint.SessionCompletions
	result.Receipt.Member = liveAcceptanceMember(member)
	result.Receipt.TerminalReplay = replay.Replay
	result.Receipt.ReplayLaunches = replay.SessionLaunches
	result.Receipt.ReplayTreeSHA256 = afterReplay
	return result, nil
}

func validateLiveAcceptanceReviewerRejectionStop(
	result DailyResult,
	maxAttempts int,
) error {
	if maxAttempts < 1 {
		maxAttempts = defaultMaxAttempts
	}
	if result.FinalState != "reviewer-rejected-awaiting-correction" ||
		result.SessionLaunches < 2 ||
		result.SessionLaunches > maxAttempts+1 ||
		result.SessionCompletions < 2 ||
		len(result.HostRuns) != 2 ||
		result.HostRuns[0].FinalMode != "reviewer-ready" ||
		result.HostRuns[0].SessionLaunches != 1 ||
		result.HostRuns[0].SessionCompletions != 1 ||
		result.HostRuns[1].FinalMode != "reviewer-rejected-awaiting-correction" ||
		result.HostRuns[1].SessionLaunches != 1 ||
		result.HostRuns[1].SessionCompletions != 1 {
		return fmt.Errorf(
			"first member and reviewer did not stop at the isolated bounded rejection boundary: %+v",
			result,
		)
	}
	for _, session := range result.HostRuns[1].Sessions {
		if session.SessionKind != "reviewer" ||
			!session.Started ||
			session.Outcome != "returned" ||
			session.Failure != nil {
			return fmt.Errorf(
				"first reviewer rejection included an incomplete or failed session: %+v",
				session,
			)
		}
	}
	return nil
}

func addLiveAcceptanceDailyResult(receipt *LiveAcceptanceReceipt, result DailyResult) {
	receipt.ExplicitOperations++
	if result.Failure != nil {
		receipt.Failure = cloneFailureDiagnosis(result.Failure)
	}
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

func bindLiveAcceptanceCaseMarker(identity *liveAcceptanceCaseIdentity, name string, data []byte) error {
	if identity == nil || identity.parent == nil || strings.TrimSpace(name) == "" || len(data) == 0 {
		return fmt.Errorf("live acceptance cleanup marker binding is missing")
	}
	identity.markerName = name
	identity.marker = append([]byte{}, data...)
	return validateLiveAcceptanceCaseRoot(identity, identity.name, false)
}

func validateLiveAcceptanceCaseRoot(identity *liveAcceptanceCaseIdentity, name string, quarantined bool) error {
	info, err := identity.parent.Lstat(name)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !os.SameFile(identity.caseInfo, info) {
		stage := "case root"
		if quarantined {
			stage = "quarantined case root"
		}
		if quarantined {
			return fmt.Errorf("live acceptance %s identity changed: %s: %w", stage, filepath.Join(identity.parentPath, name), err)
		}
		return fmt.Errorf("live acceptance cleanup refuses a replaced case root: %s: %w", filepath.Join(identity.parentPath, name), err)
	}
	root, err := identity.parent.OpenRoot(name)
	if err != nil {
		return err
	}
	defer root.Close()
	opened, err := root.Lstat(".")
	if err != nil || !opened.IsDir() || !os.SameFile(identity.caseInfo, opened) {
		return fmt.Errorf("live acceptance case identity changed while opening: %s", filepath.Join(identity.parentPath, name))
	}
	if identity.markerName != "" {
		marker, err := root.ReadFile(identity.markerName)
		if err != nil || !bytes.Equal(marker, identity.marker) {
			return fmt.Errorf("live acceptance case marker changed: %s: %w", filepath.Join(identity.parentPath, name, identity.markerName), err)
		}
	}
	return nil
}

func removeLiveAcceptanceCase(path string, identity *liveAcceptanceCaseIdentity) error {
	if identity == nil || identity.parent == nil {
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("live acceptance cleanup has no captured case root identity: %s", path)
	}
	if err := validateLiveAcceptanceCaseRoot(identity, identity.name, false); err != nil {
		return err
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
	if err := validateLiveAcceptanceCaseRoot(identity, quarantine, true); err != nil {
		return err
	}
	if current, err := identity.parent.Lstat(identity.name); err == nil {
		return fmt.Errorf("live acceptance cleanup refuses a replacement created at the case root: %s mode=%s", path, current.Mode())
	} else if !os.IsNotExist(err) {
		return err
	}
	quarantinePath := filepath.Join(identity.parentPath, quarantine)
	if err := validateLiveAcceptanceCleanupTree(quarantinePath); err != nil {
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

func liveAcceptanceCaseRoot(requested, pack string) (string, error) {
	var root string
	if strings.TrimSpace(requested) != "" {
		resolved, err := filepath.Abs(strings.TrimSpace(requested))
		if err != nil {
			return "", err
		}
		root = resolved
	} else {
		base, err := os.MkdirTemp("", "rekit-"+pack+"-live-acceptance-")
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
			SessionID: item.SessionID, Kind: item.SessionKind, Outcome: item.Outcome,
			Failure: cloneFailureDiagnosis(item.Failure), Diagnostics: append([]string{}, item.Diagnostics...),
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

func cloneFailureDiagnosis(failure *FailureDiagnosis) *FailureDiagnosis {
	if failure == nil {
		return nil
	}
	cloned := *failure
	return &cloned
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
		if inspection.TaskContext.OutputContract != nil {
			contract := *inspection.TaskContext.OutputContract
			contract.Fields = append([]string{}, inspection.TaskContext.OutputContract.Fields...)
			member.OutputContract = &contract
		}
		if inspection.TaskContext.Correction != nil {
			member.CorrectionSourceID = inspection.TaskContext.Correction.SourceEventID
		}
	}
	return member
}

func validateLiveAcceptanceOutputContract(pack string, inspection memberexecution.Inspection) error {
	if inspection.Intent == nil || inspection.TaskContext == nil || inspection.TaskContext.SchemaVersion != memberexecution.TaskContextSchemaVersion || inspection.TaskContext.OutputContract == nil {
		return fmt.Errorf("real member task context omitted the current pack output contract")
	}
	contract := inspection.TaskContext.OutputContract
	if !strings.EqualFold(inspection.TaskContext.Pack, pack) || contract.TaskType != "feature-analysis" || !strings.HasPrefix(contract.RouteID, pack+":") || len(contract.Fields) == 0 || len(contract.ManifestSHA256) != 64 || contract.ManifestPath != filepath.ToSlash(filepath.Join("packs", pack, "manifest.yml")) {
		return fmt.Errorf("real member task context did not bind the exact pack output contract: %+v", contract)
	}
	if err := memberexecution.ValidateTaskContextPackContract(inspection.Intent.CaseRoot, inspection); err != nil {
		return fmt.Errorf("real member task context pack output contract is not current: %w", err)
	}
	if pack == "web-security" && (!slices.Contains(contract.Fields, "endpoint") || !slices.Contains(contract.Fields, "feature")) {
		return fmt.Errorf("web-security member task context omitted its domain output fields: %+v", contract.Fields)
	}
	if inspection.Owner.ExecutorGeneration > 1 && inspection.TaskContext.Correction == nil {
		return fmt.Errorf("replacement member pack output contract is not correction-bound")
	}
	return nil
}

func liveAcceptanceRejection(rejection workstream.MemberReviewerRejection) *LiveAcceptanceRejection {
	return &LiveAcceptanceRejection{
		ManifestPath:              rejection.ManifestRef,
		ManifestSHA256:            rejection.ManifestSHA256,
		PacketID:                  rejection.PacketID,
		RouteID:                   rejection.RouteID,
		ShardID:                   rejection.ShardID,
		ReviewerResultInputSHA256: rejection.ReviewerResultInputSHA256,
		ReviewerSession:           rejection.ReviewerSession,
		VerificationEventID:       rejection.VerificationEventID,
		DecisionEventID:           rejection.DecisionEventID,
		Summary:                   rejection.Summary,
		EvidenceRefs:              append([]string{}, rejection.EvidenceRefs...),
		OwnerExecutor:             rejection.OwnerExecutor,
		OwnerGeneration:           rejection.OwnerGeneration,
	}
}

func liveAcceptanceAcceptance(acceptance workstream.MemberReviewerAcceptance) *LiveAcceptanceAcceptance {
	return &LiveAcceptanceAcceptance{
		ManifestPath:              acceptance.ManifestRef,
		ManifestSHA256:            acceptance.ManifestSHA256,
		PacketID:                  acceptance.PacketID,
		RouteID:                   acceptance.RouteID,
		ShardID:                   acceptance.ShardID,
		ReviewerResultInputSHA256: acceptance.ReviewerResultInputSHA256,
		ReviewerSession:           acceptance.ReviewerSession,
		VerificationEventID:       acceptance.VerificationEventID,
		DecisionEventID:           acceptance.DecisionEventID,
		OwnerExecutor:             acceptance.OwnerExecutor,
		OwnerGeneration:           acceptance.OwnerGeneration,
	}
}

func sameLiveAcceptanceStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
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
