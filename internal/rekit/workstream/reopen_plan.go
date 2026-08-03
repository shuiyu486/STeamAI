package workstream

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/lanecompletion"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

const reopenPlanHashMarker = "<reopen-exact-publication-plan-sha256>"

type reopenPublicationIdentity struct {
	SchemaVersion     int                                   `json:"schemaVersion"`
	OperationID       string                                `json:"operationId"`
	OperationSequence int                                   `json:"operationSequence"`
	RequestedLane     string                                `json:"requestedLane"`
	RequestedSelector string                                `json:"requestedSelector"`
	Actor             string                                `json:"actor"`
	Reason            string                                `json:"reason"`
	EvidenceRefs      []string                              `json:"evidenceRefs"`
	Evidence          []lanecompletion.Evidence             `json:"evidence"`
	PublicationStamp  string                                `json:"publicationStamp"`
	Targets           []lanecompletion.OperationTarget      `json:"targets"`
	Publications      []lanecompletion.OperationPublication `json:"publications"`
}

func (ctx *reopenContext) buildPublicationPlan(stamp string) error {
	stamp = strings.TrimSpace(stamp)
	if stamp == "" {
		return fmt.Errorf("reopen publication plan requires a stable publication stamp")
	}
	ctx.publicationStamp = stamp
	for index := range ctx.targets {
		target := &ctx.targets[index]
		laneIntent := lanecompletion.ReopenIntent{
			SchemaVersion: 1, Kind: "lane-reopen-intent", OperationID: ctx.operationID,
			Sequence: target.sequence, PreviousReceiptSHA: target.previousReceiptSHA256,
			SupersededCompletionSequence: target.supersededSequence,
			SupersededCompletionSHA256:   target.previousReceiptSHA256,
			Lane:                         target.lane.ID, Label: workstreamLabel(target.lane), Authority: target.lane.Authority,
			PreviousStatus: target.lane.Status, Actor: ctx.actor, Reason: ctx.reason,
			EvidenceRefs: append([]string{}, ctx.evidenceRefs...), Evidence: toLifecycleEvidence(ctx.evidence),
			PreviousExecutor:            target.lane.CurrentExecutor,
			PreviousExecutorGeneration:  target.lane.ExecutorGeneration,
			ResultingExecutorGeneration: target.lane.ExecutorGeneration + 1,
			CreatedAt:                   stamp, EventID: eventID(target.lane.ID, "lane-reopened", stamp+ctx.operationID),
			PreviewSHA256: reopenPlanHashMarker,
			NoAuthority:   true, NoConfirmed: true, NoHeavyTool: true, NoAutoResume: true,
		}
		target.laneIntent = &laneIntent
		intentBytes, err := indentedJSON(laneIntent)
		if err != nil {
			return err
		}
		intentPublication, err := buildReopenPublication(ctx.inst.CaseRoot, target.intentPath, "lane-reopen-intent", lanecompletion.PublicationCreateExclusive, intentBytes)
		if err != nil {
			return err
		}
		laneRoot, err := laneRootPath(ctx.inst.CaseRoot, target.lane)
		if err != nil {
			return err
		}
		event := map[string]any{
			"schemaVersion": 1, "eventId": laneIntent.EventID, "kind": "lane-reopened", "lane": laneIntent.Lane,
			"time": laneIntent.CreatedAt, "summary": "operational lane completion superseded: " + laneIntent.Label,
			"actor": laneIntent.Actor, "reason": laneIntent.Reason, "operationId": laneIntent.OperationID,
			"sequence": laneIntent.Sequence, "supersededCompletionSha256": laneIntent.SupersededCompletionSHA256,
			"previewSha256": reopenPlanHashMarker, "noAuthority": true, "noConfirmed": true,
			"noHeavyTool": true, "noAutoResume": true,
		}
		eventPath := LaneEventsJSONLPath(laneRoot)
		eventBefore, exists, err := readPublicationBefore(ctx.inst.CaseRoot, eventPath)
		if err != nil {
			return err
		}
		items, err := mission.ReadStrictJSONLineObjects(eventPath)
		if err != nil {
			return err
		}
		for _, item := range items {
			if mission.Value(item, "eventId") == laneIntent.EventID {
				return fmt.Errorf("fresh reopen publication event already exists: %s", laneIntent.EventID)
			}
		}
		eventLine, err := json.Marshal(event)
		if err != nil {
			return err
		}
		eventAfter := append(append(append([]byte{}, eventBefore...), eventLine...), '\r', '\n')
		eventPublication := newReopenPublication(ctx.inst.CaseRoot, eventPath, "lane-event", lanecompletion.PublicationReplaceExact, exists, eventBefore, eventAfter)

		reopened := target.lane
		reopened.Status, reopened.CurrentExecutor, reopened.ExecutorGeneration, reopened.UpdatedAt = "open", "", laneIntent.ResultingExecutorGeneration, stamp
		laneBytes, err := indentedJSON(reopened)
		if err != nil {
			return err
		}
		lanePublication, err := buildReopenPublication(ctx.inst.CaseRoot, filepath.Join(laneRoot, "lane.json"), "lane", lanecompletion.PublicationReplaceExact, laneBytes)
		if err != nil {
			return err
		}
		target.lane = reopened
		target.publications = []lanecompletion.OperationPublication{intentPublication, eventPublication, lanePublication}
	}

	boardAfter := ctx.board
	boardAfter.UpdatedAt = stamp
	for laneIndex := range boardAfter.Lanes {
		for _, target := range ctx.targets {
			if boardAfter.Lanes[laneIndex].ID != target.lane.ID {
				continue
			}
			lane := target.lane
			boardAfter.Lanes[laneIndex] = boardLane{ID: lane.ID, Type: lane.Type, Title: lane.Title, Status: lane.Status, Authority: lane.Authority, Workspace: lane.Workspace, CurrentExecutor: lane.CurrentExecutor, ExecutorGeneration: lane.ExecutorGeneration, LastTakeoverAt: lane.LastTakeoverAt, LastTakeoverBy: lane.LastTakeoverBy, LastTakeoverReason: lane.LastTakeoverReason, UpdatedAt: lane.UpdatedAt}
		}
	}
	boardBytes, err := indentedJSON(boardAfter)
	if err != nil {
		return err
	}
	boardPath := filepath.Join(ctx.inst.CaseRoot, ".rekit", "board.json")
	boardPublication, err := buildReopenPublication(ctx.inst.CaseRoot, boardPath, "board", lanecompletion.PublicationReplaceExact, boardBytes)
	if err != nil {
		return err
	}

	for index := range ctx.targets {
		target := &ctx.targets[index]
		resume, err := buildLaneResumePublication(ctx.inst.CaseRoot, ctx.manifest, target.lane, stamp)
		if err != nil {
			return err
		}
		resumePublication, err := buildReopenPublication(ctx.inst.CaseRoot, resume.ResumePath, "lane-resume", lanecompletion.PublicationReplaceExact, resume.ResumeBytes)
		if err != nil {
			return err
		}
		checkpointPublication, err := buildReopenPublication(ctx.inst.CaseRoot, resume.CheckpointPath, "lane-checkpoint", lanecompletion.PublicationReplaceExact, resume.CheckpointBytes)
		if err != nil {
			return err
		}
		intentSHA, err := lanecompletion.CanonicalSHA256(*target.laneIntent)
		if err != nil {
			return err
		}
		boardLaneSHA, err := boardLaneSHAFor(boardAfter, target.lane.ID)
		if err != nil {
			return err
		}
		receipt := lanecompletion.ReopenReceipt{
			SchemaVersion: 1, Kind: "lane-reopen", State: "committed", OperationID: ctx.operationID,
			Sequence: target.sequence, PreviousReceiptSHA: target.laneIntent.PreviousReceiptSHA,
			SupersededCompletionSequence: target.laneIntent.SupersededCompletionSequence,
			SupersededCompletionSHA256:   target.laneIntent.SupersededCompletionSHA256,
			Lane:                         target.laneIntent.Lane, Label: target.laneIntent.Label, Authority: target.laneIntent.Authority,
			PreviousStatus: target.laneIntent.PreviousStatus, Actor: target.laneIntent.Actor, Reason: target.laneIntent.Reason,
			EvidenceRefs: target.laneIntent.EvidenceRefs, Evidence: target.laneIntent.Evidence,
			PreviousExecutor:            target.laneIntent.PreviousExecutor,
			PreviousExecutorGeneration:  target.laneIntent.PreviousExecutorGeneration,
			ResultingExecutorGeneration: target.laneIntent.ResultingExecutorGeneration,
			ReopenedAt:                  stamp, EventID: target.laneIntent.EventID, PreviewSHA256: reopenPlanHashMarker,
			IntentSHA256: intentSHA, LaneSHA256: lanecompletion.SHA256Bytes(lanePublicationBytes(target.publications)),
			BoardLaneSHA256: boardLaneSHA, ResumeSHA256: lanecompletion.SHA256Bytes(resume.ResumeBytes),
			CheckpointSHA256: lanecompletion.SHA256Bytes(resume.CheckpointBytes),
			NoAuthority:      true, NoConfirmed: true, NoHeavyTool: true, NoAutoResume: true,
		}
		receiptBytes, err := indentedJSON(receipt)
		if err != nil {
			return err
		}
		receiptPublication, err := buildReopenPublication(ctx.inst.CaseRoot, target.receiptPath, "lane-reopen-commit", lanecompletion.PublicationCreateExclusive, receiptBytes)
		if err != nil {
			return err
		}
		target.publications = append(target.publications, resumePublication, checkpointPublication, receiptPublication)
	}

	identity := ctx.publicationIdentity([]lanecompletion.OperationPublication{boardPublication})
	identityIntent := lanecompletion.OperationIntent{SchemaVersion: 1, Kind: "lane-reopen-operation-intent", OperationID: identity.OperationID, Sequence: identity.OperationSequence, RequestedLane: identity.RequestedLane, RequestedSelector: identity.RequestedSelector, Actor: identity.Actor, Reason: identity.Reason, EvidenceRefs: identity.EvidenceRefs, Evidence: identity.Evidence, Targets: identity.Targets, Publications: identity.Publications, CreatedAt: identity.PublicationStamp, NoAuthority: true, NoConfirmed: true, NoHeavyTool: true, NoAutoResume: true}
	planSHA, err := lanecompletion.ExactPublicationSHA256(identityIntent)
	if err != nil {
		return err
	}
	for targetIndex := range ctx.targets {
		for publicationIndex := range ctx.targets[targetIndex].publications {
			replaceReopenPlanMarker(&ctx.targets[targetIndex].publications[publicationIndex], planSHA)
		}
		var intent lanecompletion.ReopenIntent
		if err := json.Unmarshal(ctx.targets[targetIndex].publications[0].Bytes, &intent); err != nil {
			return err
		}
		ctx.targets[targetIndex].laneIntent = &intent
		intentSHA, err := lanecompletion.CanonicalSHA256(intent)
		if err != nil {
			return err
		}
		for publicationIndex := range ctx.targets[targetIndex].publications {
			publication := &ctx.targets[targetIndex].publications[publicationIndex]
			if publication.Role != "lane-reopen-commit" {
				continue
			}
			var receipt lanecompletion.ReopenReceipt
			if err := json.Unmarshal(publication.Bytes, &receipt); err != nil {
				return err
			}
			receipt.IntentSHA256 = intentSHA
			publication.Bytes, err = indentedJSON(receipt)
			if err != nil {
				return err
			}
			publication.AfterSHA256 = lanecompletion.SHA256Bytes(publication.Bytes)
		}
	}
	replaceReopenPlanMarker(&boardPublication, planSHA)
	ctx.operationPublications = []lanecompletion.OperationPublication{boardPublication}
	ctx.exactPublicationSHA256 = planSHA
	return nil
}

