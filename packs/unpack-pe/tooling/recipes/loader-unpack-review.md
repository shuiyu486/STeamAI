# Loader unpack review recipe

## 目标

只读复核已存在的 loader trace summary、unpack candidate summary、import recovery summary 或 saved sandbox/debugger sidecar，用于确认 unpack candidate 或消除误报；本 recipe 不执行样本、不 dump、不 patch、不重建 import、不联网。

## Gate 前置

如果需要动态调试、样本执行、dump、patch、解密/解压 payload、外部联网、自动修复 import 或写 unpacked 文件，必须先登记 pending-gate request，至少包含：

```yaml
action: debug | execute-sample | dump | patch | import-rebuild | decrypt-payload | network | sandbox-run
scope: <authorized sample aliases and authorization summary>
isolation: <vm/sandbox/offline/network policy>
budget:
  runtime_s: <max seconds>
  samples: <max sample aliases>
  output_mb: <max output size>
  network: <disabled | sinkholed | explicit allowlist>
tried_light_steps:
  - PE static triage
  - loader/unpack sidecar review
stop_conditions:
  - out-of-scope sample or host
  - unexpected network egress
  - output size above budget
  - destructive patch or irreversible state change
  - credential, token, IOC, or customer data leakage
```

## 输出

- case-local sample alias / unpack candidate id。
- loader stage 摘要、unpack trigger shape、import state、diff 摘要。
- sidecar path。
- verifier verdict 或 open questions。

## 禁止

- 不主动执行样本、联网、debug、dump、patch、import rebuild、mass process 或 payload extraction。
- 不把样本、hash、IOC、dump、trace、memory snapshot、unpacked binary、patch bytes、完整 import table、section bytes、客户上下文、token 或绝对路径写入 pack。
- 不绕过用户确认执行动态动作、写入 unpacked 文件或破坏性动作。
