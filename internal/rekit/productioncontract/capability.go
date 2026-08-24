package productioncontract

import (
	"fmt"
	"sort"

	"github.com/shuiyu486/re-context-kits/internal/rekit/capabilitycontract"
)

type CapabilityAdmission struct {
	Ready         bool     `json:"ready"`
	Warnings      []string `json:"warnings"`
	PolicyClasses []string `json:"policyClasses"`
	Contract      string   `json:"contract"`
	Sinks         string   `json:"sinks"`
	Evidence      string   `json:"evidence"`
}

var sharedCapabilityContract = SourceContract{
	Identity: "shared hashed capability contract and policy validators",
	Bindings: []GoSourceBinding{
		{
			Path: "internal/rekit/capabilitycontract/contract.go",
			Symbols: []string{
				"ReadOnly", "Transport", "AuthorizedHeavy", "NonAuthorizing",
				"Bind", "RequireBindingPolicy",
			},
		},
	},
}

var sharedCapabilitySinks = SourceContract{
	Identity: "durable read-only, transport, and authorized-heavy capability lineage validators",
	Bindings: []GoSourceBinding{
		{Path: "internal/rekit/reviewersession/receipt.go", Symbols: []string{"DecodeDispatch", "ValidateCompletionDispatchLineage"}},
		{Path: "internal/rekit/externalsession/relay.go", Symbols: []string{"validateJobCapability", "validateSubmission"}},
		{Path: "internal/rekit/externalsession/dispatch.go", Symbols: []string{"validateDispatchLaunchCapability"}},
		{Path: "internal/rekit/externalsession/transport.go", Symbols: []string{"validateTransportEndpoint", "validateTransportDelivery"}},
		{Path: "internal/rekit/externalsession/transport_return.go", Symbols: []string{"validateRemoteControlReturnLineage"}},
		{Path: "internal/rekit/sessionhost/claude.go", Symbols: []string{"validateClaudeCapabilityBinding"}},
		{Path: "internal/rekit/adapterexecution/receipt.go", Symbols: []string{"ValidateDispatch", "ValidateCompletionDispatchLineage"}},
		{Path: "internal/rekit/adapterhost/authorized_run.go", Symbols: []string{"validateAuthorizedAdapterCapability"}},
		{Path: "internal/rekit/executioncontrol/result.go", Symbols: []string{"validateHeldResultReceipt"}},
	},
}

var sharedCapabilityEvidence = SourceContract{
	Identity: "reverse-authorization and exact-lineage capability regression evidence",
	Bindings: []GoSourceBinding{
		{Path: "internal/rekit/capabilitycontract/contract_test.go", Symbols: []string{"TestPolicyContractsAreStrictAndHashStable", "TestBindingRejectsPolicyAndHashDrift"}},
		{Path: "internal/rekit/reviewersession/receipt_test.go", Symbols: []string{"TestDecodeDispatchAndCompletionRejectCapabilityGrant", "TestValidateCompletionDispatchLineageRejectsDrift"}},
		{Path: "internal/rekit/externalsession/dispatch_test.go", Symbols: []string{"TestBindAttemptDispatchRejectsCapabilityPolicyDrift"}},
		{Path: "internal/rekit/externalsession/relay_test.go", Symbols: []string{"TestExternalSessionSubmissionRejectsCapabilityDrift"}},
		{Path: "internal/rekit/externalsession/transport_test.go", Symbols: []string{"TestRemoteControlTransportProviderAcknowledgementDoesNotChangeCapability", "TestRemoteControlTransportRejectsCapabilityHashAndPolicyDrift"}},
		{Path: "internal/rekit/sessionhost/claude_test.go", Symbols: []string{"TestClaudeLaunchCapabilityPolicySeparatesMemberAndReadOnlyReview"}},
		{Path: "internal/rekit/adapterexecution/receipt_test.go", Symbols: []string{"TestDispatchReceiptDecodeStrictAndCompletionLineage"}},
		{Path: "internal/rekit/adapterhost/authorized_run_capability_test.go", Symbols: []string{"TestAuthorizedAdapterCapabilityRequiresAuthorizedHeavyPolicy"}},
		{Path: "internal/rekit/executioncontrol/result_test.go", Symbols: []string{"TestHeldResultRejectsCapabilityHashDrift"}},
	},
}

func BuildCapabilityAdmission(repoRoot string) CapabilityAdmission {
	admission := CapabilityAdmission{
		Ready:    true,
		Warnings: []string{},
		Contract: sharedCapabilityContract.Identity,
		Sinks:    sharedCapabilitySinks.Identity,
		Evidence: sharedCapabilityEvidence.Identity,
	}
	policies := []struct {
		name     string
		contract capabilitycontract.Contract
	}{
		{name: capabilitycontract.PolicyClassReadOnly, contract: capabilitycontract.ReadOnly()},
		{name: capabilitycontract.PolicyClassTransport, contract: capabilitycontract.Transport()},
		{name: capabilitycontract.PolicyClassAuthorizedHeavy, contract: capabilitycontract.AuthorizedHeavy()},
	}
	for _, policy := range policies {
		binding, err := capabilitycontract.Bind(policy.contract)
		if err != nil {
			admission.Warnings = append(admission.Warnings, fmt.Sprintf("capability policy %s cannot be bound: %v", policy.name, err))
			continue
		}
		if err := capabilitycontract.RequireBindingPolicy(binding, policy.name); err != nil {
			admission.Warnings = append(admission.Warnings, fmt.Sprintf("capability policy %s binding is invalid: %v", policy.name, err))
			continue
		}
		admission.PolicyClasses = append(admission.PolicyClasses, policy.name)
	}
	legacy := capabilitycontract.LegacyContract{
		SchemaVersion:    capabilitycontract.LegacySchemaVersion,
		PolicyClass:      capabilitycontract.PolicyClassNonAuthorizing,
		NoAuthority:      true,
		NoConfirmed:      true,
		NoAuthorizedGate: true,
		NoHeavyTool:      true,
	}
	if err := capabilitycontract.ValidateLegacy(legacy); err != nil {
		admission.Warnings = append(admission.Warnings, fmt.Sprintf("legacy capability policy %s is invalid: %v", legacy.PolicyClass, err))
	} else {
		admission.PolicyClasses = append(admission.PolicyClasses, legacy.PolicyClass)
	}
	sort.Strings(admission.PolicyClasses)
	admission.Warnings = append(admission.Warnings, validateGoSourceContract(repoRoot, "capability contract", sharedCapabilityContract)...)
	admission.Warnings = append(admission.Warnings, validateGoSourceContract(repoRoot, "capability sink", sharedCapabilitySinks)...)
	admission.Warnings = append(admission.Warnings, validateGoSourceContract(repoRoot, "capability evidence", sharedCapabilityEvidence)...)
	admission.Ready = len(admission.Warnings) == 0
	return admission
}
