package reviewersession

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/capabilitycontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewpath"
)

type Owner struct {
	CurrentExecutor    string `json:"currentExecutor,omitempty"`
	ExecutorGeneration int    `json:"executorGeneration,omitempty"`
	BindingMode        string `json:"bindingMode,omitempty"`
}

type DispatchReceipt struct {
	SchemaVersion       int                        `json:"schemaVersion"`
	Kind                string                     `json:"kind"`
	DispatchID          string                     `json:"dispatchId"`
	PacketID            string                     `json:"packetId"`
	PacketPath          string                     `json:"packetPath"`
	PacketSHA256        string                     `json:"packetSha256"`
	RouteID             string                     `json:"routeId"`
	ShardID             string                     `json:"shardId"`
	Items               []string                   `json:"items"`
	PromptPath          string                     `json:"promptPath"`
	PromptSHA256        string                     `json:"promptSha256"`
	AgentType           string                     `json:"agentType"`
	ReadOnly            bool                       `json:"readOnly"`
	TargetLane          string                     `json:"targetLane"`
	PacketOwner         Owner                      `json:"packetOwner"`
	EffectiveOwner      Owner                      `json:"effectiveOwner"`
	OwnerAdoptionPath   string                     `json:"ownerAdoptionPath,omitempty"`
	OwnerAdoptionSHA256 string                     `json:"ownerAdoptionSha256,omitempty"`
	ReviewerHarness     string                     `json:"reviewerHarness"`
	ReviewerSession     string                     `json:"reviewerSession"`
	Actor               string                     `json:"actor"`
	RecordedAt          string                     `json:"recordedAt"`
	Capability          capabilitycontract.Binding `json:"capability"`
	LaunchControl       *executioncontrol.Binding  `json:"launchControl,omitempty"`
	NoSpawn             bool                       `json:"noSpawn"`
	NoHeavyTool         bool                       `json:"noHeavyTool"`
	NoAuthority         bool                       `json:"noAuthorityOrConfirmed"`
}

type CompletionReceipt struct {
	SchemaVersion             int                        `json:"schemaVersion"`
	Kind                      string                     `json:"kind"`
	DispatchID                string                     `json:"dispatchId"`
	DispatchReceiptPath       string                     `json:"dispatchReceiptPath"`
	DispatchReceiptSHA256     string                     `json:"dispatchReceiptSha256"`
	PacketID                  string                     `json:"packetId"`
	RouteID                   string                     `json:"routeId"`
	ShardID                   string                     `json:"shardId"`
	ReviewerHarness           string                     `json:"reviewerHarness"`
	ReviewerSession           string                     `json:"reviewerSession"`
	Outcome                   string                     `json:"outcome"`
	ExitStatus                string                     `json:"exitStatus"`
	ReviewerResultInputPath   string                     `json:"reviewerResultInputPath,omitempty"`
	ReviewerResultInputSHA256 string                     `json:"reviewerResultInputSha256,omitempty"`
	ReviewerResultInputBytes  int                        `json:"reviewerResultInputBytes,omitempty"`
	CompletionOwner           Owner                      `json:"completionOwner"`
	OwnerAdoptionPath         string                     `json:"ownerAdoptionPath,omitempty"`
	OwnerAdoptionSHA256       string                     `json:"ownerAdoptionSha256,omitempty"`
	Actor                     string                     `json:"actor"`
	RecordedAt                string                     `json:"recordedAt"`
	Capability                capabilitycontract.Binding `json:"capability"`
	NoCollection              bool                       `json:"noCollection"`
	NoIntake                  bool                       `json:"noIntake"`
	NoFacts                   bool                       `json:"noFactsWrite"`
	NoHeavyTool               bool                       `json:"noHeavyTool"`
	NoAuthority               bool                       `json:"noAuthorityOrConfirmed"`
}

func DispatchPath(packetPath, shardID, dispatchID string) string {
	return filepath.Join(filepath.Dir(packetPath), "sessions", shardID, "dispatches", dispatchID+".json")
}