func (ctx reopenContext) publicationIdentity(publications []lanecompletion.OperationPublication) reopenPublicationIdentity {
	targets := make([]lanecompletion.OperationTarget, 0, len(ctx.targets))
	for _, target := range ctx.targets {
		targets = append(targets, lanecompletion.OperationTarget{
			Lane: target.lane.ID, Sequence: target.sequence, PreviousReceiptSHA: target.previousReceiptSHA256,
			SupersededCompletionSequence: target.supersededSequence,
			IntentPath:                   relativePath(ctx.inst.CaseRoot, target.intentPath), ReceiptPath: relativePath(ctx.inst.CaseRoot, target.receiptPath),
			Reason: target.reason, Publications: append([]lanecompletion.OperationPublication{}, target.publications...),
		})
	}
	return reopenPublicationIdentity{SchemaVersion: 1, OperationID: ctx.operationID, OperationSequence: ctx.sequence,
		RequestedLane: ctx.requested.ID, RequestedSelector: ctx.selector, Actor: ctx.actor, Reason: ctx.reason,
		EvidenceRefs: append([]string{}, ctx.evidenceRefs...), Evidence: toLifecycleEvidence(ctx.evidence),
		PublicationStamp: ctx.publicationStamp, Targets: targets, Publications: publications}
}

