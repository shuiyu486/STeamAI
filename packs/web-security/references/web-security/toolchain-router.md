# Web/API security toolchain router

## 工具状态

| 状态 | 含义 |
|---|---|
| `mainline-template` | 多个 case 验证有效，可作为推荐主线模板。 |
| `auxiliary` | 可用但只适合特定阶段或辅助查询。 |
| `candidate` | 值得短测，尚未充分验证。 |
| `cautious` | 有明显外部副作用或数据风险，需要确认、预算和止损。 |
| `short-test-stoploss` | 只能短测，失败即止损。 |
| `deprecated` | 不再推荐，保留历史说明。 |

## 按任务路由

| 任务 | 首选工具/方式 | 备用/辅助 | 注意事项 |
|---|---|---|---|
| scope / route map | case-local notes、OpenAPI / Swagger、静态文档 | passive crawler sidecar | 不主动爆破路径；未知目标先确认范围。 |
| request/response review | saved sidecar、proxy export summary | small replay harness | 不把完整请求/响应粘入聊天或 pack。 |
| authn/authz hypothesis | manual flow map、role matrix sidecar | focused replay | 不尝试越权写操作；需要账号/角色时确认授权。 |
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

manifest 的静态 `requiresConfirmation: true` 只表示 action 必须经过 gate；request decision 由 autonomy preflight 动态产生：`pending-gate` 对应 `true`，`authorized-gate` 对应 `false`。`gate -Apply` 只记录 decision，不执行 heavy action。

## 维护规则

- 新工具先进入 `candidate` 或 `cautious`。
- 短测必须有 timeout、请求量上限、速率限制和止损条件。
- 不保存真实凭据、token、cookie、HAR、pcap、scan output 或漏洞 payload 到 pack。
- 工具成为 mainline-template 前至少经过多个授权 case 或稳定靶场复现验证。
