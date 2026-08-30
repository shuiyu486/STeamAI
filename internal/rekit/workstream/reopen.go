package workstream

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanecompletion"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/plancontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

var (
	reopenAfterOperationIntentHook func() error
	reopenPublicationHook          func(stage, lane string) error
)

func SetReopenAfterOperationIntentHookForTest(hook func() error) func() {
	previous := reopenAfterOperationIntentHook
	reopenAfterOperationIntentHook = hook
	return func() { reopenAfterOperationIntentHook = previous }
}

type ReopenOptions struct {
	Selector              string
	Actor                 string
	Reason                string
	EvidenceRefs          string
	PublicationStamp      string
	ExpectedPreviewSHA256 string
}

type ReopenTarget struct {
	Lane                         Lane   `json:"lane"`
	Sequence                     int    `json:"sequence"`
	PreviousReceiptSHA256        string `json:"previousReceiptSha256"`
	SupersededCompletionSequence int    `json:"supersededCompletionSequence"`
	Reason                       string `json:"reason"`
}

type ReopenResult struct {
	SchemaVersion               int                                      `json:"schemaVersion"`
	Command                     string                                   `json:"command"`
	CaseRoot                    string                                   `json:"caseRoot"`
	RepoRoot                    string                                   `json:"repoRoot"`
	Pack                        string                                   `json:"pack"`
	IsMutation                  bool                                     `json:"isMutation"`
	Applied                     bool                                     `json:"applied"`
	RequiresConfirmation        bool                                     `json:"requiresConfirmation"`
	RequestedLane               string                                   `json:"requestedLane"`
	EffectiveTargets            []ReopenTarget                           `json:"effectiveTargets"`
	Actor                       string                                   `json:"actor"`
	Reason                      string                                   `json:"reason"`
	EvidenceRefs                []string                                 `json:"evidenceRefs"`
	Evidence                    []CompletionEvidence                     `json:"evidence"`
	OperationID                 string                                   `json:"operationId"`
	OperationSequence           int                                      `json:"operationSequence"`
	ReopenPlanSHA256            string                                   `json:"reopenPlanSha256,omitempty"`
	ExactPublicationSHA256      string                                   `json:"exactPublicationSha256,omitempty"`
	PublicationStamp            string                                   `json:"publicationStamp,omitempty"`
	Replay                      bool                                     `json:"replay,omitempty"`
	ApplyCommand                string                                   `json:"applyCommand,omitempty"`
	ApplyArgs                   []string                                 `json:"applyArgs,omitempty"`
	OperationCommit             *lanecompletion.OperationCommit          `json:"operationCommit,omitempty"`
	MissionBrief                mission.Brief                            `json:"missionBrief"`
	MissionCommanderNextActions []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions,omitempty"`
	MissionCommanderActionQueue mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
	WouldWrites                 []StartWrite                             `json:"wouldWrites,omitempty"`
	Writes                      []StartWrite                             `json:"writes,omitempty"`
	BlockedActions              []string                                 `json:"blockedActions"`
	NextSteps                   []string                                 `json:"nextSteps"`
}

type reopenContext struct {
	inst                   instance.Instance
	manifest               *manifest.Manifest
	board                  board
	facts                  mission.LedgerFacts
	selector               string
	requested              Lane
	actor                  string
	reason                 string
	evidenceRefs           []string
	requestedEvidenceRefs  string
	evidence               []CompletionEvidence
	operation              lanecompletion.OperationInspection
	targets                []reopenTargetContext
	operationID            string
	sequence               int
	intent                 *lanecompletion.OperationIntent
	publicationStamp       string
	exactPublicationSHA256 string
	operationPublications  []lanecompletion.OperationPublication
	operationIntentPath    string
	operationCommitPath    string
	boardPath              string
	replayCommit           *lanecompletion.OperationCommit
}

type reopenTargetContext struct {
	lane                  Lane
	lifecycle             lanecompletion.Inspection
	sequence              int
	previousReceiptSHA256 string
	supersededSequence    int
	reason                string
	intentPath            string
	receiptPath           string
	laneIntent            *lanecompletion.ReopenIntent
	publications          []lanecompletion.OperationPublication
}

func ReopenPreview(repoRoot, caseRoot, pack string, opt ReopenOptions) (ReopenResult, error) {
	ctx, err := newReopenContext(repoRoot, caseRoot, pack, opt, false)
	if err != nil {
		return ReopenResult{}, err
	}
	if _, err := plancontract.ValidatePhase(
		commands.Reopen,
		"-ExpectedReopenPlanSha256",
		true,
		false,
		opt.ExpectedPreviewSHA256,
	); err != nil {
		return ReopenResult{}, err
	}
	stamp := strings.TrimSpace(opt.PublicationStamp)
	if stamp == "" {
		stamp = isoNow()
	}
	if err := ctx.buildPublicationPlan(stamp); err != nil {
		return ReopenResult{}, err
	}
	result := ctx.result(false, false)
	result.PublicationStamp = stamp
	result.WouldWrites, err = ctx.plannedWrites()
	if err != nil {
		return ReopenResult{}, err
	}
	result.ReopenPlanSHA256 = ctx.exactPublicationSHA256
	result.ApplyArgs = reopenApplyArgs(ctx, ctx.exactPublicationSHA256)
	result.ApplyCommand = reopenApplyCommandForArgs(result.ApplyArgs)
	result.NextSteps = []string{"review every effective target, superseded completion receipt, evidence identity, publication stamp, and exact write set, then run applyCommand", "after Apply, refresh status and begin only a fresh post-reopen lane campaign"}
	return result, nil
}

