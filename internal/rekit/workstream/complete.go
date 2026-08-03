package workstream

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanecompletion"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

const (
	completionIntentFile = "completion.intent.json"
	completionCommitFile = "completion.json"
)

var completionAfterIntentHook func() error

type CompleteOptions struct {
	Selector              string
	Actor                 string
	Reason                string
	EvidenceRefs          string
	ExpectedPreviewSHA256 string
}

type CompletionBlocker struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

type CompletionEvidence struct {
	Ref    string `json:"ref"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type CompletionReceipt struct {
	SchemaVersion      int                  `json:"schemaVersion"`
	Kind               string               `json:"kind"`
	State              string               `json:"state"`
	Sequence           int                  `json:"sequence,omitempty"`
	PreviousReceiptSHA string               `json:"previousReceiptSha256,omitempty"`
	Lane               string               `json:"lane"`
	Label              string               `json:"label"`
	Authority          bool                 `json:"authority"`
	PreviousStatus     string               `json:"previousStatus"`
	Actor              string               `json:"actor"`
	Reason             string               `json:"reason"`
	EvidenceRefs       []string             `json:"evidenceRefs"`
	Evidence           []CompletionEvidence `json:"evidence"`
	CurrentExecutor    string               `json:"currentExecutor,omitempty"`
	ExecutorGeneration int                  `json:"executorGeneration,omitempty"`
	CompletedAt        string               `json:"completedAt"`
	EventID            string               `json:"eventId"`
	PreviewSHA256      string               `json:"previewSha256"`
	IntentSHA256       string               `json:"intentSha256"`
	LaneSHA256         string               `json:"laneSha256"`
	BoardLaneSHA256    string               `json:"boardLaneSha256"`
	ResumeSHA256       string               `json:"resumeSha256"`
	CheckpointSHA256   string               `json:"checkpointSha256"`
	NoAuthority        bool                 `json:"noAuthority"`
	NoConfirmed        bool                 `json:"noConfirmed"`
	NoHeavyTool        bool                 `json:"noHeavyTool"`
}

type completionIntent struct {
	SchemaVersion      int                  `json:"schemaVersion"`
	Kind               string               `json:"kind"`
	Sequence           int                  `json:"sequence,omitempty"`
	PreviousReceiptSHA string               `json:"previousReceiptSha256,omitempty"`
	Lane               string               `json:"lane"`
	Label              string               `json:"label"`
	Authority          bool                 `json:"authority"`
	PreviousStatus     string               `json:"previousStatus"`
	Actor              string               `json:"actor"`
	Reason             string               `json:"reason"`
	EvidenceRefs       []string             `json:"evidenceRefs"`
	Evidence           []CompletionEvidence `json:"evidence"`
	CurrentExecutor    string               `json:"currentExecutor,omitempty"`
	ExecutorGeneration int                  `json:"executorGeneration,omitempty"`
	CreatedAt          string               `json:"createdAt"`
	EventID            string               `json:"eventId"`
	PreviewSHA256      string               `json:"previewSha256"`
	NoAuthority        bool                 `json:"noAuthority"`
	NoConfirmed        bool                 `json:"noConfirmed"`
	NoHeavyTool        bool                 `json:"noHeavyTool"`
}

type CompleteResult struct {
	SchemaVersion               int                                      `json:"schemaVersion"`
	Command                     string                                   `json:"command"`
	CaseRoot                    string                                   `json:"caseRoot"`
	RepoRoot                    string                                   `json:"repoRoot"`
	Pack                        string                                   `json:"pack"`
	IsMutation                  bool                                     `json:"isMutation"`
	Applied                     bool                                     `json:"applied"`
	RequiresConfirmation        bool                                     `json:"requiresConfirmation"`
	Selector                    string                                   `json:"selector"`
	Lane                        Lane                                     `json:"lane"`
	Actor                       string                                   `json:"actor"`
	Reason                      string                                   `json:"reason"`
	EvidenceRefs                []string                                 `json:"evidenceRefs"`
	Evidence                    []CompletionEvidence                     `json:"evidence"`
	Blocked                     bool                                     `json:"blocked"`
	Blockers                    []CompletionBlocker                      `json:"blockers,omitempty"`
	CompletionPlanSHA256        string                                   `json:"completionPlanSha256,omitempty"`
	ApplyCommand                string                                   `json:"applyCommand,omitempty"`
	CompletionReceipt           *CompletionReceipt                       `json:"completionReceipt,omitempty"`
	MissionBrief                mission.Brief                            `json:"missionBrief"`
	MissionCommanderNextActions []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions,omitempty"`
	MissionCommanderActionQueue mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
	WouldWrites                 []StartWrite                             `json:"wouldWrites,omitempty"`
	Writes                      []StartWrite                             `json:"writes,omitempty"`
	BlockedActions              []string                                 `json:"blockedActions"`
	NextSteps                   []string                                 `json:"nextSteps"`
}

type MissionCompletionHandoff struct {
	Ready                 bool                `json:"ready"`
	State                 string              `json:"state"`
	OperationallyComplete bool                `json:"operationallyComplete"`
	OpenLaneCount         int                 `json:"openLaneCount"`
	CompletedLaneCount    int                 `json:"completedLaneCount"`
	Receipts              []CompletionReceipt `json:"receipts,omitempty"`
	Summary               string              `json:"summary"`
	Boundary              []string            `json:"boundary"`
}