func CompletionPath(packetPath, shardID, dispatchID string) string {
	return filepath.Join(filepath.Dir(packetPath), "sessions", shardID, "completions", dispatchID+".json")
}

func DecodeDispatch(data []byte) (DispatchReceipt, error) {
	var receipt DispatchReceipt
	if err := decodeStrict(data, &receipt, "reviewer session dispatch receipt"); err != nil {
		return DispatchReceipt{}, err
	}
	if receipt.SchemaVersion != 1 || receipt.Kind != "reviewer-session-dispatch" || receipt.DispatchID == "" || receipt.PacketID == "" || receipt.RouteID == "" || receipt.ShardID == "" || receipt.PromptSHA256 == "" || receipt.ReviewerHarness == "" || receipt.ReviewerSession == "" || !receipt.ReadOnly || !receipt.NoSpawn || !receipt.NoHeavyTool || !receipt.NoAuthority {
		return DispatchReceipt{}, fmt.Errorf("reviewer session dispatch receipt has invalid required bindings")
	}
	if err := capabilitycontract.RequireBindingPolicy(receipt.Capability, capabilitycontract.PolicyClassReadOnly); err != nil {
		return DispatchReceipt{}, fmt.Errorf("reviewer session dispatch capability contract is invalid: %w", err)
	}
	if receipt.LaunchControl != nil {
		if err := executioncontrol.ValidateBinding(*receipt.LaunchControl); err != nil {
			return DispatchReceipt{}, fmt.Errorf("reviewer session dispatch launch control binding is invalid: %w", err)
		}
		if receipt.LaunchControl.Lane != receipt.TargetLane ||
			receipt.LaunchControl.Owner.Lane != receipt.TargetLane ||
			receipt.LaunchControl.Owner.CurrentExecutor != receipt.EffectiveOwner.CurrentExecutor ||
			receipt.LaunchControl.Owner.ExecutorGeneration != receipt.EffectiveOwner.ExecutorGeneration ||
			receipt.LaunchControl.Capability != receipt.Capability {
			return DispatchReceipt{}, fmt.Errorf("reviewer session dispatch launch control does not match owner and capability lineage")
		}
	}
	if _, err := time.Parse(time.RFC3339Nano, receipt.RecordedAt); err != nil {
		return DispatchReceipt{}, fmt.Errorf("reviewer session dispatch recordedAt is invalid")
	}
	return receipt, nil
}

type packetContract struct {
	PacketID string `json:"packetId"`
	Command  string `json:"command"`
	Route    struct {
		ID             string `json:"id"`
		OutputContract string `json:"outputContract"`
	} `json:"route"`
	Shards []struct {
		ID    string   `json:"id"`
		Items []string `json:"items"`
	} `json:"shards"`
	ShardHandoffs         []packetPromptBinding `json:"shardHandoffs"`
	ReviewerOrchestration struct {
		Dispatches []packetPromptBinding `json:"dispatches"`
	} `json:"reviewerOrchestration"`
	OutputContract string `json:"outputContract"`
}

type packetPromptBinding struct {
	ShardID              string                    `json:"shardId"`
	Items                []string                  `json:"items"`
	DispatchPrompt       string                    `json:"dispatchPrompt"`
	DispatchPromptPath   string                    `json:"dispatchPromptPath"`
	DispatchPromptSHA256 string                    `json:"dispatchPromptSha256"`
	AgentToolRequest     *packetPromptAgentRequest `json:"agentToolRequest"`
}

type packetPromptAgentRequest struct {
	Tool         string `json:"tool"`
	AgentType    string `json:"agentType"`
	ReadOnly     bool   `json:"readOnly"`
	Prompt       string `json:"prompt"`
	PromptPath   string `json:"promptPath"`
	PromptSHA256 string `json:"promptSha256"`
}

