package workstream

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
)

const continuePreviewRunID = "run-preview"

type ContinueOptions struct {
	Selector string
}

type ContinueResult struct {
	SchemaVersion        int                    `json:"schemaVersion"`
	Command              string                 `json:"command"`
	CaseRoot             string                 `json:"caseRoot"`
	RepoRoot             string                 `json:"repoRoot"`
	Pack                 string                 `json:"pack"`
	IsMutation           bool                   `json:"isMutation"`
	Applied              bool                   `json:"applied"`
	RequiresConfirmation bool                   `json:"requiresConfirmation"`
	Selector             string                 `json:"selector"`
	Lane                 Lane                   `json:"lane"`
	RunID                string                 `json:"runId"`
	BatchID              string                 `json:"batchId"`
	Summary              ContinueSummary        `json:"summary"`
	Inputs               []string               `json:"inputs"`
	PacketRefs           []string               `json:"packetRefs"`
	Events               []ContinueEventPreview `json:"events"`
	OpenRisks            []string               `json:"openRisks"`
	WouldWrites          []StartWrite           `json:"wouldWrites"`
	BlockedActions       []string               `json:"blockedActions"`
	NextSteps            []string               `json:"nextSteps"`
}

type ContinueSummary struct {
	Collected            int `json:"collected"`
	Observations         int `json:"observations"`
	Requests             int `json:"requests"`
	Routed               int `json:"routed"`
	Candidates           int `json:"candidates"`
	AcceptedCandidates   int `json:"acceptedCandidates"`
	Publications         int `json:"publications"`
	AuthorityApplied     int `json:"authorityApplied"`
	AuthorityWouldAppend int `json:"authorityWouldAppend"`
	PendingUser          int `json:"pendingUser"`
	Skipped              int `json:"skipped"`
}

type ContinueEventPreview struct {
	EventID       string         `json:"eventId"`
	Kind          string         `json:"kind"`
	Lane          string         `json:"lane"`
	Subject       string         `json:"subject,omitempty"`
	Summary       string         `json:"summary,omitempty"`
	Decision      string         `json:"decision"`
	Reason        string         `json:"reason"`
	TargetLane    string         `json:"targetLane,omitempty"`
	AuthorityFile string         `json:"authorityFile,omitempty"`
	Rows          int            `json:"rows,omitempty"`
	Verification  map[string]any `json:"verification,omitempty"`
	WouldWrites   []StartWrite   `json:"wouldWrites,omitempty"`
}

type continueContext struct {
	inst     instance.Instance
	manifest *manifest.Manifest
	board    board
	policy   continuePolicy
	selector string
	lane     Lane
}

type continuePolicy struct {
	AutoVerify                  bool
	AutoRouteRequests           bool
	AutoPublishSharedFacts      bool
	AutoAcceptLowRiskCandidates bool
	AuthorityAutoAppend         string
	RequireEvidence             bool
	RequireVerifier             bool
	MinConfidence               float64
	RequireNoConflict           bool
	RequireSchemaValid          bool
	RequireBackup               bool
	RequireDiff                 bool
	MaxAuthorityRowsPerRun      int
}

