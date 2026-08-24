package commands

import (
	"slices"
	"strings"
	"testing"
)

func TestStrictMutationContractsDriveExecutablePlanBindings(t *testing.T) {
	want := map[string]string{
		Bootstrap:    "-ExpectedInitPlanSha256",
		Complete:     "-ExpectedCompletePlanSha256",
		Continue:     "-ExpectedContinuePlanSha256",
		Control:      "-ExpectedControlPlanSha256",
		Handoff:      "-ExpectedHandoffPlanSha256",
		Init:         "-ExpectedInitPlanSha256",
		MigrateState: "-ExpectedMigrationPlanSha256",
		Onboard:      "-ExpectedOnboardingPlanSha256",
		Reopen:       "-ExpectedReopenPlanSha256",
	}
	contracts := MutationContracts()
	if !slices.IsSortedFunc(contracts, func(left, right MutationContract) int {
		if left.Command == right.Command {
			return strings.Compare(left.Mode, right.Mode)
		}
		return strings.Compare(left.Command, right.Command)
	}) {
		t.Fatalf("mutation contracts are not sorted: %+v", contracts)
	}
	for command, flag := range want {
		contract, ok := MutationContractFor(command, "")
		if !ok || contract.Currentness != MutationCurrentnessStrictPlan || contract.ExpectedFlag != flag || len(contract.ExpectedAliases) != 2 || contract.ExpectedAliases[0] != flag || !contract.ExecutablePlanValidation || len(contract.ExactCarriers) == 0 {
			t.Fatalf("strict mutation contract %s=%+v ok=%t", command, contract, ok)
		}
		canonical, aliases, ok := publicPlanBinding(command)
		if !ok || canonical != flag || !slices.Equal(aliases, contract.ExpectedAliases) {
			t.Fatalf("public plan binding drifted from contract for %s: %q %v %t", command, canonical, aliases, ok)
		}
	}
}

func TestMutationContractDefaultLookupFailsOnMixedBindings(t *testing.T) {
	if _, ok := MutationContractFor(Gate, ""); ok {
		t.Fatal("gate command-level binding must fail closed because its modes use different contracts")
	}
	if contract, ok := MutationContractFor(Onboard, ""); !ok || contract.ExpectedFlag != "-ExpectedOnboardingPlanSha256" || contract.Mode != MutationModeDefault {
		t.Fatalf("onboard shared strict binding lookup drifted: %+v ok=%t", contract, ok)
	}
}

func TestMutationContractCatalogIsWellFormed(t *testing.T) {
	currentness := map[string]bool{
		MutationCurrentnessStrictPlan:      true,
		MutationCurrentnessOuterPlan:       true,
		MutationCurrentnessResourceBinding: true,
		MutationCurrentnessFreshReplan:     true,
		MutationCurrentnessImplicit:        true,
	}
	carriers := map[string]bool{
		MutationCarrierTypedInvocation: true,
		MutationCarrierActionQueue:     true,
		MutationCarrierApplyCommand:    true,
		MutationCarrierApplyArgs:       true,
		MutationCarrierRecordCommand:   true,
		MutationCarrierRecordArgs:      true,
		MutationCarrierNone:            true,
	}
	seen := map[string]bool{}
	for _, contract := range MutationContracts() {
		key := contract.Command + "\x00" + contract.Mode
		if seen[key] {
			t.Fatalf("duplicate mutation contract for %s mode %s", contract.Command, contract.Mode)
		}
		seen[key] = true
		if contract.Command == "" || contract.Command != strings.ToLower(strings.TrimSpace(contract.Command)) {
			t.Fatalf("mutation contract has non-canonical command: %+v", contract)
		}
		if _, ok := PublicProfileMap()[contract.Command]; !ok {
			t.Fatalf("mutation contract command is not in the public catalog: %+v", contract)
		}
		if contract.Mode == "" || contract.Mode != strings.ToLower(strings.TrimSpace(contract.Mode)) {
			t.Fatalf("mutation contract has non-canonical mode: %+v", contract)
		}
		if !currentness[contract.Currentness] {
			t.Fatalf("mutation contract has unknown currentness: %+v", contract)
		}
		if len(contract.ExactCarriers) == 0 {
			t.Fatalf("mutation contract must state its exact carrier boundary: %+v", contract)
		}
		seenCarriers := map[string]bool{}
		for _, carrier := range contract.ExactCarriers {
			if !carriers[carrier] || seenCarriers[carrier] {
				t.Fatalf("mutation contract has invalid carrier %q: %+v", carrier, contract)
			}
			seenCarriers[carrier] = true
		}
		if seenCarriers[MutationCarrierNone] && len(contract.ExactCarriers) != 1 {
			t.Fatalf("none carrier cannot be combined with another carrier: %+v", contract)
		}

		hasBinding := contract.ExpectedFlag != "" || len(contract.ExpectedAliases) > 0
		switch contract.Currentness {
		case MutationCurrentnessStrictPlan, MutationCurrentnessOuterPlan, MutationCurrentnessResourceBinding:
			if contract.ExpectedFlag == "" || len(contract.ExpectedAliases) == 0 || contract.ExpectedAliases[0] != contract.ExpectedFlag {
				t.Fatalf("bound mutation contract has no canonical expected flag: %+v", contract)
			}
		case MutationCurrentnessFreshReplan, MutationCurrentnessImplicit:
			if hasBinding || len(contract.NestedExpectedFlags) > 0 {
				t.Fatalf("unbound mutation contract declares a plan binding: %+v", contract)
			}
		}
		if contract.ExecutablePlanValidation && contract.Currentness != MutationCurrentnessStrictPlan {
			t.Fatalf("generic executable validation is limited to strict-plan contracts: %+v", contract)
		}
	}
}

func TestMutationContractsReturnDefensiveCopies(t *testing.T) {
	contract, ok := MutationContractFor(Continue, MutationModeDefault)
	if !ok {
		t.Fatal("continue mutation contract missing")
	}
	contract.ExpectedAliases[0] = "changed"
	contract.ExactCarriers[0] = "changed"
	fresh, ok := MutationContractFor(Continue, "")
	if !ok || fresh.ExpectedAliases[0] != "-ExpectedContinuePlanSha256" || fresh.ExactCarriers[0] != MutationCarrierTypedInvocation {
		t.Fatalf("mutation contract catalog was mutated through a returned value: %+v", fresh)
	}
}
