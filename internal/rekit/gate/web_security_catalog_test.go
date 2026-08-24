package gate

import (
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
)

func TestWebSecurityProductionCatalogHasExactCompiledAdapters(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve repository root")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	m, err := manifest.Load(repoRoot, "web-security")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ValidateSchema(); err != nil {
		t.Fatal(err)
	}
	for _, schema := range []string{
		"tooling/schemas/openapi-inventory-v1.schema.json",
		"tooling/schemas/bounded-replay-request-v1.schema.json",
		"tooling/schemas/bounded-replay-result-v1.schema.json",
	} {
		if !slices.Contains(m.ToolingFiles, schema) {
			t.Fatalf("web-security manifest omitted production schema %s: %+v", schema, m.ToolingFiles)
		}
	}
	for _, test := range []struct {
		action string
		id     string
	}{
		{action: "inspect", id: "openapi-v3-json-inventory"},
		{action: "network", id: "bounded-http-replay"},
	} {
		candidates, err := strictAdapterToolCandidates(m, EventPreview{Gate: GateDetails{Action: test.action}})
		if err != nil {
			t.Fatal(err)
		}
		if len(candidates) != 1 || candidates[0].ID != test.id {
			t.Fatalf("web-security %s candidates are not exact: %+v", test.action, candidates)
		}
		candidate := candidates[0]
		if candidate.Status != "supported" || candidate.ToolingCatalogPath != "tooling/catalog.yml" || !slices.Contains(candidate.GateActions, test.action) || !candidate.RecordOnlyAfterGate {
			t.Fatalf("web-security adapter candidate is not supported and dispatch-bound: %+v", candidate)
		}
		if !strings.Contains(candidate.Entry, "catalog entry is provenance only and is never executed") {
			t.Fatalf("web-security candidate entry is unsafe: %+v", candidate)
		}
		if sidecarAdapterID(adapterToolCandidates(m, EventPreview{Gate: GateDetails{Action: test.action}})) != test.id {
			t.Fatalf("web-security %s did not select its unique exact adapter", test.action)
		}
	}
}