type completeContext struct {
	inst         instance.Instance
	manifest     *manifest.Manifest
	board        board
	lane         Lane
	selector     string
	actor        string
	reason       string
	evidenceRefs []string
	evidence     []CompletionEvidence
	facts        mission.LedgerFacts
	blockers     []CompletionBlocker
	lifecycle    lanecompletion.Inspection
	sequence     int
	previousSHA  string
	intent       *completionIntent
}

func CompletePreview(repoRoot, caseRoot, pack string, opt CompleteOptions) (CompleteResult, error) {
	ctx, err := newCompleteContext(repoRoot, caseRoot, pack, opt, false)
	if err != nil {
		return CompleteResult{}, err
	}
	result := ctx.result(false, false)
	result.WouldWrites = ctx.plannedWrites()
	if result.Blocked {
		result.WouldWrites = nil
		result.NextSteps = []string{"resolve every typed completion blocker, refresh status, then preview complete again"}
		return result, nil
	}
	hash, err := completePreviewSHA256(result)
	if err != nil {
		return CompleteResult{}, err
	}
	result.CompletionPlanSHA256 = hash
	result.ApplyCommand = completeApplyCommand(ctx, hash)
	result.NextSteps = []string{"review the completion evidence, blockers, and exact write set, then run applyCommand", "after apply, refresh status to select the next open lane or read the mission-complete handoff"}
	return result, nil
}

func CompleteApply(repoRoot, caseRoot, pack string, opt CompleteOptions) (result CompleteResult, err error) {
	mutationStarted := false
	defer func() {
		if err != nil && !mutationStarted {
			err = MarkZeroProgress(err)
		}
	}()
	ctx, err := newCompleteContext(repoRoot, caseRoot, pack, opt, true)
	if err != nil {
		return CompleteResult{}, err
	}
	lease, err := acquireProjectMutationLock(ctx.inst.CaseRoot)
	if err != nil {
		return CompleteResult{}, err
	}
	defer func() {
		if unlockErr := lease.Unlock(); unlockErr != nil {
			err = errors.Join(err, unlockErr)
			result = CompleteResult{}
		}
	}()
	ctx, err = newCompleteContext(repoRoot, caseRoot, pack, opt, true)
	if err != nil {
		return CompleteResult{}, err
	}
	if err := lease.Validate(); err != nil {
		return CompleteResult{}, err
	}
	expected := strings.ToLower(strings.TrimSpace(opt.ExpectedPreviewSHA256))
	if expected == "" {
		return CompleteResult{}, fmt.Errorf("complete apply requires -ExpectedCompletePlanSha256 from the reviewed preview")
	}
	if ctx.intent == nil {
		if len(ctx.blockers) > 0 {
			return CompleteResult{}, fmt.Errorf("complete is blocked: %s", completionBlockerSummary(ctx.blockers))
		}
		previewOpt := opt
		previewOpt.ExpectedPreviewSHA256 = ""
		preview, err := CompletePreview(repoRoot, caseRoot, pack, previewOpt)
		if err != nil {
			return CompleteResult{}, err
		}
		if preview.Blocked || !strings.EqualFold(expected, preview.CompletionPlanSHA256) {
			return CompleteResult{}, fmt.Errorf("complete preview sha256 mismatch: got %s want %s", expected, preview.CompletionPlanSHA256)
		}
		intent := ctx.newIntent(expected, isoNow())
		mutationStarted = true
		if err := writeCompletionExclusive(ctx.inst.CaseRoot, ctx.intentPath(), intent); err != nil {
			return CompleteResult{}, err
		}
		ctx.intent = &intent
		if completionAfterIntentHook != nil {
			if err := completionAfterIntentHook(); err != nil {
				return CompleteResult{}, err
			}
		}
	} else {
		if len(ctx.blockers) > 0 {
			return CompleteResult{}, fmt.Errorf("complete recovery is blocked: %s", completionBlockerSummary(ctx.blockers))
		}
		if err := ctx.validateIntent(expected); err != nil {
			return CompleteResult{}, err
		}
		mutationStarted = true
	}
	if err := lease.Validate(); err != nil {
		return CompleteResult{}, err
	}
	writes, receipt, err := ctx.publishCompletion()
	if err != nil {
		return CompleteResult{}, err
	}
	ctx.lane.Status = "closed"
	ctx.blockers = nil
	result = ctx.result(true, true)
	result.Lane = ctx.lane
	result.Writes = writes
	result.CompletionPlanSHA256 = receipt.PreviewSHA256
	result.CompletionReceipt = &receipt
	result.NextSteps = []string{"run /rekit status to select the next open lane or read the typed mission-complete handoff", "completion records operational closure only; no authority/confirmed conclusion was written or inferred"}
	return result, nil
}

