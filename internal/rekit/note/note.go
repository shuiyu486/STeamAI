package note

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
)

const maxListRows = 20

var validKinds = []string{
	"observation",
	"candidate",
	"request",
	"publication",
	"decision",
	"hypothesis",
	"verification",
	"intervention",
	"rollback",
}

type Options struct {
	Kind         string
	Lane         string
	Subject      string
	Summary      string
	Actor        string
	Risk         string
	Related      string
	Confidence   string
	Decision     string
	Reason       string
	Status       string
	BatchID      string
	Target       string
	Verifier     string
	Verdict      string
	Action       string
	ApprovedBy   string
	Scope        string
	Expires      string
	EvidenceRefs string
	EventID      string
}

type AppendResult struct {
	SchemaVersion int            `json:"schemaVersion"`
	Command       string         `json:"command"`
	CaseRoot      string         `json:"caseRoot"`
	RepoRoot      string         `json:"repoRoot"`
	Pack          string         `json:"pack"`
	IsMutation    bool           `json:"isMutation"`
	Applied       bool           `json:"applied"`
	EventID       string         `json:"eventId"`
	Path          string         `json:"path"`
	Reason        string         `json:"reason,omitempty"`
	Event         map[string]any `json:"event"`
}

type event map[string]any

func List(repoRoot, caseRoot, pack string, opt Options) (string, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return "", err
	}
	kind := strings.ToLower(strings.TrimSpace(opt.Kind))
	if kind != "" && !isValidKind(kind) {
		return "", fmt.Errorf("invalid note kind: %s", opt.Kind)
	}
	kinds := validKinds
	if kind != "" {
		kinds = []string{kind}
	}
	laneFilter := strings.TrimSpace(opt.Lane)
	factsRoot := filepath.Join(inst.CaseRoot, ".rekit", "facts")
	var out bytes.Buffer
	for _, k := range kinds {
		items, err := readJSONLines(filepath.Join(factsRoot, factFile(k)))
		if err != nil {
			return "", err
		}
		if laneFilter != "" {
			filtered := []event{}
			for _, item := range items {
				if stringValue(item, "lane") == laneFilter {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}
		if len(items) == 0 {
			continue
		}
		fmt.Fprintf(&out, "[%s] (%d 条)\n", k, len(items))
		shown := lastEvents(items, maxListRows)
		for _, item := range shown {
			subject := firstText(stringValue(item, "subject"), stringValue(item, "kind"))
			fmt.Fprintf(&out, "- %s | lane=%s%s\n", subject, stringValue(item, "lane"), noteExtra(k, item))
		}
		if rest := len(items) - len(shown); rest > 0 {
			fmt.Fprintf(&out, "- 另有 %d 条 %s\n", rest, k)
		}
		fmt.Fprintln(&out)
	}
	return out.String(), nil
}

func Append(repoRoot, caseRoot, pack string, opt Options, whatIf bool) (AppendResult, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return AppendResult{}, err
	}
	kind := strings.ToLower(strings.TrimSpace(opt.Kind))
	if kind == "" {
		return AppendResult{}, fmt.Errorf("note requires -Kind observation|candidate|request|publication|decision|hypothesis|verification|intervention|rollback")
	}
	if !isValidKind(kind) {
		return AppendResult{}, fmt.Errorf("invalid note kind: %s", opt.Kind)
	}
	lane := strings.TrimSpace(opt.Lane)
	if lane == "" {
		return AppendResult{}, fmt.Errorf("note requires -Lane <lane id>")
	}
	if err := assertLane(inst.CaseRoot, lane); err != nil {
		return AppendResult{}, err
	}
	if err := validateAppendOptions(kind, opt); err != nil {
		return AppendResult{}, err
	}
	event := buildEvent(kind, lane, opt)
	eventID := strings.TrimSpace(opt.EventID)
	if eventID == "" {
		eventID = eventIDFor(event)
	}
	event["eventId"] = eventID
	path := filepath.Join(inst.CaseRoot, ".rekit", "facts", factFile(kind))
	result := AppendResult{
		SchemaVersion: 1,
		Command:       "note",
		CaseRoot:      inst.CaseRoot,
		RepoRoot:      repoRoot,
		Pack:          pack,
		IsMutation:    !whatIf,
		Applied:       false,
		EventID:       eventID,
		Path:          relativeCasePath(inst.CaseRoot, path),
		Event:         event,
	}
	if whatIf {
		result.Reason = "what-if"
		return result, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return AppendResult{}, err
	}
	exists, err := eventIDExists(filepath.Join(inst.CaseRoot, ".rekit", "facts"), eventID)
	if err != nil {
		return AppendResult{}, err
	}
	if exists {
		result.Reason = "duplicate eventId"
		return result, nil
	}
	line, err := json.Marshal(event)
	if err != nil {
		return AppendResult{}, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return AppendResult{}, err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\r', '\n')); err != nil {
		return AppendResult{}, err
	}
	result.Applied = true
	return result, nil
}

