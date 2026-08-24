package capabilitycontract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	LegacySchemaVersion  = 1
	CurrentSchemaVersion = 2
	// SchemaVersion is retained as the current producer constant.
	SchemaVersion = CurrentSchemaVersion

	PolicyClassReadOnly        = "read-only"
	PolicyClassTransport       = "transport"
	PolicyClassAuthorizedHeavy = "authorized-heavy"

	// PolicyClassNonAuthorizing is retained only for historical v1 decoding.
	PolicyClassNonAuthorizing = "non-authorizing"
)

const currentHashDomain = "re-context-kits/capability-contract/v2\n"

// Contract is the current shared machine boundary carried across durable
// control-plane envelopes. It never grants authority or confirmed.
type Contract struct {
	SchemaVersion    int    `json:"schemaVersion"`
	PolicyClass      string `json:"policyClass"`
	NoAuthority      bool   `json:"noAuthority"`
	NoConfirmed      bool   `json:"noConfirmed"`
	NoAuthorizedGate bool   `json:"noAuthorizedGate"`
	NoHeavyTool      bool   `json:"noHeavyTool"`
	AuthorizedGate   bool   `json:"authorizedGate"`
	HeavyTool        bool   `json:"heavyTool"`
}

// LegacyContract is the v1 shape used only by historical inspection and exact
// replay readers. It must not be converted into a current producer contract.
type LegacyContract struct {
	SchemaVersion    int    `json:"schemaVersion"`
	PolicyClass      string `json:"policyClass"`
	NoAuthority      bool   `json:"noAuthority"`
	NoConfirmed      bool   `json:"noConfirmed"`
	NoAuthorizedGate bool   `json:"noAuthorizedGate"`
	NoHeavyTool      bool   `json:"noHeavyTool"`
	AuthorizedGate   bool   `json:"authorizedGate"`
	HeavyTool        bool   `json:"heavyTool"`
}

// Binding carries the current contract and its exact content identity.
type Binding struct {
	Contract Contract `json:"contract"`
	SHA256   string   `json:"sha256"`
}

// LegacyBinding carries a historical v1 contract without normalizing it.
type LegacyBinding struct {
	Contract LegacyContract `json:"contract"`
	SHA256   string         `json:"sha256"`
}

type DecodedContract struct {
	Version int
	Current *Contract
	Legacy  *LegacyContract
}

type DecodedBinding struct {
	Version int
	Current *Binding
	Legacy  *LegacyBinding
}

func ReadOnly() Contract {
	return denyAll(PolicyClassReadOnly)
}

func Transport() Contract {
	return denyAll(PolicyClassTransport)
}

// AuthorizedHeavy describes only a sink that has already consumed an
// independently validated strict profile and exact authorized-gate.
func AuthorizedHeavy() Contract {
	return Contract{
		SchemaVersion:    CurrentSchemaVersion,
		PolicyClass:      PolicyClassAuthorizedHeavy,
		NoAuthority:      true,
		NoConfirmed:      true,
		NoAuthorizedGate: false,
		NoHeavyTool:      false,
		AuthorizedGate:   true,
		HeavyTool:        true,
	}
}

// NonAuthorizing remains source-compatible for callers that need to reject a
// historical policy. It is not a valid current contract and cannot be bound.
func NonAuthorizing() Contract {
	return denyAll(PolicyClassNonAuthorizing)
}

func denyAll(policyClass string) Contract {
	return Contract{
		SchemaVersion:    CurrentSchemaVersion,
		PolicyClass:      policyClass,
		NoAuthority:      true,
		NoConfirmed:      true,
		NoAuthorizedGate: true,
		NoHeavyTool:      true,
		AuthorizedGate:   false,
		HeavyTool:        false,
	}
}

func Validate(contract Contract) error {
	if contract.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("capability contract schema version is unsupported: %d", contract.SchemaVersion)
	}
	if err := validateCommon(contract.PolicyClass, contract.NoAuthority, contract.NoConfirmed); err != nil {
		return err
	}
	switch strings.TrimSpace(contract.PolicyClass) {
	case PolicyClassReadOnly, PolicyClassTransport:
		if !contract.NoAuthorizedGate || contract.AuthorizedGate {
			return fmt.Errorf("%s capability contract must deny an authorized gate", contract.PolicyClass)
		}
		if !contract.NoHeavyTool || contract.HeavyTool {
			return fmt.Errorf("%s capability contract must deny heavy-tool execution", contract.PolicyClass)
		}
	case PolicyClassAuthorizedHeavy:
		if contract.NoAuthorizedGate || !contract.AuthorizedGate {
			return fmt.Errorf("authorized-heavy capability contract requires an independently validated authorized gate")
		}
		if contract.NoHeavyTool || !contract.HeavyTool {
			return fmt.Errorf("authorized-heavy capability contract must permit only the independently authorized heavy-tool sink")
		}
	case PolicyClassNonAuthorizing:
		return fmt.Errorf("capability contract policy class is decode-only: %q", contract.PolicyClass)
	default:
		return fmt.Errorf("capability contract policy class is unsupported: %q", contract.PolicyClass)
	}
	return nil
}

