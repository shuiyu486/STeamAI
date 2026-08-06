package laneid

import (
	"regexp"
	"strings"
)

var unsafeID = regexp.MustCompile(`[^a-z0-9._-]+`)

func Resolve(laneType, name string) string {
	raw := laneType
	if strings.TrimSpace(name) != "" {
		if strings.Contains(laneType, "feature") {
			raw = "feature-" + name
		} else {
			raw = laneType + "-" + name
		}
	}
	return normalize(raw)
}

func Label(laneType, id string) (string, bool) {
	id = strings.TrimSpace(id)
	prefix := normalize(laneType) + "-"
	if strings.Contains(laneType, "feature") {
		prefix = "feature-"
	}
	if !strings.HasPrefix(id, prefix) || len(id) == len(prefix) {
		return "", false
	}
	label := strings.TrimPrefix(id, prefix)
	return label, Resolve(laneType, label) == id
}

func normalize(value string) string {
	safe := unsafeID.ReplaceAllString(strings.ToLower(strings.TrimSpace(value)), "-")
	return strings.Trim(safe, "-_.")
}
