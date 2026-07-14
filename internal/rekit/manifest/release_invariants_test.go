package manifest

import (
	"encoding/json"
	"io/fs"
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
		"go run ./cmd/rekit -- -Command release-check -Format json",
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

func TestGoRuntimeDefaultPackInvariants(t *testing.T) {
	repo := repoRoot(t)
	defaultPackPackage := "internal/rekit/defaults"
	defaultPackFile := filepath.ToSlash(filepath.Join(repo, defaultPackPackage, "defaults.go"))
	manifestFile := filepath.ToSlash(filepath.Join(repo, "internal", "rekit", "manifest", "manifest.go"))
	allowedLiteralTestFiles := map[string]bool{
		filepath.ToSlash(filepath.Join(repo, "internal", "rekit", "gate", "gate_test.go")):                    true,
		filepath.ToSlash(filepath.Join(repo, "internal", "rekit", "instance", "instance_test.go")):            true,
		filepath.ToSlash(filepath.Join(repo, "internal", "rekit", "note", "note_test.go")):                    true,
		filepath.ToSlash(filepath.Join(repo, "internal", "rekit", "manifest", "manifest_test.go")):            true,
		filepath.ToSlash(filepath.Join(repo, "internal", "rekit", "manifest", "release_invariants_test.go")):  true,
		filepath.ToSlash(filepath.Join(repo, "internal", "rekit", "releasecheck", "release_handoff_test.go")): true,
		filepath.ToSlash(filepath.Join(repo, "internal", "rekit", "cli", "cli_test.go")):                      true,
	}
	goFiles := listGoFiles(t, filepath.Join(repo, "internal", "rekit"))
	for _, path := range goFiles {
		text := string(readFile(t, path))
		if !strings.Contains(text, "vmp-re") {
			continue
		}
		slash := filepath.ToSlash(path)
		if strings.HasSuffix(path, "_test.go") {
			if !allowedLiteralTestFiles[slash] {
				t.Fatalf("unexpected test literal vmp-re outside explicit pack fixtures: %s", slash)
			}
			continue
		}
		switch slash {
		case defaultPackFile:
			if !strings.Contains(text, `const DefaultPack = "vmp-re"`) || strings.Count(text, `"vmp-re"`) != 1 {
				t.Fatalf("%s must define exactly one default pack literal", slash)
			}
		case manifestFile:
			for i, line := range strings.Split(text, "\n") {
				if strings.Contains(line, "vmp-re") && !manifestNonVMPGuardLiteral(line) {
					t.Fatalf("manifest guard file has unexpected vmp-re literal at line %d: %s", i+1, line)
				}
			}
		default:
			t.Fatalf("Go runtime default pack literal must stay centralized in %s: %s", defaultPackPackage, slash)
		}
	}
	for _, productionFile := range []string{
		"internal/rekit/cli/cli.go",
		"internal/rekit/instance/instance.go",
		"internal/rekit/manifest/manifest.go",
		"internal/rekit/runtime/runtime.go",
	} {
		text := readRepoText(t, repo, productionFile)
		assertTextContains(t, text, `internal/rekit/defaults`, productionFile+" default pack import")
		assertTextContains(t, text, `defaults.DefaultPack`, productionFile+" default pack reference")
	}
}

func manifestNonVMPGuardLiteral(line string) bool {
	for _, allowed := range []string{
		"implicit vmp-re fallback is not allowed",
		"non-vmp pack declares vmp-re path",
		"non-vmp pack declares vmp-re authority path",
		"non-vmp pack declares vmp-re workstream default",
		`(^|/)vmp-re(/|$)`,
	} {
		if strings.Contains(line, allowed) {
			return true
		}
	}
	return false
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

func TestReleaseReadinessChecklistInvariants(t *testing.T) {
	repo := repoRoot(t)
	checklist := readRepoText(t, repo, "docs/release-readiness.md")
	catalog := loadTestCatalog(t, repo)

	for _, section := range []string{
		"## 读取指南",
		"## 实施摘要",
		"## 执行清单",
		"## 验证标准",
		"## 风险与注意事项",
		"## 当前 pack maturity matrix",
		"## Go-owned 与 PowerShell legacy 状态",
		"## Known gaps",
	} {
		assertTextContains(t, checklist, section, "release readiness section")
	}

	for _, command := range catalog.RecommendedMinimum {
		assertTextContains(t, checklist, command, "release readiness recommended minimum")
	}
	for _, command := range []string{
		"go run ./cmd/rekit -- -Command release-check -Format json",
		"go test ./...",
		"go vet ./...",
		"./rekit/rekit.ps1 -Command doctor",
		"./rekit/tests/facade-smoke.ps1",
		"git diff --check",
	} {
		assertTextContains(t, checklist, command, "release readiness local gate")
	}

	packs, err := List(repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, pack := range packs {
		assertTextContains(t, checklist, "`"+pack.ID+"`", "release readiness pack matrix")
	}

	for _, boundary := range []string{
		"不默认运行大型 PowerShell matrix",
		"不执行真实网络请求",
		"不写 authority/confirmed",
		"gate -Apply",
		"continue -Apply",
		"sync/promote",
		"REKIT_GO_DISABLE=1",
	} {
		assertTextContains(t, checklist, boundary, "release readiness boundary")
	}
	for _, releaseHandoff := range []string{
		"releaseHandoff.ready",
		"readFirst[]",
		"signals[]",
		"latestBatch",
		"releaseNotes.covered",
		"knownGaps[]",
		"packMaturity",
		"validation[]",
		"nextActions[]",
	} {
		assertTextContains(t, checklist, releaseHandoff, "release readiness handoff inventory")
	}

	for _, gap := range []string{
		"bounded dispatch",
		"actual heavy-tool",
		"authority/confirmed",
		"policy schema 迁移",
		"PowerShell runtime deprecation",
	} {
		assertTextContains(t, checklist, gap, "release readiness known gap")
	}

	for _, doc := range []string{"README.md", "CLAUDE.md", "docs/go-first-convergence-plan.md"} {
		assertTextContains(t, readRepoText(t, repo, doc), "docs/release-readiness.md", doc+" release readiness link")
	}
}

func TestReleaseGateWorkflowInvariants(t *testing.T) {
	repo := repoRoot(t)
	workflow := readRepoText(t, repo, ".github/workflows/release-gate.yml")
	checklist := readRepoText(t, repo, "docs/release-readiness.md")

	for _, required := range []string{
		"name: release-gate",
		"runs-on: ubuntu-latest",
		"runs-on: windows-latest",
		"uses: actions/checkout@v4",
		"uses: actions/setup-go@v5",
		"go-version: '1.26.x'",
		"go run ./cmd/rekit -- -Command release-check -Format json",
		"go test ./...",
		"go vet ./...",
		".\\rekit\\rekit.ps1 -Command doctor",
		".\\rekit\\tests\\facade-smoke.ps1",
	} {
		assertTextContains(t, workflow, required, "release gate workflow")
	}
	for _, forbidden := range []string{
		"pack-smoke-matrix.ps1",
		"pack-inventory-smoke.ps1",
		"agent-team-dryrun-smoke.ps1",
		"full-trace",
		"debug",
		"inject",
		"patch",
		"dump",
		"network",
		"symex",
	} {
		if strings.Contains(workflow, forbidden) {
			t.Fatalf("release gate workflow must not run broad matrix or heavy-tool step %q", forbidden)
		}
	}
	for _, required := range []string{
		".github/workflows/release-gate.yml",
		"Go release checks",
		"Windows facade smoke",
		"ciReleaseGate.ready",
		"requiredCommands[]",
		"forbiddenStrings[]",
		"不默认运行大型 PowerShell matrix",
	} {
		assertTextContains(t, checklist, required, "release readiness CI workflow")
	}
}

func TestAutonomousGoalGuideInvariants(t *testing.T) {
	repo := repoRoot(t)
	guide := readRepoText(t, repo, "docs/autonomous-goal.md")

	for _, section := range []string{
		"## 读取指南",
		"## 实施摘要",
		"## 执行清单",
		"## 验证标准",
		"## 风险与注意事项",
		"## 给新会话的 goal 语句",
	} {
		assertTextContains(t, guide, section, "autonomous goal guide section")
	}

	for _, direction := range []string{
		"Mission Control UX",
		"Lane protocol",
		"Replaceable session executor",
		"Tactical subagents",
		"Pre-authorized lane autonomy",
		"Pack-based team memory",
		"Go-first deterministic substrate",
	} {
		assertTextContains(t, guide, direction, "autonomous goal guide direction")
	}
	for _, required := range []string{
		"简短接手锚点",
		"不是新的限制清单",
		"Lane-centric Agent Team Mission Control",
		"中大型",
		"完成后自审、评估",
		"默认继续自主推进",
		"heavy-tool、动态调试、patch、dump、hook、网络、exploit replay",
	} {
		assertTextContains(t, guide, required, "autonomous goal guide autonomy guard")
	}

	for _, doc := range []string{"README.md", "CLAUDE.md", "docs/go-first-convergence-plan.md", "docs/release-readiness.md", "docs/vision.md", "docs/reference-absorption.md"} {
		assertTextContains(t, readRepoText(t, repo, doc), "docs/autonomous-goal.md", doc+" autonomous goal link")
		assertTextContains(t, readRepoText(t, repo, doc), "docs/mission-control-product-direction.md", doc+" mission control link")
	}
}

func TestMissionControlProductDirectionInvariants(t *testing.T) {
	repo := repoRoot(t)
	doc := readRepoText(t, repo, "docs/mission-control-product-direction.md")

	for _, section := range []string{
		"## 读取指南",
		"## 实施摘要",
		"## 执行清单",
		"## 验证标准",
		"## 风险与注意事项",
		"## 10. 推荐给新会话的接手话术",
		"## 11. 推荐长期 goal 语句",
	} {
		assertTextContains(t, doc, section, "mission control product direction section")
	}

	for _, term := range []string{
		"Lane-centric Agent Team Mission Control",
		"主 Agent / Mission Commander",
		"durable member lane",
		"可替换 Claude Code session executor",
		"短命 tactical subagents",
		"Human-in-the-Lane",
		"Pre-authorized lane autonomy",
		"heavy-tool、动态调试、patch、dump、hook、网络、exploit replay",
		"Pack-based team memory",
	} {
		assertTextContains(t, doc, term, "mission control product direction term")
	}
}

func TestPowerShellDeprecationStrategyInvariants(t *testing.T) {
	repo := repoRoot(t)
	strategy := readRepoText(t, repo, "docs/powershell-deprecation.md")

	for _, section := range []string{
		"## 读取指南",
		"## 实施摘要",
		"## 执行清单",
		"## 验证标准",
		"## 风险与注意事项",
		"## 命令归属矩阵",
		"## PowerShell 模块状态",
		"## Freeze / deprecation gates",
		"## 禁止迁移清单",
	} {
		assertTextContains(t, strategy, section, "PowerShell deprecation section")
	}

	for _, term := range []string{
		"Go-owned",
		"PowerShell façade",
		"Legacy-only",
		"Parity smoke",
		"删除前置条件",
		"REKIT_GO_DISABLE=1",
		"legacy-only",
		"blocked",
		"Removal batch",
	} {
		assertTextContains(t, strategy, term, "PowerShell deprecation status")
	}
	for _, command := range []string{
		"release-check",
		"status",
		"packs",
		"doctor",
		"validate",
		"attach",
		"repair",
		"init",
		"bootstrap",
		"sync",
		"promote",
		"overview",
		"note -List",
		"gate -WhatIf",
		"start",
		"handoff",
		"continue -WhatIf -Format json",
		"plan-subagents",
		"actual heavy-tool",
		"authority/confirmed",
	} {
		assertTextContains(t, strategy, command, "PowerShell deprecation command matrix")
	}

	assertTextContains(t, strategy, "`rekit/rekit.ps1`", "PowerShell deprecation facade module")
	libModules, err := filepath.Glob(filepath.Join(repo, "rekit", "lib", "*.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	if len(libModules) == 0 {
		t.Fatal("no PowerShell lib modules found")
	}
	for _, module := range libModules {
		assertTextContains(t, strategy, "`rekit/lib/"+filepath.Base(module)+"`", "PowerShell deprecation module matrix")
	}

	for _, blocked := range []string{
		"full-trace/debug/inject/patch/dump/network/symex/heavy-tool",
		"authority/confirmed 自动写入",
		"policy schema 迁移",
		"外部服务发布",
		"case-local shim",
	} {
		assertTextContains(t, strategy, blocked, "PowerShell deprecation forbidden migration")
	}
	for _, doc := range []string{"README.md", "CLAUDE.md", "docs/go-first-convergence-plan.md", "docs/release-readiness.md"} {
		assertTextContains(t, readRepoText(t, repo, doc), "docs/powershell-deprecation.md", doc+" PowerShell deprecation link")
	}
}

func TestPowerShellFacadeFreezeInvariants(t *testing.T) {
	repo := repoRoot(t)
	facade := readRepoText(t, repo, "rekit/rekit.ps1")
	strategy := readRepoText(t, repo, "docs/powershell-deprecation.md")
	smoke := readRepoText(t, repo, "rekit/tests/facade-smoke.ps1")

	defaultCommands := powerShellSingleQuotedArrayInFunction(t, facade, "Test-RekitGoDefaultDelegationCommand")
	expectedDefaultCommands := stringSet([]string{
		"status",
		"packs",
		"release-check",
		"doctor",
		"validate",
		"attach",
		"repair",
		"init",
		"bootstrap",
		"sync",
		"update",
		"promote",
		"overview",
		"note",
		"gate",
		"start",
		"handoff",
		"continue",
	})
	assertSameStringSet(t, "documented Go-default facade commands", expectedDefaultCommands, "Test-RekitGoDefaultDelegationCommand", defaultCommands)

	validateSet := powerShellValidateSet(t, facade)
	for command := range expectedDefaultCommands {
		if !validateSet[command] {
			t.Fatalf("ValidateSet missing Go-default command %q", command)
		}
	}
	for _, legacyOrBlocked := range []string{"plan-subagents", "full-trace", "debug", "inject", "patch", "dump", "network", "heavy-tool", "authority", "confirmed"} {
		if defaultCommands[legacyOrBlocked] {
			t.Fatalf("legacy/blocked command %q must not default to Go facade", legacyOrBlocked)
		}
	}
	if !validateSet["plan-subagents"] {
		t.Fatal("ValidateSet must keep plan-subagents as a supported PowerShell internal command")
	}

	for _, required := range []string{
		"if ($Command -eq 'release-check' -and -not [string]::IsNullOrWhiteSpace($Target)) { return $false }",
		"if ($Command -notin @('start','handoff','continue','release-check'))",
		"release-check is implemented by the Go backend only",
		"gate is implemented by the Go backend only",
	} {
		assertTextContains(t, facade, required, "PowerShell facade freeze guard")
	}
	for _, required := range []string{
		"| `release-check` | Go default | façade delegate + no PowerShell fallback |",
		"Legacy freeze",
		"PowerShell 只允许 bug fix / compatibility / safety boundary 修复",
		"actual full-trace/debug/inject/patch/dump/network/symex/heavy-tool",
		"authority/confirmed 自动写入",
		"policy schema 迁移",
		"case-local shim",
	} {
		assertTextContains(t, strategy, required, "PowerShell facade freeze strategy")
	}
	for _, required := range []string{
		"default release-check fake delegation",
		"REKIT_GO_DISABLE",
		"must-not-run",
	} {
		assertTextContains(t, smoke, required, "PowerShell facade freeze smoke")
	}
}

func powerShellValidateSet(t *testing.T, text string) map[string]bool {
	t.Helper()
	match := regexp.MustCompile(`(?s)\[ValidateSet\((.*?)\)\]`).FindStringSubmatch(text)
	if len(match) != 2 {
		t.Fatal("PowerShell ValidateSet not found")
	}
	return powerShellSingleQuotedItems(match[1])
}

func powerShellSingleQuotedArrayInFunction(t *testing.T, text, functionName string) map[string]bool {
	t.Helper()
	functionPattern := regexp.MustCompile(`(?s)function\s+` + regexp.QuoteMeta(functionName) + `\s*\{(.*?)\n\}`)
	functionMatch := functionPattern.FindStringSubmatch(text)
	if len(functionMatch) != 2 {
		t.Fatalf("PowerShell function %s not found", functionName)
	}
	arrayMatch := regexp.MustCompile(`@\((.*?)\)`).FindStringSubmatch(functionMatch[1])
	if len(arrayMatch) != 2 {
		t.Fatalf("PowerShell function %s does not contain a single-quoted array", functionName)
	}
	return powerShellSingleQuotedItems(arrayMatch[1])
}

func powerShellSingleQuotedItems(text string) map[string]bool {
	items := map[string]bool{}
	for _, match := range regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(text, -1) {
		items[match[1]] = true
	}
	return items
}

func loadTestCatalog(t *testing.T, repo string) testCatalog {
	t.Helper()
	data := []byte(readRepoText(t, repo, "rekit/tests/catalog.json"))
	var catalog testCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatalf("catalog.json did not decode: %v", err)
	}
	return catalog
}

func readRepoText(t *testing.T, repo, rel string) string {
	t.Helper()
	return string(readFile(t, filepath.Join(repo, filepath.FromSlash(rel))))
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func listGoFiles(t *testing.T, root string) []string {
	t.Helper()
	files := []string{}
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".go") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}

func assertTextContains(t *testing.T, text, want, label string) {
	t.Helper()
	if !strings.Contains(text, want) {
		t.Fatalf("%s missing %q", label, want)
	}
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
