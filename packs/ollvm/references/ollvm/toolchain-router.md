# OLLVM toolchain router

## 工具状态

| 状态 | 含义 |
|---|---|
| `mainline-template` | 多个授权 case 验证有效，可作为推荐主线模板。 |
| `auxiliary` | 可用但只适合特定阶段或辅助查询。 |
| `candidate` | 值得短测，尚未充分验证。 |
| `cautious` | 有明显执行、写入、外联或数据风险，需要确认、预算、隔离和止损。 |
| `short-test-stoploss` | 只能短测，失败即止损。 |
| `deprecated` | 不再推荐，保留历史说明。 |

## 按任务路由

| 任务 | 首选工具/方式 | 备用/辅助 | 注意事项 |
|---|---|---|---|
| scope / function inventory | case-local aliases、授权摘要 | binary metadata sidecar | 不把样本名、hash、客户上下文或绝对路径写入 pack。 |
| CFG triage | basic block / edge / dispatcher summary sidecar | decompiler summary | 原始函数体、完整 CFG 和工具 raw output 留 case-local。 |
| transform hypothesis | flattening、opaque predicate、MBA、string decode notes | saved disassembly summary | 不执行样本；只引用脱敏 region 和 sidecar id。 |
| simplification review | expression summary、candidate rewrite note | diff summary sidecar | 不写完整表达式语料、patch bytes 或 deobfuscated binary。 |
| dynamic/gated action | pending-gate request | static sidecar first | debug、trace、dump、patch、rename/comment、export 必须 gate。 |
| tooling adapter | capability card + dry-run | candidate recipe | 工具先 recipe 化，不做硬依赖。 |

## 重型工具门禁

```yaml
gate_action: full-trace | debug | inject | patch | dump | network | symex
domain_action: execute-sample | trace | rename-comment | decompile-bulk | export-deobfuscated
target_ref: <exact targetScope value>
isolation: <vm/sandbox/offline/network policy>
decision_reason: <why static/passive path failed>
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
- 短测必须有 timeout、函数数量上限、输出大小上限、网络策略和止损条件。
- 不保存样本、hash、反混淆后二进制、dump、trace、patch、完整函数体、full CFG、符号表、IOC、客户上下文或绝对路径到 pack。
- 工具成为 mainline-template 前至少经过多个授权 case 或稳定 lab 复现验证。
