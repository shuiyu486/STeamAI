package manifest

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
)

func TestLoadDefaultManifestSchema(t *testing.T) {
	m, err := Load(repoRoot(t), defaults.DefaultPack)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ValidateSchema(); err != nil {
		t.Fatal(err)
	}
	if m.WorkstreamDefaults["defaultAuthorityLane"] != "devirt-main" {
		t.Fatalf("defaultAuthorityLane = %q, want devirt-main", m.WorkstreamDefaults["defaultAuthorityLane"])
	}
	if len(m.ToolingCandidateSources) == 0 {
		t.Fatal("ToolingCandidateSources is empty")
	}
	if len(m.PromoteDenyPatterns) == 0 {
		t.Fatal("PromoteDenyPatterns is empty")
	}
	assertHeavyToolGateSet(t, m)
	if _, err := regexp.Compile(m.PromoteDenyPatterns[0]); err != nil {
		t.Fatalf("first deny pattern does not compile: %v", err)
	}
}

func TestLoadTemplateManifestSchema(t *testing.T) {
	m, err := Load(repoRoot(t), "_template")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ValidateSchema(); err != nil {
		t.Fatal(err)
	}
	if m.WorkstreamDefaults["defaultAuthorityLane"] != "main" {
		t.Fatalf("defaultAuthorityLane = %q, want main", m.WorkstreamDefaults["defaultAuthorityLane"])
	}
	assertHeavyToolGateSet(t, m)
}

func TestLoadWebSecurityManifestSchema(t *testing.T) {
	m, err := Load(repoRoot(t), "web-security")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ValidateSchema(); err != nil {
		t.Fatal(err)
	}
	if m.WorkstreamDefaults["defaultAuthorityLane"] != "main" {
		t.Fatalf("defaultAuthorityLane = %q, want main", m.WorkstreamDefaults["defaultAuthorityLane"])
	}
	if len(m.SubagentRoutes) != 2 {
		t.Fatalf("SubagentRoutes = %d, want 2", len(m.SubagentRoutes))
	}
	if m.SubagentRoutes[0].ID != "web-security:bounded-review" || m.SubagentRoutes[1].ID != "web-security:feature-analysis" {
		t.Fatalf("unexpected web-security routes: %+v", m.SubagentRoutes)
	}
	assertHeavyToolGateSet(t, m)
}

func assertHeavyToolGateSet(t *testing.T, m *Manifest) {
	t.Helper()
	want := "debug,dump,full-trace,inject,network,patch,symex"
	if got := strings.Join(m.HeavyToolGateIDs(), ","); got != want {
		t.Fatalf("HeavyToolGateIDs() = %q, want %q", got, want)
	}
	for _, action := range m.HeavyToolGateIDs() {
		gate, ok := m.HeavyToolGate(action)
		if !ok || !gate.RequiresConfirmation || len(gate.SideEffects) == 0 || len(gate.StopConditions) == 0 || strings.TrimSpace(gate.DefaultRisk) == "" {
			t.Fatalf("invalid heavyToolGate %q: ok=%t gate=%+v", action, ok, gate)
		}
	}
}

func validManifestFixture() Manifest {
	return Manifest{
		ManifestPath:         "unit-manifest.yml",
		Pack:                 "unit",
		SchemaVersion:        "1",
		Name:                 "unit-pack",
		Version:              "0.1.0",
		Description:          "Unit test pack",
		Maturity:             "template",
		ManagedFiles:         []string{"references/template/README.md"},
		PromoteFiles:         []string{"references/template/README.md"},
		ManagedBlock:         map[string]string{"file": "CLAUDE.local.md", "blockId": "rekit:router", "source": "CLAUDE.local.snippet.md"},
		explicitManagedBlock: map[string]string{"file": "CLAUDE.local.md", "blockId": "rekit:router", "source": "CLAUDE.local.snippet.md"},
		explicitLists: map[string]bool{
			"managedFiles":            true,
			"templateFiles":           true,
			"localNeverOverwrite":     true,
			"promoteFiles":            true,
			"commonPolicies":          true,
			"policyOverlays":          true,
			"subagentRoutes":          true,
			"toolingFiles":            true,
			"promptFiles":             true,
			"toolingCandidateSources": true,
			"authorityFiles":          true,
			"promoteDenyPatterns":     true,
			"heavyToolGates":          true,
			"laneTypes":               true,
		},
		ToolingCandidateSources: []string{"references/template/toolchain-router.md"},
		WorkstreamDefaults:      map[string]string{"defaultAuthorityLane": "main", "defaultStartLaneType": "feature", "backupRoot": ".rekit/backups/sync", "requestDefaultTargetLane": "main"},
		AuthorityFiles:          []string{"references/template/task-handoff.md"},
		explicitMaps: map[string]bool{
			"syncPolicy":         true,
			"workstreamDefaults": true,
			"budgets":            true,
		},
		SyncPolicy:          map[string]string{"managedFiles": "overwrite-with-backup", "templateFiles": "create-if-missing", "localFiles": "never-overwrite"},
		Budgets:             map[string]string{"defaultMarkdown": "16384"},
		PromoteDenyPatterns: []string{"artifacts[\\/]"},
		HeavyToolGates:      []HeavyToolGate{{ID: "debug", Title: "Debug", SideEffects: []string{"debug", "filesystem-write"}, DefaultRisk: "high", RequiresConfirmation: true, explicitRequiresConfirmation: "true", StopConditions: []string{"timeout"}}},
		LaneTypes: []LaneType{
			{ID: "main", Title: "Main", Authority: true, explicitAuthority: "true", WorkspaceRoot: "workspace/main", CanWrite: []string{"references/template/task-handoff.md"}, ReadOnly: []string{".rekit/facts/**"}, Outputs: []string{"publication"}},
			{ID: "feature", Title: "Feature", explicitAuthority: "false", WorkspaceRoot: "workspace/features", CanWrite: []string{"own-workspace"}, ReadOnly: []string{"references/template/**", ".rekit/facts/**"}, Outputs: []string{"observation"}},
		},
	}
}

func TestValidateSchemaRequiresExplicitSchemaVersion(t *testing.T) {
	m := validManifestFixture()
	m.SchemaVersion = ""
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "schemaVersion is missing") {
		t.Fatalf("ValidateSchema error = %v, want missing schemaVersion error", err)
	}
	m = validManifestFixture()
	m.SchemaVersion = "2"
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "schemaVersion has unsupported value: 2") {
		t.Fatalf("ValidateSchema error = %v, want unsupported schemaVersion error", err)
	}
}

func TestLoadDoesNotInferSchemaVersionDefault(t *testing.T) {
	repo := t.TempDir()
	packRoot := filepath.Join(repo, "packs", "missing-schema-version")
	if err := os.MkdirAll(packRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestText := `name: missing-schema-version
version: 0.1.0
description: test pack skeleton
maturity: skeleton

managedFiles:
  - references/test/README.md
templateFiles: []
localNeverOverwrite: []
promoteFiles:
  - references/test/README.md
managedBlock:
  file: CLAUDE.local.md
  blockId: rekit:test
  source: CLAUDE.local.snippet.md
toolingCandidateSources:
  - references/test/toolchain-router.md
workstreamDefaults:
  defaultAuthorityLane: main
  defaultStartLaneType: feature
  backupRoot: .rekit/backups/sync
  requestDefaultTargetLane: main
authorityFiles:
  - references/test/README.md
commonPolicies: []
policyOverlays: []
subagentRoutes: []
toolingFiles: []
promptFiles: []
syncPolicy:
  managedFiles: overwrite-with-backup
  templateFiles: create-if-missing
  localFiles: never-overwrite
promoteDenyPatterns:
  - "artifacts[\\/]"
budgets:
  defaultMarkdown: 16384
heavyToolGates:
  - id: debug
    title: Debug
    sideEffects: debug,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: timeout
laneTypes:
  - id: main
    title: Main
    authority: true
    workspaceRoot: workspace/main
    canWrite: references/test/README.md
    readOnly: .rekit/facts/**
    outputs: publication
  - id: feature
    title: Feature
    authority: false
    workspaceRoot: workspace/features
    canWrite: own-workspace
    readOnly: references/test/**
    outputs: observation
`
	if err := os.WriteFile(filepath.Join(packRoot, "manifest.yml"), []byte(manifestText), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(repo, "missing-schema-version")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(m.SchemaVersion) != "" {
		t.Fatalf("SchemaVersion = %q, want no implicit fallback", m.SchemaVersion)
	}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "schemaVersion is missing") {
		t.Fatalf("ValidateSchema error = %v, want missing schemaVersion error", err)
	}
}

func TestValidateSchemaRequiresExplicitIdentity(t *testing.T) {
	m := validManifestFixture()
	m.Name = ""
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "name is missing") {
		t.Fatalf("ValidateSchema error = %v, want missing name error", err)
	}
	m = validManifestFixture()
	m.Name = "unit pack"
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "name has invalid value: unit pack") {
		t.Fatalf("ValidateSchema error = %v, want invalid name error", err)
	}
	m = validManifestFixture()
	m.Version = ""
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "version is missing") {
		t.Fatalf("ValidateSchema error = %v, want missing version error", err)
	}
	m = validManifestFixture()
	m.Version = "latest"
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "version has invalid value: latest") {
		t.Fatalf("ValidateSchema error = %v, want invalid version error", err)
	}
}

