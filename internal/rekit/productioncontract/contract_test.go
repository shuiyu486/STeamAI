package productioncontract

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
)

func TestProductionRegistryMatchesMaturePacksAndCompiledAdapters(t *testing.T) {
	repoRoot := productionContractRepoRoot(t)
	summaries, err := manifest.List(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	registryAdmission := BuildRegistryAdmission(summaries)
	if !registryAdmission.Ready {
		t.Fatalf("production registry admission is not ready: %v", registryAdmission.Warnings)
	}
	for _, summary := range summaries {
		if summary.Maturity != "mature" {
			continue
		}
		admission := BuildAdmission(repoRoot, summary)
		if !admission.Ready {
			t.Fatalf("production pack %s admission is not ready: %v", summary.ID, admission.Warnings)
		}
		if admission.MaturitySource != "manifest-declared" || admission.ReadyMeaning != "repository-contract-inventory" || admission.FixtureClass != "synthetic-repository-fixture" || admission.RealClaudeReceiptObserved || admission.RealTargetToolReceiptObserved || admission.ReceiptKindMeaning != "expected-instruction-consumption-receipt-kind" {
			t.Fatalf("production pack %s admission overstates its evidence meaning: %+v", summary.ID, admission)
		}
	}
}

func TestEnabledSpecialtiesMatchSupportedCompiledCatalogEntries(t *testing.T) {
	repoRoot := productionContractRepoRoot(t)
	for pack, want := range map[string][]string{
		"binary-re":    {"static-binary-triage-sidecar", "vmp-ida-index-inspector"},
		"web-security": {"bounded-http-replay", "openapi-v3-json-inventory"},
	} {
		t.Run(pack, func(t *testing.T) {
			got, err := EnabledSpecialties(repoRoot, pack)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, want) {
				t.Fatalf("enabled specialties = %v, want %v", got, want)
			}
		})
	}
}

func TestEnabledSpecialtiesRejectContractCatalogDrift(t *testing.T) {
	repoRoot := productionContractRepoRoot(t)
	for _, test := range []struct {
		name string
		ids  []string
		want string
	}{
		{name: "missing supported adapter", ids: []string{"static-binary-triage-sidecar"}, want: "executable pack ownership"},
		{name: "cross-pack adapter", ids: []string{"bounded-http-replay", "openapi-v3-json-inventory"}, want: "executable pack ownership"},
		{name: "empty contract", ids: []string{}, want: "empty or duplicated"},
		{name: "duplicate contract", ids: []string{"static-binary-triage-sidecar", "static-binary-triage-sidecar"}, want: "empty or duplicated"},
	} {
		t.Run(test.name, func(t *testing.T) {
			withProductionRegistry(t, Registry())
			registry[0].AdapterIDs = append([]string{}, test.ids...)
			if _, err := EnabledSpecialties(repoRoot, "binary-re"); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("enabled specialties accepted contract/catalog drift: %v", err)
			}
		})
	}
}

