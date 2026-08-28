package commands

import (
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/plancontract"
)

func TestPublicInvocationRoundTripAndCLIArgs(t *testing.T) {
	invocation, err := NewPublicInvocation(Continue, "feature-mission", "-Reason", `review "again"`, "-WhatIf", "-Format", "json")
	if err != nil {
		t.Fatal(err)
	}
	text, err := invocation.Render()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParsePublicInvocation(text)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Command != invocation.Command || !slices.Equal(parsed.Arguments, invocation.Arguments) {
		t.Fatalf("public invocation round trip mismatch: got=%+v want=%+v text=%q", parsed, invocation, text)
	}
	args, err := invocation.CLIArgs()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-Command", Continue, "feature-mission", "-Reason", `review "again"`, "-WhatIf", "-Format", "json"}
	if !slices.Equal(args, want) {
		t.Fatalf("CLIArgs=%v want=%v", args, want)
	}
	if !invocation.HasFlag("-whatif") || invocation.HasFlag("-Apply") {
		t.Fatalf("unexpected typed flags: %+v", invocation)
	}
}

func TestQualifyPublicNextActionDefaultsExecutablePlansToJSONPreview(t *testing.T) {
	seen := map[string]bool{}
	for _, listed := range MutationContracts() {
		if !listed.ExecutablePlanValidation || seen[listed.Command] {
			continue
		}
		seen[listed.Command] = true
		t.Run("contract/"+listed.Command, func(t *testing.T) {
			original, err := NewPublicInvocation(listed.Command)
			if err != nil {
				t.Fatal(err)
			}
			qualified, changed, err := QualifyPublicNextAction(original)
			if err != nil {
				t.Fatal(err)
			}
			if !changed || !slices.Equal(qualified.Arguments, []string{"-WhatIf", "-Format", "json"}) {
				t.Fatalf("qualified next action = %+v changed=%t", qualified, changed)
			}
			if original.Arguments != nil {
				t.Fatalf("qualification mutated its input: %+v", original)
			}
		})
	}
	if len(seen) == 0 {
		t.Fatal("no executable plan contracts were qualified")
	}

	for _, test := range []struct {
		command string
		args    []string
		want    []string
	}{
		{
			command: Continue,
			args:    []string{"feature-mission", "-Executor", "session-one", "-ExpectedExecutorGeneration", "2"},
			want:    []string{"feature-mission", "-Executor", "session-one", "-ExpectedExecutorGeneration", "2", "-WhatIf", "-Format", "json"},
		},
		{
			command: Handoff,
			args:    []string{"feature-mission"},
			want:    []string{"feature-mission", "-WhatIf", "-Format", "json"},
		},
	} {
		t.Run(test.command, func(t *testing.T) {
			original, err := NewPublicInvocation(test.command, test.args...)
			if err != nil {
				t.Fatal(err)
			}
			qualified, changed, err := QualifyPublicNextAction(original)
			if err != nil {
				t.Fatal(err)
			}
			if !changed || !qualified.HasFlag("-WhatIf") || !slices.Equal(qualified.Arguments, test.want) {
				t.Fatalf("qualified next action = %+v changed=%t", qualified, changed)
			}
			if original.HasFlag("-WhatIf") || !slices.Equal(original.Arguments, test.args) {
				t.Fatalf("qualification mutated its input: %+v", original)
			}
		})
	}

	withFormat, err := NewPublicInvocation(Continue, "feature-mission", "-Format", "text")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := QualifyPublicNextAction(withFormat); err == nil || !strings.Contains(err.Error(), "requires -Format json") {
		t.Fatalf("non-JSON executable plan next action was not rejected: %v", err)
	}
}

func TestQualifyPublicNextActionPreservesExactPhase(t *testing.T) {
	for _, fixture := range []struct {
		name    string
		command string
		args    []string
	}{
		{name: "preview", command: Continue, args: []string{"main", "-WhatIf", "-Format", "json"}},
		{name: "apply", command: Continue, args: []string{"main", "-Apply", "-ExpectedContinuePlanSha256", strings.Repeat("a", 64), "-Format", "json"}},
		{name: "other command", command: Status, args: []string{"-Format", "json"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			invocation, err := NewPublicInvocation(fixture.command, fixture.args...)
			if err != nil {
				t.Fatal(err)
			}
			qualified, changed, err := QualifyPublicNextAction(invocation)
			if err != nil || changed || !qualified.Equivalent(invocation) {
				t.Fatalf("exact phase changed: got=%+v want=%+v changed=%t err=%v", qualified, invocation, changed, err)
			}
		})
	}
}