func replaceReopenPlanMarker(publication *lanecompletion.OperationPublication, planSHA string) {
	publication.Bytes = []byte(strings.ReplaceAll(string(publication.Bytes), reopenPlanHashMarker, planSHA))
	publication.AfterSHA256 = lanecompletion.SHA256Bytes(publication.Bytes)
}

func buildReopenPublication(caseRoot, path, role string, mode lanecompletion.PublicationMode, after []byte) (lanecompletion.OperationPublication, error) {
	before, exists, err := readPublicationBefore(caseRoot, path)
	if err != nil {
		return lanecompletion.OperationPublication{}, err
	}
	if mode == lanecompletion.PublicationCreateExclusive && exists {
		return lanecompletion.OperationPublication{}, fmt.Errorf("fresh reopen publication already exists: %s", path)
	}
	return newReopenPublication(caseRoot, path, role, mode, exists, before, after), nil
}

func newReopenPublication(caseRoot, path, role string, mode lanecompletion.PublicationMode, exists bool, before, after []byte) lanecompletion.OperationPublication {
	publication := lanecompletion.OperationPublication{Path: relativePath(caseRoot, path), Role: role, Mode: mode,
		BeforeExists: exists, AfterSHA256: lanecompletion.SHA256Bytes(after), Bytes: append([]byte{}, after...)}
	if exists {
		publication.BeforeSHA256 = lanecompletion.SHA256Bytes(before)
	}
	if !exists && mode == lanecompletion.PublicationReplaceExact {
		publication.Mode = lanecompletion.PublicationCreateExclusive
	}
	return publication
}

func readPublicationBefore(caseRoot, path string) ([]byte, bool, error) {
	data, err := lanecompletion.ReadCaseFile(caseRoot, path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func indentedJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func boardLaneSHAFor(board board, laneID string) (string, error) {
	for _, lane := range board.Lanes {
		if lane.ID == laneID {
			data, err := json.Marshal(lane)
			if err != nil {
				return "", err
			}
			return lanecompletion.SHA256Bytes(data), nil
		}
	}
	return "", fmt.Errorf("board omits reopen projection: %s", laneID)
}

func lanePublicationBytes(publications []lanecompletion.OperationPublication) []byte {
	for _, publication := range publications {
		if publication.Role == "lane" {
			return publication.Bytes
		}
	}
	return nil
}
