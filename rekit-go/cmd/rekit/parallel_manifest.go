package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type PackManifest struct {
	Path             string
	Version          string
	ParallelDefaults ParallelDefaults
	ParallelFiles    []ParallelFileConfig
	ParallelReadOnly []string
}

type ParallelDefaults struct {
	DefaultKind          string
	SessionRoot          string
	ReviewRoot           string
	WorkspaceRoot        string
	WorkspaceDateLayout  string
	InitialStatus        string
	DefaultLifecycleMode string
}

type ParallelFileConfig struct {
	Kind            string
	Path            string
	Category        string
	Template        string
	StatusColumn    string
	CounterName     string
	ArtifactKind    string
	Required        bool
	IncludeInReview bool
	IncludeInDoctor bool
}

func defaultParallelDefaults() ParallelDefaults {
	return ParallelDefaults{
		DefaultKind:          "feature-analysis",
		SessionRoot:          ".rekit/parallel",
		ReviewRoot:           ".rekit/reviews",
		WorkspaceRoot:        "captures/feature_analysis",
		WorkspaceDateLayout:  "20060102",
		InitialStatus:        "open",
		DefaultLifecycleMode: "attached_to_main",
	}
}

func loadPackManifestFromRoots(repoRoot, pack string) PackManifest {
	defaults := defaultParallelDefaults()
	path := filepath.Join(repoRoot, "packs", pack, "manifest.yml")
	m := PackManifest{Path: path, ParallelDefaults: defaults}
	b, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	lines := strings.Split(string(b), "\n")
	m.Version = yamlScalar(lines, "version", "")
	for k, v := range yamlMap(lines, "parallelDefaults") {
		switch strings.ToLower(k) {
		case "defaultkind":
			m.ParallelDefaults.DefaultKind = nonEmpty(v, m.ParallelDefaults.DefaultKind)
		case "sessionroot":
			m.ParallelDefaults.SessionRoot = nonEmpty(v, m.ParallelDefaults.SessionRoot)
		case "reviewroot":
			m.ParallelDefaults.ReviewRoot = nonEmpty(v, m.ParallelDefaults.ReviewRoot)
		case "workspaceroot":
			m.ParallelDefaults.WorkspaceRoot = nonEmpty(v, m.ParallelDefaults.WorkspaceRoot)
		case "workspacedatelayout":
			m.ParallelDefaults.WorkspaceDateLayout = nonEmpty(v, m.ParallelDefaults.WorkspaceDateLayout)
		case "initialstatus":
			m.ParallelDefaults.InitialStatus = nonEmpty(v, m.ParallelDefaults.InitialStatus)
		case "defaultlifecyclemode":
			m.ParallelDefaults.DefaultLifecycleMode = nonEmpty(v, m.ParallelDefaults.DefaultLifecycleMode)
		}
	}
	for _, item := range yamlObjectList(lines, "parallelFiles") {
		pf := ParallelFileConfig{
			Kind:            item["kind"],
			Path:            filepath.ToSlash(item["path"]),
			Category:        item["category"],
			Template:        filepath.ToSlash(item["template"]),
			StatusColumn:    item["statusColumn"],
			CounterName:     item["counterName"],
			ArtifactKind:    item["artifactKind"],
			Required:        parseBool(item["required"]),
			IncludeInReview: parseBool(item["includeInReview"]),
			IncludeInDoctor: parseBool(item["includeInDoctor"]),
		}
		if pf.Kind == "" {
			pf.Kind = m.ParallelDefaults.DefaultKind
		}
		if pf.ArtifactKind == "" {
			pf.ArtifactKind = pf.Category
		}
		if pf.Path != "" {
			m.ParallelFiles = append(m.ParallelFiles, pf)
		}
	}
	m.ParallelReadOnly = yamlList(lines, "parallelReadOnlyFiles")
	return m
}

func loadPackManifestForPaths(p Paths) PackManifest {
	return loadPackManifestFromRoots(p.RepoRoot, p.Pack)
}

func parallelFilesForKind(m PackManifest, kind string) []ParallelFileConfig {
	files := []ParallelFileConfig{}
	for _, f := range m.ParallelFiles {
		if f.Kind == "" || strings.EqualFold(f.Kind, kind) {
			files = append(files, f)
		}
	}
	return files
}

