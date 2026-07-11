# Bounded request replay recipe

## 目标

对少量授权请求做可回放验证，用于确认 candidate finding 或消除误报。

## Gate 前置

执行前必须登记 pending-gate request，至少包含：

```yaml
action: authenticated-replay | exploit-replay | network
scope: <authorized target and account/role summary>
budget:
  requests: <max requests>
  runtime_s: <max seconds>
  rate_limit: <requests per second>
tried_light_steps:
  - passive triage
  - sidecar review
stop_conditions:
  - unexpected state change
  - auth/session error
  - response size above budget
  - out-of-scope redirect or host
```

## 输出

- case-local request id。
- response摘要、status、headers allowlist、timing、diff 摘要。
- sidecar path。
- verifier verdict 或 open questions。

## 禁止

- 不做 DoS、bruteforce、credential stuffing、mass scan。
- 不把完整请求/响应、凭据、token、cookie、JWT、API key 或 payload 写入 pack。
- 不绕过用户确认执行 destructive write 或高流量动作。