func TestQualifyPublicNextActionAddsJSONToExistingPhase(t *testing.T) {
	hash := strings.Repeat("a", 64)
	for _, fixture := range []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "preview",
			args: []string{"-Lane", "main", "-WhatIf"},
			want: []string{"-Lane", "main", "-WhatIf", "-Format", "json"},
		},
		{
			name: "apply",
			args: []string{"-Lane", "main", "-Apply", "-ExpectedContinuePlanSha256", hash},
			want: []string{"-Lane", "main", "-Apply", "-ExpectedContinuePlanSha256", hash, "-Format", "json"},
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			invocation, err := NewPublicInvocation(Continue, fixture.args...)
			if err != nil {
				t.Fatal(err)
			}
			qualified, changed, err := QualifyPublicNextAction(invocation)
			if err != nil || !changed || !slices.Equal(qualified.Arguments, fixture.want) {
				t.Fatalf("qualified phase: got=%+v want=%+v changed=%t err=%v", qualified.Arguments, fixture.want, changed, err)
			}
			if !slices.Equal(invocation.Arguments, fixture.args) {
				t.Fatalf("qualification mutated its input: %+v", invocation)
			}
		})
	}
}

func TestQualifyPublicNextActionRejectsInvalidPhaseOrFormatBinding(t *testing.T) {
	for _, args := range [][]string{
		{"main", "-Format"},
		{"main", "-Format", "json", "--format", "text"},
		{"main", "-WhatIf", "-Apply"},
	} {
		invocation, err := NewPublicInvocation(Continue, args...)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := QualifyPublicNextAction(invocation); err == nil {
			t.Fatalf("invalid public next action error=%v args=%v", err, args)
		}
	}
}

