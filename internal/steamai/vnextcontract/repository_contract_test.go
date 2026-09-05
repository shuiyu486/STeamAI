package vnextcontract

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRepositoryHasOnlyTheNativeWindowsProductShell(t *testing.T) {
	repo := repoRoot(t)
	for _, rel := range []string{
		"internal/rekit",
		"rekit",
		".claude/skills/rekit",
		".claude/skills/verify",
		"go.work",
		"packs/binary-re/scripts",
	} {
		if _, err := os.Lstat(filepath.Join(repo, filepath.FromSlash(rel))); err == nil {
			t.Errorf("legacy production path still exists: %s", rel)
		} else if !os.IsNotExist(err) {
			t.Fatalf("inspect %s: %v", rel, err)
		}
	}

	allowedProductionGo := map[string]bool{
		"cmd/steamai/main.go":                                true,
		"cmd/steamai-release-manifest/main.go":               true,
		"internal/steamai/app.go":                            true,
		"internal/steamai/path.go":                           true,
		"internal/steamai/platform_other.go":                 true,
		"internal/steamai/platform_windows.go":               true,
		"internal/steamai/release_manifest.go":               true,
		"internal/steamai/setup.go":                          true,
		"internal/steamai/update.go":                         true,
		"internal/steamai/uninstall_cleanup_other.go":        true,
		"internal/steamai/uninstall_cleanup_windows.go":      true,
		"internal/steamai/casebootstrap/apply.go":            true,
		"internal/steamai/casebootstrap/current.go":          true,
		"internal/steamai/casebootstrap/path.go":             true,
		"internal/steamai/casebootstrap/preview.go":          true,
		"internal/steamai/casebootstrap/reparse_other.go":    true,
		"internal/steamai/casebootstrap/reparse_windows.go":  true,
		"internal/steamai/casebootstrap/source.go":           true,
		"internal/steamai/casebootstrap/types.go":            true,
		"internal/steamai/evaluation_cli.go":                 true,
		"internal/steamai/evaluation/reparse_other.go":       true,
		"internal/steamai/evaluation/reparse_windows.go":     true,
		"internal/steamai/evaluation/publish_linux.go":       true,
		"internal/steamai/evaluation/publish_other.go":       true,
		"internal/steamai/evaluation/publish_windows.go":     true,
		"internal/steamai/evaluation/process_other.go":       true,
		"internal/steamai/evaluation/process_windows.go":     true,
		"internal/steamai/evaluation/review_packet.go":       true,
		"internal/steamai/evaluation/run.go":                 true,
		"internal/steamai/evaluation/suite.go":               true,
		"internal/steamai/evaluation/suite_spec.go":          true,
		"internal/steamai/evaluation/types.go":               true,
		"internal/steamai/evaluation/verify.go":              true,
		"internal/steamai/learningbatch/apply.go":            true,
		"internal/steamai/learningbatch/gate3.go":            true,
		"internal/steamai/learningbatch/path.go":             true,
		"internal/steamai/learningbatch/platform_other.go":   true,
		"internal/steamai/learningbatch/platform_windows.go": true,
		"internal/steamai/learningbatch/preview.go":          true,
		"internal/steamai/learningbatch/sourcechain.go":      true,
		"internal/steamai/learningbatch/types.go":            true,
	}
	for path := range allowedProductionGo {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path))); err != nil {
			t.Errorf("required native product shell file is missing: %s: %v", path, err)
		}
	}
	for _, rel := range []string{
		"internal/steamai/uninstall_cleanup_windows.go",
		"internal/steamai/platform_windows.go",
	} {
		data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, forbidden := range []string{"power" + "shell.exe", "cmd" + ".exe", "MoveFile" + "Ex"} {
			if strings.Contains(text, forbidden) {
				t.Errorf("native uninstall path uses forbidden external runtime %q in %s", forbidden, rel)
			}
		}
	}

	for _, root := range []string{"cmd", "internal/steamai"} {
		err := filepath.WalkDir(filepath.Join(repo, filepath.FromSlash(root)), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(repo, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if !allowedProductionGo[rel] {
				t.Errorf("production Go escaped the native product shell allowlist: %s", rel)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	tracked, err := exec.Command("git", "-C", repo, "ls-files", "-z").Output()
	if err != nil {
		t.Fatalf("list tracked files: %v", err)
	}
	for raw := range strings.SplitSeq(string(tracked), "\x00") {
		ext := strings.ToLower(filepath.Ext(raw))
		if slices.Contains([]string{".ps1", ".psm1", ".psd1", ".cmd", ".bat"}, ext) {
			t.Errorf("tracked product script is forbidden: %s", raw)
		}
	}
}

func TestCanonicalCurrentSurfacesDoNotInvokeRemovedRuntime(t *testing.T) {
	repo := repoRoot(t)
	paths := []string{
		"README.md",
		"CLAUDE.md",
		"docs/context-routing.md",
		".github/workflows/release-gate.yml",
		"common/stop-hook-checklist.md",
		"vnext/README.md",
	}
	forbidden := []string{
		"go run ./cmd/rekit",
		"go run ./cmd/rekit-host",
		"cmd/skillcontractgen",
		"./rekit/rekit.ps1",
		"/rekit status",
		"/rekit continue",
		"/rekit gate",
		"/rekit doctor",
		"/rekit promote",
		".rekit/facts",
		".rekit/lanes",
		"authorized-gate",
		"project-local deterministic runtime",
		"legacy-import.md",
		"一次性只读 importer",
		"legacy importer",
	}
	for _, rel := range paths {
		data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		text := string(data)
		for _, phrase := range forbidden {
			if strings.Contains(text, phrase) {
				t.Errorf("current surface %s invokes removed runtime or importer phrase %q", rel, phrase)
			}
		}
	}

	for _, removed := range []string{
		"vnext/legacy-import.md",
		"internal/steamai/vnextcontract/legacy_import_test.go",
	} {
		if _, err := os.Lstat(filepath.Join(repo, filepath.FromSlash(removed))); err == nil {
			t.Errorf("removed legacy compatibility surface still exists: %s", removed)
		} else if !os.IsNotExist(err) {
			t.Fatalf("inspect removed compatibility surface %s: %v", removed, err)
		}
	}
}

func TestPackManifestV2Semantics(t *testing.T) {
	repo := repoRoot(t)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git for tracked-path checks: %v", err)
	}
	packsRoot := filepath.Join(repo, "packs")
	entries, err := os.ReadDir(packsRoot)
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{
		"workstreamDefaults:",
		"authorityFiles:",
		"subagentRoutes:",
		"laneTypes:",
		"mainAgentOwns:",
		"learningDenyPatterns:",
		"registry:",
		"overlays:",
		".rekit/",
		".steamai/facts/",
		"authorized-gate",
		"ledger-writeback",
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pack := entry.Name()
		rel := filepath.ToSlash(filepath.Join("packs", pack, "manifest.yml"))
		data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(rel)))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		text := string(data)
		if !strings.HasPrefix(text, "schemaVersion: 2\n") {
			t.Errorf("thin pack manifest %s must start with schemaVersion 2", rel)
		}
		for _, key := range []string{
			"schemaVersion", "name", "version", "description", "maturity", "entrypoints", "references", "templates", "policies", "tooling",
			"teamHints", "heavyActions", "learningTargets", "denyPatterns", "budgets",
		} {
			if count := countManifestKeyAtIndent(text, 0, key); count != 1 {
				t.Errorf("thin pack manifest %s top-level key %s count = %d, want 1", rel, key, count)
			}
		}
		for _, required := range []string{
			"ownerPerQuestion: 1", "verifierPerQuestion: 1", "durableTeamLimit: 3-executors-plus-1-reviewer", "requiredToolPermission: true",
		} {
			if !strings.Contains(text, required) {
				t.Errorf("thin pack manifest %s missing %q", rel, required)
			}
		}
		for _, phrase := range forbidden {
			if strings.Contains(text, phrase) {
				t.Errorf("thin pack manifest %s retains unsupported field %q", rel, phrase)
			}
		}
		for _, section := range []string{"references", "templates", "policies", "tooling"} {
			for _, declared := range manifestListValues(text, section) {
				assertDeclaredPackPath(t, repo, pack, rel, section, declared)
			}
		}
		learningTargets := manifestListValues(text, "learningTargets")
		if len(learningTargets) == 0 {
			t.Errorf("thin pack manifest %s has no learning targets", rel)
		}
		for _, target := range learningTargets {
			if filepath.Ext(target) != ".md" && !strings.HasSuffix(target, "*.md") {
				t.Errorf("thin pack manifest %s learning target is not Markdown: %s", rel, target)
				continue
			}
			matches, err := filepath.Glob(filepath.Join(packsRoot, pack, filepath.FromSlash(target)))
			if err != nil || len(matches) == 0 {
				t.Errorf("thin pack manifest %s learning target expands to no files: %s", rel, target)
				continue
			}
			for _, match := range matches {
				assertTrackedRegularFile(t, git, repo, match)
			}
		}
		if len(manifestListValues(text, "denyPatterns")) == 0 {
			t.Errorf("thin pack manifest %s has no deny patterns", rel)
		}
	}
}

