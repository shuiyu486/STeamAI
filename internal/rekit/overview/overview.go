package overview

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

const maxRows = 10

type event map[string]any

type Inventory struct {
	SchemaVersion  int              `json:"schemaVersion"`
	Command        string           `json:"command"`
	CaseRoot       string           `json:"caseRoot"`
	RepoRoot       string           `json:"repoRoot"`
	Pack           string           `json:"pack"`
	IsMutation     bool             `json:"isMutation"`
	AutomationMode string           `json:"automationMode"`
	Lanes          []LaneSummary    `json:"lanes"`
	Counts         FactCounts       `json:"counts"`
	Sections       OverviewSections `json:"sections"`
	NextSteps      []string         `json:"nextSteps"`
}

type LaneSummary struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Kind      string `json:"kind"`
	Status    string `json:"status"`
	Workspace string `json:"workspace"`
	Authority bool   `json:"authority"`
}

type FactCounts struct {
	Observations     int `json:"observations"`
	Requests         int `json:"requests"`
	Candidates       int `json:"candidates"`
	Publications     int `json:"publications"`
	PendingDecisions int `json:"pendingDecisions"`
}

type OverviewSections struct {
	OpenCandidates    EventSection `json:"openCandidates"`
	PendingGates      EventSection `json:"pendingGates"`
	Verifications     EventSection `json:"verifications"`
	Decisions         EventSection `json:"decisions"`
	Batches           BatchSection `json:"batches"`
	OpenInterventions EventSection `json:"openInterventions"`
	Interventions     EventSection `json:"interventions"`
	Rollbacks         EventSection `json:"rollbacks"`
}

type EventSection struct {
	Total  int              `json:"total"`
	Shown  int              `json:"shown"`
	Events []map[string]any `json:"events"`
}

type BatchSection struct {
	Total   int            `json:"total"`
	Shown   int            `json:"shown"`
	Batches []BatchSummary `json:"batches"`
}

type BatchSummary struct {
	ID     string         `json:"id"`
	Events int            `json:"events"`
	Kinds  map[string]int `json:"kinds"`
	Last   string         `json:"last"`
}

type overviewData struct {
	inst        instance.Instance
	board       event
	facts       factSet
	lanes       []event
	pending     int
	sections    OverviewSections
	initialized bool
}

type batchSummary struct {
	id       string
	events   []event
	lastTime string
	lastIdx  int
}

type factSet struct {
	Observations   []event
	Candidates     []event
	Requests       []event
	Publications   []event
	Decisions      []event
	Hypotheses     []event
	Verifications  []event
	Interventions  []event
	Rollbacks      []event
	AllBatchEvents []event
}

func Render(repoRoot, caseRoot, pack string) (string, error) {
	data, err := loadOverviewData(repoRoot, caseRoot, pack)
	if err != nil {
		return "", err
	}
	facts := data.facts

	var out bytes.Buffer
	fmt.Fprintf(&out, "项目概览：%s\n", data.inst.CaseRoot)
	fmt.Fprintf(&out, "自动化模式：%s\n", stringValue(data.board, "automationMode"))
	fmt.Fprintln(&out, "当前是项目总览，还没有为本会话选择具体工作线。")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "工作线：")
	for _, lane := range data.lanes {
		kind := "功能/工具支线"
		if boolValue(lane, "authority") {
			kind = "主线"
		}
		fmt.Fprintf(&out, "- %s：%s，选择名=%s，状态=%s，工作区=%s\n", kind, stringValue(lane, "id"), workstreamLabel(lane), stringValue(lane, "status"), stringValue(lane, "workspace"))
	}
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "共享事实：")
	fmt.Fprintf(&out, "- observation: %d\n", len(facts.Observations))
	fmt.Fprintf(&out, "- request: %d\n", len(facts.Requests))
	fmt.Fprintf(&out, "- candidate: %d\n", len(facts.Candidates))
	fmt.Fprintf(&out, "- publication: %d\n", len(facts.Publications))
	fmt.Fprintf(&out, "- 需要确认: %d\n", data.pending)
	fmt.Fprintln(&out)

	writeOpenCandidates(&out, facts.Candidates)
	writePendingGates(&out, facts.Requests)
	writeVerifications(&out, facts.Verifications)
	writeDecisions(&out, facts.Decisions)
	writeBatches(&out, facts.AllBatchEvents)
	writeInterventions(&out, facts.Interventions)
	writeRollbacks(&out, facts.Rollbacks)
	writeNextSteps(&out, data.lanes)
	return out.String(), nil
}

