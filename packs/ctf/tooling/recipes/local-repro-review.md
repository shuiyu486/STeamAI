# Local repro review recipe

## 目标

只读复核已存在的 local solver / repro summary sidecar，用于确认 solution candidate 或消除误报；本 recipe 不连接远程服务、不 fuzz、不 bruteforce、不 replay exploit。

## Gate 前置

如果需要远程连接、bruteforce、fuzz、exploit replay、高流量请求、debug、dump 或 patch，必须先经 `/rekit gate` preflight；`gate -Apply` 只记录 request decision，不执行动作。只有本次显式用户确认，或 strict durable autonomy profile + 覆盖本次边界的 `authorized-gate`，才允许 executor 执行。request 至少包含：

```yaml
gate_action: debug | patch | dump | network | symex
domain_action: remote-connect | bruteforce | fuzz | exploit-replay | high-rate-requests
target_ref: <exact targetScope value>
requested_budget:
  runtime_seconds: <positive integer>
  disk_mb: <positive integer>
  requests: <positive integer>
output_paths:
  - <case-relative sidecar path>
tried_light_steps:
  - challenge-triage
  - local-sidecar-review
stop_conditions:
  - <manifest/profile-covered lowercase token>
```

## 输出

- case-local challenge alias / solver id。
- 解题路线摘要、precondition、trigger shape、flag 状态摘要、diff 摘要。
- sidecar path。
- verifier verdict 或 open questions。

## 禁止

- 不主动连接远程、bruteforce、mass scan、DoS、credential stuffing 或 exploit replay。
- 不把 flag、完整 payload、solver 私有脚本、账号凭据、token、raw response、pcap、dump、trace 或 challenge 原始文件写入 pack。
- 既无本次显式用户确认、也无 strict durable autonomy profile + 对应 `authorized-gate` 时，不执行高流量动作、真实远程验证或破坏性动作；超出 grant 边界必须升级。
