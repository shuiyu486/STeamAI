package caseshim

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCurrentCanonicalShimIsAccepted(t *testing.T) {
	root := testRepoRoot(t)
	content, err := os.ReadFile(filepath.Join(root, "rekit", "templates", "case-shim", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	actual := hex.EncodeToString(sum[:])
	if actual != CurrentCanonicalSHA256 {
		t.Fatalf("current canonical shim hash = %s, contract = %s", actual, CurrentCanonicalSHA256)
	}
	if err := ValidateSHA256(actual); err != nil {
		t.Fatal(err)
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
