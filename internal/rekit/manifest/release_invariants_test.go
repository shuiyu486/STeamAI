package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type testCatalog struct {
	SchemaVersion      int                `json:"schemaVersion"`
	Description        string             `json:"description"`
	DefaultWorkRoot    string             `json:"defaultWorkRoot"`
	GlobalBoundaries   []string           `json:"globalBoundaries"`
	RecommendedMinimum []string           `json:"recommendedMinimum"`
	Tests              []testCatalogEntry `json:"tests"`
}

type testCatalogEntry struct {
	ID               string   `json:"id"`
	Script           string   `json:"script"`
	Category         string   `json:"category"`
	Pack             string   `json:"pack"`
	Purpose          string   `json:"purpose"`
	RecommendedFor   []string `json:"recommendedFor"`
	SupportsWorkRoot bool     `json:"supportsWorkRoot"`
	SupportsCaseRoot bool     `json:"supportsCaseRoot"`
	RiskBoundary     string   `json:"riskBoundary"`
	RelatedDocs      []string `json:"relatedDocs"`
}

func TestReleaseCatalogInvariants(t *testing.T) {
	repo := repoRoot(t)
	catalog := loadTestCatalog(t, repo)
	if catalog.SchemaVersion != 1 {
		t.Fatalf("catalog schemaVersion = %d, want 1", catalog.SchemaVersion)
	}
	if strings.TrimSpace(catalog.Description) == "" || strings.TrimSpace(catalog.DefaultWorkRoot) == "" {
		t.Fatalf("catalog description/defaultWorkRoot must be non-empty: %+v", catalog)
	}
	if len(catalog.GlobalBoundaries) == 0 || len(catalog.RecommendedMinimum) == 0 || len(catalog.Tests) == 0 {
		t.Fatalf("catalog globalBoundaries/recommendedMinimum/tests must be non-empty: %+v", catalog)
	}

	minimum := stringSet(catalog.RecommendedMinimum)
	for _, required := range []string{
		"facade-smoke.ps1",
		"catalog-smoke.ps1",
		"pack-smoke-matrix-selftest.ps1",
		"pack-smoke-matrix.ps1 -DiscoveryOnly",
		"pack-inventory-smoke.ps1",
		"go test ./...",
		"go vet ./...",
		"rekit/rekit.ps1 -Command doctor",
		"git diff --check",
	} {
		if !minimum[required] {
			t.Fatalf("catalog recommendedMinimum missing %q: %v", required, catalog.RecommendedMinimum)
		}
	}

	validCategories := stringSet([]string{
		"facade",
		"inventory",
		"catalog",
		"pack-matrix",
		"pack-helper",
		"pack-smoke",
		"case-scaffold",
		"subagents",
		"sync-promote",
		"agent-team",
		"gate-ledger",
		"workstream",
	})
	testScriptDir := filepath.Join(repo, "rekit", "tests")
	scriptFiles := listPowerShellScripts(t, testScriptDir)
	catalogScripts := map[string]bool{}
	ids := map[string]bool{}
	idPattern := regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

	for _, entry := range catalog.Tests {
		if strings.TrimSpace(entry.ID) == "" || !idPattern.MatchString(entry.ID) {
			t.Fatalf("invalid catalog id: %q", entry.ID)
		}
		if ids[entry.ID] {
			t.Fatalf("duplicate catalog id: %s", entry.ID)
		}
		ids[entry.ID] = true
		if strings.TrimSpace(entry.Script) == "" || strings.TrimSpace(entry.Category) == "" {
			t.Fatalf("catalog entry %s must declare script and category: %+v", entry.ID, entry)
		}
		if !validCategories[entry.Category] {
			t.Fatalf("catalog entry %s has unexpected category %q", entry.ID, entry.Category)
		}
		if strings.TrimSpace(entry.Purpose) == "" || strings.TrimSpace(entry.RiskBoundary) == "" || len(entry.RecommendedFor) == 0 || len(entry.RelatedDocs) == 0 {
			t.Fatalf("catalog entry %s has incomplete metadata: %+v", entry.ID, entry)
		}

		scriptLeaf := catalogScriptLeaf(entry.Script)
		if strings.HasSuffix(strings.ToLower(scriptLeaf), ".ps1") {
			if !scriptFiles[scriptLeaf] {
				t.Fatalf("catalog entry %s references missing script %s", entry.ID, scriptLeaf)
			}
			catalogScripts[scriptLeaf] = true
		}
		for _, doc := range entry.RelatedDocs {
			if strings.TrimSpace(doc) == "" {
				t.Fatalf("catalog entry %s has empty related doc", entry.ID)
			}
			if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(doc))); err != nil {
				t.Fatalf("catalog entry %s references missing related doc %s: %v", entry.ID, doc, err)
			}
		}
		if entry.Category == "pack-smoke" {
			if strings.TrimSpace(entry.Pack) == "" {
				t.Fatalf("pack-smoke entry %s is missing pack", entry.ID)
			}
			if entry.Script != entry.Pack+"-pack-smoke.ps1" {
				t.Fatalf("pack-smoke entry %s script = %q, want %q", entry.ID, entry.Script, entry.Pack+"-pack-smoke.ps1")
			}
			if !entry.SupportsWorkRoot || entry.SupportsCaseRoot {
				t.Fatalf("pack-smoke entry %s WorkRoot/CaseRoot flags = %v/%v, want true/false", entry.ID, entry.SupportsWorkRoot, entry.SupportsCaseRoot)
			}
		}
	}

	for _, requiredID := range []string{"facade-smoke", "pack-inventory-smoke", "catalog-smoke", "pack-smoke-matrix-selftest", "pack-smoke-matrix-discovery", "pack-smoke-matrix", "pack-smoke-lib"} {
		if !ids[requiredID] {
			t.Fatalf("catalog missing required test id %q", requiredID)
		}
	}
	for script := range scriptFiles {
		if !catalogScripts[script] {
			t.Fatalf("catalog missing script entry for %s", script)
		}
	}
	for script := range catalogScripts {
		if !scriptFiles[script] {
			t.Fatalf("catalog references unknown script %s", script)
		}
	}
}

