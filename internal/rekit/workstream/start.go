package workstream

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

const defaultPolicyText = `schemaVersion: 1
automationMode: assisted-autopilot
autoCollect: true
autoVerify: true
autoRouteRequests: true
autoSyncLanes: true
autoPublishSharedFacts: true
autoAcceptLowRiskCandidates: true
authorityAutoAppend: conditional
authorityAutoOverwrite: never
authorityAutoDelete: never
requireEvidence: true
requireVerifier: true
minConfidence: 0.90
requireNoConflict: true
requireSchemaValid: true
requireBackup: true
requireDiff: true
maxAuthorityRowsPerRun: 10
askUserWhen: conflict,overwriteAuthority,deleteAuthority,confidenceBelowThreshold,schemaChange,changesProjectBaseline,externalSideEffect,destructiveAction
`

var safeLaneIDSegment = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$`)

type StartOptions struct {
	Name                  string
	Selector              string
	Force                 bool
	Actor                 string
	Executor              string
	TakeoverReason        string
	ExpectedPreviewSHA256 string
}

type StartWrite struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Action     string `json:"action"`
	TargetPath string `json:"targetPath,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

type StartResult struct {
	SchemaVersion                    int                                      `json:"schemaVersion"`
	Command                          string                                   `json:"command"`
	CaseRoot                         string                                   `json:"caseRoot"`
	RepoRoot                         string                                   `json:"repoRoot"`
	Pack                             string                                   `json:"pack"`
	IsMutation                       bool                                     `json:"isMutation"`
	Applied                          bool                                     `json:"applied"`
	RequiresConfirmation             bool                                     `json:"requiresConfirmation"`
	Lane                             Lane                                     `json:"lane"`
	AutonomyProfile                  autonomy.Summary                         `json:"autonomyProfile"`
	LaneTakeoverPackage              *LaneTakeoverPackage                     `json:"laneTakeoverPackage,omitempty"`
	MissionBrief                     mission.Brief                            `json:"missionBrief"`
	AuthorizedGateAdapterHandoffs    []AuthorizedGateAdapterHandoff           `json:"authorizedGateAdapterHandoffs,omitempty"`
	ReviewerDispatchIntakeHandoffs   []ReviewerDispatchIntakeHandoff          `json:"reviewerDispatchIntakeHandoffs,omitempty"`
	ReviewerDispatchIntakeSummary    ReviewerDispatchIntakeSummary            `json:"reviewerDispatchIntakeSummary"`
	ReviewerPacketRetirementHandoffs []ReviewerPacketRetirementHandoff        `json:"reviewerPacketRetirementHandoffs,omitempty"`
	ReviewerPacketRetirementSummary  ReviewerPacketRetirementSummary          `json:"reviewerPacketRetirementSummary"`
	PendingGateHandoffs              []ContinuePendingGateHandoff             `json:"pendingGateHandoffs,omitempty"`
	OpenDecisionHandoffs             []ContinueOpenDecisionHandoff            `json:"openDecisionHandoffs,omitempty"`
	ExecutorAction                   laneExecutorAction                       `json:"executorAction"`
	MissionCommanderAction           mission.MissionCommanderAction           `json:"missionCommanderAction"`
	MissionCommanderNextActions      []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions,omitempty"`
	MissionCommanderActionQueue      mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
	Writes                           []StartWrite                             `json:"writes"`
	BlockedActions                   []string                                 `json:"blockedActions"`
	NextSteps                        []string                                 `json:"nextSteps"`
}

type Lane struct {
	SchemaVersion              int            `json:"schemaVersion"`
	ID                         string         `json:"id"`
	Type                       string         `json:"type"`
	Name                       string         `json:"name"`
	Title                      string         `json:"title"`
	Status                     string         `json:"status"`
	Authority                  bool           `json:"authority"`
	Workspace                  string         `json:"workspace"`
	LaneRoot                   string         `json:"laneRoot"`
	CanWrite                   []string       `json:"canWrite"`
	ReadOnly                   []string       `json:"readOnly"`
	Outputs                    []string       `json:"outputs"`
	Counters                   map[string]int `json:"counters"`
	CurrentExecutor            string         `json:"currentExecutor,omitempty"`
	ExecutorGeneration         int            `json:"executorGeneration,omitempty"`
	LastTakeoverAt             string         `json:"lastTakeoverAt,omitempty"`
	LastTakeoverBy             string         `json:"lastTakeoverBy,omitempty"`
	LastTakeoverReason         string         `json:"lastTakeoverReason,omitempty"`
	LastReconciledIntervention string         `json:"lastReconciledIntervention,omitempty"`
	LastReconcileAt            string         `json:"lastReconcileAt,omitempty"`
	CreatedAt                  string         `json:"createdAt"`
	UpdatedAt                  string         `json:"updatedAt"`
}

type laneExecutorAction = mission.ExecutorAction

type ExecutionEvidenceReviewItem = mission.ExecutionEvidenceReviewItem

type ExecutionEvidenceReviewSummary = mission.ExecutionEvidenceReviewSummary

type laneCheckpoint struct {
	SchemaVersion                    int                                      `json:"schemaVersion"`
	Lane                             string                                   `json:"lane"`
	Status                           string                                   `json:"status"`
	Workspace                        string                                   `json:"workspace"`
	CurrentExecutor                  string                                   `json:"currentExecutor"`
	ExecutorGeneration               int                                      `json:"executorGeneration"`
	LastTakeoverAt                   string                                   `json:"lastTakeoverAt"`
	LastTakeoverBy                   string                                   `json:"lastTakeoverBy"`
	LastTakeoverReason               string                                   `json:"lastTakeoverReason"`
	LastReconciledIntervention       string                                   `json:"lastReconciledIntervention"`
	LastReconcileAt                  string                                   `json:"lastReconcileAt"`
	AutonomyProfile                  autonomy.Summary                         `json:"autonomyProfile"`
	LaneTakeoverPackage              *LaneTakeoverPackage                     `json:"laneTakeoverPackage,omitempty"`
	MissionBrief                     mission.Brief                            `json:"missionBrief"`
	ExecutorAction                   laneExecutorAction                       `json:"executorAction"`
	PendingGates                     []string                                 `json:"pendingGates"`
	AuthorizedGates                  []string                                 `json:"authorizedGates"`
	ExecutionEvidenceReview          []ExecutionEvidenceReviewItem            `json:"executionEvidenceReview,omitempty"`
	ExecutionEvidenceReviewSummary   ExecutionEvidenceReviewSummary           `json:"executionEvidenceReviewSummary"`
	ReviewerWritebacks               []ReviewerWritebackItem                  `json:"reviewerWritebacks,omitempty"`
	ReviewerWritebackSummary         ReviewerWritebackSummary                 `json:"reviewerWritebackSummary"`
	ReviewerDispatchIntakeHandoffs   []ReviewerDispatchIntakeHandoff          `json:"reviewerDispatchIntakeHandoffs,omitempty"`
	ReviewerDispatchIntakeSummary    ReviewerDispatchIntakeSummary            `json:"reviewerDispatchIntakeSummary"`
	ReviewerPacketRetirementHandoffs []ReviewerPacketRetirementHandoff        `json:"reviewerPacketRetirementHandoffs,omitempty"`
	ReviewerPacketRetirementSummary  ReviewerPacketRetirementSummary          `json:"reviewerPacketRetirementSummary"`
	AuthorizedGateAdapterHandoffs    []AuthorizedGateAdapterHandoff           `json:"authorizedGateAdapterHandoffs,omitempty"`
	MissionCommanderNextActions      []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions,omitempty"`
	MissionCommanderActionQueue      mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
	OpenInterventions                []InterventionSummary                    `json:"openInterventions"`
	Inbox                            int                                      `json:"inbox"`
	Tasks                            int                                      `json:"tasks"`
	UpdatedAt                        string                                   `json:"updatedAt"`
	Resume                           string                                   `json:"resume"`
}

type board = mission.Board

type boardLane = mission.BoardLane

