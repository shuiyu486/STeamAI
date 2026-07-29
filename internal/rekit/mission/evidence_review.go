package mission

import (
	"encoding/json"
	"fmt"
	"strings"
)

func ExecutionEvidenceReviewItems(observations []map[string]any, laneID string, labelFor func(string) string, maxRows int) []ExecutionEvidenceReviewItem {
	items := []ExecutionEvidenceReviewItem{}
	for _, observation := range observations {
		item, ok := ExecutionEvidenceReviewItemFromObservation(observation, laneID, labelFor)
		if ok {
			items = append(items, item)
		}
	}
	return limitExecutionEvidenceReviewItems(items, maxRows)
}

func ExecutionEvidenceReviewItemsWithLedgerFacts(facts LedgerFacts, laneID string, labelFor func(string) string, maxRows int) []ExecutionEvidenceReviewItem {
	acknowledged := ExecutionEvidenceReviewAcknowledgedIDs(facts)
	items := []ExecutionEvidenceReviewItem{}
	for _, observation := range facts.Observations {
		item, ok := ExecutionEvidenceReviewItemFromObservation(observation, laneID, labelFor)
		if !ok {
			continue
		}
		if executionEvidenceReviewItemAcknowledged(item, acknowledged) {
			continue
		}
		items = append(items, item)
	}
	return limitExecutionEvidenceReviewItems(items, maxRows)
}

func ExecutionEvidenceReviewAcknowledgedIDs(facts LedgerFacts) map[string]bool {
	acknowledged := executionEvidenceReviewAcknowledgedIDs(facts)
	for _, observation := range facts.Observations {
		item, ok := ExecutionEvidenceReviewItemFromObservation(observation, "", nil)
		if !ok {
			continue
		}
		if executionEvidenceReviewItemAcknowledged(item, acknowledged) {
			if item.EventID != "" {
				acknowledged[item.EventID] = true
			}
			if item.GateEventID != "" {
				acknowledged[item.GateEventID] = true
			}
		}
	}
	return acknowledged
}

func limitExecutionEvidenceReviewItems(items []ExecutionEvidenceReviewItem, maxRows int) []ExecutionEvidenceReviewItem {
	if maxRows > 0 && len(items) > maxRows {
		return items[len(items)-maxRows:]
	}
	return items
}

func executionEvidenceReviewAcknowledgedIDs(facts LedgerFacts) map[string]bool {
	acknowledged := map[string]bool{}
	for _, verification := range facts.Verifications {
		if !executionEvidenceReviewClosingVerification(verification) {
			continue
		}
		for _, related := range objectStringList(verification["related"]) {
			acknowledged[related] = true
		}
	}
	for _, decision := range facts.Decisions {
		if !executionEvidenceReviewClosingDecision(decision) {
			continue
		}
		for _, related := range objectStringList(decision["related"]) {
			acknowledged[related] = true
		}
	}
	return acknowledged
}

func executionEvidenceReviewClosingStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "accepted", "rejected", "resolved", "confirmed", "superseded":
		return true
	default:
		return false
	}
}

func executionEvidenceReviewClosingVerification(verification map[string]any) bool {
	if !strings.EqualFold(firstObjectText(verification, "kind"), "verification") {
		return false
	}
	if !executionEvidenceReviewClosingStatus(firstObjectText(verification, "status")) {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(firstObjectText(verification, "verdict"))) {
	case "accepted", "rejected":
		return true
	default:
		return false
	}
}

func executionEvidenceReviewClosingDecision(decision map[string]any) bool {
	if !strings.EqualFold(firstObjectText(decision, "kind"), "decision") {
		return false
	}
	if !executionEvidenceReviewClosingStatus(firstObjectText(decision, "status")) {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(FirstText(firstObjectText(decision, "decision"), firstObjectText(decision, "action")))) {
	case "accept", "reject", "supersede":
		return true
	default:
		return false
	}
}

func executionEvidenceReviewItemAcknowledged(item ExecutionEvidenceReviewItem, acknowledged map[string]bool) bool {
	if len(acknowledged) == 0 {
		return false
	}
	return (item.EventID != "" && acknowledged[item.EventID]) || (item.GateEventID != "" && acknowledged[item.GateEventID])
}

