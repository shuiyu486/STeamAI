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
	ReviewerResultContract   ReviewerResultContract    `json:"reviewerResultContract"`
	LedgerWritebackTemplates []LedgerWritebackTemplate `json:"ledgerWritebackTemplates"`
	MainAgentNextAction      string                    `json:"mainAgentNextAction"`
	IntakeChecklist          []string                  `json:"intakeChecklist"`
	ReviewerDecisionMappings []ReviewerDecisionMapping `json:"reviewerDecisionMappings"`
	ConflictHandling         []string                  `json:"conflictHandling"`
	WritebackSequence        []WritebackSequenceStep   `json:"writebackSequence"`
	PostReviewMerge          []string                  `json:"postReviewMerge"`
	CompletionCriteria       []string                  `json:"completionCriteria"`
	FailureHandling          string                    `json:"failureHandling"`
}

type ReviewerResultContract struct {
	OutputFormat     string   `json:"outputFormat"`
	RequiredFields   []string `json:"requiredFields"`
	AllowedDecisions []string `json:"allowedDecisions"`
	EvidenceRules    []string `json:"evidenceRules"`
	ConflictSignals  []string `json:"conflictSignals"`
}

type ReviewerDecisionMapping struct {
	ReviewerDecision    string   `json:"reviewerDecision"`
	VerificationVerdict string   `json:"verificationVerdict"`
	MainDecision        string   `json:"mainDecision"`
	ApplyWhen           []string `json:"applyWhen"`
	Fallback            string   `json:"fallback"`
}

type WritebackSequenceStep struct {
	Step            string                    `json:"step"`
	Owner           string                    `json:"owner"`
	Uses            []string                  `json:"uses"`
	CommandBindings []WritebackCommandBinding `json:"commandBindings,omitempty"`
	MustPass        []string                  `json:"mustPass"`
	BlockedBy       []string                  `json:"blockedBy,omitempty"`
	NextOnSuccess   string                    `json:"nextOnSuccess"`
	NextOnFailure   string                    `json:"nextOnFailure"`
}

type WritebackCommandBinding struct {
	Binding        string   `json:"binding"`
	Source         string   `json:"source"`
	Kind           string   `json:"kind,omitempty"`
	Command        string   `json:"command,omitempty"`
	RequiredFields []string `json:"requiredFields,omitempty"`
	ExpectedOutput string   `json:"expectedOutput"`
}

type LedgerWritebackTemplate struct {
	Kind           string   `json:"kind"`
	Purpose        string   `json:"purpose"`
	Command        string   `json:"command"`
	PreviewCommand string   `json:"previewCommand"`
	ApplyCommand   string   `json:"applyCommand"`
	RequiredFields []string `json:"requiredFields"`
	AllowedValues  []string `json:"allowedValues,omitempty"`
	PreviewChecks  []string `json:"previewChecks,omitempty"`
	BlockedOutputs []string `json:"blockedOutputs,omitempty"`
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
		contract := reviewerResultContract()
		intake := intakeChecklist()
		mappings := reviewerDecisionMappings()
		conflicts := conflictHandlingSteps()
		templates := ledgerWritebackTemplates(shard)
		handoffs = append(handoffs, ShardHandoff{
			ShardID:                  shard.ID,
			Status:                   "planned",
			DispatchPrompt:           shardDispatchPrompt(shard, route, readOnlyBoundary, reviewLoop),
			Items:                    append([]string{}, shard.Items...),
			ReadOnlyBoundary:         append([]string{}, readOnlyBoundary...),
			ExpectedOutput:           route.OutputContract,
			ReviewerWriteback:        reviewLoop.VerdictWriteback,
			ReviewerResultContract:   contract,
			LedgerWritebackTemplates: templates,
			MainAgentNextAction:      "launch a read-only reviewer with dispatchPrompt, inspect reviewerResultContract output, map reviewerDecisionMappings, run writebackSequence commandBindings in order, then use ledgerWritebackTemplates previewCommand/applyCommand for reviewer verification and main merge decision",
			IntakeChecklist:          intake,
			ReviewerDecisionMappings: mappings,
			ConflictHandling:         conflicts,
			WritebackSequence:        writebackSequenceSteps(templates),
			PostReviewMerge:          postReviewMergeSteps(),
			CompletionCriteria:       append([]string{}, reviewLoop.CompletionCriteria...),
			FailureHandling:          reviewLoop.FailureHandling,
		})
	}
	return handoffs
}

