package runtime

import (
	"testing"

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