func ExecutionEvidenceReviewItemFromObservation(observation map[string]any, laneID string, labelFor func(string) string) (ExecutionEvidenceReviewItem, bool) {
	if !strings.EqualFold(firstObjectText(observation, "kind"), "observation") {
		return ExecutionEvidenceReviewItem{}, false
	}
	lane := firstObjectText(observation, "lane")
	if laneID != "" && lane != laneID {
		return ExecutionEvidenceReviewItem{}, false
	}
	execution := objectMap(observation["execution"])
	gateEventID := firstObjectText(execution, "gateEventId")
	if gateEventID == "" {
		return ExecutionEvidenceReviewItem{}, false
	}
	authorization := firstObjectText(execution, "authorization")
	if authorization == "" {
		gate := objectMap(observation["gate"])
		authorization = firstObjectText(objectMap(gate["authorization"]), "decision")
	}
	if !strings.EqualFold(authorization, "preauthorized") {
		return ExecutionEvidenceReviewItem{}, false
	}
	label := lane
	if labelFor != nil {
		label = labelFor(lane)
	}
	if label == "" {
		label = BoardLaneLabel(BoardLane{ID: lane})
	}
	if label == "" {
		label = "main"
	}
	status := firstObjectText(observation, "status")
	if status == "" {
		status = firstObjectText(execution, "status")
	}
	gate := objectMap(observation["gate"])
	adapterExecution := objectMap(execution["adapterExecution"])
	adapterDispatch := objectMap(execution["adapterExecutionDispatch"])
	adapterOwner := objectMap(adapterExecution["owner"])
	adapterBinding := objectMap(adapterExecution["adapter"])
	boundary := []string{
		"observation evidence is already recorded; do not replay heavy tool",
		"review outputRefs/evidenceRefs before any authority/confirmed outcome",
		"no authority/confirmed writes",
	}
	boundaryHits := objectStringList(execution["boundaryHits"])
	escalation := firstObjectText(execution, "escalation")
	if status == "boundary-hit" || status == "escalated" || len(boundaryHits) > 0 || escalation != "" {
		boundary = append(boundary, "boundary/escalation requires main review before autonomous continuation")
	}
	item := ExecutionEvidenceReviewItem{
		Lane:                           lane,
		EventID:                        firstObjectText(observation, "eventId"),
		GateEventID:                    gateEventID,
		Subject:                        firstObjectText(observation, "subject"),
		Summary:                        firstObjectText(observation, "summary"),
		Status:                         status,
		Action:                         firstObjectText(gate, "action"),
		Target:                         firstObjectText(observation, "target"),
		OutputRefs:                     objectStringList(execution["outputRefs"]),
		EvidenceRefs:                   objectStringList(observation["evidenceRefs"]),
		ExecutionReportPath:            firstObjectText(execution, "executionReportPath"),
		ExecutionReportSHA256:          firstObjectText(execution, "executionReportSha256"),
		AdapterExecutionDispatchID:     firstObjectText(adapterDispatch, "dispatchId"),
		AdapterExecutionDispatchPath:   firstObjectText(execution, "adapterExecutionDispatchPath"),
		AdapterExecutionDispatchSHA256: firstObjectText(execution, "adapterExecutionDispatchSha256"),
		AdapterExecutionReceiptPath:    firstObjectText(execution, "adapterExecutionReceiptPath"),
		AdapterExecutionReceiptSHA256:  firstObjectText(execution, "adapterExecutionReceiptSha256"),
		CurrentExecutor:                firstObjectText(adapterOwner, "currentExecutor"),
		ExecutorGeneration:             objectInt(adapterOwner["executorGeneration"]),
		AdapterHarness:                 firstObjectText(adapterOwner, "adapterHarness"),
		AdapterSession:                 firstObjectText(adapterOwner, "adapterSession"),
		ToolingCatalogSHA256:           firstObjectText(adapterBinding, "toolingCatalogSha256"),
		AdapterExecutionArtifactCount:  objectListLength(adapterExecution["artifacts"]),
		ActualBudget:                   executionEvidenceBudget(execution),
		AdapterID:                      firstObjectText(objectMap(execution["adapter"]), "adapterId"),
		AdapterStatus:                  firstObjectText(objectMap(execution["adapter"]), "status"),
		AdapterContext:                 executionEvidenceAdapterContext(execution),
		BoundaryHits:                   boundaryHits,
		Escalation:                     escalation,
		ReviewCommand:                  "review outputRefs/evidenceRefs for gateEventId " + gateEventID,
		HandoffCommand:                 "/rekit handoff " + label,
		Boundary:                       boundary,
	}
	item.Acknowledgement = ExecutionEvidenceReviewAcknowledgementFor(item)
	item.MissionCommanderAction = ExecutionEvidenceReviewCommanderAction(item, label)
	item.FollowThrough = ExecutionEvidenceReviewFollowThrough(item)
	item.ReviewRunbookSteps = ExecutionEvidenceReviewRunbookSteps(item, true)
	return item, true
}