func reviewerResultContract() ReviewerResultContract {
	return ReviewerResultContract{
		OutputFormat:     "single JSON object per shard; no markdown tables, file writes, ledger appends, authority, confirmed, or heavy-tool output",
		RequiredFields:   []string{"shardId", "items", "decision", "confidence", "summary", "evidenceRefs", "risks", "conflicts", "recommendedVerdict"},
		AllowedDecisions: []string{"accept", "reject", "defer", "abandon", "needs-more-evidence"},
		EvidenceRules: []string{
			"accepted or rejected reviewer decisions must cite evidenceRefs from the packet, reviewed artifacts, or bounded evidence paths",
			"missing, ambiguous, or inaccessible evidenceRefs require decision=needs-more-evidence or defer",
			"do not paste long logs; cite stable packet/evidence references and summarize the relevant observation",
		},
		ConflictSignals: []string{
			"reviewer decision conflicts with evidenceRefs or route output contract",
			"reviewer requests file writes, ledger append, authority/confirmed changes, heavy tools, or external effects",
			"reviewer output overlaps another shard or changes items outside this shard",
			"reviewer confidence is low or evidence cannot be independently inspected by the main agent",
		},
	}
}

func intakeChecklist() []string {
	return []string{
		"validate reviewer output against reviewerResultContract before using any writeback template",
		"confirm every accepted/rejected item has inspected evidenceRefs and no out-of-shard claims",
		"map reviewer decision to verification verdict before running the verification previewCommand",
		"defer the main decision when conflicts, missing evidence, or blocked outputs are present",
		"run note previewCommand before applyCommand and inspect event / wouldExecutorAction before ledger writeback",
	}
}

func reviewerDecisionMappings() []ReviewerDecisionMapping {
	return []ReviewerDecisionMapping{
		{
			ReviewerDecision:    "accept",
			VerificationVerdict: "accepted",
			MainDecision:        "accept",
			ApplyWhen:           []string{"reviewer result validates", "evidenceRefs are inspected", "no conflict signals are present"},
			Fallback:            "defer when evidenceRefs are missing, confidence is low, or conflicts are present",
		},
		{
			ReviewerDecision:    "reject",
			VerificationVerdict: "rejected",
			MainDecision:        "reject",
			ApplyWhen:           []string{"reviewer result validates", "rejection reason cites inspected evidenceRefs", "no out-of-shard claims are present"},
			Fallback:            "defer when rejection evidence cannot be independently inspected",
		},
		{
			ReviewerDecision:    "defer",
			VerificationVerdict: "inconclusive",
			MainDecision:        "defer",
			ApplyWhen:           []string{"reviewer result is valid but evidence is incomplete", "main agent needs another pass or narrower shard"},
			Fallback:            "keep decision=defer and do not apply accept/reject templates until evidence improves",
		},
		{
			ReviewerDecision:    "abandon",
			VerificationVerdict: "inconclusive",
			MainDecision:        "supersede",
			ApplyWhen:           []string{"shard is out of scope, duplicated, or superseded", "main agent has inspected the superseding evidence"},
			Fallback:            "record a defer decision when no superseding evidence has been inspected",
		},
		{
			ReviewerDecision:    "needs-more-evidence",
			VerificationVerdict: "needs-more-evidence",
			MainDecision:        "defer",
			ApplyWhen:           []string{"reviewer cites missing or inaccessible evidenceRefs", "main agent can name the next evidence collection step"},
			Fallback:            "do not apply an accept/reject main decision; collect evidence or split the shard",
		},
	}
}

func conflictHandlingSteps() []string {
	return []string{
		"if reviewer output fails reviewerResultContract validation, do not run writeback templates; retry with a smaller shard or mark decision=defer",
		"if any conflictSignal is present, map verification verdict to inconclusive or needs-more-evidence and keep main decision deferred unless independently resolved",
		"if reviewer decision and recommendedVerdict disagree, inspect evidenceRefs and record the safer non-accepting outcome",
		"if reviewer requests writes, heavy tools, authority/confirmed changes, or external effects, discard that output for ledger purposes and escalate through the lane gate path",
	}
}

