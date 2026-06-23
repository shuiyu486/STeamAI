package review

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"regexp"
	"strings"
)

type Item struct {
	Path                     string         `json:"path"`
	Kind                     string         `json:"kind"`
	Direction                string         `json:"direction"`
	Action                   string         `json:"action"`
	RiskLevel                string         `json:"riskLevel"`
	SourceHash               string         `json:"sourceHash,omitempty"`
	TargetHash               string         `json:"targetHash,omitempty"`
	CaseHash                 string         `json:"caseHash,omitempty"`
	PackHash                 string         `json:"packHash,omitempty"`
	Changed                  bool           `json:"changed,omitempty"`
	DenyViolations           []string       `json:"denyViolations,omitempty"`
	ReplacementCounts        map[string]int `json:"replacementCounts,omitempty"`
	MechanicalRecommendation string         `json:"mechanicalRecommendation,omitempty"`
}

type Summary struct {
	Changed        int  `json:"changed"`
	Blocked        int  `json:"blocked"`
	ReviewRequired bool `json:"reviewRequired"`
}

type Plan struct {
	SchemaVersion   int     `json:"schemaVersion"`
	Command         string  `json:"command"`
	Direction       string  `json:"direction"`
	CaseRoot        string  `json:"caseRoot"`
	RepoRoot        string  `json:"repoRoot"`
	Pack            string  `json:"pack"`
	ManifestPath    string  `json:"manifestPath"`
	ManifestVersion string  `json:"manifestVersion"`
	IsMutation      bool    `json:"isMutation"`
	Summary         Summary `json:"summary"`
	Items           []Item  `json:"items"`
	ToolingItems    []Item  `json:"toolingCandidateSources,omitempty"`
}

func (p Plan) ChangedItems() int {
	count := 0
	for _, item := range p.Items {
		switch item.Action {
		case "unchanged", "skip-existing-local-file", "skip-existing-support-file", "skip-missing-case-file", "skip-non-managed-promote-file":
			continue
		default:
			count++
		}
	}
	return count
}

func (p Plan) BlockedItems() int {
	count := 0
	for _, item := range p.Items {
		if strings.HasPrefix(item.Action, "blocked") {
			count++
		}
	}
	for _, item := range p.ToolingItems {
		if strings.HasPrefix(item.Action, "blocked") {
			count++
		}
	}
	return count
}

func ReadTextIfExists(path string) (string, bool, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		return string(b), true, nil
	}
	if os.IsNotExist(err) {
		return "", false, nil
	}
	return "", false, err
}

func FileHash(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func MatchAny(text string, patterns []string) []string {
	violations := []string{}
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		re, err := regexp.Compile("(?i:" + pattern + ")")
		if err != nil {
			if strings.Contains(strings.ToLower(text), strings.ToLower(pattern)) {
				violations = append(violations, pattern)
			}
			continue
		}
		if re.MatchString(text) {
			violations = append(violations, pattern)
		}
	}
	return violations
}

func ApplyManagedBlock(hostText, blockID, blockText string) string {
	block := strings.TrimSpace(blockText)
	if strings.TrimSpace(hostText) == "" {
		return "# Project Context\r\n\r\n" + block + "\r\n"
	}
	pattern := `(?s)<!-- BEGIN ` + regexp.QuoteMeta(blockID) + `.*?<!-- END ` + regexp.QuoteMeta(blockID) + ` -->`
	re := regexp.MustCompile(pattern)
	if re.MatchString(hostText) {
		return re.ReplaceAllStringFunc(hostText, func(string) string { return block })
	}
	return strings.TrimRight(hostText, "\r\n\t ") + "\r\n\r\n" + block + "\r\n"
}
