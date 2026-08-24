package externalsession

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewerresult"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewersession"
)

func compactTransportReviewerPrompt(source string, receipt reviewersession.DispatchReceipt, fields []string) (string, bool) {
	resultShape, err := transportReviewerResultShape(receipt, fields)
	if err != nil {
		return source, false
	}
	contract := reviewerresult.CurrentContract()
	owner := receipt.EffectiveOwner
	ownerBinding := fmt.Sprintf(
		"targetLane=%s, mode=%s, currentExecutor=%s, executorGeneration=%d",
		receipt.TargetLane,
		owner.BindingMode,
		owner.CurrentExecutor,
		owner.ExecutorGeneration,
	)
	lines := []string{
		"Read-only Reviewer for a committed plan-subagents evidence closure.",
		"Identity: packetId=" + receipt.PacketID + ", routeId=" + receipt.RouteID + ", shardId=" + receipt.ShardID + ", reviewerSession=" + receipt.ReviewerSession + ".",
		"Owner binding: " + ownerBinding + ".",
		"Items (preserve exactly): " + mustJSON(receipt.Items) + ".",
		"Inspect only the complete inline bundle; paths are source-side logical references, not local files. Match every closure's member-task-context, member-evidence-manifest, and member-evidence-output artifacts by role. Verify path, owner, SHA-256, and byte bindings.",
		"Compare every output with taskContext.goal, expectedOutput, correction/reviewerRejection, and explicit acceptance requirements. Manifest self-consistency is insufficient; a missing required condition is contrary evidence and requires decision=reject with recommendedVerdict=rejected, not accept or defer.",
		"Return exactly one JSON object and no Markdown or surrounding prose: " + resultShape + ".",
		"Required top-level fields: " + strings.Join(contract.RequiredFields, ", ") + ".",
		"Required routeOutput fields: " + strings.Join(fields, ", ") + ". " + compactTransportRouteOutputGuidance(fields),
		"Allowed decisions: " + strings.Join(contract.AllowedDecisions, ", ") + ". Map accept=accepted, reject=rejected, defer/abandon=inconclusive, and needs-more-evidence=needs-more-evidence; do not invent synonyms.",
		"Choose accept only when immutable evidence supports the item; choose reject only with inspected contrary evidence; use defer or needs-more-evidence only for a concrete evidence gap. Low-confidence accept/reject is not independently writable.",
		"Use packetId " + receipt.PacketID + " in evidenceRefs and routeOutput.evidence when present. Keep conflicts empty unless unresolved. Never substitute an item path, absolute path, result/candidate/diff path, or another identifier for packetId.",
		"Reply only to the sending session. The main agent owns persistence, validation, handoff, and writeback. Do not write files or ledgers, run heavy tools, request external effects, or change authority/confirmed state.",
	}
	if blocked := canonicalTransportReviewerBlockedActions(source); blocked != "" {
		lines = append(lines, "Blocked runtime actions: "+blocked+".")
	}
	compacted := strings.Join(lines, " ")
	if len(compacted) >= len(source) {
		return source, false
	}
	return compacted, true
}

func canonicalTransportReviewerBlockedActions(text string) string {
	const marker = ". Blocked runtime actions: "
	_, blocked, ok := strings.Cut(text, marker)
	if !ok {
		return ""
	}
	return strings.TrimSuffix(strings.TrimSpace(blocked), ".")
}

func transportReviewerResultShape(receipt reviewersession.DispatchReceipt, fields []string) (string, error) {
	routeOutput := map[string]string{}
	for _, field := range fields {
		switch field {
		case "item":
			routeOutput[field] = strings.Join(receipt.Items, ",")
		case "decision":
			routeOutput[field] = "same as decision"
		case "confidence":
			routeOutput[field] = "same as confidence"
		case "evidence":
			routeOutput[field] = receipt.PacketID
		case "risk":
			routeOutput[field] = "evidence-based residual risk"
		case "next_action":
			routeOutput[field] = "main-agent review"
		case "tier_used":
			routeOutput[field] = "reviewer"
		case "tool_scope":
			routeOutput[field] = "read-only"
		case "defer_reason":
			routeOutput[field] = "concrete gap or n/a"
		default:
			routeOutput[field] = "evidence-based " + field
		}
	}
	shape := struct {
		PacketID           string            `json:"packetId"`
		RouteID            string            `json:"routeId"`
		ShardID            string            `json:"shardId"`
		Items              []string          `json:"items"`
		ReviewerSession    string            `json:"reviewerSession"`
		Decision           string            `json:"decision"`
		Confidence         string            `json:"confidence"`
		Summary            string            `json:"summary"`
		EvidenceRefs       []string          `json:"evidenceRefs"`
		Risks              []string          `json:"risks"`
		Conflicts          []string          `json:"conflicts"`
		RecommendedVerdict string            `json:"recommendedVerdict"`
		RouteOutput        map[string]string `json:"routeOutput"`
	}{
		PacketID: receipt.PacketID, RouteID: receipt.RouteID, ShardID: receipt.ShardID,
		Items: append([]string{}, receipt.Items...), ReviewerSession: receipt.ReviewerSession,
		Decision: "accept|reject|defer|abandon|needs-more-evidence", Confidence: "low|medium|high",
		Summary: "evidence-based summary", EvidenceRefs: []string{receipt.PacketID}, Risks: []string{}, Conflicts: []string{},
		RecommendedVerdict: "accepted|rejected|inconclusive|needs-more-evidence", RouteOutput: routeOutput,
	}
	data, err := json.Marshal(shape)
	return string(data), err
}

func compactTransportRouteOutputGuidance(fields []string) string {
	guidance := []string{"Use evidence-based values"}
	for _, field := range fields {
		switch field {
		case "item":
			guidance = append(guidance, "item must equal the reviewed item")
		case "decision", "confidence":
			guidance = append(guidance, field+" must equal the top-level value")
		case "evidence":
			guidance = append(guidance, "evidence must be inside evidenceRefs")
		case "next_action":
			guidance = append(guidance, "next_action must be main-agent review")
		case "tool_scope":
			guidance = append(guidance, "tool_scope must be read-only")
		case "defer_reason":
			guidance = append(guidance, "defer_reason must name the gap or be n/a")
		}
	}
	return strings.Join(guidance, "; ") + "."
}

func mustJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}
