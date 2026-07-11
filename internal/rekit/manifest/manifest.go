package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
)

type LaneType struct {
	ID            string
	Title         string
	Authority     bool
	WorkspaceRoot string
	CanWrite      []string
	ReadOnly      []string
	Outputs       []string
}

type SubagentRoute struct {
	ID                  string
	TaskTypes           string
	Trigger             string
	ShardBasis          string
	TargetItemsPerAgent string
	MaxParallel         string
	Reference           string
	PolicyOverlay       string
	SubagentPermissions string
	MainAgentOwns       string
	OutputContract      string
}

type Manifest struct {
	RepoRoot                string
	Pack                    string
	PackRoot                string
	ManifestPath            string
	Name                    string
	Version                 string
	Description             string
	ManagedFiles            []string
	TemplateFiles           []string
	LocalFiles              []string
	PromoteFiles            []string
	CommonPolicies          []string
	PolicyOverlays          []string
	ToolingFiles            []string
	PromptFiles             []string
	LaneTypes               []LaneType
	WorkstreamDefaults      map[string]string
	AuthorityFiles          []string
	ToolingCandidateSources []string
	SubagentRoutes          []SubagentRoute
	PromoteDenyPatterns     []string
	Budgets                 map[string]string
	ManagedBlock            map[string]string
	SyncPolicy              map[string]string
}

func Load(repoRoot, pack string) (*Manifest, error) {
	repo, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(pack) == "" {
		pack = "vmp-re"
	}
	packRoot := filepath.Join(repo, "packs", pack)
	manifestPath := filepath.Join(packRoot, "manifest.yml")
	b, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("missing pack manifest: %s", manifestPath)
	}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	m := &Manifest{
		RepoRoot:                repo,
		Pack:                    pack,
		PackRoot:                packRoot,
		ManifestPath:            manifestPath,
		Name:                    yamlScalar(lines, "name", pack),
		Version:                 yamlScalar(lines, "version", "0.0.0"),
		Description:             yamlScalar(lines, "description", ""),
		ManagedFiles:            yamlList(lines, "managedFiles"),
		TemplateFiles:           yamlList(lines, "templateFiles"),
		LocalFiles:              yamlList(lines, "localNeverOverwrite"),
		PromoteFiles:            yamlList(lines, "promoteFiles"),
		CommonPolicies:          yamlList(lines, "commonPolicies"),
		PolicyOverlays:          yamlList(lines, "policyOverlays"),
		ToolingFiles:            yamlList(lines, "toolingFiles"),
		PromptFiles:             yamlList(lines, "promptFiles"),
		LaneTypes:               yamlLaneTypes(lines, "laneTypes"),
		WorkstreamDefaults:      yamlMap(lines, "workstreamDefaults"),
		AuthorityFiles:          yamlList(lines, "authorityFiles"),
		ToolingCandidateSources: yamlList(lines, "toolingCandidateSources"),
		SubagentRoutes:          yamlSubagentRoutes(lines, "subagentRoutes"),
		PromoteDenyPatterns:     yamlList(lines, "promoteDenyPatterns"),
		Budgets:                 yamlMap(lines, "budgets"),
		ManagedBlock:            yamlMap(lines, "managedBlock"),
		SyncPolicy:              yamlMap(lines, "syncPolicy"),
	}
	if len(m.ManagedFiles) == 0 {
		return nil, fmt.Errorf("manifest managedFiles is empty: %s", manifestPath)
	}
	if len(m.PromoteFiles) == 0 {
		m.PromoteFiles = append([]string{}, m.ManagedFiles...)
	}
	if _, ok := m.ManagedBlock["file"]; !ok {
		m.ManagedBlock["file"] = "CLAUDE.local.md"
	}
	if _, ok := m.ManagedBlock["blockId"]; !ok {
		m.ManagedBlock["blockId"] = "rekit:router"
	}
	if _, ok := m.ManagedBlock["source"]; !ok {
		m.ManagedBlock["source"] = "CLAUDE.local.snippet.md"
	}
	if _, ok := m.Budgets["defaultMarkdown"]; !ok {
		m.Budgets["defaultMarkdown"] = "16384"
	}
	if len(m.PromoteDenyPatterns) == 0 {
		m.PromoteDenyPatterns = []string{`C:\`, `artifacts[\\/]`, `captures[\\/]`, `[A-Za-z0-9_.-]*trace[A-Za-z0-9_.-]*\.(csv|jsonl|log|txt|bin)`, `[A-Za-z0-9_.-]*dump[A-Za-z0-9_.-]*\.(dmp|bin|raw|exe|dll)`, `\.dmp\b`, `0x[0-9A-Fa-f]{6,}`, `ctx[0-9]+`, `round[0-9]+`, `Task #[0-9]+`}
	}
	return m, nil
}

