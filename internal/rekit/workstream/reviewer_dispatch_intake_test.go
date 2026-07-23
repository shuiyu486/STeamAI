package workstream

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestReviewerDispatchIntakeSummaryPrefersReadyPacketBatchCommand(t *testing.T) {
	readyBatchPreview := "/rekit plan-subagents -PacketPath ready-packet.json -ReadyReviewerResults -WhatIf -Format json"
	readyBatchApply := "/rekit plan-subagents -PacketPath ready-packet.json -ReadyReviewerResults -Apply -Format json"
	items := []ReviewerDispatchIntakeHandoff{
		{
			PacketID:            "packet-ready",
			PacketPath:          "ready-packet.json",
			TargetLane:          "feature-review",
			ShardID:             "shard-01",
			State:               "ready-for-reviewer-intake-preview",
			PreviewCommand:      "/rekit plan-subagents -ReviewerResultPath ready.json -WhatIf -Format json",
			ApplyCommand:        "/rekit plan-subagents -ReviewerResultPath ready.json -Apply -Format json",
			BatchPreviewCommand: readyBatchPreview,
			BatchApplyCommand:   readyBatchApply,
		},
		{
			PacketID:            "packet-waiting",
			PacketPath:          "waiting-packet.json",
			TargetLane:          "feature-review",
			ShardID:             "shard-02",
			State:               "waiting-for-reviewer-result",
			BatchPreviewCommand: "/rekit plan-subagents -PacketPath waiting-packet.json -ReadyReviewerResults -WhatIf -Format json",
			BatchApplyCommand:   "/rekit plan-subagents -PacketPath waiting-packet.json -ReadyReviewerResults -Apply -Format json",
		},
	}

	summary := ReviewerDispatchIntakeSummaryFor(items)
	if summary.ReadyForPreview != 1 || summary.LatestBatchPreviewCommand != readyBatchPreview || summary.LatestBatchApplyCommand != readyBatchApply || summary.NextAction != readyBatchPreview {
		t.Fatalf("summary did not promote the ready packet batch command: %+v", summary)
	}
}

func TestMissionCommanderReviewerDispatchSelectsActionablePacketRepresentative(t *testing.T) {
	readyCommand := "/rekit plan-subagents -PacketPath packet.json -ReadyReviewerResults -WhatIf -Format json"
	items := MissionCommanderNextActionsWithReviewerDispatches(nil, []ReviewerDispatchIntakeHandoff{
		{PacketID: "packet-mixed", TargetLane: "feature-review", ShardID: "shard-waiting", State: "waiting-for-reviewer-result"},
		{PacketID: "packet-mixed", TargetLane: "feature-review", ShardID: "shard-ready", State: "ready-for-reviewer-intake-preview", BatchPreviewCommand: readyCommand},
	})
	queue := mission.MissionCommanderActionQueueFor(items)
	if len(items) != 1 || queue.CurrentAction == nil || queue.CurrentAction.Command != readyCommand || queue.CurrentAction.Blocked {
		t.Fatalf("mixed packet did not promote ready reviewer intake: items=%+v queue=%+v", items, queue)
	}
}

func TestMissionCommanderReviewerDispatchPreservesBoundedApplyPriority(t *testing.T) {
	startCommand := "/rekit start review -Apply"
	items := MissionCommanderNextActionsWithReviewerDispatches([]mission.MissionCommanderNextActionItem{{
		Lane: "feature-review", State: "needs-start-apply", Command: startCommand, Source: "missionCommanderActions",
	}}, []ReviewerDispatchIntakeHandoff{{
		PacketID: "packet-stale", TargetLane: "feature-review", State: "reviewer-packet-owner-adoption-required", OwnerAdoptionPreviewCommand: "adopt-preview",
	}})
	queue := mission.MissionCommanderActionQueueFor(items)
	if queue.CurrentAction == nil || queue.CurrentAction.Command != startCommand {
		t.Fatalf("bounded start apply did not remain current: %+v", queue)
	}
}

func TestMissionCommanderReviewerDispatchKeepsPacketsDistinct(t *testing.T) {
	items := MissionCommanderNextActionsWithReviewerDispatches(nil, []ReviewerDispatchIntakeHandoff{
		{PacketID: "packet-a", TargetLane: "feature-review", State: "waiting-for-reviewer-result", ReviewerResultPath: "shared.json"},
		{PacketID: "packet-b", TargetLane: "feature-review", State: "waiting-for-reviewer-result", ReviewerResultPath: "shared.json"},
	})
	if len(items) != 2 || items[0].ActionID == items[1].ActionID {
		t.Fatalf("reviewer packet actions were deduplicated across packet identity: %+v", items)
	}
}

