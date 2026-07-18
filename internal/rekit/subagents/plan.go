package subagents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
)

const commandName = "plan-subagents"

type Options struct {
	Route           string
	TaskType        string
	Items           string
	ItemsFile       string
	ItemsPerAgent   int
	MaxParallel     int
	ReviewOutputDir string
	PacketPath      string
	DiffPath        string
}

type Result struct {
	SchemaVersion         int            `json:"schemaVersion"`
	Command               string         `json:"command"`
	PlanRoot              string         `json:"planRoot"`
	RepoRoot              string         `json:"repoRoot"`
	Pack                  string         `json:"pack"`
	IsMutation            bool           `json:"isMutation"`
	WritesReviewArtifacts bool           `json:"writesReviewArtifacts"`
	ReviewRequired        bool           `json:"reviewRequired"`
	ReviewRoot            string         `json:"reviewRoot"`
	PacketPath            string         `json:"packetPath"`
	SummaryPath           string         `json:"summaryPath"`
	CombinedDiffPath      string         `json:"combinedDiffPath"`
	ItemCount             int            `json:"itemCount"`
	ShardCount            int            `json:"shardCount"`
	ShardHandoffs         []ShardHandoff `json:"shardHandoffs"`
	Observability         Observability  `json:"observability"`
	ReviewLoop            ReviewLoop     `json:"reviewLoop"`
}

type Packet struct {
	SchemaVersion             int            `json:"schemaVersion"`
	Command                   string         `json:"command"`
	IsMutation                bool           `json:"isMutation"`
	WritesReviewArtifacts     bool           `json:"writesReviewArtifacts"`
	RepoRoot                  string         `json:"repoRoot"`
	Pack                      string         `json:"pack"`
	ManifestPath              string         `json:"manifestPath"`
	Route                     Route          `json:"route"`
	Input                     Input          `json:"input"`
	ShardPolicy               ShardPolicy    `json:"shardPolicy"`
	Shards                    []Shard        `json:"shards"`
	ShardHandoffs             []ShardHandoff `json:"shardHandoffs"`
	MainAgentResponsibilities string         `json:"mainAgentResponsibilities"`
	SubagentPermissions       string         `json:"subagentPermissions"`
	OutputContract            string         `json:"outputContract"`
	ReviewRequired            bool           `json:"reviewRequired"`
	Observability             Observability  `json:"observability"`
	ReviewLoop                ReviewLoop     `json:"reviewLoop"`
}

type Observability struct {
	DispatchMode     string        `json:"dispatchMode"`
	RouteDebug       RouteDebug    `json:"routeDebug"`
	ReviewRoot       string        `json:"reviewRoot"`
	PacketPath       string        `json:"packetPath"`
	SummaryPath      string        `json:"summaryPath"`
	CombinedDiffPath string        `json:"combinedDiffPath"`
	ShardStatuses    []ShardStatus `json:"shardStatuses"`
	BlockedActions   []string      `json:"blockedActions"`
}

type RouteDebug struct {
	SelectedBy    string `json:"selectedBy"`
	RouteID       string `json:"routeId"`
	TaskTypes     string `json:"taskTypes"`
	Trigger       string `json:"trigger"`
	Reference     string `json:"reference"`
	PolicyOverlay string `json:"policyOverlay"`
}

type ShardStatus struct {
	ShardID        string `json:"shardId"`
	Status         string `json:"status"`
	ItemCount      int    `json:"itemCount"`
	ExpectedOutput string `json:"expectedOutput"`
}

type ReviewLoop struct {
	SpawnOwner         string   `json:"spawnOwner"`
	MergeOwner         string   `json:"mergeOwner"`
	MainAgentOwns      []string `json:"mainAgentOwns"`
	VerdictWriteback   string   `json:"verdictWriteback"`
	CompletionCriteria []string `json:"completionCriteria"`
	FailureHandling    string   `json:"failureHandling"`
}

type Route struct {
	ID                  string `json:"id"`
	TaskTypes           string `json:"taskTypes"`
	Trigger             string `json:"trigger"`
	ShardBasis          string `json:"shardBasis"`
	TargetItemsPerAgent string `json:"targetItemsPerAgent"`
	MaxParallel         string `json:"maxParallel"`
	Reference           string `json:"reference"`
	PolicyOverlay       string `json:"policyOverlay"`
	SubagentPermissions string `json:"subagentPermissions"`
	MainAgentOwns       string `json:"mainAgentOwns"`
	OutputContract      string `json:"outputContract"`
}