func (m *Manifest) SourcePath(rel string) (string, error) { return refsf.SafeJoin(m.PackRoot, rel) }
func (m *Manifest) RepoPath(rel string) (string, error)   { return refsf.SafeJoin(m.RepoRoot, rel) }

func (m *Manifest) BudgetLimit(rel string) int64 {
	if v, ok := m.Budgets[rel]; ok {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	if v, ok := m.Budgets["defaultMarkdown"]; ok {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return 16384
}

func (m *Manifest) ValidateSchema() error {
	managed := map[string]bool{}
	for _, rel := range m.ManagedFiles {
		managed[rel] = true
	}
	template := map[string]bool{}
	managedTargets := map[string]bool{}
	for _, rel := range m.ManagedFiles {
		managedTargets[rel] = true
	}
	for _, rel := range m.TemplateFiles {
		template[rel] = true
		managedTargets[strings.TrimSuffix(rel, ".template.md")+".md"] = true
	}
	for _, rel := range m.LocalFiles {
		if managed[rel] {
			return fmt.Errorf("localNeverOverwrite entry also appears in managedFiles: %s", rel)
		}
		if template[rel] {
			return fmt.Errorf("localNeverOverwrite entry also appears in templateFiles: %s", rel)
		}
		managedTargets[rel] = true
	}
	for _, rel := range m.PromoteFiles {
		if !managed[rel] {
			return fmt.Errorf("promoteFiles entry is not managed: %s", rel)
		}
	}
	for _, key := range []string{"file", "blockId", "source"} {
		if strings.TrimSpace(m.ManagedBlock[key]) == "" {
			return fmt.Errorf("managedBlock is missing required key: %s", key)
		}
	}
	managedTargets[m.ManagedBlock["file"]] = true
	if _, err := m.SourcePath(m.ManagedBlock["file"]); err != nil {
		return err
	}
	if _, err := m.SourcePath(m.ManagedBlock["source"]); err != nil {
		return err
	}
	if len(m.ToolingCandidateSources) == 0 {
		return fmt.Errorf("manifest must explicitly declare toolingCandidateSources; implicit vmp-re fallback is not allowed")
	}
	for _, rel := range m.ToolingCandidateSources {
		if _, err := m.SourcePath(rel); err != nil {
			return err
		}
	}
	for _, key := range []string{"defaultAuthorityLane", "defaultStartLaneType", "backupRoot", "requestDefaultTargetLane"} {
		if strings.TrimSpace(m.WorkstreamDefaults[key]) == "" {
			return fmt.Errorf("workstreamDefaults is missing required key: %s", key)
		}
	}
	if _, err := m.SourcePath(m.WorkstreamDefaults["backupRoot"]); err != nil {
		return err
	}
	if handoff := strings.TrimSpace(m.WorkstreamDefaults["handoffPath"]); handoff != "" {
		if _, err := m.SourcePath(handoff); err != nil {
			return err
		}
		if !managedTargets[handoff] {
			return fmt.Errorf("workstreamDefaults.handoffPath is not a managed, template, managed-block, or local file: %s", handoff)
		}
	}
	if len(m.AuthorityFiles) == 0 {
		return fmt.Errorf("manifest must explicitly declare authorityFiles, even if the list is intentionally minimal")
	}
	for _, key := range []string{"managedFiles", "templateFiles", "localFiles"} {
		if strings.TrimSpace(m.SyncPolicy[key]) == "" {
			return fmt.Errorf("syncPolicy is missing required key: %s", key)
		}
	}
	if m.SyncPolicy["managedFiles"] != "overwrite-with-backup" || m.SyncPolicy["templateFiles"] != "create-if-missing" || m.SyncPolicy["localFiles"] != "never-overwrite" {
		return fmt.Errorf("syncPolicy has unsupported value")
	}
	for _, pattern := range m.PromoteDenyPatterns {
		if strings.TrimSpace(pattern) == "" {
			return fmt.Errorf("promoteDenyPatterns contains an empty pattern")
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid promoteDenyPatterns regex %q: %w", pattern, err)
		}
	}
	if err := m.validateSubagentRoutes(managedTargets); err != nil {
		return err
	}
	if _, err := m.LaneType(m.WorkstreamDefaults["defaultAuthorityLane"]); err != nil {
		return err
	}
	authority, _ := m.LaneType(m.WorkstreamDefaults["defaultAuthorityLane"])
	if !authority.Authority {
		return fmt.Errorf("workstreamDefaults.defaultAuthorityLane must reference an authority lane: %s", authority.ID)
	}
	if _, err := m.LaneType(m.WorkstreamDefaults["defaultStartLaneType"]); err != nil {
		return err
	}
	if _, err := m.LaneType(m.WorkstreamDefaults["requestDefaultTargetLane"]); err != nil {
		return err
	}
	for _, rel := range m.AuthorityFiles {
		if _, err := m.SourcePath(rel); err != nil {
			return err
		}
		if !contains(authority.CanWrite, rel) {
			return fmt.Errorf("authorityFiles entry is not writable by default authority lane %s: %s", authority.ID, rel)
		}
	}
	if !strings.EqualFold(m.Pack, "vmp-re") {
		if err := m.validateNonVMPPaths(); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manifest) validateSubagentRoutes(managedTargets map[string]bool) error {
	seen := map[string]bool{}
	for _, route := range m.SubagentRoutes {
		id := strings.TrimSpace(route.ID)
		if id == "" {
			return fmt.Errorf("subagent route is missing id in %s", m.ManifestPath)
		}
		if seen[strings.ToLower(id)] {
			return fmt.Errorf("duplicate subagent route id: %s", id)
		}
		seen[strings.ToLower(id)] = true
		if strings.TrimSpace(route.TaskTypes) == "" {
			return fmt.Errorf("subagent route %s is missing taskTypes", id)
		}
		if strings.TrimSpace(route.ShardBasis) == "" {
			return fmt.Errorf("subagent route %s is missing shardBasis", id)
		}
		if n := optionPositiveInt(route.TargetItemsPerAgent); n < 1 {
			return fmt.Errorf("subagent route %s has invalid targetItemsPerAgent: %s", id, route.TargetItemsPerAgent)
		}
		if n := optionPositiveInt(route.MaxParallel); n < 1 {
			return fmt.Errorf("subagent route %s has invalid maxParallel: %s", id, route.MaxParallel)
		}
		if strings.TrimSpace(route.Reference) == "" {
			return fmt.Errorf("subagent route %s is missing reference document", id)
		}
		if _, err := m.SourcePath(route.Reference); err != nil {
			return err
		}
		if !managedTargets[route.Reference] {
			return fmt.Errorf("subagent route %s reference is not a managed/template/local file: %s", id, route.Reference)
		}
		if strings.TrimSpace(route.PolicyOverlay) != "" {
			if _, err := m.SourcePath(route.PolicyOverlay); err != nil {
				return err
			}
		}
		if strings.TrimSpace(route.SubagentPermissions) == "" {
			return fmt.Errorf("subagent route %s is missing subagentPermissions", id)
		}
		if strings.TrimSpace(route.MainAgentOwns) == "" {
			return fmt.Errorf("subagent route %s is missing mainAgentOwns", id)
		}
		if strings.TrimSpace(route.OutputContract) == "" {
			return fmt.Errorf("subagent route %s is missing outputContract", id)
		}
	}
	return nil
}

func optionPositiveInt(value string) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return n
}

func (m *Manifest) LaneType(id string) (LaneType, error) {
	for _, lane := range m.LaneTypes {
		if strings.EqualFold(lane.ID, id) {
			return lane, nil
		}
	}
	return LaneType{}, fmt.Errorf("unknown lane type: %s", id)
}

func (m *Manifest) validateNonVMPPaths() error {
	paths := []string{}
	paths = append(paths, m.ManagedFiles...)
	paths = append(paths, m.TemplateFiles...)
	paths = append(paths, m.LocalFiles...)
	paths = append(paths, m.PromoteFiles...)
	paths = append(paths, m.ToolingFiles...)
	paths = append(paths, m.ToolingCandidateSources...)
	paths = append(paths, m.PromptFiles...)
	paths = append(paths, m.PolicyOverlays...)
	paths = append(paths, m.ManagedBlock["file"], m.ManagedBlock["source"])
	for _, route := range m.SubagentRoutes {
		paths = append(paths, route.Reference, route.PolicyOverlay)
	}
	for _, rel := range paths {
		if containsVMPPath(rel) {
			return fmt.Errorf("non-vmp pack declares vmp-re path: %s", rel)
		}
	}
	for _, rel := range m.AuthorityFiles {
		if containsVMPPath(rel) {
			return fmt.Errorf("non-vmp pack declares vmp-re authority path: %s", rel)
		}
	}
	for _, key := range []string{"handoffPath", "backupRoot"} {
		if containsVMPPath(m.WorkstreamDefaults[key]) {
			return fmt.Errorf("non-vmp pack declares vmp-re workstream default: %s=%s", key, m.WorkstreamDefaults[key])
		}
	}
	return nil
}

func containsVMPPath(value string) bool {
	value = strings.ReplaceAll(value, `\\`, "/")
	return regexp.MustCompile(`(^|/)vmp-re(/|$)`).MatchString(value)
}

func convertValue(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 {
		if (strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"")) || (strings.HasPrefix(v, "'") && strings.HasSuffix(v, "'")) {
			return v[1 : len(v)-1]
		}
	}
	return v
}

func yamlScalar(lines []string, key, def string) string {
	re := regexp.MustCompile(`^` + regexp.QuoteMeta(key) + `\s*:\s*(.*)$`)
	for _, line := range lines {
		if m := re.FindStringSubmatch(line); m != nil {
			return convertValue(m[1])
		}
	}
	return def
}

func yamlList(lines []string, key string) []string {
	items := []string{}
	inside := false
	reKey := regexp.MustCompile(`^` + regexp.QuoteMeta(key) + `\s*:\s*$`)
	reItem := regexp.MustCompile(`^\s{2,}-\s*(.+?)\s*$`)
	for _, line := range lines {
		if !inside {
			if reKey.MatchString(line) {
				inside = true
			}
			continue
		}
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			break
		}
		if m := reItem.FindStringSubmatch(line); m != nil {
			value := convertValue(m[1])
			if value != "[]" {
				items = append(items, value)
			}
		}
	}
	return items
}

