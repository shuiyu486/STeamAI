# Generic binary RE toolchain router

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
| scope / binary inventory | case-local aliases、授权摘要 | metadata sidecar | 不把样本名、hash、客户上下文或绝对路径写入 pack。 |
| static triage | header / section / import / export / string summary sidecar | signature / symbol summary | 原始二进制、完整函数体和工具 raw output 留 case-local。 |
| function hypothesis | disassembly summary、decompiler note、API summary | callgraph / xref summary | 不执行样本；只引用脱敏 function 和 sidecar id。 |
| behavior review | behavior candidate summary、precondition note | saved trace/debugger summary | 不写完整 trace、dump、patch bytes 或 case-specific result。 |
| dynamic/gated action | pending-gate request | static sidecar first | debug、trace、dump、patch、rename/comment、database writeback 必须 gate。 |
| tooling adapter | capability card + dry-run | candidate recipe | 工具先 recipe 化，不做硬依赖。 |

## 重型工具门禁

```yaml
heavy_action: debug | execute-sample | trace | dump | patch | rename-comment | decompile-bulk | database-writeback | network
scope: <binary/function/artifact aliases and authorization summary>
isolation: <vm/sandbox/offline/network policy>
decision_reason: <why static/passive path failed>
tried_light_steps:
  - <step>
budget:
  runtime_s: <estimate>
  functions: <max function aliases>
  output_mb: <max output size>
  network: <disabled | sinkholed | explicit allowlist>
outputs:
  - <case-local sidecar path>
stop_conditions:
  - <stop condition>
requires_user_confirmation: true
```

## 维护规则

- 新工具先进入 `candidate` 或 `cautious`。
- 短测必须有 timeout、函数数量上限、输出大小上限、网络策略和止损条件。
- 不保存样本、hash、完整二进制、dump、trace、memory snapshot、patch、完整函数体、符号表、IOC、客户上下文或绝对路径到 pack。
- 工具成为 mainline-template 前至少经过多个授权 case 或稳定 lab 复现验证。
