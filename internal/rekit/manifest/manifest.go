package manifest

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
)

type LaneType struct {
	ID                string
	Title             string
	Authority         bool
	explicitAuthority string
	WorkspaceRoot     string
	CanWrite          []string
	ReadOnly          []string
	Outputs           []string
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

type HeavyToolGate struct {
	ID                           string
	Title                        string
	SideEffects                  []string
	DefaultRisk                  string
	RequiresConfirmation         bool
	explicitRequiresConfirmation string
	StopConditions               []string
}

type Manifest struct {
	RepoRoot                string
	Pack                    string
	PackRoot                string
	ManifestPath            string
	SchemaVersion           string
	Name                    string
	Version                 string
	Description             string
	Maturity                string
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
	HeavyToolGates          []HeavyToolGate
	PromoteDenyPatterns     []string
	Budgets                 map[string]string
	ManagedBlock            map[string]string
	SyncPolicy              map[string]string

	explicitManagedBlock map[string]string
	explicitLists        map[string]bool
	explicitMaps         map[string]bool
}

type PackSummary struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	SchemaVersion        string   `json:"schemaVersion"`
	Version              string   `json:"version"`
	Maturity             string   `json:"maturity"`
	Description          string   `json:"description"`
	ManifestPath         string   `json:"manifestPath"`
	SchemaValid          bool     `json:"schemaValid"`
	Error                string   `json:"error"`
	ManagedFiles         int      `json:"managedFiles"`
	TemplateFiles        int      `json:"templateFiles"`
	LocalFiles           int      `json:"localFiles"`
	PromoteFiles         int      `json:"promoteFiles"`
	ToolingFiles         int      `json:"toolingFiles"`
	PromptFiles          int      `json:"promptFiles"`
	SubagentRoutes       int      `json:"subagentRoutes"`
	HeavyToolGates       int      `json:"heavyToolGates"`
	HeavyToolGateActions []string `json:"heavyToolGateActions"`
	LaneTypes            int      `json:"laneTypes"`
	AuthorityFiles       int      `json:"authorityFiles"`
	DefaultAuthorityLane string   `json:"defaultAuthorityLane"`
}

var manifestListPresenceKeys = []string{
	"managedFiles",
	"templateFiles",
	"localNeverOverwrite",
	"promoteFiles",
	"commonPolicies",
	"policyOverlays",
	"subagentRoutes",
	"toolingFiles",
	"promptFiles",
	"toolingCandidateSources",
	"authorityFiles",
	"promoteDenyPatterns",
	"heavyToolGates",
	"laneTypes",
}

func Load(repoRoot, pack string) (*Manifest, error) {
	repo, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(pack) == "" {
		pack = defaults.DefaultPack
	}
	pack, err = normalizePackID(pack)
	if err != nil {
		return nil, err
	}
	packRoot := filepath.Join(repo, "packs", filepath.FromSlash(pack))
	manifestPath := filepath.Join(packRoot, "manifest.yml")
	b, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("missing pack manifest: %s", manifestPath)
	}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	explicitManagedBlock := yamlMap(lines, "managedBlock")
	explicitLists := yamlListPresence(lines, manifestListPresenceKeys...)
	explicitMaps := yamlMapPresence(lines, "syncPolicy", "workstreamDefaults", "budgets")
	m := &Manifest{
		RepoRoot:                repo,
		Pack:                    pack,
		PackRoot:                packRoot,
		ManifestPath:            manifestPath,
		SchemaVersion:           yamlScalar(lines, "schemaVersion", ""),
		Name:                    yamlScalar(lines, "name", ""),
		Version:                 yamlScalar(lines, "version", ""),
		Description:             yamlScalar(lines, "description", ""),
		Maturity:                yamlScalar(lines, "maturity", ""),
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
		HeavyToolGates:          yamlHeavyToolGates(lines, "heavyToolGates"),
		PromoteDenyPatterns:     yamlList(lines, "promoteDenyPatterns"),
		Budgets:                 yamlMap(lines, "budgets"),
		ManagedBlock:            cloneStringMap(explicitManagedBlock),
		SyncPolicy:              yamlMap(lines, "syncPolicy"),
		explicitManagedBlock:    explicitManagedBlock,
		explicitLists:           explicitLists,
		explicitMaps:            explicitMaps,
	}
	return m, nil
}