func StartPreview(repoRoot, caseRoot, pack string, opt StartOptions) (StartResult, error) {
	inst, m, laneType, laneID, name, err := startContext(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return StartResult{}, err
	}
	claim, err := startExecutorClaim(opt, laneID)
	if err != nil {
		return StartResult{}, err
	}
	laneFile, err := refsf.SafeJoin(inst.CaseRoot, relJoin(".rekit", "lanes", laneID, "lane.json"))
	if err != nil {
		return StartResult{}, err
	}
	lane, err := plannedLane(inst.CaseRoot, laneType, laneID, name, "")
	if err != nil {
		return StartResult{}, err
	}
	action := "would-create-lane"
	if refsf.Exists(laneFile) {
		existingLane, err := readLane(laneFile)
		if err != nil {
			return StartResult{}, err
		}
		action = "would-enter-existing-lane"
		if opt.Force {
			action = "would-refresh-lane-with-force"
			preserveLaneRuntimeState(&lane, existingLane)
		} else {
			lane = existingLane
		}
	}
	if claim.Enabled() {
		var changed bool
		lane, changed = applyExecutorClaim(lane, claim, "")
		if changed {
			action += "-and-claim-executor"
		}
	}
	brief := startMissionBrief(inst.CaseRoot)
	authorizedGateAdapterHandoffs := authorizedGateAdapterHandoffsForLane(m.RepoRoot, inst.CaseRoot, m.Pack, lane.ID)
	reviewerDispatchIntakeHandoffs := reviewerDispatchIntakeHandoffsForLane(inst.CaseRoot, lane.ID)
	reviewerPacketRetirementHandoffs := reviewerPacketRetirementHandoffsForLane(inst.CaseRoot, lane.ID)
	pendingGateHandoffs, openDecisionHandoffs := gateDecisionHandoffsForLane(inst.CaseRoot, lane)
	executorAction := startExecutorAction(inst.CaseRoot, lane, brief)
	if strings.HasPrefix(action, "would-create-lane") || strings.Contains(action, "claim-executor") {
		executorAction.MissionCommanderAction = startApplyCommanderAction(lane, opt, claim)
	}
	commanderNextActions := startMissionCommanderNextActions(lane, executorAction)
	commanderNextActions = MissionCommanderNextActionsWithAuthorizedGateAdapters(commanderNextActions, authorizedGateAdapterHandoffs)
	commanderNextActions = MissionCommanderNextActionsWithReviewerDispatches(commanderNextActions, reviewerDispatchIntakeHandoffs)
	commanderActionQueue := mission.MissionCommanderActionQueueFor(commanderNextActions)
	return StartResult{
		SchemaVersion:                    1,
		Command:                          "start",
		CaseRoot:                         inst.CaseRoot,
		RepoRoot:                         m.RepoRoot,
		Pack:                             m.Pack,
		IsMutation:                       false,
		Applied:                          false,
		RequiresConfirmation:             true,
		Lane:                             lane,
		LaneTakeoverPackage:              laneTakeoverPackageFor(inst.CaseRoot, lane, executorAction, commanderActionQueue, true),
		MissionBrief:                     brief,
		AuthorizedGateAdapterHandoffs:    authorizedGateAdapterHandoffs,
		ReviewerDispatchIntakeHandoffs:   reviewerDispatchIntakeHandoffs,
		ReviewerDispatchIntakeSummary:    ReviewerDispatchIntakeSummaryFor(reviewerDispatchIntakeHandoffs),
		ReviewerPacketRetirementHandoffs: reviewerPacketRetirementHandoffs,
		ReviewerPacketRetirementSummary:  ReviewerPacketRetirementSummaryFor(reviewerPacketRetirementHandoffs),
		PendingGateHandoffs:              pendingGateHandoffs,
		OpenDecisionHandoffs:             openDecisionHandoffs,
		ExecutorAction:                   executorAction,
		MissionCommanderAction:           executorAction.MissionCommanderAction,
		MissionCommanderNextActions:      commanderNextActions,
		MissionCommanderActionQueue:      commanderActionQueue,
		Writes: []StartWrite{{
			Path:       relJoin(".rekit", "lanes", laneID, "lane.json"),
			Kind:       "lane",
			Action:     action,
			TargetPath: laneFile,
		}},
		AutonomyProfile: autonomy.ReadSummary(inst.CaseRoot, laneID, m),
		BlockedActions:  []string{"authority/confirmed writes", "heavy-tool execution without a valid current authorization decision", "handoff writes", "continue auto-apply"},
		NextSteps: []string{
			"review this plan, then re-run start with -Apply to create or enter the workstream",
			"use /rekit as the Mission Commander entrypoint; JSON preview/apply is Go-owned by default",
		},
	}, nil
}

func StartApply(repoRoot, caseRoot, pack string, opt StartOptions) (result StartResult, err error) {
	inst, m, laneType, laneID, name, err := startContext(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return StartResult{}, err
	}
	lease, err := acquireLaneMutationLock(inst.CaseRoot, laneID)
	if err != nil {
		return StartResult{}, err
	}
	defer func() {
		if unlockErr := lease.Unlock(); unlockErr != nil {
			err = errors.Join(err, unlockErr)
			result = StartResult{}
		}
	}()
	if err := lease.Validate(); err != nil {
		return StartResult{}, err
	}
	lockedInst, lockedManifest, lockedLaneType, lockedLaneID, lockedName, err := startContext(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return StartResult{}, err
	}
	if !strings.EqualFold(filepath.Clean(lockedInst.CaseRoot), filepath.Clean(inst.CaseRoot)) || lockedLaneID != laneID {
		return StartResult{}, fmt.Errorf("start target changed while acquiring lane mutation lease: got case=%s lane=%s, want case=%s lane=%s", lockedInst.CaseRoot, lockedLaneID, inst.CaseRoot, laneID)
	}
	inst, m, laneType, name = lockedInst, lockedManifest, lockedLaneType, lockedName
	claim, err := startExecutorClaim(opt, laneID)
	if err != nil {
		return StartResult{}, err
	}
	if strings.TrimSpace(opt.ExpectedPreviewSHA256) != "" {
		previewOpt := opt
		previewOpt.ExpectedPreviewSHA256 = ""
		preview, err := StartPreview(repoRoot, caseRoot, pack, previewOpt)
		if err != nil {
			return StartResult{}, err
		}
		encoded, err := json.Marshal(preview)
		if err != nil {
			return StartResult{}, err
		}
		sum := sha256.Sum256(encoded)
		actual := hex.EncodeToString(sum[:])
		if !strings.EqualFold(strings.TrimSpace(opt.ExpectedPreviewSHA256), actual) {
			return StartResult{}, fmt.Errorf("start preview sha256 mismatch: got %s want %s", opt.ExpectedPreviewSHA256, actual)
		}
	}
	writes := []StartWrite{}
	if err := ensureWorkstreamState(inst.CaseRoot, m, &writes); err != nil {
		return StartResult{}, err
	}
	lane, laneWrites, err := writeLane(inst.CaseRoot, m, laneType, laneID, name, opt.Force, claim)
	if err != nil {
		return StartResult{}, err
	}
	writes = append(writes, laneWrites...)
	boardPath, err := saveBoard(inst.CaseRoot, m)
	if err != nil {
		return StartResult{}, err
	}
	writes = append(writes, StartWrite{Path: ".rekit/board.json", Kind: "board", Action: "refresh", TargetPath: boardPath})
	resumePath, checkpointPath, err := writeLaneResume(inst.CaseRoot, m, lane)
	if err != nil {
		return StartResult{}, err
	}
	writes = append(writes,
		StartWrite{Path: relativePath(inst.CaseRoot, resumePath), Kind: "lane-resume", Action: "refresh", TargetPath: resumePath},
		StartWrite{Path: relativePath(inst.CaseRoot, checkpointPath), Kind: "lane-checkpoint", Action: "refresh", TargetPath: checkpointPath},
	)
	brief := startMissionBrief(inst.CaseRoot)
	authorizedGateAdapterHandoffs := authorizedGateAdapterHandoffsForLane(m.RepoRoot, inst.CaseRoot, m.Pack, lane.ID)
	reviewerDispatchIntakeHandoffs := reviewerDispatchIntakeHandoffsForLane(inst.CaseRoot, lane.ID)
	reviewerPacketRetirementHandoffs := reviewerPacketRetirementHandoffsForLane(inst.CaseRoot, lane.ID)
	pendingGateHandoffs, openDecisionHandoffs := gateDecisionHandoffsForLane(inst.CaseRoot, lane)
	executorAction := startExecutorAction(inst.CaseRoot, lane, brief)
	executorAction = withReviewerDispatchBlocker(executorAction, reviewerDispatchIntakeHandoffs)
	commanderNextActions := startMissionCommanderNextActions(lane, executorAction)
	commanderNextActions = MissionCommanderNextActionsWithAuthorizedGateAdapters(commanderNextActions, authorizedGateAdapterHandoffs)
	commanderNextActions = MissionCommanderNextActionsWithReviewerDispatches(commanderNextActions, reviewerDispatchIntakeHandoffs)
	commanderActionQueue := mission.MissionCommanderActionQueueFor(commanderNextActions)
	return StartResult{
		SchemaVersion:                    1,
		Command:                          "start",
		CaseRoot:                         inst.CaseRoot,
		RepoRoot:                         m.RepoRoot,
		Pack:                             m.Pack,
		IsMutation:                       true,
		Applied:                          true,
		RequiresConfirmation:             false,
		Lane:                             lane,
		AutonomyProfile:                  autonomy.ReadSummary(inst.CaseRoot, lane.ID, m),
		LaneTakeoverPackage:              laneTakeoverPackageFor(inst.CaseRoot, lane, executorAction, commanderActionQueue, false),
		MissionBrief:                     brief,
		AuthorizedGateAdapterHandoffs:    authorizedGateAdapterHandoffs,
		ReviewerDispatchIntakeHandoffs:   reviewerDispatchIntakeHandoffs,
		ReviewerDispatchIntakeSummary:    ReviewerDispatchIntakeSummaryFor(reviewerDispatchIntakeHandoffs),
		ReviewerPacketRetirementHandoffs: reviewerPacketRetirementHandoffs,
		ReviewerPacketRetirementSummary:  ReviewerPacketRetirementSummaryFor(reviewerPacketRetirementHandoffs),
		PendingGateHandoffs:              pendingGateHandoffs,
		OpenDecisionHandoffs:             openDecisionHandoffs,
		ExecutorAction:                   executorAction,
		MissionCommanderAction:           executorAction.MissionCommanderAction,
		MissionCommanderNextActions:      commanderNextActions,
		MissionCommanderActionQueue:      commanderActionQueue,
		Writes:                           writes,
		BlockedActions:                   []string{"authority/confirmed writes", "heavy-tool execution without a valid current authorization decision", "handoff writes", "continue auto-apply"},
		NextSteps:                        workstreamNextSteps(executorAction, true),
	}, nil
}

func workstreamNextSteps(action laneExecutorAction, includeDoctor bool) []string {
	next := []string{}
	if includeDoctor {
		next = append(next, "run doctor after apply")
	}
	return append(next, action.NextAgentActions...)
}