func ReopenApply(repoRoot, caseRoot, pack string, opt ReopenOptions) (result ReopenResult, err error) {
	mutationStarted := false
	defer func() {
		if err != nil && !mutationStarted {
			err = MarkZeroProgress(err)
		}
	}()
	expected, err := plancontract.RequireApplyBinding(
		commands.Reopen,
		"-ExpectedReopenPlanSha256",
		opt.ExpectedPreviewSHA256,
	)
	if err != nil {
		return ReopenResult{}, err
	}
	ctx, err := newReopenContext(repoRoot, caseRoot, pack, opt, true)
	if err != nil {
		return ReopenResult{}, err
	}
	lease, err := acquireProjectMutationLock(ctx.inst.CaseRoot)
	if err != nil {
		return ReopenResult{}, err
	}
	defer func() {
		if unlockErr := lease.Unlock(); unlockErr != nil {
			err = errors.Join(err, unlockErr)
			result = ReopenResult{}
		}
	}()
	ctx, err = newReopenContext(repoRoot, caseRoot, pack, opt, true)
	if err != nil {
		return ReopenResult{}, err
	}
	if err := lease.Validate(); err != nil {
		return ReopenResult{}, err
	}
	if ctx.replayCommit != nil {
		commit := *ctx.replayCommit
		if _, err := plancontract.Match(
			commands.Reopen,
			"-ExpectedReopenPlanSha256",
			expected,
			commit.PreviewSHA256,
		); err != nil {
			return ReopenResult{}, err
		}
		result = ctx.result(true, true)
		result.Replay, result.ReopenPlanSHA256, result.OperationCommit = true, commit.PreviewSHA256, &commit
		result.Writes = committedReplayWrites(ctx)
		result.NextSteps = []string{"the exact reopen operation was already committed; refresh status", "no files were written by this committed replay"}
		return result, nil
	}
	if ctx.intent == nil {
		stamp := strings.TrimSpace(opt.PublicationStamp)
		if stamp == "" {
			return ReopenResult{}, fmt.Errorf("reopen apply requires -ReopenPublicationStamp from the reviewed preview")
		}
		if err := ctx.buildPublicationPlan(stamp); err != nil {
			return ReopenResult{}, err
		}
		if _, err := plancontract.Match(
			commands.Reopen,
			"-ExpectedReopenPlanSha256",
			expected,
			ctx.exactPublicationSHA256,
		); err != nil {
			return ReopenResult{}, err
		}
		intent := ctx.newOperationIntent(expected, stamp)
		mutationStarted = true
		if err := writeCompletionExclusive(ctx.inst.CaseRoot, ctx.operationIntentPath, intent); err != nil {
			return ReopenResult{}, err
		}
		ctx.intent = &intent
		if reopenAfterOperationIntentHook != nil {
			if err := reopenAfterOperationIntentHook(); err != nil {
				return ReopenResult{}, err
			}
		}
	} else {
		if _, err := plancontract.Match(
			commands.Reopen,
			"-ExpectedReopenPlanSha256",
			expected,
			ctx.intent.PreviewSHA256,
		); err != nil {
			return ReopenResult{}, err
		}
		if err := ctx.validateOperationIntent(expected); err != nil {
			return ReopenResult{}, err
		}
		mutationStarted = true
	}
	if err := lease.Validate(); err != nil {
		return ReopenResult{}, err
	}
	writes, commit, err := ctx.publish()
	if err != nil {
		return ReopenResult{}, err
	}
	result = ctx.result(true, true)
	result.Writes = writes
	result.ReopenPlanSHA256 = commit.PreviewSHA256
	result.OperationCommit = &commit
	result.NextSteps = []string{"run /rekit status and consume the fresh current driver request", "prior completion receipts remain immutable historical evidence; reopen does not restore old session ownership, authority, confirmed, or current-loop budget"}
	return result, nil
}