func newCompleteContext(repoRoot, caseRoot, pack string, opt CompleteOptions, allowPending bool) (completeContext, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return completeContext{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return completeContext{}, err
	}
	b, err := readBoard(inst.CaseRoot)
	if os.IsNotExist(err) {
		return completeContext{}, fmt.Errorf("complete requires existing .rekit/board.json; run start -Apply first")
	}
	if err != nil {
		return completeContext{}, err
	}
	if strings.TrimSpace(b.DefaultAuthorityLane) == "" {
		b.DefaultAuthorityLane = m.WorkstreamDefaults["defaultAuthorityLane"]
	}
	selector := strings.TrimSpace(opt.Selector)
	if selector == "" {
		return completeContext{}, fmt.Errorf("complete requires a lane selector")
	}
	lane, err := resolveHandoffLane(inst.CaseRoot, b, selector)
	if err != nil {
		return completeContext{}, err
	}
	actor := strings.TrimSpace(opt.Actor)
	reason := strings.TrimSpace(opt.Reason)
	if actor == "" || reason == "" {
		return completeContext{}, fmt.Errorf("complete requires non-empty -Actor and -Reason")
	}
	refs, evidence, err := validateCompletionEvidenceRefs(inst.CaseRoot, opt.EvidenceRefs)
	if err != nil {
		return completeContext{}, err
	}
	facts, err := mission.ReadStrictLedgerFacts(inst.CaseRoot)
	if err != nil {
		return completeContext{}, err
	}
	ctx := completeContext{inst: inst, manifest: m, board: b, lane: lane, selector: selector, actor: actor, reason: reason, evidenceRefs: refs, evidence: evidence, facts: facts}
	operations, operationErr := lanecompletion.InspectOperations(inst.CaseRoot)
	if operationErr != nil {
		return completeContext{}, operationErr
	}
	if operations.Pending {
		return completeContext{}, fmt.Errorf("complete refuses case while reopen operation publication is incomplete; recover the exact reopen Apply")
	}
	lifecycle, lifecycleErr := lanecompletion.Inspect(inst.CaseRoot, lane.ID)
	if lifecycleErr != nil {
		return completeContext{}, lifecycleErr
	}
	ctx.lifecycle = lifecycle
	switch lifecycle.State {
	case lanecompletion.StateNone:
		ctx.sequence = 1
	case lanecompletion.StateReopened:
		ctx.sequence = lifecycle.HeadSequence + 1
		ctx.previousSHA = lifecycle.HeadReceiptSHA256
	case lanecompletion.StateComplete:
		return completeContext{}, fmt.Errorf("lane is already complete: %s", lane.ID)
	case lanecompletion.StatePending:
		if lifecycle.PendingKind != "complete" {
			return completeContext{}, fmt.Errorf("lane %s has pending reopen publication; recover with the original hash-bound reopen apply", lane.ID)
		}
		if !allowPending {
			return completeContext{}, fmt.Errorf("lane %s has pending completion publication; recover with the original hash-bound complete apply", lane.ID)
		}
		ctx.sequence = lifecycle.PendingSequence
		ctx.previousSHA = lifecycle.HeadReceiptSHA256
	}
	intent, intentErr := readCompletionIntent(ctx.intentPath())
	if intentErr == nil {
		ctx.intent = &intent
	} else if !os.IsNotExist(intentErr) {
		return completeContext{}, intentErr
	}
	status := strings.ToLower(strings.TrimSpace(lane.Status))
	if status == "closed" && (ctx.intent == nil || lifecycle.State != lanecompletion.StatePending) {
		return completeContext{}, fmt.Errorf("lane %s has uncommitted closed state; recover with the original hash-bound complete apply", lane.ID)
	}
	if status == "archived" || status == "paused" {
		return completeContext{}, fmt.Errorf("target lane is not open: %s", lane.ID)
	}
	ctx.blockers, err = ctx.completionBlockers()
	if err != nil {
		return completeContext{}, err
	}
	return ctx, nil
}

func (ctx completeContext) completionBlockers() ([]CompletionBlocker, error) {
	blockers := []CompletionBlocker{}
	add := func(kind, detail string) { blockers = append(blockers, CompletionBlocker{Kind: kind, Detail: detail}) }
	laneFacts := mission.LaneFacts(ctx.facts.Facts, ctx.lane.ID)
	for _, item := range mission.EffectiveOpenLaneInterventions(ctx.facts.Facts, ctx.lane.ID) {
		add("open-intervention", firstText(mission.Value(item, "eventId"), mission.Value(item, "subject")))
	}
	for _, item := range laneFacts.Requests {
		if mission.IsPendingGateRequest(item) {
			add("pending-gate", firstText(mission.Value(item, "eventId"), mission.Value(item, "subject")))
		}
	}
	for _, item := range continueOpenDecisionItems(laneFacts) {
		add("open-"+item.SourceKind, firstText(mission.Value(item.Event, "eventId"), mission.Value(item.Event, "subject")))
	}
	laneRoot, err := laneRootPath(ctx.inst.CaseRoot, ctx.lane)
	if err != nil {
		return nil, err
	}
	tasks, err := mission.ReadStrictJSONLineObjects(LaneTasksJSONLPath(laneRoot))
	if err != nil {
		return nil, err
	}
	for _, task := range tasks {
		status := strings.ToLower(strings.TrimSpace(mission.Value(task, "status")))
		if status != "closed" && status != "resolved" {
			add("open-task", firstText(mission.Value(task, "taskId"), mission.Value(task, "eventId"), mission.Value(task, "summary")))
		}
	}
	if items, err := ReviewerDispatchIntakeHandoffs(ctx.inst.CaseRoot, ctx.facts, ctx.lane.ID); err != nil {
		return nil, err
	} else if len(items) > 0 {
		add("reviewer-dispatch-intake", fmt.Sprintf("%d reviewer action(s) remain open", len(items)))
	}
	if items, err := ReviewerPacketRetirementHandoffs(ctx.inst.CaseRoot, ctx.lane.ID); err != nil {
		return nil, err
	} else if len(items) > 0 {
		add("reviewer-packet-retirement", fmt.Sprintf("%d invalid reviewer packet(s) require disposition", len(items)))
	}
	if items := laneExecutionEvidenceReview(ctx.lane, ctx.facts); len(items) > 0 {
		add("execution-evidence-review", fmt.Sprintf("%d execution evidence item(s) require acknowledgement", len(items)))
	}
	if items := AuthorizedGateAdapterHandoffsWithAcknowledgements(ctx.manifest.RepoRoot, ctx.inst.CaseRoot, ctx.manifest.Pack, ctx.facts.Requests, ctx.lane.ID, ExecutionEvidenceReviewAcknowledgedIDs(ctx.facts)); len(items) > 0 {
		add("authorized-gate-adapter", fmt.Sprintf("%d authorized adapter action(s) remain open", len(items)))
	}
	if ctx.lane.Authority {
		for _, boardLane := range ctx.board.Lanes {
			if boardLane.ID == ctx.lane.ID {
				continue
			}
			if strings.ToLower(strings.TrimSpace(boardLane.Status)) != "closed" {
				add("open-feature-lane", boardLane.ID)
				continue
			}
			if _, err := InspectLaneCompletion(ctx.inst.CaseRoot, boardLane.ID); err != nil {
				add("uncommitted-feature-closure", boardLane.ID+": "+err.Error())
			}
		}
	}
	sort.SliceStable(blockers, func(i, j int) bool {
		if blockers[i].Kind == blockers[j].Kind {
			return blockers[i].Detail < blockers[j].Detail
		}
		return blockers[i].Kind < blockers[j].Kind
	})
	return blockers, nil
}