func ExecutionEvidenceReviewAcknowledgementFor(item ExecutionEvidenceReviewItem) *ExecutionEvidenceReviewAcknowledgement {
	related := executionEvidenceReviewAcknowledgementRelated(item)
	if len(related) == 0 {
		return nil
	}
	ack := &ExecutionEvidenceReviewAcknowledgement{
		State:         "ready-for-acknowledgement-preview",
		RecordCommand: "run the hash-bound recordCommand returned by the acknowledgement note -WhatIf",
		Related:       related,
		EvidenceRefs:  executionEvidenceReviewAcknowledgementEvidenceRefs(item),
		Boundary: []string{
			"review outputRefs/evidenceRefs before running the acknowledgement note preview",
			"acknowledgement uses note -Kind verification -WhatIf first; run only the returned hash-bound recordCommand after review",
			"acknowledgement only closes execution evidence review; no authority/confirmed writes and no heavy-tool replay",
		},
	}
	if ExecutionEvidenceReviewItemNeedsMainReview(item) {
		ack.State = "requires-main-review-before-acknowledgement"
		ack.RecordCommand = "main Agent must review boundary/escalation before choosing an acknowledgement note"
		ack.Boundary = append(ack.Boundary, "boundary/escalation requires main review before any accepted/rejected acknowledgement note")
		return ack
	}
	ack.AcceptedPreviewCommand = executionEvidenceReviewAcknowledgementCommand(item, "accepted")
	ack.RejectedPreviewCommand = executionEvidenceReviewAcknowledgementCommand(item, "rejected")
	return ack
}

func executionEvidenceReviewAcknowledgementRelated(item ExecutionEvidenceReviewItem) []string {
	return UniqueStrings(compactStrings([]string{item.EventID, item.GateEventID}))
}

func executionEvidenceReviewAcknowledgementEvidenceRefs(item ExecutionEvidenceReviewItem) []string {
	refs := append([]string{}, item.OutputRefs...)
	refs = append(refs, item.EvidenceRefs...)
	if item.ExecutionReportPath != "" {
		refs = append(refs, item.ExecutionReportPath)
	}
	return UniqueStrings(compactStrings(refs))
}

func executionEvidenceReviewAcknowledgementCommand(item ExecutionEvidenceReviewItem, verdict string) string {
	lane := evidenceReviewLane(item)
	if lane == "" {
		lane = strings.TrimSpace(item.Lane)
	}
	if lane == "" {
		lane = "main"
	}
	verdict = strings.ToLower(strings.TrimSpace(verdict))
	status := "resolved"
	subject := "execution evidence review accepted"
	summary := "accepted recorded execution evidence"
	if verdict == "rejected" {
		status = "rejected"
		subject = "execution evidence review rejected"
		summary = "rejected recorded execution evidence"
	}
	if item.GateEventID != "" {
		summary += " for gateEventId " + item.GateEventID
	}
	args := []string{
		"/rekit", "note",
		"-Kind", "verification",
		"-Lane", lane,
		"-Subject", subject,
		"-Summary", summary,
		"-Verifier", "manual-review",
		"-Verdict", verdict,
		"-Status", status,
		"-Related", strings.Join(executionEvidenceReviewAcknowledgementRelated(item), ","),
		"-Reason", "reviewed outputRefs/evidenceRefs before closing execution evidence review",
	}
	if target := FirstText(item.Target, item.ExecutionReportPath, item.GateEventID); target != "" {
		args = append(args, "-TargetRef", target)
	}
	if refs := executionEvidenceReviewAcknowledgementEvidenceRefs(item); len(refs) > 0 {
		args = append(args, "-EvidenceRefs", strings.Join(refs, ","))
	}
	args = append(args, "-WhatIf", "-Format", "json")
	for i := range args {
		args[i] = quoteCommandArg(args[i])
	}
	return strings.Join(args, " ")
}