func startMissionCommanderNextActions(lane Lane, action laneExecutorAction) []mission.MissionCommanderNextActionItem {
	items := mission.MissionCommanderNextActions([]mission.LaneExecutorActionSnapshot{laneCommanderActionSnapshot(lane, action)}, nil, action.Blocked)
	if action.MissionCommanderAction.State != "needs-start-apply" {
		return items
	}
	for idx := range items {
		items[idx].RequiresReview = true
		items[idx].Reasons = append(items[idx].Reasons, "review start preview before applying case-local lane/board/resume/checkpoint writes")
		if items[idx].Source == "missionCommanderActions" {
			items[idx].Blocked = false
			items[idx].Reasons = append(items[idx].Reasons, "start apply only performs the bounded case-local lane/executor takeover")
		}
		if items[idx].Source == "missionCommanderActions.followUp" {
			items[idx].Blocked = true
			items[idx].Reasons = append(items[idx].Reasons, "run only after start apply succeeds and the refreshed executor action remains ready")
		}
	}
	return items
}

func startApplyCommanderAction(lane Lane, opt StartOptions, claim executorClaim) mission.MissionCommanderAction {
	label := workstreamLabel(lane)
	return mission.MissionCommanderAction{
		State:          "needs-start-apply",
		Prompt:         fmt.Sprintf("按 `%s` 创建、进入或接管 lane；先 review start plan，再 apply。", label),
		PrimaryCommand: startApplyCommand(label, opt, claim),
		FollowUpCommands: []string{
			bindContinueCommand("/rekit continue "+label, lane),
			"/rekit handoff " + label,
		},
		Boundary: []string{
			"no authority/confirmed writes",
			"no heavy-tool execution",
			"start apply only writes case-local lane/board/resume/checkpoint state",
		},
	}
}

func startApplyCommand(label string, opt StartOptions, claim executorClaim) string {
	parts := []string{"/rekit", "start", label, "-Apply"}
	if opt.Force {
		parts = append(parts, "-Force")
	}
	if claim.Enabled() {
		parts = append(parts, "-Executor", claim.Executor, "-Actor", claim.Actor, "-Reason", claim.Reason)
	}
	for i, part := range parts {
		parts[i] = quoteCommandArg(part)
	}
	return strings.Join(parts, " ")
}

func quoteCommandArg(arg string) string {
	if arg == "" {
		return `""`
	}
	if !strings.ContainsAny(arg, " \t\"") {
		return arg
	}
	return `"` + strings.ReplaceAll(arg, `"`, `\"`) + `"`
}

func startMissionBrief(caseRoot string) mission.Brief {
	brief, err := mission.CaseBrief(caseRoot, mission.BuildOptions{
		MaxRows:            maxHandoffRows,
		OpenDecisionAction: "review open candidate/decision item(s) with evidence and authority boundary",
	})
	if err != nil {
		if os.IsNotExist(err) {
			return mission.Build(nil, mission.Facts{}, maxHandoffRows)
		}
		return mission.Brief{Summary: "unavailable: " + err.Error()}
	}
	return brief
}

func startExecutorAction(caseRoot string, lane Lane, brief mission.Brief) laneExecutorAction {
	facts, err := mission.ReadFacts(caseRoot)
	if err != nil {
		return laneExecutorActionFor(lane, mission.Facts{}, brief)
	}
	return laneExecutorActionFor(lane, facts, brief)
}

func reviewerDispatchIntakeHandoffsForLane(caseRoot, laneID string) []ReviewerDispatchIntakeHandoff {
	facts, err := mission.ReadLedgerFacts(caseRoot)
	if err != nil {
		return nil
	}
	items, err := ReviewerDispatchIntakeHandoffs(caseRoot, facts, laneID)
	if err != nil {
		return nil
	}
	return items
}

func reviewerPacketRetirementHandoffsForLane(caseRoot, laneID string) []ReviewerPacketRetirementHandoff {
	items, err := ReviewerPacketRetirementHandoffs(caseRoot, laneID)
	if err != nil {
		return nil
	}
	return items
}

func EnsureBoard(repoRoot, caseRoot, pack string) error {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return err
	}
	writes := []StartWrite{}
	if err := ensureWorkstreamState(inst.CaseRoot, m, &writes); err != nil {
		return err
	}
	_, err = saveBoard(inst.CaseRoot, m)
	return err
}

func startContext(repoRoot, caseRoot, pack string, opt StartOptions) (instance.Instance, *manifest.Manifest, manifest.LaneType, string, string, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return instance.Instance{}, nil, manifest.LaneType{}, "", "", err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return instance.Instance{}, nil, manifest.LaneType{}, "", "", err
	}
	name := strings.TrimSpace(opt.Name)
	selector := strings.TrimSpace(opt.Selector)
	if name == "" {
		name = selector
	}
	if name == "" {
		return instance.Instance{}, nil, manifest.LaneType{}, "", "", fmt.Errorf("start requires a feature name or lane selector, e.g. /rekit start login")
	}
	laneTypeID := strings.TrimSpace(m.WorkstreamDefaults["defaultStartLaneType"])
	if laneTypeID == "" {
		laneTypeID = defaultStartLaneType(m)
	}
	resolvedLaneID := ""
	if selector != "" {
		if b, err := readBoard(inst.CaseRoot); err == nil {
			if strings.TrimSpace(b.DefaultAuthorityLane) == "" {
				b.DefaultAuthorityLane = m.WorkstreamDefaults["defaultAuthorityLane"]
			}
			if lane, err := resolveHandoffLane(inst.CaseRoot, b, selector); err == nil {
				laneTypeID = lane.Type
				resolvedLaneID = lane.ID
				name = lane.Name
			}
		}
	}
	laneType, err := m.LaneType(laneTypeID)
	if err != nil {
		return instance.Instance{}, nil, manifest.LaneType{}, "", "", err
	}
	if resolvedLaneID == "" {
		resolvedLaneID = laneID(laneType.ID, name)
	}
	if resolvedLaneID == "" {
		return instance.Instance{}, nil, manifest.LaneType{}, "", "", fmt.Errorf("start produced an empty lane id for %q", name)
	}
	return inst, m, laneType, resolvedLaneID, name, nil
}

type executorClaim struct {
	Executor string
	Actor    string
	Reason   string
}

func (claim executorClaim) Enabled() bool {
	return strings.TrimSpace(claim.Executor) != ""
}

func startExecutorClaim(opt StartOptions, laneID string) (executorClaim, error) {
	executor := strings.TrimSpace(opt.Executor)
	if executor == "" {
		return executorClaim{}, nil
	}
	if strings.ContainsAny(executor, "\r\n") {
		return executorClaim{}, fmt.Errorf("start -Executor must be a single-line session identifier")
	}
	actor := strings.TrimSpace(opt.Actor)
	if actor == "" {
		actor = "main-agent"
	}
	if strings.ContainsAny(actor, "\r\n") {
		return executorClaim{}, fmt.Errorf("start -Actor must be a single-line actor identifier")
	}
	reason := strings.TrimSpace(opt.TakeoverReason)
	if reason == "" {
		reason = "explicit executor takeover for lane " + laneID
	}
	if strings.ContainsAny(reason, "\r\n") {
		return executorClaim{}, fmt.Errorf("start -Reason must be a single-line takeover reason")
	}
	return executorClaim{Executor: executor, Actor: actor, Reason: reason}, nil
}

func applyExecutorClaim(lane Lane, claim executorClaim, now string) (Lane, bool) {
	if !claim.Enabled() {
		return lane, false
	}
	previousExecutor := strings.TrimSpace(lane.CurrentExecutor)
	generation := max(lane.ExecutorGeneration, 0)
	if strings.EqualFold(previousExecutor, claim.Executor) {
		if generation == 0 {
			lane.ExecutorGeneration = 1
			if now != "" {
				lane.LastTakeoverAt = now
			}
			lane.LastTakeoverBy = claim.Actor
			lane.LastTakeoverReason = claim.Reason
			return lane, true
		}
		return lane, false
	}
	generation++
	lane.CurrentExecutor = claim.Executor
	lane.ExecutorGeneration = generation
	if now != "" {
		lane.LastTakeoverAt = now
	}
	lane.LastTakeoverBy = claim.Actor
	lane.LastTakeoverReason = claim.Reason
	return lane, true
}

func preserveLaneRuntimeState(lane *Lane, existing Lane) {
	if strings.TrimSpace(existing.Status) != "" {
		lane.Status = existing.Status
	}
	if existing.Counters != nil {
		lane.Counters = existing.Counters
	}
	lane.CurrentExecutor = existing.CurrentExecutor
	lane.ExecutorGeneration = existing.ExecutorGeneration
	lane.LastTakeoverAt = existing.LastTakeoverAt
	lane.LastTakeoverBy = existing.LastTakeoverBy
	lane.LastTakeoverReason = existing.LastTakeoverReason
	lane.LastReconciledIntervention = existing.LastReconciledIntervention
	lane.LastReconcileAt = existing.LastReconcileAt
	if strings.TrimSpace(existing.CreatedAt) != "" {
		lane.CreatedAt = existing.CreatedAt
	}
}

func appendExecutorClaimEvent(laneRoot, laneRootRel string, lane Lane, previousExecutor string, claim executorClaim, now string) (StartWrite, error) {
	eventKind := "executor-registered"
	eventAction := "append-executor-registered"
	summary := "executor registered: " + claim.Executor
	if strings.TrimSpace(previousExecutor) != "" && !strings.EqualFold(previousExecutor, claim.Executor) {
		eventKind = "executor-takeover"
		eventAction = "append-executor-takeover"
		summary = "executor takeover: " + claim.Executor
	}
	eventPath := LaneEventsJSONLPath(laneRoot)
	event := map[string]any{
		"eventId":            eventID(lane.ID, eventKind+"-"+claim.Executor, now),
		"kind":               eventKind,
		"lane":               lane.ID,
		"time":               now,
		"summary":            summary,
		"previousExecutor":   previousExecutor,
		"currentExecutor":    claim.Executor,
		"executorGeneration": lane.ExecutorGeneration,
		"actor":              claim.Actor,
		"reason":             claim.Reason,
		"sourceCommand":      "start",
	}
	if err := mission.AppendJSONLine(eventPath, event); err != nil {
		return StartWrite{}, err
	}
	return StartWrite{Path: relJoin(laneRootRel, "events.jsonl"), Kind: "lane-event", Action: eventAction, TargetPath: eventPath}, nil
}

