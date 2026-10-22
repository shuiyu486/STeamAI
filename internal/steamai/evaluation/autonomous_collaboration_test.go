package evaluation

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/STeamAI/internal/steamai/casebootstrap"
)

// 本文件是自主协作维护对照的默认单测：只验证预算闸门、固定次序、证据写入、
// 零写校验与 Fresh facts 组装，不启动 Claude Code、不建 Git 仓库、不产生费用。
// 真实 opt-in 运行器见 autonomous_collaboration_live_test.go。

const (
	acLiveEnv             = "STEAMAI_AUTONOMOUS_LIVE"
	acRootEnv             = "STEAMAI_AC_ROOT"
	acBudgetFileEnv       = "STEAMAI_AC_BUDGET_FILE"
	acStageEnv            = "STEAMAI_AC_STAGE"
	acBaselineSourceEnv   = "STEAMAI_AC_BASELINE_SOURCE"
	acCandidateSourceEnv  = "STEAMAI_AC_CANDIDATE_SOURCE"
	acConfirmationsEnv    = "STEAMAI_AC_CONFIRMATIONS_FILE"
	acDefaultPack         = "binary-re"
	// 单次调用上限按实测校准（2026-10-21 定点复现）：挂起形态不是网关拒绝，
	// 而是 glm-5.3 自适应思考（thinking:{type:adaptive}，effort/MAX_THINKING_TOKENS
	// 均不生效）对算术/SHA-256 类任务多轮生成 5-12 分钟，stdout 又缓冲到退出才落
	// 盘，中途杀进程表现为零输出超时。180s 必杀；420s 容纳 q2（约 2 分钟）但 q1
	// 实测 8-12 分钟仍超。共享 deadline 仍 fail-closed。
	acMaxCallSeconds      = 420
	acSharedWindowMinutes = 30
	acTotalBudgetCapUSD   = 9.0
	acArmBudgetUSD        = 1.5
	acReviewBudgetUSD     = 0.5
	acCorrectionBudgetUSD = 2.0
	acCorrectionTurnCaps  = 3
)

// acBudgetFile 由主 agent 在运行器外部准备；运行器只读，不重置共享窗口或总额。
type acBudgetFile struct {
	StartedAt time.Time `json:"startedAt"`
	Deadline  time.Time `json:"deadline"`
	MaxUSD    float64   `json:"maxUSD"`
}

func parseACBudget(data []byte, now time.Time) (acBudgetFile, error) {
	// Windows 工具（如 PowerShell Set-Content）常写 UTF-8 BOM；JSON 本身不允许 BOM，
	// 在解析边界剥掉，不放宽任何字段校验。
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var budget acBudgetFile
	if err := decoder.Decode(&budget); err != nil {
		return acBudgetFile{}, errors.New("预算文件必须是 {startedAt,deadline,maxUSD} JSON")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return acBudgetFile{}, errors.New("预算文件存在额外 JSON 内容")
	}
	if budget.MaxUSD <= 0 || budget.MaxUSD > acTotalBudgetCapUSD {
		return acBudgetFile{}, errors.New("预算文件 maxUSD 必须在 (0,9] 区间（4 arms*1.5 + 2 盲审*0.5 + 纠偏 2）")
	}
	if budget.Deadline.Sub(budget.StartedAt) <= 0 || budget.Deadline.Sub(budget.StartedAt) > acSharedWindowMinutes*time.Minute {
		return acBudgetFile{}, errors.New("预算文件 deadline 必须晚于 startedAt 且不超过 30 分钟共享窗口")
	}
	if !now.Before(budget.Deadline) {
		return acBudgetFile{}, errors.New("预算文件共享 deadline 已过期；主 agent 决定是否重开新窗口")
	}
	if now.Add(time.Duration(-acSharedWindowMinutes) * time.Minute).After(budget.StartedAt) {
		return acBudgetFile{}, errors.New("预算文件 startedAt 早于 30 分钟共享窗口，窗口已经结束")
	}
	return budget, nil
}

