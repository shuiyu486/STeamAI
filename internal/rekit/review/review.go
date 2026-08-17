package review

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

type Item struct {
	Path                     string         `json:"path"`
	Kind                     string         `json:"kind"`
	Direction                string         `json:"direction"`
	Action                   string         `json:"action"`
	RiskLevel                string         `json:"riskLevel"`
	SourcePath               string         `json:"sourcePath,omitempty"`
	TargetPath               string         `json:"targetPath,omitempty"`
	CasePath                 string         `json:"casePath,omitempty"`
	PackPath                 string         `json:"packPath,omitempty"`
	BlockID                  string         `json:"blockId,omitempty"`
	SourceHash               string         `json:"sourceHash,omitempty"`
	TargetHash               string         `json:"targetHash,omitempty"`
	CaseHash                 string         `json:"caseHash,omitempty"`
	PackHash                 string         `json:"packHash,omitempty"`
	Changed                  bool           `json:"changed,omitempty"`
	DenyViolations           []string       `json:"denyViolations,omitempty"`
	ReplacementCounts        map[string]int `json:"replacementCounts,omitempty"`
	MechanicalRecommendation string         `json:"mechanicalRecommendation,omitempty"`
	DiffPath                 string         `json:"diffPath,omitempty"`
	SanitizedPreviewPath     string         `json:"sanitizedPreviewPath,omitempty"`
	PlannedText              string         `json:"-"`
	SanitizedPreviewText     string         `json:"-"`
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
	ReviewRoot      string  `json:"reviewRoot,omitempty"`
	CreatedAt       string  `json:"createdAt,omitempty"`
	IsMutation      bool    `json:"isMutation"`
	Summary         Summary `json:"summary"`
	Items           []Item  `json:"items"`
	ToolingItems    []Item  `json:"toolingCandidateSources,omitempty"`
}

type ArtifactOptions struct {
	ReviewOutputDir string
	PacketPath      string
	DiffPath        string
}

type ArtifactResult struct {
	SchemaVersion    int    `json:"schemaVersion"`
	Command          string `json:"command"`
	CaseRoot         string `json:"caseRoot"`
	RepoRoot         string `json:"repoRoot"`
	Pack             string `json:"pack"`
	IsMutation       bool   `json:"isMutation"`
	WritesArtifacts  bool   `json:"writesArtifacts"`
	ReviewRoot       string `json:"reviewRoot"`
	PacketPath       string `json:"packetPath"`
	SummaryPath      string `json:"summaryPath"`
	CombinedDiffPath string `json:"combinedDiffPath"`
}

type artifactPaths struct {
	Root             string
	DiffRoot         string
	PreviewRoot      string
	PacketPath       string
	SummaryPath      string
	CombinedDiffPath string
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

func WriteArtifacts(plan Plan, opts ArtifactOptions) (ArtifactResult, error) {
	paths, err := reviewPaths(plan, opts)
	if err != nil {
		return ArtifactResult{}, err
	}
	if err := os.MkdirAll(paths.DiffRoot, 0o755); err != nil {
		return ArtifactResult{}, err
	}
	if err := os.MkdirAll(paths.PreviewRoot, 0o755); err != nil {
		return ArtifactResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(paths.PacketPath), 0o755); err != nil {
		return ArtifactResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(paths.CombinedDiffPath), 0o755); err != nil {
		return ArtifactResult{}, err
	}
	_ = os.Remove(paths.CombinedDiffPath)
	if err := os.WriteFile(paths.CombinedDiffPath, []byte{}, 0o644); err != nil {
		return ArtifactResult{}, err
	}

	for i := range plan.Items {
		if err := writeItemArtifact(paths, plan.Command, &plan.Items[i]); err != nil {
			return ArtifactResult{}, err
		}
	}
	for i := range plan.ToolingItems {
		if err := writeToolingArtifact(paths, &plan.ToolingItems[i]); err != nil {
			return ArtifactResult{}, err
		}
	}

	plan.IsMutation = false
	plan.Summary = Summary{Changed: plan.ChangedItems(), Blocked: plan.BlockedItems(), ReviewRequired: true}
	plan.ReviewRoot = paths.Root
	plan.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)

	packet, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return ArtifactResult{}, err
	}
	if err := os.WriteFile(paths.PacketPath, append(packet, '\n'), 0o644); err != nil {
		return ArtifactResult{}, err
	}
	if err := os.WriteFile(paths.SummaryPath, []byte(summaryText(plan)), 0o644); err != nil {
		return ArtifactResult{}, err
	}

	return ArtifactResult{SchemaVersion: 1, Command: plan.Command, CaseRoot: plan.CaseRoot, RepoRoot: plan.RepoRoot, Pack: plan.Pack, IsMutation: false, WritesArtifacts: true, ReviewRoot: paths.Root, PacketPath: paths.PacketPath, SummaryPath: paths.SummaryPath, CombinedDiffPath: paths.CombinedDiffPath}, nil
}

func reviewPaths(plan Plan, opts ArtifactOptions) (artifactPaths, error) {
	root := strings.TrimSpace(opts.ReviewOutputDir)
	var err error
	if root == "" {
		root, err = projectstate.Join(plan.CaseRoot, "reviews", time.Now().Format("20060102-150405000")+"-"+plan.Command)
		if err != nil {
			return artifactPaths{}, err
		}
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return artifactPaths{}, err
	}
	packet := strings.TrimSpace(opts.PacketPath)
	if packet == "" {
		packet = filepath.Join(root, "packet.json")
	} else if packet, err = filepath.Abs(packet); err != nil {
		return artifactPaths{}, err
	}
	diffRoot := filepath.Join(root, "diffs")
	combined := strings.TrimSpace(opts.DiffPath)
	if combined == "" {
		combined = filepath.Join(diffRoot, "combined.diff")
	} else if combined, err = filepath.Abs(combined); err != nil {
		return artifactPaths{}, err
	}
	return artifactPaths{Root: root, DiffRoot: diffRoot, PreviewRoot: filepath.Join(root, "previews"), PacketPath: packet, SummaryPath: filepath.Join(root, "summary.md"), CombinedDiffPath: combined}, nil
}

