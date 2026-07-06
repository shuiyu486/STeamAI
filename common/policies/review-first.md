# Review-first writes and user confirmation

本文件偏**流程**（先 review 再 write、确认语义）；什么能写、子 agent 默认只读等**边界**规则见 `write-boundaries.md`。两者配合：边界决定是否需要审查，流程决定审查如何进行。

目标：所有可能覆盖、删除、回流模板、发布或影响外部状态的动作，都先生成可审查事实，再由 Claude 说明优劣和风险，最后等待用户明确确认。

## 基本规则

- 先 review，再 write。
- review 输出应包含：目标、方向、变更摘要、风险、冲突、推荐动作。
- 用户确认必须绑定具体动作、目标、文件范围或候选范围。
- “继续”“好”“confirm”不能扩大授权。
- `WhatIf` 只是 dry run，不等同于结构化 review。

## 适用动作

- sync / promote / template 回流。
- 覆盖、删除、移动、重命名。
- 外部发布、推送、提交 PR、调用远程服务。
- 修改权威数据表、策略文档或跨 pack common policy。

## 输出建议

```text
action:
target:
scope:
benefit:
risk:
conflict:
recommendation:
requires_confirmation: yes | no
```

## promote 分类

可复用经验应先分类：

- common policy：影响所有 pack，必须严格 review。
- pack overlay：领域化规则，可由 pack 维护。
- reference doc：任务流程或路由。
- tooling recipe：工具用法、参数、止损条件。
- case-only：当前进度、路径、样本状态，不回流。
