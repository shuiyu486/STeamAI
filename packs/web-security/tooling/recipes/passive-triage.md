# Passive Web/API triage recipe

## 目标

在不产生外部请求或只使用用户已提供 sidecar 的前提下，建立 endpoint / route / auth boundary 的最小索引。

## 输入

- 授权范围摘要。
- OpenAPI / Swagger / route map / proxy export summary sidecar。
- 已脱敏的 endpoint 列表或 case-local path。

## 输出

```text
endpoint_id, method, route_pattern, auth_required, roles_seen, input_surfaces, evidence_ref, open_questions
```

输出写入 case-local sidecar 或 lane workspace；聊天和 Markdown 只引用摘要与路径。

## 止损

- 发现真实凭据、token、cookie、个人信息或客户敏感字段时停止提升到 pack，保留在 case-local 并标记 redaction needed。
- endpoint 数量过大时先抽样或按 feature 分片，不把完整列表粘入聊天。