func countManifestKeyAtIndent(text string, indent int, key string) int {
	prefix := strings.Repeat(" ", indent) + key + ":"
	count := 0
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(line, prefix) && (line == prefix || strings.HasPrefix(line, prefix+" ")) {
			count++
		}
	}
	return count
}

func manifestListValues(text, section string) []string {
	lines := strings.Split(text, "\n")
	start := -1
	for i, line := range lines {
		if line == section+":" {
			if start >= 0 {
				return nil
			}
			start = i + 1
		}
	}
	if start < 0 {
		return nil
	}
	var values []string
	for _, line := range lines[start:] {
		if line == "" || strings.HasPrefix(line, "  #") {
			continue
		}
		if !strings.HasPrefix(line, " ") {
			break
		}
		if strings.HasPrefix(line, "    ") || !strings.HasPrefix(line, "  - ") {
			return nil
		}
		value := strings.Trim(strings.TrimPrefix(line, "  - "), "'\"")
		if value == "" || strings.Contains(value, " #") {
			return nil
		}
		values = append(values, value)
	}
	return values
}

func assertDeclaredPackPath(t *testing.T, repo, pack, manifestRel, section, declared string) {
	t.Helper()
	packRoot := filepath.Join(repo, "packs", pack)
	candidate := filepath.Clean(filepath.Join(packRoot, filepath.FromSlash(declared)))
	allowedRoot := packRoot
	if section == "policies" && strings.HasPrefix(declared, "../../common/policies/") {
		allowedRoot = filepath.Join(repo, "common", "policies")
	}
	if !pathWithin(candidate, allowedRoot) {
		t.Errorf("thin pack manifest %s %s path escapes allowed root: %s", manifestRel, section, declared)
		return
	}
	assertRegularNonSymlink(t, candidate, manifestRel+" "+section+" path")
}