type Input struct {
	TaskType  string `json:"taskType"`
	ItemCount int    `json:"itemCount"`
	ItemsFile string `json:"itemsFile"`
}

type ShardPolicy struct {
	Basis               string `json:"basis"`
	TargetItemsPerAgent int    `json:"targetItemsPerAgent"`
	MaxParallel         int    `json:"maxParallel"`
}

type Shard struct {
	ID     string   `json:"id"`
	Items  []string `json:"items"`
	Prompt string   `json:"prompt"`
}

type ShardHandoff struct {
	ShardID                  string                    `json:"shardId"`
	Status                   string                    `json:"status"`
	DispatchPrompt           string                    `json:"dispatchPrompt"`
	Items                    []string                  `json:"items"`
	ReadOnlyBoundary         []string                  `json:"readOnlyBoundary"`
	ExpectedOutput           string                    `json:"expectedOutput"`
	ReviewerWriteback        string                    `json:"reviewerWriteback"`
	LedgerWritebackTemplates []LedgerWritebackTemplate `json:"ledgerWritebackTemplates"`
	MainAgentNextAction      string                    `json:"mainAgentNextAction"`
	PostReviewMerge          []string                  `json:"postReviewMerge"`
	CompletionCriteria       []string                  `json:"completionCriteria"`
	FailureHandling          string                    `json:"failureHandling"`
}

type LedgerWritebackTemplate struct {
	Kind           string   `json:"kind"`
	Purpose        string   `json:"purpose"`
	Command        string   `json:"command"`
	RequiredFields []string `json:"requiredFields"`
	AllowedValues  []string `json:"allowedValues,omitempty"`
}

type artifactPaths struct {
	Root             string
	DiffRoot         string
	PreviewRoot      string
	PacketPath       string
	SummaryPath      string
	CombinedDiffPath string
}

func WritePlan(repoRoot, target, pack string, opt Options) (Result, error) {
	planRoot, err := filepath.Abs(target)
	if err != nil {
		return Result{}, err
	}
	caseTarget := instance.LooksLikeCase(planRoot)
	if caseTarget {
		if _, err := instance.AssertAttached(planRoot, repoRoot, pack); err != nil {
			return Result{}, err
		}
	} else {
		if strings.TrimSpace(opt.ReviewOutputDir) == "" {
			return Result{}, fmt.Errorf("plan-subagents target must be an attached rekit case unless -ReviewOutputDir is provided for an explicit out-of-case review artifact path")
		}
		st, err := os.Stat(planRoot)
		if err != nil {
			if os.IsNotExist(err) {
				return Result{}, fmt.Errorf("plan-subagents target directory does not exist: %s", planRoot)
			}
			return Result{}, err
		}
		if !st.IsDir() {
			return Result{}, fmt.Errorf("plan-subagents target is not a directory: %s", planRoot)
		}
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return Result{}, err
	}
	route, err := selectRoute(m, opt.Route, opt.TaskType)
	if err != nil {
		return Result{}, err
	}
	items, itemsFile, err := splitItems(opt.Items, opt.ItemsFile)
	if err != nil {
		return Result{}, err
	}
	itemsPerAgent := optionInt(route.TargetItemsPerAgent, 4)
	if opt.ItemsPerAgent > 0 {
		itemsPerAgent = opt.ItemsPerAgent
	}
	maxParallel := optionInt(route.MaxParallel, 3)
	if opt.MaxParallel > 0 {
		maxParallel = opt.MaxParallel
	}
	shards := newShards(items, itemsPerAgent)
	paths, err := makeArtifactPaths(planRoot, opt)
	if err != nil {
		return Result{}, err
	}
	if err := prepareArtifactDirs(paths); err != nil {
		return Result{}, err
	}
	observability := newObservability(route, opt, paths, shards)
	reviewLoop := newReviewLoop(route)
	shardHandoffs := newShardHandoffs(shards, route, observability, reviewLoop)
	packet := Packet{
		SchemaVersion:             1,
		Command:                   commandName,
		IsMutation:                false,
		WritesReviewArtifacts:     true,
		RepoRoot:                  m.RepoRoot,
		Pack:                      m.Pack,
		ManifestPath:              m.ManifestPath,
		Route:                     route,
		Input:                     Input{TaskType: opt.TaskType, ItemCount: len(items), ItemsFile: itemsFile},
		ShardPolicy:               ShardPolicy{Basis: route.ShardBasis, TargetItemsPerAgent: itemsPerAgent, MaxParallel: maxParallel},
		Shards:                    shards,
		ShardHandoffs:             shardHandoffs,
		MainAgentResponsibilities: route.MainAgentOwns,
		SubagentPermissions:       route.SubagentPermissions,
		OutputContract:            route.OutputContract,
		ReviewRequired:            true,
		Observability:             observability,
		ReviewLoop:                reviewLoop,
	}
	if err := writeJSON(paths.PacketPath, packet); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(paths.SummaryPath, []byte(summaryText(route, opt.TaskType, len(items), len(shards), itemsPerAgent, maxParallel, observability, reviewLoop, shardHandoffs)), 0o644); err != nil {
		return Result{}, err
	}
	return Result{SchemaVersion: 1, Command: commandName, PlanRoot: planRoot, RepoRoot: m.RepoRoot, Pack: m.Pack, IsMutation: false, WritesReviewArtifacts: true, ReviewRequired: true, ReviewRoot: paths.Root, PacketPath: paths.PacketPath, SummaryPath: paths.SummaryPath, CombinedDiffPath: paths.CombinedDiffPath, ItemCount: len(items), ShardCount: len(shards), ShardHandoffs: shardHandoffs, Observability: observability, ReviewLoop: reviewLoop}, nil
}