func ensureWorkstreamState(caseRoot string, m *manifest.Manifest, writes *[]StartWrite) error {
	for _, rel := range []string{".rekit", ".rekit/lanes", ".rekit/facts", ".rekit/runs", ".rekit/reviews", ".rekit/backups"} {
		path, err := refsf.SafeJoin(caseRoot, rel)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
	}
	for _, rel := range mission.FactRelPaths() {
		path, err := refsf.SafeJoin(caseRoot, rel)
		if err != nil {
			return err
		}
		action, err := ensureEmptyFile(path)
		if err != nil {
			return err
		}
		*writes = append(*writes, StartWrite{Path: rel, Kind: "fact-jsonl", Action: action, TargetPath: path})
	}
	policyPath, err := refsf.SafeJoin(caseRoot, ".rekit/policy.yml")
	if err != nil {
		return err
	}
	policyAction := "unchanged"
	if !refsf.Exists(policyPath) {
		if err := os.WriteFile(policyPath, []byte(defaultPolicyText), 0o644); err != nil {
			return err
		}
		policyAction = "create-policy"
	}
	*writes = append(*writes, StartWrite{Path: ".rekit/policy.yml", Kind: "policy", Action: policyAction, TargetPath: policyPath})

	authorityType, err := m.LaneType(m.WorkstreamDefaults["defaultAuthorityLane"])
	if err != nil {
		return err
	}
	authorityID := laneID(authorityType.ID, "")
	authorityFile, err := refsf.SafeJoin(caseRoot, relJoin(".rekit", "lanes", authorityID, "lane.json"))
	if err != nil {
		return err
	}
	if !refsf.Exists(authorityFile) {
		_, laneWrites, err := writeLane(caseRoot, m, authorityType, authorityID, "", false, executorClaim{})
		if err != nil {
			return err
		}
		*writes = append(*writes, laneWrites...)
	}
	return nil
}

var workstreamMutationAfterLockHook func(*workstreamMutationLease)

type workstreamMutationLease = lanemutation.Lease

func acquireLaneMutationLock(caseRoot, laneID string) (*workstreamMutationLease, error) {
	lease, err := lanemutation.AcquireLane(caseRoot, laneID)
	if err != nil {
		return nil, err
	}
	if workstreamMutationAfterLockHook != nil {
		workstreamMutationAfterLockHook(lease)
	}
	if err := lease.Validate(); err != nil {
		return nil, errors.Join(err, lease.Unlock())
	}
	return lease, nil
}

func acquireProjectMutationLock(caseRoot string) (*workstreamMutationLease, error) {
	lease, err := lanemutation.AcquireProject(caseRoot)
	if err != nil {
		return nil, err
	}
	if workstreamMutationAfterLockHook != nil {
		workstreamMutationAfterLockHook(lease)
	}
	if err := lease.Validate(); err != nil {
		return nil, errors.Join(err, lease.Unlock())
	}
	return lease, nil
}

