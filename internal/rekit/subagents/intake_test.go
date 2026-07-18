package subagents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/note"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestDecodeReviewerResultStrictContract(t *testing.T) {
	valid := reviewerResultJSON("packet-fixture", "route-fixture", "accept", "accepted", nil)
	var missingRouteFields map[string]any
	if err := json.Unmarshal(valid, &missingRouteFields); err != nil {
		t.Fatal(err)
	}
	delete(missingRouteFields, "routeOutput")
	missingRouteOutput, err := json.Marshal(missingRouteFields)
	if err != nil {
		t.Fatal(err)
	}
	result, err := decodeReviewerResult(valid)
	if err != nil {
		t.Fatal(err)
	}
	if result.ShardID != "shard-01" || result.Decision != "accept" || result.RecommendedVerdict != "accepted" || result.RouteOutput["item"] != "alpha" {
		t.Fatalf("unexpected reviewer result: %+v", result)
	}
	for _, tc := range []struct {
		name string
		data string
		want string
	}{
		{name: "missing field", data: `{"shardId":"shard-01"}`, want: "missing required field"},
		{name: "unknown field", data: strings.TrimSuffix(string(valid), "}") + `,"writes":["x"]}`, want: "unknown field"},
		{name: "trailing object", data: string(valid) + ` {}`, want: "exactly one JSON object"},
		{name: "missing route output", data: string(missingRouteOutput), want: "missing required field"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeReviewerResult([]byte(tc.data))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestReviewerIntakeBlockersFailClosed(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	mapping, ok := reviewerDecisionMappingByDecision("accept")
	if !ok {
		t.Fatal("missing accept mapping")
	}
	result, err := decodeReviewerResult(reviewerResultForPacket(t, packet, "accept", "rejected", []string{"overlaps shard-02"}))
	if err != nil {
		t.Fatal(err)
	}
	result.Confidence = "low"
	result.RouteOutput["next_action"] = "run debugger"
	blocked, err := reviewerIntakeBlockers(repoRoot, caseRoot, packet, result, mapping)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"unresolved conflicts", "blocked write", "conflicts with mapped", "low-confidence"} {
		if !slices.ContainsFunc(blocked, func(item string) bool { return strings.Contains(item, marker) }) {
			t.Fatalf("blockers missing %q: %v", marker, blocked)
		}
	}
}

func TestReviewerIntakeIDCanonicalizesEquivalentJSON(t *testing.T) {
	packet := Packet{PacketID: "packet-fixture"}
	resultA, err := decodeReviewerResult(reviewerResultJSON("packet-fixture", "route-fixture", "accept", "accepted", nil))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(reviewerResultJSON("packet-fixture", "route-fixture", "accept", "accepted", nil), &fields); err != nil {
		t.Fatal(err)
	}
	reordered, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	resultB, err := decodeReviewerResult(append(reordered, '\n'))
	if err != nil {
		t.Fatal(err)
	}
	if left, right := reviewerIntakeID(packet, resultA, " feature-review "), reviewerIntakeID(packet, resultB, "feature-review"); left != right {
		t.Fatalf("equivalent reviewer JSON changed intake ID: %s != %s", left, right)
	}
}

func TestReviewerIntakeBlockedActionScanIgnoresEvidencePathAndSummaryWords(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	packet := Packet{Pack: defaults.DefaultPack, Route: Route{SubagentPermissions: "read-only"}}
	result := ReviewerResult{Summary: "confirmed evidence preserves authority metadata as read-only context", EvidenceRefs: []string{"workspace/authority/confirmed-review.md"}, RouteOutput: map[string]any{"tool_scope": "read-only", "next_action": "main-agent review"}}
	if reviewerResultRequestsBlockedAction(repoRoot, packet, result) {
		t.Fatalf("descriptive summary/evidence path caused a blocked-action false positive: %+v", result)
	}
	result.RouteOutput["next_action"] = "append ledger directly"
	if !reviewerResultRequestsBlockedAction(repoRoot, packet, result) {
		t.Fatalf("structured requested action was not blocked: %+v", result.RouteOutput)
	}
}

func TestIntakeReviewerResultRejectsWrongPacketAndRouteBindings(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	for _, tc := range []struct {
		name     string
		packetID string
		routeID  string
		want     string
	}{
		{name: "wrong packet", packetID: "packet-stale", routeID: packet.Route.ID, want: "does not match packet"},
		{name: "wrong route", packetID: packet.PacketID, routeID: "other:route", want: "does not match packet route"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resultPath := filepath.Join(t.TempDir(), "reviewer-result.json")
			if err := os.WriteFile(resultPath, reviewerResultJSON(tc.packetID, tc.routeID, "accept", "accepted", nil), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: "devirt-main", Actor: "mission-commander", WhatIf: true})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			if got := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl")); got != "" {
				t.Fatalf("invalid binding wrote verification ledger:\n%s", got)
			}
			if got := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl")); got != "" {
				t.Fatalf("invalid binding wrote decision ledger:\n%s", got)
			}
		})
	}
}