func ExecutionEvidenceReviewCommanderAction(item ExecutionEvidenceReviewItem, label string) MissionCommanderAction {
	state := "ready-for-evidence-review"
	prompt := "authorized gate `" + item.GateEventID + "` 的 observation evidence 已记录；先 review output/evidence refs，再用 acknowledgement note -WhatIf 预览关闭 review。"
	if ExecutionEvidenceReviewItemNeedsMainReview(item) {
		state = "needs-main-escalation"
		prompt = "authorized gate `" + item.GateEventID + "` 的 observation evidence 记录了 boundary/escalation；停止该 action 的自主推进并通知 main Agent。"
	}
	boundary := append([]string{}, item.Boundary...)
	if item.Acknowledgement != nil {
		boundary = append(boundary, item.Acknowledgement.Boundary...)
	}
	if len(boundary) == 0 {
		boundary = []string{
			"observation evidence is already recorded; do not replay heavy tool",
			"review outputRefs/evidenceRefs before any authority/confirmed outcome",
			"no authority/confirmed writes",
		}
	}
	followUp := []string{"/rekit overview"}
	if label == "" {
		label = "main"
	}
	if !ExecutionEvidenceReviewItemNeedsMainReview(item) {
		followUp = append(followUp, "/rekit continue "+label+" -WhatIf")
	}
	return MissionCommanderAction{
		State:            state,
		Prompt:           prompt,
		PrimaryCommand:   item.HandoffCommand,
		FollowUpCommands: followUp,
		Boundary:         UniqueStrings(compactStrings(boundary)),
	}
}

func ExecutionEvidenceReviewFollowThrough(item ExecutionEvidenceReviewItem) ExecutionEvidenceFollowThrough {
	state := strings.TrimSpace(item.MissionCommanderAction.State)
	if state == "" {
		state = "ready-for-evidence-review"
	}
	boundary := append([]string{}, item.MissionCommanderAction.Boundary...)
	if len(boundary) == 0 {
		boundary = append([]string{}, item.Boundary...)
	}
	if len(boundary) == 0 {
		boundary = []string{
			"observation evidence is already recorded; do not replay heavy tool",
			"review outputRefs/evidenceRefs before any authority/confirmed outcome",
			"no authority/confirmed writes",
		}
	}
	follow := ExecutionEvidenceFollowThrough{
		State:       state,
		GateEventID: item.GateEventID,
		Boundary:    boundary,
	}
	follow.Outcomes = []ExecutionEvidenceOutcome{executionEvidenceOutcomeFor(item, state, boundary)}
	follow.ActionQueue = MissionCommanderActionQueueFor(executionEvidenceFollowThroughActions(item, state, boundary))
	return follow
}

func ExecutionEvidenceReviewRunbookSteps(item ExecutionEvidenceReviewItem, includeContinueFollowUp bool) []string {
	steps := []string{}
	if item.GateEventID != "" {
		if item.ReviewCommand != "" {
			steps = append(steps, "review execution evidence for gateEventId "+item.GateEventID+": "+item.ReviewCommand)
		} else {
			steps = append(steps, "review execution evidence for gateEventId "+item.GateEventID)
		}
	} else if item.ReviewCommand != "" {
		steps = append(steps, "review execution evidence: "+item.ReviewCommand)
	}
	if item.ExecutionReportPath != "" || item.ExecutionReportSHA256 != "" {
		reportPath := strings.TrimSpace(item.ExecutionReportPath)
		if reportPath == "" {
			reportPath = "none"
		}
		reportSHA256 := strings.TrimSpace(item.ExecutionReportSHA256)
		if reportSHA256 == "" {
			reportSHA256 = "none"
		}
		steps = append(steps, "verify execution report currentness: path="+reportPath+" sha256="+reportSHA256)
	}
	if len(item.OutputRefs) > 0 {
		steps = append(steps, "review outputRefs: "+strings.Join(item.OutputRefs, ","))
	}
	if len(item.EvidenceRefs) > 0 {
		steps = append(steps, "review evidenceRefs: "+strings.Join(item.EvidenceRefs, ","))
	}
	if ExecutionEvidenceReviewItemNeedsMainReview(item) {
		steps = append(steps, "boundary hit or escalation in execution evidence; stop autonomous continuation and notify main Agent")
	}
	steps = append(steps, "do not replay the heavy tool or adapter action")
	steps = append(steps, "do not write authority/confirmed from evidence review")
	if ack := item.Acknowledgement; ack != nil {
		if ack.AcceptedPreviewCommand != "" {
			steps = append(steps, "after review, preview accepted acknowledgement note: "+ack.AcceptedPreviewCommand)
		}
		if ack.RejectedPreviewCommand != "" {
			steps = append(steps, "if evidence review rejects the observation, preview rejected acknowledgement note: "+ack.RejectedPreviewCommand)
		}
		if ack.RecordCommand != "" {
			steps = append(steps, ack.RecordCommand)
		}
	}
	if command := strings.TrimSpace(item.MissionCommanderAction.PrimaryCommand); command != "" {
		steps = append(steps, command)
	} else if command := strings.TrimSpace(item.HandoffCommand); command != "" {
		steps = append(steps, command)
	}
	for _, followUp := range item.MissionCommanderAction.FollowUpCommands {
		if strings.Contains(followUp, "/rekit continue") && (!includeContinueFollowUp || ExecutionEvidenceReviewItemNeedsMainReview(item)) {
			continue
		}
		steps = append(steps, followUp)
	}
	return UniqueStrings(compactStrings(steps))
}

