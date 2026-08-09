package workstream

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewerresult"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewersession"
)

type MemberReviewerRejection struct {
	Lane                      string   `json:"lane"`
	ManifestRef               string   `json:"manifestRef"`
	ManifestSHA256            string   `json:"manifestSha256"`
	PacketID                  string   `json:"packetId"`
	RouteID                   string   `json:"routeId"`
	ShardID                   string   `json:"shardId"`
	PacketPath                string   `json:"packetPath"`
	ReviewerResultPath        string   `json:"reviewerResultPath"`
	ReviewerResultSHA256      string   `json:"reviewerResultSha256"`
	ReviewerResultInputPath   string   `json:"reviewerResultInputPath"`
	ReviewerResultInputSHA256 string   `json:"reviewerResultInputSha256"`
	ReviewerResultInputBytes  int64    `json:"reviewerResultInputBytes"`
	ReviewerSession           string   `json:"reviewerSession"`
	ReviewerDispatchPath      string   `json:"reviewerDispatchPath"`
	ReviewerDispatchSHA256    string   `json:"reviewerDispatchSha256"`
	ReviewerCompletionPath    string   `json:"reviewerCompletionPath"`
	ReviewerCompletionSHA256  string   `json:"reviewerCompletionSha256"`
	VerificationEventID       string   `json:"verificationEventId"`
	DecisionEventID           string   `json:"decisionEventId"`
	Summary                   string   `json:"summary"`
	EvidenceRefs              []string `json:"evidenceRefs"`
	Risks                     []string `json:"risks,omitempty"`
	Conflicts                 []string `json:"conflicts,omitempty"`
	OwnerExecutor             string   `json:"ownerExecutor"`
	OwnerGeneration           int      `json:"ownerGeneration"`
}

func CurrentMemberManifestReviewerRejection(caseRoot, laneID, manifestRef string) (MemberReviewerRejection, bool, error) {
	facts, err := mission.ReadStrictLedgerFacts(caseRoot)
	if err != nil {
		return MemberReviewerRejection{}, false, err
	}
	return memberManifestReviewerRejection(caseRoot, laneID, manifestRef, facts)
}

func memberManifestReviewerRejection(caseRoot, laneID, manifestRef string, facts mission.LedgerFacts) (MemberReviewerRejection, bool, error) {
	manifestFull, err := anchoredReviewerRejectionPath(caseRoot, manifestRef)
	if err != nil {
		return MemberReviewerRejection{}, false, err
	}
	candidates := []MemberReviewerRejection{}
	for _, verification := range facts.Verifications {
		if mission.Value(verification, "lane") != laneID || !strings.EqualFold(mission.Value(verification, "verdict"), "rejected") || !eventTargetBindsPath(verification, manifestRef, manifestFull) {
			continue
		}
		packetID := mission.Value(verification, "packetId")
		verificationID := mission.Value(verification, "eventId")
		if packetID == "" || verificationID == "" {
			return MemberReviewerRejection{}, false, fmt.Errorf("reviewer rejection verification identity is incomplete for current member manifest %s", manifestRef)
		}
		matched := false
		for _, decision := range facts.Decisions {
			if mission.Value(decision, "lane") != laneID || mission.Value(decision, "packetId") != packetID || !strings.EqualFold(mission.Value(decision, "decision"), "reject") || !eventEvidenceReferences(decision, verificationID) {
				continue
			}
			matched = true
			lineage, err := validateMemberReviewerRejectionPair(caseRoot, laneID, manifestRef, verification, decision)
			if err != nil {
				return MemberReviewerRejection{}, false, err
			}
			candidates = append(candidates, lineage)
		}
		if !matched {
			return MemberReviewerRejection{}, false, fmt.Errorf("reviewer rejection verification %s has no canonical reject decision", verificationID)
		}
	}
	if len(candidates) == 0 {
		return MemberReviewerRejection{}, false, nil
	}
	if len(candidates) != 1 {
		return MemberReviewerRejection{}, false, fmt.Errorf("current member manifest %s requires exactly one canonical reviewer rejection lineage; got %d", manifestRef, len(candidates))
	}
	return candidates[0], true, nil
}

