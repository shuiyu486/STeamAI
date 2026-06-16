# Tool output and large artifact handling

目标：避免工具输出污染上下文，同时保留可复核路径。

## 规则

- 不把完整 build log、trace、dump、disassembly、decompile 或大 CSV 粘进 Markdown。
- 大输出保存到文件，主会话只记录路径、命令、摘要和关键错误。
- 能用脚本统计的，不手工复制长表。
- 需要 LLM 判断时，先抽取小样本、摘要或 bounded diff。

## 报告格式

```text
command:
output_path:
summary:
key_findings:
errors:
next_action:
```