func TestValidateSchemaRequiresExplicitDescription(t *testing.T) {
	m := validManifestFixture()
	m.Description = ""
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "description is missing") {
		t.Fatalf("ValidateSchema error = %v, want missing description error", err)
	}
}

func TestPackSummaryUsesExplicitMaturity(t *testing.T) {
	m := &Manifest{
		Pack:               "plain-pack",
		SchemaVersion:      "1",
		Maturity:           "experimental",
		Description:        "plain security pack",
		WorkstreamDefaults: map[string]string{},
	}
	if got := m.Summary().SchemaVersion; got != "1" {
		t.Fatalf("Summary().SchemaVersion = %q, want 1", got)
	}
	if got := m.Summary().Maturity; got != "experimental" {
		t.Fatalf("Summary().Maturity = %q, want experimental", got)
	}
	m.Maturity = ""
	if got := m.Summary().Maturity; got != "missing" {
		t.Fatalf("Summary().Maturity = %q, want missing", got)
	}
}

func TestValidateSchemaRejectsInvalidHeavyToolGates(t *testing.T) {
	base := validManifestFixture()
	withoutGates := base
	withoutGates.HeavyToolGates = nil
	if err := withoutGates.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "heavyToolGates") {
		t.Fatalf("ValidateSchema error = %v, want heavyToolGates error", err)
	}
	missingConfirmation := base
	missingConfirmation.HeavyToolGates[0].explicitRequiresConfirmation = ""
	if err := missingConfirmation.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "heavyToolGates entry debug is missing requiresConfirmation") {
		t.Fatalf("ValidateSchema error = %v, want missing requiresConfirmation error", err)
	}
	invalidConfirmation := base
	invalidConfirmation.HeavyToolGates[0].explicitRequiresConfirmation = "yes"
	if err := invalidConfirmation.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "heavyToolGates entry debug has invalid requiresConfirmation") {
		t.Fatalf("ValidateSchema error = %v, want invalid requiresConfirmation error", err)
	}
	falseConfirmation := base
	falseConfirmation.HeavyToolGates[0].RequiresConfirmation = false
	falseConfirmation.HeavyToolGates[0].explicitRequiresConfirmation = "false"
	if err := falseConfirmation.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "heavyToolGates entry debug must set requiresConfirmation: true") {
		t.Fatalf("ValidateSchema error = %v, want false requiresConfirmation error", err)
	}
	invalid := base
	invalid.HeavyToolGates = []HeavyToolGate{{ID: "debug", Title: "Debug", SideEffects: []string{"filesystem-write"}, DefaultRisk: "high", RequiresConfirmation: true, explicitRequiresConfirmation: "true", StopConditions: []string{"timeout"}}}
	if err := invalid.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "sideEffects must include") {
		t.Fatalf("ValidateSchema error = %v, want sideEffects include id error", err)
	}
	emptyEffect := base
	emptyEffect.HeavyToolGates = []HeavyToolGate{{ID: "debug", Title: "Debug", SideEffects: splitScalarList("debug,,filesystem-write"), DefaultRisk: "high", RequiresConfirmation: true, explicitRequiresConfirmation: "true", StopConditions: []string{"timeout"}}}
	if err := emptyEffect.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "heavyToolGates entry debug contains an empty sideEffects item") {
		t.Fatalf("ValidateSchema error = %v, want empty sideEffects item error", err)
	}
	emptyStop := base
	emptyStop.HeavyToolGates = []HeavyToolGate{{ID: "debug", Title: "Debug", SideEffects: []string{"debug", "filesystem-write"}, DefaultRisk: "high", RequiresConfirmation: true, explicitRequiresConfirmation: "true", StopConditions: splitScalarList("timeout,")}}
	if err := emptyStop.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "heavyToolGates entry debug contains an empty stopConditions item") {
		t.Fatalf("ValidateSchema error = %v, want empty stopConditions item error", err)
	}
	duplicateEffect := base
	duplicateEffect.HeavyToolGates = []HeavyToolGate{{ID: "debug", Title: "Debug", SideEffects: []string{"debug", "debug"}, DefaultRisk: "high", RequiresConfirmation: true, explicitRequiresConfirmation: "true", StopConditions: []string{"timeout"}}}
	if err := duplicateEffect.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "heavyToolGates entry debug contains duplicate sideEffects item: debug") {
		t.Fatalf("ValidateSchema error = %v, want duplicate sideEffects error", err)
	}
	duplicateStop := base
	duplicateStop.HeavyToolGates = []HeavyToolGate{{ID: "debug", Title: "Debug", SideEffects: []string{"debug", "filesystem-write"}, DefaultRisk: "high", RequiresConfirmation: true, explicitRequiresConfirmation: "true", StopConditions: []string{"timeout", "Timeout"}}}
	if err := duplicateStop.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "heavyToolGates entry debug contains duplicate stopConditions item: Timeout") {
		t.Fatalf("ValidateSchema error = %v, want duplicate stopConditions error", err)
	}
	valid := base
	valid.HeavyToolGates = []HeavyToolGate{{ID: "debug", Title: "Debug", SideEffects: []string{"debug", "filesystem-write"}, DefaultRisk: "high", RequiresConfirmation: true, explicitRequiresConfirmation: "true", StopConditions: []string{"timeout"}}}
	if err := valid.ValidateSchema(); err != nil {
		t.Fatalf("ValidateSchema valid heavyToolGates error = %v", err)
	}
}

func TestValidateSchemaRequiresExplicitListPresence(t *testing.T) {
	for _, key := range manifestListPresenceKeys {
		m := validManifestFixture()
		m.explicitLists[key] = false
		if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "manifest must explicitly declare "+key) {
			t.Fatalf("ValidateSchema error = %v, want explicit %s error", err, key)
		}
	}
}

func TestValidateSchemaRequiresValidManagedFileLists(t *testing.T) {
	for _, tc := range []struct {
		name   string
		want   string
		mutate func(*Manifest)
	}{
		{
			name:   "managedFiles empty path",
			want:   "managedFiles contains an empty path",
			mutate: func(m *Manifest) { m.ManagedFiles = []string{""} },
		},
		{
			name: "managedFiles duplicate path",
			want: "managedFiles contains duplicate path: references/template/README.md",
			mutate: func(m *Manifest) {
				m.ManagedFiles = []string{"references/template/README.md", "references/template/README.md"}
			},
		},
		{
			name:   "templateFiles wrong suffix",
			want:   "templateFiles entry must end with .template.md: references/template/task-handoff.md",
			mutate: func(m *Manifest) { m.TemplateFiles = []string{"references/template/task-handoff.md"} },
		},
		{
			name: "templateFiles duplicate path",
			want: "templateFiles contains duplicate path: references/template/task-handoff.template.md",
			mutate: func(m *Manifest) {
				m.TemplateFiles = []string{"references/template/task-handoff.template.md", "references/template/task-handoff.template.md"}
			},
		},
		{
			name:   "localNeverOverwrite duplicate path",
			want:   "localNeverOverwrite contains duplicate path: CLAUDE.local.md",
			mutate: func(m *Manifest) { m.LocalFiles = []string{"CLAUDE.local.md", "CLAUDE.local.md"} },
		},
		{
			name:   "localNeverOverwrite overlaps managedFiles",
			want:   "localNeverOverwrite entry also appears in managedFiles: references/template/README.md",
			mutate: func(m *Manifest) { m.LocalFiles = []string{"references/template/README.md"} },
		},
		{
			name:   "promoteFiles empty path",
			want:   "promoteFiles contains an empty path",
			mutate: func(m *Manifest) { m.PromoteFiles = []string{""} },
		},
		{
			name: "promoteFiles duplicate path",
			want: "promoteFiles contains duplicate path: references/template/README.md",
			mutate: func(m *Manifest) {
				m.PromoteFiles = []string{"references/template/README.md", "references/template/README.md"}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifestFixture()
			tc.mutate(&m)
			if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateSchema error = %v, want %s", err, tc.want)
			}
		})
	}
}

