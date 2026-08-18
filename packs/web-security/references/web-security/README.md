# Web/API security pack

`web-security` 是面向授权 Web/API 安全评估、靶场/CTF 与防御性安全工程任务的最小 Agent Team pack 骨架。它用于验证 STeamAI 的非 RE pack 扩展边界：工作线、packet、ledger、review gate、handoff、sync/promote 和 tooling adapter 契约应保持 pack-neutral。

当前不是自动化扫描器、漏洞利用器、凭据测试器或 DoS 工具；它只提供可审计、可交接、review-first 的 case workspace 规则。

## 读取指南

本文件是 `web-security` case 的 pack-local 路由入口，不是默认必读长清单。先读 case 的 `CLAUDE.local.md` 和 `/rekit status` 第一屏；只有进入 Web/API 安全评估任务时，再按下方路由读取对应顶部区，不要默认串读全部 reference。

新增 reference、tooling 或示例文档时，必须说明何时读取、不要默认读取什么；真实目标、凭据、token、request/response、scan output、payload 和 case-specific 进度只保存在 case-local artifacts。

## 路由表

| 任务 | 读取文档 | 说明 |
|---|---|---|
| 接手本 pack | `workflow-template.md` | scope、轻到重路线、candidate/review/confirmed 流程。 |
| 规划多 Agent 分片 | `agent-team.md` | subagent routes、packet 输出契约、review-first 合并边界。 |
| 选择工具或 adapter | `toolchain-router.md` | passive triage、request replay、proxy/scanner 的预算和止损。 |
| 查看通用规则 | `<templateRoot>/common/policies/agent-team.md` | 角色、packet、状态流和人工确认边界。 |
| 查看工具 adapter 规则 | `<templateRoot>/common/policies/tool-adapters.md` | capability card、sidecar 输出、heavy-tool gate。 |

## 常驻边界

- 不把真实目标、客户名、凭据、token、cookie、request/response body、HAR、pcap、scan output 或漏洞利用细节写入 pack。
- 主动请求、扫描、fuzz、登录尝试、exploit replay、数据导出等外部副作用必须在授权范围内，并记录预算、止损和确认。
- 子 agent 输出 candidate / verification；main agent 负责 ledger、handoff、review merge 和 authority 确认。
- 大输出必须保存为 case-local sidecar，只在聊天和 Markdown 中引用摘要与路径。

## 维护规则

- 新工具先进入 `tooling/catalog.yml` 的 `candidate` 或 `cautious` 状态。
- 至少两个 case 重复出现的通用 Web/API 规则，才考虑提升到 common policy 或 runtime。
- 本 pack 的目的首先是验证非 RE pack 架构，不要复制 `binary-re` 的领域细节。
