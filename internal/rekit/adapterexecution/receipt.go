package adapterexecution

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	"github.com/shuiyu486/re-context-kits/internal/rekit/capabilitycontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
)

type GateBinding struct {
	GateEventID      string            `json:"gateEventId"`
	Lane             string            `json:"lane"`
	Action           string            `json:"action"`
	Target           string            `json:"target,omitempty"`
	Authorization    autonomy.Decision `json:"authorization"`
	AuthorizedBudget autonomy.Budget   `json:"authorizedBudget"`
	OutputPaths      []string          `json:"outputPaths"`
	StopConditions   []string          `json:"stopConditions,omitempty"`
	SnapshotSHA256   string            `json:"snapshotSha256"`
}

type Candidate struct {
	ID                  string   `json:"id"`
	Status              string   `json:"status,omitempty"`
	Entry               string   `json:"entry,omitempty"`
	Purpose             string   `json:"purpose,omitempty"`
	SideEffects         []string `json:"sideEffects,omitempty"`
	GateActions         []string `json:"gateActions,omitempty"`
	ToolingCatalogPath  string   `json:"toolingCatalogPath"`
	StopConditionHints  []string `json:"stopConditionHints,omitempty"`
	RecordOnlyAfterGate bool     `json:"recordOnlyAfterGate"`
}

type AdapterBinding struct {
	Pack                    string    `json:"pack"`
	AdapterID               string    `json:"adapterId"`
	ToolingCatalogPath      string    `json:"toolingCatalogPath"`
	ToolingCatalogSHA256    string    `json:"toolingCatalogSha256"`
	ToolingCatalogBytes     int64     `json:"toolingCatalogBytes"`
	Candidate               Candidate `json:"candidate"`
	CandidateSnapshotSHA256 string    `json:"candidateSnapshotSha256"`
}

type OwnerBinding struct {
	Lane               string `json:"lane"`
	CurrentExecutor    string `json:"currentExecutor"`
	ExecutorGeneration int    `json:"executorGeneration"`
	AdapterHarness     string `json:"adapterHarness"`
	AdapterSession     string `json:"adapterSession"`
	BindingMode        string `json:"bindingMode"`
}

type ExecutionBinding struct {
	Outcome          string          `json:"outcome"`
	ExitStatus       string          `json:"exitStatus"`
	AuthorizedBudget autonomy.Budget `json:"authorizedBudget"`
	ActualBudget     autonomy.Budget `json:"actualBudget"`
	BoundaryHits     []string        `json:"boundaryHits,omitempty"`
	Escalation       string          `json:"escalation,omitempty"`
}

type DispatchReceipt struct {
	SchemaVersion int                        `json:"schemaVersion"`
	Kind          string                     `json:"kind"`
	DispatchID    string                     `json:"dispatchId"`
	Gate          GateBinding                `json:"gate"`
	Adapter       AdapterBinding             `json:"adapter"`
	Owner         OwnerBinding               `json:"owner"`
	ReportPath    string                     `json:"reportPath"`
	Actor         string                     `json:"actor"`
	RecordedAt    string                     `json:"recordedAt"`
	Capability    capabilitycontract.Binding `json:"capability"`
	LaunchControl *executioncontrol.Binding  `json:"launchControl,omitempty"`
	NoExecute     bool                       `json:"noAdapterOrHeavyToolExecution"`
	NoObservation bool                       `json:"noObservationWrite"`
	NoAuthority   bool                       `json:"noAuthorityOrConfirmed"`
}

type DispatchSemanticBinding struct {
	SchemaVersion int                        `json:"schemaVersion"`
	Kind          string                     `json:"kind"`
	Gate          GateBinding                `json:"gate"`
	Adapter       AdapterBinding             `json:"adapter"`
	Owner         OwnerBinding               `json:"owner"`
	ReportPath    string                     `json:"reportPath"`
	Actor         string                     `json:"actor"`
	Capability    capabilitycontract.Binding `json:"capability"`
	LaunchControl *executioncontrol.Binding  `json:"launchControl,omitempty"`
	NoExecute     bool                       `json:"noAdapterOrHeavyToolExecution"`
	NoObservation bool                       `json:"noObservationWrite"`
	NoAuthority   bool                       `json:"noAuthorityOrConfirmed"`
}

type ReportDispatchBinding struct {
	DispatchID string `json:"dispatchId"`
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
}

type DispatchBinding struct {
	DispatchID string `json:"dispatchId"`
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	Bytes      int64  `json:"bytes"`
}

