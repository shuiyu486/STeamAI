package note

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

const (
	maxListRows       = 20
	maxEventJSONBytes = 60 * 1024
)

type Options struct {
	Kind                            string
	Lane                            string
	Subject                         string
	Summary                         string
	Actor                           string
	Risk                            string
	Related                         string
	Confidence                      string
	Decision                        string
	Reason                          string
	Status                          string
	BatchID                         string
	Target                          string
	Verifier                        string
	Verdict                         string
	Action                          string
	ApprovedBy                      string
	Scope                           string
	Expires                         string
	EvidenceRefs                    string
	EventID                         string
	CreatedAt                       string
	ExpectedEventSHA256             string
	PacketID                        string
	RouteID                         string
	ShardID                         string
	PacketPath                      string
	ReviewerResultPath              string
	ReviewerSession                 string
	ReviewerHarness                 string
	ReviewerDispatchID              string
	ReviewerDispatchPath            string
	ReviewerDispatchSHA256          string
	ReviewerCompletionPath          string
	ReviewerCompletionSHA256        string
	ReviewerResultInputPath         string
	ReviewerResultInputSHA256       string
	ReviewerResultInputBytes        string
	ReviewerResultSHA256            string
	ReviewerManifestSHA256          string
	ReviewerVerificationID          string
	ReviewerDecisionID              string
	OwnerExecutor                   string
	OwnerGeneration                 string
	OwnerBindingMode                string
	OwnerBindingTarget              string
	ReviewerDecision                string
	RecommendedVerdict              string
	ReviewerRisks                   []string
	ReviewerConflicts               []string
	RouteOutput                     map[string]any
	ExpectedExecutionControlBinding *executioncontrol.Binding
}

type AppendResult struct {
	SchemaVersion                    int                                      `json:"schemaVersion"`
	Command                          string                                   `json:"command"`
	CaseRoot                         string                                   `json:"caseRoot"`
	RepoRoot                         string                                   `json:"repoRoot"`
	Pack                             string                                   `json:"pack"`
	IsMutation                       bool                                     `json:"isMutation"`
	Applied                          bool                                     `json:"applied"`
	EventID                          string                                   `json:"eventId"`
	Path                             string                                   `json:"path"`
	Reason                           string                                   `json:"reason,omitempty"`
	EventSHA256                      string                                   `json:"eventSha256"`
	ExpectedEventSHA256              string                                   `json:"expectedEventSha256,omitempty"`
	RecordCommand                    string                                   `json:"recordCommand,omitempty"`
	RecordArgs                       []string                                 `json:"recordArgs,omitempty"`
	Event                            map[string]any                           `json:"event"`
	MissionBrief                     mission.Brief                            `json:"missionBrief"`
	ExecutorAction                   mission.ExecutorAction                   `json:"executorAction"`
	MissionCommanderAction           mission.MissionCommanderAction           `json:"missionCommanderAction"`
	MissionCommanderNextActions      []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions,omitempty"`
	WouldExecutorAction              *mission.ExecutorAction                  `json:"wouldExecutorAction,omitempty"`
	WouldMissionCommanderAction      *mission.MissionCommanderAction          `json:"wouldMissionCommanderAction,omitempty"`
	WouldMissionCommanderNextActions []mission.MissionCommanderNextActionItem `json:"wouldMissionCommanderNextActions,omitempty"`
}

type ListResult struct {
	SchemaVersion int         `json:"schemaVersion"`
	Command       string      `json:"command"`
	CaseRoot      string      `json:"caseRoot"`
	RepoRoot      string      `json:"repoRoot"`
	Pack          string      `json:"pack"`
	IsMutation    bool        `json:"isMutation"`
	Kind          string      `json:"kind"`
	Lane          string      `json:"lane"`
	EventCount    int         `json:"eventCount"`
	Groups        []ListGroup `json:"groups"`
}

type ListGroup struct {
	Kind   string           `json:"kind"`
	Total  int              `json:"total"`
	Shown  int              `json:"shown"`
	Events []map[string]any `json:"events"`
}

