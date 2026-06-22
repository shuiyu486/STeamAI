# VMP RE Agent Team 工作方式

## 读取指南

- 新会话先读 `CLAUDE.local.md` 和 `README.md`，需要理解多 Agent 协作时再读本文件。
- 本文件说明 Agent Team 的职责、packet、证据状态和人工确认边界；具体 VMP 技术路线见 `workflow-template.md`。
- 工具选择与止损见 `toolchain-router.md`；工作线上下文和写入权限见 `lane-collaboration.md`。
- 本文件是 pack managed doc，会随 `/rekit sync` 下发到 case；不要写入真实样本名、RVA/VA、trace/dump 路径或本机绝对路径。

## 实施摘要

VMP RE case 的长期推进方式是：**主 agent 做决策与收敛，功能支线 agent 做窄范围探索，只读 reviewer 做复核，工具链按 router 执行，confirmed 数据由主线在验证后写入**。

Agent 可以并行、短命；工作线、证据和 handoff 必须持久。任何自动结论都先是 candidate，只有满足 evidence、verifier、schema、backup、diff、无冲突和必要人工确认后，才能进入 confirmed / authority 文件。

## 执行清单

- [ ] 明确当前会话接手 `main` 还是某条功能支线。
- [ ] 将任务拆成 task packet，说明目标、输入、边界、可写位置和停止条件。
- [ ] 只把窄范围 packet 分派给子 agent；不要让子 agent 自行探索全仓库或全 trace。
- [ ] 将发现写为 evidence packet 或 candidate packet，不直接写 confirmed。
- [ ] 对 candidate 做 review packet 复核，必要时交给只读 reviewer。
- [ ] 主线验证并更新 authority / confirmed 文件。
- [ ] 每轮结束更新 handoff 或 lane resume。

## 验证标准

- 功能支线不能修改 confirmed CSV、routine IR、`task-handoff.md` 或共享 IDB 状态。
- reviewer 只输出 verdict、evidence、risk、next_action，不执行写入和重型工具。
- confirmed 写入必须能追溯到 candidate、evidence、verifier 和本轮变更 diff。
- heavy trace、debug、inject、patch、dump 等动作必须有用户确认、预算和止损条件。
- Markdown 只保存摘要和证据定位；长 trace、反汇编、反编译和 tool log 放 sidecar。

## 风险与注意事项

- 不把 IDA F5 当作唯一事实源；关键算法/handler 结论需要指令级证据或 trace 复验。
- 不让多 agent 并发写同一个 IDB、confirmed CSV 或 handoff。
- 不把“猜算法”作为默认动作；先追 I/O、sink、producer、value-flow，再归纳候选。
- 不把未验证 candidate 回流到 pack 模板；可复用经验走 `/rekit promote` review-first。
- 不把用户一句“继续”解释为允许重型动态调试或破坏性写入。

## 1. Agent 角色

| 角色 | 职责 | 可写 | 不做 |
|---|---|---|---|
| 主 agent | 选择工作线、拆任务、收敛证据、执行确认后的 authority 写入、更新 handoff | canonical 文件、`.rekit/**`、所选工作线允许的文件 | 不把功能支线候选直接当 confirmed |
| 功能支线 agent | 围绕功能入口、字符串/import/xref、VM 阻塞点收集证据和候选 | 自己的 workspace | 不写 confirmed CSV、routine IR、`task-handoff.md` |
| Tooling agent | 评估工具适配、记录 recipe、生成 tooling candidate | tooling workspace 或候选文件 | 不把外部工具变硬依赖 |
| Reviewer agent | 只读复核 candidate、trace/value-flow 摘要、tooling diff | 不写 | 不扩大范围、不运行重型工具 |
| Verifier / gate | 检查 evidence、schema、冲突、备份、diff、预算和人工确认条件 | gate verdict | 不创造业务结论 |
| 人类确认者 | 对 confirmed、重型工具、外部副作用、冲突处理和回流范围做最终确认 | 通过对话授权 | 不需要手工合并普通事实 |

## 2. Packet 类型

### Task packet

用于分派任务，字段建议：

```yaml
task_id: <stable-id>
lane: main | feature-<name> | tooling-<name>
goal: <要回答的问题>
inputs:
  - <文件或 sidecar 定位>
allowed_reads:
  - <范围>
allowed_writes:
  - <workspace 或 none>
stop_conditions:
  - <何时停止或升级>
output_contract: evidence | candidate | review | request
```

### Evidence packet

用于保存证据摘要，字段建议：

```yaml
subject: <handler/function/tool finding>
evidence:
  - kind: trace | xref | disasm | value-flow | tool-output
    ref: <文件:行 或 sidecar path + filter>
    summary: <短摘要>
confidence: low | medium | high
limitations:
  - <低样本、alias、缺少 cross-run 等>
```

### Candidate packet

用于提交候选结论：

```yaml
candidate_id: <stable-id>
subject: <handler/function/algorithm/tooling rule>
claim: <候选结论>
evidence_refs:
  - <evidence id 或文件定位>
verifier: pending | accepted | rejected | needs_more_evidence
risk: low | medium | high
next_action: confirm | focused-review | request-authority | reject
```

### Review packet

用于只读复核：

```yaml
review_id: <stable-id>
candidate: <candidate-id 或摘要>
lens: correctness | evidence | simplicity | tooling-risk | schema
scope: <必须保持窄范围>
question: <需要 reviewer 判断的问题>
output: decision, confidence, evidence, risk, next_action
```

### Stuck-point packet

用于说明为什么需要升级：

```yaml
stuck_id: <stable-id>
phase: static-triage | context | focused-trace | value-flow | verification
blocked_by: <具体阻塞>
tried:
  - <已尝试的轻量动作>
need: request-main | request-tooling | heavy-trace | human-decision
budget:
  runtime_s: <估计>
  disk_mb: <估计>
```

## 3. 状态流

```text
draft
  -> candidate
  -> review: accepted | rejected | needs_more_evidence
  -> user/main confirmation when required
  -> confirmed | rejected | superseded
```

规则：

- worker / feature agent 最高只产出 `candidate`。
- reviewer 只给意见，不直接改 confirmed。
- 主线可以在 gate 通过后写 confirmed。
- 覆盖、删除、冲突、schema change、external side effect、destructive action 必须停下来问用户。
- rejected / superseded 不应消失；保留原因，避免后续 agent 重走旧路。

## 4. 推荐会话流程

1. `/rekit overview` 看项目总览。
2. `/rekit continue main` 或 `/rekit continue <feature>` 明确接手工作线。
3. 主 agent 生成 task packet，必要时分派只读 reviewer 或功能支线。
4. 功能支线写 observations / requests / candidates。
5. 主线消费 candidate，执行 review-first 合并。
6. confirmed 变更后运行验证并更新 handoff。
7. `/rekit handoff` 或 `/rekit handoff <name>` 生成接手文档。

## 5. 反漂移规则

- 每次只围绕一个明确 subject 判断，不让 agent 在大 trace 里自由漂移。
- 大函数、大反汇编、大工具输出先摘要和索引化，再窄范围读取。
- 发现多个互斥假设时，写多个 candidate，不在脑内混合。
- 被证伪的假设必须记录 rejected/superseded 原因。
- 重型步骤必须说明为什么轻路径不能闭合。