func BuildInventory(repoRoot, caseRoot, pack string) (Inventory, error) {
	data, err := loadOverviewData(repoRoot, caseRoot, pack)
	if err != nil {
		return Inventory{}, err
	}
	lanes := make([]LaneSummary, 0, len(data.lanes))
	for _, lane := range data.lanes {
		kind := "feature"
		if boolValue(lane, "authority") {
			kind = "main"
		}
		lanes = append(lanes, LaneSummary{
			ID:        stringValue(lane, "id"),
			Label:     workstreamLabel(lane),
			Kind:      kind,
			Status:    stringValue(lane, "status"),
			Workspace: stringValue(lane, "workspace"),
			Authority: boolValue(lane, "authority"),
		})
	}
	facts := data.facts
	return Inventory{
		SchemaVersion:  1,
		Command:        "overview",
		CaseRoot:       data.inst.CaseRoot,
		RepoRoot:       repoRoot,
		Pack:           pack,
		IsMutation:     data.initialized,
		AutomationMode: stringValue(data.board, "automationMode"),
		Lanes:          lanes,
		Counts: FactCounts{
			Observations:     len(facts.Observations),
			Requests:         len(facts.Requests),
			Candidates:       len(facts.Candidates),
			Publications:     len(facts.Publications),
			PendingDecisions: data.pending,
		},
		Sections:  data.sections,
		NextSteps: nextStepCommands(data.lanes),
	}, nil
}

func loadOverviewData(repoRoot, caseRoot, pack string) (overviewData, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return overviewData{}, err
	}
	boardPath := filepath.Join(inst.CaseRoot, ".rekit", "board.json")
	board, err := readJSONObject(boardPath)
	initialized := false
	if os.IsNotExist(err) {
		if err := workstream.EnsureBoard(repoRoot, inst.CaseRoot, pack); err != nil {
			return overviewData{}, err
		}
		initialized = true
		board, err = readJSONObject(boardPath)
	}
	if err != nil {
		return overviewData{}, err
	}
	facts, err := readFacts(inst.CaseRoot)
	if err != nil {
		return overviewData{}, err
	}
	lanes := laneList(board["lanes"])
	return overviewData{
		inst:        inst,
		board:       board,
		facts:       facts,
		lanes:       lanes,
		pending:     pendingDecisions(facts.Decisions),
		sections:    buildOverviewSections(facts),
		initialized: initialized,
	}, nil
}

func buildOverviewSections(facts factSet) OverviewSections {
	openCandidates := openStatusEvents(facts.Candidates)
	pendingGates := []event{}
	for _, request := range facts.Requests {
		if stringValue(request, "status") == "pending-gate" {
			pendingGates = append(pendingGates, request)
		}
	}
	openInterventions := openStatusEvents(facts.Interventions)
	return OverviewSections{
		OpenCandidates:    newEventSection(openCandidates),
		PendingGates:      newEventSection(pendingGates),
		Verifications:     newEventSection(facts.Verifications),
		Decisions:         newEventSection(facts.Decisions),
		Batches:           newBatchSection(facts.AllBatchEvents),
		OpenInterventions: newEventSection(openInterventions),
		Interventions:     newEventSection(facts.Interventions),
		Rollbacks:         newEventSection(facts.Rollbacks),
	}
}

func newEventSection(items []event) EventSection {
	shown := lastEvents(items, maxRows)
	return EventSection{Total: len(items), Shown: len(shown), Events: cloneEvents(shown)}
}