func selectRoute(m *manifest.Manifest, routeID, taskType string) (Route, error) {
	if len(m.SubagentRoutes) == 0 {
		return Route{}, fmt.Errorf("manifest has no subagentRoutes: %s", m.ManifestPath)
	}
	if strings.TrimSpace(routeID) != "" {
		for _, route := range m.SubagentRoutes {
			if strings.EqualFold(route.ID, routeID) {
				return toRoute(route), nil
			}
		}
		return Route{}, fmt.Errorf("subagent route not found: %s", routeID)
	}
	if strings.TrimSpace(taskType) != "" {
		for _, route := range m.SubagentRoutes {
			for _, task := range strings.FieldsFunc(route.TaskTypes, func(r rune) bool { return r == ',' || r == ';' }) {
				if strings.EqualFold(strings.TrimSpace(task), taskType) {
					return toRoute(route), nil
				}
			}
		}
	}
	return toRoute(m.SubagentRoutes[0]), nil
}

func toRoute(route manifest.SubagentRoute) Route {
	return Route{ID: route.ID, TaskTypes: route.TaskTypes, Trigger: route.Trigger, ShardBasis: route.ShardBasis, TargetItemsPerAgent: route.TargetItemsPerAgent, MaxParallel: route.MaxParallel, Reference: route.Reference, PolicyOverlay: route.PolicyOverlay, SubagentPermissions: route.SubagentPermissions, MainAgentOwns: route.MainAgentOwns, OutputContract: route.OutputContract}
}

func splitItems(items, itemsFile string) ([]string, string, error) {
	text := items
	originalFile := strings.TrimSpace(itemsFile)
	if originalFile != "" {
		abs, err := filepath.Abs(originalFile)
		if err != nil {
			return nil, "", err
		}
		b, err := os.ReadFile(abs)
		if os.IsNotExist(err) {
			return nil, "", fmt.Errorf("missing plan items file: %s", abs)
		}
		if err != nil {
			return nil, "", err
		}
		text = string(b)
	}
	if strings.TrimSpace(text) == "" {
		return []string{}, originalFile, nil
	}
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == ';' || r == '\r' || r == '\n' || r == '\t' || r == ' '
	})
	out := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out, originalFile, nil
}

func newShards(items []string, targetItemsPerAgent int) []Shard {
	if targetItemsPerAgent < 1 {
		targetItemsPerAgent = 4
	}
	shards := []Shard{}
	for start := 0; start < len(items); start += targetItemsPerAgent {
		end := min(start+targetItemsPerAgent, len(items))
		slice := append([]string{}, items[start:end]...)
		shards = append(shards, Shard{ID: fmt.Sprintf("shard-%02d", len(shards)+1), Items: slice, Prompt: shardPrompt(slice)})
	}
	return shards
}

