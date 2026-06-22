# Evidence ledger and intervention model

## 目的

定义后续 `.rekit/facts/*.jsonl`、lane outbox/inbox 和 review packet 的统一事件模型草案。当前文件是设计契约，不表示 runtime 已全部实现。

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
  "nextAction": "review|focused-trace|confirm|reject"
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