func (m *Manifest) SourcePath(rel string) (string, error) { return refsf.SafeJoin(m.PackRoot, rel) }
func (m *Manifest) RepoPath(rel string) (string, error)   { return refsf.SafeJoin(m.RepoRoot, rel) }

func List(repoRoot string) ([]PackSummary, error) {
	repo, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	packsRoot := filepath.Join(repo, "packs")
	entries, err := os.ReadDir(packsRoot)
	if err != nil {
		return nil, fmt.Errorf("missing packs root: %s", packsRoot)
	}
	summaries := []PackSummary{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		manifestPath := filepath.Join(packsRoot, id, "manifest.yml")
		if _, err := os.Stat(manifestPath); err != nil {
			continue
		}
		m, err := Load(repo, id)
		if err != nil {
			summaries = append(summaries, PackSummary{ID: id, Name: id, Maturity: packMaturity(""), ManifestPath: manifestPath, SchemaValid: false, Error: err.Error()})
			continue
		}
		summary := m.Summary()
		if err := m.ValidateSchema(); err != nil {
			summary.SchemaValid = false
			summary.Error = err.Error()
		}
		summaries = append(summaries, summary)
	}
	sort.Slice(summaries, func(i, j int) bool { return strings.ToLower(summaries[i].ID) < strings.ToLower(summaries[j].ID) })
	return summaries, nil
}

func (m *Manifest) Summary() PackSummary {
	return PackSummary{
		ID:                   m.Pack,
		Name:                 m.Name,
		SchemaVersion:        m.SchemaVersion,
		Version:              m.Version,
		Maturity:             packMaturity(m.Maturity),
		Description:          m.Description,
		ManifestPath:         m.ManifestPath,
		SchemaValid:          true,
		ManagedFiles:         len(m.ManagedFiles),
		TemplateFiles:        len(m.TemplateFiles),
		LocalFiles:           len(m.LocalFiles),
		PromoteFiles:         len(m.PromoteFiles),
		ToolingFiles:         len(m.ToolingFiles),
		PromptFiles:          len(m.PromptFiles),
		SubagentRoutes:       len(m.SubagentRoutes),
		HeavyToolGates:       len(m.HeavyToolGates),
		HeavyToolGateActions: m.HeavyToolGateIDs(),
		LaneTypes:            len(m.LaneTypes),
		AuthorityFiles:       len(m.AuthorityFiles),
		DefaultAuthorityLane: m.WorkstreamDefaults["defaultAuthorityLane"],
	}
}

