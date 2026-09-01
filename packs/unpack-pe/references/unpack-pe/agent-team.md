# Authorized PE unpacking research team pattern

## 建议角色

- **Commander**：确认 case 授权、目标和停止条件，按需组队并集成交付。
- **Analysis member**：围绕一个窄问题收集 artifact 索引、evidence 与 finding candidate，只写自己的允许范围。
- **Tooling member**：评估工具适用性、输入输出、预算和止损条件，不因工具存在而自动执行。
- **Reviewer**：只读 artifact/evidence/finding，只写 review；证据不足时把 `needs-evidence` 直接返回原 owner。

通常只需 1–2 名执行成员。每个问题默认一名 owner、最多一名 verifier；没有持续独立职责就用 tactical subagent，不创建 durable member。active durable team 不超过 3 名执行成员和 1 名 Reviewer。

## 协作规则

- 成员身份和当前任务由各自目录 `CLAUDE.md` 承载，不绑定 session ID。
- 当前主任务优先；定向共享关键发现或请求有界复核，不广播普通探索过程。
- 正式改派必须包含 expected current task 与 new task；任务已变化时返回 `HOLD_STALE_TASK`。
- sample_ref、section_ref、loader_ref、unpack_candidate_ref 只能是 case-local 脱敏引用。
- owner 写 evidence/finding；verifier 提供有界验证；Reviewer 不修改原 evidence/finding。
- heavy action 不由成员间消息授权，必须回到具体 case 授权、用户确认和工具权限边界。