type acBudgetState struct {
	file       acBudgetFile
	spent      float64
	blocked    bool
	blockStage string
	blockTurn  string
	blockError string
}

// gate 在启动任何真实调用前检查共享窗口与总预算；不通过时该 turn 记为 not-launched。
func (state *acBudgetState) gate(stage, turn string, turnBudget float64, now time.Time) error {
	if state.blocked {
		return errors.New("先前调用已停止运行器：" + state.blockError)
	}
	if !now.Before(state.file.Deadline) {
		return errors.New("共享 deadline 已到，不再启动后续调用")
	}
	if state.spent+turnBudget > state.file.MaxUSD {
		return errors.New("剩余预算不足以覆盖该调用上限，不再启动")
	}
	return nil
}

// charge 吸收一次真实调用的结果：缺 cost、非 completed 状态立即阻断后续 launch。
func (state *acBudgetState) charge(stage, turn string, status string, costUSD *float64, errText string) {
	if costUSD != nil && *costUSD > 0 {
		state.spent += *costUSD
	}
	if state.blocked {
		return
	}
	switch {
	case costUSD == nil:
		state.blocked = true
		state.blockError = "模型未自报 total_cost_usd，无法核账：" + errText
	case status != "completed":
		state.blocked = true
		state.blockError = "调用未完成（" + status + "）：" + errText
	}
	if state.blocked {
		state.blockStage, state.blockTurn = stage, turn
	}
}

// blockReason 返回不再 launch 的原因，供 not-launched 记录引用。
func (state *acBudgetState) blockReason() string {
	if state.blocked {
		return "先前调用已停止运行器：" + state.blockError
	}
	return "共享 deadline 或预算已耗尽"
}

type acArmPlan struct {
	Case, Question, Contract string
}

// acArmOrder 是固定 counterbalance：两道全新题各跑旧合同与候选合同一次，
// 次序写死不随机，保证不同运行之间可比。
var acArmOrder = []acArmPlan{
	{Case: "arm-1", Question: "q1", Contract: "baseline"},
	{Case: "arm-2", Question: "q2", Contract: "candidate"},
	{Case: "arm-3", Question: "q1", Contract: "candidate"},
	{Case: "arm-4", Question: "q2", Contract: "baseline"},
}

func acCaseNames() []string {
	names := make([]string, 0, len(acArmOrder)+1)
	for _, arm := range acArmOrder {
		names = append(names, arm.Case)
	}
	return append(names, "correction")
}

func acContractSource(contract string) string {
	if contract == "baseline" {
		return acBaselineSourceEnv
	}
	return acCandidateSourceEnv
}

// acQuestion 是全新合成 Read/Write 题：输入在 prepare 阶段落盘，答案由同一常量
// 生成函数推导，运行器从不读模型自报定 success。
type acQuestion struct {
	ID         string
	Input      string
	Answer     string
	ResultLine string
}

func acBuildQuestion(id string) acQuestion {
	switch id {
	case "q1":
		lines := []string{"alpha: 12", "bravo: -5", "charlie: 40", "delta: -17", "echo: 8",
			"foxtrot: 23", "golf: -9", "hotel: 31", "india: -4", "juliet: 15",
			"kilo: 27", "lima: -22", "mike: 6", "november: 19", "oscar: -11",
			"papa: 34", "quebec: -3", "romeo: 26", "sierra: -14", "tango: 9"}
		sum := 0
		for _, line := range lines {
			value, err := strconv.Atoi(strings.TrimSpace(strings.SplitN(line, ":", 2)[1]))
			if err != nil {
				panic("q1 常量必须是整数：" + err.Error())
			}
			sum += value
		}
		answer := strconv.Itoa(sum)
		return acQuestion{ID: "q1", Input: "# Synthetic ledger\n\n" + strings.Join(lines, "\n") + "\n",
			Answer: answer, ResultLine: "sum: " + answer}
	default:
		mapping := []string{"red->rot", "blue->blau", "green->gruen", "gray->grau"}
		queries := []string{"blue", "gray", "red", "green", "blue"}
		table := map[string]string{}
		for _, pair := range mapping {
			parts := strings.SplitN(pair, "->", 2)
			table[parts[0]] = parts[1]
		}
		resolved := make([]string, 0, len(queries))
		for _, query := range queries {
			value, ok := table[query]
			if !ok {
				panic("q2 查询必须全部命中常量映射：" + query)
			}
			resolved = append(resolved, value)
		}
		answer := strings.Join(resolved, ",")
		return acQuestion{ID: "q2", Input: "# Synthetic mapping\n\n" + strings.Join(mapping, "\n") +
			"\n\n# Queries\n\n" + strings.Join(queries, "\n") + "\n",
			Answer: answer, ResultLine: "resolved: " + answer}
	}
}

