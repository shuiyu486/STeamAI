<!-- BEGIN ollvm:router -->
# OLLVM pack router

本 pack 用于授权 OLLVM / native obfuscation analysis、control-flow flattening triage、opaque predicate review、MBA simplification 与 deobfuscation candidate review 场景的 Agent Team 工作流。它是多安全领域 pack 的最小骨架，当前重点是 route、ledger、handoff、tooling adapter 和 review-first 契约，不是自动反混淆引擎、patch 平台、符号恢复器或样本执行器。

- 路由入口：`references/ollvm/README.md`
- Agent Team route：`references/ollvm/agent-team.md`
- 工作流：`references/ollvm/workflow-template.md`
- 工具路由：`references/ollvm/toolchain-router.md`

规则：

- 样本、反混淆后二进制、patch、trace、dump、完整 CFG、函数体 raw dump、符号表、IOC、hash 和绝对路径留在 case-local workspace 或 sidecar，不写回 pack。
- 动态执行、调试、trace、dump、patch、批量反编译、自动重命名、自动写注释、自动导出反混淆二进制或外部联网必须有明确授权、隔离、预算和 stop condition；高风险动作先登记 pending-gate request。
- 子 agent 默认只读或仅写自己的 workspace；main agent 负责 ledger writeback、handoff、review merge 和 authority 确认。
<!-- END ollvm:router -->
