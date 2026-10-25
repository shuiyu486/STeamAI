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

输出写入 case-local sidecar 或成员 workspace；聊天和 Markdown 只引用摘要与相对 evidence ref。

## 止损

- 发现样本名、hash、IOC、客户上下文、绝对路径、dump、trace、patch、完整函数体或符号表时停止提升到 pack，保留在 case-local 并标记 redaction needed。
- 样本、函数体、trace、dump 或工具 raw output 过大时先按 binary / function / API / evidence row 分片，不把完整内容粘入聊天或 Markdown。
- 证据不足以判断函数行为或 RE 路线时产出 request，不直接升级为 confirmed。

## 无符号与误导符号

- 无符号时用结构证据建立临时别名（`sub_XXXXXX`+一句结构描述）：调用约定与参数消费、栈帧大小、引用的字符串/常量/导入、控制流形状、与已知函数的结构相似。别名只在 case-local 使用，F 中写明"结构命名，非语义命名"。
- 符号存在但与结构证据矛盾时以结构证据为准：在 F 中并列符号声明与结构反证，记录矛盾而不静默采用任一侧；符号可能是复用、误导或与当前版本错位。
- 无符号下的行为归因止步于"结构等价"层级（相同调用约定、相同引用集、相同控制流形状），不能升级为"语义相同"；两函数语义是否相同需要独立证据，keep unknown 直到有区分观察。
- 版本比较时符号漂移（同函数异名/异函数同名）不当语义证据，按 function-version-compare 的三类判定处理。
