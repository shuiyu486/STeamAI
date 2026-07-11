package manifest

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func TestLoadVMPManifestSchema(t *testing.T) {
	m, err := Load(repoRoot(t), "vmp-re")
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
	if byID["_template"].Maturity != "template" || byID["_template"].SubagentRoutes != 2 {
		t.Fatalf("unexpected template summary: %+v", byID["_template"])
	}
	if byID["vmp-re"].Maturity != "mature" || byID["vmp-re"].DefaultAuthorityLane != "devirt-main" {
		t.Fatalf("unexpected vmp summary: %+v", byID["vmp-re"])
	}
	if byID["web-security"].Maturity != "skeleton" || byID["web-security"].DefaultAuthorityLane != "main" {
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