func (ctx completeContext) result(mutating, applied bool) CompleteResult {
	brief := projectMissionBrief(ctx.board.Lanes, ctx.facts)
	actions := []mission.MissionCommanderNextActionItem{}
	queue := mission.MissionCommanderActionQueueFor(actions)
	if applied {
		if b, err := readBoard(ctx.inst.CaseRoot); err == nil {
			brief = projectMissionBrief(b.Lanes, ctx.facts)
			actions, queue = completionFreshActions(ctx.inst.CaseRoot, b, ctx.facts)
		}
	}
	return CompleteResult{
		SchemaVersion: 1, Command: "complete", CaseRoot: ctx.inst.CaseRoot, RepoRoot: ctx.manifest.RepoRoot, Pack: ctx.manifest.Pack,
		IsMutation: mutating, Applied: applied, RequiresConfirmation: !applied, Selector: ctx.selector, Lane: ctx.lane,
		Actor: ctx.actor, Reason: ctx.reason, EvidenceRefs: append([]string{}, ctx.evidenceRefs...), Evidence: append([]CompletionEvidence{}, ctx.evidence...), Blocked: len(ctx.blockers) > 0,
		Blockers: append([]CompletionBlocker{}, ctx.blockers...), MissionBrief: brief, MissionCommanderNextActions: actions, MissionCommanderActionQueue: queue,
		BlockedActions: []string{"authority/confirmed writes", "heavy-tool execution", "implicit lane reopen", "completion while durable blockers remain open"},
	}
}

func completionFreshActions(caseRoot string, b board, facts mission.LedgerFacts) ([]mission.MissionCommanderNextActionItem, mission.MissionCommanderActionQueue) {
	items := []mission.MissionCommanderNextActionItem{}
	brief := projectMissionBrief(b.Lanes, facts)
	for _, boardLane := range mission.OpenBoardLanes(b.Lanes) {
		lane, err := readLaneByID(caseRoot, boardLane.ID)
		if err != nil {
			continue
		}
		action := laneExecutorActionFor(lane, mission.LaneFacts(facts.Facts, lane.ID), brief)
		items = append(items, mission.MissionCommanderNextActions([]mission.LaneExecutorActionSnapshot{laneCommanderActionSnapshot(lane, action)}, laneExecutionEvidenceReview(lane, facts), action.Blocked)...)
	}
	return items, mission.MissionCommanderActionQueueFor(items)
}

func (ctx completeContext) plannedWrites() []StartWrite {
	laneRoot, _ := laneRootPath(ctx.inst.CaseRoot, ctx.lane)
	return []StartWrite{
		{Path: relativePath(ctx.inst.CaseRoot, ctx.intentPath()), Kind: "lane-completion-intent", Action: "would-create", TargetPath: ctx.intentPath()},
		{Path: relativePath(ctx.inst.CaseRoot, LaneEventsJSONLPath(laneRoot)), Kind: "lane-event", Action: "would-append-lane-completed", TargetPath: LaneEventsJSONLPath(laneRoot)},
		{Path: relativePath(ctx.inst.CaseRoot, filepath.Join(laneRoot, "lane.json")), Kind: "lane", Action: "would-close", TargetPath: filepath.Join(laneRoot, "lane.json")},
		{Path: ".rekit/board.json", Kind: "board", Action: "would-refresh"},
		{Path: relativePath(ctx.inst.CaseRoot, filepath.Join(laneRoot, "prompts", "RESUME.md")), Kind: "lane-resume", Action: "would-refresh"},
		{Path: relativePath(ctx.inst.CaseRoot, filepath.Join(laneRoot, "checkpoints", "latest.json")), Kind: "lane-checkpoint", Action: "would-refresh"},
		{Path: relativePath(ctx.inst.CaseRoot, ctx.commitPath()), Kind: "lane-completion-commit", Action: "would-create-last", TargetPath: ctx.commitPath()},
	}
}

