# Template toolchain router

> 这是新 pack 的工具路由模板。复制后按领域填写工具、状态和止损条件。

## 工具状态

| 状态 | 含义 |
|---|---|
| `mainline-template` | 多个 case 验证有效，可作为推荐主线模板。 |
| `auxiliary` | 可用但只适合特定阶段或辅助查询。 |
| `candidate` | 值得短测，尚未充分验证。 |
| `cautious` | 有明显风险，需要确认、预算和止损。 |
| `short-test-stoploss` | 只能短测，失败即止损。 |
| `deprecated` | 不再推荐，保留历史说明。 |

## 按任务路由

| 任务 | 首选工具 | 备用/辅助 | 注意事项 |
|---|---|---|---|
| <task> | <tool> | <fallback> | <stop condition> |

## 重型工具门禁

```yaml
gate_action: full-trace | debug | inject | patch | dump | network | symex
domain_action: <pack-specific operation mapped to gate_action>
target_ref: <exact targetScope value>
risk: medium | high | critical
decision_reason: <why light path failed>
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

- 新工具先进入 `candidate`。
- 短测必须有 timeout、输出上限和止损条件。
- 不把完整 README、日志、trace、反编译粘入模板。
- 工具成为主线前至少经过多个 case 或稳定复现验证。
