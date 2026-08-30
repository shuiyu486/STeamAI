package reviewpath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
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
	view, err := projectstate.ResolveMissionView(caseRoot)
	if err != nil {
		return CollectionNamespace{}, false
	}
	reviewsRoot := filepath.Join(view.Path, "reviews")
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

func PlannedResultSnapshotPath(caseRoot, dispatchID string) (string, error) {
	dispatchID = strings.TrimSpace(dispatchID)
	if !validPathSegment(dispatchID) || strings.ContainsAny(dispatchID, `/\`) {
		return "", fmt.Errorf("planned reviewer result snapshot requires an exact dispatch ID")
	}
	view, err := projectstate.ResolveMissionView(caseRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(view.Path, "reviews", "planned-snapshots", dispatchID+".json"), nil
}

func CollectionNamespacePathSafe(caseRoot, path string, allowMissingLeaf bool) bool {
	caseRoot, err := filepath.Abs(caseRoot)
	if err != nil {
		return false
	}
	view, err := projectstate.ResolveMissionView(caseRoot)
	if err != nil {
		return false
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return false
	}
	reviewsRoot := filepath.Join(view.Path, "reviews")
	reviewsRel, err := filepath.Rel(reviewsRoot, path)
	if err != nil || filepath.IsAbs(reviewsRel) || reviewsRel == ".." || strings.HasPrefix(reviewsRel, ".."+string(filepath.Separator)) {
		return false
	}
	rel, err := filepath.Rel(caseRoot, path)
	if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	current := caseRoot
	for part := range strings.SplitSeq(filepath.Clean(rel), string(filepath.Separator)) {
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