func ReadDispatch(caseRoot, path, expectedSHA256 string) (DispatchReceipt, error) {
	if !filepath.IsAbs(path) {
		var err error
		path, err = rekitfs.SafeJoin(caseRoot, path)
		if err != nil {
			return DispatchReceipt{}, err
		}
	}
	reviewRoot := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(path))))
	packetPath := filepath.Join(reviewRoot, "packet.json")
	if _, ok := reviewpath.CanonicalCollectionNamespace(caseRoot, packetPath); !ok || !reviewpath.CollectionNamespacePathSafe(caseRoot, path, false) {
		return DispatchReceipt{}, fmt.Errorf("reviewer dispatch receipt path is outside the active review namespace")
	}
	data, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, path, "reviewer dispatch receipt", 1<<20)
	if err != nil {
		return DispatchReceipt{}, err
	}
	if !validSHA256(expectedSHA256) || !strings.EqualFold(sha256Hex(data), expectedSHA256) {
		return DispatchReceipt{}, fmt.Errorf("reviewer dispatch receipt sha256 mismatch")
	}
	return DecodeDispatch(data)
}

func OutputContractFields(caseRoot string, receipt DispatchReceipt) ([]string, error) {
	packet, err := readPacketContract(caseRoot, receipt)
	if err != nil {
		return nil, err
	}
	fields := splitOutputContract(packet.OutputContract)
	if len(fields) == 0 {
		return nil, fmt.Errorf("reviewer dispatch packet output contract is empty")
	}
	return fields, nil
}

// CanonicalDispatchPrompt returns the exact packet-owned prompt bytes only when
// every duplicated prompt binding agrees with the current dispatch receipt.
// Legacy or custom packets remain valid reviewer inputs but are not canonical
// prompt projection sources.
func CanonicalDispatchPrompt(caseRoot string, receipt DispatchReceipt) (string, bool, error) {
	packet, err := readPacketContract(caseRoot, receipt)
	if err != nil {
		return "", false, err
	}
	if packet.Command != "plan-subagents" {
		return "", false, nil
	}
	handoff, handoffOK := exactPacketPromptBinding(packet.ShardHandoffs, receipt.ShardID)
	dispatch, dispatchOK := exactPacketPromptBinding(packet.ReviewerOrchestration.Dispatches, receipt.ShardID)
	if !handoffOK || !dispatchOK ||
		!packetPromptBindingMatchesReceipt(caseRoot, handoff, receipt) ||
		!packetPromptBindingMatchesReceipt(caseRoot, dispatch, receipt) ||
		!packetPromptBindingsEqual(caseRoot, handoff, dispatch) {
		return "", false, nil
	}
	prompt := strings.TrimRight(handoff.DispatchPrompt, "\r\n") + "\n"
	if !strings.EqualFold(sha256Hex([]byte(prompt)), receipt.PromptSHA256) {
		return "", false, nil
	}
	return prompt, true, nil
}

func readPacketContract(caseRoot string, receipt DispatchReceipt) (packetContract, error) {
	packetPath := receipt.PacketPath
	if !filepath.IsAbs(packetPath) {
		var err error
		packetPath, err = rekitfs.SafeJoin(caseRoot, packetPath)
		if err != nil {
			return packetContract{}, err
		}
	}
	if _, ok := reviewpath.CanonicalCollectionNamespace(caseRoot, packetPath); !ok || !reviewpath.CollectionNamespacePathSafe(caseRoot, packetPath, false) {
		return packetContract{}, fmt.Errorf("reviewer dispatch packet path is outside the active review namespace")
	}
	data, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, packetPath, "reviewer dispatch packet", 4<<20)
	if err != nil {
		return packetContract{}, err
	}
	if !strings.EqualFold(sha256Hex(data), receipt.PacketSHA256) {
		return packetContract{}, fmt.Errorf("reviewer dispatch packet sha256 mismatch")
	}
	var packet packetContract
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(data)))
	if err := dec.Decode(&packet); err != nil {
		return packetContract{}, fmt.Errorf("decode reviewer dispatch packet contract: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return packetContract{}, fmt.Errorf("reviewer dispatch packet contract must contain exactly one JSON object")
	}
	if packet.PacketID != receipt.PacketID || packet.Route.ID != receipt.RouteID || packet.Route.OutputContract != packet.OutputContract {
		return packetContract{}, fmt.Errorf("reviewer dispatch packet does not match receipt packet and route contract")
	}
	matchedShard := false
	for _, shard := range packet.Shards {
		if shard.ID == receipt.ShardID {
			matchedShard = true
			if !slices.Equal(shard.Items, receipt.Items) {
				return packetContract{}, fmt.Errorf("reviewer dispatch packet shard items do not match receipt")
			}
			break
		}
	}
	if !matchedShard {
		return packetContract{}, fmt.Errorf("reviewer dispatch packet omitted receipt shard")
	}
	return packet, nil
}

