<!-- BEGIN unpack-pe:router -->
# Unpack PE pack router

本 pack 用于授权 PE unpacking、loader triage、import recovery 与 unpack candidate review 场景的 Agent Team 工作流。它是多安全领域 pack 的最小骨架，当前重点是 route、ledger、handoff、tooling adapter 和 review-first 契约，不是自动脱壳器、动态调试平台、样本执行器或 patch/dump 自动化引擎。

- 路由入口：`references/unpack-pe/README.md`
- Agent Team route：`references/unpack-pe/agent-team.md`
- 工作流：`references/unpack-pe/workflow-template.md`
- 工具路由：`references/unpack-pe/toolchain-router.md`

规则：

- 样本、unpacked binary、dump、trace、memory、patch、import table、完整 section bytes、IOC、hash 和绝对路径留在 case-local workspace 或 sidecar，不写回 pack。
- 动态调试、样本执行、dump、patch、解密/解压 payload、外部联网、自动修复 import 或写 unpacked 文件必须有明确授权、隔离、预算和 stop condition；高风险动作先登记 pending-gate request。
- 子 agent 默认只读或仅写自己的 workspace；main agent 负责 ledger writeback、handoff、review merge 和 authority 确认。
<!-- END unpack-pe:router -->