func newReopenContext(repoRoot, caseRoot, pack string, opt ReopenOptions, allowPending bool) (reopenContext, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return reopenContext{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return reopenContext{}, err
	}
	selector := strings.TrimSpace(opt.Selector)
	if selector == "" {
		return reopenContext{}, fmt.Errorf("reopen requires a lane selector")
	}
	actor, reason := strings.TrimSpace(opt.Actor), strings.TrimSpace(opt.Reason)
	if actor == "" || reason == "" {
		return reopenContext{}, fmt.Errorf("reopen requires non-empty -Actor and -Reason")
	}
	operations, err := lanecompletion.InspectOperations(inst.CaseRoot)
	if err != nil {
		return reopenContext{}, err
	}
	boardPath, err := boardRelPath(inst.CaseRoot)
	if err != nil {
		return reopenContext{}, err
	}
	if allowPending && !operations.Pending && len(operations.Commits) > 0 {
		requestedRefs := mission.UniqueStrings(strings.FieldsFunc(opt.EvidenceRefs, func(r rune) bool { return r == ',' || r == ';' || r == '\n' || r == '\r' }))
		intent, commit, found, err := committedReopenOperationForRequest(
			inst.CaseRoot,
			operations,
			selector,
			actor,
			reason,
			requestedRefs,
			strings.TrimSpace(opt.ExpectedPreviewSHA256),
		)
		if err != nil {
			return reopenContext{}, err
		}
		if found {
			requested, err := requestedLaneFromOperation(intent)
			if err != nil {
				return reopenContext{}, err
			}
			facts, err := mission.ReadStrictLedgerFacts(inst.CaseRoot)
			if err != nil {
				return reopenContext{}, err
			}
			ctx := reopenContext{inst: inst, manifest: m, facts: facts, selector: selector, requested: requested, actor: actor, reason: reason, evidenceRefs: append([]string{}, intent.EvidenceRefs...), requestedEvidenceRefs: opt.EvidenceRefs, evidence: fromLifecycleEvidence(intent.Evidence), operation: operations, boardPath: boardPath}
			if err := ctx.loadCommittedOperation(intent, commit); err != nil {
				return reopenContext{}, err
			}
			return ctx, nil
		}
	}
	if operations.Pending {
		if !allowPending {
			return reopenContext{}, fmt.Errorf("reopen publication is pending; recover the exact original reopen Apply")
		}
		intent, err := lanecompletion.ReadOperationIntent(inst.CaseRoot, operations.PendingIntentPath)
		if err != nil {
			return reopenContext{}, err
		}
		requestedPublication, ok := publicationByRole(intent.Targets[0].Publications, "lane")
		if !ok {
			return reopenContext{}, fmt.Errorf("pending reopen operation lacks requested lane publication")
		}
		var requested Lane
		if err := json.Unmarshal(requestedPublication.Bytes, &requested); err != nil {
			return reopenContext{}, fmt.Errorf("pending reopen requested lane publication is invalid: %w", err)
		}
		for _, target := range intent.Targets {
			if target.Lane != intent.RequestedLane {
				continue
			}
			publication, ok := publicationByRole(target.Publications, "lane")
			if !ok {
				return reopenContext{}, fmt.Errorf("pending reopen requested lane publication is missing")
			}
			if err := json.Unmarshal(publication.Bytes, &requested); err != nil {
				return reopenContext{}, fmt.Errorf("pending reopen requested lane publication is invalid: %w", err)
			}
		}
		facts, err := mission.ReadStrictLedgerFacts(inst.CaseRoot)
		if err != nil {
			return reopenContext{}, err
		}
		ctx := reopenContext{inst: inst, manifest: m, facts: facts, selector: selector, requested: requested, actor: actor, reason: reason, evidenceRefs: append([]string{}, intent.EvidenceRefs...), requestedEvidenceRefs: opt.EvidenceRefs, evidence: fromLifecycleEvidence(intent.Evidence), operation: operations, boardPath: boardPath}
		ctx.sequence, ctx.operationID, ctx.intent = intent.Sequence, intent.OperationID, &intent
		ctx.publicationStamp, ctx.exactPublicationSHA256 = intent.CreatedAt, intent.ExactPublicationSHA256
		ctx.operationPublications = append([]lanecompletion.OperationPublication{}, intent.Publications...)
		if err := ctx.resolveOperationPaths(); err != nil {
			return reopenContext{}, err
		}
		if err := ctx.loadPendingTargets(intent); err != nil {
			return reopenContext{}, err
		}
		return ctx, nil
	}
	b, err := readBoard(inst.CaseRoot)
	if err != nil {
		return reopenContext{}, err
	}
	if strings.TrimSpace(b.DefaultAuthorityLane) == "" {
		b.DefaultAuthorityLane = m.WorkstreamDefaults["defaultAuthorityLane"]
	}
	requested, err := resolveHandoffLane(inst.CaseRoot, b, selector)
	if err != nil {
		return reopenContext{}, err
	}
	refs, evidence, err := validateCompletionEvidenceRefs(inst.CaseRoot, opt.EvidenceRefs)
	if err != nil {
		return reopenContext{}, err
	}
	facts, err := mission.ReadStrictLedgerFacts(inst.CaseRoot)
	if err != nil {
		return reopenContext{}, err
	}
	ctx := reopenContext{inst: inst, manifest: m, board: b, facts: facts, selector: selector, requested: requested, actor: actor, reason: reason, evidenceRefs: refs, requestedEvidenceRefs: opt.EvidenceRefs, evidence: evidence, operation: operations, boardPath: boardPath}
	if replayed, err := ctx.matchCommittedOperation(); err != nil {
		return reopenContext{}, err
	} else if replayed {
		return ctx, nil
	}
	ctx.sequence = operations.LatestSequence + 1
	ctx.operationID = eventID(requested.ID, "lane-reopen-operation", fmt.Sprintf("%d:%s:%s", ctx.sequence, actor, reason))
	if err := ctx.resolveOperationPaths(); err != nil {
		return reopenContext{}, err
	}
	if err := ctx.buildTargets(); err != nil {
		return reopenContext{}, err
	}
	return ctx, nil
}

func (ctx *reopenContext) resolveOperationPaths() error {
	intentPath, err := lanecompletion.OperationIntentPathE(ctx.inst.CaseRoot, ctx.sequence)
	if err != nil {
		return err
	}
	commitPath, err := lanecompletion.OperationCommitPathE(ctx.inst.CaseRoot, ctx.sequence)
	if err != nil {
		return err
	}
	ctx.operationIntentPath = intentPath
	ctx.operationCommitPath = commitPath
	return nil
}

func (ctx *reopenContext) buildTargets() error {
	requestedLifecycle, err := lanecompletion.Inspect(ctx.inst.CaseRoot, ctx.requested.ID)
	if err != nil {
		return err
	}
	if requestedLifecycle.State != lanecompletion.StateComplete || requestedLifecycle.CurrentCompletion == nil {
		return fmt.Errorf("reopen requires an effective committed completion: %s state=%s", ctx.requested.ID, requestedLifecycle.State)
	}
	lanes := []Lane{ctx.requested}
	if !ctx.requested.Authority {
		for _, item := range ctx.board.Lanes {
			if !item.Authority && item.ID != ctx.board.DefaultAuthorityLane {
				continue
			}
			main, err := readLaneByID(ctx.inst.CaseRoot, item.ID)
			if err != nil {
				return err
			}
			mainLifecycle, err := lanecompletion.Inspect(ctx.inst.CaseRoot, main.ID)
			if err != nil {
				return err
			}
			if mainLifecycle.State == lanecompletion.StateComplete {
				lanes = append(lanes, main)
			}
			break
		}
	}
	sort.Slice(lanes, func(i, j int) bool { return lanes[i].ID < lanes[j].ID })
	for _, lane := range lanes {
		lifecycle, err := lanecompletion.Inspect(ctx.inst.CaseRoot, lane.ID)
		if err != nil {
			return err
		}
		if lifecycle.State != lanecompletion.StateComplete || lifecycle.CurrentCompletion == nil {
			return fmt.Errorf("effective reopen target is not complete: %s state=%s", lane.ID, lifecycle.State)
		}
		reason := "requested lane completion is being superseded"
		if lane.Authority && lane.ID != ctx.requested.ID {
			reason = "authority aggregate completion is stale when a feature completion is superseded"
		}
		sequence := lifecycle.HeadSequence + 1
		intentPath, err := lanecompletion.IntentPathE(ctx.inst.CaseRoot, lane.ID, sequence, "reopen")
		if err != nil {
			return err
		}
		receiptPath, err := lanecompletion.ReceiptPathE(ctx.inst.CaseRoot, lane.ID, sequence, "reopen")
		if err != nil {
			return err
		}
		ctx.targets = append(ctx.targets, reopenTargetContext{lane: lane, lifecycle: lifecycle, sequence: sequence, previousReceiptSHA256: lifecycle.HeadReceiptSHA256, supersededSequence: lifecycle.HeadSequence, reason: reason, intentPath: intentPath, receiptPath: receiptPath})
	}
	return nil
}