func ContinuePreview(repoRoot, caseRoot, pack string, opt ContinueOptions) (ContinueResult, error) {
	ctx, err := newContinueContext(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return ContinueResult{}, err
	}
	known, err := knownEventIDs(ctx.inst.CaseRoot)
	if err != nil {
		return ContinueResult{}, err
	}
	inputs, err := continueInputRefs(ctx.inst.CaseRoot, ctx.lane)
	if err != nil {
		return ContinueResult{}, err
	}
	packets, err := continuePacketRefs(ctx.inst.CaseRoot, ctx.lane)
	if err != nil {
		return ContinueResult{}, err
	}
	rawEvents, err := laneOutputEvents(ctx.inst.CaseRoot, ctx.lane, ctx.manifest)
	if err != nil {
		return ContinueResult{}, err
	}
	result := ContinueResult{
		SchemaVersion:        1,
		Command:              "continue",
		CaseRoot:             ctx.inst.CaseRoot,
		RepoRoot:             ctx.manifest.RepoRoot,
		Pack:                 ctx.manifest.Pack,
		IsMutation:           false,
		Applied:              false,
		RequiresConfirmation: true,
		Selector:             ctx.selector,
		Lane:                 ctx.lane,
		RunID:                continuePreviewRunID,
		BatchID:              "batch-" + continuePreviewRunID,
		Inputs:               uniqueStrings(inputs),
		PacketRefs:           uniqueStrings(packets),
		BlockedActions:       []string{"run directory creation", "facts JSONL writes", "lane resume/checkpoint refresh", "board refresh", "authority/confirmed writes", "heavy-tool execution"},
		NextSteps: []string{
			"review this preview against PowerShell continue digest/status behavior",
			"PowerShell /rekit remains the public entrypoint; JSON preview is Go-owned by default and continue currently supports -WhatIf only",
		},
	}
	for _, raw := range rawEvents {
		event := copyEvent(raw)
		event["lane"] = ctx.lane.ID
		id := strings.TrimSpace(stringFrom(event, "eventId"))
		if id == "" {
			id = generatedEventID(ctx.lane.ID, event)
			event["eventId"] = id
		}
		if known[id] {
			result.Summary.Skipped++
			continue
		}
		preview := ctx.previewEvent(event)
		result.Summary.Collected++
		result.Events = append(result.Events, preview)
		result.WouldWrites = append(result.WouldWrites, preview.WouldWrites...)
		if preview.Decision == "defer" || preview.Decision == "pending-user" {
			result.OpenRisks = append(result.OpenRisks, riskLine(preview))
		}
		switch preview.Kind {
		case "observation":
			if preview.Decision == "accept" {
				result.Summary.Observations++
			}
		case "request":
			result.Summary.Requests++
			if preview.TargetLane != "" && preview.Decision == "accept" {
				result.Summary.Routed++
			}
		case "candidate":
			result.Summary.Candidates++
			if preview.Decision == "accept" {
				if preview.AuthorityFile != "" {
					result.Summary.AuthorityWouldAppend += preview.Rows
				} else {
					result.Summary.AcceptedCandidates++
				}
			} else {
				result.Summary.PendingUser++
			}
		case "publication":
			if preview.Decision == "accept" {
				result.Summary.Publications++
			}
		}
	}
	return result, nil
}

func newContinueContext(repoRoot, caseRoot, pack string, opt ContinueOptions) (continueContext, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return continueContext{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return continueContext{}, err
	}
	b, err := readBoard(inst.CaseRoot)
	if os.IsNotExist(err) {
		return continueContext{}, fmt.Errorf("continue -WhatIf requires existing .rekit/board.json; run PowerShell /rekit overview once to initialize the case-local board")
	}
	if err != nil {
		return continueContext{}, err
	}
	if strings.TrimSpace(b.DefaultAuthorityLane) == "" {
		b.DefaultAuthorityLane = m.WorkstreamDefaults["defaultAuthorityLane"]
	}
	selector := strings.TrimSpace(opt.Selector)
	if selector == "" {
		open := openBoardLanes(b)
		if len(open) != 1 {
			return continueContext{}, fmt.Errorf("continue requires a lane selector when multiple open lanes exist; use main or a workstream name")
		}
		lane, err := readLaneByID(inst.CaseRoot, open[0].ID)
		if err != nil {
			return continueContext{}, err
		}
		selector = workstreamLabel(lane)
	}
	lane, err := resolveHandoffLane(inst.CaseRoot, b, selector)
	if err != nil {
		return continueContext{}, err
	}
	status := strings.ToLower(strings.TrimSpace(lane.Status))
	if status == "archived" || status == "paused" || status == "closed" {
		return continueContext{}, fmt.Errorf("target lane is not open: %s", lane.ID)
	}
	policy, err := readContinuePolicy(inst.CaseRoot)
	if err != nil {
		return continueContext{}, err
	}
	return continueContext{inst: inst, manifest: m, board: b, policy: policy, selector: selector, lane: lane}, nil
}

