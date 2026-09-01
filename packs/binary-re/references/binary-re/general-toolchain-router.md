# Binary RE 通用工具路由

## 状态含义

| 状态 | 含义 |
|---|---|
| `mainline-template` | 多个授权 case 中可重复使用的 passive/read-only 模板。 |
| `auxiliary` | 只适合特定阶段或辅助查询。 |
| `candidate` | 值得有界短测，尚未充分验证。 |
| `cautious` | 有执行、写入或外联风险，必须具体确认。 |
| `short-test-stoploss` | 只能按预算短测，失败即止损。 |

## 按任务路由

| 任务 | 首选 | 备用 | 边界 |
|---|---|---|---|
| binary inventory | case-local alias + metadata sidecar | format/signature summary | 不把样本名、hash 或绝对路径写进 pack。 |
| static triage | header/section/import/export/string/symbol summary | bounded index query | 原始二进制与 raw output 留 case-local。 |
| function/API hypothesis | saved disassembly/decompiler/API summary | callgraph/xref summary | 不执行样本，只引用脱敏 ref。 |
| behavior review | behavior candidate + precondition note | saved trace/debug summary | Reviewer 默认只读。 |
| VMP/IDA 专项 | `toolchain-router.md` | `../../tooling/catalog.yml` | 按专项止损和确认边界执行。 |
| dynamic/writeback | exact user-confirmed action | 先补 passive sidecar | 必须同时满足 Claude Code 工具权限。 |

## Heavy action request

向用户展示：action、exact target、为何轻路径不足、隔离与网络策略、运行/输出/request 预算、case-relative output、rollback 和 stop conditions。确认只授权展示的具体动作；scope 或条件变化时必须停止并重新确认。

## 维护规则

- 新工具先写 capability/recipe，再进入相应状态；catalog 永远不作为可解释执行命令。
- 短测必须有 timeout、目标数量、输出上限、网络策略和 stop conditions。
- 样本、hash、二进制、dump、trace、patch、完整函数体、IOC、客户信息和绝对路径不得进入 pack。