func exactPacketPromptBinding(items []packetPromptBinding, shardID string) (packetPromptBinding, bool) {
	var matched packetPromptBinding
	found := false
	for _, item := range items {
		if item.ShardID != shardID {
			continue
		}
		if found {
			return packetPromptBinding{}, false
		}
		matched = item
		found = true
	}
	return matched, found
}

func packetPromptBindingMatchesReceipt(caseRoot string, binding packetPromptBinding, receipt DispatchReceipt) bool {
	request := binding.AgentToolRequest
	return slices.Equal(binding.Items, receipt.Items) &&
		strings.TrimSpace(binding.DispatchPrompt) != "" &&
		samePacketPromptPath(caseRoot, binding.DispatchPromptPath, receipt.PromptPath) &&
		strings.EqualFold(binding.DispatchPromptSHA256, receipt.PromptSHA256) &&
		request != nil && request.Tool == "Claude Code Agent" &&
		request.AgentType == "read-only-reviewer" && request.ReadOnly &&
		request.Prompt == binding.DispatchPrompt &&
		samePacketPromptPath(caseRoot, request.PromptPath, receipt.PromptPath) &&
		strings.EqualFold(request.PromptSHA256, receipt.PromptSHA256)
}

func packetPromptBindingsEqual(caseRoot string, left, right packetPromptBinding) bool {
	return left.ShardID == right.ShardID && slices.Equal(left.Items, right.Items) &&
		left.DispatchPrompt == right.DispatchPrompt &&
		samePacketPromptPath(caseRoot, left.DispatchPromptPath, right.DispatchPromptPath) &&
		strings.EqualFold(left.DispatchPromptSHA256, right.DispatchPromptSHA256) &&
		left.AgentToolRequest != nil && right.AgentToolRequest != nil &&
		left.AgentToolRequest.Tool == right.AgentToolRequest.Tool &&
		left.AgentToolRequest.AgentType == right.AgentToolRequest.AgentType &&
		left.AgentToolRequest.ReadOnly == right.AgentToolRequest.ReadOnly &&
		left.AgentToolRequest.Prompt == right.AgentToolRequest.Prompt &&
		samePacketPromptPath(caseRoot, left.AgentToolRequest.PromptPath, right.AgentToolRequest.PromptPath) &&
		strings.EqualFold(left.AgentToolRequest.PromptSHA256, right.AgentToolRequest.PromptSHA256)
}

func samePacketPromptPath(caseRoot, left, right string) bool {
	anchor := func(path string) (string, bool) {
		path = strings.TrimSpace(path)
		if path == "" {
			return "", false
		}
		if filepath.IsAbs(path) {
			return path, true
		}
		full, err := rekitfs.SafeJoin(caseRoot, path)
		return full, err == nil
	}
	left, leftOK := anchor(left)
	right, rightOK := anchor(right)
	return leftOK && rightOK && rekitfs.SamePath(left, right)
}

func ValidateRouteOutput(fields []string, routeOutput map[string]any) error {
	allowed := map[string]bool{}
	for _, field := range fields {
		allowed[field] = true
		value, ok := routeOutput[field]
		if !ok || value == nil {
			return fmt.Errorf("reviewer result routeOutput missing required outputContract field %q", field)
		}
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return fmt.Errorf("reviewer result routeOutput field %q must be a non-empty string", field)
		}
	}
	for field := range routeOutput {
		if !allowed[field] {
			return fmt.Errorf("reviewer result routeOutput contains unknown field %q", field)
		}
	}
	return nil
}