func writebackSequenceSteps(templates []LedgerWritebackTemplate) []WritebackSequenceStep {
	verificationTemplate := writebackTemplateByKind(templates, "verification")
	decisionTemplate := writebackTemplateByKind(templates, "decision")
	return []WritebackSequenceStep{
		{
			Step:  "validate-reviewer-result",
			Owner: "main-agent",
			Uses:  []string{"reviewerResultContract", "intakeChecklist"},
			CommandBindings: []WritebackCommandBinding{
				{
					Binding:        "reviewer-output",
					Source:         "reviewerResultContract",
					RequiredFields: reviewerResultContract().RequiredFields,
					ExpectedOutput: "single validated reviewer JSON object; no writes or heavy-tool output",
				},
			},
			MustPass:      []string{"single JSON object validates", "required fields are present", "decision uses an allowed value", "evidenceRefs are inspectable for accepted or rejected outcomes"},
			BlockedBy:     []string{"malformed reviewer output", "missing evidenceRefs", "out-of-shard claims", "reviewer requested writes or heavy tools"},
			NextOnSuccess: "map-reviewer-decision",
			NextOnFailure: "defer-or-retry-shard",
		},
		{
			Step:  "map-reviewer-decision",
			Owner: "main-agent",
			Uses:  []string{"reviewerDecisionMappings", "conflictHandling"},
			CommandBindings: []WritebackCommandBinding{
				{
					Binding:        "decision-map",
					Source:         "reviewerDecisionMappings",
					RequiredFields: []string{"reviewerDecision", "verificationVerdict", "mainDecision"},
					ExpectedOutput: "selected verification verdict and main decision",
				},
				{
					Binding:        "conflict-rules",
					Source:         "conflictHandling",
					RequiredFields: []string{"conflicts", "recommendedVerdict", "evidenceRefs"},
					ExpectedOutput: "conflicts absent, independently resolved, or mapped to safer defer outcome",
				},
			},
			MustPass:      []string{"verification verdict is selected", "main decision is selected", "conflict signals are absent or independently resolved"},
			BlockedBy:     []string{"recommendedVerdict disagreement", "unresolved conflictSignal", "low confidence without inspected evidence"},
			NextOnSuccess: "preview-verification-note",
			NextOnFailure: "record-safer-defer-decision",
		},
		{
			Step:  "preview-verification-note",
			Owner: "main-agent",
			Uses:  []string{"ledgerWritebackTemplates[kind=verification].previewCommand"},
			CommandBindings: []WritebackCommandBinding{
				writebackCommandBinding("verification-preview", "ledgerWritebackTemplates[kind=verification].previewCommand", verificationTemplate, verificationTemplate.PreviewCommand, "note WhatIf JSON preview for verification event"),
			},
			MustPass:      []string{"note WhatIf returns isMutation=false", "note WhatIf returns applied=false", "event.kind is verification", "event lane/target/evidenceRefs match this shard", "wouldExecutorAction contains only the expected lane-local delta"},
			BlockedBy:     []string{"preview fails", "wrong lane or target", "missing inspected evidenceRefs", "unexpected wouldExecutorAction"},
			NextOnSuccess: "apply-verification-note",
			NextOnFailure: "stop-before-ledger-write",
		},
		{
			Step:  "apply-verification-note",
			Owner: "main-agent",
			Uses:  []string{"ledgerWritebackTemplates[kind=verification].applyCommand"},
			CommandBindings: []WritebackCommandBinding{
				writebackCommandBinding("verification-apply", "ledgerWritebackTemplates[kind=verification].applyCommand", verificationTemplate, verificationTemplate.ApplyCommand, "applied verification ledger event owned by main agent"),
			},
			MustPass:      []string{"verification preview passed", "selected verdict matches reviewerDecisionMappings", "main agent has inspected the cited evidenceRefs"},
			NextOnSuccess: "preview-main-decision-note",
			NextOnFailure: "stop-and-inspect-ledger",
		},
		{
			Step:  "preview-main-decision-note",
			Owner: "main-agent",
			Uses:  []string{"ledgerWritebackTemplates[kind=decision].previewCommand"},
			CommandBindings: []WritebackCommandBinding{
				writebackCommandBinding("decision-preview", "ledgerWritebackTemplates[kind=decision].previewCommand", decisionTemplate, decisionTemplate.PreviewCommand, "note WhatIf JSON preview for main decision event"),
			},
			MustPass:      []string{"note WhatIf returns isMutation=false", "note WhatIf returns applied=false", "event.kind is decision", "event decision matches reviewerDecisionMappings", "event evidenceRefs cite the verification event or packet evidence"},
			BlockedBy:     []string{"verification note missing", "unresolved conflict", "decision is accepting without inspected evidence"},
			NextOnSuccess: "apply-main-decision-note",
			NextOnFailure: "keep-main-decision-deferred",
		},
		{
			Step:  "apply-main-decision-note",
			Owner: "main-agent",
			Uses:  []string{"ledgerWritebackTemplates[kind=decision].applyCommand"},
			CommandBindings: []WritebackCommandBinding{
				writebackCommandBinding("decision-apply", "ledgerWritebackTemplates[kind=decision].applyCommand", decisionTemplate, decisionTemplate.ApplyCommand, "applied main decision ledger event owned by main agent"),
			},
			MustPass:      []string{"decision preview passed", "verification evidence is referenced", "accept/reject/supersede decisions have no unresolved conflict"},
			NextOnSuccess: "post-review-validation",
			NextOnFailure: "stop-and-inspect-ledger",
		},
		{
			Step:  "post-review-validation",
			Owner: "main-agent",
			Uses:  []string{"postReviewMerge", "overview", "handoff", "doctor"},
			CommandBindings: []WritebackCommandBinding{
				{
					Binding:        "post-review-validation",
					Source:         "postReviewMerge",
					RequiredFields: []string{"overview", "handoff", "doctor"},
					ExpectedOutput: "lane state, blocker summary, and handoff readiness rechecked after ledger writes",
				},
			},
			MustPass:      []string{"accepted decisions that affect lane state are rechecked", "handoff or overview reflects the resulting blocker state", "no reviewer output was treated as a ledger event without main-agent apply"},
			NextOnSuccess: "handoff-or-continue-ready-lane",
			NextOnFailure: "open-main-agent-blocker",
		},
	}
}

