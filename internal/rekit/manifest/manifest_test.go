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
		ManifestPath:            "unit-manifest.yml",
		Maturity:                "template",
		ManagedFiles:            []string{"references/template/README.md"},
		PromoteFiles:            []string{"references/template/README.md"},
		ManagedBlock:            map[string]string{"file": "CLAUDE.local.md", "blockId": "rekit:router", "source": "CLAUDE.local.snippet.md"},
		explicitManagedBlock:    map[string]string{"file": "CLAUDE.local.md", "blockId": "rekit:router", "source": "CLAUDE.local.snippet.md"},
		ToolingCandidateSources: []string{"references/template/toolchain-router.md"},
		WorkstreamDefaults:      map[string]string{"defaultAuthorityLane": "main", "defaultStartLaneType": "feature", "backupRoot": ".rekit/backups/sync", "requestDefaultTargetLane": "main"},
		AuthorityFiles:          []string{"references/template/task-handoff.md"},
		SyncPolicy:              map[string]string{"managedFiles": "overwrite-with-backup", "templateFiles": "create-if-missing", "localFiles": "never-overwrite"},
		PromoteDenyPatterns:     []string{"artifacts[\\/]"},
		HeavyToolGates:          []HeavyToolGate{{ID: "debug", Title: "Debug", SideEffects: []string{"debug", "filesystem-write"}, DefaultRisk: "high", RequiresConfirmation: true, StopConditions: []string{"timeout"}}},
		LaneTypes: []LaneType{
			{ID: "main", Authority: true, CanWrite: []string{"references/template/task-handoff.md"}},
			{ID: "feature"},
		},
	}
}

func TestPackSummaryUsesExplicitMaturity(t *testing.T) {
	m := &Manifest{
		Pack:               "plain-pack",
		Maturity:           "experimental",
		Description:        "plain security pack",
		WorkstreamDefaults: map[string]string{},
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
	invalid := base
	invalid.HeavyToolGates = []HeavyToolGate{{ID: "debug", Title: "Debug", SideEffects: []string{"filesystem-write"}, DefaultRisk: "high", RequiresConfirmation: true, StopConditions: []string{"timeout"}}}
	if err := invalid.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "sideEffects must include") {
		t.Fatalf("ValidateSchema error = %v, want sideEffects include id error", err)
	}
	valid := base
	valid.HeavyToolGates = []HeavyToolGate{{ID: "debug", Title: "Debug", SideEffects: []string{"debug", "filesystem-write"}, DefaultRisk: "high", RequiresConfirmation: true, StopConditions: []string{"timeout"}}}
	if err := valid.ValidateSchema(); err != nil {
		t.Fatalf("ValidateSchema valid heavyToolGates error = %v", err)
	}
}

func TestValidateSchemaRequiresExplicitPromoteFiles(t *testing.T) {
	m := validManifestFixture()
	m.PromoteFiles = nil
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "manifest must explicitly declare promoteFiles") {
		t.Fatalf("ValidateSchema error = %v, want explicit promoteFiles error", err)
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

func TestValidateSchemaRequiresExplicitPromoteDenyPatterns(t *testing.T) {
	m := validManifestFixture()
	m.PromoteDenyPatterns = nil
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "manifest must explicitly declare promoteDenyPatterns") {
		t.Fatalf("ValidateSchema error = %v, want explicit promoteDenyPatterns error", err)
	}
}

func TestValidateSchemaRequiresSupportedMaturity(t *testing.T) {
	m := &Manifest{Maturity: "preview"}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "maturity has unsupported value") {
		t.Fatalf("ValidateSchema error = %v, want unsupported maturity error", err)
	}
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
		manifestText += "\nmanagedFiles:\n  - references/test/README.md\n"
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
	if byID["_template"].Maturity != "template" || byID["_template"].SubagentRoutes != 2 || byID["_template"].HeavyToolGates != 7 {
		t.Fatalf("unexpected template summary: %+v", byID["_template"])
	}
	if byID["vmp-re"].Maturity != "mature" || byID["vmp-re"].DefaultAuthorityLane != "devirt-main" || byID["vmp-re"].HeavyToolGates != 7 {
		t.Fatalf("unexpected vmp summary: %+v", byID["vmp-re"])
	}
	if byID["web-security"].Maturity != "skeleton" || byID["web-security"].DefaultAuthorityLane != "main" || byID["web-security"].HeavyToolGates != 7 {
		t.Fatalf("unexpected web-security summary: %+v", byID["web-security"])
	}
}

func TestValidateSchemaRequiresExplicitManagedBlock(t *testing.T) {
	repo := t.TempDir()
	packRoot := filepath.Join(repo, "packs", "missing-block")
	if err := os.MkdirAll(packRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestText := "schemaVersion: 1\nname: missing-block\nversion: 0.1.0\ndescription: test pack skeleton\nmaturity: skeleton\n\nmanagedFiles:\n  - references/test/README.md\n"
	if err := os.WriteFile(filepath.Join(packRoot, "manifest.yml"), []byte(manifestText), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(repo, "missing-block")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ValidateSchema(); err == nil || !strings.Contains(err.Error(), "managedBlock is missing required key") {
		t.Fatalf("ValidateSchema error = %v, want explicit managedBlock error", err)
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