func validateMemberReviewerRejectionPair(caseRoot, laneID, manifestRef string, verification, decision map[string]any) (MemberReviewerRejection, error) {
	if !strings.EqualFold(mission.Value(verification, "reviewerDecision"), "reject") || !strings.EqualFold(mission.Value(verification, "recommendedVerdict"), "rejected") ||
		!strings.EqualFold(mission.Value(decision, "reviewerDecision"), "reject") || !strings.EqualFold(mission.Value(decision, "recommendedVerdict"), "rejected") {
		return MemberReviewerRejection{}, fmt.Errorf("reviewer rejection ledger semantics are not canonical reject/rejected")
	}
	fields := []string{"packetId", "routeId", "shardId", "packetPath", "reviewerResultPath", "reviewerSession", "reviewerDispatchReceiptPath", "reviewerDispatchReceiptSha256", "reviewerCompletionReceiptPath", "reviewerCompletionReceiptSha256", "reviewerResultInputPath", "reviewerResultInputSha256", "ownerExecutor", "ownerGeneration"}
	for _, field := range fields {
		left, right := mission.Value(verification, field), mission.Value(decision, field)
		if strings.TrimSpace(left) == "" || left != right {
			return MemberReviewerRejection{}, fmt.Errorf("reviewer rejection ledger %s binding is missing or inconsistent", field)
		}
	}
	ownerGeneration, err := strconv.Atoi(mission.Value(decision, "ownerGeneration"))
	if err != nil || ownerGeneration <= 0 {
		return MemberReviewerRejection{}, fmt.Errorf("reviewer rejection owner generation is invalid")
	}
	lineage := MemberReviewerRejection{
		Lane: laneID, ManifestRef: filepath.ToSlash(manifestRef), PacketID: mission.Value(decision, "packetId"), RouteID: mission.Value(decision, "routeId"), ShardID: mission.Value(decision, "shardId"), PacketPath: mission.Value(decision, "packetPath"),
		ReviewerResultPath: mission.Value(decision, "reviewerResultPath"), ReviewerResultInputPath: mission.Value(decision, "reviewerResultInputPath"), ReviewerResultInputSHA256: mission.Value(decision, "reviewerResultInputSha256"), ReviewerSession: mission.Value(decision, "reviewerSession"),
		ReviewerDispatchPath: mission.Value(decision, "reviewerDispatchReceiptPath"), ReviewerDispatchSHA256: mission.Value(decision, "reviewerDispatchReceiptSha256"), ReviewerCompletionPath: mission.Value(decision, "reviewerCompletionReceiptPath"), ReviewerCompletionSHA256: mission.Value(decision, "reviewerCompletionReceiptSha256"),
		VerificationEventID: mission.Value(verification, "eventId"), DecisionEventID: mission.Value(decision, "eventId"), Summary: mission.Value(verification, "summary"), EvidenceRefs: eventStringList(verification["evidenceRefs"]), Risks: eventStringList(verification["reviewerRisks"]), Conflicts: eventStringList(verification["reviewerConflicts"]),
		OwnerExecutor: mission.Value(decision, "ownerExecutor"), OwnerGeneration: ownerGeneration,
	}
	if lineage.DecisionEventID == "" || lineage.Summary == "" {
		return MemberReviewerRejection{}, fmt.Errorf("reviewer rejection decision identity or summary is missing")
	}
	if err := validateMemberReviewerRejectionArtifacts(caseRoot, &lineage); err != nil {
		return MemberReviewerRejection{}, fmt.Errorf("reviewer rejection lineage is invalid: %w", err)
	}
	return lineage, nil
}

