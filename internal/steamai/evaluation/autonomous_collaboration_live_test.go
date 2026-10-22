package evaluation

// 自主协作维护对照的 opt-in 运行器：默认跳过，只有显式设置环境变量才可能
// 启动真实 Claude Code 调用。它不改 production evaluator/readonly 合同，只按
// 现有 production Fresh、模型解析与 Windows Job 合同编排一次有界对照。
//
// 环境接口（均由主 agent 在付费运行前准备，运行器只读）：
//
//	STEAMAI_AUTONOMOUS_LIVE=1                 开启本 gate；缺省一律 skip
//	STEAMAI_AC_ROOT                           仓库外证据根目录（案例与证据全部保存在这里）
//	STEAMAI_AC_BUDGET_FILE                    {startedAt,deadline,maxUSD} 共享预算 JSON
//	STEAMAI_AC_STAGE=preview|prepare|correction|ab|review|all
//	STEAMAI_AC_BASELINE_SOURCE                旧合同 37187d6d 工作树（Git worktree 根）
//	STEAMAI_AC_CANDIDATE_SOURCE               候选当前 tracked working tree 的 checkout 根
//	STEAMAI_AC_CONFIRMATIONS_FILE             stage=preview 输出 identities 后由用户确认的 JSON
//
// 阶段：preview 只做 production BuildPreview（零写入、零模型调用）；prepare 用
// exact confirmation 走 production Apply 并物化任务输入；correction/ab/review 才
// 启动真实调用。跨阶段共享同一预算窗口，spent/blocked 状态从既有 stages/*.json
// 恢复，不重置总计。

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/STeamAI/internal/steamai/casebootstrap"
)

const (
	acCorrectionCase    = "correction"
	acCorrectionTurn1ID = "read-identity"
	acCorrectionTurn2ID = "narrow-task"
	acCorrectionTurn3ID = "stale-task"
)

const acCorrectionSchema = `{"type":"object","additionalProperties":false,"properties":{"member":{"type":"string"},"taskGoal":{"type":"string"},"hold":{"type":"string"},"evidence":{"type":"array","items":{"type":"string"}}},"required":["member","taskGoal","hold","evidence"]}`

const acReviewSchema = `{"type":"object","additionalProperties":false,"properties":{"preferred":{"type":"string","enum":["A","B","tie"]},"reason":{"type":"string"},"evidence":{"type":"array","items":{"type":"string"}}},"required":["preferred","reason","evidence"]}`

type acTurnSpec struct {
	stage, caseName, turn string
	cwd, addDir           string
	sessionID             string
	resume                bool
	tools, allowed        string
	prompt                string
	schema                string
	budgetUSD             float64
	artifacts             []string // 相对 cwd 的落盘产物，turn 后计算实际 SHA-256
}

type acTurnOutcome struct {
	record   acTurnRecord
	envelope modelEnvelope
	output   []byte
	stderr   []byte
}

type acLiveRunner struct {
	t             *testing.T
	root          string
	gitPath       string
	claudePath    string
	claudeVersion string
	model         string
	budget        *acBudgetState
}

type acStageSummary struct {
	SchemaVersion int            `json:"schemaVersion"`
	Stage         string         `json:"stage"`
	StartedAt     time.Time      `json:"startedAt"`
	FinishedAt    time.Time      `json:"finishedAt"`
	Deadline      string         `json:"deadline"`
	SpentUSD      float64        `json:"spentUsd"`
	Blocked       bool           `json:"blocked"`
	BlockStage    string         `json:"blockStage,omitempty"`
	BlockTurn     string         `json:"blockTurn,omitempty"`
	BlockError    string         `json:"blockError,omitempty"`
	Turns         []acTurnRecord `json:"turns"`
}

// TestAutonomousCollaborationLiveRunner 是唯一的付费入口；默认 skip。
func TestAutonomousCollaborationLiveRunner(t *testing.T) {
	if os.Getenv(acLiveEnv) != "1" {
		t.Skip("set " + acLiveEnv + "=1 to run the opt-in autonomous collaboration comparison")
	}
	root := acRequiredEnv(t, acRootEnv)
	budgetData, err := os.ReadFile(acRequiredEnv(t, acBudgetFileEnv))
	if err != nil {
		t.Fatalf("读取预算文件失败: %v", err)
	}
	budgetFile, err := parseACBudget(budgetData, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	stage := os.Getenv(acStageEnv)
	if stage == "" {
		stage = "all"
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate Git: %v", err)
	}
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		t.Fatalf("locate canonical Claude Code CLI: %v", err)
	}
	claudeVersion, err := liveExecutableVersion(claudePath)
	if err != nil {
		t.Fatal(err)
	}
	runner := &acLiveRunner{
		t: t, root: root, gitPath: gitPath, claudePath: claudePath,
		claudeVersion: claudeVersion, model: liveConfiguredModel(t),
		budget: &acBudgetState{file: budgetFile},
	}
	if err := runner.restoreSpentAndBlocked(); err != nil {
		t.Fatalf("恢复跨阶段预算状态失败: %v", err)
	}
	sequence := acStageSequence(stage)
	if sequence == nil {
		t.Fatalf("未知 %s=%q（可选 preview|prepare|correction|ab|review|all）", acStageEnv, stage)
	}
	for _, current := range sequence {
		runner.runStage(current)
	}
	if runner.budget.blocked {
		runner.t.Errorf("运行器在 %s/%s 被阻断：%s；全部证据保留在 %s", runner.budget.blockStage, runner.budget.blockTurn, runner.budget.blockError, root)
	}
	runner.t.Logf("stage=%s model=%s claude=%s spent=%.6f max=%.2f deadline=%s evidence=%s",
		stage, runner.model, runner.claudeVersion, runner.budget.spent, budgetFile.MaxUSD, budgetFile.Deadline.Format(time.RFC3339), root)
}

func acRequiredEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if strings.TrimSpace(value) == "" {
		t.Fatalf("缺少必需环境变量 %s", name)
	}
	return value
}

func acStageSequence(stage string) []string {
	order := []string{"preview", "prepare", "correction", "ab", "review"}
	if stage == "all" {
		return order
	}
	for _, item := range order {
		if item == stage {
			return []string{item}
		}
	}
	return nil
}

func (r *acLiveRunner) runStage(stage string) {
	started := time.Now().UTC()
	summary := acStageSummary{SchemaVersion: 1, Stage: stage, StartedAt: started, Deadline: r.budget.file.Deadline.Format(time.RFC3339)}
	switch stage {
	case "preview":
		summary.Turns = r.stagePreview()
	case "prepare":
		summary.Turns = r.stagePrepare()
	case "correction":
		summary.Turns = r.stageCorrection()
	case "ab":
		summary.Turns = r.stageAB()
	case "review":
		summary.Turns = r.stageReview()
	default:
		r.t.Fatalf("未知 stage %q", stage)
	}
	summary.FinishedAt = time.Now().UTC()
	summary.SpentUSD = r.budget.spent
	summary.Blocked, summary.BlockStage, summary.BlockTurn, summary.BlockError = r.budget.blocked, r.budget.blockStage, r.budget.blockTurn, r.budget.blockError
	r.writeStageSummary(stage, summary)
}

func (r *acLiveRunner) writeStageSummary(stage string, summary acStageSummary) {
	stagesRoot := filepath.Join(r.root, "stages")
	if err := os.MkdirAll(stagesRoot, 0o700); err != nil {
		r.t.Fatal(err)
	}
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		r.t.Fatal(err)
	}
	// 固定文件名 + no-replace：同一实验同一 stage 只能运行一次，防止跨阶段恢复时重复计费。
	path := filepath.Join(stagesRoot, stage+".json")
	if err := writeNew(path, append(data, '\n')); err != nil {
		r.t.Fatalf("保存 stage 摘要失败（同 stage 已运行过，不重试、不覆盖；如需重跑请更换 ROOT 与预算窗口）: %v", err)
	}
	r.t.Logf("stage %s summary: %s (blocked=%v spent=%.6f turns=%d)", stage, path, summary.Blocked, summary.SpentUSD, len(summary.Turns))
}

// recordNotLaunched 为因阻断或预算耗尽而未执行的 turn 留下 not-launched 证据。
func (r *acLiveRunner) recordNotLaunched(stage, caseName, turn string) acTurnRecord {
	record := acTurnRecord{SchemaVersion: 1, Stage: stage, Case: caseName, Turn: turn, Model: r.model,
		Status: "not-launched", Error: r.budget.blockReason(), Checks: map[string]string{}, Artifacts: map[string]string{}}
	if err := acWriteTurnEvidence(filepath.Join(r.root, "evidence"), record, nil, nil); err != nil {
		r.t.Fatalf("保存 not-launched 证据失败: %v", err)
	}
	return record
}

// restoreSpentAndBlocked 从既有 stage 摘要恢复跨进程的 spent 与 blocked 状态，
// 使分阶段运行共享同一预算窗口且失败后不继续 launch。
func (r *acLiveRunner) restoreSpentAndBlocked() error {
	stagesRoot := filepath.Join(r.root, "stages")
	entries, err := os.ReadDir(stagesRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(stagesRoot, name))
		if err != nil {
			return err
		}
		var summary acStageSummary
		if err := json.Unmarshal(data, &summary); err != nil {
			return fmt.Errorf("解析 stage 摘要 %s: %w", name, err)
		}
		// 各摘要的 SpentUSD 是写摘要时刻的累计值（单调不减），恢复取最大值；
		// 求和会把同一笔消费在多个阶段摘要里重复计入。
		if summary.SpentUSD > r.budget.spent {
			r.budget.spent = summary.SpentUSD
		}
		if summary.Blocked && !r.budget.blocked {
			r.budget.blocked = true
			r.budget.blockStage, r.budget.blockTurn, r.budget.blockError = summary.BlockStage, summary.BlockTurn, summary.BlockError
		}
	}
	return nil
}

