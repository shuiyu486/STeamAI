package workstream

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

type ReviewerWritebackItem struct {
	Kind               string            `json:"kind"`
	EventID            string            `json:"eventId,omitempty"`
	Lane               string            `json:"lane,omitempty"`
	Subject            string            `json:"subject,omitempty"`
	Summary            string            `json:"summary,omitempty"`
	Target             string            `json:"target,omitempty"`
	Verdict            string            `json:"verdict,omitempty"`
	Decision           string            `json:"decision,omitempty"`
	Reason             string            `json:"reason,omitempty"`
	Confidence         string            `json:"confidence,omitempty"`
	BatchID            string            `json:"batchId,omitempty"`
	PacketID           string            `json:"packetId,omitempty"`
	RouteID            string            `json:"routeId,omitempty"`
	ShardID            string            `json:"shardId,omitempty"`
	PacketPath         string            `json:"packetPath,omitempty"`
	ReviewerResultPath string            `json:"reviewerResultPath,omitempty"`
	ReviewerSession    string            `json:"reviewerSession,omitempty"`
	OwnerExecutor      string            `json:"ownerExecutor,omitempty"`
	OwnerGeneration    string            `json:"ownerGeneration,omitempty"`
	OwnerBindingMode   string            `json:"ownerBindingMode,omitempty"`
	OwnerBindingTarget string            `json:"ownerBindingTarget,omitempty"`
	ReviewerDecision   string            `json:"reviewerDecision,omitempty"`
	RecommendedVerdict string            `json:"recommendedVerdict,omitempty"`
	ReviewerRisks      []string          `json:"reviewerRisks,omitempty"`
	ReviewerConflicts  []string          `json:"reviewerConflicts,omitempty"`
	RouteOutput        map[string]string `json:"routeOutput,omitempty"`
	EvidenceRefs       []string          `json:"evidenceRefs,omitempty"`
}

func ReviewerWritebackItems(facts mission.LedgerFacts, laneID string) []ReviewerWritebackItem {
	items := []ReviewerWritebackItem{}
	items = appendReviewerWritebackEvents(items, "verification", facts.Verifications, laneID)
	items = appendReviewerWritebackEvents(items, "decision", facts.Decisions, laneID)
	if maxHandoffRows > 0 && len(items) > maxHandoffRows {
		items = items[len(items)-maxHandoffRows:]
	}
	return items
}

func appendReviewerWritebackEvents(out []ReviewerWritebackItem, kind string, events []map[string]any, laneID string) []ReviewerWritebackItem {
	for _, event := range events {
		if strings.TrimSpace(laneID) != "" && firstObjectText(event, "lane") != laneID {
			continue
		}
		item, ok := reviewerWritebackItem(kind, event)
		if !ok {
			continue
		}
		out = append(out, item)
	}
	return out
}

func reviewerWritebackItem(kind string, event map[string]any) (ReviewerWritebackItem, bool) {
	item := ReviewerWritebackItem{
		Kind:               kind,
		EventID:            firstObjectText(event, "eventId"),
		Lane:               firstObjectText(event, "lane"),
		Subject:            firstObjectText(event, "subject", "kind"),
		Summary:            firstObjectText(event, "summary"),
		Target:             firstObjectText(event, "target"),
		Verdict:            firstObjectText(event, "verdict"),
		Decision:           firstObjectText(event, "decision", "action"),
		Reason:             firstObjectText(event, "reason"),
		Confidence:         firstObjectText(event, "confidence"),
		BatchID:            firstObjectText(event, "batchId"),
		PacketID:           firstObjectText(event, "packetId"),
		RouteID:            firstObjectText(event, "routeId"),
		ShardID:            firstObjectText(event, "shardId"),
		PacketPath:         firstObjectText(event, "packetPath"),
		ReviewerResultPath: firstObjectText(event, "reviewerResultPath"),
		ReviewerSession:    firstObjectText(event, "reviewerSession"),
		OwnerExecutor:      firstObjectText(event, "ownerExecutor"),
		OwnerGeneration:    firstObjectText(event, "ownerGeneration"),
		OwnerBindingMode:   firstObjectText(event, "ownerBindingMode"),
		OwnerBindingTarget: firstObjectText(event, "ownerBindingTarget"),
		ReviewerDecision:   firstObjectText(event, "reviewerDecision"),
		RecommendedVerdict: firstObjectText(event, "recommendedVerdict"),
		ReviewerRisks:      reviewerWritebackStringList(event["reviewerRisks"]),
		ReviewerConflicts:  reviewerWritebackStringList(event["reviewerConflicts"]),
		RouteOutput:        reviewerWritebackStringMap(event["routeOutput"]),
		EvidenceRefs:       reviewerWritebackStringList(event["evidenceRefs"]),
	}
	if item.PacketID == "" && item.RouteID == "" && item.ShardID == "" && item.ReviewerSession == "" && item.ReviewerResultPath == "" {
		return ReviewerWritebackItem{}, false
	}
	return item, true
}

func reviewerWritebackStringList(value any) []string {
	out := []string{}
	add := func(value string) {
		for _, part := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || r == '\n' || r == '\r' }) {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	switch t := value.(type) {
	case nil:
		return nil
	case []string:
		for _, item := range t {
			add(item)
		}
	case []any:
		for _, item := range t {
			add(objectText(item))
		}
	default:
		add(objectText(t))
	}
	return mission.UniqueStrings(out)
}