func yamlMap(lines []string, key string) map[string]string {
	out := map[string]string{}
	inside := false
	reKey := regexp.MustCompile(`^` + regexp.QuoteMeta(key) + `\s*:\s*$`)
	reItem := regexp.MustCompile(`^\s{2,}([^:#]+?)\s*:\s*(.*?)\s*$`)
	for _, line := range lines {
		if !inside {
			if reKey.MatchString(line) {
				inside = true
			}
			continue
		}
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			break
		}
		if m := reItem.FindStringSubmatch(line); m != nil {
			out[strings.TrimSpace(m[1])] = convertValue(m[2])
		}
	}
	return out
}

func yamlObjectList(lines []string, key string) []map[string]string {
	items := []map[string]string{}
	inside := false
	var current map[string]string
	reKey := regexp.MustCompile(`^` + regexp.QuoteMeta(key) + `\s*:\s*$`)
	reStart := regexp.MustCompile(`^\s{2,}-\s*(.*?)\s*$`)
	reField := regexp.MustCompile(`^\s{4}([^:#]+?)\s*:\s*(.*?)\s*$`)
	reInline := regexp.MustCompile(`^([^:#]+?)\s*:\s*(.*?)\s*$`)
	for _, line := range lines {
		if !inside {
			if reKey.MatchString(line) {
				inside = true
			}
			continue
		}
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			break
		}
		if m := reStart.FindStringSubmatch(line); m != nil {
			if current != nil {
				items = append(items, current)
			}
			current = map[string]string{}
			rest := strings.TrimSpace(m[1])
			if im := reInline.FindStringSubmatch(rest); im != nil {
				current[strings.TrimSpace(im[1])] = convertValue(im[2])
			} else if rest != "" {
				current["value"] = convertValue(rest)
			}
			continue
		}
		if current != nil {
			if m := reField.FindStringSubmatch(line); m != nil {
				current[strings.TrimSpace(m[1])] = convertValue(m[2])
			}
		}
	}
	if current != nil {
		items = append(items, current)
	}
	return items
}

