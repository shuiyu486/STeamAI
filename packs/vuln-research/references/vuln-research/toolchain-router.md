# Vulnerability research toolchain router

## 工具状态

| 状态 | 含义 |
|---|---|
| `mainline-template` | 多个 case 验证有效，可作为推荐主线模板。 |
| `auxiliary` | 可用但只适合特定阶段或辅助查询。 |
| `candidate` | 值得短测，尚未充分验证。 |
| `cautious` | 有明显外部副作用或数据风险，需要确认、预算、隔离和止损。 |
| `short-test-stoploss` | 只能短测，失败即止损。 |
| `deprecated` | 不再推荐，保留历史说明。 |

## 按任务路由

| 任务 | 首选工具/方式 | 备用/辅助 | 注意事项 |
|---|---|---|---|
| scope / target inventory | case-local aliases、授权说明、资产清单摘要 | docs / schema sidecar | 不把真实目标、凭据或内部路径写入 pack。 |
| crash triage | crash summary sidecar、stack摘要、版本信息 | minidump/core summary | 原始 crash/core/minidump 留 case-local。 |
| root cause hypothesis | source/patch diff summary、control/data-flow notes | local repro sidecar | 不把 PoC payload 或完整请求粘入模板。 |
| repro review | saved local repro summary | small replay harness | 真实目标复现、exploit replay、fuzz 需 gate。 |
| patch/remediation review | patch diff sidecar、test plan | regression checklist | 不写未公开补丁细节或客户专有代码。 |
| tooling adapter | capability card + dry-run | candidate recipe | 工具先 recipe 化，不做硬依赖。 |

## 重型工具门禁

```yaml
gate_action: full-trace | debug | inject | patch | dump | network | symex
domain_action: active-scan | fuzz | exploit-replay | live-target-validation | data-export
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
- 短测必须有 timeout、请求量/输入量上限、速率限制、输出大小上限和止损条件。
- 不保存真实目标、凭据、token、request/response、PoC payload、crash/core/minidump、pcap、trace、scan output 或漏洞利用细节到 pack。
- 工具成为 mainline-template 前至少经过多个授权 case 或稳定靶场复现验证。