func (ctx completeContext) newIntent(previewSHA, now string) completionIntent {
	return completionIntent{
		SchemaVersion: 1, Kind: "lane-completion-intent", Sequence: ctx.sequence, PreviousReceiptSHA: ctx.previousSHA, Lane: ctx.lane.ID, Label: workstreamLabel(ctx.lane), Authority: ctx.lane.Authority,
		PreviousStatus: strings.ToLower(strings.TrimSpace(ctx.lane.Status)), Actor: ctx.actor, Reason: ctx.reason, EvidenceRefs: append([]string{}, ctx.evidenceRefs...), Evidence: append([]CompletionEvidence{}, ctx.evidence...),
		CurrentExecutor: ctx.lane.CurrentExecutor, ExecutorGeneration: ctx.lane.ExecutorGeneration, CreatedAt: now,
		EventID: eventID(ctx.lane.ID, "lane-completed", now), PreviewSHA256: previewSHA, NoAuthority: true, NoConfirmed: true, NoHeavyTool: true,
	}
}

func (ctx completeContext) validateIntent(expected string) error {
	intent := ctx.intent
	if intent == nil || intent.SchemaVersion != 1 || intent.Kind != "lane-completion-intent" || intent.Sequence != ctx.sequence || !strings.EqualFold(intent.PreviousReceiptSHA, ctx.previousSHA) || intent.Lane != ctx.lane.ID ||
		intent.Actor != ctx.actor || intent.Reason != ctx.reason || !equalStrings(intent.EvidenceRefs, ctx.evidenceRefs) || !equalCompletionEvidence(intent.Evidence, ctx.evidence) ||
		!strings.EqualFold(intent.PreviewSHA256, expected) || !intent.NoAuthority || !intent.NoConfirmed || !intent.NoHeavyTool {
		return fmt.Errorf("existing lane completion intent does not match the requested actor, reason, evidence, lane, or preview hash")
	}
	if intent.CurrentExecutor != ctx.lane.CurrentExecutor || intent.ExecutorGeneration != ctx.lane.ExecutorGeneration {
		return fmt.Errorf("lane executor owner changed after completion intent")
	}
	if _, err := time.Parse(time.RFC3339Nano, intent.CreatedAt); err != nil {
		return fmt.Errorf("lane completion intent createdAt is invalid: %w", err)
	}
	return nil
}

func (ctx completeContext) publishCompletion() ([]StartWrite, CompletionReceipt, error) {
	intent := *ctx.intent
	intentSHA, err := hashJSON(intent)
	if err != nil {
		return nil, CompletionReceipt{}, err
	}
	laneRoot, err := laneRootPath(ctx.inst.CaseRoot, ctx.lane)
	if err != nil {
		return nil, CompletionReceipt{}, err
	}
	writes := []StartWrite{{Path: relativePath(ctx.inst.CaseRoot, ctx.intentPath()), Kind: "lane-completion-intent", Action: "existing", TargetPath: ctx.intentPath()}}
	event := map[string]any{
		"schemaVersion": 1, "eventId": intent.EventID, "kind": "lane-completed", "lane": ctx.lane.ID, "time": intent.CreatedAt,
		"summary": "operationally completed lane: " + workstreamLabel(ctx.lane), "previousStatus": intent.PreviousStatus,
		"actor": intent.Actor, "reason": intent.Reason, "evidenceRefs": intent.EvidenceRefs, "evidence": intent.Evidence, "currentExecutor": intent.CurrentExecutor,
		"executorGeneration": intent.ExecutorGeneration, "previewSha256": intent.PreviewSHA256, "noAuthority": true, "noConfirmed": true, "noHeavyTool": true,
	}
	eventWrite, err := appendCompletionLaneEvent(ctx.inst.CaseRoot, LaneEventsJSONLPath(laneRoot), event)
	if err != nil {
		return nil, CompletionReceipt{}, err
	}
	writes = append(writes, eventWrite)
	ctx.lane.Status = "closed"
	ctx.lane.UpdatedAt = intent.CreatedAt
	lanePath := filepath.Join(laneRoot, "lane.json")
	if err := writeJSON(lanePath, ctx.lane); err != nil {
		return nil, CompletionReceipt{}, err
	}
	writes = append(writes, StartWrite{Path: relativePath(ctx.inst.CaseRoot, lanePath), Kind: "lane", Action: "close", TargetPath: lanePath})
	boardPath, err := saveBoard(ctx.inst.CaseRoot, ctx.manifest)
	if err != nil {
		return nil, CompletionReceipt{}, err
	}
	writes = append(writes, StartWrite{Path: ".rekit/board.json", Kind: "board", Action: "refresh", TargetPath: boardPath})
	resumePath, checkpointPath, err := writeLaneResume(ctx.inst.CaseRoot, ctx.manifest, ctx.lane)
	if err != nil {
		return nil, CompletionReceipt{}, err
	}
	writes = append(writes,
		StartWrite{Path: relativePath(ctx.inst.CaseRoot, resumePath), Kind: "lane-resume", Action: "refresh", TargetPath: resumePath},
		StartWrite{Path: relativePath(ctx.inst.CaseRoot, checkpointPath), Kind: "lane-checkpoint", Action: "refresh", TargetPath: checkpointPath},
	)
	laneSHA, err := hashFile(lanePath)
	if err != nil {
		return nil, CompletionReceipt{}, err
	}
	boardLaneSHA, err := currentBoardLaneSHA(ctx.inst.CaseRoot, ctx.lane.ID)
	if err != nil {
		return nil, CompletionReceipt{}, err
	}
	resumeSHA, err := hashFile(resumePath)
	if err != nil {
		return nil, CompletionReceipt{}, err
	}
	checkpointSHA, err := hashFile(checkpointPath)
	if err != nil {
		return nil, CompletionReceipt{}, err
	}
	receipt := CompletionReceipt{
		SchemaVersion: 1, Kind: "lane-completion", State: "committed", Sequence: intent.Sequence, PreviousReceiptSHA: intent.PreviousReceiptSHA, Lane: intent.Lane, Label: intent.Label, Authority: intent.Authority,
		PreviousStatus: intent.PreviousStatus, Actor: intent.Actor, Reason: intent.Reason, EvidenceRefs: append([]string{}, intent.EvidenceRefs...), Evidence: append([]CompletionEvidence{}, intent.Evidence...),
		CurrentExecutor: intent.CurrentExecutor, ExecutorGeneration: intent.ExecutorGeneration, CompletedAt: intent.CreatedAt, EventID: intent.EventID,
		PreviewSHA256: intent.PreviewSHA256, IntentSHA256: intentSHA, LaneSHA256: laneSHA, BoardLaneSHA256: boardLaneSHA,
		ResumeSHA256: resumeSHA, CheckpointSHA256: checkpointSHA, NoAuthority: true, NoConfirmed: true, NoHeavyTool: true,
	}
	if err := writeCompletionExclusive(ctx.inst.CaseRoot, ctx.commitPath(), receipt); err != nil {
		if existing, inspectErr := InspectLaneCompletion(ctx.inst.CaseRoot, ctx.lane.ID); inspectErr != nil || !completionReceiptsEqual(existing, receipt) {
			return nil, CompletionReceipt{}, err
		}
	}
	writes = append(writes, StartWrite{Path: relativePath(ctx.inst.CaseRoot, ctx.commitPath()), Kind: "lane-completion-commit", Action: "create-last", TargetPath: ctx.commitPath()})
	return writes, receipt, nil
}

