package commands

import (
	"slices"
	"strings"
	"testing"
)

func TestScopedCommandDescriptorsComposeExistingCatalogs(t *testing.T) {
	descriptors, err := ScopedCommandDescriptors()
	if err != nil {
		t.Fatal(err)
	}
	if len(descriptors) != len(MutationContracts())+5 {
		t.Fatalf("descriptor count=%d, want mutation contracts plus read-only commands", len(descriptors))
	}
	if !slices.IsSortedFunc(descriptors, func(left, right ScopedCommandDescriptor) int {
		if left.Scope.Command == right.Scope.Command {
			return strings.Compare(left.Scope.Mode, right.Scope.Mode)
		}
		return strings.Compare(left.Scope.Command, right.Scope.Command)
	}) {
		t.Fatalf("scoped descriptors are not sorted: %+v", descriptors)
	}
	if err := ValidateScopedCommandDescriptors(descriptors); err != nil {
		t.Fatal(err)
	}

	status, ok := ScopedCommandDescriptorFor(Status, "")
	if !ok || status.Scope.Mode != MutationModeDefault || status.Mutation != nil || status.Profile.MutationBoundary != BoundaryReadOnly {
		t.Fatalf("read-only status descriptor drifted: %+v ok=%t", status, ok)
	}
	continueDefault, ok := ScopedCommandDescriptorFor(Continue, "")
	if !ok || continueDefault.Mutation == nil || continueDefault.Mutation.Currentness != MutationCurrentnessStrictPlan || continueDefault.Mutation.ExpectedFlag != "-ExpectedContinuePlanSha256" {
		t.Fatalf("continue descriptor drifted: %+v ok=%t", continueDefault, ok)
	}
	gateProfile, ok := ScopedCommandDescriptorFor(Gate, MutationModeProfile)
	if !ok || gateProfile.Mutation == nil || gateProfile.Mutation.Currentness != MutationCurrentnessStrictPlan || gateProfile.Mutation.ExpectedFlag != "-ExpectedProfilePlanSha256" || gateProfile.Profile.MutationBoundary != BoundaryCaseLocalApply {
		t.Fatalf("gate profile descriptor drifted: %+v ok=%t", gateProfile, ok)
	}
	if _, ok := ScopedCommandDescriptorFor(Gate, ""); ok {
		t.Fatal("mixed-mode gate descriptor must require an explicit mode")
	}
	if got := len(ScopedCommandDescriptorsFor(Gate)); got != 7 {
		t.Fatalf("gate descriptor modes=%d, want 7", got)
	}
}

func TestScopedCommandDescriptorValidationRejectsCatalogDrift(t *testing.T) {
	descriptors, err := ScopedCommandDescriptors()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		mutate func([]ScopedCommandDescriptor) []ScopedCommandDescriptor
		want   string
	}{
		{
			name: "missing scope",
			mutate: func(items []ScopedCommandDescriptor) []ScopedCommandDescriptor {
				return items[1:]
			},
			want: "no scoped descriptor",
		},
		{
			name: "duplicate scope",
			mutate: func(items []ScopedCommandDescriptor) []ScopedCommandDescriptor {
				return append(items, cloneScopedCommandDescriptor(items[0]))
			},
			want: "duplicate scoped command descriptor",
		},
		{
			name: "profile drift",
			mutate: func(items []ScopedCommandDescriptor) []ScopedCommandDescriptor {
				items[0].Profile.WritesKit = !items[0].Profile.WritesKit
				return items
			},
			want: "profile drifted",
		},
		{
			name: "mutation drift",
			mutate: func(items []ScopedCommandDescriptor) []ScopedCommandDescriptor {
				for index := range items {
					if items[index].Mutation != nil {
						items[index].Mutation.Mode = "changed"
						break
					}
				}
				return items
			},
			want: "mutation policy drifted",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			items := cloneScopedCommandDescriptors(descriptors)
			err := ValidateScopedCommandDescriptors(test.mutate(items))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validation error=%v, want containing %q", err, test.want)
			}
		})
	}
}

func TestScopedCommandDescriptorsReturnDefensiveCopies(t *testing.T) {
	descriptor, ok := ScopedCommandDescriptorFor(Continue, MutationModeDefault)
	if !ok || descriptor.Mutation == nil {
		t.Fatal("continue descriptor missing")
	}
	descriptor.Mutation.ExpectedAliases[0] = "changed"
	fresh, ok := ScopedCommandDescriptorFor(Continue, MutationModeDefault)
	if !ok || fresh.Mutation == nil || fresh.Mutation.ExpectedAliases[0] != "-ExpectedContinuePlanSha256" {
		t.Fatalf("descriptor catalog was mutated through a returned value: %+v", fresh)
	}
}