func (ctx continueContext) previewEvent(event map[string]any) ContinueEventPreview {
	kind := strings.ToLower(strings.TrimSpace(stringFrom(event, "kind")))
	if kind == "" {
		kind = "observation"
	}
	preview := ContinueEventPreview{
		EventID: stringFrom(event, "eventId"),
		Kind:    kind,
		Lane:    ctx.lane.ID,
		Subject: stringFrom(event, "subject"),
		Summary: stringFrom(event, "summary"),
	}
	switch kind {
	case "observation":
		if ctx.policy.AutoPublishSharedFacts {
			preview.Decision = "accept"
			preview.Reason = "shared observation"
			preview.WouldWrites = []StartWrite{wouldFact(".rekit/facts/observations.jsonl"), wouldFact(".rekit/facts/decisions.jsonl")}
		} else {
			preview.Decision = "defer"
			preview.Reason = "autoPublishSharedFacts disabled"
		}
	case "request":
		preview.Decision = "accept"
		preview.Reason = "would route request"
		preview.WouldWrites = []StartWrite{wouldFact(".rekit/facts/requests.jsonl"), wouldFact(".rekit/facts/decisions.jsonl")}
		if ctx.policy.AutoRouteRequests {
			targetLane := stringFrom(event, "targetLane")
			if targetLane == "" {
				targetLane = ctx.manifest.WorkstreamDefaults["requestDefaultTargetLane"]
			}
			if err := canRouteRequest(ctx.inst.CaseRoot, targetLane); err != nil {
				preview.Decision = "defer"
				preview.Reason = err.Error()
			} else if !requestAlreadyRouted(ctx.inst.CaseRoot, targetLane, event) {
				preview.TargetLane = targetLane
				preview.WouldWrites = append(preview.WouldWrites, wouldLane(targetLane, "tasks.jsonl"), wouldLane(targetLane, "inbox.jsonl"))
			}
		} else {
			preview.Decision = "defer"
			preview.Reason = "autoRouteRequests disabled"
		}
	case "candidate":
		verification := verifyCandidate(ctx.inst.CaseRoot, ctx.policy, event)
		preview.Verification = verification
		authorityFile := candidateAuthorityFile(event)
		preview.AuthorityFile = authorityFile
		if authorityFile != "" {
			rows := candidateRows(event)
			preview.Rows = len(rows)
			if reason := ctx.authorityAppendReason(event, verification, authorityFile, rows); reason != "" {
				preview.Decision = "defer"
				preview.Reason = reason
				preview.WouldWrites = []StartWrite{wouldFact(".rekit/facts/candidates.jsonl"), wouldFact(".rekit/facts/decisions.jsonl")}
			} else {
				preview.Decision = "accept"
				preview.Reason = "passed authority append policy"
				preview.WouldWrites = []StartWrite{wouldAuthority(authorityFile), wouldRunArtifact("backups", authorityFile), wouldRunArtifact("diffs", sanitizedDiffName(authorityFile)), wouldFact(".rekit/facts/publications.jsonl"), wouldFact(".rekit/facts/decisions.jsonl")}
			}
		} else if ctx.policy.AutoAcceptLowRiskCandidates && boolFrom(verification, "hasEvidence") && verifierAccepted(ctx.policy, verification) {
			preview.Decision = "accept"
			preview.Reason = "candidate has evidence, verifier accepted, and does not touch authority"
			preview.WouldWrites = []StartWrite{wouldFact(".rekit/facts/candidates.jsonl"), wouldFact(".rekit/facts/decisions.jsonl")}
		} else {
			preview.Decision = "defer"
			preview.Reason = "candidate lacks evidence or policy disabled"
			preview.WouldWrites = []StartWrite{wouldFact(".rekit/facts/candidates.jsonl"), wouldFact(".rekit/facts/decisions.jsonl")}
		}
	case "publication":
		preview.Decision = "accept"
		preview.Reason = "publication event"
		preview.WouldWrites = []StartWrite{wouldFact(".rekit/facts/publications.jsonl"), wouldFact(".rekit/facts/decisions.jsonl")}
	default:
		preview.Decision = "accept"
		preview.Reason = "unknown kind treated as observation: " + kind
		preview.WouldWrites = []StartWrite{wouldFact(".rekit/facts/observations.jsonl"), wouldFact(".rekit/facts/decisions.jsonl")}
	}
	return preview
}