func writebackTemplateByKind(templates []LedgerWritebackTemplate, kind string) LedgerWritebackTemplate {
	for _, template := range templates {
		if template.Kind == kind {
			return template
		}
	}
	return LedgerWritebackTemplate{Kind: kind}
}

func writebackCommandBinding(binding, source string, template LedgerWritebackTemplate, command, expectedOutput string) WritebackCommandBinding {
	return WritebackCommandBinding{
		Binding:        binding,
		Source:         source,
		Kind:           template.Kind,
		Command:        command,
		RequiredFields: append([]string{}, template.RequiredFields...),
		ExpectedOutput: expectedOutput,
	}
}

func ledgerWritebackTemplates(shard Shard) []LedgerWritebackTemplate {
	itemRef := strings.Join(shard.Items, ",")
	if itemRef == "" {
		itemRef = shard.ID
	}
	verificationBase := "/rekit note -Kind verification -Lane <lane> -Verifier manual-review -Verdict <accepted|rejected|inconclusive|needs-more-evidence> -TargetRef \"" + itemRef + "\" -Subject \"reviewer verdict for " + shard.ID + "\" -Summary \"<short reviewer verdict summary>\" -EvidenceRefs \"<packet-or-evidence-ref>\" -Actor <main-agent>"
	decisionBase := "/rekit note -Kind decision -Lane <lane> -Decision <accept|reject|defer|supersede> -TargetRef \"" + itemRef + "\" -Subject \"main merge decision for " + shard.ID + "\" -Summary \"<merge decision and reason>\" -EvidenceRefs \"<verification-event-or-packet-ref>\" -Actor <main-agent>"
	return []LedgerWritebackTemplate{
		{
			Kind:           "verification",
			Purpose:        "record a read-only reviewer verdict for this shard after the main agent inspects the reviewer output",
			Command:        verificationBase + " -Apply",
			PreviewCommand: verificationBase + " -WhatIf -Format json",
			ApplyCommand:   verificationBase + " -Apply",
			RequiredFields: []string{"lane", "verifier", "verdict", "target", "subject", "summary", "evidenceRefs", "actor"},
			AllowedValues:  []string{"verifier=manual-review|schema-check|focused-trace|parity|cross-run|tool-review", "verdict=accepted|rejected|inconclusive|needs-more-evidence"},
			PreviewChecks:  writebackPreviewChecks("verification"),
			BlockedOutputs: writebackBlockedOutputs(),
		},
		{
			Kind:           "decision",
			Purpose:        "record the main agent merge decision for this shard after validation and conflict review",
			Command:        decisionBase + " -Apply",
			PreviewCommand: decisionBase + " -WhatIf -Format json",
			ApplyCommand:   decisionBase + " -Apply",
			RequiredFields: []string{"lane", "decision", "target", "subject", "summary", "evidenceRefs", "actor"},
			AllowedValues:  []string{"decision=accept|reject|defer|supersede"},
			PreviewChecks:  writebackPreviewChecks("decision"),
			BlockedOutputs: writebackBlockedOutputs(),
		},
	}
}