func (ctx *reopenContext) loadPendingTargets(intent lanecompletion.OperationIntent) error {
	if intent.RequestedSelector != ctx.selector || intent.Actor != ctx.actor || intent.Reason != ctx.reason {
		return fmt.Errorf("pending reopen operation does not match requested selector, actor, or reason")
	}
	requestedRefs := mission.UniqueStrings(strings.FieldsFunc(ctx.requestedEvidenceRefs, func(r rune) bool { return r == ',' || r == ';' || r == '\n' || r == '\r' }))
	if len(requestedRefs) > 0 && !equalStrings(requestedRefs, intent.EvidenceRefs) {
		return fmt.Errorf("pending reopen operation does not match requested evidence refs")
	}
	if intent.RequestedLane != ctx.requested.ID {
		return fmt.Errorf("pending reopen operation targets a different requested lane: %s", intent.RequestedLane)
	}
	for _, target := range intent.Targets {
		lanePublication, ok := publicationByRole(target.Publications, "lane")
		if !ok {
			return fmt.Errorf("pending reopen target lacks exact lane publication: %s", target.Lane)
		}
		var lane Lane
		if err := json.Unmarshal(lanePublication.Bytes, &lane); err != nil || lane.ID != target.Lane {
			return fmt.Errorf("pending reopen target exact lane publication is invalid: %s", target.Lane)
		}
		lifecycle, err := lanecompletion.Inspect(ctx.inst.CaseRoot, target.Lane)
		if err != nil {
			return err
		}
		switch lifecycle.State {
		case lanecompletion.StateComplete:
			if lifecycle.HeadSequence != target.Sequence-1 || !strings.EqualFold(lifecycle.HeadReceiptSHA256, target.PreviousReceiptSHA) {
				return fmt.Errorf("pending reopen target predecessor changed: %s", target.Lane)
			}
		case lanecompletion.StatePending:
			if lifecycle.PendingKind != "reopen" || lifecycle.PendingSequence != target.Sequence || !strings.EqualFold(lifecycle.HeadReceiptSHA256, target.PreviousReceiptSHA) {
				return fmt.Errorf("pending reopen target lifecycle mismatch: %s", target.Lane)
			}
		case lanecompletion.StateReopened:
			if lifecycle.HeadSequence != target.Sequence || !strings.EqualFold(lifecycle.Transitions[len(lifecycle.Transitions)-2].ReceiptSHA256, target.PreviousReceiptSHA) {
				return fmt.Errorf("published reopen target does not match operation predecessor: %s", target.Lane)
			}
		default:
			return fmt.Errorf("pending reopen target is in invalid state: %s state=%s", target.Lane, lifecycle.State)
		}
		intentPath, err := safeCaseRelativePath(ctx.inst.CaseRoot, target.IntentPath)
		if err != nil {
			return err
		}
		receiptPath, err := safeCaseRelativePath(ctx.inst.CaseRoot, target.ReceiptPath)
		if err != nil {
			return err
		}
		var laneIntent *lanecompletion.ReopenIntent
		publishedIntent, readErr := lanecompletion.ReadReopenIntent(ctx.inst.CaseRoot, intentPath)
		if readErr == nil {
			if err := validatePendingLaneIntent(intent, target, publishedIntent); err != nil {
				return err
			}
			intentPublication, ok := publicationByRole(target.Publications, "lane-reopen-intent")
			if !ok {
				return fmt.Errorf("pending reopen target lacks immutable intent publication: %s", target.Lane)
			}
			publishedBytes, err := lanecompletion.ReadCaseFile(ctx.inst.CaseRoot, intentPath)
			if err != nil || string(publishedBytes) != string(intentPublication.Bytes) {
				return fmt.Errorf("pending reopen target immutable intent differs from exact publication: %s", target.Lane)
			}
			laneIntent = &publishedIntent
		} else if !os.IsNotExist(readErr) {
			return readErr
		} else if lifecycle.State != lanecompletion.StateComplete {
			return fmt.Errorf("pending reopen target state requires its immutable intent: %s", target.Lane)
		}
		ctx.targets = append(ctx.targets, reopenTargetContext{lane: lane, lifecycle: lifecycle, sequence: target.Sequence, previousReceiptSHA256: target.PreviousReceiptSHA, supersededSequence: target.SupersededCompletionSequence, reason: target.Reason, intentPath: intentPath, receiptPath: receiptPath, laneIntent: laneIntent, publications: append([]lanecompletion.OperationPublication{}, target.Publications...)})
	}
	return nil
}

func validatePendingLaneIntent(operation lanecompletion.OperationIntent, target lanecompletion.OperationTarget, intent lanecompletion.ReopenIntent) error {
	if intent.SchemaVersion != 1 || intent.Kind != "lane-reopen-intent" || intent.OperationID != operation.OperationID || intent.Sequence != target.Sequence || !strings.EqualFold(intent.PreviousReceiptSHA, target.PreviousReceiptSHA) || intent.SupersededCompletionSequence != target.SupersededCompletionSequence || !strings.EqualFold(intent.SupersededCompletionSHA256, target.PreviousReceiptSHA) || intent.Lane != target.Lane || intent.Actor != operation.Actor || intent.Reason != operation.Reason || !equalStrings(intent.EvidenceRefs, operation.EvidenceRefs) || !equalLifecycleEvidence(intent.Evidence, operation.Evidence) || intent.CreatedAt != operation.CreatedAt || intent.ResultingExecutorGeneration != intent.PreviousExecutorGeneration+1 || !intent.NoAuthority || !intent.NoConfirmed || !intent.NoHeavyTool || !intent.NoAutoResume {
		return fmt.Errorf("pending reopen target immutable intent differs from operation: %s", target.Lane)
	}
	return nil
}

