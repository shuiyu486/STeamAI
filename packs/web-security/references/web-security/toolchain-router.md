# Web/API security toolchain router

## 工具状态

| 状态 | 含义 |
|---|---|
| `supported` | fixed compiled-in production adapter；只执行 catalog 明确的有界能力，`entry` 仅作 provenance。 |
| `mainline-template` | 多个 case 验证有效，可作为推荐主线模板。 |
| `auxiliary` | 可用但只适合特定阶段或辅助查询。 |
| `candidate` | 值得短测，尚未充分验证。 |
| `cautious` | 有明显外部副作用或数据风险，需要确认、预算和止损。 |
| `short-test-stoploss` | 只能短测，失败即止损。 |
| `deprecated` | 不再推荐，保留历史说明。 |

## 按任务路由

| 任务 | 首选工具/方式 | 备用/辅助 | 注意事项 |
|---|---|---|---|
| OpenAPI route/auth inventory | supported `openapi-v3-json-inventory` | case-local notes、静态文档 | 只读一个 case-local OpenAPI 3 JSON；不联网、不抓 remote ref、不保留 secret/value/description/example。 |
| request/response digest review | saved sidecar、proxy export summary | supported `bounded-http-replay` | replay 仅允许 inventory-bound GET/HEAD/OPTIONS、exact loopback/injected transport、one request、zero redirect/retry、no proxy；不保存完整请求/响应。 |
| authn/authz hypothesis | manual flow map、role matrix sidecar | bounded replay with `authRef` | secret 只经 `STEAMAI_AUTH_*` 环境引用；不尝试越权写操作。 |
| input validation review | schema / validator / error摘要 | small payload checklist | 不 fuzz 高流量；payload 只写 case-local。 |
| tooling adapter | capability card + dry-run | candidate recipe | 工具先 recipe 化，不做硬依赖。 |

## 重型工具门禁

```yaml
gate_action: full-trace | debug | inject | patch | dump | network | symex
domain_action: active-scan | fuzz | exploit-replay | authenticated-replay | data-export
target_ref: <exact targetScope value>
decision_reason: <why passive/light path failed>
tried_light_steps:
  - <step>
requested_budget:
  runtime_seconds: <positive integer>
  disk_mb: <positive integer>
  requests: <positive integer>
output_paths:
  - <case-relative sidecar path>
stop_conditions:
  - <manifest/profile-covered lowercase token>
status: pending-gate | authorized-gate
requires_user_confirmation: true | false
```

manifest 的静态 `requiresConfirmation: true` 只表示 action 必须经过 gate；request decision 由 autonomy preflight 动态产生：`pending-gate` 对应 `true`，`authorized-gate` 对应 `false`。Project-local Gate Apply 只记录 decision，不执行 heavy action。Fixed replay 还要求 content-addressed request path、strict durable profile 与 exact authorized gate；delivery uncertain 为 terminal，禁止重试或 same-job replacement。

## 维护规则

- Production adapter 仅为 catalog 中两个 `supported` fixed compiled-in ID；新工具先进入 `candidate` 或 `cautious`，不得复用其成熟度。
- 短测必须有 timeout、请求量上限、速率限制和止损条件。
- 不保存真实凭据、token、cookie、HAR、pcap、scan output 或漏洞 payload 到 pack。
- 工具成为 mainline-template 前至少经过多个授权 case 或稳定靶场复现验证。
