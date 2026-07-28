package reviewerresult

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
)

const MaxBytes = 64 * 1024

type Result struct {
	PacketID           string         `json:"packetId"`
	RouteID            string         `json:"routeId"`
	ShardID            string         `json:"shardId"`
	Items              []string       `json:"items"`
	ReviewerSession    string         `json:"reviewerSession"`
	Decision           string         `json:"decision"`
	Confidence         string         `json:"confidence"`
	Summary            string         `json:"summary"`
	EvidenceRefs       []string       `json:"evidenceRefs"`
	Risks              []string       `json:"risks"`
	Conflicts          []string       `json:"conflicts"`
	RecommendedVerdict string         `json:"recommendedVerdict"`
	RouteOutput        map[string]any `json:"routeOutput,omitempty"`
}

type Contract struct {
	OutputFormat     string
	RequiredFields   []string
	AllowedDecisions []string
	EvidenceRules    []string
	ConflictSignals  []string
}

func CurrentContract() Contract {
	return Contract{
		OutputFormat:     "single JSON object per shard with route-specific fields nested under routeOutput; no markdown tables, file writes, ledger appends, authority, confirmed, or heavy-tool output",
		RequiredFields:   []string{"packetId", "routeId", "shardId", "items", "reviewerSession", "decision", "confidence", "summary", "evidenceRefs", "risks", "conflicts", "recommendedVerdict", "routeOutput"},
		AllowedDecisions: []string{"accept", "reject", "defer", "abandon", "needs-more-evidence"},
		EvidenceRules: []string{
			"accepted or rejected reviewer decisions must cite evidenceRefs from the packet, reviewed artifacts, or bounded evidence paths",
			"route-specific outputContract fields must be returned inside routeOutput so strict intake can preserve pack-specific data without allowing unknown top-level fields",
			"missing, ambiguous, or inaccessible evidenceRefs require decision=needs-more-evidence or defer",
			"do not paste long logs; cite stable packet/evidence references and summarize the relevant observation",
		},
		ConflictSignals: []string{
			"reviewer decision conflicts with evidenceRefs or route output contract",
			"reviewer requests file writes, ledger append, authority/confirmed changes, heavy tools, or external effects",
			"reviewer output overlaps another shard or changes items outside this shard",
			"reviewer confidence is low or evidence cannot be independently inspected by the main agent",
		},
	}
}

func Decode(data []byte) (Result, error) {
	if len(data) > MaxBytes {
		return Result{}, fmt.Errorf("reviewer result exceeds %d-byte limit", MaxBytes)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return Result{}, fmt.Errorf("reviewer result is empty")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		return Result{}, fmt.Errorf("reviewer result must contain exactly one JSON object: %w", err)
	}
	for _, field := range CurrentContract().RequiredFields {
		if _, ok := fields[field]; !ok {
			return Result{}, fmt.Errorf("reviewer result missing required field %q", field)
		}
	}
	for _, field := range []string{"items", "evidenceRefs", "risks", "conflicts"} {
		if err := requireJSONArrayField(fields, field); err != nil {
			return Result{}, err
		}
	}
	if err := requireJSONObjectField(fields, "routeOutput"); err != nil {
		return Result{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	var result Result
	if err := dec.Decode(&result); err != nil {
		return Result{}, fmt.Errorf("reviewer result contract validation failed: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return Result{}, fmt.Errorf("reviewer result must contain exactly one JSON object")
	}
	result.PacketID = strings.TrimSpace(result.PacketID)
	result.RouteID = strings.TrimSpace(result.RouteID)
	result.ShardID = strings.TrimSpace(result.ShardID)
	result.ReviewerSession = strings.TrimSpace(result.ReviewerSession)
	result.Decision = strings.TrimSpace(result.Decision)
	result.Confidence = strings.TrimSpace(result.Confidence)
	result.Summary = strings.TrimSpace(result.Summary)
	result.RecommendedVerdict = strings.TrimSpace(result.RecommendedVerdict)
	result.Items = cleanStrings(result.Items)
	result.EvidenceRefs = cleanStrings(result.EvidenceRefs)
	result.Risks = cleanStrings(result.Risks)
	result.Conflicts = cleanStrings(result.Conflicts)
	if result.PacketID == "" || result.RouteID == "" || result.ShardID == "" || result.ReviewerSession == "" || result.Summary == "" {
		return Result{}, fmt.Errorf("reviewer result packetId, routeId, shardId, reviewerSession, and summary must be non-empty")
	}
	if strings.ContainsAny(result.ReviewerSession, "\r\n") {
		return Result{}, fmt.Errorf("reviewer result reviewerSession must be a single-line session identifier")
	}
	if result.RouteOutput == nil {
		return Result{}, fmt.Errorf("reviewer result routeOutput must be an object, even when no route-specific values are needed")
	}
	contract := CurrentContract()
	if !slices.Contains(contract.AllowedDecisions, result.Decision) {
		return Result{}, fmt.Errorf("invalid reviewer decision %q; allowed: %s", result.Decision, strings.Join(contract.AllowedDecisions, ","))
	}
	if !slices.Contains([]string{"low", "medium", "high"}, result.Confidence) {
		return Result{}, fmt.Errorf("invalid reviewer confidence %q; allowed: low,medium,high", result.Confidence)
	}
	return result, nil
}

func requireJSONArrayField(fields map[string]json.RawMessage, field string) error {
	raw := bytes.TrimSpace(fields[field])
	if bytes.Equal(raw, []byte("null")) {
		return fmt.Errorf("reviewer result field %q must be an array, not null", field)
	}
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return fmt.Errorf("reviewer result field %q must be an array", field)
	}
	return nil
}

func requireJSONObjectField(fields map[string]json.RawMessage, field string) error {
	raw := bytes.TrimSpace(fields[field])
	if bytes.Equal(raw, []byte("null")) {
		return fmt.Errorf("reviewer result field %q must be an object, not null", field)
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("reviewer result field %q must be an object", field)
	}
	return nil
}

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