// 七字段成员合同字段名，与 vnext/templates/member/CLAUDE.md 的当前正式任务节一致。
var acTaskFields = []string{"目标", "输入", "允许读取", "允许写入", "交付", "停止或升级条件", "完成/退出条件"}

type acTaskSpec struct {
	Goal, Inputs, Reads, Writes, Deliverables, Stop, Exit string
}

func (spec acTaskSpec) rows() []string {
	return []string{
		"- 目标：`" + spec.Goal + "`",
		"- 输入：`" + spec.Inputs + "`",
		"- 允许读取：`" + spec.Reads + "`",
		"- 允许写入：`" + spec.Writes + "`",
		"- 交付：`" + spec.Deliverables + "`",
		"- 停止或升级条件：`" + spec.Stop + "`",
		"- 完成/退出条件：`" + spec.Exit + "`",
	}
}

func (spec acTaskSpec) changedFields(other acTaskSpec) []string {
	pairs := [][2]string{{"目标", spec.Goal + "|" + other.Goal}, {"输入", spec.Inputs + "|" + other.Inputs},
		{"允许读取", spec.Reads + "|" + other.Reads}, {"允许写入", spec.Writes + "|" + other.Writes},
		{"交付", spec.Deliverables + "|" + other.Deliverables}, {"停止或升级条件", spec.Stop + "|" + other.Stop},
		{"完成/退出条件", spec.Exit + "|" + other.Exit}}
	var changed []string
	for _, pair := range pairs {
		parts := strings.SplitN(pair[1], "|", 2)
		if parts[0] != parts[1] {
			changed = append(changed, pair[0])
		}
	}
	return changed
}

// acTaskBlock 渲染成任务变更消息中 expected/new 的全部七项；缺任何一项即拒绝。
func acTaskBlock(spec acTaskSpec) string {
	return strings.Join(spec.rows(), "\n")
}

// 纠偏 case：旧七字段、缩小任务（只改目标）与 stale overwrite。
func acCorrectionTaskTrio() (old, narrowed, stale acTaskSpec) {
	old = acTaskSpec{
		Goal:         "独立核对合成账目并交付单页结论",
		Inputs:       "本目录 CLAUDE.md 与父目录 .steamai-vnext/CLAUDE.md",
		Reads:        "本目录 CLAUDE.md 与父目录 .steamai-vnext/CLAUDE.md",
		Writes:       "本目录 CLAUDE.md（成员单写）",
		Deliverables: "无文件交付；正式任务变更只更新本文件当前正式任务节",
		Stop:         "expected 与当前任务节不一致时返回 HOLD_STALE_TASK 并通知 Commander",
		Exit:         "任务节更新完成或返回 hold 后即结束本轮",
	}
	narrowed = old
	narrowed.Goal = "只核对合成账目第 1 组并交付该组结论"
	stale = old
	stale.Goal = "直接改写为 stale overwrite 目标"
	return old, narrowed, stale
}