func TestIntakeReviewerResultBlocksInvalidEvidenceRefs(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	for _, ref := range []string{"n/a", "workspace/missing-evidence.md", filepath.Join(t.TempDir(), "outside.md"), "../escape.md", "workspace"} {
		t.Run(strings.ReplaceAll(ref, string(filepath.Separator), "_"), func(t *testing.T) {
			value, err := decodeReviewerResult(reviewerResultForPacket(t, packet, "accept", "accepted", nil))
			if err != nil {
				t.Fatal(err)
			}
			value.EvidenceRefs = []string{ref}
			value.RouteOutput["evidence"] = ref
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			resultPath := filepath.Join(t.TempDir(), "reviewer-result.json")
			if err := os.WriteFile(resultPath, data, 0o644); err != nil {
				t.Fatal(err)
			}
			result, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: "devirt-main", Actor: "mission-commander"})
			if err != nil {
				t.Fatal(err)
			}
			if result.WritebackStatus != "blocked" || result.ReadyForWriteback || result.Applied || len(result.BlockedReasons) == 0 {
				t.Fatalf("invalid evidence ref did not fail closed: %+v", result)
			}
		})
	}
}

func TestIntakeReviewerResultRejectsRouteOutputContractViolations(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	for _, tc := range []struct {
		name   string
		mutate func(*ReviewerResult)
		want   string
	}{
		{name: "unknown field", mutate: func(result *ReviewerResult) { result.RouteOutput["execute"] = "patch binary" }, want: "unknown field"},
		{name: "empty field", mutate: func(result *ReviewerResult) { result.RouteOutput["item"] = "" }, want: "non-empty string"},
		{name: "wrong type", mutate: func(result *ReviewerResult) { result.RouteOutput["item"] = map[string]any{"value": "alpha"} }, want: "non-empty string"},
		{name: "decision mismatch", mutate: func(result *ReviewerResult) { result.RouteOutput["decision"] = "reject" }, want: "does not match top-level value"},
		{name: "confidence mismatch", mutate: func(result *ReviewerResult) { result.RouteOutput["confidence"] = "low" }, want: "does not match top-level value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, err := decodeReviewerResult(reviewerResultForPacket(t, packet, "accept", "accepted", nil))
			if err != nil {
				t.Fatal(err)
			}
			tc.mutate(&value)
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			resultPath := filepath.Join(t.TempDir(), "reviewer-result.json")
			if err := os.WriteFile(resultPath, data, 0o644); err != nil {
				t.Fatal(err)
			}
			_, err = IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: "devirt-main", Actor: "mission-commander", WhatIf: true})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestIntakeReviewerResultRejectsStaleOwnerBinding(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	if !packet.OwnerBinding.RequiredForIntake || packet.OwnerBinding.CurrentExecutor != "session-main" {
		t.Fatalf("plan did not bind owner executor: %+v", packet.OwnerBinding)
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workstream.StartApply(root, caseRoot, defaults.DefaultPack, workstream.StartOptions{Name: "intake", Executor: "session-replacement", Actor: "mission-commander", TakeoverReason: "replace reviewer owner"}); err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(t.TempDir(), "reviewer-result.json")
	if err := os.WriteFile(resultPath, reviewerResultForPlan(t, plan.PacketPath, "accept", "accepted", nil), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: "feature-intake", Actor: "mission-commander", WhatIf: true})
	if err == nil || !strings.Contains(err.Error(), "ownerBinding is stale") {
		t.Fatalf("error = %v, want stale owner binding rejection", err)
	}
	if got := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl")); got != "" {
		t.Fatalf("stale owner binding wrote verification ledger:\n%s", got)
	}
}

func TestIntakeReviewerResultRejectsTamperedPacket(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	packetBytes, err := os.ReadFile(plan.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	var packet map[string]any
	if err := json.Unmarshal(packetBytes, &packet); err != nil {
		t.Fatal(err)
	}
	packet["outputContract"] = "item"
	tampered, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.PacketPath, tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(t.TempDir(), "reviewer-result.json")
	if err := os.WriteFile(resultPath, reviewerResultForPlan(t, plan.PacketPath, "accept", "accepted", nil), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: "devirt-main", Actor: "mission-commander", WhatIf: true})
	if err == nil || !strings.Contains(err.Error(), "not a supported non-mutating") {
		t.Fatalf("error = %v, want packet identity rejection", err)
	}
}

