# Agent Team policy

## 目的

定义跨 pack 复用的 Agent Team 协作边界。具体领域流程放在 pack reference 或 overlay 中；本文件只规定通用角色、packet、状态流和人工确认边界。

## 角色边界

| 角色 | 职责 | 默认权限 |
|---|---|---|
| Main agent | 拆解任务、分派 packet、收敛结论、维护 handoff、执行确认后的 authority 写入 | 可写当前授权范围内的 canonical / case state |
| Feature agent | 围绕一个窄功能或问题收集 evidence、request、candidate | 只写自己的 workspace |
| Tooling agent | 评估工具能力、适配成本、止损条件和 tooling candidate | 只写 tooling workspace 或候选 |
| Reviewer agent | 只读复核 candidate、evidence、tooling diff 或 packet | 不写文件 |
| Verifier / gate | 检查 schema、evidence、冲突、备份、diff、预算和确认条件 | 输出 verdict，不创造业务结论 |
| Human confirmer | 对高风险动作、confirmed/authority、外部副作用和架构取舍做最终授权 | 通过对话授权 |

Agent 可以短命，工作线和账本必须持久。任何 agent 的输出都应能被下一会话通过 packet、facts、handoff 继续消费。

## Packet 类型

### Task packet

用于分派任务：

```yaml
task_id: <stable-id>
lane: <workstream-id>
goal: <要回答的问题>
inputs:
  - <文件、sidecar 或事实引用>
allowed_reads:
  - <范围>
allowed_writes:
  - <workspace 或 none>
stop_conditions:
  - <停止、升级或询问条件>
output_contract: evidence | candidate | review | request | handoff
```

### Evidence packet

用于短证据摘要：

```yaml
evidence_id: <stable-id>
subject: <对象>
evidence:
  - kind: source | trace | disasm | decompile | xref | tool-output | test
    ref: <文件:行 或 sidecar + filter>
    summary: <短摘要>
confidence: low | medium | high
limitations:
  - <缺失、低样本、alias、未 cross-run 等>
```

`evidence_id` 必填，供 candidate 的 `evidence_refs` 引用。

### Candidate packet

用于候选结论：

```yaml
candidate_id: <stable-id>
subject: <对象>
claim: <候选结论>
evidence_refs:
  - <evidence id 或文件定位>
verifier: pending | accepted | rejected | needs_more_evidence
risk: low | medium | high
next_action: accept | review | request-authority | reject | defer
```

### Review packet

用于只读复核：

```yaml
review_id: <stable-id>
candidate: <candidate-id 或摘要>
lens: correctness | evidence | simplicity | tooling-risk | schema | security
scope: <必须保持窄范围>
question: <需要判断的问题>
output_contract: decision,confidence,evidence,risk,next_action
```

### Stuck-point packet

用于升级或请求帮助：

```yaml
stuck_id: <stable-id>
phase: <当前阶段>
blocked_by: <具体阻塞>
tried:
  - <已尝试的轻量动作>
need: request-main | request-tooling | heavy-tool | human-decision
budget:
  runtime_s: <估计>
  disk_mb: <估计>
```

## 状态流

```text
draft -> candidate -> review -> accepted | rejected | superseded | needs_more_evidence
```

规则：

- Worker / feature agent 最高只产出 candidate。
- Reviewer 只出 verdict 和 evidence，不直接写 accepted/confirmed 结论。
- accepted decision 只表示 main agent 接受该 candidate；confirmed / authority 写入仍必须由 main agent 在 gate 通过后执行。
- rejected / superseded 必须保留原因，避免后续重复走旧路。

### Reviewer verdict 与 ledger decision

Reviewer output contract 中的 `decision` 是分片复核结论，不是最终 ledger decision。主 agent 合并时按下表转换：

| reviewer output | main agent ledger 写入 | 说明 |
|---|---|---|
| `accept` | 先写 `verification.verdict=accepted`；满足 gate 后才写 `decision=accept` | `decision=accept` 不等于自动写 confirmed/authority。 |
| `reject` | 写 `verification.verdict=rejected`，通常再写 `decision=reject` | 保留 evidence/reason，避免重复复核。 |
| `defer` | 写 `verification.verdict=inconclusive` 或 `decision=defer` | 表示当前证据不足或暂不合并。 |
| `needs_l2` / `needs_l3` | 写 `verification.verdict=needs-more-evidence` 或 `request` | 升级前仍需遵守 tool_scope 与 heavy-tool gate。 |

### Decision event

每次 review 后，若 main agent 对 candidate 做出合并判断，应产出 decision event 持久化到 `.rekit/facts/decisions.jsonl`（即使手动写入也要对齐此格式）：

```yaml
event_id: <stable-id>
kind: decision
lane: <runtime 规范化后的 lane id>
subject: <candidate-id 或对象>
decision: accept | reject | defer | supersede
reason: <短理由>
superseded_by: <新 candidate-id 或 null>
status: accepted | rejected | deferred | superseded
evidence_refs:
  - <evidence-id>
created_at: <ISO 时间>
```

`decision=accept` / `status=accepted` 是当前 canonical 写法；历史事件中可能存在 `confirm`、`confirmed` 或自动流程旧字段 `action`，读层可兼容展示，但新写入不应继续使用旧值。

`status=deferred` 的 decision 仍要写入账本，让下一会话知道"为什么不再走旧路"。

## 必须询问用户的情况

- confirmed / authority 写入、覆盖或删除。
- 外部副作用：网络、发布、上传、安装、进程注入、patch、dump、动态调试。
- runtime schema 迁移或破坏兼容性的 manifest 变更。
- 需要扩大授权范围的架构取舍。
- 工具运行成本、输出规模或风险明显超出当前 packet。

## packet 文件与 facts event 的关系

- **packet 文件**是 agent 产出物，写在 lane workspace（路径由 manifest `workstreamDefaults` 驱动，以 `/rekit start` 输出为准，不写死 `.rekit/lanes/<id>/workspace`）。
- **facts event**是 runtime 视图，append 到 `.rekit/facts/*.jsonl`，供 `overview`、`handoff` 和 `note -List` 聚合。
- 当前有两条写入路径：`/rekit continue` 自动从 lane CSV/workspace 抽取低风险 event；`/rekit note` 手动 append 单条 observation/candidate/verification/decision/request/publication/intervention/rollback/hypothesis event。
- packet 仍是 agent 产出 source of truth；event 是可聚合索引。手动 packet 若需要进入 `overview` / `handoff`，主 agent 应通过 `/rekit note` 写入对应 facts event，而不是让 reviewer 直接写 authority。

## lane id 规范化

lane id 由 runtime 规范化（小写）。packet 字段中 `lane` 沿用 runtime 实际 id，不要用原始输入大小写。

## 维护要求

- Packet 必须小、结构化、可 diff。
- 长输出放 sidecar，Markdown 只放摘要和定位。
- 每批自动化都应生成 digest，说明输入、输出、决策和未决风险。
