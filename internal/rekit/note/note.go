package note

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

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
	Kind string
	Lane string
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
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}
