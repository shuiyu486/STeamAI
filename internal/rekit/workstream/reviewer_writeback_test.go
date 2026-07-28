package workstream

import (
	"fmt"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestReviewerWritebackItemsSortsAcrossLedgerKindsByCreatedAt(t *testing.T) {
	facts := mission.LedgerFacts{}
	facts.Verifications = []map[string]any{
		reviewerWritebackTestEvent("verification-newest", "2026-07-28T12:00:00Z", "shard-newest"),
		reviewerWritebackTestEvent("verification-oldest", "2026-07-28T10:00:00Z", "shard-oldest"),
	}
	facts.Decisions = []map[string]any{
		reviewerWritebackTestEvent("decision-middle", "2026-07-28T11:00:00Z", "shard-middle"),
	}

	items := ReviewerWritebackItems(facts, "main")
	if len(items) != 3 || items[0].EventID != "verification-oldest" || items[1].EventID != "decision-middle" || items[2].EventID != "verification-newest" {
		t.Fatalf("reviewer writebacks were not globally ordered: %+v", items)
	}
	summary := ReviewerWritebackSummaryFor(items)
	if summary.LatestEventID != "verification-newest" || summary.LatestKind != "verification" || summary.LatestShardID != "shard-newest" {
		t.Fatalf("reviewer writeback latest summary did not use global event time: %+v", summary)
	}
}

func TestReviewerWritebackItemsLimitsGloballyNewestEvents(t *testing.T) {
	facts := mission.LedgerFacts{}
	facts.Verifications = []map[string]any{
		reviewerWritebackTestEvent("verification-7", "2026-07-28T07:00:00Z", "shard-7"),
		reviewerWritebackTestEvent("verification-1", "2026-07-28T01:00:00Z", "shard-1"),
		reviewerWritebackTestEvent("verification-6", "2026-07-28T06:00:00Z", "shard-6"),
	}
	facts.Decisions = []map[string]any{
		reviewerWritebackTestEvent("decision-2", "2026-07-28T02:00:00Z", "shard-2"),
		reviewerWritebackTestEvent("decision-5", "2026-07-28T05:00:00Z", "shard-5"),
		reviewerWritebackTestEvent("decision-3", "2026-07-28T03:00:00Z", "shard-3"),
		reviewerWritebackTestEvent("decision-4", "2026-07-28T04:00:00Z", "shard-4"),
	}

	items := ReviewerWritebackItems(facts, "main")
	if len(items) != maxHandoffRows {
		t.Fatalf("reviewer writeback row limit = %d, want %d", len(items), maxHandoffRows)
	}
	for idx, want := range []string{"decision-3", "decision-4", "decision-5", "verification-6", "verification-7"} {
		if items[idx].EventID != want {
			t.Fatalf("reviewer writeback item %d = %q, want %q: %+v", idx, items[idx].EventID, want, items)
		}
	}
}

func TestReviewerWritebackItemsKeepsLegacyOrderBeforeTimestampedEvents(t *testing.T) {
	facts := mission.LedgerFacts{}
	facts.Verifications = []map[string]any{
		reviewerWritebackTestEvent("verification-legacy", "", "shard-legacy"),
		reviewerWritebackTestEvent("verification-timed", "2026-07-28T12:00:00Z", "shard-timed"),
	}
	facts.Decisions = []map[string]any{
		reviewerWritebackTestEvent("decision-invalid-time", "not-a-time", "shard-invalid"),
	}

	items := ReviewerWritebackItems(facts, "main")
	if len(items) != 3 || items[0].EventID != "verification-legacy" || items[1].EventID != "decision-invalid-time" || items[2].EventID != "verification-timed" {
		t.Fatalf("legacy reviewer writeback fallback order drifted: %+v", items)
	}
}

func reviewerWritebackTestEvent(eventID, createdAt, shardID string) map[string]any {
	event := map[string]any{
		"eventId":            eventID,
		"lane":               "main",
		"packetId":           "packet-1",
		"routeId":            "route-1",
		"shardId":            shardID,
		"reviewerSession":    "reviewer-session-1",
		"reviewerResultPath": fmt.Sprintf("results/%s.json", shardID),
		"reviewerDecision":   "accept",
		"recommendedVerdict": "accepted",
	}
	if createdAt != "" {
		event["createdAt"] = createdAt
	}
	return event
}