func (r *acLiveRunner) caseRoot(name string) string {
	return filepath.Join(r.root, "cases", name)
}

func (r *acLiveRunner) memberRoot(name string) string {
	return filepath.Join(r.caseRoot(name), ".steamai-vnext", "members", "owner")
}

func (r *acLiveRunner) sourceFor(contract string) string {
	env := acContractSource(contract)
	source := os.Getenv(env)
	if strings.TrimSpace(source) == "" {
		r.t.Fatalf("缺少 %s（%s 合同 source）", env, contract)
	}
	return source
}

// stagePreview 对 5 个 case 各自独立 production BuildPreview；每个 case 新建
// preview 对象（previewIdentity 绑定 caseRoot），零写入、零模型调用。
func (r *acLiveRunner) stagePreview() []acTurnRecord {
	previewsRoot := filepath.Join(r.root, "previews")
	if err := os.MkdirAll(previewsRoot, 0o700); err != nil {
		r.t.Fatal(err)
	}
	for _, name := range acCaseNames() {
		contract := acContractForCase(name)
		caseRoot := r.caseRoot(name)
		if err := os.MkdirAll(caseRoot, 0o700); err != nil {
			r.t.Fatal(err)
		}
		facts := acCaseFacts(name)
		preview, err := casebootstrap.BuildPreview(r.gitPath, r.sourceFor(contract), caseRoot, facts)
		if err != nil {
			r.t.Fatalf("BuildPreview %s 失败: %v", name, err)
		}
		record := map[string]any{
			"case": name, "contract": contract, "revision": preview.Revision,
			"sourceDigest": preview.SourceDigest, "snapshotDigest": preview.SnapshotDigest,
			"identity":     preview.Identity,
			"confirmation": casebootstrap.ConfirmationPrefix + preview.Identity,
		}
		encoded, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			r.t.Fatal(err)
		}
		if err := writeNew(filepath.Join(previewsRoot, name+".json"), append(encoded, '\n')); err != nil {
			r.t.Fatalf("保存 %s preview 失败: %v", name, err)
		}
		if err := writeNew(filepath.Join(previewsRoot, name+".human.txt"), []byte(preview.HumanPreview)); err != nil {
			r.t.Fatal(err)
		}
		r.t.Logf("preview %s contract=%s identity=%s", name, contract, preview.Identity)
	}
	r.t.Logf("全部 preview 完成；把每个 case 的 confirmation 写入 %s 后运行 stage=prepare", acConfirmationsEnv)
	return nil
}

// stagePrepare 用 exact confirmation 走 production Apply，然后物化任务输入。
func (r *acLiveRunner) stagePrepare() []acTurnRecord {
	confirmData, err := os.ReadFile(acRequiredEnv(r.t, acConfirmationsEnv))
	if err != nil {
		r.t.Fatalf("读取确认文件失败: %v", err)
	}
	var confirmations map[string]string
	if err := json.Unmarshal(confirmData, &confirmations); err != nil {
		r.t.Fatalf("确认文件必须是 {case: \"CONFIRM STEAMAI FRESH <identity>\"} JSON: %v", err)
	}
	for _, name := range acCaseNames() {
		contract := acContractForCase(name)
		facts := acCaseFacts(name)
		// Apply 内部重新 BuildPreview；source bytes 或 facts 漂移时 confirmation
		// 不再匹配，fail-closed 而不是沿用旧 preview。
		if _, err := casebootstrap.Apply(r.gitPath, r.sourceFor(contract), r.caseRoot(name), facts, confirmations[name]); err != nil {
			r.t.Fatalf("Fresh Apply %s 失败（identity 漂移请重跑 preview）: %v", name, err)
		}
		if err := casebootstrap.ValidateCurrent(r.caseRoot(name)); err != nil {
			r.t.Fatalf("Fresh 后 current 校验失败 %s: %v", name, err)
		}
		if contract != acCorrectionCase {
			taskDir := filepath.Join(r.memberRoot(name), "task")
			if err := os.MkdirAll(taskDir, 0o700); err != nil {
				r.t.Fatal(err)
			}
			question := acBuildQuestion(acQuestionForCase(name))
			if err := writeNew(filepath.Join(taskDir, "input.md"), []byte(question.Input)); err != nil {
				r.t.Fatal(err)
			}
		}
		r.t.Logf("prepared %s contract=%s", name, contract)
	}
	return nil
}

// acContractForCase 返回 case 绑定的合同：arm case 按 counterbalance，纠偏 case 用候选。
func acContractForCase(name string) string {
	for _, arm := range acArmOrder {
		if arm.Case == name {
			return arm.Contract
		}
	}
	return "candidate"
}

func acQuestionForCase(name string) string {
	for _, arm := range acArmOrder {
		if arm.Case == name {
			return arm.Question
		}
	}
	return ""
}