func buildEvent(kind, lane string, opt Options) map[string]any {
	event := map[string]any{
		"schemaVersion": 1,
		"kind":          kind,
		"lane":          lane,
		"subject":       strings.TrimSpace(opt.Subject),
		"summary":       strings.TrimSpace(opt.Summary),
		"createdAt":     time.Now().UTC().Format(time.RFC3339Nano),
	}
	addString := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			event[key] = strings.TrimSpace(value)
		}
	}
	addString("actor", opt.Actor)
	addString("risk", opt.Risk)
	if related := splitList(opt.Related); len(related) > 0 {
		event["related"] = related
	}
	addString("confidence", opt.Confidence)
	addString("decision", opt.Decision)
	addString("reason", opt.Reason)
	addString("status", opt.Status)
	addString("batchId", opt.BatchID)
	if refs := splitList(opt.EvidenceRefs); len(refs) > 0 {
		event["evidenceRefs"] = refs
	}
	addString("target", opt.Target)
	if kind == "verification" {
		addString("verifier", opt.Verifier)
		addString("verdict", opt.Verdict)
	}
	if kind == "intervention" {
		addString("action", opt.Action)
		addString("approvedBy", opt.ApprovedBy)
		addString("scope", opt.Scope)
		addString("expires", opt.Expires)
	}
	return event
}

func validateAppendOptions(kind string, opt Options) error {
	validConfidence := []string{"low", "medium", "high"}
	validDecision := []string{"accept", "reject", "defer", "supersede"}
	validStatus := []string{"open", "accepted", "rejected", "superseded", "resolved", "deferred", "pending-gate", "confirmed", "needs_more_evidence"}
	validVerifier := []string{"manual-review", "schema-check", "focused-trace", "parity", "cross-run", "tool-review"}
	validVerdict := []string{"accepted", "rejected", "inconclusive", "needs-more-evidence"}
	validInterventionAction := []string{"override", "rollback", "heavy-tool-approval", "schema-migration", "external-side-effect"}
	if confidence := strings.TrimSpace(opt.Confidence); confidence != "" && !slices.Contains(validConfidence, confidence) {
		return fmt.Errorf("invalid Confidence %q; allowed: %s", confidence, strings.Join(validConfidence, ","))
	}
	if decision := strings.TrimSpace(opt.Decision); kind == "decision" && decision != "" && !slices.Contains(validDecision, decision) {
		return fmt.Errorf("invalid Decision %q; allowed: %s", decision, strings.Join(validDecision, ","))
	}
	if verdict := strings.TrimSpace(opt.Verdict); kind == "verification" && verdict != "" && !slices.Contains(validVerdict, verdict) {
		return fmt.Errorf("invalid Verdict %q; allowed: %s", verdict, strings.Join(validVerdict, ","))
	}
	if verifier := strings.TrimSpace(opt.Verifier); kind == "verification" && verifier != "" && !slices.Contains(validVerifier, verifier) {
		return fmt.Errorf("invalid Verifier %q; allowed: %s", verifier, strings.Join(validVerifier, ","))
	}
	if action := strings.TrimSpace(opt.Action); kind == "intervention" && action != "" && !slices.Contains(validInterventionAction, action) {
		return fmt.Errorf("invalid Action %q; allowed: %s", action, strings.Join(validInterventionAction, ","))
	}
	if status := strings.TrimSpace(opt.Status); status != "" && !slices.Contains(validStatus, status) {
		return fmt.Errorf("invalid Status %q; allowed: %s", status, strings.Join(validStatus, ","))
	}
	for _, ref := range strings.FieldsFunc(opt.EvidenceRefs, func(r rune) bool { return r == ',' || r == ';' || r == '\n' }) {
		if strings.TrimSpace(ref) == "" {
			return fmt.Errorf("EvidenceRefs contains empty element")
		}
	}
	return nil
}

func isValidKind(kind string) bool {
	return slices.Contains(validKinds, kind)
}

func factFile(kind string) string {
	switch kind {
	case "observation":
		return "observations.jsonl"
	case "candidate":
		return "candidates.jsonl"
	case "request":
		return "requests.jsonl"
	case "publication":
		return "publications.jsonl"
	case "decision":
		return "decisions.jsonl"
	case "hypothesis":
		return "hypotheses.jsonl"
	case "verification":
		return "verifications.jsonl"
	case "intervention":
		return "interventions.jsonl"
	case "rollback":
		return "rollbacks.jsonl"
	default:
		return kind + "s.jsonl"
	}
}