func TestEnabledSpecialtiesAndAdmissionRejectCatalogDrift(t *testing.T) {
	repoRoot := productionContractRepoRoot(t)
	for _, test := range []struct {
		name           string
		mutateManifest func(string) string
		mutateCatalog  func(string) string
		want           string
	}{
		{
			name: "supported set drift",
			mutateCatalog: func(text string) string {
				text = strings.ReplaceAll(text, "\r\n", "\n")
				return strings.Replace(text, "  - id: vmp-ida-index-inspector\n    status: supported", "  - id: vmp-ida-index-inspector\n    status: auxiliary", 1)
			},
			want: "differs from production contract",
		},
		{
			name: "wrong pack identity",
			mutateCatalog: func(text string) string {
				text = strings.ReplaceAll(text, "\r\n", "\n")
				return strings.Replace(text, "pack: binary-re", "pack: web-security", 1)
			},
			want: "pack differs from manifest",
		},
		{
			name: "malformed row structure",
			mutateCatalog: func(text string) string {
				text = strings.ReplaceAll(text, "\r\n", "\n")
				return strings.Replace(text, "    purpose: Build a bounded typed PE/ELF", "    metadata: [\n    purpose: Build a bounded typed PE/ELF", 1)
			},
			want: "unsupported key",
		},
		{
			name: "non-exact catalog path",
			mutateManifest: func(text string) string {
				text = strings.ReplaceAll(text, "\r\n", "\n")
				return strings.Replace(text, "  - tooling/catalog.yml", "  - tooling/./catalog.yml", 1)
			},
			want: "requires exactly one tooling/catalog.yml entry",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			copyProductionContractPack(t, repoRoot, root, "binary-re")
			manifestPath := filepath.Join(root, "packs", "binary-re", "manifest.yml")
			catalogPath := filepath.Join(root, "packs", "binary-re", "tooling", "catalog.yml")
			if test.mutateManifest != nil {
				mutateProductionContractTestFile(t, manifestPath, test.mutateManifest)
			}
			if test.mutateCatalog != nil {
				mutateProductionContractTestFile(t, catalogPath, test.mutateCatalog)
			}
			if _, err := EnabledSpecialties(root, "binary-re"); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("EnabledSpecialties error = %v, want contains %q", err, test.want)
			}
			m, err := manifest.Load(root, "binary-re")
			if err != nil {
				t.Fatal(err)
			}
			admission := BuildAdmission(root, m.Summary())
			if admission.Ready || !warningsContain(admission.Warnings, test.want) {
				t.Fatalf("production admission accepted catalog drift: %+v", admission)
			}
		})
	}
}

func TestMaturePackInstructionPacketsBuildFromRealManifests(t *testing.T) {
	repoRoot := productionContractRepoRoot(t)
	for _, pack := range []string{"binary-re", "web-security"} {
		m, err := manifest.Load(repoRoot, pack)
		if err != nil {
			t.Fatal(err)
		}
		packet, err := BuildInstructionPacket(repoRoot, pack, m)
		if err != nil {
			t.Fatalf("build %s instruction packet: %v", pack, err)
		}
		identity := packet.Identity()
		if err := ValidateInstructionIdentity(identity); err != nil {
			t.Fatalf("validate %s instruction identity: %v", pack, err)
		}
		reloaded, err := ReloadInstructionPacket(repoRoot, identity)
		if err != nil {
			t.Fatalf("reload %s instruction packet: %v", pack, err)
		}
		inline, err := reloaded.InlineMarkdown()
		if err != nil {
			t.Fatalf("inline %s instruction packet: %v", pack, err)
		}
		if !strings.Contains(inline, identity.SHA256) || !strings.Contains(inline, identity.ReceiptKind) {
			t.Fatalf("%s inline instruction packet omitted durable identity", pack)
		}
		if pack == "web-security" && identity.Mode != InstructionModePolicyOnly {
			t.Fatalf("web-security instruction mode=%q, want policy-only", identity.Mode)
		}
		if pack == "binary-re" && identity.Mode != InstructionModePromptAndPolicy {
			t.Fatalf("binary-re instruction mode=%q, want prompt-and-policy", identity.Mode)
		}
	}
}

func TestProductionRegistryRejectsPackAndAdapterSetDrift(t *testing.T) {
	summaries := []manifest.PackSummary{
		{ID: "binary-re", Maturity: "mature"},
		{ID: "web-security", Maturity: "mature"},
	}
	withProductionRegistry(t, Registry())

	missingPack := BuildRegistryAdmission(append(summaries, manifest.PackSummary{ID: "unregistered", Maturity: "mature"}))
	if missingPack.Ready || !warningsContain(missingPack.Warnings, "mature pack set does not match") {
		t.Fatalf("unregistered mature pack was not rejected: %+v", missingPack)
	}

	registry[0].AdapterIDs = append(registry[0].AdapterIDs, "not-compiled-in")
	adapterDrift := BuildRegistryAdmission(summaries)
	if adapterDrift.Ready || !warningsContain(adapterDrift.Warnings, "compiled-in production adapter set does not match") {
		t.Fatalf("production adapter set drift was not rejected: %+v", adapterDrift)
	}
}

