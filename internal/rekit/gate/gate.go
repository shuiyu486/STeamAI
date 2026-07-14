package gate

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
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
	TriedLightSteps string
	StopConditions  string
}

type Plan struct {
	SchemaVersion        int          `json:"schemaVersion"`
	Command              string       `json:"command"`
	CaseRoot             string       `json:"caseRoot"`
	RepoRoot             string       `json:"repoRoot"`
	Pack                 string       `json:"pack"`
	IsMutation           bool         `json:"isMutation"`
	ReviewRequired       bool         `json:"reviewRequired"`
	RequiresConfirmation bool         `json:"requiresConfirmation"`
	EventPreview         EventPreview `json:"eventPreview"`
	BlockedActions       []string     `json:"blockedActions"`
	NextSteps            []string     `json:"nextSteps"`
}

type ApplyResult struct {
	SchemaVersion int          `json:"schemaVersion"`
	Command       string       `json:"command"`
	CaseRoot      string       `json:"caseRoot"`
	RepoRoot      string       `json:"repoRoot"`
	Pack          string       `json:"pack"`
	IsMutation    bool         `json:"isMutation"`
	Applied       bool         `json:"applied"`
	EventID       string       `json:"eventId"`
	Path          string       `json:"path"`
	Reason        string       `json:"reason,omitempty"`
	Event         EventPreview `json:"event"`
	NextSteps     []string     `json:"nextSteps"`
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
	Action                      string   `json:"action"`
	Scope                       string   `json:"scope,omitempty"`
	Budget                      string   `json:"budget,omitempty"`
	TriedLightSteps             []string `json:"triedLightSteps,omitempty"`
	StopConditions              []string `json:"stopConditions,omitempty"`
	RequiresConfirmation        bool     `json:"requiresConfirmation"`
	DeniedUntilUserConfirmation []string `json:"deniedUntilUserConfirmation"`
}

func PlanDryRun(repoRoot, caseRoot, pack string, opt Options) (Plan, error) {
	inst, preview, blocked, err := buildPreview(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return Plan{}, err
	}
	return Plan{
		SchemaVersion:        1,
		Command:              "gate",
		CaseRoot:             inst.CaseRoot,
		RepoRoot:             repoRoot,
		Pack:                 pack,
		IsMutation:           false,
		ReviewRequired:       true,
		RequiresConfirmation: true,
		EventPreview:         preview,
		BlockedActions:       blocked,
		NextSteps: []string{
			"Record the pending-gate request in the ledger only if this preview is accepted.",
			"Ask the user to confirm the exact action, target, scope, budget, and stop conditions.",
			"Run the heavy tool only after that explicit confirmation; this dry-run is not approval.",
			"After the tool run, append an observation event summarizing output path, findings, errors, and next action.",
		},
	}, nil
}

func Apply(repoRoot, caseRoot, pack string, opt Options) (ApplyResult, error) {
	if strings.TrimSpace(opt.Actor) == "" {
		return ApplyResult{}, fmt.Errorf("gate -Apply requires -Actor <confirmed-by>; this records who approved writing the pending gate")
	}
	inst, preview, _, err := buildPreview(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return ApplyResult{}, err
	}
	preview.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	preview.EventID = eventID(preview)
	path := filepath.Join(inst.CaseRoot, ".rekit", "facts", "requests.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ApplyResult{}, err
	}
	exists, err := eventIDExists(path, preview.EventID)
	if err != nil {
		return ApplyResult{}, err
	}
	result := ApplyResult{
		SchemaVersion: 1,
		Command:       "gate",
		CaseRoot:      inst.CaseRoot,
		RepoRoot:      repoRoot,
		Pack:          pack,
		IsMutation:    true,
		Applied:       false,
		EventID:       preview.EventID,
		Path:          relativeCasePath(inst.CaseRoot, path),
		Event:         preview,
		NextSteps: []string{
			"Ask the user to confirm the exact action, target, scope, budget, and stop conditions before running the heavy tool.",
			"This ledger write records a pending gate only; it is not heavy-tool approval.",
			"After the tool run, append an observation event summarizing output path, findings, errors, and next action.",
		},
	}
	if exists {
		result.Reason = "duplicate eventId"
		return result, nil
	}
	line, err := json.Marshal(preview)
	if err != nil {
		return ApplyResult{}, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return ApplyResult{}, err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\r', '\n')); err != nil {
		return ApplyResult{}, err
	}
	result.Applied = true
	return result, nil
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
	if err := assertLane(caseRoot, lane); err != nil {
		return instance.Instance{}, EventPreview{}, nil, err
	}
	subject := strings.TrimSpace(opt.Subject)
	if subject == "" {
		subject = action + " gate"
	}
	summary := strings.TrimSpace(opt.Summary)
	if summary == "" {
		summary = "Request user confirmation before running " + action
	}
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
	blocked := []string{action}
	preview := EventPreview{
		SchemaVersion: 1,
		Kind:          "request",
		Lane:          lane,
		Subject:       subject,
		Summary:       summary,
		Status:        "pending-gate",
		Actor:         strings.TrimSpace(opt.Actor),
		Risk:          risk,
		Target:        strings.TrimSpace(opt.TargetRef),
		BatchID:       strings.TrimSpace(opt.BatchID),
		Gate: GateDetails{
			Action:                      action,
			Scope:                       strings.TrimSpace(opt.Scope),
			Budget:                      strings.TrimSpace(opt.Budget),
			TriedLightSteps:             splitList(opt.TriedLightSteps),
			StopConditions:              stopConditions,
			RequiresConfirmation:        gate.RequiresConfirmation,
			DeniedUntilUserConfirmation: blocked,
		},
	}
	return inst, preview, blocked, nil
}

type boardFile struct {
	Lanes []struct {
		ID string `json:"id"`
	} `json:"lanes"`
}

func assertLane(caseRoot, lane string) error {
	path := filepath.Join(caseRoot, ".rekit", "board.json")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("gate requires .rekit/board.json to validate lane: %s", path)
		}
		return err
	}
	var board boardFile
	if err := json.Unmarshal(b, &board); err != nil {
		return fmt.Errorf("invalid board json: %w", err)
	}
	known := []string{}
	for _, item := range board.Lanes {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		known = append(known, id)
		if strings.EqualFold(id, lane) {
			return nil
		}
	}
	if len(known) == 0 {
		return fmt.Errorf("gate requires at least one lane in .rekit/board.json")
	}
	return fmt.Errorf("unknown lane %q; known: %s", lane, strings.Join(known, ","))
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
		strings.Join(event.Gate.TriedLightSteps, ","),
		strings.Join(event.Gate.StopConditions, ","),
	}, "|")
	sum := sha256.Sum256([]byte(seed))
	return "evt-" + hex.EncodeToString(sum[:])[:16]
}

func eventIDExists(path, id string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item struct {
			EventID string `json:"eventId"`
		}
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			continue
		}
		if item.EventID == id {
			return true, nil
		}
	}
	return false, scanner.Err()
}

func relativeCasePath(caseRoot, path string) string {
	rel, err := filepath.Rel(caseRoot, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