func shardPrompt(items []string) string {
	return "Review only these items: " + strings.Join(items, ", ") + ". Return the route output contract only; do not write files or paste long logs."
}

func newShardHandoffs(shards []Shard, route Route, observability Observability, reviewLoop ReviewLoop) []ShardHandoff {
	handoffs := make([]ShardHandoff, 0, len(shards))
	readOnlyBoundary := append([]string{}, observability.BlockedActions...)
	for _, shard := range shards {
		handoffs = append(handoffs, ShardHandoff{
			ShardID:                  shard.ID,
			Status:                   "planned",
			DispatchPrompt:           shardDispatchPrompt(shard, route, readOnlyBoundary, reviewLoop),
			Items:                    append([]string{}, shard.Items...),
			ReadOnlyBoundary:         append([]string{}, readOnlyBoundary...),
			ExpectedOutput:           route.OutputContract,
			ReviewerWriteback:        reviewLoop.VerdictWriteback,
			LedgerWritebackTemplates: ledgerWritebackTemplates(shard),
			MainAgentNextAction:      "launch a read-only reviewer with dispatchPrompt, then use ledgerWritebackTemplates to record reviewer verification and main merge decision",
			PostReviewMerge:          postReviewMergeSteps(),
			CompletionCriteria:       append([]string{}, reviewLoop.CompletionCriteria...),
			FailureHandling:          reviewLoop.FailureHandling,
		})
	}
	return handoffs
}

func ledgerWritebackTemplates(shard Shard) []LedgerWritebackTemplate {
	itemRef := strings.Join(shard.Items, ",")
	if itemRef == "" {
		itemRef = shard.ID
	}
	return []LedgerWritebackTemplate{
		{
			Kind:           "verification",
			Purpose:        "record a read-only reviewer verdict for this shard after the main agent inspects the reviewer output",
			Command:        "/rekit note -Kind verification -Lane <lane> -Verifier manual-review -Verdict <accepted|rejected|inconclusive|needs-more-evidence> -TargetRef \"" + itemRef + "\" -Subject \"reviewer verdict for " + shard.ID + "\" -Summary \"<short reviewer verdict summary>\" -EvidenceRefs \"<packet-or-evidence-ref>\" -Actor <main-agent> -Apply",
			RequiredFields: []string{"lane", "verifier", "verdict", "target", "subject", "summary", "evidenceRefs", "actor"},
			AllowedValues:  []string{"verifier=manual-review|schema-check|focused-trace|parity|cross-run|tool-review", "verdict=accepted|rejected|inconclusive|needs-more-evidence"},
		},
		{
			Kind:           "decision",
			Purpose:        "record the main agent merge decision for this shard after validation and conflict review",
			Command:        "/rekit note -Kind decision -Lane <lane> -Decision <accept|reject|defer|supersede> -TargetRef \"" + itemRef + "\" -Subject \"main merge decision for " + shard.ID + "\" -Summary \"<merge decision and reason>\" -EvidenceRefs \"<verification-event-or-packet-ref>\" -Actor <main-agent> -Apply",
			RequiredFields: []string{"lane", "decision", "target", "subject", "summary", "evidenceRefs", "actor"},
			AllowedValues:  []string{"decision=accept|reject|defer|supersede"},
		},
	}
}

func postReviewMergeSteps() []string {
	return []string{
		"inspect reviewer output against expectedOutput before ledger writeback",
		"record reviewer verdict with the verification template; do not let the reviewer append ledger events directly",
		"record the main merge decision with the decision template only after validation/conflict review",
		"run the relevant overview/handoff/doctor check after accepted decisions that affect lane state",
	}
}

func shardDispatchPrompt(shard Shard, route Route, readOnlyBoundary []string, reviewLoop ReviewLoop) string {
	lines := []string{
		"You are a read-only reviewer for rekit plan-subagents shard " + shard.ID + ".",
		"Route: " + route.ID + ".",
		"Items: " + strings.Join(shard.Items, ", ") + ".",
		"Return only this output contract: " + route.OutputContract + ".",
		"Do not write files, run heavy tools, append ledgers, or change authority/confirmed state.",
		"The main agent owns merge, validation, handoff, and ledger writeback: " + reviewLoop.VerdictWriteback + ".",
	}
	if len(readOnlyBoundary) > 0 {
		lines = append(lines, "Blocked runtime actions: "+strings.Join(readOnlyBoundary, "; ")+".")
	}
	return strings.Join(lines, " ")
}