func InspectLaneCompletion(caseRoot, laneID string) (CompletionReceipt, error) {
	if err := validateLaneIDSegment(laneID); err != nil {
		return CompletionReceipt{}, err
	}
	lifecycle, err := lanecompletion.Inspect(caseRoot, laneID)
	if err != nil {
		return CompletionReceipt{}, err
	}
	if lifecycle.State != lanecompletion.StateComplete || lifecycle.CurrentCompletion == nil || len(lifecycle.Transitions) == 0 {
		return CompletionReceipt{}, fmt.Errorf("lane has no current committed completion: %s state=%s", laneID, lifecycle.State)
	}
	var receipt CompletionReceipt
	if err := convertCompletionJSON(lifecycle.CurrentCompletion, &receipt); err != nil {
		return CompletionReceipt{}, err
	}
	transition := lifecycle.Transitions[len(lifecycle.Transitions)-1]
	intent, err := readCompletionIntent(transition.IntentPath)
	if err != nil {
		return CompletionReceipt{}, err
	}
	_, currentEvidence, err := validateCompletionEvidenceRefs(caseRoot, strings.Join(receipt.EvidenceRefs, ","))
	if err != nil || !equalCompletionEvidence(currentEvidence, receipt.Evidence) {
		return CompletionReceipt{}, fmt.Errorf("lane completion evidence content mismatch: %s", laneID)
	}
	if !equalStrings(intent.EvidenceRefs, receipt.EvidenceRefs) || !equalCompletionEvidence(intent.Evidence, receipt.Evidence) {
		return CompletionReceipt{}, fmt.Errorf("lane completion evidence identity mismatch: %s", laneID)
	}
	intentSHA, err := hashJSON(intent)
	if err != nil || intentSHA != receipt.IntentSHA256 {
		return CompletionReceipt{}, fmt.Errorf("lane completion intent hash mismatch: %s", laneID)
	}
	root, err := refsf.SafeJoin(caseRoot, relJoin(".rekit", "lanes", laneID))
	if err != nil {
		return CompletionReceipt{}, err
	}
	checks := []struct{ path, want, label string }{
		{filepath.Join(root, "lane.json"), receipt.LaneSHA256, "lane"},
		{filepath.Join(root, "prompts", "RESUME.md"), receipt.ResumeSHA256, "resume"},
		{filepath.Join(root, "checkpoints", "latest.json"), receipt.CheckpointSHA256, "checkpoint"},
	}
	for _, check := range checks {
		actual, hashErr := hashFile(check.path)
		if hashErr != nil || !strings.EqualFold(actual, check.want) {
			return CompletionReceipt{}, fmt.Errorf("lane completion %s hash mismatch: %s", check.label, laneID)
		}
	}
	lane, err := readLane(filepath.Join(root, "lane.json"))
	if err != nil || strings.ToLower(strings.TrimSpace(lane.Status)) != "closed" || lane.CurrentExecutor != receipt.CurrentExecutor || lane.ExecutorGeneration != receipt.ExecutorGeneration {
		return CompletionReceipt{}, fmt.Errorf("lane completion lane state mismatch: %s", laneID)
	}
	boardSHA, err := currentBoardLaneSHA(caseRoot, laneID)
	if err != nil || !strings.EqualFold(boardSHA, receipt.BoardLaneSHA256) {
		return CompletionReceipt{}, fmt.Errorf("lane completion board projection mismatch: %s", laneID)
	}
	return receipt, nil
}