func openStatusEvents(items []event) []event {
	terminal := map[string]bool{"confirmed": true, "accepted": true, "rejected": true, "superseded": true, "resolved": true}
	open := []event{}
	for _, item := range items {
		status := strings.ToLower(strings.TrimSpace(stringValue(item, "status")))
		if status == "" || !terminal[status] {
			open = append(open, item)
		}
	}
	return open
}

func newBatchSection(events []event) BatchSection {
	batches := batchSummaries(events)
	shown := batches
	if len(shown) > maxRows {
		shown = shown[len(shown)-maxRows:]
	}
	out := make([]BatchSummary, 0, len(shown))
	for _, batch := range shown {
		out = append(out, BatchSummary{ID: batch.id, Events: len(batch.events), Kinds: kindCounts(batch.events), Last: batch.lastTime})
	}
	return BatchSection{Total: len(batches), Shown: len(shown), Batches: out}
}

func batchSummaries(events []event) []batchSummary {
	byID := map[string]*batchSummary{}
	order := []*batchSummary{}
	for idx, e := range events {
		id := stringValue(e, "batchId")
		if id == "" {
			continue
		}
		b := byID[id]
		if b == nil {
			b = &batchSummary{id: id}
			byID[id] = b
			order = append(order, b)
		}
		b.events = append(b.events, e)
		b.lastIdx = idx
		b.lastTime = stringValue(e, "time")
		if b.lastTime == "" {
			b.lastTime = stringValue(e, "createdAt")
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		if order[i].lastTime == order[j].lastTime {
			return order[i].lastIdx < order[j].lastIdx
		}
		return order[i].lastTime < order[j].lastTime
	})
	out := make([]batchSummary, 0, len(order))
	for _, item := range order {
		out = append(out, *item)
	}
	return out
}

func kindCounts(events []event) map[string]int {
	counts := map[string]int{}
	for _, e := range events {
		kind := stringValue(e, "kind")
		if kind == "" {
			kind = "unknown"
		}
		counts[kind]++
	}
	return counts
}

func cloneEvents(items []event) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, maps.Clone(map[string]any(item)))
	}
	return out
}

func nextStepCommands(lanes []event) []string {
	open := openLanes(lanes)
	commands := []string{}
	if len(open) == 1 {
		commands = append(commands, "/rekit continue "+workstreamLabel(open[0]))
	} else {
		for _, lane := range open {
			commands = append(commands, "/rekit continue "+workstreamLabel(lane))
		}
	}
	commands = append(commands, "/rekit start <name>", "/rekit handoff", "/rekit handoff main 或 /rekit handoff <name>")
	return commands
}

func openLanes(lanes []event) []event {
	open := []event{}
	for _, lane := range lanes {
		status := strings.ToLower(stringValue(lane, "status"))
		if status != "archived" && status != "paused" && status != "closed" {
			open = append(open, lane)
		}
	}
	return open
}

func readFacts(caseRoot string) (factSet, error) {
	factsRoot := filepath.Join(caseRoot, ".rekit", "facts")
	read := func(name string) ([]event, error) { return readJSONLines(filepath.Join(factsRoot, name)) }
	var err error
	facts := factSet{}
	if facts.Observations, err = read("observations.jsonl"); err != nil {
		return facts, err
	}
	if facts.Candidates, err = read("candidates.jsonl"); err != nil {
		return facts, err
	}
	if facts.Requests, err = read("requests.jsonl"); err != nil {
		return facts, err
	}
	if facts.Publications, err = read("publications.jsonl"); err != nil {
		return facts, err
	}
	if facts.Decisions, err = read("decisions.jsonl"); err != nil {
		return facts, err
	}
	if facts.Hypotheses, err = read("hypotheses.jsonl"); err != nil {
		return facts, err
	}
	if facts.Verifications, err = read("verifications.jsonl"); err != nil {
		return facts, err
	}
	if facts.Interventions, err = read("interventions.jsonl"); err != nil {
		return facts, err
	}
	if facts.Rollbacks, err = read("rollbacks.jsonl"); err != nil {
		return facts, err
	}
	for _, list := range [][]event{facts.Observations, facts.Hypotheses, facts.Candidates, facts.Verifications, facts.Decisions, facts.Interventions, facts.Rollbacks, facts.Publications, facts.Requests} {
		for _, e := range list {
			if strings.TrimSpace(stringValue(e, "batchId")) != "" {
				facts.AllBatchEvents = append(facts.AllBatchEvents, e)
			}
		}
	}
	return facts, nil
}

