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
	"time"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestReviewerDispatchIntakeFailsClosedOnPacketIntegrityDrift(t *testing.T) {
	root := t.TempDir()
	reviewRoot := filepath.Join(root, ".rekit", "reviews", "integrity")
	packetPath := filepath.Join(reviewRoot, "packet.json")
	integrityPath := filepath.Join(reviewRoot, "packet.integrity.json")
	resultRoot := filepath.Join(reviewRoot, "results")
	if err := os.MkdirAll(resultRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	packet := reviewerDispatchPacket{
		PacketID:        "packet-integrity",
		PacketIntegrity: &reviewerPacketIntegrityReference{Algorithm: "sha256", Path: integrityPath},
		Command:         "plan-subagents",
		TargetLane:      "feature-review",
		ReviewerOrchestration: reviewerDispatchPacketOrchestration{
			TargetLane: "feature-review",
			PacketPath: packetPath,
			ResultRoot: resultRoot,
			Dispatches: []reviewerDispatchPacketDispatch{{ShardID: "shard-01", ReviewerResultPath: filepath.Join(resultRoot, "shard-01.json"), PreviewCommand: "preview", ApplyCommand: "apply"}},
		},
	}
	packetData, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	packetData = append(packetData, '\n')
	if err := os.WriteFile(packetPath, packetData, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(packetData)
	integrity := reviewerPacketIntegrity{SchemaVersion: 1, Kind: "reviewer-packet-integrity", Algorithm: "sha256", PacketID: packet.PacketID, TargetLane: packet.TargetLane, PacketPath: packetPath, PacketSHA256: hex.EncodeToString(sum[:]), PacketBytes: len(packetData)}
	integrityData, err := json.Marshal(integrity)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(integrityPath, append(integrityData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	items, err := ReviewerDispatchIntakeHandoffs(root, mission.LedgerFacts{}, "feature-review")
	if err != nil || len(items) != 1 || items[0].State != "waiting-for-reviewer-result" {
		t.Fatalf("valid integrity packet was not projected: items=%+v err=%v", items, err)
	}
	var tampered reviewerDispatchPacket
	if err := json.Unmarshal(packetData, &tampered); err != nil {
		t.Fatal(err)
	}
	tampered.TargetLane = "other-lane"
	tampered.ReviewerOrchestration.TargetLane = "other-lane"
	tamperedData, err := json.Marshal(tampered)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packetPath, append(tamperedData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	items, err = ReviewerDispatchIntakeHandoffs(root, mission.LedgerFacts{}, "feature-review")
	if err != nil || len(items) != 1 || items[0].State != "reviewer-packet-integrity-invalid" || items[0].IntakeAvailable || items[0].ReviewerResultCollectionCommands != nil || !strings.Contains(reviewerDispatchIntakeNextAction(items[0]), "regenerate canonical reviewer packet") {
		t.Fatalf("packet integrity drift did not fail closed: items=%+v err=%v", items, err)
	}
	actions := MissionCommanderNextActionsWithReviewerDispatches(nil, items)
	if len(actions) != 1 || !actions[0].Blocked || actions[0].State != "reviewer-packet-integrity-invalid" {
		t.Fatalf("integrity drift did not reach blocked Mission Commander action: %+v", actions)
	}
	if err := os.WriteFile(packetPath, []byte("{truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	items, err = ReviewerDispatchIntakeHandoffs(root, mission.LedgerFacts{}, "feature-review")
	if err != nil || len(items) != 1 || items[0].State != "reviewer-packet-integrity-invalid" || items[0].TargetLane != "feature-review" {
		t.Fatalf("truncated packet lost integrity lane provenance: items=%+v err=%v", items, err)
	}
	if !reviewerDispatchTestContainsSubstring(items[0].Evidence, "decode reviewer packet JSON") || !reviewerDispatchTestContainsSubstring(items[0].Evidence, "while integrity metadata remains") {
		t.Fatalf("truncated packet omitted concrete decode evidence: %+v", items[0].Evidence)
	}
}

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
	if summary.ReadyForPreview != 1 || summary.LatestBatchPreviewCommand != readyBatchPreview || summary.LatestBatchApplyCommand != readyBatchApply || summary.NextAction != readyBatchPreview || summary.NextActionShardID != "shard-01" || summary.NextActionState != "ready-for-reviewer-intake-preview" || summary.NextActionBatchPreviewCommand != readyBatchPreview || summary.NextActionBatchApplyCommand != readyBatchApply {
		t.Fatalf("summary did not promote the ready packet batch command: %+v", summary)
	}
}

func TestReviewerDispatchIntakeRunbookStepsCoverReviewerLifecycle(t *testing.T) {
	collectionCommands := &ReviewerResultCollectionCommands{PreviewCommand: "collect-preview", ApplyCommand: "collect-apply"}
	for _, tc := range []struct {
		name  string
		item  ReviewerDispatchIntakeHandoff
		wants []string
	}{
		{
			name:  "waiting dispatch",
			item:  ReviewerDispatchIntakeHandoff{State: "waiting-for-reviewer-result", ShardID: "shard-01", TargetLane: "feature-review", ReviewerResultPath: "results/shard-01.json", ReviewerResultSourcePath: "results/sources/shard-01.json", ReviewerResultSourceCaptureCommand: "capture-preview", ReviewerResultSourceCaptureApplyCommand: "capture-apply", ReviewerResultStagingCommand: "stage-preview", DispatchCommand: "dispatch read-only reviewer for shard-01"},
			wants: []string{"dispatch read-only reviewer for shard-01", "after saving reviewer JSON input, run source capture preview: capture-preview", "expected input hash", "after source capture publishes reviewerResultSourcePath, run staging preview: stage-preview", "do not continue the lane"},
		},
		{
			name:  "staging ready",
			item:  ReviewerDispatchIntakeHandoff{State: "ready-for-reviewer-result-staging-preview", ShardID: "shard-01", ReviewerResultSourcePath: "results/sources/shard-01.json", ReviewerResultStagingCommand: "stage-preview", ReviewerResultCollectionCommands: collectionCommands},
			wants: []string{"reviewer result source is ready", "run staging preview: stage-preview", "-ExpectedSourceSha256", "collection preview before apply: collect-preview"},
		},
		{
			name:  "collection ready",
			item:  ReviewerDispatchIntakeHandoff{State: "ready-for-reviewer-result-collection-preview", ShardID: "shard-01", ReviewerResultCollectionCommands: collectionCommands},
			wants: []string{"run reviewer result collection preview: collect-preview", "run collection apply: collect-apply", "ready-for-reviewer-intake-preview"},
		},
		{
			name:  "intake ready",
			item:  ReviewerDispatchIntakeHandoff{State: "ready-for-reviewer-intake-preview", ShardID: "shard-01", PreviewCommand: "single-preview", ApplyCommand: "single-apply", BatchPreviewCommand: "batch-preview", BatchApplyCommand: "batch-apply"},
			wants: []string{"run reviewer intake preview: batch-preview", "inspect verification, decision, postValidation", "bounded apply command: batch-apply"},
		},
		{
			name:  "owner adoption",
			item:  ReviewerDispatchIntakeHandoff{State: "reviewer-packet-owner-adoption-required", ShardID: "shard-01", TargetLane: "feature-review", OwnerAdoptionPreviewCommand: "adopt-preview"},
			wants: []string{"owner adoption preview: adopt-preview", "review adoptedOwner/currentExecutor/currentGeneration", "/rekit continue feature-review -WhatIf"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			steps := reviewerDispatchIntakeRunbookSteps(tc.item)
			for _, want := range tc.wants {
				if !slices.ContainsFunc(steps, func(step string) bool { return strings.Contains(step, want) }) {
					t.Fatalf("runbook steps missing %q: %+v", want, steps)
				}
			}
		})
	}
}

func TestReviewerDispatchIntakeSummaryProjectsWaitingNextAction(t *testing.T) {
	items := []ReviewerDispatchIntakeHandoff{
		{
			PacketID:                                "packet-waiting",
			PacketPath:                              "waiting-packet.json",
			TargetLane:                              "feature-review",
			ShardID:                                 "shard-01",
			State:                                   "waiting-for-reviewer-result",
			ReviewerResultSourcePath:                "results/sources/shard-01.json",
			ReviewerResultSourceState:               "missing",
			ReviewerResultCandidatePath:             "results/candidates/shard-01.json",
			ReviewerResultCandidateState:            "missing",
			AgentToolRequest:                        &ReviewerAgentToolRequest{Tool: "Claude Code Agent", AgentType: "read-only-reviewer", ReadOnly: true, Prompt: "review", ExpectedOutput: "one JSON object"},
			ReviewerResultSourceCaptureCommand:      "/rekit plan-subagents -CaptureReviewerResultSource -ShardId shard-01 -WhatIf -Format json",
			ReviewerResultSourceCaptureApplyCommand: "/rekit plan-subagents -CaptureReviewerResultSource -ShardId shard-01 -ExpectedReviewerResultInputSha256 <inputSha256-from-WhatIf> -Apply -Format json",
			ReviewerResultStagingCommand:            "/rekit plan-subagents -StageReviewerResult -ShardId shard-01 -WhatIf -Format json",
			DispatchCommand:                         "dispatch read-only reviewer for shard-01",
		},
		{
			PacketID:                 "packet-waiting",
			PacketPath:               "waiting-packet.json",
			TargetLane:               "feature-review",
			ShardID:                  "shard-02",
			State:                    "waiting-for-reviewer-result",
			ReviewerResultSourcePath: "results/sources/shard-02.json",
			DispatchCommand:          "dispatch read-only reviewer for shard-02",
		},
	}

	summary := ReviewerDispatchIntakeSummaryFor(items)
	if summary.WaitingForReviewerResult != 2 || summary.LatestShardID != "shard-02" || summary.NextActionShardID != "shard-01" || summary.NextActionState != "waiting-for-reviewer-result" || summary.NextActionReviewerResultSourcePath != "results/sources/shard-01.json" || summary.NextActionReviewerResultCandidatePath != "results/candidates/shard-01.json" || !strings.Contains(summary.NextActionReviewerResultSourceCaptureCommand, "-CaptureReviewerResultSource") || !strings.Contains(summary.NextActionReviewerResultSourceCaptureApplyCommand, "-ExpectedReviewerResultInputSha256") || !strings.Contains(summary.NextActionReviewerResultStagingCommand, "-StageReviewerResult") || !strings.Contains(summary.NextAction, "dispatch read-only reviewer for shard-01") || !slices.ContainsFunc(summary.NextActionRunbookSteps, func(step string) bool {
		return strings.Contains(step, "after saving reviewer JSON input, run source capture preview")
	}) || !slices.ContainsFunc(summary.NextActionRunbookSteps, func(step string) bool {
		return strings.Contains(step, "after source capture publishes reviewerResultSourcePath, run staging preview")
	}) {
		t.Fatalf("summary did not project first waiting next action separate from latest shard: %+v", summary)
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
			item := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, firstText(packet.ReviewerOrchestration.PacketPath, "packet.json"), "feature-review", dispatch, 0)
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
		item := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, firstText(packet.ReviewerOrchestration.PacketPath, "packet.json"), "feature-review", dispatch, 0)
		if item.State != "reviewer-result-symlink-blocked" || item.ReviewerResultState != string(refsf.RegularFileSymlink) || !strings.Contains(reviewerDispatchIntakeNextAction(item), "replace the symlink") {
			t.Fatalf("unexpected symlink classification: %+v", item)
		}
	}
}

func reviewerDispatchTestContainsSubstring(values []string, needle string) bool {
	return slices.ContainsFunc(values, func(value string) bool { return strings.Contains(value, needle) })
}

func TestReviewerDispatchPromptArtifactCurrentnessBlocksStaleDispatch(t *testing.T) {
	root := t.TempDir()
	reviewRoot := filepath.Join(root, ".rekit", "reviews", "prompt-currentness")
	packetPath := filepath.Join(reviewRoot, "packet.json")
	resultRoot := filepath.Join(reviewRoot, "results")
	promptPath := filepath.Join(reviewRoot, "prompts", "shard-01.prompt.md")
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(resultRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	promptBytes := []byte("read-only reviewer prompt\n")
	promptSHA := reviewerDispatchBytesSHA256(promptBytes)
	if err := os.WriteFile(promptPath, promptBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	packet := reviewerDispatchPacket{PacketID: "packet-prompt", ReviewerOrchestration: reviewerDispatchPacketOrchestration{TargetLane: "feature-review", PacketPath: packetPath, ResultRoot: resultRoot}}
	dispatch := reviewerDispatchPacketDispatch{
		ShardID:              "shard-01",
		ReviewerResultPath:   filepath.Join(resultRoot, "shard-01.json"),
		DispatchPromptPath:   promptPath,
		DispatchPromptSHA256: promptSHA,
		AgentToolRequest:     &ReviewerAgentToolRequest{Tool: "Claude Code Agent", AgentType: "read-only-reviewer", ReadOnly: true, PromptPath: promptPath, PromptSHA256: promptSHA, ExpectedOutput: "one JSON object"},
		PreviewCommand:       "preview",
		ApplyCommand:         "apply",
	}

	ready := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, packetPath, "feature-review", dispatch, 0)
	if ready.State != "waiting-for-reviewer-result" || ready.DispatchPromptState != "ready" || !ready.DispatchPromptCurrent || ready.DispatchPromptActualSHA256 != promptSHA || !reviewerDispatchTestContainsSubstring(ready.Evidence, "reviewerPrompt ready sha256=") {
		t.Fatalf("current prompt artifact was not projected as ready: %+v", ready)
	}

	if err := os.Remove(promptPath); err != nil {
		t.Fatal(err)
	}
	missing := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, packetPath, "feature-review", dispatch, 0)
	missingSummary := ReviewerDispatchIntakeSummaryFor([]ReviewerDispatchIntakeHandoff{missing})
	if missing.State != "reviewer-dispatch-prompt-artifact-invalid" || missing.DispatchPromptState != "missing" || missing.DispatchPromptCurrent || missingSummary.PromptArtifactBlocked != 1 || missing.DispatchPromptRepairCommand == "" || !strings.Contains(reviewerDispatchIntakeNextAction(missing), "-RepairReviewerPromptArtifact") || missingSummary.NextActionDispatchPromptRepairCommand != missing.DispatchPromptRepairCommand || !reviewerDispatchTestContainsSubstring(missing.Boundary, "reviewer prompt artifact must be present") || !reviewerDispatchTestContainsSubstring(missing.Boundary, "only creates a missing artifact") {
		t.Fatalf("missing prompt artifact did not block dispatch with repair handoff: item=%+v summary=%+v", missing, missingSummary)
	}

	driftBytes := []byte("modified prompt\n")
	if err := os.WriteFile(promptPath, driftBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	drift := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, packetPath, "feature-review", dispatch, 0)
	if drift.State != "reviewer-dispatch-prompt-artifact-drift" || drift.DispatchPromptState != "drift" || drift.DispatchPromptActualSHA256 != reviewerDispatchBytesSHA256(driftBytes) || !reviewerDispatchTestContainsSubstring(drift.Evidence, "actualSha256=") || !reviewerDispatchTestContainsSubstring(drift.Evidence, "failure=reviewer prompt artifact sha256 drift") {
		t.Fatalf("drifted prompt artifact did not fail closed: %+v", drift)
	}
}

func TestReviewerDispatchIntakeProjectsCandidateCollectionState(t *testing.T) {
	root := t.TempDir()
	reviewRoot := filepath.Join(root, ".rekit", "reviews", "review")
	packetPath := filepath.Join(reviewRoot, "packet.json")
	resultRoot := filepath.Join(reviewRoot, "results")
	candidatePath := filepath.Join(resultRoot, "candidates", "shard-01.json")
	resultPath := filepath.Join(resultRoot, "shard-01.json")
	if err := os.MkdirAll(resultRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packetPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(resultRoot, "sources", "shard-01.json")
	request := &ReviewerAgentToolRequest{Tool: "Claude Code Agent", AgentType: "read-only-reviewer", ReadOnly: true, Prompt: "review", ExpectedOutput: "one JSON object"}
	staging := &ReviewerResultStagingCommands{SourcePath: sourcePath, SourcePathArgument: sourcePath, PreviewCommand: "forged-staging-preview"}
	commands := &ReviewerResultCollectionCommands{CandidatePath: candidatePath, PreviewCommand: "collect-preview", ApplyCommand: "collect-apply"}
	packet := reviewerDispatchPacket{PacketID: "packet-collection", ReviewerOrchestration: reviewerDispatchPacketOrchestration{TargetLane: "feature-review", PacketPath: packetPath, ResultRoot: resultRoot, Dispatches: []reviewerDispatchPacketDispatch{{ShardID: "shard-01"}}}}
	dispatch := reviewerDispatchPacketDispatch{ShardID: "shard-01", ReviewerResultPath: resultPath, ReviewerResultCandidatePath: candidatePath, AgentToolRequest: request, StagingCommands: staging, CollectionCommands: commands, PreviewCommand: "intake-preview", ApplyCommand: "intake-apply"}

	missing := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, firstText(packet.ReviewerOrchestration.PacketPath, "packet.json"), "feature-review", dispatch, 0)
	if missing.State != "waiting-for-reviewer-result" || missing.ReviewerResultCandidateState != "missing" || missing.AgentToolRequest == nil || missing.ReviewerResultSourceCaptureCommand == "" || missing.ReviewerResultSourceCaptureApplyCommand == "" || missing.ReviewerResultStagingCommand == "" || missing.ReviewerResultCollectionCommands == nil || !strings.Contains(missing.DispatchCommand, "agentToolRequest.prompt") || !strings.Contains(missing.DispatchCommand, "source capture preview") || !strings.Contains(missing.DispatchCommand, "ExpectedReviewerResultInputSha256") || !strings.Contains(missing.DispatchCommand, "expected-source-hash Apply") {
		t.Fatalf("missing candidate handoff omitted typed dispatch/collection state: %+v", missing)
	}
	if missing.ReviewerResultSourcePath != sourcePath || missing.ReviewerResultSourceState != "missing" || !strings.Contains(missing.ReviewerResultSourceCaptureCommand, "-CaptureReviewerResultSource") || !strings.Contains(missing.ReviewerResultSourceCaptureCommand, "-ReviewerResultInputPath <case-local-reviewer-json-input>") || !strings.Contains(missing.ReviewerResultSourceCaptureApplyCommand, "-ExpectedReviewerResultInputSha256") || !strings.Contains(missing.ReviewerResultStagingCommand, "-StageReviewerResult") || !strings.Contains(missing.ReviewerResultStagingCommand, "-ReviewerResultSourcePath "+quoteCommandArg(sourcePath)) || strings.Contains(missing.ReviewerResultStagingCommand, "forged-staging-preview") {
		t.Fatalf("staging command was not rebuilt from canonical source bindings: %+v", missing)
	}
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceReady := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, packetPath, "feature-review", dispatch, 0)
	if sourceReady.State != "ready-for-reviewer-result-staging-preview" || sourceReady.ReviewerResultSourceState != "ready" || reviewerDispatchIntakeNextAction(sourceReady) != sourceReady.ReviewerResultStagingCommand || sourceReady.ReviewerResultCandidateState != "missing" {
		t.Fatalf("source-ready handoff did not promote staging preview: %+v", sourceReady)
	}
	if err := os.Remove(sourcePath); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(candidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(candidatePath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ready := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, firstText(packet.ReviewerOrchestration.PacketPath, "packet.json"), "feature-review", dispatch, 0)
	if ready.State != "ready-for-reviewer-result-collection-preview" || ready.ReviewerResultCandidateState != "ready" || ready.ReviewerResultCollectionCommands == nil || reviewerDispatchIntakeNextAction(ready) != ready.ReviewerResultCollectionCommands.PreviewCommand {
		t.Fatalf("ready candidate did not promote collection preview: %+v", ready)
	}
	actions := MissionCommanderNextActionsWithReviewerDispatches(nil, []ReviewerDispatchIntakeHandoff{missing, ready})
	if len(actions) != 1 || actions[0].Command != ready.ReviewerResultCollectionCommands.PreviewCommand || actions[0].Blocked {
		t.Fatalf("collection-ready shard was not actionable: %+v", actions)
	}
	if err := os.WriteFile(resultPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	collected := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, firstText(packet.ReviewerOrchestration.PacketPath, "packet.json"), "feature-review", dispatch, 0)
	if collected.State != "ready-for-reviewer-intake-preview" || collected.ReviewerResultCandidateState != "collected" || !collected.ReviewerResultPresent {
		t.Fatalf("collected candidate did not advance to intake: %+v", collected)
	}
	if err := os.WriteFile(resultPath, []byte("{\"different\":true}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	conflicting := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, packetPath, "feature-review", dispatch, 0)
	if conflicting.State != "reviewer-result-recovery-required" || conflicting.ReviewerResultRecoveryCommand == "" || reviewerDispatchIntakeNextAction(conflicting) != conflicting.ReviewerResultRecoveryCommand {
		t.Fatalf("conflicting canonical result did not promote recovery: %+v", conflicting)
	}
	if err := os.MkdirAll(filepath.Join(resultRoot, "recoveries"), 0o700); err != nil {
		t.Fatal(err)
	}
	candidateBytes, err := os.ReadFile(candidatePath)
	if err != nil {
		t.Fatal(err)
	}
	quarantined := []byte("{\"different\":true}\n")
	resultHash := reviewerDispatchBytesSHA256(quarantined)
	quarantinePath := filepath.Join(resultRoot, "recoveries", "shard-01-"+resultHash+".json")
	if err := os.WriteFile(quarantinePath, quarantined, 0o600); err != nil {
		t.Fatal(err)
	}
	inst, err := instance.Read(root)
	if err != nil {
		t.Fatal(err)
	}
	intent := reviewerResultRecoveryRecord{SchemaVersion: 1, Kind: "reviewer-result-recovery", RepoRoot: inst.TemplateRoot, CaseRoot: root, Pack: inst.TemplatePack, PacketID: packet.PacketID, PacketPath: packetPath, ShardID: "shard-01", Lane: "feature-review", CandidatePath: candidatePath, CandidateSHA256: reviewerDispatchBytesSHA256(candidateBytes), CandidateBytes: len(candidateBytes), ReviewerResultPath: resultPath, ReviewerResultKind: "regular-file", ReviewerResultSHA256: resultHash, ReviewerResultBytes: len(quarantined), QuarantinePath: quarantinePath, Actor: "mission-commander", Reason: "recover conflict", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), NoVerdict: true, NoFacts: true, NoHeavyTool: true, NoAuthority: true}
	intentBytes, err := json.Marshal(intent)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resultRoot, "recoveries", "shard-01.recovery.intent.json"), append(intentBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(resultPath); err != nil {
		t.Fatal(err)
	}
	interrupted := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, packetPath, "feature-review", dispatch, 0)
	if interrupted.State != "reviewer-result-recovery-finalize-required" || interrupted.ReviewerResultRecoveryApplyCommand == "" || reviewerDispatchIntakeNextAction(interrupted) != interrupted.ReviewerResultRecoveryApplyCommand {
		t.Fatalf("interrupted recovery did not block collection and promote exact finalize: %+v", interrupted)
	}
	if err := os.WriteFile(resultPath, candidateBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	reappeared := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, packetPath, "feature-review", dispatch, 0)
	if reappeared.State != "reviewer-result-recovery-ambiguous" || reappeared.ReviewerResultRecoveryApplyCommand != "" || reappeared.ReviewerResultRecoveryDispositionCommand == "" || !strings.Contains(reviewerDispatchIntakeNextAction(reappeared), "-RetireReviewerResultRecovery") {
		t.Fatalf("reappeared canonical result did not promote disposition preview: %+v", reappeared)
	}
	if err := os.Remove(resultPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resultPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	reappearedEmpty := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, packetPath, "feature-review", dispatch, 0)
	if reappearedEmpty.State != "reviewer-result-recovery-ambiguous" || reappearedEmpty.ReviewerResultRecoveryApplyCommand != "" || reappearedEmpty.ReviewerResultRecoveryDispositionCommand == "" {
		t.Fatalf("reappeared empty obstruction did not promote disposition preview: %+v", reappearedEmpty)
	}
	if err := os.Remove(resultPath); err != nil {
		t.Fatal(err)
	}
	actions = MissionCommanderNextActionsWithReviewerDispatches(nil, []ReviewerDispatchIntakeHandoff{interrupted})
	if len(actions) != 1 || actions[0].Blocked || actions[0].Command != interrupted.ReviewerResultRecoveryApplyCommand || !strings.Contains(actions[0].Command, "-Actor mission-commander") || !strings.Contains(actions[0].Command, "-Reason \"recover conflict\"") {
		t.Fatalf("interrupted recovery finalize was not exact and actionable: %+v", actions)
	}
	if err := os.WriteFile(filepath.Join(resultRoot, "recoveries", "shard-01.recovery.json"), append(append([]byte{}, intentBytes...), '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	committed := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, packetPath, "feature-review", dispatch, 0)
	if committed.State != "ready-for-reviewer-result-collection-preview" || committed.ReviewerResultRecoveryApplyCommand != "" {
		t.Fatalf("committed recovery did not advance to collection preview: %+v", committed)
	}
	if err := os.Remove(filepath.Join(resultRoot, "recoveries", "shard-01.recovery.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resultRoot, "recoveries", "shard-01.recovery.intent.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	invalid := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, packetPath, "feature-review", dispatch, 0)
	if invalid.State != "reviewer-result-recovery-invalid" || invalid.ReviewerResultRecoveryApplyCommand != "" {
		t.Fatalf("malformed recovery intent was not blocked: %+v", invalid)
	}
}

func TestReviewerDispatchIntakeRejectsInvalidCanonicalShape(t *testing.T) {
	root := t.TempDir()
	reviewRoot := filepath.Join(root, ".rekit", "reviews", "invalid-canonical")
	packetPath := filepath.Join(reviewRoot, "packet.json")
	resultRoot := filepath.Join(reviewRoot, "results")
	candidatePath := filepath.Join(resultRoot, "candidates", "shard-01.json")
	resultPath := filepath.Join(resultRoot, "shard-01.json")
	if err := os.MkdirAll(filepath.Dir(candidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packetPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(candidatePath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(resultPath, 0o755); err != nil {
		t.Fatal(err)
	}
	packet := reviewerDispatchPacket{PacketID: "packet-invalid-canonical", ReviewerOrchestration: reviewerDispatchPacketOrchestration{TargetLane: "feature-review", PacketPath: packetPath, ResultRoot: resultRoot, Dispatches: []reviewerDispatchPacketDispatch{{ShardID: "shard-01"}}}}
	dispatch := reviewerDispatchPacketDispatch{ShardID: "shard-01", ReviewerResultPath: resultPath, ReviewerResultCandidatePath: candidatePath, CollectionCommands: &ReviewerResultCollectionCommands{CandidatePath: candidatePath, PreviewCommand: "collect-preview", ApplyCommand: "collect-apply"}, PreviewCommand: "intake-preview", ApplyCommand: "intake-apply"}
	item := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, packetPath, "feature-review", dispatch, 0)
	if item.State != "reviewer-result-canonical-invalid" || item.ReviewerResultRecoveryCommand != "" || !strings.Contains(reviewerDispatchIntakeNextAction(item), "automatic recovery remains blocked") {
		t.Fatalf("empty canonical directory did not remain fail-closed: %+v", item)
	}
	if err := os.WriteFile(filepath.Join(resultPath, "foreign.txt"), []byte("do not remove\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	blocked := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, packetPath, "feature-review", dispatch, 0)
	if blocked.State != "reviewer-result-canonical-invalid" || blocked.ReviewerResultRecoveryCommand != "" || !strings.Contains(reviewerDispatchIntakeNextAction(blocked), "automatic recovery remains blocked") {
		t.Fatalf("non-empty canonical directory did not remain blocked: %+v", blocked)
	}
}

func TestReviewerDispatchIntakeRejectsInvalidCandidateShape(t *testing.T) {
	root := t.TempDir()
	reviewRoot := filepath.Join(root, ".rekit", "reviews", "invalid-candidate")
	packetPath := filepath.Join(reviewRoot, "packet.json")
	resultRoot := filepath.Join(reviewRoot, "results")
	candidatePath := filepath.Join(resultRoot, "candidates", "shard-01.json")
	resultPath := filepath.Join(resultRoot, "shard-01.json")
	if err := os.MkdirAll(filepath.Dir(candidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packetPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(candidatePath, 0o755); err != nil {
		t.Fatal(err)
	}
	packet := reviewerDispatchPacket{PacketID: "packet-invalid", ReviewerOrchestration: reviewerDispatchPacketOrchestration{TargetLane: "feature-review", PacketPath: packetPath, ResultRoot: resultRoot, Dispatches: []reviewerDispatchPacketDispatch{{ShardID: "shard-01"}}}}
	dispatch := reviewerDispatchPacketDispatch{ShardID: "shard-01", ReviewerResultPath: resultPath, ReviewerResultCandidatePath: candidatePath, CollectionCommands: &ReviewerResultCollectionCommands{CandidatePath: candidatePath, PreviewCommand: "collect-preview", ApplyCommand: "collect-apply"}, PreviewCommand: "intake-preview", ApplyCommand: "intake-apply"}
	item := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, packetPath, "feature-review", dispatch, 0)
	if item.State != "reviewer-result-candidate-invalid" || item.ReviewerResultCandidateState != "invalid" || !strings.Contains(reviewerDispatchIntakeNextAction(item), "replace the invalid") {
		t.Fatalf("invalid candidate did not fail closed: %+v", item)
	}
}

func TestReviewerDispatchIntakeRejectsForgedStagingSourceBinding(t *testing.T) {
	root := t.TempDir()
	reviewRoot := filepath.Join(root, ".rekit", "reviews", "review")
	packetPath := filepath.Join(reviewRoot, "packet.json")
	resultRoot := filepath.Join(reviewRoot, "results")
	candidatePath := filepath.Join(resultRoot, "candidates", "shard-01.json")
	resultPath := filepath.Join(resultRoot, "shard-01.json")
	forgedSource := filepath.Join(root, "workspace", "forged.json")
	if err := os.MkdirAll(filepath.Dir(forgedSource), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(resultRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packetPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	request := &ReviewerAgentToolRequest{Tool: "Claude Code Agent", AgentType: "read-only-reviewer", ReadOnly: true, Prompt: "review", ExpectedOutput: "one JSON object"}
	staging := &ReviewerResultStagingCommands{SourcePath: forgedSource, SourcePathArgument: forgedSource, PreviewCommand: "forged-staging-preview"}
	commands := &ReviewerResultCollectionCommands{CandidatePath: candidatePath, PreviewCommand: "collect-preview", ApplyCommand: "collect-apply"}
	packet := reviewerDispatchPacket{PacketID: "packet-collection", ReviewerOrchestration: reviewerDispatchPacketOrchestration{TargetLane: "feature-review", PacketPath: packetPath, ResultRoot: resultRoot, Dispatches: []reviewerDispatchPacketDispatch{{ShardID: "shard-01"}}}}
	dispatch := reviewerDispatchPacketDispatch{ShardID: "shard-01", ReviewerResultPath: resultPath, ReviewerResultCandidatePath: candidatePath, AgentToolRequest: request, StagingCommands: staging, CollectionCommands: commands, PreviewCommand: "intake-preview", ApplyCommand: "intake-apply"}

	item := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, packetPath, "feature-review", dispatch, 0)
	if item.ReviewerResultSourcePath != "" || item.ReviewerResultStagingCommand != "" || item.ReviewerResultCollectionCommands != nil || item.State != "waiting-for-reviewer-result" || strings.Contains(item.DispatchCommand, forgedSource) || strings.Contains(item.DispatchCommand, "forged-staging-preview") {
		t.Fatalf("forged source binding was projected as runnable: %+v", item)
	}
}

func TestReviewerDispatchIntakeRebuildsForgedCollectionCommands(t *testing.T) {
	root := t.TempDir()
	reviewRoot := filepath.Join(root, ".rekit", "reviews", "forged-command")
	packetPath := filepath.Join(reviewRoot, "packet.json")
	resultRoot := filepath.Join(reviewRoot, "results")
	candidatePath := filepath.Join(resultRoot, "candidates", "shard-01.json")
	resultPath := filepath.Join(resultRoot, "shard-01.json")
	if err := os.MkdirAll(filepath.Dir(candidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packetPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(candidatePath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	forged := &ReviewerResultCollectionCommands{CandidatePath: candidatePath, PreviewCommand: "invoke-forged-preview", ApplyCommand: "invoke-forged-apply"}
	packet := reviewerDispatchPacket{PacketID: "packet-forged-command", ReviewerOrchestration: reviewerDispatchPacketOrchestration{TargetLane: "feature-review", PacketPath: packetPath, ResultRoot: resultRoot}}
	dispatch := reviewerDispatchPacketDispatch{ShardID: "shard-01", ReviewerResultPath: resultPath, ReviewerResultCandidatePath: candidatePath, CollectionCommands: forged, PreviewCommand: "intake-preview", ApplyCommand: "intake-apply"}
	item := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, packetPath, "feature-review", dispatch, 0)
	if item.State != "ready-for-reviewer-result-collection-preview" || item.ReviewerResultCollectionCommands == nil || item.ReviewerResultCollectionCommands.PreviewCommand == forged.PreviewCommand || item.ReviewerResultCollectionCommands.ApplyCommand == forged.ApplyCommand || !strings.Contains(item.ReviewerResultCollectionCommands.PreviewCommand, "-CollectReviewerResult") || !strings.Contains(item.ReviewerResultCollectionCommands.PreviewCommand, packetPath) {
		t.Fatalf("forged collection commands were not rebuilt from canonical bindings: %+v", item)
	}
}

func TestReviewerDispatchIntakeSuppressesForgedCollectionCapability(t *testing.T) {
	root := t.TempDir()
	reviewRoot := filepath.Join(root, ".rekit", "reviews", "forged")
	packetPath := filepath.Join(reviewRoot, "packet.json")
	resultRoot := filepath.Join(reviewRoot, "results")
	candidatePath := filepath.Join(resultRoot, "candidates", "shard-01.json")
	resultPath := filepath.Join(resultRoot, "shard-01.json")
	if err := os.MkdirAll(filepath.Dir(candidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(candidatePath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commands := &ReviewerResultCollectionCommands{CandidatePath: candidatePath, PreviewCommand: "collect-preview", ApplyCommand: "collect-apply"}
	packet := reviewerDispatchPacket{PacketID: "packet-forged", ReviewerOrchestration: reviewerDispatchPacketOrchestration{TargetLane: "feature-review", PacketPath: filepath.Join(reviewRoot, "other.json"), ResultRoot: resultRoot}}
	dispatch := reviewerDispatchPacketDispatch{ShardID: "shard-01", ReviewerResultPath: resultPath, ReviewerResultCandidatePath: candidatePath, CollectionCommands: commands, PreviewCommand: "intake-preview", ApplyCommand: "intake-apply"}
	item := reviewerDispatchIntakeHandoffFor(root, mission.LedgerFacts{}, packet, packetPath, "feature-review", dispatch, 0)
	if item.ReviewerResultCollectionCommands != nil || item.ReviewerResultCandidatePath != "" || item.State == "ready-for-reviewer-result-collection-preview" || reviewerDispatchIntakeNextAction(item) == commands.PreviewCommand {
		t.Fatalf("forged collection capability survived projection: %+v", item)
	}
	summary := ReviewerDispatchIntakeSummaryFor([]ReviewerDispatchIntakeHandoff{item})
	if summary.LatestCollectionPreviewCommand != "" || summary.LatestCollectionApplyCommand != "" {
		t.Fatalf("forged collection commands reached summary: %+v", summary)
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