func executionEvidenceOutcomeFor(item ExecutionEvidenceReviewItem, state string, boundary []string) ExecutionEvidenceOutcome {
	name := "recorded-evidence-review"
	when := "bounded observation evidence was recorded for an authorized gate"
	expected := "reviewed outputRefs/evidenceRefs before any authority/confirmed outcome"
	actions := []string{
		"review outputRefs/evidenceRefs for gateEventId " + item.GateEventID,
		"run the evidence handoff command before continuing the lane",
	}
	verification := []string{item.HandoffCommand, "/rekit overview"}
	switch state {
	case "needs-main-escalation":
		name = "boundary-or-escalation-review"
		when = "recorded evidence reports boundaryHits, escalation, boundary-hit, or escalated status"
		expected = "main Agent reviews boundary/escalation before any autonomous continuation"
		actions = []string{
			"stop autonomous continuation for this action",
			"notify main Agent and review outputRefs/evidenceRefs for gateEventId " + item.GateEventID,
		}
		verification = []string{item.HandoffCommand, "/rekit overview"}
	case "evidence-already-recorded":
		name = "duplicate-record-review"
		when = "record command was replayed for an already recorded execution evidence eventId"
		expected = "duplicate replay does not append observation evidence; review the existing evidence only"
		actions = []string{
			"do not replay the heavy tool or append duplicate observation evidence",
			"review the already recorded outputRefs/evidenceRefs for gateEventId " + item.GateEventID,
		}
		verification = []string{item.HandoffCommand, "/rekit overview"}
	}
	return ExecutionEvidenceOutcome{
		Name:                 name,
		State:                state,
		When:                 when,
		Command:              item.HandoffCommand,
		Actions:              actions,
		VerificationCommands: compactStrings(verification),
		Expected:             expected,
		Evidence:             compactStrings(append(append([]string{}, item.OutputRefs...), item.EvidenceRefs...)),
		Boundary:             boundary,
	}
}

func executionEvidenceFollowThroughActions(item ExecutionEvidenceReviewItem, state string, boundary []string) []MissionCommanderNextActionItem {
	items := []MissionCommanderNextActionItem{}
	lane := evidenceReviewLane(item)
	label := evidenceReviewLabel(item)
	needsMainReview := state == "needs-main-escalation"
	if command := strings.TrimSpace(item.MissionCommanderAction.PrimaryCommand); command != "" {
		items = append(items, MissionCommanderNextActionItem{
			Lane:           lane,
			Label:          label,
			State:          state,
			Command:        command,
			Source:         "executionEvidenceReview.followThrough",
			Blocked:        needsMainReview,
			RequiresReview: true,
			Reasons:        executionEvidenceFollowThroughReasons(item, needsMainReview),
			Boundary:       append([]string{}, boundary...),
		})
	}
	for _, followUp := range item.MissionCommanderAction.FollowUpCommands {
		if strings.Contains(followUp, "/rekit continue") {
			continue
		}
		items = append(items, MissionCommanderNextActionItem{
			Lane:           lane,
			Label:          label,
			State:          state,
			Command:        followUp,
			Source:         "executionEvidenceReview.followThrough.followUp",
			Blocked:        needsMainReview,
			RequiresReview: true,
			Reasons:        executionEvidenceFollowThroughReasons(item, needsMainReview),
			Boundary:       append([]string{}, boundary...),
		})
	}
	return UniqueCommanderNextActions(items)
}

