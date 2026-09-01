# Binary RE 通用分析基线

通用 static triage、function behavior analysis、API behavior review 与 tooling sidecar review 是 `binary-re` 的基线。VMProtect/IDA 不是另一套 pack；trace/devirtualization 只在轻路径不足时按专项 route 升级。

## 轻到重路线

```text
authorized scope + case-local aliases
  → metadata/import/export/string/section/symbol sidecar
  → function/API/format/behavior hypothesis
  → evidence + finding
  → independent Reviewer decision
  → accepted finding
  → dynamic or writeback action only after exact user confirmation
```

优先用 passive、read-only、bounded 证据回答问题。只有静态路线明确卡住，才申请 debug、trace、dump、patch、rename/comment、bulk decompile、database writeback 或 network action。

## Case-local 数据边界

- 可回流内容只引用 `binary_ref`、`function_ref`、`artifact_ref`、`api_ref`、sidecar row id 和摘要。
- 样本、完整二进制、hash、IOC、完整函数体、符号表、dump、trace、patch bytes、IDB 路径、客户信息和绝对路径留在 case-local artifact/evidence。
- raw tool output 必须落 sidecar；不要整段粘入聊天、finding、review 或 reference。

## Review 边界

- 分析成员只写允许范围内的 artifact/evidence/finding，不直接改共享 analysis database 或已确认表。
- Reviewer 只读 bounded finding/evidence，只写 review；`needs-evidence` 返回原 owner。
- Commander 只有在 scope、evidence、review 与写入边界都满足后才集成交付。
- rejected、disputed、superseded 保留原因，避免后续重复误报。
