# Template Agent Team routes

> 这是新安全领域 pack 的 Agent Team 路由模板。复制到真实 pack 后，按领域替换 taskTypes、证据形态、review 标准和 heavy-tool 门禁。

## 1. 角色边界

- `main`：拆解任务、选择 route、启动只读/工作区限定子 agent、合并 verdict、写 ledger / handoff，并在确认后写 authority。
- `feature`：在自己的 workspace 中收集 observation、提出 candidate、登记 request；默认不写 authority。
- `reviewer`：只读复核 bounded shard，输出 verdict，不改文件。
- `tooling`：描述工具能力、输入输出、sidecar、预算和止损；不把工具私有状态写回 pack。

## 2. 默认 route

| route | 适用任务 | 分片 | 权限 | 输出 |
|---|---|---|---|---|
| `<pack>:bounded-review` | candidate / evidence / tooling review | `item` | read-only | reviewer verdict |
| `<pack>:lane-feature-analysis` | feature / workstream 分析 | `feature` | read-only-or-workspace-only | observation / request / candidate |

`plan-subagents` 只生成 review packet 和 observability，不自动 spawn subagent。主会话负责实际启动 agent、收集输出，并用 `/rekit note` 写回 verification / decision。

## 3. Packet 输出契约

子 agent 输出必须覆盖：

```text
item, decision, confidence, evidence, risk, next_action, tier_used, tool_scope, defer_reason
```

领域 pack 可以追加字段，但不要删除这些通用字段。`decision` 是 reviewer output decision，不等同于 ledger canonical decision；main 合并时再写 `/rekit note -Kind verification` 和 `/rekit note -Kind decision`。

## 4. Review-first 门禁

- accepted verdict 只能进入 main 合并队列，不能直接写 confirmed / authority。
- 证据不足时用 `defer` 或 `needs-more-evidence`，并给出下一步轻量验证。
- full-trace / debug / inject / patch / dump / network 等 heavy-tool 必须先经 `/rekit gate` preflight；`gate -Apply` 只记录 `pending-gate` 或 `authorized-gate` decision，不执行动作。只有本次显式用户确认，或 strict validated durable autonomy profile + 覆盖本次边界的 `authorized-gate`，才允许 lane executor / tool adapter 执行。
- 每个 shard 的失败只影响本 shard；不要阻塞无关 shard。

## 5. 新 pack 改写 checklist

- 替换 route id 前缀为真实 pack 名。
- 按领域替换 `taskTypes`、`trigger` 和 `shardBasis`。
- 明确哪些写入只能由 main 执行。
- 为领域证据补充必要的 `evidence_id`、sidecar 路径和 verifier 标准。
- 运行 `plan-subagents` smoke，确认默认 Go packet / summary 生成与 `REKIT_GO_DISABLE=1` no-fallback 边界。
