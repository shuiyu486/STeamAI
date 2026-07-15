package workstream

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

const maxHandoffRows = 5

type HandoffOptions struct {
	Selector string
}

type HandoffResult struct {
	SchemaVersion        int           `json:"schemaVersion"`
	Command              string        `json:"command"`
	CaseRoot             string        `json:"caseRoot"`
	RepoRoot             string        `json:"repoRoot"`
	Pack                 string        `json:"pack"`
	IsMutation           bool          `json:"isMutation"`
	Applied              bool          `json:"applied"`
	RequiresConfirmation bool          `json:"requiresConfirmation"`
	Selector             string        `json:"selector,omitempty"`
	Project              bool          `json:"project"`
	Lane                 *Lane         `json:"lane,omitempty"`
	MissionBrief         mission.Brief `json:"missionBrief"`
	Writes               []StartWrite  `json:"writes"`
	BlockedActions       []string      `json:"blockedActions"`
	NextSteps            []string      `json:"nextSteps"`
}

func HandoffPreview(repoRoot, caseRoot, pack string, opt HandoffOptions) (HandoffResult, error) {
	ctx, err := newHandoffContext(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return HandoffResult{}, err
	}
	writes, err := ctx.plannedWrites(false)
	if err != nil {
		return HandoffResult{}, err
	}
	return ctx.result(false, false, true, writes), nil
}

func HandoffApply(repoRoot, caseRoot, pack string, opt HandoffOptions) (HandoffResult, error) {
	ctx, err := newHandoffContext(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return HandoffResult{}, err
	}
	writes, err := ctx.write()
	if err != nil {
		return HandoffResult{}, err
	}
	return ctx.result(true, true, false, writes), nil
}

type handoffContext struct {
	inst      instance.Instance
	manifest  *manifest.Manifest
	board     board
	selector  string
	project   bool
	lane      *Lane
	stamp     string
	handovers string
}

func newHandoffContext(repoRoot, caseRoot, pack string, opt HandoffOptions) (handoffContext, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return handoffContext{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return handoffContext{}, err
	}
	b, err := readBoard(inst.CaseRoot)
	if os.IsNotExist(err) {
		return handoffContext{}, fmt.Errorf("handoff requires existing .rekit/board.json; run start -Apply or PowerShell /rekit overview once to initialize the case-local board")
	}
	if err != nil {
		return handoffContext{}, err
	}
	if strings.TrimSpace(b.DefaultAuthorityLane) == "" {
		b.DefaultAuthorityLane = m.WorkstreamDefaults["defaultAuthorityLane"]
	}
	selector := strings.TrimSpace(opt.Selector)
	ctx := handoffContext{inst: inst, manifest: m, board: b, selector: selector, project: selector == "", stamp: handoffTimestamp()}
	ctx.handovers, err = refsf.SafeJoin(inst.CaseRoot, relJoin(".rekit", "handovers"))
	if err != nil {
		return handoffContext{}, err
	}
	if !ctx.project {
		lane, err := resolveHandoffLane(inst.CaseRoot, b, selector)
		if err != nil {
			return handoffContext{}, err
		}
		ctx.lane = &lane
	}
	return ctx, nil
}

func (ctx handoffContext) result(mutating, applied, confirm bool, writes []StartWrite) HandoffResult {
	var lane *Lane
	if ctx.lane != nil {
		copyLane := *ctx.lane
		lane = &copyLane
	}
	brief, err := ctx.missionBrief()
	if err != nil {
		brief = mission.Brief{Summary: "unavailable: " + err.Error()}
	}
	next := []string{"PowerShell /rekit remains the public entrypoint; JSON preview/apply is Go-owned by default"}
	if applied {
		if ctx.project {
			next = append(next, "open .rekit/handovers/latest.md in the case to continue")
		} else if lane != nil {
			next = append(next, "use /rekit continue "+workstreamLabel(*lane)+" to enter the lane workflow")
		}
	} else {
		next = append(next, "review this plan, then re-run handoff with -Apply to write case-local handoff files")
	}
	return HandoffResult{
		SchemaVersion:        1,
		Command:              "handoff",
		CaseRoot:             ctx.inst.CaseRoot,
		RepoRoot:             ctx.manifest.RepoRoot,
		Pack:                 ctx.manifest.Pack,
		IsMutation:           mutating,
		Applied:              applied,
		RequiresConfirmation: confirm,
		Selector:             ctx.selector,
		Project:              ctx.project,
		Lane:                 lane,
		MissionBrief:         brief,
		Writes:               writes,
		BlockedActions:       []string{"authority/confirmed writes", "heavy-tool execution", "continue auto-apply", "board/facts/lane creation"},
		NextSteps:            next,
	}
}