type event map[string]any

var interventionBeforeLeaseHook func() error

func List(repoRoot, caseRoot, pack string, opt Options) (string, error) {
	result, err := ListEvents(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	for _, group := range result.Groups {
		fmt.Fprintf(&out, "[%s] (%d 条)\n", group.Kind, group.Total)
		for _, item := range group.Events {
			subject := firstText(stringValue(item, "subject"), stringValue(item, "kind"))
			fmt.Fprintf(&out, "- %s | lane=%s%s\n", subject, stringValue(item, "lane"), noteExtra(group.Kind, item))
		}
		if rest := group.Total - group.Shown; rest > 0 {
			fmt.Fprintf(&out, "- 另有 %d 条 %s\n", rest, group.Kind)
		}
		fmt.Fprintln(&out)
	}
	return out.String(), nil
}

func ListEvents(repoRoot, caseRoot, pack string, opt Options) (ListResult, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return ListResult{}, err
	}
	kind := strings.ToLower(strings.TrimSpace(opt.Kind))
	if kind != "" && !isValidKind(kind) {
		return ListResult{}, fmt.Errorf("invalid note kind: %s", opt.Kind)
	}
	kinds := mission.LedgerKinds()
	if kind != "" {
		kinds = []string{kind}
	}
	laneFilter := strings.TrimSpace(opt.Lane)
	result := ListResult{
		SchemaVersion: 1,
		Command:       "note",
		CaseRoot:      inst.CaseRoot,
		RepoRoot:      repoRoot,
		Pack:          pack,
		IsMutation:    false,
		Kind:          kind,
		Lane:          laneFilter,
		Groups:        []ListGroup{},
	}
	for _, k := range kinds {
		items, err := readFactEvents(inst.CaseRoot, k)
		if err != nil {
			return ListResult{}, err
		}
		if laneFilter != "" {
			filtered := []event{}
			for _, item := range items {
				if stringValue(item, "lane") == laneFilter {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}
		if len(items) == 0 {
			continue
		}
		shown := lastEvents(items, maxListRows)
		result.EventCount += len(items)
		result.Groups = append(result.Groups, ListGroup{Kind: k, Total: len(items), Shown: len(shown), Events: cloneEvents(shown)})
	}
	return result, nil
}

func Append(repoRoot, caseRoot, pack string, opt Options, whatIf bool) (result AppendResult, err error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return AppendResult{}, err
	}
	kind := strings.ToLower(strings.TrimSpace(opt.Kind))
	if kind == "" {
		return AppendResult{}, fmt.Errorf("note requires -Kind %s", strings.Join(mission.LedgerKinds(), "|"))
	}
	if !isValidKind(kind) {
		return AppendResult{}, fmt.Errorf("invalid note kind: %s", opt.Kind)
	}
	lane := strings.TrimSpace(opt.Lane)
	if lane == "" {
		return AppendResult{}, fmt.Errorf("note requires -Lane <lane id>")
	}
	if err := assertLane(inst.CaseRoot, lane); err != nil {
		return AppendResult{}, err
	}
	if err := validateAppendOptions(kind, opt); err != nil {
		return AppendResult{}, err
	}
	if err := lanemutation.AssertLaneOpen(inst.CaseRoot, lane, "note"); err != nil {
		return AppendResult{}, err
	}
	createdAt, err := noteCreatedAt(opt.CreatedAt)
	if err != nil {
		return AppendResult{}, err
	}
	event := buildEvent(kind, lane, createdAt, opt)
	eventID := strings.TrimSpace(opt.EventID)
	if eventID == "" {
		eventID = eventIDFor(event)
	}
	event["eventId"] = eventID
	encodedEvent, eventSHA256, err := encodeEvent(event)
	if err != nil {
		return AppendResult{}, err
	}
	if len(encodedEvent) > maxEventJSONBytes {
		return AppendResult{}, fmt.Errorf("note event exceeds %d-byte JSONL limit", maxEventJSONBytes)
	}
	relPath, _, err := mission.FactPath(inst.CaseRoot, kind)
	if err != nil {
		return AppendResult{}, err
	}
	brief, action, facts, boardLane, err := laneExecutorSnapshot(inst.CaseRoot, lane)
	if err != nil {
		return AppendResult{}, err
	}
	replayArgs, err := recordArgs(inst.CaseRoot, pack, event, eventSHA256, opt.ExpectedExecutionControlBinding)
	if err != nil {
		return AppendResult{}, err
	}
	result = AppendResult{
		SchemaVersion:               1,
		Command:                     "note",
		CaseRoot:                    inst.CaseRoot,
		RepoRoot:                    repoRoot,
		Pack:                        pack,
		IsMutation:                  !whatIf,
		Applied:                     false,
		EventID:                     eventID,
		Path:                        relPath,
		EventSHA256:                 eventSHA256,
		ExpectedEventSHA256:         strings.TrimSpace(opt.ExpectedEventSHA256),
		RecordCommand:               recordCommand(replayArgs),
		RecordArgs:                  replayArgs,
		Event:                       event,
		MissionBrief:                brief,
		ExecutorAction:              action,
		MissionCommanderAction:      action.MissionCommanderAction,
		MissionCommanderNextActions: noteMissionCommanderNextActions(boardLane, action),
	}
	if expected := strings.TrimSpace(opt.ExpectedEventSHA256); expected != "" {
		if len(expected) != 64 {
			return result, fmt.Errorf("invalid ExpectedNoteEventSha256 %q", expected)
		}
		if !strings.EqualFold(expected, eventSHA256) {
			return result, fmt.Errorf("note event sha256 changed after preview: expected %s got %s", expected, eventSHA256)
		}
	}
	exists, err := eventIDExists(inst.CaseRoot, eventID)
	if err != nil {
		return AppendResult{}, err
	}
	if exists && (whatIf || opt.ExpectedExecutionControlBinding == nil) {
		result.Reason = "duplicate eventId"
		return result, nil
	}
	if whatIf {
		wouldFacts := mission.FactsWithEvent(facts, kind, event)
		wouldBrief := mission.BuildWithOptions([]mission.Lane{{ID: boardLane.ID, Label: mission.BoardLaneLabel(boardLane), Status: boardLane.Status}}, mission.LaneFacts(wouldFacts, boardLane.ID), mission.BuildOptions{MaxRows: mission.DefaultMaxRows})
		wouldAction := mission.LaneExecutorAction(mission.Lane{ID: boardLane.ID, Label: mission.BoardLaneLabel(boardLane), Status: boardLane.Status}, wouldFacts, wouldBrief)
		result.WouldExecutorAction = &wouldAction
		result.WouldMissionCommanderAction = &wouldAction.MissionCommanderAction
		result.WouldMissionCommanderNextActions = noteMissionCommanderNextActions(boardLane, wouldAction)
		result.Reason = "what-if"
		return result, nil
	}
	var mutationLease *lanemutation.Lease
	if kind == "intervention" || opt.ExpectedExecutionControlBinding != nil {
		if kind == "intervention" && interventionBeforeLeaseHook != nil {
			if err := interventionBeforeLeaseHook(); err != nil {
				return AppendResult{}, err
			}
		}
		mutationLease, err = lanemutation.AcquireOpenLane(inst.CaseRoot, lane, "note")
		if err != nil {
			return AppendResult{}, err
		}
		defer func() {
			if unlockErr := mutationLease.Unlock(); unlockErr != nil {
				err = errors.Join(err, unlockErr)
				result = AppendResult{}
			}
		}()
		if err := mutationLease.Validate(); err != nil {
			return AppendResult{}, err
		}
		if opt.ExpectedExecutionControlBinding != nil {
			if err := executioncontrol.RequireCurrentBindingWithLease(inst.CaseRoot, mutationLease, *opt.ExpectedExecutionControlBinding); err != nil {
				return AppendResult{}, fmt.Errorf("note execution control binding is stale: %w", err)
			}
		}
		exists, err = eventIDExists(inst.CaseRoot, eventID)
		if err != nil {
			return AppendResult{}, err
		}
		if exists {
			result.Reason = "duplicate eventId"
			return result, nil
		}
	}
	if _, _, err := mission.AppendFact(inst.CaseRoot, kind, event); err != nil {
		return AppendResult{}, err
	}
	if mutationLease != nil {
		if err := mutationLease.Validate(); err != nil {
			return AppendResult{}, fmt.Errorf("note event may already be durable: %w", err)
		}
	}
	result.Applied = true
	result.MissionBrief, result.ExecutorAction, _, boardLane, err = laneExecutorSnapshot(inst.CaseRoot, lane)
	if err != nil {
		return result, err
	}
	result.MissionCommanderAction = result.ExecutorAction.MissionCommanderAction
	result.MissionCommanderNextActions = noteMissionCommanderNextActions(boardLane, result.ExecutorAction)
	return result, nil
}

func noteMissionCommanderNextActions(lane mission.BoardLane, action mission.ExecutorAction) []mission.MissionCommanderNextActionItem {
	return mission.MissionCommanderNextActions([]mission.LaneExecutorActionSnapshot{{
		Lane:           lane.ID,
		Label:          mission.BoardLaneLabel(lane),
		Status:         lane.Status,
		Workspace:      lane.Workspace,
		ExecutorAction: action,
	}}, nil, action.Blocked)
}

func laneExecutorSnapshot(caseRoot, laneID string) (mission.Brief, mission.ExecutorAction, mission.Facts, mission.BoardLane, error) {
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return mission.Brief{}, mission.ExecutorAction{}, mission.Facts{}, mission.BoardLane{}, err
	}
	lane, ok := mission.LookupBoardLane(board.Lanes, laneID, false)
	if !ok {
		return mission.Brief{}, mission.ExecutorAction{}, mission.Facts{}, mission.BoardLane{}, fmt.Errorf("unknown lane %q", laneID)
	}
	ledgerFacts, err := mission.ReadStrictLedgerFacts(caseRoot)
	if err != nil {
		return mission.Brief{}, mission.ExecutorAction{}, mission.Facts{}, mission.BoardLane{}, err
	}
	facts := ledgerFacts.Facts
	missionLane := mission.Lane{ID: lane.ID, Label: mission.BoardLaneLabel(lane), Status: lane.Status}
	brief := mission.BuildWithOptions([]mission.Lane{missionLane}, mission.LaneFacts(facts, lane.ID), mission.BuildOptions{MaxRows: mission.DefaultMaxRows})
	return brief, mission.LaneExecutorAction(missionLane, facts, brief), facts, lane, nil
}