func TestReviewerDispatchIntakeHandoffMatchesStrictResultClassification(t *testing.T) {
	root := t.TempDir()
	readyPath := filepath.Join(root, "ready.json")
	if err := os.WriteFile(readyPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	emptyPath := filepath.Join(root, "empty.json")
	if err := os.WriteFile(emptyPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	directoryPath := filepath.Join(root, "directory.json")
	if err := os.Mkdir(directoryPath, 0o755); err != nil {
		t.Fatal(err)
	}

	packet := reviewerDispatchPacket{
		PacketID: "packet-classification",
		ReviewerOrchestration: reviewerDispatchPacketOrchestration{
			TargetLane: "feature-review",
			Dispatches: []reviewerDispatchPacketDispatch{{ShardID: "shard-01"}},
		},
	}
	for _, test := range []struct {
		name      string
		path      string
		wantState string
		wantFile  refsf.RegularFileState
	}{
		{name: "missing", path: filepath.Join(root, "missing.json"), wantState: "waiting-for-reviewer-result", wantFile: refsf.RegularFileMissing},
		{name: "empty", path: emptyPath, wantState: "waiting-for-reviewer-result", wantFile: refsf.RegularFileWaiting},
		{name: "directory", path: directoryPath, wantState: "waiting-for-reviewer-result", wantFile: refsf.RegularFileWaiting},
		{name: "ready", path: readyPath, wantState: "ready-for-reviewer-intake-preview", wantFile: refsf.RegularFileReady},
	} {
		t.Run(test.name, func(t *testing.T) {
			dispatch := reviewerDispatchPacketDispatch{ShardID: "shard-01", ReviewerResultPath: test.path, PreviewCommand: "preview", ApplyCommand: "apply"}
			item := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, "packet.json", "feature-review", dispatch, 0)
			if item.State != test.wantState || item.ReviewerResultState != string(test.wantFile) || item.ReviewerResultPresent != (test.wantFile == refsf.RegularFileReady) {
				t.Fatalf("unexpected %s classification: %+v", test.name, item)
			}
		})
	}

	if runtime.GOOS != "windows" {
		symlinkPath := filepath.Join(root, "symlink.json")
		if err := os.Symlink(readyPath, symlinkPath); err != nil {
			t.Fatal(err)
		}
		dispatch := reviewerDispatchPacketDispatch{ShardID: "shard-01", ReviewerResultPath: symlinkPath, PreviewCommand: "preview", ApplyCommand: "apply"}
		item := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, "packet.json", "feature-review", dispatch, 0)
		if item.State != "reviewer-result-symlink-blocked" || item.ReviewerResultState != string(refsf.RegularFileSymlink) || !strings.Contains(reviewerDispatchIntakeNextAction(item), "replace the symlink") {
			t.Fatalf("unexpected symlink classification: %+v", item)
		}
	}
}

func TestReviewerDispatchIntakeRequiresAdoptionWhenUnassignedPacketGetsExecutor(t *testing.T) {
	root := t.TempDir()
	metadataRoot := filepath.Join(root, ".rekit")
	if err := os.MkdirAll(metadataRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	board := map[string]any{
		"schemaVersion": 1,
		"lanes": []map[string]any{{
			"id":                 "feature-review",
			"status":             "open",
			"currentExecutor":    "session-a",
			"executorGeneration": 1,
		}},
	}
	boardBytes, err := json.Marshal(board)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metadataRoot, "board.json"), boardBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	packetPath := filepath.Join(root, "packet.json")
	if err := os.WriteFile(packetPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	packet := reviewerDispatchPacket{
		PacketID:   "packet-unassigned",
		TargetLane: "feature-review",
		ReviewerOrchestration: reviewerDispatchPacketOrchestration{
			TargetLane:   "feature-review",
			OwnerBinding: reviewerDispatchPacketOwner{TargetLane: "feature-review", RequiredForIntake: true},
			Dispatches:   []reviewerDispatchPacketDispatch{{ShardID: "shard-01"}},
		},
	}

	item := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, packetPath, "feature-review", packet.ReviewerOrchestration.Dispatches[0], 0)
	if item.State != "reviewer-packet-owner-adoption-required" || !item.OwnerAdoptionRequired || item.CurrentExecutor != "session-a" || item.CurrentGeneration != 1 {
		t.Fatalf("unassigned packet owner was not classified stale after executor claim: %+v", item)
	}
}