func (ctx continueContext) authorityAppendReason(event map[string]any, verification map[string]any, authorityFile string, rows []any) string {
	if !containsString(ctx.manifest.AuthorityFiles, authorityFile) {
		return "authority file is not allowed: " + authorityFile
	}
	if strings.EqualFold(strings.TrimSpace(ctx.policy.AuthorityAutoAppend), "never") {
		return "authority auto append disabled"
	}
	confidence := eventConfidence(event)
	if confidence < ctx.policy.MinConfidence {
		return fmt.Sprintf("confidence below threshold: %s < %s", formatFloat(confidence), formatFloat(ctx.policy.MinConfidence))
	}
	if ctx.policy.RequireEvidence && !eventHasEvidence(ctx.inst.CaseRoot, event) {
		return "missing evidence"
	}
	if !verifierAccepted(ctx.policy, verification) {
		return "missing accepted verifier verdict"
	}
	path, err := refsf.SafeJoin(ctx.inst.CaseRoot, authorityFile)
	if err != nil {
		return err.Error()
	}
	if !refsf.Exists(path) {
		return "missing authority file: " + authorityFile
	}
	if !strings.HasSuffix(strings.ToLower(authorityFile), ".csv") {
		return "only csv authority append is automated"
	}
	if ctx.policy.RequireSchemaValid && !candidateCSVSchemaValid(path, rows) {
		return "candidate row does not match authority csv schema"
	}
	if ctx.policy.RequireNoConflict && candidateCSVConflict(path, rows) {
		return "authority key conflict"
	}
	if len(rows) == 0 {
		return "no candidate rows"
	}
	for _, row := range rows {
		if text, ok := row.(string); ok && strings.ContainsAny(text, "\r\n") {
			return "candidate row contains newline"
		}
	}
	if len(rows) > ctx.policy.MaxAuthorityRowsPerRun {
		return fmt.Sprintf("too many rows: %d > %d", len(rows), ctx.policy.MaxAuthorityRowsPerRun)
	}
	return ""
}

func laneOutputEvents(caseRoot string, lane Lane, m *manifest.Manifest) ([]map[string]any, error) {
	laneRoot, err := laneRootPath(caseRoot, lane)
	if err != nil {
		return nil, err
	}
	workspace, err := refsf.SafeJoin(caseRoot, lane.Workspace)
	if err != nil {
		return nil, err
	}
	var events []map[string]any
	for _, path := range []string{filepath.Join(laneRoot, "outbox.jsonl"), filepath.Join(workspace, "observations.jsonl"), filepath.Join(workspace, "requests.jsonl"), filepath.Join(workspace, "candidates.jsonl"), filepath.Join(workspace, "publications.jsonl")} {
		items, err := readJSONLineObjects(path)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			events = append(events, ensurePreviewEventID(lane.ID, item))
		}
	}
	lowering := filepath.Join(workspace, "lowering_requests.csv")
	if refsf.Exists(lowering) {
		rows, err := readCSVRows(lowering)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			status := strings.ToLower(strings.TrimSpace(row["status"]))
			if terminalStatus(status) {
				continue
			}
			summary := row["reason"]
			if strings.TrimSpace(summary) == "" {
				summary = "lowering request"
			}
			event := map[string]any{"kind": "request", "lane": lane.ID, "targetLane": m.WorkstreamDefaults["requestDefaultTargetLane"], "requestId": row["request_id"], "summary": summary, "evidence": row["evidence"], "priority": row["priority"], "status": "open", "source": relativePath(caseRoot, lowering)}
			if strings.TrimSpace(row["request_id"]) != "" {
				event["eventId"] = "evt-" + hashText(lane.ID + "|request|" + row["request_id"])[:16]
			}
			events = append(events, ensurePreviewEventID(lane.ID, event))
		}
	}
	candidatesDir := filepath.Join(workspace, "candidates")
	entries, err := os.ReadDir(candidatesDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".csv") {
				continue
			}
			path := filepath.Join(candidatesDir, entry.Name())
			rows, err := readCSVRows(path)
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				status := strings.ToLower(strings.TrimSpace(row["status"]))
				if terminalStatus(status) {
					continue
				}
				event := map[string]any{"kind": "candidate", "lane": lane.ID, "target": strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())), "summary": "candidate from " + entry.Name(), "evidence": row["evidence"], "confidence": row["confidence"], "status": "open", "source": relativePath(caseRoot, path), "row": row}
				events = append(events, ensurePreviewEventID(lane.ID, event))
			}
		}
	}
	return events, nil
}