func buildEvent(kind, lane, createdAt string, opt Options) map[string]any {
	event := map[string]any{
		"schemaVersion": 1,
		"kind":          kind,
		"lane":          lane,
		"subject":       strings.TrimSpace(opt.Subject),
		"summary":       strings.TrimSpace(opt.Summary),
		"createdAt":     createdAt,
	}
	addString := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			event[key] = strings.TrimSpace(value)
		}
	}
	addString("actor", opt.Actor)
	addString("packetId", opt.PacketID)
	addString("routeId", opt.RouteID)
	addString("shardId", opt.ShardID)
	addString("packetPath", opt.PacketPath)
	addString("reviewerResultPath", opt.ReviewerResultPath)
	addString("reviewerSession", opt.ReviewerSession)
	addString("reviewerHarness", opt.ReviewerHarness)
	addString("reviewerDispatchId", opt.ReviewerDispatchID)
	addString("reviewerDispatchReceiptPath", opt.ReviewerDispatchPath)
	addString("reviewerDispatchReceiptSha256", opt.ReviewerDispatchSHA256)
	addString("reviewerCompletionReceiptPath", opt.ReviewerCompletionPath)
	addString("reviewerCompletionReceiptSha256", opt.ReviewerCompletionSHA256)
	addString("reviewerResultInputPath", opt.ReviewerResultInputPath)
	addString("reviewerResultInputSha256", opt.ReviewerResultInputSHA256)
	addString("reviewerResultInputBytes", opt.ReviewerResultInputBytes)
	addString("reviewerResultSha256", opt.ReviewerResultSHA256)
	addString("reviewerManifestSha256", opt.ReviewerManifestSHA256)
	addString("reviewerVerificationEventId", opt.ReviewerVerificationID)
	addString("reviewerDecisionEventId", opt.ReviewerDecisionID)
	addString("ownerExecutor", opt.OwnerExecutor)
	addString("ownerGeneration", opt.OwnerGeneration)
	addString("ownerBindingMode", opt.OwnerBindingMode)
	addString("ownerBindingTarget", opt.OwnerBindingTarget)
	addString("reviewerDecision", opt.ReviewerDecision)
	addString("recommendedVerdict", opt.RecommendedVerdict)
	if risks := cleanList(opt.ReviewerRisks); len(risks) > 0 {
		event["reviewerRisks"] = risks
	}
	if conflicts := cleanList(opt.ReviewerConflicts); len(conflicts) > 0 {
		event["reviewerConflicts"] = conflicts
	}
	if routeOutput := routeOutputStrings(opt.RouteOutput); len(routeOutput) > 0 {
		event["routeOutput"] = routeOutput
	}
	addString("risk", opt.Risk)
	if related := splitList(opt.Related); len(related) > 0 {
		event["related"] = related
	}
	addString("confidence", opt.Confidence)
	addString("decision", opt.Decision)
	addString("reason", opt.Reason)
	addString("status", opt.Status)
	addString("batchId", opt.BatchID)
	if refs := splitList(opt.EvidenceRefs); len(refs) > 0 {
		event["evidenceRefs"] = refs
	}
	addString("target", opt.Target)
	if kind == "verification" {
		addString("verifier", opt.Verifier)
		addString("verdict", opt.Verdict)
	}
	if kind == "intervention" {
		addString("action", opt.Action)
		addString("approvedBy", opt.ApprovedBy)
		addString("scope", opt.Scope)
		addString("expires", opt.Expires)
	}
	return event
}