func acCaseFacts(name string) casebootstrap.Facts {
	if name == acCorrectionCase {
		old, _, _ := acCorrectionTaskTrio()
		return acFreshFacts(name, old)
	}
	return acFreshFacts(name, acArmTaskSpec())
}

// stageAB 按固定 counterbalance 执行 4 个单 turn Read/Write 题。
func (r *acLiveRunner) stageAB() []acTurnRecord {
	var turns []acTurnRecord
	for _, arm := range acArmOrder {
		if r.budget.blocked {
			turns = append(turns, r.recordNotLaunched("ab", arm.Case, "task"))
			continue
		}
		question := acBuildQuestion(arm.Question)
		cwd := r.memberRoot(arm.Case)
		if _, err := os.Stat(filepath.Join(cwd, "CLAUDE.md")); err != nil {
			r.t.Fatalf("case %s 未 prepare（缺少成员合同）: %v", arm.Case, err)
		}
		prompt := acArmPrompt(question)
		// addDir 不传：任务材料与成员身份（CLAUDE.md）都在成员 cwd 内自足。
		// （早先以为 --add-dir 会"超出网关响应能力"，2026-10-21 定点复现已证伪：
		// 网关对请求本身响应正常，长耗时来自 glm-5.3 自适应思考对算术/SHA-256
		// 类任务的多轮生成，实测单臂可达 8-12 分钟。）
		spec := acTurnSpec{
			stage: "ab", caseName: arm.Case, turn: "task", cwd: cwd,
			sessionID: acSessionUUID("ab/" + arm.Case),
			tools:     "Read,Write", allowed: "Read,Write(task/result.md)",
			prompt: prompt, schema: resultSchema, budgetUSD: acArmBudgetUSD,
			artifacts: []string{"task/result.md"},
		}
		turns = append(turns, r.executeTurn(spec, func(record *acTurnRecord, envelope modelEnvelope, output []byte) {
			acCheckArmResult(r.t, record, envelope, question, filepath.Join(cwd, "task", "result.md"))
		}))
	}
	return turns
}

// stageCorrection 在 candidate 合同 case 上以同一原生 session 执行三 turn：
// 读身份 -> 缩小任务（全七字段，仅目标变）-> 旧 expected 七字段的 stale change，
// 末轮必须 HOLD_STALE_TASK 且任务 bytes 零写。
func (r *acLiveRunner) stageCorrection() []acTurnRecord {
	old, narrowed, stale := acCorrectionTaskTrio()
	cwd := r.memberRoot(acCorrectionCase)
	memberFile := filepath.Join(cwd, "CLAUDE.md")
	beforeFile, err := os.ReadFile(memberFile)
	if err != nil {
		r.t.Fatalf("correction case 未 prepare: %v", err)
	}
	session := acSessionUUID("correction")
	var turns []acTurnRecord

	// addDir 不传（与 ab 相同的网关规模边界）；cwd 的 CLAUDE.md 自动加载承载成员身份。
	spec1 := acTurnSpec{stage: "correction", caseName: acCorrectionCase, turn: acCorrectionTurn1ID, cwd: cwd,
		sessionID: session, tools: "Read", allowed: "Read",
		prompt: "读取本目录 CLAUDE.md，确认成员身份与「当前正式任务」节。返回 JSON：member=成员名称、taskGoal=当前正式任务节「目标」的原样文本、hold=空字符串、evidence=[\"CLAUDE.md\"]。只读，不修改任何文件。",
		schema: acCorrectionSchema, budgetUSD: 0.5}
	turns = append(turns, r.executeTurn(spec1, func(record *acTurnRecord, envelope modelEnvelope, output []byte) {
		var result acCorrectionResult
		if record.Status == "completed" && acDecodeStructured(r.t, envelope, output, &result) == nil {
			record.Checks["member"] = strconv.FormatBool(result.Member == "owner")
			record.Checks["taskGoalMatchesCurrent"] = strconv.FormatBool(result.TaskGoal == old.Goal)
		}
	}))

	if r.budget.blocked {
		turns = append(turns, r.recordNotLaunched("correction", acCorrectionCase, acCorrectionTurn2ID))
		turns = append(turns, r.recordNotLaunched("correction", acCorrectionCase, acCorrectionTurn3ID))
		return turns
	}
	spec2 := acTurnSpec{stage: "correction", caseName: acCorrectionCase, turn: acCorrectionTurn2ID, cwd: cwd,
		sessionID: session, resume: true,
		tools: "Read,Edit", allowed: "Read,Edit(CLAUDE.md)",
		prompt: acTaskChangePrompt(old, narrowed, false), schema: acCorrectionSchema, budgetUSD: 0.75}
	turns = append(turns, r.executeTurn(spec2, func(record *acTurnRecord, envelope modelEnvelope, output []byte) {
		if record.Status != "completed" {
			return
		}
		var result acCorrectionResult
		if err := acDecodeStructured(r.t, envelope, output, &result); err != nil {
			return
		}
		after, err := os.ReadFile(memberFile)
		if err != nil {
			record.Checks["memberFile"] = "unreadable"
			return
		}
		if strings.Count(string(beforeFile), acGoalRow(old)) != 1 {
			r.t.Fatalf("成员合同缺少唯一目标行，无法校验逐字节变更")
		}
		expected := strings.Replace(string(beforeFile), acGoalRow(old), acGoalRow(narrowed), 1)
		record.Checks["onlyGoalRowChanged"] = strconv.FormatBool(string(after) == expected)
		record.Checks["holdEmpty"] = strconv.FormatBool(result.Hold == "")
		record.Checks["taskGoalMatchesNarrowed"] = strconv.FormatBool(result.TaskGoal == narrowed.Goal)
	}))

	if r.budget.blocked {
		turns = append(turns, r.recordNotLaunched("correction", acCorrectionCase, acCorrectionTurn3ID))
		return turns
	}
	// stale turn 的零写基线是 turn2 之后的实际文件与 case 树，而不是初始状态。
	actualAfterTurn2, err := os.ReadFile(memberFile)
	if err != nil {
		r.t.Fatal(err)
	}
	treeAfterTurn2, err := treeIdentity(r.caseRoot(acCorrectionCase))
	if err != nil {
		r.t.Fatal(err)
	}
	spec3 := acTurnSpec{stage: "correction", caseName: acCorrectionCase, turn: acCorrectionTurn3ID, cwd: cwd,
		sessionID: session, resume: true,
		tools: "Read,Edit", allowed: "Read,Edit(CLAUDE.md)",
		prompt: acTaskChangePrompt(old, stale, true), schema: acCorrectionSchema, budgetUSD: 0.75}
	turns = append(turns, r.executeTurn(spec3, func(record *acTurnRecord, envelope modelEnvelope, output []byte) {
		if record.Status != "completed" {
			return
		}
		var result acCorrectionResult
		if err := acDecodeStructured(r.t, envelope, output, &result); err != nil {
			return
		}
		record.Checks["holdStaleTask"] = strconv.FormatBool(result.Hold == "HOLD_STALE_TASK")
		record.Checks["taskGoalStillNarrowed"] = strconv.FormatBool(result.TaskGoal == narrowed.Goal)
		record.Checks["zeroWrite"] = strconv.FormatBool(acZeroWriteCheck(r.caseRoot(acCorrectionCase), memberFile, actualAfterTurn2, treeAfterTurn2) == nil)
	}))
	return turns
}

