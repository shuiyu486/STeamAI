# Handoff and session continuity

目标：让下一次会话从短入口恢复，而不是重新读取长历史。

## 接手文档应包含

- 当前目标和边界。
- 已完成事项。
- 当前状态指标。
- 待处理队列。
- 下一步命令或检查清单。
- 权威数据文件路径。

## 不应包含

- round-by-round 长历史。
- 完整工具输出。
- 大段归档内容。
- 已可从代码、git 或机器数据重建的信息。

## 子 agent 结果沉淀

主 agent 只把合并后的结论、deferred 列表和下一步写入 handoff。子 agent 原始长输出不进入 handoff。
