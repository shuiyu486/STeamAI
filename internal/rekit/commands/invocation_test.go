package commands

import (
	"slices"
	"strings"
	"testing"
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
