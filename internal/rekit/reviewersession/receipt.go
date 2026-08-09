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

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
)

type Owner struct {
	CurrentExecutor    string `json:"currentExecutor,omitempty"`
	ExecutorGeneration int    `json:"executorGeneration,omitempty"`
	BindingMode        string `json:"bindingMode,omitempty"`
}

type DispatchReceipt struct {
	SchemaVersion       int      `json:"schemaVersion"`
	Kind                string   `json:"kind"`
	DispatchID          string   `json:"dispatchId"`
	PacketID            string   `json:"packetId"`
	PacketPath          string   `json:"packetPath"`
	PacketSHA256        string   `json:"packetSha256"`
	RouteID             string   `json:"routeId"`
	ShardID             string   `json:"shardId"`
	Items               []string `json:"items"`
	PromptPath          string   `json:"promptPath"`
	PromptSHA256        string   `json:"promptSha256"`
	AgentType           string   `json:"agentType"`
	ReadOnly            bool     `json:"readOnly"`
	TargetLane          string   `json:"targetLane"`
	PacketOwner         Owner    `json:"packetOwner"`
	EffectiveOwner      Owner    `json:"effectiveOwner"`
	OwnerAdoptionPath   string   `json:"ownerAdoptionPath,omitempty"`
	OwnerAdoptionSHA256 string   `json:"ownerAdoptionSha256,omitempty"`
	ReviewerHarness     string   `json:"reviewerHarness"`
	ReviewerSession     string   `json:"reviewerSession"`
	Actor               string   `json:"actor"`
	RecordedAt          string   `json:"recordedAt"`
	NoSpawn             bool     `json:"noSpawn"`
	NoHeavyTool         bool     `json:"noHeavyTool"`
	NoAuthority         bool     `json:"noAuthorityOrConfirmed"`
}

type CompletionReceipt struct {
	SchemaVersion             int    `json:"schemaVersion"`
	Kind                      string `json:"kind"`
	DispatchID                string `json:"dispatchId"`
	DispatchReceiptPath       string `json:"dispatchReceiptPath"`
	DispatchReceiptSHA256     string `json:"dispatchReceiptSha256"`
	PacketID                  string `json:"packetId"`
	RouteID                   string `json:"routeId"`
	ShardID                   string `json:"shardId"`
	ReviewerHarness           string `json:"reviewerHarness"`
	ReviewerSession           string `json:"reviewerSession"`
	Outcome                   string `json:"outcome"`
	ExitStatus                string `json:"exitStatus"`
	ReviewerResultInputPath   string `json:"reviewerResultInputPath,omitempty"`
	ReviewerResultInputSHA256 string `json:"reviewerResultInputSha256,omitempty"`
	ReviewerResultInputBytes  int    `json:"reviewerResultInputBytes,omitempty"`
	CompletionOwner           Owner  `json:"completionOwner"`
	OwnerAdoptionPath         string `json:"ownerAdoptionPath,omitempty"`
	OwnerAdoptionSHA256       string `json:"ownerAdoptionSha256,omitempty"`
	Actor                     string `json:"actor"`
	RecordedAt                string `json:"recordedAt"`
	NoCollection              bool   `json:"noCollection"`
	NoIntake                  bool   `json:"noIntake"`
	NoFacts                   bool   `json:"noFactsWrite"`
	NoHeavyTool               bool   `json:"noHeavyTool"`
	NoAuthority               bool   `json:"noAuthorityOrConfirmed"`
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
	if _, err := time.Parse(time.RFC3339Nano, receipt.RecordedAt); err != nil {
		return DispatchReceipt{}, fmt.Errorf("reviewer session dispatch recordedAt is invalid")
	}
	return receipt, nil
}

type packetContract struct {
	PacketID string `json:"packetId"`
	Route    struct {
		ID             string `json:"id"`
		OutputContract string `json:"outputContract"`
	} `json:"route"`
	Shards []struct {
		ID    string   `json:"id"`
		Items []string `json:"items"`
	} `json:"shards"`
	OutputContract string `json:"outputContract"`
}

func ReadDispatch(caseRoot, path, expectedSHA256 string) (DispatchReceipt, error) {
	if !filepath.IsAbs(path) {
		var err error
		path, err = rekitfs.SafeJoin(caseRoot, path)
		if err != nil {
			return DispatchReceipt{}, err
		}
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
	packetPath := receipt.PacketPath
	if !filepath.IsAbs(packetPath) {
		var err error
		packetPath, err = rekitfs.SafeJoin(caseRoot, packetPath)
		if err != nil {
			return nil, err
		}
	}
	data, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, packetPath, "reviewer dispatch packet", 4<<20)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(sha256Hex(data), receipt.PacketSHA256) {
		return nil, fmt.Errorf("reviewer dispatch packet sha256 mismatch")
	}
	var packet packetContract
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(data)))
	if err := dec.Decode(&packet); err != nil {
		return nil, fmt.Errorf("decode reviewer dispatch packet contract: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("reviewer dispatch packet contract must contain exactly one JSON object")
	}
	if packet.PacketID != receipt.PacketID || packet.Route.ID != receipt.RouteID || packet.Route.OutputContract != packet.OutputContract {
		return nil, fmt.Errorf("reviewer dispatch packet does not match receipt packet and route contract")
	}
	matchedShard := false
	for _, shard := range packet.Shards {
		if shard.ID == receipt.ShardID {
			matchedShard = true
			if !slices.Equal(shard.Items, receipt.Items) {
				return nil, fmt.Errorf("reviewer dispatch packet shard items do not match receipt")
			}
			break
		}
	}
	if !matchedShard {
		return nil, fmt.Errorf("reviewer dispatch packet omitted receipt shard")
	}
	fields := splitOutputContract(packet.OutputContract)
	if len(fields) == 0 {
		return nil, fmt.Errorf("reviewer dispatch packet output contract is empty")
	}
	return fields, nil
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
	if receipt.Outcome == "succeeded" && (receipt.ReviewerResultInputPath == "" || receipt.ReviewerResultInputSHA256 == "" || receipt.ReviewerResultInputBytes <= 0) {
		return CompletionReceipt{}, fmt.Errorf("successful reviewer session completion receipt lacks result input binding")
	}
	if _, err := time.Parse(time.RFC3339Nano, receipt.RecordedAt); err != nil {
		return CompletionReceipt{}, fmt.Errorf("reviewer session completion recordedAt is invalid")
	}
	return receipt, nil
}

func ValidateCompletionDispatchLineage(completion CompletionReceipt, dispatch DispatchReceipt, dispatchPath, dispatchSHA256 string) error {
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
