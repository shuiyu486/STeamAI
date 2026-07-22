package workstream

import (
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
