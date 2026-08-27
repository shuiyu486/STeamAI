# Binary RE 通用分析基线

通用 static triage、function behavior analysis、API behavior review 与 tooling sidecar review 是 `binary-re` 的基线能力。VMProtect/IDA 不是另一套 pack；当前 enabled specialty 只覆盖已有 IDA TSV export 的 bounded inspection，trace/devirtualization 仍由顶层 router 进入 review-first workflow/recipe，不是已启用 producer。

## 何时读取

进入普通 binary inventory、format/section/import/string review、function/API hypothesis 或 saved sidecar review 时读取本文件。若目标已经明确是 VMProtect x64 trace/devirtualization，改读 `workflow-template.md`，不要同时加载两条完整路线。

## 轻到重路线

```text
authorized scope + case-local aliases
  → metadata/import/export/string/section/symbol sidecar
  → function/API/format/behavior hypothesis
  → saved disassembly or decompiler summary review
  → independent reviewer verdict
  → main decision + ledger/handoff
  → dynamic or writeback action only after exact gate
```

优先用 passive、read-only、bounded 的证据回答问题。只有静态路线明确卡住，才申请 debug、trace、dump、patch、rename/comment、bulk decompile、database writeback 或 network action。

## Case-local 数据边界

- pack-promotable 内容只引用 `binary_ref`、`function_ref`、`artifact_ref`、`api_ref`、sidecar row id 和摘要。
- 样本、完整二进制、hash、IOC、完整函数体、符号表、dump、trace、patch bytes、IDB 路径、客户信息和绝对路径留在 case-local evidence。
- raw tool output 必须落 sidecar；不要整段粘入聊天、handoff 或 reference。

## Review 与 authority

- `binary-analysis` lane 只提交 observation、request、candidate 或 summary，不直接写 confirmed/authority。
- `binary-re:bounded-review` 只读复核 bounded finding、function/API hypothesis、tooling note 或证据 packet。
- main agent 合并 verdict，记录 decision，并在 scope、evidence、review 与写入门禁均满足后更新 handoff/authority。
- rejected、deferred、superseded 保留原因，避免后续重复误报。