type FileBinding struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type ArtifactBinding struct {
	Path   string   `json:"path"`
	Roles  []string `json:"roles"`
	SHA256 string   `json:"sha256"`
	Bytes  int64    `json:"bytes"`
}

type Receipt struct {
	SchemaVersion int                        `json:"schemaVersion"`
	Kind          string                     `json:"kind"`
	ReceiptID     string                     `json:"receiptId"`
	Dispatch      DispatchBinding            `json:"dispatch"`
	Gate          GateBinding                `json:"gate"`
	Adapter       AdapterBinding             `json:"adapter"`
	Owner         OwnerBinding               `json:"owner"`
	Execution     ExecutionBinding           `json:"execution"`
	Report        FileBinding                `json:"report"`
	Artifacts     []ArtifactBinding          `json:"artifacts"`
	Actor         string                     `json:"actor"`
	RecordedAt    string                     `json:"recordedAt"`
	Capability    capabilitycontract.Binding `json:"capability"`
	NoExecute     bool                       `json:"noAdapterOrHeavyToolExecution"`
	NoAuthority   bool                       `json:"noAuthorityOrConfirmed"`
}

type Binding struct {
	SchemaVersion int                        `json:"schemaVersion"`
	Kind          string                     `json:"kind"`
	Dispatch      DispatchBinding            `json:"dispatch"`
	Gate          GateBinding                `json:"gate"`
	Adapter       AdapterBinding             `json:"adapter"`
	Owner         OwnerBinding               `json:"owner"`
	Execution     ExecutionBinding           `json:"execution"`
	Report        FileBinding                `json:"report"`
	Artifacts     []ArtifactBinding          `json:"artifacts"`
	Actor         string                     `json:"actor"`
	Capability    capabilitycontract.Binding `json:"capability"`
	NoExecute     bool                       `json:"noAdapterOrHeavyToolExecution"`
	NoAuthority   bool                       `json:"noAuthorityOrConfirmed"`
}