func TestIntakeReviewerResultWhatIfAndApply(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(t.TempDir(), "reviewer-result.json")
	if err := os.WriteFile(resultPath, reviewerResultForPlan(t, plan.PacketPath, "accept", "accepted", nil), 0o644); err != nil {
		t.Fatal(err)
	}
	beforeVerifications := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl"))
	beforeDecisions := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl"))

	preview, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: "devirt-main", Actor: "mission-commander", WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Command != commandName || preview.Mode != "reviewer-intake" || preview.IsMutation || preview.Applied || preview.WritebackStatus != "previewed" || !preview.ReadyForWriteback || !preview.ReviewRequired || preview.Verification == nil || preview.Decision == nil || preview.PostValidation == nil || !preview.PostValidation.Valid {
		t.Fatalf("unexpected intake preview: %+v", preview)
	}
	if preview.Verification.Applied || preview.Decision.Applied || preview.Verification.Reason != "what-if" || preview.Decision.Reason != "what-if" || preview.Verification.Event["verdict"] != "accepted" || preview.Decision.Event["decision"] != "accept" || preview.Verification.Event["reviewerSession"] != "reviewer-session-1" || preview.Verification.Event["ownerBindingMode"] != "unassigned-lane" || preview.Verification.Event["ownerBindingTarget"] != "devirt-main" {
		t.Fatalf("unexpected writeback previews: verification=%+v decision=%+v", preview.Verification, preview.Decision)
	}
	if got := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl")); got != beforeVerifications {
		t.Fatalf("preview changed verifications ledger:\n%s", got)
	}
	if got := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl")); got != beforeDecisions {
		t.Fatalf("preview changed decisions ledger:\n%s", got)
	}

	applied, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: "devirt-main", Actor: "mission-commander"})
	if err != nil {
		t.Fatal(err)
	}
	if !applied.IsMutation || !applied.Applied || applied.WritebackStatus != "complete" || !applied.ReadyForWriteback || applied.Verification == nil || applied.Decision == nil || !applied.Verification.Applied || !applied.Decision.Applied || applied.PostValidation == nil || !applied.PostValidation.Valid {
		t.Fatalf("unexpected intake apply: %+v", applied)
	}
	verificationLedger := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl"))
	decisionLedger := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl"))
	if strings.Count(verificationLedger, applied.Verification.EventID) != 1 || strings.Count(decisionLedger, applied.Decision.EventID) != 1 || !strings.Contains(decisionLedger, applied.Verification.EventID) || !strings.Contains(verificationLedger, "reviewer-session-1") || !strings.Contains(verificationLedger, "ownerBindingMode") {
		t.Fatalf("writeback order/evidence/provenance linkage missing:\nverification=%s\ndecision=%s", verificationLedger, decisionLedger)
	}

	duplicate, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: "devirt-main", Actor: "mission-commander"})
	if err != nil {
		t.Fatal(err)
	}
	if duplicate.Applied || duplicate.WritebackStatus != "already-complete" || duplicate.Verification.Reason != "duplicate eventId" || duplicate.Decision.Reason != "duplicate eventId" {
		t.Fatalf("repeat intake should be idempotent: %+v", duplicate)
	}
}

func TestIntakeReviewerResultBlocksConflictsWithoutWrites(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(t.TempDir(), "reviewer-result.json")
	if err := os.WriteFile(resultPath, reviewerResultForPlan(t, plan.PacketPath, "accept", "accepted", []string{"overlaps another shard"}), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: "devirt-main", Actor: "mission-commander"})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsMutation || result.Applied || result.WritebackStatus != "blocked" || result.ReadyForWriteback || !result.ReviewRequired || len(result.BlockedReasons) == 0 || result.Verification != nil || result.Decision != nil || result.PostValidation == nil {
		t.Fatalf("conflicted result did not fail closed: %+v", result)
	}
	if got := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl")); got != "" {
		t.Fatalf("blocked intake wrote verification ledger:\n%s", got)
	}
	if got := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl")); got != "" {
		t.Fatalf("blocked intake wrote decision ledger:\n%s", got)
	}
}

