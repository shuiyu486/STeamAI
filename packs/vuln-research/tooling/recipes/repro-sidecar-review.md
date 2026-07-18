# Repro sidecar review recipe

## 目标

只读复核已存在的 local repro summary sidecar，用于确认 vulnerability candidate 或消除误报；本 recipe 不主动扫描、不 fuzz、不 replay exploit、不访问真实目标。

## Gate 前置

如果需要生成新的 repro、执行 fuzz、replay exploit、访问真实目标、调试、dump、patch 或导出数据，必须先经 `/rekit gate` preflight；`gate -Apply` 只记录 request decision，不执行动作。只有本次显式用户确认，或 strict durable autonomy profile + 覆盖本次边界的 `authorized-gate`，才允许 executor 执行。request 至少包含：

```yaml
gate_action: debug | patch | dump | network | symex
domain_action: fuzz | exploit-replay | live-target-validation | data-export
target_ref: <exact targetScope value>
requested_budget:
  runtime_seconds: <positive integer>
  disk_mb: <positive integer>
  requests: <positive integer>
output_paths:
  - <case-relative sidecar path>
tried_light_steps:
  - crash-triage
  - existing-sidecar-review
stop_conditions:
  - <manifest/profile-covered lowercase token>
```

## 输出

- case-local target alias / repro id。
- 复现摘要、precondition、trigger shape、impact hypothesis、diff 摘要。
- sidecar path。
- verifier verdict 或 open questions。

## 禁止

- 不主动扫描、fuzz、bruteforce、credential stuffing、mass targeting、DoS 或 exploit replay。
- 不把完整请求/响应、PoC payload、凭据、token、cookie、core/minidump、pcap、trace 或 exploit chain 写入 pack。
- 既无本次显式用户确认、也无 strict durable autonomy profile + 对应 `authorized-gate` 时，不执行 destructive write、真实目标验证或高流量动作；超出 grant 边界必须升级。