func (ctx reopenContext) result(mutating, applied bool) ReopenResult {
	brief := projectMissionBrief(ctx.board.Lanes, ctx.facts)
	actions := []mission.MissionCommanderNextActionItem{}
	queue := mission.MissionCommanderActionQueueFor(actions)
	if applied {
		if b, err := readBoard(ctx.inst.CaseRoot); err == nil {
			brief = projectMissionBrief(b.Lanes, ctx.facts)
			actions, queue = completionFreshActions(ctx.inst.CaseRoot, b, ctx.facts)
		}
	}
	targets := make([]ReopenTarget, 0, len(ctx.targets))
	for _, target := range ctx.targets {
		targets = append(targets, ReopenTarget{Lane: target.lane, Sequence: target.sequence, PreviousReceiptSHA256: target.previousReceiptSHA256, SupersededCompletionSequence: target.supersededSequence, Reason: target.reason})
	}
	return ReopenResult{SchemaVersion: 1, Command: "reopen", CaseRoot: ctx.inst.CaseRoot, RepoRoot: ctx.manifest.RepoRoot, Pack: ctx.manifest.Pack, IsMutation: mutating, Applied: applied, RequiresConfirmation: !applied, RequestedLane: ctx.requested.ID, EffectiveTargets: targets, Actor: ctx.actor, Reason: ctx.reason, EvidenceRefs: append([]string{}, ctx.evidenceRefs...), Evidence: append([]CompletionEvidence{}, ctx.evidence...), OperationID: ctx.operationID, OperationSequence: ctx.sequence, ExactPublicationSHA256: ctx.exactPublicationSHA256, PublicationStamp: ctx.publicationStamp, MissionBrief: brief, MissionCommanderNextActions: actions, MissionCommanderActionQueue: queue, BlockedActions: []string{"authority/confirmed writes", "heavy-tool execution", "automatic external session restart", "reuse of pre-reopen current-loop budget", "ordinary lane mutation before operation commit"}}
}

func (ctx reopenContext) plannedWrites() ([]StartWrite, error) {
	writes := []StartWrite{{Path: relativePath(ctx.inst.CaseRoot, ctx.operationIntentPath), Kind: "lane-reopen-operation-intent", Action: "would-create", TargetPath: ctx.operationIntentPath}}
	for _, target := range ctx.targets {
		laneRoot, err := laneRootPath(ctx.inst.CaseRoot, target.lane)
		if err != nil {
			return nil, err
		}
		writes = append(writes,
			StartWrite{Path: relativePath(ctx.inst.CaseRoot, target.intentPath), Kind: "lane-reopen-intent", Action: "would-create", TargetPath: target.intentPath},
			StartWrite{Path: relativePath(ctx.inst.CaseRoot, LaneEventsJSONLPath(laneRoot)), Kind: "lane-event", Action: "would-append-lane-reopened", TargetPath: LaneEventsJSONLPath(laneRoot)},
			StartWrite{Path: relativePath(ctx.inst.CaseRoot, filepath.Join(laneRoot, "lane.json")), Kind: "lane", Action: "would-reopen", TargetPath: filepath.Join(laneRoot, "lane.json")},
			StartWrite{Path: relativePath(ctx.inst.CaseRoot, filepath.Join(laneRoot, "prompts", "RESUME.md")), Kind: "lane-resume", Action: "would-refresh", TargetPath: filepath.Join(laneRoot, "prompts", "RESUME.md")},
			StartWrite{Path: relativePath(ctx.inst.CaseRoot, filepath.Join(laneRoot, "checkpoints", "latest.json")), Kind: "lane-checkpoint", Action: "would-refresh", TargetPath: filepath.Join(laneRoot, "checkpoints", "latest.json")},
			StartWrite{Path: relativePath(ctx.inst.CaseRoot, target.receiptPath), Kind: "lane-reopen-commit", Action: "would-create", TargetPath: target.receiptPath},
		)
	}
	writes = append(writes, StartWrite{Path: ctx.boardPath, Kind: "board", Action: "would-refresh"}, StartWrite{Path: relativePath(ctx.inst.CaseRoot, ctx.operationCommitPath), Kind: "lane-reopen-operation-commit", Action: "would-create-last", TargetPath: ctx.operationCommitPath})
	return writes, nil
}

func (ctx reopenContext) newOperationIntent(hash, now string) lanecompletion.OperationIntent {
	targets := make([]lanecompletion.OperationTarget, 0, len(ctx.targets))
	for _, target := range ctx.targets {
		targets = append(targets, lanecompletion.OperationTarget{Lane: target.lane.ID, Sequence: target.sequence, PreviousReceiptSHA: target.previousReceiptSHA256, SupersededCompletionSequence: target.supersededSequence, IntentPath: relativePath(ctx.inst.CaseRoot, target.intentPath), ReceiptPath: relativePath(ctx.inst.CaseRoot, target.receiptPath), Reason: target.reason, Publications: append([]lanecompletion.OperationPublication{}, target.publications...)})
	}
	return lanecompletion.OperationIntent{SchemaVersion: 1, Kind: "lane-reopen-operation-intent", OperationID: ctx.operationID, Sequence: ctx.sequence, RequestedLane: ctx.requested.ID, RequestedSelector: ctx.selector, Actor: ctx.actor, Reason: ctx.reason, EvidenceRefs: append([]string{}, ctx.evidenceRefs...), Evidence: toLifecycleEvidence(ctx.evidence), Targets: targets, Publications: append([]lanecompletion.OperationPublication{}, ctx.operationPublications...), CreatedAt: now, PreviewSHA256: hash, ExactPublicationSHA256: ctx.exactPublicationSHA256, NoAuthority: true, NoConfirmed: true, NoHeavyTool: true, NoAutoResume: true}
}

