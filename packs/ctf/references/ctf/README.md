# CTF pack

`ctf` 是面向授权 CTF、靶场、课程实验与本地 challenge 分析任务的最小 Agent Team pack 骨架。它用于验证 `re-context-kits` 的多安全领域扩展边界：工作线、packet、ledger、review gate、handoff、sync/promote 和 tooling adapter 契约应保持 pack-neutral。

当前不是面向真实目标的攻击执行平台、自动利用链生成器、flag 泄漏库或远程靶场刷题器；它只提供可审计、可交接、review-first 的 case workspace 规则。

## 读取指南

本文件是 `ctf` case 的 pack-local 路由入口，不是默认必读长清单。先读 case 的 `CLAUDE.local.md` 和 `/rekit status` 第一屏；只有进入 CTF、靶场、课程实验或本地 challenge 分析任务时，再按下方路由读取对应顶部区，不要默认串读全部 reference。

新增 reference、tooling 或示例文档时，必须说明何时读取、不要默认读取什么；flag、challenge 原始文件、payload、solver 私有脚本、远程靶场地址、凭据和 case-specific 进度只保存在 case-local artifacts。

## 路由表

| 任务 | 读取文档 | 说明 |
|---|---|---|
| 接手本 pack | `workflow-template.md` | scope、轻到重路线、candidate/review/confirmed 流程。 |
| 规划多 Agent 分片 | `agent-team.md` | subagent routes、packet 输出契约、review-first 合并边界。 |
| 选择工具或 adapter | `toolchain-router.md` | challenge triage、local repro review、remote/gated action 边界。 |
| 查看通用规则 | `<templateRoot>/common/policies/agent-team.md` | 角色、packet、状态流和人工确认边界。 |
| 查看工具 adapter 规则 | `<templateRoot>/common/policies/tool-adapters.md` | capability card、sidecar 输出、heavy-tool gate。 |

## 常驻边界

- 不把 flag、challenge 原始文件、payload、solver 私有脚本、远程靶场地址、账号凭据、pcap、dump、trace 或比赛私有细节写入 pack。
- 远程连接、bruteforce、fuzz、exploit replay、高流量请求、debug、dump、patch 等外部副作用必须在授权范围内，并记录预算、止损和确认。
- 子 agent 输出 candidate / verification；main agent 负责 ledger、handoff、review merge 和 authority 确认。
- 大输出必须保存为 case-local sidecar，只在聊天和 Markdown 中引用摘要与路径。

## 维护规则

- 新工具先进入 `tooling/catalog.yml` 的 `candidate`、`auxiliary` 或 `cautious` 状态。
- 至少两个 challenge / case 重复出现的通用 CTF 工作流规则，才考虑提升到 common policy 或 runtime。
- 本 pack 的目的首先是验证多安全领域 pack 架构，不要复制 `vmp-re` 的领域细节或把具体 flag / payload / writeup 解法写入模板。