func writebackPreviewChecks(kind string) []string {
	return []string{
		"run previewCommand before applyCommand",
		"confirm note WhatIf returns isMutation=false and applied=false",
		"confirm event.kind is " + kind + " and event lane/target/evidenceRefs match the reviewed shard",
		"confirm wouldExecutorAction contains only the expected lane-local blocker delta before applying",
	}
}

func writebackBlockedOutputs() []string {
	return []string{
		"reviewer output alone must not be treated as a ledger event",
		"previewCommand must not write facts, authority, confirmed, board, lane, handoff, or source files",
		"applyCommand must not run when previewCommand fails, targets the wrong lane, or lacks inspected evidenceRefs",
	}
}

func postReviewMergeSteps() []string {
	return []string{
		"inspect reviewer output against expectedOutput before ledger writeback",
		"run each template previewCommand and inspect note WhatIf output before applyCommand",
		"record reviewer verdict with the verification applyCommand; do not let the reviewer append ledger events directly",
		"record the main merge decision with the decision applyCommand only after validation/conflict review",
		"run the relevant overview/handoff/doctor check after accepted decisions that affect lane state",
	}
}

func shardDispatchPrompt(shard Shard, route Route, readOnlyBoundary []string, reviewLoop ReviewLoop) string {
	contract := reviewerResultContract()
	lines := []string{
		"You are a read-only reviewer for rekit plan-subagents shard " + shard.ID + ".",
		"Route: " + route.ID + ".",
		"Items: " + strings.Join(shard.Items, ", ") + ".",
		"Return only this output contract: " + route.OutputContract + ".",
		"Reviewer result contract: " + contract.OutputFormat + ".",
		"Required result fields: " + strings.Join(contract.RequiredFields, ", ") + ".",
		"Allowed decisions: " + strings.Join(contract.AllowedDecisions, ", ") + ".",
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
			contract := handoff.ReviewerResultContract
			lines = append(lines, fmt.Sprintf("  - reviewer result contract: output=`%s`; required=`%s`; allowed decisions=`%s`", contract.OutputFormat, strings.Join(contract.RequiredFields, ","), strings.Join(contract.AllowedDecisions, ",")))
			for _, rule := range contract.EvidenceRules {
				lines = append(lines, "    - evidence-rule: "+rule)
			}
			for _, signal := range contract.ConflictSignals {
				lines = append(lines, "    - conflict-signal: "+signal)
			}
			for _, item := range handoff.IntakeChecklist {
				lines = append(lines, "    - intake-check: "+item)
			}
			for _, mapping := range handoff.ReviewerDecisionMappings {
				lines = append(lines, fmt.Sprintf("    - decision-map: reviewer=`%s` -> verification=`%s`, main=`%s`; when=`%s`; fallback=`%s`", mapping.ReviewerDecision, mapping.VerificationVerdict, mapping.MainDecision, strings.Join(mapping.ApplyWhen, "; "), mapping.Fallback))
			}
			for _, step := range handoff.ConflictHandling {
				lines = append(lines, "    - conflict-handling: "+step)
			}
			for _, step := range handoff.WritebackSequence {
				lines = append(lines, fmt.Sprintf("    - writeback-step: `%s`; owner=`%s`; uses=`%s`; must-pass=`%s`; next-success=`%s`; next-failure=`%s`", step.Step, step.Owner, strings.Join(step.Uses, ","), strings.Join(step.MustPass, "; "), step.NextOnSuccess, step.NextOnFailure))
				for _, binding := range step.CommandBindings {
					lines = append(lines, fmt.Sprintf("      - command-binding: `%s`; source=`%s`; kind=`%s`; command=`%s`; required=`%s`; expected=`%s`", binding.Binding, binding.Source, binding.Kind, binding.Command, strings.Join(binding.RequiredFields, ","), binding.ExpectedOutput))
				}
				for _, blocked := range step.BlockedBy {
					lines = append(lines, "      - writeback-blocker: "+blocked)
				}
			}
			for _, tmpl := range handoff.LedgerWritebackTemplates {
				lines = append(lines, fmt.Sprintf("  - %s writeback preview: `%s`; apply: `%s`; required=`%s`", tmpl.Kind, tmpl.PreviewCommand, tmpl.ApplyCommand, strings.Join(tmpl.RequiredFields, ",")))
				for _, check := range tmpl.PreviewChecks {
					lines = append(lines, "    - preview-check: "+check)
				}
				for _, blocked := range tmpl.BlockedOutputs {
					lines = append(lines, "    - blocked-output: "+blocked)
				}
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