func (ctx handoffContext) missionBrief() (mission.Brief, error) {
	facts, err := readHandoffFacts(ctx.inst.CaseRoot)
	if err != nil {
		return mission.Brief{}, err
	}
	if ctx.project {
		return projectMissionBrief(ctx.board.Lanes, facts), nil
	}
	if ctx.lane == nil {
		return mission.Brief{}, nil
	}
	return laneMissionBrief(*ctx.lane, facts), nil
}

func (ctx handoffContext) plannedWrites(apply bool) ([]StartWrite, error) {
	if ctx.project {
		return ctx.projectWrites(apply)
	}
	return ctx.laneWrites(apply, *ctx.lane)
}

func (ctx handoffContext) write() ([]StartWrite, error) {
	if ctx.project {
		text, writes, err := ctx.renderProject(true)
		if err != nil {
			return nil, err
		}
		stampPath, latestPath, err := ctx.projectHandoffPaths()
		if err != nil {
			return nil, err
		}
		if err := writeText(stampPath, text); err != nil {
			return nil, err
		}
		if err := writeText(latestPath, text); err != nil {
			return nil, err
		}
		writes = append(writes, StartWrite{Path: relativePath(ctx.inst.CaseRoot, stampPath), Kind: "handoff", Action: "write-project-handoff", TargetPath: stampPath})
		writes = append(writes, StartWrite{Path: relativePath(ctx.inst.CaseRoot, latestPath), Kind: "handoff", Action: "write-latest-project-handoff", TargetPath: latestPath})
		return writes, nil
	}
	text, writes, err := ctx.renderLane(*ctx.lane, true)
	if err != nil {
		return nil, err
	}
	stampPath, latestPath, err := ctx.laneHandoffPaths(ctx.lane.ID)
	if err != nil {
		return nil, err
	}
	if err := writeText(stampPath, text); err != nil {
		return nil, err
	}
	if err := writeText(latestPath, text); err != nil {
		return nil, err
	}
	writes = append(writes, StartWrite{Path: relativePath(ctx.inst.CaseRoot, stampPath), Kind: "handoff", Action: "write-lane-handoff", TargetPath: stampPath})
	writes = append(writes, StartWrite{Path: relativePath(ctx.inst.CaseRoot, latestPath), Kind: "handoff", Action: "write-latest-lane-handoff", TargetPath: latestPath})
	return writes, nil
}

func (ctx handoffContext) projectWrites(apply bool) ([]StartWrite, error) {
	_, writes, err := ctx.renderProject(apply)
	if err != nil {
		return nil, err
	}
	stampPath, latestPath, err := ctx.projectHandoffPaths()
	if err != nil {
		return nil, err
	}
	prefix := "would-"
	if apply {
		prefix = ""
	}
	writes = append(writes, StartWrite{Path: relativePath(ctx.inst.CaseRoot, stampPath), Kind: "handoff", Action: prefix + "write-project-handoff", TargetPath: stampPath})
	writes = append(writes, StartWrite{Path: relativePath(ctx.inst.CaseRoot, latestPath), Kind: "handoff", Action: prefix + "write-latest-project-handoff", TargetPath: latestPath})
	return writes, nil
}

func (ctx handoffContext) laneWrites(apply bool, lane Lane) ([]StartWrite, error) {
	_, writes, err := ctx.renderLane(lane, apply)
	if err != nil {
		return nil, err
	}
	stampPath, latestPath, err := ctx.laneHandoffPaths(lane.ID)
	if err != nil {
		return nil, err
	}
	prefix := "would-"
	if apply {
		prefix = ""
	}
	writes = append(writes, StartWrite{Path: relativePath(ctx.inst.CaseRoot, stampPath), Kind: "handoff", Action: prefix + "write-lane-handoff", TargetPath: stampPath})
	writes = append(writes, StartWrite{Path: relativePath(ctx.inst.CaseRoot, latestPath), Kind: "handoff", Action: prefix + "write-latest-lane-handoff", TargetPath: latestPath})
	return writes, nil
}