func assertTrackedRegularFile(t *testing.T, git, repo, path string) {
	t.Helper()
	assertRegularNonSymlink(t, path, "learning target")
	rel, err := filepath.Rel(repo, path)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(git, "ls-files", "--error-unmatch", "--", filepath.ToSlash(rel))
	cmd.Dir = repo
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("learning target is not tracked %s: %v: %s", filepath.ToSlash(rel), err, strings.TrimSpace(string(output)))
	}
}

func assertRegularNonSymlink(t *testing.T, path, label string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Errorf("%s is missing: %s: %v", label, path, err)
		return
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		t.Errorf("%s is not a regular non-symlink file: %s", label, path)
	}
}

func TestPackAndCommonSourcesDoNotExposeLegacyCommands(t *testing.T) {
	repo := repoRoot(t)
	roots := []string{"common", "packs"}
	forbidden := []string{
		"/rekit ",
		"`.rekit/",
		".rekit/facts",
		".rekit/lanes",
		"/steamai doctor",
		"/steamai promote",
		"/steamai sync",
		"authorized-gate",
		"strict durable autonomy profile",
		"go run ./cmd/",
		"runtime bundle",
	}
	for _, root := range roots {
		err := filepath.WalkDir(filepath.Join(repo, root), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if !slices.Contains([]string{".md", ".yml", ".yaml", ".json"}, ext) {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, phrase := range forbidden {
				if strings.Contains(string(data), phrase) {
					rel, _ := filepath.Rel(repo, path)
					t.Errorf("current declarative source %s retains legacy phrase %q", filepath.ToSlash(rel), phrase)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
