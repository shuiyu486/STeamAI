# Binary RE 通用 Agent Team 路由

## 角色边界

- `main`：确认授权范围、隔离和 gate 状态；合并 reviewer verdict，写 ledger/handoff，并控制 authority 写入。
- `binary-analysis`：在自己的 workspace 中分析 binary/function/API/format/behavior hypothesis，提交 observation、request、candidate 或 summary。
- `reviewer`：只读复核 bounded sidecar 与 hypothesis，输出 verdict；不执行样本、不 trace、不 patch、不写共享状态。
- `tooling`：维护 capability、输入输出、sidecar、预算与止损；默认不执行动态动作或写回分析数据库。

## Routes

| route | 适用任务 | 分片 | 权限 | 输出 |
|---|---|---|---|---|
| `binary-re:binary-analysis` | static triage、function/API behavior、string/format 分析 | binary-function-or-behavior | read-only-or-workspace-only | observation/request/candidate |
| `binary-re:bounded-review` | finding/evidence/function/API/tooling 及 VMP bounded review | function-or-finding | read-only | verifier verdict |
| `binary-re:lane-feature-analysis` | VMP feature workspace 与 lowering request | feature | read-only-or-workspace-only | observation/request/candidate |

`plan-subagents` 只生成 packet 和 observability，不自动 spawn agent。主会话启动短命 agent、收集结果，并通过 canonical typed action 写 verification/decision。

## Packet contract

所有输出必须包含：

```text
item, decision, confidence, evidence, risk, next_action, tier_used, tool_scope, defer_reason
```

通用分析可追加：

```text
binary_ref, function_ref, artifact_ref, behavior_hint, api_ref, candidate_path
```

这些 ref 必须是 case-local 脱敏 alias 或 sidecar id，不是样本路径、hash、完整函数体、dump/trace 路径、patch bytes、IDB 路径或绝对路径。子 agent 的 `decision` 不是 canonical ledger decision；main 合并后再写 verification 与 decision。

## Review-first 门禁

- accepted hypothesis 只能进入 main 合并队列，不能直接写 confirmed、authority 或发布报告。
- 证据不足使用 `defer`/`needs-more-evidence`，并给出最小下一步。
- heavy action 先走 `/steamai gate` preflight；只有 exact scope 的 fresh `authorized-gate` 加 strict durable profile 才允许 executor 执行。
- 每个 shard 的失败只影响本 shard，不阻塞无关任务。