func writeOpenCandidates(out *bytes.Buffer, candidates []event) {
	terminal := map[string]bool{"confirmed": true, "accepted": true, "rejected": true, "superseded": true, "resolved": true}
	open := []event{}
	counts := map[string]int{}
	for _, c := range candidates {
		status := strings.ToLower(strings.TrimSpace(stringValue(c, "status")))
		if status == "" || !terminal[status] {
			open = append(open, c)
			counts[stringValue(c, "subject")]++
		}
	}
	if len(open) == 0 {
		return
	}
	fmt.Fprintln(out, "未决 candidate：")
	shown := lastEvents(open, maxRows)
	for _, c := range shown {
		mark := ""
		if counts[stringValue(c, "subject")] > 1 {
			mark = " [冲突]"
		}
		fmt.Fprintf(out, "- %s | %s | confidence=%s%s\n", stringValue(c, "subject"), stringValue(c, "summary"), stringValue(c, "confidence"), mark)
	}
	if rest := len(open) - len(shown); rest > 0 {
		fmt.Fprintf(out, "- 另有 %d 条未决 candidate\n", rest)
	}
	fmt.Fprintln(out)
}

func writePendingGates(out *bytes.Buffer, requests []event) {
	pending := []event{}
	for _, r := range requests {
		if stringValue(r, "status") == "pending-gate" {
			pending = append(pending, r)
		}
	}
	if len(pending) == 0 {
		return
	}
	fmt.Fprintln(out, "pending-gate（heavy-tool 待确认）：")
	shown := lastEvents(pending, maxRows)
	for _, g := range shown {
		fmt.Fprintf(out, "- %s | %s%s\n", stringValue(g, "subject"), stringValue(g, "summary"), gateDetail(g, true, false))
	}
	if rest := len(pending) - len(shown); rest > 0 {
		fmt.Fprintf(out, "- 另有 %d 条 pending-gate\n", rest)
	}
	fmt.Fprintln(out)
}

func writeVerifications(out *bytes.Buffer, verifications []event) {
	if len(verifications) == 0 {
		return
	}
	fmt.Fprintln(out, "最近 verification：")
	shown := lastEvents(verifications, maxRows)
	for _, v := range shown {
		subject := stringValue(v, "subject")
		if strings.TrimSpace(subject) == "" {
			subject = stringValue(v, "kind")
		}
		by := ""
		if actor := stringValue(v, "actor"); strings.TrimSpace(actor) != "" {
			by = " | by=" + actor
		}
		fmt.Fprintf(out, "- %s | lane=%s | verifier=%s | verdict=%s | target=%s%s%s\n", subject, stringValue(v, "lane"), stringValue(v, "verifier"), stringValue(v, "verdict"), stringValue(v, "target"), by, batchTag(v))
	}
	if rest := len(verifications) - len(shown); rest > 0 {
		fmt.Fprintf(out, "- 另有 %d 条 verification\n", rest)
	}
	fmt.Fprintln(out)
}

func writeDecisions(out *bytes.Buffer, decisions []event) {
	if len(decisions) == 0 {
		return
	}
	fmt.Fprintln(out, "最近 decision：")
	shown := lastEvents(decisions, maxRows)
	for _, d := range shown {
		subject := stringValue(d, "subject")
		if strings.TrimSpace(subject) == "" {
			subject = stringValue(d, "kind")
		}
		decision := stringValue(d, "decision")
		if strings.TrimSpace(decision) == "" {
			decision = stringValue(d, "action")
		}
		actor := stringValue(d, "actor")
		if strings.TrimSpace(actor) == "" {
			actor = stringValue(d, "confirmedBy")
		}
		extra := ""
		if strings.TrimSpace(actor) != "" {
			extra = " | by=" + actor
		}
		fmt.Fprintf(out, "- %s | lane=%s | decision=%s%s | reason=%s\n", subject, stringValue(d, "lane"), decision, extra, stringValue(d, "reason"))
	}
	if rest := len(decisions) - len(shown); rest > 0 {
		fmt.Fprintf(out, "- 另有 %d 条 decision\n", rest)
	}
	fmt.Fprintln(out)
}

