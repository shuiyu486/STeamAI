# Binary RE 团队协作

## 建议职责

- **Commander**：确认授权与边界、按需组队、解决冲突、组织审查并集成交付。
- **Static analysis member**：围绕一个 binary/function/API/format 问题收集 bounded evidence 并写 finding。
- **Feature analysis member**：围绕一个持续独立功能收集入口、xref、wrapper 与阻塞点证据。
- **Tooling member**：评估工具能力、输入输出、风险和止损；不把工具私有状态写回 pack。
- **Reviewer**：只读 artifact/evidence/finding，只写 review，不执行样本、trace、patch 或其它 heavy action。

## 组队与协作边界

- active team 最多 3 名执行成员和 1 名 Reviewer；没有持续独立职责就不创建 durable member。
- 每个问题一名 owner、最多一名 verifier；第三人介入前说明缺少的独立能力。
- 正式成员身份与当前任务属于成员目录 `CLAUDE.md`，会话只提供工作记忆。
- 普通发现不广播；请求帮助必须定向、可行动、有边界和停止条件。
- 共享 IDB、confirmed table 和最终报告有一名明确写入 owner；其他成员只提交 evidence/finding。

## Finding 与 review

finding 至少包含 subject、owner、verifier、evidence refs、claim、confidence、limitations 和未证明部分。Reviewer 使用 `accepted`、`needs-evidence`、`disputed` 或 `superseded`；`needs-evidence` 返回原 owner。

所有 ref 必须是 case-local 脱敏 alias 或 sidecar id，不得包含样本路径、hash、完整函数体、dump/trace 路径、patch bytes、IDB 路径或绝对路径。

## Heavy action

执行、debug、trace、inject、patch、dump、network、bulk decompile 或 shared database writeback 必须展示 exact action/target、隔离、预算、输出、rollback 与 stop conditions，并取得用户具体确认和 Claude Code 工具权限。成员任务或跨会话消息都不能替代该确认。