func TestIntakeReviewerResultRecoversPartialVerificationWrite(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(t.TempDir(), "reviewer-result.json")
	if err := os.WriteFile(resultPath, reviewerResultForPlan(t, plan.PacketPath, "accept", "accepted", nil), 0o644); err != nil {
		t.Fatal(err)
	}
	originalAppend := appendReviewerNote
	appendCalls := 0
	appendReviewerNote = func(repoRoot, caseRoot, pack string, opt note.Options, whatIf bool) (note.AppendResult, error) {
		if !whatIf {
			appendCalls++
			if appendCalls == 2 {
				return note.AppendResult{}, fmt.Errorf("injected decision append failure")
			}
		}
		return originalAppend(repoRoot, caseRoot, pack, opt, whatIf)
	}
	defer func() { appendReviewerNote = originalAppend }()
	partial, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: "devirt-main", Actor: "mission-commander"})
	appendReviewerNote = originalAppend
	if err == nil || partial.WritebackStatus != "verification-recorded" || partial.Verification == nil || !partial.Verification.Applied || partial.Decision == nil || partial.Decision.Applied {
		t.Fatalf("partial writeback status was not preserved: result=%+v err=%v", partial, err)
	}
	if got := strings.Count(readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl")), partial.Verification.EventID); got != 1 {
		t.Fatalf("verification append count = %d, want 1", got)
	}

	recovered, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: "devirt-main", Actor: "mission-commander"})
	if err != nil {
		t.Fatal(err)
	}
	if recovered.WritebackStatus != "complete" || recovered.Verification == nil || recovered.Verification.Applied || recovered.Decision == nil || !recovered.Decision.Applied {
		t.Fatalf("partial writeback retry did not recover idempotently: %+v", recovered)
	}
	if got := strings.Count(readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl")), recovered.Verification.EventID); got != 1 {
		t.Fatalf("verification append count after recovery = %d, want 1", got)
	}
	if got := strings.Count(readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl")), recovered.Decision.EventID); got != 1 {
		t.Fatalf("decision append count after recovery = %d, want 1", got)
	}
}

func reviewerResultForPlan(t *testing.T, packetPath, decision, verdict string, conflicts []string) []byte {
	t.Helper()
	packet := readReviewerPacket(t, packetPath)
	caseRoot := packet.PlanRoot
	evidencePath := filepath.Join(caseRoot, "workspace", "review-evidence.md")
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return reviewerResultForPacket(t, packet, decision, verdict, conflicts)
}

func reviewerResultForPacket(t *testing.T, packet Packet, decision, verdict string, conflicts []string) []byte {
	t.Helper()
	return reviewerResultJSON(packet.PacketID, packet.Route.ID, decision, verdict, conflicts)
}

func readReviewerPacket(t *testing.T, packetPath string) Packet {
	t.Helper()
	data, err := os.ReadFile(packetPath)
	if err != nil {
		t.Fatal(err)
	}
	var packet Packet
	if err := json.Unmarshal(data, &packet); err != nil {
		t.Fatal(err)
	}
	return packet
}

func reviewerResultJSON(packetID, routeID, decision, verdict string, conflicts []string) []byte {
	if conflicts == nil {
		conflicts = []string{}
	}
	routeOutput := map[string]any{}
	for _, field := range splitCSV("item,decision,confidence,evidence,risk,next_action,tier_used,tool_scope,feature,request_id,candidate_path,defer_reason") {
		routeOutput[field] = "n/a"
	}
	routeOutput["item"] = "alpha"
	routeOutput["decision"] = decision
	routeOutput["confidence"] = "high"
	if decision == "accept" || decision == "reject" {
		routeOutput["evidence"] = "workspace/review-evidence.md"
	}
	value := ReviewerResult{
		PacketID:           packetID,
		RouteID:            routeID,
		ShardID:            "shard-01",
		Items:              []string{"alpha"},
		ReviewerSession:    "reviewer-session-1",
		Decision:           decision,
		Confidence:         "high",
		Summary:            "reviewed alpha against bounded evidence",
		EvidenceRefs:       []string{"workspace/review-evidence.md"},
		Risks:              []string{},
		Conflicts:          conflicts,
		RecommendedVerdict: verdict,
		RouteOutput:        routeOutput,
	}
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func readOptionalFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeReviewerIntakeCase(t *testing.T, repoRoot, caseRoot string) {
	t.Helper()
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := syncreview.Apply(root, caseRoot, defaults.DefaultPack, syncreview.ApplyOptions{ProjectName: "reviewer-intake-test", CreateLocalFiles: true, Command: "init"}); err != nil {
		t.Fatal(err)
	}
	if _, err := workstream.StartApply(root, caseRoot, defaults.DefaultPack, workstream.StartOptions{Name: "intake", Executor: "session-main", Actor: "mission-commander", TakeoverReason: "reviewer intake owner binding fixture"}); err != nil {
		t.Fatal(err)
	}
}