func validateMemberReviewerRejectionArtifacts(caseRoot string, lineage *MemberReviewerRejection) error {
	packetPath := filepath.FromSlash(strings.TrimSpace(lineage.PacketPath))
	if !filepath.IsAbs(packetPath) {
		packetPath = filepath.Join(caseRoot, packetPath)
	}
	packet, err := readReviewerDispatchPacket(caseRoot, packetPath)
	if err != nil {
		return err
	}
	if err := validateReviewerPacketIntegrity(caseRoot, packetPath, packet); err != nil {
		return err
	}
	if packet.PacketID != lineage.PacketID || packet.Route.ID != lineage.RouteID || firstText(packet.ReviewerOrchestration.TargetLane, packet.TargetLane, packet.ReviewerOrchestration.OwnerBinding.TargetLane) != lineage.Lane || !casebind.SamePath(packet.ReviewerOrchestration.PacketPath, packetPath) || packet.ReviewerOrchestration.OwnerBinding != packet.OwnerBinding {
		return fmt.Errorf("reviewer packet identity does not match rejection writeback")
	}
	var shard *reviewerDispatchPacketDispatch
	for index := range packet.ReviewerOrchestration.Dispatches {
		if packet.ReviewerOrchestration.Dispatches[index].ShardID == lineage.ShardID {
			shard = &packet.ReviewerOrchestration.Dispatches[index]
			break
		}
	}
	if shard == nil {
		return fmt.Errorf("reviewer packet does not contain rejected shard %s", lineage.ShardID)
	}
	input, err := refsf.ReadStableRegularFileAnchored(caseRoot, lineage.ReviewerResultInputPath, "reviewer result input", 64<<10)
	if err != nil {
		return err
	}
	if reviewerDispatchBytesSHA256(input) != strings.ToLower(lineage.ReviewerResultInputSHA256) {
		return fmt.Errorf("reviewer result input sha256 does not match rejection writeback")
	}
	result, err := reviewerresult.Decode(input)
	if err != nil {
		return err
	}
	if result.PacketID != lineage.PacketID || result.RouteID != lineage.RouteID || result.ShardID != lineage.ShardID || result.ReviewerSession != lineage.ReviewerSession || result.Decision != "reject" || result.RecommendedVerdict != "rejected" || !reviewerResultBindsManifest(caseRoot, result.Items, lineage.ManifestRef) {
		return fmt.Errorf("reviewer result input does not match canonical rejection bindings")
	}
	resultBytes, err := refsf.ReadStableRegularFileAnchored(caseRoot, lineage.ReviewerResultPath, "reviewer result", 64<<10)
	if err != nil {
		return err
	}
	collected, err := reviewerresult.Decode(resultBytes)
	if err != nil {
		return err
	}
	inputCanonical, _ := json.Marshal(result)
	resultCanonical, _ := json.Marshal(collected)
	if string(inputCanonical) != string(resultCanonical) {
		return fmt.Errorf("collected reviewer result does not match canonical rejection input")
	}
	lineage.ReviewerResultSHA256 = reviewerDispatchBytesSHA256(resultBytes)
	lineage.ReviewerResultInputBytes = int64(len(input))

	packetBytes, err := refsf.ReadStableRegularFileAnchored(caseRoot, packetPath, "reviewer packet", 4<<20)
	if err != nil {
		return err
	}
	dispatchBytes, err := refsf.ReadStableRegularFileAnchored(caseRoot, lineage.ReviewerDispatchPath, "reviewer session dispatch receipt", 256<<10)
	if err != nil || !strings.EqualFold(reviewerDispatchBytesSHA256(dispatchBytes), lineage.ReviewerDispatchSHA256) {
		return fmt.Errorf("reviewer dispatch receipt changed after rejection intake")
	}
	dispatch, err := reviewersession.DecodeDispatch(dispatchBytes)
	if err != nil || !reviewerDispatchSessionStaticBindingsCurrent(packet, packetPath, packetBytes, lineage.Lane, *shard, dispatch, lineage.ReviewerDispatchPath) || dispatch.ReviewerSession != lineage.ReviewerSession || !reviewerDispatchSessionCurrentOwnerBindings(caseRoot, packet, packetPath, dispatch, lineage.OwnerExecutor, lineage.OwnerGeneration) {
		return fmt.Errorf("reviewer dispatch receipt does not match rejection packet/session/historical owner bindings")
	}
	if !casebind.SamePath(lineage.ReviewerCompletionPath, reviewersession.CompletionPath(packetPath, lineage.ShardID, dispatch.DispatchID)) {
		return fmt.Errorf("reviewer completion receipt path is not canonical")
	}
	completionBytes, err := refsf.ReadStableRegularFileAnchored(caseRoot, lineage.ReviewerCompletionPath, "reviewer session completion receipt", 256<<10)
	if err != nil || !strings.EqualFold(reviewerDispatchBytesSHA256(completionBytes), lineage.ReviewerCompletionSHA256) {
		return fmt.Errorf("reviewer completion receipt changed after rejection intake")
	}
	completion, err := reviewersession.DecodeCompletion(completionBytes)
	if err != nil || reviewersession.ValidateCompletionDispatchLineage(completion, dispatch, lineage.ReviewerDispatchPath, lineage.ReviewerDispatchSHA256) != nil || completion.Outcome != "succeeded" || !casebind.SamePath(completion.ReviewerResultInputPath, lineage.ReviewerResultInputPath) || !strings.EqualFold(completion.ReviewerResultInputSHA256, lineage.ReviewerResultInputSHA256) || int64(completion.ReviewerResultInputBytes) != lineage.ReviewerResultInputBytes || completion.CompletionOwner != dispatch.EffectiveOwner {
		return fmt.Errorf("reviewer completion receipt does not match canonical rejection input lineage")
	}
	manifestPath, err := anchoredReviewerRejectionPath(caseRoot, lineage.ManifestRef)
	if err != nil {
		return err
	}
	manifest, err := refsf.ReadStableRegularFileAnchored(caseRoot, manifestPath, "rejected member manifest", 4<<20)
	if err != nil {
		return err
	}
	lineage.ManifestSHA256 = reviewerDispatchBytesSHA256(manifest)
	return nil
}

func reviewerResultBindsManifest(caseRoot string, items []string, manifestRef string) bool {
	manifestPath, err := anchoredReviewerRejectionPath(caseRoot, manifestRef)
	if err != nil {
		return false
	}
	for _, item := range items {
		item, _, _ = strings.Cut(strings.TrimSpace(item), "#")
		itemPath, err := anchoredReviewerRejectionPath(caseRoot, item)
		if err == nil && casebind.SamePath(itemPath, manifestPath) {
			return true
		}
	}
	return false
}

func anchoredReviewerRejectionPath(caseRoot, path string) (string, error) {
	path = filepath.FromSlash(strings.TrimSpace(path))
	if filepath.IsAbs(path) {
		rootFull, err := filepath.Abs(caseRoot)
		if err != nil {
			return "", err
		}
		pathFull, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(rootFull, pathFull)
		if err != nil || rel == "." || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("reviewer result item path escapes case root: %s", path)
		}
		return pathFull, nil
	}
	return refsf.SafeJoin(caseRoot, path)
}