func InspectMissionCompletion(caseRoot string) (MissionCompletionHandoff, error) {
	b, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return MissionCompletionHandoff{}, err
	}
	out := MissionCompletionHandoff{State: "active", OpenLaneCount: len(mission.OpenBoardLanes(b.Lanes)), Boundary: completionBoundary()}
	operations, operationErr := lanecompletion.InspectOperations(caseRoot)
	if operationErr != nil {
		out.State = "reopen-publication-incomplete"
		out.Summary = "lane reopen operation lifecycle is invalid"
		return out, operationErr
	}
	if operations.Pending {
		out.State = "reopen-publication-incomplete"
		out.Summary = fmt.Sprintf("lane reopen operation publication is pending: sequence=%d", operations.PendingSequence)
		return out, fmt.Errorf("lane reopen operation publication is incomplete: sequence=%d", operations.PendingSequence)
	}
	for _, lane := range b.Lanes {
		lifecycle, lifecycleErr := lanecompletion.Inspect(caseRoot, lane.ID)
		if lifecycleErr != nil {
			out.State = "completion-publication-incomplete"
			out.Summary = "lane completion lifecycle is invalid: " + lane.ID
			return out, lifecycleErr
		}
		closed := strings.EqualFold(strings.TrimSpace(lane.Status), "closed")
		switch lifecycle.State {
		case lanecompletion.StateNone:
			if closed {
				out.State = "completion-publication-incomplete"
				out.Summary = "closed lane lacks a committed completion publication: " + lane.ID
				return out, fmt.Errorf("closed lane lacks committed completion: %s", lane.ID)
			}
		case lanecompletion.StatePending:
			out.State = "completion-publication-incomplete"
			out.Summary = "lane completion lifecycle publication is pending: " + lane.ID
			return out, fmt.Errorf("lane completion publication is incomplete: %s transition=%s sequence=%d", lane.ID, lifecycle.PendingKind, lifecycle.PendingSequence)
		case lanecompletion.StateReopened:
			if closed {
				out.State = "completion-publication-incomplete"
				out.Summary = "reopened lane remains closed: " + lane.ID
				return out, fmt.Errorf("lane reopen projection is incomplete: %s", lane.ID)
			}
		case lanecompletion.StateComplete:
			if !closed {
				out.State = "completion-publication-incomplete"
				out.Summary = "completed lane is not closed: " + lane.ID
				return out, fmt.Errorf("lane completion projection is incomplete: %s", lane.ID)
			}
			receipt, err := InspectLaneCompletion(caseRoot, lane.ID)
			if err != nil {
				out.State = "completion-publication-incomplete"
				out.Summary = "closed lane lacks a current committed completion publication: " + lane.ID
				return out, err
			}
			out.Receipts = append(out.Receipts, receipt)
		}
	}
	sort.Slice(out.Receipts, func(i, j int) bool { return out.Receipts[i].Lane < out.Receipts[j].Lane })
	out.CompletedLaneCount = len(out.Receipts)
	if len(b.Lanes) > 0 && out.OpenLaneCount == 0 && out.CompletedLaneCount == len(b.Lanes) {
		out.Ready = true
		out.State = "mission-complete"
		out.OperationallyComplete = true
		out.Summary = "all durable lanes are operationally complete with reviewed lane-completion receipts"
	}
	return out, nil
}

func completionBoundary() []string {
	return []string{
		"operational completion is derived only from current committed lane-completion receipts",
		"completion does not write or infer authority/confirmed conclusions",
		"completion does not execute heavy tools or manage external sessions",
		"closed lanes are reopened only by the dedicated review-first reopen owner, never by start, continue, gate, note, reviewer, or executor takeover",
	}
}