func TestPromoteContinuePreviewToApplyPreservesTypedBindings(t *testing.T) {
	preview, err := NewPublicInvocation(
		Continue,
		"-Lane", "feature-mission",
		"-Executor", "session-one",
		"-ExpectedExecutorGeneration", "2",
		"-WhatIf",
		"-Format", "json",
	)
	if err != nil {
		t.Fatal(err)
	}
	planSHA256 := strings.Repeat("a", 64)
	apply, err := PromoteContinuePreviewToApply(preview, planSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(apply.Arguments, []string{"-Lane", "feature-mission", "-Executor", "session-one", "-ExpectedExecutorGeneration", "2", "-Apply", "-Format", "json", "-ExpectedContinuePlanSha256", planSHA256}) {
		t.Fatalf("promoted Apply invocation = %+v", apply)
	}
	if !preview.HasFlag("-WhatIf") || preview.HasFlag("-Apply") {
		t.Fatalf("preview promotion mutated its input: %+v", preview)
	}
}

func TestPromoteContinuePreviewToApplyRejectsNonPreviewPhases(t *testing.T) {
	for _, fixture := range []struct {
		command string
		args    []string
	}{
		{command: Continue, args: []string{"main"}},
		{command: Continue, args: []string{"main", "-Apply"}},
		{command: Continue, args: []string{"main", "-WhatIf", "-Apply"}},
		{command: Status, args: []string{"-WhatIf"}},
	} {
		invocation, err := NewPublicInvocation(fixture.command, fixture.args...)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := PromoteContinuePreviewToApply(invocation, strings.Repeat("a", 64)); err == nil {
			t.Fatalf("invalid preview promotion succeeded: %+v", invocation)
		}
	}
}

func TestValidateExecutablePlanInvocationSupportsStableBindings(t *testing.T) {
	hash := strings.Repeat("a", 64)
	for _, fixture := range []struct {
		command string
		flag    string
		alias   string
	}{
		{command: Continue, flag: "-ExpectedContinuePlanSha256", alias: "--expected-continue-plan-sha256"},
		{command: Complete, flag: "-ExpectedCompletePlanSha256", alias: "--expected-complete-plan-sha256"},
		{command: Control, flag: "-ExpectedControlPlanSha256", alias: "--expected-control-plan-sha256"},
		{command: Handoff, flag: "-ExpectedHandoffPlanSha256", alias: "--expected-handoff-plan-sha256"},
		{command: Init, flag: "-ExpectedInitPlanSha256", alias: "--expected-init-plan-sha256"},
		{command: Bootstrap, flag: "-ExpectedInitPlanSha256", alias: "--expected-init-plan-sha256"},
		{command: Onboard, flag: "-ExpectedOnboardingPlanSha256", alias: "--expected-onboarding-plan-sha256"},
		{command: MigrateState, flag: "-ExpectedMigrationPlanSha256", alias: "--expected-migration-plan-sha256"},
		{command: Reopen, flag: "-ExpectedReopenPlanSha256", alias: "--expected-reopen-plan-sha256"},
	} {
		t.Run(fixture.command, func(t *testing.T) {
			preview, err := NewPublicInvocation(fixture.command, "-WhatIf", "-Format", "json")
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateExecutablePlanInvocation(preview); err != nil {
				t.Fatalf("preview validation: %v", err)
			}
			apply, err := NewPublicInvocation(fixture.command, "-Apply", fixture.alias, strings.ToUpper(hash), "-Format", "json")
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateExecutablePlanInvocation(apply); err != nil {
				t.Fatalf("Apply validation: %v", err)
			}
			canonical, aliases, ok := publicPlanBinding(fixture.command)
			if !ok || canonical != fixture.flag || !slices.Equal(aliases, []string{fixture.flag, fixture.alias}) {
				t.Fatalf("publicPlanBinding=%q %v %t", canonical, aliases, ok)
			}
		})
	}
}

func TestValidateExecutablePlanInvocationRejectsNonJSONMachineFormat(t *testing.T) {
	for _, args := range [][]string{
		{"main", "-WhatIf"},
		{"main", "-WhatIf", "-Format", "text"},
		{"main", "-Apply", "-ExpectedContinuePlanSha256", strings.Repeat("a", 64), "--format", "text"},
	} {
		invocation, err := NewPublicInvocation(Continue, args...)
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidateExecutablePlanInvocation(invocation); err == nil || !strings.Contains(err.Error(), "-Format json") {
			t.Fatalf("non-JSON executable plan invocation error=%v args=%v", err, args)
		}
	}
}

func TestValidateExecutablePlanInvocationReturnsTypedFailures(t *testing.T) {
	hash := strings.Repeat("a", 64)
	for _, fixture := range []struct {
		name    string
		command string
		args    []string
		code    string
	}{
		{name: "missing phase", command: Continue, args: []string{"-Format", "json"}, code: plancontract.CodePhaseConflict},
		{name: "both phases", command: Continue, args: []string{"-WhatIf", "-Apply", "-ExpectedContinuePlanSha256", hash, "-Format", "json"}, code: plancontract.CodePhaseConflict},
		{name: "preview binding", command: Continue, args: []string{"-WhatIf", "-ExpectedContinuePlanSha256", hash, "-Format", "json"}, code: plancontract.CodePhaseConflict},
		{name: "Apply binding missing", command: Continue, args: []string{"-Apply", "-Format", "json"}, code: plancontract.CodePlanMissing},
		{name: "Apply binding invalid", command: Continue, args: []string{"-Apply", "-ExpectedContinuePlanSha256", "bad", "-Format", "json"}, code: plancontract.CodePlanInvalid},
		{name: "duplicate binding", command: Continue, args: []string{"-Apply", "-ExpectedContinuePlanSha256", hash, "--expected-continue-plan-sha256", hash, "-Format", "json"}, code: plancontract.CodePlanInvalid},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			invocation, err := NewPublicInvocation(fixture.command, fixture.args...)
			if err != nil {
				t.Fatal(err)
			}
			err = ValidateExecutablePlanInvocation(invocation)
			failure, ok := plancontract.FromError(err)
			if !ok || failure.Code != fixture.code || failure.MutationApplied || failure.MutationBoundary != "none" || failure.NextAction == "" {
				t.Fatalf("typed failure=%+v ok=%t err=%v", failure, ok, err)
			}
		})
	}
}

func TestValidateExecutablePlanInvocationRejectsUnsupportedCommand(t *testing.T) {
	invocation, err := NewPublicInvocation(Status, "-WhatIf", "-Format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateExecutablePlanInvocation(invocation); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("unsupported plan validation error=%v", err)
	}
}

