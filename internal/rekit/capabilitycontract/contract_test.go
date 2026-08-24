package capabilitycontract

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestPolicyContractsAreStrictAndHashStable(t *testing.T) {
	for _, contract := range []Contract{ReadOnly(), Transport(), AuthorizedHeavy()} {
		t.Run(contract.PolicyClass, func(t *testing.T) {
			if err := Validate(contract); err != nil {
				t.Fatal(err)
			}
			first, err := CanonicalBytes(contract)
			if err != nil {
				t.Fatal(err)
			}
			second, err := CanonicalBytes(contract)
			if err != nil || !bytes.Equal(first, second) {
				t.Fatalf("canonical bytes are not stable: %q %q err=%v", first, second, err)
			}
			binding, err := Bind(contract)
			if err != nil || len(binding.SHA256) != 64 {
				t.Fatalf("contract binding=%+v err=%v", binding, err)
			}
			if err := ValidateHash(contract, strings.ToUpper(binding.SHA256)); err != nil {
				t.Fatal(err)
			}
			if err := ValidateBinding(binding); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestValidateKeepsCapabilityDenialsIndependent(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*Contract)
		want   string
	}{
		{"authority", func(contract *Contract) { contract.NoAuthority = false }, "authority"},
		{"confirmed", func(contract *Contract) { contract.NoConfirmed = false }, "confirmed"},
		{"authorized gate denial", func(contract *Contract) { contract.NoAuthorizedGate = false }, "authorized gate"},
		{"authorized gate grant", func(contract *Contract) { contract.AuthorizedGate = true }, "authorized gate"},
		{"heavy tool denial", func(contract *Contract) { contract.NoHeavyTool = false }, "deny heavy-tool"},
		{"heavy tool grant", func(contract *Contract) { contract.HeavyTool = true }, "deny heavy-tool"},
	} {
		t.Run(test.name, func(t *testing.T) {
			contract := Transport()
			test.mutate(&contract)
			if err := Validate(contract); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("independent capability grant accepted: %+v err=%v", contract, err)
			}
		})
	}
	heavy := AuthorizedHeavy()
	heavy.NoHeavyTool = true
	if err := Validate(heavy); err == nil || !strings.Contains(err.Error(), "authorized-heavy") {
		t.Fatalf("authorized-heavy denial drift accepted: %v", err)
	}
	heavy = AuthorizedHeavy()
	heavy.AuthorizedGate = false
	if err := Validate(heavy); err == nil || !strings.Contains(err.Error(), "authorized gate") {
		t.Fatalf("authorized-heavy gate drift accepted: %v", err)
	}
}

func TestBindingRejectsPolicyAndHashDrift(t *testing.T) {
	binding, err := Bind(ReadOnly())
	if err != nil {
		t.Fatal(err)
	}
	if err := RequireBindingPolicy(binding, PolicyClassReadOnly); err != nil {
		t.Fatal(err)
	}
	if err := RequireBindingPolicy(binding, PolicyClassTransport); err == nil || !strings.Contains(err.Error(), "policy class mismatch") {
		t.Fatalf("cross-policy binding accepted: %v", err)
	}
	tampered := binding
	tampered.Contract = Transport()
	if err := ValidateBinding(tampered); err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("cross-envelope contract drift accepted: %v", err)
	}
	data, err := json.Marshal(binding)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeBinding(data); err != nil {
		t.Fatal(err)
	}
	unknown := append(append([]byte{}, data[:len(data)-1]...), []byte(`,"unknown":true}`)...)
	if _, err := DecodeBinding(unknown); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown binding field accepted: %v", err)
	}
}

func TestLegacyCapabilityIsDecodeOnlyAndVersionDispatched(t *testing.T) {
	legacy := LegacyContract{
		SchemaVersion: LegacySchemaVersion, PolicyClass: PolicyClassNonAuthorizing,
		NoAuthority: true, NoConfirmed: true, NoAuthorizedGate: true, NoHeavyTool: true,
	}
	binding, err := BindLegacy(legacy)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(binding)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeVersionedBinding(data)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Version != LegacySchemaVersion || decoded.Current != nil || decoded.Legacy == nil || decoded.Legacy.Contract.PolicyClass != PolicyClassNonAuthorizing {
		t.Fatalf("legacy binding was not kept decode-only: %+v", decoded)
	}
	if _, err := DecodeBinding(data); err == nil {
		t.Fatal("current binding decoder accepted legacy binding")
	}
	current, err := Bind(Transport())
	if err != nil {
		t.Fatal(err)
	}
	currentData, err := json.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	currentDecoded, err := DecodeVersionedBinding(currentData)
	if err != nil || currentDecoded.Version != CurrentSchemaVersion || currentDecoded.Current == nil || currentDecoded.Legacy != nil {
		t.Fatalf("current binding was not dispatched as v2: %+v err=%v", currentDecoded, err)
	}
	if current.SHA256 == binding.SHA256 {
		t.Fatal("current and legacy capability hashes unexpectedly share identity")
	}
}

func TestDecodeRejectsUnknownFieldsTrailingDataAndUnsupportedPolicy(t *testing.T) {
	data, err := json.Marshal(ReadOnly())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(data); err != nil {
		t.Fatal(err)
	}
	unknown := append(append([]byte{}, data[:len(data)-1]...), []byte(`,"unknown":true}`)...)
	if _, err := Decode(unknown); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown field accepted: %v", err)
	}
	if _, err := Decode(append(append([]byte{}, data...), []byte(` {}`)...)); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("trailing object accepted: %v", err)
	}
	unsupported := ReadOnly()
	unsupported.PolicyClass = "provider-acknowledged"
	if err := Validate(unsupported); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("provider acknowledgement became a capability class: %v", err)
	}
}
