package cli

import (
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

func TestScopedCommandRouteCatalogOwnsExactMigratedScopes(t *testing.T) {
	if err := validateScopedCommandRouteCatalog(); err != nil {
		t.Fatal(err)
	}
	expectedRoutes := 0
	for _, owner := range scopedCommandRuntimeOwners {
		expectedRoutes += len(commands.ScopedCommandDescriptorsFor(owner.Command))
	}
	if len(scopedCommandRoutes) != expectedRoutes {
		t.Fatalf("scoped route count=%d, want %d descriptor-backed routes", len(scopedCommandRoutes), expectedRoutes)
	}
	for _, owner := range scopedCommandRuntimeOwners {
		if len(owner.Routes) != len(commands.ScopedCommandDescriptorsFor(owner.Command)) {
			t.Fatalf("runtime owner %s route coverage drifted: %d", owner.Command, len(owner.Routes))
		}
		for _, route := range owner.Routes {
			if route.Scope.Command != owner.Command {
				t.Fatalf("runtime owner %s contains foreign route: %+v", owner.Command, route.Scope)
			}
			if _, ok := scopedCommandRouteFor(route.Scope); !ok {
				t.Fatalf("flattened scoped route missing: %+v", route.Scope)
			}
		}
	}
	for _, scope := range []commands.CommandScope{
		{Command: " " + commands.Status + " ", Mode: commands.MutationModeDefault},
		{Command: " " + commands.Control + " ", Mode: commands.MutationModeDefault},
	} {
		if route, ok := scopedCommandRouteFor(scope); ok {
			t.Fatalf("unexpected scoped route for %+v: %+v", scope, route.Scope)
		}
	}
	for _, scope := range []commands.CommandScope{
		{Command: commands.Continue, Mode: commands.MutationModeDefault},
		{Command: commands.Gate, Mode: commands.MutationModeProfile},
		{Command: commands.Gate, Mode: commands.MutationModeExecutionObservation},
		{Command: commands.Onboard, Mode: commands.MutationModeAttachedAdoption},
		{Command: commands.PlanSubagents, Mode: commands.MutationModeReviewerIntake},
		{Command: commands.Promote, Mode: commands.MutationModeCandidateDecisionDraft},
		{Command: commands.RunCurrentLoop, Mode: commands.MutationModeExternalRelay},
		{Command: commands.Sync, Mode: commands.MutationModeCurrentSync},
		{Command: commands.Update, Mode: commands.MutationModeOrdinarySync},
	} {
		if route, ok := scopedCommandRouteFor(scope); !ok || route.Scope != scope {
			t.Fatalf("migrated scoped route missing: %+v ok=%t", scope, ok)
		}
	}
}

func TestValidateScopedCommandRoutesFailsClosedOnCatalogDrift(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func([]scopedCommandRoute) []scopedCommandRoute
		want   string
	}{
		{
			name: "missing callback",
			mutate: func(routes []scopedCommandRoute) []scopedCommandRoute {
				routes[0].Handle = nil
				return routes
			},
			want: "missing binder, validator, or handler",
		},
		{
			name: "duplicate scope",
			mutate: func(routes []scopedCommandRoute) []scopedCommandRoute {
				return append(routes, routes[0])
			},
			want: "duplicate scoped command route",
		},
		{
			name: "missing descriptor",
			mutate: func(routes []scopedCommandRoute) []scopedCommandRoute {
				routes[0].Scope.Command = "unknown"
				return routes
			},
			want: "has no descriptor",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			routes := append([]scopedCommandRoute{}, scopedCommandRoutes...)
			err := validateScopedCommandRoutes(test.mutate(routes))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("catalog validation error=%v, want containing %q", err, test.want)
			}
		})
	}
}

func TestScopedCommandBindersNormalizeWithoutBroadeningCommandIdentity(t *testing.T) {
	for _, test := range []struct {
		name   string
		binder scopedCommandBinder
		opt    Options
		scope  commands.CommandScope
		format string
	}{
		{
			name:   "release default",
			binder: bindReleaseCheckCommand,
			opt:    Options{Command: commands.ReleaseCheck},
			scope:  commands.CommandScope{Command: commands.ReleaseCheck, Mode: commands.MutationModeDefault},
			format: "table",
		},
		{
			name:   "release normalized",
			binder: bindReleaseCheckCommand,
			opt:    Options{Command: commands.ReleaseCheck, Format: " JSON "},
			scope:  commands.CommandScope{Command: commands.ReleaseCheck, Mode: commands.MutationModeDefault},
			format: "json",
		},
		{
			name:   "migration default",
			binder: bindMigrateStateCommand,
			opt:    Options{Command: commands.MigrateState},
			scope:  commands.CommandScope{Command: commands.MigrateState, Mode: commands.MutationModeDefault},
			format: "json",
		},
		{
			name:   "next batch normalized",
			binder: bindNextBatchCommand,
			opt:    Options{Command: commands.NextBatch, Format: " Text "},
			scope:  commands.CommandScope{Command: commands.NextBatch, Mode: commands.MutationModeDefault},
			format: "text",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			binding, err := test.binder(test.opt, test.scope)
			if err != nil {
				t.Fatal(err)
			}
			if binding.Descriptor.Scope != test.scope || binding.Options.Format != test.format {
				t.Fatalf("binding drifted: %+v", binding)
			}
		})
	}
	if _, err := bindControlCommand(
		Options{Command: commands.Control, Format: "yaml"},
		commands.CommandScope{Command: commands.Control, Mode: commands.MutationModeDefault},
	); err == nil || !strings.Contains(err.Error(), "unsupported control format") {
		t.Fatalf("unsupported workstream format was accepted: %v", err)
	}
	if _, err := bindReleaseCheckCommand(
		Options{Command: " " + commands.ReleaseCheck + " "},
		commands.CommandScope{Command: commands.ReleaseCheck, Mode: commands.MutationModeDefault},
	); err == nil || !strings.Contains(err.Error(), "binder received") {
		t.Fatalf("binder broadened command identity: %v", err)
	}
}