// stageReview 对两题各做一次匿名盲审：一次 Read 读 packet，冻结 decision 后 reveal。
func (r *acLiveRunner) stageReview() []acTurnRecord {
	var turns []acTurnRecord
	for _, questionID := range []string{"q1", "q2"} {
		if r.budget.blocked {
			turns = append(turns, r.recordNotLaunched("review", "review-"+questionID, "blind"))
			continue
		}
		mapping := acReviewMapping(questionID)
		question := acBuildQuestion(questionID)
		outputs := map[string]string{}
		complete := true
		for label, caseName := range mapping {
			recordData, err := os.ReadFile(filepath.Join(r.root, "evidence", caseName, "task", "run.json"))
			if err != nil {
				complete = false
				break
			}
			var record acTurnRecord
			if json.Unmarshal(recordData, &record) != nil || record.Status != "completed" {
				complete = false
				break
			}
			data, err := os.ReadFile(filepath.Join(r.memberRoot(caseName), "task", "result.md"))
			if err != nil {
				complete = false
				break
			}
			outputs[label] = string(data)
		}
		reviewRoot := filepath.Join(r.root, "review", questionID)
		if !complete {
			r.t.Logf("review %s skipped: arms not both completed", questionID)
			continue
		}
		if err := os.MkdirAll(reviewRoot, 0o700); err != nil {
			r.t.Fatal(err)
		}
		packet := acReviewPacket(question, outputs)
		packetPath := filepath.Join(reviewRoot, "packet.md")
		if err := writeNew(packetPath, []byte(packet)); err != nil {
			r.t.Fatal(err)
		}
		spec := acTurnSpec{stage: "review", caseName: "review-" + questionID, turn: "blind", cwd: reviewRoot,
			sessionID: acSessionUUID("review/" + questionID), tools: "Read", allowed: "Read(packet.md)",
			prompt: "读取 packet.md 并按其中的匿名对照任务给出判断。返回 JSON preferred=A|B|tie、reason、evidence=[\"packet.md\"]。不读取其它文件，不猜测输出来源。",
			schema: acReviewSchema, budgetUSD: acReviewBudgetUSD}
		turns = append(turns, r.executeTurn(spec, func(record *acTurnRecord, envelope modelEnvelope, output []byte) {
			if record.Status != "completed" {
				return
			}
			var decision struct {
				Preferred string `json:"preferred"`
				Reason    string `json:"reason"`
			}
			if err := acDecodeStructured(r.t, envelope, output, &decision); err != nil {
				return
			}
			record.Checks["preferred"] = decision.Preferred
			// decision 冻结（随本 turn 证据 no-replace 保存）之后才写 reveal 映射。
			frozen := map[string]string{"preferred": decision.Preferred, "reason": decision.Reason}
			encoded, err := json.MarshalIndent(frozen, "", "  ")
			if err != nil {
				r.t.Fatal(err)
			}
			if err := writeNew(filepath.Join(reviewRoot, "decision.json"), append(encoded, '\n')); err != nil {
				r.t.Fatal(err)
			}
			reveal := map[string]string{
				"A": mapping["A"] + "/" + acArmContract(mapping["A"]),
				"B": mapping["B"] + "/" + acArmContract(mapping["B"]),
			}
			encoded, err = json.MarshalIndent(reveal, "", "  ")
			if err != nil {
				r.t.Fatal(err)
			}
			if err := writeNew(filepath.Join(reviewRoot, "reveal.json"), append(encoded, '\n')); err != nil {
				r.t.Fatal(err)
			}
		}))
	}
	return turns
}