func ValidateLegacy(contract LegacyContract) error {
	if contract.SchemaVersion != LegacySchemaVersion {
		return fmt.Errorf("legacy capability contract schema version is unsupported: %d", contract.SchemaVersion)
	}
	if err := validateCommon(contract.PolicyClass, contract.NoAuthority, contract.NoConfirmed); err != nil {
		return err
	}
	switch strings.TrimSpace(contract.PolicyClass) {
	case PolicyClassReadOnly, PolicyClassTransport, PolicyClassNonAuthorizing:
		if !contract.NoAuthorizedGate || contract.AuthorizedGate {
			return fmt.Errorf("legacy %s capability contract must deny an authorized gate", contract.PolicyClass)
		}
		if !contract.NoHeavyTool || contract.HeavyTool {
			return fmt.Errorf("legacy %s capability contract must deny heavy-tool execution", contract.PolicyClass)
		}
	case PolicyClassAuthorizedHeavy:
		if contract.NoAuthorizedGate || !contract.AuthorizedGate {
			return fmt.Errorf("legacy authorized-heavy capability contract requires an authorized gate")
		}
		if contract.NoHeavyTool || !contract.HeavyTool {
			return fmt.Errorf("legacy authorized-heavy capability contract must permit the heavy-tool sink")
		}
	default:
		return fmt.Errorf("legacy capability contract policy class is unsupported: %q", contract.PolicyClass)
	}
	return nil
}

func validateCommon(policyClass string, noAuthority, noConfirmed bool) error {
	if !noAuthority {
		return fmt.Errorf("capability contract must not grant authority")
	}
	if !noConfirmed {
		return fmt.Errorf("capability contract must not grant confirmed")
	}
	if strings.TrimSpace(policyClass) == "" {
		return fmt.Errorf("capability contract policy class is required")
	}
	return nil
}

func RequirePolicy(contract Contract, policyClass string) error {
	if err := Validate(contract); err != nil {
		return err
	}
	if contract.PolicyClass != policyClass {
		return fmt.Errorf("capability contract policy class mismatch: got %q want %q", contract.PolicyClass, policyClass)
	}
	return nil
}