func executionEvidenceFollowThroughReasons(item ExecutionEvidenceReviewItem, needsMainReview bool) []string {
	reasons := []string{}
	if item.GateEventID != "" {
		reasons = append(reasons, "review execution evidence for gateEventId "+item.GateEventID)
	}
	if item.ReviewCommand != "" {
		reasons = append(reasons, item.ReviewCommand)
	}
	if needsMainReview {
		reasons = append(reasons, "boundary hit or escalation in execution evidence; stop autonomous continuation and notify main Agent")
	}
	return reasons
}

func compactStrings(items []string) []string {
	out := []string{}
	for _, item := range items {
		if text := strings.TrimSpace(item); text != "" {
			out = append(out, text)
		}
	}
	return UniqueStrings(out)
}

func executionEvidenceBudget(execution map[string]any) *ExecutionEvidenceBudget {
	budget := objectMap(execution["actualBudget"])
	if len(budget) == 0 {
		return nil
	}
	return &ExecutionEvidenceBudget{
		RuntimeSeconds: objectInt(budget["runtimeSeconds"]),
		DiskMB:         objectInt(budget["diskMB"]),
		Requests:       objectInt(budget["requests"]),
	}
}

func executionEvidenceAdapterContext(execution map[string]any) *ExecutionEvidenceAdapterContext {
	context := objectMap(execution["adapterContext"])
	if len(context) == 0 {
		return nil
	}
	candidate := ExecutionEvidenceAdapterContext{
		ID:                  firstObjectText(context, "id"),
		Status:              firstObjectText(context, "status"),
		Entry:               firstObjectText(context, "entry"),
		Purpose:             firstObjectText(context, "purpose"),
		SideEffects:         objectStringList(context["sideEffects"]),
		GateActions:         objectStringList(context["gateActions"]),
		ToolingCatalogPath:  firstObjectText(context, "toolingCatalogPath"),
		ReportGuidance:      objectStringList(context["reportGuidance"]),
		EvidenceGuidance:    objectStringList(context["evidenceGuidance"]),
		StopConditionHints:  objectStringList(context["stopConditionHints"]),
		RecordOnlyAfterGate: objectBool(context["recordOnlyAfterGate"]),
	}
	if candidate.ID == "" && candidate.Status == "" && candidate.Entry == "" && candidate.Purpose == "" && len(candidate.SideEffects) == 0 && len(candidate.GateActions) == 0 && candidate.ToolingCatalogPath == "" && len(candidate.ReportGuidance) == 0 && len(candidate.EvidenceGuidance) == 0 && len(candidate.StopConditionHints) == 0 && !candidate.RecordOnlyAfterGate {
		return nil
	}
	return &candidate
}

func objectMap(value any) map[string]any {
	if item, ok := value.(map[string]any); ok {
		return item
	}
	return nil
}

func objectListLength(value any) int {
	switch items := value.(type) {
	case []any:
		return len(items)
	case []map[string]any:
		return len(items)
	default:
		return 0
	}
}

func objectStringList(value any) []string {
	switch t := value.(type) {
	case []string:
		return append([]string{}, t...)
	case []any:
		items := []string{}
		for _, item := range t {
			if text := objectText(item); text != "" {
				items = append(items, text)
			}
		}
		return items
	case string:
		text := strings.TrimSpace(t)
		if text == "" {
			return nil
		}
		items := []string{}
		for part := range strings.SplitSeq(text, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				items = append(items, trimmed)
			}
		}
		return items
	default:
		if text := objectText(value); text != "" {
			return []string{text}
		}
		return nil
	}
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

func objectInt(value any) int {
	switch t := value.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case json.Number:
		n, _ := t.Int64()
		return int(n)
	case string:
		var n int
		_, _ = fmt.Sscanf(strings.TrimSpace(t), "%d", &n)
		return n
	default:
		return 0
	}
}

func objectBool(value any) bool {
	switch t := value.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(strings.TrimSpace(t), "true")
	default:
		return false
	}
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
