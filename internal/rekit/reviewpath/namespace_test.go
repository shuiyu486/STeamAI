package reviewpath

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCanonicalCollectionNamespace(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "case")
	canonicalPacket := filepath.Join(caseRoot, ".rekit", "reviews", "review-01", "packet.json")
	namespace, ok := CanonicalCollectionNamespace(caseRoot, canonicalPacket)
	if !ok || namespace.PacketPath != canonicalPacket || namespace.ResultRoot != filepath.Join(filepath.Dir(canonicalPacket), "results") {
		t.Fatalf("canonical packet rejected: namespace=%+v ok=%t", namespace, ok)
	}
	for _, packetPath := range []string{
		filepath.Join(caseRoot, ".rekit", "reviews", "review-01", "custom.json"),
		filepath.Join(caseRoot, ".rekit", "reviews", "review-01", "Packet.JSON"),
		filepath.Join(caseRoot, ".rekit", "reviews", "review-01", "nested", "packet.json"),
		filepath.Join(caseRoot, "artifacts", "review", "packet.json"),
	} {
		if _, ok := CanonicalCollectionNamespace(caseRoot, packetPath); ok {
			t.Fatalf("noncanonical packet accepted: %s", packetPath)
		}
	}
}

func TestCollectionNamespacePathSafeRejectsSymlinkAncestors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not reliably available on Windows")
	}
	caseRoot := filepath.Join(t.TempDir(), "case")
	outside := filepath.Join(t.TempDir(), "metadata")
	if err := os.MkdirAll(filepath.Join(outside, "reviews", "review-01"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(caseRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(caseRoot, ".rekit")); err != nil {
		t.Fatal(err)
	}
	packetPath := filepath.Join(caseRoot, ".rekit", "reviews", "review-01", "packet.json")
	if CollectionNamespacePathSafe(caseRoot, packetPath, true) {
		t.Fatal(".rekit symlink ancestor was accepted")
	}
}

func TestCanonicalCollectionShard(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "case")
	packetPath := filepath.Join(caseRoot, ".rekit", "reviews", "review-01", "packet.json")
	resultRoot := filepath.Join(filepath.Dir(packetPath), "results")
	candidatePath := filepath.Join(resultRoot, "candidates", "shard-01.json")
	resultPath := filepath.Join(resultRoot, "shard-01.json")
	if !CanonicalCollectionShard(caseRoot, packetPath, resultRoot, "shard-01", candidatePath, resultPath) {
		t.Fatal("canonical shard geometry rejected")
	}
	if CanonicalCollectionShard(caseRoot, packetPath, resultRoot, "shard-01", filepath.Join(resultRoot, "candidate.json"), resultPath) {
		t.Fatal("noncanonical candidate accepted")
	}
}