func joinInside(root, relPath string) (string, error) {
	if strings.TrimSpace(relPath) == "" {
		return "", fmt.Errorf("relative path is empty")
	}
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("path must be relative: %s", relPath)
	}
	path := filepath.Join(root, filepath.FromSlash(relPath))
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	if !isChildPath(root, abs) {
		return "", fmt.Errorf("path escapes root: %s", relPath)
	}
	return abs, nil
}

func sessionAndWorkspaceRoots(caseRoot string, m PackManifest) (string, string, error) {
	sessionRoot, err := joinInside(caseRoot, m.ParallelDefaults.SessionRoot)
	if err != nil {
		return "", "", err
	}
	workspaceRoot, err := joinInside(caseRoot, m.ParallelDefaults.WorkspaceRoot)
	if err != nil {
		return "", "", err
	}
	return sessionRoot, workspaceRoot, nil
}

func workspaceName(name string, m PackManifest) string {
	layout := m.ParallelDefaults.WorkspaceDateLayout
	if layout == "" {
		layout = "20060102"
	}
	return name + "_" + time.Now().Format(layout)
}

func manifestTemplateText(p Paths, templatePath string, fallback string, st State) string {
	if templatePath == "" {
		return fallback
	}
	path, err := joinInside(filepath.Join(p.RepoRoot, "packs", p.Pack), templatePath)
	if err != nil {
		return fallback
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	return renderTemplateTokens(string(b), p, st)
}

func renderTemplateTokens(text string, p Paths, st State) string {
	workspace := rel(p.CaseRoot, p.Workspace)
	readOnly := ""
	m := loadPackManifestForPaths(p)
	if len(m.ParallelReadOnly) > 0 {
		items := []string{}
		for _, item := range m.ParallelReadOnly {
			items = append(items, "- `"+item+"`")
		}
		readOnly = strings.Join(items, "\n")
	}
	repls := map[string]string{
		"<FEATURE_NAME>":          st.Name,
		"<SESSION_NAME>":          st.Name,
		"<CASE_ROOT>":             p.CaseRoot,
		"<WORKSPACE>":             workspace,
		"<STATUS>":                st.Status,
		"<LIFECYCLE_MODE>":        st.LifecycleMode,
		"<UNRESOLVED_REQUESTS>":   fmt.Sprintf("%d", st.Counters.UnresolvedRequests),
		"<CANDIDATE_ROWS>":        fmt.Sprintf("%d", st.Counters.CandidateRows),
		"<CANDIDATE_UNRESOLVED>":  fmt.Sprintf("%d", st.Counters.CandidateUnresolvedRows),
		"<NEXT_ACTION>":           nextAction(st),
		"<READ_ONLY_FILES>":       readOnly,
		"<FEATURE_RESUME_PATH>":   rel(p.CaseRoot, filepath.Join(p.Workspace, "prompts", "FEATURE_RESUME.md")),
		"<MAIN_RESUME_PATH>":      rel(p.CaseRoot, filepath.Join(p.SessionRoot, "prompts", "MAIN_RESUME.md")),
		"<AUTHORITY_RESUME_PATH>": rel(p.CaseRoot, filepath.Join(p.SessionRoot, "prompts", "AUTHORITY_RESUME.md")),
		"<LAST_REVIEW_ROOT>":      rel(p.CaseRoot, st.LastReviewRoot),
	}
	for k, v := range repls {
		text = strings.ReplaceAll(text, k, v)
	}
	return text
}

func renderPromptTemplate(p Paths, st State, templateName string) (string, bool) {
	path := filepath.Join("templates", "parallel", st.Kind, templateName)
	root := filepath.Join(p.RepoRoot, "packs", p.Pack)
	full, err := joinInside(root, filepath.ToSlash(path))
	if err != nil {
		return "", false
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return "", false
	}
	return renderTemplateTokens(string(b), p, st), true
}

func configuredInitialFiles(p Paths, st State) map[string]string {
	m := loadPackManifestForPaths(p)
	files := map[string]string{}
	for _, f := range parallelFilesForKind(m, st.Kind) {
		if f.Template == "" {
			continue
		}
		target, err := joinInside(p.Workspace, f.Path)
		if err != nil {
			continue
		}
		files[target] = manifestTemplateText(p, f.Template, "", st)
	}
	return files
}

func configuredRequiredFiles(p Paths, st State) []string {
	m := loadPackManifestForPaths(p)
	out := []string{filepath.Join(p.SessionRoot, "state.json"), filepath.Join(p.SessionRoot, "timeline.jsonl")}
	for _, f := range parallelFilesForKind(m, st.Kind) {
		if f.Required || f.IncludeInDoctor {
			if path, err := joinInside(p.Workspace, f.Path); err == nil {
				out = append(out, path)
			}
		}
	}
	if len(out) == 2 {
		out = append(out, filepath.Join(p.Workspace, "START_HERE.md"), filepath.Join(p.Workspace, "summary.md"), filepath.Join(p.Workspace, "evidence.md"), filepath.Join(p.Workspace, "lowering_requests.csv"), filepath.Join(p.Workspace, "vm_blockers.csv"))
	}
	return out
}

func configuredReviewFiles(p Paths, st State) []string {
	m := loadPackManifestForPaths(p)
	files := []string{}
	for _, f := range parallelFilesForKind(m, st.Kind) {
		if f.IncludeInReview {
			if path, err := joinInside(p.Workspace, f.Path); err == nil {
				files = append(files, rel(p.CaseRoot, path))
			}
		}
	}
	if len(files) == 0 {
		files = []string{
			rel(p.CaseRoot, filepath.Join(p.Workspace, "summary.md")),
			rel(p.CaseRoot, filepath.Join(p.Workspace, "evidence.md")),
			rel(p.CaseRoot, filepath.Join(p.Workspace, "lowering_requests.csv")),
			rel(p.CaseRoot, filepath.Join(p.Workspace, "vm_blockers.csv")),
			rel(p.CaseRoot, filepath.Join(p.Workspace, "notes.md")),
			rel(p.CaseRoot, filepath.Join(p.Workspace, "outbox", "to-main.jsonl")),
			rel(p.CaseRoot, filepath.Join(p.SessionRoot, "checkpoints", "latest.json")),
		}
	}
	files = append(files, rel(p.CaseRoot, filepath.Join(p.Workspace, "outbox", "to-main.jsonl")), rel(p.CaseRoot, filepath.Join(p.SessionRoot, "checkpoints", "latest.json")))
	return dedupeStrings(files)
}

func scanConfiguredCounters(p Paths) (Counters, error) {
	m := loadPackManifestForPaths(p)
	counts := Counters{}
	var firstErr error
	files := parallelFilesForKind(m, "feature-analysis")
	if st, err := loadState(p); err == nil && st.Kind != "" {
		files = parallelFilesForKind(m, st.Kind)
	}
	if len(files) == 0 {
		return scanFallbackCounters(p)
	}
	for _, f := range files {
		if f.StatusColumn == "" || f.CounterName == "" {
			continue
		}
		path, err := joinInside(p.Workspace, f.Path)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		total, unresolved, err := countCSVStatusStrict(path, f.StatusColumn)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		switch strings.ToLower(f.CounterName) {
		case "loweringrequests":
			counts.LoweringRequests += total
			counts.UnresolvedRequests += unresolved
		case "vmblockers":
			counts.VMBlockers += total
		case "candidaterows":
			counts.CandidateRows += total
			counts.CandidateUnresolvedRows += unresolved
		}
	}
	counts.OutboxMessages = countLines(filepath.Join(p.Workspace, "outbox", "to-main.jsonl"))
	counts.InboxMessages = countLines(filepath.Join(p.Workspace, "inbox", "from-main.jsonl"))
	return counts, firstErr
}

func scanFallbackCounters(p Paths) (Counters, error) {
	lowerTotal, lowerUnresolved, lowerErr := countCSVStatusStrict(filepath.Join(p.Workspace, "lowering_requests.csv"), "status")
	blockers, _, blockErr := countCSVStatusStrict(filepath.Join(p.Workspace, "vm_blockers.csv"), "status")
	candidateRows := 0
	candidateUnresolved := 0
	var firstErr error
	if lowerErr != nil {
		firstErr = lowerErr
	} else if blockErr != nil {
		firstErr = blockErr
	}
	_ = filepath.WalkDir(filepath.Join(p.Workspace, "candidates"), func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(strings.ToLower(path), ".csv") {
			rows, unresolved, csvErr := countCSVStatusStrict(path, "status")
			if csvErr != nil && firstErr == nil {
				firstErr = csvErr
			}
			candidateRows += rows
			candidateUnresolved += unresolved
		}
		return nil
	})
	return Counters{LoweringRequests: lowerTotal, UnresolvedRequests: lowerUnresolved, VMBlockers: blockers, CandidateRows: candidateRows, CandidateUnresolvedRows: candidateUnresolved, OutboxMessages: countLines(filepath.Join(p.Workspace, "outbox", "to-main.jsonl")), InboxMessages: countLines(filepath.Join(p.Workspace, "inbox", "from-main.jsonl"))}, firstErr
}

