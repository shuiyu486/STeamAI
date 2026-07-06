# Agent Team dry-run script

## 读取指南

- 本文件是 `docs/agent-team-rollout-plan.md` R0 批次的产物，定义一次可重复的端到端契约压测脚本。
- 目的：手动按 `common/policies/agent-team.md` 契约跑一次 main → feature → reviewer → confirmed 全流程，压测 packet schema 与 output contract 是否够用，**不写 runtime**。
- 跑完后把字段缺口、合并痛点、handoff 还原能力记录到本文件 §6，供 R2 回写 policy。
- 本脚本不引入真实样本、trace、dump；mock 对象仅用纯文本/伪 RVA 占位，让 packet 字段有内容。
- dry-run **不执行任何 heavy-tool 动作**（full trace/debug/inject/patch/dump/network）；confirmed 只写临时 case workspace，不写 authority CSV、不写 kit 模板。

## 临时 case 准备

临时 case 路径（kit 仓库外，用完即删）：

```text
C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun
```

初始化命令（已在 kit 仓库执行）：

```powershell
.\rekit\rekit.ps1 -Command init -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re -ProjectName agent-team-dryrun
```

每次重跑前清理重建：

```powershell
Remove-Item -Recurse -Force 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
.\rekit\rekit.ps1 -Command init -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re -ProjectName agent-team-dryrun
```

## Mock 分析对象

为避免引入真实样本，用纯文本 mock 占位。设 mock 目标为一段伪 VMP handler：

```text
mock_target: handler_0x40A010
kind: vm-handler-entry
pseudo_bytes: 68 AA BB CC DD  // push imm32
notes: 占位，非真实样本字节，仅用于让 packet 字段有内容
question: 该 handler 的语义候选是什么？
```

伪 RVA `0x40A010` 仅作占位标记，不对应真实二进制；dry-run 产物不回流 kit。

## 角色分配（同一会话切换视角即可，不必真启子 agent）

为压测契约，R1 用同一会话切换视角模拟四类 agent。R5 才考虑真启子 agent。

| 步骤 | 视角 | 产物 |
|---|---|---|
| S1 | Main agent | task packet（派给 feature lane） |
| S2 | Feature agent | evidence packet + candidate packet（写 lane workspace） |
| S3 | Reviewer agent | review packet + verdict（只读） |
| S4 | Main agent | 合并 verdict 台账；gate 通过后写 confirmed（仅临时 workspace） |
| S5 | Handoff curator | 从 packet 还原 handoff，验证可接手性 |

## 步骤脚本

### S1：Main agent 派 task packet

进入临时 case 启动 Claude Code：

```powershell
cd C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun
claude
```

先开 feature 支线：

```text
/rekit start handler-0x40A010
```

然后 main agent 产出 task packet（按 `common/policies/agent-team.md` §Task packet 字段）：

```yaml
task_id: dryrun-task-001
lane: feature-handler-0x40A010
goal: 给出 handler_0x40A010 的语义候选结论
inputs:
  - mock_target: handler_0x40A010
  - pseudo_bytes: 68 AA BB CC DD
allowed_reads:
  - references/vmp-re/workflow-template.md
  - references/vmp-re/progressive-disclosure.md
allowed_writes:
  - .rekit/lanes/feature-handler-0x40A010/workspace
stop_conditions:
  - 证据不足以支持 medium 以上 confidence -> defer
  - 需要 full trace / debug / dump -> stuck-point packet，不执行
output_contract: evidence,candidate
```

记录：task packet 字段是否够 feature agent 理解任务边界？是否有字段缺失？

### S2：Feature agent 产 evidence + candidate packet

切换为 feature agent 视角，读 allowed_reads，对 mock 目标做轻量分析（纯文本推理，不调工具），产出：

evidence packet：

