package runtime

import "testing"

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
	if ctx.Pack != "vmp-re" {
		t.Fatalf("Pack = %q, want vmp-re", ctx.Pack)
	}
	if ctx.TargetProvided {
		t.Fatal("TargetProvided = true, want false")
	}
}