func (ctx handoffContext) renderProject(apply bool) (string, []StartWrite, error) {
	writes := []StartWrite{}
	facts, err := readHandoffFacts(ctx.inst.CaseRoot)
	if err != nil {
		return "", nil, err
	}
	handoffRel := strings.TrimSpace(ctx.manifest.WorkstreamDefaults["handoffPath"])
	taskHandoff := ""
	if handoffRel != "" {
		path, err := refsf.SafeJoin(ctx.inst.CaseRoot, handoffRel)
		if err != nil {
			return "", nil, err
		}
		if refsf.Exists(path) {
			taskHandoff = relativePath(ctx.inst.CaseRoot, path)
		}
	}
	latestDigest, err := latestRunDigest(ctx.inst.CaseRoot)
	if err != nil {
		return "", nil, err
	}
	var out bytes.Buffer
	fmt.Fprintln(&out, "# rekit 项目接手索引")
	fmt.Fprintln(&out)
	fmt.Fprintf(&out, "生成时间：%s\n", isoNow())
	fmt.Fprintf(&out, "case：%s\n", ctx.inst.CaseRoot)
	fmt.Fprintf(&out, "pack：%s\n", ctx.manifest.Pack)
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "## 说明")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "这是项目级索引，不代表某个会话已经选择主线或支线。新会话应先选择要接手的工作线。")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "## 推荐读取")
	fmt.Fprintln(&out)
	if taskHandoff != "" {
		fmt.Fprintf(&out, "- `%s`：case 长期主线 handoff。\n", taskHandoff)
	}
	if latestDigest != "" {
		fmt.Fprintf(&out, "- `%s`：最近一次自动整理摘要。\n", latestDigest)
	}
	fmt.Fprintln(&out)
	writeProjectMissionBrief(&out, ctx.board.Lanes, facts)
	fmt.Fprintln(&out, "## 工作线")
	fmt.Fprintln(&out)
	for _, row := range ctx.board.Lanes {
		lane, err := readLaneByID(ctx.inst.CaseRoot, row.ID)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", nil, err
		}
		resumeRel, resumeWrites, err := ctx.refreshResume(lane, apply)
		if err != nil {
			return "", nil, err
		}
		writes = append(writes, resumeWrites...)
		kind := "功能支线"
		if lane.Authority {
			kind = "主线"
		}
		label := workstreamLabel(lane)
		fmt.Fprintf(&out, "- %s `%s`：status=%s，workspace=`%s`\n", kind, lane.ID, lane.Status, lane.Workspace)
		fmt.Fprintf(&out, "  - 接手：`/rekit continue %s`\n", label)
		fmt.Fprintf(&out, "  - 指定交接：`/rekit handoff %s`\n", label)
		fmt.Fprintf(&out, "  - 接续提示：`%s`\n", resumeRel)
	}
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "## 注意边界")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "- 主线负责最终结论写入；功能支线只写自己的工作区、证据、候选和请求。")
	if handoffRel != "" {
		fmt.Fprintf(&out, "- 本索引不会覆盖 `%s`，只引用它。\n", handoffRel)
	}
	fmt.Fprintln(&out, "- 多工作线时不要使用无参数 `/rekit continue` 盲目继续，应使用 `/rekit continue main` 或 `/rekit continue <name>`。")
	return out.String(), writes, nil
}

