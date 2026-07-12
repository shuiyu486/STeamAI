# Function behavior review recipe

## 目标

只读复核已存在的 disassembly summary、decompiler summary、API behavior summary、format finding 或 saved debug/trace summary sidecar，用于确认 binary behavior candidate 或消除误报；本 recipe 不执行样本、不调试、不 trace、不 dump、不 patch、不写 rename/comment、不写分析数据库。

## Gate 前置

如果需要动态执行、调试、trace、dump、patch、批量反编译、自动重命名/注释、外部联网或写回分析数据库，必须先登记 pending-gate request，至少包含：

```yaml
action: debug | execute-sample | trace | dump | patch | rename-comment | decompile-bulk | database-writeback | network
scope: <authorized binary/function/artifact aliases and authorization summary>
isolation: <vm/sandbox/offline/network policy>
budget:
  runtime_s: <max seconds>
  functions: <max function aliases>
  output_mb: <max output size>
  network: <disabled | sinkholed | explicit allowlist>
tried_light_steps:
  - static binary triage
  - function behavior sidecar review
stop_conditions:
  - out-of-scope binary or function
  - unexpected network egress
  - output size above budget
  - destructive patch or irreversible database/binary state change
  - credential, token, IOC, or customer data leakage
```

## 输出

- case-local binary/function/API alias / behavior candidate id。
- 行为摘要、precondition、input/output shape、side effects、diff 摘要。
- sidecar path。
- verifier verdict 或 open questions。

## 禁止

- 不主动执行样本、联网、debug、trace、dump、patch、bulk decompile、rename/comment writeback 或 database writeback。
- 不把样本、hash、IOC、dump、trace、memory snapshot、patch bytes、完整函数体、符号表、客户上下文、token 或绝对路径写入 pack。
- 不绕过用户确认执行动态动作、写入分析数据库/二进制状态或破坏性动作。
