package caseshim

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"

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

func testRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