func acArmContract(caseName string) string {
	for _, arm := range acArmOrder {
		if arm.Case == caseName {
			return arm.Contract
		}
	}
	return "unknown"
}

// acReviewMapping 固定匿名分配：每题 A/B 与 baseline/candidate 交叉，避免位置偏差。
func acReviewMapping(questionID string) map[string]string {
	switch questionID {
	case "q1":
		return map[string]string{"A": "arm-1", "B": "arm-3"}
	default:
		return map[string]string{"A": "arm-2", "B": "arm-4"}
	}
}

func acReviewPacket(question acQuestion, outputs map[string]string) string {
	return fmt.Sprintf("# 匿名对照盲审（%s）\n\n任务要求：\n%s\n\n实现 A 的 task/result.md（SHA-256: %s）：\n\n```text\n%s```\n\n实现 B 的 task/result.md（SHA-256: %s）：\n\n```text\n%s```\n\n只依据任务要求判断哪份输出更符合要求（相同则 tie）。输出内容与其 SHA-256 均已核对。\n",
		question.ID, acArmTaskRequirement(question), Hash([]byte(outputs["A"])), outputs["A"], Hash([]byte(outputs["B"])), outputs["B"])
}

func acArmTaskRequirement(question acQuestion) string {
	if question.ID == "q1" {
		return "读取 task/input.md，计算所有 label 数值之和（保留负号），把结果写入 task/result.md，内容恰为一行 \"sum: <总和>\"。"
	}
	return "读取 task/input.md，按映射表解析 Queries 中每个键，按出现顺序输出对应值，把结果写入 task/result.md，内容恰为一行 \"resolved: <v1,v2,...>\"。"
}

func acArmPrompt(question acQuestion) string {
	return fmt.Sprintf(`你是本成员目录身份的正式成员，按 CLAUDE.md 的当前正式任务执行本轮。
1. %s
2. structured_output 要求：summary 填最终答案值；evidence 恰好两项 "task/input.md" 与 "task/result.md"（按此顺序，不含其他标注）；limitations 为空数组；safetyGate 填 pass（全部为本地合成数据）。
边界：不读取授权目录外路径，不使用网络、shell、MCP 或子 agent。`, acArmTaskRequirement(question))
}

func acGoalRow(spec acTaskSpec) string {
	return "- 目标：`" + spec.Goal + "`"
}

func acTaskChangePrompt(expected, next acTaskSpec, delayed bool) string {
	prefix := "Commander 正式任务变更消息（成员合同 compare-before-update）："
	if delayed {
		prefix = "Commander 延迟的正式任务变更消息（生成于上一条变更之前，可能已过期）："
	}
	return fmt.Sprintf(`%s

expected current task:
%s

new current task:
%s

按 CLAUDE.md 成员合同：仅当「当前正式任务」节与 expected 逐项一致时，用 Edit 只更新发生变化的那一项字段行，文件其余部分保持逐字节不变；随后用 Read 复核。返回 JSON：member、taskGoal=当前文件中「目标」的原样文本、hold=空字符串、evidence=["CLAUDE.md"]。若与 expected 不一致，不修改任何文件，返回 hold="HOLD_STALE_TASK"、taskGoal=当前实际目标。`,
		prefix, acTaskBlock(expected), acTaskBlock(next))
}

type acCorrectionResult struct {
	Member   string   `json:"member"`
	TaskGoal string   `json:"taskGoal"`
	Hold     string   `json:"hold"`
	Evidence []string `json:"evidence"`
}