func continueInputRefs(caseRoot string, lane Lane) ([]string, error) {
	laneRoot, err := laneRootPath(caseRoot, lane)
	if err != nil {
		return nil, err
	}
	workspace, err := refsf.SafeJoin(caseRoot, lane.Workspace)
	if err != nil {
		return nil, err
	}
	refs := []string{}
	for _, path := range []string{filepath.Join(laneRoot, "outbox.jsonl"), filepath.Join(workspace, "observations.jsonl"), filepath.Join(workspace, "requests.jsonl"), filepath.Join(workspace, "candidates.jsonl"), filepath.Join(workspace, "publications.jsonl")} {
		if refsf.Exists(path) {
			refs = append(refs, relativePath(caseRoot, path))
		}
	}
	return refs, nil
}

func continuePacketRefs(caseRoot string, lane Lane) ([]string, error) {
	workspace, err := refsf.SafeJoin(caseRoot, lane.Workspace)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(workspace)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	packets := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			packets = append(packets, relativePath(caseRoot, filepath.Join(workspace, entry.Name())))
		}
	}
	sort.Strings(packets)
	if len(packets) > 10 {
		packets = packets[len(packets)-10:]
	}
	return packets, nil
}

func readContinuePolicy(caseRoot string) (continuePolicy, error) {
	policy := defaultContinuePolicy()
	path, err := refsf.SafeJoin(caseRoot, ".rekit/policy.yml")
	if err != nil {
		return policy, err
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return policy, nil
	}
	if err != nil {
		return policy, err
	}
	values := map[string]string{}
	for line := range strings.SplitSeq(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		trim := strings.TrimSpace(line)
		key, value, ok := strings.Cut(trim, ":")
		if trim == "" || strings.HasPrefix(trim, "#") || !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	policy.AutoVerify = boolPolicy(values, "autoVerify", policy.AutoVerify)
	policy.AutoRouteRequests = boolPolicy(values, "autoRouteRequests", policy.AutoRouteRequests)
	policy.AutoPublishSharedFacts = boolPolicy(values, "autoPublishSharedFacts", policy.AutoPublishSharedFacts)
	policy.AutoAcceptLowRiskCandidates = boolPolicy(values, "autoAcceptLowRiskCandidates", policy.AutoAcceptLowRiskCandidates)
	if value := strings.TrimSpace(values["authorityAutoAppend"]); value != "" {
		policy.AuthorityAutoAppend = value
	}
	policy.RequireEvidence = boolPolicy(values, "requireEvidence", policy.RequireEvidence)
	policy.RequireVerifier = boolPolicy(values, "requireVerifier", policy.RequireVerifier)
	policy.RequireNoConflict = boolPolicy(values, "requireNoConflict", policy.RequireNoConflict)
	policy.RequireSchemaValid = boolPolicy(values, "requireSchemaValid", policy.RequireSchemaValid)
	policy.RequireBackup = boolPolicy(values, "requireBackup", policy.RequireBackup)
	policy.RequireDiff = boolPolicy(values, "requireDiff", policy.RequireDiff)
	if n, err := strconv.ParseFloat(strings.TrimSpace(values["minConfidence"]), 64); err == nil {
		policy.MinConfidence = n
	}
	if n, err := strconv.Atoi(strings.TrimSpace(values["maxAuthorityRowsPerRun"])); err == nil {
		policy.MaxAuthorityRowsPerRun = n
	}
	return policy, nil
}

func defaultContinuePolicy() continuePolicy {
	return continuePolicy{AutoVerify: true, AutoRouteRequests: true, AutoPublishSharedFacts: true, AutoAcceptLowRiskCandidates: true, AuthorityAutoAppend: "conditional", RequireEvidence: true, RequireVerifier: true, MinConfidence: 0.90, RequireNoConflict: true, RequireSchemaValid: true, RequireBackup: true, RequireDiff: true, MaxAuthorityRowsPerRun: 10}
}

func boolPolicy(values map[string]string, key string, def bool) bool {
	value := strings.ToLower(strings.TrimSpace(values[key]))
	switch value {
	case "true", "yes", "1":
		return true
	case "false", "no", "0":
		return false
	default:
		return def
	}
}

func verifyCandidate(caseRoot string, policy continuePolicy, event map[string]any) map[string]any {
	confidence := eventConfidence(event)
	hasEvidence := eventHasEvidence(caseRoot, event)
	authorityFile := candidateAuthorityFile(event)
	schemaValid := true
	conflict := false
	if authorityFile != "" {
		if path, err := refsf.SafeJoin(caseRoot, authorityFile); err == nil {
			rows := candidateRows(event)
			schemaValid = candidateCSVSchemaValid(path, rows)
			conflict = candidateCSVConflict(path, rows)
		} else {
			schemaValid = false
			conflict = true
		}
	}
	reasons := []string{}
	if policy.RequireEvidence && !hasEvidence {
		reasons = append(reasons, "missing evidence")
	}
	if confidence < policy.MinConfidence {
		reasons = append(reasons, fmt.Sprintf("confidence below threshold: %s < %s", formatFloat(confidence), formatFloat(policy.MinConfidence)))
	}
	if policy.RequireSchemaValid && !schemaValid {
		reasons = append(reasons, "schema invalid")
	}
	if conflict {
		reasons = append(reasons, "authority conflict")
	}
	verdict := "accepted"
	if len(reasons) > 0 {
		verdict = "rejected"
	}
	verifier := "rule-verifier"
	if !policy.AutoVerify {
		verifier = "policy-disabled"
		verdict = "skipped"
		reasons = []string{"autoVerify disabled"}
	}
	return map[string]any{"verifier": verifier, "verdict": verdict, "confidence": confidence, "hasEvidence": hasEvidence, "schemaValid": schemaValid, "conflict": conflict, "reasons": reasons}
}

func verifierAccepted(policy continuePolicy, verification map[string]any) bool {
	if !policy.RequireVerifier {
		return true
	}
	return strings.TrimSpace(stringFrom(verification, "verifier")) != "" && strings.EqualFold(stringFrom(verification, "verdict"), "accepted")
}

func eventConfidence(event map[string]any) float64 {
	text := strings.ToLower(strings.TrimSpace(stringFrom(event, "confidence")))
	if text == "" {
		return 0
	}
	if n, err := strconv.ParseFloat(text, 64); err == nil {
		return n
	}
	switch text {
	case "high":
		return 0.95
	case "medium_high", "medium-high":
		return 0.82
	case "medium":
		return 0.65
	case "medium_low", "medium-low":
		return 0.45
	case "low":
		return 0.25
	default:
		return 0
	}
}

func eventHasEvidence(caseRoot string, event map[string]any) bool {
	for _, item := range evidenceItems(event) {
		if strings.ContainsAny(item, `/\`) || filepath.IsAbs(item) {
			path := item
			if !filepath.IsAbs(path) {
				joined, err := refsf.SafeJoin(caseRoot, filepath.ToSlash(path))
				if err != nil {
					continue
				}
				path = joined
			}
			if refsf.Exists(path) {
				return true
			}
		} else if len(item) >= 8 {
			return true
		}
	}
	return false
}

func evidenceItems(event map[string]any) []string {
	value, ok := event["evidence"]
	if !ok || value == nil {
		return nil
	}
	if list, ok := value.([]any); ok {
		items := []string{}
		for _, item := range list {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" {
				items = append(items, text)
			}
		}
		return items
	}
	return splitScalarList(fmt.Sprint(value))
}

func candidateAuthorityFile(event map[string]any) string {
	for _, key := range []string{"authorityFile", "authorityCsv", "targetFile", "file"} {
		if value := strings.TrimSpace(stringFrom(event, key)); value != "" {
			return filepath.ToSlash(value)
		}
	}
	return ""
}

func candidateRows(event map[string]any) []any {
	for _, key := range []string{"rows", "row", "csvRow"} {
		value, ok := event[key]
		if !ok || value == nil {
			continue
		}
		if list, ok := value.([]any); ok {
			return list
		}
		return []any{value}
	}
	return nil
}

func candidateCSVSchemaValid(csvPath string, rows []any) bool {
	if !refsf.Exists(csvPath) || len(rows) == 0 {
		return false
	}
	header, err := csvHeader(csvPath)
	if err != nil || len(header) == 0 || strings.TrimSpace(header[0]) == "" {
		return false
	}
	for _, row := range rows {
		switch t := row.(type) {
		case string:
			if strings.TrimSpace(t) == "" || strings.ContainsAny(t, "\r\n") {
				return false
			}
			records, err := csv.NewReader(strings.NewReader(strings.Join(header, ",") + "\n" + t + "\n")).ReadAll()
			if err != nil || len(records) != 2 {
				return false
			}
		case map[string]any:
			for _, column := range header {
				if _, ok := t[strings.Trim(column, `"`)]; !ok {
					return false
				}
			}
		default:
			m, ok := structToMap(t)
			if !ok {
				return false
			}
			for _, column := range header {
				if _, ok := m[strings.Trim(column, `"`)]; !ok {
					return false
				}
			}
		}
	}
	return true
}