func acMemberFacts(task acTaskSpec) casebootstrap.MemberFacts {
	return casebootstrap.MemberFacts{
		Name: "owner", Kind: "execution", Role: "execution",
		Responsibility: "对当前正式任务的结果负责，自主选择读取与落盘顺序",
		TaskGoal:       task.Goal, Inputs: task.Inputs, AllowedReads: task.Reads,
		AllowedWrites: task.Writes, Deliverables: task.Deliverables,
		StopOrEscalate: task.Stop, ExitConditions: task.Exit,
	}
}

// acArmTaskSpec 是 4 个 arm case 的成员当前正式任务（七字段）。
func acArmTaskSpec() acTaskSpec {
	return acTaskSpec{
		Goal:         "完成 task/input.md 的合成判定并把结果写入 task/result.md",
		Inputs:       "task/input.md",
		Reads:        "task/input.md 与本目录 CLAUDE.md",
		Writes:       "task/result.md",
		Deliverables: "task/result.md 一行结果",
		Stop:         "输入缺失或无法判定时停止并说明缺项",
		Exit:         "result.md 写入且自报 hash 与实际一致",
	}
}

// acFreshFacts 组装 5 个 case 各自的 production Fresh facts；每个 case 独立
// BuildPreview，不复用 preview 对象。
func acFreshFacts(caseName string, memberTask acTaskSpec) casebootstrap.Facts {
	return casebootstrap.Facts{
		Name:          "autonomous-collaboration-" + caseName,
		Goal:          memberTask.Goal,
		Authorization: "仅限本 case 内的合成 fixture；允许 Read/Write/Edit 文件工具，不允许网络、shell、MCP 或子 agent",
		Prohibited:    "读取 case 外目录、使用 Bash/PowerShell、网络访问、修改 Claude Code 配置、git 提交推送",
		Stop:          "输入缺失、无法判定或 expected 任务不匹配时停止并说明",
		Pack:          acDefaultPack,
		Members:       []casebootstrap.MemberFacts{acMemberFacts(memberTask)},
	}
}

// acZeroWriteCheck 用于纠偏 stale turn：成员 CLAUDE.md bytes 与整个 case 树
// identity 都必须与 turn 前一致。
func acZeroWriteCheck(caseRoot, memberFile string, beforeFile []byte, beforeTree string) error {
	after, err := os.ReadFile(memberFile)
	if err != nil {
		return err
	}
	if Hash(after) != Hash(beforeFile) {
		return errors.New("stale turn 修改了成员任务文件 bytes")
	}
	afterTree, err := treeIdentity(caseRoot)
	if err != nil {
		return err
	}
	if afterTree != beforeTree {
		return errors.New("stale turn 修改了 case 树（task bytes 非零写）")
	}
	return nil
}

type acTurnRecord struct {
	SchemaVersion int               `json:"schemaVersion"`
	Stage         string            `json:"stage"`
	Case          string            `json:"case"`
	Turn          string            `json:"turn"`
	Status        string            `json:"status"` // completed|timeout|failed|invalid-output|not-launched
	Error         string            `json:"error,omitempty"`
	Model         string            `json:"model"`
	SessionID     string            `json:"sessionId,omitempty"`
	CostUSD       *float64          `json:"costUsd,omitempty"`
	OutputSHA256  string            `json:"outputSha256"`
	OutputBytes   int               `json:"outputBytes"`
	StderrSHA256  string            `json:"stderrSha256"`
	StderrBytes   int               `json:"stderrBytes"`
	ElapsedMillis int64             `json:"elapsedMillis"`
	Checks        map[string]string `json:"checks,omitempty"`
	Artifacts     map[string]string `json:"artifacts"` // 相对路径 -> 实际落盘 SHA-256
}

// acWriteTurnEvidence 以 no-replace 保存单次调用的全部真实证据，失败调用同样保留。
func acWriteTurnEvidence(evidenceRoot string, record acTurnRecord, output, stderr []byte) error {
	dir := filepath.Join(evidenceRoot, record.Case, record.Turn)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	for name, data := range map[string][]byte{
		"run.json":    append(encoded, '\n'),
		"output.json": output,
		"stderr.txt":  stderr,
	} {
		if err := writeNew(filepath.Join(dir, name), data); err != nil {
			return err
		}
	}
	return nil
}