func TestValidateSchemaRequiresValidPolicyAndAuxiliaryLists(t *testing.T) {
	for _, tc := range []struct {
		name   string
		want   string
		mutate func(*Manifest)
	}{
		{
			name:   "commonPolicies",
			want:   "commonPolicies contains invalid id: Agent Team",
			mutate: func(m *Manifest) { m.CommonPolicies = []string{"Agent Team"} },
		},
		{
			name:   "policyOverlays",
			want:   "policyOverlays entry must be under policies/: references/template/reviewer.overlay.md",
			mutate: func(m *Manifest) { m.PolicyOverlays = []string{"references/template/reviewer.overlay.md"} },
		},
		{
			name:   "toolingFiles",
			want:   "toolingFiles entry must be under tooling/: references/template/tooling.md",
			mutate: func(m *Manifest) { m.ToolingFiles = []string{"references/template/tooling.md"} },
		},
		{
			name:   "promptFiles",
			want:   "promptFiles entry must be under common/prompts/ or packs/unit/prompts/: prompts/unit.md",
			mutate: func(m *Manifest) { m.PromptFiles = []string{"prompts/unit.md"} },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifestFixture()
			tc.mutate(&m)
			if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateSchema error = %v, want %s", err, tc.want)
			}
		})
	}
	m := validManifestFixture()
	m.CommonPolicies = []string{"agent-team"}
	m.PolicyOverlays = []string{"policies/reviewer.overlay.md"}
	m.ToolingFiles = []string{"tooling/README.md"}
	m.PromptFiles = []string{"common/prompts/lane-main-session.md", "packs/unit/prompts/unit.md"}
	if err := m.ValidateSchema(); err != nil {
		t.Fatalf("ValidateSchema valid policy/list entries error = %v", err)
	}
}

func TestValidateSchemaRequiresSubagentRouteNamespacedID(t *testing.T) {
	m := validManifestFixture()
	m.SubagentRoutes = []SubagentRoute{{
		ID:                  "bounded-review",
		TaskTypes:           "candidate-review",
		Trigger:             "fixed-boundary read-only review",
		ShardBasis:          "item",
		TargetItemsPerAgent: "1",
		MaxParallel:         "1",
		Reference:           "references/template/README.md",
		SubagentPermissions: "read-only",
		MainAgentOwns:       "validation",
		OutputContract:      "item,decision",
	}}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "subagent route has invalid id: bounded-review") {
		t.Fatalf("ValidateSchema error = %v, want invalid route id error", err)
	}
	m.SubagentRoutes[0].ID = "unit:bounded-review"
	if err := m.ValidateSchema(); err != nil {
		t.Fatalf("ValidateSchema valid subagent route id error = %v", err)
	}
}

func TestValidateSchemaRequiresSubagentRoutePackNamespace(t *testing.T) {
	m := validManifestFixture()
	m.SubagentRoutes = []SubagentRoute{{
		ID:                  "other:bounded-review",
		TaskTypes:           "candidate-review",
		Trigger:             "fixed-boundary read-only review",
		ShardBasis:          "item",
		TargetItemsPerAgent: "1",
		MaxParallel:         "1",
		Reference:           "references/template/README.md",
		SubagentPermissions: "read-only",
		MainAgentOwns:       "validation",
		OutputContract:      "item,decision",
	}}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "subagent route id other:bounded-review must use pack namespace unit") {
		t.Fatalf("ValidateSchema error = %v, want route namespace ownership error", err)
	}
	m.SubagentRoutes[0].ID = "unit:bounded-review"
	if err := m.ValidateSchema(); err != nil {
		t.Fatalf("ValidateSchema valid subagent route namespace error = %v", err)
	}
}

func TestValidateSchemaRequiresSubagentRouteTrigger(t *testing.T) {
	m := validManifestFixture()
	m.SubagentRoutes = []SubagentRoute{{
		ID:                  "unit:bounded-review",
		TaskTypes:           "candidate-review",
		ShardBasis:          "item",
		TargetItemsPerAgent: "1",
		MaxParallel:         "1",
		Reference:           "references/template/README.md",
		SubagentPermissions: "read-only",
		MainAgentOwns:       "validation",
		OutputContract:      "item,decision",
	}}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "subagent route unit:bounded-review is missing trigger") {
		t.Fatalf("ValidateSchema error = %v, want missing route trigger error", err)
	}
	m.SubagentRoutes[0].Trigger = "fixed-boundary read-only review"
	if err := m.ValidateSchema(); err != nil {
		t.Fatalf("ValidateSchema valid subagent route error = %v", err)
	}
}

func TestValidateSchemaBoundsSubagentRouteNumericOptions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		want   string
		mutate func(*SubagentRoute)
	}{
		{
			name:   "targetItemsPerAgent",
			want:   "subagent route unit:bounded-review has targetItemsPerAgent above supported maximum 64: 65",
			mutate: func(route *SubagentRoute) { route.TargetItemsPerAgent = "65" },
		},
		{
			name:   "maxParallel",
			want:   "subagent route unit:bounded-review has maxParallel above supported maximum 16: 17",
			mutate: func(route *SubagentRoute) { route.MaxParallel = "17" },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifestFixture()
			m.SubagentRoutes = []SubagentRoute{{
				ID:                  "unit:bounded-review",
				TaskTypes:           "candidate-review",
				Trigger:             "fixed-boundary read-only review",
				ShardBasis:          "item",
				TargetItemsPerAgent: "64",
				MaxParallel:         "16",
				Reference:           "references/template/README.md",
				SubagentPermissions: "read-only",
				MainAgentOwns:       "validation",
				OutputContract:      "item,decision",
			}}
			tc.mutate(&m.SubagentRoutes[0])
			if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateSchema error = %v, want %s", err, tc.want)
			}
		})
	}
}

func TestValidateSchemaRequiresValidSubagentRouteShardBasis(t *testing.T) {
	m := validManifestFixture()
	m.SubagentRoutes = []SubagentRoute{{
		ID:                  "unit:bounded-review",
		TaskTypes:           "candidate-review",
		Trigger:             "fixed-boundary read-only review",
		ShardBasis:          "function-or-",
		TargetItemsPerAgent: "1",
		MaxParallel:         "1",
		Reference:           "references/template/README.md",
		SubagentPermissions: "read-only",
		MainAgentOwns:       "validation",
		OutputContract:      "item,decision",
	}}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "subagent route unit:bounded-review has invalid shardBasis: function-or-") {
		t.Fatalf("ValidateSchema error = %v, want invalid shardBasis error", err)
	}
	m.SubagentRoutes[0].ShardBasis = "function-or-handler"
	if err := m.ValidateSchema(); err != nil {
		t.Fatalf("ValidateSchema valid shardBasis error = %v", err)
	}
}

func TestValidateSchemaRequiresSupportedSubagentPermissions(t *testing.T) {
	m := validManifestFixture()
	m.SubagentRoutes = []SubagentRoute{{
		ID:                  "unit:bounded-review",
		TaskTypes:           "candidate-review",
		Trigger:             "fixed-boundary read-only review",
		ShardBasis:          "item",
		TargetItemsPerAgent: "1",
		MaxParallel:         "1",
		Reference:           "references/template/README.md",
		SubagentPermissions: "workspace-write",
		MainAgentOwns:       "validation",
		OutputContract:      "item,decision",
	}}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "subagent route unit:bounded-review has unsupported subagentPermissions: workspace-write") {
		t.Fatalf("ValidateSchema error = %v, want unsupported subagentPermissions error", err)
	}
	m.SubagentRoutes[0].SubagentPermissions = "read-only-or-workspace-only"
	if err := m.ValidateSchema(); err != nil {
		t.Fatalf("ValidateSchema valid subagentPermissions error = %v", err)
	}
}

func TestValidateSchemaRequiresDeclaredSubagentRoutePolicyOverlay(t *testing.T) {
	m := validManifestFixture()
	m.SubagentRoutes = []SubagentRoute{{
		ID:                  "unit:bounded-review",
		TaskTypes:           "candidate-review",
		Trigger:             "fixed-boundary read-only review",
		ShardBasis:          "item",
		TargetItemsPerAgent: "1",
		MaxParallel:         "1",
		Reference:           "references/template/README.md",
		PolicyOverlay:       "policies/reviewer.overlay.md",
		SubagentPermissions: "read-only",
		MainAgentOwns:       "validation",
		OutputContract:      "item,decision",
	}}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "subagent route unit:bounded-review policyOverlay is not declared in policyOverlays: policies/reviewer.overlay.md") {
		t.Fatalf("ValidateSchema error = %v, want undeclared policyOverlay error", err)
	}
	m.PolicyOverlays = []string{"policies/reviewer.overlay.md"}
	if err := m.ValidateSchema(); err != nil {
		t.Fatalf("ValidateSchema valid policyOverlay error = %v", err)
	}
}