```yaml
subject: handler_0x40A010
evidence:
  - kind: disasm
    ref: mock_target.pseudo_bytes:0x0
    summary: push imm32 (68 AA BB CC DD) 形似 push 0xDDCCBBAA
  - kind: source
    ref: references/vmp-re/progressive-disclosure.md
    summary: push imm32 是 vm-handler 常见语义之一
confidence: low
limitations:
  - mock 占位，非真实字节
  - 未 cross-run，未做 value-flow
```

candidate packet：

```yaml
candidate_id: dryrun-cand-001
subject: handler_0x40A010
claim: 该 handler 语义为 push imm32，操作数为 0xDDCCBBAA
evidence_refs:
  - dryrun-evidence-001
verifier: pending
risk: low
next_action: review
```

把 evidence/candidate packet 写入：

```text
.rekit/lanes/feature-handler-0x40A010/workspace/dryrun-001.md
```

记录：evidence/candidate 字段是否够用？confidence 三档够不够？limitations 字段是否被实际填写？

### S3：Reviewer agent 出 review packet + verdict

切换为 reviewer 视角，只读 feature 产物，按 review packet 字段输出：

```yaml
review_id: dryrun-review-001
candidate: dryrun-cand-001
lens: correctness
scope: handler_0x40A010 语义判断
question: claim 是否被 evidence 充分支持？
output_contract: decision,confidence,evidence,risk,next_action
```

verdict（按 `common/policies/subagents.md` output contract）：

```text
item: dryrun-cand-001
decision: defer
confidence: low
evidence: push imm32 形态匹配，但仅 mock 占位
risk: 证据不足以上 confirmed
next_action: 需真实 trace 或 cross-run；当前保持 candidate
tier_used: L1
tool_scope: packet_only
```

记录：review packet 的 lens 是否够分？output contract 字段在主 agent 合并时是否够用？`tier_used`/`tool_scope` 是否有实际意义？

### S4：Main agent 合并 + gate

切回 main agent，合并 verdict 台账：

```text
accepted: (none)
rejected: (none)
deferred: dryrun-cand-001 (reason: low confidence, mock 占位)
```

gate 判断：candidate confidence=low + evidence 仅为 mock，**不满足 confirmed 条件**，因此：

- 不写 authority CSV（`captures/vm_opcode_semantics_confirmed.csv`）。
- 仅在临时 case workspace 记录一次 decision 事件（手写，因为 ledger runtime 未实现）：

```text
.rekit/lanes/feature-handler-0x40A010/workspace/decision-dryrun-001.md
  decision: defer
  reason: low confidence + mock evidence
  superseded_by: (none)
```

记录：gate 规则是否清晰？defer 后下一步提示是否够？如果没有 ledger runtime，手写 decision 文件是否能被下一会话消费？

### S5：Handoff curator 验证还原

执行：

```text
/rekit handoff
/rekit handoff handler-0x40A010
```

然后检查生成的 handoff 是否能仅凭 packet + workspace 文件还原：

- 当前工作线、开放 candidate、defer 理由、下一步。
- 新会话只读 handoff 能否知道"不要重复走旧路"。

记录：handoff 是否引用了 packet？字段是否够接手？是否需要把 packet id 写进 handoff 模板？

## 字段缺口记录（R2 输入）

R1 已跑完 S1-S5，暴露的缺口如下（R2 回写 policy/manifest 的依据）：

### G1：handoff 不引用 workspace packet 文件

- 现象：`/rekit handoff handler-0x40a010` 生成的接手文档只指向 `.rekit/lanes/<id>/prompts/RESUME.md`，不引用 workspace 内的 packet 文件（如 `dryrun-001.md`）。
- 影响：新会话只读 handoff 无法发现已产出的 evidence/candidate/decision packet，无法还原工作线状态。
- R2 建议：在 `common/policies/handoff.md` 或 `B3.Commands.ps1` 的 lane handoff 模板中，增加"workspace packet 引用"区段，扫描 workspace 内 `.md` packet 文件并列出路径。或至少在 policy 里规定 handoff 必须列出当前 open candidate 的 packet 文件路径。

