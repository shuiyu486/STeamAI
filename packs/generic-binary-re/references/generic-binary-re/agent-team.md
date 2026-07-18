# Generic binary RE Agent Team routes

## 1. 角色边界

- `main`：确认授权 binary / function / artifact 范围、静态/动态边界、隔离要求、case-local sidecar 位置和 gate 状态；合并 reviewer verdict，写 ledger / handoff，并在确认后更新 authority 文档。
- `binary-analysis`：在自己的 workspace 中分析单个 binary alias、function alias、API behavior hypothesis、string cluster、format note 或 behavior candidate，产出 observation / request / candidate。
- `reviewer`：只读复核 bounded function hypotheses、API behavior notes、format findings、behavior candidates 或 tooling notes，输出 verdict，不执行样本、不 trace、不 patch、不写文件。
- `tooling`：描述工具能力、输入输出、sidecar、预算、隔离要求和止损；默认不执行动态动作或自动写回 IDB/二进制。

## 2. 默认 routes

| route | 适用任务 | 分片 | 权限 | 输出 |
|---|---|---|---|---|
| `generic-binary-re:bounded-review` | finding / evidence / function / API / tooling review | function-or-finding | read-only | reviewer verdict |
| `generic-binary-re:binary-analysis` | static triage / function / API behavior / string / format 分析 | binary-function-or-behavior | read-only-or-workspace-only | observation / request / candidate |

`plan-subagents` 只生成 review packet 与 observability，不自动 spawn agent。主会话负责启动 Agent 工具、收集输出，并用 `/rekit note` 写回 verification / decision。

## 3. Packet 输出契约

所有子 agent 输出都必须包含：

```text
item, decision, confidence, evidence, risk, next_action, tier_used, tool_scope, defer_reason
```

Generic binary RE route 可追加：

```text
binary_ref, function_ref, artifact_ref, behavior_hint, api_ref, candidate_path
```

`binary_ref`、`function_ref`、`artifact_ref` 与 `api_ref` 应是 case-local 脱敏引用或 sidecar id，不是样本路径、hash、完整函数体、dump 路径、trace 路径、patch bytes、IDB 路径或绝对路径。`decision` 是 reviewer output decision，不等同于 ledger canonical decision；main 合并后再写 `/rekit note -Kind verification` 与 `/rekit note -Kind decision`。

## 4. Review-first 门禁

- accepted function hypothesis / API behavior candidate / format finding 只能进入 main 合并队列，不能直接写 confirmed / authority / report。
- 证据不足时使用 `defer` 或 `needs-more-evidence`，并给出下一步轻量验证。
- 需要动态执行、调试、trace、dump、patch、批量反编译、自动重命名/注释、外部联网或写回分析数据库时，先经 `/rekit gate` preflight；只有本次显式用户确认，或 strict validated durable autonomy profile + 覆盖本次边界的 `authorized-gate`，才允许 executor 执行。`gate -Apply` 本身只记录 request decision，不执行 heavy action。
- 每个 shard 的失败只影响本 shard；不要阻塞无关 binary、function、artifact、API 或 behavior candidate。

## 5. 证据与 sidecar

- evidence 应引用 case-local sidecar 路径、binary alias、function alias、artifact alias、API alias、tool summary、时间窗口和脱敏 row id。
- 不在 pack reference 中保存样本、hash、完整二进制、dump、trace、memory snapshot、patch、完整函数体、符号表、IOC、客户上下文或绝对路径。
- 任何可复用经验进入 pack 前必须清理样本特征、hash、IOC、路径、dump/trace/patch 细节和 case-specific behavior result。
