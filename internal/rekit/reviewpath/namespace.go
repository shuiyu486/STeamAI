package reviewpath

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
)

type CollectionNamespace struct {
	ReviewRoot string
	ResultRoot string
	PacketPath string
}

func CanonicalCollectionNamespace(caseRoot, packetPath string) (CollectionNamespace, bool) {
	caseRoot, err := filepath.Abs(caseRoot)
	if err != nil {
		return CollectionNamespace{}, false
	}
	packetPath, err = filepath.Abs(packetPath)
	if err != nil {
		return CollectionNamespace{}, false
	}
	reviewsRoot := filepath.Join(caseRoot, ".rekit", "reviews")
	rel, err := filepath.Rel(reviewsRoot, packetPath)
	if err != nil || filepath.IsAbs(rel) {
		return CollectionNamespace{}, false
	}
	parts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
	if len(parts) != 2 || parts[0] == "" || parts[0] == "." || parts[0] == ".." || parts[1] != "packet.json" {
		return CollectionNamespace{}, false
	}
	reviewRoot := filepath.Join(reviewsRoot, parts[0])
	expectedPacketPath := filepath.Join(reviewRoot, "packet.json")
	if !casebind.SamePath(packetPath, expectedPacketPath) {
		return CollectionNamespace{}, false
	}
	return CollectionNamespace{
		ReviewRoot: reviewRoot,
		ResultRoot: filepath.Join(reviewRoot, "results"),
		PacketPath: expectedPacketPath,
	}, true
}

func CollectionNamespacePathSafe(caseRoot, path string, allowMissingLeaf bool) bool {
	caseRoot, err := filepath.Abs(caseRoot)
	if err != nil {
		return false
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(caseRoot, path)
	if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	current := caseRoot
	parts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		st, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) && allowMissingLeaf {
			return true
		}
		if statErr != nil || st.Mode()&os.ModeSymlink != 0 {
			return false
		}
	}
	return true
}

func CanonicalCollectionShard(caseRoot, packetPath, resultRoot, shardID, candidatePath, resultPath string) bool {
	namespace, ok := CanonicalCollectionNamespace(caseRoot, packetPath)
	if !ok || !validPathSegment(shardID) || !casebind.SamePath(resultRoot, namespace.ResultRoot) {
		return false
	}
	return casebind.SamePath(candidatePath, filepath.Join(namespace.ResultRoot, "candidates", shardID+".json")) &&
		casebind.SamePath(resultPath, filepath.Join(namespace.ResultRoot, shardID+".json"))
}

func validPathSegment(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && value != "." && value != ".." && filepath.Base(value) == value
}