func normalizePackID(pack string) (string, error) {
	id := strings.TrimSpace(pack)
	if id == "" {
		return defaults.DefaultPack, nil
	}
	if filepath.IsAbs(id) || strings.ContainsAny(id, `/\\`) || id == "." || id == ".." || strings.Contains(id, "..") {
		return "", fmt.Errorf("invalid pack id: %s", pack)
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]*$`).MatchString(id) {
		return "", fmt.Errorf("invalid pack id: %s", pack)
	}
	return id, nil
}

func packMaturity(explicit string) string {
	maturity := normalizePackMaturity(explicit)
	if maturity == "" {
		return "missing"
	}
	return maturity
}

func normalizePackMaturity(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func isSupportedPackMaturity(value string) bool {
	switch normalizePackMaturity(value) {
	case "mature", "skeleton", "template", "experimental":
		return true
	default:
		return false
	}
}

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	maps.Copy(out, in)
	return out
}

func (m *Manifest) BudgetLimit(rel string) int64 {
	if n, ok := parsePositiveBudgetLimit(m.Budgets[rel]); ok {
		return n
	}
	if n, ok := parsePositiveBudgetLimit(m.Budgets["defaultMarkdown"]); ok {
		return n
	}
	return 16384
}

func parsePositiveBudgetLimit(value string) (int64, bool) {
	n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return n, err == nil && n > 0
}

func (m *Manifest) validateBudgets() error {
	if !m.explicitMaps["budgets"] {
		return fmt.Errorf("manifest must explicitly declare budgets")
	}
	if strings.TrimSpace(m.Budgets["defaultMarkdown"]) == "" {
		return fmt.Errorf("manifest must explicitly declare budgets.defaultMarkdown")
	}
	for key, value := range m.Budgets {
		key = strings.TrimSpace(key)
		if key == "" {
			return fmt.Errorf("budgets contains an empty key")
		}
		if _, ok := parsePositiveBudgetLimit(value); !ok {
			return fmt.Errorf("budgets.%s has invalid positive integer limit: %s", key, strings.TrimSpace(value))
		}
	}
	return nil
}

func (m *Manifest) validateSyncPolicy() error {
	if !m.explicitMaps["syncPolicy"] {
		return fmt.Errorf("manifest must explicitly declare syncPolicy")
	}
	for _, entry := range []struct {
		key  string
		want string
	}{
		{key: "managedFiles", want: "overwrite-with-backup"},
		{key: "templateFiles", want: "create-if-missing"},
		{key: "localFiles", want: "never-overwrite"},
	} {
		value := strings.TrimSpace(m.SyncPolicy[entry.key])
		if value == "" {
			return fmt.Errorf("syncPolicy is missing required key: %s", entry.key)
		}
		if value != entry.want {
			return fmt.Errorf("syncPolicy.%s has unsupported value: %s", entry.key, value)
		}
	}
	return nil
}

func (m *Manifest) ValidateSchema() error {
	if strings.TrimSpace(m.SchemaVersion) == "" {
		return fmt.Errorf("schemaVersion is missing")
	}
	if strings.TrimSpace(m.SchemaVersion) != "1" {
		return fmt.Errorf("schemaVersion has unsupported value: %s", m.SchemaVersion)
	}
	maturity := normalizePackMaturity(m.Maturity)
	if maturity == "" {
		return fmt.Errorf("maturity is missing")
	}
	if !isSupportedPackMaturity(maturity) {
		return fmt.Errorf("maturity has unsupported value: %s", m.Maturity)
	}
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("name is missing")
	}
	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("version is missing")
	}
	if strings.TrimSpace(m.Description) == "" {
		return fmt.Errorf("description is missing")
	}
	for _, key := range manifestListPresenceKeys {
		if !m.explicitLists[key] {
			return fmt.Errorf("manifest must explicitly declare %s", key)
		}
	}
	for _, key := range []string{"file", "blockId", "source"} {
		if strings.TrimSpace(m.explicitManagedBlock[key]) == "" {
			return fmt.Errorf("managedBlock is missing required key: %s", key)
		}
	}
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
	if len(m.PromoteFiles) == 0 {
		return fmt.Errorf("promoteFiles must include at least one managed file")
	}
	for _, rel := range m.PromoteFiles {
		if !managed[rel] {
			return fmt.Errorf("promoteFiles entry is not managed: %s", rel)
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
		return fmt.Errorf("toolingCandidateSources must include at least one source; implicit vmp-re fallback is not allowed")
	}
	for _, rel := range m.ToolingCandidateSources {
		if _, err := m.SourcePath(rel); err != nil {
			return err
		}
	}
	if !m.explicitMaps["workstreamDefaults"] {
		return fmt.Errorf("manifest must explicitly declare workstreamDefaults")
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
		return fmt.Errorf("authorityFiles must include at least one authority file")
	}
	if err := m.validateSyncPolicy(); err != nil {
		return err
	}
	if err := m.validateBudgets(); err != nil {
		return err
	}
	if len(m.PromoteDenyPatterns) == 0 {
		return fmt.Errorf("promoteDenyPatterns must include at least one pattern")
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
	if err := m.validateHeavyToolGates(); err != nil {
		return err
	}
	if err := m.validateLaneTypes(); err != nil {
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
	if !strings.EqualFold(m.Pack, defaults.DefaultPack) {
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
		if strings.TrimSpace(route.Trigger) == "" {
			return fmt.Errorf("subagent route %s is missing trigger", id)
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

func (m *Manifest) HeavyToolGate(action string) (HeavyToolGate, bool) {
	action = strings.ToLower(strings.TrimSpace(action))
	for _, gate := range m.HeavyToolGates {
		if strings.EqualFold(gate.ID, action) {
			return gate, true
		}
	}
	return HeavyToolGate{}, false
}

func (m *Manifest) HeavyToolGateIDs() []string {
	ids := make([]string, 0, len(m.HeavyToolGates))
	seen := map[string]bool{}
	for _, gate := range m.HeavyToolGates {
		id := strings.ToLower(strings.TrimSpace(gate.ID))
		if id == "" || seen[id] {
			continue
		}
		ids = append(ids, id)
		seen[id] = true
	}
	sort.Strings(ids)
	return ids
}

func (m *Manifest) validateHeavyToolGates() error {
	if len(m.HeavyToolGates) == 0 {
		return fmt.Errorf("heavyToolGates must include at least one gate")
	}
	seen := map[string]bool{}
	idPattern := regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	for _, gate := range m.HeavyToolGates {
		id := strings.ToLower(strings.TrimSpace(gate.ID))
		if id == "" {
			return fmt.Errorf("heavyToolGates entry is missing id")
		}
		if !idPattern.MatchString(id) {
			return fmt.Errorf("heavyToolGates entry has invalid id: %s", gate.ID)
		}
		if seen[id] {
			return fmt.Errorf("duplicate heavyToolGates id: %s", gate.ID)
		}
		seen[id] = true
		if strings.TrimSpace(gate.Title) == "" {
			return fmt.Errorf("heavyToolGates entry %s is missing title", id)
		}
		if len(gate.SideEffects) == 0 {
			return fmt.Errorf("heavyToolGates entry %s is missing sideEffects", id)
		}
		effects := map[string]bool{}
		for _, effect := range gate.SideEffects {
			effect = strings.ToLower(strings.TrimSpace(effect))
			if effect == "" {
				return fmt.Errorf("heavyToolGates entry %s contains an empty sideEffects item", id)
			}
			if !idPattern.MatchString(effect) {
				return fmt.Errorf("heavyToolGates entry %s has invalid sideEffects item: %s", id, effect)
			}
			effects[effect] = true
		}
		if !effects[id] {
			return fmt.Errorf("heavyToolGates entry %s sideEffects must include the action id", id)
		}
		if strings.TrimSpace(gate.explicitRequiresConfirmation) == "" {
			return fmt.Errorf("heavyToolGates entry %s is missing requiresConfirmation", id)
		}
		requiresConfirmation, ok := parseStrictBool(gate.explicitRequiresConfirmation)
		if !ok {
			return fmt.Errorf("heavyToolGates entry %s has invalid requiresConfirmation: %s", id, gate.explicitRequiresConfirmation)
		}
		if !requiresConfirmation {
			return fmt.Errorf("heavyToolGates entry %s must set requiresConfirmation: true", id)
		}
		if !gate.RequiresConfirmation {
			return fmt.Errorf("heavyToolGates entry %s must set requiresConfirmation: true", id)
		}
		if !supportedHeavyToolRisk(gate.DefaultRisk) {
			return fmt.Errorf("heavyToolGates entry %s has unsupported defaultRisk: %s", id, gate.DefaultRisk)
		}
		if len(gate.StopConditions) == 0 {
			return fmt.Errorf("heavyToolGates entry %s is missing stopConditions", id)
		}
		for _, condition := range gate.StopConditions {
			if strings.TrimSpace(condition) == "" {
				return fmt.Errorf("heavyToolGates entry %s contains an empty stopConditions item", id)
			}
		}
	}
	return nil
}

func supportedHeavyToolRisk(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "medium", "high", "critical":
		return true
	default:
		return false
	}
}

func optionPositiveInt(value string) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return n
}

func (m *Manifest) validateLaneTypes() error {
	if len(m.LaneTypes) == 0 {
		return fmt.Errorf("laneTypes must include at least one lane type")
	}
	seen := map[string]bool{}
	for _, lane := range m.LaneTypes {
		id := strings.TrimSpace(lane.ID)
		if id == "" {
			return fmt.Errorf("laneTypes entry is missing id")
		}
		if seen[strings.ToLower(id)] {
			return fmt.Errorf("duplicate laneTypes id: %s", id)
		}
		seen[strings.ToLower(id)] = true
		if strings.TrimSpace(lane.Title) == "" {
			return fmt.Errorf("laneTypes entry %s is missing title", id)
		}
		if strings.TrimSpace(lane.WorkspaceRoot) == "" {
			return fmt.Errorf("laneTypes entry %s is missing workspaceRoot", id)
		}
		if strings.TrimSpace(lane.explicitAuthority) == "" {
			return fmt.Errorf("laneTypes entry %s is missing authority", id)
		}
		if _, ok := parseStrictBool(lane.explicitAuthority); !ok {
			return fmt.Errorf("laneTypes entry %s has invalid authority: %s", id, lane.explicitAuthority)
		}
		if _, err := m.SourcePath(lane.WorkspaceRoot); err != nil {
			return err
		}
		if len(lane.CanWrite) == 0 {
			return fmt.Errorf("laneTypes entry %s is missing canWrite", id)
		}
		if len(lane.ReadOnly) == 0 {
			return fmt.Errorf("laneTypes entry %s is missing readOnly", id)
		}
		if len(lane.Outputs) == 0 {
			return fmt.Errorf("laneTypes entry %s is missing outputs", id)
		}
		for _, rel := range lane.CanWrite {
			if strings.TrimSpace(rel) == "" {
				return fmt.Errorf("laneTypes entry %s contains an empty canWrite item", id)
			}
			if rel != "own-workspace" {
				if _, err := m.SourcePath(rel); err != nil {
					return err
				}
			}
		}
		for _, rel := range lane.ReadOnly {
			if strings.TrimSpace(rel) == "" {
				return fmt.Errorf("laneTypes entry %s contains an empty readOnly item", id)
			}
			if !strings.ContainsAny(rel, "*") {
				if _, err := m.SourcePath(rel); err != nil {
					return err
				}
			}
		}
		for _, output := range lane.Outputs {
			if strings.TrimSpace(output) == "" {
				return fmt.Errorf("laneTypes entry %s contains an empty outputs item", id)
			}
		}
	}
	return nil
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

func yamlListPresence(lines []string, keys ...string) map[string]bool {
	out := map[string]bool{}
	for _, key := range keys {
		re := regexp.MustCompile(`^` + regexp.QuoteMeta(key) + `\s*:`)
		out[key] = slices.ContainsFunc(lines, re.MatchString)
	}
	return out
}

func yamlMapPresence(lines []string, keys ...string) map[string]bool {
	return yamlListPresence(lines, keys...)
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
		lanes = append(lanes, LaneType{
			ID:                row["id"],
			Title:             row["title"],
			Authority:         parseBool(row["authority"]),
			explicitAuthority: row["authority"],
			WorkspaceRoot:     row["workspaceRoot"],
			CanWrite:          splitScalarList(row["canWrite"]),
			ReadOnly:          splitScalarList(row["readOnly"]),
			Outputs:           splitScalarList(row["outputs"]),
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

func yamlHeavyToolGates(lines []string, key string) []HeavyToolGate {
	rows := yamlObjectList(lines, key)
	gates := make([]HeavyToolGate, 0, len(rows))
	for _, row := range rows {
		gates = append(gates, HeavyToolGate{
			ID:                           strings.ToLower(strings.TrimSpace(row["id"])),
			Title:                        row["title"],
			SideEffects:                  splitScalarList(row["sideEffects"]),
			DefaultRisk:                  strings.ToLower(strings.TrimSpace(row["defaultRisk"])),
			RequiresConfirmation:         parseBool(row["requiresConfirmation"]),
			explicitRequiresConfirmation: row["requiresConfirmation"],
			StopConditions:               splitScalarList(row["stopConditions"]),
		})
	}
	return gates
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

func parseStrictBool(v string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true":
		return true, true
	case "false":
		return false, true
	default:
		return false, false
	}
}

func contains(items []string, value string) bool {
	return slices.Contains(items, value)
}
