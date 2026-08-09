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

type MemberReviewerAcceptance struct {
	Lane                      string `json:"lane"`
	ManifestRef               string `json:"manifestRef"`
	ManifestSHA256            string `json:"manifestSha256"`
	PacketID                  string `json:"packetId"`
	RouteID                   string `json:"routeId"`
	ShardID                   string `json:"shardId"`
	PacketPath                string `json:"packetPath"`
	ReviewerResultPath        string `json:"reviewerResultPath"`
	ReviewerResultSHA256      string `json:"reviewerResultSha256"`
	ReviewerResultInputPath   string `json:"reviewerResultInputPath"`
	ReviewerResultInputSHA256 string `json:"reviewerResultInputSha256"`
	ReviewerResultInputBytes  int64  `json:"reviewerResultInputBytes"`
	ReviewerSession           string `json:"reviewerSession"`
	ReviewerDispatchPath      string `json:"reviewerDispatchPath"`
	ReviewerDispatchSHA256    string `json:"reviewerDispatchSha256"`
	ReviewerCompletionPath    string `json:"reviewerCompletionPath"`
	ReviewerCompletionSHA256  string `json:"reviewerCompletionSha256"`
	VerificationEventID       string `json:"verificationEventId"`
	DecisionEventID           string `json:"decisionEventId"`
	OwnerExecutor             string `json:"ownerExecutor"`
	OwnerGeneration           int    `json:"ownerGeneration"`
}

func CurrentMemberManifestReviewerAcceptance(caseRoot, laneID, manifestRef string) (MemberReviewerAcceptance, bool, error) {
	facts, err := mission.ReadStrictLedgerFacts(caseRoot)
	if err != nil {
		return MemberReviewerAcceptance{}, false, err
	}
	if rejection, rejected, err := memberManifestReviewerRejection(caseRoot, laneID, manifestRef, facts); err != nil {
		return MemberReviewerAcceptance{}, false, err
	} else if rejected {
		return MemberReviewerAcceptance{}, false, fmt.Errorf("current member manifest %s was canonically rejected by decision %s", manifestRef, rejection.DecisionEventID)
	}
	manifestPath, err := anchoredReviewerRejectionPath(caseRoot, manifestRef)
	if err != nil {
		return MemberReviewerAcceptance{}, false, err
	}
	candidates := []MemberReviewerAcceptance{}
	for _, verification := range facts.Verifications {
		if mission.Value(verification, "lane") != laneID || !strings.EqualFold(mission.Value(verification, "verdict"), "accepted") || !eventTargetBindsPath(verification, manifestRef, manifestPath) {
			continue
		}
		for _, decision := range facts.Decisions {
			if mission.Value(decision, "lane") != laneID || mission.Value(decision, "packetId") != mission.Value(verification, "packetId") || !strings.EqualFold(mission.Value(decision, "decision"), "accept") || !eventEvidenceReferences(decision, mission.Value(verification, "eventId")) {
				continue
			}
			acceptance, err := validateMemberReviewerAcceptancePair(caseRoot, laneID, manifestRef, verification, decision)
			if err != nil {
				return MemberReviewerAcceptance{}, false, err
			}
			candidates = append(candidates, acceptance)
		}
	}
	if len(candidates) == 0 {
		return MemberReviewerAcceptance{}, false, nil
	}
	if len(candidates) != 1 {
		return MemberReviewerAcceptance{}, false, fmt.Errorf("current member manifest %s requires exactly one canonical reviewer acceptance lineage; got %d", manifestRef, len(candidates))
	}
	return candidates[0], true, nil
}