func TestParsePublicInvocationAcceptsMigrationEntrypoints(t *testing.T) {
	legacy, err := ParsePublicInvocation(`/rekit continue "feature-mission" -WhatIf -Format json`)
	if err != nil {
		t.Fatal(err)
	}
	current, err := ParsePublicInvocation(`/steamai continue "feature-mission" -WhatIf -Format json`)
	if err != nil {
		t.Fatal(err)
	}
	if !legacy.Equivalent(current) {
		t.Fatalf("migration entrypoints produced different typed identities: legacy=%+v current=%+v", legacy, current)
	}
	rendered, err := current.Render()
	if err != nil {
		t.Fatal(err)
	}
	if rendered != `/rekit continue feature-mission -WhatIf -Format json` {
		t.Fatalf("typed invocation canonical render = %q", rendered)
	}
	currentRendered, err := current.RenderForEntrypoint(CurrentPublicEntrypoint)
	if err != nil {
		t.Fatal(err)
	}
	if currentRendered != `/steamai continue feature-mission -WhatIf -Format json` {
		t.Fatalf("typed invocation current render = %q", currentRendered)
	}
	if _, err := current.RenderForEntrypoint("/other"); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("unsupported entrypoint render error = %v", err)
	}
}

func TestParsePublicInvocationRejectsUncatalogedOrReboundCommands(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "bare host", text: "rekit-host -daily", want: "must begin"},
		{name: "lookalike", text: "rekitfoo status", want: "must begin"},
		{name: "without slash", text: "rekit status", want: "must begin"},
		{name: "unknown", text: "/rekit unknown -WhatIf", want: "not in the ReKit catalog"},
		{name: "nested short command", text: "/rekit continue -Command status", want: "must not override"},
		{name: "nested long command", text: "/rekit continue --command status", want: "must not override"},
		{name: "unterminated quote", text: `/rekit continue "lane`, want: "unterminated quote"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParsePublicInvocation(test.text)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ParsePublicInvocation error=%v want substring %q", err, test.want)
			}
		})
	}
}

func TestPublicInvocationRequiresCanonicalIdentity(t *testing.T) {
	invocation := PublicInvocation{SchemaVersion: PublicInvocationSchemaVersion, Command: "Continue"}
	if err := invocation.Validate(); err == nil || !strings.Contains(err.Error(), "canonical lowercase") {
		t.Fatalf("Validate error=%v", err)
	}
}

func TestPublicInvocationMigrateStateHashIsAValueFlag(t *testing.T) {
	invocation, err := ParsePublicInvocation("/rekit migrate-state -ExpectedMigrationPlanSha256 abcdef -Apply")
	if err != nil {
		t.Fatal(err)
	}
	if invocation.Command != MigrateState || !slices.Equal(invocation.Arguments, []string{"-ExpectedMigrationPlanSha256", "abcdef", "-Apply"}) {
		t.Fatalf("unexpected migrate-state invocation: %+v", invocation)
	}
}

func TestPublicInvocationRejectsAmbiguousLaneSelectors(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "positional and lane", text: "/rekit complete main -Lane feature-other -WhatIf", want: "both positional and -Lane"},
		{name: "matching duplicate lane", text: "/rekit complete -Lane main --lane main -WhatIf", want: "repeats -Lane"},
		{name: "conflicting duplicate lane", text: "/rekit complete -Lane main -Lane feature-other -WhatIf", want: "repeats -Lane"},
		{name: "lane missing value", text: "/rekit complete -Lane -WhatIf", want: "missing a value"},
		{name: "start positional and name", text: "/rekit start login -Name admin -WhatIf", want: "both positional and -Name"},
		{name: "start matching duplicate name", text: "/rekit start -Name login --name login -WhatIf", want: "repeats -Name"},
		{name: "start conflicting duplicate name", text: "/rekit start -Name login -Name admin -WhatIf", want: "repeats -Name"},
		{name: "start name missing value", text: "/rekit start -Name -WhatIf", want: "missing a value"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParsePublicInvocation(test.text)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ParsePublicInvocation error=%v want substring %q", err, test.want)
			}
		})
	}

	invocation, err := ParsePublicInvocation("/rekit start -Name login -Lane feature-login -WhatIf")
	if err != nil {
		t.Fatalf("start -Name plus exact -Lane should remain valid: %v", err)
	}
	if invocation.Command != Start || !slices.Equal(invocation.Arguments, []string{"-Name", "login", "-Lane", "feature-login", "-WhatIf"}) {
		t.Fatalf("unexpected typed start invocation: %+v", invocation)
	}
}
