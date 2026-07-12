# Control-flow triage recipe

## 目标

在不执行样本、不 trace、不 dump、不 patch、不写 IDB/二进制的前提下，建立 binary/function 别名、CFG region 摘要、flattening / dispatcher / opaque predicate / MBA hint 和 open questions 的最小索引。

## 输入

- 授权范围摘要。
- case-local binary alias、function alias 或 CFG region id。
- 已脱敏的 function metadata、basic block summary、edge summary、dispatcher hint、decompiler note、string/import summary 或 static tool sidecar。

## 输出

```text
binary_ref, function_ref, cfg_region_ref, obfuscation_hint, transform_type, evidence_ref, open_questions
```

输出写入 case-local sidecar 或 lane workspace；聊天和 Markdown 只引用摘要与路径。

## 止损

- 发现样本名、hash、IOC、客户上下文、绝对路径、dump、trace、patch、完整函数体、full CFG 或符号表时停止提升到 pack，保留在 case-local 并标记 redaction needed。
- 函数体、CFG、trace 或工具 raw output 过大时先按 function / region / evidence row 分片，不把完整内容粘入聊天或 Markdown。
- 证据不足以判断混淆变换或简化路线时产出 request，不直接升级为 confirmed。
