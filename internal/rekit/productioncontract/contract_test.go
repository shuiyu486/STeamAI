package productioncontract

import (
	"os"
	"path/filepath"
	"runtime"
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

func warningsContain(warnings []string, fragment string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, fragment) {
			return true
		}
	}
	return false
}
