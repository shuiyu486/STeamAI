# Binary RE 按需路由入口

> `binary-re` 覆盖授权二进制逆向的 passive static triage、function/API behavior review、bounded sidecar inspection 与 review-first 动态升级。VMProtect trace/devirtualization 是专项 workflow/recipe，不是默认开局。

## 常驻原则

- 先读 case 的 `.steamai-vnext/CLAUDE.md` 与本文件，再只选择一个任务入口。
- 当前任务、成员身份和授权边界来自 case/member `CLAUDE.md`；pack 不保存 case 进度。
- 原始二进制和工具输出保留在 case-local artifact/evidence；pack 只保存可复用流程。
- 每个问题一名 owner、最多一名 verifier；重要 finding 由独立 Reviewer 复核。

## 路由表

| 任务 | 读取文档 | 说明 |
|---|---|---|
| 通用 binary/function/API 分析 | `general-analysis.md`、`general-workflow.md` | 从 passive sidecar 到 finding/review 的轻到重路线。 |
| 团队分工与复核 | `general-agent-team.md` | Commander、focused member、verifier 与 Reviewer。 |
| 通用工具选择 | `general-toolchain-router.md`、`../../tooling/README.md` | static triage、saved summary review、风险和止损。 |
| VMProtect x64 专项 | `workflow-template.md`、`agent-driven-re.md` | VMEnter、context、trace、handler value-flow 与 review。 |
| VMP/IDA 工具选择 | `toolchain-router.md` | 公开工具短测、IDA sidecar、debug/trace 边界。 |
| 批量 handler/trace 复核 | `progressive-disclosure.md`、`singleton-handler-review.md` | 固定分片、读取预算与低样本复核。 |
| 功能分析协同 | `lane-collaboration.md` | 目录成员与单写者边界；不创建 lane 状态机。 |

## 写入与执行边界

- 不把样本、完整二进制、hash、IOC、dump、trace、memory snapshot、patch、完整函数体、符号表、客户上下文或绝对路径写入 pack。
- 大输出保存为 case-local sidecar；Markdown 只引用脱敏 alias、row id、摘要和相对证据位置。
- debug、trace、inject、dump、patch、network、symex、批量反编译、rename/comment 或 database writeback 必须展示 exact target、隔离、预算、输出、回滚和 stop conditions，并取得用户对该具体动作的确认及 Claude Code 工具权限。
- learning 只从 accepted finding/review 提炼；用户确认完整 exact patch 前不写 canonical pack。
