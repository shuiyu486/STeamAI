# Vulnerability research Agent Team routes

## 1. 角色边界

- `main`：确认授权范围、目标别名、复现边界、case-local sidecar 位置和 gate 状态；合并 reviewer verdict，写 ledger / handoff，并在确认后更新 authority 文档。
- `vuln-analysis`：在自己的 workspace 中分析单个目标组件、crash、补丁 diff、repro 假设、bug class 或影响候选，产出 observation / request / candidate。
- `reviewer`：只读复核 bounded findings、repro evidence、crash notes、patch diff 或 tooling notes，输出 verdict，不主动扫描、不 replay exploit、不写文件。
- `tooling`：描述工具能力、输入输出、sidecar、预算、隔离要求和止损；默认不执行主动测试。

## 2. 默认 routes

| route | 适用任务 | 分片 | 权限 | 输出 |
|---|---|---|---|---|
| `vuln-research:bounded-review` | finding / evidence / crash / repro / patch / tooling review | finding-or-repro | read-only | reviewer verdict |
| `vuln-research:vuln-analysis` | root cause / crash triage / patch diff / repro / exploitability 分析 | target-or-bug-class | read-only-or-workspace-only | observation / request / candidate |

`plan-subagents` 只生成 review packet 与 observability，不自动 spawn agent。主会话负责启动 Agent 工具、收集输出，并用 `/rekit note` 写回 verification / decision。

## 3. Packet 输出契约

所有子 agent 输出都必须包含：

```text
item, decision, confidence, evidence, risk, next_action, tier_used, tool_scope, defer_reason
```

Vulnerability route 可追加：

```text
target_ref, bug_class, repro_ref, crash_id, patch_ref, impact_hypothesis, candidate_path
```

`target_ref`、`repro_ref`、`crash_id` 应是 case-local 脱敏引用或 sidecar id，不是目标真实域名、内部主机名、完整请求、payload 或绝对路径。`decision` 是 reviewer output decision，不等同于 ledger canonical decision；main 合并后再写 `/rekit note -Kind verification` 与 `/rekit note -Kind decision`。

## 4. Review-first 门禁

- accepted finding / root-cause / repro / patch candidate 只能进入 main 合并队列，不能直接写 confirmed / report / authority。
- 证据不足时使用 `defer` 或 `needs-more-evidence`，并给出下一步轻量验证。
- 需要主动扫描、fuzz、exploit replay、真实目标复现、debug、dump、patch、数据导出或外部副作用时，先登记 pending-gate request。
- 每个 shard 的失败只影响本 shard；不要阻塞无关目标、bug class 或 repro。

## 5. 证据与 sidecar

- evidence 应引用 case-local sidecar 路径、目标别名、crash/repro id、patch ref、工具名称、时间窗口和脱敏 row id。
- 不在 pack reference 中保存真实目标、凭据、PoC payload、request/response body、core/minidump、pcap、trace、客户路径或环境标识。
- 任何可复用经验进入 pack 前必须清理目标名、路径、payload、漏洞细节、利用链上下文和客户环境信息。