func reviewerWritebackStringMap(value any) map[string]string {
	out := map[string]string{}
	add := func(key, value string) {
		key = strings.TrimSpace(key)
		value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
		if key != "" && value != "" {
			out[key] = value
		}
	}
	switch t := value.(type) {
	case nil:
		return nil
	case map[string]string:
		for key, value := range t {
			add(key, value)
		}
	case map[string]any:
		for key, value := range t {
			add(key, objectText(value))
		}
	case map[any]any:
		for key, value := range t {
			add(objectText(key), objectText(value))
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func reviewerWritebackRouteOutputLines(routeOutput map[string]string) []string {
	if len(routeOutput) == 0 {
		return nil
	}
	keys := make([]string, 0, len(routeOutput))
	for key := range routeOutput {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, key+"="+routeOutput[key])
	}
	return lines
}

func writeReviewerWritebackEventDetail(out *bytes.Buffer, prefix string, item ReviewerWritebackItem) {
	if item.PacketID != "" || item.RouteID != "" || item.ShardID != "" {
		fmt.Fprintf(out, "%s- reviewer packet: packetId=%s routeId=%s shardId=%s\n", prefix, item.PacketID, item.RouteID, item.ShardID)
	}
	if item.ReviewerSession != "" {
		fmt.Fprintf(out, "%s- reviewer session: %s\n", prefix, item.ReviewerSession)
	}
	if item.ReviewerResultPath != "" {
		fmt.Fprintf(out, "%s- reviewer result: `%s`\n", prefix, item.ReviewerResultPath)
	}
	if item.OwnerBindingTarget != "" || item.OwnerBindingMode != "" || item.OwnerExecutor != "" || item.OwnerGeneration != "" {
		fmt.Fprintf(out, "%s- reviewer owner binding: target=%s mode=%s executor=%s generation=%s\n", prefix, item.OwnerBindingTarget, item.OwnerBindingMode, item.OwnerExecutor, item.OwnerGeneration)
	}
	if item.ReviewerDecision != "" || item.RecommendedVerdict != "" {
		fmt.Fprintf(out, "%s- reviewer decision detail: reviewerDecision=%s recommendedVerdict=%s\n", prefix, item.ReviewerDecision, item.RecommendedVerdict)
	}
	for _, risk := range mission.LimitStrings(item.ReviewerRisks, maxHandoffRows) {
		fmt.Fprintf(out, "%s- reviewer risk: %s\n", prefix, risk)
	}
	for _, conflict := range mission.LimitStrings(item.ReviewerConflicts, maxHandoffRows) {
		fmt.Fprintf(out, "%s- reviewer conflict: %s\n", prefix, conflict)
	}
	for _, line := range mission.LimitStrings(reviewerWritebackRouteOutputLines(item.RouteOutput), maxHandoffRows) {
		fmt.Fprintf(out, "%s- reviewer route output: %s\n", prefix, line)
	}
	for _, ref := range mission.LimitStrings(item.EvidenceRefs, maxHandoffRows) {
		fmt.Fprintf(out, "%s- reviewer evidence ref: %s\n", prefix, ref)
	}
}

func writeReviewerWritebackItems(out *bytes.Buffer, items []ReviewerWritebackItem) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintln(out, "## reviewer writeback")
	fmt.Fprintln(out)
	for _, item := range mission.LimitStrings(reviewerWritebackMarkdownLines(items), maxHandoffRows*6) {
		fmt.Fprintln(out, item)
	}
	fmt.Fprintln(out)
}

func reviewerWritebackMarkdownLines(items []ReviewerWritebackItem) []string {
	lines := []string{}
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("- %s | eventId=%s | lane=%s | shard=%s | reviewerSession=%s | verdict=%s | decision=%s", item.Kind, item.EventID, item.Lane, item.ShardID, item.ReviewerSession, item.Verdict, item.Decision))
		if item.PacketID != "" || item.RouteID != "" {
			lines = append(lines, fmt.Sprintf("  - packet: packetId=%s routeId=%s", item.PacketID, item.RouteID))
		}
		if item.Target != "" || item.Confidence != "" || item.BatchID != "" {
			lines = append(lines, fmt.Sprintf("  - target: %s confidence=%s batchId=%s", item.Target, item.Confidence, item.BatchID))
		}
		if item.ReviewerResultPath != "" {
			lines = append(lines, "  - reviewer result: `"+item.ReviewerResultPath+"`")
		}
		if item.ReviewerDecision != "" || item.RecommendedVerdict != "" {
			lines = append(lines, fmt.Sprintf("  - reviewer decision detail: reviewerDecision=%s recommendedVerdict=%s", item.ReviewerDecision, item.RecommendedVerdict))
		}
		for _, risk := range mission.LimitStrings(item.ReviewerRisks, maxHandoffRows) {
			lines = append(lines, "  - reviewer risk: "+risk)
		}
		for _, conflict := range mission.LimitStrings(item.ReviewerConflicts, maxHandoffRows) {
			lines = append(lines, "  - reviewer conflict: "+conflict)
		}
		for _, line := range mission.LimitStrings(reviewerWritebackRouteOutputLines(item.RouteOutput), maxHandoffRows) {
			lines = append(lines, "  - reviewer route output: "+line)
		}
		for _, ref := range mission.LimitStrings(item.EvidenceRefs, maxHandoffRows) {
			lines = append(lines, "  - evidence ref: "+ref)
		}
	}
	return lines
}

func appendResumeReviewerWritebacks(lines []string, items []ReviewerWritebackItem) []string {
	lines = append(lines, "", "## Reviewer writeback", "")
	if len(items) == 0 {
		return append(lines, "- none")
	}
	return append(lines, reviewerWritebackMarkdownLines(items)...)
}

func appendDigestReviewerWritebacks(lines []string, items []ReviewerWritebackItem) []string {
	lines = append(lines, "", "## Reviewer writeback", "")
	if len(items) == 0 {
		return append(lines, "- none")
	}
	return append(lines, reviewerWritebackMarkdownLines(items)...)
}