func validateMemberReviewerAcceptancePair(caseRoot, laneID, manifestRef string, verification, decision map[string]any) (MemberReviewerAcceptance, error) {
	if !strings.EqualFold(mission.Value(verification, "reviewerDecision"), "accept") || !strings.EqualFold(mission.Value(verification, "recommendedVerdict"), "accepted") || !strings.EqualFold(mission.Value(decision, "reviewerDecision"), "accept") || !strings.EqualFold(mission.Value(decision, "recommendedVerdict"), "accepted") {
		return MemberReviewerAcceptance{}, fmt.Errorf("reviewer acceptance ledger semantics are not canonical accept/accepted")
	}
	fields := []string{"packetId", "routeId", "shardId", "packetPath", "reviewerResultPath", "reviewerSession", "reviewerDispatchReceiptPath", "reviewerDispatchReceiptSha256", "reviewerCompletionReceiptPath", "reviewerCompletionReceiptSha256", "reviewerResultInputPath", "reviewerResultInputSha256", "ownerExecutor", "ownerGeneration"}
	for _, field := range fields {
		left, right := mission.Value(verification, field), mission.Value(decision, field)
		if strings.TrimSpace(left) == "" || left != right {
			return MemberReviewerAcceptance{}, fmt.Errorf("reviewer acceptance ledger %s binding is missing or inconsistent", field)
		}
	}
	ownerGeneration, err := strconv.Atoi(mission.Value(decision, "ownerGeneration"))
	if err != nil || ownerGeneration <= 0 {
		return MemberReviewerAcceptance{}, fmt.Errorf("reviewer acceptance owner generation is invalid")
	}
	acceptance := MemberReviewerAcceptance{
		Lane: laneID, ManifestRef: filepath.ToSlash(manifestRef), PacketID: mission.Value(decision, "packetId"), RouteID: mission.Value(decision, "routeId"), ShardID: mission.Value(decision, "shardId"), PacketPath: mission.Value(decision, "packetPath"),
		ReviewerResultPath: mission.Value(decision, "reviewerResultPath"), ReviewerResultInputPath: mission.Value(decision, "reviewerResultInputPath"), ReviewerResultInputSHA256: mission.Value(decision, "reviewerResultInputSha256"), ReviewerSession: mission.Value(decision, "reviewerSession"),
		ReviewerDispatchPath: mission.Value(decision, "reviewerDispatchReceiptPath"), ReviewerDispatchSHA256: mission.Value(decision, "reviewerDispatchReceiptSha256"), ReviewerCompletionPath: mission.Value(decision, "reviewerCompletionReceiptPath"), ReviewerCompletionSHA256: mission.Value(decision, "reviewerCompletionReceiptSha256"),
		VerificationEventID: mission.Value(verification, "eventId"), DecisionEventID: mission.Value(decision, "eventId"), OwnerExecutor: mission.Value(decision, "ownerExecutor"), OwnerGeneration: ownerGeneration,
	}
	if acceptance.VerificationEventID == "" || acceptance.DecisionEventID == "" {
		return MemberReviewerAcceptance{}, fmt.Errorf("reviewer acceptance event identity is incomplete")
	}
	inputEvidence, err := validateReviewerWritebackPacket(caseRoot, laneID, manifestRef, acceptance.PacketID, acceptance.ShardID, acceptance.PacketPath, acceptance.ReviewerResultInputSHA256)
	if err != nil {
		return MemberReviewerAcceptance{}, err
	}
	if err := validateMemberReviewerAcceptanceReceipts(caseRoot, &acceptance, int(inputEvidence.Bytes)); err != nil {
		return MemberReviewerAcceptance{}, err
	}
	input, err := refsf.ReadStableRegularFileAnchored(caseRoot, acceptance.ReviewerResultInputPath, "accepted reviewer result input", 64<<10)
	if err != nil || inputEvidence.Bytes != int64(len(input)) || !strings.EqualFold(inputEvidence.SHA256, acceptance.ReviewerResultInputSHA256) {
		return MemberReviewerAcceptance{}, fmt.Errorf("accepted reviewer result input does not match canonical acceptance evidence")
	}
	result, err := reviewerresult.Decode(input)
	if err != nil || result.PacketID != acceptance.PacketID || result.RouteID != acceptance.RouteID || result.ShardID != acceptance.ShardID || result.ReviewerSession != acceptance.ReviewerSession || result.Decision != "accept" || result.RecommendedVerdict != "accepted" || !reviewerResultBindsManifest(caseRoot, result.Items, manifestRef) {
		return MemberReviewerAcceptance{}, fmt.Errorf("accepted reviewer result input does not match canonical acceptance bindings")
	}
	resultBytes, err := refsf.ReadStableRegularFileAnchored(caseRoot, acceptance.ReviewerResultPath, "accepted reviewer result", 64<<10)
	if err != nil {
		return MemberReviewerAcceptance{}, err
	}
	collected, err := reviewerresult.Decode(resultBytes)
	if err != nil {
		return MemberReviewerAcceptance{}, err
	}
	inputCanonical, _ := json.Marshal(result)
	resultCanonical, _ := json.Marshal(collected)
	if string(inputCanonical) != string(resultCanonical) {
		return MemberReviewerAcceptance{}, fmt.Errorf("collected reviewer result does not match canonical acceptance input")
	}
	manifestPath, err := anchoredReviewerRejectionPath(caseRoot, manifestRef)
	if err != nil {
		return MemberReviewerAcceptance{}, err
	}
	manifest, err := refsf.ReadStableRegularFileAnchored(caseRoot, manifestPath, "accepted member manifest", 4<<20)
	if err != nil {
		return MemberReviewerAcceptance{}, err
	}
	acceptance.ManifestSHA256 = reviewerDispatchBytesSHA256(manifest)
	acceptance.ReviewerResultSHA256 = reviewerDispatchBytesSHA256(resultBytes)
	acceptance.ReviewerResultInputBytes = int64(len(input))
	return acceptance, nil
}