func countCSVStatusStrict(path, statusCol string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	defer f.Close()
	r := csvNewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		return 0, 0, fmt.Errorf("parse csv %s: %w", path, err)
	}
	if len(rows) <= 1 {
		return 0, 0, nil
	}
	idx := -1
	for i, h := range rows[0] {
		if strings.EqualFold(strings.TrimSpace(h), statusCol) {
			idx = i
			break
		}
	}
	total, unresolved := 0, 0
	for _, row := range rows[1:] {
		blank := true
		for _, c := range row {
			if strings.TrimSpace(c) != "" {
				blank = false
				break
			}
		}
		if blank {
			continue
		}
		total++
		status := ""
		if idx >= 0 && idx < len(row) {
			status = strings.ToLower(strings.TrimSpace(row[idx]))
		}
		if status == "" || !(status == "resolved" || status == "done" || status == "closed" || status == "accepted" || status == "rejected") {
			unresolved++
		}
	}
	return total, unresolved, nil
}

func csvNewReader(f *os.File) interface{ ReadAll() ([][]string, error) } {
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	return r
}

func yamlScalar(lines []string, key string, def string) string {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, ":") {
			continue
		}
		parts := strings.SplitN(trimmed, ":", 2)
		if parts[0] == key {
			return yamlValue(parts[1])
		}
	}
	return def
}