func validateAppendOptions(kind string, opt Options) error {
	validConfidence := []string{"low", "medium", "high"}
	validDecision := []string{"accept", "reject", "defer", "supersede"}
	validStatus := []string{"open", "accepted", "rejected", "superseded", "resolved", "deferred", "pending-gate", "confirmed", "needs_more_evidence"}
	validVerifier := []string{"manual-review", "schema-check", "focused-trace", "parity", "cross-run", "tool-review"}
	validVerdict := []string{"accepted", "rejected", "inconclusive", "needs-more-evidence"}
	validInterventionAction := []string{"override", "rollback", "heavy-tool-approval", "schema-migration", "external-side-effect"}
	if confidence := strings.TrimSpace(opt.Confidence); confidence != "" && !slices.Contains(validConfidence, confidence) {
		return fmt.Errorf("invalid Confidence %q; allowed: %s", confidence, strings.Join(validConfidence, ","))
	}
	if decision := strings.TrimSpace(opt.Decision); kind == "decision" && decision != "" && !slices.Contains(validDecision, decision) {
		return fmt.Errorf("invalid Decision %q; allowed: %s", decision, strings.Join(validDecision, ","))
	}
	if verdict := strings.TrimSpace(opt.Verdict); kind == "verification" && verdict != "" && !slices.Contains(validVerdict, verdict) {
		return fmt.Errorf("invalid Verdict %q; allowed: %s", verdict, strings.Join(validVerdict, ","))
	}
	if verifier := strings.TrimSpace(opt.Verifier); kind == "verification" && verifier != "" && !slices.Contains(validVerifier, verifier) {
		return fmt.Errorf("invalid Verifier %q; allowed: %s", verifier, strings.Join(validVerifier, ","))
	}
	if action := strings.TrimSpace(opt.Action); kind == "intervention" && action != "" && !slices.Contains(validInterventionAction, action) {
		return fmt.Errorf("invalid Action %q; allowed: %s", action, strings.Join(validInterventionAction, ","))
	}
	if status := strings.TrimSpace(opt.Status); status != "" && !slices.Contains(validStatus, status) {
		return fmt.Errorf("invalid Status %q; allowed: %s", status, strings.Join(validStatus, ","))
	}
	for _, ref := range strings.FieldsFunc(opt.EvidenceRefs, func(r rune) bool { return r == ',' || r == ';' || r == '\n' }) {
		if strings.TrimSpace(ref) == "" {
			return fmt.Errorf("EvidenceRefs contains empty element")
		}
	}
	if opt.ExpectedExecutionControlBinding != nil {
		if err := executioncontrol.ValidateBinding(*opt.ExpectedExecutionControlBinding); err != nil {
			return fmt.Errorf("invalid note execution control binding: %w", err)
		}
		if opt.ExpectedExecutionControlBinding.Lane != strings.TrimSpace(opt.Lane) {
			return fmt.Errorf("note execution control binding belongs to a different lane")
		}
	}
	return nil
}