func writeBatches(out *bytes.Buffer, events []event) {
	if len(events) == 0 {
		return
	}
	type batch struct {
		id       string
		events   []event
		lastTime string
		lastIdx  int
	}
	byID := map[string]*batch{}
	order := []*batch{}
	for idx, e := range events {
		id := stringValue(e, "batchId")
		if id == "" {
			continue
		}
		b := byID[id]
		if b == nil {
			b = &batch{id: id}
			byID[id] = b
			order = append(order, b)
		}
		b.events = append(b.events, e)
		b.lastIdx = idx
		b.lastTime = stringValue(e, "time")
		if b.lastTime == "" {
			b.lastTime = stringValue(e, "createdAt")
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		if order[i].lastTime == order[j].lastTime {
			return order[i].lastIdx < order[j].lastIdx
		}
		return order[i].lastTime < order[j].lastTime
	})
	shown := order
	if len(shown) > maxRows {
		shown = shown[len(shown)-maxRows:]
	}
	fmt.Fprintln(out, "最近 batch：")
	for _, b := range shown {
		fmt.Fprintf(out, "- %s | events=%d | kinds=%s | last=%s\n", b.id, len(b.events), kindSummary(b.events), b.lastTime)
	}
	if rest := len(order) - len(shown); rest > 0 {
		fmt.Fprintf(out, "- 另有 %d 个 batch\n", rest)
	}
	fmt.Fprintln(out)
}

func writeInterventions(out *bytes.Buffer, interventions []event) {
	terminal := map[string]bool{"confirmed": true, "accepted": true, "rejected": true, "superseded": true, "resolved": true}
	open := []event{}
	for _, i := range interventions {
		status := strings.ToLower(strings.TrimSpace(stringValue(i, "status")))
		if status == "" || !terminal[status] {
			open = append(open, i)
		}
	}
	if len(open) > 0 {
		fmt.Fprintln(out, "未解决 intervention：")
		shown := lastEvents(open, maxRows)
		for _, i := range shown {
			subject := stringValue(i, "subject")
			if strings.TrimSpace(subject) == "" {
				subject = stringValue(i, "action")
			}
			status := stringValue(i, "status")
			if strings.TrimSpace(status) == "" {
				status = "open"
			}
			batch := batchTag(i)
			fmt.Fprintf(out, "- %s | action=%s | target=%s | status=%s%s | summary=%s\n", subject, stringValue(i, "action"), stringValue(i, "target"), status, batch, stringValue(i, "summary"))
		}
		if rest := len(open) - len(shown); rest > 0 {
			fmt.Fprintf(out, "- 另有 %d 条未解决 intervention\n", rest)
		}
		fmt.Fprintln(out)
	}
	if len(interventions) == 0 {
		return
	}
	fmt.Fprintln(out, "最近 intervention：")
	shown := lastEvents(interventions, maxRows)
	for _, i := range shown {
		subject := stringValue(i, "subject")
		if strings.TrimSpace(subject) == "" {
			subject = stringValue(i, "action")
		}
		fmt.Fprintf(out, "- %s | action=%s | target=%s | approvedBy=%s | scope=%s%s\n", subject, stringValue(i, "action"), stringValue(i, "target"), stringValue(i, "approvedBy"), stringValue(i, "scope"), batchTag(i))
	}
	if rest := len(interventions) - len(shown); rest > 0 {
		fmt.Fprintf(out, "- 另有 %d 条 intervention\n", rest)
	}
	fmt.Fprintln(out)
}

