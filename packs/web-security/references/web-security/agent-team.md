# Web/API security Agent Team routes

## 1. 角色边界

- `main`：确认授权范围、拆分 endpoint / feature / finding，选择 route，合并 reviewer verdict，写 ledger / handoff，并在确认后更新 authority 文档。
- `feature`：在自己的 workspace 中分析单个 endpoint、API flow、authn/authz 边界或输入验证点，产出 observation / request / candidate。
- `reviewer`：只读复核 bounded findings、endpoint evidence 或 tooling notes，输出 verdict，不发起外部请求，不写文件。
- `tooling`：描述工具能力、输入输出、sidecar、预算、速率限制和止损；默认不执行主动扫描。

## 2. 默认 routes

| route | 适用任务 | 分片 | 权限 | 输出 |
|---|---|---|---|---|
| `web-security:bounded-review` | finding / evidence / endpoint / tooling review | finding-or-endpoint | read-only | reviewer verdict |
| `web-security:feature-analysis` | endpoint / feature / API flow / authz / input validation 分析 | endpoint-or-flow | read-only-or-workspace-only | observation / request / candidate |

`plan-subagents` 只生成 review packet 与 observability，不自动 spawn agent。主会话负责启动 Agent 工具、收集输出，并用 `/rekit note` 写回 verification / decision。

## 3. Packet 输出契约

所有子 agent 输出都必须包含：

```text
item, decision, confidence, evidence, risk, next_action, tier_used, tool_scope, defer_reason
```

Web/API route 可追加：

```text
endpoint, method, impact, feature, request_id, candidate_path
```

`decision` 是 reviewer output decision，不等同于 ledger canonical decision；main 合并后再写 `/rekit note -Kind verification` 与 `/rekit note -Kind decision`。

## 4. Review-first 门禁

- accepted finding 只能进入 main 合并队列，不能直接写 confirmed / report / authority。
- 证据不足时使用 `defer` 或 `needs-more-evidence`，并给出下一步轻量验证。
- 需要主动请求、扫描、fuzz、登录尝试、exploit replay、数据导出或高流量动作时，先经 `/rekit gate` preflight；只有本次显式用户确认，或 strict validated durable autonomy profile + 覆盖本次边界的 `authorized-gate`，才允许 executor 执行。`gate -Apply` 本身只记录 request decision，不执行 heavy action。
- 每个 shard 的失败只影响本 shard；不要阻塞无关 endpoint / finding。

## 5. 证据与 sidecar

- evidence 应引用 case-local sidecar 路径、请求 ID、响应摘要、时间窗口和工具名称。
- 不在 pack reference 中保存真实 request / response body、token、cookie、JWT、API key 或漏洞利用 payload。
- 任何可复用经验进入 pack 前必须清理目标名、凭据、路径和漏洞细节。
