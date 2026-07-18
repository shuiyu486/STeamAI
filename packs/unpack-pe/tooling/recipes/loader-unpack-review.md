# Loader unpack review recipe

## 目标

只读复核已存在的 loader trace summary、unpack candidate summary、import recovery summary 或 saved sandbox/debugger sidecar，用于确认 unpack candidate 或消除误报；本 recipe 不执行样本、不 dump、不 patch、不重建 import、不联网。

## Gate 前置

如果需要动态调试、样本执行、dump、patch、解密/解压 payload、外部联网、自动修复 import 或写 unpacked 文件，必须先经 `/rekit gate` preflight；`gate -Apply` 只记录 request decision，不执行动作。只有本次显式用户确认，或 strict durable autonomy profile + 覆盖本次边界的 `authorized-gate`，才允许 executor 执行。request 至少包含：

```yaml
gate_action: full-trace | debug | patch | dump | network | symex
domain_action: execute-sample | import-rebuild | decrypt-payload | sandbox-run
target_ref: <exact targetScope value>
isolation: <vm/sandbox/offline/network policy>
requested_budget:
  runtime_seconds: <positive integer>
  disk_mb: <positive integer>
  requests: <positive integer>
output_paths:
  - <case-relative sidecar path>
tried_light_steps:
  - pe-static-triage
  - loader-unpack-sidecar-review
stop_conditions:
  - <manifest/profile-covered lowercase token>
```

## 输出

- case-local sample alias / unpack candidate id。
- loader stage 摘要、unpack trigger shape、import state、diff 摘要。
- sidecar path。
- verifier verdict 或 open questions。

## 禁止

- 不主动执行样本、联网、debug、dump、patch、import rebuild、mass process 或 payload extraction。
- 不把样本、hash、IOC、dump、trace、memory snapshot、unpacked binary、patch bytes、完整 import table、section bytes、客户上下文、token 或绝对路径写入 pack。
- 既无本次显式用户确认、也无 strict durable autonomy profile + 对应 `authorized-gate` 时，不执行动态动作、写入 unpacked 文件或破坏性动作；超出 grant 边界必须升级。