func TestProductionRegistryRejectsCrossPackAdapterSwap(t *testing.T) {
	summaries := []manifest.PackSummary{
		{ID: "binary-re", Maturity: "mature"},
		{ID: "web-security", Maturity: "mature"},
	}
	withProductionRegistry(t, Registry())
	registry[0].AdapterIDs, registry[1].AdapterIDs = registry[1].AdapterIDs, registry[0].AdapterIDs

	admission := BuildRegistryAdmission(summaries)
	if admission.Ready || !warningsContain(admission.Warnings, "adapter ownership differs") {
		t.Fatalf("cross-pack adapter swap was not rejected: %+v", admission)
	}
	for _, summary := range summaries {
		packAdmission := BuildAdmission(productionContractRepoRoot(t), summary)
		if packAdmission.Ready || !warningsContain(packAdmission.Warnings, "executable pack ownership") {
			t.Fatalf("cross-pack admission remained ready for %s: %+v", summary.ID, packAdmission)
		}
	}
}

func TestGoSourceContractRequiresBoundSymbols(t *testing.T) {
	root := t.TempDir()
	fixture := SourceContract{
		Identity: "synthetic fixture",
		Bindings: []GoSourceBinding{{Path: "fixture/fixture_test.go", Symbols: []string{"TestSyntheticFixture"}}},
	}

	missing := validateGoSourceContract(root, "fixture", fixture)
	if !warningsContain(missing, "fixture source is missing") {
		t.Fatalf("missing fixture source was not rejected: %v", missing)
	}

	writeProductionContractTestFile(t, root, "fixture/fixture_test.go", "package fixture\nfunc TestWrongMarker() {}\n")
	missingSymbol := validateGoSourceContract(root, "fixture", fixture)
	if !warningsContain(missingSymbol, "fixture symbol is missing") {
		t.Fatalf("missing fixture symbol was not rejected: %v", missingSymbol)
	}

	writeProductionContractTestFile(t, root, "fixture/fixture_test.go", "package fixture\nfunc TestSyntheticFixture() {}\n")
	if warnings := validateGoSourceContract(root, "fixture", fixture); len(warnings) != 0 {
		t.Fatalf("complete production source contract was rejected: %v", warnings)
	}
}

func TestRegistryReturnsDeepClone(t *testing.T) {
	first := Registry()
	first[0].AdapterIDs[0] = "changed"
	first[0].Fixture.Bindings[0].Symbols[0] = "Changed"
	second := Registry()
	if second[0].AdapterIDs[0] == "changed" || second[0].Fixture.Bindings[0].Symbols[0] == "Changed" {
		t.Fatal("production registry exposed mutable nested state")
	}
}

func productionContractRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate production contract test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func withProductionRegistry(t *testing.T, replacement []Contract) {
	t.Helper()
	original := Registry()
	registry = make([]Contract, len(replacement))
	for i, contract := range replacement {
		registry[i] = cloneContract(contract)
	}
	t.Cleanup(func() { registry = original })
}

func writeProductionContractTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func copyProductionContractPack(t *testing.T, sourceRepo, targetRepo, pack string) {
	t.Helper()
	sourceRoot := filepath.Join(sourceRepo, "packs", pack)
	targetRoot := filepath.Join(targetRepo, "packs", pack)
	if err := filepath.WalkDir(sourceRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		target := filepath.Join(targetRoot, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	}); err != nil {
		t.Fatal(err)
	}
}

func mutateProductionContractTestFile(t *testing.T, path string, mutate func(string) string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	mutated := mutate(string(data))
	if mutated == string(data) {
		t.Fatalf("test mutation did not change %s", path)
	}
	if err := os.WriteFile(path, []byte(mutated), 0o644); err != nil {
		t.Fatal(err)
	}
}

func warningsContain(warnings []string, fragment string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, fragment) {
			return true
		}
	}
	return false
}