func TestValidateSchemaRejectsInvalidSubagentRouteListItems(t *testing.T) {
	for _, tc := range []struct {
		name   string
		want   string
		mutate func(*SubagentRoute)
	}{
		{
			name:   "taskTypes",
			want:   "subagent route unit:bounded-review has invalid taskTypes item: candidate review",
			mutate: func(route *SubagentRoute) { route.TaskTypes = "candidate review" },
		},
		{
			name:   "mainAgentOwns",
			want:   "subagent route unit:bounded-review has invalid mainAgentOwns item: AuthoritySync",
			mutate: func(route *SubagentRoute) { route.MainAgentOwns = "validation,AuthoritySync" },
		},
		{
			name:   "outputContract",
			want:   "subagent route unit:bounded-review has invalid outputContract item: next.action",
			mutate: func(route *SubagentRoute) { route.OutputContract = "item,next.action" },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifestFixture()
			m.SubagentRoutes = []SubagentRoute{{
				ID:                  "unit:bounded-review",
				TaskTypes:           "candidate-review",
				Trigger:             "fixed-boundary read-only review",
				ShardBasis:          "item",
				TargetItemsPerAgent: "1",
				MaxParallel:         "1",
				Reference:           "references/template/README.md",
				SubagentPermissions: "read-only",
				MainAgentOwns:       "validation",
				OutputContract:      "item,decision",
			}}
			tc.mutate(&m.SubagentRoutes[0])
			if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateSchema error = %v, want %s", err, tc.want)
			}
		})
	}
}

func TestValidateSchemaRejectsEmptySubagentRouteListItems(t *testing.T) {
	for _, tc := range []struct {
		name   string
		want   string
		mutate func(*SubagentRoute)
	}{
		{
			name:   "taskTypes",
			want:   "subagent route unit:bounded-review contains an empty taskTypes item",
			mutate: func(route *SubagentRoute) { route.TaskTypes = "candidate-review," },
		},
		{
			name:   "mainAgentOwns",
			want:   "subagent route unit:bounded-review contains an empty mainAgentOwns item",
			mutate: func(route *SubagentRoute) { route.MainAgentOwns = "validation,,handoff-update" },
		},
		{
			name:   "outputContract",
			want:   "subagent route unit:bounded-review contains an empty outputContract item",
			mutate: func(route *SubagentRoute) { route.OutputContract = "item; ;decision" },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifestFixture()
			m.SubagentRoutes = []SubagentRoute{{
				ID:                  "unit:bounded-review",
				TaskTypes:           "candidate-review",
				Trigger:             "fixed-boundary read-only review",
				ShardBasis:          "item",
				TargetItemsPerAgent: "1",
				MaxParallel:         "1",
				Reference:           "references/template/README.md",
				SubagentPermissions: "read-only",
				MainAgentOwns:       "validation",
				OutputContract:      "item,decision",
			}}
			tc.mutate(&m.SubagentRoutes[0])
			if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateSchema error = %v, want %s", err, tc.want)
			}
		})
	}
}

func TestValidateSchemaRejectsDuplicateSubagentRouteListItems(t *testing.T) {
	for _, tc := range []struct {
		name   string
		want   string
		mutate func(*SubagentRoute)
	}{
		{
			name:   "taskTypes",
			want:   "subagent route unit:bounded-review contains duplicate taskTypes item: candidate-review",
			mutate: func(route *SubagentRoute) { route.TaskTypes = "candidate-review,candidate-review" },
		},
		{
			name:   "mainAgentOwns",
			want:   "subagent route unit:bounded-review contains duplicate mainAgentOwns item: validation",
			mutate: func(route *SubagentRoute) { route.MainAgentOwns = "validation;handoff-update;validation" },
		},
		{
			name:   "outputContract",
			want:   "subagent route unit:bounded-review contains duplicate outputContract item: item",
			mutate: func(route *SubagentRoute) { route.OutputContract = "item,decision,item" },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifestFixture()
			m.SubagentRoutes = []SubagentRoute{{
				ID:                  "unit:bounded-review",
				TaskTypes:           "candidate-review",
				Trigger:             "fixed-boundary read-only review",
				ShardBasis:          "item",
				TargetItemsPerAgent: "1",
				MaxParallel:         "1",
				Reference:           "references/template/README.md",
				SubagentPermissions: "read-only",
				MainAgentOwns:       "validation",
				OutputContract:      "item,decision",
			}}
			tc.mutate(&m.SubagentRoutes[0])
			if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateSchema error = %v, want %s", err, tc.want)
			}
		})
	}
}

func TestValidateSchemaRequiresNonEmptyContractLists(t *testing.T) {
	for _, tc := range []struct {
		name   string
		want   string
		mutate func(*Manifest)
	}{
		{
			name:   "promoteFiles",
			want:   "promoteFiles must include at least one managed file",
			mutate: func(m *Manifest) { m.PromoteFiles = nil },
		},
		{
			name:   "toolingCandidateSources",
			want:   "toolingCandidateSources must include at least one source; implicit vmp-re fallback is not allowed",
			mutate: func(m *Manifest) { m.ToolingCandidateSources = nil },
		},
		{
			name:   "authorityFiles",
			want:   "authorityFiles must include at least one authority file",
			mutate: func(m *Manifest) { m.AuthorityFiles = nil },
		},
		{
			name:   "heavyToolGates",
			want:   "heavyToolGates must include at least one gate",
			mutate: func(m *Manifest) { m.HeavyToolGates = nil },
		},
		{
			name:   "laneTypes",
			want:   "laneTypes must include at least one lane type",
			mutate: func(m *Manifest) { m.LaneTypes = nil },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifestFixture()
			tc.mutate(&m)
			if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateSchema error = %v, want %s", err, tc.want)
			}
		})
	}
}

func TestValidateSchemaRequiresExplicitSyncPolicy(t *testing.T) {
	m := validManifestFixture()
	m.explicitMaps["syncPolicy"] = false
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "manifest must explicitly declare syncPolicy") {
		t.Fatalf("ValidateSchema error = %v, want explicit syncPolicy error", err)
	}
	m = validManifestFixture()
	delete(m.SyncPolicy, "templateFiles")
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "syncPolicy is missing required key: templateFiles") {
		t.Fatalf("ValidateSchema error = %v, want missing syncPolicy key error", err)
	}
	m = validManifestFixture()
	m.SyncPolicy["managedFiles"] = "overwrite"
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "syncPolicy.managedFiles has unsupported value: overwrite") {
		t.Fatalf("ValidateSchema error = %v, want unsupported syncPolicy value error", err)
	}
	m = validManifestFixture()
	m.SyncPolicy["promoteFiles"] = "review-first"
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "syncPolicy contains unsupported key: promoteFiles") {
		t.Fatalf("ValidateSchema error = %v, want unsupported syncPolicy key error", err)
	}
}

func TestLoadDoesNotInferSyncPolicyDefault(t *testing.T) {
	repo := t.TempDir()
	packRoot := filepath.Join(repo, "packs", "missing-sync-policy")
	if err := os.MkdirAll(packRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestText := `schemaVersion: 1
name: missing-sync-policy
version: 0.1.0
description: test pack skeleton
maturity: skeleton
managedFiles:
  - references/test/README.md
templateFiles: []
localNeverOverwrite: []
promoteFiles:
  - references/test/README.md
managedBlock:
  file: CLAUDE.local.md
  blockId: rekit:test
  source: CLAUDE.local.snippet.md
toolingCandidateSources:
  - references/test/toolchain-router.md
workstreamDefaults:
  defaultAuthorityLane: main
  defaultStartLaneType: feature
  backupRoot: .rekit/backups/sync
  requestDefaultTargetLane: main
authorityFiles:
  - references/test/README.md
commonPolicies: []
policyOverlays: []
subagentRoutes: []
toolingFiles: []
promptFiles: []
promoteDenyPatterns:
  - "artifacts[\\/]"
budgets:
  defaultMarkdown: 16384
heavyToolGates:
  - id: debug
    title: Debug
    sideEffects: debug,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: timeout
laneTypes:
  - id: main
    title: Main
    authority: true
    workspaceRoot: workspace/main
    canWrite: references/test/README.md
    readOnly: .rekit/facts/**
    outputs: publication
  - id: feature
    title: Feature
    authority: false
    workspaceRoot: workspace/features
    canWrite: own-workspace
    readOnly: references/test/**
    outputs: observation
`
	if err := os.WriteFile(filepath.Join(packRoot, "manifest.yml"), []byte(manifestText), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(repo, "missing-sync-policy")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.SyncPolicy) != 0 || m.explicitMaps["syncPolicy"] {
		t.Fatalf("SyncPolicy defaults = %v explicitMaps = %v, want no implicit syncPolicy", m.SyncPolicy, m.explicitMaps)
	}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "manifest must explicitly declare syncPolicy") {
		t.Fatalf("ValidateSchema error = %v, want explicit syncPolicy error", err)
	}
}