func candidateCSVConflict(csvPath string, rows []any) bool {
	if !refsf.Exists(csvPath) || len(rows) == 0 {
		return false
	}
	header, err := csvHeader(csvPath)
	if err != nil || len(header) == 0 {
		return false
	}
	key := candidateRowKey(rows[0], strings.Trim(header[0], `"`))
	if key == "" {
		return false
	}
	file, err := os.Open(csvPath)
	if err != nil {
		return false
	}
	defer file.Close()
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil || len(records) <= 1 {
		return false
	}
	for _, record := range records[1:] {
		if len(record) > 0 && record[0] == key {
			return true
		}
	}
	return false
}

func csvHeader(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	first, _, _ := strings.Cut(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	parts := strings.Split(first, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(strings.Trim(parts[i], `"`))
	}
	return parts, nil
}

func candidateRowKey(row any, firstColumn string) string {
	switch t := row.(type) {
	case string:
		key, _, _ := strings.Cut(t, ",")
		return strings.Trim(key, `"`)
	case map[string]any:
		return strings.TrimSpace(fmt.Sprint(t[firstColumn]))
	default:
		m, ok := structToMap(t)
		if !ok {
			return ""
		}
		return strings.TrimSpace(fmt.Sprint(m[firstColumn]))
	}
}

func canRouteRequest(caseRoot, laneID string) error {
	lane, err := readLaneByID(caseRoot, laneID)
	if err != nil {
		return fmt.Errorf("target lane does not exist: %s", laneID)
	}
	status := strings.ToLower(strings.TrimSpace(lane.Status))
	if status == "archived" || status == "paused" {
		return fmt.Errorf("target lane is not open: %s", laneID)
	}
	return nil
}

func requestAlreadyRouted(caseRoot, laneID string, event map[string]any) bool {
	lane, err := readLaneByID(caseRoot, laneID)
	if err != nil {
		return false
	}
	laneRoot, err := laneRootPath(caseRoot, lane)
	if err != nil {
		return false
	}
	tasks, err := readJSONLineObjects(filepath.Join(laneRoot, "tasks.jsonl"))
	if err != nil {
		return false
	}
	sourceLane := stringFrom(event, "lane")
	requestID := stringFrom(event, "requestId")
	eventID := stringFrom(event, "eventId")
	for _, task := range tasks {
		if requestID != "" {
			if stringFrom(task, "requestId") == requestID && stringFrom(task, "sourceLane") == sourceLane {
				return true
			}
		} else if eventID != "" && stringFrom(task, "eventId") == eventID {
			return true
		}
	}
	return false
}

func knownEventIDs(caseRoot string) (map[string]bool, error) {
	factsRoot, err := refsf.SafeJoin(caseRoot, ".rekit/facts")
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, file := range []string{"observations.jsonl", "candidates.jsonl", "requests.jsonl", "publications.jsonl", "decisions.jsonl", "hypotheses.jsonl", "verifications.jsonl", "interventions.jsonl", "rollbacks.jsonl"} {
		items, err := readJSONLineObjects(filepath.Join(factsRoot, file))
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if id := stringFrom(item, "eventId"); id != "" {
				known[id] = true
			}
		}
	}
	return known, nil
}