func isValidKind(kind string) bool {
	return slices.Contains(mission.LedgerKinds(), kind)
}

func noteExtra(kind string, item event) string {
	extra := ""
	switch kind {
	case "candidate":
		extra = fmt.Sprintf(" | confidence=%s | status=%s | risk=%s", stringValue(item, "confidence"), stringValue(item, "status"), stringValue(item, "risk"))
	case "decision":
		decision := firstText(stringValue(item, "decision"), stringValue(item, "action"))
		by := firstText(stringValue(item, "confirmedBy"), stringValue(item, "actor"))
		extra = fmt.Sprintf(" | decision=%s | by=%s", decision, by)
	case "request":
		extra = gateDetail(item, false, true)
	case "verification":
		extra = fmt.Sprintf(" | verifier=%s | verdict=%s | target=%s", stringValue(item, "verifier"), stringValue(item, "verdict"), stringValue(item, "target"))
	case "intervention":
		extra = fmt.Sprintf(" | action=%s | target=%s | approvedBy=%s | scope=%s | status=%s | reason=%s", stringValue(item, "action"), stringValue(item, "target"), stringValue(item, "approvedBy"), stringValue(item, "scope"), stringValue(item, "status"), stringValue(item, "reason"))
	case "rollback":
		extra = fmt.Sprintf(" | target=%s | status=%s | reason=%s", stringValue(item, "target"), stringValue(item, "status"), stringValue(item, "reason"))
	}
	if batch := stringValue(item, "batchId"); strings.TrimSpace(batch) != "" {
		extra += " | batch=" + batch
	}
	return extra
}