func newObservability(route Route, opt Options, paths artifactPaths, shards []Shard) Observability {
	statuses := make([]ShardStatus, 0, len(shards))
	for _, shard := range shards {
		statuses = append(statuses, ShardStatus{ShardID: shard.ID, Status: "planned", ItemCount: len(shard.Items), ExpectedOutput: route.OutputContract})
	}
	return Observability{
		DispatchMode: "manual-main-agent",
		RouteDebug: RouteDebug{
			SelectedBy:    routeSelectionReason(route, opt),
			RouteID:       route.ID,
			TaskTypes:     route.TaskTypes,
			Trigger:       route.Trigger,
			Reference:     route.Reference,
			PolicyOverlay: route.PolicyOverlay,
		},
		ReviewRoot:       paths.Root,
		PacketPath:       paths.PacketPath,
		SummaryPath:      paths.SummaryPath,
		CombinedDiffPath: paths.CombinedDiffPath,
		ShardStatuses:    statuses,
		BlockedActions: []string{
			"runtime does not spawn subagents",
			"subagents must not write files",
			"main agent owns ledger writeback, validation, handoff, authority, and confirmed writes",
		},
	}
}

func routeSelectionReason(route Route, opt Options) string {
	if strings.TrimSpace(opt.Route) != "" {
		return "route"
	}
	taskType := strings.TrimSpace(opt.TaskType)
	if taskType != "" {
		for _, task := range strings.FieldsFunc(route.TaskTypes, func(r rune) bool { return r == ',' || r == ';' }) {
			if strings.EqualFold(strings.TrimSpace(task), taskType) {
				return "taskType"
			}
		}
		return "manifest-default"
	}
	return "manifest-default"
}

func newReviewLoop(route Route) ReviewLoop {
	mainOwns := splitCSV(route.MainAgentOwns)
	return ReviewLoop{
		SpawnOwner:       "main-agent",
		MergeOwner:       "main-agent",
		MainAgentOwns:    mainOwns,
		VerdictWriteback: "/rekit note -Kind verification for reviewer verdicts; /rekit note -Kind decision for main merge decisions",
		CompletionCriteria: []string{
			"each planned shard is accepted, rejected, deferred, or explicitly abandoned",
			"reviewer verdicts are recorded in the ledger before main merge decisions",
			"accepted writes remain gated by main-agent validation and authority/confirmed confirmation",
		},
		FailureHandling: "discard failed shard result and retry later with a smaller bounded shard; do not block unrelated shards",
	}
}

func splitCSV(value string) []string {
	items := []string{}
	for _, part := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' }) {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func optionInt(value string, fallback int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && n > 0 {
		return n
	}
	return fallback
}

func makeArtifactPaths(planRoot string, opt Options) (artifactPaths, error) {
	defaultRoot := strings.TrimSpace(opt.ReviewOutputDir) == ""
	root := strings.TrimSpace(opt.ReviewOutputDir)
	var err error
	if defaultRoot {
		root, err = refsf.SafeJoin(planRoot, filepath.ToSlash(filepath.Join(".rekit", "reviews", time.Now().Format("20060102-150405000")+"-"+commandName)))
		if err != nil {
			return artifactPaths{}, err
		}
	} else if root, err = filepath.Abs(root); err != nil {
		return artifactPaths{}, err
	}
	packet := strings.TrimSpace(opt.PacketPath)
	if packet == "" {
		packet = filepath.Join(root, "packet.json")
	} else if packet, err = filepath.Abs(packet); err != nil {
		return artifactPaths{}, err
	}
	diffRoot := filepath.Join(root, "diffs")
	combined := strings.TrimSpace(opt.DiffPath)
	if combined == "" {
		combined = filepath.Join(diffRoot, "combined.diff")
	} else if combined, err = filepath.Abs(combined); err != nil {
		return artifactPaths{}, err
	}
	if defaultRoot {
		if err := requirePathUnder(root, packet, "packet path"); err != nil {
			return artifactPaths{}, err
		}
		if err := requirePathUnder(root, combined, "diff path"); err != nil {
			return artifactPaths{}, err
		}
	}
	return artifactPaths{Root: root, DiffRoot: diffRoot, PreviewRoot: filepath.Join(root, "previews"), PacketPath: packet, SummaryPath: filepath.Join(root, "summary.md"), CombinedDiffPath: combined}, nil
}

