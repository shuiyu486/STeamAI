# Local repro review recipe

## 目标

只读复核已存在的 local solver / repro summary sidecar，用于确认 solution candidate 或消除误报；本 recipe 不连接远程服务、不 fuzz、不 bruteforce、不 replay exploit。

## Gate 前置

如果需要远程连接、bruteforce、fuzz、exploit replay、高流量请求、debug、dump 或 patch，必须先登记 pending-gate request，至少包含：

```yaml
action: remote-connect | bruteforce | fuzz | exploit-replay | high-rate-requests | debug | dump | patch
scope: <authorized challenge aliases and competition/lab summary>
budget:
  requests: <max requests>
  runtime_s: <max seconds>
  inputs: <max corpus or payload count>
  rate_limit: <requests per second>
tried_light_steps:
  - challenge triage
  - local sidecar review
stop_conditions:
  - service instability or rate-limit warning
  - out-of-scope host or redirect
  - output size above budget
  - unexpected credential or flag leakage
```

## 输出

- case-local challenge alias / solver id。
- 解题路线摘要、precondition、trigger shape、flag 状态摘要、diff 摘要。
- sidecar path。
- verifier verdict 或 open questions。

## 禁止

- 不主动连接远程、bruteforce、mass scan、DoS、credential stuffing 或 exploit replay。
- 不把 flag、完整 payload、solver 私有脚本、账号凭据、token、raw response、pcap、dump、trace 或 challenge 原始文件写入 pack。
- 不绕过用户确认执行高流量动作、真实远程验证或破坏性动作。
