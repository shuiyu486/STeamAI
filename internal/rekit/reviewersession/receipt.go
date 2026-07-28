package reviewersession

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
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
