# Binary RE 通用工具路由

## 状态含义

| 状态 | 含义 |
|---|---|
| `mainline-template` | 可复用的 passive/read-only 主线模板。 |
| `auxiliary` | 只适合特定阶段或辅助查询。 |
| `candidate` | 值得有界短测，尚未充分验证。 |
| `cautious` | 有执行、写入、外联或数据风险，必须 exact gate。 |
| `short-test-stoploss` | 只能按预算短测，失败即止损。 |

## 按任务路由

| 任务 | 首选 | 备用 | 边界 |
|---|---|---|---|
| binary inventory | case-local alias + metadata sidecar | format/signature summary | 不把样本名、hash 或绝对路径写进 pack。 |
| static triage | header/section/import/export/string/symbol summary | bounded index query | 原始二进制与 raw output 留 case-local。 |
| function/API hypothesis | saved disassembly/decompiler/API summary | callgraph/xref summary | 不执行样本，只引用脱敏 ref。 |
| behavior review | behavior candidate + precondition note | saved trace/debug summary | reviewer 默认只读。 |
| VMP/IDA 专项 | `toolchain-router.md` | `tooling/catalog.yml` 对应 capability | 按专项止损和 gate 执行。 |
| dynamic/writeback | pending gate request | 先补 passive sidecar | debug、trace、dump、patch、rename/comment、database writeback 必须 gate。 |

## Gate request 最小字段

```yaml
gate_action: inspect | full-trace | debug | inject | patch | dump | network | symex
target_ref: <exact targetScope value>
isolation: <vm/sandbox/offline/network policy>
decision_reason: <why passive path failed>
requested_budget: <runtime/output/request limits>
output_paths: <case-relative sidecars>
stop_conditions: <manifest/profile-covered tokens>
status: pending-gate | authorized-gate
```

manifest 的 `requiresConfirmation` 表示 action 必须经过 gate；`gate -Apply` 只记录 decision，不执行 heavy action。actual adapter 必须重新验证 target、profile、fresh gate、预算和输出边界，并写 execution observation/evidence。

## 维护规则

- 新工具先写 capability/recipe，再进入 `candidate`、`auxiliary` 或 `cautious`；catalog 的 `entry` 永远不作为可解释执行命令。
- 短测必须有 timeout、目标数量上限、输出上限、网络策略和 stop conditions。
- 样本、hash、二进制、dump、trace、patch、完整函数体、IOC、客户信息和绝对路径不得进入 pack。
- 多个授权 case 或稳定 lab 重复验证后，才提升为 `mainline-template`。