func TestLoadDoesNotInferRequiredListPresence(t *testing.T) {
	repo := t.TempDir()
	packRoot := filepath.Join(repo, "packs", "missing-required-lists")
	if err := os.MkdirAll(packRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestText := `schemaVersion: 1
name: missing-required-lists
version: 0.1.0
description: test pack skeleton
maturity: skeleton

managedFiles:
  - references/test/README.md
promoteFiles:
  - references/test/README.md
managedBlock:
  file: CLAUDE.local.md
  blockId: rekit:test
  source: CLAUDE.local.snippet.md
toolingCandidateSources:
  - references/test/toolchain-router.md
workstreamDefaults:
  defaultAuthorityLane: main
  defaultStartLaneType: feature
  backupRoot: .rekit/backups/sync
  requestDefaultTargetLane: main
authorityFiles:
  - references/test/README.md
commonPolicies: []
policyOverlays: []
subagentRoutes: []
toolingFiles: []
promptFiles: []
syncPolicy:
  managedFiles: overwrite-with-backup
  templateFiles: create-if-missing
  localFiles: never-overwrite
promoteDenyPatterns:
  - "artifacts[\\/]"
budgets:
  defaultMarkdown: 16384
heavyToolGates:
  - id: debug
    title: Debug
    sideEffects: debug,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: timeout
laneTypes:
  - id: main
    title: Main
    authority: true
    workspaceRoot: workspace/main
    canWrite: references/test/README.md
    readOnly: .rekit/facts/**
    outputs: publication
  - id: feature
    title: Feature
    authority: false
    workspaceRoot: workspace/features
    canWrite: own-workspace
    readOnly: references/test/**
    outputs: observation
`
	if err := os.WriteFile(filepath.Join(packRoot, "manifest.yml"), []byte(manifestText), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(repo, "missing-required-lists")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.TemplateFiles) != 0 || len(m.LocalFiles) != 0 || m.explicitLists["templateFiles"] || m.explicitLists["localNeverOverwrite"] {
		t.Fatalf("required list defaults = templateFiles %v localNeverOverwrite %v explicit %v, want no implicit presence", m.TemplateFiles, m.LocalFiles, m.explicitLists)
	}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "manifest must explicitly declare templateFiles") {
		t.Fatalf("ValidateSchema error = %v, want explicit templateFiles error", err)
	}
}

func TestLoadDoesNotInferOptionalListPresence(t *testing.T) {
	repo := t.TempDir()
	packRoot := filepath.Join(repo, "packs", "missing-optional-lists")
	if err := os.MkdirAll(packRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestText := `schemaVersion: 1
name: missing-optional-lists
version: 0.1.0
description: test pack skeleton
maturity: skeleton

managedFiles:
  - references/test/README.md
templateFiles: []
localNeverOverwrite: []
promoteFiles:
  - references/test/README.md
managedBlock:
  file: CLAUDE.local.md
  blockId: rekit:test
  source: CLAUDE.local.snippet.md
toolingCandidateSources:
  - references/test/toolchain-router.md
workstreamDefaults:
  defaultAuthorityLane: main
  defaultStartLaneType: feature
  backupRoot: .rekit/backups/sync
  requestDefaultTargetLane: main
authorityFiles:
  - references/test/README.md
policyOverlays: []
subagentRoutes: []
toolingFiles: []
promptFiles: []
syncPolicy:
  managedFiles: overwrite-with-backup
  templateFiles: create-if-missing
  localFiles: never-overwrite
promoteDenyPatterns:
  - "artifacts[\\/]"
budgets:
  defaultMarkdown: 16384
heavyToolGates:
  - id: debug
    title: Debug
    sideEffects: debug,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: timeout
laneTypes:
  - id: main
    title: Main
    authority: true
    workspaceRoot: workspace/main
    canWrite: references/test/README.md
    readOnly: .rekit/facts/**
    outputs: publication
  - id: feature
    title: Feature
    authority: false
    workspaceRoot: workspace/features
    canWrite: own-workspace
    readOnly: references/test/**
    outputs: observation
`
	if err := os.WriteFile(filepath.Join(packRoot, "manifest.yml"), []byte(manifestText), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(repo, "missing-optional-lists")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.CommonPolicies) != 0 || m.explicitLists["commonPolicies"] {
		t.Fatalf("CommonPolicies defaults = %v explicitLists = %v, want no implicit presence", m.CommonPolicies, m.explicitLists)
	}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "manifest must explicitly declare commonPolicies") {
		t.Fatalf("ValidateSchema error = %v, want explicit commonPolicies error", err)
	}
}

func TestLoadDoesNotInferPromoteFilesFromManagedFiles(t *testing.T) {
	repo := t.TempDir()
	packRoot := filepath.Join(repo, "packs", "missing-promote-files")
	if err := os.MkdirAll(packRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestText := `schemaVersion: 1
name: missing-promote-files
version: 0.1.0
description: test pack skeleton
maturity: skeleton

managedFiles:
  - references/test/README.md
templateFiles: []
localNeverOverwrite: []
managedBlock:
  file: CLAUDE.local.md
  blockId: rekit:test
  source: CLAUDE.local.snippet.md
toolingCandidateSources:
  - references/test/toolchain-router.md
workstreamDefaults:
  defaultAuthorityLane: main
  defaultStartLaneType: feature
  backupRoot: .rekit/backups/sync
  requestDefaultTargetLane: main
authorityFiles:
  - references/test/README.md
commonPolicies: []
policyOverlays: []
subagentRoutes: []
toolingFiles: []
promptFiles: []
syncPolicy:
  managedFiles: overwrite-with-backup
  templateFiles: create-if-missing
  localFiles: never-overwrite
promoteDenyPatterns:
  - "artifacts[\\/]"
heavyToolGates:
  - id: debug
    title: Debug
    sideEffects: debug,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: timeout
laneTypes:
  - id: main
    title: Main
    authority: true
    workspaceRoot: workspace/main
    canWrite: references/test/README.md
    readOnly: .rekit/facts/**
    outputs: publication
  - id: feature
    title: Feature
    authority: false
    workspaceRoot: workspace/features
    canWrite: own-workspace
    readOnly: references/test/**
    outputs: observation
`
	if err := os.WriteFile(filepath.Join(packRoot, "manifest.yml"), []byte(manifestText), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(repo, "missing-promote-files")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.PromoteFiles) != 0 {
		t.Fatalf("PromoteFiles = %v, want no implicit fallback", m.PromoteFiles)
	}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "manifest must explicitly declare promoteFiles") {
		t.Fatalf("ValidateSchema error = %v, want explicit promoteFiles error", err)
	}
}

func TestValidateSchemaRequiresNonEmptyPromoteDenyPatterns(t *testing.T) {
	m := validManifestFixture()
	m.PromoteDenyPatterns = nil
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "promoteDenyPatterns must include at least one pattern") {
		t.Fatalf("ValidateSchema error = %v, want non-empty promoteDenyPatterns error", err)
	}
}

func TestValidateSchemaRequiresValidSourceAuthorityAndDenyLists(t *testing.T) {
	for _, tc := range []struct {
		name   string
		want   string
		mutate func(*Manifest)
	}{
		{
			name:   "toolingCandidateSources empty path",
			want:   "toolingCandidateSources contains an empty path",
			mutate: func(m *Manifest) { m.ToolingCandidateSources = []string{""} },
		},
		{
			name: "toolingCandidateSources duplicate path",
			want: "toolingCandidateSources contains duplicate path: references/template/toolchain-router.md",
			mutate: func(m *Manifest) {
				m.ToolingCandidateSources = []string{"references/template/toolchain-router.md", "references/template/toolchain-router.md"}
			},
		},
		{
			name:   "authorityFiles empty path",
			want:   "authorityFiles contains an empty path",
			mutate: func(m *Manifest) { m.AuthorityFiles = []string{""} },
		},
		{
			name: "authorityFiles duplicate path",
			want: "authorityFiles contains duplicate path: references/template/task-handoff.md",
			mutate: func(m *Manifest) {
				m.AuthorityFiles = []string{"references/template/task-handoff.md", "references/template/task-handoff.md"}
			},
		},
		{
			name:   "promoteDenyPatterns empty pattern",
			want:   "promoteDenyPatterns contains an empty pattern",
			mutate: func(m *Manifest) { m.PromoteDenyPatterns = []string{""} },
		},
		{
			name:   "promoteDenyPatterns duplicate pattern",
			want:   "promoteDenyPatterns contains duplicate pattern: artifacts[\\/]",
			mutate: func(m *Manifest) { m.PromoteDenyPatterns = []string{"artifacts[\\/]", "artifacts[\\/]"} },
		},
		{
			name:   "promoteDenyPatterns invalid regex",
			want:   "invalid promoteDenyPatterns regex",
			mutate: func(m *Manifest) { m.PromoteDenyPatterns = []string{"["} },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifestFixture()
			tc.mutate(&m)
			if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateSchema error = %v, want %s", err, tc.want)
			}
		})
	}
}

func TestValidateSchemaRequiresExplicitDefaultBudget(t *testing.T) {
	m := validManifestFixture()
	m.explicitMaps["budgets"] = false
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "manifest must explicitly declare budgets") {
		t.Fatalf("ValidateSchema error = %v, want explicit budgets map error", err)
	}
	m = validManifestFixture()
	m.Budgets = nil
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "manifest must explicitly declare budgets.defaultMarkdown") {
		t.Fatalf("ValidateSchema error = %v, want explicit default budget error", err)
	}
	m = validManifestFixture()
	m.Budgets["defaultMarkdown"] = "0"
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "budgets.defaultMarkdown has invalid positive integer limit") {
		t.Fatalf("ValidateSchema error = %v, want invalid default budget error", err)
	}
	m = validManifestFixture()
	m.Budgets["../outside.md"] = "1024"
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "budgets contains unsafe path key") {
		t.Fatalf("ValidateSchema error = %v, want unsafe budget key error", err)
	}
}