func splitOutputContract(value string) []string {
	fields := []string{}
	seen := map[string]bool{}
	for _, field := range strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
	}) {
		field = strings.TrimSpace(field)
		if field != "" && !seen[field] {
			seen[field] = true
			fields = append(fields, field)
		}
	}
	return fields
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validSHA256(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func DecodeCompletion(data []byte) (CompletionReceipt, error) {
	var receipt CompletionReceipt
	if err := decodeStrict(data, &receipt, "reviewer session completion receipt"); err != nil {
		return CompletionReceipt{}, err
	}
	if receipt.SchemaVersion != 1 || receipt.Kind != "reviewer-session-completion" || receipt.DispatchID == "" || receipt.DispatchReceiptPath == "" || receipt.DispatchReceiptSHA256 == "" || receipt.PacketID == "" || receipt.RouteID == "" || receipt.ShardID == "" || receipt.ReviewerHarness == "" || receipt.ReviewerSession == "" || (receipt.Outcome != "succeeded" && receipt.Outcome != "failed") || receipt.ExitStatus == "" || !receipt.NoCollection || !receipt.NoIntake || !receipt.NoFacts || !receipt.NoHeavyTool || !receipt.NoAuthority {
		return CompletionReceipt{}, fmt.Errorf("reviewer session completion receipt has invalid required bindings")
	}
	if err := capabilitycontract.RequireBindingPolicy(receipt.Capability, capabilitycontract.PolicyClassReadOnly); err != nil {
		return CompletionReceipt{}, fmt.Errorf("reviewer session completion capability contract is invalid: %w", err)
	}
	if receipt.Outcome == "succeeded" && (receipt.ReviewerResultInputPath == "" || receipt.ReviewerResultInputSHA256 == "" || receipt.ReviewerResultInputBytes <= 0) {
		return CompletionReceipt{}, fmt.Errorf("successful reviewer session completion receipt lacks result input binding")
	}
	if _, err := time.Parse(time.RFC3339Nano, receipt.RecordedAt); err != nil {
		return CompletionReceipt{}, fmt.Errorf("reviewer session completion recordedAt is invalid")
	}
	return receipt, nil
}

func ValidateCompletionDispatchLineage(completion CompletionReceipt, dispatch DispatchReceipt, dispatchPath, dispatchSHA256 string) error {
	if err := capabilitycontract.RequireBindingPolicy(dispatch.Capability, capabilitycontract.PolicyClassReadOnly); err != nil {
		return fmt.Errorf("reviewer session dispatch capability lineage is invalid: %w", err)
	}
	if err := capabilitycontract.RequireBindingPolicy(completion.Capability, capabilitycontract.PolicyClassReadOnly); err != nil {
		return fmt.Errorf("reviewer session completion capability lineage is invalid: %w", err)
	}
	if completion.Capability != dispatch.Capability {
		return fmt.Errorf("reviewer session completion capability contract does not match dispatch lineage")
	}
	if completion.DispatchID != dispatch.DispatchID || !casebind.SamePath(completion.DispatchReceiptPath, dispatchPath) || completion.DispatchReceiptSHA256 != dispatchSHA256 || completion.PacketID != dispatch.PacketID || completion.RouteID != dispatch.RouteID || completion.ShardID != dispatch.ShardID || completion.ReviewerHarness != dispatch.ReviewerHarness || completion.ReviewerSession != dispatch.ReviewerSession || completion.CompletionOwner != dispatch.EffectiveOwner || !sameOptionalPath(completion.OwnerAdoptionPath, dispatch.OwnerAdoptionPath) || completion.OwnerAdoptionSHA256 != dispatch.OwnerAdoptionSHA256 {
		return fmt.Errorf("reviewer session completion receipt does not match dispatch receipt lineage")
	}
	return nil
}

func sameOptionalPath(left, right string) bool {
	if left == "" || right == "" {
		return left == right
	}
	return casebind.SamePath(left, right)
}

func decodeStrict(data []byte, out any, label string) error {
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", label, err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("%s must contain exactly one JSON object", label)
	}
	return nil
}
