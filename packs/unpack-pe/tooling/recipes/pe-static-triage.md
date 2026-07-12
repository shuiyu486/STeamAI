# PE static triage recipe

## 目标

在不执行样本、不 dump、不 patch、不联网的前提下，建立 PE 样本别名、header / section / import / resource / string 摘要、packer hint、loader hypothesis 和 open questions 的最小索引。

## 输入

- 授权范围摘要。
- case-local sample alias 或 artifact id。
- 已脱敏的 file metadata、PE header summary、section table summary、import/resource summary、strings cluster、entropy notes 或 static tool sidecar。

## 输出

```text
sample_ref, packer_hint, section_ref, import_state, loader_hypothesis, evidence_ref, open_questions
```

输出写入 case-local sidecar 或 lane workspace；聊天和 Markdown 只引用摘要与路径。

## 止损

- 发现样本名、hash、IOC、客户上下文、绝对路径、dump、trace、patch、完整 import table 或 section bytes 时停止提升到 pack，保留在 case-local 并标记 redaction needed。
- 样本、dump、trace 或工具 raw output 过大时先按 section / loader stage / evidence row 分片，不把完整内容粘入聊天或 Markdown。
- 证据不足以判断 unpack 路线时产出 request，不直接升级为 confirmed。
