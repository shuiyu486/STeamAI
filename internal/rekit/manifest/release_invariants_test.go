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
		"go run ./cmd/rekit -- -Command release-check -Format json",
		"go run ./cmd/rekit -- -Command status",
		"go run ./cmd/rekit -- -Command packs",
		"go run ./cmd/rekit -- -Command doctor",
		"go test ./...",
		"go vet ./...",
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
		"go run ./cmd/rekit -- -Command status",
		"go run ./cmd/rekit -- -Command packs",
		"go run ./cmd/rekit -- -Command doctor",
		"go test ./...",
		"go vet ./...",
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
		"不默认运行 PowerShell façade smoke 或大型 PowerShell matrix",
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
		"billing/spending limit",
		"cross-platform",
		"session/reviewer orchestrator",
		"多 reviewer orchestration",
		"actual heavy-tool",
		"authority/confirmed",
		"pack-based team memory",
		"policy schema 迁移",
		"PowerShell-free / Go-native 收敛",
	} {
		assertTextContains(t, checklist, gap, "release readiness known gap")
	}

	for _, doc := range []string{"README.md", "CLAUDE.md", "docs/go-first-convergence-plan.md"} {
		assertTextContains(t, readRepoText(t, repo, doc), "docs/release-readiness.md", doc+" release readiness link")
	}

	currentDocs := map[string]string{
		"README.md":                        readRepoText(t, repo, "README.md"),
		".claude/skills/rekit/SKILL.md":    readRepoText(t, repo, ".claude/skills/rekit/SKILL.md"),
		"docs/reference-absorption.md":     readRepoText(t, repo, "docs/reference-absorption.md"),
		"docs/go-runtime-migration.md":     readRepoText(t, repo, "docs/go-runtime-migration.md"),
		"docs/case-migration.md":           readRepoText(t, repo, "docs/case-migration.md"),
		"docs/orchestration-plan.md":       readRepoText(t, repo, "docs/orchestration-plan.md"),
		"common/policies/agent-team.md":    readRepoText(t, repo, "common/policies/agent-team.md"),
		"common/policies/tool-adapters.md": readRepoText(t, repo, "common/policies/tool-adapters.md"),
		"rekit/tests/README.md":            readRepoText(t, repo, "rekit/tests/README.md"),
	}
	packsRoot := filepath.Join(repo, "packs")
	if err := filepath.WalkDir(packsRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			return nil
		}
		rel, err := filepath.Rel(repo, path)
		if err != nil {
			return err
		}
		currentDocs[filepath.ToSlash(rel)] = string(readFile(t, path))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"case smoke 验证过的 runtime | `rekit/rekit.ps1`、`rekit/lib/*.ps1`",
		"不默认接入 façade",
		"公共 PowerShell façade 仍不委托内部命令",
		"Go CLI 与 PowerShell fallback 均可列出全部 pack",
		"PowerShell text preview fallback",
		"`REKIT_GO_DISABLE=1` fallback",
		"立即 fallback 到 PowerShell",
		"`Add-RekitFactEvent` 9 种 kind",
		"`rekit/lib/B3.State.ps1`（`Add-RekitFactEvent`）",
		"`rekit/lib/B3.Auto.ps1`（`New-RekitDecision`）",
		"`REKIT_GO_DISABLE=1` 可回退 PowerShell",
		"`rekit/lib/B3.Commands.ps1` / future module",
		"当前 lane 文档、task packet 或 autonomy profile 中的预授权",
		"当前 lane 文档/packet/autonomy profile 明确预授权",
		"`gate -Apply` 只写 pending-gate request",
		"先登记 pending-gate request",
		"必须先登记 pending-gate request",
		"pending-gate request，并等待用户确认",
		"runtime 当前不强制 gate",
		"不绕过用户确认执行",
		"剩余 write/text compatibility fallback",
		"允许自动 append authority CSV",
		"随代码一起提交推送到远程 `main`",
	} {
		for path, text := range currentDocs {
			assertTextNotContains(t, text, forbidden, path+" retired current-state wording")
		}
	}
	for path, phrase := range map[string]string{
		"docs/orchestration-plan.md":                              "既无本次显式用户确认、也无有效 deterministic grant 时，不执行外部副作用",
		"common/policies/agent-team.md":                           "strict validated `.rekit/lanes/<lane>/autonomy.json` 与覆盖本次 action",
		"common/policies/tool-adapters.md":                        "`pending-gate=true`，`authorized-gate=false`",
		"packs/_template/references/template/agent-team.md":       "`gate -Apply` 只记录 `pending-gate` 或 `authorized-gate` decision，不执行动作",
		"packs/_template/references/template/toolchain-router.md": "`pending-gate` 对应 `true`，`authorized-gate` 对应 `false`",
		"packs/vmp-re/policies/verification.overlay.md":           "Go runtime 已强制 gate action/profile preflight 与 request decision 写入边界",
		"packs/vmp-re/references/vmp-re/toolchain-router.md":      "lane packet 只表达授权意图",
		"rekit/tests/README.md":                                   "WhatIf 和 Apply 都不 append authority/confirmed",
	} {
		assertTextContains(t, currentDocs[path], phrase, path+" deterministic heavy-action authorization")
	}
	canonicalGateActions := stringSet([]string{"full-trace", "debug", "inject", "patch", "dump", "network", "symex"})
	for path, text := range currentDocs {
		if (!strings.Contains(path, "/tooling/recipes/") && !strings.HasSuffix(path, "/toolchain-router.md")) || !strings.Contains(text, "gate_action:") {
			continue
		}
		for _, field := range []string{"target_ref:", "requested_budget:", "runtime_seconds:", "disk_mb:", "requests:", "output_paths:", "stop_conditions:"} {
			assertTextContains(t, text, field, path+" strict gate request contract")
		}
		for line := range strings.SplitSeq(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "gate_action:") {
				continue
			}
			for action := range strings.SplitSeq(strings.TrimSpace(strings.TrimPrefix(trimmed, "gate_action:")), "|") {
				action = strings.TrimSpace(action)
				if action == "" || !canonicalGateActions[action] {
					t.Fatalf("%s gate_action %q is not in canonical skeleton heavyToolGates", path, action)
				}
			}
		}
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
		"runs-on: macos-latest",
		"uses: actions/checkout@v4",
		"uses: actions/setup-go@v5",
		"go-version: '1.26.x'",
		"go run ./cmd/rekit -- -Command release-check -Format json",
		"go run ./cmd/rekit -- -Command status",
		"go run ./cmd/rekit -- -Command packs",
		"go run ./cmd/rekit -- -Command doctor",
		"go test ./...",
		"go vet ./...",
	} {
		assertTextContains(t, workflow, required, "release gate workflow")
	}
	for _, forbidden := range []string{
		"rekit.ps1",
		"facade-smoke.ps1",
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
		"Go release checks (Linux)",
		"Go release checks (Windows)",
		"Go release checks (macOS)",
		"ciReleaseGate.ready",
		"requiredCommands[]",
		"forbiddenStrings[]",
		"不默认运行 PowerShell façade smoke 或大型 PowerShell matrix",
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
		"main 与 origin/main 同步",
		"直接提交并推送到 origin/main",
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
	for _, doc := range []string{"README.md", "CLAUDE.md", "docs/context-routing.md", "docs/autonomous-goal.md", "docs/reference-absorption.md"} {
		text := readRepoText(t, repo, doc)
		assertTextContains(t, text, "main", doc+" main branch handoff")
		assertTextContains(t, text, "origin/main", doc+" origin main handoff")
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
		"release-run",
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
		"continue -WhatIf",
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
	if len(libModules) != 0 {
		t.Fatalf("PowerShell legacy lib modules must stay removed: %v", libModules)
	}
	for _, module := range []string{"Manifest.ps1", "Validate.ps1", "Instance.ps1", "Sync.ps1", "Promote.ps1", "Review.ps1", "B3.Core.ps1", "B3.State.ps1", "B3.Policy.ps1", "B3.Lane.ps1", "B3.Auto.ps1", "B3.Handoff.ps1", "B3.Commands.ps1"} {
		assertTextContains(t, strategy, "`rekit/lib/"+module+"`", "PowerShell retired module matrix")
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

	if !strings.Contains(facade, "if (Test-RekitNoPowerShellFallbackCommand -Name $Command)") {
		t.Fatalf("PowerShell facade must keep no-fallback guard")
	}
	for _, forbidden := range []string{
		". (Join-Path $RuntimeRoot 'lib\\Manifest.ps1')",
		". (Join-Path $RuntimeRoot 'lib\\B3.Commands.ps1')",
		"Get-RekitPackInventory -RepoRoot",
		"Invoke-RekitStart -Target",
		"Sync-RekitPack -Target",
		"Promote-RekitChanges -Target",
	} {
		assertTextNotContains(t, facade, forbidden, "retired PowerShell facade fallback dispatcher")
	}

	defaultCommands := powerShellSingleQuotedArrayInFunction(t, facade, "Test-RekitGoDefaultDelegationCommand")
	expectedDefaultCommands := stringSet([]string{
		"status",
		"packs",
		"release-check",
		"release-run",
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
		"reconcile",
		"plan-subagents",
	})
	assertSameStringSet(t, "documented Go-default facade commands", expectedDefaultCommands, "Test-RekitGoDefaultDelegationCommand", defaultCommands)

	validateSet := powerShellValidateSet(t, facade)
	for command := range expectedDefaultCommands {
		if !validateSet[command] {
			t.Fatalf("ValidateSet missing Go-default command %q", command)
		}
	}
	for _, legacyOrBlocked := range []string{"full-trace", "debug", "inject", "patch", "dump", "network", "heavy-tool", "authority", "confirmed"} {
		if defaultCommands[legacyOrBlocked] {
			t.Fatalf("legacy/blocked command %q must not default to Go facade", legacyOrBlocked)
		}
	}
	if !validateSet["plan-subagents"] {
		t.Fatal("ValidateSet must keep plan-subagents as a supported facade command")
	}

	for _, required := range []string{
		"if ($Command -in @('release-check','release-run') -and -not [string]::IsNullOrWhiteSpace($Target)) { return $false }",
		"if ($Command -notin @('start','handoff','continue','reconcile','release-check','release-run'))",
		"implemented by the Go backend only",
		"Test-RekitNoPowerShellFallbackCommand",
		"PowerShell fallback has been retired",
		"$goFormat = 'text'",
		"retired PowerShell fallback dispatcher",
		"Add-RekitGoArg ([ref]$goArgs) '-Route' $Route",
		"if ($Command -ne 'plan-subagents' -and -not [string]::IsNullOrWhiteSpace($ReviewerResultPath)) { return $false }",
		"$CallerWorkingDirectory = [System.IO.Path]::GetFullPath((Get-Location).Path)",
		"Add-RekitGoArg ([ref]$goArgs) '-ReviewerResultPath' (Resolve-RekitCallerPath $ReviewerResultPath)",
		"Add-RekitGoArg ([ref]$goArgs) '-ItemsFile' (Resolve-RekitCallerPath $ItemsFile)",
		"Add-RekitGoArg ([ref]$goArgs) '-Executor' $Executor",
		"Add-RekitGoArg ([ref]$goArgs) '-Reason' $Reason",
	} {
		assertTextContains(t, facade, required, "PowerShell facade freeze guard")
	}
	for _, required := range []string{
		"| `release-check` / `release-run` | Go default | façade delegate + no PowerShell fallback |",
		"| `status` / `packs` / `doctor` / `validate` | Go default | façade delegate + no PowerShell fallback |",
		"| case lifecycle `attach` / `repair` / `init` / `bootstrap` preview/apply | Go default | façade delegate + no PowerShell fallback |",
		"| `sync` / `update` review/apply/JSON preview | Go default | façade delegate + no PowerShell fallback |",
		"| `promote` review/artifacts/candidates/apply/JSON preview | Go default | façade delegate + no PowerShell fallback |",
		"| `overview` text/JSON 与缺 board 初始化 | Go default | façade delegate + no PowerShell fallback |",
		"| `note -List` text/table/tsv/JSON、`note` append、`note -WhatIf` | Go default | façade delegate + no PowerShell fallback |",
		"| `gate -WhatIf` / `gate -Apply` gate decision/evidence ledger | Go default | façade delegate + no PowerShell fallback |",
		"| `start` / `handoff` preview/apply/text/default | Go default | façade delegate + no PowerShell fallback |",
		"| `continue -WhatIf` / explicit `continue -Apply` text/JSON | Go default | façade delegate + no PowerShell fallback |",
		"| `reconcile -WhatIf` / explicit `reconcile -Apply` | Go default | façade delegate + no PowerShell fallback |",
		"| `plan-subagents` planning / reviewer intake | Go default | façade delegate + no PowerShell fallback |",
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

func assertTextNotContains(t *testing.T, text, forbidden, label string) {
	t.Helper()
	if strings.Contains(text, forbidden) {
		t.Fatalf("%s must not contain %q", label, forbidden)
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