### G2：overview 共享事实不反映手写 packet

- 现象：手动产出的 evidence/candidate/decision packet 写在 workspace，但 `overview` 的 observation/request/candidate/publication 计数全为 0。
- 根因：runtime 的 facts JSONL（`.rekit/facts/*.jsonl`）只在 `continue` auto 流程中由 `B3.Auto.ps1` 从 lane CSV/workspace 扫描写入，没有"手动 append 单条 evidence/candidate/decision"的入口；手写 packet 文件不被 runtime 识别。
- 影响：Agent Team 契约里"agent 产出 candidate"无法被 runtime 感知，overview stuck statistics 形同虚设。
- R2 建议：先在 `common/policies/evidence.md` 明确"packet 文件 vs facts JSONL event"的关系——packet 是 agent 产出物（文件），event 是 runtime 从 packet 抽取的账本条目。R3 决策门再决定是否补"手动 append event"runtime 入口（属 Phase 5 ledger runtime 切片）。

### G3：packet schema 字段缺口

- `evidence_id`：`agent-team.md` 的 evidence packet 模板没有显式 `evidence_id` 字段，但 candidate 的 `evidence_refs` 需要引用它。R2 应在 evidence packet 模板补 `evidence_id`。
- `event_id` / `superseded_by` / `status`：decision 事件需要这些字段，但 `agent-team.md` 状态流只写了 `draft -> candidate -> review -> confirmed|rejected|superseded|needs_more_evidence`，没给 decision event 的字段模板。R2 应补 decision event 字段定义。
- `tier_used` / `tool_scope`：在 L1 packet-only 场景下意义有限但占位无害，保留；在 `subagents.md` 补一句说明"L1 可省略 tool_scope"。

### G4：workspace 路径契约不一致

- 现象：`agent-team.md` / dry-run 脚本示例写 `.rekit/lanes/<id>/workspace`，实际 runtime 由 manifest `workstreamDefaults` 驱动到 `captures/feature_analysis/<id>`。
- 影响：文档示例误导，agent 可能写错路径。
- R2 建议：在 `agent-team.md` 的 task packet `allowed_writes` 说明里改成"用 manifest 驱动的工作区路径，由 `/rekit start` 输出为准"，不写死 `.rekit/lanes/...`。

### G5：defer 状态持久化无标准位置

- 现象：手写 decision event 只能放 workspace 自由文件，runtime 不消费；rejected/superseded 也无标准文件位置。
- 影响：下一会话无法通过 runtime 知道"为什么 defer、不要重复走旧路"。
- R2 建议：先在 policy 里规定 decision event 写入 `.rekit/facts/decisions.jsonl`（即使手动），格式与 `docs/evidence-ledger.md` 对齐；R3 再决定是否做 runtime append 入口。

### G6：lane 路径大小写

- 现象：`start handler-0x40A010` 生成的 lane id 是 `feature-handler-0x40a010`（A 被小写化），packet 里 `lane: feature-handler-0x40a010` 需对齐。
- 影响：小，但 packet 引用 lane 时必须用 runtime 实际 id。
- R2 建议：在 policy 里注明 lane id 由 runtime 规范化（小写），packet 字段沿用 runtime id。


## 验证标准

- S1-S5 全部走完，无步骤阻塞。
- 字段缺口记录非空（dry-run 目的就是暴露缺口；如果全无缺口，说明 mock 太浅，需要加复杂度）。
- 未执行任何真实 heavy-tool 动作；未写 authority CSV；未写 kit 模板。
- 临时 case `status`/`doctor` 仍通过。
- 产物仅留在临时 case workspace 与本文件 §6。

## 清理

R0 验证通过后保留临时 case 供 R1 复跑；R2 完成后删除：

```powershell
Remove-Item -Recurse -Force 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
```

确认 kit 仓库 `git status` 干净（除计划/脚本文档外）。