func TestACBudgetFileValidation(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	valid := func(mutate func(*acBudgetFile)) acBudgetFile {
		budget := acBudgetFile{StartedAt: now.Add(-2 * time.Minute), Deadline: now.Add(20 * time.Minute), MaxUSD: 9}
		mutate(&budget)
		return budget
	}
	for _, tc := range []struct {
		name  string
		file  acBudgetFile
		valid bool
	}{
		{"valid", valid(func(*acBudgetFile) {}), true},
		{"maxUSD over 9", valid(func(f *acBudgetFile) { f.MaxUSD = 9.5 }), false},
		{"maxUSD zero", valid(func(f *acBudgetFile) { f.MaxUSD = 0 }), false},
		{"deadline over 30min", valid(func(f *acBudgetFile) { f.Deadline = f.StartedAt.Add(31 * time.Minute) }), false},
		{"deadline equals start", valid(func(f *acBudgetFile) { f.Deadline = f.StartedAt }), false},
		{"deadline already passed", valid(func(f *acBudgetFile) { f.Deadline = now.Add(-time.Minute) }), false},
		{"started before window", valid(func(f *acBudgetFile) { f.StartedAt = now.Add(-31 * time.Minute); f.Deadline = now.Add(-time.Minute) }), false},
	} {
		data, err := json.Marshal(tc.file)
		if err != nil {
			t.Fatal(err)
		}
		_, parseErr := parseACBudget(data, now)
		if (parseErr == nil) != tc.valid {
			t.Fatalf("%s: err=%v", tc.name, parseErr)
		}
	}
	if _, err := parseACBudget([]byte(`{"startedAt":"x"}`), now); err == nil {
		t.Fatal("invalid JSON must fail")
	}
	if _, err := parseACBudget([]byte(`{"startedAt":"2026-10-21T10:00:00Z","deadline":"2026-10-21T10:20:00Z","maxUSD":9,"extra":1}`), now); err == nil {
		t.Fatal("unknown fields must fail")
	}
}

func TestACBudgetStateGateAndCharge(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	state := &acBudgetState{file: acBudgetFile{StartedAt: now.Add(-time.Minute), Deadline: now.Add(10 * time.Minute), MaxUSD: 1.5}}
	if err := state.gate("ab", "arm-1", 1.5, now); err != nil {
		t.Fatal(err)
	}
	cost := 0.2
	state.charge("ab", "arm-1", "completed", &cost, "")
	if state.blocked {
		t.Fatal("completed turn with cost must not block")
	}
	if err := state.gate("ab", "arm-2", acArmBudgetUSD, now); err == nil {
		t.Fatal("remaining budget must reject next arm")
	}
	state2 := &acBudgetState{file: state.file}
	state2.charge("ab", "arm-1", "completed", nil, "no cost")
	if !state2.blocked || state2.blockStage != "ab" || state2.blockTurn != "arm-1" {
		t.Fatalf("missing cost must block: %+v", state2)
	}
	if err := state2.gate("ab", "arm-2", 0.1, now); err == nil {
		t.Fatal("blocked state must reject further launches")
	}
	state3 := &acBudgetState{file: state.file}
	state3.charge("ab", "arm-1", "timeout", &cost, "arm exceeded maxSeconds")
	if !state3.blocked {
		t.Fatal("non-completed status must block")
	}
	late := &acBudgetState{file: acBudgetFile{StartedAt: now.Add(-29 * time.Minute), Deadline: now.Add(time.Second), MaxUSD: 9}}
	if err := late.gate("ab", "arm-1", 0.1, now.Add(2*time.Second)); err == nil {
		t.Fatal("past deadline must reject launch")
	}
}

