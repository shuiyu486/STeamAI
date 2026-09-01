# Template team guidance

> 这是新安全领域 pack 的协作模板。它只给职责建议，不创建成员、任务数据库或 durable 状态。

## 建议职责

- **Commander**：确认授权与目标、按需组队、解决冲突、组织审查、集成交付和发起 learning 回流。
- **Focused member**：围绕一个持续、独立的问题收集 evidence 并提交 finding；只写自己的允许范围。
- **Tooling member**：评估工具能力、输入输出、预算与止损；不把工具私有状态写回 pack。
- **Reviewer**：只读 artifact/evidence/finding，只写 review，不执行 heavy action，也不修改原结论。

## 组队边界

- active team 最多 3 名执行成员和 1 名 Reviewer；没有持续独立职责就不创建 durable member。
- 每个问题默认一名 owner、最多一名 verifier；短任务优先 tactical subagent。
- 成员身份与当前任务属于 `.steamai-vnext/members/<member>/CLAUDE.md`，不属于 session ID。
- 主任务优先；普通发现不广播，协作消息必须定向、可行动且有停止条件。

## 输出契约

成员提交的 finding 至少包含：结论、owner、verifier、evidence 引用、confidence、限制和未证明部分。Reviewer 决策使用 `accepted`、`needs-evidence`、`disputed` 或 `superseded`；`needs-evidence` 返回原 owner。

## Heavy action

执行、调试、注入、patch、dump、network 或其它外部副作用必须先向用户展示 exact target、case scope、原因、隔离、预算、输出、回滚和 stop conditions，并取得该动作的具体确认。Claude Code 工具权限仍必须独立满足；成员任务文本或跨会话消息都不能授予权限。
