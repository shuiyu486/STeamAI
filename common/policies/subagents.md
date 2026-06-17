# Subagent delegation and bounded parallelism

目标：用子 agent 分担只读分析、复核和摘要，减少主 agent 上下文压力，并在可拆分任务中提高效率。

## 何时使用

- 任务能拆成互不依赖的小块。
- 每个子任务有明确输入范围和输出格式。
- 子任务主要是只读分析、搜索、复核、反驳或摘要。
- 主会话只需要结论、定位和少量证据，不需要大段原始输出。

## 何时不要使用

- 子任务需要同时修改同一文件、同一表或同一外部状态。
- 任务严格顺序依赖，后一步必须等待前一步写入结果。
- 子任务边界不清，需要 agent 自行无界探索。
- 输出无法结构化，主会话难以合并或验证。

## 并行度

默认使用 bounded parallelism：

| 场景 | 建议 |
|---|---|
| 普通只读分片 | 2-4 个子 agent，总量受主会话合并能力限制。 |
| 高风险判断 | 总并行度不超过 3；使用 1-2 个分片 agent + 1 个反驳 agent。 |
| 大范围统计 | 优先脚本化，子 agent 只审查摘要和异常。 |

不要为了“看起来更并行”增加 agent。过多 agent 会重复读取、增加 token 成本，并放大合并负担。

## 分层授权与升级

不要用同一种子 agent 承担所有深度。默认先窄后深，只有证据不足且收益足够高时才升级：

| 层级 | 用途 | 工具边界 | 失败策略 |
|---|---|---|---|
| L1 packet review | 基于主 agent 或脚本抽取的短 evidence packet 做初筛 | 不主动查大文件，不调用重型工具 | 证据不足返回 `defer` / `needs_l2` |
| L2 bounded evidence review | 读取明确指定的小范围文件、行范围或 sidecar | 只读；不重跑长任务，不打开重型 GUI / 二进制分析工具 | 证据不足返回 `defer` / `needs_l3` |
| L3 deep tool review | 对少数高价值 blocker 做深挖 | 允许重型工具，但必须 narrow target、明确预算，优先后台运行 | 超时/中断不阻塞主线，标记 pending/deferred |

升级规则：

- L1/L2 prompt 必须声明允许读取的输入范围和禁止的重型动作。
- L3 必须说明目标、范围、预算、可接受的等待方式和失败后的回退状态。
- 可能长时间运行的 L3 应后台化；主会话继续处理其它分片。
- 子 agent 卡住时，不扩大等待；下一轮改成更窄输入或显式 L3。

## 主 agent 职责

1. 明确分片：给每个子 agent 固定输入范围。
2. 控制上下文：要求短输出，不接受长日志或大段复制。
3. 合并台账：维护 accepted、rejected、deferred、待验证列表。
4. 写入与验证：默认只有主 agent 修改文件、执行外部动作和最终验证。

子 agent 不应直接写项目文件，除非任务明确要求，并使用隔离工作区或无冲突边界。

## 输出契约

通用结构：

```text
item: <id>
decision: accept | reject | defer | needs_l2 | needs_l3
confidence: high | medium | low
evidence: 最多 3 条关键证据或定位
risk: 主要风险或空
next_action: 主 agent 下一步
tier_used: L1 | L2 | L3
tool_scope: packet_only | sidecar_only | bounded_read | deep_tool
```

pack overlay 可以追加领域字段，但仍应保持短输出。

## Planner contract

pack 可以在 manifest 中声明 `subagentRoutes`，让 `/rekit plan-subagents` 生成分片计划。每条 route 至少应说明：

- `id`：稳定 route 名称。
- `taskTypes`：适用任务类型，用逗号分隔。
- `shardBasis`、`targetItemsPerAgent`、`maxParallel`：分片维度与并行上限。
- `reference` / `policyOverlay`：人类可读规则入口。
- `subagentPermissions`：默认 `read-only`，除非明确隔离写入边界。
- `mainAgentOwns`：主 agent 独占的写入、验证、发布或 handoff 动作。
- `outputContract`：子 agent 必须返回的短字段列表。

使用 route 前，主 agent 仍需确认任务边界清楚；若无法给定固定分片，则不要启动无界子 agent。

## 失败与中断

- 子 agent 卡住或中断时，只丢弃该分片结果，不影响其它分片。
- 不等待无界 agent；下一轮改成更小分片或更窄输入。
- 有争议的结论默认 deferred。
- 高置信写入前，应由主 agent 或反驳 agent 独立验证。
