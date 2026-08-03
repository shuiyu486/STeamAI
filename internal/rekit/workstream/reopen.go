package workstream

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanecompletion"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

var (
	reopenAfterOperationIntentHook func() error
	reopenPublicationHook          func(stage, lane string) error
)

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
	stamp := strings.TrimSpace(opt.PublicationStamp)
	if stamp == "" {
		stamp = isoNow()
	}
	if err := ctx.buildPublicationPlan(stamp); err != nil {
		return ReopenResult{}, err
	}
	result := ctx.result(false, false)
	result.PublicationStamp = stamp
	result.WouldWrites = ctx.plannedWrites()
	result.ReopenPlanSHA256 = ctx.exactPublicationSHA256
	result.ApplyCommand = reopenApplyCommand(ctx, ctx.exactPublicationSHA256)
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
	expected := strings.ToLower(strings.TrimSpace(opt.ExpectedPreviewSHA256))
	if expected == "" {
		return ReopenResult{}, fmt.Errorf("reopen apply requires -ExpectedReopenPlanSha256 from the reviewed preview")
	}
	if ctx.replayCommit != nil {
		commit := *ctx.replayCommit
		if !strings.EqualFold(expected, commit.PreviewSHA256) {
			return ReopenResult{}, fmt.Errorf("latest committed reopen operation conflicts with requested preview hash")
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
		if !strings.EqualFold(expected, ctx.exactPublicationSHA256) {
			return ReopenResult{}, fmt.Errorf("reopen preview sha256 mismatch: got %s want %s", expected, ctx.exactPublicationSHA256)
		}
		intent := ctx.newOperationIntent(expected, stamp)
		mutationStarted = true
		if err := writeCompletionExclusive(ctx.inst.CaseRoot, lanecompletion.OperationIntentPath(ctx.inst.CaseRoot, ctx.sequence), intent); err != nil {
			return ReopenResult{}, err
		}
		ctx.intent = &intent
		if reopenAfterOperationIntentHook != nil {
			if err := reopenAfterOperationIntentHook(); err != nil {
				return ReopenResult{}, err
			}
		}
	} else {
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
	if allowPending && !operations.Pending && operations.LatestIntent != nil && len(operations.Commits) > 0 {
		intent := operations.LatestIntent
		requestedRefs := mission.UniqueStrings(strings.FieldsFunc(opt.EvidenceRefs, func(r rune) bool { return r == ',' || r == ';' || r == '\n' || r == '\r' }))
		if intent.RequestedSelector == selector && intent.Actor == actor && intent.Reason == reason && equalStrings(intent.EvidenceRefs, requestedRefs) && strings.EqualFold(intent.ExactPublicationSHA256, strings.TrimSpace(opt.ExpectedPreviewSHA256)) {
			requested, err := requestedLaneFromOperation(*intent)
			if err != nil {
				return reopenContext{}, err
			}
			facts, _ := mission.ReadStrictLedgerFacts(inst.CaseRoot)
			ctx := reopenContext{inst: inst, manifest: m, facts: facts, selector: selector, requested: requested, actor: actor, reason: reason, evidenceRefs: append([]string{}, intent.EvidenceRefs...), requestedEvidenceRefs: opt.EvidenceRefs, evidence: fromLifecycleEvidence(intent.Evidence), operation: operations}
			if replayed, err := ctx.matchCommittedOperation(); err != nil {
				return reopenContext{}, err
			} else if !replayed {
				return reopenContext{}, fmt.Errorf("latest committed reopen operation conflicts with requested exact replay")
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
		facts, _ := mission.ReadStrictLedgerFacts(inst.CaseRoot)
		ctx := reopenContext{inst: inst, manifest: m, facts: facts, selector: selector, requested: requested, actor: actor, reason: reason, evidenceRefs: append([]string{}, intent.EvidenceRefs...), requestedEvidenceRefs: opt.EvidenceRefs, evidence: fromLifecycleEvidence(intent.Evidence), operation: operations}
		ctx.sequence, ctx.operationID, ctx.intent = intent.Sequence, intent.OperationID, &intent
		ctx.publicationStamp, ctx.exactPublicationSHA256 = intent.CreatedAt, intent.ExactPublicationSHA256
		ctx.operationPublications = append([]lanecompletion.OperationPublication{}, intent.Publications...)
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
	ctx := reopenContext{inst: inst, manifest: m, board: b, facts: facts, selector: selector, requested: requested, actor: actor, reason: reason, evidenceRefs: refs, requestedEvidenceRefs: opt.EvidenceRefs, evidence: evidence, operation: operations}
	if replayed, err := ctx.matchCommittedOperation(); err != nil {
		return reopenContext{}, err
	} else if replayed {
		return ctx, nil
	}
	ctx.sequence = operations.LatestSequence + 1
	ctx.operationID = eventID(requested.ID, "lane-reopen-operation", fmt.Sprintf("%d:%s:%s", ctx.sequence, actor, reason))
	if err := ctx.buildTargets(); err != nil {
		return reopenContext{}, err
	}
	return ctx, nil
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
		ctx.targets = append(ctx.targets, reopenTargetContext{lane: lane, lifecycle: lifecycle, sequence: lifecycle.HeadSequence + 1, previousReceiptSHA256: lifecycle.HeadReceiptSHA256, supersededSequence: lifecycle.HeadSequence, reason: reason, intentPath: lanecompletion.IntentPath(ctx.inst.CaseRoot, lane.ID, lifecycle.HeadSequence+1, "reopen"), receiptPath: lanecompletion.ReceiptPath(ctx.inst.CaseRoot, lane.ID, lifecycle.HeadSequence+1, "reopen")})
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

func (ctx reopenContext) plannedWrites() []StartWrite {
	writes := []StartWrite{{Path: relativePath(ctx.inst.CaseRoot, lanecompletion.OperationIntentPath(ctx.inst.CaseRoot, ctx.sequence)), Kind: "lane-reopen-operation-intent", Action: "would-create", TargetPath: lanecompletion.OperationIntentPath(ctx.inst.CaseRoot, ctx.sequence)}}
	for _, target := range ctx.targets {
		laneRoot, _ := laneRootPath(ctx.inst.CaseRoot, target.lane)
		writes = append(writes,
			StartWrite{Path: relativePath(ctx.inst.CaseRoot, target.intentPath), Kind: "lane-reopen-intent", Action: "would-create", TargetPath: target.intentPath},
			StartWrite{Path: relativePath(ctx.inst.CaseRoot, LaneEventsJSONLPath(laneRoot)), Kind: "lane-event", Action: "would-append-lane-reopened", TargetPath: LaneEventsJSONLPath(laneRoot)},
			StartWrite{Path: relativePath(ctx.inst.CaseRoot, filepath.Join(laneRoot, "lane.json")), Kind: "lane", Action: "would-reopen", TargetPath: filepath.Join(laneRoot, "lane.json")},
			StartWrite{Path: relativePath(ctx.inst.CaseRoot, filepath.Join(laneRoot, "prompts", "RESUME.md")), Kind: "lane-resume", Action: "would-refresh", TargetPath: filepath.Join(laneRoot, "prompts", "RESUME.md")},
			StartWrite{Path: relativePath(ctx.inst.CaseRoot, filepath.Join(laneRoot, "checkpoints", "latest.json")), Kind: "lane-checkpoint", Action: "would-refresh", TargetPath: filepath.Join(laneRoot, "checkpoints", "latest.json")},
			StartWrite{Path: relativePath(ctx.inst.CaseRoot, target.receiptPath), Kind: "lane-reopen-commit", Action: "would-create", TargetPath: target.receiptPath},
		)
	}
	writes = append(writes, StartWrite{Path: ".rekit/board.json", Kind: "board", Action: "would-refresh"}, StartWrite{Path: relativePath(ctx.inst.CaseRoot, lanecompletion.OperationCommitPath(ctx.inst.CaseRoot, ctx.sequence)), Kind: "lane-reopen-operation-commit", Action: "would-create-last", TargetPath: lanecompletion.OperationCommitPath(ctx.inst.CaseRoot, ctx.sequence)})
	return writes
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
	writes := []StartWrite{{Path: relativePath(ctx.inst.CaseRoot, lanecompletion.OperationIntentPath(ctx.inst.CaseRoot, ctx.sequence)), Kind: "lane-reopen-operation-intent", Action: "existing", TargetPath: lanecompletion.OperationIntentPath(ctx.inst.CaseRoot, ctx.sequence)}}
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
	commitPath := lanecompletion.OperationCommitPath(ctx.inst.CaseRoot, ctx.sequence)
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

func (ctx *reopenContext) matchCommittedOperation() (bool, error) {
	if ctx.operation.LatestIntent == nil || len(ctx.operation.Commits) == 0 {
		return false, nil
	}
	intent := ctx.operation.LatestIntent
	commit := ctx.operation.Commits[len(ctx.operation.Commits)-1]
	if intent.RequestedLane != ctx.requested.ID || intent.RequestedSelector != ctx.selector || intent.Actor != ctx.actor || intent.Reason != ctx.reason || !equalStrings(intent.EvidenceRefs, ctx.evidenceRefs) || !equalLifecycleEvidence(intent.Evidence, toLifecycleEvidence(ctx.evidence)) {
		return false, nil
	}
	ctx.sequence, ctx.operationID, ctx.intent = intent.Sequence, intent.OperationID, intent
	ctx.publicationStamp, ctx.exactPublicationSHA256 = intent.CreatedAt, intent.ExactPublicationSHA256
	ctx.operationPublications = append([]lanecompletion.OperationPublication{}, intent.Publications...)
	ctx.replayCommit = &commit
	for _, target := range intent.Targets {
		lanePublication, ok := publicationByRole(target.Publications, "lane")
		if !ok {
			return false, fmt.Errorf("committed reopen target lacks exact lane publication: %s", target.Lane)
		}
		var lane Lane
		if err := json.Unmarshal(lanePublication.Bytes, &lane); err != nil {
			return false, fmt.Errorf("committed reopen target lane publication is invalid: %s", target.Lane)
		}
		ctx.targets = append(ctx.targets, reopenTargetContext{lane: lane, sequence: target.Sequence, previousReceiptSHA256: target.PreviousReceiptSHA, supersededSequence: target.SupersededCompletionSequence, reason: target.Reason, intentPath: filepath.Join(ctx.inst.CaseRoot, filepath.FromSlash(target.IntentPath)), receiptPath: filepath.Join(ctx.inst.CaseRoot, filepath.FromSlash(target.ReceiptPath)), publications: append([]lanecompletion.OperationPublication{}, target.Publications...)})
	}
	return true, nil
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

func reopenApplyCommand(ctx reopenContext, hash string) string {
	args := []string{"/rekit", "reopen", workstreamLabel(ctx.requested), "-Actor", ctx.actor, "-Reason", ctx.reason, "-EvidenceRefs", strings.Join(ctx.evidenceRefs, ","), "-ReopenPublicationStamp", ctx.publicationStamp, "-ExpectedReopenPlanSha256", hash, "-Apply", "-Format", "json"}
	for i := range args {
		args[i] = quoteCompleteCommandArg(args[i])
	}
	return strings.Join(args, " ")
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
