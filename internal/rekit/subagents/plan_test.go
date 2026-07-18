package subagents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
)

func TestWritePlanIncludesShardHandoffs(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	target := t.TempDir()
	reviewRoot := filepath.Join(t.TempDir(), "review")

	result, err := WritePlan(repoRoot, target, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha,beta gamma", ItemsPerAgent: 2, MaxParallel: 5, ReviewOutputDir: reviewRoot})
	if err != nil {
		t.Fatal(err)
	}
	if result.Command != commandName || result.IsMutation || !result.WritesReviewArtifacts || !result.ReviewRequired || result.ItemCount != 3 || result.ShardCount != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(result.ShardHandoffs) != 2 {
		t.Fatalf("ShardHandoffs = %+v, want 2", result.ShardHandoffs)
	}
	assertShardHandoff(t, result.ShardHandoffs[0], "shard-01", []string{"alpha", "beta"})

	packetBytes, err := os.ReadFile(result.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	var packet Packet
	if err := json.Unmarshal(packetBytes, &packet); err != nil {
		t.Fatalf("packet JSON did not decode: %v\n%s", err, string(packetBytes))
	}
	if packet.Command != commandName || packet.Route.ID != defaults.DefaultPack+":lane-feature-analysis" || len(packet.ShardHandoffs) != 2 {
		t.Fatalf("unexpected packet: %+v", packet)
	}
	assertShardHandoff(t, packet.ShardHandoffs[0], "shard-01", []string{"alpha", "beta"})

	summary, err := os.ReadFile(result.SummaryPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"### shard handoff prompts", "read-only reviewer", "Do not write files", "expected output=`item,decision"} {
		if !strings.Contains(string(summary), expected) {
			t.Fatalf("summary missing %q:\n%s", expected, string(summary))
		}
	}
}

func TestWritePlanNoItemsKeepsEmptyShardHandoffs(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	result, err := WritePlan(repoRoot, t.TempDir(), defaults.DefaultPack, Options{Route: defaults.DefaultPack + ":bounded-review", ReviewOutputDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if result.ItemCount != 0 || result.ShardCount != 0 || len(result.ShardHandoffs) != 0 {
		t.Fatalf("unexpected empty plan: %+v", result)
	}
	summary, err := os.ReadFile(result.SummaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(summary), "- no shard handoffs planned") {
		t.Fatalf("summary missing empty handoff marker:\n%s", string(summary))
	}
}

func assertShardHandoff(t *testing.T, handoff ShardHandoff, wantID string, wantItems []string) {
	t.Helper()
	if handoff.ShardID != wantID || handoff.Status != "planned" || strings.Join(handoff.Items, ",") != strings.Join(wantItems, ",") {
		t.Fatalf("unexpected shard handoff identity: %+v", handoff)
	}
	for _, expected := range []string{"read-only reviewer", "Do not write files", "note -Kind verification"} {
		if !strings.Contains(handoff.DispatchPrompt, expected) {
			t.Fatalf("dispatch prompt missing %q: %+v", expected, handoff)
		}
	}
	if !strings.Contains(handoff.ExpectedOutput, "decision") || !strings.Contains(handoff.ReviewerWriteback, "note -Kind verification") || !strings.Contains(handoff.MainAgentNextAction, "launch a read-only reviewer") {
		t.Fatalf("unexpected reviewer contract: %+v", handoff)
	}
	if !slices.Contains(handoff.ReadOnlyBoundary, "runtime does not spawn subagents") || !slices.Contains(handoff.ReadOnlyBoundary, "subagents must not write files") {
		t.Fatalf("missing read-only boundary: %+v", handoff)
	}
	if !slices.Contains(handoff.CompletionCriteria, "reviewer verdicts are recorded in the ledger before main merge decisions") || handoff.FailureHandling == "" {
		t.Fatalf("missing completion/failure guidance: %+v", handoff)
	}
}
