# MBA simplification review recipe

## 目标

只读复核已存在的 MBA expression summary、opaque predicate summary、string decode summary、deobfuscation candidate 或 saved disassembly/decompiler sidecar，用于确认 simplification candidate 或消除误报；本 recipe 不执行样本、不 trace、不 dump、不 patch、不写 rename/comment、不导出反混淆二进制。

## Gate 前置

如果需要动态执行、调试、trace、dump、patch、批量反编译、自动重命名、自动写注释、导出反混淆二进制或外部联网，必须先登记 pending-gate request，至少包含：

```yaml
action: debug | execute-sample | trace | dump | patch | rename-comment | decompile-bulk | export-deobfuscated | network
scope: <authorized binary/function aliases and authorization summary>
isolation: <vm/sandbox/offline/network policy>
budget:
  runtime_s: <max seconds>
  functions: <max function aliases>
  output_mb: <max output size>
  network: <disabled | sinkholed | explicit allowlist>
tried_light_steps:
  - control-flow triage
  - MBA/simplification sidecar review
stop_conditions:
  - out-of-scope binary or function
  - unexpected network egress
  - output size above budget
  - destructive patch or irreversible IDB/binary state change
  - credential, token, IOC, or customer data leakage
```

## 输出

- case-local binary/function alias / simplification candidate id。
- transform type、precondition、equivalence summary、diff 摘要。
- sidecar path。
- verifier verdict 或 open questions。

## 禁止

- 不主动执行样本、联网、debug、trace、dump、patch、bulk decompile、rename/comment writeback 或 export deobfuscated artifact。
- 不把样本、hash、IOC、dump、trace、反混淆后二进制、patch bytes、完整函数体、full CFG、符号表、客户上下文、token 或绝对路径写入 pack。
- 不绕过用户确认执行动态动作、写入 IDB/二进制状态或破坏性动作。