func TestReleaseSkeletonPackSmokeDiscoveryInvariants(t *testing.T) {
	repo := repoRoot(t)
	catalog := loadTestCatalog(t, repo)
	skeletonPacks := schemaValidSkeletonPacks(t, repo)
	catalogPacks := packSmokeCatalogPacks(catalog)
	wrapperPacks := packSmokeWrapperPacks(t, filepath.Join(repo, "rekit", "tests"))
	matrixPacks := packSmokeMatrixPacks(t, filepath.Join(repo, "rekit", "tests", "pack-smoke-matrix.ps1"))

	assertSameStringSet(t, "skeleton manifests", skeletonPacks, "catalog pack-smoke entries", catalogPacks)
	assertSameStringSet(t, "skeleton manifests", skeletonPacks, "pack smoke wrappers", wrapperPacks)
	assertSameStringSet(t, "skeleton manifests", skeletonPacks, "pack smoke matrix entries", matrixPacks)

	for _, nonSkeleton := range []string{"_template", "vmp-re"} {
		if catalogPacks[nonSkeleton] || wrapperPacks[nonSkeleton] || matrixPacks[nonSkeleton] {
			t.Fatalf("non-skeleton pack %s must not be in skeleton smoke sets: catalog=%v wrappers=%v matrix=%v", nonSkeleton, catalogPacks[nonSkeleton], wrapperPacks[nonSkeleton], matrixPacks[nonSkeleton])
		}
	}
}

func loadTestCatalog(t *testing.T, repo string) testCatalog {
	t.Helper()
	path := filepath.Join(repo, "rekit", "tests", "catalog.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var catalog testCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatalf("catalog.json did not decode: %v", err)
	}
	return catalog
}

func listPowerShellScripts(t *testing.T, dir string) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".ps1") {
			continue
		}
		out[entry.Name()] = true
	}
	if len(out) == 0 {
		t.Fatalf("no PowerShell scripts found under %s", dir)
	}
	return out
}

func catalogScriptLeaf(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func schemaValidSkeletonPacks(t *testing.T, repo string) map[string]bool {
	t.Helper()
	packs, err := List(repo)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, pack := range packs {
		if pack.SchemaValid && pack.Maturity == "skeleton" {
			out[pack.ID] = true
		}
	}
	if len(out) == 0 {
		t.Fatal("no schema-valid skeleton packs found")
	}
	return out
}

func packSmokeCatalogPacks(catalog testCatalog) map[string]bool {
	out := map[string]bool{}
	for _, entry := range catalog.Tests {
		if entry.Category == "pack-smoke" {
			out[entry.Pack] = true
		}
	}
	return out
}

func packSmokeWrapperPacks(t *testing.T, dir string) map[string]bool {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*-pack-smoke.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, match := range matches {
		leaf := filepath.Base(match)
		pack := strings.TrimSuffix(leaf, "-pack-smoke.ps1")
		out[pack] = true
	}
	return out
}

func packSmokeMatrixPacks(t *testing.T, matrixPath string) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(matrixPath)
	if err != nil {
		t.Fatal(err)
	}
	entryPattern := regexp.MustCompile(`(?m)^\s*'([^']+)'\s*=\s*'([^']+-pack-smoke\.ps1)'`)
	out := map[string]bool{}
	for _, match := range entryPattern.FindAllStringSubmatch(string(data), -1) {
		pack := match[1]
		script := match[2]
		if script != pack+"-pack-smoke.ps1" {
			t.Fatalf("pack smoke matrix entry %s points at %s", pack, script)
		}
		out[pack] = true
	}
	if len(out) == 0 {
		t.Fatalf("no pack smoke matrix entries parsed from %s", matrixPath)
	}
	return out
}

func stringSet(items []string) map[string]bool {
	out := map[string]bool{}
	for _, item := range items {
		out[item] = true
	}
	return out
}

func assertSameStringSet(t *testing.T, leftName string, left map[string]bool, rightName string, right map[string]bool) {
	t.Helper()
	missing := []string{}
	extra := []string{}
	for item := range left {
		if !right[item] {
			missing = append(missing, item)
		}
	}
	for item := range right {
		if !left[item] {
			extra = append(extra, item)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("%s and %s differ; missing in %s: %v; extra in %s: %v", leftName, rightName, rightName, missing, rightName, extra)
	}
}