func gateDetail(item event, omitStatus, omitBatch bool) string {
	parts := []string{}
	add := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			parts = append(parts, key+"="+value)
		}
	}
	if !omitStatus {
		add("status", stringValue(item, "status"))
	}
	add("by", stringValue(item, "actor"))
	add("risk", stringValue(item, "risk"))
	add("target", stringValue(item, "target"))
	if !omitBatch {
		add("batch", stringValue(item, "batchId"))
	}
	if gate, ok := item["gate"].(map[string]any); ok {
		add("action", stringValue(gate, "action"))
		add("scope", stringValue(gate, "scope"))
		add("budget", stringValue(gate, "budget"))
		add("tried", stringValue(gate, "triedLightSteps"))
		add("stop", stringValue(gate, "stopConditions"))
	}
	if len(parts) == 0 {
		return ""
	}
	return " | " + strings.Join(parts, " | ")
}

func readFactEvents(caseRoot, kind string) ([]event, error) {
	items, err := mission.ReadStrictFact(caseRoot, kind)
	if err != nil {
		return nil, err
	}
	return eventMaps(items), nil
}

func eventMaps(items []map[string]any) []event {
	out := make([]event, 0, len(items))
	for _, item := range items {
		out = append(out, event(item))
	}
	return out
}

func lastEvents(items []event, n int) []event {
	if len(items) <= n {
		return items
	}
	return items[len(items)-n:]
}

