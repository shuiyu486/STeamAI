<!-- BEGIN generic-binary-re:router -->
# Generic binary RE pack router

本 pack 用于授权通用二进制逆向、static triage、function behavior analysis、API behavior review 与 tooling sidecar review 场景的 Agent Team 工作流。它是多安全领域 pack 的最小骨架，当前重点是 route、ledger、handoff、tooling adapter 和 review-first 契约，不是自动逆向引擎、样本执行器、patch 平台或漏洞/恶意样本专项 pack 的替代品。

- 路由入口：`references/generic-binary-re/README.md`
- Agent Team route：`references/generic-binary-re/agent-team.md`
- 工作流：`references/generic-binary-re/workflow-template.md`
- 工具路由：`references/generic-binary-re/toolchain-router.md`

规则：

- 样本、完整二进制、dump、trace、memory、patch、完整函数体、符号表、IOC、hash、客户上下文和绝对路径留在 case-local workspace 或 sidecar，不写回 pack。
- 动态执行、调试、trace、dump、patch、批量反编译、自动重命名/注释、外部联网或写回分析数据库必须有隔离、预算和 stop condition，并先经 `/rekit gate` preflight；只有本次显式用户确认，或 strict durable autonomy profile + 对应 `authorized-gate`，才允许 executor 执行。
- 子 agent 默认只读或仅写自己的 workspace；main agent 负责 ledger writeback、handoff、review merge 和 authority 确认。
<!-- END generic-binary-re:router -->