func (ctx reopenContext) validateOperationIntent(expected string) error {
	intent := ctx.intent
	if intent == nil || intent.OperationID != ctx.operationID || intent.Sequence != ctx.sequence || intent.RequestedLane != ctx.requested.ID || intent.RequestedSelector != ctx.selector || intent.Actor != ctx.actor || intent.Reason != ctx.reason || !strings.EqualFold(intent.PreviewSHA256, expected) || !strings.EqualFold(intent.ExactPublicationSHA256, ctx.exactPublicationSHA256) || !intent.NoAuthority || !intent.NoConfirmed || !intent.NoHeavyTool || !intent.NoAutoResume {
		return fmt.Errorf("existing lane reopen operation intent does not match requested actor, reason, evidence, target, or preview hash")
	}
	if len(intent.Targets) != len(ctx.targets) {
		return fmt.Errorf("existing lane reopen operation target count changed")
	}
	for index, target := range ctx.targets {
		planned := intent.Targets[index]
		if planned.Lane != target.lane.ID || planned.Sequence != target.sequence || !strings.EqualFold(planned.PreviousReceiptSHA, target.previousReceiptSHA256) {
			return fmt.Errorf("existing lane reopen operation target changed: %s", target.lane.ID)
		}
	}
	return nil
}

func (ctx reopenContext) publish() ([]StartWrite, lanecompletion.OperationCommit, error) {
	intent := *ctx.intent
	operationIntentSHA, err := lanecompletion.CanonicalSHA256(intent)
	if err != nil {
		return nil, lanecompletion.OperationCommit{}, err
	}
	writes := []StartWrite{{Path: relativePath(ctx.inst.CaseRoot, ctx.operationIntentPath), Kind: "lane-reopen-operation-intent", Action: "existing", TargetPath: ctx.operationIntentPath}}
	commitTargets := make([]lanecompletion.OperationTarget, 0, len(ctx.targets))
	for roleIndex, roles := range [][]string{{"lane-reopen-intent", "lane-event", "lane"}, {"lane-resume", "lane-checkpoint", "lane-reopen-commit"}} {
		if roleIndex == 1 {
			for _, publication := range intent.Publications {
				write, err := applyReopenPublication(ctx.inst.CaseRoot, publication, "")
				if err != nil {
					return nil, lanecompletion.OperationCommit{}, err
				}
				writes = append(writes, write)
			}
		}
		for _, target := range ctx.targets {
			for _, publication := range target.publications {
				if !containsString(roles, publication.Role) {
					continue
				}
				write, err := applyReopenPublication(ctx.inst.CaseRoot, publication, target.lane.ID)
				if err != nil {
					return nil, lanecompletion.OperationCommit{}, err
				}
				writes = append(writes, write)
			}
		}
	}
	for _, target := range ctx.targets {
		receiptPublication, ok := publicationByRole(target.publications, "lane-reopen-commit")
		if !ok {
			return nil, lanecompletion.OperationCommit{}, fmt.Errorf("reopen operation target lacks exact receipt publication: %s", target.lane.ID)
		}
		commitTargets = append(commitTargets, lanecompletion.OperationTarget{Lane: target.lane.ID, Sequence: target.sequence, PreviousReceiptSHA: target.previousReceiptSHA256, SupersededCompletionSequence: target.supersededSequence, IntentPath: relativePath(ctx.inst.CaseRoot, target.intentPath), ReceiptPath: relativePath(ctx.inst.CaseRoot, target.receiptPath), ReceiptSHA256: receiptPublication.AfterSHA256, Reason: target.reason, Publications: append([]lanecompletion.OperationPublication{}, target.publications...)})
	}
	commit := lanecompletion.OperationCommit{SchemaVersion: 1, Kind: "lane-reopen-operation", State: "committed", OperationID: ctx.operationID, Sequence: ctx.sequence, RequestedLane: ctx.requested.ID, Actor: ctx.actor, Reason: ctx.reason, Targets: commitTargets, CommittedAt: intent.CreatedAt, PreviewSHA256: intent.PreviewSHA256, IntentSHA256: operationIntentSHA, NoAuthority: true, NoConfirmed: true, NoHeavyTool: true, NoAutoResume: true}
	commitPath := ctx.operationCommitPath
	if err := callReopenPublicationHook("before-commit", ""); err != nil {
		return nil, lanecompletion.OperationCommit{}, err
	}
	if err := writeOrValidateExclusive(ctx.inst.CaseRoot, commitPath, commit); err != nil {
		return nil, lanecompletion.OperationCommit{}, err
	}
	writes = append(writes, StartWrite{Path: relativePath(ctx.inst.CaseRoot, commitPath), Kind: "lane-reopen-operation-commit", Action: "create-last", TargetPath: commitPath})
	if err := callReopenPublicationHook("commit", ""); err != nil {
		return nil, lanecompletion.OperationCommit{}, err
	}
	if _, err := lanecompletion.InspectOperations(ctx.inst.CaseRoot); err != nil {
		return nil, lanecompletion.OperationCommit{}, err
	}
	return writes, commit, nil
}

func applyReopenPublication(caseRoot string, publication lanecompletion.OperationPublication, lane string) (StartWrite, error) {
	replayed, err := lanecompletion.ApplyOperationPublication(caseRoot, publication)
	if err != nil {
		return StartWrite{}, err
	}
	action := "publish-exact"
	if replayed {
		action = "already-published-exact"
	}
	if err := callReopenPublicationHook(reopenPublicationStage(publication.Role), lane); err != nil {
		return StartWrite{}, err
	}
	return StartWrite{Path: publication.Path, Kind: publication.Role, Action: action, TargetPath: filepath.Join(caseRoot, filepath.FromSlash(publication.Path))}, nil
}

func requestedLaneFromOperation(intent lanecompletion.OperationIntent) (Lane, error) {
	for _, target := range intent.Targets {
		if target.Lane != intent.RequestedLane {
			continue
		}
		publication, ok := publicationByRole(target.Publications, "lane")
		if !ok {
			return Lane{}, fmt.Errorf("reopen operation requested lane publication is missing")
		}
		var lane Lane
		if err := json.Unmarshal(publication.Bytes, &lane); err != nil || lane.ID != target.Lane {
			return Lane{}, fmt.Errorf("reopen operation requested lane publication is invalid")
		}
		return lane, nil
	}
	return Lane{}, fmt.Errorf("reopen operation omits requested lane target: %s", intent.RequestedLane)
}

