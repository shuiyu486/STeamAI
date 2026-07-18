# MBA simplification review recipe

## 目标

只读复核已存在的 MBA expression summary、opaque predicate summary、string decode summary、deobfuscation candidate 或 saved disassembly/decompiler sidecar，用于确认 simplification candidate 或消除误报；本 recipe 不执行样本、不 trace、不 dump、不 patch、不写 rename/comment、不导出反混淆二进制。

## Gate 前置

如果需要动态执行、调试、trace、dump、patch、批量反编译、自动重命名、自动写注释、导出反混淆二进制或外部联网，必须先经 `/rekit gate` preflight；`gate -Apply` 只记录 request decision，不执行动作。只有本次显式用户确认，或 strict durable autonomy profile + 覆盖本次边界的 `authorized-gate`，才允许 executor 执行。request 至少包含：

```yaml
gate_action: full-trace | debug | patch | dump | network | symex
domain_action: execute-sample | trace | rename-comment | decompile-bulk | export-deobfuscated
target_ref: <exact targetScope value>
isolation: <vm/sandbox/offline/network policy>
requested_budget:
  runtime_seconds: <positive integer>
  disk_mb: <positive integer>
  requests: <positive integer>
output_paths:
  - <case-relative sidecar path>
tried_light_steps:
  - control-flow-triage
  - mba-simplification-sidecar-review
stop_conditions:
  - <manifest/profile-covered lowercase token>
```

## 输出

- case-local binary/function alias / simplification candidate id。
- transform type、precondition、equivalence summary、diff 摘要。
- sidecar path。
- verifier verdict 或 open questions。

## 禁止

- 不主动执行样本、联网、debug、trace、dump、patch、bulk decompile、rename/comment writeback 或 export deobfuscated artifact。
- 不把样本、hash、IOC、dump、trace、反混淆后二进制、patch bytes、完整函数体、full CFG、符号表、客户上下文、token 或绝对路径写入 pack。
- 既无本次显式用户确认、也无 strict durable autonomy profile + 对应 `authorized-gate` 时，不执行动态动作、写入 IDB/二进制状态或破坏性动作；超出 grant 边界必须升级。
