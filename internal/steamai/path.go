package steamai

import (
	"path/filepath"
	"strings"
)

func pathListContains(list, candidate string) bool {
	_, found := removePathListEntry(list, candidate)
	return found
}

func setupPathDecision(currentPath, binDir, oldOwnership string, oldOwnershipExists bool) (owned, add bool) {
	present := pathListContains(currentPath, binDir)
	owned = oldOwnershipExists && oldOwnership == "true"
	add = !present
	if add {
		owned = true
	}
	return owned, add
}

func removePathListEntry(list, candidate string) (string, bool) {
	candidate = strings.TrimRight(filepath.Clean(candidate), `\/`)
	entries := strings.Split(list, ";")
	kept := make([]string, 0, len(entries))
	found := false
	for _, raw := range entries {
		entry := strings.TrimSpace(strings.Trim(raw, `"`))
		if entry != "" && strings.EqualFold(strings.TrimRight(filepath.Clean(entry), `\/`), candidate) {
			found = true
			continue
		}
		if entry != "" {
			kept = append(kept, raw)
		}
	}
	return strings.Join(kept, ";"), found
}