func TestLoadDoesNotInferDefaultBudget(t *testing.T) {
	repo := t.TempDir()
	packRoot := filepath.Join(repo, "packs", "missing-budget")
	if err := os.MkdirAll(packRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestText := `schemaVersion: 1
name: missing-budget
version: 0.1.0
description: test pack skeleton
maturity: skeleton

managedFiles:
  - references/test/README.md
templateFiles: []
localNeverOverwrite: []
promoteFiles:
  - references/test/README.md
managedBlock:
  file: CLAUDE.local.md
  blockId: rekit:test
  source: CLAUDE.local.snippet.md
toolingCandidateSources:
  - references/test/toolchain-router.md
workstreamDefaults:
  defaultAuthorityLane: main
  defaultStartLaneType: feature
  backupRoot: .rekit/backups/sync
  requestDefaultTargetLane: main
authorityFiles:
  - references/test/README.md
commonPolicies: []
policyOverlays: []
subagentRoutes: []
toolingFiles: []
promptFiles: []
syncPolicy:
  managedFiles: overwrite-with-backup
  templateFiles: create-if-missing
  localFiles: never-overwrite
promoteDenyPatterns:
  - "artifacts[\\/]"
heavyToolGates:
  - id: debug
    title: Debug
    sideEffects: debug,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: timeout
laneTypes:
  - id: main
    title: Main
    authority: true
    workspaceRoot: workspace/main
    canWrite: references/test/README.md
    readOnly: .rekit/facts/**
    outputs: publication
  - id: feature
    title: Feature
    authority: false
    workspaceRoot: workspace/features
    canWrite: own-workspace
    readOnly: references/test/**
    outputs: observation
`
	if err := os.WriteFile(filepath.Join(packRoot, "manifest.yml"), []byte(manifestText), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(repo, "missing-budget")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(m.Budgets["defaultMarkdown"]); got != "" {
		t.Fatalf("Budgets[defaultMarkdown] = %q, want no implicit fallback", got)
	}
	if m.explicitMaps["budgets"] {
		t.Fatalf("explicitMaps[budgets] = true, want no implicit budgets map presence")
	}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "manifest must explicitly declare budgets") {
		t.Fatalf("ValidateSchema error = %v, want explicit budgets map error", err)
	}
	if got := m.BudgetLimit("references/test/README.md"); got != 16384 {
		t.Fatalf("BudgetLimit fallback = %d, want runtime safety fallback 16384", got)
	}
}

func TestValidateSchemaRequiresExplicitWorkstreamDefaults(t *testing.T) {
	m := validManifestFixture()
	m.explicitMaps["workstreamDefaults"] = false
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "manifest must explicitly declare workstreamDefaults") {
		t.Fatalf("ValidateSchema error = %v, want explicit workstreamDefaults error", err)
	}
	m = validManifestFixture()
	delete(m.WorkstreamDefaults, "backupRoot")
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "workstreamDefaults is missing required key: backupRoot") {
		t.Fatalf("ValidateSchema error = %v, want missing workstreamDefaults key error", err)
	}
	m = validManifestFixture()
	m.WorkstreamDefaults["defaultMergeLane"] = "main"
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "workstreamDefaults contains unsupported key: defaultMergeLane") {
		t.Fatalf("ValidateSchema error = %v, want unsupported workstreamDefaults key error", err)
	}
}

func TestValidateSchemaRequiresExplicitLaneTypeFields(t *testing.T) {
	m := validManifestFixture()
	m.LaneTypes[0].Title = ""
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "laneTypes entry main is missing title") {
		t.Fatalf("ValidateSchema error = %v, want explicit lane title error", err)
	}
	m = validManifestFixture()
	m.LaneTypes[0].WorkspaceRoot = ""
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "laneTypes entry main is missing workspaceRoot") {
		t.Fatalf("ValidateSchema error = %v, want explicit lane workspaceRoot error", err)
	}
	m = validManifestFixture()
	m.LaneTypes[0].CanWrite = nil
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "laneTypes entry main is missing canWrite") {
		t.Fatalf("ValidateSchema error = %v, want explicit lane canWrite error", err)
	}
	m = validManifestFixture()
	m.LaneTypes[0].explicitAuthority = ""
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "laneTypes entry main is missing authority") {
		t.Fatalf("ValidateSchema error = %v, want explicit lane authority error", err)
	}
	m = validManifestFixture()
	m.LaneTypes[0].explicitAuthority = "yes"
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "laneTypes entry main has invalid authority") {
		t.Fatalf("ValidateSchema error = %v, want invalid lane authority error", err)
	}
	m = validManifestFixture()
	m.LaneTypes[0].ID = "Main Lane"
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "laneTypes entry has invalid id: Main Lane") {
		t.Fatalf("ValidateSchema error = %v, want invalid lane id error", err)
	}
	m = validManifestFixture()
	m.LaneTypes[0].CanWrite = []string{"references/template/task-handoff.md", "references/template/task-handoff.md"}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "laneTypes entry main contains duplicate canWrite item: references/template/task-handoff.md") {
		t.Fatalf("ValidateSchema error = %v, want duplicate canWrite error", err)
	}
	m = validManifestFixture()
	m.LaneTypes[0].ReadOnly = []string{".rekit/facts/**", ".REKIT/facts/**"}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "laneTypes entry main contains duplicate readOnly item: .REKIT/facts/**") {
		t.Fatalf("ValidateSchema error = %v, want duplicate readOnly error", err)
	}
	m = validManifestFixture()
	m.LaneTypes[0].Outputs = []string{"publication", "AuthoritySync"}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "laneTypes entry main has invalid outputs item: AuthoritySync") {
		t.Fatalf("ValidateSchema error = %v, want invalid outputs error", err)
	}
	m = validManifestFixture()
	m.LaneTypes[0].Outputs = []string{"publication", "Publication"}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "laneTypes entry main has invalid outputs item: Publication") {
		t.Fatalf("ValidateSchema error = %v, want invalid outputs error", err)
	}
	m = validManifestFixture()
	m.LaneTypes[0].Outputs = []string{"publication", "publication"}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "laneTypes entry main contains duplicate outputs item: publication") {
		t.Fatalf("ValidateSchema error = %v, want duplicate outputs error", err)
	}
}

func TestLoadDoesNotInferWorkstreamDefaultsMap(t *testing.T) {
	repo := t.TempDir()
	packRoot := filepath.Join(repo, "packs", "missing-workstream-defaults")
	if err := os.MkdirAll(packRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestText := `schemaVersion: 1
name: missing-workstream-defaults
version: 0.1.0
description: test pack skeleton
maturity: skeleton
managedFiles:
  - references/test/README.md
templateFiles: []
localNeverOverwrite: []
promoteFiles:
  - references/test/README.md
managedBlock:
  file: CLAUDE.local.md
  blockId: rekit:test
  source: CLAUDE.local.snippet.md
toolingCandidateSources:
  - references/test/toolchain-router.md
authorityFiles:
  - references/test/README.md
commonPolicies: []
policyOverlays: []
subagentRoutes: []
toolingFiles: []
promptFiles: []
syncPolicy:
  managedFiles: overwrite-with-backup
  templateFiles: create-if-missing
  localFiles: never-overwrite
promoteDenyPatterns:
  - "artifacts[\\/]"
budgets:
  defaultMarkdown: 16384
heavyToolGates:
  - id: debug
    title: Debug
    sideEffects: debug,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: timeout
laneTypes:
  - id: main
    title: Main
    authority: true
    workspaceRoot: workspace/main
    canWrite: references/test/README.md
    readOnly: .rekit/facts/**
    outputs: publication
  - id: feature
    title: Feature
    authority: false
    workspaceRoot: workspace/features
    canWrite: own-workspace
    readOnly: references/test/**
    outputs: observation
`
	if err := os.WriteFile(filepath.Join(packRoot, "manifest.yml"), []byte(manifestText), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(repo, "missing-workstream-defaults")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.WorkstreamDefaults) != 0 || m.explicitMaps["workstreamDefaults"] {
		t.Fatalf("WorkstreamDefaults defaults = %v explicitMaps = %v, want no implicit map presence", m.WorkstreamDefaults, m.explicitMaps)
	}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "manifest must explicitly declare workstreamDefaults") {
		t.Fatalf("ValidateSchema error = %v, want explicit workstreamDefaults map error", err)
	}
}

