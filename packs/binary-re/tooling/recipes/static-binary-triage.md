# Static binary triage recipe

## 目标

在不执行样本、不调试、不 trace、不 dump、不 patch、不写分析数据库的前提下，建立 binary/function 别名、header / section / import / export / symbol / string 摘要、format hint、behavior hypothesis 和 open questions 的最小索引。

## 输入

- 授权范围摘要。
- case-local binary alias、function alias 或 artifact id。
- 已脱敏的 file metadata、header summary、section table summary、import/export summary、symbol summary、strings cluster、format notes 或 static tool sidecar。

## 输出

```text
binary_ref, function_ref, artifact_ref, api_ref, behavior_hint, evidence_ref, open_questions
```

输出写入 case-local sidecar 或 lane workspace；聊天和 Markdown 只引用摘要与路径。

## 止损

- 发现样本名、hash、IOC、客户上下文、绝对路径、dump、trace、patch、完整函数体或符号表时停止提升到 pack，保留在 case-local 并标记 redaction needed。
- 样本、函数体、trace、dump 或工具 raw output 过大时先按 binary / function / API / evidence row 分片，不把完整内容粘入聊天或 Markdown。
- 证据不足以判断函数行为或 RE 路线时产出 request，不直接升级为 confirmed。