func validateMemberReviewerAcceptanceReceipts(caseRoot string, acceptance *MemberReviewerAcceptance, inputBytes int) error {
	packetPath := filepath.FromSlash(strings.TrimSpace(acceptance.PacketPath))
	if !filepath.IsAbs(packetPath) {
		packetPath = filepath.Join(caseRoot, packetPath)
	}
	packet, err := readReviewerDispatchPacket(caseRoot, packetPath)
	if err != nil {
		return err
	}
	packetBytes, err := refsf.ReadStableRegularFileAnchored(caseRoot, packetPath, "accepted reviewer packet", 4<<20)
	if err != nil {
		return err
	}
	var shard *reviewerDispatchPacketDispatch
	for index := range packet.ReviewerOrchestration.Dispatches {
		if packet.ReviewerOrchestration.Dispatches[index].ShardID == acceptance.ShardID {
			shard = &packet.ReviewerOrchestration.Dispatches[index]
			break
		}
	}
	if shard == nil {
		return fmt.Errorf("reviewer packet does not contain accepted shard %s", acceptance.ShardID)
	}
	dispatchBytes, err := refsf.ReadStableRegularFileAnchored(caseRoot, acceptance.ReviewerDispatchPath, "accepted reviewer dispatch receipt", 256<<10)
	if err != nil || !strings.EqualFold(reviewerDispatchBytesSHA256(dispatchBytes), acceptance.ReviewerDispatchSHA256) {
		return fmt.Errorf("reviewer dispatch receipt changed after acceptance intake")
	}
	dispatch, err := reviewersession.DecodeDispatch(dispatchBytes)
	if err != nil || !reviewerDispatchSessionStaticBindingsCurrent(packet, packetPath, packetBytes, acceptance.Lane, *shard, dispatch, acceptance.ReviewerDispatchPath) || dispatch.ReviewerSession != acceptance.ReviewerSession || !reviewerDispatchSessionCurrentOwnerBindings(caseRoot, packet, packetPath, dispatch, acceptance.OwnerExecutor, acceptance.OwnerGeneration) {
		return fmt.Errorf("reviewer dispatch receipt does not match acceptance packet/session/current owner bindings")
	}
	if !casebind.SamePath(acceptance.ReviewerCompletionPath, reviewersession.CompletionPath(packetPath, acceptance.ShardID, dispatch.DispatchID)) {
		return fmt.Errorf("reviewer completion receipt path is not canonical")
	}
	completionBytes, err := refsf.ReadStableRegularFileAnchored(caseRoot, acceptance.ReviewerCompletionPath, "accepted reviewer completion receipt", 256<<10)
	if err != nil || !strings.EqualFold(reviewerDispatchBytesSHA256(completionBytes), acceptance.ReviewerCompletionSHA256) {
		return fmt.Errorf("reviewer completion receipt changed after acceptance intake")
	}
	completion, err := reviewersession.DecodeCompletion(completionBytes)
	if err != nil || reviewersession.ValidateCompletionDispatchLineage(completion, dispatch, acceptance.ReviewerDispatchPath, acceptance.ReviewerDispatchSHA256) != nil || completion.Outcome != "succeeded" || !casebind.SamePath(completion.ReviewerResultInputPath, acceptance.ReviewerResultInputPath) || !strings.EqualFold(completion.ReviewerResultInputSHA256, acceptance.ReviewerResultInputSHA256) || completion.ReviewerResultInputBytes != inputBytes || completion.CompletionOwner != dispatch.EffectiveOwner {
		return fmt.Errorf("reviewer completion receipt does not match canonical acceptance input lineage")
	}
	return nil
}