func BindingFor(receipt Receipt) Binding {
	artifacts := append([]ArtifactBinding{}, receipt.Artifacts...)
	for i := range artifacts {
		artifacts[i].Roles = append([]string{}, artifacts[i].Roles...)
		sort.Strings(artifacts[i].Roles)
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	return Binding{
		SchemaVersion: receipt.SchemaVersion,
		Kind:          receipt.Kind,
		Dispatch:      receipt.Dispatch,
		Gate:          receipt.Gate,
		Adapter:       receipt.Adapter,
		Owner:         receipt.Owner,
		Execution:     receipt.Execution,
		Report:        receipt.Report,
		Artifacts:     artifacts,
		Actor:         receipt.Actor,
		Capability:    receipt.Capability,
		NoExecute:     receipt.NoExecute,
		NoAuthority:   receipt.NoAuthority,
	}
}

func BindingSHA256(receipt Receipt) (string, error) {
	data, err := json.Marshal(BindingFor(receipt))
	if err != nil {
		return "", err
	}
	return SHA256(data), nil
}

func DispatchBindingFor(receipt DispatchReceipt) DispatchSemanticBinding {
	return DispatchSemanticBinding{
		SchemaVersion: receipt.SchemaVersion,
		Kind:          receipt.Kind,
		Gate:          receipt.Gate,
		Adapter:       receipt.Adapter,
		Owner:         receipt.Owner,
		ReportPath:    receipt.ReportPath,
		Actor:         receipt.Actor,
		Capability:    receipt.Capability,
		LaunchControl: executioncontrol.CloneBinding(receipt.LaunchControl),
		NoExecute:     receipt.NoExecute,
		NoObservation: receipt.NoObservation,
		NoAuthority:   receipt.NoAuthority,
	}
}

func DispatchBindingSHA256(receipt DispatchReceipt) (string, error) {
	data, err := json.Marshal(DispatchBindingFor(receipt))
	if err != nil {
		return "", err
	}
	return SHA256(data), nil
}

func CandidateSHA256(candidate Candidate) (string, error) {
	data, err := json.Marshal(candidate)
	if err != nil {
		return "", err
	}
	return SHA256(data), nil
}

func GateSHA256(binding GateBinding) (string, error) {
	binding.SnapshotSHA256 = ""
	data, err := json.Marshal(binding)
	if err != nil {
		return "", err
	}
	return SHA256(data), nil
}

func ReceiptBytes(receipt Receipt) ([]byte, error) {
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func DispatchReceiptBytes(receipt DispatchReceipt) ([]byte, error) {
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func DecodeDispatch(data []byte) (DispatchReceipt, error) {
	var receipt DispatchReceipt
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&receipt); err != nil {
		return DispatchReceipt{}, fmt.Errorf("invalid adapter execution dispatch receipt: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return DispatchReceipt{}, fmt.Errorf("invalid adapter execution dispatch receipt: trailing data")
	}
	if err := ValidateDispatch(receipt); err != nil {
		return DispatchReceipt{}, err
	}
	return receipt, nil
}

func Decode(data []byte) (Receipt, error) {
	var receipt Receipt
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&receipt); err != nil {
		return Receipt{}, fmt.Errorf("invalid adapter execution receipt: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return Receipt{}, fmt.Errorf("invalid adapter execution receipt: trailing data")
	}
	if err := Validate(receipt); err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

func ValidateDispatch(receipt DispatchReceipt) error {
	if receipt.SchemaVersion != 1 || receipt.Kind != "adapter-execution-dispatch-receipt" {
		return fmt.Errorf("adapter execution dispatch receipt schema/kind is invalid")
	}
	if !validSHA256(receipt.DispatchID) || receipt.Gate.GateEventID == "" || receipt.Gate.Lane == "" || receipt.Gate.Action == "" || receipt.Gate.Authorization.Decision != autonomy.DecisionPreauthorized || !validStateRootRelativePath(receipt.Gate.Authorization.ProfilePath) || !validSHA256(receipt.Gate.SnapshotSHA256) {
		return fmt.Errorf("adapter execution dispatch receipt gate binding is invalid")
	}
	if receipt.Adapter.Pack == "" || receipt.Adapter.AdapterID == "" || receipt.Adapter.ToolingCatalogPath == "" || !validSHA256(receipt.Adapter.ToolingCatalogSHA256) || receipt.Adapter.ToolingCatalogBytes < 0 || receipt.Adapter.Candidate.ID == "" || !validSHA256(receipt.Adapter.CandidateSnapshotSHA256) {
		return fmt.Errorf("adapter execution dispatch receipt adapter binding is invalid")
	}
	if receipt.Owner.Lane != receipt.Gate.Lane || receipt.Owner.CurrentExecutor == "" || receipt.Owner.ExecutorGeneration <= 0 || receipt.Owner.AdapterHarness == "" || receipt.Owner.AdapterSession == "" || receipt.Owner.BindingMode != "durable-lane-owner" {
		return fmt.Errorf("adapter execution dispatch receipt owner binding is invalid")
	}
	if !validRelativePath(receipt.ReportPath) {
		return fmt.Errorf("adapter execution dispatch receipt report path binding is invalid")
	}
	if strings.TrimSpace(receipt.Actor) == "" || strings.TrimSpace(receipt.RecordedAt) == "" || !receipt.NoExecute || !receipt.NoObservation || !receipt.NoAuthority {
		return fmt.Errorf("adapter execution dispatch receipt boundary is invalid")
	}
	if err := capabilitycontract.RequireBindingPolicy(receipt.Capability, capabilitycontract.PolicyClassAuthorizedHeavy); err != nil {
		return fmt.Errorf("adapter execution dispatch capability contract is invalid: %w", err)
	}
	if receipt.LaunchControl != nil {
		if err := executioncontrol.ValidateBinding(*receipt.LaunchControl); err != nil {
			return fmt.Errorf("adapter execution dispatch launch control is invalid: %w", err)
		}
		if receipt.LaunchControl.Lane != receipt.Owner.Lane ||
			receipt.LaunchControl.Owner.Lane != receipt.Owner.Lane ||
			receipt.LaunchControl.Owner.CurrentExecutor != receipt.Owner.CurrentExecutor ||
			receipt.LaunchControl.Owner.ExecutorGeneration != receipt.Owner.ExecutorGeneration ||
			receipt.LaunchControl.Capability != receipt.Capability {
			return fmt.Errorf("adapter execution dispatch launch control does not match owner and capability lineage")
		}
	}
	bindingSHA, err := DispatchBindingSHA256(receipt)
	if err != nil || !strings.EqualFold(bindingSHA, receipt.DispatchID) {
		return fmt.Errorf("adapter execution dispatch receipt id does not match semantic binding")
	}
	candidateSHA, err := CandidateSHA256(receipt.Adapter.Candidate)
	if err != nil || !strings.EqualFold(candidateSHA, receipt.Adapter.CandidateSnapshotSHA256) {
		return fmt.Errorf("adapter execution dispatch receipt candidate snapshot hash mismatch")
	}
	gateSHA, err := GateSHA256(receipt.Gate)
	if err != nil || !strings.EqualFold(gateSHA, receipt.Gate.SnapshotSHA256) {
		return fmt.Errorf("adapter execution dispatch receipt gate snapshot hash mismatch")
	}
	return nil
}

func Validate(receipt Receipt) error {
	if receipt.SchemaVersion != 1 || receipt.Kind != "adapter-execution-receipt" {
		return fmt.Errorf("adapter execution receipt schema/kind is invalid")
	}
	if !validSHA256(receipt.ReceiptID) || receipt.Gate.GateEventID == "" || receipt.Gate.Lane == "" || receipt.Gate.Action == "" || receipt.Gate.Authorization.Decision != autonomy.DecisionPreauthorized || !validStateRootRelativePath(receipt.Gate.Authorization.ProfilePath) || !validSHA256(receipt.Gate.SnapshotSHA256) {
		return fmt.Errorf("adapter execution receipt gate binding is invalid")
	}
	if !validSHA256(receipt.Dispatch.DispatchID) || !validStateRootRelativePath(receipt.Dispatch.Path) || stateRootRelativePath(receipt.Dispatch.Path) != stateRootRelativePath(receipt.Gate.Authorization.ProfilePath) || !validSHA256(receipt.Dispatch.SHA256) || receipt.Dispatch.Bytes <= 0 {
		return fmt.Errorf("adapter execution receipt dispatch binding is invalid")
	}
	if receipt.Adapter.Pack == "" || receipt.Adapter.AdapterID == "" || receipt.Adapter.ToolingCatalogPath == "" || !validSHA256(receipt.Adapter.ToolingCatalogSHA256) || receipt.Adapter.ToolingCatalogBytes < 0 || receipt.Adapter.Candidate.ID == "" || !validSHA256(receipt.Adapter.CandidateSnapshotSHA256) {
		return fmt.Errorf("adapter execution receipt adapter binding is invalid")
	}
	if receipt.Owner.Lane != receipt.Gate.Lane || receipt.Owner.CurrentExecutor == "" || receipt.Owner.ExecutorGeneration <= 0 || receipt.Owner.AdapterHarness == "" || receipt.Owner.AdapterSession == "" || receipt.Owner.BindingMode != "durable-lane-owner" {
		return fmt.Errorf("adapter execution receipt owner binding is invalid")
	}
	if !validOutcome(receipt.Execution.Outcome) || strings.TrimSpace(receipt.Execution.ExitStatus) == "" || len(receipt.Execution.ExitStatus) > 256 || receipt.Execution.AuthorizedBudget != receipt.Gate.AuthorizedBudget || receipt.Execution.ActualBudget.RuntimeSeconds < 0 || receipt.Execution.ActualBudget.DiskMB < 0 || receipt.Execution.ActualBudget.Requests < 0 {
		return fmt.Errorf("adapter execution receipt execution binding is invalid")
	}
	if !validRelativePath(receipt.Report.Path) || !validSHA256(receipt.Report.SHA256) || receipt.Report.Bytes < 0 {
		return fmt.Errorf("adapter execution receipt report binding is invalid")
	}
	seen := map[string]bool{}
	for _, artifact := range receipt.Artifacts {
		if !validRelativePath(artifact.Path) || artifact.Path == receipt.Report.Path || seen[artifact.Path] || !validSHA256(artifact.SHA256) || artifact.Bytes < 0 || len(artifact.Roles) == 0 {
			return fmt.Errorf("adapter execution receipt artifact binding is invalid")
		}
		seen[artifact.Path] = true
		for _, role := range artifact.Roles {
			if role != "output" && role != "evidence" {
				return fmt.Errorf("adapter execution receipt artifact role is invalid: %s", role)
			}
		}
	}
	if strings.TrimSpace(receipt.Actor) == "" || strings.TrimSpace(receipt.RecordedAt) == "" || !receipt.NoExecute || !receipt.NoAuthority {
		return fmt.Errorf("adapter execution receipt observation boundary is invalid")
	}
	if err := capabilitycontract.RequireBindingPolicy(receipt.Capability, capabilitycontract.PolicyClassAuthorizedHeavy); err != nil {
		return fmt.Errorf("adapter execution receipt capability contract is invalid: %w", err)
	}
	bindingSHA, err := BindingSHA256(receipt)
	if err != nil || !strings.EqualFold(bindingSHA, receipt.ReceiptID) {
		return fmt.Errorf("adapter execution receipt id does not match semantic binding")
	}
	candidateSHA, err := CandidateSHA256(receipt.Adapter.Candidate)
	if err != nil || !strings.EqualFold(candidateSHA, receipt.Adapter.CandidateSnapshotSHA256) {
		return fmt.Errorf("adapter execution receipt candidate snapshot hash mismatch")
	}
	gateSHA, err := GateSHA256(receipt.Gate)
	if err != nil || !strings.EqualFold(gateSHA, receipt.Gate.SnapshotSHA256) {
		return fmt.Errorf("adapter execution receipt gate snapshot hash mismatch")
	}
	return nil
}

func SemanticEqual(left, right Receipt) bool {
	left.RecordedAt = ""
	right.RecordedAt = ""
	leftBytes, _ := json.Marshal(left)
	rightBytes, _ := json.Marshal(right)
	return bytes.Equal(leftBytes, rightBytes)
}

func DispatchSemanticEqual(left, right DispatchReceipt) bool {
	left.RecordedAt = ""
	right.RecordedAt = ""
	leftBytes, _ := json.Marshal(left)
	rightBytes, _ := json.Marshal(right)
	return bytes.Equal(leftBytes, rightBytes)
}

func ValidateCompletionDispatchLineage(receipt Receipt, dispatch DispatchReceipt, dispatchPath, dispatchSHA256 string, dispatchBytes int64) error {
	validationReceipt := receipt
	if strings.TrimSpace(validationReceipt.RecordedAt) == "" {
		validationReceipt.RecordedAt = "preview"
	}
	if err := Validate(validationReceipt); err != nil {
		return err
	}
	if err := ValidateDispatch(dispatch); err != nil {
		return err
	}
	if receipt.Capability != dispatch.Capability {
		return fmt.Errorf("adapter execution receipt capability contract does not match dispatch lineage")
	}
	if !validStateRootRelativePath(dispatchPath) || receipt.Dispatch.DispatchID != dispatch.DispatchID || receipt.Dispatch.Path != dispatchPath || !strings.EqualFold(receipt.Dispatch.SHA256, dispatchSHA256) || receipt.Dispatch.Bytes != dispatchBytes {
		return fmt.Errorf("adapter execution receipt dispatch path/hash binding mismatch")
	}
	receiptGate, _ := json.Marshal(receipt.Gate)
	dispatchGate, _ := json.Marshal(dispatch.Gate)
	if !bytes.Equal(receiptGate, dispatchGate) || receipt.Adapter.Pack != dispatch.Adapter.Pack || receipt.Adapter.AdapterID != dispatch.Adapter.AdapterID || receipt.Adapter.ToolingCatalogPath != dispatch.Adapter.ToolingCatalogPath || !strings.EqualFold(receipt.Adapter.ToolingCatalogSHA256, dispatch.Adapter.ToolingCatalogSHA256) || receipt.Adapter.ToolingCatalogBytes != dispatch.Adapter.ToolingCatalogBytes || !strings.EqualFold(receipt.Adapter.CandidateSnapshotSHA256, dispatch.Adapter.CandidateSnapshotSHA256) || receipt.Owner != dispatch.Owner {
		return fmt.Errorf("adapter execution receipt does not match dispatch gate/adapter/owner bindings")
	}
	return nil
}

func SHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validSHA256(value string) bool {
	data, err := hex.DecodeString(strings.TrimSpace(value))
	return err == nil && len(data) == sha256.Size
}

func validRelativePath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, `\`) || filepath.IsAbs(filepath.FromSlash(value)) || looksLikeWindowsAbsolutePath(value) {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	return clean == value && clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}

func validStateRootRelativePath(value string) bool {
	return stateRootRelativePath(value) != ""
}

func stateRootRelativePath(value string) string {
	if !validRelativePath(value) {
		return ""
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	for _, root := range []string{".steamai", ".rekit"} {
		if strings.HasPrefix(clean, root+"/") {
			return root
		}
	}
	return ""
}

func looksLikeWindowsAbsolutePath(value string) bool {
	return len(value) >= 3 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':' && value[2] == '/'
}

func validOutcome(value string) bool {
	switch value {
	case "succeeded", "failed", "boundary-hit", "escalated", "aborted":
		return true
	default:
		return false
	}
}
