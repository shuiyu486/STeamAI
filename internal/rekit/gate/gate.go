package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

type Options struct {
	Action          string
	Lane            string
	Subject         string
	Summary         string
	Actor           string
	Risk            string
	TargetRef       string
	BatchID         string
	Scope           string
	Budget          string
	RuntimeSeconds  int
	DiskMB          int
	Requests        int
	OutputPaths     string
	TriedLightSteps string
	StopConditions  string
}

type Plan struct {
	SchemaVersion        int                    `json:"schemaVersion"`
	Command              string                 `json:"command"`
	CaseRoot             string                 `json:"caseRoot"`
	RepoRoot             string                 `json:"repoRoot"`
	Pack                 string                 `json:"pack"`
	IsMutation           bool                   `json:"isMutation"`
	ReviewRequired       bool                   `json:"reviewRequired"`
	RequiresConfirmation bool                   `json:"requiresConfirmation"`
	EventPreview         EventPreview           `json:"eventPreview"`
	MissionBrief         mission.Brief          `json:"missionBrief"`
	ExecutorAction       mission.ExecutorAction `json:"executorAction"`
	WouldExecutorAction  mission.ExecutorAction `json:"wouldExecutorAction"`
	BlockedActions       []string               `json:"blockedActions"`
	NextSteps            []string               `json:"nextSteps"`
}

type ApplyResult struct {
	SchemaVersion  int                    `json:"schemaVersion"`
	Command        string                 `json:"command"`
	CaseRoot       string                 `json:"caseRoot"`
	RepoRoot       string                 `json:"repoRoot"`
	Pack           string                 `json:"pack"`
	IsMutation     bool                   `json:"isMutation"`
	Applied        bool                   `json:"applied"`
	EventID        string                 `json:"eventId"`
	Path           string                 `json:"path"`
	Reason         string                 `json:"reason,omitempty"`
	Event          EventPreview           `json:"event"`
	MissionBrief   mission.Brief          `json:"missionBrief"`
	ExecutorAction mission.ExecutorAction `json:"executorAction"`
	NextSteps      []string               `json:"nextSteps"`
}

type EventPreview struct {
	SchemaVersion int         `json:"schemaVersion"`
	Kind          string      `json:"kind"`
	Lane          string      `json:"lane"`
	Subject       string      `json:"subject"`
	Summary       string      `json:"summary"`
	CreatedAt     string      `json:"createdAt,omitempty"`
	Status        string      `json:"status"`
	Actor         string      `json:"actor,omitempty"`
	Risk          string      `json:"risk,omitempty"`
	Target        string      `json:"target,omitempty"`
	BatchID       string      `json:"batchId,omitempty"`
	Gate          GateDetails `json:"gate"`
	EventID       string      `json:"eventId,omitempty"`
}

type GateDetails struct {
	Action                      string            `json:"action"`
	Scope                       string            `json:"scope,omitempty"`
	Budget                      string            `json:"budget,omitempty"`
	RequestedBudget             autonomy.Budget   `json:"requestedBudget"`
	OutputPaths                 []string          `json:"outputPaths,omitempty"`
	TriedLightSteps             []string          `json:"triedLightSteps,omitempty"`
	StopConditions              []string          `json:"stopConditions,omitempty"`
	RequiresConfirmation        bool              `json:"requiresConfirmation"`
	DeniedUntilUserConfirmation []string          `json:"deniedUntilUserConfirmation"`
	Authorization               autonomy.Decision `json:"authorization"`
}

func PlanDryRun(repoRoot, caseRoot, pack string, opt Options) (Plan, error) {
	inst, preview, blocked, err := buildPreview(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return Plan{}, err
	}
	brief := gateMissionBrief(inst.CaseRoot)
	return Plan{
		SchemaVersion:        1,
		Command:              "gate",
		CaseRoot:             inst.CaseRoot,
		RepoRoot:             repoRoot,
		Pack:                 pack,
		IsMutation:           false,
		ReviewRequired:       preview.Gate.RequiresConfirmation,
		RequiresConfirmation: preview.Gate.RequiresConfirmation,
		EventPreview:         preview,
		MissionBrief:         brief,
		ExecutorAction:       gateExecutorAction(inst.CaseRoot, preview.Lane, brief),
		WouldExecutorAction:  gateWouldExecutorAction(inst.CaseRoot, preview, brief),
		BlockedActions:       blocked,
		NextSteps:            planNextSteps(preview),
	}, nil
}

