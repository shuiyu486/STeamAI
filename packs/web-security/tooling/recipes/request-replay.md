# Bounded request replay recipe

## 目标

对少量授权请求做可回放验证，用于确认 candidate finding 或消除误报。

## Gate 前置

执行前必须先经 `/rekit gate` preflight；`gate -Apply` 只记录 `pending-gate` 或 `authorized-gate` request decision，不执行动作。只有本次显式用户确认，或 strict durable autonomy profile + 覆盖本次边界的 `authorized-gate`，才允许 executor 执行。request 至少包含：

```yaml
gate_action: network
domain_action: authenticated-replay | exploit-replay
target_ref: <exact targetScope value>
requested_budget:
  runtime_seconds: <positive integer>
  disk_mb: <positive integer>
  requests: <positive integer>
output_paths:
  - <case-relative sidecar path>
tried_light_steps:
  - passive-triage
  - sidecar-review
stop_conditions:
  - live-target-ambiguity
  - unexpected-outbound-request
  - scope-drift
```

## 输出

- case-local request id。
- response摘要、status、headers allowlist、timing、diff 摘要。
- sidecar path。
- verifier verdict 或 open questions。

## 禁止

- 不做 DoS、bruteforce、credential stuffing、mass scan。
- 不把完整请求/响应、凭据、token、cookie、JWT、API key 或 payload 写入 pack。
- 既无本次显式用户确认、也无 strict durable autonomy profile + 对应 `authorized-gate` 时，不执行 destructive write 或高流量动作；超出 grant 边界必须升级。
