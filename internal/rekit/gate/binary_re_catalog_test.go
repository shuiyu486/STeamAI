package gate

import (
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
)

func TestBinaryREProductionCatalogHasExactCompiledInspectAdapters(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve repository root")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	m, err := manifest.Load(repoRoot, "binary-re")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ValidateSchema(); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(m.ToolingFiles, "tooling/catalog.yml") || !slices.Contains(m.ToolingFiles, "tooling/schemas/binary-inventory-v1.schema.json") {
		t.Fatalf("binary-re manifest omitted production catalog or typed inventory schema: %+v", m.ToolingFiles)
	}
	candidates, err := strictAdapterToolCandidates(m, EventPreview{Gate: GateDetails{Action: "inspect"}})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string][]AdapterToolCandidate{}
	for _, candidate := range candidates {
		byID[candidate.ID] = append(byID[candidate.ID], candidate)
	}
	if len(candidates) != 2 || len(byID["static-binary-triage-sidecar"]) != 1 || len(byID["vmp-ida-index-inspector"]) != 1 {
		t.Fatalf("binary-re inspect candidates are not the exact compiled pair: %+v", candidates)
	}
	for _, id := range []string{"static-binary-triage-sidecar", "vmp-ida-index-inspector"} {
		candidate := byID[id][0]
		if candidate.Status != "supported" || candidate.ToolingCatalogPath != "tooling/catalog.yml" || !slices.Contains(candidate.GateActions, "inspect") || !candidate.RecordOnlyAfterGate {
			t.Fatalf("production adapter candidate %s is not supported and dispatch-bound: %+v", id, candidate)
		}
		if !strings.Contains(candidate.Entry, "catalog entry is provenance only and is never executed") || strings.Contains(strings.ToLower(candidate.Purpose), "tooling fixture") {
			t.Fatalf("production adapter candidate %s entry/purpose is unsafe: %+v", id, candidate)
		}
	}
	if sidecarAdapterID(adapterToolCandidates(m, EventPreview{Gate: GateDetails{Action: "inspect"}})) != "<adapter-id>" {
		t.Fatal("production inspect candidates regained implicit catalog-order selection")
	}
}