func (ctx handoffContext) renderLane(lane Lane, apply bool) (string, []StartWrite, error) {
	writes := []StartWrite{}
	resumeRel, resumeWrites, err := ctx.refreshResume(lane, apply)
	if err != nil {
		return "", nil, err
	}
	writes = append(writes, resumeWrites...)
	latestDigest, err := latestRunDigest(ctx.inst.CaseRoot)
	if err != nil {
		return "", nil, err
	}
	_, latestPath, err := ctx.laneHandoffPaths(lane.ID)
	if err != nil {
		return "", nil, err
	}
	kind := "功能支线"
	if lane.Authority {
		kind = "主线"
	}
	label := workstreamLabel(lane)
	var out bytes.Buffer
	fmt.Fprintf(&out, "# rekit 工作线接手：%s\n", lane.ID)
	fmt.Fprintln(&out)
	fmt.Fprintf(&out, "生成时间：%s\n", isoNow())
	fmt.Fprintf(&out, "case：%s\n", ctx.inst.CaseRoot)
	fmt.Fprintf(&out, "pack：%s\n", ctx.manifest.Pack)
	fmt.Fprintf(&out, "类型：%s\n", kind)
	fmt.Fprintf(&out, "状态：%s\n", lane.Status)
	fmt.Fprintf(&out, "工作区：%s\n", lane.Workspace)
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "## 新会话开场")
	fmt.Fprintln(&out)
	fmt.Fprintf(&out, "直接说：按 `%s` 接手，然后执行 `/rekit continue %s`。\n", relativePath(ctx.inst.CaseRoot, latestPath), label)
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "## 推荐读取")
	fmt.Fprintln(&out)
	fmt.Fprintf(&out, "- `%s`：本工作线接续提示。\n", resumeRel)
	if lane.Authority {
		if rel := strings.TrimSpace(ctx.manifest.WorkstreamDefaults["handoffPath"]); rel != "" {
			path, err := refsf.SafeJoin(ctx.inst.CaseRoot, rel)
			if err != nil {
				return "", nil, err
			}
			if refsf.Exists(path) {
				fmt.Fprintf(&out, "- `%s`：case 长期主线 handoff。\n", relativePath(ctx.inst.CaseRoot, path))
			}
		}
	}
	if latestDigest != "" {
		fmt.Fprintf(&out, "- `%s`：最近一次自动整理摘要。\n", latestDigest)
	}
	fmt.Fprintln(&out)
	if err := writeWorkspacePackets(&out, ctx.inst.CaseRoot, lane); err != nil {
		return "", nil, err
	}
	facts, err := readHandoffFacts(ctx.inst.CaseRoot)
	if err != nil {
		return "", nil, err
	}
	writeLaneMissionBrief(&out, lane, facts)
	writeVerificationSection(&out, facts.Verifications, lane.ID)
	writeDecisionSection(&out, facts.Decisions, lane.ID)
	writePendingGateSection(&out, facts.Requests, lane.ID)
	writeInterventionSection(&out, facts.Interventions, lane.ID)
	writeRollbackSection(&out, facts.Rollbacks, lane.ID)
	fmt.Fprintln(&out, "## 边界")
	fmt.Fprintln(&out)
	if lane.Authority {
		fmt.Fprintln(&out, "- 这是主线；可以维护最终结论、验证和长期 handoff。")
	} else {
		fmt.Fprintln(&out, "- 这是功能支线；只写自己的 workspace、证据、候选和 request。")
	}
	fmt.Fprintln(&out, "- 不并发写 IDB 注释/rename/type；不把完整 trace、disasm、decompile、dump 内容复制进 Markdown。")
	return out.String(), writes, nil
}

func (ctx handoffContext) refreshResume(lane Lane, apply bool) (string, []StartWrite, error) {
	resumePath, checkpointPath, err := laneResumePaths(ctx.inst.CaseRoot, lane)
	if err != nil {
		return "", nil, err
	}
	actionPrefix := "would-"
	if apply {
		resumePath, checkpointPath, err = writeLaneResume(ctx.inst.CaseRoot, lane)
		if err != nil {
			return "", nil, err
		}
		actionPrefix = ""
	}
	writes := []StartWrite{
		{Path: relativePath(ctx.inst.CaseRoot, resumePath), Kind: "lane-resume", Action: actionPrefix + "refresh", TargetPath: resumePath},
		{Path: relativePath(ctx.inst.CaseRoot, checkpointPath), Kind: "lane-checkpoint", Action: actionPrefix + "refresh", TargetPath: checkpointPath},
	}
	return relativePath(ctx.inst.CaseRoot, resumePath), writes, nil
}

func (ctx handoffContext) projectHandoffPaths() (string, string, error) {
	stampPath, err := refsf.SafeJoin(ctx.handovers, ctx.stamp+".md")
	if err != nil {
		return "", "", err
	}
	latestPath, err := refsf.SafeJoin(ctx.handovers, "latest.md")
	if err != nil {
		return "", "", err
	}
	return stampPath, latestPath, nil
}

func (ctx handoffContext) laneHandoffPaths(laneID string) (string, string, error) {
	if err := validateLaneIDSegment(laneID); err != nil {
		return "", "", err
	}
	stampPath, err := refsf.SafeJoin(ctx.handovers, laneID+"-"+ctx.stamp+".md")
	if err != nil {
		return "", "", err
	}
	latestPath, err := refsf.SafeJoin(ctx.handovers, laneID+"-latest.md")
	if err != nil {
		return "", "", err
	}
	return stampPath, latestPath, nil
}

