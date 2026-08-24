package adapterhost

import (
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/capabilitycontract"
)

func TestAuthorizedAdapterCapabilityRequiresAuthorizedHeavyPolicy(t *testing.T) {
	authorized, err := capabilitycontract.Bind(capabilitycontract.AuthorizedHeavy())
	if err != nil {
		t.Fatal(err)
	}
	if err := validateAuthorizedAdapterCapability(adapterexecution.DispatchReceipt{Capability: authorized}); err != nil {
		t.Fatal(err)
	}

	for _, contract := range []capabilitycontract.Contract{
		capabilitycontract.ReadOnly(),
		capabilitycontract.Transport(),
	} {
		binding, err := capabilitycontract.Bind(contract)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateAuthorizedAdapterCapability(adapterexecution.DispatchReceipt{Capability: binding}); err == nil || !strings.Contains(err.Error(), "capability") {
			t.Fatalf("authorized adapter accepted policy %q: %v", contract.PolicyClass, err)
		}
	}
}