func Apply(repoRoot, caseRoot, pack string, opt Options) (ApplyResult, error) {
	if strings.TrimSpace(opt.Actor) == "" {
		return ApplyResult{}, fmt.Errorf("gate -Apply requires -Actor <recorded-by>; this records who wrote the gate authorization request")
	}
	inst, preview, _, err := buildPreview(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return ApplyResult{}, err
	}
	preview.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	preview.EventID = eventID(preview)
	relPath, _, err := mission.FactPath(inst.CaseRoot, "request")
	if err != nil {
		return ApplyResult{}, err
	}
	known, err := mission.ReadFactEventIDs(inst.CaseRoot, "request")
	if err != nil {
		return ApplyResult{}, err
	}
	exists := known[preview.EventID]
	result := ApplyResult{
		SchemaVersion: 1,
		Command:       "gate",
		CaseRoot:      inst.CaseRoot,
		RepoRoot:      repoRoot,
		Pack:          pack,
		IsMutation:    true,
		Applied:       false,
		EventID:       preview.EventID,
		Path:          relPath,
		Event:         preview,
		NextSteps:     applyNextSteps(preview),
	}
	if exists {
		result.MissionBrief = gateMissionBrief(inst.CaseRoot)
		result.ExecutorAction = gateExecutorAction(inst.CaseRoot, preview.Lane, result.MissionBrief)
		result.Reason = "duplicate eventId"
		return result, nil
	}
	if _, _, err := mission.AppendFact(inst.CaseRoot, "request", preview); err != nil {
		return ApplyResult{}, err
	}
	result.Applied = true
	result.MissionBrief = gateMissionBrief(inst.CaseRoot)
	result.ExecutorAction = gateExecutorAction(inst.CaseRoot, preview.Lane, result.MissionBrief)
	return result, nil
}

func gateMissionBrief(caseRoot string) mission.Brief {
	brief, err := mission.CaseBrief(caseRoot, mission.BuildOptions{
		MaxRows:            mission.DefaultMaxRows,
		OpenDecisionAction: "review open candidate/decision item(s) with evidence and authority boundary",
	})
	if err != nil {
		return mission.Brief{Summary: "unavailable: " + err.Error()}
	}
	return brief
}

func gateExecutorAction(caseRoot, laneID string, brief mission.Brief) mission.ExecutorAction {
	board, facts, err := gateBoardFacts(caseRoot)
	if err != nil {
		return mission.LaneExecutorAction(mission.Lane{ID: laneID, Label: mission.BoardLaneLabel(mission.BoardLane{ID: laneID})}, mission.Facts{}, brief)
	}
	lane := gateMissionLane(board, laneID)
	return mission.LaneExecutorAction(lane, facts, brief)
}

func gateWouldExecutorAction(caseRoot string, preview EventPreview, brief mission.Brief) mission.ExecutorAction {
	board, facts, err := gateBoardFacts(caseRoot)
	if err != nil {
		facts = mission.Facts{}
	}
	facts.Requests = append(append([]map[string]any{}, facts.Requests...), gatePreviewMap(preview))
	wouldBrief := brief
	if len(board.Lanes) > 0 {
		wouldBrief = mission.BuildWithOptions(mission.BoardLanes(board.Lanes), facts, mission.BuildOptions{
			MaxRows:            mission.DefaultMaxRows,
			OpenDecisionAction: "review open candidate/decision item(s) with evidence and authority boundary",
		})
	}
	return mission.LaneExecutorAction(gateMissionLane(board, preview.Lane), facts, wouldBrief)
}

func gateBoardFacts(caseRoot string) (mission.Board, mission.Facts, error) {
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return mission.Board{}, mission.Facts{}, err
	}
	facts, err := mission.ReadFacts(caseRoot)
	if err != nil {
		return board, mission.Facts{}, err
	}
	return board, facts, nil
}

func gateMissionLane(board mission.Board, laneID string) mission.Lane {
	for _, item := range board.Lanes {
		if strings.EqualFold(strings.TrimSpace(item.ID), strings.TrimSpace(laneID)) {
			return mission.Lane{ID: strings.TrimSpace(item.ID), Label: mission.BoardLaneLabel(item), Status: item.Status}
		}
	}
	return mission.Lane{ID: laneID, Label: mission.BoardLaneLabel(mission.BoardLane{ID: laneID})}
}