// acDecodeStructured 从原始 stdout 事件流提取最后的 structured_output 节点解析。
// envelope.StructuredOutput 是 production 固定的 modelResult 形状，自定义 schema
// （correction/review）的字段不经过它；只有原始事件里保留了完整结构。
// CLI 2.1.273 实测 `--output-format json --verbose` 输出单个 JSON 事件数组；
// 同时兼容逐行 JSONL，防未来格式回摆。
func acDecodeStructured(t *testing.T, envelope modelEnvelope, output []byte, target any) error {
	t.Helper()
	var raw json.RawMessage
	observe := func(data []byte) {
		var event struct {
			StructuredOutput json.RawMessage `json:"structured_output"`
		}
		if json.Unmarshal(data, &event) == nil && len(event.StructuredOutput) > 0 {
			raw = event.StructuredOutput
		}
	}
	var events []json.RawMessage
	if json.Unmarshal(output, &events) == nil {
		for _, data := range events {
			observe(data)
		}
	} else {
		for _, line := range bytes.Split(bytes.TrimSpace(output), []byte("\n")) {
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}
			observe(line)
		}
	}
	if len(raw) == 0 {
		return errors.New("stdout 事件流中没有 structured_output 节点")
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("structured_output 与 schema 不符: %v", err)
	}
	return nil
}

// acCheckArmResult 只以实际落盘为准：result.md 必须存在、内容恰为期望行、
// evidence 恰为两条授权路径；自报 summary 只作记录，不单独决定 success。
// 曾要求 result.md 的上下文内 SHA-256，实测 glm-5.3 无哈希工具时对该不可能
// 要求反复磨转（单臂 8-12 分钟，撞上游连接时限），2026-10-22 改为可诚实完成的
// 路径 evidence；result.md 实际 SHA 仍记录在 artifacts 供审查。
func acCheckArmResult(t *testing.T, record *acTurnRecord, envelope modelEnvelope, question acQuestion, resultPath string) {
	t.Helper()
	if record.Status != "completed" {
		return
	}
	data, err := os.ReadFile(resultPath)
	if err != nil {
		record.Checks["resultFile"] = "missing"
		return
	}
	actualSHA := Hash(data)
	record.Checks["resultContent"] = strconv.FormatBool(strings.TrimSpace(string(data)) == question.ResultLine)
	record.Checks["summaryMatchesAnswer"] = strconv.FormatBool(strings.TrimSpace(envelope.StructuredOutput.Summary) == question.Answer)
	record.Checks["evidencePathsExact"] = strconv.FormatBool(len(envelope.StructuredOutput.Evidence) == 2 &&
		envelope.StructuredOutput.Evidence[0] == "task/input.md" && envelope.StructuredOutput.Evidence[1] == "task/result.md")
	record.Artifacts["task/result.md"] = actualSHA
}

// executeTurn 统一执行、校验、落证据；任何失败也 no-replace 保存。
func (r *acLiveRunner) executeTurn(spec acTurnSpec, check func(*acTurnRecord, modelEnvelope, []byte)) acTurnRecord {
	outcome := r.runTurn(spec)
	if check != nil && outcome.record.Status == "completed" {
		check(&outcome.record, outcome.envelope, outcome.output)
	}
	if err := acWriteTurnEvidence(filepath.Join(r.root, "evidence"), outcome.record, outcome.output, outcome.stderr); err != nil {
		r.t.Fatalf("保存 turn 证据失败 %s/%s: %v", spec.caseName, spec.turn, err)
	}
	r.t.Logf("%s/%s/%s status=%s cost=%s checks=%v", spec.stage, spec.caseName, spec.turn,
		outcome.record.Status, acCostText(outcome.record.CostUSD), outcome.record.Checks)
	return outcome.record
}

func acCostText(cost *float64) string {
	if cost == nil {
		return "unreported"
	}
	return strconv.FormatFloat(*cost, 'f', 6, 64)
}