func TestScopedCommandValidatorsRejectBoundaryDriftBeforeHandlers(t *testing.T) {
	release, err := bindReleaseCheckCommand(
		Options{Command: commands.ReleaseCheck},
		commands.CommandScope{Command: commands.ReleaseCheck, Mode: commands.MutationModeDefault},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateScopedCommandPolicy(release); err != nil {
		t.Fatal(err)
	}
	if err := validateReleaseCheckCommand(runtime.Context{}, release); err != nil {
		t.Fatal(err)
	}

	targeted := release
	targeted.Options.Format = "json"
	if err := validateReleaseCheckCommand(runtime.Context{TargetProvided: true}, targeted); err == nil || !strings.Contains(err.Error(), "omit -Target") {
		t.Fatalf("release target boundary was accepted: %v", err)
	}
	unsupported := release
	unsupported.Options.Format = "yaml"
	if err := validateReleaseCheckCommand(runtime.Context{}, unsupported); err == nil || !strings.Contains(err.Error(), "unsupported release-check format") {
		t.Fatalf("release format boundary was accepted: %v", err)
	}

	profileDrift := release
	profileDrift.Descriptor.Profile.HeavyTool = true
	if err := validateScopedCommandPolicy(profileDrift); err == nil || !strings.Contains(err.Error(), "no-heavy-tool/no-authority") {
		t.Fatalf("heavy-tool policy drift was accepted: %v", err)
	}

	migration, err := bindMigrateStateCommand(
		Options{Command: commands.MigrateState, Apply: true},
		commands.CommandScope{Command: commands.MigrateState, Mode: commands.MutationModeDefault},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateMigrateStateCommand(runtime.Context{TargetProvided: true}, migration); err == nil || !strings.Contains(err.Error(), "requires -ExpectedMigrationPlanSha256") {
		t.Fatalf("migration Apply without exact hash was accepted: %v", err)
	}

	nextBatch, err := bindNextBatchCommand(
		Options{Command: commands.NextBatch},
		commands.CommandScope{Command: commands.NextBatch, Mode: commands.MutationModeDefault},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateNextBatchCommand(runtime.Context{}, nextBatch); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("ambiguous next-batch phase was accepted: %v", err)
	}

	control, err := bindControlCommand(
		Options{Command: commands.Control, WhatIf: true, Format: "tsv"},
		commands.CommandScope{Command: commands.Control, Mode: commands.MutationModeDefault},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateControlCommand(runtime.Context{}, control); err == nil || !strings.Contains(err.Error(), "only -Format json or text") {
		t.Fatalf("control format boundary was accepted: %v", err)
	}
}

func TestScopedCommandModeResolversOwnEveryExactMutationRoute(t *testing.T) {
	for _, test := range []struct {
		name    string
		command string
		opt     Options
		mode    string
	}{
		{"onboard ordinary", commands.Onboard, Options{}, commands.MutationModeOrdinaryOnboarding},
		{"plan reviewer completion", commands.PlanSubagents, Options{RecordReviewerCompletion: true}, commands.MutationModeRecordReviewerCompletion},
		{"plan artifacts", commands.PlanSubagents, Options{}, commands.MutationModePlanArtifacts},
		{"promote decision draft", commands.Promote, Options{DraftCandidateDecision: true, CandidateDecisionPath: "decision.json"}, commands.MutationModeCandidateDecisionDraft},
		{"promote ordinary", commands.Promote, Options{}, commands.MutationModeOrdinaryPromote},
		{"current loop dispatch", commands.RunCurrentLoop, Options{ClaimExternalSessionDispatch: true, RecordExternalSessionLaunch: true}, commands.MutationModeExternalDispatch},
		{"current loop default", commands.RunCurrentLoop, Options{}, commands.MutationModeDefault},
		{"sync current", commands.Sync, Options{sourceRepoRootProvided: true}, commands.MutationModeCurrentSync},
		{"sync selected memory", commands.Sync, Options{SelectPackMemoryChange: "candidate"}, commands.MutationModeSelectedPackMemorySync},
		{"sync consumer verification", commands.Sync, Options{VerifyPackMemoryConsumerUse: true}, commands.MutationModePackMemoryConsumerVerification},
		{"sync ordinary", commands.Sync, Options{}, commands.MutationModeOrdinarySync},
		{"update ordinary", commands.Update, Options{}, commands.MutationModeOrdinarySync},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.opt.Command = test.command
			owner, ok := scopedCommandRuntimeOwnerFor(test.command)
			if !ok {
				t.Fatalf("missing runtime owner for %s", test.command)
			}
			mode, err := owner.ResolveMode(test.opt)
			if err != nil || mode != test.mode {
				t.Fatalf("resolved mode=%q err=%v, want %q", mode, err, test.mode)
			}
			scope := commands.CommandScope{Command: test.command, Mode: mode}
			if _, ok := scopedCommandRouteForOwner(owner, scope); !ok {
				t.Fatalf("owner %s omitted resolved route %+v", test.command, scope)
			}
		})
	}
}

func TestRuntimeOwnerInventoryCoversEveryPublicCommand(t *testing.T) {
	seen := map[string]string{}
	for _, owner := range scopedCommandRuntimeOwners {
		if previous := seen[owner.Command]; previous != "" {
			t.Fatalf("public command %s has duplicate owners %s and scoped", owner.Command, previous)
		}
		seen[owner.Command] = "scoped"
	}
	for _, command := range commands.Public() {
		if seen[command] == "" {
			t.Fatalf("public command %s has no runtime callback owner", command)
		}
	}
	if len(seen) != len(commands.Public()) {
		t.Fatalf("runtime owner inventory=%d public commands=%d", len(seen), len(commands.Public()))
	}
	if len(preRuntimeCommandOwners) != 3 ||
		preRuntimeCommandOwners[0].Name != "retired-pack-migration" ||
		preRuntimeCommandOwners[0].Mode != commands.MutationModeDefault ||
		len(preRuntimeCommandOwners[0].Scopes) != 1 ||
		preRuntimeCommandOwners[0].Scopes[0] != (commands.CommandScope{Command: commands.MigrateState, Mode: commands.MutationModeDefault}) ||
		preRuntimeCommandOwners[0].Validate == nil ||
		preRuntimeCommandOwners[1].Name != "current-sync-maintenance" ||
		preRuntimeCommandOwners[1].Mode != commands.MutationModeCurrentSync ||
		len(preRuntimeCommandOwners[1].Scopes) != 1 ||
		preRuntimeCommandOwners[1].Scopes[0] != (commands.CommandScope{Command: commands.Sync, Mode: commands.MutationModeCurrentSync}) ||
		preRuntimeCommandOwners[1].Validate == nil ||
		preRuntimeCommandOwners[2].Name != "current-sync-recovery-front-door" ||
		preRuntimeCommandOwners[2].Mode != "pending-current-sync-recovery" ||
		len(preRuntimeCommandOwners[2].Scopes) != 0 ||
		preRuntimeCommandOwners[2].Validate == nil {
		t.Fatalf("pre-runtime owners drifted: %+v", preRuntimeCommandOwners)
	}
}

func TestValidatePreRuntimeCommandOwnersFailsClosedOnDrift(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func([]preRuntimeCommandOwner) []preRuntimeCommandOwner
		want   string
	}{
		{
			name: "missing validator",
			mutate: func(owners []preRuntimeCommandOwner) []preRuntimeCommandOwner {
				owners[0].Validate = nil
				return owners
			},
			want: "pre-runtime command owner is invalid",
		},
		{
			name: "duplicate exact scope",
			mutate: func(owners []preRuntimeCommandOwner) []preRuntimeCommandOwner {
				duplicate := owners[0]
				duplicate.Name = "duplicate-current-sync"
				return append(owners, duplicate)
			},
			want: "owned by both",
		},
		{
			name: "scope outside command coverage",
			mutate: func(owners []preRuntimeCommandOwner) []preRuntimeCommandOwner {
				owners[0].Commands = []string{commands.Status}
				return owners
			},
			want: "invalid scope",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			owners := append([]preRuntimeCommandOwner{}, preRuntimeCommandOwners...)
			err := validatePreRuntimeCommandOwners(test.mutate(owners))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("pre-runtime validation error=%v, want containing %q", err, test.want)
			}
		})
	}
}

func TestRunScopedCommandClaimsOnlyMigratedDefaultScopes(t *testing.T) {
	handled, err := runScopedCommand(
		runtime.Context{TargetProvided: true},
		Options{Command: commands.ReleaseCheck},
		testingWriter{t: t},
	)
	if !handled || err == nil || !strings.Contains(err.Error(), "omit -Target") {
		t.Fatalf("migrated route dispatch handled=%t err=%v", handled, err)
	}
	owner, ok := scopedCommandRuntimeOwnerFor(commands.Status)
	if !ok || owner.ResolveMode == nil || len(owner.Routes) != 1 || owner.Routes[0].Handle == nil {
		t.Fatalf("read-only status owner is incomplete: %+v", owner)
	}
}

type testingWriter struct {
	t *testing.T
}

func (writer testingWriter) Write(data []byte) (int, error) {
	writer.t.Fatalf("validator should have stopped before handler output: %q", data)
	return 0, nil
}
