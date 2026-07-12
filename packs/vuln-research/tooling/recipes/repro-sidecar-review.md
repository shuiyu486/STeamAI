# Repro sidecar review recipe

## 目标

只读复核已存在的 local repro summary sidecar，用于确认 vulnerability candidate 或消除误报；本 recipe 不主动扫描、不 fuzz、不 replay exploit、不访问真实目标。

## Gate 前置

如果需要生成新的 repro、执行 fuzz、replay exploit、访问真实目标、调试、dump、patch 或导出数据，必须先登记 pending-gate request，至少包含：

```yaml
action: fuzz | exploit-replay | live-target-validation | debug | dump | patch | data-export
scope: <authorized target aliases and environment summary>
budget:
  requests: <max requests>
  runtime_s: <max seconds>
  inputs: <max corpus or payload count>
  rate_limit: <requests per second>
tried_light_steps:
  - crash triage
  - existing sidecar review
stop_conditions:
  - unexpected state change
  - out-of-scope host or redirect
  - crash/output size above budget
  - auth/session error
```

## 输出

- case-local target alias / repro id。
- 复现摘要、precondition、trigger shape、impact hypothesis、diff 摘要。
- sidecar path。
- verifier verdict 或 open questions。

## 禁止

- 不主动扫描、fuzz、bruteforce、credential stuffing、mass targeting、DoS 或 exploit replay。
- 不把完整请求/响应、PoC payload、凭据、token、cookie、core/minidump、pcap、trace 或 exploit chain 写入 pack。
- 不绕过用户确认执行 destructive write、真实目标验证或高流量动作。