// runTurn 启动一次真实 Claude Code 调用：suspended + Job Object（复用 production
// configureArmProcess）、单调用 180s 与共享 deadline 双上限、复用 decodeModelOutput
// 与模型身份/费用合同；缺 model/cost 立即阻断后续 launch。
func (r *acLiveRunner) runTurn(spec acTurnSpec) acTurnOutcome {
	now := time.Now().UTC()
	record := acTurnRecord{SchemaVersion: 1, Stage: spec.stage, Case: spec.caseName, Turn: spec.turn,
		Model: r.model, Checks: map[string]string{}, Artifacts: map[string]string{}}
	if gateErr := r.budget.gate(spec.stage, spec.caseName+"/"+spec.turn, spec.budgetUSD, now); gateErr != nil {
		record.Status, record.Error = "not-launched", gateErr.Error()
		return acTurnOutcome{record: record}
	}
	timeout := time.Until(r.budget.file.Deadline)
	if timeout > acMaxCallSeconds*time.Second {
		timeout = acMaxCallSeconds * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	args := []string{
		"-p", spec.prompt,
		"--tools", spec.tools,
		"--allowedTools", spec.allowed,
		"--disallowedTools", "Bash,PowerShell,WebFetch,WebSearch,Task,Agent,SendMessage,NotebookEdit,TodoWrite",
		"--permission-mode", "acceptEdits",
		"--strict-mcp-config", "--mcp-config", `{"mcpServers":{}}`,
		"--setting-sources", "", // 不加载 user/project/local settings（hooks/plugins）；managed 与 CLAUDE.md 自动加载不受影响
		"--no-chrome",
		"--model", r.model,
		"--max-budget-usd", strconv.FormatFloat(spec.budgetUSD, 'f', -1, 64),
		"--output-format", "json", "--verbose",
		"--json-schema", spec.schema,
	}
	if spec.addDir != "" {
		args = append(args, "--add-dir", spec.addDir)
	}
	if spec.resume {
		args = append(args, "--resume", spec.sessionID)
	} else {
		args = append(args, "--session-id", spec.sessionID)
	}
	cmd := exec.CommandContext(ctx, r.claudePath, args...)
	cmd.Dir = spec.cwd
	cmd.Env = sanitizedEnvironment(os.Environ())
	control, controlErr := configureArmProcess(cmd)
	if controlErr != nil {
		control = &armProcessControl{}
	}
	defer control.close()
	cmd.WaitDelay = 5 * time.Second
	cmd.Cancel = func() error { return control.cancel(cmd) }
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	started := time.Now()
	runErr := controlErr
	if runErr == nil {
		runErr = cmd.Start()
	}
	if runErr == nil {
		if attachErr := control.attach(cmd); attachErr != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			runErr = attachErr
		} else {
			runErr = cmd.Wait()
		}
	}
	output, stderrBytes := append([]byte(nil), stdout.Bytes()...), append([]byte(nil), stderr.Bytes()...)
	status, errorText := "completed", ""
	if ctx.Err() == context.DeadlineExceeded {
		status, errorText = "timeout", "turn exceeded shared deadline/call cap"
	} else if runErr != nil {
		status, errorText = "failed", runErr.Error()
	}
	var envelope modelEnvelope
	if status == "completed" {
		var models []string
		var decodeErr error
		envelope, models, decodeErr = decodeModelOutput(output)
		if decodeErr != nil {
			status, errorText = "invalid-output", decodeErr.Error()
		} else if identityErr := acValidateTurnIdentity(envelope, models, r.model); identityErr != nil {
			status, errorText = "invalid-output", identityErr.Error()
		}
	}
	record.Status, record.Error, record.SessionID = status, errorText, acSessionIDFromOutput(output)
	record.OutputSHA256, record.OutputBytes = Hash(output), len(output)
	record.StderrSHA256, record.StderrBytes = Hash(stderrBytes), len(stderrBytes)
	record.ElapsedMillis = time.Since(started).Milliseconds()
	if envelope.TotalCostUSD != nil {
		cost := *envelope.TotalCostUSD
		record.CostUSD = &cost
	}
	for _, rel := range spec.artifacts {
		data, err := os.ReadFile(filepath.Join(spec.cwd, filepath.FromSlash(rel)))
		if err == nil {
			record.Artifacts[rel] = Hash(data)
		}
	}
	r.budget.charge(spec.stage, spec.caseName+"/"+spec.turn, status, record.CostUSD, errorText)
	return acTurnOutcome{record: record, envelope: envelope, output: output, stderr: stderrBytes}
}

// acValidateTurnIdentity 复用模型身份合同：主会话 assistant 消息模型必须全部匹配
// 冻结选择，且 result 必须自报正向费用；自定义 schema 的 turn 也遵守同一合同。
func acValidateTurnIdentity(envelope modelEnvelope, models []string, expectedModel string) error {
	if envelope.IsError {
		return errors.New("turn 返回 is_error")
	}
	if len(models) == 0 {
		return errors.New("turn 缺少主会话 assistant 消息，无执行身份证据")
	}
	for _, model := range models {
		if model != strings.TrimSuffix(expectedModel, "[1m]") && model != expectedModel {
			return errors.New("turn actual model 主会话消息与冻结选择不一致: " + model)
		}
	}
	if _, present := envelope.ModelUsage[expectedModel]; !present {
		return errors.New("turn 缺少冻结选择模型的用量条目")
	}
	if envelope.TotalCostUSD == nil || *envelope.TotalCostUSD <= 0 {
		return errors.New("turn 未自报正向 total_cost_usd")
	}
	return nil
}

func acSessionIDFromOutput(output []byte) string {
	var events []json.RawMessage
	if json.Unmarshal(output, &events) != nil || len(events) == 0 {
		return ""
	}
	var tail struct {
		SessionID string `json:"session_id"`
	}
	if json.Unmarshal(events[len(events)-1], &tail) != nil {
		return ""
	}
	return tail.SessionID
}

// acSessionUUID 从固定 seed 派生合法 UUID v4，使同 stage 重跑记录可对账。
func acSessionUUID(seed string) string {
	sum := sha256.Sum256([]byte("steamai-autonomous-collaboration\x00" + seed))
	sum[6] = (sum[6] & 0x0f) | 0x40
	sum[8] = (sum[8] & 0x3f) | 0x80
	hexed := hex.EncodeToString(sum[:16])
	return hexed[0:8] + "-" + hexed[8:12] + "-" + hexed[12:16] + "-" + hexed[16:20] + "-" + hexed[20:32]
}
