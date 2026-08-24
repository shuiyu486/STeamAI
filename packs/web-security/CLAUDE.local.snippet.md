<!-- BEGIN web-security:router -->
# Web/API security pack router

本 pack 用于授权 Web/API 安全评估、靶场/CTF 或防御性安全工程场景的 Agent Team 工作流。它是 mature production pack，固定能力是 OpenAPI 3 JSON typed inventory、content-addressed bounded loopback replay、durable evidence 与独立 Reviewer；不是自动化扫描或攻击平台。

- 路由入口：`references/web-security/README.md`
- Agent Team route：`references/web-security/agent-team.md`
- 工作流：`references/web-security/workflow-template.md`
- 工具路由：`references/web-security/toolchain-router.md`

规则：

- 目标 URL、凭据、请求/响应、HAR、pcap、Burp/ZAP 项目、漏洞细节和客户信息留在 case-local workspace 或 sidecar，不写回 pack。
- 外部请求、主动扫描、登录尝试、fuzz、bruteforce、exploit replay、DoS 类动作必须有预算和 stop condition，并先经 project-local gate preflight；只有本次显式用户确认，或 strict durable autonomy profile + 对应 `authorized-gate`，才允许 executor 执行。Fixed replay 仍只允许 exact loopback/injected transport。
- 子 agent 默认只读或仅写自己的 workspace；main agent 负责 ledger writeback、handoff、review merge 和 authority 确认。
<!-- END web-security:router -->