func noteExtra(kind string, item event) string {
	extra := ""
	switch kind {
	case "candidate":
		extra = fmt.Sprintf(" | confidence=%s | status=%s | risk=%s", stringValue(item, "confidence"), stringValue(item, "status"), stringValue(item, "risk"))
	case "decision":
		decision := firstText(stringValue(item, "decision"), stringValue(item, "action"))
		by := firstText(stringValue(item, "confirmedBy"), stringValue(item, "actor"))
		extra = fmt.Sprintf(" | decision=%s | by=%s", decision, by)
	case "request":
		extra = gateDetail(item, false, true)
	case "verification":
		extra = fmt.Sprintf(" | verifier=%s | verdict=%s | target=%s", stringValue(item, "verifier"), stringValue(item, "verdict"), stringValue(item, "target"))
	case "intervention":
		extra = fmt.Sprintf(" | action=%s | target=%s | approvedBy=%s | scope=%s | status=%s | reason=%s", stringValue(item, "action"), stringValue(item, "target"), stringValue(item, "approvedBy"), stringValue(item, "scope"), stringValue(item, "status"), stringValue(item, "reason"))
	case "rollback":
		extra = fmt.Sprintf(" | target=%s | status=%s | reason=%s", stringValue(item, "target"), stringValue(item, "status"), stringValue(item, "reason"))
	}
	if batch := stringValue(item, "batchId"); strings.TrimSpace(batch) != "" {
		extra += " | batch=" + batch
	}
	return extra
}

func gateDetail(item event, omitStatus, omitBatch bool) string {
	parts := []string{}
	add := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			parts = append(parts, key+"="+value)
		}
	}
	if !omitStatus {
		add("status", stringValue(item, "status"))
	}
	add("by", stringValue(item, "actor"))
	add("risk", stringValue(item, "risk"))
	add("target", stringValue(item, "target"))
	if !omitBatch {
		add("batch", stringValue(item, "batchId"))
	}
	if gate, ok := item["gate"].(map[string]any); ok {
		add("action", stringValue(gate, "action"))
		add("scope", stringValue(gate, "scope"))
		add("budget", stringValue(gate, "budget"))
		add("tried", stringValue(gate, "triedLightSteps"))
		add("stop", stringValue(gate, "stopConditions"))
	}
	if len(parts) == 0 {
		return ""
	}
	return " | " + strings.Join(parts, " | ")
}

func readJSONLines(path string) ([]event, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	out := []event{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item event
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("invalid JSONL %s: %w", path, err)
		}
		out = append(out, item)
	}
	return out, scanner.Err()
}

func lastEvents(items []event, n int) []event {
	if len(items) <= n {
		return items
	}
	return items[len(items)-n:]
}

func firstText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func stringValue(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []any:
		parts := []string{}
		for _, item := range t {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, ",")
	case []string:
		return strings.Join(t, ",")
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
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
			return fmt.Errorf("note requires .rekit/board.json to validate lane: %s", path)
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
		if id == lane {
			return nil
		}
	}
	if len(known) == 0 {
		return fmt.Errorf("note requires at least one lane in .rekit/board.json")
	}
	return fmt.Errorf("unknown lane %q; known: %s", lane, strings.Join(known, ","))
}

func eventIDFor(event map[string]any) string {
	seed := strings.Join([]string{
		stringValue(event, "kind"),
		stringValue(event, "lane"),
		stringValue(event, "subject"),
		stringValue(event, "summary"),
		stringValue(event, "actor"),
		stringValue(event, "risk"),
		stringValue(event, "related"),
		stringValue(event, "confidence"),
		stringValue(event, "decision"),
		stringValue(event, "reason"),
		stringValue(event, "status"),
		stringValue(event, "batchId"),
		stringValue(event, "evidenceRefs"),
		stringValue(event, "target"),
		stringValue(event, "verifier"),
		stringValue(event, "verdict"),
		stringValue(event, "action"),
		stringValue(event, "approvedBy"),
		stringValue(event, "scope"),
		stringValue(event, "expires"),
		stringValue(event, "createdAt"),
	}, "|")
	sum := sha256.Sum256([]byte(seed))
	return "evt-" + hex.EncodeToString(sum[:])[:16]
}

func eventIDExists(factsRoot, id string) (bool, error) {
	for _, kind := range validKinds {
		path := filepath.Join(factsRoot, factFile(kind))
		items, err := readJSONLines(path)
		if err != nil {
			return false, err
		}
		for _, item := range items {
			if stringValue(item, "eventId") == id {
				return true, nil
			}
		}
	}
	return false, nil
}

func relativeCasePath(caseRoot, path string) string {
	rel, err := filepath.Rel(caseRoot, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
