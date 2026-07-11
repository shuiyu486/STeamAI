package manifest

import (
	"path/filepath"
	"regexp"
	"runtime"
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

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