func committedReopenOperationForRequest(caseRoot string, operations lanecompletion.OperationInspection, selector, actor, reason string, evidenceRefs []string, expectedPreviewSHA256 string) (lanecompletion.OperationIntent, lanecompletion.OperationCommit, bool, error) {
	for index := len(operations.Commits) - 1; index >= 0; index-- {
		commit := operations.Commits[index]
		intentPath, err := lanecompletion.OperationIntentPathE(caseRoot, commit.Sequence)
		if err != nil {
			return lanecompletion.OperationIntent{}, lanecompletion.OperationCommit{}, false, err
		}
		intent, err := lanecompletion.ReadOperationIntent(caseRoot, intentPath)
		if err != nil {
			return lanecompletion.OperationIntent{}, lanecompletion.OperationCommit{}, false, err
		}
		if intent.OperationID != commit.OperationID || intent.Sequence != commit.Sequence || !strings.EqualFold(intent.PreviewSHA256, commit.PreviewSHA256) {
			return lanecompletion.OperationIntent{}, lanecompletion.OperationCommit{}, false, fmt.Errorf("committed reopen operation intent differs from its commit: sequence=%d", commit.Sequence)
		}
		if intent.RequestedSelector != selector || intent.Actor != actor || intent.Reason != reason || !equalStrings(intent.EvidenceRefs, evidenceRefs) || !strings.EqualFold(intent.ExactPublicationSHA256, expectedPreviewSHA256) {
			continue
		}
		return intent, commit, true, nil
	}
	return lanecompletion.OperationIntent{}, lanecompletion.OperationCommit{}, false, nil
}

