# Function behavior review recipe

## 目标

只读复核已存在的 disassembly summary、decompiler summary、API behavior summary、format finding 或 saved debug/trace summary sidecar，用于确认 binary behavior candidate 或消除误报；本 recipe 不执行样本、不调试、不 trace、不 dump、不 patch、不写 rename/comment、不写分析数据库。

## Gate 前置

如果需要动态执行、调试、trace、dump、patch、批量反编译、自动重命名/注释、外部联网或写回分析数据库，必须先经 `/rekit gate` preflight；`gate -Apply` 只记录 request decision，不执行动作。只有本次显式用户确认，或 strict durable autonomy profile + 覆盖本次边界的 `authorized-gate`，才允许 executor 执行。request 至少包含：

```yaml
gate_action: full-trace | debug | patch | dump | network | symex
domain_action: execute-sample | trace | rename-comment | decompile-bulk | database-writeback
target_ref: <exact targetScope value>
isolation: <vm/sandbox/offline/network policy>
requested_budget:
  runtime_seconds: <positive integer>
  disk_mb: <positive integer>
  requests: <positive integer>
output_paths:
  - <case-relative sidecar path>
tried_light_steps:
  - static-binary-triage
  - function-behavior-sidecar-review
stop_conditions:
  - <manifest/profile-covered lowercase token>
```

## 输出

- case-local binary/function/API alias / behavior candidate id。
- 行为摘要、precondition、input/output shape、side effects、diff 摘要。
- sidecar path。
- verifier verdict 或 open questions。

## 禁止

- 不主动执行样本、联网、debug、trace、dump、patch、bulk decompile、rename/comment writeback 或 database writeback。
- 不把样本、hash、IOC、dump、trace、memory snapshot、patch bytes、完整函数体、符号表、客户上下文、token 或绝对路径写入 pack。
- 既无本次显式用户确认、也无 strict durable autonomy profile + 对应 `authorized-gate` 时，不执行动态动作、写入分析数据库/二进制状态或破坏性动作；超出 grant 边界必须升级。