func writeLane(caseRoot string, m *manifest.Manifest, laneType manifest.LaneType, id, name string, force bool, claim executorClaim) (Lane, []StartWrite, error) {
	laneRootRel := relJoin(".rekit", "lanes", id)
	laneRoot, err := refsf.SafeJoin(caseRoot, laneRootRel)
	if err != nil {
		return Lane{}, nil, err
	}
	laneFile := filepath.Join(laneRoot, "lane.json")
	laneExists := refsf.Exists(laneFile)
	if laneExists && !force {
		lane, err := readLane(laneFile)
		if err != nil {
			return Lane{}, nil, err
		}
		writes := []StartWrite{{Path: relJoin(laneRootRel, "lane.json"), Kind: "lane", Action: "enter-existing-lane", TargetPath: laneFile}}
		if claim.Enabled() {
			now := time.Now().UTC().Format(time.RFC3339Nano)
			previousExecutor := strings.TrimSpace(lane.CurrentExecutor)
			updatedLane, changed := applyExecutorClaim(lane, claim, now)
			if changed {
				lane = updatedLane
				lane.UpdatedAt = now
				if err := writeJSON(laneFile, lane); err != nil {
					return Lane{}, nil, err
				}
				writes[0].Action = "update-executor-claim"
				if strings.TrimSpace(previousExecutor) != "" && !strings.EqualFold(previousExecutor, claim.Executor) {
					writes[0].Action = "update-executor-takeover"
				}
				eventWrite, err := appendExecutorClaimEvent(laneRoot, laneRootRel, lane, previousExecutor, claim, now)
				if err != nil {
					return Lane{}, nil, err
				}
				writes = append(writes, eventWrite)
			}
		}
		profileRel, profilePath, err := autonomy.EnsureManualProfile(caseRoot, id)
		if err != nil {
			return Lane{}, nil, err
		}
		writes = append(writes, StartWrite{Path: profileRel, Kind: "autonomy-profile", Action: "ensure-manual-profile", TargetPath: profilePath})
		return lane, writes, nil
	}
	var existingLane Lane
	if laneExists {
		existingLane, err = readLane(laneFile)
		if err != nil {
			return Lane{}, nil, err
		}
	}
	workspaceRel := relJoin(laneType.WorkspaceRoot, id)
	workspace, err := refsf.SafeJoin(caseRoot, workspaceRel)
	if err != nil {
		return Lane{}, nil, err
	}
	for _, path := range []string{laneRoot, filepath.Join(laneRoot, "checkpoints"), filepath.Join(laneRoot, "prompts"), filepath.Join(laneRoot, "reviews"), workspace} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return Lane{}, nil, err
		}
	}
	writes := []StartWrite{}
	for i, path := range LaneJSONLPaths(laneRoot) {
		file := LaneJSONLFileNames()[i]
		action, err := ensureEmptyFile(path)
		if err != nil {
			return Lane{}, nil, err
		}
		writes = append(writes, StartWrite{Path: relJoin(laneRootRel, file), Kind: "lane-jsonl", Action: action, TargetPath: path})
	}
	for i, path := range WorkspaceJSONLPaths(workspace) {
		file := WorkspaceJSONLFileNames()[i]
		action, err := ensureEmptyFile(path)
		if err != nil {
			return Lane{}, nil, err
		}
		writes = append(writes, StartWrite{Path: relJoin(workspaceRel, file), Kind: "workspace-jsonl", Action: action, TargetPath: path})
	}
	if !laneType.Authority {
		for _, file := range []string{"summary.md", "evidence.md", "notes.md"} {
			path := filepath.Join(workspace, file)
			action := "unchanged"
			if !refsf.Exists(path) {
				if err := os.WriteFile(path, []byte("# "+id+"\r\n\r\n待填写。\r\n"), 0o644); err != nil {
					return Lane{}, nil, err
				}
				action = "create-workspace-note"
			}
			writes = append(writes, StartWrite{Path: relJoin(workspaceRel, file), Kind: "workspace-note", Action: action, TargetPath: path})
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	lane, err := plannedLane(caseRoot, laneType, id, name, now)
	if err != nil {
		return Lane{}, nil, err
	}
	action := "create-lane"
	eventKind := "lane-created"
	eventAction := "append-lane-created"
	eventSummary := "lane created: " + id
	if laneExists && force {
		action = "refresh-lane-with-force"
		eventKind = "lane-refreshed"
		eventAction = "append-lane-refreshed"
		eventSummary = "lane refreshed: " + id
		preserveLaneRuntimeState(&lane, existingLane)
	}
	previousExecutor := strings.TrimSpace(lane.CurrentExecutor)
	executorChanged := false
	if claim.Enabled() {
		lane, executorChanged = applyExecutorClaim(lane, claim, now)
		if executorChanged {
			action += "-and-executor-claim"
		}
	}
	if err := writeJSON(laneFile, lane); err != nil {
		return Lane{}, nil, err
	}
	writes = append(writes, StartWrite{Path: relJoin(laneRootRel, "lane.json"), Kind: "lane", Action: action, TargetPath: laneFile})
	profileRel, profilePath, err := autonomy.EnsureManualProfile(caseRoot, id)
	if err != nil {
		return Lane{}, nil, err
	}
	writes = append(writes, StartWrite{Path: profileRel, Kind: "autonomy-profile", Action: "ensure-manual-profile", TargetPath: profilePath})
	eventPath := LaneEventsJSONLPath(laneRoot)
	event := map[string]any{"eventId": eventID(id, eventKind, now), "kind": eventKind, "lane": id, "time": now, "summary": eventSummary}
	if err := mission.AppendJSONLine(eventPath, event); err != nil {
		return Lane{}, nil, err
	}
	writes = append(writes, StartWrite{Path: relJoin(laneRootRel, "events.jsonl"), Kind: "lane-event", Action: eventAction, TargetPath: eventPath})
	if executorChanged {
		eventWrite, err := appendExecutorClaimEvent(laneRoot, laneRootRel, lane, previousExecutor, claim, now)
		if err != nil {
			return Lane{}, nil, err
		}
		writes = append(writes, eventWrite)
	}
	resumePath, checkpointPath, err := writeLaneResume(caseRoot, m, lane)
	if err != nil {
		return Lane{}, nil, err
	}
	writes = append(writes, StartWrite{Path: relJoin(laneRootRel, "prompts", "RESUME.md"), Kind: "lane-resume", Action: "refresh", TargetPath: resumePath})
	writes = append(writes, StartWrite{Path: relJoin(laneRootRel, "checkpoints", "latest.json"), Kind: "lane-checkpoint", Action: "refresh", TargetPath: checkpointPath})
	return lane, writes, nil
}

func plannedLane(caseRoot string, laneType manifest.LaneType, id, name, now string) (Lane, error) {
	workspaceRel := relJoin(laneType.WorkspaceRoot, id)
	workspace, err := refsf.SafeJoin(caseRoot, workspaceRel)
	if err != nil {
		return Lane{}, err
	}
	laneRoot, err := refsf.SafeJoin(caseRoot, relJoin(".rekit", "lanes", id))
	if err != nil {
		return Lane{}, err
	}
	title := laneType.Title
	if strings.TrimSpace(name) != "" {
		title += ": " + name
	}
	return Lane{
		SchemaVersion: 1,
		ID:            id,
		Type:          laneType.ID,
		Name:          name,
		Title:         title,
		Status:        "open",
		Authority:     laneType.Authority,
		Workspace:     relativePath(caseRoot, workspace),
		LaneRoot:      relativePath(caseRoot, laneRoot),
		CanWrite:      append([]string{}, laneType.CanWrite...),
		ReadOnly:      append([]string{}, laneType.ReadOnly...),
		Outputs:       append([]string{}, laneType.Outputs...),
		Counters:      map[string]int{"observations": 0, "requests": 0, "candidates": 0, "publications": 0, "pendingUser": 0},
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func saveBoard(caseRoot string, m *manifest.Manifest) (string, error) {
	lanesRoot, err := refsf.SafeJoin(caseRoot, ".rekit/lanes")
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(lanesRoot)
	if err != nil {
		return "", err
	}
	lanes := []boardLane{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		lane, err := readLane(filepath.Join(lanesRoot, entry.Name(), "lane.json"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		lanes = append(lanes, boardLane{ID: lane.ID, Type: lane.Type, Title: lane.Title, Status: lane.Status, Authority: lane.Authority, Workspace: lane.Workspace, CurrentExecutor: lane.CurrentExecutor, ExecutorGeneration: lane.ExecutorGeneration, LastTakeoverAt: lane.LastTakeoverAt, LastTakeoverBy: lane.LastTakeoverBy, LastTakeoverReason: lane.LastTakeoverReason, UpdatedAt: lane.UpdatedAt})
	}
	sort.SliceStable(lanes, func(i, j int) bool { return lanes[i].ID < lanes[j].ID })
	path, err := refsf.SafeJoin(caseRoot, ".rekit/board.json")
	if err != nil {
		return "", err
	}
	b := board{SchemaVersion: 1, CaseRoot: caseRoot, RepoRoot: m.RepoRoot, Pack: m.Pack, AutomationMode: readAutomationMode(caseRoot), DefaultAuthorityLane: m.WorkstreamDefaults["defaultAuthorityLane"], Lanes: lanes, FactsRoot: ".rekit/facts", UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	return path, writeJSON(path, b)
}

type laneResumePublication struct {
	ResumePath      string
	ResumeBytes     []byte
	CheckpointPath  string
	CheckpointBytes []byte
}

func buildLaneResumePublication(caseRoot string, m *manifest.Manifest, lane Lane, updatedAt ...string) (laneResumePublication, error) {
	laneRoot, err := laneRootPath(caseRoot, lane)
	if err != nil {
		return laneResumePublication{}, err
	}
	inbox, err := mission.ReadJSONLineObjects(LaneInboxJSONLPath(laneRoot))
	if err != nil {
		return laneResumePublication{}, err
	}
	tasks, err := mission.ReadJSONLineObjects(LaneTasksJSONLPath(laneRoot))
	if err != nil {
		return laneResumePublication{}, err
	}
	openInterventions, err := openLaneInterventionSummaries(caseRoot, lane.ID)
	if err != nil {
		return laneResumePublication{}, err
	}
	ledgerFacts, err := mission.ReadLedgerFacts(caseRoot)
	if err != nil {
		return laneResumePublication{}, err
	}
	laneFacts := mission.LaneFacts(ledgerFacts.Facts, lane.ID)
	reviewerWritebacks := ReviewerWritebackItems(ledgerFacts, lane.ID)
	reviewerDispatchIntakeHandoffs, err := ReviewerDispatchIntakeHandoffs(caseRoot, ledgerFacts, lane.ID)
	if err != nil {
		return laneResumePublication{}, err
	}
	reviewerPacketRetirementHandoffs, err := ReviewerPacketRetirementHandoffs(caseRoot, lane.ID)
	if err != nil {
		return laneResumePublication{}, err
	}
	brief := laneMissionBrief(lane, ledgerFacts)
	pendingGateLines := missionLines(mission.FilterLane(laneFacts.Requests, lane.ID, "pending-gate"), mission.LaneGateLine)
	authorizedGateLines := missionLines(mission.FilterLane(laneFacts.Requests, lane.ID, "authorized-gate"), mission.LaneGateLine)
	authorizedGateAdapterHandoffs := AuthorizedGateAdapterHandoffsWithAcknowledgements(m.RepoRoot, caseRoot, m.Pack, ledgerFacts.Requests, lane.ID, ExecutionEvidenceReviewAcknowledgedIDs(ledgerFacts))
	executionEvidenceReview := bindExecutionEvidenceReviewContinueCommands(laneExecutionEvidenceReview(lane, ledgerFacts), func(string) (Lane, bool) {
		return lane, true
	})
	autonomySummary := autonomy.ReadSummary(caseRoot, lane.ID, m)
	executorAction := laneExecutorActionFor(lane, laneFacts, brief)
	executorAction = withReviewerDispatchBlocker(executorAction, reviewerDispatchIntakeHandoffs)
	missionCommanderNextActions := mission.MissionCommanderNextActions([]mission.LaneExecutorActionSnapshot{laneCommanderActionSnapshot(lane, executorAction)}, executionEvidenceReview, executorAction.Blocked)
	missionCommanderNextActions = MissionCommanderNextActionsWithAuthorizedGateAdaptersAndAcknowledgements(missionCommanderNextActions, authorizedGateAdapterHandoffs, ExecutionEvidenceReviewAcknowledgedIDs(ledgerFacts))
	missionCommanderNextActions = MissionCommanderNextActionsWithReviewerDispatches(missionCommanderNextActions, reviewerDispatchIntakeHandoffs)
	missionCommanderActionQueue := mission.MissionCommanderActionQueueFor(missionCommanderNextActions)
	laneTakeoverPackage := laneTakeoverPackageFor(caseRoot, lane, executorAction, missionCommanderActionQueue, false)
	lines := []string{
		"# RESUME：" + lane.ID,
		"",
		"lane type: `" + lane.Type + "`",
		"status: `" + lane.Status + "`",
		"workspace: `" + lane.Workspace + "`",
		"current executor: `" + firstText(lane.CurrentExecutor, "unassigned") + "`",
		fmt.Sprintf("executor generation: `%d`", lane.ExecutorGeneration),
		"last takeover at: `" + firstText(lane.LastTakeoverAt, "none") + "`",
		"last takeover by: `" + firstText(lane.LastTakeoverBy, "none") + "`",
		"last takeover reason: `" + firstText(lane.LastTakeoverReason, "none") + "`",
		"last reconciled intervention: `" + firstText(lane.LastReconciledIntervention, "none") + "`",
		"autonomy mode: `" + firstText(autonomySummary.Mode, autonomy.ModeManualGate) + "`",
		"autonomy profile: `" + firstText(autonomySummary.ProfilePath, autonomy.RelPath(lane.ID)) + "`",
		"autonomy ready: `" + fmt.Sprintf("%t", autonomySummary.Ready) + "`",
		"",
		"## 边界",
		"",
		"- 只写本工作线 workspace，除非 lane.json 明确列入 canWrite。",
		"- authority 文件只由主线或 policy gate 写入。",
		"- 新发现写入 outbox.jsonl / workspace observations.jsonl / requests.jsonl / candidates.jsonl。",
		"- autonomy profile 只限定 heavy-action 预授权边界；不放宽 authority/confirmed/sync/promote。",
		"- heavy-tool execution 仍需有效 gate authorization decision，并且执行后必须记录 evidence。",
		"",
		"## Autonomy profile",
		"",
		"- mode: `" + firstText(autonomySummary.Mode, autonomy.ModeManualGate) + "`",
		"- profile: `" + firstText(autonomySummary.ProfilePath, autonomy.RelPath(lane.ID)) + "`",
		"- ready: `" + fmt.Sprintf("%t", autonomySummary.Ready) + "` valid=`" + fmt.Sprintf("%t", autonomySummary.Valid) + "` expired=`" + fmt.Sprintf("%t", autonomySummary.Expired) + "`",
		"- allowed actions: `" + firstText(strings.Join(autonomySummary.AllowedActions, ","), "none") + "`",
		"- denied actions: `" + firstText(strings.Join(autonomySummary.DeniedActions, ","), "none") + "`",
		"- output paths: `" + firstText(strings.Join(autonomySummary.OutputPaths, ","), "none") + "`",
		"- record required: `" + fmt.Sprintf("%t", autonomySummary.RecordRequired) + "`",
		"",
		"## Mission Control brief",
		"",
		"- summary: " + brief.Summary,
	}
	lines = appendResumeList(lines, "ready lanes", brief.ReadyLanes)
	lines = appendResumeList(lines, "blocked lanes", brief.BlockedLanes)
	lines = appendResumeList(lines, "pending gates", brief.PendingGates)
	lines = appendResumeList(lines, "authorized gates", brief.AuthorizedGates)
	lines = AppendAuthorizedGateAdapterHandoffDigest(lines, "Authorized gate adapter handoff", authorizedGateAdapterHandoffs)
	lines = appendResumeList(lines, "open decisions", brief.OpenDecisions)
	lines = appendResumeList(lines, "interventions", brief.Interventions)
	lines = appendResumeList(lines, "next agent actions", brief.NextAgentActions)
	lines = appendResumeList(lines, "escalations", brief.Escalations)
	lines = appendLaneTakeoverPackage(lines, laneTakeoverPackage)
	lines = append(lines,
		"",
		"## Executor action snapshot",
		"",
		"- blocked: `"+fmt.Sprintf("%t", executorAction.Blocked)+"`",
		"- ready: `"+fmt.Sprintf("%t", executorAction.Ready)+"`",
		"- pending gates: `"+fmt.Sprintf("%d", executorAction.PendingGates)+"`",
		"- open interventions: `"+fmt.Sprintf("%d", executorAction.OpenInterventions)+"`",
		"- open decisions: `"+fmt.Sprintf("%d", executorAction.OpenDecisions)+"`",
		"- reconcile required: `"+fmt.Sprintf("%t", executorAction.ReconcileRequired)+"`",
		"- pending gate required: `"+fmt.Sprintf("%t", executorAction.PendingGateRequired)+"`",
		"- open decision required: `"+fmt.Sprintf("%t", executorAction.OpenDecisionRequired)+"`",
		"- resume command: `"+executorAction.ResumeCommand+"`",
		"- handoff command: `"+executorAction.HandoffCommand+"`",
		"- commander state: `"+executorAction.MissionCommanderAction.State+"`",
		"- commander prompt: "+executorAction.MissionCommanderAction.Prompt,
		"- commander primary command: `"+executorAction.MissionCommanderAction.PrimaryCommand+"`",
	)
	lines = appendResumeList(lines, "commander follow-up commands", executorAction.MissionCommanderAction.FollowUpCommands)
	lines = appendResumeList(lines, "commander boundary", executorAction.MissionCommanderAction.Boundary)
	lines = appendResumeReviewerWritebacks(lines, reviewerWritebacks)
	lines = appendReviewerDispatchIntakeHandoff(lines, reviewerDispatchIntakeHandoffs)
	lines = appendReviewerPacketRetirementHandoff(lines, reviewerPacketRetirementHandoffs)
	lines = appendMissionCommanderActionQueue(lines, missionCommanderActionQueue)
	lines = appendResumeMissionCommanderNextActions(lines, missionCommanderNextActions)
	lines = appendResumeList(lines, "blocker reasons", executorAction.BlockerReasons)
	lines = appendResumeList(lines, "executor next actions", executorAction.NextAgentActions)
	lines = appendResumeList(lines, "executor escalations", executorAction.Escalations)
	lines = append(lines,
		"",
		"## Heavy-action gate decisions",
		"",
	)
	lines = appendResumeList(lines, "pending-gate", pendingGateLines)
	lines = appendResumeList(lines, "authorized-gate", authorizedGateLines)
	lines = appendResumeExecutionEvidenceReview(lines, executionEvidenceReview)
	lines = append(lines,
		"",
		"## 最近 inbox",
		"",
	)
	for _, msg := range lastObjects(inbox, 8) {
		lines = append(lines, "- "+firstObjectText(msg, "summary", "kind", "eventId"))
	}
	if len(inbox) == 0 {
		lines = append(lines, "- 无。")
	}
	lines = append(lines, "", "## Open interventions", "")
	for _, item := range openInterventions {
		lines = append(lines, "- "+firstText(item.Subject, item.Summary, item.EventID)+" | eventId=`"+item.EventID+"` | status=`"+item.Status+"`")
	}
	if len(openInterventions) == 0 {
		lines = append(lines, "- 无。")
	}
	openTasks := []map[string]any{}
	for _, task := range tasks {
		status := strings.ToLower(strings.TrimSpace(firstObjectText(task, "status")))
		if status != "closed" && status != "resolved" {
			openTasks = append(openTasks, task)
		}
	}
	lines = append(lines, "", "## 未关闭任务", "")
	for _, task := range lastObjects(openTasks, 12) {
		lines = append(lines, "- "+firstObjectText(task, "summary", "taskId", "eventId"))
	}
	if len(openTasks) == 0 {
		lines = append(lines, "- 无。")
	}
	resume := strings.Join(append(lines, ""), "\r\n")
	resumePath := filepath.Join(laneRoot, "prompts", "RESUME.md")
	checkpointPath := filepath.Join(laneRoot, "checkpoints", "latest.json")
	checkpointUpdatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	if len(updatedAt) > 0 && strings.TrimSpace(updatedAt[0]) != "" {
		checkpointUpdatedAt = strings.TrimSpace(updatedAt[0])
	}
	checkpoint := laneCheckpoint{
		SchemaVersion:                    1,
		Lane:                             lane.ID,
		Status:                           lane.Status,
		Workspace:                        lane.Workspace,
		CurrentExecutor:                  lane.CurrentExecutor,
		ExecutorGeneration:               lane.ExecutorGeneration,
		LastTakeoverAt:                   lane.LastTakeoverAt,
		LastTakeoverBy:                   lane.LastTakeoverBy,
		LastTakeoverReason:               lane.LastTakeoverReason,
		LastReconciledIntervention:       lane.LastReconciledIntervention,
		LastReconcileAt:                  lane.LastReconcileAt,
		AutonomyProfile:                  autonomySummary,
		LaneTakeoverPackage:              laneTakeoverPackage,
		MissionBrief:                     brief,
		ExecutorAction:                   executorAction,
		PendingGates:                     pendingGateLines,
		AuthorizedGates:                  authorizedGateLines,
		ExecutionEvidenceReview:          executionEvidenceReview,
		ExecutionEvidenceReviewSummary:   ExecutionEvidenceReviewSummaryFor(executionEvidenceReview, missionCommanderActionQueue),
		ReviewerWritebacks:               reviewerWritebacks,
		ReviewerWritebackSummary:         ReviewerWritebackSummaryFor(reviewerWritebacks),
		ReviewerDispatchIntakeHandoffs:   reviewerDispatchIntakeHandoffs,
		ReviewerDispatchIntakeSummary:    ReviewerDispatchIntakeSummaryFor(reviewerDispatchIntakeHandoffs),
		ReviewerPacketRetirementHandoffs: reviewerPacketRetirementHandoffs,
		ReviewerPacketRetirementSummary:  ReviewerPacketRetirementSummaryFor(reviewerPacketRetirementHandoffs),
		AuthorizedGateAdapterHandoffs:    authorizedGateAdapterHandoffs,
		MissionCommanderNextActions:      missionCommanderNextActions,
		MissionCommanderActionQueue:      missionCommanderActionQueue,
		OpenInterventions:                openInterventions,
		Inbox:                            len(inbox),
		Tasks:                            len(tasks),
		UpdatedAt:                        checkpointUpdatedAt,
		Resume:                           relativePath(caseRoot, resumePath),
	}
	checkpointBytes, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return laneResumePublication{}, err
	}
	return laneResumePublication{
		ResumePath:      resumePath,
		ResumeBytes:     []byte(resume),
		CheckpointPath:  checkpointPath,
		CheckpointBytes: append(checkpointBytes, '\n'),
	}, nil
}

func writeLaneResumePublication(publication laneResumePublication) error {
	if err := os.MkdirAll(filepath.Dir(publication.ResumePath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(publication.ResumePath, publication.ResumeBytes, 0o644); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(publication.CheckpointPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(publication.CheckpointPath, publication.CheckpointBytes, 0o644)
}

func writeLaneResume(caseRoot string, m *manifest.Manifest, lane Lane) (string, string, error) {
	publication, err := buildLaneResumePublication(caseRoot, m, lane)
	if err != nil {
		return "", "", err
	}
	if err := writeLaneResumePublication(publication); err != nil {
		return "", "", err
	}
	return publication.ResumePath, publication.CheckpointPath, nil
}

func laneExecutorActionFor(lane Lane, laneFacts mission.Facts, brief mission.Brief) laneExecutorAction {
	action := mission.LaneExecutorAction(mission.Lane{ID: lane.ID, Label: workstreamLabel(lane), Status: lane.Status}, laneFacts, brief)
	return bindLaneContinueCommands(action, lane)
}

func withReviewerDispatchBlocker(action laneExecutorAction, handoffs []ReviewerDispatchIntakeHandoff) laneExecutorAction {
	if len(handoffs) == 0 {
		return action
	}
	action.Blocked = true
	action.Ready = false
	action.BlockerReasons = mission.UniqueStrings(append(action.BlockerReasons, "active reviewer dispatch/intake work must complete before lane continuation"))
	action.NextAgentActions = mission.UniqueStrings(append([]string{ReviewerDispatchIntakeSummaryFor(handoffs).NextAction}, action.NextAgentActions...))
	action.MissionCommanderAction = mission.MissionCommanderAction{
		State:          "needs-reviewer-dispatch-intake",
		Prompt:         "Complete the current reviewer packet action before lane continuation.",
		PrimaryCommand: ReviewerDispatchIntakeSummaryFor(handoffs).NextAction,
		Boundary: []string{
			"do not continue while reviewer dispatch/intake work remains open",
			"reviewer intake requires explicit WhatIf before Apply and does not write authority/confirmed or execute heavy tools",
		},
	}
	return action
}

func defaultStartLaneType(m *manifest.Manifest) string {
	for _, lane := range m.LaneTypes {
		if !lane.Authority {
			return lane.ID
		}
	}
	return m.WorkstreamDefaults["defaultAuthorityLane"]
}

func laneID(laneType, name string) string {
	raw := laneType
	if strings.TrimSpace(name) != "" {
		if strings.Contains(laneType, "feature") {
			raw = "feature-" + name
		} else {
			raw = laneType + "-" + name
		}
	}
	safe := regexp.MustCompile(`[^a-z0-9._-]+`).ReplaceAllString(strings.ToLower(strings.TrimSpace(raw)), "-")
	return strings.Trim(safe, "-_.")
}

func workstreamLabel(lane Lane) string {
	if lane.Authority {
		return "main"
	}
	if name, ok := strings.CutPrefix(lane.ID, "feature-"); ok {
		return name
	}
	return lane.ID
}

func validateLaneIDSegment(id string) error {
	if !safeLaneIDSegment.MatchString(strings.TrimSpace(id)) {
		return fmt.Errorf("invalid lane id path segment: %s", id)
	}
	return nil
}

func laneRootPath(caseRoot string, lane Lane) (string, error) {
	if err := validateLaneIDSegment(lane.ID); err != nil {
		return "", err
	}
	rootFromID, err := refsf.SafeJoin(caseRoot, relJoin(".rekit", "lanes", lane.ID))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(lane.LaneRoot) == "" {
		return rootFromID, nil
	}
	rootFromLane, err := refsf.SafeJoin(caseRoot, lane.LaneRoot)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(filepath.Clean(rootFromLane), filepath.Clean(rootFromID)) {
		return "", fmt.Errorf("laneRoot mismatch for %s: got %s, want %s", lane.ID, lane.LaneRoot, relativePath(caseRoot, rootFromID))
	}
	return rootFromID, nil
}

func ensureEmptyFile(path string) (string, error) {
	if refsf.Exists(path) {
		return "unchanged", nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		return "", err
	}
	return "create-empty-file", nil
}

func readLane(path string) (Lane, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Lane{}, err
	}
	var lane Lane
	if err := json.Unmarshal(b, &lane); err != nil {
		return Lane{}, fmt.Errorf("invalid lane json %s: %w", path, err)
	}
	return lane, nil
}

func lastObjects(items []map[string]any, limit int) []map[string]any {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[len(items)-limit:]
}

func laneExecutionEvidenceReview(lane Lane, facts mission.LedgerFacts) []ExecutionEvidenceReviewItem {
	label := workstreamLabel(lane)
	return ExecutionEvidenceReviewItemsWithLedgerFacts(facts, lane.ID, func(string) string { return label })
}

func ExecutionEvidenceReviewItems(observations []map[string]any, laneID string, labelFor func(string) string) []ExecutionEvidenceReviewItem {
	return mission.ExecutionEvidenceReviewItems(observations, laneID, labelFor, maxHandoffRows)
}

func ExecutionEvidenceReviewItemsWithLedgerFacts(facts mission.LedgerFacts, laneID string, labelFor func(string) string) []ExecutionEvidenceReviewItem {
	return mission.ExecutionEvidenceReviewItemsWithLedgerFacts(facts, laneID, labelFor, maxHandoffRows)
}

func ExecutionEvidenceReviewAcknowledgedIDs(facts mission.LedgerFacts) map[string]bool {
	return mission.ExecutionEvidenceReviewAcknowledgedIDs(facts)
}

func ExecutionEvidenceReviewSummaryFor(items []ExecutionEvidenceReviewItem, queue mission.MissionCommanderActionQueue) ExecutionEvidenceReviewSummary {
	return mission.ExecutionEvidenceReviewSummaryFor(items, queue)
}

func ExecutionEvidenceReviewNeedsMainReview(items []ExecutionEvidenceReviewItem) bool {
	return mission.ExecutionEvidenceReviewNeedsMainReview(items)
}

func ExecutionEvidenceReviewNextSteps(items []ExecutionEvidenceReviewItem, includeContinueFollowUp bool) []string {
	next := []string{}
	includeContinue := includeContinueFollowUp && !ExecutionEvidenceReviewNeedsMainReview(items)
	for _, item := range items {
		steps := item.ReviewRunbookSteps
		if len(steps) == 0 {
			steps = mission.ExecutionEvidenceReviewRunbookSteps(item, includeContinue)
		}
		for _, step := range steps {
			if strings.Contains(step, "/rekit continue") && !includeContinue {
				continue
			}
			next = append(next, step)
		}
	}
	return mission.UniqueStrings(next)
}

func appendMissionCommanderActionQueue(lines []string, queue mission.MissionCommanderActionQueue) []string {
	lines = append(lines, "", "## Mission Commander action queue", "")
	lines = append(lines, "- summary: "+queue.Summary)
	lines = append(lines, fmt.Sprintf("- counts: total=%d unblocked=%d blocked=%d requiresReview=%d followUp=%d", queue.Counts.Total, queue.Counts.Unblocked, queue.Counts.Blocked, queue.Counts.RequiresReview, queue.Counts.FollowUp))
	if queue.CurrentAction == nil {
		return append(lines, "- current: none")
	}
	item := *queue.CurrentAction
	lines = append(lines, fmt.Sprintf("- current: state=%s source=%s blocked=%t requiresReview=%t command=`%s`", item.State, item.Source, item.Blocked, item.RequiresReview, item.Command))
	for _, line := range MissionCommanderActionRunLoopMarkdownLines(queue) {
		lines = append(lines, "- "+line)
	}
	return lines
}

func appendResumeMissionCommanderNextActions(lines []string, items []mission.MissionCommanderNextActionItem) []string {
	lines = append(lines, "", "## Mission Commander next actions", "")
	if len(items) == 0 {
		return append(lines, "- none")
	}
	shown := items
	if maxHandoffRows > 0 && len(shown) > maxHandoffRows {
		shown = shown[len(shown)-maxHandoffRows:]
	}
	for _, item := range shown {
		lines = append(lines, fmt.Sprintf("- state=%s source=%s blocked=%t requiresReview=%t command=`%s`", item.State, item.Source, item.Blocked, item.RequiresReview, item.Command))
		lines = appendResumeList(lines, "reasons", item.Reasons)
		lines = appendResumeList(lines, "boundary", item.Boundary)
	}
	return lines
}

func appendResumeExecutionEvidenceReview(lines []string, items []ExecutionEvidenceReviewItem) []string {
	if len(items) == 0 {
		return append(lines, "- execution evidence review: none")
	}
	summary := ExecutionEvidenceReviewSummaryFor(items, mission.MissionCommanderActionQueueFor(mission.MissionCommanderNextActions(nil, items, ExecutionEvidenceReviewNeedsMainReview(items))))
	lines = append(lines, "- execution evidence review:")
	lines = appendResumeExecutionEvidenceReviewSummary(lines, summary)
	for _, item := range items {
		lines = append(lines, "  - "+firstText(item.Subject, item.Summary, item.EventID)+" | status="+item.Status+" | gateEventId="+item.GateEventID+" | action="+firstText(item.Action, "none"))
		if refs := strings.Join(item.BoundaryHits, ","); refs != "" {
			lines = append(lines, "    - boundaryHits: "+refs)
		}
		if strings.TrimSpace(item.Escalation) != "" {
			lines = append(lines, "    - escalation: "+item.Escalation)
		}
		lines = appendResumeExecutionEvidenceReportDetail(lines, item)
		if refs := strings.Join(item.OutputRefs, ","); refs != "" {
			lines = append(lines, "    - outputRefs: "+refs)
		}
		if refs := strings.Join(item.EvidenceRefs, ","); refs != "" {
			lines = append(lines, "    - evidenceRefs: "+refs)
		}
		lines = append(lines, "    - review command: `"+item.ReviewCommand+"`")
		lines = append(lines, "    - handoff command: `"+item.HandoffCommand+"`")
		for idx, step := range item.ReviewRunbookSteps {
			lines = append(lines, fmt.Sprintf("    - review runbook: step=%d text=%s", idx+1, step))
		}
		lines = append(lines, "    - commander state: "+item.MissionCommanderAction.State)
		lines = append(lines, "    - commander primary: `"+item.MissionCommanderAction.PrimaryCommand+"`")
		lines = appendResumeExecutionEvidenceFollowThrough(lines, item.FollowThrough)
		for _, followUp := range mission.LimitStrings(item.MissionCommanderAction.FollowUpCommands, maxHandoffRows) {
			lines = append(lines, "    - commander follow-up: "+followUp)
		}
		for _, boundary := range mission.LimitStrings(item.Boundary, maxHandoffRows) {
			lines = append(lines, "    - boundary: "+boundary)
		}
	}
	return lines
}

func appendResumeExecutionEvidenceReviewSummary(lines []string, summary ExecutionEvidenceReviewSummary) []string {
	if summary.Total == 0 {
		return lines
	}
	lines = append(lines, fmt.Sprintf("  - summary: total=%d readyForReview=%d mainEscalations=%d duplicates=%d outputRefs=%d evidenceRefs=%d boundaryHits=%d latestEventId=%s gateEventId=%s status=%s action=%s", summary.Total, summary.ReadyForReviewCount, summary.MainEscalationCount, summary.DuplicateCount, summary.OutputRefCount, summary.EvidenceRefCount, summary.BoundaryHitCount, summary.LatestEventID, summary.LatestGateEventID, summary.LatestStatus, firstText(summary.LatestAction, "none")))
	if strings.TrimSpace(summary.CurrentAction) != "" {
		lines = append(lines, "    - summary current action: `"+summary.CurrentAction+"`")
	}
	if strings.TrimSpace(summary.ActionQueueSummary) != "" {
		lines = append(lines, "    - summary action queue: "+summary.ActionQueueSummary)
	}
	if strings.TrimSpace(summary.LatestReviewCommand) != "" || strings.TrimSpace(summary.LatestHandoffCommand) != "" {
		lines = append(lines, "    - summary handoff: review=`"+summary.LatestReviewCommand+"` handoff=`"+summary.LatestHandoffCommand+"`")
	}
	if strings.TrimSpace(summary.LatestCommanderState) != "" || strings.TrimSpace(summary.LatestCommanderPrimary) != "" {
		lines = append(lines, "    - summary commander: state="+summary.LatestCommanderState+" primary=`"+summary.LatestCommanderPrimary+"`")
	}
	if strings.TrimSpace(summary.LatestExecutionReportPath) != "" || strings.TrimSpace(summary.LatestExecutionReportSHA256) != "" || strings.TrimSpace(summary.LatestAdapterID) != "" || strings.TrimSpace(summary.LatestAdapterStatus) != "" {
		lines = append(lines, "    - summary report: path="+firstText(summary.LatestExecutionReportPath, "none")+" sha256="+firstText(summary.LatestExecutionReportSHA256, "none")+" adapterId="+firstText(summary.LatestAdapterID, "none")+" adapterStatus="+firstText(summary.LatestAdapterStatus, "none"))
	}
	lines = appendResumeExecutionEvidenceAdapterContext(lines, summary.LatestAdapterContext)
	for _, hit := range mission.LimitStrings(summary.LatestBoundaryHits, maxHandoffRows) {
		lines = append(lines, "    - summary latest boundary hit: "+hit)
	}
	if strings.TrimSpace(summary.LatestEscalation) != "" {
		lines = append(lines, "    - summary latest escalation: "+summary.LatestEscalation)
	}
	if strings.TrimSpace(summary.FollowThroughState) != "" || summary.OutcomeCount > 0 {
		lines = append(lines, fmt.Sprintf("    - summary follow-through: state=%s outcomes=%d", summary.FollowThroughState, summary.OutcomeCount))
	}
	for _, boundary := range mission.LimitStrings(summary.Boundary, maxHandoffRows) {
		lines = append(lines, "    - summary boundary: "+boundary)
	}
	return lines
}

func appendResumeExecutionEvidenceReportDetail(lines []string, item ExecutionEvidenceReviewItem) []string {
	if strings.TrimSpace(item.ExecutionReportPath) != "" || strings.TrimSpace(item.ExecutionReportSHA256) != "" {
		lines = append(lines, "    - execution report: "+firstText(item.ExecutionReportPath, "none")+" sha256="+firstText(item.ExecutionReportSHA256, "none"))
	}
	if item.ActualBudget != nil {
		lines = append(lines, fmt.Sprintf("    - actual budget: runtimeSeconds=%d diskMB=%d requests=%d", item.ActualBudget.RuntimeSeconds, item.ActualBudget.DiskMB, item.ActualBudget.Requests))
	}
	if strings.TrimSpace(item.AdapterID) != "" || strings.TrimSpace(item.AdapterStatus) != "" {
		lines = append(lines, fmt.Sprintf("    - adapter report: adapterId=%s status=%s", item.AdapterID, item.AdapterStatus))
	}
	if strings.TrimSpace(item.AdapterExecutionDispatchPath) != "" || strings.TrimSpace(item.AdapterExecutionDispatchSHA256) != "" {
		lines = append(lines, "    - dispatch: id="+firstText(item.AdapterExecutionDispatchID, "none")+" path="+firstText(item.AdapterExecutionDispatchPath, "none")+" sha256="+firstText(item.AdapterExecutionDispatchSHA256, "none"))
	}
	if strings.TrimSpace(item.AdapterExecutionReceiptPath) != "" || strings.TrimSpace(item.AdapterExecutionReceiptSHA256) != "" {
		lines = append(lines, "    - receipt: path="+firstText(item.AdapterExecutionReceiptPath, "none")+" sha256="+firstText(item.AdapterExecutionReceiptSHA256, "none"))
	}
	if strings.TrimSpace(item.CurrentExecutor) != "" || item.ExecutorGeneration > 0 || strings.TrimSpace(item.AdapterHarness) != "" || strings.TrimSpace(item.AdapterSession) != "" {
		lines = append(lines, fmt.Sprintf("    - execution owner: executor=%s generation=%d harness=%s session=%s", item.CurrentExecutor, item.ExecutorGeneration, item.AdapterHarness, item.AdapterSession))
	}
	if strings.TrimSpace(item.ToolingCatalogSHA256) != "" || item.AdapterExecutionArtifactCount > 0 {
		lines = append(lines, fmt.Sprintf("    - tooling provenance: catalogSha256=%s artifacts=%d", item.ToolingCatalogSHA256, item.AdapterExecutionArtifactCount))
	}
	return appendResumeExecutionEvidenceAdapterContext(lines, item.AdapterContext)
}

func appendResumeExecutionEvidenceAdapterContext(lines []string, context *mission.ExecutionEvidenceAdapterContext) []string {
	if context == nil {
		return lines
	}
	lines = append(lines, fmt.Sprintf("    - adapter context: id=%s status=%s entry=%s gateActions=%s recordOnlyAfterGate=%t toolingCatalogPath=%s", context.ID, context.Status, context.Entry, strings.Join(context.GateActions, ","), context.RecordOnlyAfterGate, context.ToolingCatalogPath))
	if strings.TrimSpace(context.Purpose) != "" {
		lines = append(lines, "    - adapter context purpose: "+context.Purpose)
	}
	if len(context.SideEffects) > 0 {
		lines = append(lines, "    - adapter context side effects: "+strings.Join(context.SideEffects, ","))
	}
	for _, guidance := range mission.LimitStrings(context.ReportGuidance, maxHandoffRows) {
		lines = append(lines, "    - adapter context report guidance: "+guidance)
	}
	for _, guidance := range mission.LimitStrings(context.EvidenceGuidance, maxHandoffRows) {
		lines = append(lines, "    - adapter context evidence guidance: "+guidance)
	}
	if len(context.StopConditionHints) > 0 {
		lines = append(lines, "    - adapter context stop conditions: "+strings.Join(context.StopConditionHints, ","))
	}
	return lines
}

func appendResumeExecutionEvidenceFollowThrough(lines []string, follow mission.ExecutionEvidenceFollowThrough) []string {
	if strings.TrimSpace(follow.State) == "" && len(follow.Outcomes) == 0 {
		return lines
	}
	lines = append(lines, "    - follow-through: state="+follow.State+" gateEventId="+follow.GateEventID+" outcomes="+fmt.Sprintf("%d", len(follow.Outcomes)))
	for _, outcome := range limitExecutionEvidenceOutcomes(follow.Outcomes, maxHandoffRows) {
		lines = append(lines, "      - outcome: name="+outcome.Name+" state="+outcome.State+" command=`"+outcome.Command+"` expected="+outcome.Expected)
		if strings.TrimSpace(outcome.When) != "" {
			lines = append(lines, "        - when: "+outcome.When)
		}
		for _, action := range mission.LimitStrings(outcome.Actions, maxHandoffRows) {
			lines = append(lines, "        - action: "+action)
		}
		for _, command := range mission.LimitStrings(outcome.VerificationCommands, maxHandoffRows) {
			lines = append(lines, "        - verification: "+command)
		}
		for _, evidence := range mission.LimitStrings(outcome.Evidence, maxHandoffRows) {
			lines = append(lines, "        - evidence: "+evidence)
		}
	}
	if strings.TrimSpace(follow.ActionQueue.Summary) != "" {
		lines = append(lines, "      - queue: "+follow.ActionQueue.Summary)
	}
	return lines
}

func limitExecutionEvidenceOutcomes(items []mission.ExecutionEvidenceOutcome, limit int) []mission.ExecutionEvidenceOutcome {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func appendResumeList(lines []string, label string, items []string) []string {
	if len(items) == 0 {
		return append(lines, "- "+label+": none")
	}
	lines = append(lines, "- "+label+":")
	for _, item := range mission.LimitStrings(items, maxHandoffRows) {
		lines = append(lines, "  - "+item)
	}
	return lines
}

func firstObjectText(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			text := objectText(value)
			if text != "" {
				return text
			}
		}
	}
	return ""
}

func objectText(value any) string {
	if value == nil {
		return ""
	}
	switch t := value.(type) {
	case string:
		return strings.TrimSpace(t)
	case []any:
		parts := []string{}
		for _, item := range t {
			text := objectText(item)
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, ",")
	default:
		text := strings.TrimSpace(fmt.Sprint(t))
		if text == "<nil>" {
			return ""
		}
		return text
	}
}

func readAutomationMode(caseRoot string) string {
	path, err := refsf.SafeJoin(caseRoot, ".rekit/policy.yml")
	if err != nil {
		return "assisted-autopilot"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "assisted-autopilot"
	}
	for line := range strings.SplitSeq(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) == 2 && parts[0] == "automationMode" {
			mode := strings.TrimSpace(parts[1])
			if mode != "" {
				return mode
			}
		}
	}
	return "assisted-autopilot"
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func eventID(laneID, kind, createdAt string) string {
	sum := sha256.Sum256([]byte(laneID + "|" + kind + "|" + createdAt))
	return "evt-" + hex.EncodeToString(sum[:])[:16]
}

func relativePath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func relJoin(parts ...string) string {
	clean := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			clean = append(clean, filepath.FromSlash(part))
		}
	}
	return filepath.ToSlash(filepath.Join(clean...))
}