func CanonicalBytes(contract Contract) ([]byte, error) {
	if err := Validate(contract); err != nil {
		return nil, err
	}
	data, err := json.Marshal(contract)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func LegacyCanonicalBytes(contract LegacyContract) ([]byte, error) {
	if err := ValidateLegacy(contract); err != nil {
		return nil, err
	}
	data, err := json.Marshal(contract)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func SHA256(contract Contract) (string, error) {
	data, err := CanonicalBytes(contract)
	if err != nil {
		return "", err
	}
	hashInput := append([]byte(currentHashDomain), data...)
	sum := sha256.Sum256(hashInput)
	return hex.EncodeToString(sum[:]), nil
}

func LegacySHA256(contract LegacyContract) (string, error) {
	data, err := LegacyCanonicalBytes(contract)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func Bind(contract Contract) (Binding, error) {
	hash, err := SHA256(contract)
	if err != nil {
		return Binding{}, err
	}
	return Binding{Contract: contract, SHA256: hash}, nil
}

func BindLegacy(contract LegacyContract) (LegacyBinding, error) {
	hash, err := LegacySHA256(contract)
	if err != nil {
		return LegacyBinding{}, err
	}
	return LegacyBinding{Contract: contract, SHA256: hash}, nil
}

func ValidateBinding(binding Binding) error {
	if strings.TrimSpace(binding.SHA256) == "" {
		return fmt.Errorf("capability contract binding sha256 is required")
	}
	return ValidateHash(binding.Contract, binding.SHA256)
}

func ValidateLegacyBinding(binding LegacyBinding) error {
	if strings.TrimSpace(binding.SHA256) == "" {
		return fmt.Errorf("legacy capability contract binding sha256 is required")
	}
	return ValidateLegacyHash(binding.Contract, binding.SHA256)
}

func RequireBindingPolicy(binding Binding, policyClass string) error {
	if err := ValidateBinding(binding); err != nil {
		return err
	}
	return RequirePolicy(binding.Contract, policyClass)
}

func Decode(data []byte) (Contract, error) {
	var contract Contract
	if err := decodeSingle(data, &contract, "capability contract"); err != nil {
		return Contract{}, err
	}
	if err := Validate(contract); err != nil {
		return Contract{}, err
	}
	return contract, nil
}

func DecodeLegacy(data []byte) (LegacyContract, error) {
	var contract LegacyContract
	if err := decodeSingle(data, &contract, "legacy capability contract"); err != nil {
		return LegacyContract{}, err
	}
	if err := ValidateLegacy(contract); err != nil {
		return LegacyContract{}, err
	}
	return contract, nil
}

func DecodeBinding(data []byte) (Binding, error) {
	var binding Binding
	if err := decodeSingle(data, &binding, "capability contract binding"); err != nil {
		return Binding{}, err
	}
	if err := ValidateBinding(binding); err != nil {
		return Binding{}, err
	}
	return binding, nil
}

func DecodeLegacyBinding(data []byte) (LegacyBinding, error) {
	var binding LegacyBinding
	if err := decodeSingle(data, &binding, "legacy capability contract binding"); err != nil {
		return LegacyBinding{}, err
	}
	if err := ValidateLegacyBinding(binding); err != nil {
		return LegacyBinding{}, err
	}
	return binding, nil
}

func DecodeVersioned(data []byte) (DecodedContract, error) {
	version, err := probeVersion(data, "capability contract")
	if err != nil {
		return DecodedContract{}, err
	}
	switch version {
	case CurrentSchemaVersion:
		value, err := Decode(data)
		if err != nil {
			return DecodedContract{}, err
		}
		return DecodedContract{Version: version, Current: &value}, nil
	case LegacySchemaVersion:
		value, err := DecodeLegacy(data)
		if err != nil {
			return DecodedContract{}, err
		}
		return DecodedContract{Version: version, Legacy: &value}, nil
	default:
		return DecodedContract{}, fmt.Errorf("capability contract schema version is unsupported: %d", version)
	}
}

func DecodeVersionedBinding(data []byte) (DecodedBinding, error) {
	version, err := probeBindingVersion(data)
	if err != nil {
		return DecodedBinding{}, err
	}
	switch version {
	case CurrentSchemaVersion:
		value, err := DecodeBinding(data)
		if err != nil {
			return DecodedBinding{}, err
		}
		return DecodedBinding{Version: version, Current: &value}, nil
	case LegacySchemaVersion:
		value, err := DecodeLegacyBinding(data)
		if err != nil {
			return DecodedBinding{}, err
		}
		return DecodedBinding{Version: version, Legacy: &value}, nil
	default:
		return DecodedBinding{}, fmt.Errorf("capability contract binding schema version is unsupported: %d", version)
	}
}

func ValidateHash(contract Contract, expected string) error {
	actual, err := SHA256(contract)
	if err != nil {
		return err
	}
	return compareHash(actual, expected, "capability contract")
}

func ValidateLegacyHash(contract LegacyContract, expected string) error {
	actual, err := LegacySHA256(contract)
	if err != nil {
		return err
	}
	return compareHash(actual, expected, "legacy capability contract")
}

func compareHash(actual, expected, label string) error {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if len(expected) != sha256.Size*2 || expected != actual {
		return fmt.Errorf("%s sha256 mismatch", label)
	}
	if _, err := hex.DecodeString(expected); err != nil {
		return fmt.Errorf("%s sha256 is invalid: %w", label, err)
	}
	return nil
}

func decodeSingle(data []byte, target any, label string) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", label, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%s must contain exactly one JSON object", label)
	}
	return nil
}

func probeBindingVersion(data []byte) (int, error) {
	var envelope map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&envelope); err != nil {
		return 0, fmt.Errorf("decode capability contract binding: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return 0, fmt.Errorf("capability contract binding must contain exactly one JSON object")
	}
	contractData, ok := envelope["contract"]
	if !ok {
		return 0, fmt.Errorf("capability contract binding contract is required")
	}
	return probeVersion(contractData, "capability contract binding contract")
}

func probeVersion(data []byte, label string) (int, error) {
	var envelope map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&envelope); err != nil {
		return 0, fmt.Errorf("decode %s: %w", label, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return 0, fmt.Errorf("%s must contain exactly one JSON object", label)
	}
	raw, ok := envelope["schemaVersion"]
	if !ok {
		return 0, fmt.Errorf("%s schemaVersion is required", label)
	}
	var version int
	if err := json.Unmarshal(raw, &version); err != nil {
		return 0, fmt.Errorf("%s schemaVersion is invalid: %w", label, err)
	}
	return version, nil
}