func writeRollbacks(out *bytes.Buffer, rollbacks []event) {
	if len(rollbacks) == 0 {
		return
	}
	fmt.Fprintln(out, "最近 rollback：")
	shown := lastEvents(rollbacks, maxRows)
	for _, r := range shown {
		subject := stringValue(r, "subject")
		if strings.TrimSpace(subject) == "" {
			subject = stringValue(r, "kind")
		}
		fmt.Fprintf(out, "- %s | target=%s | status=%s%s | reason=%s\n", subject, stringValue(r, "target"), stringValue(r, "status"), batchTag(r), stringValue(r, "reason"))
	}
	if rest := len(rollbacks) - len(shown); rest > 0 {
		fmt.Fprintf(out, "- 另有 %d 条 rollback\n", rest)
	}
	fmt.Fprintln(out)
}

func writeNextSteps(out *bytes.Buffer, lanes []event) {
	fmt.Fprintln(out, "建议下一步：")
	open := []event{}
	for _, lane := range lanes {
		status := strings.ToLower(stringValue(lane, "status"))
		if status != "archived" && status != "paused" && status != "closed" {
			open = append(open, lane)
		}
	}
	if len(open) == 1 {
		fmt.Fprintf(out, "- 接手当前工作线：/rekit continue %s\n", workstreamLabel(open[0]))
	} else {
		for _, lane := range open {
			kind := "功能支线"
			if boolValue(lane, "authority") {
				kind = "主线"
			}
			fmt.Fprintf(out, "- 接手%s %s：/rekit continue %s\n", kind, stringValue(lane, "id"), workstreamLabel(lane))
		}
	}
	fmt.Fprintln(out, "- 创建或进入功能支线：/rekit start <name>")
	fmt.Fprintln(out, "- 生成项目级接手索引：/rekit handoff")
	fmt.Fprintln(out, "- 生成指定工作线接手文档：/rekit handoff main 或 /rekit handoff <name>")
}

func pendingDecisions(decisions []event) int {
	pending := 0
	terminalStatus := map[string]bool{"confirmed": true, "accepted": true, "rejected": true, "resolved": true, "deferred": true, "superseded": true}
	for _, d := range decisions {
		decision := stringValue(d, "decision")
		status := stringValue(d, "status")
		action := stringValue(d, "action")
		if action == "pending-user" || (status == "" && decision == "defer") || (status != "" && !terminalStatus[status]) {
			pending++
		}
	}
	return pending
}

func readJSONObject(path string) (event, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out event
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("invalid JSON %s: %w", path, err)
	}
	return out, nil
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

func laneList(v any) []event {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := []event{}
	for _, item := range items {
		if lane, ok := item.(map[string]any); ok {
			out = append(out, lane)
		}
	}
	return out
}

func lastEvents(items []event, n int) []event {
	if len(items) <= n {
		return items
	}
	return items[len(items)-n:]
}

func kindSummary(events []event) string {
	counts := map[string]int{}
	keys := []string{}
	for _, e := range events {
		kind := stringValue(e, "kind")
		if kind == "" {
			kind = "unknown"
		}
		if _, ok := counts[kind]; !ok {
			keys = append(keys, kind)
		}
		counts[kind]++
	}
	sort.Strings(keys)
	parts := []string{}
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, counts[key]))
	}
	return strings.Join(parts, ",")
}

func gateDetail(e event, omitStatus, omitBatch bool) string {
	parts := []string{}
	add := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			parts = append(parts, key+"="+value)
		}
	}
	if !omitStatus {
		add("status", stringValue(e, "status"))
	}
	add("by", stringValue(e, "actor"))
	add("risk", stringValue(e, "risk"))
	add("target", stringValue(e, "target"))
	if !omitBatch {
		add("batch", stringValue(e, "batchId"))
	}
	if gate, ok := e["gate"].(map[string]any); ok {
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

func batchTag(e event) string {
	batch := stringValue(e, "batchId")
	if strings.TrimSpace(batch) == "" {
		return ""
	}
	return " | batch=" + batch
}

func workstreamLabel(lane event) string {
	if boolValue(lane, "authority") {
		return "main"
	}
	id := stringValue(lane, "id")
	if name, ok := strings.CutPrefix(id, "feature-"); ok {
		return name
	}
	return id
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

func boolValue(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(strings.TrimSpace(t), "true")
	default:
		return false
	}
}
