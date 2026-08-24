package workstream

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/plancontract"
)

func TestContinueApplyRequiresReviewedPlanIdentityBeforeMutation(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	before := snapshotWorkstreamTree(t, caseRoot)

	_, err := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{
		Selector:                   "devirt-main",
		Executor:                   "executor-one",
		ExpectedExecutorGeneration: 1,
	})
	failure, typed := plancontract.FromError(err)
	if err == nil || !typed || failure.Code != plancontract.CodePlanMissing || failure.MutationApplied || failure.MutationBoundary != "none" || !IsZeroProgress(err) {
		t.Fatalf("continue Apply without reviewed plan error = %v failure=%+v typed=%t", err, failure, typed)
	}
	if after := snapshotWorkstreamTree(t, caseRoot); after != before {
		t.Fatalf("continue Apply without reviewed plan mutated case\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestContinueApplyRejectsPreviewOnlyBlockersWithoutWrites(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*testing.T, string)
		wantError  string
		assertPlan func(*testing.T, ContinueResult)
	}{
		{
			name: "open intervention",
			setup: func(t *testing.T, caseRoot string) {
				appendContinueBlockerFact(t, caseRoot, "intervention", map[string]any{
					"eventId": "intervention-continue-blocker",
					"kind":    "intervention",
					"lane":    "devirt-main",
					"status":  "open",
					"subject": "review correction before continue",
				})
			},
			wantError: "continue Apply is blocked by an open intervention",
			assertPlan: func(t *testing.T, preview ContinueResult) {
				if !preview.ReconcileRequired || len(preview.ReconcileHandoffs) != 1 {
					t.Fatalf("intervention preview omitted reconcile handoff: %+v", preview)
				}
			},
		},
		{
			name: "pending gate",
			setup: func(t *testing.T, caseRoot string) {
				appendContinueBlockerFact(t, caseRoot, "request", map[string]any{
					"eventId": "request-continue-pending-gate",
					"kind":    "request",
					"lane":    "devirt-main",
					"status":  "pending-gate",
					"subject": "review bounded action",
				})
			},
			wantError: "continue Apply is blocked by a pending gate or open decision",
			assertPlan: func(t *testing.T, preview ContinueResult) {
				if !preview.PendingGateRequired || len(preview.PendingGateHandoffs) != 1 {
					t.Fatalf("pending gate preview omitted gate handoff: %+v", preview)
				}
			},
		},
		{
			name: "open decision",
			setup: func(t *testing.T, caseRoot string) {
				appendContinueBlockerFact(t, caseRoot, "decision", map[string]any{
					"eventId":  "decision-continue-open",
					"kind":     "decision",
					"lane":     "devirt-main",
					"status":   "open",
					"decision": "defer",
					"subject":  "choose next route",
					"target":   "route-a",
				})
			},
			wantError: "continue Apply is blocked by a pending gate or open decision",
			assertPlan: func(t *testing.T, preview ContinueResult) {
				if !preview.OpenDecisionRequired || len(preview.OpenDecisionHandoffs) != 1 {
					t.Fatalf("open decision preview omitted decision handoff: %+v", preview)
				}
			},
		},
		{
			name:      "reviewer dispatch intake",
			setup:     writeContinueReviewerDispatchBlocker,
			wantError: "continue Apply is blocked by reviewer dispatch or intake",
			assertPlan: func(t *testing.T, preview ContinueResult) {
				if len(preview.ReviewerDispatchIntakeHandoffs) != 1 {
					t.Fatalf("reviewer blocker preview omitted intake handoff: %+v", preview)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot, caseRoot := setupOwnedContinueCase(t)
			tc.setup(t, caseRoot)
			before := snapshotWorkstreamTree(t, caseRoot)
			opt := ContinueOptions{
				Selector:                   "devirt-main",
				Executor:                   "executor-one",
				ExpectedExecutorGeneration: 1,
			}

			preview, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, opt)
			if err != nil {
				t.Fatal(err)
			}
			if !preview.Blocked || preview.RequiresConfirmation || preview.ContinuePlanSHA256 != "" || continuePreviewHasExecutableApply(preview.MissionCommanderActionQueue, preview.ContinuePlanSHA256) {
				t.Fatalf("continue preview exposed executable Apply for blocker: %+v", preview)
			}
			tc.assertPlan(t, preview)

			opt.ExpectedContinuePlanSHA256 = strings.Repeat("a", sha256.Size*2)
			_, err = ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, opt)
			if err == nil || !strings.Contains(err.Error(), tc.wantError) || !IsZeroProgress(err) {
				t.Fatalf("blocked continue Apply error = %v", err)
			}
			if after := snapshotWorkstreamTree(t, caseRoot); after != before {
				t.Fatalf("blocked continue Apply mutated case\nbefore:\n%s\nafter:\n%s", before, after)
			}
		})
	}
}

func appendContinueBlockerFact(t *testing.T, caseRoot, category string, event map[string]any) {
	t.Helper()
	if _, _, err := mission.AppendFact(caseRoot, category, event); err != nil {
		t.Fatal(err)
	}
}

func writeContinueReviewerDispatchBlocker(t *testing.T, caseRoot string) {
	t.Helper()
	reviewRoot := filepath.Join(caseRoot, ".rekit", "reviews", "continue-blocker")
	packetPath := filepath.Join(reviewRoot, "packet.json")
	integrityPath := filepath.Join(reviewRoot, "packet.integrity.json")
	resultRoot := filepath.Join(reviewRoot, "results")
	if err := os.MkdirAll(resultRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	packet := reviewerDispatchPacket{
		PacketID:        "packet-continue-blocker",
		PacketIntegrity: &reviewerPacketIntegrityReference{Algorithm: "sha256", Path: integrityPath},
		Command:         "plan-subagents",
		TargetLane:      "devirt-main",
		ReviewerOrchestration: reviewerDispatchPacketOrchestration{
			TargetLane: "devirt-main",
			PacketPath: packetPath,
			ResultRoot: resultRoot,
			Dispatches: []reviewerDispatchPacketDispatch{{
				ShardID:            "shard-01",
				ReviewerResultPath: filepath.Join(resultRoot, "shard-01.json"),
				PreviewCommand:     "preview",
				ApplyCommand:       "apply",
			}},
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
	integrity := reviewerPacketIntegrity{
		SchemaVersion: 1,
		Kind:          "reviewer-packet-integrity",
		Algorithm:     "sha256",
		PacketID:      packet.PacketID,
		TargetLane:    packet.TargetLane,
		PacketPath:    packetPath,
		PacketSHA256:  hex.EncodeToString(sum[:]),
		PacketBytes:   len(packetData),
	}
	integrityData, err := json.Marshal(integrity)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(integrityPath, append(integrityData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
