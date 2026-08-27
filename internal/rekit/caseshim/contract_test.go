package caseshim

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/packidentity"
	"github.com/shuiyu486/re-context-kits/internal/rekit/sourceartifact"
)

func TestCurrentCanonicalShimIsAccepted(t *testing.T) {
	root := testRepoRoot(t)
	content, err := os.ReadFile(filepath.Join(root, "rekit", "templates", "case-shim", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		name    string
		content []byte
	}{
		{name: "checkout bytes", content: content},
		{name: "LF source", content: sourceartifact.SemanticText(content)},
		{name: "CRLF source", content: sourceartifact.CanonicalText(content)},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			canonical := sourceartifact.CanonicalText(fixture.content)
			sum := sha256.Sum256(canonical)
			actual := hex.EncodeToString(sum[:])
			if actual != CurrentCanonicalSHA256 {
				t.Fatalf("current canonical shim hash = %s, contract = %s", actual, CurrentCanonicalSHA256)
			}
			if err := ValidateSHA256(actual); err != nil {
				t.Fatal(err)
			}
		})
	}
	mutated := append([]byte{}, sourceartifact.CanonicalText(content)...)
	mutated = append(mutated, []byte("mutation\r\n")...)
	sum := sha256.Sum256(mutated)
	if err := ValidateSHA256(hex.EncodeToString(sum[:])); err == nil {
		t.Fatal("mutated canonical shim hash was accepted")
	}
}

func TestRetiredVMPRecoveryContractIsExactPreCutoverGeneration(t *testing.T) {
	const prefix = "references/vmp-re/"
	expected := map[string]PackRecoveryWrite{
		prefix + "README.md":                   generation("managed-file", "dbb5dcf99256d2ef8fecd46ea573483299f6547c7359c39b222aa41bb1f08a70"),
		prefix + "agent-driven-re.md":          generation("managed-file", "b52e2f367db70034f09b28820e1f99924987a3a9c21f4abd092c443910b24bf8"),
		prefix + "workflow-template.md":        generation("managed-file", "ea9b72db6a3820a6f2e5708a9a8d8afac5fc203b58a03fe232a5719b88198c61"),
		prefix + "progressive-disclosure.md":   generation("managed-file", "468ef2e1aa6d2bea6167bbdb8b163fcc8588661883ba6452e23c03fc7c864331"),
		prefix + "toolchain-router.md":         generation("managed-file", "816628c21cfa7df813a8c730486c9da0a7fc3f452affdf15d8405be94f750b9e"),
		prefix + "singleton-handler-review.md": generation("managed-file", "cbd2cae562d09e9a14f07454b5eb20c5d927a3d587670b571c2b00bc145dd23c"),
		prefix + "lane-collaboration.md":       generation("managed-file", "18d3963aefb6f8e9bd85c17724a12220451162b50c364e5e68ff488ca8c167b9"),
		prefix + "task-handoff.md":             generation("template-file", "6960d6ca8fad6697575c767b85e0acd8b49d5539c180b46921fdbde76ea94aa6"),
	}
	actual := PackRecoveryWrites(packidentity.RetiredVMP)
	if len(actual) != len(expected) {
		t.Fatalf("retired VMP recovery path count = %d, want %d: %+v", len(actual), len(expected), actual)
	}
	for path, want := range expected {
		got, ok := actual[path]
		if !ok || got.Kind != want.Kind || len(got.AcceptedSHA256s) != 1 {
			t.Fatalf("retired VMP recovery contract %s = %+v, want %+v", path, got, want)
		}
		for sum := range want.AcceptedSHA256s {
			if err := ValidatePackRecoveryWrite(packidentity.RetiredVMP, path, want.Kind, sum); err != nil {
				t.Fatalf("retired VMP generation %s rejected: %v", path, err)
			}
		}
	}
	if err := ValidatePackRecoveryWrite(packidentity.RetiredVMP, "references/binary-re/README.md", "managed-file", "dbb5dcf99256d2ef8fecd46ea573483299f6547c7359c39b222aa41bb1f08a70"); err == nil {
		t.Fatal("retired VMP recovery accepted canonical binary-re path as an alias")
	}
	if _, ok := actual[prefix+"general-analysis.md"]; ok {
		t.Fatal("retired VMP recovery accepted a post-cutover canonical-only generation")
	}
	if err := ValidateManagedBlockSHA256(packidentity.RetiredVMP, "f12e9febbf923f815f1113f48b2b31e20137a77fb640a12db341dac9e8e4988e"); err != nil {
		t.Fatalf("retired VMP managed block generation rejected: %v", err)
	}
	paths := ExpectedSupportPaths(packidentity.RetiredVMP)
	if len(paths) != 1 || paths[0] != ".gitignore" {
		t.Fatalf("retired VMP support paths = %v, want [.gitignore]", paths)
	}
	if err := ValidateSupportSHA256(packidentity.RetiredVMP, ".gitignore", "1f2e0c6e920633ca91511262703bb5e9c00ddc74657b0a7fd21ea372bc8fef96"); err != nil {
		t.Fatalf("retired VMP support generation rejected: %v", err)
	}
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