func TestACFixedCounterbalanceOrder(t *testing.T) {
	perQuestion := map[string]map[string]bool{}
	for index, arm := range acArmOrder {
		if arm.Case != "arm-"+strconv.Itoa(index+1) {
			t.Fatalf("arm case naming drifted: %+v", arm)
		}
		if perQuestion[arm.Question] == nil {
			perQuestion[arm.Question] = map[string]bool{}
		}
		perQuestion[arm.Question][arm.Contract] = true
	}
	for _, question := range []string{"q1", "q2"} {
		if len(perQuestion[question]) != 2 {
			t.Fatalf("question %s must run both contracts exactly once: %v", question, perQuestion[question])
		}
	}
	total := float64(len(acArmOrder))*acArmBudgetUSD + 2*acReviewBudgetUSD + acCorrectionBudgetUSD
	if total != acTotalBudgetCapUSD {
		t.Fatalf("budget allocation must sum to cap: %v", total)
	}
	if len(acCaseNames()) != 5 {
		t.Fatal("five independent cases expected")
	}
}

func TestACDecodeStructuredFormats(t *testing.T) {
	type payload struct {
		Preferred string `json:"preferred"`
	}
	array := []byte(`[{"type":"system"},{"type":"result","structured_output":{"preferred":"tie"}}]`)
	jsonl := []byte("{\"type\":\"system\"}\n{\"type\":\"result\",\"structured_output\":{\"preferred\":\"A\"}}\n")
	lastWins := []byte(`[{"structured_output":{"preferred":"A"}},{"structured_output":{"preferred":"B"}}]`)
	none := []byte(`[{"type":"system"},{"type":"result"}]`)
	badShape := []byte(`[{"structured_output":{"preferred":123}}]`)
	for _, tc := range []struct {
		name, want string
		input      []byte
		wantErr    bool
	}{
		{"json array", "tie", array, false},
		{"jsonl lines", "A", jsonl, false},
		{"last node wins", "B", lastWins, false},
		{"missing node", "", none, true},
		{"schema mismatch", "", badShape, true},
	} {
		var got payload
		err := acDecodeStructured(t, modelEnvelope{}, tc.input, &got)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("%s: expected error", tc.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got.Preferred != tc.want {
			t.Fatalf("%s: got %q want %q", tc.name, got.Preferred, tc.want)
		}
	}
}

func TestACQuestionsDeterministic(t *testing.T) {
	for _, id := range []string{"q1", "q2"} {
		first, second := acBuildQuestion(id), acBuildQuestion(id)
		if first.Input != second.Input || first.Answer != second.Answer || first.ResultLine != second.ResultLine {
			t.Fatalf("question %s is not deterministic", id)
		}
		if !strings.Contains(first.ResultLine, first.Answer) {
			t.Fatalf("question %s answer must appear in result line", id)
		}
	}
	if acBuildQuestion("q1").Answer == acBuildQuestion("q2").Answer {
		t.Fatal("two questions must be distinct")
	}
}

func TestACCorrectionTaskTrioOnlyGoalNarrows(t *testing.T) {
	old, narrowed, stale := acCorrectionTaskTrio()
	if changed := old.changedFields(narrowed); len(changed) != 1 || changed[0] != "目标" {
		t.Fatalf("narrowed task must only change goal, got %v", changed)
	}
	if changed := old.changedFields(stale); len(changed) != 1 || changed[0] != "目标" {
		t.Fatalf("stale task must only differ in goal for a clean hold check, got %v", changed)
	}
	for _, spec := range []acTaskSpec{old, narrowed, stale} {
		rows := spec.rows()
		if len(rows) != len(acTaskFields) {
			t.Fatalf("task block must render all seven fields")
		}
		for index, field := range acTaskFields {
			if !strings.HasPrefix(rows[index], "- "+field+"：`") {
				t.Fatalf("row %d must render field %s verbatim", index, field)
			}
		}
	}
}

func TestACFreshFactsValidateWithoutAuxPack(t *testing.T) {
	for _, arm := range acArmOrder {
		facts := acFreshFacts(arm.Case, acArmTaskSpec())
		if err := facts.Validate(); err != nil {
			t.Fatalf("arm %s facts invalid: %v", arm.Case, err)
		}
	}
	old, _, _ := acCorrectionTaskTrio()
	correction := acFreshFacts("correction", old)
	if err := correction.Validate(); err != nil {
		t.Fatalf("correction facts invalid: %v", err)
	}
	if correction.Name == acFreshFacts("arm-1", acArmTaskSpec()).Name {
		t.Fatal("case names must stay unique")
	}
}

func TestACTaskChangePromptEmbedsAllSevenFields(t *testing.T) {
	old, narrowed, stale := acCorrectionTaskTrio()
	for _, tc := range []struct {
		name   string
		prompt string
		next   acTaskSpec
	}{
		{"narrow", acTaskChangePrompt(old, narrowed, false), narrowed},
		{"stale", acTaskChangePrompt(old, stale, true), stale},
	} {
		for _, row := range strings.Split(acTaskBlock(tc.next), "\n") {
			if !strings.Contains(tc.prompt, row) {
				t.Fatalf("%s prompt 缺少 new 七字段中的 %q；expected/new 必须逐项完整，不得以单字符串 stub 冒充", tc.name, row)
			}
		}
		for _, row := range strings.Split(acTaskBlock(old), "\n") {
			if !strings.Contains(tc.prompt, row) {
				t.Fatalf("%s prompt 缺少 expected 七字段中的 %q", tc.name, row)
			}
		}
		if !strings.Contains(tc.prompt, "expected current task:") || !strings.Contains(tc.prompt, "new current task:") {
			t.Fatalf("%s prompt 必须同时列出 expected 与 new 任务块", tc.name)
		}
	}
	if !strings.Contains(acTaskChangePrompt(old, narrowed, false), "HOLD_STALE_TASK") {
		t.Fatal("prompt must keep compare-before-update hold semantics")
	}
}

func TestACZeroWriteCheckDetectsAnyByteChange(t *testing.T) {
	root := t.TempDir()
	memberDir := filepath.Join(root, ".steamai-vnext", "members", "owner")
	if err := os.MkdirAll(memberDir, 0o700); err != nil {
		t.Fatal(err)
	}
	memberFile := filepath.Join(memberDir, "CLAUDE.md")
	before := []byte("# member\n\n- 目标：`old`\n")
	if err := os.WriteFile(memberFile, before, 0o600); err != nil {
		t.Fatal(err)
	}
	tree, err := treeIdentity(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := acZeroWriteCheck(root, memberFile, before, tree); err != nil {
		t.Fatalf("unchanged tree must pass: %v", err)
	}
	if err := os.WriteFile(memberFile, append(before, ' '), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := acZeroWriteCheck(root, memberFile, before, tree); err == nil {
		t.Fatal("member file byte change must fail zero-write check")
	}
	if err := os.WriteFile(memberFile, before, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memberDir, "side.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := acZeroWriteCheck(root, memberFile, before, tree); err == nil {
		t.Fatal("new file inside case must fail zero-write check")
	}
}

func TestACRestoreSpentAndBlockedAcrossStages(t *testing.T) {
	root := t.TempDir()
	stagesRoot := filepath.Join(root, "stages")
	if err := os.MkdirAll(stagesRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	blockedSummary := acStageSummary{SchemaVersion: 1, Stage: "correction", SpentUSD: 0.75, Blocked: true,
		BlockStage: "correction", BlockTurn: "correction/read-identity", BlockError: "调用未完成（timeout）"}
	data, err := json.Marshal(blockedSummary)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagesRoot, "correction.json"), append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &acLiveRunner{root: root, budget: &acBudgetState{file: acBudgetFile{
		StartedAt: time.Now().UTC().Add(-time.Minute), Deadline: time.Now().UTC().Add(20 * time.Minute), MaxUSD: 9}}}
	if err := runner.restoreSpentAndBlocked(); err != nil {
		t.Fatal(err)
	}
	if runner.budget.spent != 0.75 || !runner.budget.blocked || runner.budget.blockStage != "correction" {
		t.Fatalf("cross-stage state not restored: %+v", runner.budget)
	}
	if err := runner.budget.gate("ab", "arm-1/task", 1.5, time.Now().UTC()); err == nil {
		t.Fatal("restored blocked state must reject further launches")
	}
	if err := os.WriteFile(filepath.Join(stagesRoot, "correction.json"), append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runner.restoreSpentAndBlocked(); err != nil {
		t.Fatal(err)
	}
	// 同一摘要重复恢复取最大值，不重复计费；正常路径 writeNew 已阻止同 stage 二次运行。
	if runner.budget.spent != 0.75 {
		t.Fatalf("repeated restore must keep max cumulative spent: %v", runner.budget.spent)
	}
}

// 各阶段摘要保存的是写摘要时刻的累计消费，跨阶段恢复必须取最大值；
// 求和会把同一笔真实消费在 correction 与 ab 两个摘要里重复计入。
func TestACRestoreSpentTakesMaxAcrossCumulativeStageSummaries(t *testing.T) {
	root := t.TempDir()
	stagesRoot := filepath.Join(root, "stages")
	if err := os.MkdirAll(stagesRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, stage := range []struct {
		name  string
		spent float64
	}{
		{"correction", 0.13}, {"ab", 0.13}, {"review", 0.13},
	} {
		summary := acStageSummary{SchemaVersion: 1, Stage: stage.name, SpentUSD: stage.spent}
		data, err := json.Marshal(summary)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stagesRoot, stage.name+".json"), append(data, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	runner := &acLiveRunner{root: root, budget: &acBudgetState{file: acBudgetFile{
		StartedAt: time.Now().UTC().Add(-time.Minute), Deadline: time.Now().UTC().Add(20 * time.Minute), MaxUSD: 9}}}
	if err := runner.restoreSpentAndBlocked(); err != nil {
		t.Fatal(err)
	}
	if runner.budget.spent != 0.13 {
		t.Fatalf("cumulative stage summaries must restore max, not sum: %v", runner.budget.spent)
	}
}

func TestACTurnEvidenceWritesNoReplaceAndKeepsFailures(t *testing.T) {
	root := t.TempDir()
	failed := 0.4
	record := acTurnRecord{SchemaVersion: 1, Stage: "ab", Case: "arm-1", Turn: "task", Status: "failed",
		Error: "arm exceeded maxSeconds", Model: "fixture-model", CostUSD: &failed,
		OutputSHA256: Hash([]byte("out")), OutputBytes: 3, StderrSHA256: Hash([]byte("err")), StderrBytes: 3,
		Artifacts: map[string]string{"task/result.md": Hash([]byte("sum: 84"))}}
	if err := acWriteTurnEvidence(root, record, []byte("out"), []byte("err")); err != nil {
		t.Fatal(err)
	}
	for name, contains := range map[string]string{
		filepath.Join(root, "arm-1", "task", "run.json"):    `"status": "failed"`,
		filepath.Join(root, "arm-1", "task", "output.json"): "out",
		filepath.Join(root, "arm-1", "task", "stderr.txt"):  "err",
	} {
		data, err := os.ReadFile(name)
		if err != nil || !strings.Contains(string(data), contains) {
			t.Fatalf("evidence %s missing: %v", name, err)
		}
	}
	if err := acWriteTurnEvidence(root, record, []byte("out"), []byte("err")); err == nil {
		t.Fatal("evidence must be no-replace to stop double-charging runs")
	}
	notLaunched := record
	notLaunched.Status, notLaunched.Error, notLaunched.CostUSD = "not-launched", "先前调用已停止运行器", nil
	notLaunched.Turn = "task-2"
	if err := acWriteTurnEvidence(root, notLaunched, nil, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "arm-1", "task-2", "run.json"))
	if err != nil || !strings.Contains(string(data), "not-launched") {
		t.Fatalf("unexecuted turns must be retained as not-launched: %v", err)
	}
}
