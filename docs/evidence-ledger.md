# Evidence ledger and intervention model

## 目的

定义 `.rekit/facts/*.jsonl`、lane outbox/inbox 和 review packet 的统一事件模型。当前 runtime 已支持 9 种 kind 的 append/read 基础能力；本文件仍作为字段语义的 canonical contract，后续优化应保持 append-only 和历史兼容。

目标是降低长程逆向中的漂移、重复返工和不可追溯 confirmed 写入。

## 原则

- Append-only 优先：状态变化通过新事件表达，不静默改写历史。
- 人可读优先：JSONL 应可 grep、可 diff、可手工恢复。
- Evidence refs 指向 sidecar 或文件定位，不复制大输出。
- confirmed / authority 必须能追溯到 candidate、evidence、verifier 和 decision。
- rejected / superseded 必须保留原因，避免后续 agent 重走旧路。

## 事件类型

| 类型 | 用途 |
|---|---|
| `observation` | 记录事实观察，未必形成结论。 |
| `hypothesis` | 可探索假设，低于 candidate。 |
| `candidate` | 可复核候选结论。 |
| `verification` | 复核、测试、trace、parity、schema check 的结果。 |
| `decision` | 主线或用户对 candidate 的接受、拒绝、延期。 |
| `intervention` | 人工 override、回滚、风险确认、重型工具授权。 |
| `rollback` | 批次或结论撤销，保留原事件。 |
| `publication` | 从支线向共享事实或主线发布。 |
| `request` | 向主线、工具线或用户请求处理。 |

## 基础字段

```json
{
  "schemaVersion": 1,
  "eventId": "evt-...",
  "kind": "candidate",
  "time": "<ISO8601>",
  "actor": "main|feature|reviewer|tooling|user|runtime",
  "lane": "<lane-id>",
  "subject": "<stable subject>",
  "summary": "<short summary>",
  "evidenceRefs": [],
  "related": [],
  "status": "open|accepted|rejected|superseded|resolved|deferred",
  "risk": "low|medium|high",
  "confidence": "low|medium|high"
}
```

`accepted` / `resolved` / `rejected` / `superseded` 是读层应视为终态的状态；`confirmed`、`pending-gate`、`needs_more_evidence` 是已落地 runtime 的兼容值，分别用于历史确认事件、gate request 与 packet 侧候选状态。新 decision event 应优先使用 `accepted|rejected|deferred|superseded`。

## Candidate 字段

```json
{
  "kind": "candidate",
  "candidateId": "cand-...",
  "claim": "<候选结论>",
  "evidenceRefs": ["file:line", "sidecar#filter"],
  "limitations": ["low-occurrence", "alias-heavy"],
  "proposedAuthorityWrite": {
    "file": "captures/...csv",
    "rowPreview": "..."
  },
  "nextAction": "review|focused-trace|accept|reject"
}
```

## Verification 字段

```json
{
  "kind": "verification",
  "target": "cand-...",
  "verifier": "manual-review|schema-check|focused-trace|parity|cross-run|tool-review",
  "verdict": "accepted|rejected|inconclusive|needs-more-evidence",
  "evidenceRefs": [],
  "notes": "<short>"
}
```

packet / reviewer output 中可使用 `needs_more_evidence` 表达候选仍缺证据；ledger `verification.verdict` 使用 kebab-case `needs-more-evidence`。主 agent 写回时负责这个轻量归一化，不要求历史 packet 迁移。

## Decision 字段

```json
{
  "kind": "decision",
  "target": "cand-...",
  "decision": "accept|reject|defer|supersede",
  "reason": "<why>",
  "confirmedBy": "main|user",
  "writes": [
    {"file": "<path>", "backup": "<path>", "diff": "<path>"}
  ]
}
```

`decision=accept` 是 canonical 写法；旧事件可能含 `decision=confirm` 或 `action=<auto-*|pending-user>`，读层可兼容展示，但新写入不应继续产生旧字段。`decision=accept` 只表示 candidate 被 main 接受；confirmed/authority 写入仍必须满足 evidence、verification、backup/diff、schema 与人工确认 gate。

## Request 字段

`request` 用于请求 main/tooling/user 处理，也承载 heavy-action gate 的 pending 或 durable autonomy authorized decision：

```json
{
  "kind": "request",
  "target": "<event|lane|batch|object>",
  "status": "open|pending-gate|authorized-gate|resolved|deferred|rejected",
  "risk": "medium|high|critical",
  "gate": {
    "action": "full-trace|debug|inject|patch|dump|network|symex|other",
    "scope": "<narrow authorized scope>",
    "budget": "<runtime/disk/token budget summary>",
    "triedLightSteps": ["<lighter-step>"],
    "stopConditions": ["<lowercase-slug-or_snake-token>"]
  }
}
```

`status=pending-gate` 表示需要用户确认；它不是执行授权本身。`status=authorized-gate` 表示 Go gate preflight 已找到当前 lane 的 durable autonomy profile，且 action、target、typed budget、output paths 与 stop conditions 被 profile 完全覆盖；它仍只是 ledger authorization decision，不代表 `/rekit` 已执行 heavy-tool，也不放宽 confirmed/authority/sync/promote 边界。确认或 profile 授权只覆盖 event 中列明的 action/scope/budget/risk/outputPaths/stopConditions，不得扩大到其它 heavy-tool 动作。Go `gate -WhatIf/-Apply` 使用 manifest `defaultRisk`，用户覆盖 `-Risk` 时必须是小写 `medium|high|critical`；`stopConditions` 在 preview/request ledger 中使用小写 slug/snake token 列表，人类说明应放在 summary 或 decision_reason。

## Intervention 字段

```json
{
  "kind": "intervention",
  "action": "override|rollback|heavy-tool-approval|schema-migration|external-side-effect",
  "target": "<event or batch id>",
  "reason": "<why>",
  "approvedBy": "user|main",
  "scope": "<authorized scope>",
  "expires": "<optional>"
}
```

resolution 不修改历史 intervention；runtime 应追加新的 `kind=intervention`、`status=resolved`、`resolvesEventId=<source eventId>` 事件表达关闭关系。Mission brief、overview、handoff 与 `continue` blocker 判断使用 effective open projection：只有没有被 resolution event 关闭的 open/deferred intervention 才继续阻塞 lane；单纯 `target` 字段不代表 lifecycle resolution。

`/rekit reconcile <lane> -InterventionId <eventId> -Apply` 是 Go-native resolution 入口：只写 case-local interventions ledger、lane events、lane.json、RESUME/checkpoint 和 board，刷新 current executor / executor generation，不执行 heavy-tool，也不写 authority/confirmed。

## Batch 模型

一轮自动整理、review 或工具运行应生成 `batchId`：

```json
{
  "batchId": "batch-...",
  "inputs": [],
  "outputs": [],
  "summary": "<short>",
  "rollbackPlan": "append rollback event, do not delete history"
}
```

批次可以整体接受、部分接受、拒绝或 rollback。rollback 不删除历史，只追加 `rollback` / `intervention` 事件。

## Runtime 落地顺序

1. 文档契约稳定后，先让 `/rekit continue` digest 引用这些字段。
2. 再让 `.rekit/facts/*.jsonl` 写入统一 `kind` 和 `eventId`。
3. 再在 `overview` 中展示 stuck statistics、冲突 candidate、需要确认事项。
4. 最后才考虑索引优化；默认继续 JSONL，不提前引入 SQLite。