func validateCompletionEvidenceRefs(caseRoot, value string) ([]string, []CompletionEvidence, error) {
	const maxEvidenceBytes = 16 << 20

	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || r == '\n' || r == '\r' })
	refs := mission.UniqueStrings(parts)
	if len(refs) == 0 {
		return nil, nil, fmt.Errorf("complete requires at least one case-local non-empty -EvidenceRefs file")
	}
	caseReal, err := filepath.EvalSymlinks(caseRoot)
	if err != nil {
		return nil, nil, err
	}
	evidence := make([]CompletionEvidence, 0, len(refs))
	for _, ref := range refs {
		pathPart := strings.TrimSpace(strings.SplitN(ref, "#", 2)[0])
		if pathPart == "" || filepath.IsAbs(pathPart) {
			return nil, nil, fmt.Errorf("completion evidence ref must be case-relative: %s", ref)
		}
		path, err := refsf.SafeJoin(caseRoot, pathPart)
		if err != nil {
			return nil, nil, err
		}
		pathReal, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil, nil, fmt.Errorf("completion evidence ref is unreadable: %s: %w", ref, err)
		}
		rel, err := filepath.Rel(caseReal, pathReal)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, nil, fmt.Errorf("completion evidence ref escapes case root: %s", ref)
		}
		before, err := os.Lstat(pathReal)
		if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() == 0 || before.Size() > maxEvidenceBytes {
			return nil, nil, fmt.Errorf("completion evidence ref must be a bounded non-empty regular file: %s", ref)
		}
		file, err := os.Open(pathReal)
		if err != nil {
			return nil, nil, fmt.Errorf("completion evidence ref is unreadable: %s: %w", ref, err)
		}
		opened, statErr := file.Stat()
		if statErr != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
			file.Close()
			return nil, nil, fmt.Errorf("completion evidence ref changed while opening: %s", ref)
		}
		data, readErr := io.ReadAll(io.LimitReader(file, maxEvidenceBytes+1))
		closeErr := file.Close()
		if readErr != nil || closeErr != nil || len(data) == 0 || len(data) > maxEvidenceBytes {
			return nil, nil, fmt.Errorf("completion evidence ref could not be read stably: %s", ref)
		}
		post, err := os.Lstat(pathReal)
		currentReal, realErr := filepath.EvalSymlinks(path)
		if err != nil || realErr != nil || !post.Mode().IsRegular() || post.Mode()&os.ModeSymlink != 0 || !os.SameFile(opened, post) || !strings.EqualFold(filepath.Clean(currentReal), filepath.Clean(pathReal)) {
			return nil, nil, fmt.Errorf("completion evidence ref changed while reading: %s", ref)
		}
		sum := sha256.Sum256(data)
		evidence = append(evidence, CompletionEvidence{Ref: ref, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(data))})
	}
	return refs, evidence, nil
}

func completePreviewSHA256(result CompleteResult) (string, error) {
	result.CompletionPlanSHA256 = ""
	result.ApplyCommand = ""
	data, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func completeApplyCommand(ctx completeContext, hash string) string {
	args := []string{"/rekit", "complete", workstreamLabel(ctx.lane), "-Actor", ctx.actor, "-Reason", ctx.reason, "-EvidenceRefs", strings.Join(ctx.evidenceRefs, ","), "-ExpectedCompletePlanSha256", hash, "-Apply", "-Format", "json"}
	for i := range args {
		args[i] = quoteCompleteCommandArg(args[i])
	}
	return strings.Join(args, " ")
}

func quoteCompleteCommandArg(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\r\n\"'") {
		return value
	}
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func completionBlockerSummary(items []CompletionBlocker) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, item.Kind+"="+item.Detail)
	}
	return strings.Join(parts, "; ")
}

func (ctx completeContext) intentPath() string {
	return lanecompletion.IntentPath(ctx.inst.CaseRoot, ctx.lane.ID, ctx.sequence, "complete")
}

func (ctx completeContext) commitPath() string {
	return lanecompletion.ReceiptPath(ctx.inst.CaseRoot, ctx.lane.ID, ctx.sequence, "complete")
}

func readCompletionIntent(path string) (completionIntent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return completionIntent{}, err
	}
	var intent completionIntent
	if err := json.Unmarshal(data, &intent); err != nil {
		return completionIntent{}, fmt.Errorf("invalid lane completion intent %s: %w", path, err)
	}
	return intent, nil
}

func writeCompletionExclusive(caseRoot, path string, value any) error {
	return lanecompletion.WriteExclusiveJSON(caseRoot, path, value)
}

func appendCompletionLaneEvent(caseRoot, path string, event map[string]any) (StartWrite, error) {
	rel := relativePath(caseRoot, path)
	id := mission.Value(event, "eventId")
	existing, err := mission.ReadStrictJSONLineObjects(path)
	if err != nil {
		return StartWrite{}, err
	}
	for _, item := range existing {
		if mission.Value(item, "eventId") != id {
			continue
		}
		for _, key := range []string{"kind", "lane", "actor", "reason", "previewSha256", "currentExecutor", "executorGeneration"} {
			if mission.Value(item, key) != mission.Value(event, key) {
				return StartWrite{}, fmt.Errorf("lane completion event %s differs on %s; refusing recovery", id, key)
			}
		}
		return StartWrite{Path: rel, Kind: "lane-event", Action: "already-appended", TargetPath: path}, nil
	}
	if err := mission.AppendJSONLine(path, event); err != nil {
		return StartWrite{}, err
	}
	return StartWrite{Path: rel, Kind: "lane-event", Action: "append-lane-completed", TargetPath: path}, nil
}

func currentBoardLaneSHA(caseRoot, laneID string) (string, error) {
	b, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return "", err
	}
	for _, lane := range b.Lanes {
		if lane.ID == laneID {
			return hashJSON(lane)
		}
	}
	return "", fmt.Errorf("board omits lane completion projection: %s", laneID)
}

func hashJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func completionReceiptsEqual(left, right CompletionReceipt) bool {
	leftData, _ := json.Marshal(left)
	rightData, _ := json.Marshal(right)
	return string(leftData) == string(rightData)
}

func convertCompletionJSON(source, target any) error {
	data, err := json.Marshal(source)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func equalCompletionEvidence(left, right []CompletionEvidence) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