func openBoardLanes(b board) []boardLane {
	out := []boardLane{}
	for _, lane := range b.Lanes {
		status := strings.ToLower(strings.TrimSpace(lane.Status))
		if status != "archived" && status != "paused" && status != "closed" {
			out = append(out, lane)
		}
	}
	return out
}

func readCSVRows(path string) ([]map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	header := records[0]
	rows := []map[string]string{}
	for _, record := range records[1:] {
		row := map[string]string{}
		for i, key := range header {
			if i < len(record) {
				row[key] = record[i]
			} else {
				row[key] = ""
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func terminalStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "resolved", "done", "closed", "accepted", "rejected":
		return true
	default:
		return false
	}
}

func ensurePreviewEventID(laneID string, event map[string]any) map[string]any {
	out := copyEvent(event)
	if strings.TrimSpace(stringFrom(out, "eventId")) == "" {
		out["eventId"] = generatedEventID(laneID, out)
	}
	return out
}

func generatedEventID(laneID string, event map[string]any) string {
	b, _ := json.Marshal(event)
	return "evt-" + hashText(laneID + "|" + string(b))[:16]
}

func hashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func copyEvent(event map[string]any) map[string]any {
	out := map[string]any{}
	maps.Copy(out, event)
	return out
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if strings.TrimSpace(value) == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func splitScalarList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' })
	out := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func stringFrom(m map[string]any, key string) string {
	return objectText(m[key])
}

func boolFrom(m map[string]any, key string) bool {
	v, ok := m[key].(bool)
	return ok && v
}

func structToMap(value any) (map[string]any, bool) {
	b, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, false
	}
	return out, true
}

func containsString(values []string, target string) bool {
	return slices.Contains(values, target)
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func sanitizedDiffName(rel string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(rel) + ".diff"
}

func wouldFact(rel string) StartWrite {
	return StartWrite{Path: rel, Kind: "fact-jsonl", Action: "would-append"}
}

func wouldLane(laneID, file string) StartWrite {
	return StartWrite{Path: relJoin(".rekit", "lanes", laneID, file), Kind: "lane-jsonl", Action: "would-append"}
}

func wouldAuthority(rel string) StartWrite {
	return StartWrite{Path: rel, Kind: "authority-csv", Action: "would-append"}
}

func wouldRunArtifact(kind, rel string) StartWrite {
	return StartWrite{Path: relJoin(".rekit", "runs", continuePreviewRunID, kind, rel), Kind: "run-artifact", Action: "would-create"}
}

func riskLine(preview ContinueEventPreview) string {
	subject := preview.Subject
	if subject == "" {
		subject = preview.Summary
	}
	if subject == "" {
		subject = preview.EventID
	}
	return fmt.Sprintf("%s: %s | lane=%s | reason=%s", preview.Kind, subject, preview.Lane, preview.Reason)
}
