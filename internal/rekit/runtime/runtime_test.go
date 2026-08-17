package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
)

func TestNewDiscoversRepoRoot(t *testing.T) {
	ctx, err := New("", "")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.RepoRoot == "" {
		t.Fatal("RepoRoot is empty")
	}
	if ctx.RuntimeRoot == "" {
		t.Fatal("RuntimeRoot is empty")
	}
	if ctx.Pack != defaults.DefaultPack {
		t.Fatalf("Pack = %q, want %s", ctx.Pack, defaults.DefaultPack)
	}
	if ctx.TargetProvided {
		t.Fatal("TargetProvided = true, want false")
	}
}

func TestFindRepoRootUsesGoNativeIdentityWithoutFacade(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/shuiyu486/re-context-kits\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "packs"), 0o700); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "internal", "rekit", "runtime")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	found, ok := findRepoRoot(nested)
	if !ok || found != root {
		t.Fatalf("Go-native repo root=%q found=%t, want %q", found, ok, root)
	}
	if _, err := os.Lstat(filepath.Join(root, "rekit", "rekit.ps1")); !os.IsNotExist(err) {
		t.Fatalf("fixture unexpectedly depends on compatibility facade: %v", err)
	}
}

func TestFindRepoRootRejectsUnrelatedGoModule(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.invalid/not-rekit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "packs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if found, ok := findRepoRoot(root); ok || found != "" {
		t.Fatalf("unrelated module accepted as repo root: %q", found)
	}
}

func publishRuntimeBundleFixture(t *testing.T, caseRoot, repoRoot, pack, projectName string) {
	t.Helper()
	executable := filepath.Join(t.TempDir(), runtimebundle.ExecutableName())
	if err := os.WriteFile(executable, []byte("fixture runtime executable\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	plan, err := runtimebundle.PublishForTest(caseRoot, repoRoot, pack, executable)
	if err != nil {
		t.Fatal(err)
	}
	metadata := casebind.STeamAIInstanceText(caseRoot, pack, projectName, runtimebundle.ManifestRel, plan.ManifestSHA256)
	path := filepath.Join(caseRoot, ".steamai", "instance.yml")
	if err := os.WriteFile(path, []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestValidateExecutableBindsSchemaV2ManifestAndProcessIdentity(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	caseRoot := t.TempDir()
	publishRuntimeBundleFixture(t, caseRoot, repoRoot, "_template", "process-binding")
	executable := filepath.Join(caseRoot, ".steamai", "runtime", "bin", runtimebundle.ExecutableName())
	if err := ValidateExecutable(executable); err != nil {
		t.Fatal(err)
	}
	metadataPath := filepath.Join(caseRoot, ".steamai", "instance.yml")
	metadata, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metadataPath, []byte(strings.Replace(string(metadata), "bundleManifestSHA256: ", "bundleManifestSHA256: "+strings.Repeat("0", 64)+" # ", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateExecutable(executable); err == nil {
		t.Fatal("project-local executable ignored schema v2 manifest binding tamper")
	}
}

func TestNewWithCwdUsesCallerCwdOverride(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	caseRoot := filepath.Join(t.TempDir(), "case")
	nested := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	publishRuntimeBundleFixture(t, caseRoot, repoRoot, "_template", "caller-cwd-product-path-case")

	ctx, err := NewWithCwd("", "_template", nested)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.RepoRoot != filepath.Join(caseRoot, ".steamai") || ctx.RuntimeRoot != filepath.Join(caseRoot, ".steamai", "runtime") || ctx.Cwd != nested || ctx.Target != caseRoot || ctx.TargetProvided || ctx.Pack != "_template" {
		t.Fatalf("unexpected caller cwd runtime context: %+v", ctx)
	}
}

func TestNewDiscoversProjectLocalSourceRuntime(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	caseRoot := t.TempDir()
	publishRuntimeBundleFixture(t, caseRoot, repoRoot, "_template", "self-contained")
	outside := t.TempDir()

	ctx, err := NewWithCwd(caseRoot, "_template", outside)
	if err != nil {
		t.Fatal(err)
	}
	assetRoot := filepath.Join(caseRoot, ".steamai")
	if ctx.RepoRoot != assetRoot || ctx.RuntimeRoot != filepath.Join(assetRoot, "runtime") || ctx.Target != caseRoot {
		t.Fatalf("unexpected self-contained runtime context: %+v", ctx)
	}
	if _, err := os.Lstat(filepath.Join(assetRoot, "runtime", "packs")); !os.IsNotExist(err) {
		t.Fatalf("bundle duplicated packs beneath runtime: %v", err)
	}
}

func TestNewCurrentTargetOverridesCentralKitCwd(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	caseRoot := t.TempDir()
	publishRuntimeBundleFixture(t, caseRoot, repoRoot, "_template", "target-first")

	ctx, err := NewWithCwd(caseRoot, "_template", repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.RepoRoot != filepath.Join(caseRoot, ".steamai") || ctx.RuntimeRoot != filepath.Join(caseRoot, ".steamai", "runtime") {
		t.Fatalf("central cwd overrode current project bundle: %+v", ctx)
	}
}

func TestNewRejectsNestedDualRootWithoutSelectingAncestor(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	ancestor := filepath.Join(t.TempDir(), "ancestor")
	publishRuntimeBundleFixture(t, ancestor, repoRoot, "_template", "ancestor")
	conflict := filepath.Join(ancestor, "nested-conflict")
	for _, dir := range []string{".steamai", ".rekit"} {
		if err := os.MkdirAll(filepath.Join(conflict, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cwd := filepath.Join(conflict, "workspace")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := NewWithCwd("", "_template", cwd); err == nil {
		t.Fatal("nested dual-root conflict was skipped in favor of ancestor case")
	}
}

func TestNewDiscoversRepoRootFromTargetCaseMetadata(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	caseRoot := filepath.Join(t.TempDir(), "case")
	if err := os.MkdirAll(caseRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	publishRuntimeBundleFixture(t, caseRoot, repoRoot, "_template", "product-path-case")
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(outside); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatal(err)
		}
	})

	ctx, err := New(caseRoot, "_template")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.RepoRoot != filepath.Join(caseRoot, ".steamai") || ctx.RuntimeRoot != filepath.Join(caseRoot, ".steamai", "runtime") || ctx.Target != caseRoot || !ctx.TargetProvided || ctx.Pack != "_template" {
		t.Fatalf("unexpected runtime context: %+v", ctx)
	}
}

func TestNewUsesNearestCaseRootFromNestedCaseWorkingDirectory(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	caseRoot := filepath.Join(t.TempDir(), "case")
	nested := filepath.Join(caseRoot, "workspace", "features", "login")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	publishRuntimeBundleFixture(t, caseRoot, repoRoot, "_template", "nested-product-path-case")
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatal(err)
		}
	})

	ctx, err := New("", "_template")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.RepoRoot != filepath.Join(caseRoot, ".steamai") || ctx.RuntimeRoot != filepath.Join(caseRoot, ".steamai", "runtime") || ctx.Cwd != nested || ctx.Target != caseRoot || ctx.TargetProvided || ctx.Pack != "_template" {
		t.Fatalf("unexpected nested case runtime context: %+v", ctx)
	}

	ctx, err = New("", "")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.RepoRoot != filepath.Join(caseRoot, ".steamai") || ctx.RuntimeRoot != filepath.Join(caseRoot, ".steamai", "runtime") || ctx.Cwd != nested || ctx.Target != caseRoot || ctx.TargetProvided || ctx.Pack != "_template" {
		t.Fatalf("unexpected nested case runtime context with default pack: %+v", ctx)
	}
}