func requirePathUnder(root, path, label string) error {
	rootFull, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	pathFull, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rootClean := strings.TrimRight(filepath.Clean(rootFull), string(filepath.Separator))
	pathClean := strings.TrimRight(filepath.Clean(pathFull), string(filepath.Separator))
	prefix := rootClean + string(filepath.Separator)
	if !strings.EqualFold(pathClean, rootClean) && !strings.HasPrefix(strings.ToLower(pathClean), strings.ToLower(prefix)) {
		return fmt.Errorf("%s escapes review root: %s", label, path)
	}
	return nil
}

func prepareArtifactDirs(paths artifactPaths) error {
	for _, dir := range []string{paths.Root, paths.DiffRoot, paths.PreviewRoot, filepath.Dir(paths.PacketPath), filepath.Dir(paths.CombinedDiffPath)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if err := os.Remove(paths.CombinedDiffPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func summaryText(route Route, taskType string, itemCount, shardCount, itemsPerAgent, maxParallel int, observability Observability, reviewLoop ReviewLoop, shardHandoffs []ShardHandoff) string {
	lines := []string{
		"# rekit subagent plan",
		"",
		"- route: `" + route.ID + "`",
		"- task type: `" + taskType + "`",
		fmt.Sprintf("- items: `%d`", itemCount),
		fmt.Sprintf("- shards: `%d`", shardCount),
		fmt.Sprintf("- target items per agent: `%d`", itemsPerAgent),
		fmt.Sprintf("- max parallel: `%d`", maxParallel),
		"- writes review artifacts: `true`",
		"",
		"## bounded dispatch observability",
		"",
		"- dispatch mode: `" + observability.DispatchMode + "`",
		"- route selected by: `" + observability.RouteDebug.SelectedBy + "`",
		"- review root: `" + observability.ReviewRoot + "`",
		"- packet: `" + observability.PacketPath + "`",
		"- combined diff: `" + observability.CombinedDiffPath + "`",
		"- spawn owner: `" + reviewLoop.SpawnOwner + "`",
		"- merge owner: `" + reviewLoop.MergeOwner + "`",
		"- verdict writeback: `" + reviewLoop.VerdictWriteback + "`",
		"",
		"### shard status",
		"",
	}
	if len(observability.ShardStatuses) == 0 {
		lines = append(lines, "- no shards planned")
	} else {
		for _, status := range observability.ShardStatuses {
			lines = append(lines, fmt.Sprintf("- %s: `%s`, items=`%d`", status.ShardID, status.Status, status.ItemCount))
		}
	}
	lines = append(lines,
		"",
		"### blocked runtime actions",
		"",
	)
	for _, action := range observability.BlockedActions {
		lines = append(lines, "- "+action)
	}
	lines = append(lines,
		"",
		"### shard handoff prompts",
		"",
	)
	if len(shardHandoffs) == 0 {
		lines = append(lines, "- no shard handoffs planned")
	} else {
		for _, handoff := range shardHandoffs {
			lines = append(lines, fmt.Sprintf("- %s: `%s`; expected output=`%s`", handoff.ShardID, handoff.DispatchPrompt, handoff.ExpectedOutput))
			for _, tmpl := range handoff.LedgerWritebackTemplates {
				lines = append(lines, fmt.Sprintf("  - %s writeback: `%s`; required=`%s`", tmpl.Kind, tmpl.Command, strings.Join(tmpl.RequiredFields, ",")))
			}
			for _, step := range handoff.PostReviewMerge {
				lines = append(lines, "  - post-review: "+step)
			}
		}
	}
	lines = append(lines,
		"",
		"### completion criteria",
		"",
	)
	for _, criterion := range reviewLoop.CompletionCriteria {
		lines = append(lines, "- "+criterion)
	}
	lines = append(lines,
		"",
		"Use the generated packet to launch read-only subagents. The command only writes review artifacts; the main agent owns project writes, validation, and handoff updates.",
	)
	return strings.Join(lines, "\r\n") + "\r\n"
}