func TestLoadDoesNotInferLaneTypeDefaults(t *testing.T) {
	repo := t.TempDir()
	packRoot := filepath.Join(repo, "packs", "missing-lane-defaults")
	if err := os.MkdirAll(packRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestText := `schemaVersion: 1
name: missing-lane-defaults
version: 0.1.0
description: test pack skeleton
maturity: skeleton

managedFiles:
  - references/test/README.md
templateFiles: []
localNeverOverwrite: []
promoteFiles:
  - references/test/README.md
managedBlock:
  file: CLAUDE.local.md
  blockId: rekit:test
  source: CLAUDE.local.snippet.md
toolingCandidateSources:
  - references/test/toolchain-router.md
workstreamDefaults:
  defaultAuthorityLane: main
  defaultStartLaneType: feature
  backupRoot: .rekit/backups/sync
  requestDefaultTargetLane: main
authorityFiles:
  - references/test/README.md
commonPolicies: []
policyOverlays: []
subagentRoutes: []
toolingFiles: []
promptFiles: []
syncPolicy:
  managedFiles: overwrite-with-backup
  templateFiles: create-if-missing
  localFiles: never-overwrite
promoteDenyPatterns:
  - "artifacts[\\/]"
budgets:
  defaultMarkdown: 16384
heavyToolGates:
  - id: debug
    title: Debug
    sideEffects: debug,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: timeout
laneTypes:
  - id: main
    authority: true
    canWrite: references/test/README.md
    readOnly: .rekit/facts/**
    outputs: publication
  - id: feature
    title: Feature
    authority: false
    workspaceRoot: workspace/features
    canWrite: own-workspace
    readOnly: references/test/**
    outputs: observation
`
	if err := os.WriteFile(filepath.Join(packRoot, "manifest.yml"), []byte(manifestText), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(repo, "missing-lane-defaults")
	if err != nil {
		t.Fatal(err)
	}
	lane, err := m.LaneType("main")
	if err != nil {
		t.Fatal(err)
	}
	if lane.Title != "" || lane.WorkspaceRoot != "" {
		t.Fatalf("lane defaults = title %q workspaceRoot %q, want no implicit fallback", lane.Title, lane.WorkspaceRoot)
	}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "laneTypes entry main is missing title") {
		t.Fatalf("ValidateSchema error = %v, want explicit lane title error", err)
	}
}

func TestLoadDoesNotInferLaneAuthorityDefault(t *testing.T) {
	repo := t.TempDir()
	packRoot := filepath.Join(repo, "packs", "missing-lane-authority")
	if err := os.MkdirAll(packRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestText := `schemaVersion: 1
name: missing-lane-authority
version: 0.1.0
description: test pack skeleton
maturity: skeleton

managedFiles:
  - references/test/README.md
templateFiles: []
localNeverOverwrite: []
promoteFiles:
  - references/test/README.md
managedBlock:
  file: CLAUDE.local.md
  blockId: rekit:test
  source: CLAUDE.local.snippet.md
toolingCandidateSources:
  - references/test/toolchain-router.md
workstreamDefaults:
  defaultAuthorityLane: main
  defaultStartLaneType: feature
  backupRoot: .rekit/backups/sync
  requestDefaultTargetLane: main
authorityFiles:
  - references/test/README.md
commonPolicies: []
policyOverlays: []
subagentRoutes: []
toolingFiles: []
promptFiles: []
syncPolicy:
  managedFiles: overwrite-with-backup
  templateFiles: create-if-missing
  localFiles: never-overwrite
promoteDenyPatterns:
  - "artifacts[\\/]"
budgets:
  defaultMarkdown: 16384
heavyToolGates:
  - id: debug
    title: Debug
    sideEffects: debug,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: timeout
laneTypes:
  - id: main
    title: Main
    workspaceRoot: workspace/main
    canWrite: references/test/README.md
    readOnly: .rekit/facts/**
    outputs: publication
  - id: feature
    title: Feature
    authority: false
    workspaceRoot: workspace/features
    canWrite: own-workspace
    readOnly: references/test/**
    outputs: observation
`
	if err := os.WriteFile(filepath.Join(packRoot, "manifest.yml"), []byte(manifestText), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(repo, "missing-lane-authority")
	if err != nil {
		t.Fatal(err)
	}
	lane, err := m.LaneType("main")
	if err != nil {
		t.Fatal(err)
	}
	if lane.Authority || strings.TrimSpace(lane.explicitAuthority) != "" {
		t.Fatalf("lane authority = %t explicit %q, want no implicit fallback", lane.Authority, lane.explicitAuthority)
	}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "laneTypes entry main is missing authority") {
		t.Fatalf("ValidateSchema error = %v, want explicit lane authority error", err)
	}
}

func TestValidateSchemaRequiresSupportedMaturity(t *testing.T) {
	m := validManifestFixture()
	m.Maturity = "preview"
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "maturity has unsupported value") {
		t.Fatalf("ValidateSchema error = %v, want unsupported maturity error", err)
	}
	m = validManifestFixture()
	m.Maturity = ""
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "maturity is missing") {
		t.Fatalf("ValidateSchema error = %v, want missing maturity error", err)
	}
}

func TestListMarksMissingAndInvalidMaturity(t *testing.T) {
	repo := t.TempDir()
	cases := []struct {
		pack         string
		maturityLine string
		wantMaturity string
		wantError    string
	}{
		{pack: "missing-maturity", wantMaturity: "missing", wantError: "maturity is missing"},
		{pack: "invalid-maturity", maturityLine: "maturity: preview", wantMaturity: "preview", wantError: "maturity has unsupported value"},
	}
	for _, tc := range cases {
		packRoot := filepath.Join(repo, "packs", tc.pack)
		if err := os.MkdirAll(packRoot, 0o755); err != nil {
			t.Fatal(err)
		}
		manifestText := "schemaVersion: 1\nname: " + tc.pack + "\nversion: 0.1.0\ndescription: maturity test pack\n"
		if tc.maturityLine != "" {
			manifestText += tc.maturityLine + "\n"
		}
		manifestText += "\nmanagedFiles:\n  - references/test/README.md\ntemplateFiles: []\nlocalNeverOverwrite: []\n"
		if err := os.WriteFile(filepath.Join(packRoot, "manifest.yml"), []byte(manifestText), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	packs, err := List(repo)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]PackSummary{}
	for _, pack := range packs {
		byID[pack.ID] = pack
	}
	for _, tc := range cases {
		pack := byID[tc.pack]
		if pack.SchemaValid {
			t.Fatalf("%s SchemaValid = true, want false: %+v", tc.pack, pack)
		}
		if pack.Maturity != tc.wantMaturity {
			t.Fatalf("%s Maturity = %q, want %q", tc.pack, pack.Maturity, tc.wantMaturity)
		}
		if !strings.Contains(pack.Error, tc.wantError) {
			t.Fatalf("%s Error = %q, want contains %q", tc.pack, pack.Error, tc.wantError)
		}
	}
}

func TestListPackSummaries(t *testing.T) {
	packs, err := List(repoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(packs) < 3 {
		t.Fatalf("pack count = %d, want at least 3: %+v", len(packs), packs)
	}
	byID := map[string]PackSummary{}
	for _, pack := range packs {
		byID[pack.ID] = pack
		if !pack.SchemaValid {
			t.Fatalf("pack summary schema invalid: %+v", pack)
		}
	}
	if byID["_template"].Maturity != "template" || byID["_template"].SchemaVersion != "1" || byID["_template"].SubagentRoutes != 2 || byID["_template"].HeavyToolGates != 7 {
		t.Fatalf("unexpected template summary: %+v", byID["_template"])
	}
	if byID["vmp-re"].Maturity != "mature" || byID["vmp-re"].SchemaVersion != "1" || byID["vmp-re"].DefaultAuthorityLane != "devirt-main" || byID["vmp-re"].HeavyToolGates != 7 {
		t.Fatalf("unexpected vmp summary: %+v", byID["vmp-re"])
	}
	if byID["web-security"].Maturity != "skeleton" || byID["web-security"].SchemaVersion != "1" || byID["web-security"].DefaultAuthorityLane != "main" || byID["web-security"].HeavyToolGates != 7 {
		t.Fatalf("unexpected web-security summary: %+v", byID["web-security"])
	}
}

func TestValidateSchemaRequiresExplicitManagedBlock(t *testing.T) {
	m := validManifestFixture()
	delete(m.explicitManagedBlock, "source")
	delete(m.ManagedBlock, "source")
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "managedBlock is missing required key: source") {
		t.Fatalf("ValidateSchema error = %v, want explicit managedBlock source error", err)
	}
	m = validManifestFixture()
	m.explicitManagedBlock["marker"] = "custom"
	m.ManagedBlock["marker"] = "custom"
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "managedBlock contains unsupported key: marker") {
		t.Fatalf("ValidateSchema error = %v, want unsupported managedBlock key error", err)
	}
	m = validManifestFixture()
	m.explicitManagedBlock["blockId"] = "router"
	m.ManagedBlock["blockId"] = "router"
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "managedBlock.blockId has invalid value: router") {
		t.Fatalf("ValidateSchema error = %v, want invalid managedBlock blockId error", err)
	}
}