func gatePreviewMap(preview EventPreview) map[string]any {
	authorization := map[string]any{
		"decision":  preview.Gate.Authorization.Decision,
		"profileId": preview.Gate.Authorization.ProfileID,
	}
	gate := map[string]any{
		"action":          preview.Gate.Action,
		"scope":           preview.Gate.Scope,
		"budget":          preview.Gate.Budget,
		"authorization":   authorization,
		"stopConditions":  preview.Gate.StopConditions,
		"outputPaths":     preview.Gate.OutputPaths,
		"requestedBudget": preview.Gate.RequestedBudget,
	}
	return map[string]any{
		"kind":      preview.Kind,
		"lane":      preview.Lane,
		"subject":   preview.Subject,
		"summary":   preview.Summary,
		"status":    preview.Status,
		"actor":     preview.Actor,
		"risk":      preview.Risk,
		"target":    preview.Target,
		"batchId":   preview.BatchID,
		"gate":      gate,
		"eventId":   preview.EventID,
		"createdAt": preview.CreatedAt,
	}
}

func planNextSteps(preview EventPreview) []string {
	if preview.Gate.Authorization.Decision == autonomy.DecisionPreauthorized {
		return []string{
			"Record this authorized gate decision in the ledger if the preflight matches the intended action, target, budget, output paths, and stop conditions.",
			"The actual heavy tool still runs outside /rekit; keep execution within the durable lane autonomy profile and record output evidence afterward.",
			"After the tool run, append an observation event summarizing output path, findings, errors, budget use, and next action.",
		}
	}
	return []string{
		"Record the pending-gate request in the ledger only if this preview is accepted.",
		"Ask the user to confirm the exact action, target, scope, budget, output paths, and stop conditions.",
		"Run the heavy tool only after explicit confirmation or a valid durable lane autonomy profile; this dry-run is not approval.",
		"After the tool run, append an observation event summarizing output path, findings, errors, and next action.",
	}
}

func applyNextSteps(preview EventPreview) []string {
	if preview.Gate.Authorization.Decision == autonomy.DecisionPreauthorized {
		return []string{
			"This ledger write records a durable lane autonomy authorization decision; /rekit still did not execute the heavy tool.",
			"Run the heavy tool only within the authorized target, budget, output paths, and stop conditions.",
			"After the tool run, append an observation event summarizing output path, findings, errors, budget use, and next action.",
		}
	}
	return []string{
		"Ask the user to confirm the exact action, target, scope, budget, output paths, and stop conditions before running the heavy tool.",
		"This ledger write records a pending gate only; it is not heavy-tool approval.",
		"After the tool run, append an observation event summarizing output path, findings, errors, and next action.",
	}
}