func TestReviewerDispatchAdoptionCurrentRejectsForgedReceipt(t *testing.T) {
	root := t.TempDir()
	packetPath := filepath.Join(root, "packet.json")
	packetBytes := []byte("{\"packetId\":\"packet-strict\"}\n")
	if err := os.WriteFile(packetPath, packetBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	owner := reviewerDispatchPacketOwner{
		TargetLane:             "feature-review",
		CurrentExecutor:        "session-a",
		ExecutorGeneration:     1,
		BindingMode:            "durable-lane-executor",
		RequiredForIntake:      true,
		MainAgentSpawnOwner:    "main-agent",
		RuntimeSessionBoundary: "replaceable-session",
	}
	packet := reviewerDispatchPacket{
		PacketID:   "packet-strict",
		RepoRoot:   filepath.Join(root, "repo"),
		Pack:       "_template",
		TargetLane: "feature-review",
		ReviewerOrchestration: reviewerDispatchPacketOrchestration{
			OwnerBinding: owner,
		},
	}
	sum := sha256.Sum256(packetBytes)
	adoptedOwner := owner
	adoptedOwner.CurrentExecutor = "session-b"
	adoptedOwner.ExecutorGeneration = 2
	adoptedOwner.BindingMode = "durable-lane-executor-adoption"
	receipt := map[string]any{
		"schemaVersion":          1,
		"kind":                   "reviewer-packet-owner-adoption",
		"packetId":               packet.PacketID,
		"packetPath":             packetPath,
		"packetSha256":           hex.EncodeToString(sum[:]),
		"repoRoot":               packet.RepoRoot,
		"caseRoot":               root,
		"pack":                   packet.Pack,
		"lane":                   packet.TargetLane,
		"dispatchedOwner":        owner,
		"adoptedOwner":           adoptedOwner,
		"actor":                  "main-agent",
		"reason":                 "replacement executor takeover",
		"createdAt":              "2026-07-23T00:00:00Z",
		"noSpawn":                true,
		"noHeavyTool":            true,
		"noAuthorityOrConfirmed": true,
	}
	adoptionPath := filepath.Join(root, "adoption.json")
	writeReceipt := func(value map[string]any) {
		t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(adoptionPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeReceipt(receipt)
	if !reviewerDispatchAdoptionCurrent(root, adoptionPath, packet, packetPath, "session-b", 2) {
		t.Fatal("exact adoption receipt was not accepted")
	}

	for _, test := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "unknown-field", mutate: func(value map[string]any) { value["forged"] = true }},
		{name: "wrong-case", mutate: func(value map[string]any) { value["caseRoot"] = filepath.Join(root, "other") }},
		{name: "spawn-boundary", mutate: func(value map[string]any) { value["noSpawn"] = false }},
		{name: "blank-reason", mutate: func(value map[string]any) { value["reason"] = "" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			copyReceipt := make(map[string]any, len(receipt))
			maps.Copy(copyReceipt, receipt)
			test.mutate(copyReceipt)
			writeReceipt(copyReceipt)
			if reviewerDispatchAdoptionCurrent(root, adoptionPath, packet, packetPath, "session-b", 2) {
				t.Fatalf("forged adoption receipt %s was accepted", test.name)
			}
		})
	}
}

func TestLimitReviewerDispatchIntakeHandoffsPreservesReadyPacket(t *testing.T) {
	items := []ReviewerDispatchIntakeHandoff{{ShardID: "ready", State: "ready-for-reviewer-intake-preview", BatchPreviewCommand: "ready-batch"}}
	for idx := range 6 {
		items = append(items, ReviewerDispatchIntakeHandoff{ShardID: "waiting-" + string(rune('a'+idx)), State: "waiting-for-reviewer-result"})
	}

	limited := limitReviewerDispatchIntakeHandoffs(items, 5)
	if len(limited) != 5 || !slices.ContainsFunc(limited, func(item ReviewerDispatchIntakeHandoff) bool { return item.ShardID == "ready" }) {
		t.Fatalf("limited handoffs dropped the actionable ready packet: %+v", limited)
	}
}

func TestLimitReviewerDispatchIntakeHandoffsPreservesStaleAdoptionBeforeReady(t *testing.T) {
	items := []ReviewerDispatchIntakeHandoff{{ShardID: "adopt", State: "reviewer-packet-owner-adoption-required"}, {ShardID: "ready", State: "ready-for-reviewer-intake-preview"}}
	for idx := range 5 {
		items = append(items, ReviewerDispatchIntakeHandoff{ShardID: "waiting-" + string(rune('a'+idx)), State: "waiting-for-reviewer-result"})
	}

	limited := limitReviewerDispatchIntakeHandoffs(items, 5)
	if !slices.ContainsFunc(limited, func(item ReviewerDispatchIntakeHandoff) bool { return item.ShardID == "adopt" }) {
		t.Fatalf("limited handoffs dropped stale adoption action: %+v", limited)
	}
}

func TestReviewerDispatchIntakeSummaryFallsBackForLegacyPacket(t *testing.T) {
	legacyPreview := "/rekit plan-subagents -ReviewerResultPath legacy.json -WhatIf -Format json"
	items := []ReviewerDispatchIntakeHandoff{{
		PacketID:       "packet-legacy",
		PacketPath:     "legacy-packet.json",
		TargetLane:     "feature-review",
		ShardID:        "shard-01",
		State:          "ready-for-reviewer-intake-preview",
		PreviewCommand: legacyPreview,
		ApplyCommand:   "/rekit plan-subagents -ReviewerResultPath legacy.json -Apply -Format json",
	}}

	summary := ReviewerDispatchIntakeSummaryFor(items)
	if summary.NextAction != legacyPreview || summary.LatestBatchPreviewCommand != "" || strings.Contains(summary.NextAction, "-ReadyReviewerResults") {
		t.Fatalf("legacy packet did not retain single-result intake fallback: %+v", summary)
	}
}