func TestLoadDoesNotInferManifestIdentityDefaults(t *testing.T) {
	repo := t.TempDir()
	packRoot := filepath.Join(repo, "packs", "missing-identity")
	if err := os.MkdirAll(packRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestText := `schemaVersion: 1
maturity: skeleton

managedFiles:
  - references/test/README.md
templateFiles: []
localNeverOverwrite: []
promoteFiles:
  - references/test/README.md
managedBlock:
  file: CLAUDE.local.md
  blockId: rekit:test
  source: CLAUDE.local.snippet.md
toolingCandidateSources:
  - references/test/toolchain-router.md
workstreamDefaults:
  defaultAuthorityLane: main
  defaultStartLaneType: feature
  backupRoot: .rekit/backups/sync
  requestDefaultTargetLane: main
authorityFiles:
  - references/test/README.md
commonPolicies: []
policyOverlays: []
subagentRoutes: []
toolingFiles: []
promptFiles: []
syncPolicy:
  managedFiles: overwrite-with-backup
  templateFiles: create-if-missing
  localFiles: never-overwrite
promoteDenyPatterns:
  - "artifacts[\\/]"
budgets:
  defaultMarkdown: 16384
heavyToolGates:
  - id: debug
    title: Debug
    sideEffects: debug,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: timeout
laneTypes:
  - id: main
    title: Main
    authority: true
    workspaceRoot: workspace/main
    canWrite: references/test/README.md
    readOnly: .rekit/facts/**
    outputs: publication
  - id: feature
    title: Feature
    authority: false
    workspaceRoot: workspace/features
    canWrite: own-workspace
    readOnly: references/test/**
    outputs: observation
`
	if err := os.WriteFile(filepath.Join(packRoot, "manifest.yml"), []byte(manifestText), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(repo, "missing-identity")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(m.Name) != "" || strings.TrimSpace(m.Version) != "" {
		t.Fatalf("identity defaults = name %q version %q, want no implicit fallback", m.Name, m.Version)
	}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "name is missing") {
		t.Fatalf("ValidateSchema error = %v, want missing name error", err)
	}
}

func TestLoadKeepsMissingDescriptionEmpty(t *testing.T) {
	repo := t.TempDir()
	packRoot := filepath.Join(repo, "packs", "missing-description")
	if err := os.MkdirAll(packRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestText := `schemaVersion: 1
name: missing-description
version: 0.1.0
maturity: skeleton

managedFiles:
  - references/test/README.md
templateFiles: []
localNeverOverwrite: []
promoteFiles:
  - references/test/README.md
managedBlock:
  file: CLAUDE.local.md
  blockId: rekit:test
  source: CLAUDE.local.snippet.md
toolingCandidateSources:
  - references/test/toolchain-router.md
workstreamDefaults:
  defaultAuthorityLane: main
  defaultStartLaneType: feature
  backupRoot: .rekit/backups/sync
  requestDefaultTargetLane: main
authorityFiles:
  - references/test/README.md
commonPolicies: []
policyOverlays: []
subagentRoutes: []
toolingFiles: []
promptFiles: []
syncPolicy:
  managedFiles: overwrite-with-backup
  templateFiles: create-if-missing
  localFiles: never-overwrite
promoteDenyPatterns:
  - "artifacts[\\/]"
budgets:
  defaultMarkdown: 16384
heavyToolGates:
  - id: debug
    title: Debug
    sideEffects: debug,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: timeout
laneTypes:
  - id: main
    title: Main
    authority: true
    workspaceRoot: workspace/main
    canWrite: references/test/README.md
    readOnly: .rekit/facts/**
    outputs: publication
  - id: feature
    title: Feature
    authority: false
    workspaceRoot: workspace/features
    canWrite: own-workspace
    readOnly: references/test/**
    outputs: observation
`
	if err := os.WriteFile(filepath.Join(packRoot, "manifest.yml"), []byte(manifestText), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(repo, "missing-description")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(m.Description) != "" {
		t.Fatalf("Description = %q, want no implicit fallback", m.Description)
	}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "description is missing") {
		t.Fatalf("ValidateSchema error = %v, want missing description error", err)
	}
}

func TestLoadDoesNotInferHeavyToolRequiresConfirmationDefault(t *testing.T) {
	repo := t.TempDir()
	packRoot := filepath.Join(repo, "packs", "missing-gate-confirmation")
	if err := os.MkdirAll(packRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestText := `schemaVersion: 1
name: missing-gate-confirmation
version: 0.1.0
description: test pack skeleton
maturity: skeleton

managedFiles:
  - references/test/README.md
templateFiles: []
localNeverOverwrite: []
promoteFiles:
  - references/test/README.md
managedBlock:
  file: CLAUDE.local.md
  blockId: rekit:test
  source: CLAUDE.local.snippet.md
toolingCandidateSources:
  - references/test/toolchain-router.md
workstreamDefaults:
  defaultAuthorityLane: main
  defaultStartLaneType: feature
  backupRoot: .rekit/backups/sync
  requestDefaultTargetLane: main
authorityFiles:
  - references/test/README.md
commonPolicies: []
policyOverlays: []
subagentRoutes: []
toolingFiles: []
promptFiles: []
syncPolicy:
  managedFiles: overwrite-with-backup
  templateFiles: create-if-missing
  localFiles: never-overwrite
promoteDenyPatterns:
  - "artifacts[\\/]"
budgets:
  defaultMarkdown: 16384
heavyToolGates:
  - id: debug
    title: Debug
    sideEffects: debug,filesystem-write
    defaultRisk: high
    stopConditions: timeout
laneTypes:
  - id: main
    title: Main
    authority: true
    workspaceRoot: workspace/main
    canWrite: references/test/README.md
    readOnly: .rekit/facts/**
    outputs: publication
  - id: feature
    title: Feature
    authority: false
    workspaceRoot: workspace/features
    canWrite: own-workspace
    readOnly: references/test/**
    outputs: observation
`
	if err := os.WriteFile(filepath.Join(packRoot, "manifest.yml"), []byte(manifestText), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(repo, "missing-gate-confirmation")
	if err != nil {
		t.Fatal(err)
	}
	gate, ok := m.HeavyToolGate("debug")
	if !ok {
		t.Fatal("missing debug gate")
	}
	if gate.RequiresConfirmation || strings.TrimSpace(gate.explicitRequiresConfirmation) != "" {
		t.Fatalf("requiresConfirmation = %t explicit %q, want no implicit fallback", gate.RequiresConfirmation, gate.explicitRequiresConfirmation)
	}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "heavyToolGates entry debug is missing requiresConfirmation") {
		t.Fatalf("ValidateSchema error = %v, want explicit requiresConfirmation error", err)
	}
}

func TestLoadDoesNotInferManagedBlockDefaults(t *testing.T) {
	repo := t.TempDir()
	packRoot := filepath.Join(repo, "packs", "missing-managed-block-source")
	if err := os.MkdirAll(packRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestText := `schemaVersion: 1
name: missing-managed-block-source
version: 0.1.0
description: test pack skeleton
maturity: skeleton

managedFiles:
  - references/test/README.md
templateFiles: []
localNeverOverwrite: []
promoteFiles:
  - references/test/README.md
managedBlock:
  file: CLAUDE.local.md
  blockId: rekit:test
toolingCandidateSources:
  - references/test/toolchain-router.md
workstreamDefaults:
  defaultAuthorityLane: main
  defaultStartLaneType: feature
  backupRoot: .rekit/backups/sync
  requestDefaultTargetLane: main
authorityFiles:
  - references/test/README.md
commonPolicies: []
policyOverlays: []
subagentRoutes: []
toolingFiles: []
promptFiles: []
syncPolicy:
  managedFiles: overwrite-with-backup
  templateFiles: create-if-missing
  localFiles: never-overwrite
promoteDenyPatterns:
  - "artifacts[\\/]"
budgets:
  defaultMarkdown: 16384
heavyToolGates:
  - id: debug
    title: Debug
    sideEffects: debug,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: timeout
laneTypes:
  - id: main
    title: Main
    authority: true
    workspaceRoot: workspace/main
    canWrite: references/test/README.md
    readOnly: .rekit/facts/**
    outputs: publication
  - id: feature
    title: Feature
    authority: false
    workspaceRoot: workspace/features
    canWrite: own-workspace
    readOnly: references/test/**
    outputs: observation
`
	if err := os.WriteFile(filepath.Join(packRoot, "manifest.yml"), []byte(manifestText), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(repo, "missing-managed-block-source")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(m.ManagedBlock["source"]); got != "" {
		t.Fatalf("ManagedBlock[source] = %q, want no implicit fallback", got)
	}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "managedBlock is missing required key: source") {
		t.Fatalf("ValidateSchema error = %v, want explicit managedBlock source error", err)
	}
}

func TestLoadRejectsInvalidPackID(t *testing.T) {
	for _, pack := range []string{"../vmp-re", "vmp-re/../web-security", `C:\\case`} {
		if _, err := Load(repoRoot(t), pack); err == nil {
			t.Fatalf("Load accepted invalid pack id %q", pack)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