func writeItemArtifact(paths artifactPaths, command string, item *Item) error {
	if item.Action == "unchanged" || strings.HasPrefix(item.Action, "skip") || strings.HasPrefix(item.Action, "blocked") {
		return nil
	}
	var oldLabel, newLabel, oldText, newText string
	var err error
	switch command + ":" + item.Kind {
	case "sync:managed-file", "sync:template-file", "sync:support-file":
		oldLabel = "case:" + item.Path
		newLabel = "pack:" + item.Path
		oldText, _, err = ReadTextIfExists(item.TargetPath)
		if err != nil {
			return err
		}
		if item.PlannedText != "" {
			newText = item.PlannedText
		} else {
			newText, _, err = ReadTextIfExists(item.SourcePath)
			if err != nil {
				return err
			}
		}
	case "sync:managed-block":
		oldLabel = "case:" + item.Path
		newLabel = "pack-block:" + item.SourcePath
		oldText, _, err = ReadTextIfExists(item.TargetPath)
		if err != nil {
			return err
		}
		blockText, _, err := ReadTextIfExists(item.SourcePath)
		if err != nil {
			return err
		}
		newText = ApplyManagedBlock(oldText, item.BlockID, blockText)
	case "promote:managed-doc":
		oldLabel = "pack:" + item.Path
		newLabel = "case:" + item.Path
		oldText, _, err = ReadTextIfExists(item.PackPath)
		if err != nil {
			return err
		}
		newText, _, err = ReadTextIfExists(item.CasePath)
		if err != nil {
			return err
		}
	default:
		return nil
	}
	diff := BoundedDiff(oldLabel, oldText, newLabel, newText, 120, 240)
	if strings.TrimSpace(diff) == "" {
		return nil
	}
	path := filepath.Join(paths.DiffRoot, safeFileName(item.Path)+".diff")
	if err := os.WriteFile(path, []byte(diff), 0o644); err != nil {
		return err
	}
	if err := appendText(paths.CombinedDiffPath, diff+"\r\n"); err != nil {
		return err
	}
	item.DiffPath = path
	return nil
}

func writeToolingArtifact(paths artifactPaths, item *Item) error {
	if item.SanitizedPreviewText == "" || strings.HasPrefix(item.Action, "blocked") || strings.HasPrefix(item.Action, "skip") {
		return nil
	}
	path := filepath.Join(paths.PreviewRoot, safeFileName(item.Path)+".sanitized-preview.md")
	if err := os.WriteFile(path, []byte(item.SanitizedPreviewText), 0o644); err != nil {
		return err
	}
	item.SanitizedPreviewPath = path
	return nil
}

func BoundedDiff(oldLabel, oldText, newLabel, newText string, maxChanges, maxLineChars int) string {
	if oldText == newText {
		return ""
	}
	oldLines := strings.Split(strings.ReplaceAll(oldText, "\r\n", "\n"), "\n")
	newLines := strings.Split(strings.ReplaceAll(newText, "\r\n", "\n"), "\n")
	max := len(oldLines)
	if len(newLines) > max {
		max = len(newLines)
	}
	out := []string{"--- " + oldLabel, "+++ " + newLabel}
	changes := 0
	for i := 0; i < max; i++ {
		old := ""
		new := ""
		oldOK := i < len(oldLines)
		newOK := i < len(newLines)
		if oldOK {
			old = oldLines[i]
		}
		if newOK {
			new = newLines[i]
		}
		if oldOK == newOK && old == new {
			continue
		}
		out = append(out, fmt.Sprintf("@@ line %d @@", i+1))
		if oldOK {
			out = append(out, "- "+truncateLine(old, maxLineChars))
		}
		if newOK {
			out = append(out, "+ "+truncateLine(new, maxLineChars))
		}
		changes++
		if changes >= maxChanges {
			out = append(out, "...<diff truncated>")
			break
		}
	}
	return strings.Join(out, "\r\n") + "\r\n"
}

func summaryText(plan Plan) string {
	title := "# rekit " + plan.Command + " review"
	direction := plan.Direction
	switch direction {
	case "kit-to-case":
		direction = "kit -> case"
	case "case-to-kit":
		direction = "case -> kit"
	}
	return strings.Join([]string{
		title,
		"",
		"- direction: " + direction,
		"- case: " + plan.CaseRoot,
		"- pack: " + plan.Pack + " " + plan.ManifestVersion,
		fmt.Sprintf("- changed/planned items: %d / %d", plan.Summary.Changed, len(plan.Items)),
		fmt.Sprintf("- blocked items: %d", plan.Summary.Blocked),
		"",
		"Claude should compare the packet and diffs, explain benefits/conflicts, then ask the user before running a write action.",
	}, "\r\n") + "\r\n"
}

func appendText(path, text string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(text)
	return err
}

func safeFileName(value string) string {
	replacer := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	out := strings.TrimSpace(replacer.Replace(value))
	if out == "" {
		return "item"
	}
	return out
}

func truncateLine(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max] + " ...<truncated line>"
}