func buildPreview(repoRoot, caseRoot, pack string, opt Options) (instance.Instance, EventPreview, []string, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return instance.Instance{}, EventPreview{}, nil, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return instance.Instance{}, EventPreview{}, nil, err
	}
	action := strings.ToLower(strings.TrimSpace(opt.Action))
	allowed := m.HeavyToolGateIDs()
	if len(allowed) == 0 {
		return instance.Instance{}, EventPreview{}, nil, fmt.Errorf("pack manifest must declare heavyToolGates before gate can accept heavy-tool actions")
	}
	if action == "" {
		return instance.Instance{}, EventPreview{}, nil, fmt.Errorf("gate requires -Action %s", strings.Join(allowed, "|"))
	}
	gate, ok := m.HeavyToolGate(action)
	if !ok {
		return instance.Instance{}, EventPreview{}, nil, fmt.Errorf("invalid gate action %q; allowed: %s", action, strings.Join(allowed, ","))
	}
	if !gate.RequiresConfirmation {
		return instance.Instance{}, EventPreview{}, nil, fmt.Errorf("gate action %q must require user confirmation in pack manifest", action)
	}
	if strings.TrimSpace(gate.DefaultRisk) == "" {
		return instance.Instance{}, EventPreview{}, nil, fmt.Errorf("gate action %q is missing defaultRisk in pack manifest", action)
	}
	if err := validateGateRisk("manifest defaultRisk", gate.DefaultRisk); err != nil {
		return instance.Instance{}, EventPreview{}, nil, fmt.Errorf("gate action %q has invalid %w", action, err)
	}
	if len(gate.StopConditions) == 0 {
		return instance.Instance{}, EventPreview{}, nil, fmt.Errorf("gate action %q is missing stopConditions in pack manifest", action)
	}
	lane := strings.TrimSpace(opt.Lane)
	if lane == "" {
		return instance.Instance{}, EventPreview{}, nil, fmt.Errorf("gate requires -Lane <lane id>")
	}
	lane, err = canonicalLane(inst.CaseRoot, lane)
	if err != nil {
		return instance.Instance{}, EventPreview{}, nil, err
	}
	subject := strings.TrimSpace(opt.Subject)
	if subject == "" {
		subject = action + " gate"
	}
	summary := strings.TrimSpace(opt.Summary)
	risk, err := parseGateRisk(opt.Risk)
	if err != nil {
		return instance.Instance{}, EventPreview{}, nil, err
	}
	if risk == "" {
		risk = strings.TrimSpace(gate.DefaultRisk)
	}
	stopConditions, err := parseStopConditions(opt.StopConditions)
	if err != nil {
		return instance.Instance{}, EventPreview{}, nil, err
	}
	if len(stopConditions) == 0 {
		stopConditions = append([]string{}, gate.StopConditions...)
		if err := validateStopConditions("manifest stopConditions", stopConditions); err != nil {
			return instance.Instance{}, EventPreview{}, nil, fmt.Errorf("gate action %q has invalid %w", action, err)
		}
	}
	outputPaths, err := parseOutputPaths(inst.CaseRoot, opt.OutputPaths)
	if err != nil {
		return instance.Instance{}, EventPreview{}, nil, err
	}
	requestedBudget := autonomy.Budget{RuntimeSeconds: opt.RuntimeSeconds, DiskMB: opt.DiskMB, Requests: opt.Requests}
	target := strings.TrimSpace(opt.TargetRef)
	authorization := authorizationDecision(inst.CaseRoot, lane, m, autonomy.Request{
		Lane:           lane,
		Action:         action,
		Target:         target,
		Budget:         requestedBudget,
		StopConditions: stopConditions,
		OutputPaths:    outputPaths,
	})
	status := "pending-gate"
	if authorization.Decision == autonomy.DecisionPreauthorized {
		status = "authorized-gate"
	}
	if summary == "" {
		if authorization.Decision == autonomy.DecisionPreauthorized {
			summary = "Preauthorized by durable lane autonomy profile before running " + action
		} else {
			summary = "Request user confirmation before running " + action
		}
	}
	blocked := []string{}
	if authorization.RequiresConfirmation {
		blocked = []string{action}
	}
	preview := EventPreview{
		SchemaVersion: 1,
		Kind:          "request",
		Lane:          lane,
		Subject:       subject,
		Summary:       summary,
		Status:        status,
		Actor:         strings.TrimSpace(opt.Actor),
		Risk:          risk,
		Target:        target,
		BatchID:       strings.TrimSpace(opt.BatchID),
		Gate: GateDetails{
			Action:                      action,
			Scope:                       strings.TrimSpace(opt.Scope),
			Budget:                      strings.TrimSpace(opt.Budget),
			RequestedBudget:             requestedBudget,
			OutputPaths:                 outputPaths,
			TriedLightSteps:             splitList(opt.TriedLightSteps),
			StopConditions:              stopConditions,
			RequiresConfirmation:        authorization.RequiresConfirmation,
			DeniedUntilUserConfirmation: blocked,
			Authorization:               authorization,
		},
	}
	return inst, preview, blocked, nil
}

func canonicalLane(caseRoot, lane string) (string, error) {
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("gate requires .rekit/board.json to validate lane: %s", filepath.Join(caseRoot, ".rekit", "board.json"))
		}
		return "", err
	}
	known := mission.BoardLaneIDs(board.Lanes)
	for _, item := range board.Lanes {
		if strings.EqualFold(strings.TrimSpace(item.ID), strings.TrimSpace(lane)) {
			return strings.TrimSpace(item.ID), nil
		}
	}
	if len(known) == 0 {
		return "", fmt.Errorf("gate requires at least one lane in .rekit/board.json")
	}
	return "", fmt.Errorf("unknown lane %q; known: %s", lane, strings.Join(known, ","))
}