func readBoard(caseRoot string) (board, error) {
	return mission.ReadBoard(caseRoot)
}

func resolveHandoffLane(caseRoot string, b board, selector string) (Lane, error) {
	raw := strings.TrimSpace(selector)
	if raw == "" {
		return Lane{}, fmt.Errorf("handoff selector is empty")
	}
	normalized := strings.ToLower(raw)
	candidates := []string{}
	if normalized == "main" {
		candidates = append(candidates, b.DefaultAuthorityLane)
	} else {
		candidates = append(candidates, raw)
		if !strings.HasPrefix(normalized, "feature-") {
			candidates = append(candidates, "feature-"+raw)
		}
	}
	for _, candidate := range candidates {
		for _, lane := range b.Lanes {
			if strings.EqualFold(lane.ID, candidate) {
				return readLaneByID(caseRoot, lane.ID)
			}
		}
	}
	for _, lane := range b.Lanes {
		full, err := readLaneByID(caseRoot, lane.ID)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return Lane{}, err
		}
		if strings.EqualFold(full.Name, raw) {
			return full, nil
		}
	}
	return Lane{}, fmt.Errorf("unknown workstream selector: %s; available: %s", selector, availableLaneLabels(caseRoot, b))
}

func availableLaneLabels(caseRoot string, b board) string {
	labels := []string{}
	for _, lane := range b.Lanes {
		full, err := readLaneByID(caseRoot, lane.ID)
		if err != nil {
			labels = append(labels, lane.ID)
			continue
		}
		labels = append(labels, workstreamLabel(full))
	}
	sort.Strings(labels)
	return strings.Join(labels, ", ")
}

func readLaneByID(caseRoot, laneID string) (Lane, error) {
	if err := validateLaneIDSegment(laneID); err != nil {
		return Lane{}, err
	}
	path, err := refsf.SafeJoin(caseRoot, relJoin(".rekit", "lanes", laneID, "lane.json"))
	if err != nil {
		return Lane{}, err
	}
	lane, err := readLane(path)
	if err != nil {
		return Lane{}, err
	}
	if !strings.EqualFold(lane.ID, laneID) {
		return Lane{}, fmt.Errorf("lane id mismatch for %s: lane.json declares %s", laneID, lane.ID)
	}
	return lane, nil
}

func laneResumePaths(caseRoot string, lane Lane) (string, string, error) {
	laneRoot, err := laneRootPath(caseRoot, lane)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(laneRoot, "prompts", "RESUME.md"), filepath.Join(laneRoot, "checkpoints", "latest.json"), nil
}

func latestRunDigest(caseRoot string) (string, error) {
	runs, err := refsf.SafeJoin(caseRoot, ".rekit/runs")
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(runs)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	names := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for _, name := range names {
		digest := filepath.Join(runs, name, "digest.md")
		if refsf.Exists(digest) {
			return relativePath(caseRoot, digest), nil
		}
	}
	return "", nil
}

func writeWorkspacePackets(out *bytes.Buffer, caseRoot string, lane Lane) error {
	workspace, err := refsf.SafeJoin(caseRoot, lane.Workspace)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(workspace)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	packets := []string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			continue
		}
		packets = append(packets, filepath.Join(workspace, entry.Name()))
	}
	sort.Strings(packets)
	if len(packets) == 0 {
		return nil
	}
	fmt.Fprintln(out, "## workspace packet")
	fmt.Fprintln(out)
	for _, packet := range packets {
		fmt.Fprintf(out, "- `%s`：workspace 产物，含 evidence/candidate/decision packet。\n", relativePath(caseRoot, packet))
	}
	fmt.Fprintln(out)
	return nil
}

func readHandoffFacts(caseRoot string) (mission.LedgerFacts, error) {
	return mission.ReadLedgerFacts(caseRoot)
}

func projectMissionBrief(lanes []boardLane, facts mission.LedgerFacts) mission.Brief {
	return mission.BuildWithOptions(mission.BoardLanes(lanes), facts.Facts, mission.BuildOptions{
		MaxRows:            maxHandoffRows,
		OpenDecisionAction: "review open candidate/decision item(s) with evidence and authority boundary",
	})
}