func yamlMap(lines []string, key string) map[string]string {
	out := map[string]string{}
	inside := false
	for _, line := range lines {
		if !inside {
			if strings.TrimSpace(line) == key+":" {
				inside = true
			}
			continue
		}
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			break
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-") || !strings.Contains(trimmed, ":") {
			continue
		}
		parts := strings.SplitN(trimmed, ":", 2)
		out[strings.TrimSpace(parts[0])] = yamlValue(parts[1])
	}
	return out
}

func yamlList(lines []string, key string) []string {
	out := []string{}
	inside := false
	for _, line := range lines {
		if !inside {
			if strings.TrimSpace(line) == key+":" {
				inside = true
			}
			continue
		}
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			break
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-") {
			out = append(out, yamlValue(strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))))
		}
	}
	return out
}

func yamlObjectList(lines []string, key string) []map[string]string {
	out := []map[string]string{}
	inside := false
	var current map[string]string
	for _, line := range lines {
		if !inside {
			if strings.TrimSpace(line) == key+":" {
				inside = true
			}
			continue
		}
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			break
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-") {
			if current != nil {
				out = append(out, current)
			}
			current = map[string]string{}
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			if strings.Contains(rest, ":") {
				parts := strings.SplitN(rest, ":", 2)
				current[strings.TrimSpace(parts[0])] = yamlValue(parts[1])
			}
			continue
		}
		if current != nil && strings.Contains(trimmed, ":") {
			parts := strings.SplitN(trimmed, ":", 2)
			current[strings.TrimSpace(parts[0])] = yamlValue(parts[1])
		}
	}
	if current != nil {
		out = append(out, current)
	}
	return out
}

func yamlValue(value string) string {
	v := strings.TrimSpace(value)
	if (strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"")) || (strings.HasPrefix(v, "'") && strings.HasSuffix(v, "'")) {
		return strings.Trim(v, "\"'")
	}
	return v
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "yes", "1":
		return true
	default:
		return false
	}
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func dedupeStrings(items []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range items {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}