func (ctx *reopenContext) matchCommittedOperation() (bool, error) {
	for index := len(ctx.operation.Commits) - 1; index >= 0; index-- {
		commit := ctx.operation.Commits[index]
		intentPath, err := lanecompletion.OperationIntentPathE(ctx.inst.CaseRoot, commit.Sequence)
		if err != nil {
			return false, err
		}
		intent, err := lanecompletion.ReadOperationIntent(ctx.inst.CaseRoot, intentPath)
		if err != nil {
			return false, err
		}
		if intent.RequestedLane != ctx.requested.ID || intent.RequestedSelector != ctx.selector || intent.Actor != ctx.actor || intent.Reason != ctx.reason || !equalStrings(intent.EvidenceRefs, ctx.evidenceRefs) || !equalLifecycleEvidence(intent.Evidence, toLifecycleEvidence(ctx.evidence)) {
			continue
		}
		if err := ctx.loadCommittedOperation(intent, commit); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (ctx *reopenContext) loadCommittedOperation(intent lanecompletion.OperationIntent, commit lanecompletion.OperationCommit) error {
	if intent.OperationID != commit.OperationID || intent.Sequence != commit.Sequence || intent.RequestedLane != commit.RequestedLane || intent.Actor != commit.Actor || intent.Reason != commit.Reason || !strings.EqualFold(intent.PreviewSHA256, commit.PreviewSHA256) {
		return fmt.Errorf("committed reopen operation intent differs from its commit: sequence=%d", commit.Sequence)
	}
	if err := validateCommittedReopenCurrent(ctx.inst.CaseRoot, intent, commit); err != nil {
		return err
	}
	ctx.sequence, ctx.operationID, ctx.intent = intent.Sequence, intent.OperationID, &intent
	ctx.publicationStamp, ctx.exactPublicationSHA256 = intent.CreatedAt, intent.ExactPublicationSHA256
	ctx.operationPublications = append([]lanecompletion.OperationPublication{}, intent.Publications...)
	if err := ctx.resolveOperationPaths(); err != nil {
		return err
	}
	ctx.replayCommit = &commit
	ctx.targets = nil
	for _, target := range intent.Targets {
		lanePublication, ok := publicationByRole(target.Publications, "lane")
		if !ok {
			return fmt.Errorf("committed reopen target lacks exact lane publication: %s", target.Lane)
		}
		var lane Lane
		if err := json.Unmarshal(lanePublication.Bytes, &lane); err != nil || lane.ID != target.Lane {
			return fmt.Errorf("committed reopen target lane publication is invalid: %s", target.Lane)
		}
		ctx.targets = append(ctx.targets, reopenTargetContext{lane: lane, sequence: target.Sequence, previousReceiptSHA256: target.PreviousReceiptSHA, supersededSequence: target.SupersededCompletionSequence, reason: target.Reason, intentPath: filepath.Join(ctx.inst.CaseRoot, filepath.FromSlash(target.IntentPath)), receiptPath: filepath.Join(ctx.inst.CaseRoot, filepath.FromSlash(target.ReceiptPath)), publications: append([]lanecompletion.OperationPublication{}, target.Publications...)})
	}
	return nil
}

func validateCommittedReopenCurrent(caseRoot string, intent lanecompletion.OperationIntent, commit lanecompletion.OperationCommit) error {
	if len(intent.Targets) != len(commit.Targets) || len(intent.Targets) == 0 {
		return fmt.Errorf("committed reopen operation target count changed: sequence=%d", commit.Sequence)
	}
	b, err := readBoard(caseRoot)
	if err != nil {
		return err
	}
	for index, target := range intent.Targets {
		committed := commit.Targets[index]
		if target.Lane != committed.Lane || target.Sequence != committed.Sequence || !strings.EqualFold(target.PreviousReceiptSHA, committed.PreviousReceiptSHA) {
			return fmt.Errorf("committed reopen operation target differs from its intent: sequence=%d lane=%s", commit.Sequence, target.Lane)
		}
		lifecycle, err := lanecompletion.Inspect(caseRoot, target.Lane)
		if err != nil {
			return err
		}
		if lifecycle.State != lanecompletion.StateReopened || lifecycle.CurrentReopen == nil || lifecycle.CurrentReopen.OperationID != commit.OperationID || lifecycle.CurrentReopen.Sequence != target.Sequence || !strings.EqualFold(lifecycle.CurrentReopen.PreviousReceiptSHA, target.PreviousReceiptSHA) {
			return fmt.Errorf("committed reopen operation target is no longer current: %s", target.Lane)
		}
		lane, ok := mission.LookupBoardLane(b.Lanes, target.Lane, false)
		if !ok || !strings.EqualFold(strings.TrimSpace(lane.Status), "open") || lane.ExecutorGeneration != lifecycle.CurrentReopen.ResultingExecutorGeneration || strings.TrimSpace(lane.CurrentExecutor) != "" {
			return fmt.Errorf("committed reopen operation differs from the current board lane: %s", target.Lane)
		}
		lanePath, err := projectstate.Join(caseRoot, "lanes", target.Lane, "lane.json")
		if err != nil {
			return err
		}
		laneBytes, err := lanecompletion.ReadCaseFile(caseRoot, lanePath)
		if err != nil || !strings.EqualFold(lanecompletion.SHA256Bytes(laneBytes), lifecycle.CurrentReopen.LaneSHA256) {
			return fmt.Errorf("committed reopen operation lane projection changed: %s", target.Lane)
		}
		boardLaneSHA, err := boardLaneSHAFor(b, target.Lane)
		if err != nil || !strings.EqualFold(boardLaneSHA, lifecycle.CurrentReopen.BoardLaneSHA256) {
			return fmt.Errorf("committed reopen operation board projection changed: %s", target.Lane)
		}
		resumePath, err := projectstate.Join(caseRoot, "lanes", target.Lane, "prompts", "RESUME.md")
		if err != nil {
			return err
		}
		resumeBytes, err := lanecompletion.ReadCaseFile(caseRoot, resumePath)
		if err != nil || !strings.EqualFold(lanecompletion.SHA256Bytes(resumeBytes), lifecycle.CurrentReopen.ResumeSHA256) {
			return fmt.Errorf("committed reopen operation resume projection changed: %s", target.Lane)
		}
		checkpointPath, err := projectstate.Join(caseRoot, "lanes", target.Lane, "checkpoints", "latest.json")
		if err != nil {
			return err
		}
		checkpointBytes, err := lanecompletion.ReadCaseFile(caseRoot, checkpointPath)
		if err != nil || !strings.EqualFold(lanecompletion.SHA256Bytes(checkpointBytes), lifecycle.CurrentReopen.CheckpointSHA256) {
			return fmt.Errorf("committed reopen operation checkpoint projection changed: %s", target.Lane)
		}
	}
	return nil
}

func committedReplayWrites(ctx reopenContext) []StartWrite {
	writes := []StartWrite{}
	for _, target := range ctx.targets {
		for _, publication := range target.publications {
			writes = append(writes, StartWrite{Path: publication.Path, Kind: publication.Role, Action: "already-committed", TargetPath: filepath.Join(ctx.inst.CaseRoot, filepath.FromSlash(publication.Path))})
		}
	}
	for _, publication := range ctx.operationPublications {
		writes = append(writes, StartWrite{Path: publication.Path, Kind: publication.Role, Action: "already-committed", TargetPath: filepath.Join(ctx.inst.CaseRoot, filepath.FromSlash(publication.Path))})
	}
	return writes
}

func callReopenPublicationHook(stage, lane string) error {
	if reopenPublicationHook == nil {
		return nil
	}
	return reopenPublicationHook(stage, lane)
}

func reopenPublicationStage(role string) string {
	switch role {
	case "lane-reopen-intent":
		return "intent"
	case "lane-event":
		return "event"
	case "lane":
		return "lane"
	case "board":
		return "board"
	case "lane-resume", "lane-checkpoint":
		return "resume"
	case "lane-reopen-commit":
		return "receipt"
	default:
		return role
	}
}

func publicationByRole(items []lanecompletion.OperationPublication, role string) (lanecompletion.OperationPublication, bool) {
	for _, item := range items {
		if item.Role == role {
			return item, true
		}
	}
	return lanecompletion.OperationPublication{}, false
}

func reopenApplyArgs(ctx reopenContext, hash string) []string {
	return []string{
		"-Command", commands.Reopen,
		ctx.selector,
		"-Target", ctx.inst.CaseRoot,
		"-Pack", ctx.manifest.Pack,
		"-Actor", ctx.actor,
		"-Reason", ctx.reason,
		"-EvidenceRefs", strings.Join(ctx.evidenceRefs, ","),
		"-ReopenPublicationStamp", ctx.publicationStamp,
		"-ExpectedReopenPlanSha256", hash,
		"-Apply", "-Format", "json",
	}
}

func reopenApplyCommandForArgs(args []string) string {
	parts := append([]string{"/rekit", commands.Reopen}, args[2:]...)
	for index := range parts {
		parts[index] = quoteCompleteCommandArg(parts[index])
	}
	return strings.Join(parts, " ")
}

func toLifecycleEvidence(items []CompletionEvidence) []lanecompletion.Evidence {
	out := make([]lanecompletion.Evidence, 0, len(items))
	for _, item := range items {
		out = append(out, lanecompletion.Evidence{Ref: item.Ref, SHA256: item.SHA256, Bytes: item.Bytes})
	}
	return out
}

func fromLifecycleEvidence(items []lanecompletion.Evidence) []CompletionEvidence {
	out := make([]CompletionEvidence, 0, len(items))
	for _, item := range items {
		out = append(out, CompletionEvidence{Ref: item.Ref, SHA256: item.SHA256, Bytes: item.Bytes})
	}
	return out
}
func equalLifecycleEvidence(left, right []lanecompletion.Evidence) bool {
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
func safeCaseRelativePath(caseRoot, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("reopen target path must be case-relative: %s", rel)
	}
	path := filepath.Clean(filepath.Join(caseRoot, filepath.FromSlash(rel)))
	relative, err := filepath.Rel(filepath.Clean(caseRoot), path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("reopen target path escapes case root: %s", rel)
	}
	return path, nil
}

func writeOrValidateExclusive(caseRoot, path string, value any) error {
	err := writeCompletionExclusive(caseRoot, path, value)
	if err == nil {
		return nil
	}
	if !os.IsExist(err) {
		return err
	}
	var existing any
	data, readErr := lanecompletion.ReadExactJSON(caseRoot, path, &existing)
	if readErr != nil {
		return readErr
	}
	expected, marshalErr := json.MarshalIndent(value, "", "  ")
	if marshalErr != nil {
		return marshalErr
	}
	expected = append(expected, '\n')
	if string(data) != string(expected) {
		return fmt.Errorf("existing publication differs from exact recovery bytes: %s", path)
	}
	return nil
}