func yamlLaneTypes(lines []string, key string) []LaneType {
	rows := yamlObjectList(lines, key)
	lanes := make([]LaneType, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row["id"]) == "" {
			continue
		}
		lanes = append(lanes, LaneType{
			ID:            row["id"],
			Title:         valueOr(row["title"], row["id"]),
			Authority:     parseBool(row["authority"]),
			WorkspaceRoot: valueOr(row["workspaceRoot"], "captures/lanes"),
			CanWrite:      splitScalarList(row["canWrite"]),
			ReadOnly:      splitScalarList(row["readOnly"]),
			Outputs:       splitScalarList(row["outputs"]),
		})
	}
	return lanes
}

func ObjectListFromFile(path, key string) ([]map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	return yamlObjectList(lines, key), nil
}

func yamlSubagentRoutes(lines []string, key string) []SubagentRoute {
	rows := yamlObjectList(lines, key)
	routes := make([]SubagentRoute, 0, len(rows))
	for _, row := range rows {
		routes = append(routes, SubagentRoute{ID: row["id"], TaskTypes: row["taskTypes"], Trigger: row["trigger"], ShardBasis: row["shardBasis"], TargetItemsPerAgent: row["targetItemsPerAgent"], MaxParallel: row["maxParallel"], Reference: row["reference"], PolicyOverlay: row["policyOverlay"], SubagentPermissions: row["subagentPermissions"], MainAgentOwns: row["mainAgentOwns"], OutputContract: row["outputContract"]})
	}
	return routes
}

func splitScalarList(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := regexp.MustCompile(`[;,]`).Split(v, -1)
	out := []string{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "yes", "1":
		return true
	default:
		return false
	}
}

func valueOr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func contains(items []string, value string) bool {
	return slices.Contains(items, value)
}
