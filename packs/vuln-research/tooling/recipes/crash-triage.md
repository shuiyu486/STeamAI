# Crash triage recipe

## 目标

在不执行 exploit replay 或 fuzzing 的前提下，建立目标别名、crash id、版本信息、栈摘要、可疑 bug class 和 open questions 的最小索引。

## 输入

- 授权范围摘要。
- case-local 目标别名或 crash id。
- 已脱敏的 crash / stack / build / patch summary sidecar。

## 输出

```text
target_ref, crash_id, build_ref, stack_theme, suspected_bug_class, repro_status, evidence_ref, open_questions
```

输出写入 case-local sidecar 或 lane workspace；聊天和 Markdown 只引用摘要与路径。

## 止损

- 发现真实目标、凭据、token、PoC payload、内部路径、客户敏感字段或未公开漏洞细节时停止提升到 pack，保留在 case-local 并标记 redaction needed。
- stack、trace 或 crash corpus 过大时先按 bug class / component 分片，不把完整内容粘入聊天或 Markdown。
- crash 证据不足以判断根因时产出 request，不直接升级为 confirmed。
