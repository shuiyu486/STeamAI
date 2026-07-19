package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
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
	if _, err := casebind.WriteInstance(caseRoot, repoRoot, "_template", "caller-cwd-product-path-case"); err != nil {
		t.Fatal(err)
	}

	ctx, err := NewWithCwd("", "_template", nested)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.RepoRoot != repoRoot || ctx.RuntimeRoot != filepath.Join(repoRoot, "rekit") || ctx.Cwd != nested || ctx.Target != caseRoot || ctx.TargetProvided || ctx.Pack != "_template" {
		t.Fatalf("unexpected caller cwd runtime context: %+v", ctx)
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
	if _, err := casebind.WriteInstance(caseRoot, repoRoot, "_template", "product-path-case"); err != nil {
		t.Fatal(err)
	}
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
	if ctx.RepoRoot != repoRoot || ctx.RuntimeRoot != filepath.Join(repoRoot, "rekit") || ctx.Target != caseRoot || !ctx.TargetProvided || ctx.Pack != "_template" {
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
	if _, err := casebind.WriteInstance(caseRoot, repoRoot, "_template", "nested-product-path-case"); err != nil {
		t.Fatal(err)
	}
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
	if ctx.RepoRoot != repoRoot || ctx.RuntimeRoot != filepath.Join(repoRoot, "rekit") || ctx.Cwd != nested || ctx.Target != caseRoot || ctx.TargetProvided || ctx.Pack != "_template" {
		t.Fatalf("unexpected nested case runtime context: %+v", ctx)
	}
}
