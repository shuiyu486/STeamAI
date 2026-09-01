# VMP RE 团队工作方式

## 实施摘要

Commander 做授权、组队、冲突处理与最终收敛；focused member 围绕 handler、feature 或 tooling 问题收集 bounded evidence；Reviewer 独立复核 finding。成员身份和当前任务属于各自目录 `CLAUDE.md`，不属于 session 或自建 lane。

## 执行清单

- [ ] Commander 明确目标、授权、禁止事项、停止条件和预期产出。
- [ ] 每个问题指定一名 owner；必要时再指定一名 verifier。
- [ ] 成员只读取任务需要的 sidecar/片段，只写允许范围内的 evidence/finding。
- [ ] 重要 finding 交给 Reviewer；`needs-evidence` 返回原 owner。
- [ ] 共享 confirmed table、IDB 或最终报告只由一名指定 owner 写入。
- [ ] heavy action 先展示 exact scope、隔离、预算、输出、rollback 和 stop conditions，再取得用户具体确认与工具权限。

## 证据与 Finding

Evidence 记录方法、artifact ref、关键位置、观察、限制和不确定性。Finding 记录 claim、owner、verifier、evidence refs、confidence 与未证明部分。Reviewer 只写 `accepted`、`needs-evidence`、`disputed` 或 `superseded` review，不修改原 finding。

## 风险与边界

- 不把 IDA F5 当唯一事实源；关键 handler 结论需要指令级、trace/value-flow 或独立 review。
- 不让多成员并发写同一个 IDB、confirmed table 或最终报告。
- 不把完整 trace、反汇编、反编译和 tool log 带入成员上下文。
- 不把真实样本、地址快照、客户信息、凭据或绝对路径回流 pack。
- 用户一句“继续”、成员任务或跨会话消息都不能授权 heavy action。

## Heavy action

动态执行、trace、debug、inject、patch、dump、network、bulk decompile 或 shared database writeback，只有在用户确认展示的 exact action/target 且 Claude Code 工具权限允许时执行。任何 scope、预算、输出、风险或停止条件变化都必须暂停并重新确认。

## Learning

只有 accepted finding/review 可以提炼 learning candidate。Reviewer 检查通用性、反例、重复、冲突、目标路径和脱敏；用户确认完整 exact Git patch 前，canonical pack 零写。