func cloneEvents(items []event) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, maps.Clone(map[string]any(item)))
	}
	return out
}

func firstText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func stringValue(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []any:
		parts := []string{}
		for _, item := range t {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, ",")
	case []string:
		return strings.Join(t, ",")
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func splitList(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || r == '\n' })
	return cleanList(parts)
}

func cleanList(values []string) []string {
	out := []string{}
	for _, part := range values {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func routeOutputStrings(values map[string]any) map[string]string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		if strings.TrimSpace(key) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	out := map[string]string{}
	for _, key := range keys {
		value := strings.TrimSpace(fmt.Sprint(values[key]))
		value = strings.Join(strings.Fields(value), " ")
		if value != "" {
			out[strings.TrimSpace(key)] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func assertLane(caseRoot, lane string) error {
	return mission.AssertBoardLane(caseRoot, lane, mission.LaneGuardOptions{Command: "note"})
}

func noteCreatedAt(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Now().UTC().Format(time.RFC3339Nano), nil
	}
	if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
		return "", fmt.Errorf("invalid CreatedAt %q: %w", value, err)
	}
	return value, nil
}

func encodeEvent(event map[string]any) ([]byte, string, error) {
	encoded, err := json.Marshal(event)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(encoded)
	return encoded, hex.EncodeToString(sum[:]), nil
}

var recordCommandReplayableKeys = map[string]bool{
	"schemaVersion":                   true,
	"kind":                            true,
	"lane":                            true,
	"subject":                         true,
	"summary":                         true,
	"actor":                           true,
	"risk":                            true,
	"related":                         true,
	"confidence":                      true,
	"decision":                        true,
	"reason":                          true,
	"status":                          true,
	"batchId":                         true,
	"target":                          true,
	"verifier":                        true,
	"verdict":                         true,
	"action":                          true,
	"approvedBy":                      true,
	"scope":                           true,
	"expires":                         true,
	"evidenceRefs":                    true,
	"eventId":                         true,
	"createdAt":                       true,
	"packetId":                        true,
	"routeId":                         true,
	"shardId":                         true,
	"packetPath":                      true,
	"reviewerResultPath":              true,
	"reviewerSession":                 true,
	"reviewerDispatchReceiptPath":     true,
	"reviewerDispatchReceiptSha256":   true,
	"reviewerCompletionReceiptPath":   true,
	"reviewerCompletionReceiptSha256": true,
	"reviewerResultInputPath":         true,
	"reviewerResultInputSha256":       true,
	"reviewerResultInputBytes":        true,
	"reviewerResultSha256":            true,
	"reviewerManifestSha256":          true,
	"reviewerVerificationEventId":     true,
	"reviewerDecisionEventId":         true,
	"ownerExecutor":                   true,
	"ownerGeneration":                 true,
}

func recordCommand(args []string) string {
	if len(args) == 0 {
		return ""
	}
	parts := append([]string{"/rekit", "note"}, args[2:]...)
	for i := range parts {
		parts[i] = quoteCommandArg(parts[i])
	}
	return strings.Join(parts, " ")
}

func recordArgs(caseRoot, pack string, event map[string]any, eventSHA256 string, binding *executioncontrol.Binding) ([]string, error) {
	if !recordCommandReplayable(event) {
		return nil, nil
	}
	args := []string{"-Command", "note", "-Target", caseRoot, "-Pack", pack, "-Kind", stringValue(event, "kind"), "-Lane", stringValue(event, "lane")}
	for _, item := range []struct{ flag, key string }{
		{"-Subject", "subject"},
		{"-Summary", "summary"},
		{"-Actor", "actor"},
		{"-Risk", "risk"},
		{"-Related", "related"},
		{"-Confidence", "confidence"},
		{"-Decision", "decision"},
		{"-Reason", "reason"},
		{"-Status", "status"},
		{"-BatchId", "batchId"},
		{"-TargetRef", "target"},
		{"-Verifier", "verifier"},
		{"-Verdict", "verdict"},
		{"-Action", "action"},
		{"-ApprovedBy", "approvedBy"},
		{"-Scope", "scope"},
		{"-Expires", "expires"},
		{"-EvidenceRefs", "evidenceRefs"},
		{"-EventId", "eventId"},
		{"-CreatedAt", "createdAt"},
		{"-ReviewerPacketId", "packetId"},
		{"-ReviewerRouteId", "routeId"},
		{"-ReviewerShardId", "shardId"},
		{"-ReviewerPacketPath", "packetPath"},
		{"-ReviewerResultLineagePath", "reviewerResultPath"},
		{"-ReviewerLineageSession", "reviewerSession"},
		{"-ReviewerDispatchReceiptPath", "reviewerDispatchReceiptPath"},
		{"-ReviewerDispatchReceiptSha256", "reviewerDispatchReceiptSha256"},
		{"-ReviewerCompletionReceiptPath", "reviewerCompletionReceiptPath"},
		{"-ReviewerCompletionReceiptSha256", "reviewerCompletionReceiptSha256"},
		{"-ReviewerLineageInputPath", "reviewerResultInputPath"},
		{"-ReviewerLineageInputSha256", "reviewerResultInputSha256"},
		{"-ReviewerLineageInputBytes", "reviewerResultInputBytes"},
		{"-ReviewerLineageResultSha256", "reviewerResultSha256"},
		{"-ReviewerManifestSha256", "reviewerManifestSha256"},
		{"-ReviewerVerificationEventId", "reviewerVerificationEventId"},
		{"-ReviewerDecisionEventId", "reviewerDecisionEventId"},
		{"-ReviewerOwnerExecutor", "ownerExecutor"},
		{"-ReviewerOwnerGeneration", "ownerGeneration"},
		{"-ExpectedNoteEventSha256", ""},
	} {
		value := eventSHA256
		if item.key != "" {
			value = stringValue(event, item.key)
		}
		if strings.TrimSpace(value) != "" {
			args = append(args, item.flag, value)
		}
	}
	if binding != nil {
		if err := executioncontrol.ValidateBinding(*binding); err != nil {
			return nil, fmt.Errorf("record note execution control binding: %w", err)
		}
		data, err := json.Marshal(binding)
		if err != nil {
			return nil, err
		}
		args = append(args, "-ExpectedExecutionControlBindingJson", string(data))
	}
	return append(args, "-Format", "json"), nil
}

func quoteCommandArg(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return `""`
	}
	if !strings.ContainsAny(value, " \t\"") {
		return value
	}
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func recordCommandReplayable(event map[string]any) bool {
	for key := range event {
		if !recordCommandReplayableKeys[key] {
			return false
		}
	}
	return true
}

func eventIDFor(event map[string]any) string {
	seed := strings.Join([]string{
		stringValue(event, "kind"),
		stringValue(event, "lane"),
		stringValue(event, "subject"),
		stringValue(event, "summary"),
		stringValue(event, "actor"),
		stringValue(event, "risk"),
		stringValue(event, "related"),
		stringValue(event, "confidence"),
		stringValue(event, "decision"),
		stringValue(event, "reason"),
		stringValue(event, "status"),
		stringValue(event, "batchId"),
		stringValue(event, "evidenceRefs"),
		stringValue(event, "target"),
		stringValue(event, "verifier"),
		stringValue(event, "verdict"),
		stringValue(event, "action"),
		stringValue(event, "approvedBy"),
		stringValue(event, "scope"),
		stringValue(event, "expires"),
		stringValue(event, "createdAt"),
	}, "|")
	sum := sha256.Sum256([]byte(seed))
	return "evt-" + hex.EncodeToString(sum[:])[:16]
}

func eventIDExists(caseRoot, id string) (bool, error) {
	known, err := mission.ReadStrictLedgerEventIDs(caseRoot)
	if err != nil {
		return false, err
	}
	return known[id], nil
}
