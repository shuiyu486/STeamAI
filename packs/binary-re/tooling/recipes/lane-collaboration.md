# 成员协同 recipe

目的：为 VMProtect RE case 建立可观察、可纠偏、可独立复核的成员分工，不创建 lane 状态机。

## 建立成员

1. Commander 先判断是否有持续且独立的 handler、feature 或 tooling 工作流。
2. 为需要的 durable member 创建专属目录和 `CLAUDE.md`，写明当前任务、允许读写、输入、输出和停止条件。
3. 每个问题一名 owner、最多一名 verifier；重要 finding 再交给 Reviewer。
4. 共享 confirmed table、IDB 和最终报告只有一名明确写入 owner。

## Case-local workspace

```text
.steamai-vnext/members/<member>/
  CLAUDE.md
  workspace/
```

成员通过原生 Claude Code session 保存工作记忆，并通过定向跨会话消息共享关键 finding 或请求有界验证。消息不能冒充用户纠偏或扩大授权。

## 止损

- workspace 长期只有 request 没有 evidence 时，退回补证据或结束该成员。
- 需要底层 VM 语义时向 handler owner 发送定向 request，不在 feature member 中硬猜。
- 不把完整 trace、反编译或 case 路径复制进成员说明或 pack。