func writeProjectMissionBrief(out *bytes.Buffer, lanes []boardLane, facts mission.LedgerFacts) {
	brief := projectMissionBrief(lanes, facts)
	fmt.Fprintln(out, "## Mission Control brief")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "- summary: %s\n", brief.Summary)
	writeHandoffBriefList(out, "ready lanes", brief.ReadyLanes)
	writeHandoffBriefList(out, "blocked lanes", brief.BlockedLanes)
	writeHandoffBriefList(out, "pending gates", brief.PendingGates)
	writeHandoffBriefList(out, "open decisions", brief.OpenDecisions)
	writeHandoffBriefList(out, "interventions", brief.Interventions)
	writeHandoffBriefList(out, "next agent actions", brief.NextAgentActions)
	writeHandoffBriefList(out, "escalations", brief.Escalations)
	fmt.Fprintln(out)
}

func laneMissionBrief(lane Lane, facts mission.LedgerFacts) mission.Brief {
	return mission.BuildWithOptions([]mission.Lane{{ID: lane.ID, Label: workstreamLabel(lane), Status: lane.Status}}, mission.LaneFacts(facts.Facts, lane.ID), mission.BuildOptions{
		MaxRows:            maxHandoffRows,
		OpenDecisionAction: "review open candidate/decision item(s) with evidence and authority boundary",
	})
}

func writeLaneMissionBrief(out *bytes.Buffer, lane Lane, facts mission.LedgerFacts) {
	laneFacts := mission.LaneFacts(facts.Facts, lane.ID)
	gates := mission.FilterLane(laneFacts.Requests, lane.ID, "pending-gate")
	interventions := mission.OpenEvents(laneFacts.Interventions)
	openDecisions := mission.OpenDecisionItems(laneFacts)
	fmt.Fprintln(out, "## Mission Control brief")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "- lane: %s status=%s workspace=%s\n", lane.ID, lane.Status, lane.Workspace)
	fmt.Fprintf(out, "- blocked: %t\n", len(gates) > 0 || len(interventions) > 0 || len(openDecisions) > 0)
	writeHandoffBriefList(out, "pending-gate", missionLines(gates, mission.LaneGateLine))
	writeHandoffBriefList(out, "open intervention", missionLines(interventions, mission.LaneInterventionLine))
	writeHandoffBriefList(out, "open decision", missionLines(openDecisions, mission.LaneOpenDecisionLine))
	actions := []string{}
	if len(interventions) > 0 {
		actions = append(actions, "reconcile user intervention before continuing this lane")
	}
	if len(gates) > 0 {
		actions = append(actions, "resolve or explicitly defer pending gate request(s) before heavy action")
	}
	if len(openDecisions) > 0 {
		actions = append(actions, "review open candidate/decision item(s) with evidence and authority boundary")
	}
	if len(actions) == 0 {
		actions = append(actions, "continue lane with /rekit continue "+workstreamLabel(lane))
	}
	writeHandoffBriefList(out, "next agent action", actions)
	fmt.Fprintln(out)
}

func missionLines(items []map[string]any, line func(map[string]any) string) []string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, line(item))
	}
	return lines
}

func writeHandoffBriefList(out *bytes.Buffer, label string, items []string) {
	if len(items) == 0 {
		fmt.Fprintf(out, "- %s: none\n", label)
		return
	}
	fmt.Fprintf(out, "- %s:\n", label)
	for _, item := range mission.LimitStrings(items, maxHandoffRows) {
		fmt.Fprintf(out, "  - %s\n", item)
	}
}

func writeVerificationSection(out *bytes.Buffer, verifications []map[string]any, laneID string) {
	items := filterLane(verifications, laneID, "")
	if len(items) == 0 {
		return
	}
	fmt.Fprintln(out, "## verification")
	fmt.Fprintln(out)
	for _, v := range lastObjects(items, maxHandoffRows) {
		subj := firstObjectText(v, "subject", "kind")
		by := firstObjectText(v, "actor")
		byTag := ""
		if by != "" {
			byTag = " | by=" + by
		}
		fmt.Fprintf(out, "- %s | verifier=%s | verdict=%s | target=%s%s%s\n", subj, firstObjectText(v, "verifier"), firstObjectText(v, "verdict"), firstObjectText(v, "target"), byTag, batchTag(v))
	}
	fmt.Fprintln(out)
}

