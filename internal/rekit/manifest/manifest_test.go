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
	manifestText := "schemaVersion: 1\nname: missing-block\nversion: 0.1.0\ndescription: test pack skeleton\n\nmanagedFiles:\n  - references/test/README.md\n"
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