func splitList(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || r == '\n' })
	out := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseOutputPaths(caseRoot, value string) ([]string, error) {
	paths := splitList(value)
	for _, rel := range paths {
		if filepath.IsAbs(rel) {
			return nil, fmt.Errorf("gate output path must be case-relative: %s", rel)
		}
		clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(rel)))
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
			return nil, fmt.Errorf("gate output path escapes case root: %s", rel)
		}
		if _, err := refsf.SafeJoin(caseRoot, rel); err != nil {
			return nil, fmt.Errorf("gate output path escapes case root: %s", rel)
		}
	}
	return paths, nil
}

func authorizationDecision(caseRoot, lane string, m *manifest.Manifest, req autonomy.Request) autonomy.Decision {
	profile, path, exists, err := autonomy.Read(caseRoot, lane)
	rel := autonomy.RelPath(lane)
	if err != nil {
		return autonomy.Decision{Decision: autonomy.DecisionInvalidProfile, Mode: autonomy.ModeManualGate, ProfilePath: rel, Source: "lane-autonomy-profile", RequiresConfirmation: true, Reasons: []string{err.Error()}}
	}
	if exists {
		if err := autonomy.Validate(profile, lane, m, caseRoot); err != nil {
			return autonomy.Decision{Decision: autonomy.DecisionInvalidProfile, Mode: profile.Mode, ProfileID: profile.ProfileID, ProfilePath: rel, ProfileHash: autonomy.FileHash(path), Source: "lane-autonomy-profile", RequiresConfirmation: true, Reasons: []string{err.Error()}, RecordRequired: profile.RecordRequired}
		}
	}
	return autonomy.Evaluate(profile, rel, exists, autonomy.FileHash(path), req, time.Now().UTC())
}

func parseGateRisk(value string) (string, error) {
	risk := strings.TrimSpace(value)
	if risk == "" {
		return "", nil
	}
	if err := validateGateRisk("risk", risk); err != nil {
		return "", fmt.Errorf("gate %w", err)
	}
	return risk, nil
}

func validateGateRisk(field, value string) error {
	switch strings.TrimSpace(value) {
	case "medium", "high", "critical":
		return nil
	default:
		return fmt.Errorf("%s has unsupported value: %s", field, value)
	}
}

var stopConditionTokenPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[-_][a-z0-9]+)*$`)

func parseStopConditions(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	normalized := strings.NewReplacer(";", ",", "\n", ",").Replace(value)
	conditions := []string{}
	for item := range strings.SplitSeq(normalized, ",") {
		conditions = append(conditions, strings.TrimSpace(item))
	}
	if err := validateStopConditions("stopConditions", conditions); err != nil {
		return nil, fmt.Errorf("gate %w", err)
	}
	return conditions, nil
}

func validateStopConditions(field string, conditions []string) error {
	seen := map[string]bool{}
	for _, condition := range conditions {
		if condition == "" {
			return fmt.Errorf("%s contains an empty item", field)
		}
		if !stopConditionTokenPattern.MatchString(condition) {
			return fmt.Errorf("%s has invalid item: %s", field, condition)
		}
		key := strings.ToLower(condition)
		if seen[key] {
			return fmt.Errorf("%s contains duplicate item: %s", field, condition)
		}
		seen[key] = true
	}
	return nil
}

func eventID(event EventPreview) string {
	seed := strings.Join([]string{
		event.Kind,
		event.Lane,
		event.Subject,
		event.Summary,
		event.Actor,
		event.Risk,
		event.Target,
		event.BatchID,
		event.Gate.Action,
		event.Gate.Scope,
		event.Gate.Budget,
		fmt.Sprintf("%d/%d/%d", event.Gate.RequestedBudget.RuntimeSeconds, event.Gate.RequestedBudget.DiskMB, event.Gate.RequestedBudget.Requests),
		strings.Join(event.Gate.OutputPaths, ","),
		event.Gate.Authorization.Decision,
		event.Gate.Authorization.ProfileHash,
		strings.Join(event.Gate.TriedLightSteps, ","),
		strings.Join(event.Gate.StopConditions, ","),
	}, "|")
	sum := sha256.Sum256([]byte(seed))
	return "evt-" + hex.EncodeToString(sum[:])[:16]
}