func writeDecisionSection(out *bytes.Buffer, decisions []map[string]any, laneID string) {
	items := filterLane(decisions, laneID, "")
	if len(items) == 0 {
		return
	}
	fmt.Fprintln(out, "## decision")
	fmt.Fprintln(out)
	for _, d := range lastObjects(items, maxHandoffRows) {
		subj := firstObjectText(d, "subject", "kind")
		dec := firstObjectText(d, "decision", "action")
		by := firstObjectText(d, "confirmedBy", "actor")
		byTag := ""
		if by != "" {
			byTag = " | by=" + by
		}
		fmt.Fprintf(out, "- %s | decision=%s%s | reason=%s\n", subj, dec, byTag, firstObjectText(d, "reason"))
	}
	fmt.Fprintln(out)
}

func writePendingGateSection(out *bytes.Buffer, requests []map[string]any, laneID string) {
	items := filterLane(requests, laneID, "pending-gate")
	if len(items) == 0 {
		return
	}
	fmt.Fprintln(out, "## pending-gate")
	fmt.Fprintln(out)
	for _, g := range lastObjects(items, maxHandoffRows) {
		fmt.Fprintf(out, "- %s | %s%s\n", firstObjectText(g, "subject"), firstObjectText(g, "summary"), gateRequestDetail(g, true, true))
	}
	fmt.Fprintln(out)
}

func writeInterventionSection(out *bytes.Buffer, interventions []map[string]any, laneID string) {
	items := filterLane(interventions, laneID, "")
	if len(items) == 0 {
		return
	}
	fmt.Fprintln(out, "## intervention")
	fmt.Fprintln(out)
	for _, i := range lastObjects(items, maxHandoffRows) {
		batchTag := batchTag(i)
		fmt.Fprintf(out, "- %s | action=%s | target=%s | approvedBy=%s | scope=%s | status=%s%s\n", firstObjectText(i, "subject", "action"), firstObjectText(i, "action"), firstObjectText(i, "target"), firstObjectText(i, "approvedBy"), firstObjectText(i, "scope"), firstObjectText(i, "status"), batchTag)
	}
	fmt.Fprintln(out)
}

func writeRollbackSection(out *bytes.Buffer, rollbacks []map[string]any, laneID string) {
	items := filterLane(rollbacks, laneID, "")
	if len(items) == 0 {
		return
	}
	fmt.Fprintln(out, "## rollback")
	fmt.Fprintln(out)
	for _, r := range lastObjects(items, maxHandoffRows) {
		batchTag := batchTag(r)
		fmt.Fprintf(out, "- %s | target=%s | status=%s%s | reason=%s\n", firstObjectText(r, "subject", "kind"), firstObjectText(r, "target"), firstObjectText(r, "status"), batchTag, firstObjectText(r, "reason"))
	}
	fmt.Fprintln(out)
}

func filterLane(items []map[string]any, laneID, status string) []map[string]any {
	out := []map[string]any{}
	for _, item := range items {
		if firstObjectText(item, "lane") != laneID {
			continue
		}
		if status != "" && firstObjectText(item, "status") != status {
			continue
		}
		out = append(out, item)
	}
	return out
}

func gateRequestDetail(e map[string]any, omitStatus, omitBatch bool) string {
	parts := []string{}
	add := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			parts = append(parts, key+"="+value)
		}
	}
	if !omitStatus {
		add("status", firstObjectText(e, "status"))
	}
	add("by", firstObjectText(e, "actor"))
	add("risk", firstObjectText(e, "risk"))
	add("target", firstObjectText(e, "target"))
	if !omitBatch {
		add("batch", firstObjectText(e, "batchId"))
	}
	if gate, ok := e["gate"].(map[string]any); ok {
		add("action", firstObjectText(gate, "action"))
		add("scope", firstObjectText(gate, "scope"))
		add("budget", firstObjectText(gate, "budget"))
		add("tried", firstObjectText(gate, "triedLightSteps"))
		add("stop", firstObjectText(gate, "stopConditions"))
	}
	if len(parts) == 0 {
		return ""
	}
	return " | " + strings.Join(parts, " | ")
}

func batchTag(e map[string]any) string {
	batch := firstObjectText(e, "batchId")
	if batch == "" {
		return ""
	}
	return " | batch=" + batch
}

func writeText(path, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func handoffTimestamp() string {
	return time.Now().UTC().Format("20060102-150405000")
}

func isoNow() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
