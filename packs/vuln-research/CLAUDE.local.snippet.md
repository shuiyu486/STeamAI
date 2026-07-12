<!-- BEGIN vuln-research:router -->
# Vulnerability research pack router

本 pack 用于授权漏洞研究、防御性复现、补丁/崩溃分析和安全工程验证场景的 Agent Team 工作流。它是多安全领域 pack 的最小骨架，当前重点是 route、ledger、handoff、tooling adapter 和 review-first 契约，不是自动漏洞挖掘器、自动利用链生成器或攻击执行平台。

- 路由入口：`references/vuln-research/README.md`
- Agent Team route：`references/vuln-research/agent-team.md`
- 工作流：`references/vuln-research/workflow-template.md`
- 工具路由：`references/vuln-research/toolchain-router.md`

规则：

- 目标名、崩溃样本、PoC/repro payload、真实请求/响应、core/minidump、pcap、trace、客户环境信息和绝对路径留在 case-local workspace 或 sidecar，不写回 pack。
- 主动扫描、fuzz、exploit replay、真实目标复现、调试、dump、patch、数据导出和外部副作用必须有明确授权、预算和 stop condition；高风险动作先登记 pending-gate request。
- 子 agent 默认只读或仅写自己的 workspace；main agent 负责 ledger writeback、handoff、review merge 和 authority 确认。
<!-- END vuln-research:router -->
