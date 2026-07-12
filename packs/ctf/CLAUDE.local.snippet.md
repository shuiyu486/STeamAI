<!-- BEGIN ctf:router -->
# CTF pack router

本 pack 用于授权 CTF、靶场、课程实验和本地 challenge 分析场景的 Agent Team 工作流。它是多安全领域 pack 的最小骨架，当前重点是 route、ledger、handoff、tooling adapter 和 review-first 契约，不是面向真实目标的攻击执行平台或自动利用链平台。

- 路由入口：`references/ctf/README.md`
- Agent Team route：`references/ctf/agent-team.md`
- 工作流：`references/ctf/workflow-template.md`
- 工具路由：`references/ctf/toolchain-router.md`

规则：

- flag、payload、solver、challenge 原始文件、pcap、dump、trace、远程靶场地址、账号凭据和绝对路径留在 case-local workspace 或 sidecar，不写回 pack。
- 远程连接、bruteforce、fuzz、exploit replay、高流量请求、真实目标访问、debug/dump/patch 和外部副作用必须有明确授权、预算和 stop condition；高风险动作先登记 pending-gate request。
- 子 agent 默认只读或仅写自己的 workspace；main agent 负责 ledger writeback、handoff、review merge 和 authority 确认。
<!-- END ctf:router -->
